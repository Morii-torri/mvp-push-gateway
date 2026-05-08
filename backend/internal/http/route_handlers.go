package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

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
	PublishedAt      *string         `json:"published_at"`
	CreatedAt        string          `json:"created_at"`
	UpdatedAt        string          `json:"updated_at"`
}

type routeCanvasRequest struct {
	CanvasSnapshot json.RawMessage `json:"canvas_snapshot"`
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
	TemplateVersionID string          `json:"template_version_id"`
	ChannelIDs        []string        `json:"channel_ids"`
	RecipientStrategy json.RawMessage `json:"recipient_strategy"`
	SendDedupeConfig  json.RawMessage `json:"send_dedupe_config"`
	FailurePolicy     json.RawMessage `json:"failure_policy"`
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
	ID                string          `json:"id"`
	TemplateVersionID string          `json:"template_version_id"`
	ChannelIDs        []string        `json:"channel_ids"`
	RecipientStrategy json.RawMessage `json:"recipient_strategy"`
	SendDedupeConfig  json.RawMessage `json:"send_dedupe_config"`
	FailurePolicy     json.RawMessage `json:"failure_policy"`
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
	if _, ok := h.authenticateRequest(w, r); !ok {
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
		writeJSON(w, http.StatusCreated, routeFlowDetailResponse{Flow: toRouteFlowResponse(created)})
	default:
		methodNotAllowed(w, http.MethodGet+", "+http.MethodPost)
	}
}

func (h *Handler) routeFlowDetailHandler(w http.ResponseWriter, r *http.Request) {
	if !h.requireRouteService(w) {
		return
	}
	if _, ok := h.authenticateRequest(w, r); !ok {
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
		h.routeFlowResourceHandler(w, r, flowID)
		return
	}

	switch parts[1] {
	case "versions":
		if len(parts) == 2 && r.Method == http.MethodGet {
			h.routeVersionsHandler(w, r, flowID)
			return
		}
		if len(parts) == 4 && parts[3] == "activate" && r.Method == http.MethodPost {
			h.routeActivateVersionHandler(w, r, flowID, parts[2])
			return
		}
	case "canvas":
		h.routeCanvasHandler(w, r, flowID)
		return
	case "rules":
		if len(parts) == 2 {
			h.routeRulesHandler(w, r, flowID)
			return
		}
		if len(parts) == 3 && parts[2] == "reorder" && r.Method == http.MethodPut {
			h.routeReorderHandler(w, r, flowID)
			return
		}
	case "validate":
		if len(parts) == 2 && r.Method == http.MethodPost {
			h.routeValidateHandler(w, r, flowID)
			return
		}
	case "publish":
		if len(parts) == 2 && r.Method == http.MethodPost {
			h.routePublishHandler(w, r, flowID)
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

func (h *Handler) routeFlowResourceHandler(w http.ResponseWriter, r *http.Request, flowID string) {
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
		writeJSON(w, http.StatusOK, routeFlowDetailResponse{Flow: toRouteFlowResponse(updated)})
	case http.MethodDelete:
		if err := h.routes.DeleteFlow(r.Context(), flowID); err != nil {
			status, code, message := routeErrorStatus(err)
			writeAPIError(w, status, code, message)
			return
		}
		writeJSON(w, http.StatusOK, okResponse{OK: true})
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

func (h *Handler) routeActivateVersionHandler(w http.ResponseWriter, r *http.Request, flowID string, versionID string) {
	updated, err := h.routes.ActivateVersion(r.Context(), flowID, versionID)
	if err != nil {
		status, code, message := routeErrorStatus(err)
		writeAPIError(w, status, code, message)
		return
	}
	writeJSON(w, http.StatusOK, routeFlowDetailResponse{Flow: toRouteFlowResponse(updated)})
}

func (h *Handler) routeCanvasHandler(w http.ResponseWriter, r *http.Request, flowID string) {
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
		writeJSON(w, http.StatusOK, state)
	default:
		methodNotAllowed(w, http.MethodGet+", "+http.MethodPut)
	}
}

func (h *Handler) routeRulesHandler(w http.ResponseWriter, r *http.Request, flowID string) {
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
		writeJSON(w, http.StatusOK, toRouteRulesResponse(ruleSet))
	default:
		methodNotAllowed(w, http.MethodGet+", "+http.MethodPut)
	}
}

func (h *Handler) routeReorderHandler(w http.ResponseWriter, r *http.Request, flowID string) {
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
	writeJSON(w, http.StatusOK, toRouteRulesResponse(ruleSet))
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

func (h *Handler) routePublishHandler(w http.ResponseWriter, r *http.Request, flowID string) {
	version, err := h.routes.Publish(r.Context(), flowID)
	if err != nil {
		status, code, message := routeErrorStatus(err)
		writeAPIError(w, status, code, message)
		return
	}
	writeJSON(w, http.StatusOK, routeVersionResponseWrapper{Version: toRouteVersionResponse(version)})
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
		PublishedAt:      formatOptionalTime(version.PublishedAt),
		CreatedAt:        formatTime(version.CreatedAt),
		UpdatedAt:        formatTime(version.UpdatedAt),
	}
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
