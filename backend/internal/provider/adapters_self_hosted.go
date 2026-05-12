package provider

import (
	"encoding/base64"
	"net/url"
	"strings"
)

func ntfyRequestConfig(auth, send, content map[string]any) (requestConfig, error) {
	serverURL := firstString(stringConfig(send, "server_url"), stringConfig(auth, "server_url"), "https://ntfy.sh")
	topic := firstString(stringConfig(send, "topic"), stringConfig(auth, "topic"))
	if strings.TrimSpace(serverURL) == "" || strings.TrimSpace(topic) == "" {
		return requestConfig{}, ErrInvalidInput
	}
	headers := map[string]string{
		"Content-Type": "text/plain; charset=utf-8",
		"Title":        messageTitle(content),
	}
	if priority := firstString(stringConfig(content, "priority"), stringConfig(send, "priority")); priority != "" {
		headers["Priority"] = priority
	}
	if tags := listConfig(send, "tags"); len(tags) > 0 {
		headers["Tags"] = strings.Join(tags, ",")
	} else if tags := listConfig(content, "tags"); len(tags) > 0 {
		headers["Tags"] = strings.Join(tags, ",")
	}
	if boolConfig(send, "markdown") || strings.EqualFold(stringConfig(content, "format"), "markdown") {
		headers["Markdown"] = "yes"
	}
	if clickURL := firstString(stringConfig(content, "url"), stringConfig(send, "click")); clickURL != "" {
		headers["Click"] = clickURL
	}
	if actions := stringConfig(send, "actions"); actions != "" {
		headers["Actions"] = actions
	}
	if authHeader := ntfyAuthorizationHeader(auth); authHeader != "" {
		headers["Authorization"] = authHeader
	}
	body := map[string]any{
		"message":       appendURL(messageBody(content), stringConfig(content, "url")),
		"mock_protocol": "ntfy_text_body",
	}
	return requestConfig{
		Method:            "POST",
		URL:               joinURL(serverURL, url.PathEscape(topic)),
		Headers:           headers,
		Body:              mustJSON(body),
		SkipRenderedMerge: true,
	}, nil
}

func gotifyRequestConfig(auth, send, content map[string]any) (requestConfig, error) {
	serverURL := firstString(stringConfig(send, "server_url"), stringConfig(auth, "server_url"))
	appToken := firstString(stringConfig(auth, "app_token", "token"), stringConfig(send, "app_token", "token"))
	if strings.TrimSpace(serverURL) == "" || strings.TrimSpace(appToken) == "" {
		return requestConfig{}, ErrInvalidInput
	}
	contentType := firstString(stringConfig(send, "content_type"), gotifyContentType(content), "text/plain")
	body := map[string]any{
		"title":    messageTitle(content),
		"message":  appendURL(messageBody(content), stringConfig(content, "url")),
		"priority": gotifyPriority(send, content),
		"extras": map[string]any{
			"client::display": map[string]any{"contentType": contentType},
		},
	}
	requestURL := joinURL(serverURL, "/message") + "?token=" + url.QueryEscape(appToken)
	return jsonRequest("POST", requestURL, body)
}

func ntfyAuthorizationHeader(auth map[string]any) string {
	if token := stringConfig(auth, "bearer_token", "token"); token != "" {
		return "Bearer " + token
	}
	authType := strings.ToLower(firstString(stringConfig(auth, "auth_type"), stringConfig(auth, "authType")))
	if authType == "basic" {
		username := stringConfig(auth, "username")
		password := stringConfig(auth, "password")
		if username != "" || password != "" {
			return "Basic " + base64.StdEncoding.EncodeToString([]byte(username+":"+password))
		}
	}
	return ""
}

func gotifyContentType(content map[string]any) string {
	switch strings.ToLower(stringConfig(content, "format")) {
	case "markdown":
		return "text/markdown"
	default:
		return ""
	}
}

func gotifyPriority(send, content map[string]any) any {
	if value, ok := firstValueOK(send, "priority"); ok {
		return value
	}
	if value, ok := firstValueOK(content, "priority"); ok {
		return value
	}
	return 5
}
