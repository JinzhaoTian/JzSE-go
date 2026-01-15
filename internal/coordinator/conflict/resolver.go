// Package conflict provides conflict detection and resolution.
package conflict

import (
	"context"
	"time"

	"asisaid.cn/JzSE/internal/common/logger"
	"asisaid.cn/JzSE/internal/coordinator/metadata"
	regionmeta "asisaid.cn/JzSE/internal/region/metadata"
	"go.uber.org/zap"
)

// Strategy defines how conflicts are resolved.
type Strategy string

const (
	StrategyLWW    Strategy = "last_writer_wins" // Last writer wins based on timestamp
	StrategyMerge  Strategy = "merge"            // Attempt to merge changes
	StrategyManual Strategy = "manual"           // Require manual resolution
	StrategyFork   Strategy = "fork"             // Create both versions
)

// Conflict represents a detected conflict.
type Conflict struct {
	ID            string                       `json:"id"`
	FileID        string                       `json:"file_id"`
	LocalVersion  *metadata.GlobalFileMetadata `json:"local_version"`
	RemoteVersion *metadata.GlobalFileMetadata `json:"remote_version"`
	DetectedAt    time.Time                    `json:"detected_at"`
	Status        string                       `json:"status"` // pending, resolved, escalated
}

// Resolution represents the result of conflict resolution.
type Resolution struct {
	ConflictID string                       `json:"conflict_id"`
	Strategy   Strategy                     `json:"strategy"`
	Result     *metadata.GlobalFileMetadata `json:"result"`
	Rejected   *metadata.GlobalFileMetadata `json:"rejected,omitempty"`
	ResolvedAt time.Time                    `json:"resolved_at"`
}

// Resolver handles conflict detection and resolution.
type Resolver struct {
	defaultStrategy Strategy
	logger          *zap.Logger
}

// NewResolver creates a new conflict resolver.
func NewResolver(defaultStrategy Strategy) *Resolver {
	return &Resolver{
		defaultStrategy: defaultStrategy,
		logger:          logger.WithComponent("ConflictResolver"),
	}
}

// Detect checks if two metadata versions are in conflict.
func (r *Resolver) Detect(local, remote *metadata.GlobalFileMetadata) *Conflict {
	relation := local.CompareClock(remote.VectorClock)

	if relation != regionmeta.ClockConcurrent {
		return nil // No conflict
	}

	conflict := &Conflict{
		ID:            generateConflictID(),
		FileID:        local.ID,
		LocalVersion:  local,
		RemoteVersion: remote,
		DetectedAt:    time.Now(),
		Status:        "pending",
	}

	r.logger.Warn("conflict detected",
		zap.String("conflict_id", conflict.ID),
		zap.String("file_id", conflict.FileID),
	)

	return conflict
}

// Resolve resolves a conflict using the specified strategy.
func (r *Resolver) Resolve(ctx context.Context, conflict *Conflict, strategy Strategy) (*Resolution, error) {
	if strategy == "" {
		strategy = r.defaultStrategy
	}

	r.logger.Info("resolving conflict",
		zap.String("conflict_id", conflict.ID),
		zap.String("strategy", string(strategy)),
	)

	var result *metadata.GlobalFileMetadata
	var rejected *metadata.GlobalFileMetadata

	switch strategy {
	case StrategyLWW:
		result, rejected = r.resolveLastWriterWins(conflict)
	case StrategyFork:
		result = r.resolveFork(conflict)
	default:
		result, rejected = r.resolveLastWriterWins(conflict)
	}

	resolution := &Resolution{
		ConflictID: conflict.ID,
		Strategy:   strategy,
		Result:     result,
		Rejected:   rejected,
		ResolvedAt: time.Now(),
	}

	conflict.Status = "resolved"

	r.logger.Info("conflict resolved",
		zap.String("conflict_id", conflict.ID),
		zap.String("winner_version", result.ID),
	)

	return resolution, nil
}

// resolveLastWriterWins resolves by choosing the most recent write.
func (r *Resolver) resolveLastWriterWins(conflict *Conflict) (winner, loser *metadata.GlobalFileMetadata) {
	if conflict.LocalVersion.UpdatedAt.After(conflict.RemoteVersion.UpdatedAt) {
		return conflict.LocalVersion, conflict.RemoteVersion
	}
	return conflict.RemoteVersion, conflict.LocalVersion
}

// resolveFork creates a copy with conflict suffix.
func (r *Resolver) resolveFork(conflict *Conflict) *metadata.GlobalFileMetadata {
	// Keep both versions - return the local one and rename remote
	forked := *conflict.RemoteVersion
	forked.Path = forked.Path + ".conflict"
	forked.Name = forked.Name + ".conflict"
	return &forked
}

// generateConflictID generates a unique conflict ID.
func generateConflictID() string {
	return "conflict-" + time.Now().Format("20060102150405.000")
}
