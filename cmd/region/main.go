// Package main provides the entry point for the region service.
package main

import (
	"context"
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

	// Initialize storage backend
	storageBackend, err := storage.NewBackend(cfg.Storage.Backend, cfg.Storage.Path)
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
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Error("server forced to shutdown", zap.Error(err))
	}

	log.Info("server exited")
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
