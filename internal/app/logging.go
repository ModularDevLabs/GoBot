package app

import (
	"fmt"
	"log"
	"sync"
	"time"
)

type Logger struct {
	debug   bool
	mu      sync.Mutex
	entries []string
}

func NewLogger(level string) *Logger {
	return &Logger{
		debug:   level == "debug",
		entries: make([]string, 0, 500),
	}
}

func (l *Logger) Info(msg string, args ...any) {
	l.write("INFO", msg, args...)
}

func (l *Logger) Debug(msg string, args ...any) {
	if !l.debug {
		return
	}
	l.write("DEBUG", msg, args...)
}

func (l *Logger) Error(msg string, args ...any) {
	l.write("ERROR", msg, args...)
}

func (l *Logger) write(level, msg string, args ...any) {
	formatted := fmt.Sprintf("%s: %s", level, fmt.Sprintf(msg, args...))
	log.Print(formatted)

	entry := fmt.Sprintf("%s %s", time.Now().Format(time.RFC3339), formatted)
	l.mu.Lock()
	l.entries = append(l.entries, entry)
	if len(l.entries) > 1000 {
		l.entries = l.entries[len(l.entries)-1000:]
	}
	l.mu.Unlock()
}

func (l *Logger) RecentEvents(limit int) []string {
	l.mu.Lock()
	defer l.mu.Unlock()

	if limit <= 0 || limit > len(l.entries) {
		limit = len(l.entries)
	}
	start := len(l.entries) - limit
	out := make([]string, limit)
	copy(out, l.entries[start:])
	return out
}
