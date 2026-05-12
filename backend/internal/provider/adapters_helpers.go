package provider

import (
	"encoding/json"
	"fmt"
	"strings"
)

func decodeObjectConfig(raw json.RawMessage) (map[string]any, error) {
	if len(strings.TrimSpace(string(raw))) == 0 {
		return map[string]any{}, nil
	}
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, ErrInvalidInput
	}
	if value == nil {
		value = map[string]any{}
	}
	return value, nil
}

func jsonRequest(method, requestURL string, body map[string]any) (requestConfig, error) {
	return requestConfig{
		Method:            method,
		URL:               requestURL,
		Headers:           map[string]string{"Content-Type": "application/json"},
		Body:              mustJSON(body),
		SkipRenderedMerge: true,
	}, nil
}

func tokenQueryJSONRequest(method, requestURL, fieldName string, body map[string]any) (requestConfig, error) {
	config, err := jsonRequest(method, requestURL, body)
	if err != nil {
		return requestConfig{}, err
	}
	config.Token = placementConfig{Location: PlacementQuery, FieldName: fieldName}
	return config, nil
}

func formLikeRequest(method, requestURL string, body map[string]any) (requestConfig, error) {
	return requestConfig{
		Method:            method,
		URL:               requestURL,
		Headers:           map[string]string{"Content-Type": "application/x-www-form-urlencoded"},
		Body:              mustJSON(body),
		SkipRenderedMerge: true,
	}, nil
}

func mustJSON(value any) json.RawMessage {
	raw, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return raw
}

func messageTitle(content map[string]any) string {
	title := firstString(stringConfig(content, "title"), stringConfig(content, "subject"))
	if title != "" {
		return title
	}
	body := messageBody(content)
	if body == "" {
		return "Notification"
	}
	if len([]rune(body)) <= 40 {
		return body
	}
	return string([]rune(body)[:40])
}

func messageBody(content map[string]any) string {
	for _, key := range []string{"body", "content", "description", "markdown", "text", "html"} {
		if value := stringConfig(content, key); value != "" {
			return value
		}
	}
	if nested, ok := content["text"].(map[string]any); ok {
		if value := stringConfig(nested, "content"); value != "" {
			return value
		}
	}
	return ""
}

func appendURL(body, rawURL string) string {
	if strings.TrimSpace(rawURL) == "" {
		return body
	}
	if strings.TrimSpace(body) == "" {
		return rawURL
	}
	return body + "\n\n" + rawURL
}

func firstRecipientString(recipient any) string {
	recipients := recipientStrings(recipient)
	if len(recipients) == 0 {
		return ""
	}
	return recipients[0]
}

func stringConfig(config map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := config[key]; ok && value != nil {
			var text string
			switch typed := value.(type) {
			case string:
				text = typed
			case map[string]any, []any, []string:
				continue
			default:
				text = fmt.Sprint(typed)
			}
			text = strings.TrimSpace(text)
			if text != "" {
				return text
			}
		}
	}
	return ""
}

func boolConfig(config map[string]any, keys ...string) bool {
	for _, key := range keys {
		if value, ok := config[key]; ok {
			switch typed := value.(type) {
			case bool:
				return typed
			case string:
				return strings.EqualFold(strings.TrimSpace(typed), "true")
			}
		}
	}
	return false
}

func firstValue(configA, configB map[string]any, keys ...string) any {
	if value, ok := firstValueOK(configA, keys...); ok {
		return value
	}
	if value, ok := firstValueOK(configB, keys...); ok {
		return value
	}
	return nil
}

func firstValueOK(config map[string]any, keys ...string) (any, bool) {
	for _, key := range keys {
		if value, ok := config[key]; ok && !isEmptyValue(value) {
			return value, true
		}
	}
	return nil, false
}

func copyStringField(dest map[string]any, destField string, source map[string]any, sourceFields ...string) {
	if value := stringConfig(source, sourceFields...); value != "" {
		dest[destField] = value
	}
}

func listConfig(config map[string]any, keys ...string) []string {
	return stringListFromAny(rawConfig(config, keys...))
}

func rawListConfig(config map[string]any, keys ...string) []any {
	value := rawConfig(config, keys...)
	switch typed := value.(type) {
	case []any:
		return typed
	case []string:
		values := make([]any, 0, len(typed))
		for _, item := range typed {
			values = append(values, item)
		}
		return values
	default:
		return nil
	}
}

func rawConfig(config map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := config[key]; ok {
			return value
		}
	}
	return nil
}

func recipientStrings(recipient any) []string {
	return stringListFromAny(recipient)
}

func stringListFromAny(value any) []string {
	switch typed := value.(type) {
	case nil:
		return nil
	case string:
		if strings.TrimSpace(typed) == "" {
			return nil
		}
		return []string{strings.TrimSpace(typed)}
	case []string:
		values := make([]string, 0, len(typed))
		for _, item := range typed {
			if item = strings.TrimSpace(item); item != "" {
				values = append(values, item)
			}
		}
		return values
	case []any:
		values := make([]string, 0, len(typed))
		for _, item := range typed {
			if item == nil {
				continue
			}
			text := strings.TrimSpace(fmt.Sprint(item))
			if text != "" {
				values = append(values, text)
			}
		}
		return values
	default:
		text := strings.TrimSpace(fmt.Sprint(typed))
		if text == "" {
			return nil
		}
		return []string{text}
	}
}

func joinURL(base, path string) string {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	path = strings.TrimLeft(strings.TrimSpace(path), "/")
	if path == "" {
		return base
	}
	if base == "" {
		return "/" + path
	}
	return base + "/" + path
}
