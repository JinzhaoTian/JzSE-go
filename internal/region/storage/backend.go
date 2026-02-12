// Package storage provides file storage backend implementations.
package storage

import (
	"context"
	"fmt"
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

// ObjectStorageOptions defines S3-compatible object storage configuration.
type ObjectStorageOptions struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Bucket    string
	Region    string
	UseSSL    bool
	Prefix    string
}

// S3Options defines S3 backend configuration.
type S3Options ObjectStorageOptions

// MinIOOptions defines MinIO backend configuration.
type MinIOOptions ObjectStorageOptions

// RustFSOptions defines RustFS backend configuration.
type RustFSOptions ObjectStorageOptions

// BackendOptions defines storage backend creation options.
type BackendOptions struct {
	BasePath string
	S3       S3Options
	MinIO    MinIOOptions
	RustFS   RustFSOptions
}

// NewBackend creates a new storage backend based on the type.
func NewBackend(backendType string, options BackendOptions) (Backend, error) {
	switch backendType {
	case "local_fs":
		return NewLocalFSBackend(options.BasePath)
	case "memory", "in_memory":
		return NewMemoryBackend(), nil
	case "s3":
		return NewS3Backend(options.S3)
	case "minio":
		return NewMinIOBackend(options.MinIO)
	case "rustfs":
		return NewRustFSBackend(options.RustFS)
	default:
		return nil, fmt.Errorf("unsupported storage backend: %s", backendType)
	}
}
