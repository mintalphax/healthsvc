// Package lockscreen contains the trigger mechanism between the privileged
// scheduler and the user-session component that actually locks the screen,
// plus the platform lock actions themselves.
package lockscreen

import (
	"fmt"
	"os"
	"time"
)

// Trigger signals "lock now" from the daemon (root/service context) to the
// per-user component, which is the only context allowed to lock the screen.
type Trigger interface {
	// Fire writes the trigger file.
	Fire(now time.Time) error
	// Path returns the trigger file path.
	Path() string
	// Exists reports whether the trigger is pending.
	Exists() bool
	// Clear removes a consumed trigger.
	Clear() error
}

// FileTrigger implements Trigger via a marker file. The Windows
// scheduled-task monitor.bat and the macOS LaunchAgent both poll/watch this
// file to learn when a lock is due.
type FileTrigger struct {
	TriggerFilePath string `json:"trigger_file_path"`
	DryRun          bool   `json:"-"`
}

// NewFileTrigger creates a FileTrigger at path.
func NewFileTrigger(path string) *FileTrigger {
	return &FileTrigger{TriggerFilePath: path}
}

// Fire writes the trigger file containing the fire time. Mode 0666 so a
// user-session agent can read and remove it even though the daemon is root.
func (t *FileTrigger) Fire(now time.Time) error {
	if t.DryRun {
		return nil
	}
	if err := os.WriteFile(t.TriggerFilePath, []byte(now.Format("2006-01-02 15:04:05")), 0o666); err != nil {
		return fmt.Errorf("write trigger file: %w", err)
	}
	return nil
}

// Path returns the trigger file path.
func (t *FileTrigger) Path() string { return t.TriggerFilePath }

// Exists reports whether the trigger file is present.
func (t *FileTrigger) Exists() bool {
	_, err := os.Stat(t.TriggerFilePath)
	return err == nil
}

// Clear removes the trigger file.
func (t *FileTrigger) Clear() error {
	if t.DryRun {
		return nil
	}
	err := os.Remove(t.TriggerFilePath)
	if err != nil && os.IsNotExist(err) {
		return nil
	}
	return err
}
