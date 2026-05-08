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
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	DefaultMaxPayloadBytes int64 = 1 << 20
	defaultDedupeTTL             = 24 * time.Hour
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
	DedupeStrategyFields      DedupeStrategy = "fields"
	DedupeStrategyExpression  DedupeStrategy = "expression"
)

var (
	ErrNotFound            = errors.New("source not found")
	ErrAlreadyExists       = errors.New("source already exists")
	ErrDisabled            = errors.New("source disabled")
	ErrInvalidInput        = errors.New("invalid source input")
	ErrUnauthorized        = errors.New("source unauthorized")
	ErrIPNotAllowed        = errors.New("source ip not allowed")
	ErrInvalidJSON         = errors.New("invalid json payload")
	ErrPayloadTooLarge     = errors.New("payload too large")
	ErrRateLimited         = errors.New("source rate limited")
	ErrDuplicateInbound    = errors.New("duplicate inbound payload")
	ErrInvalidDedupeConfig = errors.New("invalid inbound dedupe config")
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
	LatestPayloadSample          json.RawMessage
	LatestPayloadSampleUpdatedAt *time.Time
}

type UpdateSourceInput = CreateSourceInput

type CreateSourceParams = CreateSourceInput
type UpdateSourceParams = UpdateSourceInput

type IngestInput struct {
	SourceCode string
	Method     string
	Path       string
	Headers    http.Header
	RemoteAddr string
	Body       []byte
}

type IngestResult struct {
	TraceID string
	Status  string
	Message string
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
	JobType       string
	JobPayload    json.RawMessage
}

type Store interface {
	ListSources(ctx context.Context) ([]Source, error)
	CreateSource(ctx context.Context, params CreateSourceParams) (Source, error)
	GetSource(ctx context.Context, id string) (Source, error)
	GetSourceByCode(ctx context.Context, code string) (Source, error)
	UpdateSource(ctx context.Context, id string, params UpdateSourceParams) (Source, error)
	DeleteSource(ctx context.Context, id string) error
	UpdateLatestPayloadSample(ctx context.Context, sourceID string, payload json.RawMessage) error
	EnqueueInbound(ctx context.Context, params EnqueueInboundParams) error
}

type Service struct {
	store          Store
	now            func() time.Time
	traceID        func() string
	maxPayloadSize int64

	limiterMu sync.Mutex
	limiters  map[string]*rateWindow
}

type Option func(*Service)

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

func NewService(store Store, options ...Option) *Service {
	service := &Service{
		store:          store,
		now:            time.Now,
		traceID:        uuid.NewString,
		maxPayloadSize: DefaultMaxPayloadBytes,
		limiters:       make(map[string]*rateWindow),
	}
	for _, option := range options {
		option(service)
	}
	return service
}

func (s *Service) ListSources(ctx context.Context) ([]Source, error) {
	if s.store == nil {
		return nil, ErrNotFound
	}
	return s.store.ListSources(ctx)
}

func (s *Service) CreateSource(ctx context.Context, input CreateSourceInput) (Source, error) {
	params, err := normalizeSourceInput(input)
	if err != nil {
		return Source{}, err
	}
	return s.store.CreateSource(ctx, params)
}

func (s *Service) GetSource(ctx context.Context, id string) (Source, error) {
	if strings.TrimSpace(id) == "" {
		return Source{}, ErrInvalidInput
	}
	return s.store.GetSource(ctx, id)
}

func (s *Service) UpdateSource(ctx context.Context, id string, input UpdateSourceInput) (Source, error) {
	if strings.TrimSpace(id) == "" {
		return Source{}, ErrInvalidInput
	}
	params, err := normalizeSourceInput(input)
	if err != nil {
		return Source{}, err
	}
	return s.store.UpdateSource(ctx, id, params)
}

func (s *Service) DeleteSource(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return ErrInvalidInput
	}
	return s.store.DeleteSource(ctx, id)
}

func (s *Service) Ingest(ctx context.Context, input IngestInput) (IngestResult, error) {
	sourceCode := strings.TrimSpace(input.SourceCode)
	if sourceCode == "" {
		return IngestResult{}, ErrNotFound
	}

	configuredSource, err := s.store.GetSourceByCode(ctx, sourceCode)
	if err != nil {
		return IngestResult{}, err
	}
	if !configuredSource.Enabled {
		return IngestResult{}, ErrDisabled
	}
	if !clientAllowed(configuredSource.IPAllowlist, input.RemoteAddr) {
		return IngestResult{}, ErrIPNotAllowed
	}
	if int64(len(input.Body)) > s.maxPayloadSize {
		return IngestResult{}, ErrPayloadTooLarge
	}
	if !s.authorizeSource(configuredSource, input) {
		return IngestResult{}, ErrUnauthorized
	}

	payload, err := compactJSON(input.Body)
	if err != nil {
		return IngestResult{}, ErrInvalidJSON
	}

	if err := s.store.UpdateLatestPayloadSample(ctx, configuredSource.ID, payload); err != nil {
		return IngestResult{}, err
	}
	if s.rateLimited(configuredSource) {
		return IngestResult{}, ErrRateLimited
	}

	payloadHash := sha256Hex(input.Body)
	dedupeKey := ""
	if configuredSource.InboundDedupeEnabled {
		key, err := inboundDedupeKey(configuredSource, payloadHash)
		if err != nil {
			return IngestResult{}, err
		}
		dedupeKey = key
	}

	traceID := s.traceID()
	messageID := uuid.NewString()
	receivedAt := s.now().UTC()
	jobPayload, err := json.Marshal(map[string]string{
		"message_id": messageID,
		"source_id":  configuredSource.ID,
		"trace_id":   traceID,
	})
	if err != nil {
		return IngestResult{}, err
	}
	headers, err := json.Marshal(input.Headers)
	if err != nil {
		return IngestResult{}, err
	}

	if err := s.store.EnqueueInbound(ctx, EnqueueInboundParams{
		MessageID:     messageID,
		TraceID:       traceID,
		SourceID:      configuredSource.ID,
		Headers:       headers,
		Payload:       payload,
		PayloadHash:   payloadHash,
		ReceivedAt:    receivedAt,
		DedupeEnabled: configuredSource.InboundDedupeEnabled,
		DedupeKey:     dedupeKey,
		DedupeExpires: receivedAt.Add(defaultDedupeTTL),
		JobType:       "route_plan",
		JobPayload:    jobPayload,
	}); err != nil {
		return IngestResult{}, err
	}

	return IngestResult{
		TraceID: traceID,
		Status:  "accepted",
		Message: "accepted",
	}, nil
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
	if input.AuthMode == "" {
		input.AuthMode = AuthModeToken
	}
	if !validAuthMode(input.AuthMode) {
		return CreateSourceParams{}, ErrInvalidInput
	}
	if input.CompatMode == "" {
		input.CompatMode = "standard"
	}
	if input.InboundDedupeStrategy == "" {
		input.InboundDedupeStrategy = DedupeStrategyPayloadHash
	}
	if !validDedupeStrategy(input.InboundDedupeStrategy) {
		return CreateSourceParams{}, ErrInvalidInput
	}
	for _, cidr := range input.IPAllowlist {
		if _, err := netip.ParsePrefix(strings.TrimSpace(cidr)); err != nil {
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
	latestPayloadSample, err := normalizeOptionalJSON(input.LatestPayloadSample)
	if err != nil {
		return CreateSourceParams{}, err
	}

	input.IPAllowlist = cleanStringSlice(input.IPAllowlist)
	input.InboundDedupeConfig = dedupeConfig
	input.RateLimitConfig = rateLimitConfig
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

func validDedupeStrategy(strategy DedupeStrategy) bool {
	switch strategy {
	case DedupeStrategyPayloadHash, DedupeStrategyFields, DedupeStrategyExpression:
		return true
	default:
		return false
	}
}

func normalizeJSONConfig(raw json.RawMessage) (json.RawMessage, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return json.RawMessage(`{}`), nil
	}
	return normalizeOptionalJSON(raw)
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

func (s *Service) authorizeSource(configuredSource Source, input IngestInput) bool {
	switch configuredSource.AuthMode {
	case AuthModeNone:
		return true
	case AuthModeToken:
		return sourceBearerToken(input.Headers) == configuredSource.AuthToken && configuredSource.AuthToken != ""
	case AuthModeHMAC:
		return validHMAC(configuredSource.HMACSecret, input.Method, input.Path, input.Headers, input.Body)
	case AuthModeTokenAndHMAC:
		return sourceBearerToken(input.Headers) == configuredSource.AuthToken &&
			configuredSource.AuthToken != "" &&
			validHMAC(configuredSource.HMACSecret, input.Method, input.Path, input.Headers, input.Body)
	default:
		return false
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

func validHMAC(secret string, method string, path string, headers http.Header, body []byte) bool {
	secret = strings.TrimSpace(secret)
	timestamp := strings.TrimSpace(headers.Get("X-MGP-Timestamp"))
	nonce := strings.TrimSpace(headers.Get("X-MGP-Nonce"))
	signature := strings.TrimSpace(headers.Get("X-MGP-Signature"))
	if secret == "" || timestamp == "" || nonce == "" || signature == "" {
		return false
	}
	if !strings.HasPrefix(signature, "sha256=") {
		return false
	}
	provided, err := hex.DecodeString(strings.TrimPrefix(signature, "sha256="))
	if err != nil {
		return false
	}

	bodyHash := sha256Hex(body)
	signingString := fmt.Sprintf("%s\n%s\n%s\n%s\n%s", method, path, timestamp, nonce, bodyHash)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signingString))
	expected := mac.Sum(nil)
	return hmac.Equal(provided, expected)
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
	for _, cidr := range allowlist {
		prefix, err := netip.ParsePrefix(strings.TrimSpace(cidr))
		if err != nil {
			return false
		}
		if prefix.Contains(clientIP) {
			return true
		}
	}
	return false
}

func inboundDedupeKey(configuredSource Source, payloadHash string) (string, error) {
	switch configuredSource.InboundDedupeStrategy {
	case "", DedupeStrategyPayloadHash:
		return payloadHash, nil
	case DedupeStrategyFields, DedupeStrategyExpression:
		return "", ErrInvalidDedupeConfig
	default:
		return "", ErrInvalidDedupeConfig
	}
}

type rateLimitConfig struct {
	Enabled   bool    `json:"enabled"`
	QPS       float64 `json:"qps"`
	PerMinute int     `json:"per_minute"`
	Burst     int     `json:"burst"`
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

	limit := config.PerMinute
	window := time.Minute
	if limit <= 0 && config.QPS > 0 {
		limit = int(config.QPS)
		window = time.Second
	}
	if config.Burst > 0 && config.Burst > limit {
		limit = config.Burst
	}
	if limit <= 0 {
		return false
	}

	now := s.now()
	s.limiterMu.Lock()
	defer s.limiterMu.Unlock()

	state := s.limiters[configuredSource.ID]
	if state == nil || now.Sub(state.windowStart) >= window {
		s.limiters[configuredSource.ID] = &rateWindow{windowStart: now, count: 1}
		return false
	}
	if state.count >= limit {
		return true
	}
	state.count++
	return false
}
