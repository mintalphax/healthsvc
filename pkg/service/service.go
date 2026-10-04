// Package service implements the scheduler core: a loop that checks the
// (NTP-trusted) time against the configured lock times and fires the trigger
// file for the per-user lock component. It is platform-neutral; the Windows
// service wrapper lives in windows_svc.go, macOS runs the same Run loop under
// launchd.
package service

import (
	"path/filepath"
	"time"

	"healthsvc/pkg/config"
	"healthsvc/pkg/lockscreen"
	"healthsvc/pkg/logger"
	"healthsvc/pkg/ntp"
)

const (
	// stateDateLayout is the entry format of _state.dat
	// ("2026-10-03-23:10").
	stateDateLayout = "2006-01-02-15:04"
	// lastCheckLayout is the "last_check" field format.
	lastCheckLayout = "2006-01-02 15:04:05"

	stateFileName     = "_state.dat"
	triggerFileName   = "_h.dat"
	keepStateDays     = 7
	fallbackLockDelay = 90 * time.Second
)

// LockService is the scheduler daemon.
type LockService struct {
	cfgMgr     *config.Manager
	log        *logger.Logger
	ntpClient  *ntp.Client
	trigger    lockscreen.Trigger
	statePath  string
	directLock func() error // root-context fallback (macOS); nil on Windows
	dryRun     bool

	state    State
	firedAt  time.Time
	stopCh   chan struct{}
	cfgChg   chan struct{}
	stopOnce chan struct{}
}

// New builds a LockService rooted at baseDir (the binary's directory:
// configs/, logs/, _state.dat, _h.dat all live there).
func New(baseDir string, log *logger.Logger, dryRun bool) (*LockService, error) {
	cfgMgr, err := config.NewManager(filepath.Join(baseDir, "configs", "config.yaml"))
	if err != nil {
		return nil, err
	}
	cfg := cfgMgr.GetConfig()
	s := &LockService{
		cfgMgr:     cfgMgr,
		log:        log,
		ntpClient:  &ntp.Client{Servers: cfg.NTP.Servers},
		trigger:    lockscreen.NewFileTrigger(filepath.Join(baseDir, triggerFileName)),
		statePath:  filepath.Join(baseDir, stateFileName),
		dryRun:     dryRun,
		directLock: directLockFunc(),
		stopCh:     make(chan struct{}),
		cfgChg:     make(chan struct{}, 1),
		stopOnce:   make(chan struct{}),
	}
	s.loadState()
	return s, nil
}

// Stop asks the run loop to finish (idempotent).
func (s *LockService) Stop() {
	select {
	case <-s.stopOnce:
	default:
		close(s.stopOnce)
		close(s.stopCh)
	}
}

// Run is the main loop; it returns after Stop or on a fatal config error.
func (s *LockService) Run() error {
	cfg := s.cfgMgr.GetConfig()
	s.log.Infof("health service starting (lock times: %v, weekdays: %v, enable: %v, check interval: %ds)",
		cfg.Schedule.LockTimes, cfg.Schedule.Weekdays, cfg.Schedule.Enable, cfg.Schedule.CheckInterval)
	defer s.cfgMgr.Stop()

	s.cfgMgr.StartWatcher(10*time.Second, func() {
		select {
		case s.cfgChg <- struct{}{}:
		default:
		}
	})

	interval := cfg.GetCheckInterval()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	s.tick() // check immediately: a machine booted past its lock time must lock at once

	for {
		select {
		case <-s.stopCh:
			s.log.Infof("health service stopped")
			return nil
		case <-s.cfgChg:
			newInterval := s.cfgMgr.GetConfig().GetCheckInterval()
			s.log.Infof("config reloaded (check interval: %ds)", s.cfgMgr.GetConfig().Schedule.CheckInterval)
			if newInterval != interval {
				ticker.Reset(newInterval)
				interval = newInterval
			}
		case <-ticker.C:
			s.tick()
		}
	}
}

// tick performs one schedule check.
func (s *LockService) tick() {
	if _, err := s.cfgMgr.PollReload(); err != nil {
		s.log.Warnf("config reload failed (keeping previous): %v", err)
	}

	now, source := s.trustedNow()
	if now.IsZero() {
		return // clock untrusted and local time disallowed: skip this round
	}

	s.checkAndLock(now, source)
	s.verifyFired()

	s.state.LastCheck = now.Format(lastCheckLayout)
	s.saveState()
}

// trustedNow returns the current trusted time and its source ("ntp:host" or
// "local"). A zero time means "no trusted time available and local time is
// disallowed".
func (s *LockService) trustedNow() (time.Time, string) {
	cfg := s.cfgMgr.GetConfig()
	res, err := s.ntpClient.GetTime(time.Now())
	if err == nil {
		if cfg.NTP.MaxTimeOffset > 0 && (res.Offset > time.Duration(cfg.NTP.MaxTimeOffset)*time.Second ||
			res.Offset < -time.Duration(cfg.NTP.MaxTimeOffset)*time.Second) {
			s.log.Warnf("local clock offset %v exceeds allowed %ds (using NTP time from %s, rtt %v)",
				res.Offset, cfg.NTP.MaxTimeOffset, res.Server, res.RTT)
		} else {
			s.log.Debugf("ntp time ok from %s (offset %v, rtt %v)", res.Server, res.Offset, res.RTT)
		}
		return res.Time, "ntp:" + res.Server
	}

	s.log.Warnf("ntp time unavailable: %v", err)
	if cfg.NTP.AllowLocalTime {
		return time.Now(), "local"
	}
	s.log.Errorf("local time fallback is disabled (ntp.allow_local_time=false); skipping this check")
	return time.Time{}, ""
}

// checkAndLock fires the trigger for every due, not-yet-locked time today.
func (s *LockService) checkAndLock(now time.Time, source string) {
	cfg := s.cfgMgr.GetConfig()
	if !cfg.ShouldLockToday(now) {
		return
	}
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	for _, d := range cfg.LockTimesOfDay() {
		at := midnight.Add(d)
		if now.Before(at) {
			continue
		}
		key := at.Format(stateDateLayout)
		if s.isLockedDate(key) {
			continue
		}
		if s.dryRun {
			s.log.Infof("[DRY-RUN] due at %s: would fire trigger %s", key, s.trigger.Path())
		} else {
			if err := s.trigger.Fire(now); err != nil {
				s.log.Errorf("fire trigger for %s failed: %v", key, err)
				continue
			}
			s.log.Infof("lock time %s reached (time source %s); trigger fired", key, source)
			s.firedAt = time.Now()
		}
		s.markLocked(key)
		s.log.Infof("recorded lock %s in state", key)
	}
	s.pruneState(now)
}

// verifyFired is the anti-tamper fallback: if the trigger file was not
// consumed by the user-session component within fallbackLockDelay (e.g. the
// LaunchAgent was unloaded), lock directly if the platform allows it.
func (s *LockService) verifyFired() {
	if s.firedAt.IsZero() || !s.trigger.Exists() {
		return
	}
	if time.Since(s.firedAt) < fallbackLockDelay {
		return
	}
	if s.directLock == nil {
		s.log.Warnf("trigger %s not consumed within %v and no direct lock available on this platform; check the user-session task/agent",
			s.trigger.Path(), fallbackLockDelay)
		s.firedAt = time.Now() // warn once per interval
		return
	}
	s.log.Warnf("trigger not consumed within %v; performing direct lock fallback", fallbackLockDelay)
	if err := s.directLock(); err != nil {
		s.log.Errorf("direct lock failed: %v", err)
	}
	if err := s.trigger.Clear(); err != nil {
		s.log.Errorf("clear trigger failed: %v", err)
	}
	s.firedAt = time.Time{}
}

// Path accessors used by the platform installers (WatchPaths on macOS).
func (s *LockService) TriggerPath() string { return s.trigger.Path() }

// ConfigManager exposes the manager for platform install helpers.
func (s *LockService) ConfigManager() *config.Manager { return s.cfgMgr }
