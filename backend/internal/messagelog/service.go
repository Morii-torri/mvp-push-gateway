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
	MatchedFlowID       string
	MatchedFlowName     string
	MatchedRuleIDs      []string
	ErrorCode           string
	ErrorMessage        string
	OutboundStatus      string
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
	return s.store.ListMessages(ctx, filter)
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
		Status:      detail.Status,
		Description: "入站请求已接收",
		ErrorCode:   detail.ErrorCode,
	}}
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
