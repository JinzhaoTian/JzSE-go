package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"asisaid.cn/JzSE/internal/common/errors"
	"asisaid.cn/JzSE/internal/coordinator/metadata"
	"asisaid.cn/JzSE/internal/coordinator/registry"
	coordsync "asisaid.cn/JzSE/internal/coordinator/sync"
	regionmeta "asisaid.cn/JzSE/internal/region/metadata"
	"asisaid.cn/JzSE/internal/region/service"
	"asisaid.cn/JzSE/internal/region/storage"
	regionsync "asisaid.cn/JzSE/internal/region/sync"
)

// SyncTestEnv provides a full environment with both region and coordinator.
type SyncTestEnv struct {
	// Coordinator side
	CoordRouter  *gin.Engine
	CoordServer  *httptest.Server
	MetaManager  metadata.Manager
	Registry     *registry.Registry
	SyncEngine   *coordsync.Engine

	// Region side
	RegionService *service.FileService
	SyncAgent     *regionsync.Agent
	RegionMeta    regionmeta.Store
	RegionStorage storage.Backend
	TmpDir        string
}

func SetupSyncTestEnv(t *testing.T) *SyncTestEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)

	// Setup coordinator
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

	coordRouter := gin.New()
	registerTestRoutes(coordRouter, metaManager, reg, syncEngine)
	coordServer := httptest.NewServer(coordRouter)

	// Setup region
	tmpDir, err := os.MkdirTemp("", "jzse-sync-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	regionStorage, err := storage.NewLocalFSBackend(tmpDir + "/storage")
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to create storage: %v", err)
	}

	regionMeta, err := regionmeta.NewBadgerStore(tmpDir + "/metadata")
	if err != nil {
		regionStorage.Close()
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to create metadata store: %v", err)
	}

	regionService := service.NewFileService("region-test", regionStorage, regionMeta)

	syncAgent := regionsync.NewAgent(regionsync.AgentConfig{
		RegionID:       "region-test",
		CoordinatorURL: coordServer.URL,
		Mode:           "push",
		BatchSize:      100,
		BatchInterval:  1 * time.Second,
		RetryInterval:  1 * time.Second,
		MaxRetries:     3,
	}, regionMeta)

	regionService.SetSyncAgent(syncAgent)

	return &SyncTestEnv{
		CoordRouter:   coordRouter,
		CoordServer:   coordServer,
		MetaManager:   metaManager,
		Registry:      reg,
		SyncEngine:    syncEngine,
		RegionService: regionService,
		SyncAgent:     syncAgent,
		RegionMeta:    regionMeta,
		RegionStorage: regionStorage,
		TmpDir:        tmpDir,
	}
}

func (e *SyncTestEnv) Cleanup() {
	e.CoordServer.Close()
	if e.MetaManager != nil {
		e.MetaManager.Close()
	}
	if e.RegionMeta != nil {
		e.RegionMeta.Close()
	}
	if e.RegionStorage != nil {
		e.RegionStorage.Close()
	}
	if e.TmpDir != "" {
		os.RemoveAll(e.TmpDir)
	}
}

func TestSyncPipeline_UploadPushesToCoordinator(t *testing.T) {
	env := SetupSyncTestEnv(t)
	defer env.Cleanup()

	ctx := context.Background()

	// Register the region with coordinator so it can receive events
	env.SyncEngine.RegisterRegion("region-test")

	// Start sync agent
	if err := env.SyncAgent.Start(ctx); err != nil {
		t.Fatalf("failed to start sync agent: %v", err)
	}
	defer env.SyncAgent.Stop()

	// Upload a file
	testContent := []byte("hello sync world")
	uploadResp, err := env.RegionService.Upload(ctx, &service.UploadRequest{
		Path:    "/",
		Name:    "sync-test.txt",
		Size:    int64(len(testContent)),
		Content: bytes.NewReader(testContent),
		OwnerID: "test-user",
	})
	if err != nil {
		t.Fatalf("Upload failed: %v", err)
	}

	// Wait for the sync agent to push the event
	time.Sleep(500 * time.Millisecond)

	// Verify coordinator has the metadata
	globalMeta, err := env.MetaManager.Get(ctx, uploadResp.FileID)
	if err != nil {
		t.Fatalf("coordinator should have metadata: %v", err)
	}

	if globalMeta.Name != "sync-test.txt" {
		t.Errorf("Name = %v, want sync-test.txt", globalMeta.Name)
	}
	if globalMeta.Size != int64(len(testContent)) {
		t.Errorf("Size = %v, want %v", globalMeta.Size, len(testContent))
	}
}

func TestSyncPipeline_DeletePushesToCoordinator(t *testing.T) {
	env := SetupSyncTestEnv(t)
	defer env.Cleanup()

	ctx := context.Background()
	env.SyncEngine.RegisterRegion("region-test")

	if err := env.SyncAgent.Start(ctx); err != nil {
		t.Fatalf("failed to start sync agent: %v", err)
	}
	defer env.SyncAgent.Stop()

	// Upload a file first
	testContent := []byte("to be deleted")
	uploadResp, err := env.RegionService.Upload(ctx, &service.UploadRequest{
		Path:    "/",
		Name:    "delete-me.txt",
		Size:    int64(len(testContent)),
		Content: bytes.NewReader(testContent),
		OwnerID: "test-user",
	})
	if err != nil {
		t.Fatalf("Upload failed: %v", err)
	}

	// Wait for create to sync
	time.Sleep(500 * time.Millisecond)

	// Delete the file
	if err := env.RegionService.Delete(ctx, uploadResp.FileID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Wait for delete to sync
	time.Sleep(500 * time.Millisecond)

	// Verify coordinator no longer has the metadata
	_, err = env.MetaManager.Get(ctx, uploadResp.FileID)
	if !errors.IsNotFound(err) {
		t.Errorf("expected not found after delete, got: %v", err)
	}
}

func TestSyncPipeline_PullChangesFromCoordinator(t *testing.T) {
	env := SetupSyncTestEnv(t)
	defer env.Cleanup()

	ctx := context.Background()

	// Register two regions
	env.SyncEngine.RegisterRegion("region-test")
	env.SyncEngine.RegisterRegion("region-other")

	// Simulate a change from "region-other" via coordinator API
	event := coordsync.ChangeEvent{
		ID:       "remote-evt-1",
		Type:     "CREATE",
		FileID:   "remote-file-1",
		RegionID: "region-other",
		Metadata: &metadata.GlobalFileMetadata{},
		VectorClock: map[string]uint64{
			"region-other": 1,
		},
		Timestamp: time.Now(),
	}
	event.Metadata.ID = "remote-file-1"
	event.Metadata.Name = "remote.txt"
	event.Metadata.Path = "/remote.txt"
	event.Metadata.Size = 42

	body, _ := json.Marshal(event)
	req := httptest.NewRequest("POST", "/api/v1/sync/changes", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	env.CoordRouter.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST /sync/changes = %v, want %v, body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	// Start the sync agent (it will pull pending changes)
	if err := env.SyncAgent.Start(ctx); err != nil {
		t.Fatalf("failed to start sync agent: %v", err)
	}
	defer env.SyncAgent.Stop()

	// Wait for pull cycle
	time.Sleep(2 * time.Second)

	// Verify local metadata store has the remote file
	localMeta, err := env.RegionMeta.Get(ctx, "remote-file-1")
	if err != nil {
		t.Fatalf("expected local metadata for remote file: %v", err)
	}

	if localMeta.Name != "remote.txt" {
		t.Errorf("Name = %v, want remote.txt", localMeta.Name)
	}
	if localMeta.LocalState != regionmeta.LocalStatePending {
		t.Errorf("LocalState = %v, want pending (no local file data)", localMeta.LocalState)
	}
	if localMeta.SyncState != regionmeta.SyncStateSynced {
		t.Errorf("SyncState = %v, want synced", localMeta.SyncState)
	}
}

func TestSyncPipeline_CrossRegionBroadcast(t *testing.T) {
	env := SetupSyncTestEnv(t)
	defer env.Cleanup()

	ctx := context.Background()

	// Register two regions with the sync engine
	env.SyncEngine.RegisterRegion("region-a")
	env.SyncEngine.RegisterRegion("region-b")

	// Create a change from region-a
	event := &coordsync.ChangeEvent{
		ID:       "cross-evt-1",
		Type:     "CREATE",
		FileID:   "cross-file-1",
		RegionID: "region-a",
		Metadata: &metadata.GlobalFileMetadata{},
	}
	event.Metadata.ID = "cross-file-1"
	event.Metadata.Path = "/cross.txt"
	event.Metadata.Name = "cross.txt"

	err := env.SyncEngine.HandleChange(ctx, event)
	if err != nil {
		t.Fatalf("HandleChange failed: %v", err)
	}

	// region-b should have the pending event
	pendingB, _ := env.SyncEngine.GetPendingChanges(ctx, "region-b")
	if len(pendingB) != 1 {
		t.Errorf("region-b pending = %v, want 1", len(pendingB))
	}

	// region-a (source) should have no pending
	pendingA, _ := env.SyncEngine.GetPendingChanges(ctx, "region-a")
	if len(pendingA) != 0 {
		t.Errorf("region-a pending = %v, want 0", len(pendingA))
	}
}
