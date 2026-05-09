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

func TestOverviewStatisticsEndpointReturnsStableDashboardShape(t *testing.T) {
	handler := httpapi.NewHandler(
		testConfig(),
		httpapi.WithAuthService(fakeAuthService{authenticatedToken: "admin-session"}),
		httpapi.WithStatisticsService(&fakeStatisticsService{
			overview: statistics.Overview{
				Summary: statistics.Summary{
					TotalSent:   12,
					Successful:  10,
					Failed:      2,
					SuccessRate: 83.33,
					AverageQPS:  0.14,
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

type fakeMonitoringService struct {
	queueSnapshot monitoring.QueueSnapshot
	cleanupStatus monitoring.CleanupStatus
	cleanupInput  monitoring.RetentionCleanupParams
}

func (f *fakeMonitoringService) GetQueueMonitoringSnapshot(context.Context) (monitoring.QueueSnapshot, error) {
	return f.queueSnapshot, nil
}

func (f *fakeMonitoringService) RunRetentionCleanup(_ context.Context, params monitoring.RetentionCleanupParams) (monitoring.CleanupStatus, error) {
	f.cleanupInput = params
	return f.cleanupStatus, nil
}

type fakeStatisticsService struct {
	overview statistics.Overview
}

func (f *fakeStatisticsService) GetOverview(context.Context) (statistics.Overview, error) {
	return f.overview, nil
}
