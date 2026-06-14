package planning

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	"mvp-push-gateway/backend/internal/delivery"
	"mvp-push-gateway/backend/internal/perftiming"
	"mvp-push-gateway/backend/internal/provider"
	"mvp-push-gateway/backend/internal/queue"
	"mvp-push-gateway/backend/internal/route"
	msgtemplate "mvp-push-gateway/backend/internal/template"
)

const (
	ErrorCodeNoRoute   = "MGP-PLAN-NOROUTE"
	ErrorCodeRoute     = "MGP-PLAN-ROUTE"
	ErrorCodeTemplate  = "MGP-PLAN-TPL"
	ErrorCodeRecipient = "MGP-PLAN-RCPT"
	ErrorCodeChannel   = "MGP-PLAN-CHANNEL"
	ErrorCodeJob       = "MGP-PLAN-JOB"
)

var (
	ErrInvalidInput = errors.New("invalid planning input")
	ErrNoRoute      = errors.New("no published route for source")
	ErrNotFound     = errors.New("planning resource not found")
)

type RoutePlanJobPayload struct {
	MessageID string `json:"message_id"`
	SourceID  string `json:"source_id"`
	TraceID   string `json:"trace_id"`
}

type MessageRecord struct {
	ID         string
	TraceID    string
	SourceID   string
	Headers    json.RawMessage
	Payload    json.RawMessage
	Status     string
	ReceivedAt time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type RouteVersionRef struct {
	SourceID  string
	FlowID    string
	VersionID string
}

type RoutePlan struct {
	Flow             route.Flow
	Version          route.Version
	Rules            []route.Rule
	PreparedRules    []PreparedRouteRule
	MatchGroups      map[string][]string
	Channels         map[string]provider.Channel
	TemplateVersions map[string]msgtemplate.TemplateVersion
	Capabilities     map[string]provider.Capability
}

type PreparedRouteRule struct {
	Rule      route.Rule
	Condition route.PreparedConditionTree
}

type RuleMetric struct {
	SourceID       string
	FlowID         string
	RouteVersionID string
	RuleID         string
	Evaluated      bool
	Matched        bool
	DurationMS     int
}

type TimingStage string

const (
	TimingClaimJobs      TimingStage = "planning_claim"
	TimingRoutePlan      TimingStage = "route_plan_lookup"
	TimingRouteCondition TimingStage = "route_condition"
	TimingTemplateRender TimingStage = "planning_template_render"
	TimingComplete       TimingStage = "planning_complete"
)

type TimingRecorder interface {
	RecordPlanningTiming(traceID string, stage TimingStage, duration time.Duration)
}

type timingRecorderContextKey struct{}

func WithTimingRecorder(ctx context.Context, recorder TimingRecorder) context.Context {
	if recorder == nil {
		return ctx
	}
	return context.WithValue(ctx, timingRecorderContextKey{}, recorder)
}

func recordTiming(ctx context.Context, traceID string, stage TimingStage, duration time.Duration) {
	recorder, ok := ctx.Value(timingRecorderContextKey{}).(TimingRecorder)
	if !ok || recorder == nil {
		perftiming.RecordStageTiming(traceID, string(stage), duration)
		return
	}
	recorder.RecordPlanningTiming(traceID, stage, duration)
}

type DeliveryAttemptPlan struct {
	ID                string
	MessageID         string
	SourceID          string
	ChannelID         string
	ProviderType      string
	TemplateVersionID string
	RecipientSnapshot json.RawMessage
	JobPayload        json.RawMessage
	MaxAttempts       int
	DedupeKey         string
	DedupeTTLSeconds  int
	InboundHeaders    json.RawMessage
	InboundPayload    json.RawMessage
	InboundReceivedAt time.Time
}

type CompletePlanningParams struct {
	JobID             string
	WorkerID          string
	MessageID         string
	SourceID          string
	TraceID           string
	InboundHeaders    json.RawMessage
	InboundPayload    json.RawMessage
	InboundReceivedAt time.Time
	FlowID            string
	MatchedRuleIDs    []string
	HitRuleKey        string
	FinishedAt        time.Time
	DurationMS        int
	Attempts          []DeliveryAttemptPlan
	RuleMetrics       []RuleMetric
	ExternalSendQueue bool
}

type planningLifecycleBreakdown struct {
	RoutePlanStartedAt       time.Time
	RouteConditionFinishedAt time.Time
	RouteConditionDurationMS int
	TemplateRenderFinishedAt time.Time
	TemplateRenderDurationMS int
	SendEventBuiltAt         time.Time
	SendEventBuildDurationMS int
}

type FinishPlanningParams struct {
	JobID          string
	WorkerID       string
	MessageID      string
	SourceID       string
	TraceID        string
	Headers        json.RawMessage
	Payload        json.RawMessage
	ReceivedAt     time.Time
	Status         string
	FlowID         string
	MatchedRuleIDs []string
	ErrorCode      string
	ErrorMessage   string
	FinishedAt     time.Time
	DurationMS     int
	RuleMetrics    []RuleMetric
}

type ResolveSystemRecipientsParams struct {
	ProviderType      provider.ProviderType
	ChannelID         string
	IdentityKind      string
	UserIDs           []string
	OrgIDs            []string
	RecipientGroupIDs []string
	ExcludedUserIDs   []string
	ExcludedOrgIDs    []string
}

type Repository interface {
	ClaimJobs(context.Context, queue.ClaimParams) ([]queue.Job, error)
	FailJob(context.Context, queue.FailParams) (queue.FailResult, error)

	GetPlanningMessage(context.Context, string) (MessageRecord, error)
	GetCurrentRouteVersionRef(context.Context, string) (RouteVersionRef, error)
	LoadRoutePlan(context.Context, string, string) (RoutePlan, error)
	LoadMatchGroupValues(context.Context, []route.Rule) (map[string][]string, error)
	GetTemplateVersion(context.Context, string) (msgtemplate.TemplateVersion, error)
	GetChannel(context.Context, string) (provider.Channel, error)
	GetProviderCapability(context.Context, provider.ProviderType, string) (provider.Capability, error)
	ResolveSystemRecipients(context.Context, ResolveSystemRecipientsParams) ([]string, error)

	CompletePlanning(context.Context, CompletePlanningParams) error
	FinishPlanning(context.Context, FinishPlanningParams) error
}

type SendPublisher interface {
	PublishSend(context.Context, queue.SendMessageEvent) (queue.PublishResult, error)
}

type SendBatchPublisher interface {
	PublishSendBatch(context.Context, []queue.SendMessageEvent) ([]queue.PublishResult, error)
}

type Worker struct {
	repo          Repository
	workerID      string
	now           func() time.Time
	retryDelay    time.Duration
	sendPublisher SendPublisher

	cacheMu    sync.RWMutex
	routeCache map[string]RoutePlan

	cacheHits       int64
	cacheMisses     int64
	cacheLoadTimeMS int64
}

type RouteCacheStats struct {
	Hits       int64
	Misses     int64
	LoadTimeMS int64
}

type WorkerOption func(*Worker)

func WithWorkerID(workerID string) WorkerOption {
	return func(w *Worker) {
		if strings.TrimSpace(workerID) != "" {
			w.workerID = strings.TrimSpace(workerID)
		}
	}
}

func WithNow(now func() time.Time) WorkerOption {
	return func(w *Worker) {
		if now != nil {
			w.now = now
		}
	}
}

func WithRetryDelay(delay time.Duration) WorkerOption {
	return func(w *Worker) {
		if delay > 0 {
			w.retryDelay = delay
		}
	}
}

func WithSendPublisher(publisher SendPublisher) WorkerOption {
	return func(w *Worker) {
		w.sendPublisher = publisher
	}
}

func NewWorker(repo Repository, options ...WorkerOption) *Worker {
	worker := &Worker{
		repo:     repo,
		workerID: "planning-worker",
		now: func() time.Time {
			return time.Now().UTC()
		},
		retryDelay: time.Minute,
		routeCache: make(map[string]RoutePlan),
	}
	for _, option := range options {
		option(worker)
	}
	return worker
}

func (w *Worker) ProcessBatch(ctx context.Context, limit int) (int, error) {
	if w == nil || w.repo == nil {
		return 0, ErrInvalidInput
	}
	if limit <= 0 {
		limit = 1
	}
	now := w.now()
	claimStartedAt := time.Now()
	jobs, err := w.repo.ClaimJobs(ctx, queue.ClaimParams{
		WorkerID: w.workerID,
		Types:    []queue.JobType{queue.JobTypeRoutePlan},
		Limit:    limit,
		Now:      now,
	})
	if err != nil {
		return 0, err
	}
	claimDuration := time.Since(claimStartedAt)
	for _, job := range jobs {
		if payload, err := decodeRoutePlanPayload(job.Payload); err == nil {
			recordTiming(ctx, payload.TraceID, TimingClaimJobs, claimDuration)
		}
	}

	var firstErr error
	for _, job := range jobs {
		if err := w.ProcessOne(ctx, job); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return len(jobs), firstErr
}

func (w *Worker) ProcessRoutePlanMessage(ctx context.Context, message queue.RoutePlanMessage) error {
	if w == nil || w.repo == nil {
		return ErrInvalidInput
	}
	if len(bytes.TrimSpace(message.Event.Payload)) > 0 {
		if err := message.Event.Validate(); err != nil {
			return errors.Join(err, nakRoutePlanMessage(message, w.retryDelay))
		}
		record := MessageRecord{
			ID:         strings.TrimSpace(message.Event.MessageID),
			TraceID:    strings.TrimSpace(message.Event.TraceID),
			SourceID:   strings.TrimSpace(message.Event.SourceID),
			Headers:    append(json.RawMessage(nil), message.Event.Headers...),
			Payload:    append(json.RawMessage(nil), message.Event.Payload...),
			Status:     "accepted",
			ReceivedAt: message.Event.ReceivedAt,
			CreatedAt:  message.Event.ReceivedAt,
			UpdatedAt:  message.Event.ReceivedAt,
		}
		if record.ID == "" {
			record.ID = record.TraceID
		}
		if record.ReceivedAt.IsZero() {
			record.ReceivedAt = w.now().UTC()
			record.CreatedAt = record.ReceivedAt
			record.UpdatedAt = record.ReceivedAt
		}
		job := queue.Job{
			Type:        queue.JobTypeRoutePlan,
			Status:      queue.JobStatusProcessing,
			RunAt:       time.Now().UTC(),
			MaxAttempts: 1,
			QueueKey:    record.SourceID,
		}
		if err := w.processMessage(ctx, job, record, true); err != nil {
			return errors.Join(err, nakRoutePlanMessage(message, w.retryDelay))
		}
		if message.Ack != nil {
			return message.Ack()
		}
		return nil
	}
	job, err := routePlanJobFromEvent(message.Event)
	if err != nil {
		return errors.Join(err, nakRoutePlanMessage(message, w.retryDelay))
	}
	if err := w.ProcessOne(ctx, job); err != nil {
		return errors.Join(err, nakRoutePlanMessage(message, w.retryDelay))
	}
	if message.Ack != nil {
		return message.Ack()
	}
	return nil
}

func (w *Worker) ProcessOne(ctx context.Context, job queue.Job) error {
	startedAt := w.now()
	payload, err := decodeRoutePlanPayload(job.Payload)
	if err != nil {
		return w.failJob(ctx, job, ErrorCodeJob, err)
	}
	messageID := strings.TrimSpace(payload.MessageID)
	if messageID == "" {
		return w.failJob(ctx, job, ErrorCodeJob, errors.New("message_id is required"))
	}

	message, err := w.repo.GetPlanningMessage(ctx, messageID)
	if err != nil {
		return w.failJob(ctx, job, ErrorCodeJob, err)
	}
	return w.processMessage(ctx, job, message, false, startedAt)
}

func (w *Worker) processMessage(ctx context.Context, job queue.Job, message MessageRecord, direct bool, startedAtOverride ...time.Time) error {
	startedAt := w.now()
	if len(startedAtOverride) > 0 && !startedAtOverride[0].IsZero() {
		startedAt = startedAtOverride[0]
	}
	lifecycle := planningLifecycleBreakdown{RoutePlanStartedAt: startedAt.UTC()}
	payloadMap, err := decodeJSONObject(message.Payload)
	if err != nil {
		return w.finishBusinessFailure(ctx, job, message, "", nil, ErrorCodeJob, err, startedAt, nil)
	}

	routePlanStartedAt := time.Now()
	plan, err := w.routePlan(ctx, message.SourceID)
	recordTiming(ctx, message.TraceID, TimingRoutePlan, time.Since(routePlanStartedAt))
	if err != nil {
		if errors.Is(err, ErrNoRoute) {
			return w.finishBusinessFailure(ctx, job, message, "", nil, ErrorCodeNoRoute, err, startedAt, nil)
		}
		return w.failJob(ctx, job, ErrorCodeRoute, err)
	}

	conditionStartedAt := time.Now()
	matchedRule, metrics, err := evaluateRules(plan, payloadMap)
	conditionDuration := time.Since(conditionStartedAt)
	recordTiming(ctx, message.TraceID, TimingRouteCondition, conditionDuration)
	lifecycle.RouteConditionFinishedAt = w.now().UTC()
	lifecycle.RouteConditionDurationMS = int(conditionDuration.Milliseconds())
	if err != nil {
		return w.finishBusinessFailure(ctx, job, message, plan.Flow.ID, nil, ErrorCodeRoute, err, startedAt, metrics)
	}
	if matchedRule == nil {
		return w.finishBusinessFailure(ctx, job, message, plan.Flow.ID, nil, ErrorCodeNoRoute, ErrNoRoute, startedAt, metrics)
	}

	attempts, err := w.buildAttempts(ctx, message, plan, *matchedRule, payloadMap, &lifecycle)
	if err != nil {
		return w.finishBusinessFailure(ctx, job, message, plan.Flow.ID, []string{matchedRule.RuleKey}, planningErrorCode(err), err, startedAt, metrics)
	}

	finishedAt := w.now()
	lifecycle.SendEventBuiltAt = finishedAt.UTC()
	if !lifecycle.TemplateRenderFinishedAt.IsZero() {
		lifecycle.SendEventBuildDurationMS = durationMS(lifecycle.TemplateRenderFinishedAt, lifecycle.SendEventBuiltAt)
	}
	var annotateErr error
	attempts, annotateErr = annotateAttemptLifecycle(attempts, lifecycle, finishedAt)
	if annotateErr != nil {
		return annotateErr
	}
	completeStartedAt := time.Now()
	completeParams := CompletePlanningParams{
		JobID:             job.ID,
		WorkerID:          w.workerID,
		MessageID:         message.ID,
		SourceID:          message.SourceID,
		TraceID:           message.TraceID,
		InboundHeaders:    append(json.RawMessage(nil), message.Headers...),
		InboundPayload:    append(json.RawMessage(nil), message.Payload...),
		InboundReceivedAt: message.ReceivedAt,
		FlowID:            plan.Flow.ID,
		MatchedRuleIDs:    []string{matchedRule.RuleKey},
		HitRuleKey:        matchedRule.RuleKey,
		FinishedAt:        finishedAt,
		DurationMS:        durationMS(startedAt, finishedAt),
		Attempts:          attempts,
		RuleMetrics:       metrics,
		ExternalSendQueue: w.sendPublisher != nil,
	}
	if w.sendPublisher != nil {
		if err := w.publishSendEvents(ctx, message.TraceID, attempts); err != nil {
			return err
		}
		if direct {
			w.completePlanningBestEffort(message.TraceID, completeParams)
			return nil
		}
		if strings.TrimSpace(job.ID) == "" {
			w.completePlanningBestEffort(message.TraceID, completeParams)
			return nil
		}
	}
	err = w.repo.CompletePlanning(ctx, completeParams)
	recordTiming(ctx, message.TraceID, TimingComplete, time.Since(completeStartedAt))
	if err != nil {
		return err
	}
	if w.sendPublisher == nil {
		return nil
	}
	return nil
}

func (w *Worker) completePlanningBestEffort(traceID string, params CompletePlanningParams) {
	if w == nil || w.repo == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		startedAt := time.Now()
		if err := w.repo.CompletePlanning(ctx, params); err == nil {
			recordTiming(ctx, traceID, TimingComplete, time.Since(startedAt))
		}
	}()
}

func (w *Worker) routePlan(ctx context.Context, sourceID string) (RoutePlan, error) {
	w.cacheMu.RLock()
	cached, ok := w.routeCache[sourceID]
	w.cacheMu.RUnlock()
	if ok {
		atomic.AddInt64(&w.cacheHits, 1)
		return cached, nil
	}

	loaded, err := w.loadCurrentRoutePlan(ctx, sourceID)
	if err != nil {
		return RoutePlan{}, err
	}
	stored := w.storeRoutePlan(loaded)
	return stored, nil
}

func (w *Worker) RefreshRoutePlan(ctx context.Context, sourceID string) error {
	loaded, err := w.loadCurrentRoutePlan(ctx, sourceID)
	if err != nil {
		if errors.Is(err, ErrNoRoute) {
			w.InvalidateRoutePlan(sourceID)
		}
		return err
	}
	w.storeRoutePlan(loaded)
	return nil
}

func (w *Worker) InvalidateRoutePlan(sourceID string) {
	sourceID = strings.TrimSpace(sourceID)
	if sourceID == "" {
		return
	}
	w.cacheMu.Lock()
	delete(w.routeCache, sourceID)
	w.cacheMu.Unlock()
}

func (w *Worker) CachedRouteSourceIDs() []string {
	w.cacheMu.RLock()
	defer w.cacheMu.RUnlock()
	sourceIDs := make([]string, 0, len(w.routeCache))
	for sourceID := range w.routeCache {
		sourceIDs = append(sourceIDs, sourceID)
	}
	sort.Strings(sourceIDs)
	return sourceIDs
}

func (w *Worker) loadCurrentRoutePlan(ctx context.Context, sourceID string) (RoutePlan, error) {
	ref, err := w.repo.GetCurrentRouteVersionRef(ctx, sourceID)
	if err != nil {
		return RoutePlan{}, err
	}
	loadStartedAt := time.Now()
	loaded, err := w.repo.LoadRoutePlan(ctx, ref.SourceID, ref.VersionID)
	if err != nil {
		return RoutePlan{}, err
	}
	w.hydrateRoutePlanTargetResources(ctx, &loaded)
	atomic.AddInt64(&w.cacheMisses, 1)
	atomic.AddInt64(&w.cacheLoadTimeMS, int64(time.Since(loadStartedAt).Milliseconds()))
	return loaded, nil
}

func (w *Worker) hydrateRoutePlanTargetResources(ctx context.Context, plan *RoutePlan) {
	if w == nil || w.repo == nil || plan == nil {
		return
	}
	if plan.Channels == nil {
		plan.Channels = make(map[string]provider.Channel)
	}
	if plan.TemplateVersions == nil {
		plan.TemplateVersions = make(map[string]msgtemplate.TemplateVersion)
	}
	if plan.Capabilities == nil {
		plan.Capabilities = make(map[string]provider.Capability)
	}
	for _, rule := range plan.Rules {
		for _, target := range enabledActionTargets(rule.Action) {
			channel, ok := plan.Channels[target.ChannelID]
			if !ok {
				loaded, err := w.repo.GetChannel(ctx, target.ChannelID)
				if err != nil {
					continue
				}
				channel = loaded
				plan.Channels[target.ChannelID] = channel
			}

			templateVersion, ok := plan.TemplateVersions[target.TemplateVersionID]
			if !ok {
				loaded, err := w.repo.GetTemplateVersion(ctx, target.TemplateVersionID)
				if err != nil {
					continue
				}
				templateVersion = loaded
				plan.TemplateVersions[target.TemplateVersionID] = templateVersion
			}

			key := capabilityCacheKey(channel.ProviderType, templateVersion.MessageType)
			if _, ok := plan.Capabilities[key]; ok {
				continue
			}
			capability, err := w.repo.GetProviderCapability(ctx, channel.ProviderType, templateVersion.MessageType)
			if err != nil {
				continue
			}
			plan.Capabilities[key] = capability
		}
	}
}

func (w *Worker) storeRoutePlan(plan RoutePlan) RoutePlan {
	sourceID := strings.TrimSpace(plan.Flow.SourceID)
	if sourceID == "" {
		return plan
	}
	prepared, err := prepareRoutePlanSnapshot(plan)
	if err == nil {
		plan = prepared
	} else {
		plan.Rules = sortRoutePlanRules(plan.Rules)
		plan.PreparedRules = nil
	}
	w.cacheMu.Lock()
	w.routeCache[sourceID] = plan
	w.cacheMu.Unlock()
	return plan
}

func (w *Worker) CacheStats() RouteCacheStats {
	return RouteCacheStats{
		Hits:       atomic.LoadInt64(&w.cacheHits),
		Misses:     atomic.LoadInt64(&w.cacheMisses),
		LoadTimeMS: atomic.LoadInt64(&w.cacheLoadTimeMS),
	}
}

func evaluateRules(plan RoutePlan, payload map[string]any) (*route.Rule, []RuleMetric, error) {
	scope := map[string]any{"payload": payload}
	preparedRules := plan.PreparedRules
	if len(preparedRules) == 0 {
		prepared, err := prepareRoutePlanSnapshot(plan)
		if err != nil {
			return nil, nil, err
		}
		preparedRules = prepared.PreparedRules
	}

	metrics := make([]RuleMetric, 0, len(preparedRules))
	for _, prepared := range preparedRules {
		rule := prepared.Rule
		if !rule.Enabled {
			continue
		}
		coarse, err := prepared.Condition.CoarseFilter(scope)
		if err != nil {
			return nil, metrics, err
		}
		if coarse.Skipped {
			metrics = append(metrics, RuleMetric{
				SourceID:       plan.Flow.SourceID,
				FlowID:         plan.Flow.ID,
				RouteVersionID: plan.Version.ID,
				RuleID:         rule.ID,
				Evaluated:      false,
				Matched:        false,
				DurationMS:     0,
			})
			continue
		}
		startedAt := time.Now()
		matched, err := prepared.Condition.Evaluate(scope, plan.MatchGroups)
		duration := int(time.Since(startedAt).Milliseconds())
		metrics = append(metrics, RuleMetric{
			SourceID:       plan.Flow.SourceID,
			FlowID:         plan.Flow.ID,
			RouteVersionID: plan.Version.ID,
			RuleID:         rule.ID,
			Evaluated:      true,
			Matched:        matched,
			DurationMS:     duration,
		})
		if err != nil {
			return nil, metrics, err
		}
		if matched {
			matchedRule := rule
			return &matchedRule, metrics, nil
		}
	}
	return nil, metrics, nil
}

func prepareRoutePlanSnapshot(plan RoutePlan) (RoutePlan, error) {
	plan.Rules = sortRoutePlanRules(plan.Rules)
	plan.PreparedRules = make([]PreparedRouteRule, 0, len(plan.Rules))
	for _, rule := range plan.Rules {
		prepared, err := route.PrepareConditionTree(rule.ConditionTree)
		if err != nil {
			return RoutePlan{}, err
		}
		plan.PreparedRules = append(plan.PreparedRules, PreparedRouteRule{
			Rule:      rule,
			Condition: prepared,
		})
	}
	return plan, nil
}

func sortRoutePlanRules(rules []route.Rule) []route.Rule {
	sorted := append([]route.Rule(nil), rules...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].SortOrder == sorted[j].SortOrder {
			return sorted[i].RuleKey < sorted[j].RuleKey
		}
		return sorted[i].SortOrder < sorted[j].SortOrder
	})
	return sorted
}

func (w *Worker) buildAttempts(ctx context.Context, message MessageRecord, plan RoutePlan, rule route.Rule, payload map[string]any, lifecycle *planningLifecycleBreakdown) ([]DeliveryAttemptPlan, error) {
	targets := enabledActionTargets(rule.Action)
	if len(targets) == 0 {
		return nil, fmt.Errorf("route rule %s has no delivery targets", rule.RuleKey)
	}

	attempts := make([]DeliveryAttemptPlan, 0, len(targets))
	for _, target := range targets {
		channel, err := w.channelForPlan(ctx, plan, target.ChannelID)
		if err != nil {
			return nil, err
		}
		if !channel.Enabled {
			return nil, fmt.Errorf("delivery channel %s is disabled", target.ChannelID)
		}

		templateVersion, err := w.templateVersionForPlan(ctx, plan, target.TemplateVersionID)
		if err != nil {
			return nil, fmt.Errorf("%w: load template version %s: %w", errTemplatePlanning, target.TemplateVersionID, err)
		}
		templateProviderType := strings.TrimSpace(templateVersion.TargetProviderType)
		if templateProviderType != "" && templateProviderType != string(channel.ProviderType) {
			return nil, fmt.Errorf("template %s targets provider %s but channel %s is %s", templateVersion.ID, templateProviderType, channel.ID, channel.ProviderType)
		}
		templateStartedAt := time.Now()
		body, err := renderTemplate(templateVersion, message, payload, w.now())
		templateDuration := time.Since(templateStartedAt)
		recordTiming(ctx, message.TraceID, TimingTemplateRender, templateDuration)
		if lifecycle != nil {
			lifecycle.TemplateRenderFinishedAt = w.now().UTC()
			lifecycle.TemplateRenderDurationMS += int(templateDuration.Milliseconds())
		}
		if err != nil {
			return nil, fmt.Errorf("%w: render template version %s: %w", errTemplatePlanning, templateVersion.ID, err)
		}

		capability, err := w.capabilityForPlan(ctx, plan, channel.ProviderType, templateVersion.MessageType)
		if err != nil {
			return nil, err
		}
		recipientValue, err := w.resolveRecipient(ctx, rule.Action.RecipientStrategy, payload, channel, capability)
		if err != nil {
			return nil, err
		}
		if isEmptyValue(recipientValue) && capability.RecipientRequired && !capability.AllowNoRecipient {
			return nil, fmt.Errorf("%w: recipient is required for %s/%s", errRecipientResolution, channel.ProviderType, templateVersion.MessageType)
		}

		dedupeKey, dedupeTTL := resolveDedupe(rule.Action.SendDedupeConfig, message, payload)
		attemptID := uuid.NewString()
		jobPayload, err := json.Marshal(delivery.SendMessageJobPayload{
			DeliveryAttemptID: attemptID,
			MessageID:         message.ID,
			SourceID:          message.SourceID,
			ChannelID:         channel.ID,
			TemplateVersionID: templateVersion.ID,
			DedupeKey:         dedupeKey,
			DedupeTTLSeconds:  dedupeTTL,
			MessageType:       templateVersion.MessageType,
			TraceID:           message.TraceID,
			Recipient:         recipientValue,
			Body:              body,
			InboundPayload:    append(json.RawMessage(nil), message.Payload...),
			InboundHeaders:    append(json.RawMessage(nil), message.Headers...),
			InboundReceivedAt: message.ReceivedAt,
		})
		if err != nil {
			return nil, fmt.Errorf("%w: encode send message job payload: %w", errJobPlanning, err)
		}
		recipientSnapshot, err := json.Marshal(map[string]any{
			"strategy":  snapshotJSON(rule.Action.RecipientStrategy),
			"recipient": recipientValue,
		})
		if err != nil {
			return nil, fmt.Errorf("%w: encode recipient snapshot: %w", errJobPlanning, err)
		}
		attempts = append(attempts, DeliveryAttemptPlan{
			ID:                attemptID,
			MessageID:         message.ID,
			SourceID:          message.SourceID,
			ChannelID:         channel.ID,
			ProviderType:      string(channel.ProviderType),
			TemplateVersionID: templateVersion.ID,
			RecipientSnapshot: recipientSnapshot,
			JobPayload:        jobPayload,
			MaxAttempts:       maxAttemptsFrom(channel.RetryPolicy),
			DedupeKey:         dedupeKey,
			DedupeTTLSeconds:  dedupeTTL,
			InboundHeaders:    append(json.RawMessage(nil), message.Headers...),
			InboundPayload:    append(json.RawMessage(nil), message.Payload...),
			InboundReceivedAt: message.ReceivedAt,
		})
	}
	return attempts, nil
}

func (w *Worker) channelForPlan(ctx context.Context, plan RoutePlan, channelID string) (provider.Channel, error) {
	channelID = strings.TrimSpace(channelID)
	if channelID == "" {
		return provider.Channel{}, ErrInvalidInput
	}
	if plan.Channels != nil {
		if channel, ok := plan.Channels[channelID]; ok {
			return channel, nil
		}
	}
	return w.repo.GetChannel(ctx, channelID)
}

func (w *Worker) templateVersionForPlan(ctx context.Context, plan RoutePlan, templateVersionID string) (msgtemplate.TemplateVersion, error) {
	templateVersionID = strings.TrimSpace(templateVersionID)
	if templateVersionID == "" {
		return msgtemplate.TemplateVersion{}, ErrInvalidInput
	}
	if plan.TemplateVersions != nil {
		if templateVersion, ok := plan.TemplateVersions[templateVersionID]; ok {
			return templateVersion, nil
		}
	}
	return w.repo.GetTemplateVersion(ctx, templateVersionID)
}

func (w *Worker) capabilityForPlan(ctx context.Context, plan RoutePlan, providerType provider.ProviderType, messageType string) (provider.Capability, error) {
	key := capabilityCacheKey(providerType, messageType)
	if plan.Capabilities != nil {
		if capability, ok := plan.Capabilities[key]; ok {
			return capability, nil
		}
	}
	return w.repo.GetProviderCapability(ctx, providerType, messageType)
}

func annotateAttemptLifecycle(attempts []DeliveryAttemptPlan, lifecycle planningLifecycleBreakdown, at time.Time) ([]DeliveryAttemptPlan, error) {
	for index := range attempts {
		var payload delivery.SendMessageJobPayload
		if err := json.Unmarshal(attempts[index].JobPayload, &payload); err != nil {
			return nil, fmt.Errorf("%w: decode send message job payload: %w", errJobPlanning, err)
		}
		payload.RoutePlanStartedAt = lifecycle.RoutePlanStartedAt
		payload.RouteConditionFinishedAt = lifecycle.RouteConditionFinishedAt
		payload.RouteConditionDurationMS = lifecycle.RouteConditionDurationMS
		payload.TemplateRenderFinishedAt = lifecycle.TemplateRenderFinishedAt
		payload.TemplateRenderDurationMS = lifecycle.TemplateRenderDurationMS
		payload.SendEventBuiltAt = lifecycle.SendEventBuiltAt
		payload.SendEventBuildDurationMS = lifecycle.SendEventBuildDurationMS
		payload.RoutePlannedAt = at
		payload.DeliveryCreatedAt = at
		raw, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("%w: encode send message job payload lifecycle: %w", errJobPlanning, err)
		}
		attempts[index].JobPayload = raw
	}
	return attempts, nil
}

func capabilityCacheKey(providerType provider.ProviderType, messageType string) string {
	return string(providerType) + "\x00" + strings.TrimSpace(messageType)
}

func (w *Worker) publishSendEvents(ctx context.Context, traceID string, attempts []DeliveryAttemptPlan) error {
	events := make([]queue.SendMessageEvent, 0, len(attempts))
	for _, attempt := range attempts {
		events = append(events, queue.SendMessageEvent{
			DeliveryAttemptID: strings.TrimSpace(attempt.ID),
			MessageID:         strings.TrimSpace(attempt.MessageID),
			SourceID:          strings.TrimSpace(attempt.SourceID),
			ChannelID:         strings.TrimSpace(attempt.ChannelID),
			ProviderType:      strings.TrimSpace(attempt.ProviderType),
			TraceID:           strings.TrimSpace(traceID),
			MaxAttempts:       attempt.MaxAttempts,
			Payload:           append(json.RawMessage(nil), attempt.JobPayload...),
		})
	}
	if batchPublisher, ok := w.sendPublisher.(SendBatchPublisher); ok {
		_, err := batchPublisher.PublishSendBatch(ctx, events)
		return err
	}
	var err error
	for _, event := range events {
		if _, itemErr := w.sendPublisher.PublishSend(ctx, event); itemErr != nil {
			err = errors.Join(err, itemErr)
		}
	}
	return err
}

func enabledActionTargets(action route.Action) []route.ActionTarget {
	if len(action.Targets) > 0 {
		targets := make([]route.ActionTarget, 0, len(action.Targets))
		for _, target := range action.Targets {
			if !target.Enabled {
				continue
			}
			target.ChannelID = strings.TrimSpace(target.ChannelID)
			target.TemplateVersionID = strings.TrimSpace(target.TemplateVersionID)
			if target.ChannelID == "" || target.TemplateVersionID == "" {
				continue
			}
			targets = append(targets, target)
		}
		return targets
	}

	templateVersionID := strings.TrimSpace(action.TemplateVersionID)
	if templateVersionID == "" {
		return nil
	}
	channelIDs := cleanStrings(action.ChannelIDs)
	targets := make([]route.ActionTarget, 0, len(channelIDs))
	for index, channelID := range channelIDs {
		targets = append(targets, route.ActionTarget{
			ChannelID:         channelID,
			TemplateVersionID: templateVersionID,
			Enabled:           true,
			SortOrder:         (index + 1) * 10,
		})
	}
	return targets
}

func (w *Worker) resolveRecipient(ctx context.Context, raw json.RawMessage, payload map[string]any, channel provider.Channel, capability provider.Capability) (any, error) {
	strategy := decodeRecipientStrategy(raw)
	mode := strategy.mode()
	if mode == "" || mode == "none" {
		return nil, nil
	}
	if mode == "payload" {
		path := firstNonEmpty(strategy.Path, strategy.PayloadPath, strategy.PayloadRecipientPath)
		value, ok := lookupPath(map[string]any{"payload": payload}, path)
		if !ok || isEmptyValue(value) {
			return nil, fmt.Errorf("%w: payload recipient path %q is empty", errRecipientResolution, path)
		}
		return value, nil
	}
	if mode == "system" {
		if len(strategy.IdentityValues) > 0 {
			return append([]string(nil), strategy.IdentityValues...), nil
		}
		if !isEmptyValue(strategy.Recipients) {
			return strategy.Recipients, nil
		}
		hasSelectors := len(strategy.UserIDs) > 0 ||
			len(strategy.OrgIDs) > 0 ||
			len(strategy.RecipientGroupIDs) > 0 ||
			len(strategy.GroupIDs) > 0
		if !hasSelectors {
			if capability.RecipientRequired && !capability.AllowNoRecipient {
				return nil, fmt.Errorf("%w: system recipient strategy has no selected recipients", errRecipientResolution)
			}
			return nil, nil
		}
		values, err := w.repo.ResolveSystemRecipients(ctx, ResolveSystemRecipientsParams{
			ProviderType:      channel.ProviderType,
			ChannelID:         channel.ID,
			IdentityKind:      capability.IdentityKind,
			UserIDs:           strategy.UserIDs,
			OrgIDs:            strategy.OrgIDs,
			RecipientGroupIDs: append(cleanStrings(strategy.RecipientGroupIDs), cleanStrings(strategy.GroupIDs)...),
			ExcludedUserIDs:   strategy.ExcludedUserIDs,
			ExcludedOrgIDs:    strategy.ExcludedOrgIDs,
		})
		if err != nil {
			return nil, fmt.Errorf("%w: %v", errRecipientResolution, err)
		}
		if len(values) == 0 {
			return nil, fmt.Errorf("%w: system recipient strategy resolved no identities", errRecipientResolution)
		}
		return values, nil
	}
	return nil, fmt.Errorf("%w: unknown recipient strategy mode %q", errRecipientResolution, mode)
}

func (w *Worker) finishBusinessFailure(ctx context.Context, job queue.Job, message MessageRecord, flowID string, matchedRuleIDs []string, code string, cause error, startedAt time.Time, metrics []RuleMetric) error {
	status := "failed"
	if code == ErrorCodeNoRoute {
		status = "no_route"
	}
	finishedAt := w.now()
	return w.repo.FinishPlanning(ctx, FinishPlanningParams{
		JobID:          job.ID,
		WorkerID:       w.workerID,
		MessageID:      message.ID,
		SourceID:       message.SourceID,
		TraceID:        message.TraceID,
		Headers:        append(json.RawMessage(nil), message.Headers...),
		Payload:        append(json.RawMessage(nil), message.Payload...),
		ReceivedAt:     message.ReceivedAt,
		Status:         status,
		FlowID:         flowID,
		MatchedRuleIDs: matchedRuleIDs,
		ErrorCode:      code,
		ErrorMessage:   cause.Error(),
		FinishedAt:     finishedAt,
		DurationMS:     durationMS(startedAt, finishedAt),
		RuleMetrics:    metrics,
	})
}

func (w *Worker) failJob(ctx context.Context, job queue.Job, code string, cause error) error {
	if strings.TrimSpace(job.ID) == "" {
		return cause
	}
	_, failErr := w.repo.FailJob(ctx, queue.FailParams{
		JobID:        job.ID,
		WorkerID:     w.workerID,
		ErrorCode:    code,
		ErrorMessage: cause.Error(),
		RetryAt:      w.now().Add(w.retryDelay),
		Now:          w.now(),
	})
	return errors.Join(cause, failErr)
}

func routePlanJobFromEvent(event queue.RoutePlanEvent) (queue.Job, error) {
	if err := event.Validate(); err != nil {
		return queue.Job{}, err
	}
	payload, err := json.Marshal(RoutePlanJobPayload{
		MessageID: strings.TrimSpace(event.MessageID),
		SourceID:  strings.TrimSpace(event.SourceID),
		TraceID:   strings.TrimSpace(event.TraceID),
	})
	if err != nil {
		return queue.Job{}, err
	}
	return queue.Job{
		Type:        queue.JobTypeRoutePlan,
		Status:      queue.JobStatusProcessing,
		Payload:     payload,
		RunAt:       time.Now().UTC(),
		MaxAttempts: 1,
		QueueKey:    strings.TrimSpace(event.SourceID),
	}, nil
}

func nakRoutePlanMessage(message queue.RoutePlanMessage, delay time.Duration) error {
	if message.Nak == nil {
		return nil
	}
	if delay <= 0 {
		delay = time.Minute
	}
	return message.Nak(delay)
}

func decodeRoutePlanPayload(raw json.RawMessage) (RoutePlanJobPayload, error) {
	var payload RoutePlanJobPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return RoutePlanJobPayload{}, fmt.Errorf("decode route_plan job payload: %w", err)
	}
	return payload, nil
}

func renderTemplate(version msgtemplate.TemplateVersion, message MessageRecord, payload map[string]any, now time.Time) (json.RawMessage, error) {
	rendered, err := msgtemplate.DefaultTemplateEngine().Render(version.TemplateBody, map[string]any{
		"payload": payload,
		"message": map[string]any{
			"id":       message.ID,
			"trace_id": message.TraceID,
		},
		"source": map[string]any{
			"id": message.SourceID,
		},
		"now": now.UTC().Format(time.RFC3339),
	})
	if err != nil {
		return nil, err
	}
	var compacted bytes.Buffer
	if err := json.Compact(&compacted, []byte(rendered)); err != nil {
		return nil, fmt.Errorf("rendered template is not valid json: %w", err)
	}
	return append(json.RawMessage(nil), compacted.Bytes()...), nil
}

func decodeJSONObject(raw json.RawMessage) (map[string]any, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return map[string]any{}, nil
	}
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	if value == nil {
		return map[string]any{}, nil
	}
	return value, nil
}

type recipientStrategy struct {
	Mode                 string   `json:"mode"`
	Type                 string   `json:"type"`
	RecipientMode        string   `json:"recipient_mode"`
	Path                 string   `json:"path"`
	PayloadPath          string   `json:"payload_path"`
	PayloadRecipientPath string   `json:"payload_recipient_path"`
	UserIDs              []string `json:"user_ids"`
	OrgIDs               []string `json:"org_ids"`
	RecipientGroupIDs    []string `json:"recipient_group_ids"`
	GroupIDs             []string `json:"group_ids"`
	ExcludedUserIDs      []string `json:"excluded_user_ids"`
	ExcludedOrgIDs       []string `json:"excluded_org_ids"`
	IdentityValues       []string `json:"identity_values"`
	Recipients           any      `json:"recipients"`
}

func decodeRecipientStrategy(raw json.RawMessage) recipientStrategy {
	var strategy recipientStrategy
	if len(bytes.TrimSpace(raw)) == 0 {
		return strategy
	}
	_ = json.Unmarshal(raw, &strategy)
	return strategy
}

func (s recipientStrategy) mode() string {
	mode := strings.ToLower(strings.TrimSpace(firstNonEmpty(s.Mode, s.Type, s.RecipientMode)))
	if mode != "" {
		return mode
	}
	if firstNonEmpty(s.Path, s.PayloadPath, s.PayloadRecipientPath) != "" {
		return "payload"
	}
	if len(s.UserIDs) > 0 || len(s.OrgIDs) > 0 || len(s.RecipientGroupIDs) > 0 || len(s.GroupIDs) > 0 || len(s.IdentityValues) > 0 || !isEmptyValue(s.Recipients) {
		return "system"
	}
	return ""
}

type sendDedupeConfig struct {
	Enabled    bool   `json:"enabled"`
	Strategy   string `json:"strategy"`
	Key        string `json:"key"`
	DedupeKey  string `json:"dedupe_key"`
	KeyPath    string `json:"key_path"`
	TTLSeconds int    `json:"ttl_seconds"`
}

func resolveDedupe(raw json.RawMessage, message MessageRecord, payload map[string]any) (string, int) {
	var config sendDedupeConfig
	if len(bytes.TrimSpace(raw)) == 0 {
		return "", 0
	}
	if err := json.Unmarshal(raw, &config); err != nil {
		return "", 0
	}
	strategy := strings.ToLower(strings.TrimSpace(config.Strategy))
	if !config.Enabled && strategy == "" && strings.TrimSpace(config.Key) == "" && strings.TrimSpace(config.DedupeKey) == "" && strings.TrimSpace(config.KeyPath) == "" {
		return "", 0
	}
	if strategy == "trace_id" {
		return strings.TrimSpace(message.TraceID), config.TTLSeconds
	}
	key := firstNonEmpty(config.Key, config.DedupeKey)
	if config.KeyPath != "" {
		if value, ok := lookupPath(map[string]any{"payload": payload}, config.KeyPath); ok {
			key = fmt.Sprint(value)
		}
	}
	return strings.TrimSpace(key), config.TTLSeconds
}

type retryPolicy struct {
	MaxAttempts int `json:"max_attempts"`
}

func maxAttemptsFrom(raw json.RawMessage) int {
	var policy retryPolicy
	if len(bytes.TrimSpace(raw)) > 0 {
		_ = json.Unmarshal(raw, &policy)
	}
	if policy.MaxAttempts <= 0 {
		return 3
	}
	return policy.MaxAttempts
}

func lookupPath(scope map[string]any, path string) (any, bool) {
	path = strings.TrimSpace(strings.TrimPrefix(path, "$."))
	if path == "" {
		return nil, false
	}
	parts := strings.Split(path, ".")
	var current any = scope
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		mapped, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = mapped[part]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

var (
	errRecipientResolution = errors.New("recipient resolution failed")
	errTemplatePlanning    = errors.New("template planning failed")
	errJobPlanning         = errors.New("job planning failed")
)

func planningErrorCode(err error) string {
	switch {
	case errors.Is(err, errTemplatePlanning):
		return ErrorCodeTemplate
	case errors.Is(err, errRecipientResolution):
		return ErrorCodeRecipient
	case errors.Is(err, errJobPlanning):
		return ErrorCodeJob
	default:
		return ErrorCodeChannel
	}
}

func isEmptyValue(value any) bool {
	switch typed := value.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(typed) == ""
	case []string:
		return len(typed) == 0
	case []any:
		return len(typed) == 0
	default:
		return false
	}
}

func cleanStrings(values []string) []string {
	result := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func snapshotJSON(raw json.RawMessage) any {
	if len(bytes.TrimSpace(raw)) == 0 {
		return map[string]any{}
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return string(raw)
	}
	return value
}

func durationMS(startedAt time.Time, finishedAt time.Time) int {
	if finishedAt.Before(startedAt) {
		return 0
	}
	return int(finishedAt.Sub(startedAt).Milliseconds())
}
