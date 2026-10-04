package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadDefaultsAndSchedule(t *testing.T) {
	path := writeTemp(t, `
service:
  name: "KeepHealthService"
schedule:
  lock_times:
    - "23:10"
    - "06:30"
  weekdays:
  enable: true
  check_interval: 30
ntp:
  allow_local_time: true
  max_time_offset: 300
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := len(cfg.LockTimesOfDay()); got != 2 {
		t.Fatalf("want 2 lock times, got %d", got)
	}
	if d := cfg.LockTimesOfDay()[0]; d != 23*time.Hour+10*time.Minute {
		t.Fatalf("unexpected first lock time %v", d)
	}
	if cfg.GetCheckInterval() != 30*time.Second {
		t.Fatalf("unexpected check interval %v", cfg.GetCheckInterval())
	}
	if len(cfg.NTP.Servers) != len(defaultNTPServers) {
		t.Fatalf("ntp servers default not applied")
	}
	// empty weekdays => every day
	if !cfg.ShouldLockToday(time.Date(2026, 10, 4, 12, 0, 0, 0, time.Local)) {
		t.Fatal("empty weekdays should match every day")
	}
}

func TestWeekdaysAndWildcard(t *testing.T) {
	path := writeTemp(t, `
schedule:
  lock_times: ["22:00"]
  weekdays: ["Monday", "tue"]
  enable: true
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	monday := time.Date(2026, 10, 5, 12, 0, 0, 0, time.Local) // a Monday
	if !cfg.ShouldLockToday(monday) {
		t.Fatal("Monday should match")
	}
	sunday := time.Date(2026, 10, 4, 12, 0, 0, 0, time.Local)
	if cfg.ShouldLockToday(sunday) {
		t.Fatal("Sunday should not match")
	}

	wild := writeTemp(t, `
schedule:
  lock_times: ["22:00"]
  weekdays: ["*"]
  enable: true
`)
	cfg2, _ := Load(wild)
	if !cfg2.ShouldLockToday(sunday) {
		t.Fatal("wildcard should match every day")
	}
}

func TestDisabledAndInvalid(t *testing.T) {
	path := writeTemp(t, `
schedule:
  lock_times: ["22:00"]
  enable: false
`)
	cfg, _ := Load(path)
	if cfg.ShouldLockToday(time.Now()) {
		t.Fatal("disabled schedule should never lock")
	}

	bad := writeTemp(t, `
schedule:
  lock_times: ["25:99"]
  enable: true
`)
	if _, err := Load(bad); err == nil {
		t.Fatal("invalid lock time should fail")
	}

	missing := writeTemp(t, `
schedule:
  weekdays: ["Funday"]
  enable: true
`)
	if _, err := Load(missing); err == nil {
		t.Fatal("missing lock_times should fail")
	}
}

func TestManagerReloadOnChange(t *testing.T) {
	path := writeTemp(t, `
schedule:
  lock_times: ["23:10"]
  enable: true
`)
	m, err := NewManager(path)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	defer m.Stop()

	// Touch the file with a new schedule.
	if err := os.WriteFile(path, []byte("schedule:\n  lock_times: [\"01:00\"]\n  enable: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, time.Now().Add(time.Second), time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	changed, err := m.PollReload()
	if err != nil {
		t.Fatalf("PollReload: %v", err)
	}
	if !changed {
		t.Fatal("config change not detected")
	}
	if d := m.GetConfig().LockTimesOfDay()[0]; d != time.Hour {
		t.Fatalf("reloaded config not picked up: %v", d)
	}
}
