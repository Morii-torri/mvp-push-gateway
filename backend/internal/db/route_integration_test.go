package db

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"mvp-push-gateway/backend/internal/route"
)

func TestRouteFlowEnabledUniquenessVersionsAndCounters(t *testing.T) {
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
	if _, err := pool.Exec(ctx, `INSERT INTO inbound_sources (id, code, name) VALUES ($1, $2, $3)`, "00000000-0000-0000-0000-000000000011", "orders", "Orders"); err != nil {
		t.Fatalf("insert source: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO templates (id, name, source_id)
		VALUES ($1, $2, $3)
	`, "00000000-0000-0000-0000-000000000410", "Alert Template", "00000000-0000-0000-0000-000000000011"); err != nil {
		t.Fatalf("insert template: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO template_versions (
			id,
			template_id,
			version_no,
			message_type,
			target_provider_type,
			template_body,
			message_body_schema,
			sample_payload,
			validation_status,
			validation_errors
		)
		VALUES (
			$1, $2, 1, 'text', 'webhook', 'body', '{}'::jsonb, '{}'::jsonb, 'valid', '[]'::jsonb
		)
	`, "00000000-0000-0000-0000-000000000401", "00000000-0000-0000-0000-000000000410"); err != nil {
		t.Fatalf("insert template version: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO delivery_channels (id, provider_type, name)
		VALUES ($1, 'webhook', 'Webhook A')
	`, "00000000-0000-0000-0000-000000000501"); err != nil {
		t.Fatalf("insert channel: %v", err)
	}

	first, err := repository.CreateFlow(ctx, route.CreateFlowParams{
		ID:       "00000000-0000-0000-0000-000000000101",
		SourceID: "00000000-0000-0000-0000-000000000011",
		Name:     "Primary",
		Enabled:  true,
		Mode:     route.ModeTable,
	})
	if err != nil {
		t.Fatalf("create first flow: %v", err)
	}
	if _, err := repository.CreateFlow(ctx, route.CreateFlowParams{
		ID:       "00000000-0000-0000-0000-000000000102",
		SourceID: first.SourceID,
		Name:     "Duplicate",
		Enabled:  true,
		Mode:     route.ModeTable,
	}); err == nil {
		t.Fatal("expected duplicate enabled flow to fail")
	} else if err != route.ErrEnabledFlowExists {
		t.Fatalf("expected ErrEnabledFlowExists, got %v", err)
	}

	draft, err := repository.GetDraft(ctx, first.ID)
	if err != nil {
		t.Fatalf("get draft: %v", err)
	}
	rules, err := repository.ReplaceRules(ctx, first.ID, draft.Version.ID, []route.Rule{
		{
			ID:            "00000000-0000-0000-0000-000000000201",
			FlowID:        first.ID,
			VersionID:     draft.Version.ID,
			RuleKey:       "00000000-0000-0000-0000-000000000301",
			SortOrder:     10,
			Name:          "Match title",
			ConditionTree: json.RawMessage(`{"operator":"equals","path":"payload.title","value":"critical"}`),
			Enabled:       true,
			Action: route.Action{
				TemplateVersionID: "00000000-0000-0000-0000-000000000401",
				ChannelIDs:        []string{"00000000-0000-0000-0000-000000000501"},
			},
		},
		{
			ID:            "00000000-0000-0000-0000-000000000202",
			FlowID:        first.ID,
			VersionID:     draft.Version.ID,
			RuleKey:       "00000000-0000-0000-0000-000000000302",
			SortOrder:     20,
			Name:          "Fallback",
			ConditionTree: json.RawMessage(`{"operator":"contains","path":"payload.title","value":"cri"}`),
			Enabled:       true,
			Action:        route.Action{},
		},
	})
	if err != nil {
		t.Fatalf("replace rules: %v", err)
	}
	if len(rules) != 2 || rules[0].HitCount != 0 || rules[1].HitCount != 0 {
		t.Fatalf("expected new rules to start at zero hit count, got %+v", rules)
	}

	now := time.Date(2026, 5, 8, 8, 0, 0, 0, time.UTC)
	if _, err := pool.Exec(ctx, `
		INSERT INTO route_rule_counters (flow_id, rule_key, hit_count, last_hit_at)
		VALUES ($1, $2, 99998, $3)
		ON CONFLICT (flow_id, rule_key)
		DO UPDATE SET hit_count = excluded.hit_count, last_hit_at = excluded.last_hit_at
	`, first.ID, "00000000-0000-0000-0000-000000000301", now); err != nil {
		t.Fatalf("seed counter: %v", err)
	}
	if err := repository.IncrementRuleCounter(ctx, first.ID, "00000000-0000-0000-0000-000000000301", now.Add(time.Minute)); err != nil {
		t.Fatalf("increment counter to 99999: %v", err)
	}
	if err := repository.IncrementRuleCounter(ctx, first.ID, "00000000-0000-0000-0000-000000000301", now.Add(2*time.Minute)); err != nil {
		t.Fatalf("increment counter cap: %v", err)
	}

	reordered, err := repository.ReorderRules(ctx, first.ID, draft.Version.ID, []string{
		"00000000-0000-0000-0000-000000000302",
		"00000000-0000-0000-0000-000000000301",
	})
	if err != nil {
		t.Fatalf("reorder rules: %v", err)
	}
	if reordered[1].HitCount != 99999 {
		t.Fatalf("expected counter to persist after reorder and cap at 99999, got %+v", reordered[1])
	}

	publishedV1, err := repository.Publish(ctx, route.PublishParams{
		FlowID:           first.ID,
		DraftVersionID:   draft.Version.ID,
		CompiledRules:    json.RawMessage(`{"execution_mode":"first_match_stop","rules":[{"rule_key":"00000000-0000-0000-0000-000000000302"},{"rule_key":"00000000-0000-0000-0000-000000000301"}]}`),
		ValidationStatus: "valid",
		ValidationErrors: json.RawMessage(`[]`),
		PublishedAt:      now.Add(3 * time.Minute),
	})
	if err != nil {
		t.Fatalf("publish v1: %v", err)
	}

	nextDraft, err := repository.GetDraft(ctx, first.ID)
	if err != nil {
		t.Fatalf("get next draft: %v", err)
	}
	edited, err := repository.ReplaceRules(ctx, first.ID, nextDraft.Version.ID, []route.Rule{
		{
			ID:            "00000000-0000-0000-0000-000000000203",
			FlowID:        first.ID,
			VersionID:     nextDraft.Version.ID,
			RuleKey:       "00000000-0000-0000-0000-000000000301",
			SortOrder:     10,
			Name:          "Match title v2",
			ConditionTree: json.RawMessage(`{"operator":"equals","path":"payload.title","value":"critical"}`),
			Enabled:       true,
		},
	})
	if err != nil {
		t.Fatalf("replace rules in draft v2: %v", err)
	}
	if len(edited) != 1 || edited[0].HitCount != 99999 {
		t.Fatalf("expected counter to survive edit into next draft, got %+v", edited)
	}

	publishedV2, err := repository.Publish(ctx, route.PublishParams{
		FlowID:           first.ID,
		DraftVersionID:   nextDraft.Version.ID,
		CompiledRules:    json.RawMessage(`{"execution_mode":"first_match_stop","rules":[{"rule_key":"00000000-0000-0000-0000-000000000301"}]}`),
		ValidationStatus: "valid",
		ValidationErrors: json.RawMessage(`[]`),
		PublishedAt:      now.Add(4 * time.Minute),
	})
	if err != nil {
		t.Fatalf("publish v2: %v", err)
	}

	if _, err := repository.ActivateVersion(ctx, first.ID, publishedV1.ID); err != nil {
		t.Fatalf("activate v1: %v", err)
	}
	afterV1, err := repository.GetFlow(ctx, first.ID)
	if err != nil {
		t.Fatalf("get flow after activating v1: %v", err)
	}
	if afterV1.CurrentVersionID != publishedV1.ID {
		t.Fatalf("expected current version %s after activate v1, got %s", publishedV1.ID, afterV1.CurrentVersionID)
	}

	if _, err := repository.ActivateVersion(ctx, first.ID, publishedV2.ID); err != nil {
		t.Fatalf("activate v2: %v", err)
	}
	afterV2, err := repository.GetFlow(ctx, first.ID)
	if err != nil {
		t.Fatalf("get flow after activating v2: %v", err)
	}
	if afterV2.CurrentVersionID != publishedV2.ID {
		t.Fatalf("expected current version %s after activate v2, got %s", publishedV2.ID, afterV2.CurrentVersionID)
	}
}
