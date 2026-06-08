package audit

import (
	"context"
	"encoding/json"
	"testing"
)

func TestRecordRedactsSensitiveSnapshotFields(t *testing.T) {
	store := &memoryStore{}
	service := NewService(store)

	_, err := service.Record(context.Background(), RecordInput{
		Action:       "update",
		ResourceType: "channel",
		ResourceID:   "channel-1",
		RequestSnapshot: json.RawMessage(`{
			"auth_token":"source-token",
			"hmac_secret":"source-secret",
			"password":"plain",
			"auth_config":{"webhook_url":"https://example.test/hook?token=abc","safe":"visible"},
			"nested":{"access_token":"abc","items":[{"client_secret":"def"}]}
		}`),
		ResponseSnapshot: json.RawMessage(`{"token_config":{"refresh_token":"secret"},"name":"channel"}`),
	})
	if err != nil {
		t.Fatalf("record audit log: %v", err)
	}

	var request map[string]any
	if err := json.Unmarshal(store.recordInput.RequestSnapshot, &request); err != nil {
		t.Fatalf("decode request snapshot: %v", err)
	}
	if request["auth_token"] != "[REDACTED]" || request["hmac_secret"] != "[REDACTED]" || request["password"] != "[REDACTED]" {
		t.Fatalf("expected top-level secrets redacted, got %s", store.recordInput.RequestSnapshot)
	}
	nested := request["nested"].(map[string]any)
	items := nested["items"].([]any)
	if nested["access_token"] != "[REDACTED]" || items[0].(map[string]any)["client_secret"] != "[REDACTED]" {
		t.Fatalf("expected nested secrets redacted, got %s", store.recordInput.RequestSnapshot)
	}
	authConfig := request["auth_config"].(map[string]any)
	if authConfig["safe"] != "visible" || authConfig["webhook_url"] != "[REDACTED]" {
		t.Fatalf("expected auth config secret URL redacted without dropping safe fields, got %s", store.recordInput.RequestSnapshot)
	}
	if string(store.recordInput.ResponseSnapshot) != `{"name":"channel","token_config":"[REDACTED]"}` {
		t.Fatalf("unexpected response snapshot: %s", store.recordInput.ResponseSnapshot)
	}
}

type memoryStore struct {
	recordInput RecordInput
}

func (m *memoryStore) ListLogs(context.Context, ListFilter) (ListResult, error) {
	return ListResult{}, nil
}

func (m *memoryStore) GetLog(context.Context, string) (Log, error) {
	return Log{}, nil
}

func (m *memoryStore) Record(_ context.Context, input RecordInput) (Log, error) {
	m.recordInput = input
	return Log{ID: "audit-1", Action: input.Action, ResourceType: input.ResourceType}, nil
}
