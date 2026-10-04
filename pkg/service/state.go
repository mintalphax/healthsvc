package service

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// State is persisted to _state.dat: locked_dates dedupes triggers and
// last_check is diagnostic.
type State struct {
	LockedDates []string `json:"locked_dates"`
	LastCheck   string   `json:"last_check"`
}

// loadState reads _state.dat; a missing or corrupt file starts fresh.
func (s *LockService) loadState() {
	data, err := os.ReadFile(s.statePath)
	if err != nil {
		s.state.LockedDates = []string{}
		return
	}
	var st State
	if err := json.Unmarshal(data, &st); err != nil {
		s.log.Warnf("state file %s unreadable (%v); starting fresh", s.statePath, err)
		s.state.LockedDates = []string{}
		return
	}
	if st.LockedDates == nil {
		st.LockedDates = []string{}
	}
	s.state = st
}

// saveState atomically writes _state.dat.
func (s *LockService) saveState() {
	data, err := json.MarshalIndent(&s.state, "", "  ")
	if err != nil {
		s.log.Errorf("marshal state: %v", err)
		return
	}
	tmp := s.statePath + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o666); err != nil {
		s.log.Errorf("write state: %v", err)
		return
	}
	if err := os.Rename(tmp, s.statePath); err != nil {
		s.log.Errorf("rename state: %v", err)
	}
}

// isLockedDate reports whether key (e.g. "2026-10-03-23:10") was already
// triggered.
func (s *LockService) isLockedDate(key string) bool {
	for _, k := range s.state.LockedDates {
		if k == key {
			return true
		}
	}
	return false
}

// markLocked records key as triggered.
func (s *LockService) markLocked(key string) {
	if s.isLockedDate(key) {
		return
	}
	s.state.LockedDates = append(s.state.LockedDates, key)
}

// unmarkLocked removes key so it can trigger again (used when pruning).
func (s *LockService) unmarkLocked(key string) {
	out := s.state.LockedDates[:0]
	for _, k := range s.state.LockedDates {
		if k != key {
			out = append(out, k)
		}
	}
	s.state.LockedDates = out
}

// pruneState drops entries older than keepStateDays so the file stays small.
func (s *LockService) pruneState(now time.Time) {
	cutoff := now.AddDate(0, 0, -keepStateDays)
	for _, k := range append([]string(nil), s.state.LockedDates...) {
		t, err := time.ParseInLocation(stateDateLayout, k, now.Location())
		if err != nil {
			s.log.Warnf("dropping malformed state entry %q", k)
			s.unmarkLocked(k)
			continue
		}
		if t.Before(cutoff) {
			s.unmarkLocked(k)
		}
	}
}

// String aids debugging.
func (st State) String() string {
	return fmt.Sprintf("State{locked=%v, lastCheck=%q}", st.LockedDates, st.LastCheck)
}
