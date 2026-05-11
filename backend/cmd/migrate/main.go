package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if err := run(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	dsn := strings.TrimSpace(os.Getenv("MGP_POSTGRES_DSN"))
	if dsn == "" {
		return fmt.Errorf("MGP_POSTGRES_DSN is required")
	}

	migrationsDir := migrationsDir()
	paths, err := filepath.Glob(filepath.Join(migrationsDir, "*.sql"))
	if err != nil {
		return fmt.Errorf("list migrations: %w", err)
	}
	sort.Strings(paths)
	if len(paths) == 0 {
		return fmt.Errorf("no migration files found in %s", migrationsDir)
	}

	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return fmt.Errorf("connect postgres: %w", err)
	}
	defer conn.Close(ctx)

	if _, err := conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version text PRIMARY KEY,
			filename text NOT NULL,
			applied_at timestamptz NOT NULL DEFAULT now()
		)
	`); err != nil {
		return fmt.Errorf("ensure schema_migrations: %w", err)
	}

	for _, path := range paths {
		filename := filepath.Base(path)
		version := strings.SplitN(filename, "_", 2)[0]
		if version == "" || version == filename {
			return fmt.Errorf("migration filename must start with a version prefix: %s", filename)
		}

		var applied bool
		if err := conn.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)`, version).Scan(&applied); err != nil {
			return fmt.Errorf("check migration %s: %w", filename, err)
		}
		if applied {
			fmt.Printf("migration %s already applied\n", filename)
			continue
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", filename, err)
		}
		upSQL := extractGooseUp(string(content))
		if strings.TrimSpace(upSQL) == "" {
			return fmt.Errorf("migration %s has no goose Up SQL", filename)
		}

		tx, err := conn.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin migration %s: %w", filename, err)
		}
		if _, err := tx.Exec(ctx, upSQL); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("apply migration %s: %w", filename, err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version, filename) VALUES ($1, $2)`, version, filename); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("record migration %s: %w", filename, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit migration %s: %w", filename, err)
		}
		fmt.Printf("applied migration %s\n", filename)
	}

	return nil
}

func migrationsDir() string {
	if value := strings.TrimSpace(os.Getenv("MGP_MIGRATIONS_DIR")); value != "" {
		return value
	}
	if _, err := os.Stat("/app/backend/migrations"); err == nil {
		return "/app/backend/migrations"
	}
	return "backend/migrations"
}

func extractGooseUp(migration string) string {
	var builder strings.Builder
	inUp := false
	for _, line := range strings.Split(migration, "\n") {
		switch {
		case strings.HasPrefix(line, "-- +goose Up"):
			inUp = true
			continue
		case strings.HasPrefix(line, "-- +goose Down"):
			return builder.String()
		case strings.HasPrefix(line, "-- +goose"):
			continue
		case inUp:
			builder.WriteString(line)
			builder.WriteByte('\n')
		}
	}
	return builder.String()
}
