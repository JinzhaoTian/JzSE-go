// Package sync provides the sync engine for the coordinator.
package sync

import (
	"context"
	"sync"
	"time"

	"asisaid.cn/JzSE/internal/common/logger"
	"asisaid.cn/JzSE/internal/coordinator/metadata"
	"go.uber.org/zap"
)

// ChangeEvent represents a change event from a region.
type ChangeEvent struct {
	ID          string                       `json:"id"`
	Type        string                       `json:"type"` // CREATE, UPDATE, DELETE
	FileID      string                       `json:"file_id"`
	Metadata    *metadata.GlobalFileMetadata `json:"metadata"`
	VectorClock map[string]uint64            `json:"vector_clock"`
	Timestamp   time.Time                    `json:"timestamp"`
	RegionID    string                       `json:"region_id"`
}

// EngineConfig holds sync engine configuration.
type EngineConfig struct {
	DefaultStrategy  string // eager, lazy, on_demand
	BatchSize        int
	BroadcastTimeout time.Duration
}

// Engine coordinates synchronization across regions.
type Engine struct {
	config      EngineConfig
	metaManager metadata.Manager
	regions     map[string]*RegionState
	mu          sync.RWMutex
	logger      *zap.Logger

	stopCh chan struct{}
	wg     sync.WaitGroup
}

// RegionState tracks the state of a connected region.
type RegionState struct {
	RegionID      string
	LastEventID   string
	LastSyncAt    time.Time
	PendingEvents []*ChangeEvent
}

// NewEngine creates a new sync engine.
func NewEngine(cfg EngineConfig, metaManager metadata.Manager) *Engine {
	return &Engine{
		config:      cfg,
		metaManager: metaManager,
		regions:     make(map[string]*RegionState),
		logger:      logger.WithComponent("SyncEngine"),
		stopCh:      make(chan struct{}),
	}
}

// Start starts the sync engine.
func (e *Engine) Start(ctx context.Context) error {
	e.logger.Info("starting sync engine",
		zap.String("strategy", e.config.DefaultStrategy),
	)

	e.wg.Add(1)
	go e.runEventLoop(ctx)

	return nil
}

// Stop stops the sync engine.
func (e *Engine) Stop() {
	e.logger.Info("stopping sync engine")
	close(e.stopCh)
	e.wg.Wait()
}

// HandleChange processes a change event from a region.
func (e *Engine) HandleChange(ctx context.Context, event *ChangeEvent) error {
	e.logger.Debug("handling change",
		zap.String("event_id", event.ID),
		zap.String("file_id", event.FileID),
		zap.String("region_id", event.RegionID),
		zap.String("type", event.Type),
	)

	// Update global metadata
	switch event.Type {
	case "CREATE":
		if err := e.metaManager.Register(ctx, event.Metadata); err != nil {
			return err
		}
	case "UPDATE":
		if err := e.metaManager.Update(ctx, event.Metadata); err != nil {
			return err
		}
	case "DELETE":
		if err := e.metaManager.Delete(ctx, event.FileID); err != nil {
			return err
		}
	}

	// Broadcast to other regions
	if e.config.DefaultStrategy == "eager" {
		e.broadcastChange(ctx, event)
	}

	return nil
}

// BroadcastChange broadcasts a change to all regions except the source.
func (e *Engine) broadcastChange(ctx context.Context, event *ChangeEvent) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	for regionID, state := range e.regions {
		if regionID == event.RegionID {
			continue
		}

		// Add to pending events for the region
		state.PendingEvents = append(state.PendingEvents, event)
		e.logger.Debug("queued event for region",
			zap.String("region_id", regionID),
			zap.String("event_id", event.ID),
		)
	}
}

// RegisterRegion registers a region with the sync engine.
func (e *Engine) RegisterRegion(regionID string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.regions[regionID] = &RegionState{
		RegionID:      regionID,
		LastSyncAt:    time.Now(),
		PendingEvents: make([]*ChangeEvent, 0),
	}

	e.logger.Info("region registered", zap.String("region_id", regionID))
}

// UnregisterRegion unregisters a region.
func (e *Engine) UnregisterRegion(regionID string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	delete(e.regions, regionID)
	e.logger.Info("region unregistered", zap.String("region_id", regionID))
}

// GetPendingChanges returns pending changes for a region.
func (e *Engine) GetPendingChanges(ctx context.Context, regionID string) ([]*ChangeEvent, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	state, ok := e.regions[regionID]
	if !ok {
		return nil, nil
	}

	events := state.PendingEvents
	state.PendingEvents = make([]*ChangeEvent, 0)
	state.LastSyncAt = time.Now()

	return events, nil
}

// runEventLoop runs the main event processing loop.
func (e *Engine) runEventLoop(ctx context.Context) {
	defer e.wg.Done()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-e.stopCh:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Periodic maintenance tasks
			e.cleanupStaleRegions()
		}
	}
}

// cleanupStaleRegions removes regions that haven't synced recently.
func (e *Engine) cleanupStaleRegions() {
	e.mu.Lock()
	defer e.mu.Unlock()

	staleThreshold := 5 * time.Minute
	now := time.Now()

	for regionID, state := range e.regions {
		if now.Sub(state.LastSyncAt) > staleThreshold {
			e.logger.Warn("region appears stale",
				zap.String("region_id", regionID),
				zap.Time("last_sync", state.LastSyncAt),
			)
		}
	}
}
