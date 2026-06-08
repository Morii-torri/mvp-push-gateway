package db

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"mvp-push-gateway/backend/internal/route"
	msgtemplate "mvp-push-gateway/backend/internal/template"
)

func TestPublishTemplateVersionDoesNotRewriteRouteTargets(t *testing.T) {
	pool := openTemplateMigratedPool(t)
	defer pool.Close()

	ctx := context.Background()
	repository := NewRepository(pool)
	seedTemplateRouteDependency(t, ctx, pool)

	version, err := repository.PublishTemplateVersion(ctx, "00000000-0000-0000-0000-00000000a010", msgtemplate.PublishTemplateVersionParams{
		VersionInput: msgtemplate.VersionInput{
			MessageType:        "json",
			TargetProviderType: "webhook",
			TemplateBody:       `{"body":{"title":"{{ payload.title }}"}}`,
			MessageBodySchema:  json.RawMessage(`{"type":"object","required":["body"],"properties":{"body":{"type":"object"}}}`),
			SamplePayload:      json.RawMessage(`{"title":"新版本"}`),
		},
		CompiledPreview:  json.RawMessage(`{"rendered":"{\"body\":{\"title\":\"新版本\"}}"}`),
		UsedVariables:    []string{"payload.title"},
		ValidationStatus: "valid",
		ValidationErrors: json.RawMessage(`[]`),
	})
	if err != nil {
		t.Fatalf("publish template version: %v", err)
	}
	if version.ID == "00000000-0000-0000-0000-00000000a011" {
		t.Fatal("expected a newly created version id")
	}

	assertRouteTemplateVersion(t, ctx, pool, "route_action_targets", "00000000-0000-0000-0000-00000000a011")
	assertRouteTemplateVersion(t, ctx, pool, "route_actions", "00000000-0000-0000-0000-00000000a011")
}

func TestPlanningGetsReferencedTemplateVersion(t *testing.T) {
	pool := openTemplateMigratedPool(t)
	defer pool.Close()

	ctx := context.Background()
	repository := NewRepository(pool)
	seedTemplateRouteDependency(t, ctx, pool)

	version, err := repository.GetTemplateVersion(ctx, "00000000-0000-0000-0000-00000000a011")
	if err != nil {
		t.Fatalf("get referenced template version: %v", err)
	}
	if version.ID != "00000000-0000-0000-0000-00000000a011" || version.TemplateBody != `{"body":{"title":"old"}}` {
		t.Fatalf("expected referenced old version, got id=%s body=%s", version.ID, version.TemplateBody)
	}
}

func TestRouteValidationUsesReferencedTemplateVersionMetadata(t *testing.T) {
	pool := openTemplateMigratedPool(t)
	defer pool.Close()

	ctx := context.Background()
	repository := NewRepository(pool)
	seedTemplateRouteDependency(t, ctx, pool)

	errors, err := repository.ValidateRuleReferences(ctx, "flow-1", "version-1", []route.Rule{
		{
			RuleKey:       "00000000-0000-0000-0000-00000000a060",
			ConditionTree: json.RawMessage(`{"operator":"always"}`),
			Enabled:       true,
			Action: route.Action{
				Targets: []route.ActionTarget{
					{
						ChannelID:         "00000000-0000-0000-0000-00000000a030",
						TemplateVersionID: "00000000-0000-0000-0000-00000000a011",
						Enabled:           true,
						SortOrder:         10,
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("validate references: %v", err)
	}
	if len(errors) != 0 {
		t.Fatalf("expected referenced webhook/json version to validate, got %+v", errors)
	}
}

func seedTemplateRouteDependency(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()

	if _, err := pool.Exec(ctx, `
		INSERT INTO inbound_sources (id, code, name)
		VALUES ($1, 'template-deps', 'Template Deps')
	`, "00000000-0000-0000-0000-00000000a001"); err != nil {
		t.Fatalf("insert source: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO templates (id, name, source_id, enabled)
		VALUES ($1, 'Alert Template', $2, true)
	`, "00000000-0000-0000-0000-00000000a010", "00000000-0000-0000-0000-00000000a001"); err != nil {
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
			validation_errors,
			published_at
		)
		VALUES
			($1, $3, 1, 'json', 'webhook', '{"body":{"title":"old"}}', '{"type":"object","required":["body"],"properties":{"body":{"type":"object"}}}'::jsonb, '{}'::jsonb, 'valid', '[]'::jsonb, now()),
			($2, $3, 2, 'email', 'email', '{"subject":"current","body":"current","format":"text"}', '{"type":"object","required":["subject","body","format"]}'::jsonb, '{}'::jsonb, 'valid', '[]'::jsonb, now())
	`, "00000000-0000-0000-0000-00000000a011", "00000000-0000-0000-0000-00000000a012", "00000000-0000-0000-0000-00000000a010"); err != nil {
		t.Fatalf("insert template versions: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		UPDATE templates
		SET current_version_id = $2
		WHERE id = $1
	`, "00000000-0000-0000-0000-00000000a010", "00000000-0000-0000-0000-00000000a012"); err != nil {
		t.Fatalf("set current template version: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO delivery_channels (id, provider_type, name, enabled)
		VALUES ($1, 'webhook', 'Webhook A', true)
	`, "00000000-0000-0000-0000-00000000a030"); err != nil {
		t.Fatalf("insert channel: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO provider_capabilities (
			id,
			provider_type,
			message_type,
			message_schema,
			recipient_required,
			allow_no_recipient,
			recipient_location,
			recipient_requirement
		)
		VALUES ($1, 'webhook', 'json', '{"type":"object"}'::jsonb, false, true, 'none', 'none')
	`, "00000000-0000-0000-0000-00000000a031"); err != nil {
		t.Fatalf("insert provider capability: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO route_flows (id, source_id, name, enabled, mode)
		VALUES ($1, $2, 'Template Dependency Flow', false, 'table')
	`, "00000000-0000-0000-0000-00000000a040", "00000000-0000-0000-0000-00000000a001"); err != nil {
		t.Fatalf("insert route flow: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO route_versions (id, flow_id, version_no, validation_status, validation_errors, published_at)
		VALUES ($1, $2, 1, 'valid', '[]'::jsonb, now())
	`, "00000000-0000-0000-0000-00000000a041", "00000000-0000-0000-0000-00000000a040"); err != nil {
		t.Fatalf("insert route version: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO route_rules (id, flow_id, version_id, rule_key, sort_order, name, condition_tree, enabled)
		VALUES ($1, $2, $3, $4, 10, 'Rule A', '{}'::jsonb, true)
	`, "00000000-0000-0000-0000-00000000a050", "00000000-0000-0000-0000-00000000a040", "00000000-0000-0000-0000-00000000a041", "00000000-0000-0000-0000-00000000a060"); err != nil {
		t.Fatalf("insert route rule: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO route_actions (id, rule_id, template_version_id, channel_ids)
		VALUES ($1, $2, $3, ARRAY[$4]::uuid[])
	`, "00000000-0000-0000-0000-00000000a070", "00000000-0000-0000-0000-00000000a050", "00000000-0000-0000-0000-00000000a011", "00000000-0000-0000-0000-00000000a030"); err != nil {
		t.Fatalf("insert route action: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO route_action_targets (id, action_id, channel_id, template_version_id, enabled, sort_order)
		VALUES ($1, $2, $3, $4, true, 10)
	`, "00000000-0000-0000-0000-00000000a080", "00000000-0000-0000-0000-00000000a070", "00000000-0000-0000-0000-00000000a030", "00000000-0000-0000-0000-00000000a011"); err != nil {
		t.Fatalf("insert route action target: %v", err)
	}
}

func assertRouteTemplateVersion(t *testing.T, ctx context.Context, pool *pgxpool.Pool, table string, expected string) {
	t.Helper()

	var actual string
	if err := pool.QueryRow(ctx, "SELECT template_version_id::text FROM "+table+" WHERE id = $1", "00000000-0000-0000-0000-00000000a080").Scan(&actual); err != nil {
		if table == "route_actions" {
			err = pool.QueryRow(ctx, "SELECT template_version_id::text FROM route_actions WHERE id = $1", "00000000-0000-0000-0000-00000000a070").Scan(&actual)
		}
		if err != nil {
			t.Fatalf("query %s template version: %v", table, err)
		}
	}
	if actual != expected {
		t.Fatalf("expected %s to keep template version %s, got %s", table, expected, actual)
	}
}

func openTemplateMigratedPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dsn := os.Getenv("MGP_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("MGP_TEST_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	schemaName := createMigratedTestSchema(ctx, t, dsn)
	t.Cleanup(func() {
		dropTestSchema(schemaName)
	})

	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parse pool config: %v", err)
	}
	poolConfig.ConnConfig.RuntimeParams["search_path"] = schemaName

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		t.Fatalf("open test pool: %v", err)
	}
	return pool
}
