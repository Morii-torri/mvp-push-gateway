package provider

import (
	"net/url"
	"strings"
)

func weComRobotRequestConfig(auth, send, content map[string]any, recipient any) (requestConfig, error) {
	requestURL := firstString(stringConfig(auth, "webhook_url", "webhookUrl"), stringConfig(send, "webhook_url", "webhookUrl"))
	if requestURL == "" {
		key := firstString(stringConfig(auth, "key"), stringConfig(send, "key"))
		if key == "" {
			return requestConfig{}, ErrInvalidInput
		}
		baseURL := firstString(stringConfig(send, "base_url"), stringConfig(auth, "base_url"), "https://qyapi.weixin.qq.com")
		requestURL = joinURL(baseURL, "/cgi-bin/webhook/send") + "?key=" + url.QueryEscape(key)
	}
	text := map[string]any{"content": messageBody(content)}
	if recipients := recipientStrings(recipient); len(recipients) > 0 {
		text["mentioned_list"] = recipients
	}
	body := map[string]any{"msgtype": "text", "text": text}
	return jsonRequest("POST", requestURL, body)
}

func weComAppRequestConfig(auth, send, content map[string]any, recipient any) (requestConfig, error) {
	baseURL := firstString(stringConfig(send, "base_url"), stringConfig(auth, "base_url"), "https://qyapi.weixin.qq.com")
	body := map[string]any{
		"touser":  strings.Join(recipientStrings(recipient), "|"),
		"msgtype": "text",
		"agentid": firstValue(send, auth, "agentid", "agent_id"),
		"text":    map[string]any{"content": messageBody(content)},
	}
	if value, ok := send["safe"]; ok {
		body["safe"] = value
	}
	return tokenQueryJSONRequest("POST", joinURL(baseURL, "/cgi-bin/message/send"), "access_token", body)
}

func dingTalkRobotRequestConfig(auth, send, content map[string]any, recipient any) (requestConfig, error) {
	requestURL := firstString(stringConfig(auth, "webhook_url", "webhookUrl"), stringConfig(send, "webhook_url", "webhookUrl"))
	if requestURL == "" {
		accessToken := firstString(stringConfig(auth, "access_token"), stringConfig(send, "access_token"))
		if accessToken == "" {
			return requestConfig{}, ErrInvalidInput
		}
		requestURL = "https://oapi.dingtalk.com/robot/send?access_token=" + url.QueryEscape(accessToken)
	}
	body := map[string]any{
		"msgtype": "text",
		"text":    map[string]any{"content": messageBody(content)},
		"at": map[string]any{
			"atMobiles": recipientStrings(recipient),
			"isAtAll":   boolConfig(send, "is_at_all", "at_all", "isAtAll"),
		},
	}
	return jsonRequest("POST", requestURL, body)
}

func dingTalkWorkRequestConfig(auth, send, content map[string]any, recipient any) (requestConfig, error) {
	baseURL := firstString(stringConfig(send, "base_url"), stringConfig(auth, "base_url"), "https://oapi.dingtalk.com")
	body := map[string]any{
		"agent_id":    firstValue(send, auth, "agent_id", "agentid"),
		"userid_list": strings.Join(recipientStrings(recipient), ","),
		"msg": map[string]any{
			"msgtype": "text",
			"text":    map[string]any{"content": messageBody(content)},
		},
	}
	if boolConfig(send, "to_all_user", "toAllUser") {
		body["to_all_user"] = true
	}
	return tokenQueryJSONRequest("POST", joinURL(baseURL, "/topapi/message/corpconversation/asyncsend_v2"), "access_token", body)
}

func feishuRobotRequestConfig(auth, send, content map[string]any) (requestConfig, error) {
	requestURL := firstString(stringConfig(auth, "webhook_url", "webhookUrl"), stringConfig(send, "webhook_url", "webhookUrl"))
	if requestURL == "" {
		token := firstString(stringConfig(auth, "token"), stringConfig(send, "token"))
		if token == "" {
			return requestConfig{}, ErrInvalidInput
		}
		baseURL := firstString(stringConfig(send, "base_url"), stringConfig(auth, "base_url"), "https://open.feishu.cn")
		requestURL = joinURL(baseURL, "/open-apis/bot/v2/hook/"+url.PathEscape(token))
	}
	body := map[string]any{
		"msg_type": "text",
		"content":  map[string]any{"text": messageBody(content)},
	}
	return jsonRequest("POST", requestURL, body)
}

func govCloudRequestConfig(auth, send, content map[string]any, recipient any) (requestConfig, error) {
	baseURL := firstString(stringConfig(send, "base_url"), stringConfig(auth, "base_url"), govCloudDefaultBaseURL)
	body := map[string]any{
		"touser":      strings.Join(recipientStrings(recipient), "|"),
		"toparty":     stringConfig(send, "toparty"),
		"totag":       stringConfig(send, "totag"),
		"msgtype":     "text",
		"description": firstString(stringConfig(content, "description"), messageBody(content)),
	}
	if boolConfig(send, "at_all", "allow_at_all") {
		body["touser"] = "@all"
		body["toparty"] = ""
		body["totag"] = ""
	}
	return tokenQueryJSONRequest("POST", joinURL(baseURL, "/request/message/send"), "access_token", body)
}
