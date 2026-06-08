package delivery

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"mvp-push-gateway/backend/internal/queue"
)

type DeliveryResultEvent struct {
	Action            ResultAction    `json:"action"`
	JobID             string          `json:"job_id,omitempty"`
	WorkerID          string          `json:"worker_id,omitempty"`
	AttemptID         string          `json:"delivery_attempt_id"`
	MessageID         string          `json:"message_id,omitempty"`
	SourceID          string          `json:"source_id,omitempty"`
	ChannelID         string          `json:"channel_id,omitempty"`
	TemplateVersionID string          `json:"template_version_id,omitempty"`
	RecipientSnapshot json.RawMessage `json:"recipient_snapshot,omitempty"`
	DeliveryCreatedAt time.Time       `json:"delivery_created_at,omitempty"`
	TraceID           string          `json:"trace_id,omitempty"`
	AttemptNo         int             `json:"attempt_no"`
	Status            Status          `json:"status"`
	ErrorCode         string          `json:"error_code,omitempty"`
	ErrorMessage      string          `json:"error_message,omitempty"`
	RequestSnapshot   json.RawMessage `json:"request_snapshot,omitempty"`
	ResponseSnapshot  json.RawMessage `json:"response_snapshot,omitempty"`
	DurationMS        int             `json:"duration_ms"`
	RetryAt           *time.Time      `json:"retry_at,omitempty"`
	FinishedAt        time.Time       `json:"finished_at"`
	InboundHeaders    json.RawMessage `json:"inbound_headers,omitempty"`
	InboundPayload    json.RawMessage `json:"inbound_payload,omitempty"`
	InboundReceivedAt time.Time       `json:"inbound_received_at,omitempty"`
}

type ResultAction string

const (
	ResultActionComplete   ResultAction = "complete"
	ResultActionRetry      ResultAction = "retry"
	ResultActionDeadLetter ResultAction = "dead_letter"
)

type ResultPublisher interface {
	PublishDeliveryResult(context.Context, DeliveryResultEvent) error
}

func NewDeliveryResultEvent(params CompleteDeliveryParams) DeliveryResultEvent {
	return DeliveryResultEvent{
		Action:            ResultActionComplete,
		JobID:             strings.TrimSpace(params.JobID),
		WorkerID:          strings.TrimSpace(params.WorkerID),
		AttemptID:         strings.TrimSpace(params.AttemptID),
		MessageID:         strings.TrimSpace(params.MessageID),
		SourceID:          strings.TrimSpace(params.SourceID),
		ChannelID:         strings.TrimSpace(params.ChannelID),
		TemplateVersionID: strings.TrimSpace(params.TemplateVersionID),
		RecipientSnapshot: cloneRawMessage(params.RecipientSnapshot),
		DeliveryCreatedAt: params.DeliveryCreatedAt,
		TraceID:           strings.TrimSpace(params.TraceID),
		AttemptNo:         params.AttemptNo,
		Status:            params.Status,
		RequestSnapshot:   cloneRawMessage(params.RequestSnapshot),
		ResponseSnapshot:  cloneRawMessage(params.ResponseSnapshot),
		DurationMS:        params.DurationMS,
		FinishedAt:        params.FinishedAt,
		InboundHeaders:    cloneRawMessage(params.InboundHeaders),
		InboundPayload:    cloneRawMessage(params.InboundPayload),
		InboundReceivedAt: params.InboundReceivedAt,
	}
}

func NewRetryDeliveryResultEvent(params RetryDeliveryParams) DeliveryResultEvent {
	retryAt := params.RetryAt
	return DeliveryResultEvent{
		Action:            ResultActionRetry,
		JobID:             strings.TrimSpace(params.JobID),
		WorkerID:          strings.TrimSpace(params.WorkerID),
		AttemptID:         strings.TrimSpace(params.AttemptID),
		MessageID:         strings.TrimSpace(params.MessageID),
		SourceID:          strings.TrimSpace(params.SourceID),
		ChannelID:         strings.TrimSpace(params.ChannelID),
		TemplateVersionID: strings.TrimSpace(params.TemplateVersionID),
		RecipientSnapshot: cloneRawMessage(params.RecipientSnapshot),
		DeliveryCreatedAt: params.DeliveryCreatedAt,
		TraceID:           strings.TrimSpace(params.TraceID),
		AttemptNo:         params.AttemptNo,
		Status:            StatusFailed,
		ErrorCode:         strings.TrimSpace(params.ErrorCode),
		ErrorMessage:      params.ErrorMessage,
		RequestSnapshot:   cloneRawMessage(params.RequestSnapshot),
		ResponseSnapshot:  cloneRawMessage(params.ResponseSnapshot),
		DurationMS:        params.DurationMS,
		RetryAt:           &retryAt,
		FinishedAt:        params.FinishedAt,
		InboundHeaders:    cloneRawMessage(params.InboundHeaders),
		InboundPayload:    cloneRawMessage(params.InboundPayload),
		InboundReceivedAt: params.InboundReceivedAt,
	}
}

func NewDeadLetterDeliveryResultEvent(params DeadLetterDeliveryParams) DeliveryResultEvent {
	return DeliveryResultEvent{
		Action:            ResultActionDeadLetter,
		JobID:             strings.TrimSpace(params.JobID),
		WorkerID:          strings.TrimSpace(params.WorkerID),
		AttemptID:         strings.TrimSpace(params.AttemptID),
		ChannelID:         strings.TrimSpace(params.ChannelID),
		MessageID:         strings.TrimSpace(params.MessageID),
		SourceID:          strings.TrimSpace(params.SourceID),
		TemplateVersionID: strings.TrimSpace(params.TemplateVersionID),
		RecipientSnapshot: cloneRawMessage(params.RecipientSnapshot),
		DeliveryCreatedAt: params.DeliveryCreatedAt,
		TraceID:           strings.TrimSpace(params.TraceID),
		AttemptNo:         params.AttemptNo,
		Status:            StatusFailed,
		ErrorCode:         strings.TrimSpace(params.ErrorCode),
		ErrorMessage:      params.ErrorMessage,
		RequestSnapshot:   cloneRawMessage(params.RequestSnapshot),
		ResponseSnapshot:  cloneRawMessage(params.ResponseSnapshot),
		DurationMS:        params.DurationMS,
		FinishedAt:        params.FinishedAt,
		InboundHeaders:    cloneRawMessage(params.InboundHeaders),
		InboundPayload:    cloneRawMessage(params.InboundPayload),
		InboundReceivedAt: params.InboundReceivedAt,
	}
}

func (e DeliveryResultEvent) Validate() error {
	if e.Action == "" {
		e.Action = ResultActionComplete
	}
	if strings.TrimSpace(e.AttemptID) == "" || e.AttemptNo <= 0 || strings.TrimSpace(string(e.Status)) == "" {
		return queue.ErrInvalidInput
	}
	switch e.Action {
	case ResultActionComplete, ResultActionRetry, ResultActionDeadLetter:
	default:
		return queue.ErrInvalidInput
	}
	if e.Action == ResultActionRetry && e.RetryAt == nil {
		return queue.ErrInvalidInput
	}
	return nil
}

func (e DeliveryResultEvent) CompleteDeliveryParams() CompleteDeliveryParams {
	return CompleteDeliveryParams{
		JobID:             strings.TrimSpace(e.JobID),
		WorkerID:          strings.TrimSpace(e.WorkerID),
		AttemptID:         strings.TrimSpace(e.AttemptID),
		MessageID:         strings.TrimSpace(e.MessageID),
		SourceID:          strings.TrimSpace(e.SourceID),
		ChannelID:         strings.TrimSpace(e.ChannelID),
		TemplateVersionID: strings.TrimSpace(e.TemplateVersionID),
		RecipientSnapshot: cloneRawMessage(e.RecipientSnapshot),
		DeliveryCreatedAt: e.DeliveryCreatedAt,
		TraceID:           strings.TrimSpace(e.TraceID),
		AttemptNo:         e.AttemptNo,
		Status:            e.Status,
		RequestSnapshot:   cloneRawMessage(e.RequestSnapshot),
		ResponseSnapshot:  cloneRawMessage(e.ResponseSnapshot),
		DurationMS:        e.DurationMS,
		FinishedAt:        e.FinishedAt,
		InboundHeaders:    cloneRawMessage(e.InboundHeaders),
		InboundPayload:    cloneRawMessage(e.InboundPayload),
		InboundReceivedAt: e.InboundReceivedAt,
	}
}

func (e DeliveryResultEvent) RetryDeliveryParams() RetryDeliveryParams {
	var retryAt time.Time
	if e.RetryAt != nil {
		retryAt = *e.RetryAt
	}
	return RetryDeliveryParams{
		JobID:             strings.TrimSpace(e.JobID),
		WorkerID:          strings.TrimSpace(e.WorkerID),
		AttemptID:         strings.TrimSpace(e.AttemptID),
		MessageID:         strings.TrimSpace(e.MessageID),
		SourceID:          strings.TrimSpace(e.SourceID),
		ChannelID:         strings.TrimSpace(e.ChannelID),
		TemplateVersionID: strings.TrimSpace(e.TemplateVersionID),
		RecipientSnapshot: cloneRawMessage(e.RecipientSnapshot),
		DeliveryCreatedAt: e.DeliveryCreatedAt,
		TraceID:           strings.TrimSpace(e.TraceID),
		AttemptNo:         e.AttemptNo,
		ErrorCode:         strings.TrimSpace(e.ErrorCode),
		ErrorMessage:      e.ErrorMessage,
		RequestSnapshot:   cloneRawMessage(e.RequestSnapshot),
		ResponseSnapshot:  cloneRawMessage(e.ResponseSnapshot),
		DurationMS:        e.DurationMS,
		RetryAt:           retryAt,
		FinishedAt:        e.FinishedAt,
		InboundHeaders:    cloneRawMessage(e.InboundHeaders),
		InboundPayload:    cloneRawMessage(e.InboundPayload),
		InboundReceivedAt: e.InboundReceivedAt,
	}
}

func (e DeliveryResultEvent) DeadLetterDeliveryParams() DeadLetterDeliveryParams {
	return DeadLetterDeliveryParams{
		JobID:             strings.TrimSpace(e.JobID),
		WorkerID:          strings.TrimSpace(e.WorkerID),
		AttemptID:         strings.TrimSpace(e.AttemptID),
		ChannelID:         strings.TrimSpace(e.ChannelID),
		MessageID:         strings.TrimSpace(e.MessageID),
		SourceID:          strings.TrimSpace(e.SourceID),
		TemplateVersionID: strings.TrimSpace(e.TemplateVersionID),
		RecipientSnapshot: cloneRawMessage(e.RecipientSnapshot),
		DeliveryCreatedAt: e.DeliveryCreatedAt,
		TraceID:           strings.TrimSpace(e.TraceID),
		AttemptNo:         e.AttemptNo,
		ErrorCode:         strings.TrimSpace(e.ErrorCode),
		ErrorMessage:      e.ErrorMessage,
		RequestSnapshot:   cloneRawMessage(e.RequestSnapshot),
		ResponseSnapshot:  cloneRawMessage(e.ResponseSnapshot),
		DurationMS:        e.DurationMS,
		FinishedAt:        e.FinishedAt,
		InboundHeaders:    cloneRawMessage(e.InboundHeaders),
		InboundPayload:    cloneRawMessage(e.InboundPayload),
		InboundReceivedAt: e.InboundReceivedAt,
	}
}

func (e DeliveryResultEvent) QueueEvent() (queue.DeliveryResultEvent, error) {
	if err := e.Validate(); err != nil {
		return queue.DeliveryResultEvent{}, err
	}
	payload, err := json.Marshal(e)
	if err != nil {
		return queue.DeliveryResultEvent{}, err
	}
	return queue.DeliveryResultEvent{
		DeliveryAttemptID: strings.TrimSpace(e.AttemptID),
		AttemptNo:         e.AttemptNo,
		Status:            string(e.Status),
		TraceID:           strings.TrimSpace(e.TraceID),
		Payload:           payload,
	}, nil
}

func DeliveryResultEventFromQueue(event queue.DeliveryResultEvent) (DeliveryResultEvent, error) {
	var result DeliveryResultEvent
	if len(event.Payload) > 0 {
		if err := json.Unmarshal(event.Payload, &result); err != nil {
			return DeliveryResultEvent{}, err
		}
		if err := result.Validate(); err != nil {
			return DeliveryResultEvent{}, err
		}
		return result, nil
	}
	result = DeliveryResultEvent{
		AttemptID: strings.TrimSpace(event.DeliveryAttemptID),
		TraceID:   strings.TrimSpace(event.TraceID),
		AttemptNo: event.AttemptNo,
		Status:    Status(strings.TrimSpace(event.Status)),
	}
	if err := result.Validate(); err != nil {
		return DeliveryResultEvent{}, err
	}
	return result, nil
}

type QueueResultPublisher struct {
	broker queue.Broker
}

func NewQueueResultPublisher(broker queue.Broker) *QueueResultPublisher {
	return &QueueResultPublisher{broker: broker}
}

func (p *QueueResultPublisher) PublishDeliveryResult(ctx context.Context, event DeliveryResultEvent) error {
	if p == nil || p.broker == nil {
		return queue.ErrInvalidInput
	}
	queueEvent, err := event.QueueEvent()
	if err != nil {
		return err
	}
	_, err = p.broker.PublishResult(ctx, queueEvent)
	return err
}

func cloneRawMessage(raw json.RawMessage) json.RawMessage {
	if raw == nil {
		return nil
	}
	return append(json.RawMessage(nil), raw...)
}
