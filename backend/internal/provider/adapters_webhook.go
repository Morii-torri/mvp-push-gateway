package provider

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
)

var (
	webhookRawIdentityPlaceholder     = regexp.MustCompile(`\{\{\s*identity\s*\}\}`)
	webhookEncodedIdentityPlaceholder = regexp.MustCompile(`(?i)%7b%7b(?:%20|\+)*identity(?:%20|\+)*%7d%7d`)
)

func webhookRequestConfig(send, content map[string]any, recipient any) (requestConfig, error) {
	requestURL := strings.TrimSpace(stringConfig(send, "url"))
	if requestURL == "" {
		return requestConfig{}, ErrInvalidInput
	}
	identity := firstRecipientString(recipient)
	if webhookRawIdentityPlaceholder.MatchString(requestURL) || webhookEncodedIdentityPlaceholder.MatchString(requestURL) {
		if identity == "" {
			return requestConfig{}, ErrInvalidInput
		}
		escapedIdentity := url.PathEscape(identity)
		requestURL = webhookRawIdentityPlaceholder.ReplaceAllString(requestURL, escapedIdentity)
		requestURL = webhookEncodedIdentityPlaceholder.ReplaceAllString(requestURL, escapedIdentity)
	}
	if hasWebhookTemplatePlaceholder(requestURL) {
		return requestConfig{}, ErrInvalidInput
	}

	method := strings.ToUpper(firstString(stringConfig(send, "method"), "POST"))
	if method != "POST" && method != "GET" {
		return requestConfig{}, ErrInvalidInput
	}

	bodyValue, ok := content["body"]
	if !ok || bodyValue == nil {
		return requestConfig{}, ErrInvalidInput
	}
	body, err := webhookBodyJSON(bodyValue)
	if err != nil {
		return requestConfig{}, err
	}

	headers := webhookHeaders(send["headers"])
	if method == "GET" {
		requestURL, err = appendWebhookQuery(requestURL, body)
		if err != nil {
			return requestConfig{}, err
		}
		return requestConfig{
			Method:            method,
			URL:               requestURL,
			Headers:           headers,
			SkipRenderedMerge: true,
			OmitBody:          true,
		}, nil
	}
	if headers == nil {
		headers = map[string]string{}
	}
	if headerValue(headers, "Content-Type") == "" {
		headers["Content-Type"] = "application/json"
	}
	rawBody, err := json.Marshal(body)
	if err != nil {
		return requestConfig{}, ErrInvalidInput
	}
	return requestConfig{
		Method:            method,
		URL:               requestURL,
		Headers:           headers,
		Body:              rawBody,
		SkipRenderedMerge: true,
	}, nil
}

func hasWebhookTemplatePlaceholder(value string) bool {
	if strings.Contains(value, "{{") || strings.Contains(value, "}}") || strings.Contains(value, "{recipient}") {
		return true
	}
	if decoded, err := url.PathUnescape(value); err == nil {
		if strings.Contains(decoded, "{{") || strings.Contains(decoded, "}}") || strings.Contains(decoded, "{recipient}") {
			return true
		}
	}
	if decoded, err := url.QueryUnescape(value); err == nil {
		if strings.Contains(decoded, "{{") || strings.Contains(decoded, "}}") || strings.Contains(decoded, "{recipient}") {
			return true
		}
	}
	return false
}

func webhookBodyJSON(value any) (any, error) {
	if text, ok := value.(string); ok {
		trimmed := strings.TrimSpace(text)
		if trimmed == "" {
			return map[string]any{}, nil
		}
		var parsed any
		if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
			return nil, ErrInvalidInput
		}
		return parsed, nil
	}
	return value, nil
}

func webhookHeaders(value any) map[string]string {
	object, ok := value.(map[string]any)
	if !ok {
		if typed, ok := value.(map[string]string); ok {
			headers := map[string]string{}
			for key, item := range typed {
				key = strings.TrimSpace(key)
				item = strings.TrimSpace(item)
				if key != "" && item != "" {
					headers[key] = item
				}
			}
			return headers
		}
		return map[string]string{}
	}
	headers := map[string]string{}
	for key, item := range object {
		key = strings.TrimSpace(key)
		value := strings.TrimSpace(fmt.Sprint(item))
		if key != "" && value != "" {
			headers[key] = value
		}
	}
	return headers
}

func appendWebhookQuery(requestURL string, body any) (string, error) {
	parsed, err := url.Parse(requestURL)
	if err != nil {
		return "", ErrInvalidInput
	}
	values := parsed.Query()
	if object, ok := body.(map[string]any); ok {
		keys := webhookSortedKeys(object)
		for _, key := range keys {
			values.Set(key, webhookQueryValue(object[key]))
		}
	} else {
		values.Set("body", webhookQueryValue(body))
	}
	parsed.RawQuery = values.Encode()
	return parsed.String(), nil
}

func webhookSortedKeys(object map[string]any) []string {
	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func webhookQueryValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case float64, float32, int, int64, int32, bool:
		return fmt.Sprint(typed)
	default:
		raw, err := json.Marshal(typed)
		if err != nil {
			return fmt.Sprint(typed)
		}
		return string(raw)
	}
}

func headerValue(headers map[string]string, name string) string {
	for key, value := range headers {
		if strings.EqualFold(key, name) {
			return value
		}
	}
	return ""
}
