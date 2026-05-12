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
	"strings"
	"sync"
	"time"

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

type Attempt struct {
	ID                string
	MessageID         string
	ChannelID         string
	TemplateVersionID string
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
}

type SendMessageJobPayload struct {
	DeliveryAttemptID string          `json:"delivery_attempt_id"`
	DedupeKey         string          `json:"dedupe_key"`
	DedupeTTLSeconds  int             `json:"dedupe_ttl_seconds"`
	Token             string          `json:"token"`
	Recipient         any             `json:"recipient"`
	Body              json.RawMessage `json:"body"`
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
	JobID            string
	WorkerID         string
	AttemptID        string
	AttemptNo        int
	Status           Status
	RequestSnapshot  json.RawMessage
	ResponseSnapshot json.RawMessage
	DurationMS       int
	FinishedAt       time.Time
}

type RetryDeliveryParams struct {
	JobID            string
	WorkerID         string
	AttemptID        string
	AttemptNo        int
	ErrorCode        string
	ErrorMessage     string
	RequestSnapshot  json.RawMessage
	ResponseSnapshot json.RawMessage
	DurationMS       int
	RetryAt          time.Time
	FinishedAt       time.Time
}

type DeadLetterDeliveryParams struct {
	JobID            string
	WorkerID         string
	AttemptID        string
	AttemptNo        int
	ErrorCode        string
	ErrorMessage     string
	RequestSnapshot  json.RawMessage
	ResponseSnapshot json.RawMessage
	DurationMS       int
	FinishedAt       time.Time
}

type Repository interface {
	ClaimSendJobs(context.Context, queue.ClaimParams) ([]queue.Job, error)
	GetChannel(context.Context, string) (provider.Channel, error)
	GetAttempt(context.Context, string) (Attempt, error)
	MarkAttemptProcessing(context.Context, MarkAttemptProcessingParams) error
	InsertSendDedupeKey(context.Context, SendDedupeParams) (bool, error)
	CompleteDelivery(context.Context, CompleteDeliveryParams) error
	RetryDelivery(context.Context, RetryDeliveryParams) error
	DeadLetterDelivery(context.Context, DeadLetterDeliveryParams) error
}

type Worker struct {
	repo              Repository
	workerID          string
	now               func() time.Time
	httpClientFactory func(time.Duration) *http.Client
	buildRequest      func(provider.Channel, provider.BuildDeliveryRequestInput) (provider.BuiltRequest, error)

	mu         sync.Mutex
	semaphores map[string]chan struct{}
	limiters   map[string]*channelLimiter
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

func NewWorker(repo Repository, opts ...WorkerOption) *Worker {
	worker := &Worker{
		repo:     repo,
		workerID: "delivery-worker",
		now: func() time.Time {
			return time.Now().UTC()
		},
		httpClientFactory: func(timeout time.Duration) *http.Client {
			return &http.Client{Timeout: timeout}
		},
		buildRequest: provider.BuildDeliveryRequest,
		semaphores:   map[string]chan struct{}{},
		limiters:     map[string]*channelLimiter{},
	}
	for _, opt := range opts {
		opt(worker)
	}
	return worker
}

func (w *Worker) ProcessBatch(ctx context.Context, limit int) (int, error) {
	if limit <= 0 {
		limit = 1
	}
	now := w.now()
	jobs, err := w.repo.ClaimSendJobs(ctx, queue.ClaimParams{
		WorkerID: w.workerID,
		Types:    []queue.JobType{queue.JobTypeSendMessage},
		Limit:    limit,
		Now:      now,
	})
	if err != nil {
		return 0, err
	}
	if len(jobs) == 0 {
		return 0, nil
	}

	var wg sync.WaitGroup
	var firstErr error
	var errMu sync.Mutex
	for _, job := range jobs {
		job := job
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := w.ProcessOne(ctx, job); err != nil {
				errMu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				errMu.Unlock()
			}
		}()
	}
	wg.Wait()
	return len(jobs), firstErr
}

func (w *Worker) ProcessOne(ctx context.Context, job queue.Job) error {
	payload, err := decodePayload(job.Payload)
	if err != nil {
		return err
	}
	if strings.TrimSpace(payload.DeliveryAttemptID) == "" {
		return errors.New("delivery attempt id is required")
	}

	attempt, err := w.repo.GetAttempt(ctx, payload.DeliveryAttemptID)
	if err != nil {
		return fmt.Errorf("load attempt %s: %w", payload.DeliveryAttemptID, err)
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
	if err := w.repo.MarkAttemptProcessing(ctx, MarkAttemptProcessingParams{
		AttemptID: attempt.ID,
		AttemptNo: attemptNo,
		StartedAt: startedAt,
	}); err != nil {
		return fmt.Errorf("mark attempt processing: %w", err)
	}

	if dedupeKey := strings.TrimSpace(payload.DedupeKey); dedupeKey != "" {
		expiresAt := startedAt.Add(defaultDedupeTTL)
		if payload.DedupeTTLSeconds > 0 {
			expiresAt = startedAt.Add(time.Duration(payload.DedupeTTLSeconds) * time.Second)
		}
		inserted, err := w.repo.InsertSendDedupeKey(ctx, SendDedupeParams{
			ChannelID: channelID,
			DedupeKey: dedupeKey,
			ExpiresAt: expiresAt,
			MessageID: attempt.MessageID,
		})
		if err != nil {
			return fmt.Errorf("insert send dedupe key: %w", err)
		}
		if !inserted {
			return w.repo.CompleteDelivery(ctx, CompleteDeliveryParams{
				JobID:      job.ID,
				WorkerID:   w.workerID,
				AttemptID:  attempt.ID,
				AttemptNo:  attemptNo,
				Status:     StatusDeduped,
				DurationMS: durationMS(startedAt, w.now()),
				FinishedAt: w.now(),
			})
		}
	}

	targetContext := provider.DeliveryTargetContext{
		DeliveryAttemptID: attempt.ID,
		MessageID:         attempt.MessageID,
		ChannelID:         channel.ID,
		ChannelName:       channel.Name,
		ProviderType:      string(channel.ProviderType),
		TemplateVersionID: attempt.TemplateVersionID,
		JobID:             job.ID,
	}
	requestSnapshot := map[string]any{
		"target_context":      targetContext,
		"rendered_message":    snapshotValue(payload.Body),
		"resolved_recipients": payload.Recipient,
	}
	responseSnapshot := map[string]any{}
	resolvedToken := strings.TrimSpace(payload.Token)

	effectiveChannel := channel
	resolver, placement, err := parseTokenBehavior(channel)
	if err != nil {
		return w.failAttempt(ctx, job, attempt, attemptNo, startedAt, "MGP-TOKEN-001", err.Error(), requestSnapshot, responseSnapshot, retryPolicyFrom(channel.RetryPolicy))
	}
	if len(placement) > 0 {
		effectiveChannel.TokenConfig = placement
	}

	if resolver != nil {
		token, tokenRequestSnapshot, tokenResponseSnapshot, err := w.resolveToken(ctx, channel, *resolver)
		requestSnapshot["token_exchange"] = tokenRequestSnapshot
		responseSnapshot["token_exchange"] = tokenResponseSnapshot
		if err != nil {
			return w.failAttempt(ctx, job, attempt, attemptNo, startedAt, "MGP-TOKEN-002", err.Error(), requestSnapshot, responseSnapshot, retryPolicyFrom(channel.RetryPolicy))
		}
		resolvedToken = token
	}

	builtRequest, err := w.buildRequest(effectiveChannel, provider.BuildDeliveryRequestInput{
		Token: resolvedToken,
		RenderedMessage: provider.RenderedMessage{
			ProviderType: channel.ProviderType,
			Content:      payload.Body,
		},
		ResolvedRecipients: provider.ResolvedRecipientsFromValue(payload.Recipient),
		TargetContext:      targetContext,
	})
	if err != nil {
		return w.failAttempt(ctx, job, attempt, attemptNo, startedAt, "MGP-SEND-001", err.Error(), requestSnapshot, responseSnapshot, retryPolicyFrom(channel.RetryPolicy))
	}

	requestSnapshot["resolved_token"] = resolvedToken
	requestSnapshot["final_request"] = map[string]any{
		"method":  builtRequest.Method,
		"url":     builtRequest.URL,
		"headers": builtRequest.Headers,
		"query":   builtRequest.Query,
		"body":    snapshotValue(builtRequest.Body),
	}
	requestSnapshot["send"] = map[string]any{
		"method":    builtRequest.Method,
		"url":       builtRequest.URL,
		"headers":   builtRequest.Headers,
		"query":     builtRequest.Query,
		"recipient": payload.Recipient,
		"body":      snapshotValue(builtRequest.Body),
	}

	statusCode, responseHeaders, responseBody, sendErr := w.send(ctx, channel, builtRequest)
	upstreamResponse := map[string]any{
		"status_code": statusCode,
		"headers":     responseHeaders,
		"body":        snapshotValue(responseBody),
	}
	responseSnapshot["upstream_response"] = upstreamResponse
	responseSnapshot["send"] = upstreamResponse
	if sendErr != nil {
		upstreamResponse["error"] = sendErr.Error()
		errorCode := "MGP-SEND-003"
		if isTimeoutError(sendErr) {
			errorCode = "MGP-SEND-002"
		}
		return w.failAttempt(ctx, job, attempt, attemptNo, startedAt, errorCode, sendErr.Error(), requestSnapshot, responseSnapshot, retryPolicyFrom(channel.RetryPolicy))
	}
	if statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices {
		return w.failAttempt(ctx, job, attempt, attemptNo, startedAt, "MGP-SEND-004", fmt.Sprintf("upstream returned status %d", statusCode), requestSnapshot, responseSnapshot, retryPolicyFrom(channel.RetryPolicy))
	}

	requestRaw, err := marshalSnapshot(requestSnapshot)
	if err != nil {
		return err
	}
	responseRaw, err := marshalSnapshot(responseSnapshot)
	if err != nil {
		return err
	}
	return w.repo.CompleteDelivery(ctx, CompleteDeliveryParams{
		JobID:            job.ID,
		WorkerID:         w.workerID,
		AttemptID:        attempt.ID,
		AttemptNo:        attemptNo,
		Status:           StatusSent,
		RequestSnapshot:  requestRaw,
		ResponseSnapshot: responseRaw,
		DurationMS:       durationMS(startedAt, w.now()),
		FinishedAt:       w.now(),
	})
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

	if attemptNo >= maxAttempts {
		return w.repo.DeadLetterDelivery(ctx, DeadLetterDeliveryParams{
			JobID:            job.ID,
			WorkerID:         w.workerID,
			AttemptID:        attempt.ID,
			AttemptNo:        attemptNo,
			ErrorCode:        errorCode,
			ErrorMessage:     errorMessage,
			RequestSnapshot:  requestRaw,
			ResponseSnapshot: responseRaw,
			DurationMS:       duration,
			FinishedAt:       finishedAt,
		})
	}

	return w.repo.RetryDelivery(ctx, RetryDeliveryParams{
		JobID:            job.ID,
		WorkerID:         w.workerID,
		AttemptID:        attempt.ID,
		AttemptNo:        attemptNo,
		ErrorCode:        errorCode,
		ErrorMessage:     errorMessage,
		RequestSnapshot:  requestRaw,
		ResponseSnapshot: responseRaw,
		DurationMS:       duration,
		RetryAt:          finishedAt.Add(retryPolicy.Delay()),
		FinishedAt:       finishedAt,
	})
}

func (w *Worker) resolveToken(ctx context.Context, channel provider.Channel, config tokenResolverConfig) (string, map[string]any, map[string]any, error) {
	method := strings.ToUpper(strings.TrimSpace(config.Request.Method))
	if method == "" {
		method = http.MethodPost
	}
	body := config.Request.Body
	if len(bytes.TrimSpace(body)) == 0 {
		body = json.RawMessage(`{}`)
	}
	requestSnapshot := map[string]any{
		"method":  method,
		"url":     config.Request.URL,
		"headers": config.Request.Headers,
		"body":    snapshotValue(body),
	}

	req, err := http.NewRequestWithContext(ctx, method, strings.TrimSpace(config.Request.URL), bytes.NewReader(body))
	if err != nil {
		return "", requestSnapshot, map[string]any{"error": err.Error()}, err
	}
	for key, value := range config.Request.Headers {
		req.Header.Set(key, value)
	}
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	statusCode, headers, responseBody, err := w.doRequest(channel.TimeoutMS, req)
	responseSnapshot := map[string]any{
		"status_code": statusCode,
		"headers":     headers,
		"body":        snapshotValue(responseBody),
	}
	if err != nil {
		responseSnapshot["error"] = err.Error()
		return "", requestSnapshot, responseSnapshot, err
	}
	if statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices {
		return "", requestSnapshot, responseSnapshot, fmt.Errorf("token endpoint returned status %d", statusCode)
	}

	token, err := extractJSONPath(responseBody, config.ResponsePath)
	if err != nil {
		responseSnapshot["error"] = err.Error()
		return "", requestSnapshot, responseSnapshot, err
	}
	responseSnapshot["resolved_token"] = token
	return token, requestSnapshot, responseSnapshot, nil
}

func (w *Worker) send(ctx context.Context, channel provider.Channel, built provider.BuiltRequest) (int, map[string][]string, []byte, error) {
	body := built.Body
	if len(bytes.TrimSpace(body)) == 0 {
		body = json.RawMessage(`{}`)
	}
	req, err := http.NewRequestWithContext(ctx, built.Method, built.URL, bytes.NewReader(body))
	if err != nil {
		return 0, nil, nil, err
	}
	for key, value := range built.Headers {
		req.Header.Set(key, value)
	}
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	return w.doRequest(channel.TimeoutMS, req)
}

func (w *Worker) doRequest(timeoutMS int, req *http.Request) (int, map[string][]string, []byte, error) {
	timeout := time.Duration(timeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	client := w.httpClientFactory(timeout)
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, nil, err
	}
	defer resp.Body.Close()

	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return resp.StatusCode, resp.Header, body, readErr
	}
	return resp.StatusCode, resp.Header, body, nil
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

type tokenResolverConfig struct {
	Request      tokenRequestConfig `json:"request"`
	ResponsePath string             `json:"response_path"`
	Placement    json.RawMessage    `json:"placement"`
}

type tokenRequestConfig struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
	Body    json.RawMessage   `json:"body"`
}

func parseTokenBehavior(channel provider.Channel) (*tokenResolverConfig, json.RawMessage, error) {
	placement := extractPlacement(channel.TokenConfig)
	if len(placement) == 0 {
		placement = extractPlacement(channel.AuthConfig)
	}

	if resolver, err := decodeResolver(channel.TokenConfig); err != nil {
		return nil, nil, err
	} else if resolver != nil {
		if len(placement) == 0 && len(resolver.Placement) > 0 {
			placement = append(json.RawMessage(nil), resolver.Placement...)
		}
		return resolver, placement, nil
	}

	resolver, err := decodeResolver(channel.AuthConfig)
	if err != nil {
		return nil, nil, err
	}
	if resolver != nil && len(placement) == 0 && len(resolver.Placement) > 0 {
		placement = append(json.RawMessage(nil), resolver.Placement...)
	}
	return resolver, placement, nil
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
	Enabled   bool    `json:"enabled"`
	QPS       float64 `json:"qps"`
	PerMinute float64 `json:"per_minute"`
	Burst     int     `json:"burst"`
}

func rateLimitFrom(raw json.RawMessage) rateLimitConfig {
	var cfg rateLimitConfig
	if len(bytes.TrimSpace(raw)) == 0 {
		return cfg
	}
	_ = json.Unmarshal(raw, &cfg)
	if !cfg.Enabled {
		return cfg
	}
	if cfg.Burst <= 0 {
		cfg.Burst = 1
	}
	return cfg
}

type channelLimiter struct {
	now   func() time.Time
	rate  float64
	burst float64

	mu     sync.Mutex
	tokens float64
	last   time.Time
}

func newChannelLimiter(cfg rateLimitConfig, now func() time.Time) *channelLimiter {
	rate := cfg.QPS
	if rate <= 0 && cfg.PerMinute > 0 {
		rate = cfg.PerMinute / 60
	}
	if rate <= 0 {
		rate = math.Inf(1)
	}
	return &channelLimiter{
		now:   now,
		rate:  rate,
		burst: float64(cfg.Burst),
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
			l.tokens = l.burst
		}
		elapsed := now.Sub(l.last).Seconds()
		if elapsed > 0 {
			l.tokens = math.Min(l.burst, l.tokens+(elapsed*l.rate))
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
