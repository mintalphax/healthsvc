//go:build !darwin

package service

// directLockFunc returns nil outside macOS: Windows cannot lock the
// interactive session from the SYSTEM service, and the scheduled task covers
// the trigger anyway.
func directLockFunc() func() error { return nil }
