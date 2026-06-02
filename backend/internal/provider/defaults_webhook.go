package provider

func webhookCapability() Capability {
	return capability(capabilitySpec{
		ProviderType:         ProviderWebhook,
		DisplayName:          "Generic Webhook",
		Category:             "advanced",
		MessageType:          "json",
		MessageSchema:        webhookContentSchema(),
		CredentialSchema:     rawJSON(`{"type":"object","properties":{}}`),
		ChannelConfigSchema:  rawJSON(`{"type":"object","required":["url"],"field_order":["url","method","headers"],"properties":{"url":{"type":"string"},"method":{"type":"string","default":"POST","enum":["POST","GET"]},"headers":{"type":"object","additionalProperties":{"type":"string"}}}}`),
		RecipientRequired:    false,
		AllowNoRecipient:     true,
		RecipientRequirement: "none",
		RecipientLocation:    PlacementNone,
		RecipientFormat:      "string",
		TokenLocation:        PlacementNone,
		TokenStrategy:        rawJSON(`{"strategy":"none","cacheable":false,"placement":{"location":"none"}}`),
		SendAPI:              rawJSON(`{"method":"POST","url_template":"{{ channel.url }}","content_type":"application/json","live_test_status":"configuration_dependent","notes":"URL 可使用 {{ identity }} 引用接收人的平台身份字段；POST 使用模板 body 作为 JSON Body，GET 将 body 对象转为 Query。"}`),
		SuccessRule:          rawJSON(`{"type":"status_code","status_codes":[200,201,202,204]}`),
		RetryRule:            rawJSON(`{"status_codes":[408,429,500,502,503,504],"network_errors":true,"non_retryable_status_classes":[400]}`),
		DefaultRateLimit:     rawJSON(`{"qps":10}`),
		DefaultConcurrency:   5,
		DefaultTimeoutMS:     5000,
		DefaultRetryPolicy:   rawJSON(`{"max_attempts":3,"delay_ms":1000,"backoff":"linear"}`),
		RequestExamples:      rawJSON(`{"body":{"title":"Disk alert","body":"Disk 95%"}}`),
		CustomBodyAllowed:    true,
	})
}
