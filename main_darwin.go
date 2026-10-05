//go:build darwin

package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"healthsvc/pkg/lockscreen"
	"healthsvc/pkg/logger"
)

const (
	daemonLabel = "com.family.healthsvc"
	agentLabel  = "com.family.healthsvc.agent"

	daemonPlistPath = "/Library/LaunchDaemons/" + daemonLabel + ".plist"
	agentPlistPath  = "/Library/LaunchAgents/" + agentLabel + ".plist"

	staffGID = 20
)

func isWindowsService() (bool, error) { return false, nil }

func runWindowsService() {} // unreachable on darwin

// defaultRunsDaemon: launchd starts the binary without arguments, so running
// bare is the normal daemon mode on macOS.
func defaultRunsDaemon() bool { return true }

// platformInstall writes the LaunchDaemon (root scheduler) and LaunchAgent
// (per-user lock component) plists and loads them. Must run as root, exactly
// like the Windows version requires an administrator.
func platformInstall(baseDir string, log *logger.Logger) error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("must run as root (sudo %s -install)", filepath.Base(os.Args[0]))
	}
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}
	exe, _ = filepath.EvalSymlinks(exe)
	triggerPath := filepath.Join(filepath.Dir(exe), "_h.dat")

	// The root daemon creates the trigger; the user agent must be able to
	// remove it, hence group-staff writability on the install directory.
	_ = os.Chown(filepath.Dir(exe), 0, staffGID)
	_ = os.Chmod(filepath.Dir(exe), 0o775)

	if err := os.MkdirAll(filepath.Dir(daemonPlistPath), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(agentPlistPath), 0o755); err != nil {
		return err
	}

	daemonPlist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>%s</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
	</array>
	<key>WorkingDirectory</key>
	<string>%s</string>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<true/>
	<key>ProcessType</key>
	<string>Background</string>
	<key>StandardOutPath</key>
	<string>%s</string>
	<key>StandardErrorPath</key>
	<string>%s</string>
</dict>
</plist>
`, daemonLabel, exe, filepath.Dir(exe),
		filepath.Join(baseDir, "logs", "launchd.log"),
		filepath.Join(baseDir, "logs", "launchd.log"))

	agentPlist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>%s</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
		<string>-agent</string>
	</array>
	<key>WatchPaths</key>
	<array>
		<string>%s</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
</dict>
</plist>
`, agentLabel, exe, triggerPath)

	if err := os.WriteFile(daemonPlistPath, []byte(daemonPlist), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", daemonPlistPath, err)
	}
	if err := os.WriteFile(agentPlistPath, []byte(agentPlist), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", agentPlistPath, err)
	}

	// Replace a previous daemon, then load the new one.
	_, _ = exec.Command("/bin/launchctl", "bootout", "system/"+daemonLabel).CombinedOutput()
	if out, err := exec.Command("/bin/launchctl", "bootstrap", "system", daemonPlistPath).CombinedOutput(); err != nil {
		return fmt.Errorf("bootstrap daemon: %v: %s", err, strings.TrimSpace(string(out)))
	}
	if err := exec.Command("/bin/launchctl", "print", "system/"+daemonLabel).Run(); err != nil {
		return fmt.Errorf("daemon %s did not load; see logs/launchd.log", daemonLabel)
	}

	// Load the agent into the current console user's GUI session so the
	// machine is protected without waiting for the next login.
	uid := consoleUID()
	if uid <= 0 {
		log.Warnf("no console user found; the lock agent will load on the next user login")
	} else {
		gui := fmt.Sprintf("gui/%d", uid)
		_, _ = exec.Command("/bin/launchctl", "bootout", gui+"/"+agentLabel).CombinedOutput()
		bootOut, bootErr := exec.Command("/bin/launchctl", "bootstrap", gui, agentPlistPath).CombinedOutput()
		if bootErr != nil {
			// Some setups reject a cross-user bootstrap; retry inside the
			// user's per-user launchd context.
			bootOut, bootErr = exec.Command("/bin/launchctl", "asuser", fmt.Sprint(uid),
				"/bin/launchctl", "bootstrap", gui, agentPlistPath).CombinedOutput()
		}
		if bootErr != nil || exec.Command("/bin/launchctl", "print", gui+"/"+agentLabel).Run() != nil {
			return fmt.Errorf("lock agent failed to load into gui/%d: %v: %s; "+
				"log out and back in, then re-run install (without the agent the screen will not lock)",
				uid, bootErr, strings.TrimSpace(string(bootOut)))
		}
		// Enforce "require password immediately" for the console user so the
		// pmset-based lock is airtight even before the first agent run.
		enforcePasswordForUser(uid, log)
	}

	log.Infof("daemon installed: %s", daemonPlistPath)
	log.Infof("agent installed: %s (watching %s)", agentPlistPath, triggerPath)
	return nil
}

// platformUninstall removes both plists and unloads the jobs. Installed
// program files are kept; scripts-darwin/uninstall.sh --purge removes them.
func platformUninstall(baseDir string, log *logger.Logger) error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("must run as root (sudo %s -uninstall)", filepath.Base(os.Args[0]))
	}
	_, _ = exec.Command("/bin/launchctl", "bootout", "system/"+daemonLabel).CombinedOutput()
	if uid := consoleUID(); uid > 0 {
		_, _ = exec.Command("/bin/launchctl", "bootout", fmt.Sprintf("gui/%d/%s", uid, agentLabel)).CombinedOutput()
	}
	var errs []error
	for _, p := range []string{daemonPlistPath, agentPlistPath} {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			errs = append(errs, err)
		} else {
			log.Infof("removed %s", p)
		}
	}
	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

// runAgent is the user-session component: when the trigger file appears,
// enforce the password policy, lock the screen, then clear the trigger.
func runAgent(baseDir string, log *logger.Logger, dryRun bool) {
	triggerPath := filepath.Join(baseDir, "_h.dat")
	tr := lockscreen.NewFileTrigger(triggerPath)
	tr.DryRun = dryRun
	if !tr.Exists() {
		return // WatchPaths also fires on deletion; nothing to do.
	}
	content, _ := os.ReadFile(triggerPath)
	log.Infof("trigger fired at %s (fired %s)", triggerPath, strings.TrimSpace(string(content)))

	if dryRun {
		log.Infof("[DRY-RUN] would enforce password policy and lock the screen")
	} else {
		if err := lockscreen.LockScreen(); err != nil {
			log.Errorf("lock screen failed: %v", err)
			// Keep the trigger so the daemon's direct-lock fallback engages.
			return
		}
		log.Infof("screen locked")
		time.Sleep(2 * time.Second) // let other sessions' agents see the trigger too
	}
	if err := tr.Clear(); err != nil {
		log.Errorf("clear trigger failed: %v", err)
	}
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

// enforcePasswordForUser runs the screensaver password defaults as the
// console user (best effort).
func enforcePasswordForUser(uid int, log *logger.Logger) {
	u, err := user.LookupId(fmt.Sprint(uid))
	if err != nil {
		log.Warnf("cannot resolve uid %d to enforce password policy: %v", uid, err)
		return
	}
	for _, kv := range [][2]string{
		{"askForPassword", "1"},
		{"askForPasswordDelay", "0"},
	} {
		cmd := exec.Command("/usr/bin/sudo", "-u", u.Username, "/usr/bin/defaults", "-currentHost",
			"write", "com.apple.screensaver", kv[0], "-int", kv[1])
		if out, err := cmd.CombinedOutput(); err != nil {
			log.Warnf("defaults write %s for %s failed: %v: %s", kv[0], u.Username, err, strings.TrimSpace(string(out)))
		}
	}
	log.Infof("password-after-sleep policy enforced for uid %d (%s)", uid, u.Username)
}
