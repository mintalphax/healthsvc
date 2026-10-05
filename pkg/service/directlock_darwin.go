//go:build darwin

package service

import "healthsvc/pkg/lockscreen"

// directLockFunc returns the root-context direct lock used as a fallback when
// the user-session agent fails to consume the trigger file (macOS only). It
// prefers kickstarting the agent so the lock happens in user context;
// triggerPath lets it wait for the agent to consume the trigger.
func directLockFunc() func(triggerPath string) error { return lockscreen.LockScreenDirect }

// nudgeFunc returns a hook called right after the daemon fires the trigger:
// root kickstarts the agent so delivery never depends on WatchPaths alone.
func nudgeFunc() func() error { return lockscreen.KickstartAgent }
