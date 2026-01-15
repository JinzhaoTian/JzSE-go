// Package metadata provides global metadata management using etcd.
package metadata

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"asisaid.cn/JzSE/internal/common/errors"
	"asisaid.cn/JzSE/internal/common/logger"
	regionmeta "asisaid.cn/JzSE/internal/region/metadata"
	"go.uber.org/zap"
)

// GlobalFileMetadata extends FileMetadata with global information.
type GlobalFileMetadata struct {
	regionmeta.FileMetadata
	Locations []RegionLocation `json:"locations"`
	Primary   string           `json:"primary"`
	Replicas  int              `json:"replicas"`
}

// RegionLocation represents where a file is stored.
type RegionLocation struct {
	RegionID   string    `json:"region_id"`
	State      string    `json:"state"` // synced, syncing, stale
	LastSyncAt time.Time `json:"last_sync_at"`
}

// Manager provides global metadata operations.
type Manager interface {
	// Get retrieves file metadata.
	Get(ctx context.Context, fileID string) (*GlobalFileMetadata, error)

	// Update updates file metadata.
	Update(ctx context.Context, meta *GlobalFileMetadata) error

	// Register registers a new file.
	Register(ctx context.Context, meta *GlobalFileMetadata) error

	// Delete removes file metadata.
	Delete(ctx context.Context, fileID string) error

	// GetLocations returns file locations.
	GetLocations(ctx context.Context, fileID string) ([]RegionLocation, error)

	// Close closes the manager.
	Close() error
}

// ManagerConfig holds configuration for the metadata manager.
type ManagerConfig struct {
	Endpoints   []string
	DialTimeout time.Duration
	Username    string
	Password    string
}

// EtcdManager implements Manager using etcd.
type EtcdManager struct {
	config ManagerConfig
	// client *clientv3.Client // Will be added when etcd is integrated
	store  map[string]*GlobalFileMetadata // In-memory store for now
	logger *zap.Logger
}

// NewEtcdManager creates a new etcd-based metadata manager.
func NewEtcdManager(cfg ManagerConfig) (*EtcdManager, error) {
	log := logger.WithComponent("GlobalMetadataManager")

	// TODO: Initialize etcd client
	// For now, use in-memory store for development
	log.Info("initializing global metadata manager (in-memory mode)")

	return &EtcdManager{
		config: cfg,
		store:  make(map[string]*GlobalFileMetadata),
		logger: log,
	}, nil
}

// Get retrieves file metadata.
func (m *EtcdManager) Get(ctx context.Context, fileID string) (*GlobalFileMetadata, error) {
	meta, ok := m.store[fileID]
	if !ok {
		return nil, errors.ErrNotFound
	}
	return meta, nil
}

// Update updates file metadata.
func (m *EtcdManager) Update(ctx context.Context, meta *GlobalFileMetadata) error {
	existing, ok := m.store[meta.ID]
	if !ok {
		return errors.ErrNotFound
	}

	// Check vector clock for conflicts
	relation := existing.CompareClock(meta.VectorClock)
	if relation == regionmeta.ClockConcurrent {
		m.logger.Warn("conflict detected",
			zap.String("file_id", meta.ID),
		)
		return errors.ErrConflict
	}

	m.store[meta.ID] = meta
	m.logger.Debug("metadata updated",
		zap.String("file_id", meta.ID),
		zap.Int64("version", meta.Version),
	)
	return nil
}

// Register registers a new file.
func (m *EtcdManager) Register(ctx context.Context, meta *GlobalFileMetadata) error {
	if _, ok := m.store[meta.ID]; ok {
		return errors.ErrAlreadyExists
	}

	m.store[meta.ID] = meta
	m.logger.Debug("file registered",
		zap.String("file_id", meta.ID),
		zap.String("path", meta.Path),
	)
	return nil
}

// Delete removes file metadata.
func (m *EtcdManager) Delete(ctx context.Context, fileID string) error {
	if _, ok := m.store[fileID]; !ok {
		return errors.ErrNotFound
	}

	delete(m.store, fileID)
	m.logger.Debug("file deleted", zap.String("file_id", fileID))
	return nil
}

// GetLocations returns file locations.
func (m *EtcdManager) GetLocations(ctx context.Context, fileID string) ([]RegionLocation, error) {
	meta, ok := m.store[fileID]
	if !ok {
		return nil, errors.ErrNotFound
	}
	return meta.Locations, nil
}

// Close closes the manager.
func (m *EtcdManager) Close() error {
	// TODO: Close etcd client
	return nil
}

// MarshalMetadata serializes metadata to JSON.
func MarshalMetadata(meta *GlobalFileMetadata) ([]byte, error) {
	return json.Marshal(meta)
}

// UnmarshalMetadata deserializes metadata from JSON.
func UnmarshalMetadata(data []byte) (*GlobalFileMetadata, error) {
	var meta GlobalFileMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}
	return &meta, nil
}
