package provider

import (
	"regexp"
	"strings"
)

var (
	serverChanSendKeyPattern = regexp.MustCompile(`^sctp(\d+)t`)
	serverChanURLPattern     = regexp.MustCompile(`^https://([^.]+)\.push\.ft07\.com/send/([^.]+)\.send$`)
)

func pushPlusRequestConfig(_ map[string]any, send, content map[string]any, recipient any) (requestConfig, error) {
	token := firstRecipientString(recipient)
	if strings.TrimSpace(token) == "" {
		return requestConfig{}, ErrInvalidInput
	}
	body := map[string]any{
		"token":   token,
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

func serverChanRequestConfig(auth, send, content map[string]any, recipient any) (requestConfig, error) {
	sendKey := firstRecipientString(recipient)
	if strings.TrimSpace(sendKey) == "" {
		return requestConfig{}, ErrInvalidInput
	}
	matches := serverChanSendKeyPattern.FindStringSubmatch(sendKey)
	if len(matches) != 2 || strings.TrimSpace(matches[1]) == "" {
		return requestConfig{}, ErrInvalidInput
	}
	requestURL := firstString(
		stringConfig(send, "url", "send_url", "api_url"),
		stringConfig(auth, "url", "send_url", "api_url"),
		"https://<uid>.push.ft07.com/send/<sendkey>.send",
	)

	if strings.Contains(requestURL, "<uid>") || strings.Contains(requestURL, "{uid}") || strings.Contains(requestURL, "<sendkey>") || strings.Contains(requestURL, "{sendkey}") || strings.Contains(requestURL, "%3Cuid%3E") || strings.Contains(requestURL, "%3Csendkey%3E") {
		requestURL = strings.NewReplacer(
			"<uid>", matches[1],
			"{uid}", matches[1],
			"%3Cuid%3E", matches[1],
			"<sendkey>", sendKey,
			"{sendkey}", sendKey,
			"%3Csendkey%3E", sendKey,
		).Replace(requestURL)
	} else if serverChanURLPattern.MatchString(requestURL) {
		requestURL = serverChanURLPattern.ReplaceAllString(requestURL, "https://"+matches[1]+".push.ft07.com/send/"+sendKey+".send")
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
	deviceKeys := recipientStrings(recipient)
	if strings.TrimSpace(serverURL) == "" || len(deviceKeys) == 0 {
		return requestConfig{}, ErrInvalidInput
	}
	body := map[string]any{}
	if len(deviceKeys) > 1 {
		body["device_keys"] = deviceKeys
	} else {
		body["device_key"] = deviceKeys[0]
	}

	for _, field := range []string{"title", "subtitle", "url", "group", "sound", "level", "icon", "image"} {
		copyStringField(body, field, content, field)
	}
	if markdown := stringConfig(content, "markdown"); markdown != "" {
		body["markdown"] = markdown
	} else if message := messageBody(content); message != "" {
		body["body"] = message
	}
	return jsonRequest("POST", joinURL(serverURL, "/push"), body)
}

func pushMeRequestConfig(auth, send, content map[string]any, recipient any) (requestConfig, error) {
	serverURL := firstString(stringConfig(send, "server_url"), stringConfig(auth, "server_url"), "https://push.i-i.me")
	pushKey := firstRecipientString(recipient)
	if strings.TrimSpace(serverURL) == "" || strings.TrimSpace(pushKey) == "" {
		return requestConfig{}, ErrInvalidInput
	}
	body := map[string]any{
		"push_key": pushKey,
		"title":    messageTitle(content),
		"content":  messageBody(content),
		"type":     pushMeMessageType(stringConfig(content, "type")),
	}
	return jsonRequest("POST", serverURL, body)
}

func pushMeMessageType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "text", "markdown", "html":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "markdown"
	}
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
