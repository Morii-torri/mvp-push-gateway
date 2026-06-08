package source

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"mvp-push-gateway/backend/internal/queue"
)

const (
	DefaultMaxPayloadBytes            int64 = 5 << 20
	defaultDedupeTTL                        = 24 * time.Hour
	maxQuietHoursWindows                    = 5
	hmacTimestampWindow                     = 5 * time.Minute
	hmacNonceTTL                            = 10 * time.Minute
	defaultSourceConfigCacheTTL             = 5 * time.Second
	defaultLatestPayloadFlushInterval       = time.Second
	latestPayloadFlushTimeout               = 5 * time.Second
)

type AuthMode string

const (
	AuthModeToken        AuthMode = "token"
	AuthModeHMAC         AuthMode = "hmac"
	AuthModeTokenAndHMAC AuthMode = "token_and_hmac"
	AuthModeNone         AuthMode = "none"
)

type DedupeStrategy string

const (
	DedupeStrategyPayloadHash DedupeStrategy = "payload_hash"
)

var (
	ErrNotFound             = errors.New("source not found")
	ErrAlreadyExists        = errors.New("source already exists")
	ErrDisabled             = errors.New("source disabled")
	ErrInvalidInput         = errors.New("invalid source input")
	ErrUnauthorized         = errors.New("source unauthorized")
	ErrIPNotAllowed         = errors.New("source ip not allowed")
	ErrInvalidJSON          = errors.New("invalid json payload")
	ErrPayloadTooLarge      = errors.New("payload too large")
	ErrRateLimited          = errors.New("source rate limited")
	ErrDuplicateInbound     = errors.New("duplicate inbound payload")
	ErrInvalidDedupeConfig  = errors.New("invalid inbound dedupe config")
	ErrDedupeStoreFailed    = errors.New("inbound dedupe store failed")
	ErrHMACNonceStoreFailed = errors.New("hmac nonce store failed")
	ErrQueuePublishFailed   = errors.New("source queue publish failed")
)

type Source struct {
	ID                           string
	Code                         string
	Name                         string
	Enabled                      bool
	AuthMode                     AuthMode
	AuthToken                    string
	HMACSecret                   string
	IPAllowlist                  []string
	CompatMode                   string
	InboundDedupeEnabled         bool
	InboundDedupeStrategy        DedupeStrategy
	InboundDedupeConfig          json.RawMessage
	RateLimitConfig              json.RawMessage
	QuietHoursConfig             json.RawMessage
	LatestPayloadSample          json.RawMessage
	LatestPayloadSampleUpdatedAt *time.Time
	CreatedAt                    time.Time
	UpdatedAt                    time.Time
}

type CreateSourceInput struct {
	Code                         string
	Name                         string
	Enabled                      bool
	AuthMode                     AuthMode
	AuthToken                    string
	HMACSecret                   string
	IPAllowlist                  []string
	CompatMode                   string
	InboundDedupeEnabled         bool
	InboundDedupeStrategy        DedupeStrategy
	InboundDedupeConfig          json.RawMessage
	RateLimitConfig              json.RawMessage
	QuietHoursConfig             json.RawMessage
	LatestPayloadSample          json.RawMessage
	LatestPayloadSampleUpdatedAt *time.Time
}

type UpdateSourceInput = CreateSourceInput

type CreateSourceParams = CreateSourceInput
type UpdateSourceParams = UpdateSourceInput

type IngestInput struct {
	SourceCode        string
	Method            string
	Path              string
	Headers           http.Header
	RemoteAddr        string
	Body              []byte
	PersistBeforePlan bool
}

type IngestResult struct {
	TraceID string
	Status  string
	Message string
}

type IngestTimingStage string

const (
	IngestTimingSourceLookup             IngestTimingStage = "source_lookup"
	IngestTimingLatestPayloadUpdate      IngestTimingStage = "latest_payload"
	IngestTimingEnqueueInbound           IngestTimingStage = "enqueue_inbound"
	IngestTimingInsertMessageRecord      IngestTimingStage = "insert_message_record"
	IngestTimingInsertInboundDedupeKey   IngestTimingStage = "insert_inbound_dedupe_key"
	IngestTimingInsertRoutePlanJob       IngestTimingStage = "insert_route_plan_job"
	IngestTimingCommitInboundTransaction IngestTimingStage = "commit_inbound"
)

type IngestTimingRecorder interface {
	RecordIngestTiming(stage IngestTimingStage, duration time.Duration)
}

type ingestTimingRecorderContextKey struct{}

func WithIngestTimingRecorder(ctx context.Context, recorder IngestTimingRecorder) context.Context {
	if recorder == nil {
		return ctx
	}
	return context.WithValue(ctx, ingestTimingRecorderContextKey{}, recorder)
}

func RecordIngestTiming(ctx context.Context, stage IngestTimingStage, duration time.Duration) {
	recorder, ok := ctx.Value(ingestTimingRecorderContextKey{}).(IngestTimingRecorder)
	if !ok || recorder == nil {
		return
	}
	recorder.RecordIngestTiming(stage, duration)
}

type RuntimeStats struct {
	DBPoolAcquireCount     int64
	DBPoolWaitCount        int64
	DBPoolWaitDurationMS   int64
	DBPoolAcquiredConns    int32
	DBPoolTotalConns       int32
	PostgresMaxConnections int32
	PostgresBlocksRead     int64
	PostgresBlocksHit      int64
	PostgresTempBytes      int64
}

type EnqueueInboundParams struct {
	MessageID     string
	TraceID       string
	SourceID      string
	Headers       json.RawMessage
	Payload       json.RawMessage
	PayloadHash   string
	ReceivedAt    time.Time
	DedupeEnabled bool
	DedupeKey     string
	DedupeExpires time.Time
	Status        string
	ErrorCode     string
	ErrorMessage  string
	SkipRoutePlan bool
	JobType       string
	JobPayload    json.RawMessage
}

type quietHoursConfig struct {
	Enabled bool               `json:"enabled"`
	Windows []quietHoursWindow `json:"windows"`
}

type quietHoursWindow struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

type Store interface {
	ListSources(ctx context.Context) ([]Source, error)
	CreateSource(ctx context.Context, params CreateSourceParams) (Source, error)
	GetSource(ctx context.Context, id string) (Source, error)
	GetSourceByCode(ctx context.Context, code string) (Source, error)
	UpdateSource(ctx context.Context, id string, params UpdateSourceParams) (Source, error)
	DeleteSource(ctx context.Context, id string) error
	UpdateLatestPayloadSample(ctx context.Context, sourceID string, payload json.RawMessage, sampledAt time.Time) error
	ReserveHMACNonce(ctx context.Context, sourceID string, nonce string, now time.Time, expiresAt time.Time) (bool, error)
	EnqueueInbound(ctx context.Context, params EnqueueInboundParams) error
	DeleteSourceRuntimeData(ctx context.Context, sourceID string) error
}

type runtimeStatsStore interface {
	RuntimeStats(ctx context.Context) (RuntimeStats, error)
}

type performanceDeliveryStatusStore interface {
	PerformanceDeliveryStatuses(ctx context.Context, traceIDs []string) (map[string]bool, error)
	PerformanceDeliveryStatusDetails(ctx context.Context, traceIDs []string) (map[string]PerformanceDeliveryStatus, error)
}

type PerformanceDeliveryStatus struct {
	Sent        bool
	ReceivedAt  time.Time
	FinishedAt  time.Time
	PersistedAt time.Time
}

type RoutePlanPublisher interface {
	PublishRoutePlan(context.Context, queue.RoutePlanEvent) (queue.PublishResult, error)
}

type LatestPayloadStore interface {
	PutLatestPayloadSample(ctx context.Context, sourceID string, payload json.RawMessage, sampledAt time.Time) error
	GetLatestPayloadSample(ctx context.Context, sourceID string) (json.RawMessage, time.Time, bool, error)
	DeleteLatestPayloadSample(ctx context.Context, sourceID string) error
}

type InboundDedupeStore interface {
	ReserveInboundDedupeKey(ctx context.Context, sourceID string, dedupeKey string, messageID string, expiresAt time.Time) (bool, error)
}

type HMACNonceStore interface {
	ReserveHMACNonce(ctx context.Context, sourceID string, nonce string, now time.Time, expiresAt time.Time) (bool, error)
}

type Service struct {
	store          Store
	now            func() time.Time
	traceID        func() string
	maxPayloadSize int64
	maxPayloadFunc func(context.Context) int64

	sourceCacheMu     sync.RWMutex
	sourceCacheByCode map[string]sourceConfigCacheEntry
	sourceCacheTTL    time.Duration

	limiterMu sync.Mutex
	limiters  map[string]*rateWindow

	latestPayloadMu            sync.Mutex
	latestPayloadPending       map[string]latestPayloadSample
	latestPayloadTimers        map[string]*time.Timer
	latestPayloadFlushInterval time.Duration

	routePlanPublisher RoutePlanPublisher
	latestPayloadStore LatestPayloadStore
	inboundDedupeStore InboundDedupeStore
	hmacNonceStore     HMACNonceStore
}

type Option func(*Service)

type latestPayloadSample struct {
	payload   json.RawMessage
	sampledAt time.Time
}

type sourceConfigCacheEntry struct {
	source    Source
	expiresAt time.Time
}

func WithNow(now func() time.Time) Option {
	return func(s *Service) {
		if now != nil {
			s.now = now
		}
	}
}

func WithTraceIDGenerator(traceID func() string) Option {
	return func(s *Service) {
		if traceID != nil {
			s.traceID = traceID
		}
	}
}

func WithMaxPayloadSizeFunc(maxPayloadFunc func(context.Context) int64) Option {
	return func(s *Service) {
		if maxPayloadFunc != nil {
			s.maxPayloadFunc = maxPayloadFunc
		}
	}
}

func WithRoutePlanPublisher(publisher RoutePlanPublisher) Option {
	return func(s *Service) {
		s.routePlanPublisher = publisher
	}
}

func WithLatestPayloadStore(store LatestPayloadStore) Option {
	return func(s *Service) {
		s.latestPayloadStore = store
	}
}

func WithInboundDedupeStore(store InboundDedupeStore) Option {
	return func(s *Service) {
		s.inboundDedupeStore = store
	}
}

func WithHMACNonceStore(store HMACNonceStore) Option {
	return func(s *Service) {
		s.hmacNonceStore = store
	}
}

func WithSourceConfigCacheTTL(ttl time.Duration) Option {
	return func(s *Service) {
		s.sourceCacheTTL = ttl
	}
}

func WithLatestPayloadFlushInterval(interval time.Duration) Option {
	return func(s *Service) {
		if interval >= 0 {
			s.latestPayloadFlushInterval = interval
		}
	}
}

func NewService(store Store, options ...Option) *Service {
	service := &Service{
		store:                      store,
		now:                        time.Now,
		traceID:                    uuid.NewString,
		maxPayloadSize:             DefaultMaxPayloadBytes,
		sourceCacheByCode:          make(map[string]sourceConfigCacheEntry),
		sourceCacheTTL:             defaultSourceConfigCacheTTL,
		limiters:                   make(map[string]*rateWindow),
		latestPayloadPending:       make(map[string]latestPayloadSample),
		latestPayloadTimers:        make(map[string]*time.Timer),
		latestPayloadFlushInterval: defaultLatestPayloadFlushInterval,
	}
	for _, option := range options {
		option(service)
	}
	return service
}

func (s *Service) RuntimeStats(ctx context.Context) (RuntimeStats, error) {
	statsStore, ok := s.store.(runtimeStatsStore)
	if !ok {
		return RuntimeStats{}, nil
	}
	return statsStore.RuntimeStats(ctx)
}

func (s *Service) scheduleLatestPayloadSample(sourceID string, payload json.RawMessage, sampledAt time.Time) {
	if sourceID == "" {
		return
	}
	copiedPayload := append(json.RawMessage(nil), payload...)
	s.latestPayloadMu.Lock()
	defer s.latestPayloadMu.Unlock()
	if s.latestPayloadPending == nil {
		s.latestPayloadPending = make(map[string]latestPayloadSample)
	}
	if s.latestPayloadTimers == nil {
		s.latestPayloadTimers = make(map[string]*time.Timer)
	}
	s.latestPayloadPending[sourceID] = latestPayloadSample{payload: copiedPayload, sampledAt: sampledAt}
	if _, exists := s.latestPayloadTimers[sourceID]; exists {
		return
	}
	interval := s.latestPayloadFlushInterval
	s.latestPayloadTimers[sourceID] = time.AfterFunc(interval, func() {
		s.flushLatestPayloadSample(sourceID)
	})
}

func (s *Service) flushLatestPayloadSample(sourceID string) {
	if sourceID == "" {
		return
	}
	s.latestPayloadMu.Lock()
	sample, ok := s.latestPayloadPending[sourceID]
	delete(s.latestPayloadPending, sourceID)
	delete(s.latestPayloadTimers, sourceID)
	s.latestPayloadMu.Unlock()
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), latestPayloadFlushTimeout)
	defer cancel()
	if s.latestPayloadStore != nil {
		_ = s.latestPayloadStore.PutLatestPayloadSample(ctx, sourceID, sample.payload, sample.sampledAt)
		return
	}
	_ = s.store.UpdateLatestPayloadSample(ctx, sourceID, sample.payload, sample.sampledAt)
}

func (s *Service) clearLatestPayloadSamplePending(sourceID string) {
	if sourceID == "" {
		return
	}
	s.latestPayloadMu.Lock()
	defer s.latestPayloadMu.Unlock()
	delete(s.latestPayloadPending, sourceID)
	if timer, ok := s.latestPayloadTimers[sourceID]; ok {
		timer.Stop()
		delete(s.latestPayloadTimers, sourceID)
	}
}

func (s *Service) ListSources(ctx context.Context) ([]Source, error) {
	if s.store == nil {
		return nil, ErrNotFound
	}
	sources, err := s.store.ListSources(ctx)
	if err != nil {
		return nil, err
	}
	for index := range sources {
		sources[index] = s.applyLatestPayloadSample(ctx, sources[index])
	}
	return sources, nil
}

func (s *Service) CreateSource(ctx context.Context, input CreateSourceInput) (Source, error) {
	params, err := normalizeSourceInput(input)
	if err != nil {
		return Source{}, err
	}
	created, err := s.store.CreateSource(ctx, params)
	if err != nil {
		return Source{}, err
	}
	s.setSourceConfigCache(created)
	return created, nil
}

func (s *Service) GetSource(ctx context.Context, id string) (Source, error) {
	if strings.TrimSpace(id) == "" {
		return Source{}, ErrInvalidInput
	}
	configuredSource, err := s.store.GetSource(ctx, id)
	if err != nil {
		return Source{}, err
	}
	return s.applyLatestPayloadSample(ctx, configuredSource), nil
}

func (s *Service) UpdateSource(ctx context.Context, id string, input UpdateSourceInput) (Source, error) {
	if strings.TrimSpace(id) == "" {
		return Source{}, ErrInvalidInput
	}
	params, err := normalizeSourceInput(input)
	if err != nil {
		return Source{}, err
	}
	existing, err := s.store.GetSource(ctx, id)
	if err != nil {
		return Source{}, err
	}
	if existing.Code != params.Code {
		return Source{}, ErrInvalidInput
	}
	updated, err := s.store.UpdateSource(ctx, id, params)
	if err != nil {
		return Source{}, err
	}
	s.setSourceConfigCache(updated)
	return updated, nil
}

func (s *Service) DeleteSource(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return ErrInvalidInput
	}
	if err := s.store.DeleteSource(ctx, id); err != nil {
		return err
	}
	s.invalidateSourceConfigCache("")
	s.clearLatestPayloadSamplePending(id)
	s.deleteLatestPayloadSample(ctx, id)
	return nil
}

func (s *Service) DeleteSourceRuntimeData(ctx context.Context, sourceID string) error {
	if strings.TrimSpace(sourceID) == "" {
		return ErrInvalidInput
	}
	if err := s.store.DeleteSourceRuntimeData(ctx, sourceID); err != nil {
		return err
	}
	s.clearLatestPayloadSamplePending(sourceID)
	s.deleteLatestPayloadSample(ctx, sourceID)
	return nil
}

func (s *Service) PerformanceDeliveryStatuses(ctx context.Context, traceIDs []string) (map[string]bool, error) {
	store, ok := s.store.(performanceDeliveryStatusStore)
	if !ok {
		return map[string]bool{}, nil
	}
	cleaned := cleanStringSlice(traceIDs)
	if len(cleaned) == 0 {
		return map[string]bool{}, nil
	}
	return store.PerformanceDeliveryStatuses(ctx, cleaned)
}

func (s *Service) PerformanceDeliveryStatusDetails(ctx context.Context, traceIDs []string) (map[string]PerformanceDeliveryStatus, error) {
	store, ok := s.store.(performanceDeliveryStatusStore)
	if !ok {
		return map[string]PerformanceDeliveryStatus{}, nil
	}
	cleaned := cleanStringSlice(traceIDs)
	if len(cleaned) == 0 {
		return map[string]PerformanceDeliveryStatus{}, nil
	}
	return store.PerformanceDeliveryStatusDetails(ctx, cleaned)
}

func (s *Service) Ingest(ctx context.Context, input IngestInput) (IngestResult, error) {
	sourceCode := strings.TrimSpace(input.SourceCode)
	if sourceCode == "" {
		return IngestResult{}, ErrNotFound
	}

	startedAt := time.Now()
	configuredSource, err := s.sourceByCode(ctx, sourceCode)
	RecordIngestTiming(ctx, IngestTimingSourceLookup, time.Since(startedAt))
	if err != nil {
		return IngestResult{}, err
	}
	if !configuredSource.Enabled {
		return IngestResult{}, ErrDisabled
	}
	if !clientAllowed(configuredSource.IPAllowlist, input.RemoteAddr) {
		return IngestResult{}, ErrIPNotAllowed
	}
	if int64(len(input.Body)) > s.maxPayloadBytes(ctx) {
		return IngestResult{}, ErrPayloadTooLarge
	}
	authorized, err := s.authorizeSource(ctx, configuredSource, input)
	if err != nil {
		return IngestResult{}, err
	}
	if !authorized {
		return IngestResult{}, ErrUnauthorized
	}

	payload, err := compactJSON(input.Body)
	if err != nil {
		return IngestResult{}, ErrInvalidJSON
	}

	payloadHash := sha256Hex(input.Body)
	traceID := s.traceID()
	messageID := uuid.NewString()
	now := s.now()
	receivedAt := now.UTC()
	s.scheduleLatestPayloadSample(configuredSource.ID, payload, receivedAt)
	headers, err := json.Marshal(input.Headers)
	if err != nil {
		return IngestResult{}, err
	}
	if sourceInQuietHours(configuredSource, now) {
		startedAt = time.Now()
		err := s.store.EnqueueInbound(ctx, EnqueueInboundParams{
			MessageID:     messageID,
			TraceID:       traceID,
			SourceID:      configuredSource.ID,
			Headers:       headers,
			Payload:       payload,
			PayloadHash:   payloadHash,
			ReceivedAt:    receivedAt,
			Status:        "silenced",
			ErrorCode:     "MGP-DND-001",
			ErrorMessage:  "消息免打扰时间段内静默",
			SkipRoutePlan: true,
		})
		RecordIngestTiming(ctx, IngestTimingEnqueueInbound, time.Since(startedAt))
		if err != nil {
			return IngestResult{}, err
		}
		return IngestResult{
			TraceID: traceID,
			Status:  "silenced",
			Message: "silenced",
		}, nil
	}

	if s.rateLimited(configuredSource) {
		return IngestResult{}, ErrRateLimited
	}

	dedupeKey := ""
	dedupeTTL := defaultDedupeTTL
	dedupeReservedByRuntimeStore := false
	if configuredSource.InboundDedupeEnabled {
		key, err := inboundDedupeKey(configuredSource, payloadHash)
		if err != nil {
			return IngestResult{}, err
		}
		dedupeKey = key
		dedupeTTL = inboundDedupeTTL(configuredSource.InboundDedupeConfig)
		if s.inboundDedupeStore != nil && s.routePlanPublisher != nil {
			startedAt = time.Now()
			reserved, err := s.inboundDedupeStore.ReserveInboundDedupeKey(ctx, configuredSource.ID, dedupeKey, messageID, receivedAt.Add(dedupeTTL))
			RecordIngestTiming(ctx, IngestTimingInsertInboundDedupeKey, time.Since(startedAt))
			if err != nil {
				return IngestResult{}, fmt.Errorf("%w: %v", ErrDedupeStoreFailed, err)
			}
			if !reserved {
				startedAt = time.Now()
				err := s.store.EnqueueInbound(ctx, EnqueueInboundParams{
					MessageID:     messageID,
					TraceID:       traceID,
					SourceID:      configuredSource.ID,
					Headers:       headers,
					Payload:       payload,
					PayloadHash:   payloadHash,
					ReceivedAt:    receivedAt,
					Status:        "deduped",
					ErrorCode:     "MGP-DEDUPE-001",
					ErrorMessage:  "入站重复",
					SkipRoutePlan: true,
				})
				RecordIngestTiming(ctx, IngestTimingEnqueueInbound, time.Since(startedAt))
				if err != nil {
					return IngestResult{}, err
				}
				return IngestResult{}, ErrDuplicateInbound
			}
			dedupeReservedByRuntimeStore = true
		}
	}

	jobPayload, err := json.Marshal(map[string]string{
		"message_id": messageID,
		"source_id":  configuredSource.ID,
		"trace_id":   traceID,
	})
	if err != nil {
		return IngestResult{}, err
	}

	startedAt = time.Now()
	if s.routePlanPublisher != nil && !input.PersistBeforePlan && (!configuredSource.InboundDedupeEnabled || dedupeReservedByRuntimeStore) {
		if _, err := s.routePlanPublisher.PublishRoutePlan(ctx, queue.RoutePlanEvent{
			MessageID:  messageID,
			SourceID:   configuredSource.ID,
			TraceID:    traceID,
			Headers:    headers,
			Payload:    payload,
			ReceivedAt: receivedAt,
		}); err != nil {
			RecordIngestTiming(ctx, IngestTimingEnqueueInbound, time.Since(startedAt))
			return IngestResult{}, fmt.Errorf("%w: %v", ErrQueuePublishFailed, err)
		}
		RecordIngestTiming(ctx, IngestTimingEnqueueInbound, time.Since(startedAt))
		return IngestResult{
			TraceID: traceID,
			Status:  "accepted",
			Message: "accepted",
		}, nil
	}

	enqueueParams := EnqueueInboundParams{
		MessageID:     messageID,
		TraceID:       traceID,
		SourceID:      configuredSource.ID,
		Headers:       headers,
		Payload:       payload,
		PayloadHash:   payloadHash,
		ReceivedAt:    receivedAt,
		DedupeEnabled: configuredSource.InboundDedupeEnabled,
		DedupeKey:     dedupeKey,
		DedupeExpires: receivedAt.Add(dedupeTTL),
		Status:        "accepted",
		JobType:       "route_plan",
		JobPayload:    jobPayload,
	}
	if s.routePlanPublisher != nil {
		enqueueParams.SkipRoutePlan = true
		enqueueParams.JobType = ""
		enqueueParams.JobPayload = nil
	}
	err = s.store.EnqueueInbound(ctx, enqueueParams)
	RecordIngestTiming(ctx, IngestTimingEnqueueInbound, time.Since(startedAt))
	if err != nil {
		return IngestResult{}, err
	}
	if s.routePlanPublisher != nil {
		if _, err := s.routePlanPublisher.PublishRoutePlan(ctx, queue.RoutePlanEvent{
			MessageID: messageID,
			SourceID:  configuredSource.ID,
			TraceID:   traceID,
		}); err != nil {
			return IngestResult{}, fmt.Errorf("%w: %v", ErrQueuePublishFailed, err)
		}
	}

	return IngestResult{
		TraceID: traceID,
		Status:  "accepted",
		Message: "accepted",
	}, nil
}

func (s *Service) sourceByCode(ctx context.Context, code string) (Source, error) {
	if s == nil || s.store == nil {
		return Source{}, ErrNotFound
	}
	code = strings.TrimSpace(code)
	if code == "" {
		return Source{}, ErrNotFound
	}
	ttl := s.sourceCacheTTL
	if ttl > 0 {
		now := time.Now()
		s.sourceCacheMu.RLock()
		entry, ok := s.sourceCacheByCode[code]
		s.sourceCacheMu.RUnlock()
		if ok && entry.expiresAt.After(now) {
			return cloneSource(entry.source), nil
		}

		s.sourceCacheMu.Lock()
		defer s.sourceCacheMu.Unlock()
		entry, ok = s.sourceCacheByCode[code]
		if ok && entry.expiresAt.After(now) {
			return cloneSource(entry.source), nil
		}
		configuredSource, err := s.store.GetSourceByCode(ctx, code)
		if err != nil {
			return Source{}, err
		}
		if s.sourceCacheByCode == nil {
			s.sourceCacheByCode = make(map[string]sourceConfigCacheEntry)
		}
		s.sourceCacheByCode[code] = sourceConfigCacheEntry{
			source:    cloneSource(configuredSource),
			expiresAt: time.Now().Add(ttl),
		}
		return cloneSource(configuredSource), nil
	}
	configuredSource, err := s.store.GetSourceByCode(ctx, code)
	if err != nil {
		return Source{}, err
	}
	return configuredSource, nil
}

func (s *Service) invalidateSourceConfigCache(code string) {
	if s == nil {
		return
	}
	code = strings.TrimSpace(code)
	s.sourceCacheMu.Lock()
	defer s.sourceCacheMu.Unlock()
	if code == "" {
		s.sourceCacheByCode = make(map[string]sourceConfigCacheEntry)
		return
	}
	delete(s.sourceCacheByCode, code)
}

func (s *Service) setSourceConfigCache(configuredSource Source) {
	if s == nil || s.sourceCacheTTL <= 0 {
		return
	}
	code := strings.TrimSpace(configuredSource.Code)
	if code == "" {
		return
	}
	s.sourceCacheMu.Lock()
	defer s.sourceCacheMu.Unlock()
	if s.sourceCacheByCode == nil {
		s.sourceCacheByCode = make(map[string]sourceConfigCacheEntry)
	}
	s.sourceCacheByCode[code] = sourceConfigCacheEntry{
		source:    cloneSource(configuredSource),
		expiresAt: time.Now().Add(s.sourceCacheTTL),
	}
}

func (s *Service) applyLatestPayloadSample(ctx context.Context, configuredSource Source) Source {
	if s == nil || s.latestPayloadStore == nil || strings.TrimSpace(configuredSource.ID) == "" {
		return configuredSource
	}
	payload, sampledAt, found, err := s.latestPayloadStore.GetLatestPayloadSample(ctx, configuredSource.ID)
	if err != nil || !found {
		return configuredSource
	}
	configuredSource.LatestPayloadSample = append(json.RawMessage(nil), payload...)
	value := sampledAt.UTC()
	configuredSource.LatestPayloadSampleUpdatedAt = &value
	return configuredSource
}

func (s *Service) deleteLatestPayloadSample(ctx context.Context, sourceID string) {
	if s == nil || s.latestPayloadStore == nil || strings.TrimSpace(sourceID) == "" {
		return
	}
	_ = s.latestPayloadStore.DeleteLatestPayloadSample(ctx, sourceID)
}

func cloneSource(input Source) Source {
	input.IPAllowlist = append([]string(nil), input.IPAllowlist...)
	input.InboundDedupeConfig = append(json.RawMessage(nil), input.InboundDedupeConfig...)
	input.RateLimitConfig = append(json.RawMessage(nil), input.RateLimitConfig...)
	input.QuietHoursConfig = append(json.RawMessage(nil), input.QuietHoursConfig...)
	input.LatestPayloadSample = append(json.RawMessage(nil), input.LatestPayloadSample...)
	return input
}

func (s *Service) maxPayloadBytes(ctx context.Context) int64 {
	if s.maxPayloadFunc == nil {
		return s.maxPayloadSize
	}
	maxPayloadSize := s.maxPayloadFunc(ctx)
	if maxPayloadSize <= 0 {
		return s.maxPayloadSize
	}
	return maxPayloadSize
}

func normalizeSourceInput(input CreateSourceInput) (CreateSourceParams, error) {
	input.Code = strings.TrimSpace(input.Code)
	input.Name = strings.TrimSpace(input.Name)
	input.AuthToken = strings.TrimSpace(input.AuthToken)
	input.HMACSecret = strings.TrimSpace(input.HMACSecret)
	input.CompatMode = strings.TrimSpace(input.CompatMode)
	if input.Code == "" || input.Name == "" {
		return CreateSourceParams{}, ErrInvalidInput
	}
	if !isAlphanumeric(input.Code) ||
		(input.AuthToken != "" && !isAlphanumeric(input.AuthToken)) ||
		(input.HMACSecret != "" && !isAlphanumeric(input.HMACSecret)) {
		return CreateSourceParams{}, ErrInvalidInput
	}
	if input.AuthMode == "" {
		input.AuthMode = AuthModeToken
	}
	if !validAuthMode(input.AuthMode) {
		return CreateSourceParams{}, ErrInvalidInput
	}
	if input.CompatMode == "" {
		input.CompatMode = "standard"
	}
	input.InboundDedupeStrategy = DedupeStrategyPayloadHash
	for _, entry := range input.IPAllowlist {
		if _, err := parseIPAllowlistEntry(entry); err != nil {
			return CreateSourceParams{}, ErrInvalidInput
		}
	}

	dedupeConfig, err := normalizeJSONConfig(input.InboundDedupeConfig)
	if err != nil {
		return CreateSourceParams{}, err
	}
	rateLimitConfig, err := normalizeJSONConfig(input.RateLimitConfig)
	if err != nil {
		return CreateSourceParams{}, err
	}
	quietHoursConfig, err := normalizeQuietHoursConfig(input.QuietHoursConfig)
	if err != nil {
		return CreateSourceParams{}, err
	}
	latestPayloadSample, err := normalizeOptionalJSON(input.LatestPayloadSample)
	if err != nil {
		return CreateSourceParams{}, err
	}

	input.IPAllowlist = cleanStringSlice(input.IPAllowlist)
	input.InboundDedupeConfig = dedupeConfig
	input.RateLimitConfig = rateLimitConfig
	input.QuietHoursConfig = quietHoursConfig
	input.LatestPayloadSample = latestPayloadSample
	return input, nil
}

func validAuthMode(authMode AuthMode) bool {
	switch authMode {
	case AuthModeToken, AuthModeHMAC, AuthModeTokenAndHMAC, AuthModeNone:
		return true
	default:
		return false
	}
}

func isAlphanumeric(value string) bool {
	if value == "" {
		return false
	}
	for _, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') {
			continue
		}
		return false
	}
	return true
}

func normalizeJSONConfig(raw json.RawMessage) (json.RawMessage, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return json.RawMessage(`{}`), nil
	}
	return normalizeOptionalJSON(raw)
}

func normalizeQuietHoursConfig(raw json.RawMessage) (json.RawMessage, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return json.RawMessage(`{"enabled":false,"windows":[]}`), nil
	}
	config, err := decodeQuietHoursConfig(raw)
	if err != nil {
		return nil, err
	}
	if !config.Enabled {
		config.Windows = []quietHoursWindow{}
		return marshalQuietHoursConfig(config)
	}
	if len(config.Windows) == 0 || len(config.Windows) > maxQuietHoursWindows {
		return nil, ErrInvalidInput
	}
	for index := range config.Windows {
		start, err := parseClockMinute(config.Windows[index].Start)
		if err != nil {
			return nil, err
		}
		end, err := parseClockMinute(config.Windows[index].End)
		if err != nil {
			return nil, err
		}
		if start == end {
			return nil, ErrInvalidInput
		}
		config.Windows[index].Start = formatClockMinute(start)
		config.Windows[index].End = formatClockMinute(end)
	}
	return marshalQuietHoursConfig(config)
}

func decodeQuietHoursConfig(raw json.RawMessage) (quietHoursConfig, error) {
	normalized, err := normalizeOptionalJSON(raw)
	if err != nil {
		return quietHoursConfig{}, err
	}
	var config quietHoursConfig
	if err := json.Unmarshal(normalized, &config); err != nil {
		return quietHoursConfig{}, ErrInvalidInput
	}
	if config.Windows == nil {
		config.Windows = []quietHoursWindow{}
	}
	return config, nil
}

func marshalQuietHoursConfig(config quietHoursConfig) (json.RawMessage, error) {
	raw, err := json.Marshal(config)
	if err != nil {
		return nil, ErrInvalidInput
	}
	return json.RawMessage(raw), nil
}

func normalizeOptionalJSON(raw json.RawMessage) (json.RawMessage, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, nil
	}
	if !json.Valid(raw) {
		return nil, ErrInvalidInput
	}
	normalized := append(json.RawMessage(nil), bytes.TrimSpace(raw)...)
	return normalized, nil
}

func cleanStringSlice(values []string) []string {
	cleaned := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			cleaned = append(cleaned, value)
		}
	}
	return cleaned
}

func compactJSON(raw []byte) (json.RawMessage, error) {
	var buffer bytes.Buffer
	if err := json.Compact(&buffer, raw); err != nil {
		return nil, err
	}
	return append(json.RawMessage(nil), buffer.Bytes()...), nil
}

func (s *Service) authorizeSource(ctx context.Context, configuredSource Source, input IngestInput) (bool, error) {
	switch configuredSource.AuthMode {
	case AuthModeNone:
		return true, nil
	case AuthModeToken:
		return sourceBearerToken(input.Headers) == configuredSource.AuthToken && configuredSource.AuthToken != "", nil
	case AuthModeHMAC:
		return s.validHMAC(ctx, configuredSource.ID, configuredSource.HMACSecret, input.Method, input.Path, input.Headers, input.Body)
	case AuthModeTokenAndHMAC:
		if sourceBearerToken(input.Headers) != configuredSource.AuthToken || configuredSource.AuthToken == "" {
			return false, nil
		}
		return s.validHMAC(ctx, configuredSource.ID, configuredSource.HMACSecret, input.Method, input.Path, input.Headers, input.Body)
	default:
		return false, nil
	}
}

func sourceBearerToken(headers http.Header) string {
	value := strings.TrimSpace(headers.Get("Authorization"))
	if value == "" {
		return ""
	}
	const prefix = "Bearer "
	if !strings.HasPrefix(value, prefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(value, prefix))
}

func (s *Service) validHMAC(ctx context.Context, sourceID string, secret string, method string, path string, headers http.Header, body []byte) (bool, error) {
	now := s.now()
	nonce, ok := validHMACSignature(secret, method, path, headers, body, now)
	if !ok {
		return false, nil
	}
	if s.hmacNonceStore != nil {
		reserved, err := s.hmacNonceStore.ReserveHMACNonce(ctx, sourceID, nonce, now, now.Add(hmacNonceTTL))
		if err != nil {
			return false, fmt.Errorf("%w: %v", ErrHMACNonceStoreFailed, err)
		}
		return reserved, nil
	}
	return s.store.ReserveHMACNonce(ctx, sourceID, nonce, now, now.Add(hmacNonceTTL))
}

func validHMACSignature(secret string, method string, path string, headers http.Header, body []byte, now time.Time) (string, bool) {
	secret = strings.TrimSpace(secret)
	timestamp := strings.TrimSpace(headers.Get("X-MGP-Timestamp"))
	nonce := strings.TrimSpace(headers.Get("X-MGP-Nonce"))
	signature := strings.TrimSpace(headers.Get("X-MGP-Signature"))
	if secret == "" || timestamp == "" || nonce == "" || signature == "" {
		return "", false
	}
	signedAt, err := parseHMACTimestamp(timestamp)
	if err != nil {
		return "", false
	}
	now = now.UTC()
	if signedAt.Before(now.Add(-hmacTimestampWindow)) || signedAt.After(now.Add(hmacTimestampWindow)) {
		return "", false
	}
	if !strings.HasPrefix(signature, "sha256=") {
		return "", false
	}
	provided, err := hex.DecodeString(strings.TrimPrefix(signature, "sha256="))
	if err != nil {
		return "", false
	}

	bodyHash := sha256Hex(body)
	signingString := fmt.Sprintf("%s\n%s\n%s\n%s\n%s", method, path, timestamp, nonce, bodyHash)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signingString))
	expected := mac.Sum(nil)
	return nonce, hmac.Equal(provided, expected)
}

func parseHMACTimestamp(value string) (time.Time, error) {
	if unixSeconds, err := strconv.ParseInt(value, 10, 64); err == nil {
		return time.Unix(unixSeconds, 0).UTC(), nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, err
	}
	return parsed.UTC(), nil
}

func sha256Hex(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func clientAllowed(allowlist []string, remoteAddr string) bool {
	if len(allowlist) == 0 {
		return true
	}

	ipText := remoteAddr
	if host, _, err := net.SplitHostPort(remoteAddr); err == nil {
		ipText = host
	}
	clientIP, err := netip.ParseAddr(ipText)
	if err != nil {
		return false
	}
	for _, rawEntry := range allowlist {
		entry, err := parseIPAllowlistEntry(rawEntry)
		if err != nil {
			return false
		}
		if entry.contains(clientIP) {
			return true
		}
	}
	return false
}

type ipAllowlistEntry struct {
	prefix  netip.Prefix
	startIP netip.Addr
	endIP   netip.Addr
}

func parseIPAllowlistEntry(raw string) (ipAllowlistEntry, error) {
	entry := strings.TrimSpace(raw)
	if entry == "" {
		return ipAllowlistEntry{}, ErrInvalidInput
	}
	if strings.Contains(entry, "-") {
		if strings.Count(entry, "-") != 1 {
			return ipAllowlistEntry{}, ErrInvalidInput
		}
		parts := strings.Split(entry, "-")
		startIP, err := netip.ParseAddr(strings.TrimSpace(parts[0]))
		if err != nil {
			return ipAllowlistEntry{}, err
		}
		endIP, err := netip.ParseAddr(strings.TrimSpace(parts[1]))
		if err != nil {
			return ipAllowlistEntry{}, err
		}
		startIP = startIP.Unmap()
		endIP = endIP.Unmap()
		if startIP.Is4() != endIP.Is4() || startIP.Compare(endIP) > 0 {
			return ipAllowlistEntry{}, ErrInvalidInput
		}
		return ipAllowlistEntry{startIP: startIP, endIP: endIP}, nil
	}
	if prefix, err := netip.ParsePrefix(entry); err == nil {
		return ipAllowlistEntry{prefix: prefix.Masked()}, nil
	}
	ip, err := netip.ParseAddr(entry)
	if err != nil {
		return ipAllowlistEntry{}, err
	}
	ip = ip.Unmap()
	return ipAllowlistEntry{startIP: ip, endIP: ip}, nil
}

func (entry ipAllowlistEntry) contains(ip netip.Addr) bool {
	ip = ip.Unmap()
	if entry.prefix.IsValid() {
		return entry.prefix.Contains(ip)
	}
	if !entry.startIP.IsValid() || !entry.endIP.IsValid() || entry.startIP.Is4() != ip.Is4() {
		return false
	}
	return entry.startIP.Compare(ip) <= 0 && entry.endIP.Compare(ip) >= 0
}

func sourceInQuietHours(configuredSource Source, at time.Time) bool {
	config, err := decodeQuietHoursConfig(configuredSource.QuietHoursConfig)
	if err != nil || !config.Enabled {
		return false
	}
	currentMinute := at.Hour()*60 + at.Minute()
	for _, window := range config.Windows {
		if quietWindowContains(window, currentMinute) {
			return true
		}
	}
	return false
}

func quietWindowContains(window quietHoursWindow, currentMinute int) bool {
	start, err := parseClockMinute(window.Start)
	if err != nil {
		return false
	}
	end, err := parseClockMinute(window.End)
	if err != nil || start == end {
		return false
	}
	if start < end {
		return currentMinute >= start && currentMinute < end
	}
	return currentMinute >= start || currentMinute < end
}

func parseClockMinute(value string) (int, error) {
	value = strings.TrimSpace(value)
	if len(value) != 5 || value[2] != ':' {
		return 0, ErrInvalidInput
	}
	hourTens := value[0] - '0'
	hourOnes := value[1] - '0'
	minuteTens := value[3] - '0'
	minuteOnes := value[4] - '0'
	if hourTens > 9 || hourOnes > 9 || minuteTens > 9 || minuteOnes > 9 {
		return 0, ErrInvalidInput
	}
	hour := int(hourTens)*10 + int(hourOnes)
	minute := int(minuteTens)*10 + int(minuteOnes)
	if hour > 23 || minute > 59 {
		return 0, ErrInvalidInput
	}
	return hour*60 + minute, nil
}

func formatClockMinute(value int) string {
	hour := value / 60
	minute := value % 60
	return fmt.Sprintf("%02d:%02d", hour, minute)
}

func inboundDedupeKey(configuredSource Source, payloadHash string) (string, error) {
	switch configuredSource.InboundDedupeStrategy {
	case "", DedupeStrategyPayloadHash:
		return payloadHash, nil
	default:
		return "", ErrInvalidDedupeConfig
	}
}

type inboundDedupeConfig struct {
	TTLSeconds int `json:"ttl_seconds"`
}

func inboundDedupeTTL(raw json.RawMessage) time.Duration {
	var config inboundDedupeConfig
	if len(raw) == 0 {
		return defaultDedupeTTL
	}
	if err := json.Unmarshal(raw, &config); err != nil {
		return defaultDedupeTTL
	}
	if config.TTLSeconds <= 0 {
		return defaultDedupeTTL
	}
	return time.Duration(config.TTLSeconds) * time.Second
}

type rateLimitConfig struct {
	Enabled   bool    `json:"enabled"`
	QPS       float64 `json:"qps"`
	PerSecond int     `json:"per_second"`
}

type rateWindow struct {
	windowStart time.Time
	count       int
}

func (s *Service) rateLimited(configuredSource Source) bool {
	var config rateLimitConfig
	if len(configuredSource.RateLimitConfig) == 0 {
		return false
	}
	if err := json.Unmarshal(configuredSource.RateLimitConfig, &config); err != nil {
		return false
	}
	if !config.Enabled {
		return false
	}

	limit := config.PerSecond
	if limit <= 0 && config.QPS > 0 {
		limit = int(config.QPS)
		if float64(limit) < config.QPS {
			limit++
		}
	}
	if limit <= 0 {
		return false
	}

	now := s.now()
	s.limiterMu.Lock()
	defer s.limiterMu.Unlock()

	state := s.limiters[configuredSource.ID]
	if state == nil || now.Sub(state.windowStart) >= time.Second {
		s.limiters[configuredSource.ID] = &rateWindow{windowStart: now, count: 1}
		return false
	}
	if state.count >= limit {
		return true
	}
	state.count++
	return false
}
