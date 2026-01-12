// Package http provides HTTP API handlers.
package http

import (
	"io"
	"net/http"
	"path/filepath"

	"github.com/gin-gonic/gin"

	"asisaid.cn/JzSE/internal/region/service"
)

// Handler provides HTTP handlers for the region API.
type Handler struct {
	fileService *service.FileService
}

// NewHandler creates a new Handler.
func NewHandler(fileService *service.FileService) *Handler {
	return &Handler{
		fileService: fileService,
	}
}

// RegisterRoutes registers all API routes.
func (h *Handler) RegisterRoutes(r *gin.Engine) {
	api := r.Group("/api/v1")
	{
		// File operations
		api.POST("/files", h.UploadFile)
		api.GET("/files/:id", h.DownloadFile)
		api.DELETE("/files/:id", h.DeleteFile)
		api.GET("/files/:id/metadata", h.GetFileMetadata)

		// Directory operations
		api.GET("/directories/*path", h.ListDirectory)

		// Health check
		api.GET("/health", h.HealthCheck)

		// Region status
		api.GET("/region/status", h.RegionStatus)
	}
}

// UploadFile handles file upload.
// POST /api/v1/files
func (h *Handler) UploadFile(c *gin.Context) {
	// Get file from form
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "no file provided",
		})
		return
	}
	defer file.Close()

	// Get path from form
	path := c.PostForm("path")
	if path == "" {
		path = "/"
	}

	// Get owner ID (from auth in production)
	ownerID := c.GetString("user_id")
	if ownerID == "" {
		ownerID = "anonymous"
	}

	req := &service.UploadRequest{
		Path:     path,
		Name:     header.Filename,
		Size:     header.Size,
		Content:  file,
		MimeType: header.Header.Get("Content-Type"),
		OwnerID:  ownerID,
	}

	resp, err := h.fileService.Upload(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, resp)
}

// DownloadFile handles file download.
// GET /api/v1/files/:id
func (h *Handler) DownloadFile(c *gin.Context) {
	fileID := c.Param("id")

	resp, err := h.fileService.Download(c.Request.Context(), fileID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": err.Error(),
		})
		return
	}
	defer resp.Content.Close()

	// Set headers
	c.Header("Content-Type", resp.Metadata.MimeType)
	c.Header("Content-Disposition", "attachment; filename="+resp.Metadata.Name)
	c.Header("X-File-ID", resp.Metadata.ID)
	c.Header("X-Content-Hash", resp.Metadata.ContentHash)

	// Stream file content
	c.Status(http.StatusOK)
	io.Copy(c.Writer, resp.Content)
}

// DeleteFile handles file deletion.
// DELETE /api/v1/files/:id
func (h *Handler) DeleteFile(c *gin.Context) {
	fileID := c.Param("id")

	if err := h.fileService.Delete(c.Request.Context(), fileID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.Status(http.StatusNoContent)
}

// GetFileMetadata retrieves file metadata.
// GET /api/v1/files/:id/metadata
func (h *Handler) GetFileMetadata(c *gin.Context) {
	fileID := c.Param("id")

	meta, err := h.fileService.GetMetadata(c.Request.Context(), fileID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, meta)
}

// ListDirectory lists files in a directory.
// GET /api/v1/directories/*path
func (h *Handler) ListDirectory(c *gin.Context) {
	path := c.Param("path")
	if path == "" {
		path = "/"
	}
	path = filepath.Clean(path)

	entries, err := h.fileService.ListDirectory(c.Request.Context(), path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"path":    path,
		"entries": entries,
	})
}

// HealthCheck handles health check requests.
// GET /api/v1/health
func (h *Handler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "healthy",
	})
}

// RegionStatus returns the region status.
// GET /api/v1/region/status
func (h *Handler) RegionStatus(c *gin.Context) {
	// TODO: Implement proper status checking
	c.JSON(http.StatusOK, gin.H{
		"status":     "healthy",
		"sync_state": "connected",
	})
}
