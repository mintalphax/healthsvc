//go:build darwin

package service

import "healthsvc/pkg/lockscreen"

// directLockFunc returns the root-context direct lock used as a fallback when
// the user-session agent fails to consume the trigger file (macOS only: the
// LaunchDaemon runs as root and pmset is setuid).
func directLockFunc() func() error { return lockscreen.LockScreenDirect }
