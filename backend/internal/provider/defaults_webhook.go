package provider

func webhookCapability() Capability {
	return capability(capabilitySpec{
		ProviderType:         ProviderWebhook,
		DisplayName:          "Generic Webhook",
		Category:             "advanced",
		MessageType:          "json",
		MessageSchema:        webhookContentSchema(),
		CredentialSchema:     rawJSON(`{"type":"object","properties":{"secret":{"type":"string","format":"password"},"headers":{"type":"object","additionalProperties":{"type":"string"}}}}`),
		ChannelConfigSchema:  rawJSON(`{"type":"object","required":["url"],"properties":{"url":{"type":"string"},"method":{"type":"string","default":"POST"},"headers":{"type":"object","additionalProperties":{"type":"string"}},"body":{"type":"object","additionalProperties":true},"recipient":{"type":"object","additionalProperties":true}}}`),
		RecipientRequired:    false,
		AllowNoRecipient:     true,
		RecipientRequirement: "none",
		RecipientLocation:    PlacementNone,
		RecipientFormat:      "string",
		TokenLocation:        PlacementNone,
		TokenStrategy:        rawJSON(`{"strategy":"none","cacheable":false,"placement":{"location":"none"}}`),
		SendAPI:              rawJSON(`{"method":"POST","url_template":"{{ channel.url }}","content_type":"application/json","live_test_status":"configuration_dependent","notes":"Advanced legacy send_config.url/body/recipient/token mappings remain authoritative."}`),
		SuccessRule:          rawJSON(`{"type":"status_code","status_codes":[200,201,202,204]}`),
		RetryRule:            rawJSON(`{"status_codes":[408,429,500,502,503,504],"network_errors":true,"non_retryable_status_classes":[400]}`),
		DefaultRateLimit:     rawJSON(`{"qps":10}`),
		DefaultConcurrency:   5,
		DefaultTimeoutMS:     5000,
		DefaultRetryPolicy:   rawJSON(`{"max_attempts":3,"delay_ms":1000,"backoff":"linear"}`),
		RequestExamples:      rawJSON(`{"payload":{"title":"Disk alert","body":"Disk 95%"}}`),
		CustomBodyAllowed:    true,
	})
}
