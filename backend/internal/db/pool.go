package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"mvp-push-gateway/backend/internal/config"
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
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) Repository {
	return Repository{pool: pool}
}

func (r Repository) Ping(ctx context.Context) error {
	if r.pool == nil {
		return errors.New("postgres pool is nil")
	}
	return r.pool.Ping(ctx)
}
