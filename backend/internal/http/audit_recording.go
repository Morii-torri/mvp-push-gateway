package httpapi

import (
	"encoding/json"
	"net/http"

	"mvp-push-gateway/backend/internal/audit"
	"mvp-push-gateway/backend/internal/auth"
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
		IPAddress:        clientIP(r),
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
