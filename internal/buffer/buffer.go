package buffer

import (
	"encoding/json"
	"sync"
)

// EventBuffer is a thread-safe, bounded buffer for event batches.
type EventBuffer struct {
	mu   sync.Mutex
	cond *sync.Cond
	buf  []json.RawMessage
	max  int
}

// NewEventBuffer creates a new EventBuffer with the given max size.
func NewEventBuffer(max int) *EventBuffer {
	b := &EventBuffer{
		buf: make([]json.RawMessage, 0, max),
		max: max,
	}
	b.cond = sync.NewCond(&b.mu)
	return b
}

// Add adds events to the buffer, dropping oldest if full.
func (b *EventBuffer) Add(events []json.RawMessage) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, ev := range events {
		if len(b.buf) >= b.max {
			// Drop oldest
			b.buf = b.buf[1:]
		}
		b.buf = append(b.buf, ev)
	}
	b.cond.Broadcast()
}

// Drain removes up to n events from the buffer and returns them.
func (b *EventBuffer) Drain(n int) []json.RawMessage {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.buf) == 0 {
		return nil
	}
	count := n
	if len(b.buf) < n {
		count = len(b.buf)
	}
	out := make([]json.RawMessage, count)
	copy(out, b.buf[:count])
	b.buf = b.buf[count:]
	return out
}

// Size returns the current number of events in the buffer.
func (b *EventBuffer) Size() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.buf)
}
