package db

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
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
			$1, $2, 2, 'text', 'webhook', 'body v2', '{}'::jsonb, '{}'::jsonb, 'valid', '[]'::jsonb
		)
	`, "00000000-0000-0000-0000-000000000402", "00000000-0000-0000-0000-000000000410"); err != nil {
		t.Fatalf("insert second template version: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO delivery_channels (id, provider_type, name)
		VALUES ($1, 'webhook', 'Webhook A')
	`, "00000000-0000-0000-0000-000000000501"); err != nil {
		t.Fatalf("insert channel: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO delivery_channels (id, provider_type, name)
		VALUES ($1, 'webhook', 'Webhook B')
	`, "00000000-0000-0000-0000-000000000502"); err != nil {
		t.Fatalf("insert second channel: %v", err)
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
	canvas, err := repository.UpdateCanvas(ctx, first.ID, json.RawMessage(`{"nodes":[{"id":"condition-1","data":{"kind":"condition","title":"条件组"}}],"edges":[]}`), route.ModeCanvas)
	if err != nil {
		t.Fatalf("update canvas snapshot: %v", err)
	}
	if !strings.Contains(string(canvas.Version.CanvasSnapshot), `"condition-1"`) {
		t.Fatalf("expected canvas snapshot to be persisted, got %s", canvas.Version.CanvasSnapshot)
	}
	flowAfterCanvas, err := repository.GetFlow(ctx, first.ID)
	if err != nil {
		t.Fatalf("get flow after canvas update: %v", err)
	}
	if flowAfterCanvas.Mode != route.ModeCanvas {
		t.Fatalf("expected canvas save to switch flow mode, got %s", flowAfterCanvas.Mode)
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
				Targets: []route.ActionTarget{
					{
						ID:                "00000000-0000-0000-0000-000000000601",
						ChannelID:         "00000000-0000-0000-0000-000000000501",
						TemplateVersionID: "00000000-0000-0000-0000-000000000401",
						Enabled:           true,
						SortOrder:         10,
					},
					{
						ID:                "00000000-0000-0000-0000-000000000602",
						ChannelID:         "00000000-0000-0000-0000-000000000502",
						TemplateVersionID: "00000000-0000-0000-0000-000000000402",
						Enabled:           true,
						SortOrder:         20,
					},
				},
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
	var persistedTargetRows int
	if err := pool.QueryRow(ctx, `
		SELECT count(*)::integer
		FROM route_action_targets
		WHERE action_id = $1
	`, rules[0].Action.ID).Scan(&persistedTargetRows); err != nil {
		t.Fatalf("count persisted route action targets: %v", err)
	}
	if persistedTargetRows != 2 {
		t.Fatalf("expected 2 persisted route action target rows, got %d", persistedTargetRows)
	}
	assertRouteActionTargets(t, rules[0].Action, []expectedRouteActionTarget{
		{ChannelID: "00000000-0000-0000-0000-000000000501", TemplateVersionID: "00000000-0000-0000-0000-000000000401", SortOrder: 10},
		{ChannelID: "00000000-0000-0000-0000-000000000502", TemplateVersionID: "00000000-0000-0000-0000-000000000402", SortOrder: 20},
	})
	if rules[0].Action.TemplateVersionID != "00000000-0000-0000-0000-000000000401" {
		t.Fatalf("expected compatibility template_version_id from first target, got %+v", rules[0].Action)
	}
	if len(rules[0].Action.ChannelIDs) != 2 || rules[0].Action.ChannelIDs[0] != "00000000-0000-0000-0000-000000000501" || rules[0].Action.ChannelIDs[1] != "00000000-0000-0000-0000-000000000502" {
		t.Fatalf("expected compatibility channel_ids from targets, got %+v", rules[0].Action.ChannelIDs)
	}

	draftAfterReplace, err := repository.GetDraft(ctx, first.ID)
	if err != nil {
		t.Fatalf("get draft after target replace: %v", err)
	}
	assertRouteActionTargets(t, findRouteRule(t, draftAfterReplace.Rules, "00000000-0000-0000-0000-000000000301").Action, []expectedRouteActionTarget{
		{ChannelID: "00000000-0000-0000-0000-000000000501", TemplateVersionID: "00000000-0000-0000-0000-000000000401", SortOrder: 10},
		{ChannelID: "00000000-0000-0000-0000-000000000502", TemplateVersionID: "00000000-0000-0000-0000-000000000402", SortOrder: 20},
	})

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
	routePlanV1, err := repository.LoadRoutePlan(ctx, first.SourceID, publishedV1.ID)
	if err != nil {
		t.Fatalf("load published route plan v1: %v", err)
	}
	assertRouteActionTargets(t, findRouteRule(t, routePlanV1.Rules, "00000000-0000-0000-0000-000000000301").Action, []expectedRouteActionTarget{
		{ChannelID: "00000000-0000-0000-0000-000000000501", TemplateVersionID: "00000000-0000-0000-0000-000000000401", SortOrder: 10},
		{ChannelID: "00000000-0000-0000-0000-000000000502", TemplateVersionID: "00000000-0000-0000-0000-000000000402", SortOrder: 20},
	})

	nextDraft, err := repository.GetDraft(ctx, first.ID)
	if err != nil {
		t.Fatalf("get next draft: %v", err)
	}
	assertRouteActionTargets(t, findRouteRule(t, nextDraft.Rules, "00000000-0000-0000-0000-000000000301").Action, []expectedRouteActionTarget{
		{ChannelID: "00000000-0000-0000-0000-000000000501", TemplateVersionID: "00000000-0000-0000-0000-000000000401", SortOrder: 10},
		{ChannelID: "00000000-0000-0000-0000-000000000502", TemplateVersionID: "00000000-0000-0000-0000-000000000402", SortOrder: 20},
	})
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

func TestDeleteRouteVersionRemovesOnlyHistoricalPublishedVersion(t *testing.T) {
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
	if _, err := pool.Exec(ctx, `INSERT INTO inbound_sources (id, code, name) VALUES ($1, $2, $3)`, "00000000-0000-0000-0000-00000000d010", "route-delete", "Route Delete"); err != nil {
		t.Fatalf("insert source: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO route_flows (id, source_id, name, enabled, mode)
		VALUES ($1, $2, 'Route Delete Flow', false, 'table')
	`, "00000000-0000-0000-0000-00000000d020", "00000000-0000-0000-0000-00000000d010"); err != nil {
		t.Fatalf("insert flow: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO route_versions (id, flow_id, version_no, validation_status, validation_errors, published_at)
		VALUES
			($1, $2, 1, 'valid', '[]'::jsonb, now()),
			($3, $2, 2, 'valid', '[]'::jsonb, now())
	`, "00000000-0000-0000-0000-00000000d021", "00000000-0000-0000-0000-00000000d020", "00000000-0000-0000-0000-00000000d022"); err != nil {
		t.Fatalf("insert versions: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		UPDATE route_flows
		SET current_version_id = $2
		WHERE id = $1
	`, "00000000-0000-0000-0000-00000000d020", "00000000-0000-0000-0000-00000000d021"); err != nil {
		t.Fatalf("set current version: %v", err)
	}

	if err := repository.DeleteVersion(ctx, "00000000-0000-0000-0000-00000000d020", "00000000-0000-0000-0000-00000000d022"); err != nil {
		t.Fatalf("delete historical version: %v", err)
	}
	versions, err := repository.ListVersions(ctx, "00000000-0000-0000-0000-00000000d020")
	if err != nil {
		t.Fatalf("list versions after delete: %v", err)
	}
	if len(versions) != 1 || versions[0].ID != "00000000-0000-0000-0000-00000000d021" {
		t.Fatalf("expected only current version to remain, got %+v", versions)
	}
	if err := repository.DeleteVersion(ctx, "00000000-0000-0000-0000-00000000d020", "00000000-0000-0000-0000-00000000d021"); !errors.Is(err, route.ErrInvalidInput) {
		t.Fatalf("expected current version delete to return invalid input, got %v", err)
	}
}

func TestRoutePublishNotifiesRuntimeCacheInvalidation(t *testing.T) {
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

	listener, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire listener connection: %v", err)
	}
	defer listener.Release()
	if _, err := listener.Exec(ctx, `LISTEN `+RouteRuntimeChangeChannel); err != nil {
		t.Fatalf("listen route runtime channel: %v", err)
	}

	repository := NewRepository(pool)
	sourceID := "00000000-0000-0000-0000-00000000e010"
	flowID := "00000000-0000-0000-0000-00000000e020"
	draftID := "00000000-0000-0000-0000-00000000e030"
	if _, err := pool.Exec(ctx, `INSERT INTO inbound_sources (id, code, name) VALUES ($1, 'notify-route', 'Notify Route')`, sourceID); err != nil {
		t.Fatalf("insert source: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO route_flows (id, source_id, name, enabled, mode)
		VALUES ($1, $2, 'Notify Flow', true, 'table')
	`, flowID, sourceID); err != nil {
		t.Fatalf("insert flow: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO route_versions (id, flow_id, version_no, validation_status, validation_errors)
		VALUES ($1, $2, 1, 'draft', '[]'::jsonb)
	`, draftID, flowID); err != nil {
		t.Fatalf("insert draft: %v", err)
	}

	if _, err := repository.Publish(ctx, route.PublishParams{
		FlowID:           flowID,
		DraftVersionID:   draftID,
		CompiledRules:    json.RawMessage(`{"rules":[]}`),
		ValidationStatus: "valid",
		ValidationErrors: json.RawMessage(`[]`),
		PublishedAt:      time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("publish route: %v", err)
	}

	waitCtx, waitCancel := context.WithTimeout(ctx, time.Second)
	defer waitCancel()
	notification, err := listener.Conn().WaitForNotification(waitCtx)
	if err != nil {
		t.Fatalf("wait for route runtime notification: %v", err)
	}
	if notification.Channel != RouteRuntimeChangeChannel || notification.Payload != sourceID {
		t.Fatalf("unexpected route runtime notification: channel=%q payload=%q", notification.Channel, notification.Payload)
	}
}

type expectedRouteActionTarget struct {
	ChannelID         string
	TemplateVersionID string
	SortOrder         int
}

func assertRouteActionTargets(t *testing.T, action route.Action, expected []expectedRouteActionTarget) {
	t.Helper()

	if len(action.Targets) != len(expected) {
		t.Fatalf("expected %d action targets, got %+v", len(expected), action.Targets)
	}
	for index, target := range action.Targets {
		if target.ChannelID != expected[index].ChannelID || target.TemplateVersionID != expected[index].TemplateVersionID || target.SortOrder != expected[index].SortOrder || !target.Enabled {
			t.Fatalf("unexpected target at index %d: %+v", index, target)
		}
		if target.ID == "" || target.ActionID == "" || target.CreatedAt.IsZero() {
			t.Fatalf("expected target identity and created_at to be loaded, got %+v", target)
		}
	}
}

func findRouteRule(t *testing.T, rules []route.Rule, ruleKey string) route.Rule {
	t.Helper()

	for _, item := range rules {
		if item.RuleKey == ruleKey {
			return item
		}
	}
	t.Fatalf("route rule %s not found in %+v", ruleKey, rules)
	return route.Rule{}
}
