// Package config provides configuration management for the JzSE system.
package config

import (
	"fmt"
	"reflect"
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

const envPrefix = "JzSE"

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
	Backend  string       `mapstructure:"backend"` // local_fs, minio, s3, rustfs
	Path     string       `mapstructure:"path"`
	TempPath string       `mapstructure:"temp_path"`
	MinIO    MinIOConfig  `mapstructure:"minio"`
	S3       S3Config     `mapstructure:"s3"`
	RustFS   RustFSConfig `mapstructure:"rustfs"`
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

// S3Config holds S3 backend configuration.
type S3Config struct {
	Endpoint  string `mapstructure:"endpoint"`
	AccessKey string `mapstructure:"access_key"`
	SecretKey string `mapstructure:"secret_key"`
	Bucket    string `mapstructure:"bucket"`
	Region    string `mapstructure:"region"`
	UseSSL    bool   `mapstructure:"use_ssl"`
	Prefix    string `mapstructure:"prefix"`
}

// RustFSConfig holds RustFS backend configuration.
type RustFSConfig struct {
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
			S3: S3Config{
				Endpoint:  "s3.amazonaws.com",
				AccessKey: "",
				SecretKey: "",
				Bucket:    "jzse",
				Region:    "us-east-1",
				UseSSL:    true,
				Prefix:    "",
			},
			RustFS: RustFSConfig{
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

	// Bind all known config keys to exact-case environment variables (JzSE_*).
	bindEnvOverrides(v, envPrefix)

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

// bindEnvOverrides binds known configuration keys to exact-case environment variables.
func bindEnvOverrides(v *viper.Viper, prefix string) {
	replacer := strings.NewReplacer(".", "_")
	for _, key := range collectMapstructureKeys(reflect.TypeOf(Config{}), "") {
		envKey := prefix + "_" + strings.ToUpper(replacer.Replace(key))
		_ = v.BindEnv(key, envKey)
	}
}

func collectMapstructureKeys(t reflect.Type, parent string) []string {
	keys := make([]string, 0)
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		tag := field.Tag.Get("mapstructure")
		if tag == "" || tag == "-" {
			continue
		}

		key := tag
		if parent != "" {
			key = parent + "." + tag
		}

		if field.Type.Kind() == reflect.Struct {
			nested := collectMapstructureKeys(field.Type, key)
			if len(nested) > 0 {
				keys = append(keys, nested...)
				continue
			}
		}

		keys = append(keys, key)
	}
	return keys
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
	v.SetDefault("storage.s3.endpoint", defaults.Storage.S3.Endpoint)
	v.SetDefault("storage.s3.access_key", defaults.Storage.S3.AccessKey)
	v.SetDefault("storage.s3.secret_key", defaults.Storage.S3.SecretKey)
	v.SetDefault("storage.s3.bucket", defaults.Storage.S3.Bucket)
	v.SetDefault("storage.s3.region", defaults.Storage.S3.Region)
	v.SetDefault("storage.s3.use_ssl", defaults.Storage.S3.UseSSL)
	v.SetDefault("storage.s3.prefix", defaults.Storage.S3.Prefix)
	v.SetDefault("storage.rustfs.endpoint", defaults.Storage.RustFS.Endpoint)
	v.SetDefault("storage.rustfs.access_key", defaults.Storage.RustFS.AccessKey)
	v.SetDefault("storage.rustfs.secret_key", defaults.Storage.RustFS.SecretKey)
	v.SetDefault("storage.rustfs.bucket", defaults.Storage.RustFS.Bucket)
	v.SetDefault("storage.rustfs.region", defaults.Storage.RustFS.Region)
	v.SetDefault("storage.rustfs.use_ssl", defaults.Storage.RustFS.UseSSL)
	v.SetDefault("storage.rustfs.prefix", defaults.Storage.RustFS.Prefix)

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
