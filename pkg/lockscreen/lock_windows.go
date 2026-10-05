//go:build windows

package lockscreen

import (
	"fmt"
	"syscall"
)

var (
	user32          = syscall.NewLazyDLL("user32.dll")
	procLockWorkSta = user32.NewProc("LockWorkStation")
)

// LockScreen locks the workstation (Windows). It only works when the calling
// process runs in the interactive user session; the system service therefore
// uses the trigger file + per-user scheduled task instead.
func LockScreen() error {
	ret, _, err := procLockWorkSta.Call()
	if ret == 0 {
		if err != syscall.Errno(0) {
			return fmt.Errorf("LockWorkStation: %w", err)
		}
		return fmt.Errorf("LockWorkStation failed")
	}
	return nil
}

// EnforcePasswordPolicy is a no-op on Windows (the logon-screen password
// policy is a user/account setting, not something the tool changes).
func EnforcePasswordPolicy() error { return nil }

// LockScreenDirect is unused on Windows (SYSTEM session cannot lock the
// interactive desktop); the scheduled task covers this case.
func LockScreenDirect(triggerPath string) error { return nil }
