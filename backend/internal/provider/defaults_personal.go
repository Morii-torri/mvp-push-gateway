package provider

import "encoding/json"

func pushPlusCapability() Capability {
	return capability(capabilitySpec{
		ProviderType:         ProviderPushPlus,
		DisplayName:          "PushPlus",
		Category:             "personal_gateway",
		MessageType:          "html",
		MessageSchema:        pushPlusContentSchema(),
		CredentialSchema:     rawJSON(`{"type":"object","properties":{}}`),
		ChannelConfigSchema:  rawJSON(`{"type":"object","properties":{}}`),
		RecipientRequired:    true,
		RecipientRequirement: "system",
		RecipientFieldName:   "token",
		RecipientLocation:    PlacementBody,
		RecipientPath:        "token",
		RecipientFormat:      "string",
		IdentityKind:         "pushplus_token",
		TokenLocation:        PlacementBody,
		TokenFieldName:       "token",
		TokenStrategy:        rawJSON(`{"strategy":"target_token","cacheable":false,"placement":{"location":"body","field_name":"token"},"notes":"PushPlus token is resolved from route recipients or personnel platform identity, not from channel credentials."}`),
		SendAPI:              rawJSON(`{"method":"POST","url":"https://www.pushplus.plus/send","content_type":"application/json","live_test_status":"implemented_but_not_live_tested","notes":"No live PushPlus account is configured in this environment."}`),
		SuccessRule:          rawJSON(`{"type":"json_field","status_codes":[200],"field":"code","equals":200}`),
		RetryRule:            rawJSON(`{"status_codes":[408,429,500,502,503,504],"network_errors":true,"retryable_json_codes":[500],"non_retryable_json_codes":[401,403]}`),
		DefaultRateLimit:     rawJSON(`{"qps":2}`),
		DefaultConcurrency:   2,
		DefaultTimeoutMS:     5000,
		DefaultRetryPolicy:   rawJSON(`{"max_attempts":3,"delay_ms":1000,"backoff":"linear"}`),
		RequestExamples:      rawJSON(`{"recipient_identity":{"pushplus_token":"pushplus-token"},"title":"Disk alert","content":"Disk 95%","topic":"ops"}`),
		CustomBodyAllowed:    false,
	})
}

func pushPlusContentSchema() json.RawMessage {
	return rawJSON(`{"type":"object","required":["content"],"properties":{"content":{"type":"string","title":"content","format_hint":"支持 HTML","default":"{{ payload.content }}"},"title":{"type":"string","title":"title","default":"{{ payload.title }}"},"topic":{"type":"string","title":"topic","default":"{{ payload.topic }}"}}}`)
}

func wxPusherCapability() Capability {
	return capability(capabilitySpec{
		ProviderType:         ProviderWxPusher,
		DisplayName:          "WxPusher",
		Category:             "personal_gateway",
		MessageType:          "html",
		MessageSchema:        wxPusherContentSchema(),
		CredentialSchema:     rawJSON(`{"type":"object","required":["app_token"],"properties":{"app_token":{"type":"string","format":"password","title":"WxPusher AppToken"}}}`),
		ChannelConfigSchema:  rawJSON(`{"type":"object","properties":{"topic_ids":{"type":"array","title":"Topic ID 列表","input_type":"textarea","items":{"type":"integer"}}}}`),
		RecipientRequired:    false,
		AllowNoRecipient:     true,
		RecipientRequirement: "system_or_channel",
		RecipientFieldName:   "uids",
		RecipientLocation:    PlacementBody,
		RecipientPath:        "uids",
		RecipientFormat:      "array",
		IdentityKind:         "wxpusher_uid",
		TokenLocation:        PlacementBody,
		TokenFieldName:       "appToken",
		TokenStrategy:        rawJSON(`{"strategy":"static_token","cacheable":false,"placement":{"location":"body","field_name":"appToken"}}`),
		SendAPI:              rawJSON(`{"method":"POST","url":"https://wxpusher.zjiecode.com/api/send/message","content_type":"application/json","live_test_status":"implemented_but_not_live_tested","notes":"Standard POST API only. No live WxPusher account is configured in this environment."}`),
		SuccessRule:          rawJSON(`{"type":"json_field","status_codes":[200],"field":"code","equals":1000}`),
		RetryRule:            rawJSON(`{"status_codes":[408,429,500,502,503,504],"network_errors":true,"non_retryable_json_codes":[1000,1001]}`),
		DefaultRateLimit:     rawJSON(`{"qps":2}`),
		DefaultConcurrency:   2,
		DefaultTimeoutMS:     5000,
		DefaultRetryPolicy:   rawJSON(`{"max_attempts":3,"delay_ms":1000,"backoff":"linear"}`),
		RequestExamples:      rawJSON(`{"appToken":"wxpusher-app-token","content":"<h1>Disk 95%</h1>","summary":"Disk alert","contentType":2,"uids":["UID_xxx"],"topicIds":[101],"url":"https://example.test/detail","verifyPayType":0}`),
		CustomBodyAllowed:    false,
	})
}

func wxPusherContentSchema() json.RawMessage {
	return rawJSON(`{"type":"object","required":["content"],"properties":{"content":{"type":"string","title":"content","format_hint":"支持 HTML","template_expression":"{{ payload.content }}"},"summary":{"type":"string","title":"summary","template_expression":"{{ payload.title }}"},"url":{"type":"string","title":"url","template_expression":"{{ payload.url }}"}}}`)
}

func serverChanCapability() Capability {
	return capability(capabilitySpec{
		ProviderType:         ProviderServerChan,
		DisplayName:          "Server酱 3",
		Category:             "personal_gateway",
		MessageType:          "markdown",
		MessageSchema:        rawJSON(`{"type":"object","required":["title"],"properties":{"title":{"type":"string","title":"title","template_expression":"{{ payload.title }}"},"desp":{"type":"string","title":"desp","format_hint":"支持 Markdown","template_expression":"{{ payload.content }}"},"short":{"type":"string","title":"short","template_expression":"{{ payload.short }}"}}}`),
		CredentialSchema:     rawJSON(`{"type":"object","properties":{}}`),
		ChannelConfigSchema:  rawJSON(`{"type":"object","required":["url"],"properties":{"url":{"type":"string","title":"API URL","format":"uri","default":"https://<uid>.push.ft07.com/send/<sendkey>.send"}}}`),
		RecipientRequired:    true,
		RecipientRequirement: "system",
		RecipientFieldName:   "sendkey",
		RecipientLocation:    PlacementPath,
		RecipientPath:        "sendkey",
		RecipientFormat:      "string",
		IdentityKind:         "serverchan_sendkey",
		TokenLocation:        PlacementNone,
		TokenStrategy:        rawJSON(`{"strategy":"none","cacheable":false,"placement":{"location":"none"}}`),
		SendAPI:              rawJSON(`{"method":"POST","url_pattern":"https://<uid>.push.ft07.com/send/<sendkey>.send","content_type":"application/json","live_test_status":"configuration_dependent","notes":"ServerChan 3 sendKey is resolved from route recipients or personnel platform identity; uid is derived from sendKey."}`),
		SuccessRule:          rawJSON(`{"type":"json_field","status_codes":[200],"field":"code","equals":0}`),
		RetryRule:            rawJSON(`{"status_codes":[408,429,500,502,503,504],"network_errors":true,"non_retryable_json_codes":[40001,40003]}`),
		DefaultRateLimit:     rawJSON(`{"qps":1}`),
		DefaultConcurrency:   1,
		DefaultTimeoutMS:     5000,
		DefaultRetryPolicy:   rawJSON(`{"max_attempts":3,"delay_ms":1500,"backoff":"linear"}`),
		RequestExamples:      rawJSON(`{"recipient_identity":{"serverchan_sendkey":"sctp21329tfauqvvbhe2wpeb5lufz4gz"},"title":"Disk alert","desp":"**Disk 95%**","short":"Disk alert"}`),
		CustomBodyAllowed:    false,
	})
}

func barkCapability() Capability {
	return capability(capabilitySpec{
		ProviderType:         ProviderBark,
		DisplayName:          "Bark",
		Category:             "personal_gateway",
		MessageType:          "notice",
		MessageSchema:        barkContentSchema(),
		CredentialSchema:     rawJSON(`{"type":"object","required":["server_url"],"properties":{"server_url":{"type":"string","title":"服务地址","default":"https://api.day.app"}}}`),
		ChannelConfigSchema:  rawJSON(`{"type":"object","properties":{}}`),
		RecipientRequired:    true,
		RecipientRequirement: "system",
		RecipientFieldName:   "device_key",
		RecipientLocation:    PlacementBody,
		RecipientPath:        "device_key",
		RecipientFormat:      "string",
		IdentityKind:         "bark_device_key",
		TokenLocation:        PlacementBody,
		TokenFieldName:       "device_key",
		TokenStrategy:        rawJSON(`{"strategy":"target_device_key","cacheable":false,"placement":{"location":"body","field_name":"device_key"},"notes":"Device key is resolved from route recipients or personnel platform identity, not from channel credentials."}`),
		SendAPI:              rawJSON(`{"method":"POST","url_template":"{server_url}/push","content_type":"application/json","adapter":"mock_http","live_test_status":"implemented_but_not_live_tested","notes":"No live Bark device key is configured in this environment."}`),
		SuccessRule:          rawJSON(`{"type":"json_field","status_codes":[200],"field":"code","equals":200}`),
		RetryRule:            rawJSON(`{"status_codes":[408,429,500,502,503,504],"network_errors":true,"non_retryable_json_codes":[400,401,403,404]}`),
		DefaultRateLimit:     rawJSON(`{"qps":5}`),
		DefaultConcurrency:   2,
		DefaultTimeoutMS:     5000,
		DefaultRetryPolicy:   rawJSON(`{"max_attempts":3,"delay_ms":1000,"backoff":"linear"}`),
		RequestExamples:      rawJSON(`{"device_key":"bark-device-key","title":"Disk alert","body":"Disk 95%","group":"ops","level":"active"}`),
		CustomBodyAllowed:    false,
	})
}

func pushMeCapability() Capability {
	return capability(capabilitySpec{
		ProviderType:         ProviderPushMe,
		DisplayName:          "PushMe",
		Category:             "personal_gateway",
		MessageType:          "notice",
		MessageSchema:        pushMeContentSchema(),
		CredentialSchema:     rawJSON(`{"type":"object","required":["server_url"],"properties":{"server_url":{"type":"string","title":"服务地址","default":"https://push.i-i.me"}}}`),
		ChannelConfigSchema:  rawJSON(`{"type":"object","properties":{}}`),
		RecipientRequired:    true,
		RecipientRequirement: "system",
		RecipientFieldName:   "push_key",
		RecipientLocation:    PlacementBody,
		RecipientPath:        "push_key",
		RecipientFormat:      "string",
		IdentityKind:         "pushme_push_key",
		TokenLocation:        PlacementBody,
		TokenFieldName:       "push_key",
		TokenStrategy:        rawJSON(`{"strategy":"target_push_key","cacheable":false,"placement":{"location":"body","field_name":"push_key"},"supported_fields":["push_key"],"notes":"PushMe Push Key is resolved from route recipients or personnel platform identity, not from channel credentials."}`),
		SendAPI:              rawJSON(`{"method":"POST","url_template":"{server_url}","content_type":"application/json","adapter":"mock_http","live_test_status":"implemented_but_not_live_tested","notes":"No live PushMe key is configured in this environment."}`),
		SuccessRule:          rawJSON(`{"type":"text_or_json","status_codes":[200],"text_contains":["success"],"json_field":"errcode","json_equals":0}`),
		RetryRule:            rawJSON(`{"status_codes":[408,429,500,502,503,504],"network_errors":true,"non_retryable_status_codes":[400,401,403,404]}`),
		DefaultRateLimit:     rawJSON(`{"qps":2}`),
		DefaultConcurrency:   2,
		DefaultTimeoutMS:     5000,
		DefaultRetryPolicy:   rawJSON(`{"max_attempts":3,"delay_ms":1000,"backoff":"linear"}`),
		RequestExamples:      rawJSON(`{"recipient_identity":{"pushme_push_key":"pushme-key"},"title":"Disk alert","content":"Disk 95%","type":"markdown"}`),
		CustomBodyAllowed:    false,
	})
}

func pushMeContentSchema() json.RawMessage {
	return rawJSON(`{"type":"object","required":["title","content","type"],"properties":{"title":{"type":"string","title":"title","template_expression":"{{ payload.title }}"},"content":{"type":"string","title":"content","template_expression":"{{ payload.content }}"},"type":{"type":"string","title":"type","enum":["text","markdown","html"],"default":"markdown","template_expression":"markdown"}}}`)
}
