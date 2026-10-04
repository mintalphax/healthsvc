package service

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"healthsvc/pkg/logger"
)

func newTestService(t *testing.T, configYAML string) *LockService {
	t.Helper()
	base := t.TempDir()
	if err := os.MkdirAll(filepath.Join(base, "configs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "configs", "config.yaml"), []byte(configYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	log, err := logger.New(filepath.Join(base, "logs"), "test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { log.Close() })
	s, err := New(base, log, true)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return s
}

const testConfig = `
schedule:
  lock_times: ["23:10", "06:30"]
  enable: true
ntp:
  allow_local_time: true
`

func TestMarkAndDedup(t *testing.T) {
	s := newTestService(t, testConfig)

	key := "2026-10-04-23:10"
	if s.isLockedDate(key) {
		t.Fatal("fresh state should not be locked")
	}
	s.markLocked(key)
	s.markLocked(key) // dedup
	if !s.isLockedDate(key) {
		t.Fatal("marked date should be locked")
	}
	if len(s.state.LockedDates) != 1 {
		t.Fatalf("dedup failed: %v", s.state.LockedDates)
	}

	s.unmarkLocked(key)
	if s.isLockedDate(key) {
		t.Fatal("unmarked date should be unlocked")
	}
}

func TestSaveLoadStateRoundTrip(t *testing.T) {
	s := newTestService(t, testConfig)
	s.markLocked("2026-10-04-23:10")
	s.markLocked("2026-10-03-06:30")
	s.state.LastCheck = "2026-10-04 23:23:02"
	s.saveState()

	// A new instance must see the same state.
	s2 := newTestService(t, testConfig)
	// Point s2 at s's state file.
	s2.statePath = s.statePath
	s2.loadState()
	if !s2.isLockedDate("2026-10-04-23:10") || !s2.isLockedDate("2026-10-03-06:30") {
		t.Fatalf("round trip lost entries: %v", s2.state)
	}
	if s2.state.LastCheck != "2026-10-04 23:23:02" {
		t.Fatalf("last_check mismatch: %q", s2.state.LastCheck)
	}
}

func TestPruneState(t *testing.T) {
	s := newTestService(t, testConfig)
	now := time.Date(2026, 10, 4, 23, 0, 0, 0, time.Local)
	s.markLocked("2026-10-04-23:10") // today: keep
	s.markLocked("2026-09-29-06:30") // 5 days ago: keep
	s.markLocked("2026-09-01-06:30") // >7 days: prune
	s.markLocked("garbage-entry")    // malformed: prune
	s.pruneState(now)

	if len(s.state.LockedDates) != 2 {
		t.Fatalf("prune left %v", s.state.LockedDates)
	}
	if !s.isLockedDate("2026-10-04-23:10") || !s.isLockedDate("2026-09-29-06:30") {
		t.Fatalf("unexpected survivors: %v", s.state.LockedDates)
	}
}

func TestCheckAndLockDryRun(t *testing.T) {
	s := newTestService(t, testConfig)
	// 23:10 and 06:30 configured; simulate 23:15 => both are due.
	now := time.Date(2026, 10, 4, 23, 15, 0, 0, time.Local)
	s.checkAndLock(now, "local")

	if len(s.state.LockedDates) != 2 {
		t.Fatalf("expected both lock times recorded, got %v", s.state.LockedDates)
	}
	for _, want := range []string{"2026-10-04-06:30", "2026-10-04-23:10"} {
		if !s.isLockedDate(want) {
			t.Fatalf("missing %s in %v", want, s.state.LockedDates)
		}
	}

	// Second pass must not duplicate (locked today dedup).
	s.checkAndLock(now.Add(time.Minute), "local")
	if len(s.state.LockedDates) != 2 {
		t.Fatalf("dedup failed: %v", s.state.LockedDates)
	}

	// At 07:00 only 06:30 is due.
	s2 := newTestService(t, testConfig)
	s2.checkAndLock(time.Date(2026, 10, 4, 7, 0, 0, 0, time.Local), "local")
	if len(s2.state.LockedDates) != 1 || !s2.isLockedDate("2026-10-04-06:30") {
		t.Fatalf("future lock time should be skipped: %v", s2.state.LockedDates)
	}
}

func TestCheckAndLockDisabledWeekday(t *testing.T) {
	// 2026-10-04 is a Sunday; only Monday allowed => nothing recorded.
	s := newTestService(t, `
schedule:
  lock_times: ["06:00"]
  weekdays: ["Monday"]
  enable: true
ntp:
  allow_local_time: true
`)
	sunday := time.Date(2026, 10, 4, 7, 0, 0, 0, time.Local)
	s.checkAndLock(sunday, "local")
	if len(s.state.LockedDates) != 0 {
		t.Fatalf("sunday should not lock: %v", s.state.LockedDates)
	}
}
