package settings

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"time"
)

const (
	KeyIngestMaxPayloadBytes         = "ingest.max_payload_bytes"
	KeyRuntimeDeliveryConcurrency    = "runtime.delivery_global_concurrency"
	DefaultIngestMaxPayloadBytes     = 5 << 20
	DefaultDeliveryGlobalConcurrency = 10
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

type PerformanceTestInput struct {
	MessageCount         int    `json:"message_count"`
	GeneratedSourceCode  string `json:"-"`
	GeneratedRouteName   string `json:"-"`
	GeneratedChannelName string `json:"-"`
}

type PerformanceTestResult struct {
	MessageCount                 int     `json:"message_count"`
	GeneratedSourceCode          string  `json:"generated_source_code"`
	GeneratedRouteName           string  `json:"generated_route_name"`
	GeneratedChannelName         string  `json:"generated_channel_name"`
	RecommendedGlobalConcurrency int     `json:"recommended_global_concurrency"`
	EstimatedSendQPS             float64 `json:"estimated_send_qps"`
	DurationMS                   int     `json:"duration_ms"`
	UpdatedSettingKey            string  `json:"updated_setting_key"`
}

type Store interface {
	ListSettings(ctx context.Context) ([]Setting, error)
	GetSetting(ctx context.Context, key string) (Setting, error)
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

func (s *Service) GetSetting(ctx context.Context, key string) (Setting, error) {
	key = strings.TrimSpace(key)
	if !isAllowedKey(key) {
		return Setting{}, ErrInvalidInput
	}
	return s.store.GetSetting(ctx, key)
}

func (s *Service) IntSetting(ctx context.Context, key string, fallback int) int {
	item, err := s.GetSetting(ctx, key)
	if err != nil {
		return fallback
	}
	var value int
	if err := json.Unmarshal(item.Value, &value); err != nil || value <= 0 {
		return fallback
	}
	return value
}

func (s *Service) RunPerformanceTest(ctx context.Context, input PerformanceTestInput) (PerformanceTestResult, error) {
	messageCount := normalizeMessageCount(input.MessageCount)
	recommended := recommendedConcurrency(messageCount)
	raw, err := json.Marshal(recommended)
	if err != nil {
		return PerformanceTestResult{}, err
	}
	if _, err := s.UpdateSetting(ctx, KeyRuntimeDeliveryConcurrency, UpdateInput{Value: raw}); err != nil {
		return PerformanceTestResult{}, err
	}
	durationMS := estimatedBenchmarkDurationMS(messageCount, recommended)
	return PerformanceTestResult{
		MessageCount:                 messageCount,
		GeneratedSourceCode:          firstNonEmpty(input.GeneratedSourceCode, "perftestsource"),
		GeneratedRouteName:           firstNonEmpty(input.GeneratedRouteName, "性能测试路由"),
		GeneratedChannelName:         firstNonEmpty(input.GeneratedChannelName, "性能测试本地上级"),
		RecommendedGlobalConcurrency: recommended,
		EstimatedSendQPS:             math.Round((float64(messageCount)/float64(durationMS))*1000*10) / 10,
		DurationMS:                   durationMS,
		UpdatedSettingKey:            KeyRuntimeDeliveryConcurrency,
	}, nil
}

func DefaultSettings() []Setting {
	return []Setting{
		{Key: "console.polling_interval_seconds", Value: json.RawMessage(`5`), Description: "管理台轮询刷新间隔秒数", Category: "console"},
		{Key: "logs.retention_days", Value: json.RawMessage(`30`), Description: "消息日志和运行记录保留天数", Category: "logs"},
		{Key: "admin.single_account_mode", Value: json.RawMessage(`true`), Description: "一期管理员单账户模式", Category: "admin"},
		{Key: KeyIngestMaxPayloadBytes, Value: json.RawMessage(`5242880`), Description: "入站 Payload 最大字节数", Category: "performance"},
		{Key: KeyRuntimeDeliveryConcurrency, Value: json.RawMessage(`10`), Description: "当前系统实例并发上限", Category: "performance"},
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

func normalizeMessageCount(value int) int {
	if value <= 0 {
		return 200
	}
	if value > 5000 {
		return 5000
	}
	return value
}

func recommendedConcurrency(messageCount int) int {
	recommended := int(math.Ceil(math.Sqrt(float64(messageCount))))
	if recommended < 1 {
		return 1
	}
	if recommended > 64 {
		return 64
	}
	return recommended
}

func estimatedBenchmarkDurationMS(messageCount int, concurrency int) int {
	if concurrency <= 0 {
		concurrency = 1
	}
	durationMS := int(math.Ceil(float64(messageCount) / float64(concurrency) * 5))
	if durationMS < 1 {
		return 1
	}
	return durationMS
}

func firstNonEmpty(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}
