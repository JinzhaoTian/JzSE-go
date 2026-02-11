// Package sync provides the sync agent for region-to-coordinator communication.
package sync

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"

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
	RegionID       string
	CoordinatorURL string
	Mode           string // push, batch, pull
	BatchSize      int
	BatchInterval  time.Duration
	RetryInterval  time.Duration
	MaxRetries     int
}

// Agent handles synchronization between region and coordinator.
type Agent struct {
	config     AgentConfig
	metaStore  metadata.Store
	queue      *ChangeQueue
	httpClient *http.Client
	logger     *zap.Logger

	stopCh chan struct{}
	wg     sync.WaitGroup
}

// NewAgent creates a new sync agent.
func NewAgent(cfg AgentConfig, metaStore metadata.Store) *Agent {
	return &Agent{
		config:    cfg,
		metaStore: metaStore,
		queue:     NewChangeQueue(10000),
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		logger: logger.WithComponent("SyncAgent"),
		stopCh: make(chan struct{}),
	}
}

// Start starts the sync agent.
func (a *Agent) Start(ctx context.Context) error {
	a.logger.Info("starting sync agent",
		zap.String("region_id", a.config.RegionID),
		zap.String("mode", a.config.Mode),
		zap.String("coordinator_url", a.config.CoordinatorURL),
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

	// Always run pull loop alongside push/batch to receive changes from other regions
	if a.config.Mode != "pull" {
		a.wg.Add(1)
		go a.runPullMode(ctx)
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
		ID:          uuid.New().String(),
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

	interval := a.config.BatchInterval
	if interval == 0 {
		interval = 5 * time.Second
	}

	ticker := time.NewTicker(interval)
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

// syncEvent syncs a single event to the coordinator via HTTP POST.
func (a *Agent) syncEvent(ctx context.Context, event *ChangeEvent) error {
	if a.config.CoordinatorURL == "" {
		a.logger.Debug("no coordinator URL configured, skipping sync")
		return nil
	}

	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	url := a.config.CoordinatorURL + "/api/v1/sync/changes"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send event to coordinator: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		a.logger.Debug("event synced successfully",
			zap.String("event_id", event.ID),
			zap.String("file_id", event.FileID),
			zap.String("type", string(event.Type)),
		)
		return nil
	}

	respBody, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("coordinator returned status %d: %s", resp.StatusCode, string(respBody))
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

// pullChanges pulls changes from the coordinator and applies them locally.
func (a *Agent) pullChanges(ctx context.Context) {
	if a.config.CoordinatorURL == "" {
		return
	}

	url := a.config.CoordinatorURL + "/api/v1/sync/pending/" + a.config.RegionID
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		a.logger.Error("failed to create pull request", zap.Error(err))
		return
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		a.logger.Warn("failed to pull changes from coordinator", zap.Error(err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		a.logger.Warn("coordinator returned non-OK status for pull",
			zap.Int("status", resp.StatusCode),
		)
		return
	}

	var events []*ChangeEvent
	if err := json.NewDecoder(resp.Body).Decode(&events); err != nil {
		a.logger.Error("failed to decode pull response", zap.Error(err))
		return
	}

	if len(events) == 0 {
		return
	}

	a.logger.Info("pulled changes from coordinator", zap.Int("count", len(events)))

	for _, event := range events {
		a.applyRemoteChange(ctx, event)
	}
}

// applyRemoteChange applies a change received from the coordinator to local metadata.
func (a *Agent) applyRemoteChange(ctx context.Context, event *ChangeEvent) {
	if event.Metadata == nil {
		a.logger.Warn("received event with nil metadata", zap.String("event_id", event.ID))
		return
	}

	switch event.Type {
	case ChangeTypeCreate, ChangeTypeUpdate:
		// Save metadata locally — file data is not transferred yet (metadata-only sync)
		meta := event.Metadata
		meta.LocalState = metadata.LocalStatePending
		meta.SyncState = metadata.SyncStateSynced
		meta.MergeClock(event.VectorClock)

		if err := a.metaStore.Save(ctx, meta); err != nil {
			a.logger.Error("failed to apply remote change",
				zap.String("event_id", event.ID),
				zap.String("file_id", event.FileID),
				zap.Error(err),
			)
			return
		}

	case ChangeTypeDelete:
		existing, err := a.metaStore.Get(ctx, event.FileID)
		if err != nil {
			// File doesn't exist locally, nothing to delete
			return
		}
		existing.LocalState = metadata.LocalStateDeleted
		existing.SyncState = metadata.SyncStateSynced
		existing.MergeClock(event.VectorClock)

		if err := a.metaStore.Save(ctx, existing); err != nil {
			a.logger.Error("failed to apply remote delete",
				zap.String("event_id", event.ID),
				zap.String("file_id", event.FileID),
				zap.Error(err),
			)
		}
	}

	a.logger.Debug("applied remote change",
		zap.String("event_id", event.ID),
		zap.String("file_id", event.FileID),
		zap.String("type", string(event.Type)),
	)
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
