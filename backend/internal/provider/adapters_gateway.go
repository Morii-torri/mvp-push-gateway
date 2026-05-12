package provider

import (
	"net/url"
	"strings"
)

func selfRequestConfig(auth, send, content map[string]any, input BuildRequestInput) (requestConfig, error) {
	baseURL := firstString(stringConfig(send, "base_url"), stringConfig(auth, "base_url"))
	sourceCode := firstString(stringConfig(send, "source_code"), stringConfig(auth, "source_code"))
	if strings.TrimSpace(baseURL) == "" || strings.TrimSpace(sourceCode) == "" {
		return requestConfig{}, ErrInvalidInput
	}
	apiPrefix := firstString(stringConfig(send, "api_prefix"), stringConfig(auth, "api_prefix"), "/api/v1")
	requestURL := joinURL(joinURL(baseURL, apiPrefix), "ingest/"+url.PathEscape(sourceCode))
	body := content
	if firstString(stringConfig(send, "payload_mode"), "wrapped") != "raw" {
		body = map[string]any{
			"message":    content,
			"recipients": recipientStrings(input.Recipient),
		}
	}
	headers := map[string]string{}
	if token := firstString(input.Token, stringConfig(auth, "source_token"), stringConfig(auth, "token")); token != "" {
		headers["Authorization"] = "Bearer " + token
	}
	return requestConfig{
		Method:            "POST",
		URL:               requestURL,
		Headers:           headers,
		Body:              mustJSON(body),
		SkipRenderedMerge: true,
	}, nil
}
