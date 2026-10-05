//go:build !windows && !darwin

package service

// directLockFunc returns nil on platforms without a lock implementation.
func directLockFunc() func(triggerPath string) error { return nil }

// nudgeFunc returns nil on platforms without a per-user task to kick.
func nudgeFunc() func() error { return nil }
