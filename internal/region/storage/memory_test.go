package storage

import (
	"bytes"
	"context"
	"io"
	"testing"
)

func TestMemoryBackend(t *testing.T) {
	backend := NewMemoryBackend()
	defer backend.Close()

	ctx := context.Background()
	key := "docs/a.txt"
	content := []byte("hello-memory")

	t.Run("Put and Get", func(t *testing.T) {
		if err := backend.Put(ctx, key, bytes.NewReader(content), int64(len(content))); err != nil {
			t.Fatalf("Put failed: %v", err)
		}

		reader, err := backend.Get(ctx, key)
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		defer reader.Close()

		got, err := io.ReadAll(reader)
		if err != nil {
			t.Fatalf("ReadAll failed: %v", err)
		}

		if !bytes.Equal(got, content) {
			t.Fatalf("content mismatch: got=%q want=%q", got, content)
		}
	})

	t.Run("Exists", func(t *testing.T) {
		exists, err := backend.Exists(ctx, key)
		if err != nil {
			t.Fatalf("Exists failed: %v", err)
		}
		if !exists {
			t.Fatal("expected key to exist")
		}
	})

	t.Run("Stat", func(t *testing.T) {
		info, err := backend.Stat(ctx, key)
		if err != nil {
			t.Fatalf("Stat failed: %v", err)
		}
		if info.Key != key {
			t.Fatalf("unexpected key: %s", info.Key)
		}
		if info.Size != int64(len(content)) {
			t.Fatalf("unexpected size: %d", info.Size)
		}
	})

	t.Run("List by prefix", func(t *testing.T) {
		if err := backend.Put(ctx, "docs/b.txt", bytes.NewReader([]byte("b")), 1); err != nil {
			t.Fatalf("Put failed: %v", err)
		}
		if err := backend.Put(ctx, "images/c.txt", bytes.NewReader([]byte("c")), 1); err != nil {
			t.Fatalf("Put failed: %v", err)
		}

		list, err := backend.List(ctx, "docs/")
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}
		if len(list) != 2 {
			t.Fatalf("unexpected list length: %d", len(list))
		}
		if list[0].Key != "docs/a.txt" || list[1].Key != "docs/b.txt" {
			t.Fatalf("unexpected list order/content: %+v", list)
		}
	})

	t.Run("Delete", func(t *testing.T) {
		if err := backend.Delete(ctx, key); err != nil {
			t.Fatalf("Delete failed: %v", err)
		}
		exists, _ := backend.Exists(ctx, key)
		if exists {
			t.Fatal("expected key to be deleted")
		}
	})

	t.Run("Delete non-existent", func(t *testing.T) {
		if err := backend.Delete(ctx, "missing"); err == nil {
			t.Fatal("expected error for missing key")
		}
	})

	t.Run("size mismatch", func(t *testing.T) {
		err := backend.Put(ctx, "bad-size", bytes.NewReader([]byte("123")), 2)
		if err == nil {
			t.Fatal("expected size mismatch error")
		}
	})
}
