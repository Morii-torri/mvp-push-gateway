package queue

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

const (
	RoutePlanSubjectPrefix    = "mgp.route_plan"
	SendSubjectPrefix         = "mgp.send"
	ResultSubjectPrefix       = "mgp.result"
	RouteChangedSubjectPrefix = "mgp.route.changed"
)

type PublishResult struct {
	Stream    string
	Sequence  uint64
	Duplicate bool
}

type RoutePlanEvent struct {
	MessageID  string          `json:"message_id"`
	SourceID   string          `json:"source_id"`
	TraceID    string          `json:"trace_id"`
	Headers    json.RawMessage `json:"headers,omitempty"`
	Payload    json.RawMessage `json:"payload,omitempty"`
	ReceivedAt time.Time       `json:"received_at,omitempty"`
}

func (e RoutePlanEvent) Subject() string {
	return RoutePlanSubjectPrefix + "." + subjectToken(e.SourceID, "default")
}

func (e RoutePlanEvent) MessageIDForDedup() string {
	if traceID := strings.TrimSpace(e.TraceID); traceID != "" {
		return traceID
	}
	return strings.TrimSpace(e.MessageID)
}

func (e RoutePlanEvent) Validate() error {
	if e.MessageIDForDedup() == "" || strings.TrimSpace(e.SourceID) == "" {
		return ErrInvalidInput
	}
	return nil
}

type SendMessageEvent struct {
	DeliveryAttemptID string          `json:"delivery_attempt_id"`
	MessageID         string          `json:"message_id,omitempty"`
	SourceID          string          `json:"source_id,omitempty"`
	ChannelID         string          `json:"channel_id"`
	ProviderType      string          `json:"provider_type"`
	TraceID           string          `json:"trace_id,omitempty"`
	MaxAttempts       int             `json:"max_attempts,omitempty"`
	Payload           json.RawMessage `json:"payload,omitempty"`
}

func (e SendMessageEvent) Subject() string {
	return SendSubjectPrefix + "." + subjectToken(e.ProviderType, "default") + "." + subjectToken(e.ChannelID, "default")
}

func (e SendMessageEvent) MessageIDForDedup() string {
	return strings.TrimSpace(e.DeliveryAttemptID)
}

func (e SendMessageEvent) Validate() error {
	if e.MessageIDForDedup() == "" || strings.TrimSpace(e.ChannelID) == "" {
		return ErrInvalidInput
	}
	return nil
}

type DeliveryResultEvent struct {
	DeliveryAttemptID string          `json:"delivery_attempt_id"`
	AttemptNo         int             `json:"attempt_no"`
	Status            string          `json:"status"`
	TraceID           string          `json:"trace_id,omitempty"`
	Payload           json.RawMessage `json:"payload,omitempty"`
}

type RoutePlanChangeEvent struct {
	SourceID string `json:"source_id"`
}

func (e DeliveryResultEvent) Subject() string {
	return ResultSubjectPrefix + "." + subjectToken(e.Status, "unknown")
}

func (e DeliveryResultEvent) MessageIDForDedup() string {
	attemptID := strings.TrimSpace(e.DeliveryAttemptID)
	if attemptID == "" || e.AttemptNo <= 0 {
		return ""
	}
	return attemptID + ":" + strconv.Itoa(e.AttemptNo)
}

func (e DeliveryResultEvent) Validate() error {
	if e.MessageIDForDedup() == "" {
		return ErrInvalidInput
	}
	return nil
}

type RoutePlanMessage struct {
	Event RoutePlanEvent
	Ack   func() error
	Nak   func(time.Duration) error
}

type SendMessage struct {
	Event         SendMessageEvent
	DeliveryCount int
	Ack           func() error
	Nak           func(time.Duration) error
}

type ResultMessage struct {
	Event DeliveryResultEvent
	Ack   func() error
	Nak   func(time.Duration) error
}

type RoutePlanHandler func(context.Context, RoutePlanMessage) error
type SendHandler func(context.Context, SendMessage) error
type ResultHandler func(context.Context, ResultMessage) error

type Broker interface {
	PublishRoutePlan(context.Context, RoutePlanEvent) (PublishResult, error)
	PublishSend(context.Context, SendMessageEvent) (PublishResult, error)
	PublishResult(context.Context, DeliveryResultEvent) (PublishResult, error)
	SubscribeRoutePlan(context.Context, RoutePlanHandler) error
	SubscribeSend(context.Context, SendHandler) error
	SubscribeResult(context.Context, ResultHandler) error
}

func subjectToken(value string, fallback string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		value = fallback
	}
	var builder strings.Builder
	for _, char := range value {
		switch {
		case char >= 'a' && char <= 'z':
			builder.WriteRune(char)
		case char >= '0' && char <= '9':
			builder.WriteRune(char)
		case char == '-' || char == '_':
			builder.WriteRune(char)
		default:
			builder.WriteRune('_')
		}
	}
	cleaned := strings.Trim(builder.String(), "_")
	if cleaned == "" {
		return fallback
	}
	return cleaned
}
