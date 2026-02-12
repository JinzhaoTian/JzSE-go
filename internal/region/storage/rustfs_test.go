package storage

import (
	"bytes"
	"context"
	"io"
	"testing"
)

func TestRustFSBackend_WithPrefix(t *testing.T) {
	store := newFakeMinIOStore()
	backend := newRustFSBackendWithStore(store, "/tenant-rustfs/")

	ctx := context.Background()
	key := "docs/a.txt"
	content := []byte("hello-rustfs")

	if err := backend.Put(ctx, key, bytes.NewReader(content), int64(len(content))); err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	if _, ok := store.objects["tenant-rustfs/docs/a.txt"]; !ok {
		t.Fatalf("expected object key with prefix, got keys: %+v", store.objects)
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
}

func TestRustFSBackend_Exists(t *testing.T) {
	store := newFakeMinIOStore()
	backend := newRustFSBackendWithStore(store, "")

	ctx := context.Background()
	exists, err := backend.Exists(ctx, "missing")
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if exists {
		t.Fatal("missing object should not exist")
	}

	if err := backend.Put(ctx, "exists.txt", bytes.NewReader([]byte("1")), 1); err != nil {
		t.Fatalf("Put failed: %v", err)
	}
	exists, err = backend.Exists(ctx, "exists.txt")
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if !exists {
		t.Fatal("object should exist")
	}
}

func TestRustFSOptionsValidate(t *testing.T) {
	valid := RustFSOptions{
		Endpoint:  "localhost:9000",
		AccessKey: "key",
		SecretKey: "secret",
		Bucket:    "jzse",
	}

	if err := valid.validate(); err != nil {
		t.Fatalf("validate failed for valid options: %v", err)
	}

	invalidCases := []RustFSOptions{
		{AccessKey: "a", SecretKey: "s", Bucket: "b"},
		{Endpoint: "e", SecretKey: "s", Bucket: "b"},
		{Endpoint: "e", AccessKey: "a", Bucket: "b"},
		{Endpoint: "e", AccessKey: "a", SecretKey: "s"},
	}

	for i, tc := range invalidCases {
		if err := tc.validate(); err == nil {
			t.Fatalf("expected validate error for case %d", i)
		}
	}
}
