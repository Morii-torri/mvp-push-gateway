package provider

import "encoding/json"

func emailCapability(messageType string) Capability {
	return capability(capabilitySpec{
		ProviderType:         ProviderEmail,
		DisplayName:          "SMTP email",
		Category:             "email",
		MessageType:          messageType,
		MessageSchema:        emailContentSchema(),
		CredentialSchema:     rawJSON(`{"type":"object","required":["host","port","username","password","from"],"properties":{"host":{"type":"string"},"port":{"type":"integer","default":587},"secure":{"type":"boolean"},"start_tls":{"type":"boolean","default":true},"username":{"type":"string"},"password":{"type":"string","format":"password"},"from":{"type":"string","format":"email"},"reply_to":{"type":"string","format":"email"}}}`),
		ChannelConfigSchema:  rawJSON(`{"type":"object","properties":{"cc":{"type":"array","items":{"type":"string","format":"email"}},"bcc":{"type":"array","items":{"type":"string","format":"email"}}}}`),
		RecipientRequired:    true,
		RecipientRequirement: "system",
		RecipientFieldName:   "to",
		RecipientLocation:    PlacementBody,
		RecipientPath:        "to",
		RecipientFormat:      "array",
		IdentityKind:         "email",
		TokenLocation:        PlacementNone,
		TokenStrategy:        rawJSON(`{"strategy":"smtp_auth","cacheable":false,"placement":{"location":"none"}}`),
		SendAPI:              rawJSON(`{"method":"SMTP_SEND","protocol":"smtp","content_type":"text/html","live_test_status":"implemented_but_not_live_tested","notes":"Build-request snapshot only; no SMTP server is contacted by provider tests."}`),
		SuccessRule:          rawJSON(`{"type":"transport_ack","success":"smtp_server_accepted"}`),
		RetryRule:            rawJSON(`{"smtp_codes":[421,450,451,452],"network_errors":true,"non_retryable_smtp_codes":[535,550,553]}`),
		DefaultRateLimit:     rawJSON(`{"qps":5}`),
		DefaultConcurrency:   2,
		DefaultTimeoutMS:     10000,
		DefaultRetryPolicy:   rawJSON(`{"max_attempts":3,"delay_ms":2000,"backoff":"exponential"}`),
		RequestExamples:      rawJSON(`{"to":["user@example.com"],"subject":"Disk alert","html":"<p>Disk 95%</p>","text":"Disk 95%"}`),
		CustomBodyAllowed:    false,
	})
}

func smsLegacyCapability() Capability {
	return capability(capabilitySpec{
		ProviderType:         ProviderSMS,
		DisplayName:          "SMS provider (legacy aggregate)",
		Category:             "sms",
		MessageType:          "text",
		MessageSchema:        smsContentSchema(),
		CredentialSchema:     rawJSON(`{"type":"object","required":["provider_subtype"],"properties":{"provider_subtype":{"type":"string","enum":["aliyun","tencent","baidu"]},"access_key_id":{"type":"string"},"access_key_secret":{"type":"string","format":"password"},"secret_id":{"type":"string"},"secret_key":{"type":"string","format":"password"}}}`),
		ChannelConfigSchema:  rawJSON(`{"type":"object","required":["sign_name","template_id"],"properties":{"sign_name":{"type":"string"},"template_id":{"type":"string"},"region":{"type":"string"},"endpoint":{"type":"string"}}}`),
		RecipientRequired:    true,
		RecipientRequirement: "system",
		RecipientFieldName:   "phones",
		RecipientLocation:    PlacementBody,
		RecipientPath:        "phones",
		RecipientFormat:      "array",
		IdentityKind:         "mobile",
		TokenLocation:        PlacementNone,
		TokenStrategy:        rawJSON(`{"strategy":"vendor_signature","cacheable":false,"placement":{"location":"none"},"live_test_status":"implemented_but_not_live_tested"}`),
		SendAPI:              rawJSON(`{"method":"VENDOR_SEND_SMS","protocol":"mock_http","content_type":"application/json","live_test_status":"implemented_but_not_live_tested","notes":"Legacy sms remains as an aggregate alias. Prefer aliyun_sms, tencent_sms, or baidu_sms for first-batch defaults."}`),
		SuccessRule:          rawJSON(`{"type":"vendor_code","success_values":["OK","Ok","Success"]}`),
		RetryRule:            rawJSON(`{"status_codes":[408,429,500,502,503,504],"vendor_codes":["Throttling","InternalError","RequestLimitExceeded"]}`),
		DefaultRateLimit:     rawJSON(`{"qps":10}`),
		DefaultConcurrency:   3,
		DefaultTimeoutMS:     8000,
		DefaultRetryPolicy:   rawJSON(`{"max_attempts":3,"delay_ms":1000,"backoff":"exponential"}`),
		RequestExamples:      rawJSON(`{"phones":["13800138000"],"template_params":{"code":"1234"}}`),
		CustomBodyAllowed:    false,
	})
}

func smsVendorCapability(providerType ProviderType, displayName string, vendor string, sendAPI, example json.RawMessage) Capability {
	return capability(capabilitySpec{
		ProviderType:         providerType,
		DisplayName:          displayName,
		Category:             "sms",
		MessageType:          "sms_template",
		MessageSchema:        smsTemplateContentSchema(),
		CredentialSchema:     smsCredentialSchema(vendor),
		ChannelConfigSchema:  smsChannelSchema(vendor),
		RecipientRequired:    true,
		RecipientRequirement: "system",
		RecipientFieldName:   "phones",
		RecipientLocation:    PlacementBody,
		RecipientPath:        "phones",
		RecipientFormat:      "array",
		IdentityKind:         "mobile",
		TokenLocation:        PlacementNone,
		TokenStrategy:        rawJSON(`{"strategy":"vendor_signature","cacheable":false,"placement":{"location":"none"},"live_test_status":"implemented_but_not_live_tested","notes":"Signature metadata only. This phase does not use vendor SDKs or call live SMS endpoints."}`),
		SendAPI:              sendAPI,
		SuccessRule:          smsSuccessRule(vendor),
		RetryRule:            rawJSON(`{"status_codes":[408,429,500,502,503,504],"network_errors":true,"retryable_vendor_codes":["Throttling","InternalError","RequestLimitExceeded"],"non_retryable_vendor_codes":["InvalidAccessKeyId","AuthFailure.SecretIdNotFound","SignatureDoesNotMatch","TemplateNotApproved","InsufficientBalance"]}`),
		DefaultRateLimit:     rawJSON(`{"qps":5}`),
		DefaultConcurrency:   2,
		DefaultTimeoutMS:     8000,
		DefaultRetryPolicy:   rawJSON(`{"max_attempts":3,"delay_ms":1000,"backoff":"exponential"}`),
		RequestExamples:      example,
		CustomBodyAllowed:    false,
	})
}

func smsCredentialSchema(vendor string) json.RawMessage {
	switch vendor {
	case "tencent":
		return rawJSON(`{"type":"object","required":["secret_id","secret_key"],"properties":{"secret_id":{"type":"string"},"secret_key":{"type":"string","format":"password"}}}`)
	case "baidu":
		return rawJSON(`{"type":"object","required":["access_key_id","secret_access_key"],"properties":{"access_key_id":{"type":"string"},"secret_access_key":{"type":"string","format":"password"}}}`)
	default:
		return rawJSON(`{"type":"object","required":["access_key_id","access_key_secret"],"properties":{"access_key_id":{"type":"string"},"access_key_secret":{"type":"string","format":"password"}}}`)
	}
}

func smsChannelSchema(vendor string) json.RawMessage {
	switch vendor {
	case "tencent":
		return rawJSON(`{"type":"object","required":["sms_sdk_app_id","sign_name","template_id"],"properties":{"sms_sdk_app_id":{"type":"string"},"sign_name":{"type":"string"},"template_id":{"type":"string"},"region":{"type":"string","default":"ap-shanghai"},"endpoint":{"type":"string"}}}`)
	case "baidu":
		return rawJSON(`{"type":"object","required":["signature_id","template_id"],"properties":{"signature_id":{"type":"string"},"signature":{"type":"string"},"template_id":{"type":"string"},"region":{"type":"string","default":"bj"},"endpoint":{"type":"string"}}}`)
	default:
		return rawJSON(`{"type":"object","required":["sign_name","template_id"],"properties":{"sign_name":{"type":"string"},"template_id":{"type":"string"},"region":{"type":"string","default":"cn-hangzhou"},"endpoint":{"type":"string","default":"https://dysmsapi.aliyuncs.com/"}}}`)
	}
}

func smsSuccessRule(vendor string) json.RawMessage {
	switch vendor {
	case "tencent":
		return rawJSON(`{"type":"json_array_field","status_codes":[200],"array_field":"SendStatusSet","field":"Code","equals":"Ok"}`)
	case "baidu":
		return rawJSON(`{"type":"vendor_response_code","status_codes":[200],"success_values":["1000","OK","Success"]}`)
	default:
		return rawJSON(`{"type":"json_field","status_codes":[200],"field":"Code","equals":"OK"}`)
	}
}
