// Package main provides the entry point for the coordinator service.
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
	"asisaid.cn/JzSE/internal/coordinator/conflict"
	"asisaid.cn/JzSE/internal/coordinator/metadata"
	"asisaid.cn/JzSE/internal/coordinator/registry"
	coordsync "asisaid.cn/JzSE/internal/coordinator/sync"
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
	log.Info("starting coordinator service", zap.String("version", version))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize metadata manager
	metaManager, err := metadata.NewEtcdManager(metadata.ManagerConfig{
		Endpoints:   cfg.Coordinator.Endpoints,
		DialTimeout: cfg.Coordinator.DialTimeout,
	})
	if err != nil {
		log.Fatal("failed to initialize metadata manager", zap.Error(err))
	}
	defer metaManager.Close()

	// Initialize region registry
	regionRegistry := registry.NewRegistry()
	if err := regionRegistry.Start(ctx); err != nil {
		log.Fatal("failed to start region registry", zap.Error(err))
	}
	defer regionRegistry.Stop()

	// Initialize sync engine
	syncEngine := coordsync.NewEngine(coordsync.EngineConfig{
		DefaultStrategy:  "lazy",
		BatchSize:        100,
		BroadcastTimeout: 10 * time.Second,
	}, metaManager)
	if err := syncEngine.Start(ctx); err != nil {
		log.Fatal("failed to start sync engine", zap.Error(err))
	}
	defer syncEngine.Stop()

	// Initialize conflict resolver
	conflictResolver := conflict.NewResolver(conflict.StrategyLWW)
	_ = conflictResolver // Will be used in API handlers

	// Setup Gin
	if !cfg.Logger.Development {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(ginLogger())

	// Register API routes
	registerRoutes(router, metaManager, regionRegistry, syncEngine)

	// Create HTTP server
	server := &http.Server{
		Addr:         cfg.Server.HTTPAddr,
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	// Start server
	go func() {
		log.Info("HTTP server starting", zap.String("addr", cfg.Server.HTTPAddr))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("failed to start HTTP server", zap.Error(err))
		}
	}()

	// Wait for interrupt
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutting down server...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error("server forced to shutdown", zap.Error(err))
	}

	log.Info("server exited")
}

// registerRoutes registers all coordinator API routes.
func registerRoutes(r *gin.Engine, metaManager metadata.Manager, reg *registry.Registry, sync *coordsync.Engine) {
	api := r.Group("/api/v1")
	{
		// Health check
		api.GET("/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "healthy"})
		})

		// Metadata operations
		api.GET("/metadata/:id", func(c *gin.Context) {
			fileID := c.Param("id")
			meta, err := metaManager.Get(c.Request.Context(), fileID)
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, meta)
		})

		// Region management
		regions := api.Group("/regions")
		{
			regions.GET("", func(c *gin.Context) {
				list, _ := reg.GetActiveRegions(c.Request.Context())
				c.JSON(http.StatusOK, list)
			})

			regions.GET("/:id", func(c *gin.Context) {
				regionID := c.Param("id")
				info, err := reg.GetRegion(c.Request.Context(), regionID)
				if err != nil {
					c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
					return
				}
				c.JSON(http.StatusOK, info)
			})

			regions.POST("/:id/heartbeat", func(c *gin.Context) {
				regionID := c.Param("id")
				var status registry.RegionStatus
				if err := c.ShouldBindJSON(&status); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
					return
				}
				if err := reg.Heartbeat(c.Request.Context(), regionID, &status); err != nil {
					c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
					return
				}
				c.Status(http.StatusNoContent)
			})
		}

		// Sync operations
		api.GET("/sync/pending/:region_id", func(c *gin.Context) {
			regionID := c.Param("region_id")
			events, _ := sync.GetPendingChanges(c.Request.Context(), regionID)
			c.JSON(http.StatusOK, events)
		})
	}
}

// ginLogger returns a Gin middleware for request logging.
func ginLogger() gin.HandlerFunc {
	log := logger.WithComponent("http")

	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path

		c.Next()

		log.Info("request",
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("latency", time.Since(start)),
		)
	}
}
