//go:build windows

package service

import (
	"fmt"
	"os/exec"
)

// monitorTaskName is the scheduled task created by create_task.ps1.
const monitorTaskName = "HealthMonitorTask"

// directLockFunc returns nil: the SYSTEM service cannot lock the interactive
// desktop, the scheduled task covers the trigger.
func directLockFunc() func(triggerPath string) error { return nil }

// nudgeFunc runs the monitor task immediately instead of waiting up to a
// full minute for its next poll. The service runs as SYSTEM, which may
// control Task Scheduler; the task instance itself starts in the logged-in
// user's session, so LockWorkStation still happens in the right context.
func nudgeFunc() func() error {
	return func() error {
		out, err := exec.Command("schtasks", "/Run", "/TN", monitorTaskName).CombinedOutput()
		if err != nil {
			return fmt.Errorf("schtasks /Run %s: %v: %s", monitorTaskName, err, string(out))
		}
		return nil
	}
}
