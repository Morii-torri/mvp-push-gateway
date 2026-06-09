package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"mvp-push-gateway/backend/internal/config"
	"mvp-push-gateway/backend/internal/db"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	if err := run(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	cfg := config.Load()
	if cfg.Postgres.DSN == "" {
		return fmt.Errorf("MGP_POSTGRES_DSN is required")
	}
	pool, err := db.OpenPool(ctx, cfg.Postgres.DSN, cfg.Postgres.MaintenancePool)
	if err != nil {
		return err
	}
	defer pool.Close()

	repository := db.NewRepository(pool)
	stats, err := repository.BackfillSensitiveLogData(ctx)
	if err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(stats, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(encoded))
	return nil
}
