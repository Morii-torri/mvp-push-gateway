package deadletter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"mvp-push-gateway/backend/internal/queue"
)

var (
	ErrInvalidInput = errors.New("invalid dead-letter input")
	ErrNotFound     = errors.New("dead-letter job not found")
)

type Job struct {
	ID             string
	JobID          string
	TraceID        string
	Type           string
	Payload        json.RawMessage
	ChannelID      string
	ChannelName    string
	ProviderType   string
	ErrorCode      string
	ErrorMessage   string
	Attempts       int
	DeadLetteredAt time.Time
	ReplayedAt     *time.Time
	HandledAt      *time.Time
	HandledReason  string
	CreatedAt      time.Time
}

type ListFilter struct {
	Status    string
	ChannelID string
	Keyword   string
	Limit     int
	Offset    int
}

type ListResult struct {
	Items  []Job
	Total  int
	Limit  int
	Offset int
}

type BatchInput struct {
	IDs       []string `json:"ids"`
	All       bool     `json:"all"`
	Status    string   `json:"status"`
	ChannelID string   `json:"channel_id"`
	Now       time.Time
}

type HandleInput struct {
	IDs       []string `json:"ids"`
	All       bool     `json:"all"`
	Status    string   `json:"status"`
	ChannelID string   `json:"channel_id"`
	Reason    string   `json:"reason"`
	Now       time.Time
}

type BatchResult struct {
	Processed int      `json:"processed"`
	IDs       []string `json:"ids"`
}

type ExternalReplayEvent struct {
	ID    string
	Event queue.SendMessageEvent
}

type Store interface {
	ListDeadLetters(context.Context, ListFilter) (ListResult, error)
	MarkDeadLettersHandled(context.Context, HandleInput) (BatchResult, error)
	DeleteDeadLetters(context.Context, BatchInput) (BatchResult, error)
}

type ExternalReplayStore interface {
	ListExternalReplayEvents(context.Context, BatchInput) ([]ExternalReplayEvent, error)
	MarkExternalDeadLettersReplayed(context.Context, []string, time.Time) (BatchResult, error)
}

type SendPublisher interface {
	PublishSend(context.Context, queue.SendMessageEvent) (queue.PublishResult, error)
}

type Option func(*Service)

type Service struct {
	store         Store
	sendPublisher SendPublisher
}

func WithSendPublisher(publisher SendPublisher) Option {
	return func(s *Service) {
		s.sendPublisher = publisher
	}
}

func NewService(store Store, options ...Option) *Service {
	service := &Service{store: store}
	for _, option := range options {
		option(service)
	}
	return service
}

func (s *Service) ListDeadLetters(ctx context.Context, filter ListFilter) (ListResult, error) {
	filter.Status = normalizeStatus(filter.Status)
	filter.ChannelID = strings.TrimSpace(filter.ChannelID)
	filter.Keyword = strings.TrimSpace(filter.Keyword)
	if filter.Limit <= 0 {
		filter.Limit = 50
	}
	if filter.Limit > 200 {
		filter.Limit = 200
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	return s.store.ListDeadLetters(ctx, filter)
}

func (s *Service) ReplayDeadLetters(ctx context.Context, input BatchInput) (BatchResult, error) {
	input = normalizeBatchInput(input)
	if len(input.IDs) == 0 && !input.All {
		return BatchResult{}, ErrInvalidInput
	}
	if input.Now.IsZero() {
		input.Now = time.Now().UTC()
	}
	return s.replayExternalDeadLetters(ctx, input)
}

func (s *Service) replayExternalDeadLetters(ctx context.Context, input BatchInput) (BatchResult, error) {
	if s.sendPublisher == nil || input.Status == "replayed" || input.Status == "handled" {
		return BatchResult{}, nil
	}
	store, ok := s.store.(ExternalReplayStore)
	if !ok {
		return BatchResult{}, nil
	}
	events, err := store.ListExternalReplayEvents(ctx, input)
	if err != nil {
		return BatchResult{}, err
	}
	publishedIDs := make([]string, 0, len(events))
	for _, item := range events {
		if strings.TrimSpace(item.ID) == "" {
			continue
		}
		if _, err := s.sendPublisher.PublishSend(ctx, item.Event); err != nil {
			if len(publishedIDs) > 0 {
				_, _ = store.MarkExternalDeadLettersReplayed(ctx, publishedIDs, input.Now)
			}
			return BatchResult{}, fmt.Errorf("publish external dead-letter replay: %w", err)
		}
		publishedIDs = append(publishedIDs, item.ID)
	}
	if len(publishedIDs) == 0 {
		return BatchResult{}, nil
	}
	return store.MarkExternalDeadLettersReplayed(ctx, publishedIDs, input.Now)
}

func (s *Service) MarkDeadLettersHandled(ctx context.Context, input HandleInput) (BatchResult, error) {
	input.IDs = normalizeIDs(input.IDs)
	input.Status = normalizeStatus(input.Status)
	input.ChannelID = strings.TrimSpace(input.ChannelID)
	if len(input.IDs) == 0 && !input.All {
		return BatchResult{}, ErrInvalidInput
	}
	input.Reason = strings.TrimSpace(input.Reason)
	if input.Reason == "" {
		input.Reason = "manual"
	}
	if input.Now.IsZero() {
		input.Now = time.Now().UTC()
	}
	return s.store.MarkDeadLettersHandled(ctx, input)
}

func (s *Service) DeleteDeadLetters(ctx context.Context, input BatchInput) (BatchResult, error) {
	input = normalizeBatchInput(input)
	if len(input.IDs) == 0 && !input.All {
		return BatchResult{}, ErrInvalidInput
	}
	if input.Now.IsZero() {
		input.Now = time.Now().UTC()
	}
	return s.store.DeleteDeadLetters(ctx, input)
}

func normalizeBatchInput(input BatchInput) BatchInput {
	input.IDs = normalizeIDs(input.IDs)
	input.Status = normalizeStatus(input.Status)
	input.ChannelID = strings.TrimSpace(input.ChannelID)
	return input
}

func normalizeStatus(value string) string {
	switch strings.TrimSpace(value) {
	case "all", "replayed", "handled":
		return strings.TrimSpace(value)
	default:
		return "pending"
	}
}

func normalizeIDs(values []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		id := strings.TrimSpace(value)
		if id == "" {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}
