package db

import (
	"context"
	"encoding/json"
	"os"
	"strings"
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
	if len(capabilities) == 0 {
		t.Fatal("expected default capabilities")
	}
	capabilities[0].DisplayName = "Seeded Display v1"
	capabilities[0].Category = "seeded_category"
	capabilities[0].CustomBodyAllowed = true
	capabilities[0].TokenStrategy = json.RawMessage(`{"strategy":"seeded","placement":{"location":"header","field_name":"Authorization"}}`)
	capabilities[0].DefaultTimeoutMS = 9100
	capabilities[0].DefaultConcurrencyLimit = 7

	if err := repository.SeedProviderCapabilities(ctx, capabilities); err != nil {
		t.Fatalf("first seed provider capabilities: %v", err)
	}
	var p2ProviderTypeCount int
	if err := pool.QueryRow(ctx, `SELECT count(*)::integer FROM provider_types WHERE provider_type IN ('ntfy', 'gotify', 'bark', 'pushme')`).Scan(&p2ProviderTypeCount); err != nil {
		t.Fatalf("count p2 provider types: %v", err)
	}
	if p2ProviderTypeCount != 4 {
		t.Fatalf("expected 4 P2 provider types in registry, got %d", p2ProviderTypeCount)
	}
	if _, err := repository.CreateChannel(ctx, provider.CreateChannelParams{
		ProviderType:     provider.ProviderNtfy,
		Name:             "ntfy registry smoke",
		Enabled:          true,
		ConcurrencyLimit: 1,
		TimeoutMS:        1000,
	}); err != nil {
		t.Fatalf("create ntfy channel after registry seed: %v", err)
	}
	if err := repository.SeedProviderCapabilities(ctx, capabilities); err != nil {
		t.Fatalf("second seed provider capabilities: %v", err)
	}

	stale := capabilities[0]
	stale.ID = ""
	stale.MessageType = "legacy_stale"
	if err := repository.SeedProviderCapabilities(ctx, append(append([]provider.Capability{}, capabilities...), stale)); err != nil {
		t.Fatalf("seed provider capabilities with stale row: %v", err)
	}
	if err := repository.SeedProviderCapabilities(ctx, capabilities); err != nil {
		t.Fatalf("seed provider capabilities after stale row removed from defaults: %v", err)
	}

	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*)::integer FROM provider_capabilities`).Scan(&count); err != nil {
		t.Fatalf("count provider capabilities: %v", err)
	}
	if count != len(capabilities) {
		t.Fatalf("expected %d provider capabilities after repeated seed, got %d", len(capabilities), count)
	}
	var staleCount int
	if err := pool.QueryRow(ctx, `SELECT count(*)::integer FROM provider_capabilities WHERE provider_type = $1 AND message_type = $2`, stale.ProviderType, stale.MessageType).Scan(&staleCount); err != nil {
		t.Fatalf("count stale provider capability: %v", err)
	}
	if staleCount != 0 {
		t.Fatalf("expected stale capability %s/%s to be pruned, got %d", stale.ProviderType, stale.MessageType, staleCount)
	}

	listed, err := repository.ListProviderCapabilities(ctx)
	if err != nil {
		t.Fatalf("list provider capabilities: %v", err)
	}
	seeded := findDBCapability(t, listed, capabilities[0].ProviderType, capabilities[0].MessageType)
	if seeded.DisplayName != "Seeded Display v1" || seeded.Category != "seeded_category" {
		t.Fatalf("expected seeded display/category, got display=%q category=%q", seeded.DisplayName, seeded.Category)
	}
	if !seeded.CustomBodyAllowed {
		t.Fatal("expected custom body allowed to round trip")
	}
	if seeded.DefaultTimeoutMS != 9100 || seeded.DefaultConcurrencyLimit != 7 {
		t.Fatalf("expected timeout/concurrency defaults to round trip, got timeout=%d concurrency=%d", seeded.DefaultTimeoutMS, seeded.DefaultConcurrencyLimit)
	}
	assertRawJSONContains(t, seeded.TokenStrategy, "seeded")
	assertRawJSONContains(t, seeded.CredentialSchema, "properties")
	assertRawJSONContains(t, seeded.SendAPI, "method")

	capabilities[0].DisplayName = "Seeded Display v2"
	capabilities[0].DefaultTimeoutMS = 9200
	capabilities[0].DefaultConcurrencyLimit = 8
	if err := repository.SeedProviderCapabilities(ctx, capabilities); err != nil {
		t.Fatalf("third seed provider capabilities: %v", err)
	}
	updatedCapabilities, err := repository.ListProviderCapabilities(ctx)
	if err != nil {
		t.Fatalf("list provider capabilities after update: %v", err)
	}
	updated := findDBCapability(t, updatedCapabilities, capabilities[0].ProviderType, capabilities[0].MessageType)
	if updated.DisplayName != "Seeded Display v2" || updated.DefaultTimeoutMS != 9200 || updated.DefaultConcurrencyLimit != 8 {
		t.Fatalf("expected upserted metadata, got display=%q timeout=%d concurrency=%d", updated.DisplayName, updated.DefaultTimeoutMS, updated.DefaultConcurrencyLimit)
	}
}

func findDBCapability(t *testing.T, capabilities []provider.Capability, providerType provider.ProviderType, messageType string) provider.Capability {
	t.Helper()

	for _, capability := range capabilities {
		if capability.ProviderType == providerType && capability.MessageType == messageType {
			return capability
		}
	}
	t.Fatalf("capability %s/%s not found", providerType, messageType)
	return provider.Capability{}
}

func assertRawJSONContains(t *testing.T, raw json.RawMessage, needle string) {
	t.Helper()

	if !json.Valid(raw) {
		t.Fatalf("expected valid json, got %s", raw)
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatalf("decode json: %v", err)
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("re-encode json: %v", err)
	}
	if !strings.Contains(string(encoded), needle) {
		t.Fatalf("expected json %s to contain %q", encoded, needle)
	}
}
