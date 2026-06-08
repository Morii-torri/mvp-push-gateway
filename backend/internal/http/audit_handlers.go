package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"mvp-push-gateway/backend/internal/audit"
)

type auditLogsResponse struct {
	AuditLogs []auditLogResponse `json:"audit_logs"`
	Total     int                `json:"total"`
	Limit     int                `json:"limit"`
	Offset    int                `json:"offset"`
}

type auditLogBody struct {
	AuditLog auditLogResponse `json:"audit_log"`
}

type auditLogResponse struct {
	ID               string          `json:"id"`
	ActorAdminID     string          `json:"actor_admin_id"`
	ActorUsername    string          `json:"actor_username"`
	Action           string          `json:"action"`
	ResourceType     string          `json:"resource_type"`
	ResourceID       string          `json:"resource_id"`
	RequestSnapshot  json.RawMessage `json:"request_snapshot"`
	ResponseSnapshot json.RawMessage `json:"response_snapshot"`
	IPAddress        string          `json:"ip_address"`
	UserAgent        string          `json:"user_agent"`
	CreatedAt        string          `json:"created_at"`
}

func (h *Handler) auditLogsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	if !h.requireAuditService(w) {
		return
	}
	if _, ok := h.authenticateRequest(w, r); !ok {
		return
	}
	result, err := h.audit.ListLogs(r.Context(), audit.ListFilter{
		Actor:        r.URL.Query().Get("actor"),
		Action:       r.URL.Query().Get("action"),
		ResourceType: r.URL.Query().Get("resource_type"),
		Limit:        queryInt(r, "limit"),
		Offset:       queryInt(r, "offset"),
	})
	if err != nil {
		status, code, message := auditErrorStatus(err)
		writeAPIError(w, status, code, message)
		return
	}
	response := auditLogsResponse{AuditLogs: make([]auditLogResponse, 0, len(result.Logs)), Total: result.Total, Limit: result.Limit, Offset: result.Offset}
	for _, item := range result.Logs {
		response.AuditLogs = append(response.AuditLogs, toAuditLogListResponse(item))
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) auditLogDetailHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	id := singleIDFromPath(r.URL.Path, h.cfg.Server.APIPrefix+"/audit-logs/")
	if id == "" {
		writeAPIError(w, http.StatusNotFound, "MGP-AUDIT-001", "审计记录不存在")
		return
	}
	if !h.requireAuditService(w) {
		return
	}
	if _, ok := h.authenticateRequest(w, r); !ok {
		return
	}
	item, err := h.audit.GetLog(r.Context(), id)
	if err != nil {
		status, code, message := auditErrorStatus(err)
		writeAPIError(w, status, code, message)
		return
	}
	writeJSON(w, http.StatusOK, auditLogBody{AuditLog: toAuditLogResponse(item)})
}

func toAuditLogResponse(item audit.Log) auditLogResponse {
	return auditLogResponse{
		ID:               item.ID,
		ActorAdminID:     item.ActorAdminID,
		ActorUsername:    item.ActorUsername,
		Action:           item.Action,
		ResourceType:     item.ResourceType,
		ResourceID:       item.ResourceID,
		RequestSnapshot:  defaultRawJSON(item.RequestSnapshot),
		ResponseSnapshot: defaultRawJSON(item.ResponseSnapshot),
		IPAddress:        item.IPAddress,
		UserAgent:        item.UserAgent,
		CreatedAt:        formatTime(item.CreatedAt),
	}
}

func toAuditLogListResponse(item audit.Log) auditLogResponse {
	response := toAuditLogResponse(item)
	response.RequestSnapshot = json.RawMessage(`{}`)
	response.ResponseSnapshot = json.RawMessage(`{}`)
	return response
}

func auditErrorStatus(err error) (int, string, string) {
	switch {
	case errors.Is(err, audit.ErrInvalidInput):
		return http.StatusBadRequest, "MGP-REQ-001", "请求参数不合法"
	case errors.Is(err, audit.ErrNotFound):
		return http.StatusNotFound, "MGP-AUDIT-001", "审计记录不存在"
	default:
		return http.StatusInternalServerError, "MGP-AUDIT-999", "审计服务内部错误"
	}
}
