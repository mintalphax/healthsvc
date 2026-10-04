// Command healthsvc is a cross-platform (Windows + macOS) scheduled
// screen-lock service that curbs teen computer overuse: it checks the time
// against a configurable schedule and locks the screen when a lock time is
// reached, using NTP-verified time so changing the system clock does not
// bypass the schedule.
//
// Architecture:
//
//	privileged daemon (Windows service / macOS LaunchDaemon)
//	  └─ fires trigger file _h.dat when a lock time is reached
//	        └─ user-session component (scheduled task / LaunchAgent)
//	              └─ locks the screen and clears the trigger
package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"

	"healthsvc/pkg/logger"
	"healthsvc/pkg/service"
)

// version is stamped at build time via -ldflags "-X main.version=vX.Y.Z".
var version = "dev"

func main() {
	// When launched by the Windows service control manager, run as service.
	if win, err := isWindowsService(); win {
		runWindowsService()
		return
	} else if err != nil {
		fmt.Fprintln(os.Stderr, "check service context failed:", err)
	}

	var (
		install     = flag.Bool("install", false, "install the daemon (Windows service / macOS launchd plists)")
		uninstall   = flag.Bool("uninstall", false, "uninstall the daemon")
		runFlag     = flag.Bool("run", false, "run the scheduler in the foreground (debug)")
		agentFlag   = flag.Bool("agent", false, "run as the user-session lock agent (macOS, triggered by launchd WatchPaths)")
		configFlag  = flag.String("config", "", "path to config.yaml (default: <binary dir>/configs/config.yaml)")
		dryRun      = flag.Bool("dry-run", false, "log what would happen; never fire triggers or lock")
		showVersion = flag.Bool("version", false, "print version and exit")
	)
	flag.Parse()

	if *showVersion {
		fmt.Printf("healthsvc %s (%s/%s)\n", version, osName(), archName())
		return
	}

	baseDir := baseDirFor(*configFlag)
	logName := "health"
	if *agentFlag {
		logName = "agent"
	}
	log, err := logger.New(filepath.Join(baseDir, "logs"), logName)
	if err != nil {
		fmt.Fprintln(os.Stderr, "init logger:", err)
		os.Exit(1)
	}
	log.SetConsole(*runFlag || *agentFlag)

	switch {
	case *install:
		if err := platformInstall(baseDir, log); err != nil {
			log.Errorf("install failed: %v", err)
			fmt.Fprintln(os.Stderr, "install failed:", err)
			os.Exit(1)
		}
		log.Infof("install complete")
	case *uninstall:
		if err := platformUninstall(baseDir, log); err != nil {
			log.Errorf("uninstall failed: %v", err)
			fmt.Fprintln(os.Stderr, "uninstall failed:", err)
			os.Exit(1)
		}
		log.Infof("uninstall complete")
	case *agentFlag:
		runAgent(baseDir, log, *dryRun)
	default:
		// Foreground/debug run on all platforms; launchd also starts the
		// daemon with no arguments, so on macOS this is the default mode.
		if !*runFlag && !defaultRunsDaemon() {
			flag.Usage()
			return
		}
		runScheduler(baseDir, log, *dryRun)
	}
}

// baseDirFor resolves the install directory. Config/log/state/trigger paths
// are all anchored there.
func baseDirFor(configFlag string) string {
	if configFlag != "" {
		return filepath.Dir(configFlag)
	}
	exe, err := os.Executable()
	if err != nil {
		wd, _ := os.Getwd()
		return wd
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return filepath.Dir(exe)
}

func runScheduler(baseDir string, log *logger.Logger, dryRun bool) {
	if dryRun {
		log.Infof("[DRY-RUN] scheduler will not fire triggers or lock the screen")
	}
	svc, err := service.New(baseDir, log, dryRun)
	if err != nil {
		log.Errorf("init service: %v", err)
		os.Exit(1)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		s := <-sig
		log.Infof("received %v, stopping", s)
		svc.Stop()
	}()

	if err := svc.Run(); err != nil {
		log.Errorf("service run failed: %v", err)
		os.Exit(1)
	}
}

func osName() string   { return runtime.GOOS }
func archName() string { return runtime.GOARCH }
