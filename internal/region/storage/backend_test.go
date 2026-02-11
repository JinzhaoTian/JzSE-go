package storage

import "testing"

func TestNewBackend(t *testing.T) {
	t.Run("local_fs", func(t *testing.T) {
		backend, err := NewBackend("local_fs", t.TempDir())
		if err != nil {
			t.Fatalf("NewBackend failed: %v", err)
		}
		defer backend.Close()

		if _, ok := backend.(*LocalFSBackend); !ok {
			t.Fatalf("unexpected backend type: %T", backend)
		}
	})

	t.Run("memory", func(t *testing.T) {
		backend, err := NewBackend("memory", "")
		if err != nil {
			t.Fatalf("NewBackend failed: %v", err)
		}
		defer backend.Close()

		if _, ok := backend.(*MemoryBackend); !ok {
			t.Fatalf("unexpected backend type: %T", backend)
		}
	})

	t.Run("in_memory", func(t *testing.T) {
		backend, err := NewBackend("in_memory", "")
		if err != nil {
			t.Fatalf("NewBackend failed: %v", err)
		}
		defer backend.Close()

		if _, ok := backend.(*MemoryBackend); !ok {
			t.Fatalf("unexpected backend type: %T", backend)
		}
	})

	t.Run("unsupported", func(t *testing.T) {
		if _, err := NewBackend("unknown", ""); err == nil {
			t.Fatal("expected unsupported backend error")
		}
	})
}
