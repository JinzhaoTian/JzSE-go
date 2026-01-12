// Package service provides the region service implementation.
package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"hash"
	"io"
	"mime"
	"path/filepath"
	"time"

	"github.com/google/uuid"

	"asisaid.cn/JzSE/internal/common/errors"
	"asisaid.cn/JzSE/internal/common/logger"
	"asisaid.cn/JzSE/internal/region/metadata"
	"asisaid.cn/JzSE/internal/region/storage"
	"go.uber.org/zap"
)

// FileService handles file operations.
type FileService struct {
	regionID string
	storage  storage.Backend
	metadata metadata.Store
	logger   *zap.Logger
}

// NewFileService creates a new FileService.
func NewFileService(regionID string, storageBackend storage.Backend, metaStore metadata.Store) *FileService {
	return &FileService{
		regionID: regionID,
		storage:  storageBackend,
		metadata: metaStore,
		logger:   logger.WithComponent("FileService"),
	}
}

// UploadRequest represents a file upload request.
type UploadRequest struct {
	Path     string
	Name     string
	Size     int64
	Content  io.Reader
	MimeType string
	OwnerID  string
}

// UploadResponse represents a file upload response.
type UploadResponse struct {
	FileID      string
	Path        string
	Size        int64
	ContentHash string
	Version     int64
	CreatedAt   time.Time
}

// Upload uploads a file.
func (s *FileService) Upload(ctx context.Context, req *UploadRequest) (*UploadResponse, error) {
	// Generate file ID
	fileID := uuid.New().String()

	s.logger.Info("uploading file",
		zap.String("file_id", fileID),
		zap.String("path", req.Path),
		zap.String("name", req.Name),
		zap.Int64("size", req.Size),
	)

	// Calculate hash while uploading
	hashReader := newHashingReader(req.Content)

	// Store the file
	if err := s.storage.Put(ctx, fileID, hashReader, req.Size); err != nil {
		s.logger.Error("failed to store file", zap.Error(err))
		return nil, errors.E("FileService.Upload", errors.ErrStorageFull, err)
	}

	contentHash := hashReader.Hash()

	// Detect MIME type if not provided
	mimeType := req.MimeType
	if mimeType == "" {
		mimeType = mime.TypeByExtension(filepath.Ext(req.Name))
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}
	}

	// Create metadata
	fullPath := filepath.Join(req.Path, req.Name)
	meta := metadata.NewFileMetadata(fileID, req.Name, fullPath)
	meta.Size = req.Size
	meta.ContentHash = contentHash
	meta.MimeType = mimeType
	meta.OwnerID = req.OwnerID
	meta.OriginRegion = s.regionID
	meta.CreatedBy = req.OwnerID
	meta.UpdatedBy = req.OwnerID
	meta.IncrementClock(s.regionID)

	// Save metadata
	if err := s.metadata.Save(ctx, meta); err != nil {
		// Try to clean up the stored file
		_ = s.storage.Delete(ctx, fileID)
		s.logger.Error("failed to save metadata", zap.Error(err))
		return nil, errors.E("FileService.Upload", errors.ErrInvalidMetadata, err)
	}

	s.logger.Info("file uploaded successfully",
		zap.String("file_id", fileID),
		zap.String("content_hash", contentHash),
	)

	return &UploadResponse{
		FileID:      fileID,
		Path:        fullPath,
		Size:        req.Size,
		ContentHash: contentHash,
		Version:     meta.Version,
		CreatedAt:   meta.CreatedAt,
	}, nil
}

// DownloadResponse represents a file download response.
type DownloadResponse struct {
	Content  io.ReadCloser
	Metadata *metadata.FileMetadata
}

// Download downloads a file by ID.
func (s *FileService) Download(ctx context.Context, fileID string) (*DownloadResponse, error) {
	// Get metadata
	meta, err := s.metadata.Get(ctx, fileID)
	if err != nil {
		return nil, err
	}

	// Check local state
	if meta.LocalState != metadata.LocalStatePresent {
		// TODO: Trigger fetch from origin region
		return nil, errors.E("FileService.Download", errors.ErrNotFound, nil, "file not available locally")
	}

	// Get file content
	content, err := s.storage.Get(ctx, fileID)
	if err != nil {
		return nil, errors.E("FileService.Download", errors.ErrNotFound, err)
	}

	return &DownloadResponse{
		Content:  content,
		Metadata: meta,
	}, nil
}

// GetMetadata retrieves file metadata.
func (s *FileService) GetMetadata(ctx context.Context, fileID string) (*metadata.FileMetadata, error) {
	return s.metadata.Get(ctx, fileID)
}

// Delete deletes a file.
func (s *FileService) Delete(ctx context.Context, fileID string) error {
	// Get metadata first
	meta, err := s.metadata.Get(ctx, fileID)
	if err != nil {
		return err
	}

	// Delete from storage
	if err := s.storage.Delete(ctx, fileID); err != nil && !errors.IsNotFound(err) {
		return errors.E("FileService.Delete", errors.ErrStorageFull, err)
	}

	// Mark metadata as deleted (tombstone for sync)
	meta.LocalState = metadata.LocalStateDeleted
	meta.SyncState = metadata.SyncStatePending
	meta.IncrementClock(s.regionID)

	if err := s.metadata.Save(ctx, meta); err != nil {
		return errors.E("FileService.Delete", errors.ErrInvalidMetadata, err)
	}

	s.logger.Info("file deleted",
		zap.String("file_id", fileID),
	)

	return nil
}

// ListDirectory lists files in a directory.
func (s *FileService) ListDirectory(ctx context.Context, path string) ([]*metadata.DirectoryEntry, error) {
	return s.metadata.List(ctx, path)
}

// hashingReader wraps a reader to compute hash while reading.
type hashingReader struct {
	reader io.Reader
	hasher hash.Hash
}

func newHashingReader(r io.Reader) *hashingReader {
	h := sha256.New()
	return &hashingReader{
		reader: io.TeeReader(r, h),
		hasher: h,
	}
}

func (h *hashingReader) Read(p []byte) (n int, err error) {
	return h.reader.Read(p)
}

func (h *hashingReader) Hash() string {
	return hex.EncodeToString(h.hasher.Sum(nil))
}

// Ensure hashingReader implements io.Reader
var _ io.Reader = (*hashingReader)(nil)
