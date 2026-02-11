package storage

import (
	"bytes"
	"context"
	"io"
	"testing"
)

func TestS3Backend_WithPrefix(t *testing.T) {
	store := newFakeMinIOStore()
	backend := newS3BackendWithStore(store, "/tenant-a/")

	ctx := context.Background()
	key := "docs/a.txt"
	content := []byte("hello-s3")

	if err := backend.Put(ctx, key, bytes.NewReader(content), int64(len(content))); err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	if _, ok := store.objects["tenant-a/docs/a.txt"]; !ok {
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

	info, err := backend.Stat(ctx, key)
	if err != nil {
		t.Fatalf("Stat failed: %v", err)
	}
	if info.Key != key {
		t.Fatalf("unexpected stat key: %s", info.Key)
	}

	list, err := backend.List(ctx, "docs/")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("unexpected list size: %d", len(list))
	}
	if list[0].Key != key {
		t.Fatalf("unexpected list key: %s", list[0].Key)
	}
}

func TestS3Backend_Exists(t *testing.T) {
	store := newFakeMinIOStore()
	backend := newS3BackendWithStore(store, "")

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

func TestS3OptionsValidate(t *testing.T) {
	valid := S3Options{
		Endpoint:  "s3.amazonaws.com",
		AccessKey: "key",
		SecretKey: "secret",
		Bucket:    "jzse",
	}

	if err := valid.validate(); err != nil {
		t.Fatalf("validate failed for valid options: %v", err)
	}

	invalidCases := []S3Options{
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
