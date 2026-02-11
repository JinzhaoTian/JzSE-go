// Package storage provides file storage backend implementations.
package storage

import (
	"bytes"
	"context"
	"io"
	"sort"
	"strings"
	"sync"
	"time"

	"asisaid.cn/JzSE/internal/common/errors"
)

type memoryObject struct {
	data    []byte
	modTime time.Time
}

// MemoryBackend implements Backend using in-memory storage.
type MemoryBackend struct {
	mu      sync.RWMutex
	objects map[string]memoryObject
}

// NewMemoryBackend creates a new MemoryBackend.
func NewMemoryBackend() *MemoryBackend {
	return &MemoryBackend{
		objects: make(map[string]memoryObject),
	}
}

// Put stores a file.
func (b *MemoryBackend) Put(ctx context.Context, key string, reader io.Reader, size int64) error {
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}

	if size > 0 && int64(len(data)) != size {
		return errors.E("MemoryBackend.Put", errors.ErrInvalidInput, nil, "size mismatch")
	}

	b.mu.Lock()
	b.objects[key] = memoryObject{
		data:    data,
		modTime: time.Now(),
	}
	b.mu.Unlock()

	return nil
}

// Get retrieves a file.
func (b *MemoryBackend) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	b.mu.RLock()
	obj, ok := b.objects[key]
	b.mu.RUnlock()
	if !ok {
		return nil, errors.ErrNotFound
	}

	return io.NopCloser(bytes.NewReader(obj.data)), nil
}

// Delete removes a file.
func (b *MemoryBackend) Delete(ctx context.Context, key string) error {
	b.mu.Lock()
	_, ok := b.objects[key]
	if !ok {
		b.mu.Unlock()
		return errors.ErrNotFound
	}
	delete(b.objects, key)
	b.mu.Unlock()

	return nil
}

// Exists checks if a file exists.
func (b *MemoryBackend) Exists(ctx context.Context, key string) (bool, error) {
	b.mu.RLock()
	_, ok := b.objects[key]
	b.mu.RUnlock()

	return ok, nil
}

// Stat returns file information.
func (b *MemoryBackend) Stat(ctx context.Context, key string) (*FileInfo, error) {
	b.mu.RLock()
	obj, ok := b.objects[key]
	b.mu.RUnlock()
	if !ok {
		return nil, errors.ErrNotFound
	}

	return &FileInfo{
		Key:     key,
		Size:    int64(len(obj.data)),
		ModTime: obj.modTime,
		IsDir:   false,
	}, nil
}

// List lists files with the given prefix.
func (b *MemoryBackend) List(ctx context.Context, prefix string) ([]*FileInfo, error) {
	b.mu.RLock()
	result := make([]*FileInfo, 0)
	for key, obj := range b.objects {
		if prefix != "" && !strings.HasPrefix(key, prefix) {
			continue
		}
		result = append(result, &FileInfo{
			Key:     key,
			Size:    int64(len(obj.data)),
			ModTime: obj.modTime,
			IsDir:   false,
		})
	}
	b.mu.RUnlock()

	sort.Slice(result, func(i, j int) bool {
		return result[i].Key < result[j].Key
	})

	return result, nil
}

// Close closes the backend.
func (b *MemoryBackend) Close() error {
	return nil
}
