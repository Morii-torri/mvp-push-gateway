package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	httpapi "mvp-push-gateway/backend/internal/http"
	"mvp-push-gateway/backend/internal/monitoring"
	"mvp-push-gateway/backend/internal/queue"
	"mvp-push-gateway/backend/internal/statistics"
)

func TestMonitoringEndpointsRequireAdminAuthentication(t *testing.T) {
	handler := httpapi.NewHandler(
		testConfig(),
		httpapi.WithAuthService(fakeAuthService{authenticatedToken: "admin-session"}),
		httpapi.WithMonitoringService(&fakeMonitoringService{}),
		httpapi.WithStatisticsService(&fakeStatisticsService{}),
	)

	for _, path := range []string{
		"/api/v1/monitoring/queue",
		"/api/v1/monitor/queues",
		"/api/v1/monitor/notifications/stream",
		"/api/v1/statistics/overview",
		"/api/v1/stats/overview",
		"/api/v1/maintenance/retention/cleanup",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		if strings.HasSuffix(path, "/cleanup") {
			req = httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected %s to require admin auth, got %d", path, rec.Code)
		}
	}
}

func TestNotificationStreamEndpointWritesSSESnapshot(t *testing.T) {
	handler := httpapi.NewHandler(
		testConfig(),
		httpapi.WithAuthService(fakeAuthService{authenticatedToken: "admin-session"}),
		httpapi.WithMonitoringService(&fakeMonitoringService{
			queueSnapshot: monitoring.QueueSnapshot{
				Summary: monitoring.QueueSummary{
					RoutePlanPending: 4,
					DeadLetterCount:  2,
				},
			},
		}),
		httpapi.WithStatisticsService(&fakeStatisticsService{
			overview: statistics.Overview{
				Summary: statistics.Summary{TotalSent: 9},
			},
		}),
	)
	server := httptest.NewServer(handler)
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/api/v1/monitor/notifications/stream", nil)
	if err != nil {
		t.Fatalf("create stream request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer admin-session")
	resp, err := server.Client().Do(req)
	if err != nil {
		t.Fatalf("open notification stream: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected notification stream status 200, got %d", resp.StatusCode)
	}
	if contentType := resp.Header.Get("Content-Type"); !strings.Contains(contentType, "text/event-stream") {
		t.Fatalf("expected event-stream content type, got %q", contentType)
	}
	buf := make([]byte, 1024)
	n, err := resp.Body.Read(buf)
	if err != nil {
		t.Fatalf("read first notification event: %v", err)
	}
	chunk := string(buf[:n])
	if !strings.Contains(chunk, "event: notifications") ||
		!strings.Contains(chunk, `"route_plan_pending":4`) ||
		!strings.Contains(chunk, `"total_sent":9`) {
		t.Fatalf("unexpected notification event: %s", chunk)
	}
}

func TestQueueMonitoringEndpointReturnsCleanupStatus(t *testing.T) {
	lastRun := time.Date(2026, 5, 9, 12, 30, 0, 0, time.UTC)
	handler := httpapi.NewHandler(
		testConfig(),
		httpapi.WithAuthService(fakeAuthService{authenticatedToken: "admin-session"}),
		httpapi.WithMonitoringService(&fakeMonitoringService{
			queueSnapshot: monitoring.QueueSnapshot{
				Summary: monitoring.QueueSummary{
					RoutePlanPending:      2,
					SendMessagePending:    3,
					OldestJobWaitSeconds:  120,
					PlanningAvgDurationMS: 80,
					PlanningP95DurationMS: 140,
					SendingAvgDurationMS:  180,
					SendingP95DurationMS:  320,
					PlatformFailureRate:   12.5,
					RateLimitedCount:      4,
					DeadLetterCount:       1,
				},
				CleanupStatus: monitoring.CleanupStatus{
					LastRunAt:        &lastRun,
					RetentionDays:    30,
					BatchSize:        200,
					LastBatchDeleted: 10,
					TotalDeleted:     25,
					Completed:        false,
					HasMore:          true,
				},
			},
		}),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/monitor/queues", nil)
	req.Header.Set("Authorization", "Bearer admin-session")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected queue monitoring status 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var body struct {
		Summary struct {
			RoutePlanPending int `json:"route_plan_pending"`
		} `json:"summary"`
		CleanupStatus struct {
			RetentionDays int  `json:"retention_days"`
			TotalDeleted  int  `json:"total_deleted"`
			HasMore       bool `json:"has_more"`
		} `json:"cleanup_status"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode queue monitoring response: %v", err)
	}
	if body.Summary.RoutePlanPending != 2 || body.CleanupStatus.RetentionDays != 30 || body.CleanupStatus.TotalDeleted != 25 || !body.CleanupStatus.HasMore {
		t.Fatalf("unexpected queue monitoring payload: %+v", body)
	}
}

func TestQueueMonitoringEndpointReturnsJetStreamStats(t *testing.T) {
	handler := httpapi.NewHandler(
		testConfig(),
		httpapi.WithAuthService(fakeAuthService{authenticatedToken: "admin-session"}),
		httpapi.WithMonitoringService(&fakeMonitoringService{
			queueSnapshot: monitoring.QueueSnapshot{
				JetStream: queue.JetStreamSnapshot{
					Enabled: true,
					Streams: []queue.JetStreamStreamStats{{
						Name:     "MGP_SEND",
						Messages: 18,
					}},
					Consumers: []queue.JetStreamConsumerStats{{
						Stream:     "MGP_SEND",
						Name:       "send-workers",
						Pending:    7,
						AckPending: 3,
					}},
				},
			},
		}),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/monitor/queues", nil)
	req.Header.Set("Authorization", "Bearer admin-session")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected queue monitoring status 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var body struct {
		JetStream struct {
			Enabled bool `json:"enabled"`
			Streams []struct {
				Name     string `json:"name"`
				Messages uint64 `json:"messages"`
			} `json:"streams"`
			Consumers []struct {
				Name       string `json:"name"`
				Pending    uint64 `json:"pending"`
				AckPending int    `json:"ack_pending"`
			} `json:"consumers"`
		} `json:"jetstream"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode queue monitoring response: %v", err)
	}
	if !body.JetStream.Enabled || len(body.JetStream.Streams) != 1 || body.JetStream.Streams[0].Messages != 18 {
		t.Fatalf("unexpected stream stats: %+v", body.JetStream.Streams)
	}
	if len(body.JetStream.Consumers) != 1 || body.JetStream.Consumers[0].Pending != 7 || body.JetStream.Consumers[0].AckPending != 3 {
		t.Fatalf("unexpected consumer stats: %+v", body.JetStream.Consumers)
	}
}

func TestMonitoringEndpointsPassWindowQueryToServices(t *testing.T) {
	monitoringService := &fakeMonitoringService{}
	statisticsService := &fakeStatisticsService{}
	handler := httpapi.NewHandler(
		testConfig(),
		httpapi.WithAuthService(fakeAuthService{authenticatedToken: "admin-session"}),
		httpapi.WithMonitoringService(monitoringService),
		httpapi.WithStatisticsService(statisticsService),
	)

	queueReq := httptest.NewRequest(http.MethodGet, "/api/v1/monitor/queues?window=1h", nil)
	queueReq.Header.Set("Authorization", "Bearer admin-session")
	queueRec := httptest.NewRecorder()
	handler.ServeHTTP(queueRec, queueReq)
	if queueRec.Code != http.StatusOK {
		t.Fatalf("expected queue monitoring status 200, got %d body=%s", queueRec.Code, queueRec.Body.String())
	}
	if monitoringService.queueParams.Window != time.Hour {
		t.Fatalf("expected queue window 1h, got %s", monitoringService.queueParams.Window)
	}

	overviewReq := httptest.NewRequest(http.MethodGet, "/api/v1/stats/overview?window=7d", nil)
	overviewReq.Header.Set("Authorization", "Bearer admin-session")
	overviewRec := httptest.NewRecorder()
	handler.ServeHTTP(overviewRec, overviewReq)
	if overviewRec.Code != http.StatusOK {
		t.Fatalf("expected overview status 200, got %d body=%s", overviewRec.Code, overviewRec.Body.String())
	}
	if statisticsService.params.Window != 7*24*time.Hour {
		t.Fatalf("expected overview window 7d, got %s", statisticsService.params.Window)
	}
}

func TestOverviewStatisticsEndpointReturnsStableDashboardShape(t *testing.T) {
	handler := httpapi.NewHandler(
		testConfig(),
		httpapi.WithAuthService(fakeAuthService{authenticatedToken: "admin-session"}),
		httpapi.WithStatisticsService(&fakeStatisticsService{
			overview: statistics.Overview{
				Summary: statistics.Summary{
					TotalSent:     12,
					Successful:    10,
					Failed:        2,
					SuccessRate:   83.33,
					AverageQPS:    0.14,
					TotalReceived: 15,
				},
				Trend:            []statistics.TrendPoint{{Sent: 1}},
				PlatformRankings: []statistics.PlatformRanking{{Name: "Webhook A", Sent: 12}},
			},
		}),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats/overview", nil)
	req.Header.Set("Authorization", "Bearer admin-session")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected overview status 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Summary struct {
			TotalSent int `json:"total_sent"`
		} `json:"summary"`
		Trend            []any `json:"trend"`
		PlatformRankings []any `json:"platform_rankings"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode overview response: %v", err)
	}
	if body.Summary.TotalSent != 12 || len(body.Trend) != 1 || len(body.PlatformRankings) != 1 {
		t.Fatalf("unexpected overview response: %+v", body)
	}
}

func TestRetentionCleanupEndpointRunsSingleBatch(t *testing.T) {
	lastRun := time.Date(2026, 5, 9, 13, 0, 0, 0, time.UTC)
	service := &fakeMonitoringService{
		cleanupStatus: monitoring.CleanupStatus{
			LastRunAt:        &lastRun,
			RetentionDays:    30,
			BatchSize:        150,
			LastBatchDeleted: 7,
			TotalDeleted:     42,
			Completed:        true,
			HasMore:          false,
		},
	}
	handler := httpapi.NewHandler(
		testConfig(),
		httpapi.WithAuthService(fakeAuthService{authenticatedToken: "admin-session"}),
		httpapi.WithMonitoringService(service),
	)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/maintenance/retention/cleanup", strings.NewReader(`{"retention_days":30,"batch_size":150}`))
	req.Header.Set("Authorization", "Bearer admin-session")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected cleanup status 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if service.cleanupInput.RetentionDays != 30 || service.cleanupInput.BatchSize != 150 {
		t.Fatalf("unexpected cleanup input: %+v", service.cleanupInput)
	}
}

func TestRetentionCleanupHandlerRecordsAudit(t *testing.T) {
	lastRun := time.Date(2026, 5, 9, 13, 0, 0, 0, time.UTC)
	service := &fakeMonitoringService{
		cleanupStatus: monitoring.CleanupStatus{
			LastRunAt:        &lastRun,
			RetentionDays:    45,
			BatchSize:        150,
			LastBatchDeleted: 7,
			TotalDeleted:     42,
			Completed:        true,
		},
	}
	auditService := &fakeAuditService{}
	handler := httpapi.NewHandler(
		testConfig(),
		httpapi.WithAuthService(fakeAuthService{authenticatedToken: "admin-session"}),
		httpapi.WithMonitoringService(service),
		httpapi.WithAuditService(auditService),
	)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/maintenance/retention/cleanup", strings.NewReader(`{"retention_days":45,"batch_size":150}`))
	req.Header.Set("Authorization", "Bearer admin-session")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected cleanup status 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if auditService.recordCalls != 1 || auditService.recordInputs[0].Action != "run" || auditService.recordInputs[0].ResourceType != "retention_cleanup" {
		t.Fatalf("expected retention cleanup audit record, calls=%d inputs=%+v", auditService.recordCalls, auditService.recordInputs)
	}
	if !strings.Contains(string(auditService.recordInputs[0].RequestSnapshot), `"retention_days":45`) {
		t.Fatalf("expected cleanup parameters in audit request, got %s", auditService.recordInputs[0].RequestSnapshot)
	}
}

type fakeMonitoringService struct {
	queueSnapshot  monitoring.QueueSnapshot
	queueSnapshots []monitoring.QueueSnapshot
	cleanupStatus  monitoring.CleanupStatus
	cleanupInput   monitoring.RetentionCleanupParams
	queueParams    monitoring.QueryParams
	queueCalls     int
}

func (f *fakeMonitoringService) GetQueueMonitoringSnapshot(_ context.Context, params monitoring.QueryParams) (monitoring.QueueSnapshot, error) {
	f.queueParams = params
	f.queueCalls++
	if len(f.queueSnapshots) > 0 {
		index := f.queueCalls - 1
		if index >= len(f.queueSnapshots) {
			index = len(f.queueSnapshots) - 1
		}
		return f.queueSnapshots[index], nil
	}
	return f.queueSnapshot, nil
}

func (f *fakeMonitoringService) RunRetentionCleanup(_ context.Context, params monitoring.RetentionCleanupParams) (monitoring.CleanupStatus, error) {
	f.cleanupInput = params
	return f.cleanupStatus, nil
}

type fakeStatisticsService struct {
	overview statistics.Overview
	params   statistics.QueryParams
}

func (f *fakeStatisticsService) GetOverview(_ context.Context, params statistics.QueryParams) (statistics.Overview, error) {
	f.params = params
	return f.overview, nil
}
