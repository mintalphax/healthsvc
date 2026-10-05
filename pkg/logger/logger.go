// Package logger writes timestamped log lines to a rotating log file with an
// optional console mirror.
package logger

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	defaultMaxBytes  = 10 << 20 // 10 MB per file
	defaultKeepFiles = 10
	keepRotatedDays  = 30
)

// Logger is a safe-for-concurrent-use file logger with size rotation.
type Logger struct {
	mu      sync.Mutex
	dir     string
	name    string // e.g. "health" -> logs/health.log
	file    *os.File
	console bool
	maxSize int64
	keep    int
}

// New creates logs/<name>.log under dir. The directory is created if missing.
func New(dir, name string) (*Logger, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create log dir: %w", err)
	}
	l := &Logger{dir: dir, name: name, maxSize: defaultMaxBytes, keep: defaultKeepFiles}
	if err := l.openLogFile(); err != nil {
		return nil, err
	}
	l.cleanupOldLogs()
	return l, nil
}

// NewConsole returns a logger that only mirrors to stdout and never touches
// files. Fallback for the user-session agent: a broken or unwritable log
// file must never prevent the lock itself.
func NewConsole() *Logger {
	return &Logger{console: true, maxSize: defaultMaxBytes, keep: defaultKeepFiles}
}

// SetConsole mirrors every line to stdout (used in foreground/debug runs).
func (l *Logger) SetConsole(on bool) {
	l.mu.Lock()
	l.console = on
	l.mu.Unlock()
}

func (l *Logger) path() string { return filepath.Join(l.dir, l.name+".log") }

func (l *Logger) openLogFile() error {
	f, err := os.OpenFile(l.path(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}
	l.file = f
	return nil
}

// Infof logs an informational line.
func (l *Logger) Infof(format string, args ...any) { l.write("INFO", format, args...) }

// Warnf logs a warning line.
func (l *Logger) Warnf(format string, args ...any) { l.write("WARN", format, args...) }

// Errorf logs an error line.
func (l *Logger) Errorf(format string, args ...any) { l.write("ERROR", format, args...) }

// Debugf logs a debug line.
func (l *Logger) Debugf(format string, args ...any) { l.write("DEBUG", format, args...) }

func (l *Logger) write(level, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	line := time.Now().Format("2006-01-02 15:04:05") + " [" + level + "] " + strings.TrimRight(msg, "\n") + "\n"

	l.mu.Lock()
	defer l.mu.Unlock()
	if l.console {
		_, _ = os.Stdout.WriteString(line)
	}
	if l.file == nil {
		return
	}
	l.rotateIfNeededLocked()
	if l.file == nil {
		return
	}
	_, _ = l.file.WriteString(line)
}

// rotateIfNeededLocked renames the current log when it exceeds the size limit.
func (l *Logger) rotateIfNeededLocked() {
	info, err := l.file.Stat()
	if err != nil || info.Size() < l.maxSize {
		return
	}
	stamp := time.Now().Format("20060102_150405")
	rotated := filepath.Join(l.dir, fmt.Sprintf("%s_%s.log", l.name, stamp))
	_ = l.file.Close()
	if err := os.Rename(l.path(), rotated); err != nil {
		l.file, _ = os.OpenFile(l.path(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		return
	}
	if err := l.openLogFile(); err != nil {
		l.file = nil
	}
	l.cleanupOldLogs()
}

// cleanupOldLogs deletes rotated logs beyond the keep limit or older than
// keepRotatedDays. The active log file is never touched.
func (l *Logger) cleanupOldLogs() {
	entries, err := os.ReadDir(l.dir)
	if err != nil {
		return
	}
	prefix := l.name + "_"
	var rotated []string
	cutoff := time.Now().AddDate(0, 0, -keepRotatedDays)
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() || !strings.HasPrefix(n, prefix) || !strings.HasSuffix(n, ".log") {
			continue
		}
		p := filepath.Join(l.dir, n)
		if info, err := e.Info(); err == nil && info.ModTime().Before(cutoff) {
			_ = os.Remove(p)
			continue
		}
		rotated = append(rotated, p)
	}
	if len(rotated) <= l.keep {
		return
	}
	// Name contains a sortable timestamp; delete the oldest extras.
	sort.Strings(rotated)
	for _, p := range rotated[:len(rotated)-l.keep] {
		_ = os.Remove(p)
	}
}

// Close flushes and closes the underlying file.
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file == nil {
		return errors.New("logger already closed")
	}
	err := l.file.Close()
	l.file = nil
	return err
}
