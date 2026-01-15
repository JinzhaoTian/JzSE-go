// Package sync provides the sync agent for region-to-coordinator communication.
package sync

import (
	"context"
	"sync"
	"time"

	"asisaid.cn/JzSE/internal/common/logger"
	"asisaid.cn/JzSE/internal/region/metadata"
	"go.uber.org/zap"
)

// ChangeType represents the type of a change event.
type ChangeType string

const (
	ChangeTypeCreate ChangeType = "CREATE"
	ChangeTypeUpdate ChangeType = "UPDATE"
	ChangeTypeDelete ChangeType = "DELETE"
)

// ChangeEvent represents a change that needs to be synced.
type ChangeEvent struct {
	ID          string                 `json:"id"`
	Type        ChangeType             `json:"type"`
	FileID      string                 `json:"file_id"`
	Metadata    *metadata.FileMetadata `json:"metadata"`
	VectorClock map[string]uint64      `json:"vector_clock"`
	Timestamp   time.Time              `json:"timestamp"`
	RegionID    string                 `json:"region_id"`
	Attempts    int                    `json:"attempts"`
}

// AgentConfig holds configuration for the sync agent.
type AgentConfig struct {
	RegionID      string
	Mode          string // push, batch, pull
	BatchSize     int
	BatchInterval time.Duration
	RetryInterval time.Duration
	MaxRetries    int
}

// Agent handles synchronization between region and coordinator.
type Agent struct {
	config    AgentConfig
	metaStore metadata.Store
	queue     *ChangeQueue
	logger    *zap.Logger

	stopCh chan struct{}
	wg     sync.WaitGroup
}

// NewAgent creates a new sync agent.
func NewAgent(cfg AgentConfig, metaStore metadata.Store) *Agent {
	return &Agent{
		config:    cfg,
		metaStore: metaStore,
		queue:     NewChangeQueue(10000),
		logger:    logger.WithComponent("SyncAgent"),
		stopCh:    make(chan struct{}),
	}
}

// Start starts the sync agent.
func (a *Agent) Start(ctx context.Context) error {
	a.logger.Info("starting sync agent",
		zap.String("region_id", a.config.RegionID),
		zap.String("mode", a.config.Mode),
	)

	switch a.config.Mode {
	case "push":
		a.wg.Add(1)
		go a.runPushMode(ctx)
	case "batch":
		a.wg.Add(1)
		go a.runBatchMode(ctx)
	case "pull":
		a.wg.Add(1)
		go a.runPullMode(ctx)
	default:
		a.wg.Add(1)
		go a.runPushMode(ctx)
	}

	return nil
}

// Stop stops the sync agent.
func (a *Agent) Stop() {
	a.logger.Info("stopping sync agent")
	close(a.stopCh)
	a.wg.Wait()
}

// QueueChange adds a change event to the sync queue.
func (a *Agent) QueueChange(changeType ChangeType, meta *metadata.FileMetadata) {
	event := &ChangeEvent{
		ID:          generateEventID(),
		Type:        changeType,
		FileID:      meta.ID,
		Metadata:    meta,
		VectorClock: meta.VectorClock,
		Timestamp:   time.Now(),
		RegionID:    a.config.RegionID,
	}

	if err := a.queue.Push(event); err != nil {
		a.logger.Error("failed to queue change", zap.Error(err))
	}
}

// GetQueueSize returns the current queue size.
func (a *Agent) GetQueueSize() int {
	return a.queue.Len()
}

// runPushMode immediately pushes changes to coordinator.
func (a *Agent) runPushMode(ctx context.Context) {
	defer a.wg.Done()

	for {
		select {
		case <-a.stopCh:
			return
		case <-ctx.Done():
			return
		default:
			event := a.queue.Pop()
			if event == nil {
				time.Sleep(100 * time.Millisecond)
				continue
			}

			if err := a.syncEvent(ctx, event); err != nil {
				a.handleSyncError(event, err)
			}
		}
	}
}

// runBatchMode batches changes before syncing.
func (a *Agent) runBatchMode(ctx context.Context) {
	defer a.wg.Done()

	ticker := time.NewTicker(a.config.BatchInterval)
	defer ticker.Stop()

	for {
		select {
		case <-a.stopCh:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.syncBatch(ctx)
		}
	}
}

// runPullMode periodically pulls changes from coordinator.
func (a *Agent) runPullMode(ctx context.Context) {
	defer a.wg.Done()

	ticker := time.NewTicker(a.config.BatchInterval)
	defer ticker.Stop()

	for {
		select {
		case <-a.stopCh:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.pullChanges(ctx)
		}
	}
}

// syncEvent syncs a single event to the coordinator.
func (a *Agent) syncEvent(ctx context.Context, event *ChangeEvent) error {
	// TODO: Implement actual coordinator communication
	a.logger.Debug("syncing event",
		zap.String("event_id", event.ID),
		zap.String("file_id", event.FileID),
		zap.String("type", string(event.Type)),
	)
	return nil
}

// syncBatch syncs a batch of events.
func (a *Agent) syncBatch(ctx context.Context) {
	events := a.queue.PopN(a.config.BatchSize)
	if len(events) == 0 {
		return
	}

	a.logger.Debug("syncing batch", zap.Int("count", len(events)))

	for _, event := range events {
		if err := a.syncEvent(ctx, event); err != nil {
			a.handleSyncError(event, err)
		}
	}
}

// pullChanges pulls changes from the coordinator.
func (a *Agent) pullChanges(ctx context.Context) {
	// TODO: Implement pulling from coordinator
	a.logger.Debug("pulling changes from coordinator")
}

// handleSyncError handles sync errors with retry logic.
func (a *Agent) handleSyncError(event *ChangeEvent, err error) {
	event.Attempts++
	a.logger.Warn("sync failed",
		zap.String("event_id", event.ID),
		zap.Int("attempts", event.Attempts),
		zap.Error(err),
	)

	if event.Attempts < a.config.MaxRetries {
		// Re-queue for retry
		_ = a.queue.Push(event)
	} else {
		a.logger.Error("max retries exceeded, dropping event",
			zap.String("event_id", event.ID),
		)
	}
}

// generateEventID generates a unique event ID.
func generateEventID() string {
	return time.Now().Format("20060102150405.000000000")
}
