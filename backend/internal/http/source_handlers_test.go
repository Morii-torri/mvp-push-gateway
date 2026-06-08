package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"mvp-push-gateway/backend/internal/auth"
	httpapi "mvp-push-gateway/backend/internal/http"
	"mvp-push-gateway/backend/internal/route"
	"mvp-push-gateway/backend/internal/settings"
	"mvp-push-gateway/backend/internal/source"
)

func ptrTime(value string) *time.Time {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		panic(err)
	}
	return &parsed
}

func TestSourceCRUDRequiresAdminBearerAuthentication(t *testing.T) {
	sourceService := &fakeSourceService{
		listResult: []source.Source{{
			ID:       "source-1",
			Code:     "orders",
			Name:     "Orders",
			Enabled:  true,
			AuthMode: source.AuthModeToken,
		}},
	}
	handler := httpapi.NewHandler(
		testConfig(),
		httpapi.WithAuthService(fakeAuthService{authenticatedToken: "admin-session"}),
		httpapi.WithSourceService(sourceService),
	)

	unauthenticated := httptest.NewRequest(http.MethodGet, "/api/v1/sources", nil)
	unauthenticatedRec := httptest.NewRecorder()
	handler.ServeHTTP(unauthenticatedRec, unauthenticated)
	if unauthenticatedRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected source list without admin bearer to return 401, got %d", unauthenticatedRec.Code)
	}
	if sourceService.listCalls != 0 {
		t.Fatalf("expected source service not to be called without admin auth, got %d calls", sourceService.listCalls)
	}

	authenticated := httptest.NewRequest(http.MethodGet, "/api/v1/sources", nil)
	authenticated.Header.Set("Authorization", "Bearer admin-session")
	authenticatedRec := httptest.NewRecorder()
	handler.ServeHTTP(authenticatedRec, authenticated)
	if authenticatedRec.Code != http.StatusOK {
		t.Fatalf("expected source list with admin bearer to return 200, got %d", authenticatedRec.Code)
	}
	if sourceService.listCalls != 1 {
		t.Fatalf("expected one source list call, got %d", sourceService.listCalls)
	}
}

func TestSourceListOmitsLargeLatestPayloadSample(t *testing.T) {
	sourceService := &fakeSourceService{
		listResult: []source.Source{{
			ID:                           "source-1",
			Code:                         "orders",
			Name:                         "Orders",
			Enabled:                      true,
			AuthMode:                     source.AuthModeToken,
			LatestPayloadSample:          json.RawMessage(`{"title":"large-sample"}`),
			LatestPayloadSampleUpdatedAt: ptrTime("2026-05-08T10:30:00Z"),
		}},
	}
	handler := httpapi.NewHandler(
		testConfig(),
		httpapi.WithAuthService(fakeAuthService{authenticatedToken: "admin-session"}),
		httpapi.WithSourceService(sourceService),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sources", nil)
	req.Header.Set("Authorization", "Bearer admin-session")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected source list status 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Sources []map[string]json.RawMessage `json:"sources"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode source list: %v", err)
	}
	if len(body.Sources) != 1 {
		t.Fatalf("expected one source, got %+v", body.Sources)
	}
	if _, ok := body.Sources[0]["latest_payload_sample"]; ok || strings.Contains(rec.Body.String(), "large-sample") {
		t.Fatalf("expected source list to omit latest payload body, got %s", rec.Body.String())
	}
	if _, ok := body.Sources[0]["latest_payload_sample_updated_at"]; !ok {
		t.Fatalf("expected source list to keep latest payload timestamp, got %+v", body.Sources[0])
	}
}

func TestSourceCRUDRoutesUseAdminAuthentication(t *testing.T) {
	sourceService := &fakeSourceService{
		getResult: source.Source{
			ID:       "source-1",
			Code:     "orders",
			Name:     "Orders",
			Enabled:  true,
			AuthMode: source.AuthModeToken,
		},
		updateResult: source.Source{
			ID:       "source-1",
			Code:     "ordersUpdated",
			Name:     "Orders Updated",
			Enabled:  true,
			AuthMode: source.AuthModeHMAC,
		},
	}
	handler := httpapi.NewHandler(
		testConfig(),
		httpapi.WithAuthService(fakeAuthService{authenticatedToken: "admin-session"}),
		httpapi.WithSourceService(sourceService),
	)

	for _, tc := range []struct {
		name           string
		method         string
		path           string
		body           string
		expectedStatus int
	}{
		{name: "create", method: http.MethodPost, path: "/api/v1/sources", body: `{"code":"orders","name":"Orders","auth_mode":"token","auth_token":"sourceToken"}`, expectedStatus: http.StatusCreated},
		{name: "get", method: http.MethodGet, path: "/api/v1/sources/source-1", expectedStatus: http.StatusOK},
		{name: "update", method: http.MethodPut, path: "/api/v1/sources/source-1", body: `{"code":"ordersUpdated","name":"Orders Updated","auth_mode":"hmac","hmac_secret":"hmacSecret"}`, expectedStatus: http.StatusOK},
		{name: "delete", method: http.MethodDelete, path: "/api/v1/sources/source-1", expectedStatus: http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			req.Header.Set("Authorization", "Bearer admin-session")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != tc.expectedStatus {
				t.Fatalf("expected status %d, got %d body=%s", tc.expectedStatus, rec.Code, rec.Body.String())
			}
		})
	}

	if sourceService.createCalls != 1 || sourceService.getCalls != 1 || sourceService.updateCalls != 1 || sourceService.deleteCalls != 1 {
		t.Fatalf("unexpected CRUD calls: create=%d get=%d update=%d delete=%d",
			sourceService.createCalls,
			sourceService.getCalls,
			sourceService.updateCalls,
			sourceService.deleteCalls,
		)
	}
}

func TestSourceCreateAutoCreatesDefaultRouteFlow(t *testing.T) {
	sourceService := &fakeSourceService{}
	routeService := &fakeRouteService{}
	handler := httpapi.NewHandler(
		testConfig(),
		httpapi.WithAuthService(fakeAuthService{authenticatedToken: "admin-session"}),
		httpapi.WithSourceService(sourceService),
		httpapi.WithRouteService(routeService),
	)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/sources", strings.NewReader(`{
		"code":"orders",
		"name":"订单系统",
		"auth_mode":"token",
		"auth_token":"sourceToken"
	}`))
	req.Header.Set("Authorization", "Bearer admin-session")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d body=%s", rec.Code, rec.Body.String())
	}
	if routeService.createCalls != 1 {
		t.Fatalf("expected one default route flow create call, got %d", routeService.createCalls)
	}
	if routeService.createInput.SourceID != "source-1" || routeService.createInput.Name != "订单系统 路由组" || !routeService.createInput.Enabled || routeService.createInput.Mode != route.ModeTable {
		t.Fatalf("unexpected default route flow input: %+v", routeService.createInput)
	}
}

func TestSourcePUTDoesNotPassLatestPayloadFieldsToServiceInput(t *testing.T) {
	sourceService := &fakeSourceService{}
	handler := httpapi.NewHandler(
		testConfig(),
		httpapi.WithAuthService(fakeAuthService{authenticatedToken: "admin-session"}),
		httpapi.WithSourceService(sourceService),
	)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/sources/source-1", strings.NewReader(`{
		"code":"ordersUpdated",
		"name":"Orders Updated",
		"auth_mode":"token",
		"auth_token":"sourceToken",
		"latest_payload_sample":{"title":"should-not-pass"},
		"latest_payload_sample_updated_at":"2026-05-08T10:30:00Z"
	}`))
	req.Header.Set("Authorization", "Bearer admin-session")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if len(sourceService.updateInput.LatestPayloadSample) != 0 {
		t.Fatalf("expected latest payload not to be passed to service update input, got %s", sourceService.updateInput.LatestPayloadSample)
	}
	if sourceService.updateInput.LatestPayloadSampleUpdatedAt != nil {
		t.Fatalf("expected latest payload timestamp not to be passed to service update input, got %v", sourceService.updateInput.LatestPayloadSampleUpdatedAt)
	}
}

func TestIngestHandlerReturnsAcceptedResponse(t *testing.T) {
	sourceService := &fakeSourceService{
		ingestResult: source.IngestResult{
			TraceID: "trace-http",
			Status:  "accepted",
			Message: "accepted",
		},
	}
	handler := httpapi.NewHandler(testConfig(), httpapi.WithSourceService(sourceService))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/ingest/orders", strings.NewReader(`{"title":"paid"}`))
	req.Header.Set("Authorization", "Bearer sourceToken")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202 accepted, got %d body=%s", rec.Code, rec.Body.String())
	}

	var body struct {
		TraceID string `json:"trace_id"`
		Status  string `json:"status"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode ingest response: %v", err)
	}
	if body.TraceID != "trace-http" || body.Status != "accepted" || body.Message != "accepted" {
		t.Fatalf("unexpected ingest response: %+v", body)
	}
	if sourceService.ingestInput.SourceCode != "orders" || sourceService.ingestInput.Path != "/api/v1/ingest/orders" {
		t.Fatalf("unexpected ingest input: %+v", sourceService.ingestInput)
	}
}

func TestIngestHandlerUsesForwardedClientIPOnlyFromTrustedProxy(t *testing.T) {
	sourceService := &fakeSourceService{
		ingestResult: source.IngestResult{
			TraceID: "trace-http",
			Status:  "accepted",
			Message: "accepted",
		},
	}
	cfg := testConfig()
	cfg.Server.TrustedProxies = []string{"10.0.0.0/8"}
	handler := httpapi.NewHandler(cfg, httpapi.WithSourceService(sourceService))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/ingest/orders", strings.NewReader(`{"title":"paid"}`))
	req.RemoteAddr = "10.0.0.5:1234"
	req.Header.Set("X-Forwarded-For", "203.0.113.7, 10.0.0.5")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202 accepted, got %d body=%s", rec.Code, rec.Body.String())
	}
	if sourceService.ingestInput.RemoteAddr != "203.0.113.7" {
		t.Fatalf("expected trusted proxy client ip 203.0.113.7, got %q", sourceService.ingestInput.RemoteAddr)
	}
}

func TestIngestHandlerIgnoresForwardedClientIPFromUntrustedPeer(t *testing.T) {
	sourceService := &fakeSourceService{
		ingestResult: source.IngestResult{
			TraceID: "trace-http",
			Status:  "accepted",
			Message: "accepted",
		},
	}
	cfg := testConfig()
	cfg.Server.TrustedProxies = []string{"10.0.0.0/8"}
	handler := httpapi.NewHandler(cfg, httpapi.WithSourceService(sourceService))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/ingest/orders", strings.NewReader(`{"title":"paid"}`))
	req.RemoteAddr = "198.51.100.10:1234"
	req.Header.Set("X-Forwarded-For", "203.0.113.7")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202 accepted, got %d body=%s", rec.Code, rec.Body.String())
	}
	if sourceService.ingestInput.RemoteAddr != "198.51.100.10" {
		t.Fatalf("expected untrusted proxy to fall back to remote addr, got %q", sourceService.ingestInput.RemoteAddr)
	}
}

func TestIngestHandlerRecordsSecurityAuditForRejectedSourceRequests(t *testing.T) {
	sourceService := &fakeSourceService{ingestErr: source.ErrUnauthorized}
	auditService := &fakeAuditService{}
	handler := httpapi.NewHandler(
		testConfig(),
		httpapi.WithSourceService(sourceService),
		httpapi.WithAuditService(auditService),
	)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/ingest/orders", strings.NewReader(`{"title":"paid"}`))
	req.RemoteAddr = "203.0.113.9:1234"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 unauthorized, got %d body=%s", rec.Code, rec.Body.String())
	}
	if auditService.recordCalls != 1 {
		t.Fatalf("expected one security audit record, got %d", auditService.recordCalls)
	}
	if auditService.recordInput.Action != "reject_unauthorized" || auditService.recordInput.ResourceType != "source_ingest" || auditService.recordInput.ResourceID != "orders" {
		t.Fatalf("unexpected security audit input: %+v", auditService.recordInput)
	}
	if auditService.recordInput.IPAddress != "203.0.113.9" {
		t.Fatalf("expected audit ip to use request client ip, got %q", auditService.recordInput.IPAddress)
	}
}

func TestIngestHandlerRecordsSecurityAuditForOversizedPayload(t *testing.T) {
	auditService := &fakeAuditService{}
	settingsService := &fakeSettingsService{
		intValues: map[string]int{
			settings.KeyIngestMaxPayloadBytes: 16,
		},
	}
	handler := httpapi.NewHandler(
		testConfig(),
		httpapi.WithSourceService(&fakeSourceService{}),
		httpapi.WithSettingsService(settingsService),
		httpapi.WithAuditService(auditService),
	)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/ingest/orders", strings.NewReader(`{"title":"payload too large"}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413 payload too large, got %d body=%s", rec.Code, rec.Body.String())
	}
	if auditService.recordCalls != 1 {
		t.Fatalf("expected one security audit record, got %d", auditService.recordCalls)
	}
	if auditService.recordInput.Action != "reject_payload_too_large" || auditService.recordInput.ResourceType != "source_ingest" || auditService.recordInput.ResourceID != "orders" {
		t.Fatalf("unexpected oversized payload audit input: %+v", auditService.recordInput)
	}
}

func TestIngestHandlerUsesSystemPayloadLimit(t *testing.T) {
	sourceService := &fakeSourceService{
		ingestResult: source.IngestResult{
			TraceID: "trace-http",
			Status:  "accepted",
			Message: "accepted",
		},
	}
	settingsService := &fakeSettingsService{
		intValues: map[string]int{
			settings.KeyIngestMaxPayloadBytes: 32,
		},
	}
	handler := httpapi.NewHandler(
		testConfig(),
		httpapi.WithSourceService(sourceService),
		httpapi.WithSettingsService(settingsService),
	)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/ingest/orders", strings.NewReader(`{"title":"payload larger than configured limit"}`))
	req.Header.Set("Authorization", "Bearer sourceToken")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected configured payload limit to reject request, got %d body=%s", rec.Code, rec.Body.String())
	}
	if sourceService.ingestInput.SourceCode != "" {
		t.Fatalf("expected oversized request not to reach source service, got %+v", sourceService.ingestInput)
	}
	if settingsService.intCalls != 1 {
		t.Fatalf("expected one settings lookup, got %d", settingsService.intCalls)
	}
}

func TestIngestWithOnlyXMGPTokensReturnsPublishedAuthErrorCode(t *testing.T) {
	store := &httpSourceStore{
		configuredSource: source.Source{
			ID:        "source-1",
			Code:      "orders",
			Name:      "Orders",
			Enabled:   true,
			AuthMode:  source.AuthModeToken,
			AuthToken: "sourceToken",
		},
	}
	handler := httpapi.NewHandler(testConfig(), httpapi.WithSourceService(source.NewService(store)))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/ingest/orders", strings.NewReader(`{"title":"paid"}`))
	req.Header.Set("X-MGP-Token", "sourceToken")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 unauthorized, got %d body=%s", rec.Code, rec.Body.String())
	}
	if got := responseErrorCode(t, rec); got != "MGP-AUTH-001" {
		t.Fatalf("expected MGP-AUTH-001, got %q", got)
	}
	if store.latestPayloadUpdates != 0 {
		t.Fatalf("expected latest payload to remain unchanged, got %d updates", store.latestPayloadUpdates)
	}
}

func TestIngestErrorCodesMatchPublishedContract(t *testing.T) {
	for _, tc := range []struct {
		name           string
		err            error
		expectedStatus int
		expectedCode   string
	}{
		{name: "source unauthorized", err: source.ErrUnauthorized, expectedStatus: http.StatusUnauthorized, expectedCode: "MGP-AUTH-001"},
		{name: "source not found", err: source.ErrNotFound, expectedStatus: http.StatusNotFound, expectedCode: "MGP-SRC-001"},
		{name: "source disabled", err: source.ErrDisabled, expectedStatus: http.StatusForbidden, expectedCode: "MGP-SRC-001"},
		{name: "ip denied", err: source.ErrIPNotAllowed, expectedStatus: http.StatusForbidden, expectedCode: "MGP-SRC-002"},
		{name: "invalid json", err: source.ErrInvalidJSON, expectedStatus: http.StatusBadRequest, expectedCode: "MGP-PAYLOAD-001"},
		{name: "payload too large", err: source.ErrPayloadTooLarge, expectedStatus: http.StatusRequestEntityTooLarge, expectedCode: "MGP-PAYLOAD-002"},
		{name: "duplicate inbound", err: source.ErrDuplicateInbound, expectedStatus: http.StatusConflict, expectedCode: "MGP-DEDUPE-001"},
		{name: "invalid dedupe config", err: source.ErrInvalidDedupeConfig, expectedStatus: http.StatusBadRequest, expectedCode: "MGP-DEDUPE-001"},
		{name: "dedupe store failed", err: source.ErrDedupeStoreFailed, expectedStatus: http.StatusServiceUnavailable, expectedCode: "MGP-DEDUPE-002"},
		{name: "hmac nonce store failed", err: source.ErrHMACNonceStoreFailed, expectedStatus: http.StatusServiceUnavailable, expectedCode: "MGP-HMAC-002"},
		{name: "rate limited", err: source.ErrRateLimited, expectedStatus: http.StatusTooManyRequests, expectedCode: "MGP-RATE-001"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			handler := httpapi.NewHandler(testConfig(), httpapi.WithSourceService(&fakeSourceService{ingestErr: tc.err}))
			req := httptest.NewRequest(http.MethodPost, "/api/v1/ingest/orders", strings.NewReader(`{"title":"paid"}`))
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != tc.expectedStatus {
				t.Fatalf("expected status %d, got %d body=%s", tc.expectedStatus, rec.Code, rec.Body.String())
			}
			if got := responseErrorCode(t, rec); got != tc.expectedCode {
				t.Fatalf("expected error code %q, got %q", tc.expectedCode, got)
			}
		})
	}
}

func TestSourceCRUDErrorCodesAvoidLegacySourceCodes(t *testing.T) {
	handler := httpapi.NewHandler(
		testConfig(),
		httpapi.WithAuthService(fakeAuthService{authenticatedToken: "admin-session"}),
		httpapi.WithSourceService(&fakeSourceService{createErr: source.ErrAlreadyExists, getErr: source.ErrNotFound}),
	)

	create := httptest.NewRequest(http.MethodPost, "/api/v1/sources", strings.NewReader(`{"code":"orders","name":"Orders"}`))
	create.Header.Set("Authorization", "Bearer admin-session")
	createRec := httptest.NewRecorder()
	handler.ServeHTTP(createRec, create)
	if createRec.Code != http.StatusConflict {
		t.Fatalf("expected duplicate source conflict, got %d body=%s", createRec.Code, createRec.Body.String())
	}
	if got := responseErrorCode(t, createRec); got != "MGP-SRC-001" {
		t.Fatalf("expected duplicate source to use MGP-SRC-001, got %q", got)
	}

	get := httptest.NewRequest(http.MethodGet, "/api/v1/sources/source-1", nil)
	get.Header.Set("Authorization", "Bearer admin-session")
	getRec := httptest.NewRecorder()
	handler.ServeHTTP(getRec, get)
	if getRec.Code != http.StatusNotFound {
		t.Fatalf("expected missing source 404, got %d body=%s", getRec.Code, getRec.Body.String())
	}
	if got := responseErrorCode(t, getRec); got != "MGP-SRC-001" {
		t.Fatalf("expected missing source to use MGP-SRC-001, got %q", got)
	}
}

type fakeSourceService struct {
	mu sync.Mutex

	listResult          []source.Source
	getResult           source.Source
	updateResult        source.Source
	ingestResult        source.IngestResult
	runtimeStatsResults []source.RuntimeStats
	deliveryStatuses    map[string]bool
	deliveryStatusSteps []map[string]bool

	createErr error
	getErr    error
	updateErr error
	deleteErr error
	ingestErr error

	listCalls               int
	createCalls             int
	getCalls                int
	updateCalls             int
	deleteCalls             int
	ingestCalls             int
	runtimeStatsCalls       int
	deliveryStatusCalls     int
	cleanupRuntimeDataCalls int

	ingestInput      source.IngestInput
	ingestInputs     []source.IngestInput
	createInputs     []source.CreateSourceInput
	deletedSourceIDs []string
	updateInput      source.UpdateSourceInput
}

func (f *fakeSourceService) ListSources(context.Context) ([]source.Source, error) {
	f.listCalls++
	return f.listResult, nil
}

func (f *fakeSourceService) CreateSource(_ context.Context, input source.CreateSourceInput) (source.Source, error) {
	f.createCalls++
	f.createInputs = append(f.createInputs, input)
	if f.createErr != nil {
		return source.Source{}, f.createErr
	}
	sourceID := "source-1"
	if f.createCalls > 1 {
		sourceID = "source-" + strconv.Itoa(f.createCalls)
	}
	return source.Source{
		ID:         sourceID,
		Code:       input.Code,
		Name:       input.Name,
		Enabled:    input.Enabled,
		AuthMode:   input.AuthMode,
		AuthToken:  input.AuthToken,
		HMACSecret: input.HMACSecret,
	}, nil
}

func (f *fakeSourceService) GetSource(context.Context, string) (source.Source, error) {
	f.getCalls++
	if f.getErr != nil {
		return source.Source{}, f.getErr
	}
	return f.getResult, nil
}

func (f *fakeSourceService) UpdateSource(_ context.Context, _ string, input source.UpdateSourceInput) (source.Source, error) {
	f.updateCalls++
	f.updateInput = input
	if f.updateErr != nil {
		return source.Source{}, f.updateErr
	}
	if f.updateResult.ID != "" {
		return f.updateResult, nil
	}
	return source.Source{
		ID:       "source-1",
		Code:     input.Code,
		Name:     input.Name,
		Enabled:  input.Enabled,
		AuthMode: input.AuthMode,
	}, nil
}

func (f *fakeSourceService) DeleteSource(_ context.Context, id string) error {
	f.deleteCalls++
	f.deletedSourceIDs = append(f.deletedSourceIDs, id)
	if f.deleteErr != nil {
		return f.deleteErr
	}
	return nil
}

func (f *fakeSourceService) DeleteSourceRuntimeData(context.Context, string) error {
	f.cleanupRuntimeDataCalls++
	return nil
}

func (f *fakeSourceService) Ingest(_ context.Context, input source.IngestInput) (source.IngestResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ingestCalls++
	f.ingestInput = input
	f.ingestInputs = append(f.ingestInputs, input)
	if f.ingestErr != nil {
		return source.IngestResult{}, f.ingestErr
	}
	return f.ingestResult, nil
}

func (f *fakeSourceService) RuntimeStats(context.Context) (source.RuntimeStats, error) {
	f.runtimeStatsCalls++
	if len(f.runtimeStatsResults) == 0 {
		return source.RuntimeStats{}, nil
	}
	index := f.runtimeStatsCalls - 1
	if index >= len(f.runtimeStatsResults) {
		index = len(f.runtimeStatsResults) - 1
	}
	return f.runtimeStatsResults[index], nil
}

func (f *fakeSourceService) PerformanceDeliveryStatuses(_ context.Context, traceIDs []string) (map[string]bool, error) {
	f.deliveryStatusCalls++
	currentStatuses := f.deliveryStatuses
	if len(f.deliveryStatusSteps) > 0 {
		index := f.deliveryStatusCalls - 1
		if index >= len(f.deliveryStatusSteps) {
			index = len(f.deliveryStatusSteps) - 1
		}
		currentStatuses = f.deliveryStatusSteps[index]
	}
	statuses := make(map[string]bool, len(traceIDs))
	for _, traceID := range traceIDs {
		if currentStatuses == nil {
			statuses[traceID] = true
			continue
		}
		statuses[traceID] = currentStatuses[traceID]
	}
	return statuses, nil
}

type httpSourceStore struct {
	configuredSource     source.Source
	latestPayloadUpdates int
}

func (s *httpSourceStore) ListSources(context.Context) ([]source.Source, error) {
	return nil, nil
}

func (s *httpSourceStore) CreateSource(context.Context, source.CreateSourceParams) (source.Source, error) {
	return source.Source{}, nil
}

func (s *httpSourceStore) GetSource(context.Context, string) (source.Source, error) {
	return source.Source{}, source.ErrNotFound
}

func (s *httpSourceStore) GetSourceByCode(_ context.Context, code string) (source.Source, error) {
	if s.configuredSource.Code != code {
		return source.Source{}, source.ErrNotFound
	}
	return s.configuredSource, nil
}

func (s *httpSourceStore) UpdateSource(context.Context, string, source.UpdateSourceParams) (source.Source, error) {
	return source.Source{}, nil
}

func (s *httpSourceStore) DeleteSource(context.Context, string) error {
	return nil
}

func (s *httpSourceStore) DeleteSourceRuntimeData(context.Context, string) error {
	return nil
}

func (s *httpSourceStore) UpdateLatestPayloadSample(context.Context, string, json.RawMessage, time.Time) error {
	s.latestPayloadUpdates++
	return nil
}

func (s *httpSourceStore) ReserveHMACNonce(context.Context, string, string, time.Time, time.Time) (bool, error) {
	return true, nil
}

func (s *httpSourceStore) EnqueueInbound(context.Context, source.EnqueueInboundParams) error {
	return nil
}

func responseErrorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()

	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode error response: %v body=%s", err, rec.Body.String())
	}
	return body.Error.Code
}

var _ = auth.Admin{}
