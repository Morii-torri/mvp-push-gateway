package provider

import "encoding/json"

func builtInCapabilities() []Capability {
	return []Capability{
		webhookCapability(),
		selfCapability("json"),
		selfCapability("text"),
		pushPlusCapability(),
		wxPusherCapability(),
		serverChanCapability(),
		emailCapability("email"),
		emailCapability("text"),
		smsLegacyCapability(),
		smsVendorCapability(ProviderAliyunSMS, "Aliyun SMS", "aliyun", rawJSON(`{"method":"POST","url":"https://dysmsapi.aliyuncs.com/","content_type":"application/json","adapter":"mock_http","live_test_status":"implemented_but_not_live_tested","notes":"No SDK or live SMS account is used in this build-request adapter."}`), rawJSON(`{"vendor":"aliyun","phone_numbers":["13800138000"],"sign_name":"Ops","template_code":"SMS_001","template_param":{"code":"1234"}}`)),
		smsVendorCapability(ProviderTencentSMS, "Tencent Cloud SMS", "tencent", rawJSON(`{"method":"POST","url":"https://sms.tencentcloudapi.com/","content_type":"application/json","adapter":"mock_http","live_test_status":"implemented_but_not_live_tested","notes":"No SDK or live SMS account is used in this build-request adapter."}`), rawJSON(`{"vendor":"tencent","PhoneNumberSet":["13800138000"],"SmsSdkAppId":"1400001","SignName":"Ops","TemplateId":"1001","TemplateParamSet":["1234"]}`)),
		smsVendorCapability(ProviderBaiduSMS, "Baidu Cloud SMS", "baidu", rawJSON(`{"method":"POST","url":"https://sms.bj.baidubce.com/bce/v2/message","content_type":"application/json","adapter":"mock_http","live_test_status":"implemented_but_not_live_tested","notes":"No SDK or live SMS account is used in this build-request adapter."}`), rawJSON(`{"vendor":"baidu","mobile":["13800138000"],"signature_id":"sig","template":"tpl","content_var":{"code":"1234"}}`)),
		weComRobotCapability(),
		weComAppCapability(ProviderWeComApp, "WeCom application message"),
		weComAppCapability(ProviderWeCom, "WeCom application message (legacy alias)"),
		dingTalkRobotCapability(),
		dingTalkWorkCapability(ProviderDingTalkWork, "DingTalk work message"),
		dingTalkWorkCapability(ProviderDingTalk, "DingTalk work message (legacy alias)"),
		feishuRobotCapability(),
		feishuLegacyCapability(),
		govCloudCapability(),
		customTokenCapability(),
	}
}

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
		DefaultRateLimit:     rawJSON(`{"qps":10,"burst":20}`),
		DefaultConcurrency:   5,
		DefaultTimeoutMS:     5000,
		DefaultRetryPolicy:   rawJSON(`{"max_attempts":3,"delay_ms":1000,"backoff":"linear"}`),
		RequestExamples:      rawJSON(`{"payload":{"title":"Disk alert","body":"Disk 95%"}}`),
		CustomBodyAllowed:    true,
	})
}

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

func pushPlusCapability() Capability {
	return capability(capabilitySpec{
		ProviderType:         ProviderPushPlus,
		DisplayName:          "PushPlus",
		Category:             "personal_gateway",
		MessageType:          "notice",
		MessageSchema:        noticeContentSchema(),
		CredentialSchema:     rawJSON(`{"type":"object","required":["token"],"properties":{"token":{"type":"string","format":"password"}}}`),
		ChannelConfigSchema:  rawJSON(`{"type":"object","properties":{"topic":{"type":"string"},"channel":{"type":"string"},"template":{"type":"string","enum":["txt","markdown","html","json"],"default":"markdown"}}}`),
		RecipientRequired:    false,
		AllowNoRecipient:     true,
		RecipientRequirement: "none",
		RecipientLocation:    PlacementNone,
		RecipientFormat:      "none",
		TokenLocation:        PlacementBody,
		TokenFieldName:       "token",
		TokenStrategy:        rawJSON(`{"strategy":"static_token","cacheable":false,"placement":{"location":"body","field_name":"token"}}`),
		SendAPI:              rawJSON(`{"method":"POST","url":"https://www.pushplus.plus/send","content_type":"application/json","live_test_status":"implemented_but_not_live_tested","notes":"No live PushPlus account is configured in this environment."}`),
		SuccessRule:          rawJSON(`{"type":"json_field","status_codes":[200],"field":"code","equals":200}`),
		RetryRule:            rawJSON(`{"status_codes":[408,429,500,502,503,504],"network_errors":true,"retryable_json_codes":[500],"non_retryable_json_codes":[401,403]}`),
		DefaultRateLimit:     rawJSON(`{"qps":2,"burst":5}`),
		DefaultConcurrency:   2,
		DefaultTimeoutMS:     5000,
		DefaultRetryPolicy:   rawJSON(`{"max_attempts":3,"delay_ms":1000,"backoff":"linear"}`),
		RequestExamples:      rawJSON(`{"token":"pushplus-token","title":"Disk alert","content":"Disk 95%","template":"markdown","topic":"ops"}`),
		CustomBodyAllowed:    false,
	})
}

func wxPusherCapability() Capability {
	return capability(capabilitySpec{
		ProviderType:         ProviderWxPusher,
		DisplayName:          "WxPusher",
		Category:             "personal_gateway",
		MessageType:          "notice",
		MessageSchema:        noticeContentSchema(),
		CredentialSchema:     rawJSON(`{"type":"object","properties":{"app_token":{"type":"string","format":"password"},"spt":{"type":"string","format":"password"}}}`),
		ChannelConfigSchema:  rawJSON(`{"type":"object","properties":{"mode":{"type":"string","enum":["standard","simple"],"default":"standard"},"topic_ids":{"type":"array","items":{"type":"integer"}},"content_type":{"type":"integer","enum":[1,2,3],"default":3}}}`),
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
		SendAPI:              rawJSON(`{"method":"POST","url":"https://wxpusher.zjiecode.com/api/send/message","simple_url":"https://wxpusher.zjiecode.com/api/send/message/simple-push","content_type":"application/json","live_test_status":"implemented_but_not_live_tested","notes":"No live WxPusher account is configured in this environment."}`),
		SuccessRule:          rawJSON(`{"type":"json_field","status_codes":[200],"field":"success","equals":true}`),
		RetryRule:            rawJSON(`{"status_codes":[408,429,500,502,503,504],"network_errors":true,"non_retryable_json_codes":[1000,1001]}`),
		DefaultRateLimit:     rawJSON(`{"qps":2,"burst":5}`),
		DefaultConcurrency:   2,
		DefaultTimeoutMS:     5000,
		DefaultRetryPolicy:   rawJSON(`{"max_attempts":3,"delay_ms":1000,"backoff":"linear"}`),
		RequestExamples:      rawJSON(`{"appToken":"wxpusher-app-token","content":"Disk 95%","summary":"Disk alert","contentType":3,"uids":["UID_xxx"],"topicIds":[101]}`),
		CustomBodyAllowed:    false,
	})
}

func serverChanCapability() Capability {
	return capability(capabilitySpec{
		ProviderType:         ProviderServerChan,
		DisplayName:          "ServerChan",
		Category:             "personal_gateway",
		MessageType:          "notice",
		MessageSchema:        noticeContentSchema(),
		CredentialSchema:     rawJSON(`{"type":"object","required":["send_key"],"properties":{"version":{"type":"string","enum":["turbo","v3"],"default":"turbo"},"send_key":{"type":"string","format":"password"},"uid":{"type":"string"}}}`),
		ChannelConfigSchema:  rawJSON(`{"type":"object","properties":{"channel":{"type":"string"},"openid":{"type":"string"},"noip":{"type":"boolean"},"tags":{"type":"string"},"short":{"type":"string"}}}`),
		RecipientRequired:    false,
		AllowNoRecipient:     true,
		RecipientRequirement: "none",
		RecipientLocation:    PlacementNone,
		RecipientFormat:      "none",
		TokenLocation:        PlacementPath,
		TokenFieldName:       "send_key",
		TokenStrategy:        rawJSON(`{"strategy":"static_path_key","cacheable":false,"placement":{"location":"path","field_name":"send_key"}}`),
		SendAPI:              rawJSON(`{"method":"POST","turbo_url":"https://sctapi.ftqq.com/{send_key}.send","v3_url":"https://{uid}.push.ft07.com/send/{send_key}.send","content_type":"application/x-www-form-urlencoded","live_test_status":"implemented_but_not_live_tested","notes":"The request snapshot is HTTP-like JSON for form fields; no live ServerChan account is configured."}`),
		SuccessRule:          rawJSON(`{"type":"json_field","status_codes":[200],"field":"code","equals":0}`),
		RetryRule:            rawJSON(`{"status_codes":[408,429,500,502,503,504],"network_errors":true,"non_retryable_json_codes":[40001,40003]}`),
		DefaultRateLimit:     rawJSON(`{"qps":1,"burst":3}`),
		DefaultConcurrency:   1,
		DefaultTimeoutMS:     5000,
		DefaultRetryPolicy:   rawJSON(`{"max_attempts":3,"delay_ms":1500,"backoff":"linear"}`),
		RequestExamples:      rawJSON(`{"title":"Disk alert","desp":"Disk 95%","channel":"9"}`),
		CustomBodyAllowed:    false,
	})
}

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
		DefaultRateLimit:     rawJSON(`{"qps":5,"burst":10}`),
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
		DefaultRateLimit:     rawJSON(`{"qps":10,"burst":20}`),
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
		DefaultRateLimit:     rawJSON(`{"qps":5,"burst":10}`),
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

func weComRobotCapability() Capability {
	return capability(capabilitySpec{
		ProviderType:         ProviderWeComRobot,
		DisplayName:          "WeCom group robot",
		Category:             "enterprise_robot",
		MessageType:          "text",
		MessageSchema:        robotTextContentSchema(),
		CredentialSchema:     rawJSON(`{"type":"object","properties":{"webhook_url":{"type":"string"},"key":{"type":"string","format":"password"}}}`),
		ChannelConfigSchema:  rawJSON(`{"type":"object","properties":{"base_url":{"type":"string","default":"https://qyapi.weixin.qq.com"},"allow_at_all":{"type":"boolean","default":false}}}`),
		RecipientRequired:    false,
		AllowNoRecipient:     true,
		RecipientRequirement: "system_or_channel",
		RecipientFieldName:   "mentioned_list",
		RecipientLocation:    PlacementBody,
		RecipientPath:        "text.mentioned_list",
		RecipientFormat:      "array",
		IdentityKind:         "wecom_userid",
		TokenLocation:        PlacementNone,
		TokenStrategy:        rawJSON(`{"strategy":"webhook_key","cacheable":false,"placement":{"location":"query","field_name":"key"}}`),
		SendAPI:              rawJSON(`{"method":"POST","url":"https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key={key}","content_type":"application/json","live_test_status":"implemented_but_not_live_tested","notes":"No live WeCom robot account is configured in this environment."}`),
		SuccessRule:          rawJSON(`{"type":"json_field","status_codes":[200],"field":"errcode","equals":0}`),
		RetryRule:            rawJSON(`{"status_codes":[408,429,500,502,503,504],"network_errors":true,"retryable_json_codes":[-1],"non_retryable_json_codes":[40001,40003,93000]}`),
		DefaultRateLimit:     rawJSON(`{"qps":1,"burst":3}`),
		DefaultConcurrency:   1,
		DefaultTimeoutMS:     5000,
		DefaultRetryPolicy:   rawJSON(`{"max_attempts":3,"delay_ms":1500,"backoff":"linear"}`),
		RequestExamples:      rawJSON(`{"msgtype":"text","text":{"content":"Disk 95%","mentioned_list":["zhangsan"]}}`),
		CustomBodyAllowed:    false,
	})
}

func weComAppCapability(providerType ProviderType, displayName string) Capability {
	return capability(capabilitySpec{
		ProviderType:         providerType,
		DisplayName:          displayName,
		Category:             "enterprise_app",
		MessageType:          "text",
		MessageSchema:        robotTextContentSchema(),
		CredentialSchema:     rawJSON(`{"type":"object","required":["corpid","corpsecret","agentid"],"properties":{"corpid":{"type":"string"},"corpsecret":{"type":"string","format":"password"},"agentid":{"type":["string","integer"]},"allow_at_all":{"type":"boolean","default":false}}}`),
		ChannelConfigSchema:  rawJSON(`{"type":"object","properties":{"base_url":{"type":"string","default":"https://qyapi.weixin.qq.com"},"safe":{"type":"integer","default":0},"enable_id_trans":{"type":"integer","default":0},"enable_duplicate_check":{"type":"integer","default":0},"duplicate_check_interval":{"type":"integer","default":1800}}}`),
		RecipientRequired:    true,
		RecipientRequirement: "system",
		RecipientFieldName:   "touser",
		RecipientLocation:    PlacementBody,
		RecipientPath:        "touser",
		RecipientFormat:      "pipe_string",
		IdentityKind:         "wecom_userid",
		TokenLocation:        PlacementQuery,
		TokenFieldName:       "access_token",
		TokenStrategy:        rawJSON(`{"strategy":"client_credentials","cacheable":true,"cache_key_fields":["corpid","corpsecret"],"token_url":"https://qyapi.weixin.qq.com/cgi-bin/gettoken","request":{"method":"GET","query_fields":["corpid","corpsecret"]},"response_token_path":"access_token","response_expires_in_path":"expires_in","placement":{"location":"query","field_name":"access_token"},"refresh_on_json_codes":[40014,42001]}`),
		SendAPI:              rawJSON(`{"method":"POST","url":"https://qyapi.weixin.qq.com/cgi-bin/message/send","content_type":"application/json","live_test_status":"implemented_but_not_live_tested","notes":"No live WeCom application account is configured in this environment."}`),
		SuccessRule:          rawJSON(`{"type":"json_field","status_codes":[200],"field":"errcode","equals":0}`),
		RetryRule:            rawJSON(`{"status_codes":[408,429,500,502,503,504],"network_errors":true,"refresh_token_codes":[40014,42001],"retryable_json_codes":[-1,45009],"non_retryable_json_codes":[40003,40013,60020]}`),
		DefaultRateLimit:     rawJSON(`{"qps":20,"burst":40}`),
		DefaultConcurrency:   2,
		DefaultTimeoutMS:     5000,
		DefaultRetryPolicy:   rawJSON(`{"max_attempts":3,"delay_ms":1000,"backoff":"linear"}`),
		RequestExamples:      rawJSON(`{"touser":"user1|user2","msgtype":"text","agentid":1000001,"text":{"content":"Disk 95%"}}`),
		CustomBodyAllowed:    false,
	})
}

func dingTalkRobotCapability() Capability {
	return capability(capabilitySpec{
		ProviderType:         ProviderDingTalkRobot,
		DisplayName:          "DingTalk group robot",
		Category:             "enterprise_robot",
		MessageType:          "text",
		MessageSchema:        robotTextContentSchema(),
		CredentialSchema:     rawJSON(`{"type":"object","required":["webhook_url"],"properties":{"webhook_url":{"type":"string"},"secret":{"type":"string","format":"password"}}}`),
		ChannelConfigSchema:  rawJSON(`{"type":"object","properties":{"allow_at_all":{"type":"boolean","default":false},"keyword_note":{"type":"string"}}}`),
		RecipientRequired:    false,
		AllowNoRecipient:     true,
		RecipientRequirement: "system_or_channel",
		RecipientFieldName:   "atMobiles",
		RecipientLocation:    PlacementBody,
		RecipientPath:        "at.atMobiles",
		RecipientFormat:      "array",
		IdentityKind:         "mobile",
		TokenLocation:        PlacementNone,
		TokenStrategy:        rawJSON(`{"strategy":"webhook_access_token","cacheable":false,"placement":{"location":"query","field_name":"access_token"},"signing":{"algorithm":"HMAC-SHA256","fields":["timestamp","sign"],"secret_field":"secret"}}`),
		SendAPI:              rawJSON(`{"method":"POST","url":"https://oapi.dingtalk.com/robot/send?access_token={access_token}","content_type":"application/json","live_test_status":"implemented_but_not_live_tested","notes":"No live DingTalk robot account is configured in this environment."}`),
		SuccessRule:          rawJSON(`{"type":"json_field","status_codes":[200],"field":"errcode","equals":0}`),
		RetryRule:            rawJSON(`{"status_codes":[408,429,500,502,503,504],"network_errors":true,"retryable_json_codes":[88],"non_retryable_json_codes":[310000,300001]}`),
		DefaultRateLimit:     rawJSON(`{"qps":1,"burst":3}`),
		DefaultConcurrency:   1,
		DefaultTimeoutMS:     5000,
		DefaultRetryPolicy:   rawJSON(`{"max_attempts":3,"delay_ms":1500,"backoff":"linear"}`),
		RequestExamples:      rawJSON(`{"msgtype":"text","text":{"content":"Disk 95%"},"at":{"atMobiles":["13800138000"],"isAtAll":false}}`),
		CustomBodyAllowed:    false,
	})
}

func dingTalkWorkCapability(providerType ProviderType, displayName string) Capability {
	return capability(capabilitySpec{
		ProviderType:         providerType,
		DisplayName:          displayName,
		Category:             "enterprise_app",
		MessageType:          "text",
		MessageSchema:        robotTextContentSchema(),
		CredentialSchema:     rawJSON(`{"type":"object","required":["app_key","app_secret","agent_id"],"properties":{"app_key":{"type":"string"},"app_secret":{"type":"string","format":"password"},"agent_id":{"type":["string","integer"]}}}`),
		ChannelConfigSchema:  rawJSON(`{"type":"object","properties":{"base_url":{"type":"string","default":"https://oapi.dingtalk.com"},"to_all_user":{"type":"boolean","default":false}}}`),
		RecipientRequired:    true,
		RecipientRequirement: "system",
		RecipientFieldName:   "userid_list",
		RecipientLocation:    PlacementBody,
		RecipientPath:        "userid_list",
		RecipientFormat:      "comma_string",
		IdentityKind:         "dingtalk_userid",
		TokenLocation:        PlacementQuery,
		TokenFieldName:       "access_token",
		TokenStrategy:        rawJSON(`{"strategy":"app_access_token","cacheable":true,"cache_key_fields":["app_key","app_secret"],"token_url":"https://oapi.dingtalk.com/gettoken","request":{"method":"GET","query_fields":["appkey","appsecret"]},"response_token_path":"access_token","response_expires_in_path":"expires_in","placement":{"location":"query","field_name":"access_token"},"refresh_on_json_codes":[40001,42001]}`),
		SendAPI:              rawJSON(`{"method":"POST","url":"https://oapi.dingtalk.com/topapi/message/corpconversation/asyncsend_v2","content_type":"application/json","live_test_status":"implemented_but_not_live_tested","notes":"No live DingTalk work-message account is configured in this environment."}`),
		SuccessRule:          rawJSON(`{"type":"json_field","status_codes":[200],"field":"errcode","equals":0}`),
		RetryRule:            rawJSON(`{"status_codes":[408,429,500,502,503,504],"network_errors":true,"refresh_token_codes":[40001,42001],"retryable_json_codes":[88],"non_retryable_json_codes":[40035,40036,60020]}`),
		DefaultRateLimit:     rawJSON(`{"qps":20,"burst":40}`),
		DefaultConcurrency:   2,
		DefaultTimeoutMS:     5000,
		DefaultRetryPolicy:   rawJSON(`{"max_attempts":3,"delay_ms":1000,"backoff":"linear"}`),
		RequestExamples:      rawJSON(`{"agent_id":123,"userid_list":"user1,user2","msg":{"msgtype":"text","text":{"content":"Disk 95%"}}}`),
		CustomBodyAllowed:    false,
	})
}

func feishuRobotCapability() Capability {
	return capability(capabilitySpec{
		ProviderType:         ProviderFeishuRobot,
		DisplayName:          "Feishu group robot",
		Category:             "enterprise_robot",
		MessageType:          "text",
		MessageSchema:        robotTextContentSchema(),
		CredentialSchema:     rawJSON(`{"type":"object","required":["webhook_url"],"properties":{"webhook_url":{"type":"string"},"secret":{"type":"string","format":"password"}}}`),
		ChannelConfigSchema:  rawJSON(`{"type":"object","properties":{"enable_signature":{"type":"boolean","default":false}}}`),
		RecipientRequired:    false,
		AllowNoRecipient:     true,
		RecipientRequirement: "system_or_channel",
		RecipientFieldName:   "mentions",
		RecipientLocation:    PlacementBody,
		RecipientPath:        "content.mentions",
		RecipientFormat:      "array",
		IdentityKind:         "feishu_open_id",
		TokenLocation:        PlacementNone,
		TokenStrategy:        rawJSON(`{"strategy":"webhook_token","cacheable":false,"placement":{"location":"path","field_name":"token"},"signing":{"algorithm":"HMAC-SHA256","fields":["timestamp","sign"],"secret_field":"secret"}}`),
		SendAPI:              rawJSON(`{"method":"POST","url":"https://open.feishu.cn/open-apis/bot/v2/hook/{token}","content_type":"application/json","live_test_status":"implemented_but_not_live_tested","notes":"No live Feishu robot account is configured in this environment."}`),
		SuccessRule:          rawJSON(`{"type":"json_field","status_codes":[200],"field":"code","equals":0}`),
		RetryRule:            rawJSON(`{"status_codes":[408,429,500,502,503,504],"network_errors":true,"retryable_json_codes":[99991663,99991664],"non_retryable_json_codes":[19021,19022]}`),
		DefaultRateLimit:     rawJSON(`{"qps":1,"burst":3}`),
		DefaultConcurrency:   1,
		DefaultTimeoutMS:     5000,
		DefaultRetryPolicy:   rawJSON(`{"max_attempts":3,"delay_ms":1500,"backoff":"linear"}`),
		RequestExamples:      rawJSON(`{"msg_type":"text","content":{"text":"Disk 95%"}}`),
		CustomBodyAllowed:    false,
	})
}

func feishuLegacyCapability() Capability {
	return capability(capabilitySpec{
		ProviderType:         ProviderFeishu,
		DisplayName:          "Feishu application message (legacy)",
		Category:             "enterprise_app",
		MessageType:          "text",
		MessageSchema:        robotTextContentSchema(),
		CredentialSchema:     rawJSON(`{"type":"object","required":["app_id","app_secret"],"properties":{"app_id":{"type":"string"},"app_secret":{"type":"string","format":"password"}}}`),
		ChannelConfigSchema:  rawJSON(`{"type":"object","properties":{"base_url":{"type":"string","default":"https://open.feishu.cn"},"receive_id_type":{"type":"string","default":"open_id"}}}`),
		RecipientRequired:    true,
		RecipientRequirement: "system",
		RecipientFieldName:   "receive_id",
		RecipientLocation:    PlacementBody,
		RecipientPath:        "receive_id",
		RecipientFormat:      "string",
		IdentityKind:         "feishu_open_id",
		TokenLocation:        PlacementHeader,
		TokenFieldName:       "Authorization",
		TokenStrategy:        rawJSON(`{"strategy":"tenant_access_token","cacheable":true,"placement":{"location":"header","field_name":"Authorization","prefix":"Bearer "}}`),
		SendAPI:              rawJSON(`{"method":"POST","path":"/open-apis/im/v1/messages","content_type":"application/json","live_test_status":"implemented_but_not_live_tested","notes":"Legacy feishu capability is retained; prefer feishu_robot for robot webhooks."}`),
		SuccessRule:          rawJSON(`{"type":"json_field","status_codes":[200],"field":"code","equals":0}`),
		RetryRule:            rawJSON(`{"status_codes":[408,429,500,502,503,504],"json_codes":[99991663,99991664]}`),
		DefaultRateLimit:     rawJSON(`{"qps":20,"burst":40}`),
		DefaultConcurrency:   2,
		DefaultTimeoutMS:     5000,
		DefaultRetryPolicy:   rawJSON(`{"max_attempts":3,"delay_ms":1000,"backoff":"linear"}`),
		RequestExamples:      rawJSON(`{"receive_id":"ou_xxx","msg_type":"text","content":"{\"text\":\"Disk 95%\"}"}`),
		CustomBodyAllowed:    false,
	})
}

func govCloudCapability() Capability {
	return capability(capabilitySpec{
		ProviderType:         ProviderGovCloud,
		DisplayName:          "Government cloud message",
		Category:             "government",
		MessageType:          "text",
		MessageSchema:        govCloudTextContentSchema(),
		CredentialSchema:     rawJSON(`{"type":"object","required":["corpsecret"],"properties":{"base_url":{"type":"string","default":"https://www.ywxt.sh.cegn.cn/api-gateway/uranus/uranus/cgi-bin/"},"corpsecret":{"type":"string","format":"password"},"allow_at_all":{"type":"boolean","default":false}}}`),
		ChannelConfigSchema:  rawJSON(`{"type":"object","properties":{"base_url":{"type":"string","default":"https://www.ywxt.sh.cegn.cn/api-gateway/uranus/uranus/cgi-bin/"},"allow_at_all":{"type":"boolean","default":false},"toparty":{"type":"string"},"totag":{"type":"string"}}}`),
		RecipientRequired:    true,
		RecipientRequirement: "system",
		RecipientFieldName:   "touser",
		RecipientLocation:    PlacementBody,
		RecipientPath:        "touser",
		RecipientFormat:      "pipe_string",
		IdentityKind:         "gov_userid",
		TokenLocation:        PlacementQuery,
		TokenFieldName:       "access_token",
		TokenStrategy:        rawJSON(`{"strategy":"corpsecret_gettoken","cacheable":true,"cache_key_fields":["corpsecret"],"token_url":"https://www.ywxt.sh.cegn.cn/api-gateway/uranus/uranus/cgi-bin/gettoken","request":{"method":"GET","query_secret_field":"corpsecret"},"response_success_rule":{"field":"errcode","equals":0},"response_token_path":"access_token","response_expires_in_path":"expires_in","expires_in_seconds":3600,"placement":{"location":"query","field_name":"access_token"},"refresh_on_json_codes":[401,40014,42001],"live_test_status":"implemented_but_not_live_tested","notes":"Current development environment cannot access the government cloud network; implement only, do not live-test."}`),
		SendAPI:              rawJSON(`{"method":"POST","url":"https://www.ywxt.sh.cegn.cn/api-gateway/uranus/uranus/cgi-bin/request/message/send","content_type":"application/json","live_test_status":"implemented_but_not_live_tested","notes":"Current development environment cannot access this government cloud endpoint; build request only."}`),
		SuccessRule:          rawJSON(`{"type":"json_field","status_codes":[200],"field":"errcode","equals":0}`),
		RetryRule:            rawJSON(`{"status_codes":[408,429,500,502,503,504],"network_errors":true,"refresh_token_codes":[401,40014,42001],"retryable_json_codes":[-1,500,10701,10702,10911,523,60047,4400044],"non_retryable_json_codes":[40001,40003,40031,40032,40091,41004,48002,50003,82001,82002,82003,301002,10001004,500011]}`),
		DefaultRateLimit:     rawJSON(`{"qps":5,"burst":10}`),
		DefaultConcurrency:   2,
		DefaultTimeoutMS:     8000,
		DefaultRetryPolicy:   rawJSON(`{"max_attempts":3,"delay_ms":1500,"backoff":"linear","refresh_token_once":true}`),
		RequestExamples:      rawJSON(`{"touser":"UserID1|UserID2","toparty":"","totag":"","msgtype":"text","description":"Disk 95%"}`),
		CustomBodyAllowed:    false,
	})
}

func customTokenCapability() Capability {
	return capability(capabilitySpec{
		ProviderType:         ProviderCustomToken,
		DisplayName:          "Custom token platform",
		Category:             "advanced",
		MessageType:          "json",
		MessageSchema:        customTokenContentSchema(),
		CredentialSchema:     rawJSON(`{"type":"object","required":["token_url","client_id","client_secret"],"properties":{"token_url":{"type":"string"},"client_id":{"type":"string"},"client_secret":{"type":"string","format":"password"}}}`),
		ChannelConfigSchema:  rawJSON(`{"type":"object","required":["send_url"],"properties":{"send_url":{"type":"string"},"method":{"type":"string","default":"POST"},"token_json_path":{"type":"string","default":"access_token"}}}`),
		RecipientRequired:    true,
		AllowNoRecipient:     true,
		RecipientRequirement: "payload",
		RecipientFieldName:   "recipient",
		RecipientLocation:    PlacementBody,
		RecipientPath:        "recipient",
		RecipientFormat:      "string",
		IdentityKind:         "custom",
		TokenLocation:        PlacementHeader,
		TokenFieldName:       "Authorization",
		TokenStrategy:        rawJSON(`{"strategy":"custom_token_api","cacheable":true,"placement":{"location":"header","field_name":"Authorization","prefix":"Bearer "}}`),
		SendAPI:              rawJSON(`{"method":"POST","url_template":"{{ channel.send_url }}","content_type":"application/json","live_test_status":"configuration_dependent","notes":"Custom token provider remains configurable and may be live-tested only with user supplied endpoints."}`),
		SuccessRule:          rawJSON(`{"type":"configurable","default_status_codes":[200,201,202]}`),
		RetryRule:            rawJSON(`{"status_codes":[408,429,500,502,503,504],"network_errors":true}`),
		DefaultRateLimit:     rawJSON(`{"qps":10,"burst":20}`),
		DefaultConcurrency:   5,
		DefaultTimeoutMS:     5000,
		DefaultRetryPolicy:   rawJSON(`{"max_attempts":3,"delay_ms":1000,"backoff":"linear"}`),
		RequestExamples:      rawJSON(`{"message":"Disk 95%"}`),
		CustomBodyAllowed:    true,
	})
}

func noticeContentSchema() json.RawMessage {
	return rawJSON(`{"type":"object","required":["title","body"],"properties":{"title":{"type":"string","title":"Title","default":"{{ payload.title }}"},"body":{"type":"string","title":"Body","default":"{{ payload.content }}"},"format":{"type":"string","enum":["text","markdown","html","json"],"default":"markdown"},"url":{"type":"string"}}}`)
}

func cascadeContentSchema() json.RawMessage {
	return rawJSON(`{"type":"object","properties":{"title":{"type":"string","default":"{{ payload.title }}"},"body":{"type":"string","default":"{{ payload.content }}"},"payload":{"type":"object","additionalProperties":true}},"additionalProperties":true}`)
}

func robotTextContentSchema() json.RawMessage {
	return rawJSON(`{"type":"object","required":["content"],"properties":{"title":{"type":"string","default":"{{ payload.title }}"},"body":{"type":"string","default":"{{ payload.content }}"},"content":{"type":"string","default":"{{ payload.content }}"},"markdown":{"type":"string"}}}`)
}

func smsTemplateContentSchema() json.RawMessage {
	return rawJSON(`{"type":"object","required":["template_params"],"properties":{"template_params":{"type":["object","array"],"title":"Template parameters"},"out_id":{"type":"string"},"sign_name":{"type":"string"},"template_id":{"type":"string"}}}`)
}

func govCloudTextContentSchema() json.RawMessage {
	return rawJSON(`{"type":"object","properties":{"title":{"type":"string"},"description":{"type":"string","default":"{{ payload.content }}"},"body":{"type":"string"},"content":{"type":"string"}}}`)
}
