// Package config provides configuration management for the JzSE system.
package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config is the root configuration structure.
type Config struct {
	Server      ServerConfig      `mapstructure:"server"`
	Region      RegionConfig      `mapstructure:"region"`
	Coordinator CoordinatorConfig `mapstructure:"coordinator"`
	Storage     StorageConfig     `mapstructure:"storage"`
	Metadata    MetadataConfig    `mapstructure:"metadata"`
	Sync        SyncConfig        `mapstructure:"sync"`
	Logger      LoggerConfig      `mapstructure:"logger"`
}

// ServerConfig holds HTTP/gRPC server configuration.
type ServerConfig struct {
	HTTPAddr     string        `mapstructure:"http_addr"`
	GRPCAddr     string        `mapstructure:"grpc_addr"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
}

// RegionConfig holds region-specific configuration.
type RegionConfig struct {
	ID       string `mapstructure:"id"`
	Name     string `mapstructure:"name"`
	Location string `mapstructure:"location"`
}

// CoordinatorConfig holds coordinator connection configuration.
type CoordinatorConfig struct {
	URL         string        `mapstructure:"url"`
	Endpoints   []string      `mapstructure:"endpoints"`
	DialTimeout time.Duration `mapstructure:"dial_timeout"`
}

// StorageConfig holds storage backend configuration.
type StorageConfig struct {
	Backend  string      `mapstructure:"backend"` // local_fs, minio, s3
	Path     string      `mapstructure:"path"`
	TempPath string      `mapstructure:"temp_path"`
	MinIO    MinIOConfig `mapstructure:"minio"`
}

// MinIOConfig holds MinIO backend configuration.
type MinIOConfig struct {
	Endpoint  string `mapstructure:"endpoint"`
	AccessKey string `mapstructure:"access_key"`
	SecretKey string `mapstructure:"secret_key"`
	Bucket    string `mapstructure:"bucket"`
	Region    string `mapstructure:"region"`
	UseSSL    bool   `mapstructure:"use_ssl"`
	Prefix    string `mapstructure:"prefix"`
}

// MetadataConfig holds metadata storage configuration.
type MetadataConfig struct {
	DBPath    string `mapstructure:"db_path"`
	CacheSize string `mapstructure:"cache_size"`
}

// SyncConfig holds sync agent configuration.
type SyncConfig struct {
	Mode          string        `mapstructure:"mode"` // push, batch, pull
	BatchSize     int           `mapstructure:"batch_size"`
	BatchInterval time.Duration `mapstructure:"batch_interval"`
	RetryInterval time.Duration `mapstructure:"retry_interval"`
	MaxRetries    int           `mapstructure:"max_retries"`
}

// LoggerConfig holds logger configuration.
type LoggerConfig struct {
	Level       string `mapstructure:"level"`
	Format      string `mapstructure:"format"`
	Output      string `mapstructure:"output"`
	Development bool   `mapstructure:"development"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			HTTPAddr:     ":8080",
			GRPCAddr:     ":9090",
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
		},
		Region: RegionConfig{
			ID:       "region-default",
			Name:     "Default Region",
			Location: "unknown",
		},
		Storage: StorageConfig{
			Backend:  "local_fs",
			Path:     "./data/storage",
			TempPath: "./data/temp",
			MinIO: MinIOConfig{
				Endpoint:  "localhost:9000",
				AccessKey: "",
				SecretKey: "",
				Bucket:    "jzse",
				Region:    "us-east-1",
				UseSSL:    false,
				Prefix:    "",
			},
		},
		Metadata: MetadataConfig{
			DBPath:    "./data/metadata",
			CacheSize: "256MB",
		},
		Sync: SyncConfig{
			Mode:          "push",
			BatchSize:     100,
			BatchInterval: 5 * time.Second,
			RetryInterval: 30 * time.Second,
			MaxRetries:    10,
		},
		Logger: LoggerConfig{
			Level:       "info",
			Format:      "json",
			Output:      "stdout",
			Development: false,
		},
	}
}

// Load loads configuration from file and environment variables.
func Load(configPath string) (*Config, error) {
	v := viper.New()

	// Set defaults
	setDefaults(v)

	// Configure Viper
	v.SetConfigType("yaml")
	v.SetEnvPrefix("JZSE")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Load config file if specified
	if configPath != "" {
		v.SetConfigFile(configPath)
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	// Unmarshal configuration
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &cfg, nil
}

// setDefaults sets default values in Viper.
func setDefaults(v *viper.Viper) {
	defaults := DefaultConfig()

	// Server defaults
	v.SetDefault("server.http_addr", defaults.Server.HTTPAddr)
	v.SetDefault("server.grpc_addr", defaults.Server.GRPCAddr)
	v.SetDefault("server.read_timeout", defaults.Server.ReadTimeout)
	v.SetDefault("server.write_timeout", defaults.Server.WriteTimeout)

	// Region defaults
	v.SetDefault("region.id", defaults.Region.ID)
	v.SetDefault("region.name", defaults.Region.Name)
	v.SetDefault("region.location", defaults.Region.Location)

	// Storage defaults
	v.SetDefault("storage.backend", defaults.Storage.Backend)
	v.SetDefault("storage.path", defaults.Storage.Path)
	v.SetDefault("storage.temp_path", defaults.Storage.TempPath)
	v.SetDefault("storage.minio.endpoint", defaults.Storage.MinIO.Endpoint)
	v.SetDefault("storage.minio.access_key", defaults.Storage.MinIO.AccessKey)
	v.SetDefault("storage.minio.secret_key", defaults.Storage.MinIO.SecretKey)
	v.SetDefault("storage.minio.bucket", defaults.Storage.MinIO.Bucket)
	v.SetDefault("storage.minio.region", defaults.Storage.MinIO.Region)
	v.SetDefault("storage.minio.use_ssl", defaults.Storage.MinIO.UseSSL)
	v.SetDefault("storage.minio.prefix", defaults.Storage.MinIO.Prefix)

	// Metadata defaults
	v.SetDefault("metadata.db_path", defaults.Metadata.DBPath)
	v.SetDefault("metadata.cache_size", defaults.Metadata.CacheSize)

	// Sync defaults
	v.SetDefault("sync.mode", defaults.Sync.Mode)
	v.SetDefault("sync.batch_size", defaults.Sync.BatchSize)
	v.SetDefault("sync.batch_interval", defaults.Sync.BatchInterval)
	v.SetDefault("sync.retry_interval", defaults.Sync.RetryInterval)
	v.SetDefault("sync.max_retries", defaults.Sync.MaxRetries)

	// Logger defaults
	v.SetDefault("logger.level", defaults.Logger.Level)
	v.SetDefault("logger.format", defaults.Logger.Format)
	v.SetDefault("logger.output", defaults.Logger.Output)
	v.SetDefault("logger.development", defaults.Logger.Development)
}
