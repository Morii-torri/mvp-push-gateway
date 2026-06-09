package db

import (
	"bytes"
	"encoding/json"

	"mvp-push-gateway/backend/internal/safedata"
)

const maxStoredMessagePayloadBytes = 16 * 1024

func storedMessagePayload(raw json.RawMessage) json.RawMessage {
	minimized, _ := minimizeStoredLogJSON(defaultJSON(raw), maxStoredMessagePayloadBytes)
	return minimized
}

func minimizeStoredLogJSON(raw json.RawMessage, maxBytes int) (json.RawMessage, bool) {
	normalized := defaultJSON(raw)
	minimized := safedata.MinimizeJSON(normalized, maxBytes)
	return minimized, !bytes.Equal(bytes.TrimSpace(normalized), bytes.TrimSpace(minimized))
}
