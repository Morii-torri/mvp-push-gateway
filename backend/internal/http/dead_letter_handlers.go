package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"mvp-push-gateway/backend/internal/auth"
	"mvp-push-gateway/backend/internal/deadletter"
)

type deadLettersResponse struct {
	DeadLetters []deadLetterResponse `json:"dead_letters"`
	Total       int                  `json:"total"`
	Limit       int                  `json:"limit"`
	Offset      int                  `json:"offset"`
}

type deadLetterResponse struct {
	ID             string          `json:"id"`
	JobID          string          `json:"job_id"`
	Type           string          `json:"type"`
	Payload        json.RawMessage `json:"payload"`
	ChannelID      string          `json:"channel_id"`
	ChannelName    string          `json:"channel_name"`
	ProviderType   string          `json:"provider_type"`
	ErrorCode      string          `json:"error_code"`
	ErrorMessage   string          `json:"error_message"`
	Attempts       int             `json:"attempts"`
	DeadLetteredAt string          `json:"dead_lettered_at"`
	ReplayedAt     *string         `json:"replayed_at"`
	HandledAt      *string         `json:"handled_at"`
	HandledReason  string          `json:"handled_reason"`
	CreatedAt      string          `json:"created_at"`
}

type deadLetterBatchRequest struct {
	IDs       []string `json:"ids"`
	All       bool     `json:"all"`
	Status    string   `json:"status"`
	ChannelID string   `json:"channel_id"`
	Reason    string   `json:"reason"`
}

type deadLetterBatchResponse struct {
	Result deadletter.BatchResult `json:"result"`
}

func (h *Handler) deadLettersHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	if !h.requireDeadLetterService(w) {
		return
	}
	if _, ok := h.authenticateRequest(w, r); !ok {
		return
	}
	result, err := h.deadLetters.ListDeadLetters(r.Context(), deadletter.ListFilter{
		Status:    r.URL.Query().Get("status"),
		ChannelID: r.URL.Query().Get("channel_id"),
		Limit:     queryInt(r, "limit"),
		Offset:    queryInt(r, "offset"),
	})
	if err != nil {
		status, code, message := deadLetterErrorStatus(err)
		writeAPIError(w, status, code, message)
		return
	}
	response := deadLettersResponse{DeadLetters: make([]deadLetterResponse, 0, len(result.Items)), Total: result.Total, Limit: result.Limit, Offset: result.Offset}
	for _, item := range result.Items {
		response.DeadLetters = append(response.DeadLetters, toDeadLetterResponse(item))
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) deadLetterActionHandler(w http.ResponseWriter, r *http.Request) {
	action := strings.Trim(strings.TrimPrefix(r.URL.Path, h.cfg.Server.APIPrefix+"/dead-letters/"), "/")
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	if !h.requireDeadLetterService(w) {
		return
	}
	adminUser, ok := h.authenticateRequest(w, r)
	if !ok {
		return
	}

	switch action {
	case "batch-replay":
		h.deadLetterBatchReplayHandler(w, r, adminUser)
	case "batch-handle":
		h.deadLetterBatchHandleHandler(w, r, adminUser)
	case "batch-delete":
		h.deadLetterBatchDeleteHandler(w, r, adminUser)
	default:
		writeAPIError(w, http.StatusNotFound, "MGP-DEAD-001", "死信任务不存在")
	}
}

func (h *Handler) deadLetterBatchReplayHandler(w http.ResponseWriter, r *http.Request, adminUser auth.Admin) {
	var request deadLetterBatchRequest
	if err := decodeJSON(r, &request); err != nil {
		writeAPIError(w, http.StatusBadRequest, "MGP-REQ-001", "请求 JSON 不合法")
		return
	}
	result, err := h.deadLetters.ReplayDeadLetters(r.Context(), deadletter.BatchInput{
		IDs:       request.IDs,
		All:       request.All,
		Status:    request.Status,
		ChannelID: request.ChannelID,
	})
	if err != nil {
		status, code, message := deadLetterErrorStatus(err)
		writeAPIError(w, status, code, message)
		return
	}
	response := deadLetterBatchResponse{Result: result}
	h.recordAudit(r, adminUser, "batch_replay", "dead_letter", strings.Join(result.IDs, ","), map[string]any{"count": result.Processed}, response)
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) deadLetterBatchHandleHandler(w http.ResponseWriter, r *http.Request, adminUser auth.Admin) {
	var request deadLetterBatchRequest
	if err := decodeJSON(r, &request); err != nil {
		writeAPIError(w, http.StatusBadRequest, "MGP-REQ-001", "请求 JSON 不合法")
		return
	}
	result, err := h.deadLetters.MarkDeadLettersHandled(r.Context(), deadletter.HandleInput{
		IDs:       request.IDs,
		All:       request.All,
		Status:    request.Status,
		ChannelID: request.ChannelID,
		Reason:    request.Reason,
	})
	if err != nil {
		status, code, message := deadLetterErrorStatus(err)
		writeAPIError(w, status, code, message)
		return
	}
	response := deadLetterBatchResponse{Result: result}
	h.recordAudit(r, adminUser, "batch_handle", "dead_letter", strings.Join(result.IDs, ","), map[string]any{"count": result.Processed}, response)
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) deadLetterBatchDeleteHandler(w http.ResponseWriter, r *http.Request, adminUser auth.Admin) {
	var request deadLetterBatchRequest
	if err := decodeJSON(r, &request); err != nil {
		writeAPIError(w, http.StatusBadRequest, "MGP-REQ-001", "请求 JSON 不合法")
		return
	}
	result, err := h.deadLetters.DeleteDeadLetters(r.Context(), deadletter.BatchInput{
		IDs:       request.IDs,
		All:       request.All,
		Status:    request.Status,
		ChannelID: request.ChannelID,
	})
	if err != nil {
		status, code, message := deadLetterErrorStatus(err)
		writeAPIError(w, status, code, message)
		return
	}
	response := deadLetterBatchResponse{Result: result}
	h.recordAudit(r, adminUser, "batch_delete", "dead_letter", strings.Join(result.IDs, ","), map[string]any{"count": result.Processed}, response)
	writeJSON(w, http.StatusOK, response)
}

func toDeadLetterResponse(item deadletter.Job) deadLetterResponse {
	return deadLetterResponse{
		ID:             item.ID,
		JobID:          item.JobID,
		Type:           item.Type,
		Payload:        defaultRawJSON(item.Payload),
		ChannelID:      item.ChannelID,
		ChannelName:    item.ChannelName,
		ProviderType:   item.ProviderType,
		ErrorCode:      item.ErrorCode,
		ErrorMessage:   item.ErrorMessage,
		Attempts:       item.Attempts,
		DeadLetteredAt: formatTime(item.DeadLetteredAt),
		ReplayedAt:     formatOptionalTime(item.ReplayedAt),
		HandledAt:      formatOptionalTime(item.HandledAt),
		HandledReason:  item.HandledReason,
		CreatedAt:      formatTime(item.CreatedAt),
	}
}

func deadLetterErrorStatus(err error) (int, string, string) {
	switch {
	case errors.Is(err, deadletter.ErrInvalidInput):
		return http.StatusBadRequest, "MGP-REQ-001", "请求参数不合法"
	case errors.Is(err, deadletter.ErrNotFound):
		return http.StatusNotFound, "MGP-DEAD-001", "死信任务不存在"
	default:
		return http.StatusInternalServerError, "MGP-DEAD-999", "死信服务内部错误"
	}
}
