package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"mvp-push-gateway/backend/internal/audit"
	"mvp-push-gateway/backend/internal/auth"
	"mvp-push-gateway/backend/internal/source"
)

func (h *Handler) recordAudit(r *http.Request, adminUser auth.Admin, action string, resourceType string, resourceID string, request any, response any) {
	if h.audit == nil {
		return
	}
	requestSnapshot := mustMarshalAuditSnapshot(request)
	responseSnapshot := mustMarshalAuditSnapshot(response)
	_, _ = h.audit.Record(r.Context(), audit.RecordInput{
		ActorAdminID:     adminUser.ID,
		ActorUsername:    adminUser.Username,
		Action:           action,
		ResourceType:     resourceType,
		ResourceID:       resourceID,
		RequestSnapshot:  requestSnapshot,
		ResponseSnapshot: responseSnapshot,
		IPAddress:        h.clientIP(r),
		UserAgent:        r.UserAgent(),
	})
}

func (h *Handler) recordSourceRejectAudit(r *http.Request, sourceCode string, err error, status int, code string) {
	if h.audit == nil {
		return
	}
	action := ""
	switch {
	case errors.Is(err, source.ErrUnauthorized):
		action = "reject_unauthorized"
	case errors.Is(err, source.ErrIPNotAllowed):
		action = "reject_ip_not_allowed"
	case errors.Is(err, source.ErrRateLimited):
		action = "reject_rate_limited"
	case errors.Is(err, source.ErrPayloadTooLarge):
		action = "reject_payload_too_large"
	default:
		return
	}
	requestSnapshot := mustMarshalAuditSnapshot(map[string]string{
		"source_code": strings.TrimSpace(sourceCode),
		"method":      r.Method,
		"path":        r.URL.Path,
	})
	responseSnapshot := mustMarshalAuditSnapshot(map[string]any{
		"status":     status,
		"error_code": code,
	})
	_, _ = h.audit.Record(r.Context(), audit.RecordInput{
		Action:           action,
		ResourceType:     "source_ingest",
		ResourceID:       strings.TrimSpace(sourceCode),
		RequestSnapshot:  requestSnapshot,
		ResponseSnapshot: responseSnapshot,
		IPAddress:        h.clientIP(r),
		UserAgent:        r.UserAgent(),
	})
}

func mustMarshalAuditSnapshot(value any) json.RawMessage {
	if value == nil {
		return json.RawMessage(`{}`)
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return raw
}
