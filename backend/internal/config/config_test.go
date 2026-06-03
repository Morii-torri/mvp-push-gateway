package config_test

import (
	"testing"

	"mvp-push-gateway/backend/internal/config"
)

func TestLoadUsesSafeDefaults(t *testing.T) {
	t.Setenv("MGP_HOST", "")
	t.Setenv("MGP_PORT", "")
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
