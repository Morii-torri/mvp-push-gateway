package provider

func selfCapability(messageType string) Capability {
	return capability(capabilitySpec{
		ProviderType:         ProviderSelf,
		DisplayName:          "MVP Push Gateway cascade",
		Category:             "gateway",
		MessageType:          messageType,
		MessageSchema:        cascadeContentSchema(),
		CredentialSchema:     rawJSON(`{"type":"object","required":["base_url","source_code"],"properties":{"base_url":{"type":"string"},"source_code":{"type":"string"},"source_token":{"type":"string","format":"password"},"hmac_secret":{"type":"string","format":"password"},"auth_mode":{"type":"string","enum":["token","hmac","token_and_hmac","none"],"default":"token"}}}`),
		ChannelConfigSchema:  rawJSON(`{"type":"object","properties":{"api_prefix":{"type":"string","default":"/api/v1"},"source_code":{"type":"string"},"payload_mode":{"type":"string","enum":["wrapped","raw"],"default":"wrapped"},"include_trace_id":{"type":"boolean","default":true},"include_source_context":{"type":"boolean","default":true}}}`),
		RecipientRequired:    false,
		AllowNoRecipient:     true,
		RecipientRequirement: "none",
		RecipientFieldName:   "recipients",
		RecipientLocation:    PlacementBody,
		RecipientPath:        "recipients",
		RecipientFormat:      "array",
		IdentityKind:         "system_user_id",
		TokenLocation:        PlacementHeader,
		TokenFieldName:       "Authorization",
		TokenStrategy:        rawJSON(`{"strategy":"static_bearer","cacheable":false,"placement":{"location":"header","field_name":"Authorization","prefix":"Bearer "}}`),
		SendAPI:              rawJSON(`{"method":"POST","path":"/api/v1/ingest/{source_code}","content_type":"application/json","success_status":202,"live_test_status":"implemented_but_not_live_tested","notes":"Build-request adapter targets another MVP Push Gateway instance and does not perform live cascade tests."}`),
		SuccessRule:          rawJSON(`{"type":"status_and_json_field","status_codes":[202],"field":"status","equals":"accepted"}`),
		RetryRule:            rawJSON(`{"status_codes":[408,429,500,502,503,504],"network_errors":true,"non_retryable_status_codes":[401,403,404,413]}`),
		DefaultRateLimit:     rawJSON(`{"qps":20,"burst":40}`),
		DefaultConcurrency:   4,
		DefaultTimeoutMS:     5000,
		DefaultRetryPolicy:   rawJSON(`{"max_attempts":3,"delay_ms":1000,"backoff":"linear"}`),
		RequestExamples:      rawJSON(`{"message":{"title":"Disk alert","body":"Disk 95%"},"recipients":["user-1"],"context":{"source":"downstream"}}`),
		CustomBodyAllowed:    false,
	})
}
