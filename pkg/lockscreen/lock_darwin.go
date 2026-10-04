//go:build darwin

package lockscreen

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
)

const cgSessionPath = "/System/Library/CoreServices/Menu Extras/User.menu/Contents/Resources/CGSession"

// EnforcePasswordPolicy makes macOS require the password immediately when the
// display wakes, so pmset displaysleepnow acts as a real screen lock. Must run
// in the user's session (LaunchAgent context).
func EnforcePasswordPolicy() error {
	if err := exec.Command("/usr/bin/defaults", "-currentHost", "write",
		"com.apple.screensaver", "askForPassword", "-int", "1").Run(); err != nil {
		return fmt.Errorf("set askForPassword: %w", err)
	}
	if err := exec.Command("/usr/bin/defaults", "-currentHost", "write",
		"com.apple.screensaver", "askForPasswordDelay", "-int", "0").Run(); err != nil {
		return fmt.Errorf("set askForPasswordDelay: %w", err)
	}
	return nil
}

// LockScreen locks the screen from within a user GUI session (LaunchAgent).
// Order: pmset display sleep (primary), osascript Ctrl+Cmd+Q (needs
// Accessibility if pmset is unavailable), legacy CGSession as a last resort.
func LockScreen() error {
	// Best effort: make sure waking the display requires a password.
	_ = EnforcePasswordPolicy()

	if err := exec.Command("/usr/bin/pmset", "displaysleepnow").Run(); err == nil {
		return nil
	} else {
		lastErr := fmt.Errorf("pmset displaysleepnow: %w", err)

		script := `tell application "System Events" to keystroke "q" using {command down, control down}`
		if err := exec.Command("/usr/bin/osascript", "-e", script).Run(); err == nil {
			return nil
		}

		if _, statErr := os.Stat(cgSessionPath); statErr == nil {
			if err := exec.Command(cgSessionPath, "-suspend").Run(); err == nil {
				return nil
			}
		}
		return lastErr
	}
}

// LockScreenDirect locks the screen from a root LaunchDaemon without a GUI
// session. pmset is setuid root so display sleep works here; the password
// policy is assumed to have been enforced earlier by the agent.
func LockScreenDirect() error {
	if _, err := os.Stat("/usr/bin/pmset"); err != nil {
		return errors.New("pmset not available")
	}
	if err := exec.Command("/usr/bin/pmset", "displaysleepnow").Run(); err != nil {
		return fmt.Errorf("pmset displaysleepnow (direct): %w", err)
	}
	return nil
}
