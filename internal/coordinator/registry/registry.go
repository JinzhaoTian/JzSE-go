// Package registry provides region registration and health management.
package registry

import (
	"context"
	"sync"
	"time"

	"asisaid.cn/JzSE/internal/common/errors"
	"asisaid.cn/JzSE/internal/common/logger"
	"go.uber.org/zap"
)

// RegionInfo holds information about a registered region.
type RegionInfo struct {
	ID         string       `json:"id"`
	Name       string       `json:"name"`
	Endpoint   string       `json:"endpoint"`
	Location   GeoLocation  `json:"location"`
	Capacity   Capacity     `json:"capacity"`
	Status     RegionStatus `json:"status"`
	JoinedAt   time.Time    `json:"joined_at"`
	LastSeenAt time.Time    `json:"last_seen_at"`
}

// GeoLocation represents geographical location.
type GeoLocation struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	City      string  `json:"city"`
	Country   string  `json:"country"`
}

// Capacity represents storage capacity.
type Capacity struct {
	TotalBytes int64 `json:"total_bytes"`
	UsedBytes  int64 `json:"used_bytes"`
	FreeBytes  int64 `json:"free_bytes"`
}

// RegionStatus represents the health status of a region.
type RegionStatus struct {
	State       string    `json:"state"` // healthy, degraded, offline
	SyncLag     int64     `json:"sync_lag"`
	LoadLevel   float64   `json:"load_level"`
	LastCheckAt time.Time `json:"last_check_at"`
}

// Registry manages region registration and health.
type Registry struct {
	regions map[string]*RegionInfo
	mu      sync.RWMutex
	logger  *zap.Logger

	stopCh chan struct{}
	wg     sync.WaitGroup
}

// NewRegistry creates a new region registry.
func NewRegistry() *Registry {
	return &Registry{
		regions: make(map[string]*RegionInfo),
		logger:  logger.WithComponent("RegionRegistry"),
		stopCh:  make(chan struct{}),
	}
}

// Start starts the registry health checker.
func (r *Registry) Start(ctx context.Context) error {
	r.logger.Info("starting region registry")

	r.wg.Add(1)
	go r.runHealthChecker(ctx)

	return nil
}

// Stop stops the registry.
func (r *Registry) Stop() {
	r.logger.Info("stopping region registry")
	close(r.stopCh)
	r.wg.Wait()
}

// Register registers a new region.
func (r *Registry) Register(ctx context.Context, region *RegionInfo) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.regions[region.ID]; ok {
		return errors.ErrAlreadyExists
	}

	region.JoinedAt = time.Now()
	region.LastSeenAt = time.Now()
	region.Status.State = "healthy"
	region.Status.LastCheckAt = time.Now()

	r.regions[region.ID] = region

	r.logger.Info("region registered",
		zap.String("region_id", region.ID),
		zap.String("name", region.Name),
		zap.String("endpoint", region.Endpoint),
	)

	return nil
}

// Deregister removes a region.
func (r *Registry) Deregister(ctx context.Context, regionID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.regions[regionID]; !ok {
		return errors.ErrNotFound
	}

	delete(r.regions, regionID)

	r.logger.Info("region deregistered", zap.String("region_id", regionID))
	return nil
}

// Heartbeat updates region health status.
func (r *Registry) Heartbeat(ctx context.Context, regionID string, status *RegionStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	region, ok := r.regions[regionID]
	if !ok {
		return errors.ErrNotFound
	}

	region.LastSeenAt = time.Now()
	region.Status = *status
	region.Status.LastCheckAt = time.Now()

	return nil
}

// GetRegion returns information about a specific region.
func (r *Registry) GetRegion(ctx context.Context, regionID string) (*RegionInfo, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	region, ok := r.regions[regionID]
	if !ok {
		return nil, errors.ErrNotFound
	}

	return region, nil
}

// GetActiveRegions returns all active regions.
func (r *Registry) GetActiveRegions(ctx context.Context) ([]*RegionInfo, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*RegionInfo
	for _, region := range r.regions {
		if region.Status.State != "offline" {
			result = append(result, region)
		}
	}

	return result, nil
}

// GetHealthyRegions returns only healthy regions.
func (r *Registry) GetHealthyRegions(ctx context.Context) ([]*RegionInfo, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*RegionInfo
	for _, region := range r.regions {
		if region.Status.State == "healthy" {
			result = append(result, region)
		}
	}

	return result, nil
}

// runHealthChecker periodically checks region health.
func (r *Registry) runHealthChecker(ctx context.Context) {
	defer r.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.stopCh:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.checkHealth()
		}
	}
}

// checkHealth updates region health based on last seen time.
func (r *Registry) checkHealth() {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	degradedThreshold := 1 * time.Minute
	offlineThreshold := 5 * time.Minute

	for _, region := range r.regions {
		elapsed := now.Sub(region.LastSeenAt)

		switch {
		case elapsed > offlineThreshold:
			if region.Status.State != "offline" {
				region.Status.State = "offline"
				r.logger.Warn("region marked offline",
					zap.String("region_id", region.ID),
					zap.Duration("elapsed", elapsed),
				)
			}
		case elapsed > degradedThreshold:
			if region.Status.State == "healthy" {
				region.Status.State = "degraded"
				r.logger.Warn("region marked degraded",
					zap.String("region_id", region.ID),
					zap.Duration("elapsed", elapsed),
				)
			}
		}
	}
}
