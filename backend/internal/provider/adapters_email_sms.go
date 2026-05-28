package provider

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

func emailRequestConfig(auth, send, content map[string]any, recipient any) (requestConfig, error) {
	host := firstString(stringConfig(auth, "host"), stringConfig(send, "host"))
	port := firstPositiveInt(rawConfig(auth, "port"), rawConfig(send, "port"))
	security := strings.ToUpper(firstString(stringConfig(auth, "security"), stringConfig(send, "security"), "SSL"))
	if security != "SSL" && security != "STARTTLS" {
		security = "SSL"
	}
	if port == 0 {
		if security == "STARTTLS" {
			port = 587
		} else {
			port = 465
		}
	}

	requestURL := ""
	if host != "" {
		requestURL = fmt.Sprintf("smtp://%s:%d", host, port)
	}
	username := firstString(stringConfig(auth, "username"), stringConfig(send, "username"))
	from := firstString(stringConfig(send, "from"), stringConfig(auth, "from"))
	if normalizedFrom, err := smtpFromHeader(username, from); err == nil {
		from = normalizedFrom
	}
	body := map[string]any{
		"host":                host,
		"port":                port,
		"security":            security,
		"username":            username,
		"from":                from,
		"to":                  recipientStrings(recipient),
		"subject":             firstString(stringConfig(content, "subject"), stringConfig(content, "title"), "通知"),
		"body":                firstString(stringConfig(content, "body"), stringConfig(content, "text"), stringConfig(content, "content"), stringConfig(content, "html")),
		"format":              normalizedEmailContentFormat(firstString(stringConfig(content, "format"), stringConfig(content, "content_type"))),
		"password_configured": firstString(stringConfig(auth, "password"), stringConfig(send, "password")) != "",
		"live_test_status":    "implemented_but_not_live_tested",
	}
	if envelopeFrom := smtpEnvelopeFrom(body["username"].(string), body["from"].(string)); envelopeFrom != "" {
		body["smtp_envelope_from"] = envelopeFrom
	}
	if cc := listConfig(send, "cc"); len(cc) > 0 {
		body["cc"] = cc
	}
	if bcc := listConfig(send, "bcc"); len(bcc) > 0 {
		body["bcc"] = bcc
	}
	copyStringField(body, "reply_to", send, "reply_to")
	return jsonRequest("SMTP_SEND", requestURL, body)
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

func firstPositiveInt(values ...any) int {
	for _, value := range values {
		switch typed := value.(type) {
		case int:
			if typed > 0 {
				return typed
			}
		case int64:
			if typed > 0 {
				return int(typed)
			}
		case float64:
			if typed > 0 {
				return int(typed)
			}
		case json.Number:
			if number, err := strconv.Atoi(string(typed)); err == nil && number > 0 {
				return number
			}
		case string:
			if number, err := strconv.Atoi(strings.TrimSpace(typed)); err == nil && number > 0 {
				return number
			}
		}
	}
	return 0
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
