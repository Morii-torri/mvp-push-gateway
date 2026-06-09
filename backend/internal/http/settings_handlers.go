package httpapi

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	dbrepo "mvp-push-gateway/backend/internal/db"
	deliverypkg "mvp-push-gateway/backend/internal/delivery"
	"mvp-push-gateway/backend/internal/monitoring"
	"mvp-push-gateway/backend/internal/perftiming"
	planningpkg "mvp-push-gateway/backend/internal/planning"
	"mvp-push-gateway/backend/internal/provider"
	"mvp-push-gateway/backend/internal/route"
	"mvp-push-gateway/backend/internal/settings"
	"mvp-push-gateway/backend/internal/source"
	msgtemplate "mvp-push-gateway/backend/internal/template"
)

type settingsResponse struct {
	Settings []settingResponse `json:"settings"`
}

type settingBody struct {
	Setting settingResponse `json:"setting"`
}

type performanceTestBody struct {
	Result settings.PerformanceTestResult `json:"result"`
}

type performanceTestRunBody struct {
	Run performanceTestRunResponse `json:"run"`
}

type performanceTestRunResponse struct {
	ID                   string                          `json:"id"`
	Status               string                          `json:"status"`
	ProgressPercent      int                             `json:"progress_percent"`
	CurrentConcurrency   int                             `json:"current_concurrency"`
	CompletedConcurrency int                             `json:"completed_concurrency"`
	TotalConcurrency     int                             `json:"total_concurrency"`
	Result               *settings.PerformanceTestResult `json:"result,omitempty"`
	Error                string                          `json:"error,omitempty"`
	CreatedAt            string                          `json:"created_at"`
	UpdatedAt            string                          `json:"updated_at"`
}

type settingResponse struct {
	Key         string          `json:"key"`
	Value       json.RawMessage `json:"value"`
	Description string          `json:"description"`
	Category    string          `json:"category"`
	CreatedAt   string          `json:"created_at"`
	UpdatedAt   string          `json:"updated_at"`
}

const (
	performanceTestPlanningWorkers  = 20
	performanceTestDeliveryWorkers  = 20
	performanceTestWorkerBatchSize  = 64
	performanceTestWorkerIdleSleep  = 5 * time.Millisecond
	performanceTestWorkerDrainLimit = 2 * time.Minute
)

type performanceTestRunStore struct {
	mu   sync.Mutex
	runs map[string]performanceTestRunState
}

type performanceTestUpstreamStore struct {
	mu       sync.Mutex
	received map[string]time.Time
}

type performanceTestRunState struct {
	id                   string
	status               string
	progressPercent      int
	currentConcurrency   int
	completedConcurrency int
	totalConcurrency     int
	result               *settings.PerformanceTestResult
	err                  string
	cancel               context.CancelFunc
	createdAt            time.Time
	updatedAt            time.Time
}

func newPerformanceTestRunStore() *performanceTestRunStore {
	return &performanceTestRunStore{runs: make(map[string]performanceTestRunState)}
}

func newPerformanceTestUpstreamStore() *performanceTestUpstreamStore {
	return &performanceTestUpstreamStore{received: make(map[string]time.Time)}
}

func (s *performanceTestUpstreamStore) reset() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.received = make(map[string]time.Time)
}

func (s *performanceTestUpstreamStore) record(sampleID string, at time.Time) {
	if s == nil {
		return
	}
	sampleID = strings.TrimSpace(sampleID)
	if sampleID == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.received == nil {
		s.received = make(map[string]time.Time)
	}
	if _, exists := s.received[sampleID]; !exists {
		s.received[sampleID] = at
	}
}

func (s *performanceTestUpstreamStore) receivedAt(sampleID string) (time.Time, bool) {
	if s == nil {
		return time.Time{}, false
	}
	sampleID = strings.TrimSpace(sampleID)
	if sampleID == "" {
		return time.Time{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	at, ok := s.received[sampleID]
	return at, ok
}

func (s *performanceTestRunStore) create(totalConcurrency int) performanceTestRunState {
	now := time.Now()
	state := performanceTestRunState{
		id:               "perf-" + randomPerformanceText(16),
		status:           "running",
		totalConcurrency: maxPerformanceDuration(1, totalConcurrency),
		createdAt:        now,
		updatedAt:        now,
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runs[state.id] = state
	return state
}

func (s *performanceTestRunStore) update(id string, mutate func(*performanceTestRunState)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, ok := s.runs[id]
	if !ok {
		return
	}
	if state.status == "cancelled" {
		return
	}
	mutate(&state)
	state.updatedAt = time.Now()
	s.runs[id] = state
}

func (s *performanceTestRunStore) setCancel(id string, cancel context.CancelFunc) bool {
	s.mu.Lock()
	state, ok := s.runs[id]
	if !ok {
		s.mu.Unlock()
		return false
	}
	if state.status == "cancelled" {
		s.mu.Unlock()
		cancel()
		return false
	}
	state.cancel = cancel
	state.updatedAt = time.Now()
	s.runs[id] = state
	s.mu.Unlock()
	return true
}

func (s *performanceTestRunStore) cancel(id string) (performanceTestRunState, bool) {
	var cancel context.CancelFunc
	s.mu.Lock()
	state, ok := s.runs[id]
	if !ok {
		s.mu.Unlock()
		return performanceTestRunState{}, false
	}
	if state.status == "running" {
		cancel = state.cancel
		state.status = "cancelled"
		state.progressPercent = 100
		state.err = "性能测试已取消"
		state.updatedAt = time.Now()
		s.runs[id] = state
	}
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	return state, true
}

func (s *performanceTestRunStore) get(id string) (performanceTestRunState, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, ok := s.runs[id]
	return state, ok
}

func (h *Handler) settingsPerformanceTestHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	if !h.requireSettingsService(w) {
		return
	}
	adminUser, ok := h.authenticateRequest(w, r)
	if !ok {
		return
	}
	var request settings.PerformanceTestInput
	if r.Body != nil && r.ContentLength != 0 {
		if err := decodeJSON(r, &request); err != nil {
			writeAPIError(w, http.StatusBadRequest, "MGP-REQ-001", "请求 JSON 不合法")
			return
		}
	}
	result, err := h.executePerformanceTest(r.Context(), request, nil)
	if err != nil {
		status, code, message := settingsErrorStatus(err)
		writeAPIError(w, status, code, message)
		return
	}
	response := performanceTestBody{Result: result}
	h.recordAudit(r, adminUser, "run", "performance_test", result.UpdatedSettingKey, request, response)
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) settingsPerformanceTestRunsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	if !h.requireSettingsService(w) {
		return
	}
	if _, ok := h.authenticateRequest(w, r); !ok {
		return
	}
	var request settings.PerformanceTestInput
	if r.Body != nil && r.ContentLength != 0 {
		if err := decodeJSON(r, &request); err != nil {
			writeAPIError(w, http.StatusBadRequest, "MGP-REQ-001", "请求 JSON 不合法")
			return
		}
	}
	candidates := settings.PerformanceConcurrencyCandidates(request)
	run := h.perfRuns.create(len(candidates))
	go h.executePerformanceTestRun(run.id, request)
	writeJSON(w, http.StatusAccepted, performanceTestRunBody{Run: performanceTestRunResponseFromState(run)})
}

func (h *Handler) settingsPerformanceTestRunDetailHandler(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, h.cfg.Server.APIPrefix+"/settings/performance-test/runs/"))
	if strings.HasSuffix(path, "/cancel") {
		id := strings.TrimSpace(strings.TrimSuffix(strings.TrimSuffix(path, "/cancel"), "/"))
		h.settingsPerformanceTestRunCancelHandler(w, r, id)
		return
	}
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	if !h.requireSettingsService(w) {
		return
	}
	if _, ok := h.authenticateRequest(w, r); !ok {
		return
	}
	if path == "" {
		writeAPIError(w, http.StatusNotFound, "MGP-SETTINGS-004", "性能测试任务不存在")
		return
	}
	run, ok := h.perfRuns.get(path)
	if !ok {
		writeAPIError(w, http.StatusNotFound, "MGP-SETTINGS-004", "性能测试任务不存在")
		return
	}
	writeJSON(w, http.StatusOK, performanceTestRunBody{Run: performanceTestRunResponseFromState(run)})
}

func (h *Handler) settingsPerformanceTestRunCancelHandler(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	if !h.requireSettingsService(w) {
		return
	}
	if _, ok := h.authenticateRequest(w, r); !ok {
		return
	}
	if id == "" {
		writeAPIError(w, http.StatusNotFound, "MGP-SETTINGS-004", "性能测试任务不存在")
		return
	}
	run, ok := h.perfRuns.cancel(id)
	if !ok {
		writeAPIError(w, http.StatusNotFound, "MGP-SETTINGS-004", "性能测试任务不存在")
		return
	}
	writeJSON(w, http.StatusOK, performanceTestRunBody{Run: performanceTestRunResponseFromState(run)})
}

func (h *Handler) settingsPerformanceTestFakeUpstreamHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	if !isLoopbackRemoteAddr(r.RemoteAddr) {
		writeAPIError(w, http.StatusForbidden, "MGP-SETTINGS-005", "性能测试回调仅允许本机访问")
		return
	}
	if r.Body != nil && h.perfUpstream != nil {
		raw, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if sampleID := performanceSampleIDFromRequest(raw); sampleID != "" {
			h.perfUpstream.record(sampleID, time.Now())
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func isLoopbackRemoteAddr(remoteAddr string) bool {
	host, _, err := net.SplitHostPort(strings.TrimSpace(remoteAddr))
	if err != nil {
		host = strings.TrimSpace(remoteAddr)
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func (h *Handler) executePerformanceTestRun(runID string, request settings.PerformanceTestInput) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	if !h.perfRuns.setCancel(runID, cancel) {
		return
	}
	defer cancel()
	result, err := h.executePerformanceTest(ctx, request, func(update performanceTestProgressUpdate) {
		h.perfRuns.update(runID, func(state *performanceTestRunState) {
			state.status = "running"
			state.currentConcurrency = update.currentConcurrency
			state.completedConcurrency = update.completedConcurrency
			state.totalConcurrency = update.totalConcurrency
			state.progressPercent = update.progressPercent
			state.result = &update.result
		})
	})
	if err != nil {
		if errors.Is(err, context.Canceled) {
			h.perfRuns.update(runID, func(state *performanceTestRunState) {
				state.status = "cancelled"
				state.progressPercent = 100
				state.err = "性能测试已取消"
			})
			return
		}
		h.perfRuns.update(runID, func(state *performanceTestRunState) {
			state.status = "failed"
			state.progressPercent = 100
			state.err = err.Error()
		})
		return
	}
	h.perfRuns.update(runID, func(state *performanceTestRunState) {
		state.status = "completed"
		state.progressPercent = 100
		state.completedConcurrency = state.totalConcurrency
		state.currentConcurrency = result.RecommendedGlobalConcurrency
		state.result = &result
	})
}

type performanceTestProgressUpdate struct {
	currentConcurrency   int
	completedConcurrency int
	totalConcurrency     int
	progressPercent      int
	result               settings.PerformanceTestResult
}

func performanceTestRunResponseFromState(state performanceTestRunState) performanceTestRunResponse {
	return performanceTestRunResponse{
		ID:                   state.id,
		Status:               state.status,
		ProgressPercent:      state.progressPercent,
		CurrentConcurrency:   state.currentConcurrency,
		CompletedConcurrency: state.completedConcurrency,
		TotalConcurrency:     state.totalConcurrency,
		Result:               state.result,
		Error:                state.err,
		CreatedAt:            state.createdAt.Format(time.RFC3339),
		UpdatedAt:            state.updatedAt.Format(time.RFC3339),
	}
}

type performanceTestResources struct {
	Sources       []performanceTestSourceResource
	ChannelID     string
	ChannelName   string
	SourceCode    string
	RouteName     string
	TemplateID    string
	TemplateInput msgtemplate.VersionInput
}

type performanceTestSourceResource struct {
	SourceID    string
	SourceCode  string
	SourceToken string
	HMACSecret  string
	AuthMode    source.AuthMode
	RouteID     string
	RouteName   string
	TemplateID  string
}

func (h *Handler) createPerformanceTestResources(ctx context.Context, input settings.PerformanceTestInput) (performanceTestResources, error) {
	if h.sources == nil || h.providers == nil || h.templates == nil || h.routes == nil {
		return performanceTestResources{}, nil
	}
	h.cleanupExistingPerformanceTestArtifacts(ctx)
	channelName := "性能测试模拟上级"
	resources := performanceTestResources{
		ChannelName: channelName,
	}
	created := false
	defer func() {
		if created {
			h.cleanupPerformanceTestResources(ctx, resources)
		}
	}()

	channel, err := h.providers.CreateChannel(ctx, provider.CreateChannelInput{
		ProviderType:     provider.ProviderWebhook,
		Name:             channelName,
		Enabled:          true,
		SendConfig:       json.RawMessage(fmt.Sprintf(`{"method":"POST","url":%q,"recipient":{"location":"none"}}`, h.performanceTestFakeUpstreamURL())),
		RateLimitConfig:  json.RawMessage(`{"enabled":true,"qps":100000}`),
		ConcurrencyLimit: performanceTestMaxConcurrency(input),
		TimeoutMS:        1000,
		RetryPolicy:      json.RawMessage(`{"max_attempts":1,"delay_ms":100}`),
		DeadLetterPolicy: json.RawMessage(`{"policy":"retry_exhausted_or_upstream_error","retention_days":7,"replay":false}`),
	})
	if err != nil {
		return performanceTestResources{}, err
	}
	resources.ChannelID = channel.ID
	created = true
	sourceCount := normalizePerformanceSourceCount(input.SourceCount)
	authMode := normalizePerformanceSourceAuthMode(input.SourceAuthMode)
	for index := 0; index < sourceCount; index++ {
		sourceResource, err := h.createPerformanceTestSource(ctx, index, authMode)
		if err != nil {
			return performanceTestResources{}, err
		}
		resources.Sources = append(resources.Sources, sourceResource)
		if resources.SourceCode == "" {
			resources.SourceCode = sourceResource.SourceCode
			resources.RouteName = sourceResource.RouteName
		}
	}
	template, err := h.templates.CreateTemplate(ctx, msgtemplate.TemplateInput{
		Name:     "性能测试模板",
		SourceID: resources.Sources[0].SourceID,
		Enabled:  true,
	})
	if err != nil {
		return performanceTestResources{}, err
	}
	resources.Sources[0].TemplateID = template.ID
	resources.TemplateID = template.ID
	templateInput := msgtemplate.VersionInput{
		MessageType:        "json",
		TargetProviderType: string(provider.ProviderWebhook),
		TemplateBody:       `{"body":{"title":"{{ payload.title | default('【模版】性能测试') }}","content":"{{ payload.content | default('【模版】性能测试消息') }}","route_key":"{{ payload.route_key }}","sample_id":"{{ payload.sample_id }}","timestamp":"{{ payload.timestamp }}"}}`,
		MessageBodySchema:  json.RawMessage(`{"type":"object","required":["body"],"properties":{"body":{"type":"object"}}}`),
		SamplePayload:      json.RawMessage(`{"timestamp":"2026-06-05T00:00:00+08:00","route_key":"a","sample_id":"perf-sample-000000","title":"性能测试","content":"这是一条性能测试消息，随机消息-000000000000"}`),
	}
	resources.TemplateInput = templateInput
	version, err := h.templates.Publish(ctx, template.ID, templateInput)
	if err != nil {
		return performanceTestResources{}, err
	}
	routeKeys := performanceRouteKeys(normalizePerformancePayloadVariantCount(input.PayloadVariantCount))
	for index := range resources.Sources {
		if err := h.createPerformanceTestRoute(ctx, &resources.Sources[index], channel.ID, version.ID, routeKeys); err != nil {
			return performanceTestResources{}, err
		}
	}
	created = false
	return resources, nil
}

func (h *Handler) performanceTestFakeUpstreamURL() string {
	port := strings.TrimSpace(h.cfg.Server.Port)
	if port == "" {
		port = "8080"
	}
	return "http://127.0.0.1:" + port + h.cfg.Server.APIPrefix + "/settings/performance-test/fake-upstream"
}

func (h *Handler) createPerformanceTestSource(ctx context.Context, index int, authMode source.AuthMode) (performanceTestSourceResource, error) {
	sourceCode := fmt.Sprintf("test%04d", index+1)
	sourceToken := fmt.Sprintf("testtoken%04d", index+1)
	hmacSecret := fmt.Sprintf("testhmac%04d", index+1)
	routeName := fmt.Sprintf("性能测试路由%02d", index+1)
	resource := performanceTestSourceResource{
		SourceCode:  sourceCode,
		SourceToken: sourceToken,
		HMACSecret:  hmacSecret,
		AuthMode:    authMode,
		RouteName:   routeName,
	}
	created := false
	defer func() {
		if created {
			h.cleanupPerformanceTestResources(ctx, performanceTestResources{Sources: []performanceTestSourceResource{resource}})
		}
	}()
	createdSource, err := h.sources.CreateSource(ctx, source.CreateSourceInput{
		Code:            sourceCode,
		Name:            fmt.Sprintf("性能测试%02d", index+1),
		Enabled:         true,
		AuthMode:        authMode,
		AuthToken:       sourceToken,
		HMACSecret:      hmacSecret,
		CompatMode:      "standard",
		RateLimitConfig: json.RawMessage(`{"enabled":false}`),
	})
	if err != nil {
		return performanceTestSourceResource{}, err
	}
	resource.SourceID = createdSource.ID
	created = true
	created = false
	return resource, nil
}

func (h *Handler) createPerformanceTestRoute(ctx context.Context, resource *performanceTestSourceResource, channelID string, templateVersionID string, routeKeys []string) error {
	flow, err := h.routes.CreateFlow(ctx, route.CreateFlowInput{
		SourceID: resource.SourceID,
		Name:     resource.RouteName,
		Enabled:  true,
		Mode:     route.ModeTable,
	})
	if err != nil {
		return err
	}
	resource.RouteID = flow.ID
	rules := make([]route.RuleInput, 0, len(routeKeys)+1)
	for index, key := range routeKeys {
		rules = append(rules, performanceRouteRule(key, (index+1)*10, channelID, templateVersionID))
	}
	rules = append(rules, route.RuleInput{
		SortOrder:     (len(routeKeys) + 1) * 10,
		Name:          "性能测试默认命中",
		ConditionTree: json.RawMessage(`{"operator":"always"}`),
		Enabled:       true,
		Action: route.ActionInput{
			Targets: []route.ActionTargetInput{{
				ChannelID:         channelID,
				TemplateVersionID: templateVersionID,
				Enabled:           true,
			}},
			RecipientStrategy: json.RawMessage(`{"mode":"none"}`),
			SendDedupeConfig:  json.RawMessage(`{"enabled":false}`),
		},
	})
	if _, err := h.routes.SaveRules(ctx, flow.ID, route.SaveRulesInput{Rules: rules}); err != nil {
		return err
	}
	if _, err := h.routes.Publish(ctx, flow.ID, "性能测试自动发布"); err != nil {
		return err
	}
	return nil
}

func performanceRouteRule(routeKey string, sortOrder int, channelID string, templateVersionID string) route.RuleInput {
	condition, _ := json.Marshal(map[string]any{
		"operator": "equals",
		"path":     "payload.route_key",
		"value":    routeKey,
	})
	return route.RuleInput{
		SortOrder:     sortOrder,
		Name:          "性能测试 route_key=" + routeKey,
		ConditionTree: condition,
		Enabled:       true,
		Action: route.ActionInput{
			Targets: []route.ActionTargetInput{{
				ChannelID:         channelID,
				TemplateVersionID: templateVersionID,
				Enabled:           true,
			}},
			RecipientStrategy: json.RawMessage(`{"mode":"none"}`),
			SendDedupeConfig:  json.RawMessage(`{"enabled":false}`),
		},
	}
}

func (h *Handler) cleanupPerformanceTestResources(ctx context.Context, resources performanceTestResources) {
	for _, item := range resources.Sources {
		if item.SourceID != "" && h.sources != nil {
			_ = h.sources.DeleteSourceRuntimeData(ctx, item.SourceID)
		}
	}
	for _, item := range resources.Sources {
		if item.RouteID != "" && h.routes != nil {
			_ = h.routes.DeleteFlow(ctx, item.RouteID)
		}
	}
	if resources.TemplateID != "" && h.templates != nil {
		_ = h.templates.DeleteTemplate(ctx, resources.TemplateID)
	}
	for _, item := range resources.Sources {
		if item.TemplateID != "" && item.TemplateID != resources.TemplateID && h.templates != nil {
			_ = h.templates.DeleteTemplate(ctx, item.TemplateID)
		}
	}
	if resources.ChannelID != "" && h.providers != nil {
		_ = h.providers.DeleteChannel(ctx, resources.ChannelID)
	}
	for _, item := range resources.Sources {
		if item.SourceID != "" && h.sources != nil {
			_ = h.sources.DeleteSource(ctx, item.SourceID)
		}
	}
}

func (h *Handler) cleanupExistingPerformanceTestArtifacts(ctx context.Context) {
	if h.routes != nil {
		if flows, err := h.routes.ListFlows(ctx); err == nil {
			for _, flow := range flows {
				if strings.HasPrefix(flow.Name, "性能测试路由") {
					_ = h.routes.DeleteFlow(ctx, flow.ID)
				}
			}
		}
	}
	if h.templates != nil {
		if templates, err := h.templates.ListTemplates(ctx); err == nil {
			for _, item := range templates {
				if item.Name == "性能测试模板" {
					_ = h.templates.DeleteTemplate(ctx, item.ID)
				}
			}
		}
	}
	if h.providers != nil {
		if channels, err := h.providers.ListChannels(ctx); err == nil {
			for _, channel := range channels {
				if channel.Name == "性能测试模拟上级" {
					_ = h.providers.DeleteChannel(ctx, channel.ID)
				}
			}
		}
	}
	if h.sources != nil {
		if sources, err := h.sources.ListSources(ctx); err == nil {
			for _, item := range sources {
				if isPerformanceTestSource(item) {
					_ = h.sources.DeleteSourceRuntimeData(ctx, item.ID)
					_ = h.sources.DeleteSource(ctx, item.ID)
				}
			}
		}
	}
}

func (h *Handler) executePerformanceTest(
	ctx context.Context,
	request settings.PerformanceTestInput,
	progress func(performanceTestProgressUpdate),
) (settings.PerformanceTestResult, error) {
	releaseRuntimeWorkers := h.pauseRuntimeWorkersForPerformanceTest()
	defer releaseRuntimeWorkers()

	startSnapshot := h.performanceRuntimeSnapshot(ctx)
	prepared, err := h.createPerformanceTestResources(ctx, request)
	if err != nil {
		return settings.PerformanceTestResult{}, err
	}
	defer h.cleanupPerformanceTestResources(context.Background(), prepared)
	if h.perfUpstream != nil {
		h.perfUpstream.reset()
	}
	request.GeneratedSourceCode = prepared.SourceCode
	request.GeneratedRouteName = prepared.RouteName
	request.GeneratedChannelName = prepared.ChannelName
	observations, concurrencyDiagnostics, err := h.runPerformanceTestSamples(ctx, prepared, request, func(update performanceTestSampleProgress) {
		if progress == nil {
			return
		}
		partial := request
		partial.Observations = append([]settings.PerformanceTestObservation(nil), update.observations...)
		partial.ConcurrencyDiagnostics = append([]settings.PerformanceConcurrencyDiagnostics(nil), update.concurrencyDiagnostics...)
		partial.Diagnostics = performanceRuntimeDiagnostics(startSnapshot, h.performanceRuntimeSnapshot(ctx))
		result, err := h.settings.BuildPerformanceTestResult(partial)
		if err != nil {
			return
		}
		progress(performanceTestProgressUpdate{
			currentConcurrency:   update.currentConcurrency,
			completedConcurrency: update.completedConcurrency,
			totalConcurrency:     update.totalConcurrency,
			progressPercent:      update.progressPercent,
			result:               result,
		})
	})
	if err != nil {
		return settings.PerformanceTestResult{}, err
	}
	request.Observations = observations
	request.ConcurrencyDiagnostics = concurrencyDiagnostics
	endSnapshot := h.performanceRuntimeSnapshot(ctx)
	request.Diagnostics = performanceRuntimeDiagnostics(startSnapshot, endSnapshot)
	return h.settings.RunPerformanceTest(ctx, request)
}

func (h *Handler) pauseRuntimeWorkersForPerformanceTest() func() {
	if h == nil || h.workerPause == nil {
		return func() {}
	}
	release := h.workerPause.PauseWorkers()
	if release == nil {
		return func() {}
	}
	return release
}

type performanceTestSampleProgress struct {
	currentConcurrency     int
	completedConcurrency   int
	totalConcurrency       int
	progressPercent        int
	observations           []settings.PerformanceTestObservation
	concurrencyDiagnostics []settings.PerformanceConcurrencyDiagnostics
}

type performanceDeliveryWaitResult struct {
	statuses   map[string]bool
	found      bool
	durationMS int
	err        error
}

type performanceWorkerTimingRecorder struct {
	mu        sync.Mutex
	startedAt time.Time
	timings   map[string]map[string]int
	elapsed   map[string]map[string]int
	dbTimings map[string]map[string][]int
	globalDB  map[string][]int
}

func newPerformanceWorkerTimingRecorder(startedAt time.Time) *performanceWorkerTimingRecorder {
	return &performanceWorkerTimingRecorder{
		startedAt: startedAt,
		timings:   map[string]map[string]int{},
		elapsed:   map[string]map[string]int{},
		dbTimings: map[string]map[string][]int{},
		globalDB:  map[string][]int{},
	}
}

func (r *performanceWorkerTimingRecorder) RecordPlanningTiming(traceID string, stage planningpkg.TimingStage, duration time.Duration) {
	r.record(traceID, string(stage), duration)
}

func (r *performanceWorkerTimingRecorder) RecordDeliveryTiming(traceID string, stage deliverypkg.TimingStage, duration time.Duration) {
	r.record(traceID, string(stage), duration)
}

func (r *performanceWorkerTimingRecorder) RecordSQLTiming(traceID string, stage dbrepo.SQLTimingStage, duration time.Duration) {
	r.recordDB(traceID, string(stage), duration)
}

func (r *performanceWorkerTimingRecorder) RecordStageTiming(traceID string, stage string, duration time.Duration) {
	r.record(traceID, stage, duration)
}

func (r *performanceWorkerTimingRecorder) RecordDBStageTiming(traceID string, stage string, duration time.Duration) {
	r.recordDB(traceID, stage, duration)
}

func (r *performanceWorkerTimingRecorder) record(traceID string, stage string, duration time.Duration) {
	traceID = strings.TrimSpace(traceID)
	stage = strings.TrimSpace(stage)
	if traceID == "" || stage == "" {
		return
	}
	durationMS := maxPerformanceDuration(1, int(duration.Milliseconds()))
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.timings[traceID] == nil {
		r.timings[traceID] = map[string]int{}
	}
	r.timings[traceID][stage] += durationMS
	if !r.startedAt.IsZero() {
		if r.elapsed[traceID] == nil {
			r.elapsed[traceID] = map[string]int{}
		}
		r.elapsed[traceID][stage] = maxPerformanceDuration(1, int(time.Since(r.startedAt).Milliseconds()))
	}
}

func (r *performanceWorkerTimingRecorder) recordDB(traceID string, stage string, duration time.Duration) {
	traceID = strings.TrimSpace(traceID)
	stage = strings.TrimSpace(stage)
	if stage == "" {
		return
	}
	durationMS := maxPerformanceDuration(1, int(duration.Milliseconds()))
	r.mu.Lock()
	defer r.mu.Unlock()
	if traceID == "" {
		r.globalDB[stage] = append(r.globalDB[stage], durationMS)
		return
	}
	if r.dbTimings[traceID] == nil {
		r.dbTimings[traceID] = map[string][]int{}
	}
	r.dbTimings[traceID][stage] = append(r.dbTimings[traceID][stage], durationMS)
}

func (r *performanceWorkerTimingRecorder) apply(observations []settings.PerformanceTestObservation) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for index := range observations {
		traceID := strings.TrimSpace(observations[index].TraceID)
		if traceID == "" {
			continue
		}
		timings := r.timings[traceID]
		assignPositiveInt(&observations[index].PlanningClaimDurationMS, timings[string(planningpkg.TimingClaimJobs)])
		assignPositiveInt(&observations[index].RoutePlanLookupDurationMS, timings[string(planningpkg.TimingRoutePlan)])
		assignPositiveInt(&observations[index].RouteConditionDurationMS, timings[string(planningpkg.TimingRouteCondition)])
		assignPositiveInt(&observations[index].PlanningTemplateRenderDurationMS, timings[string(planningpkg.TimingTemplateRender)])
		assignPositiveInt(&observations[index].PlanningCompleteDurationMS, timings[string(planningpkg.TimingComplete)])
		assignPositiveInt(&observations[index].DeliveryClaimDurationMS, timings[string(deliverypkg.TimingClaimJobs)])
		assignPositiveInt(&observations[index].DeliveryDispatchDurationMS, timings[string(deliverypkg.TimingDispatchHTTP)])
		assignPositiveInt(&observations[index].DeliverySendDurationMS, timings[string(deliverypkg.TimingSendHTTP)])
		assignPositiveInt(&observations[index].DeliveryCompleteDurationMS, timings[string(deliverypkg.TimingComplete)])
		if dbTimings := r.dbTimings[traceID]; len(dbTimings) > 0 {
			observations[index].DBTimings = clonePerformanceDBTimings(dbTimings)
		}
	}
	if len(observations) > 0 && len(r.globalDB) > 0 {
		if observations[0].DBTimings == nil {
			observations[0].DBTimings = map[string][]int{}
		}
		for stage, durations := range r.globalDB {
			observations[0].DBTimings[stage] = append(observations[0].DBTimings[stage], durations...)
		}
	}
}

func assignPositiveInt(target *int, value int) {
	if target == nil || value <= 0 {
		return
	}
	*target = value
}

func (r *performanceWorkerTimingRecorder) elapsedDurationMS(traceID string, stage string) int {
	if r == nil {
		return 0
	}
	traceID = strings.TrimSpace(traceID)
	stage = strings.TrimSpace(stage)
	if traceID == "" || stage == "" {
		return 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.elapsed[traceID][stage]
}

func (r *performanceWorkerTimingRecorder) hasStageForAll(traceIDs []string, stage string) bool {
	stage = strings.TrimSpace(stage)
	if len(traceIDs) == 0 || stage == "" {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, traceID := range traceIDs {
		traceID = strings.TrimSpace(traceID)
		if traceID == "" {
			return false
		}
		if r.timings[traceID][stage] <= 0 {
			return false
		}
	}
	return true
}

func performancePlanningDuration(observation settings.PerformanceTestObservation) int {
	duration := observation.PlanningClaimDurationMS +
		observation.RoutePlanLookupDurationMS +
		observation.RouteConditionDurationMS +
		observation.PlanningTemplateRenderDurationMS +
		observation.PlanningCompleteDurationMS
	if duration <= 0 {
		return observation.RouteDurationMS
	}
	return duration
}

func clonePerformanceDBTimings(input map[string][]int) map[string][]int {
	if len(input) == 0 {
		return nil
	}
	output := make(map[string][]int, len(input))
	for key, values := range input {
		output[key] = append([]int(nil), values...)
	}
	return output
}

type performanceDeliveryStatusReader interface {
	PerformanceDeliveryStatuses(context.Context, []string) (map[string]bool, error)
}

type performanceDeliveryStatusDetailReader interface {
	PerformanceDeliveryStatusDetails(context.Context, []string) (map[string]source.PerformanceDeliveryStatus, error)
}

func (h *Handler) runPerformanceTestSamples(
	ctx context.Context,
	resources performanceTestResources,
	input settings.PerformanceTestInput,
	progress func(performanceTestSampleProgress),
) ([]settings.PerformanceTestObservation, []settings.PerformanceConcurrencyDiagnostics, error) {
	if len(resources.Sources) == 0 || h.routes == nil || h.sources == nil {
		return nil, nil, nil
	}
	candidates := settings.PerformanceConcurrencyCandidates(input)
	observations := make([]settings.PerformanceTestObservation, 0)
	concurrencyDiagnostics := make([]settings.PerformanceConcurrencyDiagnostics, 0, len(candidates))
	offset := 0
	totalConcurrency := maxPerformanceDuration(1, len(candidates))
	emitProgress := func(candidateIndex int, concurrency int, phase float64, bucket []settings.PerformanceTestObservation, extraDiagnostics []settings.PerformanceConcurrencyDiagnostics) {
		if progress == nil {
			return
		}
		if phase < 0 {
			phase = 0
		}
		if phase > 1 {
			phase = 1
		}
		completedConcurrency := candidateIndex
		if phase >= 1 {
			completedConcurrency = candidateIndex + 1
		}
		progressPercent := int((float64(candidateIndex) + phase) / float64(totalConcurrency) * 100)
		if progressPercent <= 0 && phase > 0 {
			progressPercent = 1
		}
		if progressPercent >= 100 && completedConcurrency < totalConcurrency {
			progressPercent = 99
		}
		observationSnapshot := make([]settings.PerformanceTestObservation, 0, len(observations)+len(bucket))
		observationSnapshot = append(observationSnapshot, observations...)
		observationSnapshot = append(observationSnapshot, bucket...)
		diagnosticSnapshot := make([]settings.PerformanceConcurrencyDiagnostics, 0, len(concurrencyDiagnostics)+len(extraDiagnostics))
		diagnosticSnapshot = append(diagnosticSnapshot, concurrencyDiagnostics...)
		diagnosticSnapshot = append(diagnosticSnapshot, extraDiagnostics...)
		progress(performanceTestSampleProgress{
			currentConcurrency:     concurrency,
			completedConcurrency:   completedConcurrency,
			totalConcurrency:       totalConcurrency,
			progressPercent:        progressPercent,
			observations:           observationSnapshot,
			concurrencyDiagnostics: diagnosticSnapshot,
		})
	}
	for candidateIndex, concurrency := range candidates {
		if err := ctx.Err(); err != nil {
			return observations, concurrencyDiagnostics, err
		}
		messageCount := settings.PerformanceMessageCountForConcurrency(input, concurrency)
		bucketStartSnapshot := h.performanceRuntimeSnapshot(ctx)
		emitProgress(candidateIndex, concurrency, 0.05, nil, nil)
		bucket, err := h.runPerformanceTestConcurrency(ctx, resources, input, concurrency, messageCount, offset, func(bucket []settings.PerformanceTestObservation, phase float64) {
			emitProgress(candidateIndex, concurrency, phase, bucket, nil)
		})
		bucketDiagnostics := performanceRuntimeDiagnostics(bucketStartSnapshot, h.performanceRuntimeSnapshot(ctx))
		observations = append(observations, bucket...)
		concurrencyDiagnostics = append(concurrencyDiagnostics, settings.PerformanceConcurrencyDiagnostics{
			Concurrency: concurrency,
			Diagnostics: bucketDiagnostics,
		})
		if err != nil {
			return observations, concurrencyDiagnostics, err
		}
		offset += messageCount
		emitProgress(candidateIndex, concurrency, 1, nil, nil)
	}
	return observations, concurrencyDiagnostics, ctx.Err()
}

func (h *Handler) runPerformanceTestConcurrency(ctx context.Context, resources performanceTestResources, input settings.PerformanceTestInput, concurrency int, messageCount int, offset int, progress func([]settings.PerformanceTestObservation, float64)) ([]settings.PerformanceTestObservation, error) {
	if concurrency <= 0 {
		concurrency = 1
	}
	if messageCount <= 0 {
		return nil, ctx.Err()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	planningWorkerCount, deliveryWorkerCount := h.performanceTestWorkerCounts(ctx, input, concurrency, messageCount)
	deliveryDrainWorkerCount := performanceTestBatchDemandWorkerCount(deliveryWorkerCount, messageCount, performanceTestWorkerBatchSize)
	workerCtx, cancelWorkerDrains := context.WithCancel(ctx)
	defer cancelWorkerDrains()
	startedAt := time.Now()
	timingRecorder := newPerformanceWorkerTimingRecorder(startedAt)
	unregisterTimingRecorder := perftiming.Register(timingRecorder)
	defer unregisterTimingRecorder()
	workerCtx = dbrepo.WithSQLTimingRecorder(workerCtx, timingRecorder)
	workerCtx = planningpkg.WithTimingRecorder(workerCtx, timingRecorder)
	workerCtx = deliverypkg.WithTimingRecorder(workerCtx, timingRecorder)
	sampleCtx := dbrepo.WithSQLTimingRecorder(ctx, timingRecorder)
	workerCount := minPerformanceInt(concurrency, messageCount)
	actualWorkerCount := maxPerformanceDuration(planningWorkerCount, deliveryDrainWorkerCount)
	jobs := make(chan int)
	results := make(chan settings.PerformanceTestObservation, messageCount)
	var wg sync.WaitGroup
	for worker := 0; worker < workerCount; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case index, ok := <-jobs:
					if !ok {
						return
					}
					results <- h.runPerformanceTestSample(sampleCtx, resources, input, offset+index)
				}
			}
		}()
	}
	var dispatchErr error
dispatch:
	for index := 0; index < messageCount; index++ {
		select {
		case <-ctx.Done():
			dispatchErr = ctx.Err()
			break dispatch
		case jobs <- index:
		}
	}
	close(jobs)
	wg.Wait()
	close(results)
	observations := make([]settings.PerformanceTestObservation, 0, messageCount)
	for observation := range results {
		observation.Concurrency = concurrency
		observations = append(observations, observation)
	}
	acceptedRunDurationMS := elapsedMilliseconds(startedAt)
	if progress != nil {
		progress(append([]settings.PerformanceTestObservation(nil), observations...), 0.5)
	}
	if h.performanceTestUsesJetStreamQueue() {
		observations, err := h.waitPerformanceTestJetStreamCompletion(ctx, observations, startedAt, acceptedRunDurationMS, actualWorkerCount, timingRecorder)
		if progress != nil {
			progress(append([]settings.PerformanceTestObservation(nil), observations...), 1)
		}
		if dispatchErr != nil {
			return observations, dispatchErr
		}
		if err != nil {
			return observations, err
		}
		return observations, ctx.Err()
	}

	statusCtx, cancelStatusWait := context.WithCancel(ctx)
	defer cancelStatusWait()
	statusWait := make(chan performanceDeliveryWaitResult, 1)
	go func() {
		statusWait <- h.waitPerformanceDeliverySuccess(statusCtx, observations, startedAt)
	}()
	planningDrain := h.startPerformanceWorkerDrain(workerCtx, h.planning, messageCount, planningWorkerCount)
	planningWait := make(chan performanceWorkerDrainResult, 1)
	go func() {
		planningWait <- planningDrain.wait(ctx)
	}()

	var planningResult performanceWorkerDrainResult
	var deliveryResult performanceWorkerDrainResult
	var deliveryStatuses map[string]bool
	var deliveryStatusFound bool
	deliveryAlreadyComplete := false
	select {
	case planningResult = <-planningWait:
	case delivered := <-statusWait:
		if delivered.found {
			deliveryAlreadyComplete = true
			deliveryStatuses = delivered.statuses
			deliveryStatusFound = true
			planningResult = performanceWorkerDrainResult{processed: messageCount, durationMS: delivered.durationMS}
			deliveryResult = performanceWorkerDrainResult{processed: messageCount, durationMS: delivered.durationMS}
			cancelWorkerDrains()
		} else {
			planningResult = performanceWorkerDrainResult{err: delivered.err}
		}
	}
	if planningResult.err != nil || planningResult.processed < messageCount {
		cancelWorkerDrains()
	}
	for index := range observations {
		observations[index].RouteDurationMS = planningResult.durationMS
		observations[index].WorkerCount = actualWorkerCount
	}
	if progress != nil {
		progress(append([]settings.PerformanceTestObservation(nil), observations...), 0.75)
	}
	var dispatchResult performanceDeliveryWaitResult
	if !deliveryAlreadyComplete {
		deliveryDrain := h.startPerformanceWorkerDrain(workerCtx, h.delivery, messageCount, deliveryDrainWorkerCount)
		deliveryWait := make(chan performanceWorkerDrainResult, 1)
		go func() {
			deliveryWait <- deliveryDrain.wait(ctx)
		}()
		dispatchCtx, cancelDispatchWait := context.WithCancel(ctx)
		dispatchWait := make(chan performanceDeliveryWaitResult, 1)
		go func() {
			dispatchWait <- h.waitPerformanceDeliveryDispatch(dispatchCtx, observations, timingRecorder, startedAt)
		}()
		select {
		case deliveryResult = <-deliveryWait:
		case delivered := <-statusWait:
			if delivered.found {
				deliveryStatuses = delivered.statuses
				deliveryStatusFound = true
				deliveryResult = performanceWorkerDrainResult{processed: messageCount, durationMS: delivered.durationMS}
				cancelWorkerDrains()
			} else {
				deliveryResult = performanceWorkerDrainResult{err: delivered.err}
			}
		}
		select {
		case dispatchResult = <-dispatchWait:
		case <-time.After(10 * time.Millisecond):
			dispatchResult = performanceDeliveryWaitResult{durationMS: deliveryResult.durationMS}
		}
		cancelDispatchWait()
	}
	cancelStatusWait()
	if !deliveryStatusFound {
		deliveryStatuses, deliveryStatusFound = h.performanceDeliveryStatuses(ctx, observations)
	}
	timingRecorder.apply(observations)
	runDurationMS := elapsedMilliseconds(startedAt)
	drainErr := errors.Join(planningResult.err, deliveryResult.err)
	for index := range observations {
		sampleStartedAt := observations[index].StartedAt
		if sampleStartedAt.IsZero() {
			sampleStartedAt = startedAt
		}
		traceID := strings.TrimSpace(observations[index].TraceID)
		dispatchElapsedMS := h.performanceFakeUpstreamElapsedMS(observations[index].SampleID, sampleStartedAt)
		if dispatchElapsedMS <= 0 {
			dispatchElapsedMS = timingRecorder.elapsedDurationMS(traceID, string(deliverypkg.TimingDispatchHTTP))
		}
		if dispatchElapsedMS <= 0 {
			dispatchElapsedMS = dispatchResult.durationMS
		}
		if dispatchElapsedMS <= 0 {
			dispatchElapsedMS = deliveryResult.durationMS
		}
		observations[index].ConcurrencyRunDurationMS = runDurationMS
		observations[index].AcceptedRunDurationMS = acceptedRunDurationMS
		observations[index].DispatchRunDurationMS = firstPositivePerformanceInt(dispatchResult.durationMS, dispatchElapsedMS, deliveryResult.durationMS)
		observations[index].DispatchDurationMS = maxPerformanceDuration(1, observations[index].DeliveryDispatchDurationMS)
		observations[index].ReceiveDurationMS = deliveryResult.durationMS
		if deliveryStatusFound {
			if observations[index].TraceID == "" || !deliveryStatuses[observations[index].TraceID] {
				observations[index].Success = false
			}
		} else if drainErr != nil {
			observations[index].Success = false
		}
		observations[index].EndToEndDurationMS = maxPerformanceDuration(1, dispatchElapsedMS)
		observations[index].CompletionEndToEndDurationMS = maxPerformanceDuration(observations[index].EndToEndDurationMS, runDurationMS)
	}
	if dispatchErr != nil {
		return observations, dispatchErr
	}
	if err := ctx.Err(); err != nil {
		return observations, err
	}
	return observations, nil
}

func (h *Handler) waitPerformanceTestJetStreamCompletion(
	ctx context.Context,
	observations []settings.PerformanceTestObservation,
	startedAt time.Time,
	acceptedRunDurationMS int,
	actualWorkerCount int,
	timingRecorder *performanceWorkerTimingRecorder,
) ([]settings.PerformanceTestObservation, error) {
	statusCtx, cancelStatusWait := context.WithCancel(ctx)
	statusWait := make(chan performanceDeliveryWaitResult, 1)
	go func() {
		statusWait <- h.waitPerformanceDeliverySuccess(statusCtx, observations, startedAt)
	}()
	upstreamCtx, cancelUpstreamWait := context.WithCancel(ctx)
	upstreamWait := make(chan performanceDeliveryWaitResult, 1)
	go func() {
		upstreamWait <- h.waitPerformanceFakeUpstreamDispatch(upstreamCtx, observations, startedAt)
	}()
	dispatchCtx, cancelDispatchWait := context.WithCancel(ctx)
	dispatchWait := make(chan performanceDeliveryWaitResult, 1)
	go func() {
		dispatchWait <- h.waitPerformanceDeliveryDispatch(dispatchCtx, observations, timingRecorder, startedAt)
	}()
	var dispatchResult performanceDeliveryWaitResult
	var delivered performanceDeliveryWaitResult
	deliveredAlreadyFound := false
	select {
	case dispatchResult = <-upstreamWait:
	case dispatchResult = <-dispatchWait:
	case delivered = <-statusWait:
		deliveredAlreadyFound = true
		if delivered.found {
			select {
			case dispatchResult = <-upstreamWait:
			case dispatchResult = <-dispatchWait:
			case <-time.After(10 * time.Millisecond):
				dispatchResult = performanceDeliveryWaitResult{found: true, durationMS: delivered.durationMS}
			case <-ctx.Done():
				dispatchResult = performanceDeliveryWaitResult{err: ctx.Err()}
			}
		} else {
			dispatchResult = performanceDeliveryWaitResult{err: delivered.err}
		}
	case <-ctx.Done():
		dispatchResult = performanceDeliveryWaitResult{err: ctx.Err()}
	}
	cancelUpstreamWait()
	cancelDispatchWait()
	if !dispatchResult.found {
		cancelStatusWait()
		for index := range observations {
			observations[index].WorkerCount = actualWorkerCount
			observations[index].Success = false
		}
		return observations, dispatchResult.err
	}

	timingRecorder.apply(observations)
	for index := range observations {
		traceID := strings.TrimSpace(observations[index].TraceID)
		sampleStartedAt := observations[index].StartedAt
		if sampleStartedAt.IsZero() {
			sampleStartedAt = startedAt
		}
		dispatchElapsedMS := h.performanceFakeUpstreamElapsedMS(observations[index].SampleID, sampleStartedAt)
		if dispatchElapsedMS <= 0 {
			dispatchElapsedMS = timingRecorder.elapsedDurationMS(traceID, string(deliverypkg.TimingDispatchHTTP))
		}
		if dispatchElapsedMS <= 0 {
			dispatchElapsedMS = dispatchResult.durationMS
		}
		observations[index].WorkerCount = actualWorkerCount
		observations[index].RouteDurationMS = performancePlanningDuration(observations[index])
		if observations[index].DeliveryDispatchDurationMS <= 0 {
			observations[index].DeliveryDispatchDurationMS = 1
		}
		observations[index].DispatchDurationMS = maxPerformanceDuration(1, observations[index].DeliveryDispatchDurationMS)
		observations[index].DispatchRunDurationMS = dispatchResult.durationMS
		observations[index].AcceptedRunDurationMS = acceptedRunDurationMS
		observations[index].EndToEndDurationMS = maxPerformanceDuration(1, dispatchElapsedMS)
	}

	if !deliveredAlreadyFound {
		select {
		case delivered = <-statusWait:
		case <-ctx.Done():
			delivered = performanceDeliveryWaitResult{err: ctx.Err()}
		}
	}
	cancelStatusWait()
	timingRecorder.apply(observations)
	runDurationMS := elapsedMilliseconds(startedAt)
	if delivered.found {
		deliveryDetails, detailsFound := h.performanceDeliveryStatusDetails(ctx, observations)
		timingRecorder.apply(observations)
		for index := range observations {
			traceID := strings.TrimSpace(observations[index].TraceID)
			completeElapsedMS := 0
			if detailsFound {
				if detail := deliveryDetails[traceID]; detail.Sent {
					completeElapsedMS = performanceDurationSince(
						firstNonZeroTime(observations[index].StartedAt, detail.ReceivedAt, startedAt),
						firstNonZeroTime(detail.PersistedAt, detail.FinishedAt),
					)
				}
			}
			if completeElapsedMS <= 0 {
				completeElapsedMS = timingRecorder.elapsedDurationMS(traceID, string(deliverypkg.TimingComplete))
			}
			if completeElapsedMS <= 0 {
				completeElapsedMS = delivered.durationMS
			}
			observations[index].ReceiveDurationMS = completeElapsedMS
			observations[index].ConcurrencyRunDurationMS = runDurationMS
			observations[index].CompletionEndToEndDurationMS = maxPerformanceDuration(observations[index].EndToEndDurationMS, completeElapsedMS)
			if observations[index].TraceID == "" || !delivered.statuses[observations[index].TraceID] {
				observations[index].Success = false
			}
		}
		return observations, nil
	}
	for index := range observations {
		observations[index].WorkerCount = actualWorkerCount
		observations[index].Success = false
		observations[index].ConcurrencyRunDurationMS = runDurationMS
		if observations[index].CompletionEndToEndDurationMS <= 0 {
			observations[index].CompletionEndToEndDurationMS = runDurationMS
		}
	}
	return observations, delivered.err
}

func (h *Handler) performanceTestUsesJetStreamQueue() bool {
	return h != nil && strings.EqualFold(strings.TrimSpace(h.cfg.Queue.Backend), "jetstream")
}

func (h *Handler) performanceDeliveryStatuses(ctx context.Context, observations []settings.PerformanceTestObservation) (map[string]bool, bool) {
	reader, ok := h.sources.(performanceDeliveryStatusReader)
	if !ok || reader == nil {
		return nil, false
	}
	traceIDs := make([]string, 0, len(observations))
	for _, observation := range observations {
		if strings.TrimSpace(observation.TraceID) != "" {
			traceIDs = append(traceIDs, observation.TraceID)
		}
	}
	if len(traceIDs) == 0 {
		return nil, false
	}
	statuses, err := reader.PerformanceDeliveryStatuses(ctx, traceIDs)
	if err != nil {
		return nil, false
	}
	return statuses, true
}

func (h *Handler) performanceDeliveryStatusDetails(ctx context.Context, observations []settings.PerformanceTestObservation) (map[string]source.PerformanceDeliveryStatus, bool) {
	reader, ok := h.sources.(performanceDeliveryStatusDetailReader)
	if !ok || reader == nil {
		return nil, false
	}
	traceIDs := make([]string, 0, len(observations))
	for _, observation := range observations {
		if strings.TrimSpace(observation.TraceID) != "" {
			traceIDs = append(traceIDs, observation.TraceID)
		}
	}
	if len(traceIDs) == 0 {
		return nil, false
	}
	statuses, err := reader.PerformanceDeliveryStatusDetails(ctx, traceIDs)
	if err != nil {
		return nil, false
	}
	return statuses, true
}

func (h *Handler) waitPerformanceDeliverySuccess(ctx context.Context, observations []settings.PerformanceTestObservation, startedAt time.Time) performanceDeliveryWaitResult {
	if len(observations) == 0 {
		return performanceDeliveryWaitResult{}
	}
	ticker := time.NewTicker(performanceTestWorkerIdleSleep)
	defer ticker.Stop()
	for {
		statuses, found := h.performanceDeliveryStatuses(ctx, observations)
		if found && performanceDeliveryStatusesAllSuccessful(observations, statuses) {
			return performanceDeliveryWaitResult{
				statuses:   statuses,
				found:      true,
				durationMS: elapsedMilliseconds(startedAt),
			}
		}
		select {
		case <-ctx.Done():
			return performanceDeliveryWaitResult{err: ctx.Err()}
		case <-ticker.C:
		}
	}
}

func performanceDeliveryStatusesAllSuccessful(observations []settings.PerformanceTestObservation, statuses map[string]bool) bool {
	hasTrace := false
	for _, observation := range observations {
		traceID := strings.TrimSpace(observation.TraceID)
		if traceID == "" {
			return false
		}
		hasTrace = true
		if !statuses[traceID] {
			return false
		}
	}
	return hasTrace
}

func (h *Handler) waitPerformanceDeliveryDispatch(ctx context.Context, observations []settings.PerformanceTestObservation, recorder *performanceWorkerTimingRecorder, startedAt time.Time) performanceDeliveryWaitResult {
	traceIDs := make([]string, 0, len(observations))
	for _, observation := range observations {
		if strings.TrimSpace(observation.TraceID) != "" {
			traceIDs = append(traceIDs, observation.TraceID)
		}
	}
	if len(traceIDs) == 0 || recorder == nil {
		return performanceDeliveryWaitResult{}
	}
	ticker := time.NewTicker(performanceTestWorkerIdleSleep)
	defer ticker.Stop()
	for {
		if recorder.hasStageForAll(traceIDs, string(deliverypkg.TimingDispatchHTTP)) {
			return performanceDeliveryWaitResult{
				found:      true,
				durationMS: elapsedMilliseconds(startedAt),
			}
		}
		select {
		case <-ctx.Done():
			return performanceDeliveryWaitResult{err: ctx.Err()}
		case <-ticker.C:
		}
	}
}

func (h *Handler) waitPerformanceFakeUpstreamDispatch(ctx context.Context, observations []settings.PerformanceTestObservation, startedAt time.Time) performanceDeliveryWaitResult {
	sampleIDs := make([]string, 0, len(observations))
	for _, observation := range observations {
		if strings.TrimSpace(observation.SampleID) != "" {
			sampleIDs = append(sampleIDs, observation.SampleID)
		}
	}
	if len(sampleIDs) == 0 || h.perfUpstream == nil {
		return performanceDeliveryWaitResult{}
	}
	ticker := time.NewTicker(performanceTestWorkerIdleSleep)
	defer ticker.Stop()
	for {
		allReceived := true
		for _, sampleID := range sampleIDs {
			if _, ok := h.perfUpstream.receivedAt(sampleID); !ok {
				allReceived = false
				break
			}
		}
		if allReceived {
			return performanceDeliveryWaitResult{
				found:      true,
				durationMS: elapsedMilliseconds(startedAt),
			}
		}
		select {
		case <-ctx.Done():
			return performanceDeliveryWaitResult{err: ctx.Err()}
		case <-ticker.C:
		}
	}
}

func (h *Handler) performanceFakeUpstreamElapsedMS(sampleID string, referenceAt time.Time) int {
	if h == nil || h.perfUpstream == nil || referenceAt.IsZero() {
		return 0
	}
	receivedAt, ok := h.perfUpstream.receivedAt(sampleID)
	if !ok {
		return 0
	}
	return performanceDurationSince(referenceAt, receivedAt)
}

func (h *Handler) performanceTestWorkerCounts(ctx context.Context, input settings.PerformanceTestInput, concurrency int, messageCount int) (int, int) {
	if messageCount <= 0 {
		return 1, 1
	}
	if settings.PerformanceWorkerMode(input) == settings.PerformanceWorkerModeConcurrency {
		count := minPerformanceInt(concurrency, messageCount)
		if count <= 0 {
			count = 1
		}
		return count, count
	}
	planningWorkerCount := minPerformanceInt(performanceTestPlanningWorkers, messageCount)
	if planningWorkerCount <= 0 {
		planningWorkerCount = 1
	}
	deliveryWorkerCount := performanceTestDeliveryWorkers
	if h.settings != nil {
		deliveryWorkerCount = h.settings.IntSetting(ctx, settings.KeyRuntimeDeliveryConcurrency, settings.DefaultDeliveryGlobalConcurrency)
	}
	if deliveryWorkerCount <= 0 {
		deliveryWorkerCount = settings.DefaultDeliveryGlobalConcurrency
	}
	deliveryWorkerCount = minPerformanceInt(deliveryWorkerCount, messageCount)
	if deliveryWorkerCount <= 0 {
		deliveryWorkerCount = 1
	}
	return planningWorkerCount, deliveryWorkerCount
}

func performanceTestBatchDemandWorkerCount(workerCount int, target int, batchSize int) int {
	if workerCount <= 0 {
		workerCount = 1
	}
	if target <= 0 {
		return 1
	}
	if batchSize <= 0 {
		batchSize = 1
	}
	batches := (target + batchSize - 1) / batchSize
	if batches <= 0 {
		batches = 1
	}
	return minPerformanceInt(workerCount, batches)
}

func (h *Handler) runPerformanceTestSample(ctx context.Context, resources performanceTestResources, input settings.PerformanceTestInput, index int) settings.PerformanceTestObservation {
	payloadVariantCount := normalizePerformancePayloadVariantCount(input.PayloadVariantCount)
	resource := resources.Sources[index%len(resources.Sources)]
	sampleID := performanceSampleID(index)
	payload := performancePayload(index, payloadVariantCount, sampleID)
	observation := settings.PerformanceTestObservation{SampleID: sampleID, Success: true}

	templateStartedAt := time.Now()
	templateInput := resources.TemplateInput
	templateInput.SamplePayload = payload
	if h.templates != nil && templateInput.TemplateBody != "" {
		if _, previewErr := h.templates.Preview(templateInput); previewErr != nil {
			observation.Success = false
		}
	}
	observation.TemplateRenderDurationMS = elapsedMilliseconds(templateStartedAt)

	inboundStartedAt := time.Now()
	observation.StartedAt = inboundStartedAt
	path := "/api/v1/ingest/" + resource.SourceCode
	timingRecorder := newPerformanceIngestTimingRecorder()
	ingestCtx := source.WithIngestTimingRecorder(ctx, timingRecorder)
	ingestResult, ingestErr := h.sources.Ingest(ingestCtx, source.IngestInput{
		SourceCode: resource.SourceCode,
		Method:     http.MethodPost,
		Path:       path,
		Headers:    performanceIngestHeaders(resource.AuthMode, resource.SourceToken, resource.HMACSecret, http.MethodPost, path, payload),
		RemoteAddr: "127.0.0.1",
		Body:       payload,
	})
	observation.TraceID = ingestResult.TraceID
	observation.InboundDurationMS = elapsedMilliseconds(inboundStartedAt)
	observation.SourceLookupDurationMS = timingRecorder.durationMS(source.IngestTimingSourceLookup)
	observation.LatestPayloadUpdateDurationMS = timingRecorder.durationMS(source.IngestTimingLatestPayloadUpdate)
	observation.EnqueueInboundDurationMS = timingRecorder.durationMS(source.IngestTimingEnqueueInbound)
	observation.InsertMessageRecordDurationMS = timingRecorder.durationMS(source.IngestTimingInsertMessageRecord)
	observation.InsertInboundDedupeKeyDurationMS = timingRecorder.durationMS(source.IngestTimingInsertInboundDedupeKey)
	observation.InsertRoutePlanJobDurationMS = timingRecorder.durationMS(source.IngestTimingInsertRoutePlanJob)
	observation.CommitInboundTransactionDurationMS = timingRecorder.durationMS(source.IngestTimingCommitInboundTransaction)
	if ingestErr != nil {
		observation.Success = false
	}

	observation.EndToEndDurationMS = maxPerformanceDuration(1, observation.TemplateRenderDurationMS+observation.InboundDurationMS)
	return observation
}

type performanceBatchWorker interface {
	ProcessBatch(context.Context, int) (int, error)
}

type performanceWorkerDrain struct {
	done chan performanceWorkerDrainResult
}

type performanceWorkerDrainResult struct {
	processed  int
	durationMS int
	err        error
}

func (h *Handler) startPerformanceWorkerDrain(ctx context.Context, worker performanceBatchWorker, target int, workerCount int) performanceWorkerDrain {
	drain := performanceWorkerDrain{done: make(chan performanceWorkerDrainResult, 1)}
	if worker == nil || target <= 0 {
		drain.done <- performanceWorkerDrainResult{processed: maxPerformanceDuration(0, target)}
		return drain
	}
	if workerCount <= 0 {
		workerCount = 1
	}
	workerCount = minPerformanceInt(workerCount, target)
	go func() {
		startedAt := time.Now()
		drainCtx, cancel := context.WithTimeout(ctx, performanceTestWorkerDrainLimit)
		defer cancel()

		var processed int64
		errs := make(chan error, workerCount)
		var wg sync.WaitGroup
		for index := 0; index < workerCount; index++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for {
					if atomic.LoadInt64(&processed) >= int64(target) {
						return
					}
					if err := drainCtx.Err(); err != nil {
						return
					}
					count, err := worker.ProcessBatch(drainCtx, performanceTestWorkerBatchSize)
					if err != nil {
						select {
						case errs <- err:
						default:
						}
						return
					}
					if count > 0 {
						atomic.AddInt64(&processed, int64(count))
						continue
					}
					timer := time.NewTimer(performanceTestWorkerIdleSleep)
					select {
					case <-drainCtx.Done():
						timer.Stop()
						return
					case <-timer.C:
					}
				}
			}()
		}
		wg.Wait()
		close(errs)

		result := performanceWorkerDrainResult{
			processed:  int(atomic.LoadInt64(&processed)),
			durationMS: elapsedMilliseconds(startedAt),
		}
		for err := range errs {
			result.err = errors.Join(result.err, err)
		}
		if result.err == nil && result.processed < target {
			if drainCtx.Err() != nil {
				result.err = drainCtx.Err()
			} else {
				result.err = fmt.Errorf("performance worker drained %d/%d jobs", result.processed, target)
			}
		}
		drain.done <- result
	}()
	return drain
}

func (d performanceWorkerDrain) wait(ctx context.Context) performanceWorkerDrainResult {
	select {
	case result := <-d.done:
		return result
	case <-ctx.Done():
		return performanceWorkerDrainResult{err: ctx.Err()}
	}
}

type performanceIngestTimingRecorder struct {
	mu        sync.Mutex
	durations map[source.IngestTimingStage]time.Duration
}

func newPerformanceIngestTimingRecorder() *performanceIngestTimingRecorder {
	return &performanceIngestTimingRecorder{durations: make(map[source.IngestTimingStage]time.Duration)}
}

func (r *performanceIngestTimingRecorder) RecordIngestTiming(stage source.IngestTimingStage, duration time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.durations[stage] += duration
}

func (r *performanceIngestTimingRecorder) durationMS(stage source.IngestTimingStage) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	duration := r.durations[stage]
	if duration <= 0 {
		return 0
	}
	milliseconds := int(duration.Milliseconds())
	if milliseconds <= 0 {
		return 1
	}
	return milliseconds
}

func performancePayload(index int, payloadVariantCount int, sampleID string) json.RawMessage {
	routeKeys := performanceRouteKeys(payloadVariantCount)
	payload, _ := json.Marshal(map[string]any{
		"timestamp": time.Now().Format(time.RFC3339),
		"route_key": routeKeys[index%len(routeKeys)],
		"sample_id": sampleID,
		"title":     "性能测试",
		"content":   "这是一条性能测试消息，随机消息-" + randomPerformanceText(12),
	})
	return payload
}

func performanceSampleID(index int) string {
	return fmt.Sprintf("perf-sample-%06d", index)
}

func performanceSampleIDFromRequest(raw []byte) string {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return ""
	}
	return findPerformanceSampleID(value)
}

func performanceDurationSince(startedAt time.Time, finishedAt time.Time) int {
	if startedAt.IsZero() || finishedAt.IsZero() {
		return 0
	}
	return maxPerformanceDuration(1, int(finishedAt.Sub(startedAt).Milliseconds()))
}

func firstNonZeroTime(values ...time.Time) time.Time {
	for _, value := range values {
		if !value.IsZero() {
			return value
		}
	}
	return time.Time{}
}

func findPerformanceSampleID(value any) string {
	switch typed := value.(type) {
	case map[string]any:
		if sampleID := strings.TrimSpace(fmt.Sprint(typed["sample_id"])); sampleID != "" && sampleID != "<nil>" {
			return sampleID
		}
		for _, child := range typed {
			if sampleID := findPerformanceSampleID(child); sampleID != "" {
				return sampleID
			}
		}
	case []any:
		for _, child := range typed {
			if sampleID := findPerformanceSampleID(child); sampleID != "" {
				return sampleID
			}
		}
	}
	return ""
}

func performanceRouteKeys(payloadVariantCount int) []string {
	keys := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l"}
	count := maxPerformanceDuration(1, minPerformanceInt(payloadVariantCount, len(keys)))
	return append([]string(nil), keys[:count]...)
}

func performanceIngestHeaders(authMode source.AuthMode, token string, hmacSecret string, method string, path string, body []byte) http.Header {
	headers := http.Header{
		"Content-Type": []string{"application/json"},
	}
	switch authMode {
	case source.AuthModeToken, source.AuthModeTokenAndHMAC:
		headers.Set("Authorization", "Bearer "+token)
	}
	switch authMode {
	case source.AuthModeHMAC, source.AuthModeTokenAndHMAC:
		timestamp := fmt.Sprintf("%d", time.Now().Unix())
		nonce := "perf" + randomPerformanceText(12)
		bodyHash := sha256.Sum256(body)
		signingString := fmt.Sprintf("%s\n%s\n%s\n%s\n%s", method, path, timestamp, nonce, hex.EncodeToString(bodyHash[:]))
		mac := hmac.New(sha256.New, []byte(hmacSecret))
		mac.Write([]byte(signingString))
		headers.Set("X-MGP-Timestamp", timestamp)
		headers.Set("X-MGP-Nonce", nonce)
		headers.Set("X-MGP-Signature", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	}
	return headers
}

type sourceRuntimeStatsReader interface {
	RuntimeStats(context.Context) (source.RuntimeStats, error)
}

type performanceRuntimeSnapshot struct {
	sourceStats        source.RuntimeStats
	queueBacklog       int
	queueRoutePlan     int
	queueSendMessage   int
	queueOldestWaitSec int64
	goroutines         int
	cpuCount           int
	goMaxProcs         int
	mem                runtime.MemStats
}

func (h *Handler) performanceRuntimeSnapshot(ctx context.Context) performanceRuntimeSnapshot {
	snapshot := performanceRuntimeSnapshot{
		goroutines: runtime.NumGoroutine(),
		cpuCount:   runtime.NumCPU(),
		goMaxProcs: runtime.GOMAXPROCS(0),
	}
	runtime.ReadMemStats(&snapshot.mem)
	if statsReader, ok := h.sources.(sourceRuntimeStatsReader); ok {
		if stats, err := statsReader.RuntimeStats(ctx); err == nil {
			snapshot.sourceStats = stats
		}
	}
	if h.monitoring != nil {
		if queue, err := h.monitoring.GetQueueMonitoringSnapshot(ctx, monitoring.QueryParams{Window: time.Hour}); err == nil {
			snapshot.queueRoutePlan = queue.Summary.RoutePlanPending
			snapshot.queueSendMessage = queue.Summary.SendMessagePending
			snapshot.queueBacklog = snapshot.queueRoutePlan + snapshot.queueSendMessage
			snapshot.queueOldestWaitSec = queue.Summary.OldestJobWaitSeconds
		}
	}
	return snapshot
}

func performanceRuntimeDiagnostics(before performanceRuntimeSnapshot, after performanceRuntimeSnapshot) settings.PerformanceRuntimeDiagnostics {
	return settings.PerformanceRuntimeDiagnostics{
		DBPoolAcquireCountDelta:   after.sourceStats.DBPoolAcquireCount - before.sourceStats.DBPoolAcquireCount,
		DBPoolWaitCountDelta:      after.sourceStats.DBPoolWaitCount - before.sourceStats.DBPoolWaitCount,
		DBPoolWaitDurationDeltaMS: after.sourceStats.DBPoolWaitDurationMS - before.sourceStats.DBPoolWaitDurationMS,
		DBPoolAcquiredConnsBefore: before.sourceStats.DBPoolAcquiredConns,
		DBPoolAcquiredConnsAfter:  after.sourceStats.DBPoolAcquiredConns,
		DBPoolTotalConnsBefore:    before.sourceStats.DBPoolTotalConns,
		DBPoolTotalConnsAfter:     after.sourceStats.DBPoolTotalConns,
		PostgresMaxConnections:    after.sourceStats.PostgresMaxConnections,
		PostgresBlocksRead:        after.sourceStats.PostgresBlocksRead,
		PostgresBlocksHit:         after.sourceStats.PostgresBlocksHit,
		PostgresTempBytes:         after.sourceStats.PostgresTempBytes,
		PostgresBlocksReadDelta:   after.sourceStats.PostgresBlocksRead - before.sourceStats.PostgresBlocksRead,
		PostgresBlocksHitDelta:    after.sourceStats.PostgresBlocksHit - before.sourceStats.PostgresBlocksHit,
		PostgresTempBytesDelta:    after.sourceStats.PostgresTempBytes - before.sourceStats.PostgresTempBytes,
		CPUCount:                  after.cpuCount,
		GoMaxProcs:                after.goMaxProcs,
		QueueBacklogBefore:        before.queueBacklog,
		QueueBacklogAfter:         after.queueBacklog,
		QueueRoutePlanBefore:      before.queueRoutePlan,
		QueueRoutePlanAfter:       after.queueRoutePlan,
		QueueSendMessageBefore:    before.queueSendMessage,
		QueueSendMessageAfter:     after.queueSendMessage,
		QueueOldestWaitBefore:     before.queueOldestWaitSec,
		QueueOldestWaitAfter:      after.queueOldestWaitSec,
		GoroutinesBefore:          before.goroutines,
		GoroutinesAfter:           after.goroutines,
		GoroutineGrowthWarning:    after.goroutines-before.goroutines > 100,
		MemoryAllocBytesBefore:    before.mem.Alloc,
		MemoryAllocBytesAfter:     after.mem.Alloc,
		MemorySysBytesBefore:      before.mem.Sys,
		MemorySysBytesAfter:       after.mem.Sys,
		GCCountDelta:              after.mem.NumGC - before.mem.NumGC,
		GCPauseTotalDeltaMS:       (after.mem.PauseTotalNs - before.mem.PauseTotalNs) / uint64(time.Millisecond),
	}
}

func normalizePerformanceSourceAuthMode(value string) source.AuthMode {
	switch source.AuthMode(value) {
	case source.AuthModeToken, source.AuthModeHMAC, source.AuthModeTokenAndHMAC, source.AuthModeNone:
		return source.AuthMode(value)
	default:
		return source.AuthModeToken
	}
}

func performanceTestMaxConcurrency(input settings.PerformanceTestInput) int {
	candidates := settings.PerformanceConcurrencyCandidates(input)
	if len(candidates) == 0 {
		return 16
	}
	return candidates[len(candidates)-1]
}

func isPerformanceTestSource(item source.Source) bool {
	if len(item.Code) != 8 || !strings.HasPrefix(item.Code, "test") {
		return false
	}
	for _, value := range item.Code[4:] {
		if value < '0' || value > '9' {
			return false
		}
	}
	return strings.HasPrefix(item.Name, "性能测试")
}

func randomPerformanceText(length int) string {
	const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	if length <= 0 {
		return ""
	}
	raw := make([]byte, length)
	if _, err := rand.Read(raw); err != nil {
		return fmt.Sprintf("%012d", time.Now().UnixNano()%1_000_000_000_000)
	}
	for index, value := range raw {
		raw[index] = alphabet[int(value)%len(alphabet)]
	}
	return string(raw)
}

func elapsedMilliseconds(start time.Time) int {
	return maxPerformanceDuration(1, int(time.Since(start).Milliseconds()))
}

func normalizePerformanceSourceCount(value int) int {
	if value <= 0 {
		return 1
	}
	if value > 5 {
		return 5
	}
	return value
}

func normalizePerformancePayloadVariantCount(value int) int {
	if value <= 0 {
		return 3
	}
	if value > 12 {
		return 12
	}
	return value
}

func maxPerformanceDuration(left int, right int) int {
	if left > right {
		return left
	}
	return right
}

func minPerformanceInt(left int, right int) int {
	if left < right {
		return left
	}
	return right
}

func firstPositivePerformanceInt(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func (h *Handler) settingsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	if !h.requireSettingsService(w) {
		return
	}
	if _, ok := h.authenticateRequest(w, r); !ok {
		return
	}
	items, err := h.settings.ListSettings(r.Context())
	if err != nil {
		status, code, message := settingsErrorStatus(err)
		writeAPIError(w, status, code, message)
		return
	}
	response := settingsResponse{Settings: make([]settingResponse, 0, len(items))}
	for _, item := range items {
		response.Settings = append(response.Settings, toSettingResponse(item))
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) settingDetailHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		methodNotAllowed(w, http.MethodPut)
		return
	}
	key := singleIDFromPath(r.URL.Path, h.cfg.Server.APIPrefix+"/settings/")
	if key == "" {
		writeAPIError(w, http.StatusNotFound, "MGP-SETTINGS-001", "系统设置不存在")
		return
	}
	if !h.requireSettingsService(w) {
		return
	}
	adminUser, ok := h.authenticateRequest(w, r)
	if !ok {
		return
	}
	var request settings.UpdateInput
	if err := decodeJSON(r, &request); err != nil {
		writeAPIError(w, http.StatusBadRequest, "MGP-REQ-001", "请求 JSON 不合法")
		return
	}
	item, err := h.settings.UpdateSetting(r.Context(), key, request)
	if err != nil {
		status, code, message := settingsErrorStatus(err)
		writeAPIError(w, status, code, message)
		return
	}
	response := settingBody{Setting: toSettingResponse(item)}
	h.recordAudit(r, adminUser, "update", "system_setting", key, request, response)
	writeJSON(w, http.StatusOK, response)
}

func toSettingResponse(item settings.Setting) settingResponse {
	return settingResponse{
		Key:         item.Key,
		Value:       defaultRawJSON(item.Value),
		Description: item.Description,
		Category:    item.Category,
		CreatedAt:   formatTime(item.CreatedAt),
		UpdatedAt:   formatTime(item.UpdatedAt),
	}
}

func settingsErrorStatus(err error) (int, string, string) {
	switch {
	case errors.Is(err, settings.ErrInvalidInput):
		return http.StatusBadRequest, "MGP-REQ-001", "请求参数不合法"
	case errors.Is(err, settings.ErrNotFound):
		return http.StatusNotFound, "MGP-SETTINGS-001", "系统设置不存在"
	default:
		return http.StatusInternalServerError, "MGP-SETTINGS-999", "系统设置服务内部错误"
	}
}
