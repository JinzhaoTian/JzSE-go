// Package main provides the entry point for the region service.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"asisaid.cn/JzSE/internal/common/config"
	"asisaid.cn/JzSE/internal/common/logger"
	"asisaid.cn/JzSE/internal/region/metadata"
	"asisaid.cn/JzSE/internal/region/service"
	"asisaid.cn/JzSE/internal/region/storage"
	regionsync "asisaid.cn/JzSE/internal/region/sync"
	httpapi "asisaid.cn/JzSE/pkg/api/http"
	"go.uber.org/zap"
)

var (
	configPath = flag.String("config", "", "path to config file")
	version    = "dev"
)

func main() {
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	logCfg := logger.Config{
		Level:       cfg.Logger.Level,
		Format:      cfg.Logger.Format,
		Output:      cfg.Logger.Output,
		Development: cfg.Logger.Development,
	}
	if err := logger.Init(logCfg); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	log := logger.WithComponent("main")
	log.Info("starting region service",
		zap.String("version", version),
		zap.String("region_id", cfg.Region.ID),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize storage backend
	storageBackend, err := storage.NewBackend(cfg.Storage.Backend, storage.BackendOptions{
		BasePath: cfg.Storage.Path,
		MinIO: storage.MinIOOptions{
			Endpoint:  cfg.Storage.MinIO.Endpoint,
			AccessKey: cfg.Storage.MinIO.AccessKey,
			SecretKey: cfg.Storage.MinIO.SecretKey,
			Bucket:    cfg.Storage.MinIO.Bucket,
			Region:    cfg.Storage.MinIO.Region,
			UseSSL:    cfg.Storage.MinIO.UseSSL,
			Prefix:    cfg.Storage.MinIO.Prefix,
		},
		S3: storage.S3Options{
			Endpoint:  cfg.Storage.S3.Endpoint,
			AccessKey: cfg.Storage.S3.AccessKey,
			SecretKey: cfg.Storage.S3.SecretKey,
			Bucket:    cfg.Storage.S3.Bucket,
			Region:    cfg.Storage.S3.Region,
			UseSSL:    cfg.Storage.S3.UseSSL,
			Prefix:    cfg.Storage.S3.Prefix,
		},
	})
	if err != nil {
		log.Fatal("failed to initialize storage", zap.Error(err))
	}
	defer storageBackend.Close()

	// Initialize metadata store
	metaStore, err := metadata.NewBadgerStore(cfg.Metadata.DBPath)
	if err != nil {
		log.Fatal("failed to initialize metadata store", zap.Error(err))
	}
	defer metaStore.Close()

	// Create file service
	fileService := service.NewFileService(cfg.Region.ID, storageBackend, metaStore)

	// Initialize and start sync agent
	syncAgent := regionsync.NewAgent(regionsync.AgentConfig{
		RegionID:       cfg.Region.ID,
		CoordinatorURL: cfg.Coordinator.URL,
		Mode:           cfg.Sync.Mode,
		BatchSize:      cfg.Sync.BatchSize,
		BatchInterval:  cfg.Sync.BatchInterval,
		RetryInterval:  cfg.Sync.RetryInterval,
		MaxRetries:     cfg.Sync.MaxRetries,
	}, metaStore)

	fileService.SetSyncAgent(syncAgent)

	if err := syncAgent.Start(ctx); err != nil {
		log.Fatal("failed to start sync agent", zap.Error(err))
	}
	defer syncAgent.Stop()

	// Self-register with coordinator (non-blocking)
	if cfg.Coordinator.URL != "" {
		go registerWithCoordinator(cfg, log)
	}

	// Create HTTP handler
	handler := httpapi.NewHandler(fileService)

	// Setup Gin
	if !cfg.Logger.Development {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(ginLogger())

	// Register routes
	handler.RegisterRoutes(router)

	// Create HTTP server
	server := &http.Server{
		Addr:         cfg.Server.HTTPAddr,
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	// Start server in goroutine
	go func() {
		log.Info("HTTP server starting", zap.String("addr", cfg.Server.HTTPAddr))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("failed to start HTTP server", zap.Error(err))
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutting down server...")

	// Graceful shutdown with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error("server forced to shutdown", zap.Error(err))
	}

	log.Info("server exited")
}

// registerWithCoordinator attempts to register this region with the coordinator.
func registerWithCoordinator(cfg *config.Config, log *zap.Logger) {
	regionInfo := map[string]interface{}{
		"id":       cfg.Region.ID,
		"name":     cfg.Region.Name,
		"endpoint": "http://localhost" + cfg.Server.HTTPAddr,
		"location": map[string]interface{}{
			"city": cfg.Region.Location,
		},
		"status": map[string]interface{}{
			"state": "healthy",
		},
	}

	body, err := json.Marshal(regionInfo)
	if err != nil {
		log.Warn("failed to marshal region info for registration", zap.Error(err))
		return
	}

	url := cfg.Coordinator.URL + "/api/v1/regions"
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		log.Warn("failed to register with coordinator (coordinator may not be running yet)",
			zap.Error(err),
		)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		log.Info("successfully registered with coordinator",
			zap.String("coordinator_url", cfg.Coordinator.URL),
		)
	} else {
		log.Warn("coordinator returned non-success status for registration",
			zap.Int("status", resp.StatusCode),
		)
	}
}

// ginLogger returns a Gin middleware that logs requests using zap.
func ginLogger() gin.HandlerFunc {
	log := logger.WithComponent("http")

	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()

		log.Info("request",
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.Int("status", status),
			zap.Duration("latency", latency),
			zap.String("client_ip", c.ClientIP()),
		)
	}
}
