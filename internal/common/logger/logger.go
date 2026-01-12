// Package logger provides structured logging for the JzSE system.
package logger

import (
	"os"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	globalLogger *zap.Logger
	once         sync.Once
)

// Config holds logger configuration.
type Config struct {
	Level       string `mapstructure:"level"`       // debug, info, warn, error
	Format      string `mapstructure:"format"`      // json, console
	Output      string `mapstructure:"output"`      // stdout, stderr, file path
	Development bool   `mapstructure:"development"` // Enable development mode
}

// DefaultConfig returns the default logger configuration.
func DefaultConfig() Config {
	return Config{
		Level:       "info",
		Format:      "json",
		Output:      "stdout",
		Development: false,
	}
}

// Init initializes the global logger with the given configuration.
func Init(cfg Config) error {
	var err error
	once.Do(func() {
		globalLogger, err = newLogger(cfg)
	})
	return err
}

// newLogger creates a new zap logger based on configuration.
func newLogger(cfg Config) (*zap.Logger, error) {
	// Parse log level
	level, err := zapcore.ParseLevel(cfg.Level)
	if err != nil {
		level = zapcore.InfoLevel
	}

	// Configure encoder
	var encoderConfig zapcore.EncoderConfig
	if cfg.Development {
		encoderConfig = zap.NewDevelopmentEncoderConfig()
	} else {
		encoderConfig = zap.NewProductionEncoderConfig()
	}
	encoderConfig.TimeKey = "timestamp"
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	// Create encoder
	var encoder zapcore.Encoder
	if cfg.Format == "console" {
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	} else {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	}

	// Configure output
	var writeSyncer zapcore.WriteSyncer
	switch cfg.Output {
	case "stdout":
		writeSyncer = zapcore.AddSync(os.Stdout)
	case "stderr":
		writeSyncer = zapcore.AddSync(os.Stderr)
	default:
		file, err := os.OpenFile(cfg.Output, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return nil, err
		}
		writeSyncer = zapcore.AddSync(file)
	}

	// Create core
	core := zapcore.NewCore(encoder, writeSyncer, level)

	// Create logger
	logger := zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))

	return logger, nil
}

// L returns the global logger.
func L() *zap.Logger {
	if globalLogger == nil {
		// Fallback to development logger if not initialized
		globalLogger, _ = zap.NewDevelopment()
	}
	return globalLogger
}

// S returns the global sugared logger.
func S() *zap.SugaredLogger {
	return L().Sugar()
}

// With returns a logger with additional fields.
func With(fields ...zap.Field) *zap.Logger {
	return L().With(fields...)
}

// WithComponent returns a logger with component field.
func WithComponent(component string) *zap.Logger {
	return L().With(zap.String("component", component))
}

// WithRegion returns a logger with region field.
func WithRegion(regionID string) *zap.Logger {
	return L().With(zap.String("region_id", regionID))
}

// Sync flushes any buffered log entries.
func Sync() error {
	if globalLogger != nil {
		return globalLogger.Sync()
	}
	return nil
}
