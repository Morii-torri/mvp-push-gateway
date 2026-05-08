package db

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestMigrationsApplyToPostgres(t *testing.T) {
	dsn := os.Getenv("MGP_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("MGP_TEST_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connect postgres: %v", err)
	}
	defer conn.Close(ctx)

	schemaName := "mgp_migration_test_" + strings.ReplaceAll(time.Now().UTC().Format("20060102150405.000000000"), ".", "_")
	if _, err := conn.Exec(ctx, "CREATE SCHEMA "+schemaName); err != nil {
		t.Fatalf("create test schema: %v", err)
	}
	defer conn.Exec(context.Background(), "DROP SCHEMA "+schemaName+" CASCADE")

	if _, err := conn.Exec(ctx, "SET search_path TO "+schemaName); err != nil {
		t.Fatalf("set search_path: %v", err)
	}

	for _, migration := range readGooseUpMigrations(t) {
		if _, err := conn.Exec(ctx, migration); err != nil {
			t.Fatalf("apply migration: %v", err)
		}
	}

	assertDuplicateEnabledRouteFlowBlocked(ctx, t, conn)
	assertInboundDedupeScopedBySource(ctx, t, conn)
	assertSendDedupeScopedByChannel(ctx, t, conn)
}

func assertDuplicateEnabledRouteFlowBlocked(ctx context.Context, t *testing.T, conn *pgx.Conn) {
	t.Helper()

	sourceID := "00000000-0000-0000-0000-000000000001"
	flowID1 := "00000000-0000-0000-0000-000000000101"
	flowID2 := "00000000-0000-0000-0000-000000000102"
	if _, err := conn.Exec(ctx, `INSERT INTO inbound_sources (id, code, name) VALUES ($1, $2, $3)`, sourceID, "source-a", "source A"); err != nil {
		t.Fatalf("insert source: %v", err)
	}
	if _, err := conn.Exec(ctx, `INSERT INTO route_flows (id, source_id, name, enabled) VALUES ($1, $2, $3, true)`, flowID1, sourceID, "flow 1"); err != nil {
		t.Fatalf("insert first enabled route flow: %v", err)
	}
	if _, err := conn.Exec(ctx, `INSERT INTO route_flows (id, source_id, name, enabled) VALUES ($1, $2, $3, true)`, flowID2, sourceID, "flow 2"); err == nil {
		t.Fatal("expected duplicate enabled route flow to be blocked")
	}
}

func assertInboundDedupeScopedBySource(ctx context.Context, t *testing.T, conn *pgx.Conn) {
	t.Helper()

	if _, err := conn.Exec(ctx, `INSERT INTO dedupe_keys (id, scope, source_id, dedupe_key, expires_at) VALUES ($1, 'inbound', $2, 'same-key', now() + interval '1 hour')`, "00000000-0000-0000-0000-000000000201", "00000000-0000-0000-0000-000000000001"); err != nil {
		t.Fatalf("insert inbound dedupe key: %v", err)
	}
	if _, err := conn.Exec(ctx, `INSERT INTO dedupe_keys (id, scope, source_id, dedupe_key, expires_at) VALUES ($1, 'inbound', $2, 'same-key', now() + interval '1 hour')`, "00000000-0000-0000-0000-000000000202", "00000000-0000-0000-0000-000000000001"); err == nil {
		t.Fatal("expected duplicate inbound dedupe key in the same source to be blocked")
	}
}

func assertSendDedupeScopedByChannel(ctx context.Context, t *testing.T, conn *pgx.Conn) {
	t.Helper()

	channelID := "00000000-0000-0000-0000-000000000301"
	if _, err := conn.Exec(ctx, `INSERT INTO delivery_channels (id, provider_type, name) VALUES ($1, 'webhook', 'webhook A')`, channelID); err != nil {
		t.Fatalf("insert channel: %v", err)
	}
	if _, err := conn.Exec(ctx, `INSERT INTO dedupe_keys (id, scope, channel_id, dedupe_key, expires_at) VALUES ($1, 'send', $2, 'same-key', now() + interval '1 hour')`, "00000000-0000-0000-0000-000000000401", channelID); err != nil {
		t.Fatalf("insert send dedupe key: %v", err)
	}
	if _, err := conn.Exec(ctx, `INSERT INTO dedupe_keys (id, scope, channel_id, dedupe_key, expires_at) VALUES ($1, 'send', $2, 'same-key', now() + interval '1 hour')`, "00000000-0000-0000-0000-000000000402", channelID); err == nil {
		t.Fatal("expected duplicate send dedupe key in the same channel to be blocked")
	}
}

func extractGooseUp(migration string) string {
	var builder strings.Builder
	for _, line := range strings.Split(migration, "\n") {
		if strings.HasPrefix(line, "-- +goose Down") {
			break
		}
		builder.WriteString(line)
		builder.WriteByte('\n')
	}
	return builder.String()
}

func readGooseUpMigrations(t *testing.T) []string {
	t.Helper()

	paths, err := filepath.Glob("../../migrations/*.sql")
	if err != nil {
		t.Fatalf("list migrations: %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("expected at least one migration")
	}

	migrations := make([]string, 0, len(paths))
	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read migration %s: %v", path, err)
		}
		migrations = append(migrations, extractGooseUp(string(content)))
	}
	return migrations
}
