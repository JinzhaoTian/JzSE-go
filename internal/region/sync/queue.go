// Package sync provides the sync agent for region-to-coordinator communication.
package sync

import (
	"sync"

	"asisaid.cn/JzSE/internal/common/errors"
)

// ChangeQueue is a thread-safe queue for change events.
type ChangeQueue struct {
	mu      sync.Mutex
	items   []*ChangeEvent
	maxSize int
}

// NewChangeQueue creates a new change queue.
func NewChangeQueue(maxSize int) *ChangeQueue {
	return &ChangeQueue{
		items:   make([]*ChangeEvent, 0),
		maxSize: maxSize,
	}
}

// Push adds an event to the queue.
func (q *ChangeQueue) Push(event *ChangeEvent) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.items) >= q.maxSize {
		return errors.ErrQueueFull
	}

	q.items = append(q.items, event)
	return nil
}

// Pop removes and returns the first event from the queue.
func (q *ChangeQueue) Pop() *ChangeEvent {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.items) == 0 {
		return nil
	}

	event := q.items[0]
	q.items = q.items[1:]
	return event
}

// PopN removes and returns up to n events from the queue.
func (q *ChangeQueue) PopN(n int) []*ChangeEvent {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.items) == 0 {
		return nil
	}

	count := n
	if count > len(q.items) {
		count = len(q.items)
	}

	events := make([]*ChangeEvent, count)
	copy(events, q.items[:count])
	q.items = q.items[count:]
	return events
}

// Len returns the number of events in the queue.
func (q *ChangeQueue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items)
}

// Clear removes all events from the queue.
func (q *ChangeQueue) Clear() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.items = q.items[:0]
}
