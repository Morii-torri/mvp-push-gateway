package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"mvp-push-gateway/backend/internal/auth"
	"mvp-push-gateway/backend/internal/route"
)

type routeFlowListResponse struct {
	Flows []routeFlowResponse `json:"flows"`
}

type routeFlowResponse struct {
	ID               string `json:"id"`
	SourceID         string `json:"source_id"`
	Name             string `json:"name"`
	Enabled          bool   `json:"enabled"`
	Mode             string `json:"mode"`
	CurrentVersionID string `json:"current_version_id"`
	RuleCount        int    `json:"rule_count"`
	TotalHitCount    int    `json:"total_hit_count"`
	CreatedAt        string `json:"created_at"`
	UpdatedAt        string `json:"updated_at"`
}

type routeFlowRequest struct {
	ID       string         `json:"id"`
	SourceID string         `json:"source_id"`
	Name     string         `json:"name"`
	Enabled  bool           `json:"enabled"`
	Mode     route.FlowMode `json:"mode"`
}

type routeFlowDetailResponse struct {
	Flow routeFlowResponse `json:"flow"`
}

type routeVersionsResponse struct {
	Versions []routeVersionResponse `json:"versions"`
}

type routeVersionResponse struct {
	ID               string          `json:"id"`
	FlowID           string          `json:"flow_id"`
	VersionNo        int             `json:"version_no"`
	CanvasSnapshot   json.RawMessage `json:"canvas_snapshot"`
	CompiledRules    json.RawMessage `json:"compiled_rules"`
	ValidationStatus string          `json:"validation_status"`
	ValidationErrors json.RawMessage `json:"validation_errors"`
	VersionInfo      string          `json:"version_info"`
	PublishedAt      *string         `json:"published_at"`
	CreatedAt        string          `json:"created_at"`
	UpdatedAt        string          `json:"updated_at"`
}

type routeCanvasRequest struct {
	CanvasSnapshot json.RawMessage `json:"canvas_snapshot"`
}

type routePublishRequest struct {
	VersionInfo string `json:"version_info"`
}

type routeRulesRequest struct {
	Rules []routeRuleRequest `json:"rules"`
}

type routeRuleRequest struct {
	RuleKey       string             `json:"rule_key"`
	SortOrder     int                `json:"sort_order"`
	Name          string             `json:"name"`
	ConditionTree json.RawMessage    `json:"condition_tree"`
	Enabled       bool               `json:"enabled"`
	Action        routeActionRequest `json:"action"`
}

type routeActionRequest struct {
	Targets           []routeActionTargetRequest `json:"targets"`
	TemplateVersionID string                     `json:"template_version_id,omitempty"`
	ChannelIDs        []string                   `json:"channel_ids,omitempty"`
	RecipientStrategy json.RawMessage            `json:"recipient_strategy"`
	SendDedupeConfig  json.RawMessage            `json:"send_dedupe_config"`
	FailurePolicy     json.RawMessage            `json:"failure_policy"`
}

type routeActionTargetRequest struct {
	ChannelID         string `json:"channel_id"`
	TemplateVersionID string `json:"template_version_id"`
	Enabled           bool   `json:"enabled"`
}

type routeRulesResponse struct {
	VersionID string              `json:"version_id"`
	Rules     []routeRuleResponse `json:"rules"`
}

type routeRuleResponse struct {
	ID            string              `json:"id"`
	RuleKey       string              `json:"rule_key"`
	SortOrder     int                 `json:"sort_order"`
	Name          string              `json:"name"`
	ConditionTree json.RawMessage     `json:"condition_tree"`
	Enabled       bool                `json:"enabled"`
	Action        routeActionResponse `json:"action"`
	HitCount      int                 `json:"hit_count"`
	LastHitAt     *string             `json:"last_hit_at"`
	CreatedAt     string              `json:"created_at"`
	UpdatedAt     string              `json:"updated_at"`
}

type routeActionResponse struct {
	ID                string                      `json:"id"`
	Targets           []routeActionTargetResponse `json:"targets"`
	TemplateVersionID string                      `json:"template_version_id,omitempty"`
	ChannelIDs        []string                    `json:"channel_ids,omitempty"`
	RecipientStrategy json.RawMessage             `json:"recipient_strategy"`
	SendDedupeConfig  json.RawMessage             `json:"send_dedupe_config"`
	FailurePolicy     json.RawMessage             `json:"failure_policy"`
}

type routeActionTargetResponse struct {
	ID                string `json:"id"`
	ChannelID         string `json:"channel_id"`
	TemplateVersionID string `json:"template_version_id"`
	Enabled           bool   `json:"enabled"`
	SortOrder         int    `json:"sort_order"`
}

type routeReorderRequest struct {
	RuleKeys []string `json:"rule_keys"`
}

type routeValidationResponse struct {
	VersionID string                  `json:"version_id"`
	Status    string                  `json:"status"`
	Errors    []route.ValidationError `json:"errors"`
}

type routeSimulationRequest struct {
	Payload json.RawMessage `json:"payload"`
}

func (h *Handler) routeFlowsHandler(w http.ResponseWriter, r *http.Request) {
	if !h.requireRouteService(w) {
		return
	}
	adminUser, ok := h.authenticateRequest(w, r)
	if !ok {
		return
	}

	switch r.Method {
	case http.MethodGet:
		flows, err := h.routes.ListFlows(r.Context())
		if err != nil {
			status, code, message := routeErrorStatus(err)
			writeAPIError(w, status, code, message)
			return
		}
		items := make([]routeFlowResponse, 0, len(flows))
		for _, flow := range flows {
			items = append(items, toRouteFlowResponse(flow))
		}
		writeJSON(w, http.StatusOK, routeFlowListResponse{Flows: items})
	case http.MethodPost:
		var request routeFlowRequest
		if err := decodeJSON(r, &request); err != nil {
			writeAPIError(w, http.StatusBadRequest, "MGP-REQ-001", "请求 JSON 不合法")
			return
		}
		created, err := h.routes.CreateFlow(r.Context(), route.CreateFlowInput{
			ID:       request.ID,
			SourceID: request.SourceID,
			Name:     request.Name,
			Enabled:  request.Enabled,
			Mode:     request.Mode,
		})
		if err != nil {
			status, code, message := routeErrorStatus(err)
			writeAPIError(w, status, code, message)
			return
		}
		response := routeFlowDetailResponse{Flow: toRouteFlowResponse(created)}
		h.recordAudit(r, adminUser, "create", "route_flow", created.ID, request, response)
		writeJSON(w, http.StatusCreated, response)
	default:
		methodNotAllowed(w, http.MethodGet+", "+http.MethodPost)
	}
}

func (h *Handler) routeFlowDetailHandler(w http.ResponseWriter, r *http.Request) {
	if !h.requireRouteService(w) {
		return
	}
	adminUser, ok := h.authenticateRequest(w, r)
	if !ok {
		return
	}

	path := strings.TrimPrefix(r.URL.Path, h.cfg.Server.APIPrefix+"/route-flows/")
	path = strings.Trim(path, "/")
	if path == "" {
		writeAPIError(w, http.StatusNotFound, "MGP-ROUTE-001", "路由组不存在")
		return
	}
	parts := strings.Split(path, "/")
	flowID := strings.TrimSpace(parts[0])
	if flowID == "" {
		writeAPIError(w, http.StatusNotFound, "MGP-ROUTE-001", "路由组不存在")
		return
	}

	if len(parts) == 1 {
		h.routeFlowResourceHandler(w, r, flowID, adminUser)
		return
	}

	switch parts[1] {
	case "versions":
		if len(parts) == 2 && r.Method == http.MethodGet {
			h.routeVersionsHandler(w, r, flowID)
			return
		}
		if len(parts) == 3 && r.Method == http.MethodDelete {
			h.routeDeleteVersionHandler(w, r, flowID, parts[2], adminUser)
			return
		}
		if len(parts) == 4 && parts[3] == "rules" && r.Method == http.MethodGet {
			h.routeVersionRulesHandler(w, r, flowID, parts[2])
			return
		}
		if len(parts) == 4 && parts[3] == "activate" && r.Method == http.MethodPost {
			h.routeActivateVersionHandler(w, r, flowID, parts[2], adminUser)
			return
		}
	case "canvas":
		h.routeCanvasHandler(w, r, flowID, adminUser)
		return
	case "rules":
		if len(parts) == 2 {
			h.routeRulesHandler(w, r, flowID, adminUser)
			return
		}
		if len(parts) == 3 && parts[2] == "reorder" && r.Method == http.MethodPut {
			h.routeReorderHandler(w, r, flowID, adminUser)
			return
		}
	case "validate":
		if len(parts) == 2 && r.Method == http.MethodPost {
			h.routeValidateHandler(w, r, flowID)
			return
		}
	case "publish":
		if len(parts) == 2 && r.Method == http.MethodPost {
			h.routePublishHandler(w, r, flowID, adminUser)
			return
		}
	case "simulate":
		if len(parts) == 2 && r.Method == http.MethodPost {
			h.routeSimulateHandler(w, r, flowID)
			return
		}
	}

	writeAPIError(w, http.StatusNotFound, "MGP-ROUTE-001", "路由组不存在")
}

func (h *Handler) routeFlowResourceHandler(w http.ResponseWriter, r *http.Request, flowID string, adminUser auth.Admin) {
	switch r.Method {
	case http.MethodGet:
		flow, err := h.routes.GetFlow(r.Context(), flowID)
		if err != nil {
			status, code, message := routeErrorStatus(err)
			writeAPIError(w, status, code, message)
			return
		}
		writeJSON(w, http.StatusOK, routeFlowDetailResponse{Flow: toRouteFlowResponse(flow)})
	case http.MethodPut:
		var request routeFlowRequest
		if err := decodeJSON(r, &request); err != nil {
			writeAPIError(w, http.StatusBadRequest, "MGP-REQ-001", "请求 JSON 不合法")
			return
		}
		updated, err := h.routes.UpdateFlow(r.Context(), flowID, route.UpdateFlowInput{
			ID:       request.ID,
			SourceID: request.SourceID,
			Name:     request.Name,
			Enabled:  request.Enabled,
			Mode:     request.Mode,
		})
		if err != nil {
			status, code, message := routeErrorStatus(err)
			writeAPIError(w, status, code, message)
			return
		}
		response := routeFlowDetailResponse{Flow: toRouteFlowResponse(updated)}
		h.recordAudit(r, adminUser, "update", "route_flow", flowID, request, response)
		writeJSON(w, http.StatusOK, response)
	case http.MethodDelete:
		if err := h.routes.DeleteFlow(r.Context(), flowID); err != nil {
			status, code, message := routeErrorStatus(err)
			writeAPIError(w, status, code, message)
			return
		}
		response := okResponse{OK: true}
		h.recordAudit(r, adminUser, "delete", "route_flow", flowID, nil, response)
		writeJSON(w, http.StatusOK, response)
	default:
		methodNotAllowed(w, http.MethodGet+", "+http.MethodPut+", "+http.MethodDelete)
	}
}

func (h *Handler) routeVersionsHandler(w http.ResponseWriter, r *http.Request, flowID string) {
	versions, err := h.routes.ListVersions(r.Context(), flowID)
	if err != nil {
		status, code, message := routeErrorStatus(err)
		writeAPIError(w, status, code, message)
		return
	}
	items := make([]routeVersionResponse, 0, len(versions))
	for _, version := range versions {
		items = append(items, toRouteVersionResponse(version))
	}
	writeJSON(w, http.StatusOK, routeVersionsResponse{Versions: items})
}

func (h *Handler) routeActivateVersionHandler(w http.ResponseWriter, r *http.Request, flowID string, versionID string, adminUser auth.Admin) {
	updated, err := h.routes.ActivateVersion(r.Context(), flowID, versionID)
	if err != nil {
		status, code, message := routeErrorStatus(err)
		writeAPIError(w, status, code, message)
		return
	}
	response := routeFlowDetailResponse{Flow: toRouteFlowResponse(updated)}
	h.recordAudit(r, adminUser, "activate", "route_version", versionID, map[string]string{"flow_id": flowID}, response)
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) routeDeleteVersionHandler(w http.ResponseWriter, r *http.Request, flowID string, versionID string, adminUser auth.Admin) {
	if err := h.routes.DeleteVersion(r.Context(), flowID, versionID); err != nil {
		status, code, message := routeErrorStatus(err)
		writeAPIError(w, status, code, message)
		return
	}
	response := okResponse{OK: true}
	h.recordAudit(r, adminUser, "delete", "route_version", versionID, map[string]string{"flow_id": flowID}, response)
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) routeCanvasHandler(w http.ResponseWriter, r *http.Request, flowID string, adminUser auth.Admin) {
	switch r.Method {
	case http.MethodGet:
		state, err := h.routes.GetCanvas(r.Context(), flowID)
		if err != nil {
			status, code, message := routeErrorStatus(err)
			writeAPIError(w, status, code, message)
			return
		}
		writeJSON(w, http.StatusOK, state)
	case http.MethodPut:
		var request routeCanvasRequest
		if err := decodeJSON(r, &request); err != nil {
			writeAPIError(w, http.StatusBadRequest, "MGP-REQ-001", "请求 JSON 不合法")
			return
		}
		state, err := h.routes.SaveCanvas(r.Context(), flowID, route.SaveCanvasInput{CanvasSnapshot: request.CanvasSnapshot})
		if err != nil {
			status, code, message := routeErrorStatus(err)
			writeAPIError(w, status, code, message)
			return
		}
		h.recordAudit(r, adminUser, "save_canvas", "route_flow", flowID, request, state)
		writeJSON(w, http.StatusOK, state)
	default:
		methodNotAllowed(w, http.MethodGet+", "+http.MethodPut)
	}
}

func (h *Handler) routeRulesHandler(w http.ResponseWriter, r *http.Request, flowID string, adminUser auth.Admin) {
	switch r.Method {
	case http.MethodGet:
		ruleSet, err := h.routes.GetRules(r.Context(), flowID)
		if err != nil {
			status, code, message := routeErrorStatus(err)
			writeAPIError(w, status, code, message)
			return
		}
		writeJSON(w, http.StatusOK, toRouteRulesResponse(ruleSet))
	case http.MethodPut:
		var request routeRulesRequest
		if err := decodeJSON(r, &request); err != nil {
			writeAPIError(w, http.StatusBadRequest, "MGP-REQ-001", "请求 JSON 不合法")
			return
		}
		input := route.SaveRulesInput{Rules: make([]route.RuleInput, 0, len(request.Rules))}
		for _, item := range request.Rules {
			input.Rules = append(input.Rules, route.RuleInput{
				RuleKey:       item.RuleKey,
				SortOrder:     item.SortOrder,
				Name:          item.Name,
				ConditionTree: item.ConditionTree,
				Enabled:       item.Enabled,
				Action: route.ActionInput{
					Targets:           routeActionTargetsInput(item.Action.Targets),
					TemplateVersionID: item.Action.TemplateVersionID,
					ChannelIDs:        item.Action.ChannelIDs,
					RecipientStrategy: item.Action.RecipientStrategy,
					SendDedupeConfig:  item.Action.SendDedupeConfig,
					FailurePolicy:     item.Action.FailurePolicy,
				},
			})
		}
		ruleSet, err := h.routes.SaveRules(r.Context(), flowID, input)
		if err != nil {
			status, code, message := routeErrorStatus(err)
			writeAPIError(w, status, code, message)
			return
		}
		response := toRouteRulesResponse(ruleSet)
		h.recordAudit(r, adminUser, "save_rules", "route_flow", flowID, request, response)
		writeJSON(w, http.StatusOK, response)
	default:
		methodNotAllowed(w, http.MethodGet+", "+http.MethodPut)
	}
}

func (h *Handler) routeVersionRulesHandler(w http.ResponseWriter, r *http.Request, flowID string, versionID string) {
	ruleSet, err := h.routes.GetVersionRules(r.Context(), flowID, versionID)
	if err != nil {
		status, code, message := routeErrorStatus(err)
		writeAPIError(w, status, code, message)
		return
	}
	writeJSON(w, http.StatusOK, toRouteRulesResponse(ruleSet))
}

func (h *Handler) routeReorderHandler(w http.ResponseWriter, r *http.Request, flowID string, adminUser auth.Admin) {
	var request routeReorderRequest
	if err := decodeJSON(r, &request); err != nil {
		writeAPIError(w, http.StatusBadRequest, "MGP-REQ-001", "请求 JSON 不合法")
		return
	}
	ruleSet, err := h.routes.ReorderRules(r.Context(), flowID, route.ReorderRulesInput{RuleKeys: request.RuleKeys})
	if err != nil {
		status, code, message := routeErrorStatus(err)
		writeAPIError(w, status, code, message)
		return
	}
	response := toRouteRulesResponse(ruleSet)
	h.recordAudit(r, adminUser, "reorder_rules", "route_flow", flowID, request, response)
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) routeValidateHandler(w http.ResponseWriter, r *http.Request, flowID string) {
	result, err := h.routes.Validate(r.Context(), flowID)
	if err != nil {
		status, code, message := routeErrorStatus(err)
		writeAPIError(w, status, code, message)
		return
	}
	writeJSON(w, http.StatusOK, routeValidationResponse{
		VersionID: result.VersionID,
		Status:    result.Status,
		Errors:    result.Errors,
	})
}

func (h *Handler) routePublishHandler(w http.ResponseWriter, r *http.Request, flowID string, adminUser auth.Admin) {
	var request routePublishRequest
	if r.Body != nil && r.Body != http.NoBody && r.ContentLength != 0 {
		if err := decodeJSON(r, &request); err != nil {
			writeAPIError(w, http.StatusBadRequest, "MGP-REQ-001", "请求 JSON 不合法")
			return
		}
	}
	version, err := h.routes.Publish(r.Context(), flowID, request.VersionInfo)
	if err != nil {
		status, code, message := routeErrorStatus(err)
		writeAPIError(w, status, code, message)
		return
	}
	response := routeVersionResponseWrapper{Version: toRouteVersionResponse(version)}
	h.recordAudit(r, adminUser, "publish", "route_flow", flowID, request, response)
	writeJSON(w, http.StatusOK, response)
}

type routeVersionResponseWrapper struct {
	Version routeVersionResponse `json:"version"`
}

func (h *Handler) routeSimulateHandler(w http.ResponseWriter, r *http.Request, flowID string) {
	var request routeSimulationRequest
	if err := decodeJSON(r, &request); err != nil {
		writeAPIError(w, http.StatusBadRequest, "MGP-REQ-001", "请求 JSON 不合法")
		return
	}
	result, err := h.routes.Simulate(r.Context(), flowID, route.SimulateInput{Payload: request.Payload})
	if err != nil {
		status, code, message := routeErrorStatus(err)
		writeAPIError(w, status, code, message)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func toRouteFlowResponse(flow route.Flow) routeFlowResponse {
	return routeFlowResponse{
		ID:               flow.ID,
		SourceID:         flow.SourceID,
		Name:             flow.Name,
		Enabled:          flow.Enabled,
		Mode:             string(flow.Mode),
		CurrentVersionID: flow.CurrentVersionID,
		RuleCount:        flow.RuleCount,
		TotalHitCount:    flow.TotalHitCount,
		CreatedAt:        formatTime(flow.CreatedAt),
		UpdatedAt:        formatTime(flow.UpdatedAt),
	}
}

func toRouteVersionResponse(version route.Version) routeVersionResponse {
	return routeVersionResponse{
		ID:               version.ID,
		FlowID:           version.FlowID,
		VersionNo:        version.VersionNo,
		CanvasSnapshot:   nullableRawJSON(version.CanvasSnapshot),
		CompiledRules:    nullableRawJSON(version.CompiledRules),
		ValidationStatus: version.ValidationStatus,
		ValidationErrors: nullableRawJSON(version.ValidationErrors),
		VersionInfo:      routeVersionInfo(version.CompiledRules),
		PublishedAt:      formatOptionalTime(version.PublishedAt),
		CreatedAt:        formatTime(version.CreatedAt),
		UpdatedAt:        formatTime(version.UpdatedAt),
	}
}

func routeVersionInfo(compiledRules json.RawMessage) string {
	var value struct {
		VersionInfo string `json:"version_info"`
	}
	if len(compiledRules) == 0 {
		return ""
	}
	_ = json.Unmarshal(compiledRules, &value)
	return strings.TrimSpace(value.VersionInfo)
}

func toRouteRulesResponse(ruleSet route.RuleSet) routeRulesResponse {
	items := make([]routeRuleResponse, 0, len(ruleSet.Rules))
	for _, item := range ruleSet.Rules {
		items = append(items, routeRuleResponse{
			ID:            item.ID,
			RuleKey:       item.RuleKey,
			SortOrder:     item.SortOrder,
			Name:          item.Name,
			ConditionTree: nullableRawJSON(item.ConditionTree),
			Enabled:       item.Enabled,
			Action: routeActionResponse{
				ID:                item.Action.ID,
				Targets:           toRouteActionTargetResponses(item.Action.Targets),
				TemplateVersionID: item.Action.TemplateVersionID,
				ChannelIDs:        item.Action.ChannelIDs,
				RecipientStrategy: nullableRawJSON(item.Action.RecipientStrategy),
				SendDedupeConfig:  nullableRawJSON(item.Action.SendDedupeConfig),
				FailurePolicy:     nullableRawJSON(item.Action.FailurePolicy),
			},
			HitCount:  item.HitCount,
			LastHitAt: formatOptionalTime(item.LastHitAt),
			CreatedAt: formatTime(item.CreatedAt),
			UpdatedAt: formatTime(item.UpdatedAt),
		})
	}
	return routeRulesResponse{VersionID: ruleSet.VersionID, Rules: items}
}

func routeActionTargetsInput(items []routeActionTargetRequest) []route.ActionTargetInput {
	targets := make([]route.ActionTargetInput, 0, len(items))
	for _, item := range items {
		targets = append(targets, route.ActionTargetInput{
			ChannelID:         item.ChannelID,
			TemplateVersionID: item.TemplateVersionID,
			Enabled:           item.Enabled,
		})
	}
	return targets
}

func toRouteActionTargetResponses(items []route.ActionTarget) []routeActionTargetResponse {
	targets := make([]routeActionTargetResponse, 0, len(items))
	for _, item := range items {
		targets = append(targets, routeActionTargetResponse{
			ID:                item.ID,
			ChannelID:         item.ChannelID,
			TemplateVersionID: item.TemplateVersionID,
			Enabled:           item.Enabled,
			SortOrder:         item.SortOrder,
		})
	}
	return targets
}

func routeErrorStatus(err error) (int, string, string) {
	switch {
	case errors.Is(err, route.ErrInvalidInput):
		return http.StatusBadRequest, "MGP-REQ-001", "请求参数不合法"
	case errors.Is(err, route.ErrEnabledFlowExists):
		return http.StatusConflict, "MGP-ROUTE-003", "路由组已存在"
	case errors.Is(err, route.ErrInvalidConfig):
		return http.StatusConflict, "MGP-ROUTE-002", "路由配置无效"
	case errors.Is(err, route.ErrNotFound):
		return http.StatusNotFound, "MGP-ROUTE-001", "路由组不存在"
	default:
		return http.StatusInternalServerError, "MGP-ROUTE-002", "路由服务异常"
	}
}
