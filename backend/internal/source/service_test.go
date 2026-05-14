package source

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestIngestRejectsLegacyXMGPTokens(t *testing.T) {
	store := newMemoryStore(Source{
		ID:        "source-1",
		Code:      "orders",
		Name:      "Orders",
		Enabled:   true,
		AuthMode:  AuthModeToken,
		AuthToken: "sourceToken",
	})
	service := NewService(store, WithTraceIDGenerator(func() string { return "trace-x-token" }))

	headers := http.Header{}
	headers.Set("X-MGP-Token", "sourceToken")
	_, err := service.Ingest(context.Background(), IngestInput{
		SourceCode: "orders",
		Method:     http.MethodPost,
		Path:       "/api/v1/ingest/orders",
		Headers:    headers,
		RemoteAddr: "127.0.0.1:4321",
		Body:       []byte(`{"title":"paid"}`),
	})

	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected unauthorized for X-MGP-Token, got %v", err)
	}
	if store.latestPayloadUpdates != 0 {
		t.Fatalf("expected latest payload to remain unchanged, got %d updates", store.latestPayloadUpdates)
	}
}

func TestIngestAcceptsBearerSourceToken(t *testing.T) {
	store := newMemoryStore(Source{
		ID:        "source-1",
		Code:      "orders",
		Name:      "Orders",
		Enabled:   true,
		AuthMode:  AuthModeToken,
		AuthToken: "sourceToken",
	})
	service := NewService(store, WithTraceIDGenerator(func() string { return "trace-token" }))

	headers := http.Header{}
	headers.Set("Authorization", "Bearer sourceToken")
	result, err := service.Ingest(context.Background(), IngestInput{
		SourceCode: "orders",
		Method:     http.MethodPost,
		Path:       "/api/v1/ingest/orders",
		Headers:    headers,
		RemoteAddr: "127.0.0.1:4321",
		Body:       []byte(`{"title":"paid"}`),
	})
	if err != nil {
		t.Fatalf("ingest with bearer token: %v", err)
	}

	if result.TraceID != "trace-token" || result.Status != "accepted" {
		t.Fatalf("unexpected ingest result: %+v", result)
	}
	if store.latestPayloadUpdates != 1 {
		t.Fatalf("expected latest payload update, got %d", store.latestPayloadUpdates)
	}
	if len(store.enqueued) != 1 {
		t.Fatalf("expected one queued route_plan job, got %d", len(store.enqueued))
	}
	if store.enqueued[0].MessageID == "" || store.enqueued[0].SourceID != "source-1" || store.enqueued[0].TraceID != "trace-token" {
		t.Fatalf("unexpected queued message: %+v", store.enqueued[0])
	}
}

func TestIngestAcceptsValidHMACSignature(t *testing.T) {
	body := []byte(`{"title":"paid"}`)
	headers := signedHeaders("hmacSecret", http.MethodPost, "/api/v1/ingest/orders", body)
	store := newMemoryStore(Source{
		ID:         "source-1",
		Code:       "orders",
		Name:       "Orders",
		Enabled:    true,
		AuthMode:   AuthModeHMAC,
		HMACSecret: "hmacSecret",
	})
	service := NewService(store, WithTraceIDGenerator(func() string { return "trace-hmac" }))

	result, err := service.Ingest(context.Background(), IngestInput{
		SourceCode: "orders",
		Method:     http.MethodPost,
		Path:       "/api/v1/ingest/orders",
		Headers:    headers,
		RemoteAddr: "127.0.0.1:4321",
		Body:       body,
	})
	if err != nil {
		t.Fatalf("ingest with hmac: %v", err)
	}
	if result.TraceID != "trace-hmac" {
		t.Fatalf("unexpected trace id %q", result.TraceID)
	}
}

func TestIngestRequiresTokenAndHMACWhenConfigured(t *testing.T) {
	body := []byte(`{"title":"paid"}`)
	store := newMemoryStore(Source{
		ID:         "source-1",
		Code:       "orders",
		Name:       "Orders",
		Enabled:    true,
		AuthMode:   AuthModeTokenAndHMAC,
		AuthToken:  "sourceToken",
		HMACSecret: "hmacSecret",
	})
	service := NewService(store, WithTraceIDGenerator(func() string { return "trace-both" }))

	missingHMAC := http.Header{}
	missingHMAC.Set("Authorization", "Bearer sourceToken")
	if _, err := service.Ingest(context.Background(), IngestInput{
		SourceCode: "orders",
		Method:     http.MethodPost,
		Path:       "/api/v1/ingest/orders",
		Headers:    missingHMAC,
		RemoteAddr: "127.0.0.1:4321",
		Body:       body,
	}); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected unauthorized without hmac, got %v", err)
	}

	headers := signedHeaders("hmacSecret", http.MethodPost, "/api/v1/ingest/orders", body)
	headers.Set("Authorization", "Bearer sourceToken")
	result, err := service.Ingest(context.Background(), IngestInput{
		SourceCode: "orders",
		Method:     http.MethodPost,
		Path:       "/api/v1/ingest/orders",
		Headers:    headers,
		RemoteAddr: "127.0.0.1:4321",
		Body:       body,
	})
	if err != nil {
		t.Fatalf("ingest with token and hmac: %v", err)
	}
	if result.Status != "accepted" {
		t.Fatalf("unexpected status %q", result.Status)
	}
}

func TestIngestRejectsCIDRDeniedClientIP(t *testing.T) {
	store := newMemoryStore(Source{
		ID:          "source-1",
		Code:        "orders",
		Name:        "Orders",
		Enabled:     true,
		AuthMode:    AuthModeNone,
		IPAllowlist: []string{"10.0.0.0/8"},
	})
	service := NewService(store)

	_, err := service.Ingest(context.Background(), IngestInput{
		SourceCode: "orders",
		Method:     http.MethodPost,
		Path:       "/api/v1/ingest/orders",
		Headers:    http.Header{},
		RemoteAddr: "192.168.1.10:4321",
		Body:       []byte(`{"title":"paid"}`),
	})
	if !errors.Is(err, ErrIPNotAllowed) {
		t.Fatalf("expected CIDR rejection, got %v", err)
	}
	if store.latestPayloadUpdates != 0 {
		t.Fatalf("expected latest payload to remain unchanged, got %d updates", store.latestPayloadUpdates)
	}
}

func TestIngestAllowsCIDRSingleIPAndIPRangeAllowlist(t *testing.T) {
	store := newMemoryStore(Source{
		ID:       "source-1",
		Code:     "orders",
		Name:     "Orders",
		Enabled:  true,
		AuthMode: AuthModeNone,
		IPAllowlist: []string{
			"192.168.66.0/24",
			"172.16.30.0/24",
			"127.0.0.1",
			"172.169.10.11-172.169.10.13",
		},
	})
	service := NewService(store, WithTraceIDGenerator(func() string { return "trace-ip-allowlist" }))

	for _, remoteAddr := range []string{
		"192.168.66.20:4321",
		"172.16.30.99:4321",
		"127.0.0.1:4321",
		"172.169.10.12:4321",
	} {
		t.Run(remoteAddr, func(t *testing.T) {
			_, err := service.Ingest(context.Background(), IngestInput{
				SourceCode: "orders",
				Method:     http.MethodPost,
				Path:       "/api/v1/ingest/orders",
				Headers:    http.Header{},
				RemoteAddr: remoteAddr,
				Body:       []byte(`{"title":"paid"}`),
			})
			if err != nil {
				t.Fatalf("ingest with allowed client ip %s: %v", remoteAddr, err)
			}
		})
	}
}

func TestIngestRejectsClientIPOutsideExplicitRange(t *testing.T) {
	store := newMemoryStore(Source{
		ID:          "source-1",
		Code:        "orders",
		Name:        "Orders",
		Enabled:     true,
		AuthMode:    AuthModeNone,
		IPAllowlist: []string{"172.169.10.11-172.169.10.13"},
	})
	service := NewService(store)

	_, err := service.Ingest(context.Background(), IngestInput{
		SourceCode: "orders",
		Method:     http.MethodPost,
		Path:       "/api/v1/ingest/orders",
		Headers:    http.Header{},
		RemoteAddr: "172.169.10.14:4321",
		Body:       []byte(`{"title":"paid"}`),
	})
	if !errors.Is(err, ErrIPNotAllowed) {
		t.Fatalf("expected range rejection, got %v", err)
	}
}

func TestIngestUpdatesLatestPayloadAndQueuesWithoutRoutes(t *testing.T) {
	store := newMemoryStore(Source{
		ID:       "source-1",
		Code:     "orders",
		Name:     "Orders",
		Enabled:  true,
		AuthMode: AuthModeNone,
	})
	service := NewService(
		store,
		WithTraceIDGenerator(func() string { return "trace-latest" }),
		WithNow(func() time.Time { return time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC) }),
	)

	_, err := service.Ingest(context.Background(), IngestInput{
		SourceCode: "orders",
		Method:     http.MethodPost,
		Path:       "/api/v1/ingest/orders",
		Headers:    http.Header{},
		RemoteAddr: "127.0.0.1:4321",
		Body:       []byte(`{"title":"paid","level":"warning"}`),
	})
	if err != nil {
		t.Fatalf("ingest without routes: %v", err)
	}

	var latest map[string]string
	if err := json.Unmarshal(store.latestPayload, &latest); err != nil {
		t.Fatalf("decode latest payload: %v", err)
	}
	if latest["title"] != "paid" || latest["level"] != "warning" {
		t.Fatalf("unexpected latest payload: %+v", latest)
	}
	if len(store.enqueued) != 1 || store.enqueued[0].JobType != "route_plan" {
		t.Fatalf("expected one route_plan job, got %+v", store.enqueued)
	}
}

func TestIngestUsesPayloadHashDedupeTTLConfig(t *testing.T) {
	now := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)
	store := newMemoryStore(Source{
		ID:                   "source-1",
		Code:                 "orders",
		Name:                 "Orders",
		Enabled:              true,
		AuthMode:             AuthModeNone,
		InboundDedupeEnabled: true,
		InboundDedupeConfig:  json.RawMessage(`{"ttl_seconds":60}`),
	})
	service := NewService(
		store,
		WithTraceIDGenerator(func() string { return "trace-dedupe-ttl" }),
		WithNow(func() time.Time { return now }),
	)

	_, err := service.Ingest(context.Background(), IngestInput{
		SourceCode: "orders",
		Method:     http.MethodPost,
		Path:       "/api/v1/ingest/orders",
		Headers:    http.Header{},
		RemoteAddr: "127.0.0.1:4321",
		Body:       []byte(`{"title":"paid"}`),
	})
	if err != nil {
		t.Fatalf("ingest with dedupe ttl config: %v", err)
	}
	if len(store.enqueued) != 1 {
		t.Fatalf("expected one queued route_plan job, got %d", len(store.enqueued))
	}
	if expected := now.Add(60 * time.Second); !store.enqueued[0].DedupeExpires.Equal(expected) {
		t.Fatalf("expected dedupe expiry %s, got %s", expected, store.enqueued[0].DedupeExpires)
	}
}

func TestIngestInvalidJSONDoesNotUpdateLatestPayload(t *testing.T) {
	store := newMemoryStore(Source{
		ID:       "source-1",
		Code:     "orders",
		Name:     "Orders",
		Enabled:  true,
		AuthMode: AuthModeNone,
	})
	service := NewService(store)

	_, err := service.Ingest(context.Background(), IngestInput{
		SourceCode: "orders",
		Method:     http.MethodPost,
		Path:       "/api/v1/ingest/orders",
		Headers:    http.Header{},
		RemoteAddr: "127.0.0.1:4321",
		Body:       []byte(`{"title":`),
	})
	if !errors.Is(err, ErrInvalidJSON) {
		t.Fatalf("expected invalid JSON, got %v", err)
	}
	if store.latestPayloadUpdates != 0 {
		t.Fatalf("expected latest payload to remain unchanged, got %d updates", store.latestPayloadUpdates)
	}
}

func TestCreateSourceRejectsNonAlphanumericCredentials(t *testing.T) {
	tests := []struct {
		name  string
		input CreateSourceInput
	}{
		{
			name: "source code with hyphen",
			input: CreateSourceInput{
				Code:      "orders-api",
				Name:      "Orders",
				AuthMode:  AuthModeToken,
				AuthToken: "sourceToken",
			},
		},
		{
			name: "auth token with hyphen",
			input: CreateSourceInput{
				Code:      "ordersapi",
				Name:      "Orders",
				AuthMode:  AuthModeToken,
				AuthToken: "source-token",
			},
		},
		{
			name: "hmac secret with hyphen",
			input: CreateSourceInput{
				Code:       "ordersapi",
				Name:       "Orders",
				AuthMode:   AuthModeHMAC,
				HMACSecret: "hmac-secret",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := normalizeSourceInput(tt.input)
			if !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("expected invalid input, got %v", err)
			}
		})
	}
}

func TestCreateSourceAcceptsAlphanumericCredentials(t *testing.T) {
	created, err := normalizeSourceInput(CreateSourceInput{
		Code:       "ordersapi",
		Name:       "Orders",
		AuthMode:   AuthModeTokenAndHMAC,
		AuthToken:  "sourceToken",
		HMACSecret: "hmacSecret",
	})
	if err != nil {
		t.Fatalf("create source with alphanumeric credentials: %v", err)
	}
	if created.Code != "ordersapi" || created.AuthToken != "sourceToken" || created.HMACSecret != "hmacSecret" {
		t.Fatalf("unexpected created source: %+v", created)
	}
}

func TestNormalizeSourceInputAcceptsCIDRSingleIPAndIPRangeAllowlist(t *testing.T) {
	normalized, err := normalizeSourceInput(CreateSourceInput{
		Code:      "ordersapi",
		Name:      "Orders",
		AuthMode:  AuthModeToken,
		AuthToken: "sourceToken",
		IPAllowlist: []string{
			"192.168.66.0/24",
			"172.16.30.0/24",
			"127.0.0.1",
			"172.169.10.11-172.169.10.13",
		},
	})
	if err != nil {
		t.Fatalf("normalize source input with mixed ip allowlist: %v", err)
	}
	if len(normalized.IPAllowlist) != 4 {
		t.Fatalf("expected four allowlist entries, got %+v", normalized.IPAllowlist)
	}
}

func TestNormalizeSourceInputRejectsInvalidIPRangeAllowlist(t *testing.T) {
	_, err := normalizeSourceInput(CreateSourceInput{
		Code:        "ordersapi",
		Name:        "Orders",
		AuthMode:    AuthModeNone,
		IPAllowlist: []string{"172.169.10.13-172.169.10.11"},
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected invalid input for descending ip range, got %v", err)
	}
}

func TestNormalizeSourceInputAlwaysUsesPayloadHashDedupeStrategy(t *testing.T) {
	for _, strategy := range []DedupeStrategy{"", DedupeStrategyPayloadHash, DedupeStrategy("fields"), DedupeStrategy("expression")} {
		t.Run(string(strategy), func(t *testing.T) {
			normalized, err := normalizeSourceInput(CreateSourceInput{
				Code:                  "ordersapi",
				Name:                  "Orders",
				AuthMode:              AuthModeToken,
				AuthToken:             "sourceToken",
				InboundDedupeStrategy: strategy,
			})
			if err != nil {
				t.Fatalf("normalize source input with dedupe strategy %q: %v", strategy, err)
			}
			if normalized.InboundDedupeStrategy != DedupeStrategyPayloadHash {
				t.Fatalf("expected payload_hash dedupe strategy, got %q", normalized.InboundDedupeStrategy)
			}
		})
	}
}

func TestUpdateSourceRejectsCodeChanges(t *testing.T) {
	store := newMemoryStore(Source{
		ID:        "source-1",
		Code:      "orders",
		Name:      "Orders",
		Enabled:   true,
		AuthMode:  AuthModeToken,
		AuthToken: "sourceToken",
	})
	service := NewService(store)

	_, err := service.UpdateSource(context.Background(), "source-1", UpdateSourceInput{
		Code:      "ordersnew",
		Name:      "Orders",
		Enabled:   true,
		AuthMode:  AuthModeToken,
		AuthToken: "sourceToken",
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected invalid input when source code changes, got %v", err)
	}
	if store.updateCalls != 0 {
		t.Fatalf("expected update to be blocked before store call, got %d calls", store.updateCalls)
	}
}

func TestUpdateSourceAllowsExistingCode(t *testing.T) {
	store := newMemoryStore(Source{
		ID:        "source-1",
		Code:      "orders",
		Name:      "Orders",
		Enabled:   true,
		AuthMode:  AuthModeToken,
		AuthToken: "sourceToken",
	})
	service := NewService(store)

	updated, err := service.UpdateSource(context.Background(), "source-1", UpdateSourceInput{
		Code:      "orders",
		Name:      "Orders API",
		Enabled:   true,
		AuthMode:  AuthModeToken,
		AuthToken: "sourceToken",
	})
	if err != nil {
		t.Fatalf("update source with existing code: %v", err)
	}
	if updated.Code != "orders" || updated.Name != "Orders API" {
		t.Fatalf("unexpected updated source: %+v", updated)
	}
	if store.updateCalls != 1 {
		t.Fatalf("expected one update call, got %d", store.updateCalls)
	}
}

func signedHeaders(secret string, method string, path string, body []byte) http.Header {
	timestamp := "1778138400"
	nonce := "nonce-1"
	bodyHash := sha256.Sum256(body)
	signingString := fmt.Sprintf("%s\n%s\n%s\n%s\n%s", method, path, timestamp, nonce, hex.EncodeToString(bodyHash[:]))
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signingString))
	signature := mac.Sum(nil)

	headers := http.Header{}
	headers.Set("X-MGP-Timestamp", timestamp)
	headers.Set("X-MGP-Nonce", nonce)
	headers.Set("X-MGP-Signature", "sha256="+hex.EncodeToString(signature))
	return headers
}

type memoryStore struct {
	sources              map[string]Source
	latestPayload        json.RawMessage
	latestPayloadUpdates int
	enqueued             []EnqueueInboundParams
	updateCalls          int
}

func newMemoryStore(sources ...Source) *memoryStore {
	store := &memoryStore{sources: make(map[string]Source)}
	for _, source := range sources {
		store.sources[source.Code] = source
	}
	return store
}

func (m *memoryStore) ListSources(context.Context) ([]Source, error) {
	panic("not used")
}

func (m *memoryStore) CreateSource(context.Context, CreateSourceParams) (Source, error) {
	panic("not used")
}

func (m *memoryStore) GetSource(_ context.Context, id string) (Source, error) {
	for _, configuredSource := range m.sources {
		if configuredSource.ID == id {
			return configuredSource, nil
		}
	}
	return Source{}, ErrNotFound
}

func (m *memoryStore) GetSourceByCode(_ context.Context, code string) (Source, error) {
	source, ok := m.sources[code]
	if !ok {
		return Source{}, ErrNotFound
	}
	return source, nil
}

func (m *memoryStore) UpdateSource(_ context.Context, id string, params UpdateSourceParams) (Source, error) {
	m.updateCalls++
	existing, err := m.GetSource(context.Background(), id)
	if err != nil {
		return Source{}, err
	}
	updated := Source{
		ID:                           existing.ID,
		Code:                         params.Code,
		Name:                         params.Name,
		Enabled:                      params.Enabled,
		AuthMode:                     params.AuthMode,
		AuthToken:                    params.AuthToken,
		HMACSecret:                   params.HMACSecret,
		IPAllowlist:                  params.IPAllowlist,
		CompatMode:                   params.CompatMode,
		InboundDedupeEnabled:         params.InboundDedupeEnabled,
		InboundDedupeStrategy:        params.InboundDedupeStrategy,
		InboundDedupeConfig:          params.InboundDedupeConfig,
		RateLimitConfig:              params.RateLimitConfig,
		LatestPayloadSample:          params.LatestPayloadSample,
		LatestPayloadSampleUpdatedAt: params.LatestPayloadSampleUpdatedAt,
		CreatedAt:                    existing.CreatedAt,
		UpdatedAt:                    existing.UpdatedAt,
	}
	delete(m.sources, existing.Code)
	m.sources[updated.Code] = updated
	return updated, nil
}

func (m *memoryStore) DeleteSource(context.Context, string) error {
	panic("not used")
}

func (m *memoryStore) UpdateLatestPayloadSample(_ context.Context, sourceID string, payload json.RawMessage) error {
	m.latestPayloadUpdates++
	m.latestPayload = append(json.RawMessage(nil), payload...)
	return nil
}

func (m *memoryStore) EnqueueInbound(_ context.Context, params EnqueueInboundParams) error {
	m.enqueued = append(m.enqueued, params)
	return nil
}
