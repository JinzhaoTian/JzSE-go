// Package integration provides integration tests for the JzSE system.
package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"asisaid.cn/JzSE/internal/common/errors"
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

func registerTestRoutes(r *gin.Engine, metaManager metadata.Manager, reg *registry.Registry, syncEngine *coordsync.Engine) {
	api := r.Group("/api/v1")
	{
		api.GET("/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "healthy"})
		})

		api.POST("/regions", func(c *gin.Context) {
			var info registry.RegionInfo
			if err := c.ShouldBindJSON(&info); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			if err := reg.Register(c.Request.Context(), &info); err != nil {
				c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
				return
			}
			syncEngine.RegisterRegion(info.ID)
			c.JSON(http.StatusCreated, info)
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

		api.POST("/sync/changes", func(c *gin.Context) {
			var event coordsync.ChangeEvent
			if err := c.ShouldBindJSON(&event); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			if err := syncEngine.HandleChange(c.Request.Context(), &event); err != nil {
				if errors.IsConflict(err) {
					c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
					return
				}
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"status": "accepted"})
		})

		api.GET("/sync/pending/:region_id", func(c *gin.Context) {
			regionID := c.Param("region_id")
			events, _ := syncEngine.GetPendingChanges(c.Request.Context(), regionID)
			c.JSON(http.StatusOK, events)
		})

		api.GET("/metadata/:id", func(c *gin.Context) {
			fileID := c.Param("id")
			meta, err := metaManager.Get(c.Request.Context(), fileID)
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, meta)
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
		DefaultStrategy: "lazy",
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

func TestCoordinatorAPI_RegisterRegion(t *testing.T) {
	env := SetupCoordinatorTestEnv(t)
	defer env.Cleanup()

	regionInfo := registry.RegionInfo{
		ID:       "region-new",
		Name:     "New Region",
		Endpoint: "http://localhost:8080",
	}
	body, _ := json.Marshal(regionInfo)

	req := httptest.NewRequest("POST", "/api/v1/regions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	env.Router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("status = %v, want %v, body: %s", w.Code, http.StatusCreated, w.Body.String())
	}

	// Verify region is registered in the registry
	ctx := context.Background()
	info, err := env.Registry.GetRegion(ctx, "region-new")
	if err != nil {
		t.Fatalf("GetRegion failed: %v", err)
	}
	if info.Name != "New Region" {
		t.Errorf("Name = %v, want New Region", info.Name)
	}

	// Duplicate registration should fail
	req = httptest.NewRequest("POST", "/api/v1/regions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	env.Router.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("duplicate register status = %v, want %v", w.Code, http.StatusConflict)
	}
}

func TestCoordinatorAPI_SyncChanges(t *testing.T) {
	env := SetupCoordinatorTestEnv(t)
	defer env.Cleanup()

	// Register two regions first
	env.SyncEngine.RegisterRegion("region-a")
	env.SyncEngine.RegisterRegion("region-b")

	// Post a change event from region-a
	event := coordsync.ChangeEvent{
		ID:       "evt-1",
		Type:     "CREATE",
		FileID:   "file-100",
		RegionID: "region-a",
		Metadata: &metadata.GlobalFileMetadata{},
	}
	event.Metadata.ID = "file-100"
	event.Metadata.Path = "/sync-test.txt"
	event.Metadata.Name = "sync-test.txt"

	body, _ := json.Marshal(event)
	req := httptest.NewRequest("POST", "/api/v1/sync/changes", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	env.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST /sync/changes status = %v, want %v, body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	// Verify metadata was stored
	req = httptest.NewRequest("GET", "/api/v1/metadata/file-100", nil)
	w = httptest.NewRecorder()
	env.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET /metadata status = %v, want %v", w.Code, http.StatusOK)
	}

	// Verify region-b has pending changes
	req = httptest.NewRequest("GET", "/api/v1/sync/pending/region-b", nil)
	w = httptest.NewRecorder()
	env.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /sync/pending status = %v, want %v", w.Code, http.StatusOK)
	}

	var pending []coordsync.ChangeEvent
	if err := json.NewDecoder(w.Body).Decode(&pending); err != nil {
		t.Fatalf("failed to decode pending: %v", err)
	}
	if len(pending) != 1 {
		t.Errorf("pending count = %v, want 1", len(pending))
	}

	// Verify region-a (source) has no pending
	req = httptest.NewRequest("GET", "/api/v1/sync/pending/region-a", nil)
	w = httptest.NewRecorder()
	env.Router.ServeHTTP(w, req)

	var pendingA []coordsync.ChangeEvent
	json.NewDecoder(w.Body).Decode(&pendingA)
	if len(pendingA) != 0 {
		t.Errorf("source region pending count = %v, want 0", len(pendingA))
	}
}
