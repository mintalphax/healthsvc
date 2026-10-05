//go:build darwin

package lockscreen

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"syscall"
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
// session. Before sleeping the display it enforces the password policy for
// the console user — without it, waking would not require the password and
// the "lock" would be a mere display sleep.
func LockScreenDirect() error {
	if _, err := os.Stat("/usr/bin/pmset"); err != nil {
		return errors.New("pmset not available")
	}
	if err := enforcePasswordForConsoleUser(); err != nil {
		// Non-fatal: the agent may have enforced the policy already.
		_ = err
	}
	if err := exec.Command("/usr/bin/pmset", "displaysleepnow").Run(); err != nil {
		return fmt.Errorf("pmset displaysleepnow (direct): %w", err)
	}
	return nil
}

// enforcePasswordForConsoleUser writes the screensaver password policy into
// the console user's preferences (a root daemon would otherwise only change
// root's own prefs). Best effort.
func enforcePasswordForConsoleUser() error {
	uid := consoleUID()
	if uid <= 0 {
		return errors.New("no console user found")
	}
	u, err := user.LookupId(strconv.Itoa(uid))
	if err != nil {
		return fmt.Errorf("resolve uid %d: %w", uid, err)
	}
	for _, kv := range [][2]string{
		{"askForPassword", "1"},
		{"askForPasswordDelay", "0"},
	} {
		cmd := exec.Command("/usr/bin/sudo", "-u", u.Username, "/usr/bin/defaults", "-currentHost",
			"write", "com.apple.screensaver", kv[0], "-int", kv[1])
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("defaults write %s for %s: %w", kv[0], u.Username, err)
		}
	}
	return nil
}

// consoleUID returns the uid of the logged-in console user (0 if unknown).
func consoleUID() int {
	fi, err := os.Stat("/dev/console")
	if err != nil {
		return 0
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return 0
	}
	return int(st.Uid)
}
