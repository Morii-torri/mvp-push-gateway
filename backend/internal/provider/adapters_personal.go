package provider

import (
	"strings"
)

func pushPlusRequestConfig(auth, send, content map[string]any) (requestConfig, error) {
	body := map[string]any{
		"token":   firstString(stringConfig(auth, "token"), stringConfig(send, "token")),
		"content": messageBody(content),
	}
	copyStringField(body, "title", content, "title", "subject")
	if topic := firstString(stringConfig(content, "topic"), stringConfig(send, "topic")); topic != "" {
		body["topic"] = topic
	}
	return jsonRequest("POST", "https://www.pushplus.plus/send", body)
}

func wxPusherRequestConfig(auth, send, content map[string]any, recipient any) (requestConfig, error) {
	body := map[string]any{
		"appToken":      firstString(stringConfig(auth, "app_token", "appToken"), stringConfig(send, "app_token", "appToken")),
		"content":       messageBody(content),
		"contentType":   2,
		"verifyPayType": 0,
	}
	copyStringField(body, "summary", content, "summary")
	copyStringField(body, "url", content, "url")
	if value, ok := firstValueOK(content, "verifyPayType", "verify_pay_type"); ok {
		body["verifyPayType"] = value
	} else if value, ok := firstValueOK(send, "verifyPayType", "verify_pay_type"); ok {
		body["verifyPayType"] = value
	}
	if uids := firstNonEmptyStringList(recipientStrings(recipient), listConfig(content, "uids"), listConfig(send, "uids")); len(uids) > 0 {
		body["uids"] = uids
	}
	if topicIDs := firstNonEmptyRawList(rawListConfig(content, "topicIds", "topic_ids"), rawListConfig(send, "topic_ids", "topicIds")); len(topicIDs) > 0 {
		body["topicIds"] = topicIDs
	}
	return jsonRequest("POST", "https://wxpusher.zjiecode.com/api/send/message", body)
}

func serverChanRequestConfig(auth, send, content map[string]any) (requestConfig, error) {
	requestURL := firstString(stringConfig(send, "url", "send_url", "api_url"), stringConfig(auth, "url", "send_url", "api_url"))
	if strings.TrimSpace(requestURL) == "" {
		return requestConfig{}, ErrInvalidInput
	}
	body := map[string]any{
		"title": messageTitle(content),
	}
	if desp := firstString(stringConfig(content, "desp"), messageBody(content)); desp != "" {
		body["desp"] = desp
	}
	copyStringField(body, "short", content, "short")
	return jsonRequest("POST", requestURL, body)
}

func barkRequestConfig(auth, send, content map[string]any, recipient any) (requestConfig, error) {
	serverURL := firstString(stringConfig(send, "server_url"), stringConfig(auth, "server_url"), "https://api.day.app")
	deviceKey := firstString(firstRecipientString(recipient), stringConfig(send, "device_key"), stringConfig(auth, "device_key"))
	if deviceKey == "" {
		if deviceKeys := listConfig(auth, "device_keys"); len(deviceKeys) > 0 {
			deviceKey = deviceKeys[0]
		}
	}
	if strings.TrimSpace(serverURL) == "" || strings.TrimSpace(deviceKey) == "" {
		return requestConfig{}, ErrInvalidInput
	}
	body := map[string]any{
		"device_key": deviceKey,
		"title":      messageTitle(content),
		"body":       messageBody(content),
	}
	for _, field := range []string{"subtitle", "url", "level"} {
		copyStringField(body, field, content, field)
	}
	for _, field := range []string{"group", "sound", "level", "icon", "url"} {
		copyStringField(body, field, send, field)
	}
	return jsonRequest("POST", joinURL(serverURL, "/push"), body)
}

func pushMeRequestConfig(auth, send, content map[string]any) (requestConfig, error) {
	serverURL := firstString(stringConfig(send, "server_url"), stringConfig(auth, "server_url"), "https://push.i-i.me")
	pushKey := firstString(stringConfig(auth, "push_key"), stringConfig(send, "push_key"), stringConfig(auth, "temp_key"), stringConfig(send, "temp_key"))
	if strings.TrimSpace(serverURL) == "" || strings.TrimSpace(pushKey) == "" {
		return requestConfig{}, ErrInvalidInput
	}
	body := map[string]any{
		"push_key":         pushKey,
		"title":            messageTitle(content),
		"content":          messageBody(content),
		"type":             firstString(stringConfig(send, "type"), stringConfig(content, "format"), "markdown"),
		"live_test_status": "implemented_but_not_live_tested",
	}
	if _, ok := firstValueOK(auth, "temp_key"); ok && stringConfig(auth, "push_key") == "" {
		body["temp_key"] = pushKey
		delete(body, "push_key")
	}
	return jsonRequest(firstString(stringConfig(send, "method"), "POST"), serverURL, body)
}

func pushPlusTemplate(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "text", "txt":
		return "txt"
	case "html":
		return "html"
	case "json":
		return "json"
	default:
		return "markdown"
	}
}

func firstNonEmptyStringList(values ...[]string) []string {
	for _, value := range values {
		if len(value) > 0 {
			return value
		}
	}
	return nil
}

func firstNonEmptyRawList(values ...[]any) []any {
	for _, value := range values {
		if len(value) > 0 {
			return value
		}
	}
	return nil
}
