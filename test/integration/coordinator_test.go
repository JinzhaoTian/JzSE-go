// Package integration provides integration tests for the JzSE system.
package integration

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"asisaid.cn/JzSE/internal/coordinator/conflict"
	"asisaid.cn/JzSE/internal/coordinator/metadata"
	"asisaid.cn/JzSE/internal/coordinator/registry"
	coordsync "asisaid.cn/JzSE/internal/coordinator/sync"
)

// CoordinatorTestEnv provides a test environment for coordinator tests.
type CoordinatorTestEnv struct {
	Router      *gin.Engine
	MetaManager metadata.Manager
	Registry    *registry.Registry
	SyncEngine  *coordsync.Engine
	Resolver    *conflict.Resolver
}

// SetupCoordinatorTestEnv creates a new coordinator test environment.
func SetupCoordinatorTestEnv(t *testing.T) *CoordinatorTestEnv {
	t.Helper()

	gin.SetMode(gin.TestMode)

	metaManager, err := metadata.NewEtcdManager(metadata.ManagerConfig{})
	if err != nil {
		t.Fatalf("failed to create metadata manager: %v", err)
	}

	reg := registry.NewRegistry()

	syncEngine := coordsync.NewEngine(coordsync.EngineConfig{
		DefaultStrategy:  "lazy",
		BatchSize:        100,
		BroadcastTimeout: 10 * time.Second,
	}, metaManager)

	resolver := conflict.NewResolver(conflict.StrategyLWW)

	router := gin.New()
	registerTestRoutes(router, metaManager, reg, syncEngine)

	return &CoordinatorTestEnv{
		Router:      router,
		MetaManager: metaManager,
		Registry:    reg,
		SyncEngine:  syncEngine,
		Resolver:    resolver,
	}
}

func registerTestRoutes(r *gin.Engine, metaManager metadata.Manager, reg *registry.Registry, sync *coordsync.Engine) {
	api := r.Group("/api/v1")
	{
		api.GET("/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "healthy"})
		})

		api.GET("/regions", func(c *gin.Context) {
			regions, _ := reg.GetActiveRegions(c.Request.Context())
			c.JSON(http.StatusOK, regions)
		})

		api.GET("/regions/:id", func(c *gin.Context) {
			regionID := c.Param("id")
			info, err := reg.GetRegion(c.Request.Context(), regionID)
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, info)
		})
	}
}

// Cleanup cleans up the test environment.
func (e *CoordinatorTestEnv) Cleanup() {
	if e.MetaManager != nil {
		e.MetaManager.Close()
	}
}

func TestCoordinatorAPI_HealthCheck(t *testing.T) {
	env := SetupCoordinatorTestEnv(t)
	defer env.Cleanup()

	req := httptest.NewRequest("GET", "/api/v1/health", nil)
	w := httptest.NewRecorder()
	env.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %v, want %v", w.Code, http.StatusOK)
	}
}

func TestCoordinatorAPI_RegionManagement(t *testing.T) {
	env := SetupCoordinatorTestEnv(t)
	defer env.Cleanup()

	ctx := context.Background()

	// Register a region
	region := &registry.RegionInfo{
		ID:       "region-test-1",
		Name:     "Test Region",
		Endpoint: "localhost:9090",
	}
	err := env.Registry.Register(ctx, region)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// List regions
	req := httptest.NewRequest("GET", "/api/v1/regions", nil)
	w := httptest.NewRecorder()
	env.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %v, want %v", w.Code, http.StatusOK)
	}

	// Get specific region
	req = httptest.NewRequest("GET", "/api/v1/regions/region-test-1", nil)
	w = httptest.NewRecorder()
	env.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %v, want %v", w.Code, http.StatusOK)
	}

	// Get non-existent region
	req = httptest.NewRequest("GET", "/api/v1/regions/non-existent", nil)
	w = httptest.NewRecorder()
	env.Router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %v, want %v", w.Code, http.StatusNotFound)
	}
}

func TestRegistry_HealthCheck(t *testing.T) {
	ctx := context.Background()
	reg := registry.NewRegistry()

	// Register multiple regions
	for i := 0; i < 3; i++ {
		region := &registry.RegionInfo{
			ID:       string(rune('a'+i)) + "-region",
			Name:     "Region " + string(rune('A'+i)),
			Endpoint: "localhost:909" + string(rune('0'+i)),
		}
		reg.Register(ctx, region)
	}

	// Get active regions
	active, err := reg.GetActiveRegions(ctx)
	if err != nil {
		t.Fatalf("GetActiveRegions failed: %v", err)
	}

	if len(active) != 3 {
		t.Errorf("active regions = %v, want 3", len(active))
	}

	// Get healthy regions
	healthy, err := reg.GetHealthyRegions(ctx)
	if err != nil {
		t.Fatalf("GetHealthyRegions failed: %v", err)
	}

	if len(healthy) != 3 {
		t.Errorf("healthy regions = %v, want 3", len(healthy))
	}

	// Deregister one
	reg.Deregister(ctx, "a-region")

	active, _ = reg.GetActiveRegions(ctx)
	if len(active) != 2 {
		t.Errorf("active regions after deregister = %v, want 2", len(active))
	}
}

func TestSyncEngine_HandleChange(t *testing.T) {
	metaManager, _ := metadata.NewEtcdManager(metadata.ManagerConfig{})
	defer metaManager.Close()

	engine := coordsync.NewEngine(coordsync.EngineConfig{
		DefaultStrategy: "eager",
		BatchSize:       100,
	}, metaManager)

	ctx := context.Background()

	// Register regions
	engine.RegisterRegion("region-a")
	engine.RegisterRegion("region-b")

	// Handle a change
	event := &coordsync.ChangeEvent{
		ID:       "event-1",
		Type:     "CREATE",
		FileID:   "file-1",
		RegionID: "region-a",
		Metadata: &metadata.GlobalFileMetadata{},
	}
	event.Metadata.ID = "file-1"
	event.Metadata.Path = "/test.txt"

	err := engine.HandleChange(ctx, event)
	if err != nil {
		t.Fatalf("HandleChange failed: %v", err)
	}

	// Check pending changes for region-b
	pending, _ := engine.GetPendingChanges(ctx, "region-b")
	if len(pending) != 1 {
		t.Errorf("pending changes = %v, want 1", len(pending))
	}

	// region-a should have no pending (it's the source)
	pendingA, _ := engine.GetPendingChanges(ctx, "region-a")
	if len(pendingA) != 0 {
		t.Errorf("pending for source region = %v, want 0", len(pendingA))
	}
}
