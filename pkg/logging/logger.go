// Package logging provides structured, context-aware logging for the AEGIS platform.
//
// It wraps uber-go/zap to provide:
//   - JSON-formatted structured logs for production
//   - Request-scoped fields (request ID, user ID, trace ID)
//   - Context injection via middleware
//   - Log level configuration at runtime
//
// Usage:
//
//	logger := logging.NewLogger(logging.Config{Level: "info", Environment: "production"})
//	logger.Info("item created", zap.String("item_id", id), zap.String("status", "DRAFT"))
//
//	// In HTTP handlers, use the context-enriched logger:
//	log := logging.FromContext(ctx)
//	log.Info("processing request")
package logging

import (
	"context"
	"os"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// contextKey is a private type for context keys to prevent collisions.
type contextKey struct{}

// loggerKey is the context key for storing the logger.
var loggerKey = contextKey{}

// Config holds logging configuration.
type Config struct {
	// Level sets the minimum log level (debug, info, warn, error).
	Level string `mapstructure:"level" json:"level"`
	// Environment determines the output format (development=console, production=json).
	Environment string `mapstructure:"environment" json:"environment"`
	// ServiceName is the name of the service for log attribution.
	ServiceName string `mapstructure:"service_name" json:"service_name"`
	// Version is the service version for log attribution.
	Version string `mapstructure:"version" json:"version"`
}

// NewLogger creates a new production-quality zap logger.
// In production mode, it outputs JSON with timestamps in ISO 8601 format.
// In development mode, it outputs human-readable console format.
func NewLogger(cfg Config) (*zap.Logger, error) {
	level, err := zapcore.ParseLevel(cfg.Level)
	if err != nil {
		level = zapcore.InfoLevel
	}

	var encoder zapcore.Encoder
	var outputPaths []string

	if cfg.Environment == "development" {
		encoderCfg := zap.NewDevelopmentEncoderConfig()
		encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder
		encoderCfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
		encoder = zapcore.NewConsoleEncoder(encoderCfg)
		outputPaths = []string{"stderr"}
	} else {
		encoderCfg := zap.NewProductionEncoderConfig()
		encoderCfg.TimeKey = "timestamp"
		encoderCfg.EncodeTime = zapcore.TimeEncoderOfLayout(time.RFC3339Nano)
		encoderCfg.EncodeLevel = zapcore.LowercaseLevelEncoder
		encoderCfg.EncodeDuration = zapcore.MillisDurationEncoder
		encoderCfg.EncodeCaller = zapcore.ShortCallerEncoder
		encoderCfg.StacktraceKey = "stacktrace"
		encoder = zapcore.NewJSONEncoder(encoderCfg)
		outputPaths = []string{"stderr"}
	}

	// Build core with level filter
	core := zapcore.NewCore(
		encoder,
		zapcore.AddSync(os.Stderr),
		level,
	)

	// Build logger with standard fields
	logger := zap.New(core,
		zap.AddCaller(),
		zap.AddCallerSkip(0),
		zap.AddStacktrace(zapcore.ErrorLevel),
	)

	// Add service-level fields
	if cfg.ServiceName != "" {
		logger = logger.With(zap.String("service", cfg.ServiceName))
	}
	if cfg.Version != "" {
		logger = logger.With(zap.String("version", cfg.Version))
	}

	hostname, err := os.Hostname()
	if err == nil {
		logger = logger.With(zap.String("hostname", hostname))
	}

	_ = outputPaths // Used for documentation; actual output configured via core

	return logger, nil
}

// MustNewLogger creates a new logger and panics on failure.
func MustNewLogger(cfg Config) *zap.Logger {
	logger, err := NewLogger(cfg)
	if err != nil {
		panic("failed to create logger: " + err.Error())
	}
	return logger
}

// WithContext returns a new context with the logger attached.
func WithContext(ctx context.Context, logger *zap.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, logger)
}

// FromContext extracts the logger from the context.
// Returns a no-op logger if none is found.
func FromContext(ctx context.Context) *zap.Logger {
	if logger, ok := ctx.Value(loggerKey).(*zap.Logger); ok {
		return logger
	}
	return zap.NewNop()
}

// WithRequestID returns a logger enriched with the request ID.
func WithRequestID(logger *zap.Logger, requestID string) *zap.Logger {
	return logger.With(zap.String("request_id", requestID))
}

// WithUserID returns a logger enriched with the user ID.
func WithUserID(logger *zap.Logger, userID string) *zap.Logger {
	return logger.With(zap.String("user_id", userID))
}

// WithTraceID returns a logger enriched with the distributed trace ID.
func WithTraceID(logger *zap.Logger, traceID string) *zap.Logger {
	return logger.With(zap.String("trace_id", traceID))
}

// WithOrganizationID returns a logger enriched with the organization ID for multi-tenant context.
func WithOrganizationID(logger *zap.Logger, orgID string) *zap.Logger {
	return logger.With(zap.String("organization_id", orgID))
}

// WithExamID returns a logger enriched with the exam ID for exam-scoped operations.
func WithExamID(logger *zap.Logger, examID string) *zap.Logger {
	return logger.With(zap.String("exam_id", examID))
}

// WithComponent returns a logger enriched with a component name for intra-service attribution.
func WithComponent(logger *zap.Logger, component string) *zap.Logger {
	return logger.With(zap.String("component", component))
}
