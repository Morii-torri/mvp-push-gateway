package planning_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	dbrepo "mvp-push-gateway/backend/internal/db"
	"mvp-push-gateway/backend/internal/planning"
	"mvp-push-gateway/backend/internal/provider"
	"mvp-push-gateway/backend/internal/queue"
	"mvp-push-gateway/backend/internal/route"
	"mvp-push-gateway/backend/internal/source"
	msgtemplate "mvp-push-gateway/backend/internal/template"
)

func TestWorkerPlansLegacyActionIntoDeliveryAttemptAndSendJob(t *testing.T) {
	pool := openMigratedPool(t)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	repository := dbrepo.NewRepository(pool)
	if err := repository.SeedProviderCapabilities(ctx, provider.DefaultCapabilities()); err != nil {
		t.Fatalf("seed provider capabilities: %v", err)
	}

	sourceService := source.NewService(repository, source.WithTraceIDGenerator(func() string { return "trace-planning-happy" }))
	templateService := msgtemplate.NewService(repository)
	routeService := route.NewService(repository)

	inboundSource, err := sourceService.CreateSource(ctx, source.CreateSourceInput{
		Code:       "orders",
		Name:       "Orders",
		Enabled:    true,
		AuthMode:   source.AuthModeNone,
		CompatMode: "standard",
	})
	if err != nil {
		t.Fatalf("create source: %v", err)
	}
	channel, err := repository.CreateChannel(ctx, provider.CreateChannelParams{
		ProviderType:     provider.ProviderWebhook,
		Name:             "Webhook",
		Enabled:          true,
		SendConfig:       json.RawMessage(`{"method":"POST","url":"https://example.test/send","recipient":{"location":"none"}}`),
		RateLimitConfig:  json.RawMessage(`{}`),
		ConcurrencyLimit: 1,
		TimeoutMS:        1000,
		RetryPolicy:      json.RawMessage(`{"max_attempts":2}`),
		DeadLetterPolicy: json.RawMessage(`{}`),
	})
	if err != nil {
		t.Fatalf("create channel: %v", err)
	}
	template, err := templateService.CreateTemplate(ctx, msgtemplate.TemplateInput{
		Name:     "Alert Template",
		SourceID: inboundSource.ID,
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("create template: %v", err)
	}
	templateVersion, err := templateService.Publish(ctx, template.ID, msgtemplate.VersionInput{
		MessageType:        "json",
		TargetProviderType: string(provider.ProviderWebhook),
		TemplateBody:       `{"title":"{{ payload.title }}","severity":"{{ payload.severity }}"}`,
		MessageBodySchema:  json.RawMessage(`{"type":"object"}`),
		SamplePayload:      json.RawMessage(`{"title":"critical","severity":"high"}`),
	})
	if err != nil {
		t.Fatalf("publish template: %v", err)
	}

	groupID := "00000000-0000-0000-0000-000000021001"
	if _, err := pool.Exec(ctx, `
		INSERT INTO match_groups (id, name, group_type, enabled)
		VALUES ($1, 'Severities', 'value', true)
	`, groupID); err != nil {
		t.Fatalf("insert match group: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO match_group_items (id, group_id, value, value_type)
		VALUES ($1, $2, 'high', 'text')
	`, "00000000-0000-0000-0000-000000021002", groupID); err != nil {
		t.Fatalf("insert match group item: %v", err)
	}

	flow, err := routeService.CreateFlow(ctx, route.CreateFlowInput{
		SourceID: inboundSource.ID,
		Name:     "Orders Flow",
		Enabled:  true,
		Mode:     route.ModeTable,
	})
	if err != nil {
		t.Fatalf("create route flow: %v", err)
	}
	ruleKey := "00000000-0000-0000-0000-000000021003"
	if _, err := routeService.SaveRules(ctx, flow.ID, route.SaveRulesInput{Rules: []route.RuleInput{{
		RuleKey:   ruleKey,
		SortOrder: 10,
		Name:      "High severity",
		ConditionTree: json.RawMessage(`{
			"operator":"in_match_group",
			"path":"payload.severity",
			"match_group_id":"` + groupID + `"
		}`),
		Enabled: true,
		Action: route.ActionInput{
			TemplateVersionID: templateVersion.ID,
			ChannelIDs:        []string{channel.ID},
			RecipientStrategy: json.RawMessage(`{}`),
			SendDedupeConfig:  json.RawMessage(`{"enabled":true,"key_path":"payload.order_id","ttl_seconds":600}`),
		},
	}}}); err != nil {
		t.Fatalf("save route rules: %v", err)
	}
	if _, err := routeService.Publish(ctx, flow.ID); err != nil {
		t.Fatalf("publish route: %v", err)
	}

	if _, err := sourceService.Ingest(ctx, source.IngestInput{
		SourceCode: "orders",
		Method:     "POST",
		Path:       "/api/v1/ingest/orders",
		Body:       []byte(`{"title":"critical","severity":"high","order_id":"A-1001"}`),
	}); err != nil {
		t.Fatalf("ingest inbound: %v", err)
	}

	worker := planning.NewWorker(repository, planning.WithWorkerID("planner-integration"))
	processed, err := worker.ProcessBatch(ctx, 1)
	if err != nil {
		t.Fatalf("process planning batch: %v", err)
	}
	if processed != 1 {
		t.Fatalf("expected one route_plan job processed, got %d", processed)
	}

	var messageID string
	var status string
	var matchedFlowID string
	var matchedRuleIDs []string
	if err := pool.QueryRow(ctx, `
		SELECT id::text, status, COALESCE(matched_flow_id::text, ''), matched_rule_ids::text[]
		FROM message_records
		WHERE trace_id = 'trace-planning-happy'
	`).Scan(&messageID, &status, &matchedFlowID, &matchedRuleIDs); err != nil {
		t.Fatalf("query planned message: %v", err)
	}
	if status != "planned" || matchedFlowID != flow.ID || len(matchedRuleIDs) != 1 || matchedRuleIDs[0] != ruleKey {
		t.Fatalf("unexpected planned message state: status=%s flow=%s rules=%v", status, matchedFlowID, matchedRuleIDs)
	}

	var attemptID string
	var attemptBody json.RawMessage
	if err := pool.QueryRow(ctx, `
		SELECT attempt.id::text, job.payload
		FROM delivery_attempts AS attempt
		JOIN jobs AS job ON (job.payload->>'delivery_attempt_id')::uuid = attempt.id
		WHERE attempt.message_id = $1
			AND job.type = 'send_message'
			AND job.status = 'queued'
	`, messageID).Scan(&attemptID, &attemptBody); err != nil {
		t.Fatalf("query delivery attempt and send job: %v", err)
	}
	var sendPayload map[string]any
	if err := json.Unmarshal(attemptBody, &sendPayload); err != nil {
		t.Fatalf("decode send job payload: %v", err)
	}
	if sendPayload["delivery_attempt_id"] != attemptID || sendPayload["dedupe_key"] != "A-1001" {
		t.Fatalf("unexpected send job payload: %+v", sendPayload)
	}
	body, ok := sendPayload["body"].(map[string]any)
	if !ok || body["title"] != "critical" || body["severity"] != "high" {
		t.Fatalf("expected rendered template body in send job payload, got %+v", sendPayload)
	}

	var hitCount int
	if err := pool.QueryRow(ctx, `
		SELECT hit_count FROM route_rule_counters WHERE flow_id = $1 AND rule_key = $2::uuid
	`, flow.ID, ruleKey).Scan(&hitCount); err != nil {
		t.Fatalf("query route hit counter: %v", err)
	}
	if hitCount != 1 {
		t.Fatalf("expected route hit counter to be 1, got %d", hitCount)
	}
}

func TestWorkerFansOutActionTargetsIntoDeliveryAttemptsAndSendJobs(t *testing.T) {
	pool := openMigratedPool(t)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	repository := dbrepo.NewRepository(pool)
	if err := repository.SeedProviderCapabilities(ctx, provider.DefaultCapabilities()); err != nil {
		t.Fatalf("seed provider capabilities: %v", err)
	}

	sourceService := source.NewService(repository, source.WithTraceIDGenerator(func() string { return "trace-target-fanout" }))
	templateService := msgtemplate.NewService(repository)
	routeService := route.NewService(repository)

	inboundSource, err := sourceService.CreateSource(ctx, source.CreateSourceInput{
		Code:       "targetfanout",
		Name:       "Target Fanout",
		Enabled:    true,
		AuthMode:   source.AuthModeNone,
		CompatMode: "standard",
	})
	if err != nil {
		t.Fatalf("create source: %v", err)
	}
	webhookChannel, err := repository.CreateChannel(ctx, provider.CreateChannelParams{
		ProviderType:     provider.ProviderWebhook,
		Name:             "Webhook",
		Enabled:          true,
		SendConfig:       json.RawMessage(`{"method":"POST","url":"https://example.test/webhook","recipient":{"location":"none"}}`),
		RateLimitConfig:  json.RawMessage(`{}`),
		ConcurrencyLimit: 1,
		TimeoutMS:        1000,
		RetryPolicy:      json.RawMessage(`{"max_attempts":2}`),
		DeadLetterPolicy: json.RawMessage(`{}`),
	})
	if err != nil {
		t.Fatalf("create webhook channel: %v", err)
	}
	customChannel, err := repository.CreateChannel(ctx, provider.CreateChannelParams{
		ProviderType:     provider.ProviderCustomToken,
		Name:             "Custom Token",
		Enabled:          true,
		SendConfig:       json.RawMessage(`{"method":"POST","url":"https://example.test/custom","recipient":{"location":"body","path":"recipient"}}`),
		RateLimitConfig:  json.RawMessage(`{}`),
		ConcurrencyLimit: 1,
		TimeoutMS:        1000,
		RetryPolicy:      json.RawMessage(`{"max_attempts":4}`),
		DeadLetterPolicy: json.RawMessage(`{}`),
	})
	if err != nil {
		t.Fatalf("create custom token channel: %v", err)
	}

	webhookTemplate, err := templateService.CreateTemplate(ctx, msgtemplate.TemplateInput{Name: "Webhook Template", SourceID: inboundSource.ID, Enabled: true})
	if err != nil {
		t.Fatalf("create webhook template: %v", err)
	}
	webhookVersion, err := templateService.Publish(ctx, webhookTemplate.ID, msgtemplate.VersionInput{
		MessageType:        "json",
		TargetProviderType: string(provider.ProviderWebhook),
		TemplateBody:       `{"target":"webhook","title":"{{ payload.title }}"}`,
		MessageBodySchema:  json.RawMessage(`{"type":"object"}`),
		SamplePayload:      json.RawMessage(`{"title":"critical"}`),
	})
	if err != nil {
		t.Fatalf("publish webhook template: %v", err)
	}
	customTemplate, err := templateService.CreateTemplate(ctx, msgtemplate.TemplateInput{Name: "Custom Template", SourceID: inboundSource.ID, Enabled: true})
	if err != nil {
		t.Fatalf("create custom template: %v", err)
	}
	customVersion, err := templateService.Publish(ctx, customTemplate.ID, msgtemplate.VersionInput{
		MessageType:        "json",
		TargetProviderType: string(provider.ProviderCustomToken),
		TemplateBody:       `{"target":"custom","ticket":"{{ payload.ticket }}"}`,
		MessageBodySchema:  json.RawMessage(`{"type":"object"}`),
		SamplePayload:      json.RawMessage(`{"ticket":"T-1001"}`),
	})
	if err != nil {
		t.Fatalf("publish custom template: %v", err)
	}

	flow, err := routeService.CreateFlow(ctx, route.CreateFlowInput{
		SourceID: inboundSource.ID,
		Name:     "Target Fanout Flow",
		Enabled:  true,
		Mode:     route.ModeTable,
	})
	if err != nil {
		t.Fatalf("create route flow: %v", err)
	}
	ruleKey := "00000000-0000-0000-0000-000000024001"
	if _, err := routeService.SaveRules(ctx, flow.ID, route.SaveRulesInput{Rules: []route.RuleInput{{
		RuleKey:       ruleKey,
		SortOrder:     10,
		Name:          "Always fan out",
		ConditionTree: json.RawMessage(`{"operator":"always"}`),
		Enabled:       true,
		Action: route.ActionInput{
			Targets: []route.ActionTargetInput{
				{ChannelID: webhookChannel.ID, TemplateVersionID: webhookVersion.ID, Enabled: true},
				{ChannelID: customChannel.ID, TemplateVersionID: customVersion.ID, Enabled: true},
			},
			RecipientStrategy: json.RawMessage(`{}`),
			SendDedupeConfig:  json.RawMessage(`{"enabled":true,"key_path":"payload.ticket","ttl_seconds":600}`),
		},
	}}}); err != nil {
		t.Fatalf("save route rules: %v", err)
	}
	if _, err := routeService.Publish(ctx, flow.ID); err != nil {
		t.Fatalf("publish route: %v", err)
	}

	if _, err := sourceService.Ingest(ctx, source.IngestInput{
		SourceCode: "targetfanout",
		Method:     "POST",
		Path:       "/api/v1/ingest/targetfanout",
		Body:       []byte(`{"title":"critical","ticket":"T-1001"}`),
	}); err != nil {
		t.Fatalf("ingest inbound: %v", err)
	}

	worker := planning.NewWorker(repository, planning.WithWorkerID("planner-target-fanout"))
	processed, err := worker.ProcessBatch(ctx, 1)
	if err != nil {
		t.Fatalf("process planning batch: %v", err)
	}
	if processed != 1 {
		t.Fatalf("expected one route_plan job processed, got %d", processed)
	}

	var messageID string
	var status string
	if err := pool.QueryRow(ctx, `
		SELECT id::text, status
		FROM message_records
		WHERE trace_id = 'trace-target-fanout'
	`).Scan(&messageID, &status); err != nil {
		t.Fatalf("query planned message: %v", err)
	}
	if status != "planned" {
		t.Fatalf("expected planned message, got %s", status)
	}

	rows, err := pool.Query(ctx, `
		SELECT
			attempt.channel_id::text,
			attempt.template_version_id::text,
			job.payload
		FROM delivery_attempts AS attempt
		JOIN jobs AS job ON (job.payload->>'delivery_attempt_id')::uuid = attempt.id
		WHERE attempt.message_id = $1
			AND job.type = 'send_message'
			AND job.status = 'queued'
		ORDER BY attempt.channel_id::text
	`, messageID)
	if err != nil {
		t.Fatalf("query delivery attempts and send jobs: %v", err)
	}
	defer rows.Close()

	type plannedTarget struct {
		channelID         string
		templateVersionID string
		payload           map[string]any
	}
	planned := map[string]plannedTarget{}
	for rows.Next() {
		var channelID string
		var templateVersionID string
		var rawPayload json.RawMessage
		if err := rows.Scan(&channelID, &templateVersionID, &rawPayload); err != nil {
			t.Fatalf("scan send job: %v", err)
		}
		var decoded map[string]any
		if err := json.Unmarshal(rawPayload, &decoded); err != nil {
			t.Fatalf("decode send job payload: %v", err)
		}
		planned[templateVersionID] = plannedTarget{channelID: channelID, templateVersionID: templateVersionID, payload: decoded}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("send job rows: %v", err)
	}
	if len(planned) != 2 {
		t.Fatalf("expected two delivery attempts, got %d: %+v", len(planned), planned)
	}

	webhookTarget, ok := planned[webhookVersion.ID]
	if !ok || webhookTarget.channelID != webhookChannel.ID {
		t.Fatalf("missing webhook target attempt: %+v", planned)
	}
	webhookBody, ok := webhookTarget.payload["body"].(map[string]any)
	if !ok || webhookBody["target"] != "webhook" || webhookBody["title"] != "critical" {
		t.Fatalf("expected webhook rendered body, got %+v", webhookTarget.payload)
	}
	customTarget, ok := planned[customVersion.ID]
	if !ok || customTarget.channelID != customChannel.ID {
		t.Fatalf("missing custom target attempt: %+v", planned)
	}
	customBody, ok := customTarget.payload["body"].(map[string]any)
	if !ok || customBody["target"] != "custom" || customBody["ticket"] != "T-1001" {
		t.Fatalf("expected custom rendered body, got %+v", customTarget.payload)
	}
}

func TestWorkerMarksBusinessPlanningFailuresDone(t *testing.T) {
	t.Run("no route marks message no_route", func(t *testing.T) {
		pool := openMigratedPool(t)
		defer pool.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		repository := dbrepo.NewRepository(pool)
		sourceService := source.NewService(repository, source.WithTraceIDGenerator(func() string { return "trace-no-route" }))
		if _, err := sourceService.CreateSource(ctx, source.CreateSourceInput{Code: "noroute", Name: "No Route", Enabled: true, AuthMode: source.AuthModeNone, CompatMode: "standard_json"}); err != nil {
			t.Fatalf("create source: %v", err)
		}
		if _, err := sourceService.Ingest(ctx, source.IngestInput{SourceCode: "noroute", Method: "POST", Path: "/api/v1/ingest/noroute", Body: []byte(`{"title":"x"}`)}); err != nil {
			t.Fatalf("ingest: %v", err)
		}
		worker := planning.NewWorker(repository, planning.WithWorkerID("planner-no-route"))
		if _, err := worker.ProcessBatch(ctx, 1); err != nil {
			t.Fatalf("process no-route batch: %v", err)
		}
		assertMessageAndRouteJob(t, ctx, pool, "trace-no-route", "no_route", "MGP-PLAN-NOROUTE")
	})

	t.Run("missing template marks message failed", func(t *testing.T) {
		pool := openMigratedPool(t)
		defer pool.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		repository := dbrepo.NewRepository(pool)
		if err := repository.SeedProviderCapabilities(ctx, provider.DefaultCapabilities()); err != nil {
			t.Fatalf("seed provider capabilities: %v", err)
		}
		sourceService := source.NewService(repository, source.WithTraceIDGenerator(func() string { return "trace-template-error" }))
		templateService := msgtemplate.NewService(repository)
		inboundSource, err := sourceService.CreateSource(ctx, source.CreateSourceInput{Code: "tplerror", Name: "Template Error", Enabled: true, AuthMode: source.AuthModeNone, CompatMode: "standard_json"})
		if err != nil {
			t.Fatalf("create source: %v", err)
		}
		channel, err := repository.CreateChannel(ctx, provider.CreateChannelParams{
			ProviderType:     provider.ProviderWebhook,
			Name:             "Webhook",
			Enabled:          true,
			SendConfig:       json.RawMessage(`{"method":"POST","url":"https://example.test/send","recipient":{"location":"none"}}`),
			RateLimitConfig:  json.RawMessage(`{}`),
			ConcurrencyLimit: 1,
			TimeoutMS:        1000,
			RetryPolicy:      json.RawMessage(`{"max_attempts":2}`),
			DeadLetterPolicy: json.RawMessage(`{}`),
		})
		if err != nil {
			t.Fatalf("create channel: %v", err)
		}
		template, err := templateService.CreateTemplate(ctx, msgtemplate.TemplateInput{Name: "Missing Body", SourceID: inboundSource.ID, Enabled: true})
		if err != nil {
			t.Fatalf("create template: %v", err)
		}
		templateVersion, err := templateService.Publish(ctx, template.ID, msgtemplate.VersionInput{
			MessageType:        "json",
			TargetProviderType: string(provider.ProviderWebhook),
			TemplateBody:       `{"title":"{{ payload.title }}"}`,
			MessageBodySchema:  json.RawMessage(`{"type":"object"}`),
			SamplePayload:      json.RawMessage(`{"title":"x"}`),
		})
		if err != nil {
			t.Fatalf("publish template: %v", err)
		}
		createPublishedRoute(t, ctx, repository, inboundSource.ID, "00000000-0000-0000-0000-000000022001", templateVersion.ID, channel.ID, json.RawMessage(`{}`))
		if _, err := pool.Exec(ctx, `UPDATE template_versions SET validation_status = 'invalid' WHERE id = $1`, templateVersion.ID); err != nil {
			t.Fatalf("invalidate template version: %v", err)
		}
		if _, err := sourceService.Ingest(ctx, source.IngestInput{SourceCode: "tplerror", Method: "POST", Path: "/api/v1/ingest/tplerror", Body: []byte(`{"title":"x"}`)}); err != nil {
			t.Fatalf("ingest: %v", err)
		}
		worker := planning.NewWorker(repository, planning.WithWorkerID("planner-template-error"))
		if _, err := worker.ProcessBatch(ctx, 1); err != nil {
			t.Fatalf("process template-error batch: %v", err)
		}
		assertMessageAndRouteJob(t, ctx, pool, "trace-template-error", "failed", "MGP-PLAN-TPL")
	})

	t.Run("missing payload recipient marks message failed", func(t *testing.T) {
		pool := openMigratedPool(t)
		defer pool.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		repository := dbrepo.NewRepository(pool)
		if err := repository.SeedProviderCapabilities(ctx, provider.DefaultCapabilities()); err != nil {
			t.Fatalf("seed provider capabilities: %v", err)
		}
		sourceService := source.NewService(repository, source.WithTraceIDGenerator(func() string { return "trace-recipient-error" }))
		templateService := msgtemplate.NewService(repository)
		inboundSource, err := sourceService.CreateSource(ctx, source.CreateSourceInput{Code: "rcpterror", Name: "Recipient Error", Enabled: true, AuthMode: source.AuthModeNone, CompatMode: "standard_json"})
		if err != nil {
			t.Fatalf("create source: %v", err)
		}
		channel, err := repository.CreateChannel(ctx, provider.CreateChannelParams{
			ProviderType:     provider.ProviderCustomToken,
			Name:             "Custom Token",
			Enabled:          true,
			SendConfig:       json.RawMessage(`{"method":"POST","url":"https://example.test/send","recipient":{"location":"body","path":"recipient"}}`),
			RateLimitConfig:  json.RawMessage(`{}`),
			ConcurrencyLimit: 1,
			TimeoutMS:        1000,
			RetryPolicy:      json.RawMessage(`{"max_attempts":2}`),
			DeadLetterPolicy: json.RawMessage(`{}`),
		})
		if err != nil {
			t.Fatalf("create channel: %v", err)
		}
		template, err := templateService.CreateTemplate(ctx, msgtemplate.TemplateInput{Name: "Body", SourceID: inboundSource.ID, Enabled: true})
		if err != nil {
			t.Fatalf("create template: %v", err)
		}
		templateVersion, err := templateService.Publish(ctx, template.ID, msgtemplate.VersionInput{
			MessageType:        "json",
			TargetProviderType: string(provider.ProviderCustomToken),
			TemplateBody:       `{"message":"{{ payload.title }}"}`,
			MessageBodySchema:  json.RawMessage(`{"type":"object"}`),
			SamplePayload:      json.RawMessage(`{"title":"x"}`),
		})
		if err != nil {
			t.Fatalf("publish template: %v", err)
		}
		createPublishedRoute(t, ctx, repository, inboundSource.ID, "00000000-0000-0000-0000-000000023001", templateVersion.ID, channel.ID, json.RawMessage(`{"mode":"payload","path":"payload.to_user"}`))
		if _, err := sourceService.Ingest(ctx, source.IngestInput{SourceCode: "rcpterror", Method: "POST", Path: "/api/v1/ingest/rcpterror", Body: []byte(`{"title":"x"}`)}); err != nil {
			t.Fatalf("ingest: %v", err)
		}
		worker := planning.NewWorker(repository, planning.WithWorkerID("planner-recipient-error"))
		if _, err := worker.ProcessBatch(ctx, 1); err != nil {
			t.Fatalf("process recipient-error batch: %v", err)
		}
		assertMessageAndRouteJob(t, ctx, pool, "trace-recipient-error", "failed", "MGP-PLAN-RCPT")
	})

	t.Run("template provider mismatch marks channel planning failure", func(t *testing.T) {
		pool := openMigratedPool(t)
		defer pool.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		repository := dbrepo.NewRepository(pool)
		if err := repository.SeedProviderCapabilities(ctx, provider.DefaultCapabilities()); err != nil {
			t.Fatalf("seed provider capabilities: %v", err)
		}
		sourceService := source.NewService(repository, source.WithTraceIDGenerator(func() string { return "trace-provider-mismatch" }))
		templateService := msgtemplate.NewService(repository)
		routeService := route.NewService(repository)
		inboundSource, err := sourceService.CreateSource(ctx, source.CreateSourceInput{Code: "providermismatch", Name: "Provider Mismatch", Enabled: true, AuthMode: source.AuthModeNone, CompatMode: "standard_json"})
		if err != nil {
			t.Fatalf("create source: %v", err)
		}
		channel, err := repository.CreateChannel(ctx, provider.CreateChannelParams{
			ProviderType:     provider.ProviderWeCom,
			Name:             "WeCom",
			Enabled:          true,
			SendConfig:       json.RawMessage(`{"base_url":"https://qyapi.weixin.qq.com","safe":0}`),
			RateLimitConfig:  json.RawMessage(`{}`),
			ConcurrencyLimit: 1,
			TimeoutMS:        1000,
			RetryPolicy:      json.RawMessage(`{"max_attempts":2}`),
			DeadLetterPolicy: json.RawMessage(`{}`),
		})
		if err != nil {
			t.Fatalf("create channel: %v", err)
		}
		template, err := templateService.CreateTemplate(ctx, msgtemplate.TemplateInput{Name: "DingTalk Body", SourceID: inboundSource.ID, Enabled: true})
		if err != nil {
			t.Fatalf("create template: %v", err)
		}
		templateVersion, err := templateService.Publish(ctx, template.ID, msgtemplate.VersionInput{
			MessageType:        "text",
			TargetProviderType: string(provider.ProviderDingTalk),
			TemplateBody:       `{"content":"{{ payload.title }}"}`,
			MessageBodySchema:  json.RawMessage(`{"type":"object"}`),
			SamplePayload:      json.RawMessage(`{"title":"x"}`),
		})
		if err != nil {
			t.Fatalf("publish template: %v", err)
		}
		flow, err := routeService.CreateFlow(ctx, route.CreateFlowInput{SourceID: inboundSource.ID, Name: "Provider Mismatch Flow", Enabled: true, Mode: route.ModeTable})
		if err != nil {
			t.Fatalf("create route flow: %v", err)
		}
		if _, err := routeService.SaveRules(ctx, flow.ID, route.SaveRulesInput{Rules: []route.RuleInput{{
			RuleKey:       "00000000-0000-0000-0000-000000025001",
			SortOrder:     10,
			Name:          "Always mismatch",
			ConditionTree: json.RawMessage(`{"operator":"always"}`),
			Enabled:       true,
			Action: route.ActionInput{
				Targets: []route.ActionTargetInput{
					{ChannelID: channel.ID, TemplateVersionID: templateVersion.ID, Enabled: true},
				},
				RecipientStrategy: json.RawMessage(`{}`),
				SendDedupeConfig:  json.RawMessage(`{}`),
			},
		}}}); err != nil {
			t.Fatalf("save route rules: %v", err)
		}
		if _, err := routeService.Publish(ctx, flow.ID); err != nil {
			t.Fatalf("publish route: %v", err)
		}
		if _, err := sourceService.Ingest(ctx, source.IngestInput{SourceCode: "providermismatch", Method: "POST", Path: "/api/v1/ingest/providermismatch", Body: []byte(`{"title":"x"}`)}); err != nil {
			t.Fatalf("ingest: %v", err)
		}
		worker := planning.NewWorker(repository, planning.WithWorkerID("planner-provider-mismatch"))
		if _, err := worker.ProcessBatch(ctx, 1); err != nil {
			t.Fatalf("process provider-mismatch batch: %v", err)
		}
		assertMessageAndRouteJob(t, ctx, pool, "trace-provider-mismatch", "failed", "MGP-PLAN-CHANNEL")

		var attempts int
		if err := pool.QueryRow(ctx, `
			SELECT count(*)
			FROM delivery_attempts AS attempt
			JOIN message_records AS message ON message.id = attempt.message_id
			WHERE message.trace_id = 'trace-provider-mismatch'
		`).Scan(&attempts); err != nil {
			t.Fatalf("query delivery attempt count: %v", err)
		}
		if attempts != 0 {
			t.Fatalf("expected no delivery attempts for provider mismatch, got %d", attempts)
		}
	})
}

func createPublishedRoute(t *testing.T, ctx context.Context, repository dbrepo.Repository, sourceID string, ruleKey string, templateVersionID string, channelID string, recipientStrategy json.RawMessage) {
	t.Helper()
	routeService := route.NewService(repository)
	flow, err := routeService.CreateFlow(ctx, route.CreateFlowInput{SourceID: sourceID, Name: "Flow " + ruleKey[len(ruleKey)-4:], Enabled: true, Mode: route.ModeTable})
	if err != nil {
		t.Fatalf("create route flow: %v", err)
	}
	if _, err := routeService.SaveRules(ctx, flow.ID, route.SaveRulesInput{Rules: []route.RuleInput{{
		RuleKey:       ruleKey,
		SortOrder:     10,
		Name:          "Always",
		ConditionTree: json.RawMessage(`{"operator":"always"}`),
		Enabled:       true,
		Action: route.ActionInput{
			TemplateVersionID: templateVersionID,
			ChannelIDs:        []string{channelID},
			RecipientStrategy: recipientStrategy,
			SendDedupeConfig:  json.RawMessage(`{}`),
		},
	}}}); err != nil {
		t.Fatalf("save route rules: %v", err)
	}
	if _, err := routeService.Publish(ctx, flow.ID); err != nil {
		t.Fatalf("publish route: %v", err)
	}
}

func assertMessageAndRouteJob(t *testing.T, ctx context.Context, pool *pgxpool.Pool, traceID string, expectedStatus string, expectedErrorCode string) {
	t.Helper()
	var messageStatus string
	var errorCode string
	var routeJobStatus string
	if err := pool.QueryRow(ctx, `
		SELECT message.status, COALESCE(message.error_code, ''), job.status
		FROM message_records AS message
		JOIN jobs AS job ON job.payload->>'message_id' = message.id::text
		WHERE message.trace_id = $1
			AND job.type = 'route_plan'
	`, traceID).Scan(&messageStatus, &errorCode, &routeJobStatus); err != nil {
		t.Fatalf("query message and route job: %v", err)
	}
	if messageStatus != expectedStatus || errorCode != expectedErrorCode || routeJobStatus != string(queue.JobStatusDone) {
		t.Fatalf("unexpected planning failure state: message=%s error=%s job=%s", messageStatus, errorCode, routeJobStatus)
	}
}

func openMigratedPool(t *testing.T) *pgxpool.Pool {
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

func createMigratedTestSchema(ctx context.Context, t *testing.T, dsn string) string {
	t.Helper()

	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connect postgres: %v", err)
	}
	defer conn.Close(ctx)

	schemaName := "mgp_planning_test_" + strings.ReplaceAll(time.Now().UTC().Format("20060102150405.000000000"), ".", "_")
	if _, err := conn.Exec(ctx, "CREATE SCHEMA "+schemaName); err != nil {
		t.Fatalf("create test schema: %v", err)
	}
	if _, err := conn.Exec(ctx, "SET search_path TO "+schemaName); err != nil {
		t.Fatalf("set search_path: %v", err)
	}
	for _, migration := range readGooseUpMigrations(t) {
		if _, err := conn.Exec(ctx, migration); err != nil {
			t.Fatalf("apply migration: %v", err)
		}
	}
	return schemaName
}

func dropTestSchema(schemaName string) {
	dsn := os.Getenv("MGP_TEST_DATABASE_URL")
	if dsn == "" || schemaName == "" {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return
	}
	defer conn.Close(ctx)
	_, _ = conn.Exec(ctx, "DROP SCHEMA "+schemaName+" CASCADE")
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

func extractGooseUp(migration string) string {
	var builder strings.Builder
	for _, line := range strings.Split(migration, "\n") {
		if strings.HasPrefix(line, "-- +goose Down") {
			break
		}
		if strings.HasPrefix(line, "-- +goose") {
			continue
		}
		builder.WriteString(line)
		builder.WriteByte('\n')
	}
	return builder.String()
}
