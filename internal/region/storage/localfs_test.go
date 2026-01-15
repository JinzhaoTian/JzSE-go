package storage

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalFSBackend(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "jzse-storage-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	backend, err := NewLocalFSBackend(tmpDir)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}
	defer backend.Close()

	ctx := context.Background()
	testKey := "test-file-001"
	testContent := []byte("hello, world!")

	t.Run("Put", func(t *testing.T) {
		reader := bytes.NewReader(testContent)
		err := backend.Put(ctx, testKey, reader, int64(len(testContent)))
		if err != nil {
			t.Fatalf("Put failed: %v", err)
		}
	})

	t.Run("Exists after Put", func(t *testing.T) {
		exists, err := backend.Exists(ctx, testKey)
		if err != nil {
			t.Fatalf("Exists failed: %v", err)
		}
		if !exists {
			t.Error("file should exist after Put")
		}
	})

	t.Run("Get", func(t *testing.T) {
		reader, err := backend.Get(ctx, testKey)
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		defer reader.Close()

		content, err := io.ReadAll(reader)
		if err != nil {
			t.Fatalf("failed to read content: %v", err)
		}

		if !bytes.Equal(content, testContent) {
			t.Errorf("content = %q, want %q", content, testContent)
		}
	})

	t.Run("Stat", func(t *testing.T) {
		info, err := backend.Stat(ctx, testKey)
		if err != nil {
			t.Fatalf("Stat failed: %v", err)
		}

		if info.Key != testKey {
			t.Errorf("Key = %v, want %v", info.Key, testKey)
		}
		if info.Size != int64(len(testContent)) {
			t.Errorf("Size = %v, want %v", info.Size, len(testContent))
		}
		if info.IsDir {
			t.Error("IsDir should be false")
		}
	})

	t.Run("Delete", func(t *testing.T) {
		err := backend.Delete(ctx, testKey)
		if err != nil {
			t.Fatalf("Delete failed: %v", err)
		}

		exists, _ := backend.Exists(ctx, testKey)
		if exists {
			t.Error("file should not exist after Delete")
		}
	})

	t.Run("Get non-existent", func(t *testing.T) {
		_, err := backend.Get(ctx, "non-existent-key")
		if err == nil {
			t.Error("Get should fail for non-existent key")
		}
	})

	t.Run("Delete non-existent", func(t *testing.T) {
		err := backend.Delete(ctx, "non-existent-key")
		if err == nil {
			t.Error("Delete should fail for non-existent key")
		}
	})
}

func TestLocalFSBackend_List(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "jzse-storage-list-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	backend, err := NewLocalFSBackend(tmpDir)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}
	defer backend.Close()

	ctx := context.Background()

	// Put some files
	for i := 0; i < 3; i++ {
		key := filepath.Join("testdir", "file"+string(rune('a'+i)))
		content := bytes.NewReader([]byte("content"))
		if err := backend.Put(ctx, key, content, 7); err != nil {
			t.Fatalf("Put failed: %v", err)
		}
	}

	// List is implementation-specific, just ensure no error
	_, err = backend.List(ctx, "testdir")
	if err != nil {
		t.Errorf("List failed: %v", err)
	}
}

func TestComputeHash(t *testing.T) {
	content := bytes.NewReader([]byte("hello"))
	hash, err := ComputeHash(content)
	if err != nil {
		t.Fatalf("ComputeHash failed: %v", err)
	}

	if len(hash) != 64 { // SHA-256 hex is 64 chars
		t.Errorf("hash length = %v, want 64", len(hash))
	}

	// Same content should produce same hash
	content2 := bytes.NewReader([]byte("hello"))
	hash2, _ := ComputeHash(content2)
	if hash != hash2 {
		t.Error("same content should produce same hash")
	}

	// Different content should produce different hash
	content3 := bytes.NewReader([]byte("world"))
	hash3, _ := ComputeHash(content3)
	if hash == hash3 {
		t.Error("different content should produce different hash")
	}
}
