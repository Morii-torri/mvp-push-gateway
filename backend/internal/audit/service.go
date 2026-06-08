package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"time"
)

var (
	ErrNotFound     = errors.New("audit log not found")
	ErrInvalidInput = errors.New("invalid audit input")
)

type ListFilter struct {
	Actor        string
	Action       string
	ResourceType string
	Limit        int
	Offset       int
}

type ListResult struct {
	Logs   []Log
	Total  int
	Limit  int
	Offset int
}

type Log struct {
	ID               string
	ActorAdminID     string
	ActorUsername    string
	Action           string
	ResourceType     string
	ResourceID       string
	RequestSnapshot  json.RawMessage
	ResponseSnapshot json.RawMessage
	IPAddress        string
	UserAgent        string
	CreatedAt        time.Time
}

type RecordInput struct {
	ActorAdminID     string
	ActorUsername    string
	Action           string
	ResourceType     string
	ResourceID       string
	RequestSnapshot  json.RawMessage
	ResponseSnapshot json.RawMessage
	IPAddress        string
	UserAgent        string
}

type Store interface {
	ListLogs(ctx context.Context, filter ListFilter) (ListResult, error)
	GetLog(ctx context.Context, id string) (Log, error)
	Record(ctx context.Context, input RecordInput) (Log, error)
}

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

func (s *Service) ListLogs(ctx context.Context, filter ListFilter) (ListResult, error) {
	filter.Actor = strings.TrimSpace(filter.Actor)
	filter.Action = strings.TrimSpace(filter.Action)
	filter.ResourceType = strings.TrimSpace(filter.ResourceType)
	if filter.Limit <= 0 {
		filter.Limit = 50
	}
	if filter.Limit > 200 {
		filter.Limit = 200
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	return s.store.ListLogs(ctx, filter)
}

func (s *Service) GetLog(ctx context.Context, id string) (Log, error) {
	if strings.TrimSpace(id) == "" {
		return Log{}, ErrInvalidInput
	}
	return s.store.GetLog(ctx, strings.TrimSpace(id))
}

func (s *Service) Record(ctx context.Context, input RecordInput) (Log, error) {
	input.ActorAdminID = strings.TrimSpace(input.ActorAdminID)
	input.ActorUsername = strings.TrimSpace(input.ActorUsername)
	input.Action = strings.TrimSpace(input.Action)
	input.ResourceType = strings.TrimSpace(input.ResourceType)
	input.ResourceID = strings.TrimSpace(input.ResourceID)
	input.IPAddress = strings.TrimSpace(input.IPAddress)
	input.UserAgent = strings.TrimSpace(input.UserAgent)
	if input.Action == "" || input.ResourceType == "" {
		return Log{}, ErrInvalidInput
	}
	input.RequestSnapshot = redactJSON(normalizeJSON(input.RequestSnapshot))
	input.ResponseSnapshot = redactJSON(normalizeJSON(input.ResponseSnapshot))
	return s.store.Record(ctx, input)
}

func normalizeJSON(raw json.RawMessage) json.RawMessage {
	if len(bytes.TrimSpace(raw)) == 0 || !json.Valid(raw) {
		return json.RawMessage(`{}`)
	}
	return append(json.RawMessage(nil), bytes.TrimSpace(raw)...)
}

func redactJSON(raw json.RawMessage) json.RawMessage {
	var value any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return raw
	}
	encoded, err := json.Marshal(redactValue(value, ""))
	if err != nil {
		return raw
	}
	return encoded
}

func redactValue(value any, key string) any {
	switch current := value.(type) {
	case map[string]any:
		redacted := make(map[string]any, len(current))
		for childKey, childValue := range current {
			if redactWholeField(childKey) || redactField(childKey, childValue) {
				redacted[childKey] = "[REDACTED]"
				continue
			}
			redacted[childKey] = redactValue(childValue, childKey)
		}
		return redacted
	case []any:
		for index, childValue := range current {
			current[index] = redactValue(childValue, key)
		}
		return current
	default:
		if redactField(key, current) {
			return "[REDACTED]"
		}
		return current
	}
}

func redactWholeField(key string) bool {
	switch normalizeAuditKey(key) {
	case "tokenconfig", "credentialconfig", "credentials":
		return true
	default:
		return false
	}
}

func redactField(key string, value any) bool {
	normalizedKey := normalizeAuditKey(key)
	switch {
	case normalizedKey == "":
		return false
	case strings.Contains(normalizedKey, "password"),
		strings.Contains(normalizedKey, "secret"),
		strings.Contains(normalizedKey, "token"),
		strings.Contains(normalizedKey, "credential"),
		strings.Contains(normalizedKey, "privatekey"),
		strings.Contains(normalizedKey, "accesskey"),
		strings.Contains(normalizedKey, "authorization"),
		strings.Contains(normalizedKey, "cookie"),
		strings.Contains(normalizedKey, "signature"):
		return true
	}
	text, ok := value.(string)
	return ok && strings.Contains(normalizedKey, "url") && urlContainsSensitiveQuery(text)
}

func normalizeAuditKey(key string) string {
	key = strings.ToLower(strings.TrimSpace(key))
	replacer := strings.NewReplacer("_", "", "-", "", ".", "", " ", "")
	return replacer.Replace(key)
}

func urlContainsSensitiveQuery(value string) bool {
	parsed, err := url.Parse(value)
	if err != nil || parsed.RawQuery == "" {
		return false
	}
	for key := range parsed.Query() {
		normalizedKey := normalizeAuditKey(key)
		if strings.Contains(normalizedKey, "token") ||
			strings.Contains(normalizedKey, "secret") ||
			strings.Contains(normalizedKey, "password") ||
			strings.Contains(normalizedKey, "key") ||
			strings.Contains(normalizedKey, "signature") {
			return true
		}
	}
	return false
}
