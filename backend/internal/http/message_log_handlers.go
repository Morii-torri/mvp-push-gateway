package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"mvp-push-gateway/backend/internal/messagelog"
)

type messagesResponse struct {
	Messages []messageSummaryResponse `json:"messages"`
	Total    int                      `json:"total"`
	Limit    int                      `json:"limit"`
	Offset   int                      `json:"offset"`
}

type messageBody struct {
	Message messageDetailResponse `json:"message"`
}

type messageDeliveriesResponse struct {
	Deliveries []deliveryAttemptResponse `json:"deliveries"`
}

type messageSummaryResponse struct {
	ID                  string   `json:"id"`
	TraceID             string   `json:"trace_id"`
	SourceID            string   `json:"source_id"`
	SourceName          string   `json:"source_name"`
	ReceivedAt          string   `json:"received_at"`
	Status              string   `json:"status"`
	InboundStatus       string   `json:"inbound_status"`
	MatchedFlowID       string   `json:"matched_flow_id"`
	MatchedFlowName     string   `json:"matched_flow_name"`
	MatchedRuleIDs      []string `json:"matched_rule_ids"`
	ErrorCode           string   `json:"error_code"`
	ErrorMessage        string   `json:"error_message"`
	OutboundStatus      string   `json:"outbound_status"`
	FirstOutboundAt     *string  `json:"first_outbound_at"`
	LastOutboundAt      *string  `json:"last_outbound_at"`
	AttemptCount        int      `json:"attempt_count"`
	TargetChannelIDs    []string `json:"target_channel_ids"`
	TargetChannelNames  []string `json:"target_channel_names"`
	TargetProviderTypes []string `json:"target_provider_types"`
	DurationMS          int      `json:"duration_ms"`
	CreatedAt           string   `json:"created_at"`
	UpdatedAt           string   `json:"updated_at"`
}

type messageDetailResponse struct {
	messageSummaryResponse
	Headers     json.RawMessage            `json:"headers"`
	Payload     json.RawMessage            `json:"payload"`
	PayloadHash string                     `json:"payload_hash"`
	Attempts    []deliveryAttemptResponse  `json:"attempts"`
	Timeline    []messagelog.TimelineEvent `json:"timeline"`
}

type deliveryAttemptResponse struct {
	ID                 string          `json:"id"`
	MessageID          string          `json:"message_id"`
	ChannelID          string          `json:"channel_id"`
	ChannelName        string          `json:"channel_name"`
	ProviderType       string          `json:"provider_type"`
	TemplateVersionID  string          `json:"template_version_id"`
	RecipientSnapshot  json.RawMessage `json:"recipient_snapshot"`
	RequestSnapshot    json.RawMessage `json:"request_snapshot"`
	ResponseSnapshot   json.RawMessage `json:"response_snapshot"`
	TargetContext      json.RawMessage `json:"target_context"`
	RenderedMessage    json.RawMessage `json:"rendered_message"`
	ResolvedRecipients json.RawMessage `json:"resolved_recipients"`
	FinalRequest       json.RawMessage `json:"final_request"`
	UpstreamResponse   json.RawMessage `json:"upstream_response"`
	Status             string          `json:"status"`
	ErrorCode          string          `json:"error_code"`
	ErrorMessage       string          `json:"error_message"`
	DurationMS         int             `json:"duration_ms"`
	AttemptNo          int             `json:"attempt_no"`
	NextRetryAt        *string         `json:"next_retry_at"`
	DeadLetteredAt     *string         `json:"dead_lettered_at"`
	QueuedAt           *string         `json:"queued_at"`
	StartedAt          *string         `json:"started_at"`
	FinishedAt         *string         `json:"finished_at"`
	CreatedAt          string          `json:"created_at"`
	UpdatedAt          string          `json:"updated_at"`
}

func (h *Handler) messagesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	if !h.requireMessageLogService(w) {
		return
	}
	if _, ok := h.authenticateRequest(w, r); !ok {
		return
	}
	result, err := h.messageLogs.ListMessages(r.Context(), messagelog.ListFilter{
		TraceID:        r.URL.Query().Get("trace_id"),
		Keyword:        r.URL.Query().Get("keyword"),
		SourceID:       r.URL.Query().Get("source_id"),
		SourceName:     r.URL.Query().Get("source_name"),
		Status:         r.URL.Query().Get("status"),
		ChannelID:      r.URL.Query().Get("channel_id"),
		TargetProvider: r.URL.Query().Get("target_provider"),
		ErrorCode:      r.URL.Query().Get("error_code"),
		Limit:          queryInt(r, "limit"),
		Offset:         queryInt(r, "offset"),
	})
	if err != nil {
		status, code, message := messageLogErrorStatus(err)
		writeAPIError(w, status, code, message)
		return
	}
	response := messagesResponse{Messages: make([]messageSummaryResponse, 0, len(result.Messages)), Total: result.Total, Limit: result.Limit, Offset: result.Offset}
	for _, item := range result.Messages {
		response.Messages = append(response.Messages, toMessageSummaryResponse(item))
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) messageDetailHandler(w http.ResponseWriter, r *http.Request) {
	id := singleIDFromPath(r.URL.Path, h.cfg.Server.APIPrefix+"/messages/")
	deliveriesPath := stringsTrimDeliveries(r.URL.Path, h.cfg.Server.APIPrefix+"/messages/")
	if deliveriesPath != "" {
		id = deliveriesPath
	}
	if id == "" {
		writeAPIError(w, http.StatusNotFound, "MGP-MSG-001", "消息记录不存在")
		return
	}
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	if !h.requireMessageLogService(w) {
		return
	}
	if _, ok := h.authenticateRequest(w, r); !ok {
		return
	}
	detail, err := h.messageLogs.GetMessage(r.Context(), id)
	if err != nil {
		status, code, message := messageLogErrorStatus(err)
		writeAPIError(w, status, code, message)
		return
	}
	if deliveriesPath != "" {
		response := messageDeliveriesResponse{Deliveries: make([]deliveryAttemptResponse, 0, len(detail.Attempts))}
		for _, attempt := range detail.Attempts {
			response.Deliveries = append(response.Deliveries, toDeliveryAttemptResponse(attempt))
		}
		writeJSON(w, http.StatusOK, response)
		return
	}
	writeJSON(w, http.StatusOK, messageBody{Message: toMessageDetailResponse(detail)})
}

func stringsTrimDeliveries(path string, prefix string) string {
	rest := strings.Trim(strings.TrimPrefix(path, prefix), "/")
	parts := strings.Split(rest, "/")
	if len(parts) == 2 && parts[0] != "" && parts[1] == "deliveries" {
		return parts[0]
	}
	return ""
}

func queryInt(r *http.Request, key string) int {
	value := r.URL.Query().Get(key)
	if value == "" {
		return 0
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return parsed
}

func toMessageSummaryResponse(item messagelog.MessageSummary) messageSummaryResponse {
	return messageSummaryResponse{
		ID:                  item.ID,
		TraceID:             item.TraceID,
		SourceID:            item.SourceID,
		SourceName:          item.SourceName,
		ReceivedAt:          formatTime(item.ReceivedAt),
		Status:              item.Status,
		InboundStatus:       item.InboundStatus,
		MatchedFlowID:       item.MatchedFlowID,
		MatchedFlowName:     item.MatchedFlowName,
		MatchedRuleIDs:      item.MatchedRuleIDs,
		ErrorCode:           item.ErrorCode,
		ErrorMessage:        item.ErrorMessage,
		OutboundStatus:      item.OutboundStatus,
		FirstOutboundAt:     formatOptionalTime(item.FirstOutboundAt),
		LastOutboundAt:      formatOptionalTime(item.LastOutboundAt),
		AttemptCount:        item.AttemptCount,
		TargetChannelIDs:    item.TargetChannelIDs,
		TargetChannelNames:  item.TargetChannelNames,
		TargetProviderTypes: item.TargetProviderTypes,
		DurationMS:          item.DurationMS,
		CreatedAt:           formatTime(item.CreatedAt),
		UpdatedAt:           formatTime(item.UpdatedAt),
	}
}

func toMessageDetailResponse(item messagelog.MessageDetail) messageDetailResponse {
	attempts := make([]deliveryAttemptResponse, 0, len(item.Attempts))
	for _, attempt := range item.Attempts {
		attempts = append(attempts, toDeliveryAttemptResponse(attempt))
	}
	return messageDetailResponse{
		messageSummaryResponse: toMessageSummaryResponse(item.MessageSummary),
		Headers:                defaultRawJSON(item.Headers),
		Payload:                defaultRawJSON(item.Payload),
		PayloadHash:            item.PayloadHash,
		Attempts:               attempts,
		Timeline:               item.Timeline,
	}
}

func toDeliveryAttemptResponse(item messagelog.DeliveryAttempt) deliveryAttemptResponse {
	return deliveryAttemptResponse{
		ID:                 item.ID,
		MessageID:          item.MessageID,
		ChannelID:          item.ChannelID,
		ChannelName:        item.ChannelName,
		ProviderType:       item.ProviderType,
		TemplateVersionID:  item.TemplateVersionID,
		RecipientSnapshot:  defaultRawJSON(item.RecipientSnapshot),
		RequestSnapshot:    defaultRawJSON(item.RequestSnapshot),
		ResponseSnapshot:   defaultRawJSON(item.ResponseSnapshot),
		TargetContext:      defaultRawJSON(item.TargetContext),
		RenderedMessage:    defaultRawJSON(item.RenderedMessage),
		ResolvedRecipients: defaultRawJSONArray(item.ResolvedRecipients),
		FinalRequest:       defaultRawJSON(item.FinalRequest),
		UpstreamResponse:   defaultRawJSON(item.UpstreamResponse),
		Status:             item.Status,
		ErrorCode:          item.ErrorCode,
		ErrorMessage:       item.ErrorMessage,
		DurationMS:         item.DurationMS,
		AttemptNo:          item.AttemptNo,
		NextRetryAt:        formatOptionalTime(item.NextRetryAt),
		DeadLetteredAt:     formatOptionalTime(item.DeadLetteredAt),
		QueuedAt:           formatOptionalTime(item.QueuedAt),
		StartedAt:          formatOptionalTime(item.StartedAt),
		FinishedAt:         formatOptionalTime(item.FinishedAt),
		CreatedAt:          formatTime(item.CreatedAt),
		UpdatedAt:          formatTime(item.UpdatedAt),
	}
}

func defaultRawJSONArray(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage(`[]`)
	}
	return raw
}

func messageLogErrorStatus(err error) (int, string, string) {
	switch {
	case errors.Is(err, messagelog.ErrInvalidInput):
		return http.StatusBadRequest, "MGP-REQ-001", "请求参数不合法"
	case errors.Is(err, messagelog.ErrNotFound):
		return http.StatusNotFound, "MGP-MSG-001", "消息记录不存在"
	default:
		return http.StatusInternalServerError, "MGP-MSG-999", "消息日志服务内部错误"
	}
}
