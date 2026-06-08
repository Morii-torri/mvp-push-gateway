package httpapi_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	httpapi "mvp-push-gateway/backend/internal/http"
	"mvp-push-gateway/backend/internal/monitoring"
	"mvp-push-gateway/backend/internal/provider"
	"mvp-push-gateway/backend/internal/route"
	"mvp-push-gateway/backend/internal/settings"
	"mvp-push-gateway/backend/internal/source"
	msgtemplate "mvp-push-gateway/backend/internal/template"
)

func TestSettingsPerformanceTestLeavesRuleKeyForRouteServiceGenerator(t *testing.T) {
	settingsService := &fakeSettingsService{}
	routeService := &fakeRouteService{}
	handler := httpapi.NewHandler(
		testConfig(),
		httpapi.WithAuthService(fakeAuthService{authenticatedToken: "admin-session"}),
		httpapi.WithSettingsService(settingsService),
		httpapi.WithSourceService(&fakeSourceService{}),
		httpapi.WithProviderService(&fakeProviderService{}),
		httpapi.WithTemplateService(&fakePerformanceTemplateService{}),
		httpapi.WithRouteService(routeService),
	)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/settings/performance-test", strings.NewReader(`{"message_count":10}`))
	req.Header.Set("Authorization", "Bearer admin-session")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected performance test to return 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if routeService.saveRulesCalls != 1 {
		t.Fatalf("expected one route rule save, got %d", routeService.saveRulesCalls)
	}
	if got := routeService.saveRulesInput.Rules[0].RuleKey; got != "" {
		t.Fatalf("expected performance test to let route service generate UUID rule_key, got %q", got)
	}
	if settingsService.performanceTestInput.GeneratedSourceCode == "" ||
		settingsService.performanceTestInput.GeneratedRouteName == "" ||
		settingsService.performanceTestInput.GeneratedChannelName == "" {
		t.Fatalf("expected generated resource names to be passed to performance test, got %+v", settingsService.performanceTestInput)
	}
}

func TestSettingsPerformanceTestCleansGeneratedResourcesAfterSuccess(t *testing.T) {
	settingsService := &fakeSettingsService{}
	sourceService := &fakeSourceService{}
	providerService := &fakeProviderService{}
	templateService := &fakePerformanceTemplateService{}
	routeService := &fakeRouteService{}
	handler := httpapi.NewHandler(
		testConfig(),
		httpapi.WithAuthService(fakeAuthService{authenticatedToken: "admin-session"}),
		httpapi.WithSettingsService(settingsService),
		httpapi.WithSourceService(sourceService),
		httpapi.WithProviderService(providerService),
		httpapi.WithTemplateService(templateService),
		httpapi.WithRouteService(routeService),
	)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/settings/performance-test", strings.NewReader(`{"message_count":10}`))
	req.Header.Set("Authorization", "Bearer admin-session")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected performance test to return 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if routeService.deleteCalls != 1 || templateService.deleteCalls != 1 || providerService.deleteChannelCalls != 1 || sourceService.deleteCalls != 1 {
		t.Fatalf("expected generated resources to be cleaned after success, got route=%d template=%d channel=%d source=%d",
			routeService.deleteCalls,
			templateService.deleteCalls,
			providerService.deleteChannelCalls,
			sourceService.deleteCalls,
		)
	}
}

func TestSettingsPerformanceTestRunsPlanningWorkerAndIngestSamples(t *testing.T) {
	settingsService := &fakeSettingsService{}
	sourceService := &fakeSourceService{}
	routeService := &fakeRouteService{}
	planningWorker := &fakePlanningWorker{processed: 1}
	deliveryWorker := &fakePlanningWorker{processed: 1}
	handler := httpapi.NewHandler(
		testConfig(),
		httpapi.WithAuthService(fakeAuthService{authenticatedToken: "admin-session"}),
		httpapi.WithSettingsService(settingsService),
		httpapi.WithSourceService(sourceService),
		httpapi.WithProviderService(&fakeProviderService{}),
		httpapi.WithTemplateService(&fakePerformanceTemplateService{}),
		httpapi.WithRouteService(routeService),
		httpapi.WithPlanningWorker(planningWorker),
		httpapi.WithDeliveryWorker(deliveryWorker),
	)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/settings/performance-test", strings.NewReader(`{"message_count":6,"source_count":2,"payload_variant_count":3,"max_concurrency":5}`))
	req.Header.Set("Authorization", "Bearer admin-session")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected performance test to return 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if sourceService.createCalls != 2 {
		t.Fatalf("expected two generated sources, got %d", sourceService.createCalls)
	}
	if routeService.createCalls != 2 {
		t.Fatalf("expected one route flow per generated source, got %d", routeService.createCalls)
	}
	if routeService.simulateCalls != 0 {
		t.Fatalf("expected performance test not to use route simulation, got %d calls", routeService.simulateCalls)
	}
	if planningWorker.processCalls < 30 || deliveryWorker.processCalls < 30 {
		t.Fatalf("expected planning and delivery worker to drain every generated payload, got planning=%d delivery=%d", planningWorker.processCalls, deliveryWorker.processCalls)
	}
	if sourceService.ingestCalls != 30 {
		t.Fatalf("expected source ingest for every generated payload, got %d", sourceService.ingestCalls)
	}
	if len(settingsService.performanceTestInput.Observations) != 30 {
		t.Fatalf("expected observed sample metrics to reach settings service, got %+v", settingsService.performanceTestInput)
	}
	if settingsService.performanceTestInput.SourceCount != 2 || settingsService.performanceTestInput.PayloadVariantCount != 3 {
		t.Fatalf("expected benchmark shape to reach settings service, got %+v", settingsService.performanceTestInput)
	}
	if settingsService.performanceTestInput.MaxConcurrency != 5 {
		t.Fatalf("expected max concurrency to reach settings service, got %+v", settingsService.performanceTestInput)
	}
	if len(settingsService.performanceTestInput.ConcurrencyCandidates) != 0 {
		t.Fatalf("expected UI max concurrency mode to omit manual candidates, got %+v", settingsService.performanceTestInput)
	}
	if got := sourceService.ingestInput.Headers.Get("Authorization"); !strings.HasPrefix(got, "Bearer testtoken") {
		t.Fatalf("expected generated source token to be used for ingest, got %q", got)
	}
	if sourceService.cleanupRuntimeDataCalls != 2 {
		t.Fatalf("expected runtime data cleanup before deleting generated sources, got %d", sourceService.cleanupRuntimeDataCalls)
	}
	if !strings.Contains(rec.Body.String(), "success_rate") || !strings.Contains(rec.Body.String(), "stage_results") {
		t.Fatalf("expected enriched performance result response, got %s", rec.Body.String())
	}
}

func TestSettingsPerformanceTestPausesRuntimeWorkersWhileDraining(t *testing.T) {
	settingsService := &fakeSettingsService{}
	pauseController := &fakeRuntimeWorkerPauseController{}
	planningWorker := &fakePlanningWorker{processed: 1, pauseController: pauseController}
	deliveryWorker := &fakePlanningWorker{processed: 1, pauseController: pauseController}
	handler := httpapi.NewHandler(
		testConfig(),
		httpapi.WithAuthService(fakeAuthService{authenticatedToken: "admin-session"}),
		httpapi.WithSettingsService(settingsService),
		httpapi.WithSourceService(&fakeSourceService{}),
		httpapi.WithProviderService(&fakeProviderService{}),
		httpapi.WithTemplateService(&fakePerformanceTemplateService{}),
		httpapi.WithRouteService(&fakeRouteService{}),
		httpapi.WithPlanningWorker(planningWorker),
		httpapi.WithDeliveryWorker(deliveryWorker),
		httpapi.WithRuntimeWorkerPauseController(pauseController),
	)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/settings/performance-test", strings.NewReader(`{"source_count":1,"payload_variant_count":1,"concurrency_start":2,"concurrency_end":2}`))
	req.Header.Set("Authorization", "Bearer admin-session")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected performance test to return 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if pauseController.pauseCalls != 1 || pauseController.releaseCalls != 1 || pauseController.active() {
		t.Fatalf("expected runtime worker pause to be acquired and released once, got pause=%d release=%d active=%v",
			pauseController.pauseCalls,
			pauseController.releaseCalls,
			pauseController.active(),
		)
	}
	if !planningWorker.sawPauseActive || !deliveryWorker.sawPauseActive {
		t.Fatalf("expected performance drains to run while runtime workers are paused, got planning=%v delivery=%v",
			planningWorker.sawPauseActive,
			deliveryWorker.sawPauseActive,
		)
	}
}

func TestSettingsPerformanceTestRunsEveryConcurrencyLevelWithRuntimeDiagnostics(t *testing.T) {
	settingsService := &fakeSettingsService{}
	sourceService := &fakeSourceService{
		runtimeStatsResults: []source.RuntimeStats{
			{
				DBPoolWaitCount:        1,
				DBPoolWaitDurationMS:   5,
				DBPoolAcquiredConns:    2,
				DBPoolTotalConns:       4,
				PostgresMaxConnections: 100,
				PostgresBlocksRead:     20,
				PostgresBlocksHit:      100,
				PostgresTempBytes:      64,
			},
			{
				DBPoolWaitCount:        4,
				DBPoolWaitDurationMS:   17,
				DBPoolAcquiredConns:    3,
				DBPoolTotalConns:       5,
				PostgresMaxConnections: 100,
				PostgresBlocksRead:     35,
				PostgresBlocksHit:      140,
				PostgresTempBytes:      128,
			},
		},
	}
	templateService := &fakePerformanceTemplateService{}
	routeService := &fakeRouteService{}
	planningWorker := &fakePlanningWorker{processed: 1}
	deliveryWorker := &fakePlanningWorker{processed: 1}
	monitoringService := &fakeMonitoringService{
		queueSnapshots: []monitoring.QueueSnapshot{
			{Summary: monitoring.QueueSummary{RoutePlanPending: 2, SendMessagePending: 3}},
			{Summary: monitoring.QueueSummary{RoutePlanPending: 4, SendMessagePending: 5}},
		},
	}
	handler := httpapi.NewHandler(
		testConfig(),
		httpapi.WithAuthService(fakeAuthService{authenticatedToken: "admin-session"}),
		httpapi.WithSettingsService(settingsService),
		httpapi.WithSourceService(sourceService),
		httpapi.WithProviderService(&fakeProviderService{}),
		httpapi.WithTemplateService(templateService),
		httpapi.WithRouteService(routeService),
		httpapi.WithPlanningWorker(planningWorker),
		httpapi.WithDeliveryWorker(deliveryWorker),
		httpapi.WithMonitoringService(monitoringService),
	)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/settings/performance-test", strings.NewReader(`{"message_count":2,"source_count":1,"payload_variant_count":2,"concurrency_start":1,"concurrency_end":3}`))
	req.Header.Set("Authorization", "Bearer admin-session")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected performance test to return 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if routeService.simulateCalls != 0 || planningWorker.processCalls < 6 || deliveryWorker.processCalls < 6 || sourceService.ingestCalls != 6 || templateService.previewCalls != 6 {
		t.Fatalf("expected two samples per requested concurrency and drained workers, got simulate=%d planning=%d delivery=%d ingest=%d preview=%d",
			routeService.simulateCalls,
			planningWorker.processCalls,
			deliveryWorker.processCalls,
			sourceService.ingestCalls,
			templateService.previewCalls,
		)
	}
	if settingsService.performanceTestInput.ConcurrencyStart != 1 || settingsService.performanceTestInput.ConcurrencyEnd != 3 {
		t.Fatalf("expected concurrency endpoints to reach settings service, got %+v", settingsService.performanceTestInput)
	}
	countByConcurrency := map[int]int{}
	for _, observation := range settingsService.performanceTestInput.Observations {
		countByConcurrency[observation.Concurrency]++
		if observation.TemplateRenderDurationMS <= 0 {
			t.Fatalf("expected template render duration to be measured, got %+v", observation)
		}
		if observation.ConcurrencyRunDurationMS <= 0 {
			t.Fatalf("expected concurrency wall-clock duration to be measured, got %+v", observation)
		}
	}
	if countByConcurrency[1] != 2 || countByConcurrency[2] != 2 || countByConcurrency[3] != 2 {
		t.Fatalf("expected two observations for each requested concurrency, got %+v", countByConcurrency)
	}
	diagnostics := settingsService.performanceTestInput.Diagnostics
	if diagnostics.DBPoolWaitCountDelta != 3 ||
		diagnostics.DBPoolWaitDurationDeltaMS != 12 ||
		diagnostics.QueueBacklogBefore != 5 ||
		diagnostics.QueueBacklogAfter != 9 ||
		diagnostics.PostgresMaxConnections != 100 ||
		diagnostics.PostgresBlocksReadDelta != 15 ||
		diagnostics.PostgresBlocksHitDelta != 40 ||
		diagnostics.PostgresTempBytesDelta != 64 ||
		diagnostics.CPUCount <= 0 ||
		diagnostics.GoMaxProcs <= 0 ||
		diagnostics.GoroutinesBefore <= 0 ||
		diagnostics.GoroutinesAfter <= 0 {
		t.Fatalf("expected PostgreSQL, queue and goroutine diagnostics, got %+v", diagnostics)
	}
	if len(settingsService.performanceTestInput.ConcurrencyDiagnostics) != 3 {
		t.Fatalf("expected diagnostics for each concurrency bucket, got %+v", settingsService.performanceTestInput.ConcurrencyDiagnostics)
	}
}

func TestSettingsPerformanceTestUsesSystemWorkerPoolByDefault(t *testing.T) {
	settingsService := &fakeSettingsService{
		intValues: map[string]int{
			settings.KeyRuntimeDeliveryConcurrency: 8,
		},
	}
	planningWorker := &fakePlanningWorker{processed: 1, processHold: 20 * time.Millisecond}
	deliveryWorker := &fakePlanningWorker{processed: 1, processHold: 20 * time.Millisecond}
	handler := httpapi.NewHandler(
		testConfig(),
		httpapi.WithAuthService(fakeAuthService{authenticatedToken: "admin-session"}),
		httpapi.WithSettingsService(settingsService),
		httpapi.WithSourceService(&fakeSourceService{}),
		httpapi.WithProviderService(&fakeProviderService{}),
		httpapi.WithTemplateService(&fakePerformanceTemplateService{}),
		httpapi.WithRouteService(&fakeRouteService{}),
		httpapi.WithPlanningWorker(planningWorker),
		httpapi.WithDeliveryWorker(deliveryWorker),
	)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/settings/performance-test", strings.NewReader(`{"source_count":1,"payload_variant_count":1,"concurrency_start":50,"concurrency_end":50}`))
	req.Header.Set("Authorization", "Bearer admin-session")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected performance test to return 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if planningWorker.maxInflight > 20 {
		t.Fatalf("expected planning worker pool to cap concurrent processing at 20, got %d", planningWorker.maxInflight)
	}
	if deliveryWorker.maxInflight > 8 {
		t.Fatalf("expected delivery worker pool to follow current system setting 8, got %d", deliveryWorker.maxInflight)
	}
	if deliveryWorker.processCalls < 50 {
		t.Fatalf("expected delivery worker to drain generated send jobs, got %d", deliveryWorker.processCalls)
	}
	if !containsInt(planningWorker.limits, 64) || !containsInt(deliveryWorker.limits, 64) {
		t.Fatalf("expected performance drains to claim jobs in batches of 64, got planning=%+v delivery=%+v", planningWorker.limits, deliveryWorker.limits)
	}
}

func TestSettingsPerformanceTestCanFollowTestConcurrencyForWorkers(t *testing.T) {
	settingsService := &fakeSettingsService{
		intValues: map[string]int{
			settings.KeyRuntimeDeliveryConcurrency: 8,
		},
	}
	planningWorker := &fakePlanningWorker{processed: 1, processHold: 20 * time.Millisecond}
	deliveryWorker := &fakePlanningWorker{processed: 1, processHold: 20 * time.Millisecond}
	handler := httpapi.NewHandler(
		testConfig(),
		httpapi.WithAuthService(fakeAuthService{authenticatedToken: "admin-session"}),
		httpapi.WithSettingsService(settingsService),
		httpapi.WithSourceService(&fakeSourceService{}),
		httpapi.WithProviderService(&fakeProviderService{}),
		httpapi.WithTemplateService(&fakePerformanceTemplateService{}),
		httpapi.WithRouteService(&fakeRouteService{}),
		httpapi.WithPlanningWorker(planningWorker),
		httpapi.WithDeliveryWorker(deliveryWorker),
	)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/settings/performance-test", strings.NewReader(`{"source_count":1,"payload_variant_count":1,"concurrency_start":50,"concurrency_end":50,"worker_mode":"concurrency"}`))
	req.Header.Set("Authorization", "Bearer admin-session")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected performance test to return 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if planningWorker.maxInflight <= 20 {
		t.Fatalf("expected planning worker pool to follow test concurrency, got max inflight %d", planningWorker.maxInflight)
	}
	if deliveryWorker.maxInflight > 1 {
		t.Fatalf("expected delivery drain to stay capped by batch demand, got max inflight %d", deliveryWorker.maxInflight)
	}
	if settingsService.performanceTestInput.WorkerMode != settings.PerformanceWorkerModeConcurrency {
		t.Fatalf("expected worker mode to reach settings service, got %+v", settingsService.performanceTestInput)
	}
}

func TestSettingsPerformanceTestCapsDeliveryDrainWorkersByBatchDemand(t *testing.T) {
	settingsService := &fakeSettingsService{
		intValues: map[string]int{
			settings.KeyRuntimeDeliveryConcurrency: 200,
		},
	}
	planningWorker := &fakePlanningWorker{processed: 64, processHold: 20 * time.Millisecond}
	deliveryWorker := &fakePlanningWorker{processed: 64, processHold: 20 * time.Millisecond}
	handler := httpapi.NewHandler(
		testConfig(),
		httpapi.WithAuthService(fakeAuthService{authenticatedToken: "admin-session"}),
		httpapi.WithSettingsService(settingsService),
		httpapi.WithSourceService(&fakeSourceService{}),
		httpapi.WithProviderService(&fakeProviderService{}),
		httpapi.WithTemplateService(&fakePerformanceTemplateService{}),
		httpapi.WithRouteService(&fakeRouteService{}),
		httpapi.WithPlanningWorker(planningWorker),
		httpapi.WithDeliveryWorker(deliveryWorker),
	)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/settings/performance-test", strings.NewReader(`{"source_count":1,"payload_variant_count":1,"concurrency_start":500,"concurrency_end":500}`))
	req.Header.Set("Authorization", "Bearer admin-session")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected performance test to return 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if deliveryWorker.maxInflight > 8 {
		t.Fatalf("expected delivery drain to cap 500 messages to 8 batch workers, got %d", deliveryWorker.maxInflight)
	}
	if deliveryWorker.processCalls > 16 {
		t.Fatalf("expected delivery drain not to issue excessive empty claims, got %d calls", deliveryWorker.processCalls)
	}
	if settingsService.performanceTestInput.Observations[0].WorkerCount > 20 {
		t.Fatalf("expected observed worker count to use effective drain workers, got %+v", settingsService.performanceTestInput.Observations[0])
	}
}

func TestSettingsPerformanceTestRunCanBePolledUntilComplete(t *testing.T) {
	settingsService := &fakeSettingsService{}
	handler := httpapi.NewHandler(
		testConfig(),
		httpapi.WithAuthService(fakeAuthService{authenticatedToken: "admin-session"}),
		httpapi.WithSettingsService(settingsService),
		httpapi.WithSourceService(&fakeSourceService{}),
		httpapi.WithProviderService(&fakeProviderService{}),
		httpapi.WithTemplateService(&fakePerformanceTemplateService{}),
		httpapi.WithRouteService(&fakeRouteService{}),
		httpapi.WithMonitoringService(&fakeMonitoringService{}),
	)

	startReq := httptest.NewRequest(http.MethodPost, "/api/v1/settings/performance-test/runs", strings.NewReader(`{"message_count":1,"source_count":1,"payload_variant_count":1,"concurrency_start":1,"concurrency_end":2}`))
	startReq.Header.Set("Authorization", "Bearer admin-session")
	startRec := httptest.NewRecorder()
	handler.ServeHTTP(startRec, startReq)

	if startRec.Code != http.StatusAccepted {
		t.Fatalf("expected performance test run to return 202, got %d body=%s", startRec.Code, startRec.Body.String())
	}
	var startBody struct {
		Run struct {
			ID               string `json:"id"`
			Status           string `json:"status"`
			TotalConcurrency int    `json:"total_concurrency"`
		} `json:"run"`
	}
	if err := json.Unmarshal(startRec.Body.Bytes(), &startBody); err != nil {
		t.Fatalf("decode start response: %v", err)
	}
	if startBody.Run.ID == "" || startBody.Run.TotalConcurrency != 2 {
		t.Fatalf("expected run id and two concurrency buckets, got %+v", startBody.Run)
	}

	var pollBody struct {
		Run struct {
			ID                   string                         `json:"id"`
			Status               string                         `json:"status"`
			ProgressPercent      int                            `json:"progress_percent"`
			CompletedConcurrency int                            `json:"completed_concurrency"`
			TotalConcurrency     int                            `json:"total_concurrency"`
			Result               settings.PerformanceTestResult `json:"result"`
		} `json:"run"`
	}
	for attempts := 0; attempts < 20; attempts++ {
		pollReq := httptest.NewRequest(http.MethodGet, "/api/v1/settings/performance-test/runs/"+startBody.Run.ID, nil)
		pollReq.Header.Set("Authorization", "Bearer admin-session")
		pollRec := httptest.NewRecorder()
		handler.ServeHTTP(pollRec, pollReq)
		if pollRec.Code != http.StatusOK {
			t.Fatalf("expected poll to return 200, got %d body=%s", pollRec.Code, pollRec.Body.String())
		}
		if err := json.Unmarshal(pollRec.Body.Bytes(), &pollBody); err != nil {
			t.Fatalf("decode poll response: %v", err)
		}
		if pollBody.Run.Status == "completed" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if pollBody.Run.Status != "completed" {
		t.Fatalf("expected run to complete, got %+v", pollBody.Run)
	}
	if pollBody.Run.ProgressPercent != 100 ||
		pollBody.Run.CompletedConcurrency != 2 ||
		pollBody.Run.Result.AcceptedCount != 2 {
		t.Fatalf("expected completed progress and result, got %+v", pollBody.Run)
	}
}

func TestSettingsPerformanceTestRunCanBeCancelled(t *testing.T) {
	settingsService := &fakeSettingsService{}
	processStarted := make(chan struct{})
	processBlock := make(chan struct{})
	planningWorker := &fakePlanningWorker{
		processed:       1,
		processStarted:  processStarted,
		processBlock:    processBlock,
		processBlockAll: true,
	}
	deliveryWorker := &fakePlanningWorker{processed: 1}
	handler := httpapi.NewHandler(
		testConfig(),
		httpapi.WithAuthService(fakeAuthService{authenticatedToken: "admin-session"}),
		httpapi.WithSettingsService(settingsService),
		httpapi.WithSourceService(&fakeSourceService{}),
		httpapi.WithProviderService(&fakeProviderService{}),
		httpapi.WithTemplateService(&fakePerformanceTemplateService{}),
		httpapi.WithRouteService(&fakeRouteService{}),
		httpapi.WithPlanningWorker(planningWorker),
		httpapi.WithDeliveryWorker(deliveryWorker),
		httpapi.WithMonitoringService(&fakeMonitoringService{}),
	)

	startReq := httptest.NewRequest(http.MethodPost, "/api/v1/settings/performance-test/runs", strings.NewReader(`{"source_count":1,"payload_variant_count":1,"concurrency_start":2,"concurrency_end":2}`))
	startReq.Header.Set("Authorization", "Bearer admin-session")
	startRec := httptest.NewRecorder()
	handler.ServeHTTP(startRec, startReq)

	if startRec.Code != http.StatusAccepted {
		t.Fatalf("expected performance test run to return 202, got %d body=%s", startRec.Code, startRec.Body.String())
	}
	var startBody struct {
		Run struct {
			ID string `json:"id"`
		} `json:"run"`
	}
	if err := json.Unmarshal(startRec.Body.Bytes(), &startBody); err != nil {
		t.Fatalf("decode start response: %v", err)
	}
	select {
	case <-processStarted:
	case <-time.After(time.Second):
		t.Fatal("expected asynchronous performance test to enter planning worker")
	}

	runningReq := httptest.NewRequest(http.MethodGet, "/api/v1/settings/performance-test/runs/"+startBody.Run.ID, nil)
	runningReq.Header.Set("Authorization", "Bearer admin-session")
	runningRec := httptest.NewRecorder()
	handler.ServeHTTP(runningRec, runningReq)
	if runningRec.Code != http.StatusOK {
		t.Fatalf("expected running poll to return 200, got %d body=%s", runningRec.Code, runningRec.Body.String())
	}
	var runningBody struct {
		Run struct {
			Status             string `json:"status"`
			CurrentConcurrency int    `json:"current_concurrency"`
			ProgressPercent    int    `json:"progress_percent"`
		} `json:"run"`
	}
	if err := json.Unmarshal(runningRec.Body.Bytes(), &runningBody); err != nil {
		t.Fatalf("decode running poll response: %v", err)
	}
	if runningBody.Run.Status != "running" ||
		runningBody.Run.CurrentConcurrency != 2 ||
		runningBody.Run.ProgressPercent <= 0 {
		t.Fatalf("expected blocked run to report in-bucket progress, got %+v", runningBody.Run)
	}

	cancelReq := httptest.NewRequest(http.MethodPost, "/api/v1/settings/performance-test/runs/"+startBody.Run.ID+"/cancel", nil)
	cancelReq.Header.Set("Authorization", "Bearer admin-session")
	cancelRec := httptest.NewRecorder()
	handler.ServeHTTP(cancelRec, cancelReq)
	if cancelRec.Code != http.StatusOK {
		t.Fatalf("expected cancel to return 200, got %d body=%s", cancelRec.Code, cancelRec.Body.String())
	}
	var cancelBody struct {
		Run struct {
			Status          string `json:"status"`
			ProgressPercent int    `json:"progress_percent"`
			Error           string `json:"error"`
		} `json:"run"`
	}
	if err := json.Unmarshal(cancelRec.Body.Bytes(), &cancelBody); err != nil {
		t.Fatalf("decode cancel response: %v", err)
	}
	if cancelBody.Run.Status != "cancelled" || cancelBody.Run.ProgressPercent != 100 {
		t.Fatalf("expected cancelled run response, got %+v", cancelBody.Run)
	}

	close(processBlock)
	time.Sleep(20 * time.Millisecond)

	pollReq := httptest.NewRequest(http.MethodGet, "/api/v1/settings/performance-test/runs/"+startBody.Run.ID, nil)
	pollReq.Header.Set("Authorization", "Bearer admin-session")
	pollRec := httptest.NewRecorder()
	handler.ServeHTTP(pollRec, pollReq)
	if pollRec.Code != http.StatusOK {
		t.Fatalf("expected poll to return 200, got %d body=%s", pollRec.Code, pollRec.Body.String())
	}
	var pollBody struct {
		Run struct {
			Status string `json:"status"`
		} `json:"run"`
	}
	if err := json.Unmarshal(pollRec.Body.Bytes(), &pollBody); err != nil {
		t.Fatalf("decode poll response: %v", err)
	}
	if pollBody.Run.Status != "cancelled" {
		t.Fatalf("expected cancelled status to survive worker shutdown, got %+v", pollBody.Run)
	}
	if settingsService.performanceTestCalls != 0 {
		t.Fatalf("expected cancelled test not to produce final performance result, got %d calls", settingsService.performanceTestCalls)
	}
}

func TestSettingsPerformanceTestUsesDeliveryStatusAfterWorkerDrainError(t *testing.T) {
	settingsService := &fakeSettingsService{}
	sourceService := &fakeSourceService{
		ingestResult: source.IngestResult{TraceID: "trace-delivered", Status: "accepted", Message: "accepted"},
		deliveryStatuses: map[string]bool{
			"trace-delivered": true,
		},
	}
	handler := httpapi.NewHandler(
		testConfig(),
		httpapi.WithAuthService(fakeAuthService{authenticatedToken: "admin-session"}),
		httpapi.WithSettingsService(settingsService),
		httpapi.WithSourceService(sourceService),
		httpapi.WithProviderService(&fakeProviderService{}),
		httpapi.WithTemplateService(&fakePerformanceTemplateService{}),
		httpapi.WithRouteService(&fakeRouteService{}),
		httpapi.WithPlanningWorker(&fakePlanningWorker{processed: 1}),
		httpapi.WithDeliveryWorker(&fakePlanningWorker{processed: 1, err: errors.New("worker drain reported stale completion")}),
	)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/settings/performance-test", strings.NewReader(`{"source_count":1,"payload_variant_count":1,"concurrency_start":1,"concurrency_end":1}`))
	req.Header.Set("Authorization", "Bearer admin-session")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected performance test to return 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if len(settingsService.performanceTestInput.Observations) != 1 {
		t.Fatalf("expected one observation, got %+v", settingsService.performanceTestInput.Observations)
	}
	if !settingsService.performanceTestInput.Observations[0].Success {
		t.Fatalf("expected delivered trace to remain successful despite drain worker error, got %+v", settingsService.performanceTestInput.Observations[0])
	}
}

func TestSettingsPerformanceTestStartsDeliveryDrainAfterPlanningDrain(t *testing.T) {
	settingsService := &fakeSettingsService{}
	sourceService := &fakeSourceService{
		ingestResult: source.IngestResult{TraceID: "trace-delivery-after-planning", Status: "accepted", Message: "accepted"},
		deliveryStatuses: map[string]bool{
			"trace-delivery-after-planning": false,
		},
	}
	planningStarted := make(chan struct{})
	planningRelease := make(chan struct{})
	deliveryStarted := make(chan struct{})
	planningWorker := &fakePlanningWorker{
		processed:      1,
		processStarted: planningStarted,
		processBlock:   planningRelease,
	}
	deliveryWorker := &fakePlanningWorker{
		processed:      1,
		processStarted: deliveryStarted,
	}
	handler := httpapi.NewHandler(
		testConfig(),
		httpapi.WithAuthService(fakeAuthService{authenticatedToken: "admin-session"}),
		httpapi.WithSettingsService(settingsService),
		httpapi.WithSourceService(sourceService),
		httpapi.WithProviderService(&fakeProviderService{}),
		httpapi.WithTemplateService(&fakePerformanceTemplateService{}),
		httpapi.WithRouteService(&fakeRouteService{}),
		httpapi.WithPlanningWorker(planningWorker),
		httpapi.WithDeliveryWorker(deliveryWorker),
	)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/settings/performance-test", strings.NewReader(`{"source_count":1,"payload_variant_count":1,"concurrency_start":1,"concurrency_end":1}`))
	req.Header.Set("Authorization", "Bearer admin-session")
	rec := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		handler.ServeHTTP(rec, req)
		close(done)
	}()

	select {
	case <-planningStarted:
	case <-time.After(time.Second):
		t.Fatal("expected planning drain to start")
	}
	select {
	case <-deliveryStarted:
		t.Fatal("delivery drain started before planning drain completed")
	case <-time.After(50 * time.Millisecond):
	}
	close(planningRelease)

	select {
	case <-deliveryStarted:
	case <-time.After(time.Second):
		t.Fatal("expected delivery drain to start after planning drain completes")
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("expected performance test request to finish")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected performance test to return 200, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestSettingsPerformanceTestDoesNotWaitForOwnDrainCountWhenRuntimeWorkersDelivered(t *testing.T) {
	settingsService := &fakeSettingsService{}
	sourceService := &fakeSourceService{
		ingestResult: source.IngestResult{TraceID: "trace-delivered", Status: "accepted", Message: "accepted"},
		deliveryStatuses: map[string]bool{
			"trace-delivered": true,
		},
	}
	handler := httpapi.NewHandler(
		testConfig(),
		httpapi.WithAuthService(fakeAuthService{authenticatedToken: "admin-session"}),
		httpapi.WithSettingsService(settingsService),
		httpapi.WithSourceService(sourceService),
		httpapi.WithProviderService(&fakeProviderService{}),
		httpapi.WithTemplateService(&fakePerformanceTemplateService{}),
		httpapi.WithRouteService(&fakeRouteService{}),
		httpapi.WithPlanningWorker(&fakePlanningWorker{returnZero: true}),
		httpapi.WithDeliveryWorker(&fakePlanningWorker{returnZero: true}),
	)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/settings/performance-test", strings.NewReader(`{"source_count":1,"payload_variant_count":1,"concurrency_start":1,"concurrency_end":1}`))
	req.Header.Set("Authorization", "Bearer admin-session")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected performance test to return 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if len(settingsService.performanceTestInput.Observations) != 1 {
		t.Fatalf("expected one observation, got %+v", settingsService.performanceTestInput.Observations)
	}
	observation := settingsService.performanceTestInput.Observations[0]
	if !observation.Success {
		t.Fatalf("expected delivered trace to be successful even when perf drain processed 0 jobs, got %+v", observation)
	}
	if observation.RouteDurationMS >= 100000 || observation.ReceiveDurationMS >= 100000 {
		t.Fatalf("expected performance test not to wait for drain timeout, got route=%d receive=%d", observation.RouteDurationMS, observation.ReceiveDurationMS)
	}
}

func TestSettingsPerformanceTestDoesNotDrainPostgresWorkersInJetStreamMode(t *testing.T) {
	settingsService := &fakeSettingsService{}
	sourceService := &fakeSourceService{
		ingestResult: source.IngestResult{TraceID: "trace-jetstream-delivered", Status: "accepted", Message: "accepted"},
		deliveryStatusSteps: []map[string]bool{
			{"trace-jetstream-delivered": false},
			{"trace-jetstream-delivered": true},
		},
	}
	planningWorker := &fakePlanningWorker{err: errors.New("postgres planning drain should not run")}
	deliveryWorker := &fakePlanningWorker{err: errors.New("postgres delivery drain should not run")}
	cfg := testConfig()
	cfg.Queue.Backend = "jetstream"
	handler := httpapi.NewHandler(
		cfg,
		httpapi.WithAuthService(fakeAuthService{authenticatedToken: "admin-session"}),
		httpapi.WithSettingsService(settingsService),
		httpapi.WithSourceService(sourceService),
		httpapi.WithProviderService(&fakeProviderService{}),
		httpapi.WithTemplateService(&fakePerformanceTemplateService{}),
		httpapi.WithRouteService(&fakeRouteService{}),
		httpapi.WithPlanningWorker(planningWorker),
		httpapi.WithDeliveryWorker(deliveryWorker),
	)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/settings/performance-test", strings.NewReader(`{"source_count":1,"payload_variant_count":1,"concurrency_start":1,"concurrency_end":1}`))
	req.Header.Set("Authorization", "Bearer admin-session")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected performance test to return 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if planningWorker.processCalls != 0 || deliveryWorker.processCalls != 0 {
		t.Fatalf("expected JetStream performance test not to drain PostgreSQL workers, planning=%d delivery=%d", planningWorker.processCalls, deliveryWorker.processCalls)
	}
	if len(settingsService.performanceTestInput.Observations) != 1 || !settingsService.performanceTestInput.Observations[0].Success {
		t.Fatalf("expected JetStream performance test to use delivered status, got %+v", settingsService.performanceTestInput.Observations)
	}
}

func TestSettingsPerformanceTestCleansPreviousFixedArtifactsBeforeRun(t *testing.T) {
	settingsService := &fakeSettingsService{}
	sourceService := &fakeSourceService{
		listResult: []source.Source{{ID: "old-source", Code: "test0001", Name: "性能测试01"}},
	}
	providerService := &fakeProviderService{
		listResult: []provider.Channel{{ID: "old-channel", Name: "性能测试模拟上级"}},
	}
	templateService := &fakePerformanceTemplateService{
		listResult: []msgtemplate.Template{{ID: "old-template", Name: "性能测试模板"}},
	}
	routeService := &fakeRouteService{
		listResult: []route.Flow{{ID: "old-flow", Name: "性能测试路由01"}},
	}
	handler := httpapi.NewHandler(
		testConfig(),
		httpapi.WithAuthService(fakeAuthService{authenticatedToken: "admin-session"}),
		httpapi.WithSettingsService(settingsService),
		httpapi.WithSourceService(sourceService),
		httpapi.WithProviderService(providerService),
		httpapi.WithTemplateService(templateService),
		httpapi.WithRouteService(routeService),
	)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/settings/performance-test", strings.NewReader(`{"message_count":1}`))
	req.Header.Set("Authorization", "Bearer admin-session")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected performance test to return 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !containsString(routeService.deletedFlowIDs, "old-flow") ||
		!containsString(templateService.deletedTemplateIDs, "old-template") ||
		!containsString(providerService.deletedChannelIDs, "old-channel") ||
		!containsString(sourceService.deletedSourceIDs, "old-source") {
		t.Fatalf("expected previous fixed artifacts to be deleted before run, got flows=%+v templates=%+v channels=%+v sources=%+v",
			routeService.deletedFlowIDs,
			templateService.deletedTemplateIDs,
			providerService.deletedChannelIDs,
			sourceService.deletedSourceIDs,
		)
	}
}

func TestSettingsPerformanceTestBuildsFixedBenchmarkResources(t *testing.T) {
	settingsService := &fakeSettingsService{}
	sourceService := &fakeSourceService{}
	providerService := &fakeProviderService{}
	templateService := &fakePerformanceTemplateService{}
	routeService := &fakeRouteService{}
	handler := httpapi.NewHandler(
		testConfig(),
		httpapi.WithAuthService(fakeAuthService{authenticatedToken: "admin-session"}),
		httpapi.WithSettingsService(settingsService),
		httpapi.WithSourceService(sourceService),
		httpapi.WithProviderService(providerService),
		httpapi.WithTemplateService(templateService),
		httpapi.WithRouteService(routeService),
	)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/settings/performance-test", strings.NewReader(`{"message_count":5,"source_count":5,"payload_variant_count":5,"auth_mode":"token_and_hmac"}`))
	req.Header.Set("Authorization", "Bearer admin-session")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected performance test to return 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if providerService.createChannelCalls != 1 || providerService.createChannelInput.Name != "性能测试模拟上级" {
		t.Fatalf("expected one fixed fake upstream channel, got calls=%d input=%+v", providerService.createChannelCalls, providerService.createChannelInput)
	}
	if templateService.createCalls != 1 || templateService.publishCalls != 1 {
		t.Fatalf("expected one shared template, got create=%d publish=%d", templateService.createCalls, templateService.publishCalls)
	}
	if templateService.createInputs[0].Name != "性能测试模板" {
		t.Fatalf("expected fixed template name, got %+v", templateService.createInputs[0])
	}
	templateBody := templateService.publishInput.TemplateBody
	if !strings.Contains(templateBody, "payload.route_key") ||
		!strings.Contains(templateBody, `"body":{`) ||
		!strings.Contains(templateBody, "default('【模版】性能测试')") ||
		!strings.Contains(templateBody, "default('【模版】性能测试消息')") {
		t.Fatalf("expected shared webhook template to wrap body and use route_key with marked defaults, got %s", templateBody)
	}
	if len(sourceService.createInputs) != 5 {
		t.Fatalf("expected five generated sources, got %+v", sourceService.createInputs)
	}
	for index, input := range sourceService.createInputs {
		expectedCode := "test000" + string(rune('1'+index))
		expectedName := "性能测试0" + string(rune('1'+index))
		if input.Code != expectedCode || input.Name != expectedName {
			t.Fatalf("expected fixed source %d as %s/%s, got %+v", index, expectedCode, expectedName, input)
		}
		if input.AuthMode != source.AuthModeTokenAndHMAC || input.AuthToken == "" || input.HMACSecret == "" {
			t.Fatalf("expected token+hmac source credentials, got %+v", input)
		}
	}
	if routeService.createCalls != 5 || len(routeService.saveRulesInputs) != 5 {
		t.Fatalf("expected one route flow per source, got create=%d save=%d", routeService.createCalls, len(routeService.saveRulesInputs))
	}
	firstRuleSet := routeService.saveRulesInputs[0]
	if len(firstRuleSet.Rules) != 6 {
		t.Fatalf("expected five route_key rules plus default, got %+v", firstRuleSet.Rules)
	}
	for index, rule := range firstRuleSet.Rules[:5] {
		var condition map[string]any
		if err := json.Unmarshal(rule.ConditionTree, &condition); err != nil {
			t.Fatalf("decode condition: %v", err)
		}
		expectedValue := string(rune('a' + index))
		if condition["path"] != "payload.route_key" || condition["value"] != expectedValue {
			t.Fatalf("expected route_key=%s rule, got %+v", expectedValue, condition)
		}
	}
	if routeService.simulateCalls != 0 || len(sourceService.ingestInputs) != 80 {
		t.Fatalf("expected one real ingest and no route simulation per payload, got simulate=%d ingest=%d", routeService.simulateCalls, len(sourceService.ingestInputs))
	}
	var payload map[string]any
	if err := json.Unmarshal(sourceService.ingestInputs[0].Body, &payload); err != nil {
		t.Fatalf("decode ingested payload: %v", err)
	}
	content, _ := payload["content"].(string)
	if payload["route_key"] != "a" || payload["title"] != "性能测试" || payload["timestamp"] == "" {
		t.Fatalf("expected fixed benchmark payload fields, got %+v", payload)
	}
	randomPart := strings.TrimPrefix(content, "这是一条性能测试消息，随机消息-")
	if !regexp.MustCompile(`^[0-9A-Za-z]{12}$`).MatchString(randomPart) {
		t.Fatalf("expected 12 character alphanumeric random suffix, got %q", content)
	}
	headers := sourceService.ingestInputs[0].Headers
	if !strings.HasPrefix(headers.Get("Authorization"), "Bearer ") ||
		headers.Get("X-MGP-Timestamp") == "" ||
		headers.Get("X-MGP-Nonce") == "" ||
		!strings.HasPrefix(headers.Get("X-MGP-Signature"), "sha256=") {
		t.Fatalf("expected token+hmac ingest headers, got %+v", headers)
	}
	if settingsService.performanceTestInput.GeneratedSourceCode != "test0001" ||
		settingsService.performanceTestInput.GeneratedChannelName != "性能测试模拟上级" {
		t.Fatalf("expected fixed generated resource names in result input, got %+v", settingsService.performanceTestInput)
	}
}

func TestSettingsPerformanceTestRateLimitsRepeatedRuns(t *testing.T) {
	settingsService := &fakeSettingsService{}
	handler := httpapi.NewHandler(
		testConfig(),
		httpapi.WithAuthService(fakeAuthService{authenticatedToken: "admin-session"}),
		httpapi.WithSettingsService(settingsService),
		httpapi.WithSourceService(&fakeSourceService{}),
		httpapi.WithProviderService(&fakeProviderService{}),
		httpapi.WithTemplateService(&fakePerformanceTemplateService{}),
		httpapi.WithRouteService(&fakeRouteService{}),
	)

	firstReq := httptest.NewRequest(http.MethodPost, "/api/v1/settings/performance-test", strings.NewReader(`{"message_count":10}`))
	firstReq.Header.Set("Authorization", "Bearer admin-session")
	firstRec := httptest.NewRecorder()
	handler.ServeHTTP(firstRec, firstReq)
	if firstRec.Code != http.StatusOK {
		t.Fatalf("expected first performance test to return 200, got %d body=%s", firstRec.Code, firstRec.Body.String())
	}

	secondReq := httptest.NewRequest(http.MethodPost, "/api/v1/settings/performance-test", strings.NewReader(`{"message_count":10}`))
	secondReq.Header.Set("Authorization", "Bearer admin-session")
	secondRec := httptest.NewRecorder()
	handler.ServeHTTP(secondRec, secondReq)
	if secondRec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected repeated performance test to return 429, got %d body=%s", secondRec.Code, secondRec.Body.String())
	}
	if got := responseErrorCode(t, secondRec); got != "MGP-SETTINGS-002" {
		t.Fatalf("expected MGP-SETTINGS-002, got %q", got)
	}
	if settingsService.performanceTestCalls != 1 {
		t.Fatalf("expected repeated run not to reach performance service, got %d calls", settingsService.performanceTestCalls)
	}
}

func TestSettingsPerformanceTestCleansGeneratedResourcesWhenRuleSaveFails(t *testing.T) {
	settingsService := &fakeSettingsService{}
	sourceService := &fakeSourceService{}
	providerService := &fakeProviderService{}
	templateService := &fakePerformanceTemplateService{}
	routeService := &fakeRouteService{saveRulesErr: route.ErrInvalidInput}
	handler := httpapi.NewHandler(
		testConfig(),
		httpapi.WithAuthService(fakeAuthService{authenticatedToken: "admin-session"}),
		httpapi.WithSettingsService(settingsService),
		httpapi.WithSourceService(sourceService),
		httpapi.WithProviderService(providerService),
		httpapi.WithTemplateService(templateService),
		httpapi.WithRouteService(routeService),
	)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/settings/performance-test", strings.NewReader(`{"message_count":10}`))
	req.Header.Set("Authorization", "Bearer admin-session")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected resource generation failure to return 500, got %d body=%s", rec.Code, rec.Body.String())
	}
	if got := responseErrorCode(t, rec); got != "MGP-SETTINGS-999" {
		t.Fatalf("expected MGP-SETTINGS-999, got %q", got)
	}
	if settingsService.performanceTestCalls != 0 {
		t.Fatalf("expected performance runner not to start after resource generation failure, got %d calls", settingsService.performanceTestCalls)
	}
	if routeService.deleteCalls != 1 || templateService.deleteCalls != 1 || providerService.deleteChannelCalls != 1 || sourceService.deleteCalls != 1 {
		t.Fatalf("expected generated resources to be cleaned, got route=%d template=%d channel=%d source=%d",
			routeService.deleteCalls,
			templateService.deleteCalls,
			providerService.deleteChannelCalls,
			sourceService.deleteCalls,
		)
	}
}

func TestSettingsPerformanceTestCleansGeneratedResourcesWhenRunnerFails(t *testing.T) {
	settingsService := &fakeSettingsService{performanceTestErr: errors.New("runner failed")}
	sourceService := &fakeSourceService{}
	providerService := &fakeProviderService{}
	templateService := &fakePerformanceTemplateService{}
	routeService := &fakeRouteService{}
	handler := httpapi.NewHandler(
		testConfig(),
		httpapi.WithAuthService(fakeAuthService{authenticatedToken: "admin-session"}),
		httpapi.WithSettingsService(settingsService),
		httpapi.WithSourceService(sourceService),
		httpapi.WithProviderService(providerService),
		httpapi.WithTemplateService(templateService),
		httpapi.WithRouteService(routeService),
	)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/settings/performance-test", strings.NewReader(`{"message_count":10}`))
	req.Header.Set("Authorization", "Bearer admin-session")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected runner failure to return 500, got %d body=%s", rec.Code, rec.Body.String())
	}
	if got := responseErrorCode(t, rec); got != "MGP-SETTINGS-999" {
		t.Fatalf("expected MGP-SETTINGS-999, got %q", got)
	}
	if routeService.deleteCalls != 1 || templateService.deleteCalls != 1 || providerService.deleteChannelCalls != 1 || sourceService.deleteCalls != 1 {
		t.Fatalf("expected generated resources to be cleaned after runner failure, got route=%d template=%d channel=%d source=%d",
			routeService.deleteCalls,
			templateService.deleteCalls,
			providerService.deleteChannelCalls,
			sourceService.deleteCalls,
		)
	}
}

type fakePerformanceTemplateService struct {
	mu sync.Mutex

	createCalls  int
	publishCalls int
	previewCalls int
	deleteCalls  int

	createInputs       []msgtemplate.TemplateInput
	publishInput       msgtemplate.VersionInput
	listResult         []msgtemplate.Template
	deletedTemplateIDs []string

	createErr  error
	publishErr error
	deleteErr  error
}

func (f *fakePerformanceTemplateService) ListTemplates(context.Context) ([]msgtemplate.Template, error) {
	return f.listResult, nil
}

func (f *fakePerformanceTemplateService) CreateTemplate(_ context.Context, input msgtemplate.TemplateInput) (msgtemplate.Template, error) {
	f.createCalls++
	f.createInputs = append(f.createInputs, input)
	if f.createErr != nil {
		return msgtemplate.Template{}, f.createErr
	}
	return msgtemplate.Template{ID: "template-1", Name: input.Name, SourceID: input.SourceID, Enabled: input.Enabled}, nil
}

func (f *fakePerformanceTemplateService) GetTemplate(context.Context, string) (msgtemplate.Template, error) {
	return msgtemplate.Template{}, nil
}

func (f *fakePerformanceTemplateService) UpdateTemplate(context.Context, string, msgtemplate.TemplateInput) (msgtemplate.Template, error) {
	return msgtemplate.Template{}, nil
}

func (f *fakePerformanceTemplateService) DeleteTemplate(_ context.Context, id string) error {
	f.deleteCalls++
	f.deletedTemplateIDs = append(f.deletedTemplateIDs, id)
	if f.deleteErr != nil {
		return f.deleteErr
	}
	return nil
}

func (f *fakePerformanceTemplateService) ListTemplateVersions(context.Context, string) ([]msgtemplate.TemplateVersion, error) {
	return nil, nil
}

func (f *fakePerformanceTemplateService) Parse(msgtemplate.VersionInput) (msgtemplate.ValidationResult, error) {
	return msgtemplate.ValidationResult{}, nil
}

func (f *fakePerformanceTemplateService) Preview(msgtemplate.VersionInput) (msgtemplate.ValidationResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.previewCalls++
	return msgtemplate.ValidationResult{}, nil
}

func (f *fakePerformanceTemplateService) Validate(msgtemplate.VersionInput) msgtemplate.ValidationResult {
	return msgtemplate.ValidationResult{Status: "valid"}
}

func (f *fakePerformanceTemplateService) Publish(_ context.Context, templateID string, input msgtemplate.VersionInput) (msgtemplate.TemplateVersion, error) {
	f.publishCalls++
	f.publishInput = input
	if f.publishErr != nil {
		return msgtemplate.TemplateVersion{}, f.publishErr
	}
	return msgtemplate.TemplateVersion{ID: "template-version-1", TemplateID: templateID, VersionNo: 1}, nil
}

func (f *fakePerformanceTemplateService) RestoreTemplateVersion(context.Context, string, string) (msgtemplate.TemplateVersion, error) {
	return msgtemplate.TemplateVersion{}, nil
}

type fakePlanningWorker struct {
	mu sync.Mutex

	processed   int
	err         error
	processHold time.Duration
	returnZero  bool

	processStarted     chan struct{}
	processBlock       chan struct{}
	processStartedOnce sync.Once
	processBlockAll    bool
	pauseController    *fakeRuntimeWorkerPauseController

	processCalls   int
	inflight       int
	maxInflight    int
	limits         []int
	sawPauseActive bool
}

func (f *fakePlanningWorker) ProcessBatch(ctx context.Context, limit int) (int, error) {
	f.mu.Lock()
	f.processCalls++
	if f.pauseController != nil && f.pauseController.active() {
		f.sawPauseActive = true
	}
	f.inflight++
	if f.inflight > f.maxInflight {
		f.maxInflight = f.inflight
	}
	f.limits = append(f.limits, limit)
	shouldBlock := f.processBlock != nil && (f.processBlockAll || f.processCalls == 1)
	if f.processStarted != nil {
		f.processStartedOnce.Do(func() {
			close(f.processStarted)
		})
	}
	f.mu.Unlock()
	if shouldBlock {
		select {
		case <-ctx.Done():
			f.finishProcessCall()
			return 0, ctx.Err()
		case <-f.processBlock:
		}
	}
	if f.processHold > 0 {
		timer := time.NewTimer(f.processHold)
		select {
		case <-ctx.Done():
			timer.Stop()
			f.finishProcessCall()
			return 0, ctx.Err()
		case <-timer.C:
		}
	}
	defer f.finishProcessCall()
	if f.err != nil {
		return 0, f.err
	}
	if f.returnZero {
		return 0, nil
	}
	if f.processed <= 0 {
		return 1, nil
	}
	return f.processed, nil
}

type fakeRuntimeWorkerPauseController struct {
	mu           sync.Mutex
	activeCount  int
	pauseCalls   int
	releaseCalls int
}

func (f *fakeRuntimeWorkerPauseController) PauseWorkers() func() {
	f.mu.Lock()
	f.activeCount++
	f.pauseCalls++
	f.mu.Unlock()

	var once sync.Once
	return func() {
		once.Do(func() {
			f.mu.Lock()
			if f.activeCount > 0 {
				f.activeCount--
			}
			f.releaseCalls++
			f.mu.Unlock()
		})
	}
}

func (f *fakeRuntimeWorkerPauseController) active() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.activeCount > 0
}

func (f *fakePlanningWorker) finishProcessCall() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.inflight--
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func containsInt(values []int, expected int) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
