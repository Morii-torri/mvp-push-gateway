package provider

import (
	"net/url"
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
	mode := firstString(stringConfig(send, "mode"), "standard")
	endpoint := "https://wxpusher.zjiecode.com/api/send/message"
	body := map[string]any{
		"content":     appendURL(messageBody(content), stringConfig(content, "url")),
		"summary":     messageTitle(content),
		"contentType": wxPusherContentType(send, content),
	}
	recipients := recipientStrings(recipient)
	if mode == "simple" {
		endpoint = "https://wxpusher.zjiecode.com/api/send/message/simple-push"
		body["spt"] = firstString(stringConfig(auth, "spt"), stringConfig(send, "spt"))
		if sptList := listConfig(auth, "spt_list", "sptList"); len(sptList) > 0 {
			body["sptList"] = sptList
		}
	} else {
		body["appToken"] = firstString(stringConfig(auth, "app_token", "appToken"), stringConfig(send, "app_token", "appToken"))
		if len(recipients) > 0 {
			body["uids"] = recipients
		} else if uids := listConfig(send, "uids"); len(uids) > 0 {
			body["uids"] = uids
		}
		if topicIDs := rawListConfig(send, "topic_ids", "topicIds"); len(topicIDs) > 0 {
			body["topicIds"] = topicIDs
		}
	}
	return jsonRequest("POST", firstString(stringConfig(send, "url"), endpoint), body)
}

func serverChanRequestConfig(auth, send, content map[string]any) (requestConfig, error) {
	version := firstString(stringConfig(send, "version"), stringConfig(auth, "version"), "turbo")
	sendKey := firstString(stringConfig(auth, "send_key", "sendKey"), stringConfig(send, "send_key", "sendKey"))
	if strings.TrimSpace(sendKey) == "" {
		return requestConfig{}, ErrInvalidInput
	}
	requestURL := "https://sctapi.ftqq.com/" + url.PathEscape(sendKey) + ".send"
	if version == "v3" {
		uid := firstString(stringConfig(auth, "uid"), stringConfig(send, "uid"))
		if strings.TrimSpace(uid) == "" {
			return requestConfig{}, ErrInvalidInput
		}
		requestURL = "https://" + url.PathEscape(uid) + ".push.ft07.com/send/" + url.PathEscape(sendKey) + ".send"
	}
	body := map[string]any{
		"title": messageTitle(content),
		"desp":  messageBody(content),
	}
	for _, field := range []string{"channel", "openid", "tags", "short"} {
		copyStringField(body, field, send, field)
	}
	if value, ok := send["noip"]; ok {
		body["noip"] = value
	}
	return formLikeRequest("POST", firstString(stringConfig(send, "url"), requestURL), body)
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

func wxPusherContentType(send, content map[string]any) any {
	if value, ok := firstValueOK(send, "content_type", "contentType"); ok {
		return value
	}
	switch strings.ToLower(firstString(stringConfig(content, "format"), stringConfig(send, "format"))) {
	case "text", "txt":
		return 1
	case "html":
		return 2
	default:
		return 3
	}
}
