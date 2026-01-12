// Package storage provides file storage backend implementations.
package storage

import (
	"context"
	"io"
	"time"
)

// FileInfo represents information about a stored file.
type FileInfo struct {
	Key     string
	Size    int64
	ModTime time.Time
	IsDir   bool
}

// Backend defines the interface for file storage backends.
type Backend interface {
	// Put stores a file.
	Put(ctx context.Context, key string, reader io.Reader, size int64) error

	// Get retrieves a file.
	Get(ctx context.Context, key string) (io.ReadCloser, error)

	// Delete removes a file.
	Delete(ctx context.Context, key string) error

	// Exists checks if a file exists.
	Exists(ctx context.Context, key string) (bool, error)

	// Stat returns file information.
	Stat(ctx context.Context, key string) (*FileInfo, error)

	// List lists files with the given prefix.
	List(ctx context.Context, prefix string) ([]*FileInfo, error)

	// Close closes the backend.
	Close() error
}

// NewBackend creates a new storage backend based on the type.
func NewBackend(backendType, basePath string) (Backend, error) {
	switch backendType {
	case "local_fs":
		return NewLocalFSBackend(basePath)
	// Future implementations:
	// case "minio":
	//     return NewMinIOBackend(...)
	// case "s3":
	//     return NewS3Backend(...)
	default:
		return NewLocalFSBackend(basePath)
	}
}
