package db

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestMinimizeStoredLogJSONRedactsSensitiveValues(t *testing.T) {
	minimized, changed := minimizeStoredLogJSON(json.RawMessage(`{
		"title":"paid",
		"access_token":"token-1",
		"user":{"email":"person@example.com","name":"Alice"}
	}`), maxStoredMessagePayloadBytes)
	if !changed {
		t.Fatal("expected sensitive log json to be changed")
	}
	text := string(minimized)
	if strings.Contains(text, "token-1") || strings.Contains(text, "person@example.com") {
		t.Fatalf("minimized log json leaked sensitive values: %s", text)
	}
	if !strings.Contains(text, `"title":"paid"`) || !strings.Contains(text, `"name":"Alice"`) {
		t.Fatalf("minimized log json should preserve non-sensitive context, got %s", text)
	}
}

func TestMinimizeStoredLogJSONReportsUnchangedSafeJSON(t *testing.T) {
	minimized, changed := minimizeStoredLogJSON(json.RawMessage(`{"title":"paid","user":{"name":"Alice"}}`), maxStoredMessagePayloadBytes)
	if changed {
		t.Fatalf("expected already safe json to stay unchanged, got %s", minimized)
	}
}
