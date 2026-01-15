package sync

import (
	"testing"

	"asisaid.cn/JzSE/internal/common/errors"
)

func TestChangeQueue(t *testing.T) {
	q := NewChangeQueue(5)

	t.Run("initial state", func(t *testing.T) {
		if q.Len() != 0 {
			t.Errorf("Len() = %v, want 0", q.Len())
		}
	})

	t.Run("Push and Len", func(t *testing.T) {
		event := &ChangeEvent{ID: "event-1", FileID: "file-1"}
		err := q.Push(event)
		if err != nil {
			t.Fatalf("Push failed: %v", err)
		}

		if q.Len() != 1 {
			t.Errorf("Len() = %v, want 1", q.Len())
		}
	})

	t.Run("Pop", func(t *testing.T) {
		event := q.Pop()
		if event == nil {
			t.Fatal("Pop returned nil")
		}
		if event.ID != "event-1" {
			t.Errorf("event.ID = %v, want event-1", event.ID)
		}
		if q.Len() != 0 {
			t.Errorf("Len() = %v, want 0", q.Len())
		}
	})

	t.Run("Pop empty", func(t *testing.T) {
		event := q.Pop()
		if event != nil {
			t.Error("Pop on empty queue should return nil")
		}
	})

	t.Run("PopN", func(t *testing.T) {
		// Push 3 events
		for i := 0; i < 3; i++ {
			q.Push(&ChangeEvent{ID: string(rune('a' + i))})
		}

		events := q.PopN(2)
		if len(events) != 2 {
			t.Errorf("PopN(2) returned %v events, want 2", len(events))
		}
		if q.Len() != 1 {
			t.Errorf("Len() = %v, want 1", q.Len())
		}
	})

	t.Run("PopN more than available", func(t *testing.T) {
		events := q.PopN(10)
		if len(events) != 1 {
			t.Errorf("PopN(10) returned %v events, want 1", len(events))
		}
	})

	t.Run("queue full", func(t *testing.T) {
		q2 := NewChangeQueue(2)
		q2.Push(&ChangeEvent{ID: "1"})
		q2.Push(&ChangeEvent{ID: "2"})

		err := q2.Push(&ChangeEvent{ID: "3"})
		if !errors.IsNotFound(err) && err != errors.ErrQueueFull {
			// Just check there's an error
			if err == nil {
				t.Error("Push to full queue should fail")
			}
		}
	})

	t.Run("Clear", func(t *testing.T) {
		q3 := NewChangeQueue(10)
		q3.Push(&ChangeEvent{ID: "1"})
		q3.Push(&ChangeEvent{ID: "2"})

		q3.Clear()
		if q3.Len() != 0 {
			t.Errorf("Len() after Clear = %v, want 0", q3.Len())
		}
	})
}
