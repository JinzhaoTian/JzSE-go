package conflict

import (
	"context"
	"testing"
	"time"

	"asisaid.cn/JzSE/internal/coordinator/metadata"
	regionmeta "asisaid.cn/JzSE/internal/region/metadata"
)

func TestResolver_Detect(t *testing.T) {
	resolver := NewResolver(StrategyLWW)

	t.Run("no conflict - equal clocks", func(t *testing.T) {
		local := &metadata.GlobalFileMetadata{
			FileMetadata: regionmeta.FileMetadata{
				ID:          "file-1",
				VectorClock: map[string]uint64{"a": 1, "b": 2},
			},
		}
		remote := &metadata.GlobalFileMetadata{
			FileMetadata: regionmeta.FileMetadata{
				ID:          "file-1",
				VectorClock: map[string]uint64{"a": 1, "b": 2},
			},
		}

		conflict := resolver.Detect(local, remote)
		if conflict != nil {
			t.Error("should not detect conflict for equal clocks")
		}
	})

	t.Run("no conflict - one before other", func(t *testing.T) {
		local := &metadata.GlobalFileMetadata{
			FileMetadata: regionmeta.FileMetadata{
				ID:          "file-1",
				VectorClock: map[string]uint64{"a": 1, "b": 1},
			},
		}
		remote := &metadata.GlobalFileMetadata{
			FileMetadata: regionmeta.FileMetadata{
				ID:          "file-1",
				VectorClock: map[string]uint64{"a": 2, "b": 2},
			},
		}

		conflict := resolver.Detect(local, remote)
		if conflict != nil {
			t.Error("should not detect conflict when one is before other")
		}
	})

	t.Run("conflict - concurrent", func(t *testing.T) {
		local := &metadata.GlobalFileMetadata{
			FileMetadata: regionmeta.FileMetadata{
				ID:          "file-1",
				VectorClock: map[string]uint64{"a": 2, "b": 1},
			},
		}
		remote := &metadata.GlobalFileMetadata{
			FileMetadata: regionmeta.FileMetadata{
				ID:          "file-1",
				VectorClock: map[string]uint64{"a": 1, "b": 2},
			},
		}

		conflict := resolver.Detect(local, remote)
		if conflict == nil {
			t.Fatal("should detect conflict for concurrent clocks")
		}
		if conflict.FileID != "file-1" {
			t.Errorf("FileID = %v, want file-1", conflict.FileID)
		}
		if conflict.Status != "pending" {
			t.Errorf("Status = %v, want pending", conflict.Status)
		}
	})
}

func TestResolver_Resolve_LWW(t *testing.T) {
	resolver := NewResolver(StrategyLWW)
	ctx := context.Background()

	now := time.Now()
	earlier := now.Add(-time.Hour)

	conflict := &Conflict{
		ID:     "conflict-1",
		FileID: "file-1",
		LocalVersion: &metadata.GlobalFileMetadata{
			FileMetadata: regionmeta.FileMetadata{
				ID:        "file-1",
				Name:      "local.txt",
				UpdatedAt: earlier,
			},
		},
		RemoteVersion: &metadata.GlobalFileMetadata{
			FileMetadata: regionmeta.FileMetadata{
				ID:        "file-1",
				Name:      "remote.txt",
				UpdatedAt: now,
			},
		},
		Status: "pending",
	}

	resolution, err := resolver.Resolve(ctx, conflict, StrategyLWW)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}

	if resolution.Strategy != StrategyLWW {
		t.Errorf("Strategy = %v, want %v", resolution.Strategy, StrategyLWW)
	}

	// Remote is newer, should win
	if resolution.Result.Name != "remote.txt" {
		t.Errorf("Result.Name = %v, want remote.txt", resolution.Result.Name)
	}

	if resolution.Rejected == nil || resolution.Rejected.Name != "local.txt" {
		t.Error("Rejected should be local version")
	}

	if conflict.Status != "resolved" {
		t.Errorf("conflict.Status = %v, want resolved", conflict.Status)
	}
}

func TestResolver_Resolve_Fork(t *testing.T) {
	resolver := NewResolver(StrategyFork)
	ctx := context.Background()

	conflict := &Conflict{
		ID:     "conflict-2",
		FileID: "file-2",
		LocalVersion: &metadata.GlobalFileMetadata{
			FileMetadata: regionmeta.FileMetadata{
				ID:   "file-2",
				Name: "file.txt",
				Path: "/path/file.txt",
			},
		},
		RemoteVersion: &metadata.GlobalFileMetadata{
			FileMetadata: regionmeta.FileMetadata{
				ID:   "file-2",
				Name: "file.txt",
				Path: "/path/file.txt",
			},
		},
		Status: "pending",
	}

	resolution, err := resolver.Resolve(ctx, conflict, StrategyFork)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}

	if resolution.Strategy != StrategyFork {
		t.Errorf("Strategy = %v, want %v", resolution.Strategy, StrategyFork)
	}

	// Forked file should have .conflict suffix
	if resolution.Result.Name != "file.txt.conflict" {
		t.Errorf("Result.Name = %v, want file.txt.conflict", resolution.Result.Name)
	}
}

func TestStrategy_Constants(t *testing.T) {
	strategies := []Strategy{
		StrategyLWW,
		StrategyMerge,
		StrategyManual,
		StrategyFork,
	}

	for _, s := range strategies {
		if s == "" {
			t.Error("strategy should not be empty")
		}
	}
}
