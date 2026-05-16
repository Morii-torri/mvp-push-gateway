package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

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

type settingResponse struct {
	Key         string          `json:"key"`
	Value       json.RawMessage `json:"value"`
	Description string          `json:"description"`
	Category    string          `json:"category"`
	CreatedAt   string          `json:"created_at"`
	UpdatedAt   string          `json:"updated_at"`
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
	if prepared, err := h.createPerformanceTestResources(r.Context()); err == nil {
		request.GeneratedSourceCode = prepared.SourceCode
		request.GeneratedRouteName = prepared.RouteName
		request.GeneratedChannelName = prepared.ChannelName
	} else {
		writeAPIError(w, http.StatusInternalServerError, "MGP-SETTINGS-999", "性能测试配置生成失败")
		return
	}
	result, err := h.settings.RunPerformanceTest(r.Context(), request)
	if err != nil {
		status, code, message := settingsErrorStatus(err)
		writeAPIError(w, status, code, message)
		return
	}
	response := performanceTestBody{Result: result}
	h.recordAudit(r, adminUser, "run", "performance_test", result.UpdatedSettingKey, request, response)
	writeJSON(w, http.StatusOK, response)
}

type performanceTestResources struct {
	SourceCode  string
	RouteName   string
	ChannelName string
}

func (h *Handler) createPerformanceTestResources(ctx context.Context) (performanceTestResources, error) {
	if h.sources == nil || h.providers == nil || h.templates == nil || h.routes == nil {
		return performanceTestResources{}, nil
	}
	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)
	sourceCode := "perftest" + suffix
	channelName := "性能测试本地上级-" + suffix
	routeName := "性能测试路由-" + suffix

	createdSource, err := h.sources.CreateSource(ctx, source.CreateSourceInput{
		Code:            sourceCode,
		Name:            "性能测试来源-" + suffix,
		Enabled:         true,
		AuthMode:        source.AuthModeToken,
		AuthToken:       "perftesttoken" + suffix,
		CompatMode:      "standard",
		RateLimitConfig: json.RawMessage(`{"enabled":false}`),
	})
	if err != nil {
		return performanceTestResources{}, err
	}
	channel, err := h.providers.CreateChannel(ctx, provider.CreateChannelInput{
		ProviderType:     provider.ProviderWebhook,
		Name:             channelName,
		Enabled:          true,
		SendConfig:       json.RawMessage(`{"method":"POST","url":"http://127.0.0.1/local-performance-test/fake-upstream","recipient":{"location":"none"}}`),
		RateLimitConfig:  json.RawMessage(`{"enabled":true,"qps":100000}`),
		ConcurrencyLimit: 64,
		TimeoutMS:        1000,
		RetryPolicy:      json.RawMessage(`{"max_attempts":1,"delay_ms":100}`),
		DeadLetterPolicy: json.RawMessage(`{"policy":"retry_exhausted_or_upstream_error","retention_days":7,"replay":false}`),
	})
	if err != nil {
		return performanceTestResources{}, err
	}
	template, err := h.templates.CreateTemplate(ctx, msgtemplate.TemplateInput{
		Name:     "性能测试模板-" + suffix,
		SourceID: createdSource.ID,
		Enabled:  true,
	})
	if err != nil {
		return performanceTestResources{}, err
	}
	version, err := h.templates.Publish(ctx, template.ID, msgtemplate.VersionInput{
		MessageType:        "json",
		TargetProviderType: string(provider.ProviderWebhook),
		TemplateBody:       `{"title":"{{ payload.title }}","content":"{{ payload.content }}"}`,
		MessageBodySchema:  json.RawMessage(`{"type":"object"}`),
		SamplePayload:      json.RawMessage(`{"title":"性能测试","content":"本地 fake upstream"}`),
	})
	if err != nil {
		return performanceTestResources{}, err
	}
	flow, err := h.routes.CreateFlow(ctx, route.CreateFlowInput{
		SourceID: createdSource.ID,
		Name:     routeName,
		Enabled:  true,
		Mode:     route.ModeTable,
	})
	if err != nil {
		return performanceTestResources{}, err
	}
	if _, err := h.routes.SaveRules(ctx, flow.ID, route.SaveRulesInput{Rules: []route.RuleInput{{
		RuleKey:       "perftest" + suffix,
		SortOrder:     10,
		Name:          "性能测试默认命中",
		ConditionTree: json.RawMessage(`{"operator":"always"}`),
		Enabled:       true,
		Action: route.ActionInput{
			Targets: []route.ActionTargetInput{{
				ChannelID:         channel.ID,
				TemplateVersionID: version.ID,
				Enabled:           true,
			}},
			RecipientStrategy: json.RawMessage(`{"type":"none"}`),
			SendDedupeConfig:  json.RawMessage(`{"enabled":false}`),
		},
	}}}); err != nil {
		return performanceTestResources{}, err
	}
	if _, err := h.routes.Publish(ctx, flow.ID, "性能测试自动发布"); err != nil {
		return performanceTestResources{}, err
	}
	return performanceTestResources{SourceCode: sourceCode, RouteName: routeName, ChannelName: channelName}, nil
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
