// Package main is the entry point for the AEGIS Question Bank Service.
//
// It initializes all dependencies (database, cache, logger, metrics),
// wires up the HTTP server with middleware, and handles graceful shutdown.
//
// Usage:
//
//	AEGIS_DATABASE_HOST=localhost AEGIS_DATABASE_PASSWORD=secret ./question-bank
//	./question-bank --config=/etc/aegis/config.yaml
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

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	"github.com/aegis-platform/aegis/internal/api/handler"
	"github.com/aegis-platform/aegis/internal/infrastructure/postgres"
	"github.com/aegis-platform/aegis/pkg/config"
	"github.com/aegis-platform/aegis/pkg/logging"
	"github.com/aegis-platform/aegis/pkg/middleware"
)

var (
	version   = "dev"
	buildTime = "unknown"
	commit    = "unknown"
)

func main() {
	configPath := flag.String("config", "", "Path to configuration file")
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	logger, err := logging.NewLogger(logging.Config{
		Level:       cfg.Logging.Level,
		Environment: cfg.Logging.Environment,
		ServiceName: "question-bank",
		Version:     version,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	logger.Info("starting Question Bank Service",
		zap.String("version", version),
		zap.String("build_time", buildTime),
		zap.String("commit", commit),
	)

	// Initialize database pool
	dbPool, err := postgres.NewPool(context.Background(), cfg.Database, logger)
	if err != nil {
		logger.Fatal("failed to connect to database", zap.Error(err))
	}
	defer dbPool.Close()

	// Initialize repositories
	itemRepo := postgres.NewItemRepository(dbPool, logger)

	// Initialize services (placeholder — would include full DI)
	_ = itemRepo // Will be injected into service layer

	// Build HTTP router
	mux := http.NewServeMux()

	// Health check endpoints
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy","service":"question-bank","version":"` + version + `"}`))
	})

	mux.HandleFunc("GET /ready", func(w http.ResponseWriter, r *http.Request) {
		// Check database connectivity
		if err := dbPool.Ping(r.Context()); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"status":"not_ready","reason":"database unavailable"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ready"}`))
	})

	// Register API routes
	itemHandler := handler.NewItemHandler(nil, logger) // Service would be injected
	itemHandler.RegisterRoutes(mux)

	// Build middleware chain
	chain := middleware.Chain(
		middleware.Recovery(logger),
		middleware.RequestID,
		middleware.Logger(logger),
		middleware.SecurityHeaders,
		middleware.Timeout(30*time.Second),
	)

	// Application server
	appServer := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      chain(mux),
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	// Metrics server (separate port for Prometheus scraping)
	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", promhttp.Handler())
	metricsServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Observability.MetricsPort),
		Handler: metricsMux,
	}

	// Start servers
	errChan := make(chan error, 2)

	go func() {
		logger.Info("starting HTTP server", zap.String("addr", appServer.Addr))
		if err := appServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- fmt.Errorf("HTTP server error: %w", err)
		}
	}()

	go func() {
		logger.Info("starting metrics server", zap.Int("port", cfg.Observability.MetricsPort))
		if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- fmt.Errorf("metrics server error: %w", err)
		}
	}()

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-quit:
		logger.Info("received shutdown signal", zap.String("signal", sig.String()))
	case err := <-errChan:
		logger.Error("server error", zap.Error(err))
	}

	// Graceful shutdown
	logger.Info("initiating graceful shutdown", zap.Duration("timeout", cfg.Server.ShutdownTimeout))
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()

	if err := appServer.Shutdown(ctx); err != nil {
		logger.Error("HTTP server shutdown error", zap.Error(err))
	}
	if err := metricsServer.Shutdown(ctx); err != nil {
		logger.Error("metrics server shutdown error", zap.Error(err))
	}

	logger.Info("Question Bank Service stopped gracefully")
}
