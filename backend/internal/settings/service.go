package settings

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var (
	ErrNotFound     = errors.New("setting not found")
	ErrInvalidInput = errors.New("invalid setting input")
)

type Setting struct {
	Key         string
	Value       json.RawMessage
	Description string
	Category    string
	UpdatedAt   time.Time
	CreatedAt   time.Time
}

type UpdateInput struct {
	Value json.RawMessage `json:"value"`
}

type Store interface {
	ListSettings(ctx context.Context) ([]Setting, error)
	UpdateSetting(ctx context.Context, key string, input UpdateInput) (Setting, error)
	EnsureDefaultSettings(ctx context.Context, defaults []Setting) error
}

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

func (s *Service) EnsureDefaults(ctx context.Context) error {
	return s.store.EnsureDefaultSettings(ctx, DefaultSettings())
}

func (s *Service) ListSettings(ctx context.Context) ([]Setting, error) {
	return s.store.ListSettings(ctx)
}

func (s *Service) UpdateSetting(ctx context.Context, key string, input UpdateInput) (Setting, error) {
	key = strings.TrimSpace(key)
	if !isAllowedKey(key) {
		return Setting{}, ErrInvalidInput
	}
	value, err := normalizeJSON(input.Value)
	if err != nil {
		return Setting{}, err
	}
	input.Value = value
	return s.store.UpdateSetting(ctx, key, input)
}

func DefaultSettings() []Setting {
	return []Setting{
		{Key: "console.polling_interval_seconds", Value: json.RawMessage(`5`), Description: "管理台轮询刷新间隔秒数", Category: "console"},
		{Key: "logs.retention_days", Value: json.RawMessage(`30`), Description: "消息日志和运行记录保留天数", Category: "logs"},
		{Key: "admin.single_account_mode", Value: json.RawMessage(`true`), Description: "一期管理员单账户模式", Category: "admin"},
	}
}

func isAllowedKey(key string) bool {
	for _, item := range DefaultSettings() {
		if item.Key == key {
			return true
		}
	}
	return false
}

func normalizeJSON(raw json.RawMessage) (json.RawMessage, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, ErrInvalidInput
	}
	if !json.Valid(raw) {
		return nil, ErrInvalidInput
	}
	return append(json.RawMessage(nil), bytes.TrimSpace(raw)...), nil
}
