//go:build !darwin

package service

// directLockFunc returns nil outside macOS: Windows cannot lock the
// interactive session from the SYSTEM service, and the scheduled task covers
// the trigger anyway.
func directLockFunc() func(triggerPath string) error { return nil }

// nudgeFunc returns nil outside macOS (Windows delivers the trigger via the
// per-minute scheduled task).
func nudgeFunc() func() error { return nil }
