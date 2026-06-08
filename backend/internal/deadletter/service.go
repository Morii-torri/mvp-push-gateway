package deadletter

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var (
	ErrInvalidInput = errors.New("invalid dead-letter input")
	ErrNotFound     = errors.New("dead-letter job not found")
)

type Job struct {
	ID             string
	JobID          string
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
	IDs []string `json:"ids"`
	Now time.Time
}

type HandleInput struct {
	IDs    []string `json:"ids"`
	Reason string   `json:"reason"`
	Now    time.Time
}

type BatchResult struct {
	Processed int      `json:"processed"`
	IDs       []string `json:"ids"`
}

type Store interface {
	ListDeadLetters(context.Context, ListFilter) (ListResult, error)
	ReplayDeadLetters(context.Context, BatchInput) (BatchResult, error)
	MarkDeadLettersHandled(context.Context, HandleInput) (BatchResult, error)
	DeleteDeadLetters(context.Context, BatchInput) (BatchResult, error)
}

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

func (s *Service) ListDeadLetters(ctx context.Context, filter ListFilter) (ListResult, error) {
	filter.Status = normalizeStatus(filter.Status)
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
	return s.store.ListDeadLetters(ctx, filter)
}

func (s *Service) ReplayDeadLetters(ctx context.Context, input BatchInput) (BatchResult, error) {
	input.IDs = normalizeIDs(input.IDs)
	if len(input.IDs) == 0 {
		return BatchResult{}, ErrInvalidInput
	}
	if input.Now.IsZero() {
		input.Now = time.Now().UTC()
	}
	return s.store.ReplayDeadLetters(ctx, input)
}

func (s *Service) MarkDeadLettersHandled(ctx context.Context, input HandleInput) (BatchResult, error) {
	input.IDs = normalizeIDs(input.IDs)
	if len(input.IDs) == 0 {
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
	input.IDs = normalizeIDs(input.IDs)
	if len(input.IDs) == 0 {
		return BatchResult{}, ErrInvalidInput
	}
	if input.Now.IsZero() {
		input.Now = time.Now().UTC()
	}
	return s.store.DeleteDeadLetters(ctx, input)
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
