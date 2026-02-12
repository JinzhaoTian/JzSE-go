package config

import (
	"strings"
	"testing"
)

func TestLoad_UsesJzSEPrefix(t *testing.T) {
	t.Setenv("JzSE_SERVER_HTTP_ADDR", ":18080")
	t.Setenv("JzSE_COORDINATOR_URL", "http://127.0.0.1:9001")
	t.Setenv("JzSE_STORAGE_MINIO_BUCKET", "bucket-from-env")
	t.Setenv("JzSE_LOGGER_LEVEL", "debug")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Server.HTTPAddr != ":18080" {
		t.Fatalf("Server.HTTPAddr = %q, want %q", cfg.Server.HTTPAddr, ":18080")
	}
	if cfg.Coordinator.URL != "http://127.0.0.1:9001" {
		t.Fatalf("Coordinator.URL = %q, want %q", cfg.Coordinator.URL, "http://127.0.0.1:9001")
	}
	if cfg.Storage.MinIO.Bucket != "bucket-from-env" {
		t.Fatalf("Storage.MinIO.Bucket = %q, want %q", cfg.Storage.MinIO.Bucket, "bucket-from-env")
	}
	if cfg.Logger.Level != "debug" {
		t.Fatalf("Logger.Level = %q, want %q", cfg.Logger.Level, "debug")
	}
}

func TestLoad_IgnoresLegacyUppercasePrefix(t *testing.T) {
	t.Setenv(strings.ToUpper(envPrefix)+"_LOGGER_LEVEL", "debug")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Logger.Level != "info" {
		t.Fatalf("Logger.Level = %q, want %q", cfg.Logger.Level, "info")
	}
}
