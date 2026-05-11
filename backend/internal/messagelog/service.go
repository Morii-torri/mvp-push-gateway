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
	ID                string
	MessageID         string
	ChannelID         string
	ChannelName       string
	ProviderType      string
	TemplateVersionID string
	RecipientSnapshot json.RawMessage
	RequestSnapshot   json.RawMessage
	ResponseSnapshot  json.RawMessage
	Status            string
	ErrorCode         string
	ErrorMessage      string
	DurationMS        int
	AttemptNo         int
	NextRetryAt       *time.Time
	DeadLetteredAt    *time.Time
	QueuedAt          *time.Time
	StartedAt         *time.Time
	FinishedAt        *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
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
