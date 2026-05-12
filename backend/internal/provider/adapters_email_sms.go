package provider

import (
	"fmt"
	"sort"
)

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
