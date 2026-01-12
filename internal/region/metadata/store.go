// Package metadata provides local metadata storage using BadgerDB.
package metadata

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/dgraph-io/badger/v4"

	"asisaid.cn/JzSE/internal/common/errors"
	"asisaid.cn/JzSE/internal/common/logger"
)

// Store provides local metadata storage operations.
type Store interface {
	// Get retrieves file metadata by ID.
	Get(ctx context.Context, fileID string) (*FileMetadata, error)

	// GetByPath retrieves file metadata by path.
	GetByPath(ctx context.Context, path string) (*FileMetadata, error)

	// Save saves or updates file metadata.
	Save(ctx context.Context, meta *FileMetadata) error

	// Delete removes file metadata.
	Delete(ctx context.Context, fileID string) error

	// List lists files in a directory.
	List(ctx context.Context, dirPath string) ([]*DirectoryEntry, error)

	// ListByState lists files by sync state.
	ListByState(ctx context.Context, state SyncState, limit int) ([]*FileMetadata, error)

	// Close closes the store.
	Close() error
}

// BadgerStore implements Store using BadgerDB.
type BadgerStore struct {
	db *badger.DB
}

// Key prefixes for different indexes.
const (
	prefixFile      = "files:"     // files:<file_id> -> metadata
	prefixPath      = "paths:"     // paths:<path_hash> -> file_id
	prefixDir       = "dirs:"      // dirs:<parent_hash>:<name> -> file_id
	prefixSyncState = "syncstate:" // syncstate:<state>:<updated_at>:<file_id> -> ""
)

// NewBadgerStore creates a new BadgerStore.
func NewBadgerStore(dbPath string) (*BadgerStore, error) {
	opts := badger.DefaultOptions(dbPath)
	opts.Logger = nil // Disable badger's default logger

	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to open badger db: %w", err)
	}

	logger.L().Info("BadgerDB opened")

	return &BadgerStore{db: db}, nil
}

// Get retrieves file metadata by ID.
func (s *BadgerStore) Get(ctx context.Context, fileID string) (*FileMetadata, error) {
	var meta FileMetadata

	err := s.db.View(func(txn *badger.Txn) error {
		key := []byte(prefixFile + fileID)
		item, err := txn.Get(key)
		if err == badger.ErrKeyNotFound {
			return errors.ErrNotFound
		}
		if err != nil {
			return err
		}

		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, &meta)
		})
	})

	if err != nil {
		if errors.IsNotFound(err) {
			return nil, err
		}
		return nil, errors.E("BadgerStore.Get", errors.ErrNotFound, err)
	}

	return &meta, nil
}

// GetByPath retrieves file metadata by path.
func (s *BadgerStore) GetByPath(ctx context.Context, path string) (*FileMetadata, error) {
	var fileID string

	err := s.db.View(func(txn *badger.Txn) error {
		key := []byte(prefixPath + hashPath(path))
		item, err := txn.Get(key)
		if err == badger.ErrKeyNotFound {
			return errors.ErrNotFound
		}
		if err != nil {
			return err
		}

		return item.Value(func(val []byte) error {
			fileID = string(val)
			return nil
		})
	})

	if err != nil {
		return nil, err
	}

	return s.Get(ctx, fileID)
}

// Save saves or updates file metadata.
func (s *BadgerStore) Save(ctx context.Context, meta *FileMetadata) error {
	data, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	return s.db.Update(func(txn *badger.Txn) error {
		// Save main record
		fileKey := []byte(prefixFile + meta.ID)
		if err := txn.Set(fileKey, data); err != nil {
			return err
		}

		// Save path index
		pathKey := []byte(prefixPath + hashPath(meta.Path))
		if err := txn.Set(pathKey, []byte(meta.ID)); err != nil {
			return err
		}

		// Save directory index
		parentPath := filepath.Dir(meta.Path)
		dirKey := []byte(prefixDir + hashPath(parentPath) + ":" + meta.Name)
		if err := txn.Set(dirKey, []byte(meta.ID)); err != nil {
			return err
		}

		// Save sync state index
		syncKey := []byte(fmt.Sprintf("%s%s:%s:%s",
			prefixSyncState,
			meta.SyncState,
			meta.UpdatedAt.Format("20060102150405"),
			meta.ID,
		))
		if err := txn.Set(syncKey, nil); err != nil {
			return err
		}

		return nil
	})
}

// Delete removes file metadata.
func (s *BadgerStore) Delete(ctx context.Context, fileID string) error {
	// First get the metadata to remove indexes
	meta, err := s.Get(ctx, fileID)
	if err != nil {
		return err
	}

	return s.db.Update(func(txn *badger.Txn) error {
		// Delete main record
		fileKey := []byte(prefixFile + fileID)
		if err := txn.Delete(fileKey); err != nil {
			return err
		}

		// Delete path index
		pathKey := []byte(prefixPath + hashPath(meta.Path))
		if err := txn.Delete(pathKey); err != nil && err != badger.ErrKeyNotFound {
			return err
		}

		// Delete directory index
		parentPath := filepath.Dir(meta.Path)
		dirKey := []byte(prefixDir + hashPath(parentPath) + ":" + meta.Name)
		if err := txn.Delete(dirKey); err != nil && err != badger.ErrKeyNotFound {
			return err
		}

		return nil
	})
}

// List lists files in a directory.
func (s *BadgerStore) List(ctx context.Context, dirPath string) ([]*DirectoryEntry, error) {
	var entries []*DirectoryEntry
	prefix := []byte(prefixDir + hashPath(dirPath) + ":")

	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = prefix
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			var fileID string
			err := item.Value(func(val []byte) error {
				fileID = string(val)
				return nil
			})
			if err != nil {
				continue
			}

			// Get file metadata
			meta, err := s.Get(ctx, fileID)
			if err != nil {
				continue
			}

			entries = append(entries, &DirectoryEntry{
				Name:      meta.Name,
				Path:      meta.Path,
				IsDir:     false,
				Size:      meta.Size,
				UpdatedAt: meta.UpdatedAt,
			})
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return entries, nil
}

// ListByState lists files by sync state.
func (s *BadgerStore) ListByState(ctx context.Context, state SyncState, limit int) ([]*FileMetadata, error) {
	var result []*FileMetadata
	prefix := []byte(prefixSyncState + string(state) + ":")

	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = prefix
		it := txn.NewIterator(opts)
		defer it.Close()

		count := 0
		for it.Seek(prefix); it.ValidForPrefix(prefix) && (limit <= 0 || count < limit); it.Next() {
			key := string(it.Item().Key())
			parts := strings.Split(key, ":")
			if len(parts) < 4 {
				continue
			}
			fileID := parts[len(parts)-1]

			meta, err := s.Get(ctx, fileID)
			if err != nil {
				continue
			}

			result = append(result, meta)
			count++
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}

// Close closes the store.
func (s *BadgerStore) Close() error {
	return s.db.Close()
}

// hashPath creates a simple hash of a path for indexing.
func hashPath(path string) string {
	// Simple implementation - in production, use a proper hash
	return strings.ReplaceAll(path, "/", "_")
}
