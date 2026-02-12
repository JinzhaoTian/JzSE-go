package storage

import (
	"strings"
	"testing"
)

func TestNewBackend(t *testing.T) {
	t.Run("local_fs", func(t *testing.T) {
		backend, err := NewBackend("local_fs", BackendOptions{BasePath: t.TempDir()})
		if err != nil {
			t.Fatalf("NewBackend failed: %v", err)
		}
		defer backend.Close()

		if _, ok := backend.(*LocalFSBackend); !ok {
			t.Fatalf("unexpected backend type: %T", backend)
		}
	})

	t.Run("memory", func(t *testing.T) {
		backend, err := NewBackend("memory", BackendOptions{})
		if err != nil {
			t.Fatalf("NewBackend failed: %v", err)
		}
		defer backend.Close()

		if _, ok := backend.(*MemoryBackend); !ok {
			t.Fatalf("unexpected backend type: %T", backend)
		}
	})

	t.Run("in_memory", func(t *testing.T) {
		backend, err := NewBackend("in_memory", BackendOptions{})
		if err != nil {
			t.Fatalf("NewBackend failed: %v", err)
		}
		defer backend.Close()

		if _, ok := backend.(*MemoryBackend); !ok {
			t.Fatalf("unexpected backend type: %T", backend)
		}
	})

	t.Run("unsupported", func(t *testing.T) {
		if _, err := NewBackend("unknown", BackendOptions{}); err == nil {
			t.Fatal("expected unsupported backend error")
		}
	})

	t.Run("s3", func(t *testing.T) {
		_, err := NewBackend("s3", BackendOptions{})
		if err == nil {
			t.Fatal("expected s3 validation error")
		}
		if strings.Contains(err.Error(), "unsupported storage backend") {
			t.Fatalf("s3 backend branch not reached: %v", err)
		}
	})

	t.Run("rustfs", func(t *testing.T) {
		_, err := NewBackend("rustfs", BackendOptions{})
		if err == nil {
			t.Fatal("expected rustfs validation error")
		}
		if strings.Contains(err.Error(), "unsupported storage backend") {
			t.Fatalf("rustfs backend branch not reached: %v", err)
		}
	})
}
