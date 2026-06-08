package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"mvp-push-gateway/backend/internal/config"
	"mvp-push-gateway/backend/internal/secretbox"
	"mvp-push-gateway/backend/internal/source"
)

var ErrMissingDSN = errors.New("postgres dsn is required")

type PoolSet struct {
	API         *pgxpool.Pool
	Planning    *pgxpool.Pool
	Sending     *pgxpool.Pool
	Maintenance *pgxpool.Pool
}

func OpenPool(ctx context.Context, dsn string, poolConfig config.PoolConfig) (*pgxpool.Pool, error) {
	if dsn == "" {
		return nil, ErrMissingDSN
	}

	parsed, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse postgres dsn: %w", err)
	}
	parsed.MaxConns = poolConfig.MaxConns
	parsed.MinConns = poolConfig.MinConns
	parsed.HealthCheckPeriod = 30 * time.Second

	pool, err := pgxpool.NewWithConfig(ctx, parsed)
	if err != nil {
		return nil, fmt.Errorf("open postgres pool: %w", err)
	}
	return pool, nil
}

func OpenPools(ctx context.Context, postgres config.PostgresConfig) (*PoolSet, error) {
	apiPool, err := OpenPool(ctx, postgres.DSN, postgres.APIPool)
	if err != nil {
		return nil, fmt.Errorf("open api pool: %w", err)
	}

	planningPool, err := OpenPool(ctx, postgres.DSN, postgres.PlanningPool)
	if err != nil {
		apiPool.Close()
		return nil, fmt.Errorf("open planning pool: %w", err)
	}

	sendingPool, err := OpenPool(ctx, postgres.DSN, postgres.SendingPool)
	if err != nil {
		apiPool.Close()
		planningPool.Close()
		return nil, fmt.Errorf("open sending pool: %w", err)
	}

	maintenancePool, err := OpenPool(ctx, postgres.DSN, postgres.MaintenancePool)
	if err != nil {
		apiPool.Close()
		planningPool.Close()
		sendingPool.Close()
		return nil, fmt.Errorf("open maintenance pool: %w", err)
	}

	return &PoolSet{
		API:         apiPool,
		Planning:    planningPool,
		Sending:     sendingPool,
		Maintenance: maintenancePool,
	}, nil
}

func (p *PoolSet) Close() {
	if p == nil {
		return
	}
	if p.API != nil {
		p.API.Close()
	}
	if p.Planning != nil {
		p.Planning.Close()
	}
	if p.Sending != nil {
		p.Sending.Close()
	}
	if p.Maintenance != nil {
		p.Maintenance.Close()
	}
}

type Repository struct {
	pool             *pgxpool.Pool
	asyncRuntimeLogs *AsyncRuntimeLogWriter
	secretCipher     *secretbox.Cipher
}

type RepositoryOption func(*Repository)

func WithSecretCipher(cipher *secretbox.Cipher) RepositoryOption {
	return func(r *Repository) {
		r.secretCipher = cipher
	}
}

func NewRepository(pool *pgxpool.Pool, options ...RepositoryOption) Repository {
	repository := Repository{pool: pool}
	for _, option := range options {
		option(&repository)
	}
	return repository
}

func NewRepositoryWithAsyncRuntimeLogWriter(pool *pgxpool.Pool, writer *AsyncRuntimeLogWriter, options ...RepositoryOption) Repository {
	repository := Repository{pool: pool, asyncRuntimeLogs: writer}
	for _, option := range options {
		option(&repository)
	}
	return repository
}

func (r Repository) acquireConn(ctx context.Context, traceID string, stage SQLTimingStage) (*pgxpool.Conn, error) {
	startedAt := time.Now()
	conn, err := r.pool.Acquire(ctx)
	recordSQLTiming(ctx, traceID, stage, time.Since(startedAt))
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func (r Repository) Ping(ctx context.Context) error {
	if r.pool == nil {
		return errors.New("postgres pool is nil")
	}
	return r.pool.Ping(ctx)
}

func (r Repository) RuntimeStats(ctx context.Context) (source.RuntimeStats, error) {
	if r.pool == nil {
		return source.RuntimeStats{}, nil
	}
	stats := r.pool.Stat()
	result := source.RuntimeStats{
		DBPoolAcquireCount:   stats.AcquireCount(),
		DBPoolWaitCount:      stats.EmptyAcquireCount(),
		DBPoolWaitDurationMS: stats.AcquireDuration().Milliseconds(),
		DBPoolAcquiredConns:  stats.AcquiredConns(),
		DBPoolTotalConns:     stats.TotalConns(),
	}
	_ = r.pool.QueryRow(ctx, `
		SELECT
			current_setting('max_connections')::int,
			COALESCE(blks_read, 0),
			COALESCE(blks_hit, 0),
			COALESCE(temp_bytes, 0)
		FROM pg_stat_database
		WHERE datname = current_database()
	`).Scan(
		&result.PostgresMaxConnections,
		&result.PostgresBlocksRead,
		&result.PostgresBlocksHit,
		&result.PostgresTempBytes,
	)
	return result, nil
}
