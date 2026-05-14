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
	maxQuietHoursWindows         = 5
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
	existing, err := s.store.GetSource(ctx, id)
	if err != nil {
		return Source{}, err
	}
	if existing.Code != params.Code {
		return Source{}, ErrInvalidInput
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

	payloadHash := sha256Hex(input.Body)
	traceID := s.traceID()
	messageID := uuid.NewString()
	now := s.now()
	receivedAt := now.UTC()
	headers, err := json.Marshal(input.Headers)
	if err != nil {
		return IngestResult{}, err
	}
	if sourceInQuietHours(configuredSource, now) {
		if err := s.store.EnqueueInbound(ctx, EnqueueInboundParams{
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
		}); err != nil {
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
	if configuredSource.InboundDedupeEnabled {
		key, err := inboundDedupeKey(configuredSource, payloadHash)
		if err != nil {
			return IngestResult{}, err
		}
		dedupeKey = key
		dedupeTTL = inboundDedupeTTL(configuredSource.InboundDedupeConfig)
	}

	jobPayload, err := json.Marshal(map[string]string{
		"message_id": messageID,
		"source_id":  configuredSource.ID,
		"trace_id":   traceID,
	})
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
		DedupeExpires: receivedAt.Add(dedupeTTL),
		Status:        "accepted",
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
