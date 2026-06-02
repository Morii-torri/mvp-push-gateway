package planning

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"mvp-push-gateway/backend/internal/provider"
	"mvp-push-gateway/backend/internal/queue"
	"mvp-push-gateway/backend/internal/route"
	msgtemplate "mvp-push-gateway/backend/internal/template"
)

func TestRenderTemplateUsesGatewayTemplateEngineDefaultFilterSyntax(t *testing.T) {
	body, err := renderTemplate(
		msgtemplate.TemplateVersion{
			TemplateBody: `{"content":"{{ payload.summary | default('通知') }}"}`,
		},
		MessageRecord{
			ID:       "message-1",
			TraceID:  "trace-1",
			SourceID: "source-1",
		},
		map[string]any{},
		time.Date(2026, 5, 14, 10, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("render template with default filter syntax: %v", err)
	}

	var rendered map[string]string
	if err := json.Unmarshal(body, &rendered); err != nil {
		t.Fatalf("decode rendered template: %v", err)
	}
	if rendered["content"] != "通知" {
		t.Fatalf("expected default content, got %q", rendered["content"])
	}
}

func TestRenderTemplateUsesGatewayTemplateEngineGlobalMissingPayloadFallback(t *testing.T) {
	body, err := renderTemplate(
		msgtemplate.TemplateVersion{
			TemplateBody: `{"content":"{{ payload.summary }}"}`,
		},
		MessageRecord{
			ID:       "message-1",
			TraceID:  "trace-1",
			SourceID: "source-1",
		},
		map[string]any{},
		time.Date(2026, 5, 14, 10, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("render template with global missing payload fallback: %v", err)
	}

	var rendered map[string]string
	if err := json.Unmarshal(body, &rendered); err != nil {
		t.Fatalf("decode rendered template: %v", err)
	}
	if rendered["content"] != "-" {
		t.Fatalf("expected global fallback content, got %q", rendered["content"])
	}
}

func TestResolveDedupeSupportsTraceIDStrategy(t *testing.T) {
	key, ttl := resolveDedupe(
		json.RawMessage(`{"strategy":"trace_id","ttl_seconds":600}`),
		MessageRecord{TraceID: "trace-1001"},
		map[string]any{"order_id": "A-1001"},
	)
	if key != "trace-1001" || ttl != 600 {
		t.Fatalf("expected trace_id dedupe key and ttl, got key=%q ttl=%d", key, ttl)
	}
}

func TestResolveDedupeSupportsPayloadPathStrategy(t *testing.T) {
	key, ttl := resolveDedupe(
		json.RawMessage(`{"strategy":"payload_path","key_path":"payload.order_id","ttl_seconds":300}`),
		MessageRecord{TraceID: "trace-1001"},
		map[string]any{"order_id": "A-1001"},
	)
	if key != "A-1001" || ttl != 300 {
		t.Fatalf("expected payload_path dedupe key and ttl, got key=%q ttl=%d", key, ttl)
	}
}

func TestRoutePlanCacheStatsAndFreshMatchGroups(t *testing.T) {
	repo := &routePlanCacheRepo{
		ref: RouteVersionRef{SourceID: "source-1", FlowID: "flow-1", VersionID: "version-1"},
		plan: RoutePlan{
			Flow:    route.Flow{ID: "flow-1", SourceID: "source-1"},
			Version: route.Version{ID: "version-1"},
			Rules: []route.Rule{{
				RuleKey:       "rule-1",
				ConditionTree: json.RawMessage(`{"operator":"in_match_group","path":"payload.level","match_group_id":"group-1"}`),
				Enabled:       true,
			}},
			MatchGroups: map[string][]string{"group-1": {"initial"}},
		},
		matchGroups: map[string][]string{"group-1": {"fresh"}},
	}
	worker := NewWorker(repo)

	first, err := worker.routePlan(context.Background(), "source-1")
	if err != nil {
		t.Fatalf("load first route plan: %v", err)
	}
	if first.MatchGroups["group-1"][0] != "initial" {
		t.Fatalf("expected initial match group values on cache miss, got %+v", first.MatchGroups)
	}
	repo.matchGroups = map[string][]string{"group-1": {"updated"}}
	second, err := worker.routePlan(context.Background(), "source-1")
	if err != nil {
		t.Fatalf("load cached route plan: %v", err)
	}
	if second.MatchGroups["group-1"][0] != "updated" {
		t.Fatalf("expected refreshed match group values on cache hit, got %+v", second.MatchGroups)
	}
	if repo.loadRoutePlanCalls != 1 || repo.loadMatchGroupCalls != 1 {
		t.Fatalf("expected one route plan load and one fresh match group load, got plan=%d match_groups=%d", repo.loadRoutePlanCalls, repo.loadMatchGroupCalls)
	}
	stats := worker.CacheStats()
	if stats.Misses != 1 || stats.Hits != 1 {
		t.Fatalf("expected cache stats miss=1 hit=1, got %+v", stats)
	}
}

type routePlanCacheRepo struct {
	ref                 RouteVersionRef
	plan                RoutePlan
	matchGroups         map[string][]string
	loadRoutePlanCalls  int
	loadMatchGroupCalls int
}

func (r *routePlanCacheRepo) ClaimJobs(context.Context, queue.ClaimParams) ([]queue.Job, error) {
	return nil, nil
}

func (r *routePlanCacheRepo) FailJob(context.Context, queue.FailParams) (queue.FailResult, error) {
	return queue.FailResult{}, nil
}

func (r *routePlanCacheRepo) GetPlanningMessage(context.Context, string) (MessageRecord, error) {
	return MessageRecord{}, nil
}

func (r *routePlanCacheRepo) GetCurrentRouteVersionRef(context.Context, string) (RouteVersionRef, error) {
	return r.ref, nil
}

func (r *routePlanCacheRepo) LoadRoutePlan(context.Context, string, string) (RoutePlan, error) {
	r.loadRoutePlanCalls++
	return r.plan, nil
}

func (r *routePlanCacheRepo) LoadMatchGroupValues(context.Context, []route.Rule) (map[string][]string, error) {
	r.loadMatchGroupCalls++
	return r.matchGroups, nil
}

func (r *routePlanCacheRepo) GetTemplateVersion(context.Context, string) (msgtemplate.TemplateVersion, error) {
	return msgtemplate.TemplateVersion{}, nil
}

func (r *routePlanCacheRepo) GetChannel(context.Context, string) (provider.Channel, error) {
	return provider.Channel{}, nil
}

func (r *routePlanCacheRepo) GetProviderCapability(context.Context, provider.ProviderType, string) (provider.Capability, error) {
	return provider.Capability{}, nil
}

func (r *routePlanCacheRepo) ResolveSystemRecipients(context.Context, ResolveSystemRecipientsParams) ([]string, error) {
	return nil, nil
}

func (r *routePlanCacheRepo) CompletePlanning(context.Context, CompletePlanningParams) error {
	return nil
}

func (r *routePlanCacheRepo) FinishPlanning(context.Context, FinishPlanningParams) error {
	return nil
}
