package db

import (
	"os"
	"strings"
	"testing"
)

func TestInitialMigrationContainsCriticalConstraints(t *testing.T) {
	migration := readInitialMigration(t)
	required := []string{
		"CREATE TABLE inbound_sources",
		"CREATE TABLE delivery_channels",
		"CREATE TABLE templates",
		"CREATE TABLE route_flows",
		"CREATE TABLE route_rule_counters",
		"CREATE TABLE message_records",
		"CREATE TABLE delivery_attempts",
		"CREATE TABLE dedupe_keys",
		"CREATE TABLE jobs",
		"CREATE TABLE worker_metrics",
		"CREATE TABLE setup_state",
		"CREATE TABLE audit_logs",
		"CREATE UNIQUE INDEX ux_route_flows_enabled_source",
		"ON route_flows(source_id)",
		"WHERE enabled",
		"CREATE UNIQUE INDEX ux_dedupe_inbound_source_key",
		"ON dedupe_keys(scope, source_id, dedupe_key)",
		"WHERE scope = 'inbound'",
		"CREATE UNIQUE INDEX ux_dedupe_send_channel_key",
		"ON dedupe_keys(scope, channel_id, dedupe_key)",
		"WHERE scope = 'send'",
		"heartbeat_at timestamptz",
		"CHECK (hit_count >= 0 AND hit_count <= 99999)",
	}

	for _, snippet := range required {
		if !strings.Contains(migration, snippet) {
			t.Fatalf("migration missing required snippet: %s", snippet)
		}
	}
}

func TestInitialMigrationContainsRetentionIndexes(t *testing.T) {
	migration := readInitialMigration(t)
	required := []string{
		"idx_message_records_received_at",
		"idx_delivery_attempts_channel_queued",
		"idx_delivery_attempts_status_queued",
		"idx_jobs_retention_finished",
		"idx_dead_letter_jobs_dead_lettered",
		"idx_dedupe_keys_expires_at",
		"idx_audit_logs_created_at",
		"idx_worker_metrics_bucket_job",
	}

	for _, snippet := range required {
		if !strings.Contains(migration, snippet) {
			t.Fatalf("migration missing retention/index snippet: %s", snippet)
		}
	}
}

func TestInitialMigrationDoesNotReintroduceScheduledSend(t *testing.T) {
	migration := strings.ToLower(readInitialMigration(t))
	if strings.Contains(migration, "scheduled_send") || strings.Contains(migration, "scheduled_messages") {
		t.Fatal("step 2 migration must not include scheduled send artifacts")
	}
}

func readInitialMigration(t *testing.T) string {
	t.Helper()

	content, err := os.ReadFile("../../migrations/000001_init.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	return string(content)
}
