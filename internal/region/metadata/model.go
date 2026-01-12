// Package metadata defines data models for file metadata.
package metadata

import (
	"time"
)

// FileMetadata represents the metadata of a file.
type FileMetadata struct {
	// Basic information
	ID          string `json:"id"`           // Global unique ID (UUID)
	Name        string `json:"name"`         // File name
	Path        string `json:"path"`         // Full path
	Size        int64  `json:"size"`         // File size in bytes
	ContentHash string `json:"content_hash"` // SHA-256 hash of content
	MimeType    string `json:"mime_type"`    // MIME type

	// Versioning
	Version     int64             `json:"version"`      // Version number
	VectorClock map[string]uint64 `json:"vector_clock"` // Vector clock for conflict detection

	// Ownership and timestamps
	OwnerID   string    `json:"owner_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	CreatedBy string    `json:"created_by"`
	UpdatedBy string    `json:"updated_by"`

	// Distribution information
	OriginRegion string     `json:"origin_region"` // Region where file was created
	LocalState   LocalState `json:"local_state"`   // State of file in local storage
	SyncState    SyncState  `json:"sync_state"`    // Sync status

	// Custom metadata
	CustomMeta map[string]string `json:"custom_meta,omitempty"`
}

// LocalState represents the local storage state of a file.
type LocalState string

const (
	LocalStatePresent LocalState = "present" // File exists locally
	LocalStatePending LocalState = "pending" // Waiting to be downloaded
	LocalStateDeleted LocalState = "deleted" // Deleted (tombstone)
)

// SyncState represents the synchronization state of a file.
type SyncState string

const (
	SyncStateSynced   SyncState = "synced"   // Synced with global
	SyncStatePending  SyncState = "pending"  // Waiting to be synced
	SyncStateConflict SyncState = "conflict" // Conflict detected
)

// NewFileMetadata creates a new FileMetadata with default values.
func NewFileMetadata(id, name, path string) *FileMetadata {
	now := time.Now()
	return &FileMetadata{
		ID:          id,
		Name:        name,
		Path:        path,
		Version:     1,
		VectorClock: make(map[string]uint64),
		LocalState:  LocalStatePresent,
		SyncState:   SyncStatePending,
		CreatedAt:   now,
		UpdatedAt:   now,
		CustomMeta:  make(map[string]string),
	}
}

// IncrementClock increments the vector clock for the given region.
func (m *FileMetadata) IncrementClock(regionID string) {
	if m.VectorClock == nil {
		m.VectorClock = make(map[string]uint64)
	}
	m.VectorClock[regionID]++
	m.Version++
	m.UpdatedAt = time.Now()
}

// MergeClock merges another vector clock into this one.
func (m *FileMetadata) MergeClock(other map[string]uint64) {
	if m.VectorClock == nil {
		m.VectorClock = make(map[string]uint64)
	}
	for k, v := range other {
		if m.VectorClock[k] < v {
			m.VectorClock[k] = v
		}
	}
}

// ClockRelation represents the relationship between two vector clocks.
type ClockRelation int

const (
	ClockBefore     ClockRelation = iota // This clock happened before other
	ClockAfter                           // This clock happened after other
	ClockEqual                           // Clocks are equal
	ClockConcurrent                      // Clocks are concurrent (potential conflict)
)

// CompareClock compares this vector clock with another.
func (m *FileMetadata) CompareClock(other map[string]uint64) ClockRelation {
	if m.VectorClock == nil && other == nil {
		return ClockEqual
	}
	if m.VectorClock == nil {
		return ClockBefore
	}
	if other == nil {
		return ClockAfter
	}

	// Collect all keys
	allKeys := make(map[string]bool)
	for k := range m.VectorClock {
		allKeys[k] = true
	}
	for k := range other {
		allKeys[k] = true
	}

	lessThan := false
	greaterThan := false

	for k := range allKeys {
		v1 := m.VectorClock[k]
		v2 := other[k]
		if v1 < v2 {
			lessThan = true
		} else if v1 > v2 {
			greaterThan = true
		}
	}

	switch {
	case lessThan && greaterThan:
		return ClockConcurrent
	case lessThan:
		return ClockBefore
	case greaterThan:
		return ClockAfter
	default:
		return ClockEqual
	}
}

// DirectoryEntry represents an entry in a directory listing.
type DirectoryEntry struct {
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	IsDir     bool      `json:"is_dir"`
	Size      int64     `json:"size,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}
