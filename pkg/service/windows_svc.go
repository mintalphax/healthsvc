//go:build windows

package service

import (
	"fmt"
	"strings"
	"time"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/eventlog"
	"golang.org/x/sys/windows/svc/mgr"

	"healthsvc/pkg/config"
)

// WindowsService adapts LockService to the Windows service control manager.
type WindowsService struct {
	lock *LockService
}

// NewWindowsService wraps a LockService as an SCM handler.
func NewWindowsService(lock *LockService) *WindowsService {
	return &WindowsService{lock: lock}
}

// Execute implements svc.Handler.
func (ws *WindowsService) Execute(args []string, r <-chan svc.ChangeRequest, status chan<- svc.Status) (bool, uint32) {
	const accepted = svc.AcceptStop | svc.AcceptShutdown
	status <- svc.Status{State: svc.StartPending}

	done := make(chan struct{})
	go func() {
		ws.lock.Run()
		close(done)
	}()

	status <- svc.Status{State: svc.Running, Accepts: accepted}

	for {
		select {
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				status <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				status <- svc.Status{State: svc.StopPending}
				ws.lock.Stop()
				<-done
				status <- svc.Status{State: svc.Stopped}
				return false, 0
			default:
				// Unknown command: ignore.
			}
		case <-done:
			status <- svc.Status{State: svc.Stopped}
			return false, 0
		}
	}
}

// IsWindowsService reports whether we were launched by the SCM.
func IsWindowsService() (bool, error) {
	return svc.IsWindowsService()
}

// RunAsService blocks running the service handler (call after IsWindowsService).
func RunAsService(lock *LockService) error {
	return svc.Run(lock.ConfigManager().GetConfig().Service.Name, NewWindowsService(lock))
}

// InstallWindowsService registers the service with auto start and restart
// recovery.
func InstallWindowsService(exePath string, sc config.ServiceConfig) error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connect service manager (run as admin): %w", err)
	}
	defer m.Disconnect()

	if s, err := m.OpenService(sc.Name); err == nil {
		s.Close()
		return fmt.Errorf("service %q already exists; remove it first (sc delete %s)", sc.Name, sc.Name)
	}

	s, err := m.CreateService(sc.Name, exePath, mgr.Config{
		DisplayName: sc.DisplayName,
		Description: sc.Description,
		StartType:   mgr.StartAutomatic,
	})
	if err != nil {
		return fmt.Errorf("create service: %w", err)
	}
	defer s.Close()

	// Restart the service 10s after a crash; reset the failure counter after a minute.
	_ = s.SetRecoveryActions([]mgr.RecoveryAction{
		{Type: mgr.ServiceRestart, Delay: 10 * time.Second},
	}, uint32(60))

	if err := eventlog.InstallAsEventCreate(sc.Name, eventlog.Error|eventlog.Warning|eventlog.Info); err != nil {
		if !strings.Contains(err.Error(), "Exists") {
			return fmt.Errorf("install event log source: %w", err)
		}
	}
	return nil
}

// UninstallWindowsService removes the service registration. The service is
// stopped first: Delete() only marks the service for deletion, and a running
// instance keeps running (and writing state files) until it is stopped.
func UninstallWindowsService(name string) error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connect service manager (run as admin): %w", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(name)
	if err != nil {
		return fmt.Errorf("open service %s: %w", name, err)
	}
	defer s.Close()

	if status, err := s.Query(); err == nil && status.State != svc.Stopped {
		if _, err := s.Control(svc.Stop); err != nil {
			return fmt.Errorf("send stop to service %s: %w (try: sc stop %s)", name, err, name)
		}
		deadline := time.Now().Add(15 * time.Second)
		for {
			st, err := s.Query()
			if err == nil && st.State == svc.Stopped {
				break
			}
			if time.Now().After(deadline) {
				return fmt.Errorf("service %s did not stop within 15s; stop it manually (sc stop %s) and retry",
					name, name)
			}
			time.Sleep(300 * time.Millisecond)
		}
	}

	if err := s.Delete(); err != nil {
		return fmt.Errorf("delete service: %w", err)
	}
	_ = eventlog.Remove(name)
	return nil
}
