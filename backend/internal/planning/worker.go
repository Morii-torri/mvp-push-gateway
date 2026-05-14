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
	"time"

	"github.com/google/uuid"

	"mvp-push-gateway/backend/internal/delivery"
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
	ID        string
	TraceID   string
	SourceID  string
	Headers   json.RawMessage
	Payload   json.RawMessage
	Status    string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type RouteVersionRef struct {
	SourceID  string
	FlowID    string
	VersionID string
}

type RoutePlan struct {
	Flow        route.Flow
	Version     route.Version
	Rules       []route.Rule
	MatchGroups map[string][]string
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

type DeliveryAttemptPlan struct {
	ID                string
	MessageID         string
	ChannelID         string
	TemplateVersionID string
	RecipientSnapshot json.RawMessage
	JobPayload        json.RawMessage
	MaxAttempts       int
	DedupeKey         string
	DedupeTTLSeconds  int
}

type CompletePlanningParams struct {
	JobID          string
	WorkerID       string
	MessageID      string
	FlowID         string
	MatchedRuleIDs []string
	HitRuleKey     string
	FinishedAt     time.Time
	DurationMS     int
	Attempts       []DeliveryAttemptPlan
	RuleMetrics    []RuleMetric
}

type FinishPlanningParams struct {
	JobID          string
	WorkerID       string
	MessageID      string
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
	GetTemplateVersion(context.Context, string) (msgtemplate.TemplateVersion, error)
	GetChannel(context.Context, string) (provider.Channel, error)
	GetProviderCapability(context.Context, provider.ProviderType, string) (provider.Capability, error)
	ResolveSystemRecipients(context.Context, ResolveSystemRecipientsParams) ([]string, error)

	CompletePlanning(context.Context, CompletePlanningParams) error
	FinishPlanning(context.Context, FinishPlanningParams) error
}

type Worker struct {
	repo       Repository
	workerID   string
	now        func() time.Time
	retryDelay time.Duration

	cacheMu    sync.RWMutex
	routeCache map[string]RoutePlan
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
	jobs, err := w.repo.ClaimJobs(ctx, queue.ClaimParams{
		WorkerID: w.workerID,
		Types:    []queue.JobType{queue.JobTypeRoutePlan},
		Limit:    limit,
		Now:      now,
	})
	if err != nil {
		return 0, err
	}

	var firstErr error
	for _, job := range jobs {
		if err := w.ProcessOne(ctx, job); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return len(jobs), firstErr
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
	payloadMap, err := decodeJSONObject(message.Payload)
	if err != nil {
		return w.finishBusinessFailure(ctx, job, message, "", nil, ErrorCodeJob, err, startedAt, nil)
	}

	plan, err := w.routePlan(ctx, message.SourceID)
	if err != nil {
		if errors.Is(err, ErrNoRoute) {
			return w.finishBusinessFailure(ctx, job, message, "", nil, ErrorCodeNoRoute, err, startedAt, nil)
		}
		return w.failJob(ctx, job, ErrorCodeRoute, err)
	}

	matchedRule, metrics, err := evaluateRules(plan, payloadMap)
	if err != nil {
		return w.finishBusinessFailure(ctx, job, message, plan.Flow.ID, nil, ErrorCodeRoute, err, startedAt, metrics)
	}
	if matchedRule == nil {
		return w.finishBusinessFailure(ctx, job, message, plan.Flow.ID, nil, ErrorCodeNoRoute, ErrNoRoute, startedAt, metrics)
	}

	attempts, err := w.buildAttempts(ctx, message, *matchedRule, payloadMap)
	if err != nil {
		return w.finishBusinessFailure(ctx, job, message, plan.Flow.ID, []string{matchedRule.RuleKey}, planningErrorCode(err), err, startedAt, metrics)
	}

	finishedAt := w.now()
	return w.repo.CompletePlanning(ctx, CompletePlanningParams{
		JobID:          job.ID,
		WorkerID:       w.workerID,
		MessageID:      message.ID,
		FlowID:         plan.Flow.ID,
		MatchedRuleIDs: []string{matchedRule.RuleKey},
		HitRuleKey:     matchedRule.RuleKey,
		FinishedAt:     finishedAt,
		DurationMS:     durationMS(startedAt, finishedAt),
		Attempts:       attempts,
		RuleMetrics:    metrics,
	})
}

func (w *Worker) routePlan(ctx context.Context, sourceID string) (RoutePlan, error) {
	ref, err := w.repo.GetCurrentRouteVersionRef(ctx, sourceID)
	if err != nil {
		return RoutePlan{}, err
	}
	key := ref.SourceID + ":" + ref.VersionID

	w.cacheMu.RLock()
	cached, ok := w.routeCache[key]
	w.cacheMu.RUnlock()
	if ok {
		return cached, nil
	}

	loaded, err := w.repo.LoadRoutePlan(ctx, ref.SourceID, ref.VersionID)
	if err != nil {
		return RoutePlan{}, err
	}
	w.cacheMu.Lock()
	w.routeCache[key] = loaded
	w.cacheMu.Unlock()
	return loaded, nil
}

func evaluateRules(plan RoutePlan, payload map[string]any) (*route.Rule, []RuleMetric, error) {
	scope := map[string]any{"payload": payload}
	rules := append([]route.Rule(nil), plan.Rules...)
	sort.SliceStable(rules, func(i, j int) bool {
		if rules[i].SortOrder == rules[j].SortOrder {
			return rules[i].RuleKey < rules[j].RuleKey
		}
		return rules[i].SortOrder < rules[j].SortOrder
	})

	metrics := make([]RuleMetric, 0, len(rules))
	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		startedAt := time.Now()
		matched, err := route.EvaluateConditionTreeWithMatchGroups(rule.ConditionTree, scope, plan.MatchGroups)
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

func (w *Worker) buildAttempts(ctx context.Context, message MessageRecord, rule route.Rule, payload map[string]any) ([]DeliveryAttemptPlan, error) {
	targets := enabledActionTargets(rule.Action)
	if len(targets) == 0 {
		return nil, fmt.Errorf("route rule %s has no delivery targets", rule.RuleKey)
	}

	attempts := make([]DeliveryAttemptPlan, 0, len(targets))
	for _, target := range targets {
		channel, err := w.repo.GetChannel(ctx, target.ChannelID)
		if err != nil {
			return nil, err
		}
		if !channel.Enabled {
			return nil, fmt.Errorf("delivery channel %s is disabled", target.ChannelID)
		}

		templateVersion, err := w.repo.GetTemplateVersion(ctx, target.TemplateVersionID)
		if err != nil {
			return nil, fmt.Errorf("%w: load template version %s: %w", errTemplatePlanning, target.TemplateVersionID, err)
		}
		templateProviderType := strings.TrimSpace(templateVersion.TargetProviderType)
		if templateProviderType != "" && templateProviderType != string(channel.ProviderType) {
			return nil, fmt.Errorf("template %s targets provider %s but channel %s is %s", templateVersion.ID, templateProviderType, channel.ID, channel.ProviderType)
		}
		body, err := renderTemplate(templateVersion, message, payload, w.now())
		if err != nil {
			return nil, fmt.Errorf("%w: render template version %s: %w", errTemplatePlanning, templateVersion.ID, err)
		}

		capability, err := w.repo.GetProviderCapability(ctx, channel.ProviderType, templateVersion.MessageType)
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

		dedupeKey, dedupeTTL := resolveDedupe(rule.Action.SendDedupeConfig, payload)
		attemptID := uuid.NewString()
		jobPayload, err := json.Marshal(delivery.SendMessageJobPayload{
			DeliveryAttemptID: attemptID,
			DedupeKey:         dedupeKey,
			DedupeTTLSeconds:  dedupeTTL,
			MessageType:       templateVersion.MessageType,
			Recipient:         recipientValue,
			Body:              body,
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
			ChannelID:         channel.ID,
			TemplateVersionID: templateVersion.ID,
			RecipientSnapshot: recipientSnapshot,
			JobPayload:        jobPayload,
			MaxAttempts:       maxAttemptsFrom(channel.RetryPolicy),
			DedupeKey:         dedupeKey,
			DedupeTTLSeconds:  dedupeTTL,
		})
	}
	return attempts, nil
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
		values, err := w.repo.ResolveSystemRecipients(ctx, ResolveSystemRecipientsParams{
			ProviderType:      channel.ProviderType,
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
	Key        string `json:"key"`
	DedupeKey  string `json:"dedupe_key"`
	KeyPath    string `json:"key_path"`
	TTLSeconds int    `json:"ttl_seconds"`
}

func resolveDedupe(raw json.RawMessage, payload map[string]any) (string, int) {
	var config sendDedupeConfig
	if len(bytes.TrimSpace(raw)) == 0 {
		return "", 0
	}
	if err := json.Unmarshal(raw, &config); err != nil || !config.Enabled {
		return "", 0
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
