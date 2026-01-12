// Package storage provides file storage backend implementations.
package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"asisaid.cn/JzSE/internal/common/errors"
)

// LocalFSBackend implements Backend using the local file system.
type LocalFSBackend struct {
	basePath string
	tempPath string
}

// NewLocalFSBackend creates a new LocalFSBackend.
func NewLocalFSBackend(basePath string) (*LocalFSBackend, error) {
	// Ensure base directory exists
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create base directory: %w", err)
	}

	tempPath := filepath.Join(basePath, ".temp")
	if err := os.MkdirAll(tempPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	return &LocalFSBackend{
		basePath: basePath,
		tempPath: tempPath,
	}, nil
}

// Put stores a file.
func (b *LocalFSBackend) Put(ctx context.Context, key string, reader io.Reader, size int64) error {
	filePath := b.keyToPath(key)
	dir := filepath.Dir(filePath)

	// Ensure directory exists
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write to temp file first
	tempFile, err := os.CreateTemp(b.tempPath, "upload-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath) // Clean up temp file on failure

	// Copy data to temp file
	written, err := io.Copy(tempFile, reader)
	if err != nil {
		tempFile.Close()
		return fmt.Errorf("failed to write data: %w", err)
	}
	tempFile.Close()

	// Verify size if provided
	if size > 0 && written != size {
		return fmt.Errorf("size mismatch: expected %d, got %d", size, written)
	}

	// Move temp file to final location
	if err := os.Rename(tempPath, filePath); err != nil {
		return fmt.Errorf("failed to move file: %w", err)
	}

	return nil
}

// Get retrieves a file.
func (b *LocalFSBackend) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	filePath := b.keyToPath(key)

	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.ErrNotFound
		}
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	return file, nil
}

// Delete removes a file.
func (b *LocalFSBackend) Delete(ctx context.Context, key string) error {
	filePath := b.keyToPath(key)

	if err := os.Remove(filePath); err != nil {
		if os.IsNotExist(err) {
			return errors.ErrNotFound
		}
		return fmt.Errorf("failed to delete file: %w", err)
	}

	return nil
}

// Exists checks if a file exists.
func (b *LocalFSBackend) Exists(ctx context.Context, key string) (bool, error) {
	filePath := b.keyToPath(key)

	_, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

// Stat returns file information.
func (b *LocalFSBackend) Stat(ctx context.Context, key string) (*FileInfo, error) {
	filePath := b.keyToPath(key)

	info, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.ErrNotFound
		}
		return nil, err
	}

	return &FileInfo{
		Key:     key,
		Size:    info.Size(),
		ModTime: info.ModTime(),
		IsDir:   info.IsDir(),
	}, nil
}

// List lists files with the given prefix.
func (b *LocalFSBackend) List(ctx context.Context, prefix string) ([]*FileInfo, error) {
	var result []*FileInfo
	prefixPath := b.keyToPath(prefix)

	err := filepath.Walk(prefixPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip temporary directory
		if strings.Contains(path, ".temp") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		key, err := b.pathToKey(path)
		if err != nil {
			return nil // Skip files we can't convert
		}

		result = append(result, &FileInfo{
			Key:     key,
			Size:    info.Size(),
			ModTime: info.ModTime(),
			IsDir:   info.IsDir(),
		})

		return nil
	})

	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	return result, nil
}

// Close closes the backend.
func (b *LocalFSBackend) Close() error {
	return nil // Nothing to close for local filesystem
}

// keyToPath converts a storage key to a file path.
// Uses a two-level directory structure based on key hash.
func (b *LocalFSBackend) keyToPath(key string) string {
	hash := sha256.Sum256([]byte(key))
	hashStr := hex.EncodeToString(hash[:])

	// Use first 2 and next 2 characters for directory structure
	return filepath.Join(b.basePath, hashStr[:2], hashStr[2:4], key)
}

// pathToKey converts a file path back to a storage key.
func (b *LocalFSBackend) pathToKey(path string) (string, error) {
	rel, err := filepath.Rel(b.basePath, path)
	if err != nil {
		return "", err
	}

	// Remove the hash directories (first 4 characters split by /)
	parts := strings.Split(rel, string(os.PathSeparator))
	if len(parts) < 3 {
		return "", fmt.Errorf("invalid path structure")
	}

	return strings.Join(parts[2:], string(os.PathSeparator)), nil
}

// ComputeHash computes the SHA-256 hash of a file.
func ComputeHash(reader io.Reader) (string, error) {
	hash := sha256.New()
	if _, err := io.Copy(hash, reader); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
