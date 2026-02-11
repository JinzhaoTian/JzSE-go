package storage

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	commonerrors "asisaid.cn/JzSE/internal/common/errors"
)

type fakeMinIOStore struct {
	objects map[string][]byte
	modTime map[string]time.Time
}

func newFakeMinIOStore() *fakeMinIOStore {
	return &fakeMinIOStore{
		objects: make(map[string][]byte),
		modTime: make(map[string]time.Time),
	}
}

func (f *fakeMinIOStore) Put(ctx context.Context, key string, reader io.Reader, size int64) error {
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	if size >= 0 && int64(len(data)) != size {
		return commonerrors.ErrInvalidInput
	}
	f.objects[key] = data
	f.modTime[key] = time.Now()
	return nil
}

func (f *fakeMinIOStore) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	data, ok := f.objects[key]
	if !ok {
		return nil, commonerrors.ErrNotFound
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (f *fakeMinIOStore) Delete(ctx context.Context, key string) error {
	if _, ok := f.objects[key]; !ok {
		return commonerrors.ErrNotFound
	}
	delete(f.objects, key)
	delete(f.modTime, key)
	return nil
}

func (f *fakeMinIOStore) Stat(ctx context.Context, key string) (*FileInfo, error) {
	data, ok := f.objects[key]
	if !ok {
		return nil, commonerrors.ErrNotFound
	}
	return &FileInfo{
		Key:     key,
		Size:    int64(len(data)),
		ModTime: f.modTime[key],
		IsDir:   false,
	}, nil
}

func (f *fakeMinIOStore) List(ctx context.Context, prefix string) ([]*FileInfo, error) {
	result := make([]*FileInfo, 0)
	for key, data := range f.objects {
		if prefix != "" && !strings.HasPrefix(key, prefix) {
			continue
		}
		result = append(result, &FileInfo{
			Key:     key,
			Size:    int64(len(data)),
			ModTime: f.modTime[key],
			IsDir:   false,
		})
	}
	return result, nil
}

func (f *fakeMinIOStore) Close() error {
	return nil
}

func TestMinIOBackend_WithPrefix(t *testing.T) {
	store := newFakeMinIOStore()
	backend := newMinIOBackendWithStore(store, "/tenant-a/")

	ctx := context.Background()
	key := "docs/a.txt"
	content := []byte("hello-minio")

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

func TestMinIOBackend_Exists(t *testing.T) {
	store := newFakeMinIOStore()
	backend := newMinIOBackendWithStore(store, "")

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

func TestMinIOOptionsValidate(t *testing.T) {
	valid := MinIOOptions{
		Endpoint:  "localhost:9000",
		AccessKey: "minioadmin",
		SecretKey: "minioadmin",
		Bucket:    "jzse",
	}

	if err := valid.validate(); err != nil {
		t.Fatalf("validate failed for valid options: %v", err)
	}

	invalidCases := []MinIOOptions{
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
