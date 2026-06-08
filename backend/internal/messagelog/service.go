package messagelog

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var (
	ErrNotFound     = errors.New("message log not found")
	ErrInvalidInput = errors.New("invalid message log input")
)

type ListFilter struct {
	TraceID   string
	SourceID  string
	Status    string
	ChannelID string
	Limit     int
	Offset    int
}

type ListResult struct {
	Messages []MessageSummary
	Total    int
	Limit    int
	Offset   int
}

type MessageSummary struct {
	ID                  string
	TraceID             string
	SourceID            string
	SourceName          string
	ReceivedAt          time.Time
	Status              string
	InboundStatus       string
	MatchedFlowID       string
	MatchedFlowName     string
	MatchedRuleIDs      []string
	ErrorCode           string
	ErrorMessage        string
	OutboundStatus      string
	FirstOutboundAt     *time.Time
	LastOutboundAt      *time.Time
	AttemptCount        int
	TargetChannelIDs    []string
	TargetChannelNames  []string
	TargetProviderTypes []string
	DurationMS          int
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type MessageDetail struct {
	MessageSummary
	Headers     json.RawMessage
	Payload     json.RawMessage
	PayloadHash string
	Attempts    []DeliveryAttempt
	Timeline    []TimelineEvent
}

type DeliveryAttempt struct {
	ID                 string
	MessageID          string
	ChannelID          string
	ChannelName        string
	ProviderType       string
	TemplateVersionID  string
	RecipientSnapshot  json.RawMessage
	RequestSnapshot    json.RawMessage
	ResponseSnapshot   json.RawMessage
	TargetContext      json.RawMessage
	RenderedMessage    json.RawMessage
	ResolvedRecipients json.RawMessage
	FinalRequest       json.RawMessage
	UpstreamResponse   json.RawMessage
	Status             string
	ErrorCode          string
	ErrorMessage       string
	DurationMS         int
	AttemptNo          int
	NextRetryAt        *time.Time
	DeadLetteredAt     *time.Time
	QueuedAt           *time.Time
	StartedAt          *time.Time
	FinishedAt         *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type TimelineEvent struct {
	Stage       string    `json:"stage"`
	At          time.Time `json:"at"`
	Status      string    `json:"status"`
	Description string    `json:"description"`
	ErrorCode   string    `json:"error_code,omitempty"`
}

type Store interface {
	ListMessages(ctx context.Context, filter ListFilter) (ListResult, error)
	GetMessage(ctx context.Context, id string) (MessageDetail, error)
}

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

func (s *Service) ListMessages(ctx context.Context, filter ListFilter) (ListResult, error) {
	filter = normalizeFilter(filter)
	result, err := s.store.ListMessages(ctx, filter)
	if err != nil {
		return ListResult{}, err
	}
	for index := range result.Messages {
		result.Messages[index] = normalizeSummary(result.Messages[index])
	}
	return result, nil
}

func (s *Service) GetMessage(ctx context.Context, id string) (MessageDetail, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return MessageDetail{}, ErrInvalidInput
	}
	detail, err := s.store.GetMessage(ctx, id)
	if err != nil {
		return MessageDetail{}, err
	}
	detail.Attempts = deriveAttemptSnapshots(detail.Attempts)
	detail.MessageSummary = normalizeDetailSummary(detail.MessageSummary, detail.Attempts)
	detail.Timeline = buildTimeline(detail)
	return detail, nil
}

func normalizeFilter(filter ListFilter) ListFilter {
	filter.TraceID = strings.TrimSpace(filter.TraceID)
	filter.SourceID = strings.TrimSpace(filter.SourceID)
	filter.Status = strings.TrimSpace(filter.Status)
	filter.ChannelID = strings.TrimSpace(filter.ChannelID)
	if filter.Limit <= 0 {
		filter.Limit = 50
	}
	if filter.Limit > 200 {
		filter.Limit = 200
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	return filter
}

func buildTimeline(detail MessageDetail) []TimelineEvent {
	events := []TimelineEvent{{
		Stage:       "inbound_received",
		At:          detail.ReceivedAt,
		Status:      detail.InboundStatus,
		Description: "入站请求已接收",
		ErrorCode:   detail.ErrorCode,
	}}
	if detail.Status == "no_route" {
		events = append(events, TimelineEvent{
			Stage:       "route_no_match",
			At:          detail.UpdatedAtOrReceivedAt(),
			Status:      detail.Status,
			Description: "路由规划未命中可执行规则",
			ErrorCode:   detail.ErrorCode,
		})
	} else if len(detail.MatchedRuleIDs) > 0 {
		events = append(events, TimelineEvent{
			Stage:       "route_matched",
			At:          detail.UpdatedAtOrReceivedAt(),
			Status:      detail.InboundStatus,
			Description: "路由规划已命中规则",
		})
	}
	for _, attempt := range detail.Attempts {
		if attempt.QueuedAt != nil {
			events = append(events, TimelineEvent{
				Stage:       "delivery_queued",
				At:          *attempt.QueuedAt,
				Status:      attempt.Status,
				Description: "出站投递任务已排队",
			})
		}
		if attempt.StartedAt != nil {
			events = append(events, TimelineEvent{
				Stage:       "delivery_started",
				At:          *attempt.StartedAt,
				Status:      attempt.Status,
				Description: "开始调用上级平台",
			})
		}
		if attempt.FinishedAt != nil {
			events = append(events, TimelineEvent{
				Stage:       "delivery_finished",
				At:          *attempt.FinishedAt,
				Status:      attempt.Status,
				Description: "上级平台调用结束",
				ErrorCode:   attempt.ErrorCode,
			})
		}
	}
	return events
}

func (detail MessageDetail) UpdatedAtOrReceivedAt() time.Time {
	if !detail.UpdatedAt.IsZero() {
		return detail.UpdatedAt
	}
	return detail.ReceivedAt
}

func normalizeDetailSummary(summary MessageSummary, attempts []DeliveryAttempt) MessageSummary {
	summary.OutboundStatus = deriveOutboundStatus(attempts)
	summary.FirstOutboundAt, summary.LastOutboundAt = outboundTimes(attempts)
	if len(attempts) > 0 {
		summary.DurationMS = messageDurationMS(summary.ReceivedAt, summary.LastOutboundAt, summary.DurationMS)
	}
	return normalizeSummary(summary)
}

func deriveOutboundStatus(attempts []DeliveryAttempt) string {
	if len(attempts) == 0 {
		return ""
	}
	allSent := true
	anySent := false
	for _, attempt := range attempts {
		if attempt.DeadLetteredAt != nil {
			return "dead"
		}
		switch attempt.Status {
		case "failed":
			return "failed"
		case "processing":
			return "processing"
		case "queued":
			return "queued"
		case "sent":
			anySent = true
		default:
			allSent = false
		}
	}
	if allSent && anySent {
		return "sent"
	}
	if anySent {
		return "partial_sent"
	}
	return attempts[len(attempts)-1].Status
}

func normalizeSummary(summary MessageSummary) MessageSummary {
	inboundStatus := strings.TrimSpace(summary.InboundStatus)
	if inboundStatus == "" {
		inboundStatus = strings.TrimSpace(summary.Status)
	}
	summary.InboundStatus = inboundStatus
	summary.Status = lifecycleStatus(inboundStatus, summary.OutboundStatus)
	if summary.Status == "no_route" && len(summary.MatchedRuleIDs) == 0 {
		summary.MatchedFlowID = ""
		summary.MatchedFlowName = ""
	}
	return summary
}

func lifecycleStatus(inboundStatus string, outboundStatus string) string {
	outboundStatus = strings.TrimSpace(outboundStatus)
	if outboundStatus != "" {
		switch outboundStatus {
		case "queued":
			return "queued"
		case "processing":
			return "processing"
		case "sent":
			return "sent"
		case "partial_sent":
			return "partial_sent"
		case "failed":
			return "failed"
		case "dead":
			return "dead"
		case "deduped":
			return "deduped"
		case "skipped":
			return "skipped"
		}
	}
	switch strings.TrimSpace(inboundStatus) {
	case "accepted", "deduped", "silenced", "planned", "partial_sent", "sent", "failed", "no_route", "skipped", "queued", "processing", "dead":
		return strings.TrimSpace(inboundStatus)
	default:
		return "accepted"
	}
}

func outboundTimes(attempts []DeliveryAttempt) (*time.Time, *time.Time) {
	var first *time.Time
	var last *time.Time
	for _, attempt := range attempts {
		for _, candidate := range []*time.Time{attempt.QueuedAt, attempt.StartedAt, attempt.FinishedAt} {
			if candidate == nil {
				continue
			}
			if first == nil || candidate.Before(*first) {
				value := candidate.UTC()
				first = &value
			}
			if last == nil || candidate.After(*last) {
				value := candidate.UTC()
				last = &value
			}
		}
	}
	return first, last
}

func messageDurationMS(receivedAt time.Time, lastOutboundAt *time.Time, fallback int) int {
	if receivedAt.IsZero() || lastOutboundAt == nil {
		return fallback
	}
	duration := lastOutboundAt.Sub(receivedAt)
	if duration < 0 {
		return fallback
	}
	return int(duration / time.Millisecond)
}

func deriveAttemptSnapshots(attempts []DeliveryAttempt) []DeliveryAttempt {
	for index := range attempts {
		attempts[index] = deriveAttemptSnapshot(attempts[index])
	}
	return attempts
}

func deriveAttemptSnapshot(attempt DeliveryAttempt) DeliveryAttempt {
	if !hasJSON(attempt.TargetContext) {
		attempt.TargetContext = snapshotField(attempt.RequestSnapshot, "target_context")
		if !hasJSON(attempt.TargetContext) {
			attempt.TargetContext = synthesizedTargetContext(attempt)
		}
	}
	if !hasJSON(attempt.RenderedMessage) {
		attempt.RenderedMessage = snapshotField(attempt.RequestSnapshot, "rendered_message")
		if !hasJSON(attempt.RenderedMessage) {
			attempt.RenderedMessage = nestedSnapshotField(attempt.RequestSnapshot, "send", "body")
		}
	}
	if !hasJSON(attempt.ResolvedRecipients) {
		attempt.ResolvedRecipients = snapshotField(attempt.RequestSnapshot, "resolved_recipients")
		if !hasJSON(attempt.ResolvedRecipients) {
			attempt.ResolvedRecipients = normalizeRecipients(nestedSnapshotField(attempt.RequestSnapshot, "send", "recipient"))
		}
		if !hasJSON(attempt.ResolvedRecipients) {
			attempt.ResolvedRecipients = normalizeRecipients(snapshotField(attempt.RecipientSnapshot, "recipient"))
		}
	}
	if !hasJSON(attempt.FinalRequest) {
		attempt.FinalRequest = snapshotField(attempt.RequestSnapshot, "final_request")
		if !hasJSON(attempt.FinalRequest) {
			attempt.FinalRequest = snapshotField(attempt.RequestSnapshot, "send")
		}
	}
	if !hasJSON(attempt.UpstreamResponse) {
		attempt.UpstreamResponse = snapshotField(attempt.ResponseSnapshot, "upstream_response")
		if !hasJSON(attempt.UpstreamResponse) {
			attempt.UpstreamResponse = snapshotField(attempt.ResponseSnapshot, "send")
		}
	}
	return attempt
}

func snapshotField(snapshot json.RawMessage, key string) json.RawMessage {
	var fields map[string]json.RawMessage
	if !decodeSnapshotObject(snapshot, &fields) {
		return nil
	}
	return cloneRawJSON(fields[key])
}

func nestedSnapshotField(snapshot json.RawMessage, parent string, key string) json.RawMessage {
	parentRaw := snapshotField(snapshot, parent)
	var fields map[string]json.RawMessage
	if !decodeSnapshotObject(parentRaw, &fields) {
		return nil
	}
	return cloneRawJSON(fields[key])
}

func decodeSnapshotObject(snapshot json.RawMessage, target *map[string]json.RawMessage) bool {
	if !hasJSON(snapshot) {
		return false
	}
	if err := json.Unmarshal(snapshot, target); err != nil {
		return false
	}
	return *target != nil
}

func cloneRawJSON(raw json.RawMessage) json.RawMessage {
	if !hasJSON(raw) {
		return nil
	}
	return append(json.RawMessage(nil), raw...)
}

func hasJSON(raw json.RawMessage) bool {
	trimmed := strings.TrimSpace(string(raw))
	return trimmed != "" && trimmed != "null"
}

func synthesizedTargetContext(attempt DeliveryAttempt) json.RawMessage {
	context := map[string]string{}
	if strings.TrimSpace(attempt.ChannelID) != "" {
		context["channel_id"] = attempt.ChannelID
	}
	if strings.TrimSpace(attempt.ChannelName) != "" {
		context["channel_name"] = attempt.ChannelName
	}
	if strings.TrimSpace(attempt.ProviderType) != "" {
		context["provider_type"] = attempt.ProviderType
	}
	if strings.TrimSpace(attempt.TemplateVersionID) != "" {
		context["template_version_id"] = attempt.TemplateVersionID
	}
	if len(context) == 0 {
		return nil
	}
	raw, err := json.Marshal(context)
	if err != nil {
		return nil
	}
	return raw
}

func normalizeRecipients(raw json.RawMessage) json.RawMessage {
	if !hasJSON(raw) {
		return nil
	}
	var array []json.RawMessage
	if err := json.Unmarshal(raw, &array); err == nil {
		return cloneRawJSON(raw)
	}
	wrapped, err := json.Marshal([]json.RawMessage{raw})
	if err != nil {
		return nil
	}
	return wrapped
}
