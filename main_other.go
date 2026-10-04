//go:build !windows && !darwin

package main

import (
	"errors"

	"healthsvc/pkg/logger"
)

func isWindowsService() (bool, error) { return false, nil }

func runWindowsService() {}

func defaultRunsDaemon() bool { return true }

func runAgent(baseDir string, log *logger.Logger, dryRun bool) {
	log.Errorf("-agent is not supported on this platform")
}

func platformInstall(baseDir string, log *logger.Logger) error {
	return errors.New("install is only supported on Windows and macOS")
}

func platformUninstall(baseDir string, log *logger.Logger) error {
	return errors.New("uninstall is only supported on Windows and macOS")
}
