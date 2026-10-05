//go:build darwin

package lockscreen

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	cgSessionPath  = "/System/Library/CoreServices/Menu Extras/User.menu/Contents/Resources/CGSession"
	screenSaverApp = "/System/Library/CoreServices/ScreenSaverEngine.app"
	// AgentLabel is the launchd label of the per-user lock agent; a root
	// daemon uses it to kickstart the agent directly.
	AgentLabel = "com.family.healthsvc.agent"
)

// EnforcePasswordPolicy makes macOS require the password immediately when the
// screen saver starts or the display wakes, so the screen saver acts as a
// real screen lock. Must run in the user's session (LaunchAgent context).
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
// Order: start the screen saver (macOS 26/Tahoe no longer engages the
// lock-on-wake flow for pmset display sleep), then pmset display sleep for
// older macOS, then the osascript lock shortcut and legacy CGSession.
func LockScreen() error {
	// Best effort: make sure waking from the screen saver requires a password.
	_ = EnforcePasswordPolicy()

	var lastErr error

	// Primary: the screen saver. Waking from it honours askForPassword.
	if _, err := os.Stat(screenSaverApp); err == nil {
		if err := exec.Command("/usr/bin/open", "-a", screenSaverApp).Run(); err == nil {
			return nil
		}
		lastErr = fmt.Errorf("start ScreenSaverEngine: %w", err)
	}

	// Secondary: display sleep (locks on macOS < 26 when askForPassword is set).
	if err := exec.Command("/usr/bin/pmset", "displaysleepnow").Run(); err != nil {
		lastErr = fmt.Errorf("pmset displaysleepnow: %w", err)
	} else {
		return nil
	}

	// Tertiary: the lock-screen keyboard shortcut (needs Accessibility TCC).
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

// KickstartAgent starts the user-session lock agent from a root context,
// bypassing any WatchPaths unreliability.
func KickstartAgent() error {
	uid := consoleUID()
	if uid <= 0 {
		return errors.New("no console user found")
	}
	out, err := exec.Command("/bin/launchctl", "kickstart",
		"gui/"+strconv.Itoa(uid)+"/"+AgentLabel).CombinedOutput()
	if err != nil {
		return fmt.Errorf("kickstart gui/%d/%s: %v: %s", uid, AgentLabel, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// LockScreenDirect performs the fallback lock from a root LaunchDaemon. The
// lock itself must happen inside the user's GUI session, so the preferred
// path is kickstarting the agent and waiting for it to consume the trigger;
// root-context pmset is only a last resort (some macOS versions will not ask
// for a password on wake for display sleeps initiated outside the session).
func LockScreenDirect(triggerPath string) error {
	if _, err := os.Stat("/usr/bin/pmset"); err != nil {
		return errors.New("pmset not available")
	}

	if err := KickstartAgent(); err == nil {
		for i := 0; i < 16; i++ {
			time.Sleep(500 * time.Millisecond)
			if _, err := os.Stat(triggerPath); os.IsNotExist(err) {
				return nil // agent consumed the trigger: policy + lock ran in user context
			}
		}
	}

	if err := enforcePasswordForConsoleUser(); err != nil {
		_ = err // non-fatal: the agent may have enforced the policy already
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
