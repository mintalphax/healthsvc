//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"healthsvc/pkg/logger"
	"healthsvc/pkg/service"
)

func isWindowsService() (bool, error) { return service.IsWindowsService() }

func runWindowsService() {
	exe, err := os.Executable()
	if err != nil {
		os.Exit(1)
	}
	baseDir := filepath.Dir(exe)
	log, err := logger.New(filepath.Join(baseDir, "logs"), "health")
	if err != nil {
		os.Exit(1)
	}
	svc, err := service.New(baseDir, log, false)
	if err != nil {
		log.Errorf("init service: %v", err)
		os.Exit(1)
	}
	if err := service.RunAsService(svc); err != nil {
		log.Errorf("service run failed: %v", err)
		os.Exit(1)
	}
}

// defaultRunsDaemon: on Windows a bare invocation shows usage instead of
// running; the service is started by the service control manager.
func defaultRunsDaemon() bool { return false }

// runAgent has no role on Windows: locking is done by the per-user scheduled
// task (monitor.bat) created by scripts-windows/create_task.ps1.
func runAgent(baseDir string, log *logger.Logger, dryRun bool) {
	log.Errorf("-agent is not supported on windows")
}

// platformInstall registers the Windows service (the per-user scheduled task
// is created separately by scripts-windows/create_task.ps1).
func platformInstall(baseDir string, log *logger.Logger) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}
	svc, err := service.New(baseDir, log, false)
	if err != nil {
		return err
	}
	cfg := svc.ConfigManager().GetConfig()
	if err := service.InstallWindowsService(exe, cfg.Service); err != nil {
		return err
	}
	log.Infof("windows service %q installed (exe: %s)", cfg.Service.Name, exe)
	return nil
}

func platformUninstall(baseDir string, log *logger.Logger) error {
	svc, err := service.New(baseDir, log, false)
	if err != nil {
		return err
	}
	name := svc.ConfigManager().GetConfig().Service.Name
	if err := service.UninstallWindowsService(name); err != nil {
		return err
	}
	log.Infof("windows service %q removed", name)
	return nil
}
