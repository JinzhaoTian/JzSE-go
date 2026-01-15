// Package integration provides integration tests for the JzSE system.
package integration

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"

	"asisaid.cn/JzSE/internal/region/metadata"
	"asisaid.cn/JzSE/internal/region/service"
	"asisaid.cn/JzSE/internal/region/storage"
	httpapi "asisaid.cn/JzSE/pkg/api/http"
)

// TestEnv provides a test environment for integration tests.
type TestEnv struct {
	Router   *gin.Engine
	TmpDir   string
	Storage  storage.Backend
	Metadata metadata.Store
	Service  *service.FileService
}

// SetupTestEnv creates a new test environment.
func SetupTestEnv(t *testing.T) *TestEnv {
	t.Helper()

	gin.SetMode(gin.TestMode)

	tmpDir, err := os.MkdirTemp("", "jzse-integration-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	storageBackend, err := storage.NewLocalFSBackend(tmpDir + "/storage")
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to create storage: %v", err)
	}

	metaStore, err := metadata.NewBadgerStore(tmpDir + "/metadata")
	if err != nil {
		storageBackend.Close()
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to create metadata store: %v", err)
	}

	fileService := service.NewFileService("test-region", storageBackend, metaStore)
	handler := httpapi.NewHandler(fileService)

	router := gin.New()
	handler.RegisterRoutes(router)

	return &TestEnv{
		Router:   router,
		TmpDir:   tmpDir,
		Storage:  storageBackend,
		Metadata: metaStore,
		Service:  fileService,
	}
}

// Cleanup cleans up the test environment.
func (e *TestEnv) Cleanup() {
	if e.Metadata != nil {
		e.Metadata.Close()
	}
	if e.Storage != nil {
		e.Storage.Close()
	}
	if e.TmpDir != "" {
		os.RemoveAll(e.TmpDir)
	}
}

func TestRegionAPI_HealthCheck(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	req := httptest.NewRequest("GET", "/api/v1/health", nil)
	w := httptest.NewRecorder()
	env.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %v, want %v", w.Code, http.StatusOK)
	}
}

func TestRegionAPI_FileUploadDownload(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	ctx := context.Background()
	testContent := []byte("integration test content")

	// Upload via service directly
	uploadResp, err := env.Service.Upload(ctx, &service.UploadRequest{
		Path:    "/",
		Name:    "test.txt",
		Size:    int64(len(testContent)),
		Content: bytes.NewReader(testContent),
		OwnerID: "test-user",
	})
	if err != nil {
		t.Fatalf("Upload failed: %v", err)
	}

	if uploadResp.FileID == "" {
		t.Error("FileID should not be empty")
	}

	// Download
	downloadResp, err := env.Service.Download(ctx, uploadResp.FileID)
	if err != nil {
		t.Fatalf("Download failed: %v", err)
	}
	defer downloadResp.Content.Close()

	content, err := io.ReadAll(downloadResp.Content)
	if err != nil {
		t.Fatalf("failed to read content: %v", err)
	}

	if !bytes.Equal(content, testContent) {
		t.Errorf("content mismatch")
	}

	// Get metadata
	meta, err := env.Service.GetMetadata(ctx, uploadResp.FileID)
	if err != nil {
		t.Fatalf("GetMetadata failed: %v", err)
	}

	if meta.Name != "test.txt" {
		t.Errorf("Name = %v, want test.txt", meta.Name)
	}
	if meta.Size != int64(len(testContent)) {
		t.Errorf("Size = %v, want %v", meta.Size, len(testContent))
	}
}

func TestRegionAPI_FileDelete(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	ctx := context.Background()

	// Upload first
	uploadResp, err := env.Service.Upload(ctx, &service.UploadRequest{
		Path:    "/",
		Name:    "todelete.txt",
		Size:    5,
		Content: bytes.NewReader([]byte("hello")),
		OwnerID: "test-user",
	})
	if err != nil {
		t.Fatalf("Upload failed: %v", err)
	}

	// Delete
	err = env.Service.Delete(ctx, uploadResp.FileID)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify metadata shows deleted state
	meta, err := env.Metadata.Get(ctx, uploadResp.FileID)
	if err != nil {
		t.Fatalf("Get metadata failed: %v", err)
	}

	if meta.LocalState != metadata.LocalStateDeleted {
		t.Errorf("LocalState = %v, want deleted", meta.LocalState)
	}
}

func TestRegionAPI_MetadataEndpoint(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	ctx := context.Background()

	// Upload
	uploadResp, _ := env.Service.Upload(ctx, &service.UploadRequest{
		Path:    "/test",
		Name:    "meta.txt",
		Size:    4,
		Content: bytes.NewReader([]byte("test")),
		OwnerID: "user",
	})

	// Test metadata endpoint
	req := httptest.NewRequest("GET", "/api/v1/files/"+uploadResp.FileID+"/metadata", nil)
	w := httptest.NewRecorder()
	env.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %v, want %v", w.Code, http.StatusOK)
	}
}

func TestRegionAPI_NotFound(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	req := httptest.NewRequest("GET", "/api/v1/files/non-existent-id", nil)
	w := httptest.NewRecorder()
	env.Router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %v, want %v", w.Code, http.StatusNotFound)
	}
}
