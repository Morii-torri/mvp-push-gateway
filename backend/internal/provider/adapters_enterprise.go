package provider

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"
)

func weComRobotRequestConfig(auth, send, content map[string]any, recipient any) (requestConfig, error) {
	requestURL := firstString(stringConfig(auth, "webhook_url", "webhookUrl"), stringConfig(send, "webhook_url", "webhookUrl"))
	if requestURL == "" {
		requestURL = "https://qyapi.weixin.qq.com/cgi-bin/webhook/send"
	}
	if firstRecipientString(recipient) == "" {
		if key := firstString(stringConfig(auth, "key"), stringConfig(send, "key")); key != "" {
			parsed, err := url.Parse(requestURL)
			if err != nil {
				return requestConfig{}, ErrInvalidInput
			}
			values := parsed.Query()
			values.Set("key", key)
			parsed.RawQuery = values.Encode()
			requestURL = parsed.String()
		}
	}
	msgType := firstString(stringConfig(content, "msgtype", "msg_type"), "text")
	body := map[string]any{"msgtype": msgType}
	switch msgType {
	case "markdown":
		body["markdown"] = map[string]any{"content": firstString(stringConfig(content, "content"), stringConfig(content, "markdown"), messageBody(content))}
	default:
		body["msgtype"] = "text"
		body["text"] = map[string]any{"content": firstString(stringConfig(content, "content"), messageBody(content))}
	}
	config, err := jsonRequest("POST", requestURL, body)
	if err != nil {
		return requestConfig{}, err
	}
	config.Recipient = placementConfig{Location: PlacementQuery, FieldName: "key"}
	return config, nil
}

func weComAppRequestConfig(auth, send, content map[string]any, recipient any) (requestConfig, error) {
	baseURL := firstString(stringConfig(send, "base_url"), stringConfig(auth, "base_url"), "https://qyapi.weixin.qq.com")
	msgType := firstString(stringConfig(content, "msgtype", "msg_type"), stringConfig(send, "msgtype", "msg_type"), "text")
	body := map[string]any{
		"touser":  strings.Join(recipientStrings(recipient), "|"),
		"msgtype": msgType,
		"agentid": firstValue(send, auth, "agentid", "agent_id"),
	}
	switch msgType {
	case "markdown":
		body["markdown"] = map[string]any{"content": firstString(stringConfig(content, "markdown"), messageBody(content))}
	case "textcard":
		textcard := map[string]any{
			"title":       firstString(stringConfig(content, "title"), messageTitle(content)),
			"description": firstString(stringConfig(content, "description"), messageBody(content)),
			"url":         firstString(stringConfig(content, "url"), stringConfig(send, "url")),
		}
		if btntxt := firstString(stringConfig(content, "btntxt", "btn_txt"), stringConfig(send, "btntxt", "btn_txt")); btntxt != "" {
			textcard["btntxt"] = btntxt
		}
		body["textcard"] = textcard
	default:
		body["msgtype"] = "text"
		body["text"] = map[string]any{"content": messageBody(content)}
	}
	if value, ok := send["safe"]; ok {
		body["safe"] = value
	}
	for _, key := range []string{"enable_id_trans", "enable_duplicate_check", "duplicate_check_interval"} {
		if value, ok := send[key]; ok {
			body[key] = value
		}
	}
	return tokenQueryJSONRequest("POST", joinURL(baseURL, "/cgi-bin/message/send"), "access_token", body)
}

func dingTalkRobotRequestConfig(auth, send, content map[string]any, recipient any) (requestConfig, error) {
	baseURL := firstString(stringConfig(send, "base_url"), stringConfig(auth, "base_url"), "https://oapi.dingtalk.com")
	accessToken := firstRecipientString(recipient)
	if accessToken == "" {
		return requestConfig{}, ErrInvalidInput
	}
	requestURL := joinURL(baseURL, "/robot/send")
	parsed, err := url.Parse(requestURL)
	if err != nil {
		return requestConfig{}, ErrInvalidInput
	}
	values := parsed.Query()
	values.Set("access_token", accessToken)
	if secret := firstString(stringConfig(auth, "secret"), stringConfig(auth, "robot_secret")); secret != "" {
		timestamp := time.Now().UnixMilli()
		sign, err := dingTalkRobotSign(secret, timestamp)
		if err != nil {
			return requestConfig{}, ErrInvalidInput
		}
		values.Set("timestamp", fmt.Sprintf("%d", timestamp))
		values.Set("sign", sign)
	}
	parsed.RawQuery = values.Encode()
	msgType := normalizedDingTalkRobotMessageType(content)
	body := map[string]any{
		"msgtype": msgType,
		"at": map[string]any{
			"isAtAll": boolConfig(send, "isAtAll", "is_at_all", "at_all", "allow_at_all"),
		},
	}
	if msgType == "text" {
		body["text"] = map[string]any{"content": firstString(stringConfig(content, "content"), messageBody(content))}
	} else {
		body["markdown"] = map[string]any{
			"title": messageTitle(content),
			"text":  firstString(stringConfig(content, "text"), stringConfig(content, "markdown"), messageBody(content)),
		}
	}
	config, err := jsonRequest("POST", parsed.String(), body)
	if err != nil {
		return requestConfig{}, err
	}
	config.SkipRenderedMerge = true
	return config, nil
}

func dingTalkRobotSign(secret string, timestamp int64) (string, error) {
	stringToSign := fmt.Sprintf("%d\n%s", timestamp, secret)
	h := hmac.New(sha256.New, []byte(secret))
	if _, err := h.Write([]byte(stringToSign)); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(h.Sum(nil)), nil
}

func dingTalkWorkRequestConfig(auth, send, content map[string]any, recipient any) (requestConfig, error) {
	baseURL := firstString(stringConfig(send, "base_url"), stringConfig(auth, "base_url"), "https://api.dingtalk.com")
	msgKey := normalizedDingTalkWorkMsgKey(content)
	msgParam, err := dingTalkWorkMsgParam(msgKey, content)
	if err != nil {
		return requestConfig{}, ErrInvalidInput
	}
	body := map[string]any{
		"robotCode": firstString(stringConfig(send, "robot_code", "robotCode"), stringConfig(auth, "robot_code", "robotCode")),
		"userIds":   recipientStrings(recipient),
		"msgKey":    msgKey,
		"msgParam":  msgParam,
	}
	config, err := jsonRequest("POST", joinURL(baseURL, "/v1.0/robot/oToMessages/batchSend"), body)
	if err != nil {
		return requestConfig{}, err
	}
	config.Token = placementConfig{Location: PlacementHeader, FieldName: "x-acs-dingtalk-access-token"}
	config.Recipient = placementConfig{Location: PlacementBody, Path: "userIds", Format: "array"}
	config.SkipRenderedMerge = true
	return config, nil
}

func normalizedDingTalkRobotMessageType(content map[string]any) string {
	msgType := strings.TrimSpace(firstString(stringConfig(content, "msgtype", "msg_type", "msgKey", "msg_key", "type")))
	if strings.EqualFold(msgType, "text") {
		return "text"
	}
	if strings.EqualFold(msgType, "markdown") {
		return "markdown"
	}
	if stringConfig(content, "content") != "" && stringConfig(content, "title") == "" && stringConfig(content, "text") == "" {
		return "text"
	}
	return "markdown"
}

func normalizedDingTalkWorkMsgKey(content map[string]any) string {
	msgKey := strings.TrimSpace(firstString(stringConfig(content, "msgKey", "msg_key", "message_type", "type")))
	switch msgKey {
	case "sampleText", "text":
		return "sampleText"
	case "sampleMarkdown", "markdown":
		return "sampleMarkdown"
	default:
		if stringConfig(content, "content") != "" && stringConfig(content, "title") == "" && stringConfig(content, "text") == "" {
			return "sampleText"
		}
		return "sampleMarkdown"
	}
}

func dingTalkWorkMsgParam(msgKey string, content map[string]any) (string, error) {
	var payload map[string]string
	if msgKey == "sampleText" {
		payload = map[string]string{"content": firstString(stringConfig(content, "content"), messageBody(content))}
	} else {
		payload = map[string]string{
			"title": messageTitle(content),
			"text":  firstString(stringConfig(content, "text"), stringConfig(content, "markdown"), messageBody(content)),
		}
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func feishuRobotRequestConfig(auth, send, content map[string]any, recipient any) (requestConfig, error) {
	baseURL := firstString(stringConfig(send, "base_url"), stringConfig(auth, "base_url"), "https://open.feishu.cn/open-apis")
	contentString, err := json.Marshal(map[string]string{"text": messageBody(content)})
	if err != nil {
		return requestConfig{}, ErrInvalidInput
	}
	body := map[string]any{
		"receive_id": firstRecipientString(recipient),
		"msg_type":   "text",
		"content":    string(contentString),
	}
	requestURL := joinURL(baseURL, "/im/v1/messages")
	parsed, err := url.Parse(requestURL)
	if err != nil {
		return requestConfig{}, ErrInvalidInput
	}
	values := parsed.Query()
	values.Set("receive_id_type", "open_id")
	parsed.RawQuery = values.Encode()
	config, err := jsonRequest("POST", parsed.String(), body)
	if err != nil {
		return requestConfig{}, err
	}
	config.Token = placementConfig{Location: PlacementHeader, FieldName: "Authorization", Prefix: "Bearer "}
	config.Recipient = placementConfig{Location: PlacementBody, Path: "receive_id", Format: "string"}
	return config, nil
}

func feishuGroupRequestConfig(auth, send, content map[string]any, recipient any) (requestConfig, error) {
	baseURL := firstString(stringConfig(send, "base_url"), stringConfig(auth, "base_url"), "https://open.feishu.cn/open-apis")
	token := firstRecipientString(recipient)
	if token == "" {
		return requestConfig{}, ErrInvalidInput
	}
	body := map[string]any{
		"msg_type": "text",
		"content": map[string]any{
			"text": messageBody(content),
		},
	}
	if secret := firstString(stringConfig(auth, "sign_secret"), stringConfig(send, "sign_secret"), stringConfig(auth, "secret")); secret != "" {
		timestamp := time.Now().Unix()
		sign, err := feishuGroupSign(secret, timestamp)
		if err != nil {
			return requestConfig{}, ErrInvalidInput
		}
		body["timestamp"] = fmt.Sprintf("%d", timestamp)
		body["sign"] = sign
	}
	requestURL := joinURL(baseURL, "/bot/v2/hook/"+url.PathEscape(token))
	config, err := jsonRequest("POST", requestURL, body)
	if err != nil {
		return requestConfig{}, err
	}
	config.Recipient = placementConfig{Location: PlacementPath, FieldName: "token", Format: "string"}
	return config, nil
}

func feishuGroupSign(secret string, timestamp int64) (string, error) {
	stringToSign := fmt.Sprintf("%d", timestamp) + "\n" + secret
	h := hmac.New(sha256.New, []byte(stringToSign))
	if _, err := h.Write(nil); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(h.Sum(nil)), nil
}
