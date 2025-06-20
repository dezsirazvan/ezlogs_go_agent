package buffer

import (
	"encoding/json"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEventBuffer_AddAndDrain(t *testing.T) {
	buf := NewEventBuffer(3)
	e1 := json.RawMessage(`{"a":1}`)
	e2 := json.RawMessage(`{"b":2}`)
	e3 := json.RawMessage(`{"c":3}`)
	e4 := json.RawMessage(`{"d":4}`)

	// Add events up to capacity
	buf.Add([]json.RawMessage{e1, e2})
	assert.Equal(t, 2, buf.Size())

	// Add more to fill
	buf.Add([]json.RawMessage{e3})
	assert.Equal(t, 3, buf.Size())

	// Add one more, should drop oldest (e1)
	buf.Add([]json.RawMessage{e4})
	assert.Equal(t, 3, buf.Size())
	batch := buf.Drain(10)
	assert.Equal(t, 3, len(batch))
	assert.Equal(t, e2, batch[0])
	assert.Equal(t, e3, batch[1])
	assert.Equal(t, e4, batch[2])
}

func TestEventBuffer_DrainPartial(t *testing.T) {
	buf := NewEventBuffer(5)
	e1 := json.RawMessage(`{"a":1}`)
	e2 := json.RawMessage(`{"b":2}`)
	buf.Add([]json.RawMessage{e1, e2})
	batch := buf.Drain(1)
	assert.Equal(t, 1, len(batch))
	assert.Equal(t, e1, batch[0])
	assert.Equal(t, 1, buf.Size())
	batch2 := buf.Drain(5)
	assert.Equal(t, 1, len(batch2))
	assert.Equal(t, e2, batch2[0])
	assert.Equal(t, 0, buf.Size())
}

func TestEventBuffer_ConcurrentAddDrain(t *testing.T) {
	buf := NewEventBuffer(100)
	var wg sync.WaitGroup
	event := json.RawMessage(`{"x":42}`)

	// Add events concurrently
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				buf.Add([]json.RawMessage{event})
			}
		}()
	}

	// Drain concurrently
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				_ = buf.Drain(10)
			}
		}()
	}

	wg.Wait()
	// Buffer should never exceed max size
	assert.LessOrEqual(t, buf.Size(), 100)
}
