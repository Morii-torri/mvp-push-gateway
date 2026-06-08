package delivery

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"strings"
	"sync"
	"time"

	"mvp-push-gateway/backend/internal/perftiming"
	"mvp-push-gateway/backend/internal/provider"
	"mvp-push-gateway/backend/internal/queue"
)

type Status string

const (
	StatusQueued     Status = "queued"
	StatusProcessing Status = "processing"
	StatusSent       Status = "sent"
	StatusFailed     Status = "failed"
	StatusDeduped    Status = "deduped"
	StatusSkipped    Status = "skipped"
)

const (
	defaultDedupeTTL  = 24 * time.Hour
	defaultRetryDelay = time.Second
)

const maxUpstreamResponseBodyBytes = 64 * 1024

var ErrRetryScheduled = errors.New("delivery retry scheduled")

func defaultDeliveryHTTPClientFactory(timeout time.Duration) *http.Client {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return provider.NewEgressHTTPClient(timeout)
}

type Attempt struct {
	ID                string
	MessageID         string
	SourceID          string
	ChannelID         string
	TemplateVersionID string
	RecipientSnapshot json.RawMessage
	Status            Status
	ErrorCode         string
	ErrorMessage      string
	RequestSnapshot   json.RawMessage
	ResponseSnapshot  json.RawMessage
	DurationMS        int
	AttemptNo         int
	NextRetryAt       *time.Time
	DeadLetteredAt    *time.Time
	QueuedAt          *time.Time
	StartedAt         *time.Time
	FinishedAt        *time.Time
	InboundHeaders    json.RawMessage
	InboundPayload    json.RawMessage
	InboundReceivedAt time.Time
}

type SendMessageJobPayload struct {
	DeliveryAttemptID string          `json:"delivery_attempt_id"`
	MessageID         string          `json:"message_id,omitempty"`
	SourceID          string          `json:"source_id,omitempty"`
	ChannelID         string          `json:"channel_id,omitempty"`
	TemplateVersionID string          `json:"template_version_id,omitempty"`
	RecipientSnapshot json.RawMessage `json:"recipient_snapshot,omitempty"`
	RoutePlannedAt    time.Time       `json:"route_planned_at,omitempty"`
	DeliveryCreatedAt time.Time       `json:"delivery_created_at,omitempty"`
	DedupeKey         string          `json:"dedupe_key"`
	DedupeTTLSeconds  int             `json:"dedupe_ttl_seconds"`
	MessageType       string          `json:"message_type"`
	TraceID           string          `json:"trace_id,omitempty"`
	Token             string          `json:"token"`
	Recipient         any             `json:"recipient"`
	Body              json.RawMessage `json:"body"`
	InboundHeaders    json.RawMessage `json:"inbound_headers,omitempty"`
	InboundPayload    json.RawMessage `json:"inbound_payload,omitempty"`
	InboundReceivedAt time.Time       `json:"inbound_received_at,omitempty"`
}

type TimingStage string

const (
	TimingClaimJobs    TimingStage = "delivery_claim"
	TimingDispatchHTTP TimingStage = "delivery_dispatch"
	TimingSendHTTP     TimingStage = "delivery_send"
	TimingComplete     TimingStage = "delivery_complete"
)

type TimingRecorder interface {
	RecordDeliveryTiming(traceID string, stage TimingStage, duration time.Duration)
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
	recorder.RecordDeliveryTiming(traceID, stage, duration)
}

type DeadLetterRecord struct {
	JobID        string
	ChannelID    string
	ErrorCode    string
	ErrorMessage string
}

type MarkAttemptProcessingParams struct {
	AttemptID string
	AttemptNo int
	StartedAt time.Time
}

type SendDedupeParams struct {
	ChannelID string
	DedupeKey string
	ExpiresAt time.Time
	MessageID string
}

type CompleteDeliveryParams struct {
	JobID             string
	WorkerID          string
	AttemptID         string
	MessageID         string
	SourceID          string
	ChannelID         string
	TemplateVersionID string
	RecipientSnapshot json.RawMessage
	DeliveryCreatedAt time.Time
	TraceID           string
	AttemptNo         int
	Status            Status
	RequestSnapshot   json.RawMessage
	ResponseSnapshot  json.RawMessage
	DurationMS        int
	FinishedAt        time.Time
	InboundHeaders    json.RawMessage
	InboundPayload    json.RawMessage
	InboundReceivedAt time.Time
}

type RetryDeliveryParams struct {
	JobID             string
	WorkerID          string
	AttemptID         string
	MessageID         string
	SourceID          string
	ChannelID         string
	TemplateVersionID string
	RecipientSnapshot json.RawMessage
	DeliveryCreatedAt time.Time
	TraceID           string
	AttemptNo         int
	ErrorCode         string
	ErrorMessage      string
	RequestSnapshot   json.RawMessage
	ResponseSnapshot  json.RawMessage
	DurationMS        int
	RetryAt           time.Time
	FinishedAt        time.Time
	InboundHeaders    json.RawMessage
	InboundPayload    json.RawMessage
	InboundReceivedAt time.Time
}

type DeadLetterDeliveryParams struct {
	JobID             string
	WorkerID          string
	AttemptID         string
	ChannelID         string
	MessageID         string
	SourceID          string
	TemplateVersionID string
	RecipientSnapshot json.RawMessage
	DeliveryCreatedAt time.Time
	TraceID           string
	AttemptNo         int
	ErrorCode         string
	ErrorMessage      string
	RequestSnapshot   json.RawMessage
	ResponseSnapshot  json.RawMessage
	DurationMS        int
	FinishedAt        time.Time
	InboundHeaders    json.RawMessage
	InboundPayload    json.RawMessage
	InboundReceivedAt time.Time
}

type Repository interface {
	ClaimSendJobs(context.Context, queue.ClaimParams) ([]queue.Job, error)
	GetChannel(context.Context, string) (provider.Channel, error)
	GetProviderCapability(context.Context, provider.ProviderType, string) (provider.Capability, error)
	GetAttempt(context.Context, string) (Attempt, error)
	MarkAttemptProcessing(context.Context, MarkAttemptProcessingParams) error
	InsertSendDedupeKey(context.Context, SendDedupeParams) (bool, error)
	CompleteDelivery(context.Context, CompleteDeliveryParams) error
	RetryDelivery(context.Context, RetryDeliveryParams) error
	DeadLetterDelivery(context.Context, DeadLetterDeliveryParams) error
}

type BatchCompleteRepository interface {
	CompleteDeliveries(context.Context, []CompleteDeliveryParams) error
}

type completeDeliveryCollectorKey struct{}

type completeDeliveryCollector struct {
	mu    sync.Mutex
	items []CompleteDeliveryParams
}

func withCompleteDeliveryCollector(ctx context.Context, collector *completeDeliveryCollector) context.Context {
	if collector == nil {
		return ctx
	}
	return context.WithValue(ctx, completeDeliveryCollectorKey{}, collector)
}

func completeDeliveryCollectorFromContext(ctx context.Context) *completeDeliveryCollector {
	collector, _ := ctx.Value(completeDeliveryCollectorKey{}).(*completeDeliveryCollector)
	return collector
}

func (c *completeDeliveryCollector) add(params CompleteDeliveryParams) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = append(c.items, params)
}

func (c *completeDeliveryCollector) snapshot() []CompleteDeliveryParams {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]CompleteDeliveryParams(nil), c.items...)
}

type Worker struct {
	repo              Repository
	workerID          string
	now               func() time.Time
	httpClientFactory func(time.Duration) *http.Client
	buildRequest      func(provider.Channel, provider.BuildDeliveryRequestInput) (provider.BuiltRequest, error)
	resultPublisher   ResultPublisher

	mu         sync.Mutex
	semaphores map[string]chan struct{}
	limiters   map[string]*channelLimiter

	tokenManager *provider.TokenManager
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

func WithHTTPClientFactory(factory func(time.Duration) *http.Client) WorkerOption {
	return func(w *Worker) {
		if factory != nil {
			w.httpClientFactory = factory
		}
	}
}

func WithResultPublisher(publisher ResultPublisher) WorkerOption {
	return func(w *Worker) {
		w.resultPublisher = publisher
	}
}

func NewWorker(repo Repository, opts ...WorkerOption) *Worker {
	worker := &Worker{
		repo:     repo,
		workerID: "delivery-worker",
		now: func() time.Time {
			return time.Now().UTC()
		},
		httpClientFactory: defaultDeliveryHTTPClientFactory,
		buildRequest:      provider.BuildDeliveryRequest,
		semaphores:        map[string]chan struct{}{},
		limiters:          map[string]*channelLimiter{},
	}
	for _, opt := range opts {
		opt(worker)
	}
	var tokenStore provider.TokenCacheStore
	if candidate, ok := repo.(provider.TokenCacheStore); ok {
		tokenStore = candidate
	}
	worker.tokenManager = provider.NewTokenManager(
		tokenStore,
		provider.WithTokenManagerNow(worker.now),
		provider.WithTokenManagerOwner(worker.workerID),
		provider.WithTokenManagerHTTPClientFactory(worker.httpClientFactory),
	)
	return worker
}

func (w *Worker) ProcessBatch(ctx context.Context, limit int) (int, error) {
	if limit <= 0 {
		limit = 1
	}
	now := w.now()
	claimStartedAt := time.Now()
	jobs, err := w.repo.ClaimSendJobs(ctx, queue.ClaimParams{
		WorkerID: w.workerID,
		Types:    []queue.JobType{queue.JobTypeSendMessage},
		Limit:    limit,
		Now:      now,
	})
	if err != nil {
		return 0, err
	}
	claimDuration := time.Since(claimStartedAt)
	for _, job := range jobs {
		if payload, err := decodePayload(job.Payload); err == nil {
			recordTiming(ctx, payload.TraceID, TimingClaimJobs, claimDuration)
		}
	}
	if len(jobs) == 0 {
		return 0, nil
	}

	collector := &completeDeliveryCollector{}
	batchCtx := withCompleteDeliveryCollector(ctx, collector)
	var wg sync.WaitGroup
	var firstErr error
	var errMu sync.Mutex
	for _, job := range jobs {
		job := job
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := w.ProcessOne(batchCtx, job); err != nil {
				errMu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				errMu.Unlock()
			}
		}()
	}
	wg.Wait()
	if err := w.flushCompleteDeliveryCollector(ctx, collector); err != nil {
		if firstErr == nil {
			firstErr = err
		}
	}
	return len(jobs), firstErr
}

func (w *Worker) ProcessSendMessage(ctx context.Context, message queue.SendMessage) error {
	if w == nil || w.repo == nil {
		return errors.New("delivery worker is not configured")
	}
	job, err := sendJobFromEvent(message.Event, message.DeliveryCount)
	if err != nil {
		return errors.Join(err, nakSendMessage(message, defaultRetryDelay))
	}
	if err := w.ProcessOne(ctx, job); err != nil {
		return errors.Join(err, nakSendMessage(message, defaultRetryDelay))
	}
	if message.Ack != nil {
		return message.Ack()
	}
	return nil
}

func (w *Worker) flushCompleteDeliveryCollector(ctx context.Context, collector *completeDeliveryCollector) error {
	params := collector.snapshot()
	if len(params) == 0 {
		return nil
	}
	completeStartedAt := time.Now()
	var err error
	if w.resultPublisher != nil {
		for _, item := range params {
			if itemErr := w.resultPublisher.PublishDeliveryResult(ctx, NewDeliveryResultEvent(item)); itemErr != nil {
				err = errors.Join(err, itemErr)
			}
		}
	} else if batchRepo, ok := w.repo.(BatchCompleteRepository); ok {
		err = batchRepo.CompleteDeliveries(ctx, params)
	} else {
		for _, item := range params {
			if itemErr := w.repo.CompleteDelivery(ctx, item); itemErr != nil {
				err = errors.Join(err, itemErr)
			}
		}
	}
	duration := time.Since(completeStartedAt)
	for _, item := range params {
		recordTiming(ctx, item.TraceID, TimingComplete, duration)
	}
	return err
}

func (w *Worker) ProcessOne(ctx context.Context, job queue.Job) error {
	payload, err := decodePayload(job.Payload)
	if err != nil {
		return err
	}
	if strings.TrimSpace(payload.DeliveryAttemptID) == "" {
		return errors.New("delivery attempt id is required")
	}

	attempt, directAttempt := attemptFromDirectPayload(payload)
	if !directAttempt {
		var err error
		attempt, err = w.repo.GetAttempt(ctx, payload.DeliveryAttemptID)
		if err != nil {
			return fmt.Errorf("load attempt %s: %w", payload.DeliveryAttemptID, err)
		}
	}

	channelID := strings.TrimSpace(job.ChannelID)
	if channelID == "" {
		channelID = strings.TrimSpace(attempt.ChannelID)
	}
	if channelID == "" {
		return fmt.Errorf("job %s missing channel id", job.ID)
	}
	channel, err := w.repo.GetChannel(ctx, channelID)
	if err != nil {
		return fmt.Errorf("load channel %s: %w", channelID, err)
	}
	messageType := strings.TrimSpace(payload.MessageType)
	capability, capabilitySource, err := w.loadCapability(ctx, channel.ProviderType, messageType)
	if err != nil {
		return fmt.Errorf("load provider capability %s/%s: %w", channel.ProviderType, messageType, err)
	}

	release, err := w.acquireSemaphore(ctx, channelID, channel.ConcurrencyLimit)
	if err != nil {
		return err
	}
	defer release()

	if err := w.waitRateLimit(ctx, channelID, channel.RateLimitConfig); err != nil {
		return err
	}

	startedAt := w.now()
	attemptNo := job.Attempts
	if attemptNo <= 0 {
		attemptNo = 1
	}
	if !directAttempt {
		if err := w.repo.MarkAttemptProcessing(ctx, MarkAttemptProcessingParams{
			AttemptID: attempt.ID,
			AttemptNo: attemptNo,
			StartedAt: startedAt,
		}); err != nil {
			return fmt.Errorf("mark attempt processing: %w", err)
		}
	}

	targetContext := provider.DeliveryTargetContext{
		DeliveryAttemptID: attempt.ID,
		MessageID:         attempt.MessageID,
		ChannelID:         channel.ID,
		ChannelName:       channel.Name,
		ProviderType:      string(channel.ProviderType),
		MessageType:       messageType,
		TemplateVersionID: attempt.TemplateVersionID,
		JobID:             job.ID,
	}
	requestSnapshot := map[string]any{
		"target_context":      targetContext,
		"rendered_message":    snapshotValue(payload.Body),
		"resolved_recipients": payload.Recipient,
		"lifecycle":           deliveryLifecycleSnapshot(payload),
		"capability": map[string]any{
			"provider_type": string(channel.ProviderType),
			"message_type":  messageType,
			"source":        capabilitySource,
		},
	}
	responseSnapshot := map[string]any{}

	if dedupeKey := strings.TrimSpace(payload.DedupeKey); dedupeKey != "" {
		effectiveDedupeKey := effectiveSendDedupeKey(dedupeKey, attempt.TemplateVersionID)
		requestSnapshot["dedupe"] = map[string]any{
			"configured_key":      dedupeKey,
			"effective_key":       effectiveDedupeKey,
			"scope":               effectiveDedupeScope(attempt.TemplateVersionID),
			"template_version_id": attempt.TemplateVersionID,
			"dedupe_ttl_seconds":  payload.DedupeTTLSeconds,
		}
		expiresAt := startedAt.Add(defaultDedupeTTL)
		if payload.DedupeTTLSeconds > 0 {
			expiresAt = startedAt.Add(time.Duration(payload.DedupeTTLSeconds) * time.Second)
		}
		inserted, err := w.repo.InsertSendDedupeKey(ctx, SendDedupeParams{
			ChannelID: channelID,
			DedupeKey: effectiveDedupeKey,
			ExpiresAt: expiresAt,
			MessageID: attempt.MessageID,
		})
		if err != nil {
			return fmt.Errorf("insert send dedupe key: %w", err)
		}
		if !inserted {
			responseSnapshot["dedupe"] = map[string]any{
				"deduped": true,
				"source":  "send_dedupe",
			}
			requestRaw, err := marshalSnapshot(requestSnapshot)
			if err != nil {
				return err
			}
			responseRaw, err := marshalSnapshot(responseSnapshot)
			if err != nil {
				return err
			}
			return w.completeDelivery(ctx, CompleteDeliveryParams{
				JobID:            job.ID,
				WorkerID:         w.workerID,
				AttemptID:        attempt.ID,
				TraceID:          payload.TraceID,
				AttemptNo:        attemptNo,
				Status:           StatusDeduped,
				RequestSnapshot:  requestRaw,
				ResponseSnapshot: responseRaw,
				DurationMS:       durationMS(startedAt, w.now()),
				FinishedAt:       w.now(),
			})
		}
	}

	resolvedToken := strings.TrimSpace(payload.Token)

	effectiveChannel := channel
	tokenBehavior, err := parseTokenBehavior(channel, capability, capabilitySource)
	if err != nil {
		return w.failAttempt(ctx, job, attempt, attemptNo, startedAt, "MGP-TOKEN-001", err.Error(), requestSnapshot, responseSnapshot, retryPolicyFrom(channel.RetryPolicy), retryableFailure())
	}
	requestSnapshot["token_behavior"] = tokenBehavior.snapshot()
	if len(tokenBehavior.Placement) > 0 {
		effectiveChannel.TokenConfig = tokenBehavior.Placement
	}

	var resolvedTokenCacheKey string
	if tokenBehavior.Resolver != nil {
		resolution, err := w.tokenManager.ResolveWithResolver(ctx, provider.TokenResolveInput{
			Capability:   capability,
			Channel:      channel,
			Resolver:     *tokenBehavior.Resolver,
			Strategy:     tokenBehavior.Strategy,
			ForceRefresh: false,
		})
		requestSnapshot["token_exchange"] = resolution.RequestSnapshot
		responseSnapshot["token_exchange"] = resolution.ResponseSnapshot
		if err != nil {
			return w.failAttempt(ctx, job, attempt, attemptNo, startedAt, "MGP-TOKEN-002", err.Error(), requestSnapshot, responseSnapshot, retryPolicyFrom(channel.RetryPolicy), retryableFailure())
		}
		resolvedToken = resolution.Token
		resolvedTokenCacheKey = resolution.CacheKey
	}

	builtRequest, err := w.buildRequest(effectiveChannel, provider.BuildDeliveryRequestInput{
		Token: resolvedToken,
		RenderedMessage: provider.RenderedMessage{
			ProviderType: channel.ProviderType,
			MessageType:  messageType,
			Content:      payload.Body,
		},
		ResolvedRecipients: provider.ResolvedRecipientsFromValue(payload.Recipient),
		TargetContext:      targetContext,
	})
	if err != nil {
		return w.failAttempt(ctx, job, attempt, attemptNo, startedAt, "MGP-SEND-001", err.Error(), requestSnapshot, responseSnapshot, retryPolicyFrom(channel.RetryPolicy), retryableFailure())
	}

	requestSnapshot["final_request"] = redactedBuiltRequestSnapshot(builtRequest, nil)
	requestSnapshot["send"] = redactedBuiltRequestSnapshot(builtRequest, payload.Recipient)

	sendStartedAt := time.Now()
	setSnapshotTime(requestSnapshot, "lifecycle", "request_started_at", sendStartedAt)
	statusCode, responseHeaders, responseBody, responseBodyTruncated, sendErr := w.send(ctx, channel, builtRequest, payload.TraceID, sendStartedAt)
	sendFinishedAt := time.Now()
	recordTiming(ctx, payload.TraceID, TimingSendHTTP, sendFinishedAt.Sub(sendStartedAt))
	if sendErr == nil && tokenBehavior.Resolver != nil && shouldRefreshToken(responseBody, capability, capabilitySource, tokenBehavior.Resolver.RefreshCodes) {
		_ = w.tokenManager.Invalidate(ctx, resolvedTokenCacheKey, "upstream token refresh code")
		resolution, tokenErr := w.tokenManager.ResolveWithResolver(ctx, provider.TokenResolveInput{
			Capability:   capability,
			Channel:      channel,
			Resolver:     *tokenBehavior.Resolver,
			Strategy:     tokenBehavior.Strategy,
			ForceRefresh: true,
		})
		requestSnapshot["token_refresh_exchange"] = resolution.RequestSnapshot
		responseSnapshot["token_refresh_exchange"] = resolution.ResponseSnapshot
		if tokenErr != nil {
			return w.failAttempt(ctx, job, attempt, attemptNo, startedAt, "MGP-TOKEN-003", tokenErr.Error(), requestSnapshot, responseSnapshot, retryPolicyFrom(channel.RetryPolicy), retryableFailure())
		}
		resolvedToken = resolution.Token
		resolvedTokenCacheKey = resolution.CacheKey
		builtRequest, err = w.buildRequest(effectiveChannel, provider.BuildDeliveryRequestInput{
			Token: resolvedToken,
			RenderedMessage: provider.RenderedMessage{
				ProviderType: channel.ProviderType,
				MessageType:  messageType,
				Content:      payload.Body,
			},
			ResolvedRecipients: provider.ResolvedRecipientsFromValue(payload.Recipient),
			TargetContext:      targetContext,
		})
		if err != nil {
			return w.failAttempt(ctx, job, attempt, attemptNo, startedAt, "MGP-SEND-001", err.Error(), requestSnapshot, responseSnapshot, retryPolicyFrom(channel.RetryPolicy), retryableFailure())
		}
		requestSnapshot["final_request"] = redactedBuiltRequestSnapshot(builtRequest, nil)
		requestSnapshot["send"] = redactedBuiltRequestSnapshot(builtRequest, payload.Recipient)
		sendStartedAt = time.Now()
		setSnapshotTime(requestSnapshot, "lifecycle", "request_started_at", sendStartedAt)
		statusCode, responseHeaders, responseBody, responseBodyTruncated, sendErr = w.send(ctx, channel, builtRequest, payload.TraceID, sendStartedAt)
		sendFinishedAt = time.Now()
		recordTiming(ctx, payload.TraceID, TimingSendHTTP, sendFinishedAt.Sub(sendStartedAt))
		responseSnapshot["token_refreshed"] = true
	}
	responseSnapshot["lifecycle"] = map[string]any{
		"request_finished_at": sendFinishedAt.Format(time.RFC3339Nano),
		"request_duration_ms": durationMS(sendStartedAt, sendFinishedAt),
	}
	upstreamResponse := map[string]any{
		"status_code": statusCode,
		"headers":     responseHeaders,
		"body":        snapshotValue(responseBody),
	}
	if responseBodyTruncated {
		upstreamResponse["body_truncated"] = true
		upstreamResponse["body_limit_bytes"] = maxUpstreamResponseBodyBytes
	}
	responseSnapshot["upstream_response"] = upstreamResponse
	responseSnapshot["send"] = legacyResponseSnapshot(upstreamResponse)
	if sendErr != nil {
		upstreamResponse["error"] = sendErr.Error()
		errorCode := "MGP-SEND-003"
		if isTimeoutError(sendErr) {
			errorCode = "MGP-SEND-002"
		}
		retryDecision := classifyTransportRetry(capability, capabilitySource, sendErr)
		responseSnapshot["retry_rule"] = retryDecision.snapshot()
		return w.failAttempt(ctx, job, attempt, attemptNo, startedAt, errorCode, sendErr.Error(), requestSnapshot, responseSnapshot, retryPolicyFrom(channel.RetryPolicy), retryDecision.failureClassification())
	}
	successDecision := classifySuccess(statusCode, responseBody, capability, capabilitySource)
	responseSnapshot["success_rule"] = successDecision.snapshot()
	if !successDecision.Success {
		retryDecision := classifyResponseRetry(statusCode, responseBody, capability, capabilitySource, successDecision.JSONField)
		responseSnapshot["retry_rule"] = retryDecision.snapshot()
		errorMessage := successDecision.ErrorMessage
		if strings.TrimSpace(errorMessage) == "" {
			errorMessage = fmt.Sprintf("upstream response did not match success rule: status %d", statusCode)
		}
		return w.failAttempt(ctx, job, attempt, attemptNo, startedAt, "MGP-SEND-004", errorMessage, requestSnapshot, responseSnapshot, retryPolicyFrom(channel.RetryPolicy), retryDecision.failureClassification())
	}
	responseSnapshot["retry_rule"] = noRetryNeededDecision(capabilitySource, capability.RetryRule).snapshot()

	requestRaw, err := marshalSnapshot(requestSnapshot)
	if err != nil {
		return err
	}
	responseRaw, err := marshalSnapshot(responseSnapshot)
	if err != nil {
		return err
	}
	completeStartedAt := time.Now()
	err = w.completeDelivery(ctx, CompleteDeliveryParams{
		JobID:             job.ID,
		WorkerID:          w.workerID,
		AttemptID:         attempt.ID,
		MessageID:         attempt.MessageID,
		SourceID:          attempt.SourceID,
		ChannelID:         attempt.ChannelID,
		TemplateVersionID: attempt.TemplateVersionID,
		RecipientSnapshot: attempt.RecipientSnapshot,
		DeliveryCreatedAt: timeFromPtr(attempt.QueuedAt),
		TraceID:           payload.TraceID,
		AttemptNo:         attemptNo,
		Status:            StatusSent,
		RequestSnapshot:   requestRaw,
		ResponseSnapshot:  responseRaw,
		DurationMS:        durationMS(startedAt, w.now()),
		FinishedAt:        w.now(),
		InboundHeaders:    attempt.InboundHeaders,
		InboundPayload:    attempt.InboundPayload,
		InboundReceivedAt: attempt.InboundReceivedAt,
	})
	if completeDeliveryCollectorFromContext(ctx) == nil {
		recordTiming(ctx, payload.TraceID, TimingComplete, time.Since(completeStartedAt))
	}
	return err
}

func (w *Worker) completeDelivery(ctx context.Context, params CompleteDeliveryParams) error {
	if collector := completeDeliveryCollectorFromContext(ctx); collector != nil {
		collector.add(params)
		return nil
	}
	if w.resultPublisher != nil {
		return w.resultPublisher.PublishDeliveryResult(ctx, NewDeliveryResultEvent(params))
	}
	return w.repo.CompleteDelivery(ctx, params)
}

func attemptFromDirectPayload(payload SendMessageJobPayload) (Attempt, bool) {
	attemptID := strings.TrimSpace(payload.DeliveryAttemptID)
	messageID := strings.TrimSpace(payload.MessageID)
	channelID := strings.TrimSpace(payload.ChannelID)
	templateVersionID := strings.TrimSpace(payload.TemplateVersionID)
	if attemptID == "" || messageID == "" || channelID == "" || templateVersionID == "" {
		return Attempt{}, false
	}
	return Attempt{
		ID:                attemptID,
		MessageID:         messageID,
		SourceID:          strings.TrimSpace(payload.SourceID),
		ChannelID:         channelID,
		TemplateVersionID: templateVersionID,
		RecipientSnapshot: append(json.RawMessage(nil), payload.RecipientSnapshot...),
		Status:            StatusProcessing,
		QueuedAt:          timePtrIfSet(payload.DeliveryCreatedAt),
		InboundHeaders:    append(json.RawMessage(nil), payload.InboundHeaders...),
		InboundPayload:    append(json.RawMessage(nil), payload.InboundPayload...),
		InboundReceivedAt: payload.InboundReceivedAt,
	}, true
}

func deliveryLifecycleSnapshot(payload SendMessageJobPayload) map[string]any {
	lifecycle := map[string]any{}
	if !payload.RoutePlannedAt.IsZero() {
		lifecycle["route_planned_at"] = payload.RoutePlannedAt.Format(time.RFC3339Nano)
	}
	if !payload.DeliveryCreatedAt.IsZero() {
		lifecycle["delivery_created_at"] = payload.DeliveryCreatedAt.Format(time.RFC3339Nano)
	}
	return lifecycle
}

func setSnapshotTime(snapshot map[string]any, parent string, key string, value time.Time) {
	if value.IsZero() {
		return
	}
	parentValue, _ := snapshot[parent].(map[string]any)
	if parentValue == nil {
		parentValue = map[string]any{}
		snapshot[parent] = parentValue
	}
	parentValue[key] = value.Format(time.RFC3339Nano)
}

func timePtrIfSet(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	return &value
}

func timeFromPtr(value *time.Time) time.Time {
	if value == nil {
		return time.Time{}
	}
	return *value
}

func (w *Worker) loadCapability(ctx context.Context, providerType provider.ProviderType, messageType string) (provider.Capability, string, error) {
	capability := provider.Capability{
		ProviderType: providerType,
		MessageType:  strings.TrimSpace(messageType),
	}
	if strings.TrimSpace(messageType) == "" {
		return capability, "legacy.no_message_type", nil
	}
	loaded, err := w.repo.GetProviderCapability(ctx, providerType, messageType)
	if err != nil {
		if errors.Is(err, provider.ErrNotFound) {
			return capability, "legacy.capability_not_found", nil
		}
		return provider.Capability{}, "", err
	}
	return loaded, "capability", nil
}

func effectiveSendDedupeKey(configuredKey string, templateVersionID string) string {
	configuredKey = strings.TrimSpace(configuredKey)
	templateVersionID = strings.TrimSpace(templateVersionID)
	if configuredKey == "" || templateVersionID == "" {
		return configuredKey
	}
	return "template_version:" + templateVersionID + ":" + configuredKey
}

func effectiveDedupeScope(templateVersionID string) string {
	if strings.TrimSpace(templateVersionID) == "" {
		return "channel"
	}
	return "channel_template_version"
}

func (w *Worker) failAttempt(
	ctx context.Context,
	job queue.Job,
	attempt Attempt,
	attemptNo int,
	startedAt time.Time,
	errorCode string,
	errorMessage string,
	requestSnapshot map[string]any,
	responseSnapshot map[string]any,
	retryPolicy retryPolicy,
	classification failureClassification,
) error {
	requestRaw, err := marshalSnapshot(requestSnapshot)
	if err != nil {
		return err
	}
	responseRaw, err := marshalSnapshot(responseSnapshot)
	if err != nil {
		return err
	}
	finishedAt := w.now()
	duration := durationMS(startedAt, finishedAt)

	maxAttempts := job.MaxAttempts
	if retryPolicy.MaxAttempts > 0 && (maxAttempts == 0 || retryPolicy.MaxAttempts < maxAttempts) {
		maxAttempts = retryPolicy.MaxAttempts
	}
	if maxAttempts <= 0 {
		maxAttempts = 1
	}

	if !classification.Retryable || attemptNo >= maxAttempts {
		params := DeadLetterDeliveryParams{
			JobID:             job.ID,
			WorkerID:          w.workerID,
			AttemptID:         attempt.ID,
			ChannelID:         attempt.ChannelID,
			MessageID:         attempt.MessageID,
			SourceID:          attempt.SourceID,
			TemplateVersionID: attempt.TemplateVersionID,
			RecipientSnapshot: attempt.RecipientSnapshot,
			DeliveryCreatedAt: timeFromPtr(attempt.QueuedAt),
			AttemptNo:         attemptNo,
			ErrorCode:         errorCode,
			ErrorMessage:      errorMessage,
			RequestSnapshot:   requestRaw,
			ResponseSnapshot:  responseRaw,
			DurationMS:        duration,
			FinishedAt:        finishedAt,
			InboundHeaders:    attempt.InboundHeaders,
			InboundPayload:    attempt.InboundPayload,
			InboundReceivedAt: attempt.InboundReceivedAt,
		}
		if w.resultPublisher != nil {
			return w.resultPublisher.PublishDeliveryResult(ctx, NewDeadLetterDeliveryResultEvent(params))
		}
		return w.repo.DeadLetterDelivery(ctx, params)
	}

	params := RetryDeliveryParams{
		JobID:             job.ID,
		WorkerID:          w.workerID,
		AttemptID:         attempt.ID,
		MessageID:         attempt.MessageID,
		SourceID:          attempt.SourceID,
		ChannelID:         attempt.ChannelID,
		TemplateVersionID: attempt.TemplateVersionID,
		RecipientSnapshot: attempt.RecipientSnapshot,
		DeliveryCreatedAt: timeFromPtr(attempt.QueuedAt),
		AttemptNo:         attemptNo,
		ErrorCode:         errorCode,
		ErrorMessage:      errorMessage,
		RequestSnapshot:   requestRaw,
		ResponseSnapshot:  responseRaw,
		DurationMS:        duration,
		RetryAt:           finishedAt.Add(retryPolicy.Delay()),
		FinishedAt:        finishedAt,
		InboundHeaders:    attempt.InboundHeaders,
		InboundPayload:    attempt.InboundPayload,
		InboundReceivedAt: attempt.InboundReceivedAt,
	}
	if w.resultPublisher != nil {
		if err := w.resultPublisher.PublishDeliveryResult(ctx, NewRetryDeliveryResultEvent(params)); err != nil {
			return err
		}
		return ErrRetryScheduled
	}
	return w.repo.RetryDelivery(ctx, params)
}

type failureClassification struct {
	Retryable bool
}

func retryableFailure() failureClassification {
	return failureClassification{Retryable: true}
}

func (w *Worker) send(ctx context.Context, channel provider.Channel, built provider.BuiltRequest, traceID string, sendStartedAt time.Time) (int, map[string][]string, []byte, bool, error) {
	body := built.Body
	if len(bytes.TrimSpace(body)) == 0 {
		body = json.RawMessage(`{}`)
	}
	req, err := http.NewRequestWithContext(ctx, built.Method, built.URL, bytes.NewReader(body))
	if err != nil {
		return 0, nil, nil, false, err
	}
	for key, value := range built.Headers {
		req.Header.Set(key, value)
	}
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	return w.doRequest(channel.TimeoutMS, req, traceID, sendStartedAt)
}

func (w *Worker) doRequest(timeoutMS int, req *http.Request, traceID string, sendStartedAt time.Time) (int, map[string][]string, []byte, bool, error) {
	timeout := time.Duration(timeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	var dispatchOnce sync.Once
	recordDispatch := func() {
		if strings.TrimSpace(traceID) == "" || sendStartedAt.IsZero() {
			return
		}
		recordTiming(req.Context(), traceID, TimingDispatchHTTP, time.Since(sendStartedAt))
	}
	if strings.TrimSpace(traceID) != "" && !sendStartedAt.IsZero() {
		trace := &httptrace.ClientTrace{
			WroteRequest: func(httptrace.WroteRequestInfo) {
				dispatchOnce.Do(recordDispatch)
			},
		}
		req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))
	}
	client := w.httpClientFactory(timeout)
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, nil, false, err
	}
	dispatchOnce.Do(recordDispatch)
	defer resp.Body.Close()

	body, truncated, readErr := readBoundedUpstreamBody(resp.Body)
	if readErr != nil {
		return resp.StatusCode, resp.Header, body, truncated, readErr
	}
	return resp.StatusCode, resp.Header, body, truncated, nil
}

func readBoundedUpstreamBody(reader io.Reader) ([]byte, bool, error) {
	if reader == nil {
		return nil, false, nil
	}
	body, err := io.ReadAll(io.LimitReader(reader, int64(maxUpstreamResponseBodyBytes)+1))
	if err != nil {
		return body, false, err
	}
	if len(body) <= maxUpstreamResponseBodyBytes {
		return body, false, nil
	}
	return body[:maxUpstreamResponseBodyBytes], true, nil
}

func legacyResponseSnapshot(upstream map[string]any) map[string]any {
	snapshot := map[string]any{}
	for _, key := range []string{"status_code", "headers", "error", "body_truncated", "body_limit_bytes"} {
		if value, ok := upstream[key]; ok {
			snapshot[key] = value
		}
	}
	if _, truncated := upstream["body_truncated"]; !truncated {
		if body, ok := upstream["body"]; ok {
			snapshot["body"] = body
		}
	}
	return snapshot
}

func (w *Worker) acquireSemaphore(ctx context.Context, channelID string, limit int) (func(), error) {
	if limit <= 0 {
		limit = 1
	}
	w.mu.Lock()
	sem, ok := w.semaphores[channelID]
	if !ok {
		sem = make(chan struct{}, limit)
		w.semaphores[channelID] = sem
	}
	w.mu.Unlock()

	select {
	case sem <- struct{}{}:
		return func() { <-sem }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (w *Worker) waitRateLimit(ctx context.Context, channelID string, raw json.RawMessage) error {
	cfg := rateLimitFrom(raw)
	if !cfg.Enabled {
		return nil
	}
	w.mu.Lock()
	limiter, ok := w.limiters[channelID]
	if !ok {
		limiter = newChannelLimiter(cfg, w.now)
		w.limiters[channelID] = limiter
	}
	w.mu.Unlock()
	return limiter.Wait(ctx)
}

func decodePayload(raw json.RawMessage) (SendMessageJobPayload, error) {
	var payload SendMessageJobPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return SendMessageJobPayload{}, fmt.Errorf("decode send job payload: %w", err)
	}
	return payload, nil
}

func sendJobFromEvent(event queue.SendMessageEvent, deliveryCount int) (queue.Job, error) {
	if err := event.Validate(); err != nil {
		return queue.Job{}, err
	}
	payload := append(json.RawMessage(nil), event.Payload...)
	if len(bytes.TrimSpace(payload)) == 0 {
		payload, _ = json.Marshal(SendMessageJobPayload{
			DeliveryAttemptID: strings.TrimSpace(event.DeliveryAttemptID),
			MessageID:         strings.TrimSpace(event.MessageID),
			SourceID:          strings.TrimSpace(event.SourceID),
			ChannelID:         strings.TrimSpace(event.ChannelID),
			TraceID:           strings.TrimSpace(event.TraceID),
		})
	}
	return queue.Job{
		Type:        queue.JobTypeSendMessage,
		Status:      queue.JobStatusProcessing,
		Payload:     payload,
		ChannelID:   strings.TrimSpace(event.ChannelID),
		Attempts:    positiveInt(deliveryCount, 1),
		MaxAttempts: positiveInt(event.MaxAttempts, 1),
		QueueKey:    strings.TrimSpace(event.ChannelID),
	}, nil
}

func positiveInt(value int, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}

func nakSendMessage(message queue.SendMessage, delay time.Duration) error {
	if message.Nak == nil {
		return nil
	}
	if delay <= 0 {
		delay = defaultRetryDelay
	}
	return message.Nak(delay)
}

type tokenResolverConfig = provider.TokenResolverConfig
type tokenRequestConfig = provider.TokenRequestConfig

type tokenBehavior struct {
	Resolver  *tokenResolverConfig
	Placement json.RawMessage
	Source    string
	Strategy  string
}

func (b tokenBehavior) snapshot() map[string]any {
	source := b.Source
	if strings.TrimSpace(source) == "" {
		source = "none"
	}
	return map[string]any{
		"source":       source,
		"strategy":     b.Strategy,
		"has_resolver": b.Resolver != nil,
		"placement":    snapshotValue(b.Placement),
	}
}

func parseTokenBehavior(channel provider.Channel, capability provider.Capability, capabilitySource string) (tokenBehavior, error) {
	behavior := tokenBehavior{Source: "none"}
	placement := extractPlacement(channel.TokenConfig)
	placementSource := "channel.token_config"
	if len(placement) == 0 {
		placement = extractPlacement(channel.AuthConfig)
		placementSource = "channel.auth_config"
	}

	if resolver, err := decodeResolver(channel.TokenConfig); err != nil {
		return tokenBehavior{}, err
	} else if resolver != nil {
		if len(placement) == 0 && len(resolver.Placement) > 0 {
			placement = append(json.RawMessage(nil), resolver.Placement...)
			placementSource = "channel.token_config"
		}
		return tokenBehavior{Resolver: resolver, Placement: placement, Source: "channel.token_config"}, nil
	}

	resolver, err := decodeResolver(channel.AuthConfig)
	if err != nil {
		return tokenBehavior{}, err
	}
	if resolver != nil {
		if len(placement) == 0 && len(resolver.Placement) > 0 {
			placement = append(json.RawMessage(nil), resolver.Placement...)
			placementSource = "channel.auth_config"
		}
		return tokenBehavior{Resolver: resolver, Placement: placement, Source: "channel.auth_config"}, nil
	}
	if len(placement) > 0 {
		behavior.Placement = placement
		behavior.Source = placementSource
		return behavior, nil
	}

	if capabilitySource == "capability" {
		capabilityPlacement := extractPlacement(capability.TokenStrategy)
		capabilityResolver, strategy, err := decodeCapabilityResolver(capability.TokenStrategy, channel)
		if err != nil {
			return tokenBehavior{}, err
		}
		if capabilityResolver != nil && len(capabilityPlacement) == 0 && len(capabilityResolver.Placement) > 0 {
			capabilityPlacement = append(json.RawMessage(nil), capabilityResolver.Placement...)
		}
		if capabilityResolver != nil || len(capabilityPlacement) > 0 || strings.TrimSpace(strategy) != "" {
			return tokenBehavior{
				Resolver:  capabilityResolver,
				Placement: capabilityPlacement,
				Source:    "capability.token_strategy",
				Strategy:  strategy,
			}, nil
		}
	}
	return behavior, nil
}

func decodeResolver(raw json.RawMessage) (*tokenResolverConfig, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, nil
	}
	var candidate tokenResolverConfig
	if err := json.Unmarshal(raw, &candidate); err != nil {
		return nil, fmt.Errorf("decode token config: %w", err)
	}
	if strings.TrimSpace(candidate.Request.URL) == "" || strings.TrimSpace(candidate.ResponsePath) == "" {
		return nil, nil
	}
	if candidate.Request.Headers == nil {
		candidate.Request.Headers = map[string]string{}
	}
	return &candidate, nil
}

type capabilityTokenStrategyConfig struct {
	Strategy              string                 `json:"strategy"`
	TokenURL              string                 `json:"token_url"`
	Cacheable             bool                   `json:"cacheable"`
	ResponseTokenPath     string                 `json:"response_token_path"`
	ResponseExpiresInPath string                 `json:"response_expires_in_path"`
	ExpiresInSeconds      int                    `json:"expires_in_seconds"`
	RefreshTokenCodes     []any                  `json:"refresh_on_json_codes"`
	Request               capabilityTokenRequest `json:"request"`
	Placement             json.RawMessage        `json:"placement"`
}

type capabilityTokenRequest struct {
	Method           string            `json:"method"`
	QueryFields      []string          `json:"query_fields"`
	QuerySecretField string            `json:"query_secret_field"`
	BodyFields       []string          `json:"body_fields"`
	Headers          map[string]string `json:"headers"`
	Body             json.RawMessage   `json:"body"`
}

func decodeCapabilityResolver(raw json.RawMessage, channel provider.Channel) (*tokenResolverConfig, string, error) {
	return provider.DecodeCapabilityResolver(raw, channel)
}

func capabilityStrategy(raw json.RawMessage) string {
	var object struct {
		Strategy string `json:"strategy"`
	}
	_ = json.Unmarshal(raw, &object)
	return strings.TrimSpace(object.Strategy)
}

func mergeCredentialMaps(rawValues ...json.RawMessage) map[string]any {
	merged := map[string]any{}
	for _, raw := range rawValues {
		if len(bytes.TrimSpace(raw)) == 0 || !json.Valid(raw) {
			continue
		}
		var object map[string]any
		if err := json.Unmarshal(raw, &object); err != nil {
			continue
		}
		for key, value := range object {
			merged[key] = value
		}
	}
	return merged
}

func extractPlacement(raw json.RawMessage) json.RawMessage {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || !json.Valid(raw) {
		return nil
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		return append(json.RawMessage(nil), raw...)
	}
	if nested, ok := object["placement"]; ok {
		return append(json.RawMessage(nil), bytes.TrimSpace(nested)...)
	}
	if nested, ok := object["token"]; ok {
		return append(json.RawMessage(nil), bytes.TrimSpace(nested)...)
	}
	if _, ok := object["location"]; ok {
		return append(json.RawMessage(nil), raw...)
	}
	return nil
}

type successRuleConfig struct {
	Type               string `json:"type"`
	StatusCode         int    `json:"status_code"`
	StatusCodes        []int  `json:"status_codes"`
	DefaultStatusCodes []int  `json:"default_status_codes"`
	Field              string `json:"field"`
	Equals             any    `json:"equals"`
}

type successDecision struct {
	Success      bool
	Source       string
	Rule         json.RawMessage
	Type         string
	JSONField    string
	ErrorMessage string
	Fallback     string
}

func (d successDecision) snapshot() map[string]any {
	return map[string]any{
		"source":        d.Source,
		"type":          d.Type,
		"matched":       d.Success,
		"field":         d.JSONField,
		"rule":          snapshotValue(d.Rule),
		"fallback":      d.Fallback,
		"error_message": d.ErrorMessage,
	}
}

func classifySuccess(statusCode int, responseBody []byte, capability provider.Capability, capabilitySource string) successDecision {
	if capabilitySource != "capability" {
		return successDecision{
			Success: statusCode >= http.StatusOK && statusCode < http.StatusMultipleChoices,
			Source:  "legacy_2xx",
			Type:    "status_code",
		}
	}

	rule, known := decodeSuccessRule(capability.SuccessRule)
	if !known {
		return successDecision{
			Success:  statusCode >= http.StatusOK && statusCode < http.StatusMultipleChoices,
			Source:   "legacy_2xx",
			Rule:     capability.SuccessRule,
			Type:     strings.TrimSpace(rule.Type),
			Fallback: "unknown_success_rule",
		}
	}

	decision := successDecision{
		Source:    "capability.success_rule",
		Rule:      capability.SuccessRule,
		Type:      normalizedSuccessRuleType(rule),
		JSONField: strings.TrimSpace(rule.Field),
	}
	statusCodes := successRuleStatusCodes(rule)
	statusMatches := len(statusCodes) == 0 || containsInt(statusCodes, statusCode)

	switch normalizedSuccessRuleType(rule) {
	case "status_code":
		decision.Success = statusMatches
		if !decision.Success {
			decision.ErrorMessage = fmt.Sprintf("upstream status %d did not match success status codes", statusCode)
		}
	case "json_field":
		if !statusMatches {
			decision.Success = false
			decision.ErrorMessage = fmt.Sprintf("upstream status %d did not match success status codes", statusCode)
			return decision
		}
		value, ok, err := lookupJSONValue(responseBody, rule.Field)
		if err != nil {
			decision.ErrorMessage = err.Error()
			return decision
		}
		if !ok {
			decision.ErrorMessage = fmt.Sprintf("success field %q not found", rule.Field)
			return decision
		}
		decision.Success = jsonValueEqual(value, rule.Equals)
		if !decision.Success {
			decision.ErrorMessage = fmt.Sprintf("success field %q did not match expected value", rule.Field)
		}
	case "status_and_json_field":
		if !statusMatches {
			decision.Success = false
			decision.ErrorMessage = fmt.Sprintf("upstream status %d did not match success status codes", statusCode)
			return decision
		}
		value, ok, err := lookupJSONValue(responseBody, rule.Field)
		if err != nil {
			decision.ErrorMessage = err.Error()
			return decision
		}
		if !ok {
			decision.ErrorMessage = fmt.Sprintf("success field %q not found", rule.Field)
			return decision
		}
		decision.Success = jsonValueEqual(value, rule.Equals)
		if !decision.Success {
			decision.ErrorMessage = fmt.Sprintf("success field %q did not match expected value", rule.Field)
		}
	}
	return decision
}

func decodeSuccessRule(raw json.RawMessage) (successRuleConfig, bool) {
	var rule successRuleConfig
	if len(bytes.TrimSpace(raw)) == 0 || string(bytes.TrimSpace(raw)) == "{}" {
		return rule, false
	}
	if err := json.Unmarshal(raw, &rule); err != nil {
		return rule, false
	}
	switch normalizedSuccessRuleType(rule) {
	case "status_code":
		return rule, len(successRuleStatusCodes(rule)) > 0
	case "json_field", "status_and_json_field":
		return rule, strings.TrimSpace(rule.Field) != ""
	default:
		return rule, false
	}
}

func normalizedSuccessRuleType(rule successRuleConfig) string {
	ruleType := strings.ToLower(strings.TrimSpace(rule.Type))
	switch ruleType {
	case "status_code", "status_codes":
		return "status_code"
	case "json_field", "status_and_json_field":
		return ruleType
	case "configurable":
		return "status_code"
	case "":
		if strings.TrimSpace(rule.Field) != "" {
			return "json_field"
		}
	}
	return ruleType
}

func successRuleStatusCodes(rule successRuleConfig) []int {
	statusCodes := append([]int(nil), rule.StatusCodes...)
	if rule.StatusCode > 0 {
		statusCodes = append(statusCodes, rule.StatusCode)
	}
	statusCodes = append(statusCodes, rule.DefaultStatusCodes...)
	return uniqueInts(statusCodes)
}

type retryRuleConfig struct {
	StatusCodes               []int  `json:"status_codes"`
	NetworkErrors             bool   `json:"network_errors"`
	NonRetryableStatusCodes   []int  `json:"non_retryable_status_codes"`
	NonRetryableStatusClasses []int  `json:"non_retryable_status_classes"`
	RetryableJSONCodes        []any  `json:"retryable_json_codes"`
	JSONCodes                 []any  `json:"json_codes"`
	RefreshTokenCodes         []any  `json:"refresh_token_codes"`
	NonRetryableJSONCodes     []any  `json:"non_retryable_json_codes"`
	RetryableVendorCodes      []any  `json:"retryable_vendor_codes"`
	VendorCodes               []any  `json:"vendor_codes"`
	NonRetryableVendorCodes   []any  `json:"non_retryable_vendor_codes"`
	JSONCodeField             string `json:"json_code_field"`
	Field                     string `json:"field"`
}

type retryDecision struct {
	Retryable bool
	Source    string
	Decision  string
	Reason    string
	Rule      json.RawMessage
	Status    int
	JSONField string
	JSONCode  any
}

func (d retryDecision) snapshot() map[string]any {
	return map[string]any{
		"source":      d.Source,
		"decision":    d.Decision,
		"retryable":   d.Retryable,
		"reason":      d.Reason,
		"rule":        snapshotValue(d.Rule),
		"status_code": d.Status,
		"json_field":  d.JSONField,
		"json_code":   d.JSONCode,
	}
}

func (d retryDecision) failureClassification() failureClassification {
	return failureClassification{Retryable: d.Retryable}
}

func classifyTransportRetry(capability provider.Capability, capabilitySource string, err error) retryDecision {
	source := "legacy_retry_policy"
	rule := json.RawMessage(nil)
	if capabilitySource == "capability" {
		source = "capability.retry_rule"
		rule = capability.RetryRule
	}
	reason := "network_error"
	if isTimeoutError(err) {
		reason = "timeout"
	}
	return retryDecision{
		Retryable: true,
		Source:    source,
		Decision:  "retry",
		Reason:    reason,
		Rule:      rule,
	}
}

func classifyResponseRetry(statusCode int, responseBody []byte, capability provider.Capability, capabilitySource string, successJSONField string) retryDecision {
	if capabilitySource != "capability" {
		return retryDecision{
			Retryable: true,
			Source:    "legacy_retry_policy",
			Decision:  "retry",
			Reason:    "legacy_response_failure",
			Status:    statusCode,
		}
	}

	rule := decodeRetryRule(capability.RetryRule)
	field := firstNonEmpty(rule.JSONCodeField, rule.Field, successJSONField)
	jsonCode, jsonField := responseJSONCode(responseBody, field)
	decision := retryDecision{
		Source:    "capability.retry_rule",
		Rule:      capability.RetryRule,
		Status:    statusCode,
		JSONField: jsonField,
		JSONCode:  jsonCode,
	}

	switch {
	case containsInt(rule.NonRetryableStatusCodes, statusCode):
		decision.Decision = "dead"
		decision.Reason = "non_retryable_status_code"
	case jsonCodeMatches(jsonCode, appendAny(rule.NonRetryableJSONCodes, rule.NonRetryableVendorCodes)):
		decision.Decision = "dead"
		decision.Reason = "non_retryable_json_code"
	case containsInt(rule.StatusCodes, statusCode):
		decision.Retryable = true
		decision.Decision = "retry"
		decision.Reason = "retryable_status_code"
	case jsonCodeMatches(jsonCode, appendAny(rule.RetryableJSONCodes, rule.JSONCodes, rule.RefreshTokenCodes, rule.RetryableVendorCodes, rule.VendorCodes)):
		decision.Retryable = true
		decision.Decision = "retry"
		decision.Reason = "retryable_json_code"
	case containsStatusClass(rule.NonRetryableStatusClasses, statusCode):
		decision.Decision = "dead"
		decision.Reason = "non_retryable_status_class"
	case statusCode == http.StatusRequestTimeout || statusCode == http.StatusTooManyRequests || statusCode >= http.StatusInternalServerError:
		decision.Retryable = true
		decision.Decision = "retry"
		decision.Reason = "default_retryable_status"
	default:
		decision.Decision = "dead"
		decision.Reason = "not_retryable"
	}
	return decision
}

func noRetryNeededDecision(capabilitySource string, retryRule json.RawMessage) retryDecision {
	source := "legacy_retry_policy"
	rule := json.RawMessage(nil)
	if capabilitySource == "capability" {
		source = "capability.retry_rule"
		rule = retryRule
	}
	return retryDecision{
		Retryable: false,
		Source:    source,
		Decision:  "none",
		Reason:    "success",
		Rule:      rule,
	}
}

func decodeRetryRule(raw json.RawMessage) retryRuleConfig {
	var rule retryRuleConfig
	if len(bytes.TrimSpace(raw)) > 0 {
		_ = json.Unmarshal(raw, &rule)
	}
	return rule
}

func lookupJSONValue(raw []byte, path string) (any, bool, error) {
	path = strings.TrimSpace(strings.TrimPrefix(path, "$."))
	if path == "" {
		return nil, false, nil
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, false, fmt.Errorf("decode upstream response json: %w", err)
	}
	current := value
	for _, part := range strings.Split(path, ".") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		object, ok := current.(map[string]any)
		if !ok {
			return nil, false, nil
		}
		current, ok = object[part]
		if !ok {
			return nil, false, nil
		}
	}
	return current, true, nil
}

func responseJSONCode(raw []byte, preferredField string) (any, string) {
	fields := []string{strings.TrimSpace(preferredField), "errcode", "code", "Code", "error_code", "status"}
	seen := map[string]bool{}
	for _, field := range fields {
		if field == "" || seen[field] {
			continue
		}
		seen[field] = true
		value, ok, err := lookupJSONValue(raw, field)
		if err == nil && ok {
			return value, field
		}
	}
	return nil, ""
}

func shouldRefreshToken(responseBody []byte, capability provider.Capability, capabilitySource string, resolverCodes []any) bool {
	candidates := append([]any(nil), resolverCodes...)
	var field string
	if capabilitySource == "capability" {
		rule := decodeRetryRule(capability.RetryRule)
		field = rule.Field
		candidates = append(candidates, rule.RefreshTokenCodes...)
	}
	if len(candidates) == 0 {
		return false
	}
	jsonCode, _ := responseJSONCode(responseBody, field)
	return jsonCodeMatches(jsonCode, candidates)
}

func jsonCodeMatches(value any, candidates []any) bool {
	if value == nil || len(candidates) == 0 {
		return false
	}
	for _, candidate := range candidates {
		if jsonValueEqual(value, candidate) {
			return true
		}
	}
	return false
}

func jsonValueEqual(left any, right any) bool {
	if leftNumber, ok := numericValue(left); ok {
		if rightNumber, ok := numericValue(right); ok {
			return leftNumber == rightNumber
		}
	}
	switch leftValue := left.(type) {
	case bool:
		rightValue, ok := right.(bool)
		return ok && leftValue == rightValue
	case string:
		rightValue, ok := right.(string)
		if ok {
			return leftValue == rightValue
		}
	}
	return fmt.Sprint(left) == fmt.Sprint(right)
}

func numericValue(value any) (float64, bool) {
	switch typed := value.(type) {
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case float64:
		return typed, true
	case json.Number:
		number, err := typed.Float64()
		return number, err == nil
	default:
		return 0, false
	}
}

func containsInt(values []int, target int) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func containsStatusClass(classes []int, statusCode int) bool {
	if statusCode <= 0 {
		return false
	}
	statusClass := (statusCode / 100) * 100
	return containsInt(classes, statusClass)
}

func uniqueInts(values []int) []int {
	seen := map[int]bool{}
	result := make([]int, 0, len(values))
	for _, value := range values {
		if value <= 0 || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

func appendAny(values ...[]any) []any {
	total := 0
	for _, items := range values {
		total += len(items)
	}
	result := make([]any, 0, total)
	for _, items := range values {
		result = append(result, items...)
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

type retryPolicy struct {
	MaxAttempts int `json:"max_attempts"`
	DelayMS     int `json:"delay_ms"`
	DelaySec    int `json:"delay_seconds"`
}

func retryPolicyFrom(raw json.RawMessage) retryPolicy {
	policy := retryPolicy{MaxAttempts: 3}
	if len(bytes.TrimSpace(raw)) == 0 {
		return policy
	}
	_ = json.Unmarshal(raw, &policy)
	if policy.MaxAttempts <= 0 {
		policy.MaxAttempts = 3
	}
	return policy
}

func (p retryPolicy) Delay() time.Duration {
	if p.DelayMS > 0 {
		return time.Duration(p.DelayMS) * time.Millisecond
	}
	if p.DelaySec > 0 {
		return time.Duration(p.DelaySec) * time.Second
	}
	return defaultRetryDelay
}

type rateLimitConfig struct {
	Enabled bool    `json:"enabled"`
	QPS     float64 `json:"qps"`
}

func rateLimitFrom(raw json.RawMessage) rateLimitConfig {
	var cfg rateLimitConfig
	if len(bytes.TrimSpace(raw)) == 0 {
		return cfg
	}
	_ = json.Unmarshal(raw, &cfg)
	if !cfg.Enabled || cfg.QPS <= 0 {
		cfg.Enabled = false
		return cfg
	}
	return cfg
}

type channelLimiter struct {
	now      func() time.Time
	rate     float64
	capacity float64

	mu     sync.Mutex
	tokens float64
	last   time.Time
}

func newChannelLimiter(cfg rateLimitConfig, now func() time.Time) *channelLimiter {
	rate := cfg.QPS
	if rate <= 0 {
		rate = math.Inf(1)
	}
	return &channelLimiter{
		now:      now,
		rate:     rate,
		capacity: math.Max(1, math.Ceil(rate)),
	}
}

func (l *channelLimiter) Wait(ctx context.Context) error {
	if math.IsInf(l.rate, 1) {
		return nil
	}
	for {
		l.mu.Lock()
		now := l.now()
		if l.last.IsZero() {
			l.last = now
			l.tokens = l.capacity
		}
		elapsed := now.Sub(l.last).Seconds()
		if elapsed > 0 {
			l.tokens = math.Min(l.capacity, l.tokens+(elapsed*l.rate))
			l.last = now
		}
		if l.tokens >= 1 {
			l.tokens--
			l.mu.Unlock()
			return nil
		}
		waitFor := time.Duration(((1 - l.tokens) / l.rate) * float64(time.Second))
		l.mu.Unlock()

		timer := time.NewTimer(waitFor)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func extractJSONPath(raw []byte, path string) (string, error) {
	parts := strings.Split(strings.TrimSpace(path), ".")
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", fmt.Errorf("decode token response: %w", err)
	}
	current := value
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		object, ok := current.(map[string]any)
		if !ok {
			return "", fmt.Errorf("token response path %q not found", path)
		}
		current, ok = object[part]
		if !ok {
			return "", fmt.Errorf("token response path %q not found", path)
		}
	}
	token, ok := current.(string)
	if !ok || strings.TrimSpace(token) == "" {
		return "", fmt.Errorf("token response path %q is not a string", path)
	}
	return token, nil
}

func snapshotValue(raw []byte) any {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return nil
	}
	var value any
	if err := json.Unmarshal(raw, &value); err == nil {
		return value
	}
	return string(raw)
}

func redactedBuiltRequestSnapshot(built provider.BuiltRequest, recipient any) map[string]any {
	snapshot := map[string]any{
		"method":  built.Method,
		"url":     redactSnapshotURL(built.URL),
		"headers": redactSnapshotHeaders(built.Headers),
		"query":   redactSnapshotQuery(built.Query),
		"body":    snapshotValue(built.Body),
	}
	if recipient != nil {
		snapshot["recipient"] = recipient
	}
	return snapshot
}

func redactSnapshotHeaders(headers map[string]string) map[string]string {
	redacted := map[string]string{}
	for key, value := range headers {
		if isSensitiveTokenField(key) {
			redacted[key] = "***"
			continue
		}
		if strings.EqualFold(key, "Authorization") && strings.TrimSpace(value) != "" {
			redacted[key] = "Bearer ***"
			continue
		}
		redacted[key] = value
	}
	return redacted
}

func redactSnapshotQuery(query map[string]string) map[string]string {
	redacted := map[string]string{}
	for key, value := range query {
		if isSensitiveTokenField(key) {
			redacted[key] = "***"
			continue
		}
		redacted[key] = value
	}
	return redacted
}

func redactSnapshotURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	values := parsed.Query()
	changed := false
	for key := range values {
		if isSensitiveTokenField(key) {
			values.Set(key, "***")
			changed = true
		}
	}
	if changed {
		parsed.RawQuery = values.Encode()
	}
	return parsed.String()
}

func isSensitiveTokenField(key string) bool {
	normalized := strings.ToLower(strings.TrimSpace(key))
	return normalized == "access_token" ||
		normalized == "authorization" ||
		normalized == "token" ||
		normalized == "corpsecret" ||
		normalized == "secret" ||
		normalized == "appsecret"
}

func marshalSnapshot(snapshot map[string]any) (json.RawMessage, error) {
	if len(snapshot) == 0 {
		return json.RawMessage(`{}`), nil
	}
	raw, err := json.Marshal(snapshot)
	if err != nil {
		return nil, err
	}
	return raw, nil
}

func durationMS(start time.Time, end time.Time) int {
	if end.Before(start) {
		return 0
	}
	return int(end.Sub(start).Milliseconds())
}

func isTimeoutError(err error) bool {
	type timeout interface {
		Timeout() bool
	}
	var timeoutErr timeout
	return errors.As(err, &timeoutErr) && timeoutErr.Timeout()
}
