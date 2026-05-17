package settings

import (
	"context"
	"encoding/json"
	"testing"
)

func TestDefaultSettingsExposePerformanceControls(t *testing.T) {
	defaults := DefaultSettings()

	if settingValue(defaults, "ingest.max_payload_bytes") != "5242880" {
		t.Fatalf("expected 5MiB ingest payload default, got %s", settingValue(defaults, "ingest.max_payload_bytes"))
	}
	concurrency := settingByKey(defaults, "runtime.delivery_global_concurrency")
	if string(concurrency.Value) != "10" {
		t.Fatalf("expected default global delivery concurrency 10, got %s", concurrency.Value)
	}
	if concurrency.Description != "当前系统实例并发上限" {
		t.Fatalf("expected system instance concurrency wording, got %q", concurrency.Description)
	}
}

func TestRunPerformanceTestUpdatesGlobalDeliveryConcurrency(t *testing.T) {
	store := newMemorySettingsStore()
	service := NewService(store)

	result, err := service.RunPerformanceTest(context.Background(), PerformanceTestInput{MessageCount: 80})
	if err != nil {
		t.Fatalf("run performance test: %v", err)
	}
	if result.RecommendedGlobalConcurrency < 1 {
		t.Fatalf("expected positive recommendation, got %+v", result)
	}
	if string(store.values["runtime.delivery_global_concurrency"]) != jsonNumber(result.RecommendedGlobalConcurrency) {
		t.Fatalf("expected runtime setting to be updated, got %s result=%+v", store.values["runtime.delivery_global_concurrency"], result)
	}
	if result.GeneratedSourceCode == "" || result.GeneratedRouteName == "" || result.GeneratedChannelName == "" {
		t.Fatalf("expected generated test resource names, got %+v", result)
	}
}

func settingValue(settings []Setting, key string) string {
	return string(settingByKey(settings, key).Value)
}

func settingByKey(settings []Setting, key string) Setting {
	for _, item := range settings {
		if item.Key == key {
			return item
		}
	}
	return Setting{}
}

func jsonNumber(value int) string {
	raw, _ := json.Marshal(value)
	return string(raw)
}

type memorySettingsStore struct {
	values map[string]json.RawMessage
}

func newMemorySettingsStore() *memorySettingsStore {
	values := map[string]json.RawMessage{}
	for _, item := range DefaultSettings() {
		values[item.Key] = item.Value
	}
	return &memorySettingsStore{values: values}
}

func (m *memorySettingsStore) ListSettings(context.Context) ([]Setting, error) {
	items := make([]Setting, 0, len(m.values))
	for key, value := range m.values {
		items = append(items, Setting{Key: key, Value: value})
	}
	return items, nil
}

func (m *memorySettingsStore) GetSetting(_ context.Context, key string) (Setting, error) {
	value, ok := m.values[key]
	if !ok {
		return Setting{}, ErrNotFound
	}
	return Setting{Key: key, Value: value}, nil
}

func (m *memorySettingsStore) UpdateSetting(_ context.Context, key string, input UpdateInput) (Setting, error) {
	m.values[key] = input.Value
	return Setting{Key: key, Value: input.Value}, nil
}

func (m *memorySettingsStore) EnsureDefaultSettings(_ context.Context, defaults []Setting) error {
	for _, item := range defaults {
		m.values[item.Key] = item.Value
	}
	return nil
}
