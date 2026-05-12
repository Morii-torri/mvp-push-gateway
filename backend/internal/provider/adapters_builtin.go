package provider

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
)

const govCloudDefaultBaseURL = "https://www.ywxt.sh.cegn.cn/api-gateway/uranus/uranus/cgi-bin/"

func builtInRequestConfig(channel Channel, input BuildRequestInput) (requestConfig, bool, error) {
	auth, err := decodeObjectConfig(channel.AuthConfig)
	if err != nil {
		return requestConfig{}, false, err
	}
	send, err := decodeObjectConfig(channel.SendConfig)
	if err != nil {
		return requestConfig{}, false, err
	}
	content, err := decodeObjectConfig(input.Body)
	if err != nil {
		return requestConfig{}, false, err
	}

	switch channel.ProviderType {
	case ProviderPushPlus:
		config, err := pushPlusRequestConfig(auth, send, content)
		return config, true, err
	case ProviderWxPusher:
		config, err := wxPusherRequestConfig(auth, send, content, input.Recipient)
		return config, true, err
	case ProviderServerChan:
		config, err := serverChanRequestConfig(auth, send, content)
		return config, true, err
	case ProviderSelf:
		config, err := selfRequestConfig(auth, send, content, input)
		return config, true, err
	case ProviderSMS, ProviderAliyunSMS, ProviderTencentSMS, ProviderBaiduSMS:
		config, err := smsRequestConfig(channel.ProviderType, auth, send, content, input.Recipient)
		return config, true, err
	case ProviderWeComRobot:
		config, err := weComRobotRequestConfig(auth, send, content, input.Recipient)
		return config, true, err
	case ProviderWeCom, ProviderWeComApp:
		config, err := weComAppRequestConfig(auth, send, content, input.Recipient)
		return config, true, err
	case ProviderDingTalkRobot:
		config, err := dingTalkRobotRequestConfig(auth, send, content, input.Recipient)
		return config, true, err
	case ProviderDingTalk, ProviderDingTalkWork:
		config, err := dingTalkWorkRequestConfig(auth, send, content, input.Recipient)
		return config, true, err
	case ProviderFeishuRobot:
		config, err := feishuRobotRequestConfig(auth, send, content)
		return config, true, err
	case ProviderGovCloud:
		config, err := govCloudRequestConfig(auth, send, content, input.Recipient)
		return config, true, err
	default:
		return requestConfig{}, false, nil
	}
}

func pushPlusRequestConfig(auth, send, content map[string]any) (requestConfig, error) {
	bodyText := appendURL(messageBody(content), stringConfig(content, "url"))
	template := firstString(stringConfig(send, "template"), pushPlusTemplate(stringConfig(content, "format")), "markdown")
	body := map[string]any{
		"token":    firstString(stringConfig(auth, "token"), stringConfig(send, "token")),
		"title":    messageTitle(content),
		"content":  bodyText,
		"template": template,
	}
	copyStringField(body, "topic", send, "topic")
	copyStringField(body, "channel", send, "channel")
	return jsonRequest("POST", firstString(stringConfig(send, "url"), "https://www.pushplus.plus/send"), body)
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

func smsRequestConfig(providerType ProviderType, auth, send, content map[string]any, recipient any) (requestConfig, error) {
	vendor := smsVendor(providerType, auth, send)
	switch vendor {
	case "tencent":
		return tencentSMSRequestConfig(send, content, recipient)
	case "baidu":
		return baiduSMSRequestConfig(send, content, recipient)
	default:
		return aliyunSMSRequestConfig(send, content, recipient)
	}
}

func aliyunSMSRequestConfig(send, content map[string]any, recipient any) (requestConfig, error) {
	body := map[string]any{
		"vendor":           "aliyun",
		"Action":           "SendSms",
		"phone_numbers":    recipientStrings(recipient),
		"sign_name":        firstString(stringConfig(content, "sign_name"), stringConfig(send, "sign_name")),
		"template_code":    firstString(stringConfig(content, "template_id", "template_code"), stringConfig(send, "template_id", "template_code")),
		"template_param":   smsTemplateParams(content),
		"mock_signature":   true,
		"live_test_status": "implemented_but_not_live_tested",
	}
	copyStringField(body, "region", send, "region")
	copyStringField(body, "out_id", content, "out_id")
	return jsonRequest("POST", firstString(stringConfig(send, "endpoint"), "https://dysmsapi.aliyuncs.com/"), body)
}

func tencentSMSRequestConfig(send, content map[string]any, recipient any) (requestConfig, error) {
	body := map[string]any{
		"vendor":           "tencent",
		"Action":           "SendSms",
		"PhoneNumberSet":   recipientStrings(recipient),
		"SmsSdkAppId":      stringConfig(send, "sms_sdk_app_id", "SmsSdkAppId"),
		"SignName":         firstString(stringConfig(content, "sign_name"), stringConfig(send, "sign_name", "SignName")),
		"TemplateId":       firstString(stringConfig(content, "template_id", "TemplateId"), stringConfig(send, "template_id", "TemplateId")),
		"TemplateParamSet": smsTemplateParamSet(content),
		"mock_signature":   true,
		"live_test_status": "implemented_but_not_live_tested",
	}
	copyStringField(body, "Region", send, "region", "Region")
	return jsonRequest("POST", firstString(stringConfig(send, "endpoint"), "https://sms.tencentcloudapi.com/"), body)
}

func baiduSMSRequestConfig(send, content map[string]any, recipient any) (requestConfig, error) {
	body := map[string]any{
		"vendor":           "baidu",
		"mobile":           recipientStrings(recipient),
		"signature_id":     firstString(stringConfig(content, "signature_id"), stringConfig(send, "signature_id")),
		"template":         firstString(stringConfig(content, "template_id", "template"), stringConfig(send, "template_id", "template")),
		"content_var":      smsTemplateParams(content),
		"mock_signature":   true,
		"live_test_status": "implemented_but_not_live_tested",
	}
	copyStringField(body, "region", send, "region")
	endpoint := firstString(stringConfig(send, "endpoint"), "https://sms.bj.baidubce.com/bce/v2/message")
	return jsonRequest("POST", endpoint, body)
}

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

func smsVendor(providerType ProviderType, auth, send map[string]any) string {
	switch providerType {
	case ProviderTencentSMS:
		return "tencent"
	case ProviderBaiduSMS:
		return "baidu"
	case ProviderAliyunSMS:
		return "aliyun"
	default:
		return firstString(stringConfig(send, "provider_subtype", "vendor"), stringConfig(auth, "provider_subtype", "vendor"), "aliyun")
	}
}

func smsTemplateParams(content map[string]any) any {
	if value, ok := content["template_params"]; ok {
		return value
	}
	if value, ok := content["TemplateParam"]; ok {
		return value
	}
	if body := messageBody(content); body != "" {
		return map[string]any{"content": body}
	}
	return map[string]any{}
}

func smsTemplateParamSet(content map[string]any) []string {
	value := smsTemplateParams(content)
	switch typed := value.(type) {
	case []any:
		values := make([]string, 0, len(typed))
		for _, item := range typed {
			values = append(values, fmt.Sprint(item))
		}
		return values
	case []string:
		return append([]string(nil), typed...)
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		values := make([]string, 0, len(keys))
		for _, key := range keys {
			values = append(values, fmt.Sprint(typed[key]))
		}
		return values
	default:
		if value == nil {
			return nil
		}
		return []string{fmt.Sprint(value)}
	}
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
