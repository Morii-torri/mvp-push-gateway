package config_test

import (
	"testing"

	"mvp-push-gateway/backend/internal/config"
)

func TestLoadUsesSafeDefaults(t *testing.T) {
	t.Setenv("MGP_HOST", "")
	t.Setenv("MGP_PORT", "")
	t.Setenv("MGP_PPROF_PORT", "")
	t.Setenv("MGP_API_PREFIX", "")
	t.Setenv("MGP_APP_NAME", "")
	t.Setenv("MGP_ENVIRONMENT", "")
	t.Setenv("MGP_POSTGRES_DSN", "")

	cfg := config.Load()

	if cfg.Server.Host != "0.0.0.0" {
		t.Fatalf("expected default host 0.0.0.0, got %q", cfg.Server.Host)
	}
	if cfg.Server.Port != "8080" {
		t.Fatalf("expected default port 8080, got %q", cfg.Server.Port)
	}
	if cfg.Server.PprofPort != "" {
		t.Fatalf("expected pprof port to be disabled by default, got %q", cfg.Server.PprofPort)
	}
	if cfg.Server.APIPrefix != "/api/v1" {
		t.Fatalf("expected default API prefix /api/v1, got %q", cfg.Server.APIPrefix)
	}
	if cfg.App.Name != "MVP Push Gateway" {
		t.Fatalf("expected default app name, got %q", cfg.App.Name)
	}
	if cfg.App.Environment != "development" {
		t.Fatalf("expected default environment development, got %q", cfg.App.Environment)
	}
	if cfg.Postgres.DSN != "" {
		t.Fatalf("expected empty default PostgreSQL DSN placeholder, got %q", cfg.Postgres.DSN)
	}
	if cfg.Postgres.APIPool.MaxConns != 60 {
		t.Fatalf("expected default API pool max connections 60, got %d", cfg.Postgres.APIPool.MaxConns)
	}
	if cfg.Postgres.PlanningPool.MaxConns != 20 {
		t.Fatalf("expected default planning pool max connections 20, got %d", cfg.Postgres.PlanningPool.MaxConns)
	}
	if cfg.Postgres.SendingPool.MaxConns != 20 {
		t.Fatalf("expected default sending pool max connections 20, got %d", cfg.Postgres.SendingPool.MaxConns)
	}
	if cfg.Postgres.MaintenancePool.MaxConns != 6 {
		t.Fatalf("expected default maintenance pool max connections 6, got %d", cfg.Postgres.MaintenancePool.MaxConns)
	}
}

func TestLoadAllowsPprofPortOverride(t *testing.T) {
	t.Setenv("MGP_PPROF_PORT", "6060")

	cfg := config.Load()

	if cfg.Server.PprofPort != "6060" {
		t.Fatalf("expected pprof port override 6060, got %q", cfg.Server.PprofPort)
	}
}

func TestLoadAllowsPostgresPoolEnvironmentOverrides(t *testing.T) {
	t.Setenv("MGP_POSTGRES_API_MAX_CONNS", "70")
	t.Setenv("MGP_POSTGRES_PLANNING_MAX_CONNS", "25")
	t.Setenv("MGP_POSTGRES_SENDING_MAX_CONNS", "30")
	t.Setenv("MGP_POSTGRES_MAINTENANCE_MAX_CONNS", "8")

	cfg := config.Load()

	if cfg.Postgres.APIPool.MaxConns != 70 ||
		cfg.Postgres.PlanningPool.MaxConns != 25 ||
		cfg.Postgres.SendingPool.MaxConns != 30 ||
		cfg.Postgres.MaintenancePool.MaxConns != 8 {
		t.Fatalf("expected pool max connection overrides to apply, got %+v", cfg.Postgres)
	}
}

func TestLoadUsesJetStreamQueueDefaults(t *testing.T) {
	t.Setenv("MGP_QUEUE_BACKEND", "")
	t.Setenv("MGP_NATS_URL", "")
	t.Setenv("MGP_NATS_CREDS", "")
	t.Setenv("MGP_NATS_STREAM_REPLICAS", "")
	t.Setenv("MGP_NATS_ROUTE_CONSUMERS", "")
	t.Setenv("MGP_NATS_SEND_CONSUMERS", "")
	t.Setenv("MGP_NATS_RESULT_CONSUMERS", "")
	t.Setenv("MGP_NATS_LOGIN_CAPTCHA_KV_BUCKET", "")
	t.Setenv("MGP_RESULT_WRITER_BATCH_SIZE", "")
	t.Setenv("MGP_RESULT_WRITER_FLUSH_INTERVAL_MS", "")

	cfg := config.Load()

	if cfg.Queue.Backend != "jetstream" {
		t.Fatalf("expected default queue backend jetstream, got %q", cfg.Queue.Backend)
	}
	if cfg.Queue.NATS.URL != "nats://127.0.0.1:4222" {
		t.Fatalf("expected default NATS URL, got %q", cfg.Queue.NATS.URL)
	}
	if cfg.Queue.NATS.CredsPath != "" {
		t.Fatalf("expected empty default NATS creds path, got %q", cfg.Queue.NATS.CredsPath)
	}
	if cfg.Queue.NATS.StreamReplicas != 1 ||
		cfg.Queue.NATS.RouteConsumers != 20 ||
		cfg.Queue.NATS.SendConsumers != 20 ||
		cfg.Queue.NATS.ResultConsumers != 10 ||
		cfg.Queue.ResultWriter.BatchSize != 500 ||
		cfg.Queue.ResultWriter.FlushIntervalMS != 50 {
		t.Fatalf("expected JetStream defaults, got %+v", cfg.Queue)
	}
}

func TestLoadAllowsJetStreamQueueEnvironmentOverrides(t *testing.T) {
	t.Setenv("MGP_QUEUE_BACKEND", "jetstream")
	t.Setenv("MGP_NATS_URL", "nats://nats:4222")
	t.Setenv("MGP_NATS_CREDS", "/run/secrets/nats.creds")
	t.Setenv("MGP_NATS_STREAM_REPLICAS", "3")
	t.Setenv("MGP_NATS_ROUTE_CONSUMERS", "12")
	t.Setenv("MGP_NATS_SEND_CONSUMERS", "24")
	t.Setenv("MGP_NATS_RESULT_CONSUMERS", "6")
	t.Setenv("MGP_NATS_LOGIN_CAPTCHA_KV_BUCKET", "MGP_CUSTOM_CAPTCHA")
	t.Setenv("MGP_RESULT_WRITER_BATCH_SIZE", "250")
	t.Setenv("MGP_RESULT_WRITER_FLUSH_INTERVAL_MS", "25")

	cfg := config.Load()

	if cfg.Queue.Backend != "jetstream" ||
		cfg.Queue.NATS.URL != "nats://nats:4222" ||
		cfg.Queue.NATS.CredsPath != "/run/secrets/nats.creds" ||
		cfg.Queue.NATS.StreamReplicas != 3 ||
		cfg.Queue.NATS.RouteConsumers != 12 ||
		cfg.Queue.NATS.SendConsumers != 24 ||
		cfg.Queue.NATS.ResultConsumers != 6 ||
		cfg.Queue.NATS.LoginCaptchaKVBucket != "MGP_CUSTOM_CAPTCHA" ||
		cfg.Queue.ResultWriter.BatchSize != 250 ||
		cfg.Queue.ResultWriter.FlushIntervalMS != 25 {
		t.Fatalf("expected JetStream queue overrides, got %+v", cfg.Queue)
	}
}

func TestLoadParsesTrustedProxyEntries(t *testing.T) {
	t.Setenv("MGP_TRUSTED_PROXIES", "10.0.0.0/8, 192.168.1.10\n127.0.0.1")

	cfg := config.Load()

	expected := []string{"10.0.0.0/8", "192.168.1.10", "127.0.0.1"}
	if len(cfg.Server.TrustedProxies) != len(expected) {
		t.Fatalf("expected trusted proxies %+v, got %+v", expected, cfg.Server.TrustedProxies)
	}
	for index := range expected {
		if cfg.Server.TrustedProxies[index] != expected[index] {
			t.Fatalf("expected trusted proxy %d to be %q, got %q", index, expected[index], cfg.Server.TrustedProxies[index])
		}
	}
}

func TestLoadAllowsSecretEncryptionKeyConfig(t *testing.T) {
	t.Setenv("MGP_SECRET_ENCRYPTION_KEY", "base64-key-material")
	t.Setenv("MGP_SECRET_ENCRYPTION_KEY_ID", "primary-2026-06")

	cfg := config.Load()

	if cfg.Security.SecretEncryptionKey != "base64-key-material" {
		t.Fatalf("expected secret encryption key from environment, got %q", cfg.Security.SecretEncryptionKey)
	}
	if cfg.Security.SecretEncryptionKeyID != "primary-2026-06" {
		t.Fatalf("expected secret encryption key id from environment, got %q", cfg.Security.SecretEncryptionKeyID)
	}
}
