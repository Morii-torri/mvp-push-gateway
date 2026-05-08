package db

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"mvp-push-gateway/backend/internal/provider"
)

func TestProviderCapabilitySeedIsIdempotent(t *testing.T) {
	dsn := os.Getenv("MGP_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("MGP_TEST_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	schemaName := createMigratedTestSchema(ctx, t, dsn)
	defer dropTestSchema(schemaName)

	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parse pool config: %v", err)
	}
	poolConfig.ConnConfig.RuntimeParams["search_path"] = schemaName

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		t.Fatalf("open test pool: %v", err)
	}
	defer pool.Close()

	repository := NewRepository(pool)
	capabilities := provider.DefaultCapabilities()
	if err := repository.SeedProviderCapabilities(ctx, capabilities); err != nil {
		t.Fatalf("first seed provider capabilities: %v", err)
	}
	if err := repository.SeedProviderCapabilities(ctx, capabilities); err != nil {
		t.Fatalf("second seed provider capabilities: %v", err)
	}

	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*)::integer FROM provider_capabilities`).Scan(&count); err != nil {
		t.Fatalf("count provider capabilities: %v", err)
	}
	if count != 9 {
		t.Fatalf("expected 9 provider capabilities after repeated seed, got %d", count)
	}
}
