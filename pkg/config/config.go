// Package config loads and validates configs/config.yaml and keeps the
// in-memory copy hot-reloadable (poll based, works on both Windows and macOS).
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	// Embedded zoneinfo so pinned schedule.timezones resolve on Windows too
	// (Windows has no /usr/share/zoneinfo).
	_ "time/tzdata"

	"gopkg.in/yaml.v3"
)

const (
	defaultCheckInterval = 30
	defaultMaxRetries    = 3
	defaultRetryInterval = 5
	defaultMaxTimeOffset = 300

	defaultMaxTimeOffsetDisabled = 0
)

var defaultNTPServers = []string{
	"pool.ntp.org",
	"time.apple.com",
	"time.windows.com",
	"time.google.com",
	"cn.pool.ntp.org",
}

// ServiceConfig describes the Windows service registration (ignored on macOS).
type ServiceConfig struct {
	Name        string `yaml:"name"`
	DisplayName string `yaml:"display_name"`
	Description string `yaml:"description"`
}

// ScheduleConfig controls when the screen gets locked.
type ScheduleConfig struct {
	LockTimes     []string `yaml:"lock_times"`
	Weekdays      []string `yaml:"weekdays"`
	Enable        bool     `yaml:"enable"`
	CheckInterval int      `yaml:"check_interval"`
	// Timezone optionally pins the zone used to interpret lock_times and
	// weekdays (IANA name, e.g. "Asia/Shanghai"). Empty = the machine's
	// system timezone.
	Timezone string `yaml:"timezone"`
}

// NTPConfig controls trusted time acquisition.
type NTPConfig struct {
	Servers        []string `yaml:"servers"`
	MaxRetries     int      `yaml:"max_retries"`
	RetryInterval  int      `yaml:"retry_interval"`
	AllowLocalTime bool     `yaml:"allow_local_time"`
	MaxTimeOffset  int      `yaml:"max_time_offset"`
}

// Config is the whole config.yaml file.
type Config struct {
	Service  ServiceConfig  `yaml:"service"`
	Schedule ScheduleConfig `yaml:"schedule"`
	NTP      NTPConfig      `yaml:"ntp"`

	lockTimes []time.Duration
	weekdays  []time.Weekday
	wildcard  bool
	loc       *time.Location // pinned schedule timezone; nil = system local
}

// Load reads and validates a config file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config %s: %w", path, err)
	}
	return &cfg, nil
}

// Validate fills defaults and checks the values, parsing schedule entries once.
func (c *Config) Validate() error {
	if strings.TrimSpace(c.Service.Name) == "" {
		c.Service.Name = "KeepHealthService"
	}
	if strings.TrimSpace(c.Service.DisplayName) == "" {
		c.Service.DisplayName = "System Health Service"
	}
	if c.Schedule.CheckInterval <= 0 {
		c.Schedule.CheckInterval = defaultCheckInterval
	}
	if len(c.Schedule.LockTimes) == 0 {
		return fmt.Errorf("schedule.lock_times is empty")
	}
	times, err := parseLockTimes(c.Schedule.LockTimes)
	if err != nil {
		return err
	}
	c.lockTimes = times

	weekdays, wildcard, err := parseWeekdays(c.Schedule.Weekdays)
	if err != nil {
		return err
	}
	c.weekdays = weekdays
	c.wildcard = wildcard

	if tz := strings.TrimSpace(c.Schedule.Timezone); tz == "" {
		c.loc = nil
	} else {
		loc, err := time.LoadLocation(tz)
		if err != nil {
			return fmt.Errorf("schedule.timezone %q is not a valid IANA zone name "+
				"(common zones are listed in the config comments; verify with healthsvc -check): %w", tz, err)
		}
		c.loc = loc
	}

	if len(c.NTP.Servers) == 0 {
		c.NTP.Servers = append([]string(nil), defaultNTPServers...)
	}
	if c.NTP.MaxRetries <= 0 {
		c.NTP.MaxRetries = defaultMaxRetries
	}
	if c.NTP.RetryInterval <= 0 {
		c.NTP.RetryInterval = defaultRetryInterval
	}
	if c.NTP.MaxTimeOffset < 0 {
		c.NTP.MaxTimeOffset = defaultMaxTimeOffsetDisabled
	}
	return nil
}

// GetCheckInterval returns the schedule poll interval.
func (c *Config) GetCheckInterval() time.Duration {
	return time.Duration(c.Schedule.CheckInterval) * time.Second
}

// GetRetryInterval returns the NTP retry interval.
func (c *Config) GetRetryInterval() time.Duration {
	return time.Duration(c.NTP.RetryInterval) * time.Second
}

// LockTimesOfDay returns parsed lock times as durations since midnight.
func (c *Config) LockTimesOfDay() []time.Duration {
	return c.lockTimes
}

// Location returns the pinned schedule timezone, or the system's local zone
// when schedule.timezone is empty.
func (c *Config) Location() *time.Location {
	if c.loc != nil {
		return c.loc
	}
	return time.Local
}

// ShouldLockToday reports whether the schedule is enabled and now's weekday
// (interpreted in the schedule timezone) matches. An empty weekdays list
// (or "*") means every day.
func (c *Config) ShouldLockToday(now time.Time) bool {
	if !c.Schedule.Enable {
		return false
	}
	if c.wildcard || len(c.weekdays) == 0 {
		return true
	}
	wd := now.In(c.Location()).Weekday()
	for _, w := range c.weekdays {
		if w == wd {
			return true
		}
	}
	return false
}

func parseLockTimes(raw []string) ([]time.Duration, error) {
	seen := map[time.Duration]bool{}
	out := make([]time.Duration, 0, len(raw))
	for _, s := range raw {
		parts := strings.Split(strings.TrimSpace(s), ":")
		if len(parts) != 2 {
			return nil, fmt.Errorf("lock time %q is not HH:MM", s)
		}
		var h, m int
		if _, err := fmt.Sscanf(parts[0], "%d", &h); err != nil || h < 0 || h > 23 {
			return nil, fmt.Errorf("lock time %q has invalid hour", s)
		}
		if _, err := fmt.Sscanf(parts[1], "%d", &m); err != nil || m < 0 || m > 59 {
			return nil, fmt.Errorf("lock time %q has invalid minute", s)
		}
		d := time.Duration(h)*time.Hour + time.Duration(m)*time.Minute
		if !seen[d] {
			seen[d] = true
			out = append(out, d)
		}
	}
	return out, nil
}

var weekdayNames = map[string]time.Weekday{
	"sunday": time.Sunday, "sun": time.Sunday,
	"monday": time.Monday, "mon": time.Monday,
	"tuesday": time.Tuesday, "tue": time.Tuesday,
	"wednesday": time.Wednesday, "wed": time.Wednesday,
	"thursday": time.Thursday, "thu": time.Thursday,
	"friday": time.Friday, "fri": time.Friday,
	"saturday": time.Saturday, "sat": time.Saturday,
}

func parseWeekdays(raw []string) ([]time.Weekday, bool, error) {
	var out []time.Weekday
	for _, s := range raw {
		s = strings.TrimSpace(strings.ToLower(s))
		if s == "" {
			continue
		}
		if s == "*" {
			return nil, true, nil
		}
		wd, ok := weekdayNames[s]
		if !ok {
			return nil, false, fmt.Errorf("unknown weekday %q (use e.g. Monday or *)", s)
		}
		out = append(out, wd)
	}
	return out, false, nil
}

// Manager owns the current *Config and reloads it when the file changes.
type Manager struct {
	mu      sync.RWMutex
	path    string
	cfg     *Config
	modTime time.Time
	size    int64

	stopOnce sync.Once
	stopCh   chan struct{}
}

// NewManager loads the config file once; it errors if the file is missing.
func NewManager(path string) (*Manager, error) {
	m := &Manager{path: path, stopCh: make(chan struct{})}
	if err := m.Reload(); err != nil {
		return nil, err
	}
	return m, nil
}

// GetConfig returns the current config snapshot.
func (m *Manager) GetConfig() *Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cfg
}

// Path returns the watched config file path.
func (m *Manager) Path() string { return m.path }

// Reload re-reads the config file. On parse errors the previous config is kept.
func (m *Manager) Reload() error {
	cfg, err := Load(m.path)
	if err != nil {
		return err
	}
	info, statErr := os.Stat(m.path)
	m.mu.Lock()
	m.cfg = cfg
	if statErr == nil {
		m.modTime = info.ModTime()
		m.size = info.Size()
	}
	m.mu.Unlock()
	return nil
}

// PollReload reloads the config if the file changed; it returns true when a
// new config was picked up.
func (m *Manager) PollReload() (bool, error) {
	info, err := os.Stat(m.path)
	if err != nil {
		return false, err
	}
	m.mu.RLock()
	changed := !info.ModTime().Equal(m.modTime) || info.Size() != m.size
	m.mu.RUnlock()
	if !changed {
		return false, nil
	}
	if err := m.Reload(); err != nil {
		return false, err
	}
	return true, nil
}

// StartWatcher polls the config file in the background and calls onChange
// whenever a new config has been loaded.
func (m *Manager) StartWatcher(interval time.Duration, onChange func()) {
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-m.stopCh:
				return
			case <-t.C:
				changed, err := m.PollReload()
				if changed && err == nil && onChange != nil {
					onChange()
				}
			}
		}
	}()
}

// Stop stops the background watcher.
func (m *Manager) Stop() {
	m.stopOnce.Do(func() { close(m.stopCh) })
}

// Dir returns the directory containing the config file.
func (m *Manager) Dir() string { return filepath.Dir(m.path) }
