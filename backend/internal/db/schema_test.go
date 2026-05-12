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

func TestProviderCapabilityMetadataMigrationContainsNewColumns(t *testing.T) {
	migration, err := os.ReadFile("../../migrations/000005_provider_capability_metadata.sql")
	if err != nil {
		t.Fatalf("read provider capability metadata migration: %v", err)
	}
	content := string(migration)
	required := []string{
		"ADD COLUMN display_name text",
		"ADD COLUMN category text",
		"ADD COLUMN credential_schema jsonb",
		"ADD COLUMN channel_config_schema jsonb",
		"ADD COLUMN custom_body_allowed boolean",
		"ADD COLUMN recipient_requirement text",
		"ADD COLUMN token_strategy jsonb",
		"ADD COLUMN send_api jsonb",
		"ADD COLUMN success_rule jsonb",
		"ADD COLUMN retry_rule jsonb",
		"ADD COLUMN default_rate_limit jsonb",
		"ADD COLUMN default_timeout_ms integer",
		"ADD COLUMN default_concurrency_limit integer",
		"ADD COLUMN default_retry_policy jsonb",
	}

	for _, snippet := range required {
		if !strings.Contains(content, snippet) {
			t.Fatalf("provider capability metadata migration missing snippet: %s", snippet)
		}
	}
}

func TestRouteActionTargetsMigrationContainsTableAndBackfill(t *testing.T) {
	migration, err := os.ReadFile("../../migrations/000006_route_action_targets.sql")
	if err != nil {
		t.Fatalf("read route action targets migration: %v", err)
	}
	content := string(migration)
	required := []string{
		"CREATE TABLE route_action_targets",
		"action_id uuid NOT NULL REFERENCES route_actions(id) ON DELETE CASCADE",
		"channel_id uuid NOT NULL REFERENCES delivery_channels(id) ON DELETE RESTRICT",
		"template_version_id uuid NOT NULL REFERENCES template_versions(id) ON DELETE RESTRICT",
		"UNIQUE (action_id, sort_order)",
		"CREATE INDEX idx_route_action_targets_action",
		"INSERT INTO route_action_targets",
		"CROSS JOIN LATERAL unnest(action.channel_ids) WITH ORDINALITY",
		"DROP TABLE IF EXISTS route_action_targets",
	}

	for _, snippet := range required {
		if !strings.Contains(content, snippet) {
			t.Fatalf("route action targets migration missing snippet: %s", snippet)
		}
	}
}

func TestProviderTypeExpansionMigrationContainsNewProviderTypes(t *testing.T) {
	migration, err := os.ReadFile("../../migrations/000007_provider_type_expansion.sql")
	if err != nil {
		t.Fatalf("read provider type expansion migration: %v", err)
	}
	content := string(migration)
	required := []string{
		"DROP CONSTRAINT IF EXISTS delivery_channels_provider_type_check",
		"DROP CONSTRAINT IF EXISTS provider_capabilities_provider_type_check",
		"ADD CONSTRAINT delivery_channels_provider_type_check",
		"ADD CONSTRAINT provider_capabilities_provider_type_check",
		"ADD CONSTRAINT provider_capabilities_recipient_format_check",
		"ADD CONSTRAINT provider_capabilities_recipient_requirement_check",
		"'pushplus'",
		"'wxpusher'",
		"'serverchan'",
		"'aliyun_sms'",
		"'tencent_sms'",
		"'baidu_sms'",
		"'wecom_robot'",
		"'wecom_app'",
		"'dingtalk_robot'",
		"'dingtalk_work'",
		"'feishu_robot'",
		"'none'",
		"'system_or_channel'",
	}

	for _, snippet := range required {
		if !strings.Contains(content, snippet) {
			t.Fatalf("provider type expansion migration missing snippet: %s", snippet)
		}
	}
}

func TestProviderTypeRegistryMigrationRemovesHardCodedProviderTypeChecks(t *testing.T) {
	migration, err := os.ReadFile("../../migrations/000008_provider_type_registry.sql")
	if err != nil {
		t.Fatalf("read provider type registry migration: %v", err)
	}
	content := string(migration)
	required := []string{
		"CREATE TABLE IF NOT EXISTS provider_types",
		"provider_type text PRIMARY KEY",
		"INSERT INTO provider_types",
		"SELECT DISTINCT provider_type FROM delivery_channels",
		"SELECT DISTINCT provider_type FROM provider_capabilities",
		"DROP CONSTRAINT IF EXISTS delivery_channels_provider_type_check",
		"DROP CONSTRAINT IF EXISTS provider_capabilities_provider_type_check",
		"ADD CONSTRAINT delivery_channels_provider_type_fkey",
		"ADD CONSTRAINT provider_capabilities_provider_type_fkey",
		"'ntfy'",
		"'gotify'",
		"'bark'",
		"'pushme'",
	}

	for _, snippet := range required {
		if !strings.Contains(content, snippet) {
			t.Fatalf("provider type registry migration missing snippet: %s", snippet)
		}
	}
	if strings.Contains(content, "ADD CONSTRAINT delivery_channels_provider_type_check") ||
		strings.Contains(content, "ADD CONSTRAINT provider_capabilities_provider_type_check") {
		t.Fatal("registry migration should not reintroduce hard-coded provider_type CHECK constraints in the up migration")
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
