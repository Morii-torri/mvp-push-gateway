package provider

func ntfyCapability() Capability {
	return capability(capabilitySpec{
		ProviderType:         ProviderNtfy,
		DisplayName:          "ntfy",
		Category:             "self_hosted",
		MessageType:          "notice",
		MessageSchema:        noticeContentSchema(),
		CredentialSchema:     rawJSON(`{"type":"object","required":["server_url"],"properties":{"server_url":{"type":"string","default":"https://ntfy.sh"},"auth_type":{"type":"string","enum":["none","basic","bearer"],"default":"none"},"username":{"type":"string"},"password":{"type":"string","format":"password"},"bearer_token":{"type":"string","format":"password"}}}`),
		ChannelConfigSchema:  rawJSON(`{"type":"object","required":["topic"],"properties":{"topic":{"type":"string"},"priority":{"type":["string","integer"],"default":"default"},"tags":{"type":"array","items":{"type":"string"}},"markdown":{"type":"boolean","default":false},"actions":{"type":"array","items":{"type":"object","additionalProperties":true}}}}`),
		RecipientRequired:    false,
		AllowNoRecipient:     true,
		RecipientRequirement: "none",
		RecipientLocation:    PlacementNone,
		RecipientFormat:      "none",
		TokenLocation:        PlacementHeader,
		TokenFieldName:       "Authorization",
		TokenStrategy:        rawJSON(`{"strategy":"static_auth","cacheable":false,"placement":{"location":"header","field_name":"Authorization"},"supported":["none","basic","bearer"]}`),
		SendAPI:              rawJSON(`{"method":"POST","url_template":"{server_url}/{topic}","content_type":"text/plain","adapter":"mock_http","live_test_status":"configuration_dependent","notes":"ntfy may target ntfy.sh or a self-hosted server. This adapter builds a deterministic HTTP request and does not live-test a server."}`),
		SuccessRule:          rawJSON(`{"type":"status_code","status_codes":[200,201,202]}`),
		RetryRule:            rawJSON(`{"status_codes":[408,429,500,502,503,504],"network_errors":true,"non_retryable_status_codes":[400,401,403,404]}`),
		DefaultRateLimit:     rawJSON(`{"qps":5}`),
		DefaultConcurrency:   2,
		DefaultTimeoutMS:     5000,
		DefaultRetryPolicy:   rawJSON(`{"max_attempts":3,"delay_ms":1000,"backoff":"linear"}`),
		RequestExamples:      rawJSON(`{"headers":{"Title":"Disk alert","Priority":"high","Tags":"warning,disk"},"body":"Disk 95%"}`),
		CustomBodyAllowed:    false,
	})
}

func gotifyCapability() Capability {
	return capability(capabilitySpec{
		ProviderType:         ProviderGotify,
		DisplayName:          "Gotify",
		Category:             "self_hosted",
		MessageType:          "notice",
		MessageSchema:        noticeContentSchema(),
		CredentialSchema:     rawJSON(`{"type":"object","required":["server_url","app_token"],"properties":{"server_url":{"type":"string"},"app_token":{"type":"string","format":"password"}}}`),
		ChannelConfigSchema:  rawJSON(`{"type":"object","properties":{"priority":{"type":"integer","default":5},"content_type":{"type":"string","enum":["text/plain","text/markdown"],"default":"text/plain"}}}`),
		RecipientRequired:    false,
		AllowNoRecipient:     true,
		RecipientRequirement: "none",
		RecipientLocation:    PlacementNone,
		RecipientFormat:      "none",
		TokenLocation:        PlacementQuery,
		TokenFieldName:       "token",
		TokenStrategy:        rawJSON(`{"strategy":"static_query_token","cacheable":false,"placement":{"location":"query","field_name":"token"}}`),
		SendAPI:              rawJSON(`{"method":"POST","url_template":"{server_url}/message?token={app_token}","content_type":"application/json","adapter":"mock_http","live_test_status":"configuration_dependent","notes":"Gotify is usually self-hosted. This adapter builds a deterministic HTTP request and does not live-test a server."}`),
		SuccessRule:          rawJSON(`{"type":"status_code","status_codes":[200,201,202]}`),
		RetryRule:            rawJSON(`{"status_codes":[408,429,500,502,503,504],"network_errors":true,"non_retryable_status_codes":[400,401,403,404]}`),
		DefaultRateLimit:     rawJSON(`{"qps":10}`),
		DefaultConcurrency:   3,
		DefaultTimeoutMS:     5000,
		DefaultRetryPolicy:   rawJSON(`{"max_attempts":3,"delay_ms":1000,"backoff":"linear"}`),
		RequestExamples:      rawJSON(`{"title":"Disk alert","message":"Disk 95%","priority":5,"extras":{"client::display":{"contentType":"text/markdown"}}}`),
		CustomBodyAllowed:    false,
	})
}
