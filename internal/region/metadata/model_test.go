package metadata

import (
	"testing"
	"time"
)

func TestNewFileMetadata(t *testing.T) {
	meta := NewFileMetadata("test-id", "test.txt", "/path/test.txt")

	if meta.ID != "test-id" {
		t.Errorf("ID = %v, want test-id", meta.ID)
	}
	if meta.Name != "test.txt" {
		t.Errorf("Name = %v, want test.txt", meta.Name)
	}
	if meta.Path != "/path/test.txt" {
		t.Errorf("Path = %v, want /path/test.txt", meta.Path)
	}
	if meta.Version != 1 {
		t.Errorf("Version = %v, want 1", meta.Version)
	}
	if meta.VectorClock == nil {
		t.Error("VectorClock should not be nil")
	}
	if meta.LocalState != LocalStatePresent {
		t.Errorf("LocalState = %v, want present", meta.LocalState)
	}
	if meta.SyncState != SyncStatePending {
		t.Errorf("SyncState = %v, want pending", meta.SyncState)
	}
}

func TestFileMetadata_IncrementClock(t *testing.T) {
	meta := NewFileMetadata("id", "name", "/path")

	meta.IncrementClock("region-a")
	if meta.VectorClock["region-a"] != 1 {
		t.Errorf("VectorClock[region-a] = %v, want 1", meta.VectorClock["region-a"])
	}
	if meta.Version != 2 {
		t.Errorf("Version = %v, want 2", meta.Version)
	}

	meta.IncrementClock("region-a")
	if meta.VectorClock["region-a"] != 2 {
		t.Errorf("VectorClock[region-a] = %v, want 2", meta.VectorClock["region-a"])
	}

	meta.IncrementClock("region-b")
	if meta.VectorClock["region-b"] != 1 {
		t.Errorf("VectorClock[region-b] = %v, want 1", meta.VectorClock["region-b"])
	}
}

func TestFileMetadata_MergeClock(t *testing.T) {
	meta := NewFileMetadata("id", "name", "/path")
	meta.VectorClock["region-a"] = 2
	meta.VectorClock["region-b"] = 3

	other := map[string]uint64{
		"region-a": 1, // lower, should not change
		"region-b": 5, // higher, should update
		"region-c": 2, // new, should add
	}

	meta.MergeClock(other)

	if meta.VectorClock["region-a"] != 2 {
		t.Errorf("VectorClock[region-a] = %v, want 2", meta.VectorClock["region-a"])
	}
	if meta.VectorClock["region-b"] != 5 {
		t.Errorf("VectorClock[region-b] = %v, want 5", meta.VectorClock["region-b"])
	}
	if meta.VectorClock["region-c"] != 2 {
		t.Errorf("VectorClock[region-c] = %v, want 2", meta.VectorClock["region-c"])
	}
}

func TestFileMetadata_CompareClock(t *testing.T) {
	tests := []struct {
		name   string
		vc1    map[string]uint64
		vc2    map[string]uint64
		expect ClockRelation
	}{
		{
			name:   "equal",
			vc1:    map[string]uint64{"a": 1, "b": 2},
			vc2:    map[string]uint64{"a": 1, "b": 2},
			expect: ClockEqual,
		},
		{
			name:   "before",
			vc1:    map[string]uint64{"a": 1, "b": 1},
			vc2:    map[string]uint64{"a": 2, "b": 2},
			expect: ClockBefore,
		},
		{
			name:   "after",
			vc1:    map[string]uint64{"a": 2, "b": 2},
			vc2:    map[string]uint64{"a": 1, "b": 1},
			expect: ClockAfter,
		},
		{
			name:   "concurrent",
			vc1:    map[string]uint64{"a": 2, "b": 1},
			vc2:    map[string]uint64{"a": 1, "b": 2},
			expect: ClockConcurrent,
		},
		{
			name:   "both nil",
			vc1:    nil,
			vc2:    nil,
			expect: ClockEqual,
		},
		{
			name:   "first nil",
			vc1:    nil,
			vc2:    map[string]uint64{"a": 1},
			expect: ClockBefore,
		},
		{
			name:   "second nil",
			vc1:    map[string]uint64{"a": 1},
			vc2:    nil,
			expect: ClockAfter,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta := &FileMetadata{VectorClock: tt.vc1}
			result := meta.CompareClock(tt.vc2)
			if result != tt.expect {
				t.Errorf("CompareClock() = %v, want %v", result, tt.expect)
			}
		})
	}
}

func TestDirectoryEntry(t *testing.T) {
	entry := DirectoryEntry{
		Name:      "file.txt",
		Path:      "/path/file.txt",
		IsDir:     false,
		Size:      1024,
		UpdatedAt: time.Now(),
	}

	if entry.Name != "file.txt" {
		t.Errorf("Name = %v, want file.txt", entry.Name)
	}
	if entry.IsDir {
		t.Error("IsDir should be false")
	}
}
