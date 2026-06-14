package planning

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"mvp-push-gateway/backend/internal/delivery"
	"mvp-push-gateway/backend/internal/perftiming"
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

func TestRoutePlanCacheUsesFullSnapshotUntilInvalidated(t *testing.T) {
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
	repo.plan.MatchGroups = map[string][]string{"group-1": {"updated"}}
	second, err := worker.routePlan(context.Background(), "source-1")
	if err != nil {
		t.Fatalf("load cached route plan: %v", err)
	}
	if second.MatchGroups["group-1"][0] != "initial" {
		t.Fatalf("expected cached full snapshot to keep initial match group values, got %+v", second.MatchGroups)
	}
	if repo.currentRefCalls != 1 || repo.loadRoutePlanCalls != 1 || repo.loadMatchGroupCalls != 0 {
		t.Fatalf("expected one version lookup, one plan load, no match group reload on hit; got ref=%d plan=%d match_groups=%d", repo.currentRefCalls, repo.loadRoutePlanCalls, repo.loadMatchGroupCalls)
	}
	stats := worker.CacheStats()
	if stats.Misses != 1 || stats.Hits != 1 {
		t.Fatalf("expected cache stats miss=1 hit=1, got %+v", stats)
	}

	worker.InvalidateRoutePlan("source-1")
	third, err := worker.routePlan(context.Background(), "source-1")
	if err != nil {
		t.Fatalf("reload invalidated route plan: %v", err)
	}
	if third.MatchGroups["group-1"][0] != "updated" {
		t.Fatalf("expected invalidation to reload full snapshot, got %+v", third.MatchGroups)
	}
	if repo.currentRefCalls != 2 || repo.loadRoutePlanCalls != 2 {
		t.Fatalf("expected invalidation to reload ref and plan, got ref=%d plan=%d", repo.currentRefCalls, repo.loadRoutePlanCalls)
	}
}

func TestRefreshRoutePlanReloadsCurrentVersionForSource(t *testing.T) {
	repo := &routePlanCacheRepo{
		ref: RouteVersionRef{SourceID: "source-1", FlowID: "flow-1", VersionID: "version-1"},
		plan: RoutePlan{
			Flow:        route.Flow{ID: "flow-1", SourceID: "source-1"},
			Version:     route.Version{ID: "version-1"},
			Rules:       []route.Rule{{RuleKey: "rule-1", ConditionTree: json.RawMessage(`{"operator":"always"}`), Enabled: true}},
			MatchGroups: map[string][]string{},
		},
	}
	worker := NewWorker(repo)

	if _, err := worker.routePlan(context.Background(), "source-1"); err != nil {
		t.Fatalf("prime route plan: %v", err)
	}
	repo.ref = RouteVersionRef{SourceID: "source-1", FlowID: "flow-1", VersionID: "version-2"}
	repo.plan.Version = route.Version{ID: "version-2"}

	if err := worker.RefreshRoutePlan(context.Background(), "source-1"); err != nil {
		t.Fatalf("refresh route plan: %v", err)
	}
	plan, err := worker.routePlan(context.Background(), "source-1")
	if err != nil {
		t.Fatalf("load refreshed route plan: %v", err)
	}
	if plan.Version.ID != "version-2" {
		t.Fatalf("expected refreshed cache to use version-2, got %s", plan.Version.ID)
	}
	if repo.currentRefCalls != 2 || repo.loadRoutePlanCalls != 2 {
		t.Fatalf("expected refresh to reload once and following hit to avoid DB, got ref=%d plan=%d", repo.currentRefCalls, repo.loadRoutePlanCalls)
	}
}

func TestStoreRoutePlanSortsRulesAndPreparesConditions(t *testing.T) {
	worker := NewWorker(&routePlanCacheRepo{})
	worker.storeRoutePlan(RoutePlan{
		Flow:    route.Flow{ID: "flow-1", SourceID: "source-1"},
		Version: route.Version{ID: "version-1"},
		Rules: []route.Rule{
			{
				ID:            "rule-late",
				RuleKey:       "rule-late",
				SortOrder:     20,
				ConditionTree: json.RawMessage(`{"operator":"always"}`),
				Enabled:       true,
			},
			{
				ID:            "rule-first",
				RuleKey:       "rule-first",
				SortOrder:     10,
				ConditionTree: json.RawMessage(`{"operator":"equals","path":"payload.level","value":"critical"}`),
				Enabled:       true,
			},
		},
	})

	plan, err := worker.routePlan(context.Background(), "source-1")
	if err != nil {
		t.Fatalf("load cached plan: %v", err)
	}
	if len(plan.Rules) != 2 || plan.Rules[0].RuleKey != "rule-first" || plan.Rules[1].RuleKey != "rule-late" {
		t.Fatalf("expected rules to be sorted once in cache snapshot, got %+v", plan.Rules)
	}
	if len(plan.PreparedRules) != 2 {
		t.Fatalf("expected cached route plan to prepare executable rules, got %d", len(plan.PreparedRules))
	}
	if plan.PreparedRules[0].Rule.RuleKey != "rule-first" {
		t.Fatalf("expected prepared rules to keep sorted order, got %+v", plan.PreparedRules)
	}
	matched, _, err := evaluateRules(plan, map[string]any{"level": "critical"})
	if err != nil {
		t.Fatalf("evaluate prepared cached plan: %v", err)
	}
	if matched == nil || matched.RuleKey != "rule-first" {
		t.Fatalf("expected prepared first rule to match, got %+v", matched)
	}
}

func TestProcessRoutePlanMessageAcksAfterSuccessfulPlanningWithoutPostgresJob(t *testing.T) {
	repo := &routePlanMessageRepo{
		message: MessageRecord{
			ID:       "message-jetstream",
			TraceID:  "trace-jetstream",
			SourceID: "source-1",
			Payload:  json.RawMessage(`{"title":"paid"}`),
		},
		plan: RoutePlan{
			Flow:    route.Flow{ID: "flow-1", SourceID: "source-1"},
			Version: route.Version{ID: "version-1"},
			Rules: []route.Rule{{
				ID:            "rule-id-1",
				RuleKey:       "00000000-0000-0000-0000-000000000001",
				ConditionTree: json.RawMessage(`{"operator":"always"}`),
				Enabled:       true,
				Action: route.Action{
					Targets: []route.ActionTarget{{
						ChannelID:         "channel-1",
						TemplateVersionID: "template-version-1",
						Enabled:           true,
						SortOrder:         10,
					}},
				},
			}},
			MatchGroups: map[string][]string{},
		},
		channel: provider.Channel{
			ID:           "channel-1",
			ProviderType: provider.ProviderWebhook,
			Enabled:      true,
		},
		capability: provider.Capability{
			ProviderType:     provider.ProviderWebhook,
			MessageType:      "json",
			AllowNoRecipient: true,
		},
		templateVersion: msgtemplate.TemplateVersion{
			ID:                 "template-version-1",
			MessageType:        "json",
			TargetProviderType: string(provider.ProviderWebhook),
			TemplateBody:       `{"body":{"title":"{{ payload.title | default('【模版】性能测试') }}"}}`,
		},
	}
	worker := NewWorker(repo, WithWorkerID("planner-jetstream"))
	acked := false
	nacked := false

	err := worker.ProcessRoutePlanMessage(context.Background(), queue.RoutePlanMessage{
		Event: queue.RoutePlanEvent{
			MessageID: "message-jetstream",
			SourceID:  "source-1",
			TraceID:   "trace-jetstream",
		},
		Ack: func() error {
			acked = true
			return nil
		},
		Nak: func(time.Duration) error {
			nacked = true
			return nil
		},
	})
	if err != nil {
		t.Fatalf("process route plan message: %v", err)
	}
	if !acked || nacked {
		t.Fatalf("expected successful route plan message to ack only, acked=%v nacked=%v", acked, nacked)
	}
	if repo.completePlanningCalls != 1 {
		t.Fatalf("expected one planning completion, got %d", repo.completePlanningCalls)
	}
	if repo.completed.JobID != "" {
		t.Fatalf("expected JetStream route-plan completion not to require a PostgreSQL job id, got %q", repo.completed.JobID)
	}
	if len(repo.completed.Attempts) != 1 {
		t.Fatalf("expected one delivery attempt, got %+v", repo.completed.Attempts)
	}
	if repo.completed.Attempts[0].ChannelID != "channel-1" || repo.completed.Attempts[0].JobPayload == nil {
		t.Fatalf("unexpected delivery attempt plan: %+v", repo.completed.Attempts[0])
	}
	var payload delivery.SendMessageJobPayload
	if err := json.Unmarshal(repo.completed.Attempts[0].JobPayload, &payload); err != nil {
		t.Fatalf("decode attempt payload: %v", err)
	}
	if payload.RoutePlanStartedAt.IsZero() ||
		payload.RouteConditionFinishedAt.IsZero() ||
		payload.TemplateRenderFinishedAt.IsZero() ||
		payload.SendEventBuiltAt.IsZero() {
		t.Fatalf("expected planning lifecycle breakdown in send payload, got %+v", payload)
	}
}

func TestProcessRoutePlanMessagePublishesSendWhenPlanningPersistenceFails(t *testing.T) {
	repo := newRoutePlanMessageRepoForSendPublisherTest()
	repo.completePlanningErr = errors.New("planning persistence is slow")
	repo.completePlanningCh = make(chan struct{}, 1)
	publisher := &recordingSendPublisher{}
	worker := NewWorker(repo, WithSendPublisher(publisher))
	acked := false
	nacked := false

	err := worker.ProcessRoutePlanMessage(context.Background(), queue.RoutePlanMessage{
		Event: queue.RoutePlanEvent{
			MessageID: "message-jetstream",
			SourceID:  "source-1",
			TraceID:   "trace-jetstream",
		},
		Ack: func() error {
			acked = true
			return nil
		},
		Nak: func(time.Duration) error {
			nacked = true
			return nil
		},
	})
	if err != nil {
		t.Fatalf("process route plan message with failing planning persistence: %v", err)
	}
	if !acked || nacked {
		t.Fatalf("expected route plan message to ack after send publish, acked=%v nacked=%v", acked, nacked)
	}
	if len(publisher.events) != 1 {
		t.Fatalf("expected send event to be published before planning persistence, got %d events", len(publisher.events))
	}
	select {
	case <-repo.completePlanningCh:
	case <-time.After(time.Second):
		t.Fatalf("expected best-effort planning persistence to be attempted")
	}
}

func TestWorkerPublishesSendEventsWhenSendPublisherConfigured(t *testing.T) {
	repo := newRoutePlanMessageRepoForSendPublisherTest()
	publisher := &recordingSendPublisher{}
	worker := NewWorker(repo, WithSendPublisher(publisher))
	job := queue.Job{
		ID:       "job-route-plan",
		Type:     queue.JobTypeRoutePlan,
		Status:   queue.JobStatusProcessing,
		LockedBy: "planning-worker",
		Payload:  json.RawMessage(`{"message_id":"message-jetstream","source_id":"source-1","trace_id":"trace-jetstream"}`),
	}

	if err := worker.ProcessOne(context.Background(), job); err != nil {
		t.Fatalf("process route plan job with send publisher: %v", err)
	}
	if !repo.completed.ExternalSendQueue {
		t.Fatalf("expected planning completion to skip PostgreSQL send jobs when send publisher is configured")
	}
	if len(publisher.events) != 1 {
		t.Fatalf("expected one send event, got %d", len(publisher.events))
	}
	event := publisher.events[0]
	attempt := repo.completed.Attempts[0]
	if event.DeliveryAttemptID != attempt.ID || event.ChannelID != "channel-1" || event.ProviderType != "webhook" || event.TraceID != "trace-jetstream" {
		t.Fatalf("unexpected send event: %+v attempt=%+v", event, attempt)
	}
	var payload delivery.SendMessageJobPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		t.Fatalf("decode send event payload: %v", err)
	}
	if payload.DeliveryAttemptID != attempt.ID || payload.TraceID != "trace-jetstream" {
		t.Fatalf("expected send event payload to preserve attempt and trace ids, got %+v", payload)
	}
}

func TestProcessRoutePlanMessageUsesEventPayloadWithoutLoadingMessageRecord(t *testing.T) {
	repo := newRoutePlanMessageRepoForSendPublisherTest()
	repo.failOnGetPlanningMessage = true
	repo.completePlanningCh = make(chan struct{}, 1)
	publisher := &recordingSendPublisher{}
	worker := NewWorker(repo, WithSendPublisher(publisher))
	receivedAt := time.Date(2026, 6, 8, 10, 30, 0, 0, time.UTC)

	err := worker.ProcessRoutePlanMessage(context.Background(), queue.RoutePlanMessage{
		Event: queue.RoutePlanEvent{
			MessageID:  "message-direct",
			SourceID:   "source-1",
			TraceID:    "trace-direct",
			Payload:    json.RawMessage(`{"title":"direct"}`),
			Headers:    json.RawMessage(`{"Content-Type":["application/json"]}`),
			ReceivedAt: receivedAt,
		},
	})
	if err != nil {
		t.Fatalf("process direct route plan event: %v", err)
	}
	if repo.getPlanningMessageCalls != 0 {
		t.Fatalf("expected direct route plan event not to load message record, got %d calls", repo.getPlanningMessageCalls)
	}
	select {
	case <-repo.completePlanningCh:
	case <-time.After(time.Second):
		t.Fatalf("expected direct route plan event to persist planning output asynchronously")
	}
	if repo.completed.SourceID != "source-1" ||
		string(repo.completed.InboundPayload) != `{"title":"direct"}` ||
		repo.completed.InboundReceivedAt.IsZero() {
		t.Fatalf("expected direct planning completion to carry inbound snapshot, got %+v", repo.completed)
	}
	if len(publisher.events) != 1 {
		t.Fatalf("expected one direct send event, got %d", len(publisher.events))
	}
	event := publisher.events[0]
	if event.MessageID != "message-direct" || event.SourceID != "source-1" || event.TraceID != "trace-direct" {
		t.Fatalf("expected send event to carry message identity, got %+v", event)
	}
	var payload delivery.SendMessageJobPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		t.Fatalf("decode send event payload: %v", err)
	}
	if payload.MessageID != "message-direct" || payload.SourceID != "source-1" || string(payload.InboundPayload) != `{"title":"direct"}` || payload.InboundReceivedAt.IsZero() {
		t.Fatalf("expected direct send payload to carry inbound metadata, got %+v", payload)
	}
	if !strings.Contains(string(payload.Body), "direct") {
		t.Fatalf("expected template to render from event payload, got %s", payload.Body)
	}
}

func TestProcessRoutePlanMessageRecordsRuntimeTimingWithoutContextRecorder(t *testing.T) {
	repo := newRoutePlanMessageRepoForSendPublisherTest()
	publisher := &recordingSendPublisher{}
	worker := NewWorker(repo, WithSendPublisher(publisher))
	recorder := &runtimeTimingRecorder{}
	unregister := perftiming.Register(recorder)
	defer unregister()

	err := worker.ProcessRoutePlanMessage(context.Background(), queue.RoutePlanMessage{
		Event: queue.RoutePlanEvent{
			MessageID:  "message-direct",
			SourceID:   "source-1",
			TraceID:    "trace-runtime",
			Payload:    json.RawMessage(`{"title":"runtime"}`),
			Headers:    json.RawMessage(`{}`),
			ReceivedAt: time.Date(2026, 6, 8, 10, 30, 0, 0, time.UTC),
		},
	})
	if err != nil {
		t.Fatalf("process direct route plan event: %v", err)
	}
	if recorder.count("trace-runtime", string(TimingRouteCondition)) == 0 {
		t.Fatalf("expected global runtime timing recorder to capture route condition timing, got %+v", recorder.timings)
	}
	if recorder.count("trace-runtime", string(TimingTemplateRender)) == 0 {
		t.Fatalf("expected global runtime timing recorder to capture template render timing, got %+v", recorder.timings)
	}
}

func TestRoutePlanCacheHydratesTargetResourcesOnce(t *testing.T) {
	repo := newRoutePlanMessageRepoForSendPublisherTest()
	publisher := &recordingSendPublisher{}
	worker := NewWorker(repo, WithSendPublisher(publisher))

	for _, traceID := range []string{"trace-cache-1", "trace-cache-2"} {
		err := worker.ProcessRoutePlanMessage(context.Background(), queue.RoutePlanMessage{
			Event: queue.RoutePlanEvent{
				MessageID:  "message-" + traceID,
				SourceID:   "source-1",
				TraceID:    traceID,
				Payload:    json.RawMessage(`{"title":"cached"}`),
				Headers:    json.RawMessage(`{}`),
				ReceivedAt: time.Date(2026, 6, 8, 10, 30, 0, 0, time.UTC),
			},
		})
		if err != nil {
			t.Fatalf("process direct route plan event %s: %v", traceID, err)
		}
	}

	if repo.getChannelCalls != 1 || repo.getTemplateVersionCalls != 1 || repo.getProviderCapabilityCalls != 1 {
		t.Fatalf("expected target resources to be loaded once with the cached route plan, got channel=%d template=%d capability=%d", repo.getChannelCalls, repo.getTemplateVersionCalls, repo.getProviderCapabilityCalls)
	}
	if len(publisher.events) != 2 {
		t.Fatalf("expected both messages to publish send events, got %d", len(publisher.events))
	}
}

func TestEvaluateRulesCoarseSkipsMissingPayloadFields(t *testing.T) {
	plan := RoutePlan{
		Flow:    route.Flow{ID: "flow-1", SourceID: "source-1"},
		Version: route.Version{ID: "version-1"},
		Rules: []route.Rule{
			{
				ID:            "rule-1",
				RuleKey:       "rule-1",
				SortOrder:     1,
				ConditionTree: json.RawMessage(`{"operator":"equals","path":"payload.severity","value":"critical"}`),
				Enabled:       true,
			},
			{
				ID:            "rule-2",
				RuleKey:       "rule-2",
				SortOrder:     2,
				ConditionTree: json.RawMessage(`{"operator":"always"}`),
				Enabled:       true,
			},
		},
	}

	matched, metrics, err := evaluateRules(plan, map[string]any{"title": "critical"})
	if err != nil {
		t.Fatalf("evaluate rules: %v", err)
	}
	if matched == nil || matched.RuleKey != "rule-2" {
		t.Fatalf("expected fallback rule to match, got %+v", matched)
	}
	if len(metrics) != 2 {
		t.Fatalf("expected both rules to emit metrics, got %+v", metrics)
	}
	if metrics[0].Evaluated || metrics[0].Matched || metrics[0].DurationMS != 0 {
		t.Fatalf("expected first rule metric to be skipped without evaluation, got %+v", metrics[0])
	}
	if !metrics[1].Evaluated || !metrics[1].Matched {
		t.Fatalf("expected second rule metric to be evaluated and matched, got %+v", metrics[1])
	}
}

type routePlanCacheRepo struct {
	ref                 RouteVersionRef
	plan                RoutePlan
	matchGroups         map[string][]string
	currentRefCalls     int
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
	r.currentRefCalls++
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

type routePlanMessageRepo struct {
	message                    MessageRecord
	plan                       RoutePlan
	channel                    provider.Channel
	capability                 provider.Capability
	templateVersion            msgtemplate.TemplateVersion
	completePlanningCalls      int
	getPlanningMessageCalls    int
	getChannelCalls            int
	getTemplateVersionCalls    int
	getProviderCapabilityCalls int
	failOnGetPlanningMessage   bool
	completePlanningErr        error
	completePlanningCh         chan struct{}
	completed                  CompletePlanningParams
}

func (r *routePlanMessageRepo) ClaimJobs(context.Context, queue.ClaimParams) ([]queue.Job, error) {
	return nil, nil
}

func (r *routePlanMessageRepo) FailJob(context.Context, queue.FailParams) (queue.FailResult, error) {
	return queue.FailResult{}, nil
}

func (r *routePlanMessageRepo) GetPlanningMessage(_ context.Context, id string) (MessageRecord, error) {
	r.getPlanningMessageCalls++
	if r.failOnGetPlanningMessage {
		return MessageRecord{}, errors.New("GetPlanningMessage should not be called")
	}
	if id != r.message.ID {
		return MessageRecord{}, ErrNotFound
	}
	return r.message, nil
}

func (r *routePlanMessageRepo) GetCurrentRouteVersionRef(context.Context, string) (RouteVersionRef, error) {
	return RouteVersionRef{SourceID: r.plan.Flow.SourceID, FlowID: r.plan.Flow.ID, VersionID: r.plan.Version.ID}, nil
}

func (r *routePlanMessageRepo) LoadRoutePlan(context.Context, string, string) (RoutePlan, error) {
	return r.plan, nil
}

func (r *routePlanMessageRepo) LoadMatchGroupValues(context.Context, []route.Rule) (map[string][]string, error) {
	return r.plan.MatchGroups, nil
}

func (r *routePlanMessageRepo) GetTemplateVersion(context.Context, string) (msgtemplate.TemplateVersion, error) {
	r.getTemplateVersionCalls++
	return r.templateVersion, nil
}

func (r *routePlanMessageRepo) GetChannel(context.Context, string) (provider.Channel, error) {
	r.getChannelCalls++
	return r.channel, nil
}

func (r *routePlanMessageRepo) GetProviderCapability(context.Context, provider.ProviderType, string) (provider.Capability, error) {
	r.getProviderCapabilityCalls++
	return r.capability, nil
}

func (r *routePlanMessageRepo) ResolveSystemRecipients(context.Context, ResolveSystemRecipientsParams) ([]string, error) {
	return nil, nil
}

func (r *routePlanMessageRepo) CompletePlanning(_ context.Context, params CompletePlanningParams) error {
	r.completePlanningCalls++
	r.completed = params
	if r.completePlanningCh != nil {
		select {
		case r.completePlanningCh <- struct{}{}:
		default:
		}
	}
	return r.completePlanningErr
}

func (r *routePlanMessageRepo) FinishPlanning(context.Context, FinishPlanningParams) error {
	return nil
}

func newRoutePlanMessageRepoForSendPublisherTest() *routePlanMessageRepo {
	return &routePlanMessageRepo{
		message: MessageRecord{
			ID:       "message-jetstream",
			TraceID:  "trace-jetstream",
			SourceID: "source-1",
			Payload:  json.RawMessage(`{"title":"paid"}`),
		},
		plan: RoutePlan{
			Flow:    route.Flow{ID: "flow-1", SourceID: "source-1"},
			Version: route.Version{ID: "version-1"},
			Rules: []route.Rule{{
				ID:            "rule-id-1",
				RuleKey:       "00000000-0000-0000-0000-000000000001",
				ConditionTree: json.RawMessage(`{"operator":"always"}`),
				Enabled:       true,
				Action: route.Action{
					Targets: []route.ActionTarget{{
						ChannelID:         "channel-1",
						TemplateVersionID: "template-version-1",
						Enabled:           true,
						SortOrder:         10,
					}},
				},
			}},
			MatchGroups: map[string][]string{},
		},
		channel: provider.Channel{
			ID:           "channel-1",
			ProviderType: provider.ProviderWebhook,
			Enabled:      true,
		},
		capability: provider.Capability{
			ProviderType:     provider.ProviderWebhook,
			MessageType:      "json",
			AllowNoRecipient: true,
		},
		templateVersion: msgtemplate.TemplateVersion{
			ID:                 "template-version-1",
			MessageType:        "json",
			TargetProviderType: string(provider.ProviderWebhook),
			TemplateBody:       `{"body":{"title":"{{ payload.title | default('【模版】性能测试') }}"}}`,
		},
	}
}

type recordingSendPublisher struct {
	events []queue.SendMessageEvent
	err    error
}

func (p *recordingSendPublisher) PublishSend(_ context.Context, event queue.SendMessageEvent) (queue.PublishResult, error) {
	p.events = append(p.events, event)
	return queue.PublishResult{Stream: "MGP_SEND", Sequence: uint64(len(p.events))}, p.err
}

type runtimeTimingRecorder struct {
	timings map[string]map[string]int
}

func (r *runtimeTimingRecorder) RecordStageTiming(traceID string, stage string, _ time.Duration) {
	if r.timings == nil {
		r.timings = map[string]map[string]int{}
	}
	if r.timings[traceID] == nil {
		r.timings[traceID] = map[string]int{}
	}
	r.timings[traceID][stage]++
}

func (r *runtimeTimingRecorder) RecordDBStageTiming(string, string, time.Duration) {}

func (r *runtimeTimingRecorder) count(traceID string, stage string) int {
	if r == nil || r.timings == nil {
		return 0
	}
	return r.timings[traceID][stage]
}
