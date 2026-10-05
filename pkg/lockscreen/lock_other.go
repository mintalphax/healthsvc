//go:build !windows && !darwin

package lockscreen

import "errors"

// LockScreen is unsupported on platforms other than Windows and macOS.
func LockScreen() error { return errors.New("lock screen unsupported on this platform") }

// EnforcePasswordPolicy is a no-op where the concept does not exist.
func EnforcePasswordPolicy() error { return nil }

// LockScreenDirect is unsupported on platforms other than Windows and macOS.
func LockScreenDirect(triggerPath string) error {
	return errors.New("lock screen unsupported on this platform")
}
