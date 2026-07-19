// Package postgres provides PostgreSQL connection pool management for AEGIS services.
//
// It wraps pgxpool with health checking, connection lifecycle management,
// and structured logging. The pool is configured for high-concurrency workloads
// typical of exam delivery (thousands of concurrent answer submissions).
package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/aegis-platform/aegis/pkg/config"
)

// NewPool creates a new pgxpool.Pool with the given database configuration.
// It validates connectivity with a ping before returning.
func NewPool(ctx context.Context, cfg config.DatabaseConfig, logger *zap.Logger) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("parsing database DSN: %w", err)
	}

	// Connection pool settings
	poolCfg.MaxConns = cfg.MaxConns
	poolCfg.MinConns = cfg.MinConns
	poolCfg.MaxConnLifetime = cfg.MaxConnLifetime
	poolCfg.MaxConnIdleTime = cfg.MaxConnIdleTime
	poolCfg.HealthCheckPeriod = 30 * time.Second

	// Connection-level settings applied to each new connection
	poolCfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		// Set statement timeout to prevent long-running queries
		_, err := conn.Exec(ctx, "SET statement_timeout = '30s'")
		if err != nil {
			logger.Warn("failed to set statement_timeout", zap.Error(err))
		}
		// Set search path
		_, err = conn.Exec(ctx, "SET search_path TO public")
		return err
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("creating connection pool: %w", err)
	}

	// Verify connectivity
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("database ping failed: %w", err)
	}

	logger.Info("database connection pool established",
		zap.String("host", cfg.Host),
		zap.Int("port", cfg.Port),
		zap.String("database", cfg.DBName),
		zap.Int32("max_conns", cfg.MaxConns),
		zap.Int32("min_conns", cfg.MinConns),
	)

	return pool, nil
}

// SetOrganizationContext sets the current organization ID for row-level security.
// This must be called at the start of each request that accesses tenant-scoped data.
func SetOrganizationContext(ctx context.Context, pool *pgxpool.Pool, orgID string) error {
	_, err := pool.Exec(ctx, fmt.Sprintf("SET app.current_organization_id = '%s'", orgID))
	return err
}
