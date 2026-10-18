// Package logger provides process-local logging helpers.
package logger

import "sync"

// Entry is one formatted log line with a stable, monotonically increasing ID.
type Entry struct {
	ID   uint64 `json:"id"`
	Line string `json:"line"`
}

// Buffer is a fixed-size ring of the most recent log entries.
// Concurrent-safe. Since(0) returns everything still buffered.
type Buffer struct {
	mu      sync.RWMutex
	entries []Entry
	maxSize int
	nextID  uint64
}

func NewBuffer(maxSize int) *Buffer {
	if maxSize <= 0 {
		maxSize = 1000
	}
	return &Buffer{maxSize: maxSize}
}

func (b *Buffer) Add(line string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.nextID++
	b.entries = append(b.entries, Entry{ID: b.nextID, Line: line})
	if len(b.entries) > b.maxSize {
		b.entries = b.entries[len(b.entries)-b.maxSize:]
	}
}

// Since returns entries with ID > since, in insertion order.
func (b *Buffer) Since(since uint64) []Entry {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if len(b.entries) == 0 {
		return nil
	}
	out := make([]Entry, 0, len(b.entries))
	for _, e := range b.entries {
		if e.ID > since {
			out = append(out, e)
		}
	}
	return out
}

// LastID returns the ID of the most recently added entry (0 if empty).
func (b *Buffer) LastID() uint64 {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.nextID
}
