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
	"strings"
	"sync"
	"testing"
	"time"

	"mvp-push-gateway/backend/internal/queue"
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
	service := NewService(
		store,
		WithTraceIDGenerator(func() string { return "trace-token" }),
		WithLatestPayloadFlushInterval(10*time.Millisecond),
	)

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
	waitForLatestPayloadUpdates(t, store, 1)
	if len(store.enqueued) != 1 {
		t.Fatalf("expected one queued route_plan job, got %d", len(store.enqueued))
	}
	if store.enqueued[0].MessageID == "" || store.enqueued[0].SourceID != "source-1" || store.enqueued[0].TraceID != "trace-token" {
		t.Fatalf("unexpected queued message: %+v", store.enqueued[0])
	}
}

func TestIngestRedactsSensitiveHeadersBeforeLogging(t *testing.T) {
	store := newMemoryStore(Source{
		ID:        "source-1",
		Code:      "orders",
		Name:      "Orders",
		Enabled:   true,
		AuthMode:  AuthModeToken,
		AuthToken: "sourceToken",
	})
	service := NewService(
		store,
		WithTraceIDGenerator(func() string { return "trace-redacted-headers" }),
		WithLatestPayloadFlushInterval(10*time.Millisecond),
	)

	headers := http.Header{}
	headers.Set("Authorization", "Bearer sourceToken")
	headers.Set("X-MGP-Signature", "sha256=signature")
	headers.Set("X-API-Key", "api-key-1")
	headers.Set("X-Business-ID", "order-1001")
	_, err := service.Ingest(context.Background(), IngestInput{
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
	waitForLatestPayloadUpdates(t, store, 1)
	if len(store.enqueued) != 1 {
		t.Fatalf("expected one queued route_plan job, got %d", len(store.enqueued))
	}

	var storedHeaders map[string][]string
	if err := json.Unmarshal(store.enqueued[0].Headers, &storedHeaders); err != nil {
		t.Fatalf("decode stored headers: %v", err)
	}
	if got := strings.Join(storedHeaders["Authorization"], ","); strings.Contains(got, "sourceToken") {
		t.Fatalf("stored headers leaked source token: %s", store.enqueued[0].Headers)
	}
	if got := strings.Join(storedHeaders["X-Mgp-Signature"], ","); strings.Contains(got, "signature") {
		t.Fatalf("stored headers leaked hmac signature: %s", store.enqueued[0].Headers)
	}
	if got := strings.Join(storedHeaders["X-Api-Key"], ","); strings.Contains(got, "api-key-1") {
		t.Fatalf("stored headers leaked api key: %s", store.enqueued[0].Headers)
	}
	if got := strings.Join(storedHeaders["X-Business-Id"], ","); got != "order-1001" {
		t.Fatalf("expected non-sensitive business header to be preserved, got %q in %s", got, store.enqueued[0].Headers)
	}
}

func TestIngestMinimizesLatestPayloadSample(t *testing.T) {
	store := newMemoryStore(Source{
		ID:        "source-1",
		Code:      "orders",
		Name:      "Orders",
		Enabled:   true,
		AuthMode:  AuthModeToken,
		AuthToken: "sourceToken",
	})
	service := NewService(
		store,
		WithTraceIDGenerator(func() string { return "trace-minimize-latest" }),
		WithLatestPayloadFlushInterval(10*time.Millisecond),
	)

	headers := http.Header{}
	headers.Set("Authorization", "Bearer sourceToken")
	_, err := service.Ingest(context.Background(), IngestInput{
		SourceCode: "orders",
		Method:     http.MethodPost,
		Path:       "/api/v1/ingest/orders",
		Headers:    headers,
		RemoteAddr: "127.0.0.1:4321",
		Body:       []byte(`{"title":"paid","access_token":"payload-token","nested":{"email":"person@example.com","content":"ok"}}`),
	})
	if err != nil {
		t.Fatalf("ingest with sensitive payload: %v", err)
	}
	waitForLatestPayloadUpdates(t, store, 1)

	latest := store.latestPayloadString()
	if strings.Contains(latest, "payload-token") || strings.Contains(latest, "person@example.com") {
		t.Fatalf("latest payload sample leaked sensitive values: %s", latest)
	}
	if !strings.Contains(latest, `"title":"paid"`) || !strings.Contains(latest, `"content":"ok"`) {
		t.Fatalf("latest payload sample should preserve non-sensitive context, got %s", latest)
	}
}

func TestIngestPublishesRoutePlanEventWhenPublisherConfigured(t *testing.T) {
	store := newMemoryStore(Source{
		ID:                   "source-1",
		Code:                 "orders",
		Name:                 "Orders",
		Enabled:              true,
		AuthMode:             AuthModeToken,
		AuthToken:            "sourceToken",
		InboundDedupeEnabled: true,
		InboundDedupeConfig:  json.RawMessage(`{"ttl_seconds":60}`),
	})
	publisher := &recordingRoutePlanPublisher{}
	service := NewService(
		store,
		WithTraceIDGenerator(func() string { return "trace-route-publish" }),
		WithRoutePlanPublisher(publisher),
	)

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
		t.Fatalf("ingest with route plan publisher: %v", err)
	}
	if result.Status != "accepted" || result.TraceID != "trace-route-publish" {
		t.Fatalf("unexpected ingest result: %+v", result)
	}
	if len(store.enqueued) != 1 {
		t.Fatalf("expected one persisted inbound record when inbound dedupe is enabled, got %d", len(store.enqueued))
	}
	persisted := store.enqueued[0]
	if !persisted.SkipRoutePlan || persisted.JobType != "" || len(persisted.JobPayload) != 0 {
		t.Fatalf("expected PostgreSQL route_plan job to be skipped when publisher is configured, got %+v", persisted)
	}
	if len(publisher.events) != 1 {
		t.Fatalf("expected one route plan event, got %d", len(publisher.events))
	}
	event := publisher.events[0]
	if event.MessageID != persisted.MessageID || event.SourceID != "source-1" || event.TraceID != "trace-route-publish" {
		t.Fatalf("unexpected route plan event: %+v persisted=%+v", event, persisted)
	}
	if event.MessageIDForDedup() != "trace-route-publish" {
		t.Fatalf("expected trace id to be used as route-plan dedupe id, got %q", event.MessageIDForDedup())
	}
	if len(event.Payload) != 0 {
		t.Fatalf("expected persisted route-plan event to omit payload so planning reloads the message record, got %s", event.Payload)
	}
}

func TestIngestWithRoutePlanPublisherSendsPayloadEventWithoutInboundDBWrite(t *testing.T) {
	store := newMemoryStore(Source{
		ID:        "source-1",
		Code:      "orders",
		Name:      "Orders",
		Enabled:   true,
		AuthMode:  AuthModeToken,
		AuthToken: "sourceToken",
	})
	publisher := &recordingRoutePlanPublisher{}
	service := NewService(
		store,
		WithTraceIDGenerator(func() string { return "trace-direct-route" }),
		WithRoutePlanPublisher(publisher),
	)
	body := []byte(`{"title":"paid"}`)
	headers := http.Header{"Authorization": []string{"Bearer sourceToken"}}

	result, err := service.Ingest(context.Background(), IngestInput{
		SourceCode: "orders",
		Method:     http.MethodPost,
		Path:       "/api/v1/ingest/orders",
		Headers:    headers,
		RemoteAddr: "127.0.0.1:4321",
		Body:       body,
	})
	if err != nil {
		t.Fatalf("ingest with route plan publisher: %v", err)
	}
	if result.TraceID != "trace-direct-route" {
		t.Fatalf("unexpected trace id: %+v", result)
	}
	if len(store.enqueued) != 0 {
		t.Fatalf("expected direct JetStream path to skip inbound DB write, got %d enqueued records", len(store.enqueued))
	}
	if len(publisher.events) != 1 {
		t.Fatalf("expected one route plan event, got %d", len(publisher.events))
	}
	event := publisher.events[0]
	if event.MessageID == "" || event.SourceID != "source-1" || event.TraceID != "trace-direct-route" {
		t.Fatalf("unexpected route plan event identifiers: %+v", event)
	}
	if string(event.Payload) != `{"title":"paid"}` {
		t.Fatalf("expected route plan event to carry inbound payload, got %s", event.Payload)
	}
	if len(event.Headers) == 0 || event.ReceivedAt.IsZero() {
		t.Fatalf("expected route plan event to carry headers and receive time, got %+v", event)
	}
}

func TestIngestWithPersistentLogUsesRecordOnlyRoutePlanEvent(t *testing.T) {
	store := newMemoryStore(Source{
		ID:        "source-1",
		Code:      "orders",
		Name:      "Orders",
		Enabled:   true,
		AuthMode:  AuthModeToken,
		AuthToken: "sourceToken",
	})
	publisher := &recordingRoutePlanPublisher{}
	service := NewService(
		store,
		WithTraceIDGenerator(func() string { return "trace-console-test" }),
		WithRoutePlanPublisher(publisher),
	)

	result, err := service.Ingest(context.Background(), IngestInput{
		SourceCode:        "orders",
		Method:            http.MethodPost,
		Path:              "/api/v1/ingest/orders",
		Headers:           http.Header{"Authorization": []string{"Bearer sourceToken"}},
		RemoteAddr:        "127.0.0.1:4321",
		Body:              []byte(`{"title":"paid"}`),
		PersistBeforePlan: true,
	})
	if err != nil {
		t.Fatalf("ingest with persistent route-plan log: %v", err)
	}
	if result.TraceID != "trace-console-test" {
		t.Fatalf("unexpected trace id: %+v", result)
	}
	if len(store.enqueued) != 1 {
		t.Fatalf("expected one persisted inbound record, got %d", len(store.enqueued))
	}
	if !store.enqueued[0].SkipRoutePlan || store.enqueued[0].JobPayload != nil {
		t.Fatalf("expected record-only inbound enqueue, got %+v", store.enqueued[0])
	}
	if len(publisher.events) != 1 {
		t.Fatalf("expected one route plan event, got %d", len(publisher.events))
	}
	event := publisher.events[0]
	if event.TraceID != "trace-console-test" || len(event.Payload) != 0 {
		t.Fatalf("expected persisted route-plan event without payload, got %+v", event)
	}
}

func TestIngestWithRuntimeDedupeStoreSendsPayloadEventWithoutInboundDBWrite(t *testing.T) {
	store := newMemoryStore(Source{
		ID:                   "source-1",
		Code:                 "orders",
		Name:                 "Orders",
		Enabled:              true,
		AuthMode:             AuthModeToken,
		AuthToken:            "sourceToken",
		InboundDedupeEnabled: true,
		InboundDedupeConfig:  json.RawMessage(`{"ttl_seconds":60}`),
	})
	runtimeStore := &recordingRuntimeStateStore{dedupeReserved: true}
	publisher := &recordingRoutePlanPublisher{}
	service := NewService(
		store,
		WithTraceIDGenerator(func() string { return "trace-runtime-dedupe" }),
		WithRoutePlanPublisher(publisher),
		WithInboundDedupeStore(runtimeStore),
	)

	result, err := service.Ingest(context.Background(), IngestInput{
		SourceCode: "orders",
		Method:     http.MethodPost,
		Path:       "/api/v1/ingest/orders",
		Headers:    http.Header{"Authorization": []string{"Bearer sourceToken"}},
		RemoteAddr: "127.0.0.1:4321",
		Body:       []byte(`{"title":"paid"}`),
	})
	if err != nil {
		t.Fatalf("ingest with runtime dedupe store: %v", err)
	}
	if result.TraceID != "trace-runtime-dedupe" || len(store.enqueued) != 0 {
		t.Fatalf("expected fast ingest without inbound DB write, result=%+v enqueued=%d", result, len(store.enqueued))
	}
	if runtimeStore.reserveCalls != 1 || runtimeStore.lastDedupeKey == "" || runtimeStore.lastExpiresAt.IsZero() {
		t.Fatalf("expected one runtime dedupe reservation, got %+v", runtimeStore)
	}
	if len(publisher.events) != 1 || string(publisher.events[0].Payload) != `{"title":"paid"}` {
		t.Fatalf("expected payload route event, got %+v", publisher.events)
	}
}

func TestIngestWithRuntimeDedupeStoreRecordsDuplicateWithoutPublishing(t *testing.T) {
	store := newMemoryStore(Source{
		ID:                   "source-1",
		Code:                 "orders",
		Name:                 "Orders",
		Enabled:              true,
		AuthMode:             AuthModeToken,
		AuthToken:            "sourceToken",
		InboundDedupeEnabled: true,
		InboundDedupeConfig:  json.RawMessage(`{"ttl_seconds":60}`),
	})
	runtimeStore := &recordingRuntimeStateStore{dedupeReserved: false}
	publisher := &recordingRoutePlanPublisher{}
	service := NewService(
		store,
		WithTraceIDGenerator(func() string { return "trace-runtime-duplicate" }),
		WithRoutePlanPublisher(publisher),
		WithInboundDedupeStore(runtimeStore),
	)

	_, err := service.Ingest(context.Background(), IngestInput{
		SourceCode: "orders",
		Method:     http.MethodPost,
		Path:       "/api/v1/ingest/orders",
		Headers:    http.Header{"Authorization": []string{"Bearer sourceToken"}},
		RemoteAddr: "127.0.0.1:4321",
		Body:       []byte(`{"title":"paid"}`),
	})
	if !errors.Is(err, ErrDuplicateInbound) {
		t.Fatalf("expected duplicate inbound error, got %v", err)
	}
	if len(publisher.events) != 0 {
		t.Fatalf("expected duplicate not to publish route plan event, got %+v", publisher.events)
	}
	if len(store.enqueued) != 1 {
		t.Fatalf("expected duplicate marker record, got %d", len(store.enqueued))
	}
	record := store.enqueued[0]
	if record.Status != "deduped" || record.ErrorCode != "MGP-DEDUPE-001" || !record.SkipRoutePlan || record.DedupeEnabled {
		t.Fatalf("unexpected duplicate marker: %+v", record)
	}
}

func TestListSourcesOverlaysLatestPayloadFromRuntimeStore(t *testing.T) {
	store := newMemoryStore(Source{
		ID:        "source-1",
		Code:      "orders",
		Name:      "Orders",
		Enabled:   true,
		AuthMode:  AuthModeNone,
		CreatedAt: time.Date(2026, 6, 8, 10, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 6, 8, 10, 0, 0, 0, time.UTC),
	})
	sampledAt := time.Date(2026, 6, 8, 10, 1, 0, 0, time.UTC)
	runtimeStore := &recordingRuntimeStateStore{
		latestPayloads: map[string]latestPayloadSample{
			"source-1": {payload: json.RawMessage(`{"title":"runtime"}`), sampledAt: sampledAt},
		},
	}
	service := NewService(store, WithLatestPayloadStore(runtimeStore))

	sources, err := service.ListSources(context.Background())
	if err != nil {
		t.Fatalf("list sources: %v", err)
	}
	if len(sources) != 1 {
		t.Fatalf("expected one source, got %d", len(sources))
	}
	if string(sources[0].LatestPayloadSample) != `{"title":"runtime"}` ||
		sources[0].LatestPayloadSampleUpdatedAt == nil ||
		!sources[0].LatestPayloadSampleUpdatedAt.Equal(sampledAt) {
		t.Fatalf("expected runtime latest payload overlay, got %+v", sources[0])
	}
}

func TestIngestSilencesMessageDuringQuietHours(t *testing.T) {
	quietNow := time.Date(2026, 5, 14, 23, 15, 0, 0, time.FixedZone("CST", 8*60*60))
	store := newMemoryStore(Source{
		ID:                   "source-1",
		Code:                 "orders",
		Name:                 "Orders",
		Enabled:              true,
		AuthMode:             AuthModeNone,
		QuietHoursConfig:     json.RawMessage(`{"enabled":true,"windows":[{"start":"22:00","end":"08:00"}]}`),
		RateLimitConfig:      json.RawMessage(`{"enabled":true,"per_second":1}`),
		InboundDedupeEnabled: true,
	})
	service := NewService(
		store,
		WithNow(func() time.Time { return quietNow }),
		WithTraceIDGenerator(func() string { return "trace-silenced" }),
		WithLatestPayloadFlushInterval(10*time.Millisecond),
	)

	result, err := service.Ingest(context.Background(), IngestInput{
		SourceCode: "orders",
		Method:     http.MethodPost,
		Path:       "/api/v1/ingest/orders",
		Headers:    http.Header{},
		RemoteAddr: "127.0.0.1:4321",
		Body:       []byte(`{"title":"quiet"}`),
	})
	if err != nil {
		t.Fatalf("ingest during quiet hours: %v", err)
	}
	if result.TraceID != "trace-silenced" || result.Status != "silenced" || result.Message != "silenced" {
		t.Fatalf("unexpected silenced ingest result: %+v", result)
	}
	waitForLatestPayloadUpdates(t, store, 1)
	if len(store.enqueued) != 1 {
		t.Fatalf("expected one stored inbound record, got %d", len(store.enqueued))
	}
	stored := store.enqueued[0]
	if stored.Status != "silenced" || !stored.SkipRoutePlan || stored.JobType != "" {
		t.Fatalf("expected silenced record without route job, got %+v", stored)
	}
	if stored.ErrorCode != "MGP-DND-001" || stored.ErrorMessage != "消息免打扰时间段内静默" {
		t.Fatalf("unexpected silenced message fields: code=%q message=%q", stored.ErrorCode, stored.ErrorMessage)
	}
}

func TestIngestQueuesMessageOutsideQuietHours(t *testing.T) {
	activeNow := time.Date(2026, 5, 14, 9, 15, 0, 0, time.FixedZone("CST", 8*60*60))
	store := newMemoryStore(Source{
		ID:               "source-1",
		Code:             "orders",
		Name:             "Orders",
		Enabled:          true,
		AuthMode:         AuthModeNone,
		QuietHoursConfig: json.RawMessage(`{"enabled":true,"windows":[{"start":"22:00","end":"08:00"}]}`),
	})
	service := NewService(
		store,
		WithNow(func() time.Time { return activeNow }),
		WithTraceIDGenerator(func() string { return "trace-active" }),
	)

	result, err := service.Ingest(context.Background(), IngestInput{
		SourceCode: "orders",
		Method:     http.MethodPost,
		Path:       "/api/v1/ingest/orders",
		Headers:    http.Header{},
		RemoteAddr: "127.0.0.1:4321",
		Body:       []byte(`{"title":"active"}`),
	})
	if err != nil {
		t.Fatalf("ingest outside quiet hours: %v", err)
	}
	if result.Status != "accepted" || len(store.enqueued) != 1 {
		t.Fatalf("expected accepted queued message, result=%+v enqueued=%+v", result, store.enqueued)
	}
	if store.enqueued[0].Status != "accepted" || store.enqueued[0].SkipRoutePlan || store.enqueued[0].JobType != "route_plan" {
		t.Fatalf("expected normal route_plan enqueue, got %+v", store.enqueued[0])
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
	service := NewService(
		store,
		WithTraceIDGenerator(func() string { return "trace-hmac" }),
		WithNow(func() time.Time { return time.Unix(1778138400, 0).UTC() }),
	)

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

func TestIngestRejectsExpiredHMACTimestamp(t *testing.T) {
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
	service := NewService(store, WithNow(func() time.Time {
		return time.Unix(1778138400, 0).UTC().Add(6 * time.Minute)
	}))

	_, err := service.Ingest(context.Background(), IngestInput{
		SourceCode: "orders",
		Method:     http.MethodPost,
		Path:       "/api/v1/ingest/orders",
		Headers:    headers,
		RemoteAddr: "127.0.0.1:4321",
		Body:       body,
	})

	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected expired hmac timestamp to be unauthorized, got %v", err)
	}
	if store.latestPayloadUpdates != 0 {
		t.Fatalf("expected rejected signature not to update latest payload, got %d updates", store.latestPayloadUpdates)
	}
}

func TestIngestRejectsReplayedHMACNonce(t *testing.T) {
	body := []byte(`{"title":"paid"}`)
	headers := signedHeaders("hmacSecret", http.MethodPost, "/api/v1/ingest/orders", body)
	now := time.Unix(1778138400, 0).UTC()
	store := newMemoryStore(Source{
		ID:         "source-1",
		Code:       "orders",
		Name:       "Orders",
		Enabled:    true,
		AuthMode:   AuthModeHMAC,
		HMACSecret: "hmacSecret",
	})
	service := NewService(
		store,
		WithNow(func() time.Time { return now }),
		WithLatestPayloadFlushInterval(10*time.Millisecond),
	)
	input := IngestInput{
		SourceCode: "orders",
		Method:     http.MethodPost,
		Path:       "/api/v1/ingest/orders",
		Headers:    headers,
		RemoteAddr: "127.0.0.1:4321",
		Body:       body,
	}

	if _, err := service.Ingest(context.Background(), input); err != nil {
		t.Fatalf("first signed ingest should pass: %v", err)
	}
	if _, err := service.Ingest(context.Background(), input); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected replayed hmac nonce to be unauthorized, got %v", err)
	}
	waitForLatestPayloadUpdates(t, store, 1)
}

func TestIngestRejectsReplayedHMACNonceAcrossServiceInstances(t *testing.T) {
	body := []byte(`{"title":"paid"}`)
	headers := signedHeaders("hmacSecret", http.MethodPost, "/api/v1/ingest/orders", body)
	now := time.Unix(1778138400, 0).UTC()
	store := newMemoryStore(Source{
		ID:         "source-1",
		Code:       "orders",
		Name:       "Orders",
		Enabled:    true,
		AuthMode:   AuthModeHMAC,
		HMACSecret: "hmacSecret",
	})
	firstService := NewService(
		store,
		WithNow(func() time.Time { return now }),
		WithLatestPayloadFlushInterval(10*time.Millisecond),
	)
	secondService := NewService(
		store,
		WithNow(func() time.Time { return now }),
		WithLatestPayloadFlushInterval(10*time.Millisecond),
	)
	input := IngestInput{
		SourceCode: "orders",
		Method:     http.MethodPost,
		Path:       "/api/v1/ingest/orders",
		Headers:    headers,
		RemoteAddr: "127.0.0.1:4321",
		Body:       body,
	}

	if _, err := firstService.Ingest(context.Background(), input); err != nil {
		t.Fatalf("first service signed ingest should pass: %v", err)
	}
	if _, err := secondService.Ingest(context.Background(), input); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected second service replayed hmac nonce to be unauthorized, got %v", err)
	}
	waitForLatestPayloadUpdates(t, store, 1)
}

func TestIngestUsesRuntimeHMACNonceStoreWhenConfigured(t *testing.T) {
	now := time.Unix(1778138400, 0).UTC()
	store := newMemoryStore(Source{
		ID:         "source-1",
		Code:       "orders",
		Name:       "Orders",
		Enabled:    true,
		AuthMode:   AuthModeHMAC,
		HMACSecret: "hmacSecret",
	})
	runtimeStore := &recordingRuntimeStateStore{hmacReserved: true}
	service := NewService(
		store,
		WithNow(func() time.Time { return now }),
		WithHMACNonceStore(runtimeStore),
		WithLatestPayloadFlushInterval(10*time.Millisecond),
	)
	body := []byte(`{"title":"paid"}`)

	_, err := service.Ingest(context.Background(), IngestInput{
		SourceCode: "orders",
		Method:     http.MethodPost,
		Path:       "/api/v1/ingest/orders",
		Headers:    signedHeaders("hmacSecret", http.MethodPost, "/api/v1/ingest/orders", body),
		RemoteAddr: "127.0.0.1:4321",
		Body:       body,
	})
	if err != nil {
		t.Fatalf("ingest with runtime hmac nonce store: %v", err)
	}
	if runtimeStore.hmacCalls != 1 || runtimeStore.lastHMACNonce != "nonce-1" || runtimeStore.lastHMACExpiresAt.IsZero() {
		t.Fatalf("expected runtime hmac nonce reservation, got %+v", runtimeStore)
	}
	if len(store.hmacNonces) != 0 {
		t.Fatalf("expected database hmac nonce store to be bypassed, got %+v", store.hmacNonces)
	}
	waitForLatestPayloadUpdates(t, store, 1)
}

func TestIngestRejectsRuntimeHMACNonceDuplicate(t *testing.T) {
	now := time.Unix(1778138400, 0).UTC()
	store := newMemoryStore(Source{
		ID:         "source-1",
		Code:       "orders",
		Name:       "Orders",
		Enabled:    true,
		AuthMode:   AuthModeHMAC,
		HMACSecret: "hmacSecret",
	})
	runtimeStore := &recordingRuntimeStateStore{hmacReserved: false}
	service := NewService(
		store,
		WithNow(func() time.Time { return now }),
		WithHMACNonceStore(runtimeStore),
	)
	body := []byte(`{"title":"paid"}`)

	_, err := service.Ingest(context.Background(), IngestInput{
		SourceCode: "orders",
		Method:     http.MethodPost,
		Path:       "/api/v1/ingest/orders",
		Headers:    signedHeaders("hmacSecret", http.MethodPost, "/api/v1/ingest/orders", body),
		RemoteAddr: "127.0.0.1:4321",
		Body:       body,
	})
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected duplicate runtime hmac nonce to be unauthorized, got %v", err)
	}
	if runtimeStore.hmacCalls != 1 {
		t.Fatalf("expected one runtime hmac nonce reservation, got %+v", runtimeStore)
	}
	if store.latestPayloadUpdates != 0 {
		t.Fatalf("expected duplicate hmac nonce to stop before latest payload update, got %d", store.latestPayloadUpdates)
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
	service := NewService(
		store,
		WithTraceIDGenerator(func() string { return "trace-both" }),
		WithNow(func() time.Time { return time.Unix(1778138400, 0).UTC() }),
	)

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
		WithLatestPayloadFlushInterval(10*time.Millisecond),
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

	waitForLatestPayloadUpdates(t, store, 1)
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

func TestIngestCoalescesLatestPayloadSampleUpdatesPerSource(t *testing.T) {
	now := time.Date(2026, 6, 5, 10, 0, 0, 0, time.UTC)
	store := newMemoryStore(Source{ID: "source-1", Code: "orders", Name: "Orders", Enabled: true, AuthMode: AuthModeNone})
	service := NewService(
		store,
		WithNow(func() time.Time { return now }),
		WithTraceIDGenerator(func() string { return "trace-coalesce" }),
		WithLatestPayloadFlushInterval(10*time.Millisecond),
	)
	input := IngestInput{
		SourceCode: "orders",
		Method:     http.MethodPost,
		Path:       "/api/v1/ingest/orders",
		Body:       []byte(`{"title":"first"}`),
	}

	if _, err := service.Ingest(context.Background(), input); err != nil {
		t.Fatalf("first ingest: %v", err)
	}
	input.Body = []byte(`{"title":"second"}`)
	if _, err := service.Ingest(context.Background(), input); err != nil {
		t.Fatalf("second ingest inside throttle window: %v", err)
	}
	waitForLatestPayloadUpdates(t, store, 1)
	if latest := store.latestPayloadString(); latest != `{"title":"second"}` {
		t.Fatalf("expected latest payload flush to keep last payload in window, got %s", latest)
	}

	now = now.Add(time.Second)
	input.Body = []byte(`{"title":"third"}`)
	if _, err := service.Ingest(context.Background(), input); err != nil {
		t.Fatalf("third ingest after throttle window: %v", err)
	}
	waitForLatestPayloadUpdates(t, store, 2)
	if latest := store.latestPayloadString(); latest != `{"title":"third"}` {
		t.Fatalf("expected latest payload to refresh after throttle window, got %s", store.latestPayload)
	}
}

func TestIngestRateLimitUsesPerSecondWindow(t *testing.T) {
	now := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)
	currentTime := now
	store := newMemoryStore(Source{
		ID:              "source-1",
		Code:            "orders",
		Name:            "Orders",
		Enabled:         true,
		AuthMode:        AuthModeNone,
		RateLimitConfig: json.RawMessage(`{"enabled":true,"per_second":1}`),
	})
	service := NewService(
		store,
		WithNow(func() time.Time { return currentTime }),
		WithTraceIDGenerator(func() string { return "trace-rate-limit" }),
	)

	input := IngestInput{
		SourceCode: "orders",
		Method:     http.MethodPost,
		Path:       "/api/v1/ingest/orders",
		Headers:    http.Header{},
		RemoteAddr: "127.0.0.1:4321",
		Body:       []byte(`{"title":"paid"}`),
	}
	if _, err := service.Ingest(context.Background(), input); err != nil {
		t.Fatalf("first ingest should pass: %v", err)
	}
	if _, err := service.Ingest(context.Background(), input); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("second ingest should be rate limited, got %v", err)
	}

	currentTime = currentTime.Add(time.Second)
	if _, err := service.Ingest(context.Background(), input); err != nil {
		t.Fatalf("ingest in next second should pass: %v", err)
	}
}

func TestIngestRateLimitRequiresPerSecondLimit(t *testing.T) {
	store := newMemoryStore(Source{
		ID:              "source-1",
		Code:            "orders",
		Name:            "Orders",
		Enabled:         true,
		AuthMode:        AuthModeNone,
		RateLimitConfig: json.RawMessage(`{"enabled":true}`),
	})
	service := NewService(
		store,
		WithNow(func() time.Time { return time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC) }),
		WithTraceIDGenerator(func() string { return "trace-no-rate-limit-value" }),
	)
	input := IngestInput{
		SourceCode: "orders",
		Method:     http.MethodPost,
		Path:       "/api/v1/ingest/orders",
		Headers:    http.Header{},
		RemoteAddr: "127.0.0.1:4321",
		Body:       []byte(`{"title":"paid"}`),
	}

	if _, err := service.Ingest(context.Background(), input); err != nil {
		t.Fatalf("first ingest should pass: %v", err)
	}
	if _, err := service.Ingest(context.Background(), input); err != nil {
		t.Fatalf("enabled rate limit without per-second value should not reject: %v", err)
	}
}

func TestIngestAllowsFiveMegabytePayloadByDefault(t *testing.T) {
	store := newMemoryStore(Source{
		ID:       "source-1",
		Code:     "orders",
		Name:     "Orders",
		Enabled:  true,
		AuthMode: AuthModeNone,
	})
	service := NewService(store, WithTraceIDGenerator(func() string { return "trace-large-payload" }))
	body := []byte(`{"blob":"` + strings.Repeat("a", (2<<20)) + `"}`)

	if _, err := service.Ingest(context.Background(), IngestInput{
		SourceCode: "orders",
		Method:     http.MethodPost,
		Path:       "/api/v1/ingest/orders",
		Headers:    http.Header{},
		RemoteAddr: "127.0.0.1:4321",
		Body:       body,
	}); err != nil {
		t.Fatalf("2MiB payload should pass with the new default limit: %v", err)
	}
}

func TestIngestUsesConfiguredMaxPayloadSize(t *testing.T) {
	store := newMemoryStore(Source{
		ID:       "source-1",
		Code:     "orders",
		Name:     "Orders",
		Enabled:  true,
		AuthMode: AuthModeNone,
	})
	service := NewService(
		store,
		WithMaxPayloadSizeFunc(func(context.Context) int64 { return 32 }),
		WithTraceIDGenerator(func() string { return "trace-configured-payload-limit" }),
	)

	_, err := service.Ingest(context.Background(), IngestInput{
		SourceCode: "orders",
		Method:     http.MethodPost,
		Path:       "/api/v1/ingest/orders",
		Headers:    http.Header{},
		RemoteAddr: "127.0.0.1:4321",
		Body:       []byte(`{"title":"payload larger than configured limit"}`),
	})
	if !errors.Is(err, ErrPayloadTooLarge) {
		t.Fatalf("expected configured payload limit to reject oversized body, got %v", err)
	}
	if store.latestPayloadUpdates != 0 {
		t.Fatalf("expected payload rejected before latest payload update, got %d", store.latestPayloadUpdates)
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

func TestNormalizeSourceInputAcceptsQuietHoursWindows(t *testing.T) {
	normalized, err := normalizeSourceInput(CreateSourceInput{
		Code:                 "orders",
		Name:                 "Orders",
		Enabled:              true,
		AuthMode:             AuthModeNone,
		CompatMode:           "standard",
		QuietHoursConfig:     json.RawMessage(`{"enabled":true,"windows":[{"start":"22:00","end":"08:00"},{"start":"12:30","end":"13:15"}]}`),
		RateLimitConfig:      json.RawMessage(`{}`),
		InboundDedupeConfig:  json.RawMessage(`{}`),
		InboundDedupeEnabled: false,
	})
	if err != nil {
		t.Fatalf("normalize source input with quiet hours: %v", err)
	}
	if string(normalized.QuietHoursConfig) != `{"enabled":true,"windows":[{"start":"22:00","end":"08:00"},{"start":"12:30","end":"13:15"}]}` {
		t.Fatalf("unexpected quiet hours config: %s", normalized.QuietHoursConfig)
	}
}

func TestNormalizeSourceInputRejectsInvalidQuietHoursWindows(t *testing.T) {
	tests := []struct {
		name   string
		config string
	}{
		{name: "enabled without windows", config: `{"enabled":true,"windows":[]}`},
		{name: "too many windows", config: `{"enabled":true,"windows":[{"start":"00:00","end":"01:00"},{"start":"02:00","end":"03:00"},{"start":"04:00","end":"05:00"},{"start":"06:00","end":"07:00"},{"start":"08:00","end":"09:00"},{"start":"10:00","end":"11:00"}]}`},
		{name: "invalid time", config: `{"enabled":true,"windows":[{"start":"25:00","end":"08:00"}]}`},
		{name: "same start and end", config: `{"enabled":true,"windows":[{"start":"22:00","end":"22:00"}]}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := normalizeSourceInput(CreateSourceInput{
				Code:                 "orders",
				Name:                 "Orders",
				Enabled:              true,
				AuthMode:             AuthModeNone,
				CompatMode:           "standard",
				QuietHoursConfig:     json.RawMessage(tt.config),
				RateLimitConfig:      json.RawMessage(`{}`),
				InboundDedupeConfig:  json.RawMessage(`{}`),
				InboundDedupeEnabled: false,
			})
			if !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("expected invalid input for %s, got %v", tt.config, err)
			}
		})
	}
}

func TestIngestCachesSourceConfigAndInvalidatesAfterUpdate(t *testing.T) {
	store := newMemoryStore(Source{
		ID:        "source-1",
		Code:      "orders",
		Name:      "Orders",
		Enabled:   true,
		AuthMode:  AuthModeToken,
		AuthToken: "sourceToken",
	})
	traceIndex := 0
	service := NewService(
		store,
		WithTraceIDGenerator(func() string {
			traceIndex++
			return fmt.Sprintf("trace-cache-%d", traceIndex)
		}),
		WithLatestPayloadFlushInterval(0),
		WithSourceConfigCacheTTL(time.Minute),
	)
	input := IngestInput{
		SourceCode: "orders",
		Method:     http.MethodPost,
		Path:       "/api/v1/ingest/orders",
		Headers:    http.Header{"Authorization": []string{"Bearer sourceToken"}},
		RemoteAddr: "127.0.0.1:4321",
		Body:       []byte(`{"title":"paid"}`),
	}

	if _, err := service.Ingest(context.Background(), input); err != nil {
		t.Fatalf("first ingest: %v", err)
	}
	if _, err := service.Ingest(context.Background(), input); err != nil {
		t.Fatalf("second ingest: %v", err)
	}
	if store.getSourceByCodeCalls != 1 {
		t.Fatalf("expected source config cache hit, got %d source lookups", store.getSourceByCodeCalls)
	}

	if _, err := service.UpdateSource(context.Background(), "source-1", UpdateSourceInput{
		Code:      "orders",
		Name:      "Orders API",
		Enabled:   true,
		AuthMode:  AuthModeToken,
		AuthToken: "sourceToken",
	}); err != nil {
		t.Fatalf("update source: %v", err)
	}
	if _, err := service.Ingest(context.Background(), input); err != nil {
		t.Fatalf("ingest after source update: %v", err)
	}
	if store.getSourceByCodeCalls != 1 {
		t.Fatalf("expected update to refresh source config cache, got %d source lookups", store.getSourceByCodeCalls)
	}
}

func TestIngestSourceConfigCacheCoalescesConcurrentMisses(t *testing.T) {
	store := newMemoryStore(Source{
		ID:        "source-1",
		Code:      "orders",
		Name:      "Orders",
		Enabled:   true,
		AuthMode:  AuthModeToken,
		AuthToken: "sourceToken",
	})
	service := NewService(
		store,
		WithLatestPayloadFlushInterval(0),
		WithSourceConfigCacheTTL(time.Minute),
	)
	input := IngestInput{
		SourceCode: "orders",
		Method:     http.MethodPost,
		Path:       "/api/v1/ingest/orders",
		Headers:    http.Header{"Authorization": []string{"Bearer sourceToken"}},
		RemoteAddr: "127.0.0.1:4321",
		Body:       []byte(`{"title":"paid"}`),
	}

	var wg sync.WaitGroup
	errs := make(chan error, 32)
	for index := 0; index < 32; index++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := service.Ingest(context.Background(), input)
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent ingest: %v", err)
		}
	}
	if calls := store.getSourceByCodeCallCount(); calls != 1 {
		t.Fatalf("expected concurrent source cache miss to be coalesced, got %d source lookups", calls)
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

func TestUpdateSourcePreservesWriteOnlyCredentialsWhenOmitted(t *testing.T) {
	store := newMemoryStore(Source{
		ID:         "source-1",
		Code:       "orders",
		Name:       "Orders",
		Enabled:    true,
		AuthMode:   AuthModeTokenAndHMAC,
		AuthToken:  "sourceToken",
		HMACSecret: "hmacSecret",
	})
	service := NewService(store)

	updated, err := service.UpdateSource(context.Background(), "source-1", UpdateSourceInput{
		Code:      "orders",
		Name:      "Orders API",
		Enabled:   true,
		AuthMode:  AuthModeTokenAndHMAC,
		AuthToken: "",
	})
	if err != nil {
		t.Fatalf("update source with omitted write-only credentials: %v", err)
	}
	if updated.AuthToken != "sourceToken" || updated.HMACSecret != "hmacSecret" {
		t.Fatalf("expected omitted write-only credentials to be preserved, got auth=%q hmac=%q", updated.AuthToken, updated.HMACSecret)
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
	mu                   sync.Mutex
	sources              map[string]Source
	latestPayload        json.RawMessage
	latestPayloadUpdates int
	latestPayloadAt      time.Time
	enqueued             []EnqueueInboundParams
	updateCalls          int
	getSourceByCodeCalls int
	hmacNonces           map[string]time.Time
}

type recordingRoutePlanPublisher struct {
	events []queue.RoutePlanEvent
	err    error
}

func (p *recordingRoutePlanPublisher) PublishRoutePlan(_ context.Context, event queue.RoutePlanEvent) (queue.PublishResult, error) {
	p.events = append(p.events, event)
	return queue.PublishResult{Stream: "MGP_ROUTE_PLAN", Sequence: uint64(len(p.events))}, p.err
}

type recordingRuntimeStateStore struct {
	latestPayloads map[string]latestPayloadSample
	deletedLatest  []string

	dedupeReserved bool
	reserveErr     error
	reserveCalls   int
	lastSourceID   string
	lastDedupeKey  string
	lastMessageID  string
	lastExpiresAt  time.Time

	hmacReserved      bool
	hmacErr           error
	hmacCalls         int
	lastHMACSourceID  string
	lastHMACNonce     string
	lastHMACExpiresAt time.Time
}

func (s *recordingRuntimeStateStore) PutLatestPayloadSample(_ context.Context, sourceID string, payload json.RawMessage, sampledAt time.Time) error {
	if s.latestPayloads == nil {
		s.latestPayloads = make(map[string]latestPayloadSample)
	}
	s.latestPayloads[sourceID] = latestPayloadSample{
		payload:   append(json.RawMessage(nil), payload...),
		sampledAt: sampledAt,
	}
	return nil
}

func (s *recordingRuntimeStateStore) GetLatestPayloadSample(_ context.Context, sourceID string) (json.RawMessage, time.Time, bool, error) {
	if s == nil || s.latestPayloads == nil {
		return nil, time.Time{}, false, nil
	}
	sample, ok := s.latestPayloads[sourceID]
	if !ok {
		return nil, time.Time{}, false, nil
	}
	return append(json.RawMessage(nil), sample.payload...), sample.sampledAt, true, nil
}

func (s *recordingRuntimeStateStore) DeleteLatestPayloadSample(_ context.Context, sourceID string) error {
	s.deletedLatest = append(s.deletedLatest, sourceID)
	delete(s.latestPayloads, sourceID)
	return nil
}

func (s *recordingRuntimeStateStore) ReserveInboundDedupeKey(_ context.Context, sourceID string, dedupeKey string, messageID string, expiresAt time.Time) (bool, error) {
	s.reserveCalls++
	s.lastSourceID = sourceID
	s.lastDedupeKey = dedupeKey
	s.lastMessageID = messageID
	s.lastExpiresAt = expiresAt
	if s.reserveErr != nil {
		return false, s.reserveErr
	}
	return s.dedupeReserved, nil
}

func (s *recordingRuntimeStateStore) ReserveHMACNonce(_ context.Context, sourceID string, nonce string, _ time.Time, expiresAt time.Time) (bool, error) {
	s.hmacCalls++
	s.lastHMACSourceID = sourceID
	s.lastHMACNonce = nonce
	s.lastHMACExpiresAt = expiresAt
	if s.hmacErr != nil {
		return false, s.hmacErr
	}
	return s.hmacReserved, nil
}

func newMemoryStore(sources ...Source) *memoryStore {
	store := &memoryStore{
		sources:    make(map[string]Source),
		hmacNonces: make(map[string]time.Time),
	}
	for _, source := range sources {
		store.sources[source.Code] = source
	}
	return store
}

func (m *memoryStore) ListSources(context.Context) ([]Source, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	sources := make([]Source, 0, len(m.sources))
	for _, configuredSource := range m.sources {
		sources = append(sources, configuredSource)
	}
	return sources, nil
}

func (m *memoryStore) CreateSource(context.Context, CreateSourceParams) (Source, error) {
	panic("not used")
}

func (m *memoryStore) GetSource(_ context.Context, id string) (Source, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, configuredSource := range m.sources {
		if configuredSource.ID == id {
			return configuredSource, nil
		}
	}
	return Source{}, ErrNotFound
}

func (m *memoryStore) GetSourceByCode(_ context.Context, code string) (Source, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.getSourceByCodeCalls++
	source, ok := m.sources[code]
	if !ok {
		return Source{}, ErrNotFound
	}
	return source, nil
}

func (m *memoryStore) getSourceByCodeCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.getSourceByCodeCalls
}

func (m *memoryStore) UpdateSource(_ context.Context, id string, params UpdateSourceParams) (Source, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updateCalls++
	var existing Source
	found := false
	for _, configuredSource := range m.sources {
		if configuredSource.ID == id {
			existing = configuredSource
			found = true
			break
		}
	}
	if !found {
		return Source{}, ErrNotFound
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
		QuietHoursConfig:             params.QuietHoursConfig,
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

func (m *memoryStore) DeleteSourceRuntimeData(context.Context, string) error {
	return nil
}

func (m *memoryStore) UpdateLatestPayloadSample(_ context.Context, sourceID string, payload json.RawMessage, sampledAt time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.latestPayloadUpdates++
	m.latestPayload = append(json.RawMessage(nil), payload...)
	m.latestPayloadAt = sampledAt
	return nil
}

func (m *memoryStore) latestPayloadString() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return string(m.latestPayload)
}

func (m *memoryStore) latestPayloadUpdateCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.latestPayloadUpdates
}

func waitForLatestPayloadUpdates(t *testing.T, store *memoryStore, expected int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if store.latestPayloadUpdateCount() == expected {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("expected %d latest payload updates, got %d", expected, store.latestPayloadUpdateCount())
}

func (m *memoryStore) ReserveHMACNonce(_ context.Context, sourceID string, nonce string, now time.Time, expiresAt time.Time) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := sourceID + "\x00" + nonce
	for existingKey, existingExpiresAt := range m.hmacNonces {
		if !existingExpiresAt.After(now) {
			delete(m.hmacNonces, existingKey)
		}
	}
	if existingExpiresAt, ok := m.hmacNonces[key]; ok && existingExpiresAt.After(now) {
		return false, nil
	}
	m.hmacNonces[key] = expiresAt
	return true, nil
}

func (m *memoryStore) EnqueueInbound(_ context.Context, params EnqueueInboundParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.enqueued = append(m.enqueued, params)
	return nil
}
