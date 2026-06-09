package safedata

import (
	"bytes"
	"encoding/json"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	RedactedValue         = "***"
	DefaultMaxStringRunes = 512
	DefaultMaxArrayItems  = 50
)

func MinimizeJSON(raw json.RawMessage, maxBytes int) json.RawMessage {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return json.RawMessage(`null`)
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return boundedPreview(raw, maxBytes)
	}
	minimized := minimizeValue(value, "")
	encoded, err := json.Marshal(minimized)
	if err != nil {
		return boundedPreview(raw, maxBytes)
	}
	if maxBytes > 0 && len(encoded) > maxBytes {
		return boundedPreview(encoded, maxBytes)
	}
	return encoded
}

func minimizeValue(value any, key string) any {
	if IsSensitiveKey(key) {
		return RedactedValue
	}
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for childKey, childValue := range typed {
			result[childKey] = minimizeValue(childValue, childKey)
		}
		return result
	case []any:
		limit := len(typed)
		truncated := false
		if limit > DefaultMaxArrayItems {
			limit = DefaultMaxArrayItems
			truncated = true
		}
		result := make([]any, 0, limit+1)
		for index := 0; index < limit; index++ {
			result = append(result, minimizeValue(typed[index], key))
		}
		if truncated {
			result = append(result, map[string]any{
				"_truncated":       true,
				"_remaining_items": len(typed) - limit,
			})
		}
		return result
	case string:
		return truncateString(typed, DefaultMaxStringRunes)
	default:
		return value
	}
}

func IsSensitiveKey(key string) bool {
	normalized := normalizeKey(key)
	if normalized == "" {
		return false
	}
	switch normalized {
	case "tokenbehavior", "tokenexchange", "tokenrefreshexchange", "tokenrefreshed", "tokenstrategy", "refreshtokencodes", "retryablejsoncodes":
		return false
	}
	if normalized == "authorization" || normalized == "cookie" || normalized == "signature" {
		return true
	}
	for _, marker := range []string{
		"token",
		"secret",
		"password",
		"credential",
		"privatekey",
		"accesskey",
		"apikey",
		"appkey",
		"hmac",
		"email",
		"mobile",
		"phone",
		"openid",
		"userid",
		"idcard",
		"identity",
	} {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	return false
}

func normalizeKey(key string) string {
	key = strings.ToLower(strings.TrimSpace(key))
	var builder strings.Builder
	for _, char := range key {
		if unicode.IsLetter(char) || unicode.IsDigit(char) {
			builder.WriteRune(char)
		}
	}
	return builder.String()
}

func truncateString(value string, maxRunes int) string {
	if maxRunes <= 0 || utf8.RuneCountInString(value) <= maxRunes {
		return value
	}
	runes := []rune(value)
	return string(runes[:maxRunes]) + "...[truncated]"
}

func boundedPreview(raw []byte, maxBytes int) json.RawMessage {
	if maxBytes <= 0 || len(raw) <= maxBytes {
		encoded, err := json.Marshal(string(raw))
		if err != nil {
			return json.RawMessage(`null`)
		}
		return encoded
	}
	previewBytes := maxBytes
	if previewBytes > 1024 {
		previewBytes = 1024
	}
	preview := string(raw[:previewBytes])
	encoded, err := json.Marshal(map[string]any{
		"_truncated":      true,
		"_original_bytes": len(raw),
		"preview":         preview,
	})
	if err != nil {
		return json.RawMessage(`{"_truncated":true}`)
	}
	return encoded
}
