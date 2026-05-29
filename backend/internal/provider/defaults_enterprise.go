package provider

func weComRobotCapability() Capability {
	return capability(capabilitySpec{
		ProviderType:         ProviderWeComRobot,
		DisplayName:          "WeCom group robot",
		Category:             "enterprise_robot",
		MessageType:          "text",
		MessageSchema:        weComRobotContentSchema(),
		CredentialSchema:     rawJSON(`{"type":"object","required":["webhook_url"],"properties":{"webhook_url":{"type":"string","default":"https://qyapi.weixin.qq.com/cgi-bin/webhook/send"}}}`),
		ChannelConfigSchema:  rawJSON(`{"type":"object","properties":{}}`),
		RecipientRequired:    true,
		RecipientRequirement: "system",
		RecipientFieldName:   "key",
		RecipientLocation:    PlacementQuery,
		RecipientFormat:      "string",
		IdentityKind:         "wecom_robot_key",
		TokenLocation:        PlacementNone,
		TokenStrategy:        rawJSON(`{"strategy":"none","cacheable":false,"placement":{"location":"none"}}`),
		SendAPI:              rawJSON(`{"method":"POST","url":"https://qyapi.weixin.qq.com/cgi-bin/webhook/send","content_type":"application/json","live_test_status":"implemented_but_not_live_tested","notes":"No live WeCom robot account is configured in this environment."}`),
		SuccessRule:          rawJSON(`{"type":"json_field","status_codes":[200],"field":"errcode","equals":0}`),
		RetryRule:            rawJSON(`{"status_codes":[408,429,500,502,503,504],"network_errors":true,"retryable_json_codes":[-1],"non_retryable_json_codes":[40001,40003,93000]}`),
		DefaultRateLimit:     rawJSON(`{"qps":1}`),
		DefaultConcurrency:   1,
		DefaultTimeoutMS:     5000,
		DefaultRetryPolicy:   rawJSON(`{"max_attempts":3,"delay_ms":1500,"backoff":"linear"}`),
		RequestExamples:      rawJSON(`{"recipient_identity":{"wecom_robot_key":"693axxx6-7aoc-4bc4-97a0-0ec2sifa5aaa"},"msgtype":"markdown","content":"**Disk 95%**"}`),
		CustomBodyAllowed:    false,
	})
}

func weComAppCapability(providerType ProviderType, displayName string) Capability {
	return capability(capabilitySpec{
		ProviderType:         providerType,
		DisplayName:          displayName,
		Category:             "enterprise_app",
		MessageType:          "text",
		MessageSchema:        weComAppContentSchema(),
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
		TokenStrategy:        rawJSON(`{"strategy":"client_credentials","cacheable":true,"cache_key_fields":["corpid","corpsecret"],"token_url":"https://qyapi.weixin.qq.com/cgi-bin/gettoken","request":{"method":"GET","query_fields":["corpid","corpsecret"]},"response_token_path":"access_token","response_expires_in_path":"expires_in","placement":{"location":"query","field_name":"access_token"},"refresh_on_json_codes":[41001,40014,42001]}`),
		SendAPI:              rawJSON(`{"method":"POST","url":"https://qyapi.weixin.qq.com/cgi-bin/message/send","content_type":"application/json","live_test_status":"implemented_but_not_live_tested","notes":"No live WeCom application account is configured in this environment."}`),
		SuccessRule:          rawJSON(`{"type":"json_field","status_codes":[200],"field":"errcode","equals":0}`),
		RetryRule:            rawJSON(`{"status_codes":[408,429,500,502,503,504],"network_errors":true,"refresh_token_codes":[41001,40014,42001],"retryable_json_codes":[-1,45009],"non_retryable_json_codes":[40003,40013,60020]}`),
		DefaultRateLimit:     rawJSON(`{"qps":20}`),
		DefaultConcurrency:   2,
		DefaultTimeoutMS:     5000,
		DefaultRetryPolicy:   rawJSON(`{"max_attempts":3,"delay_ms":1000,"backoff":"linear"}`),
		RequestExamples:      rawJSON(`{"touser":"user1|user2","msgtype":"text","agentid":1000001,"text":{"content":"Disk 95%"}}`),
		CustomBodyAllowed:    false,
	})
}

func dingTalkRobotCapability(messageType string) Capability {
	messageSchema := dingTalkRobotMarkdownContentSchema()
	if messageType == "text" {
		messageSchema = dingTalkRobotTextContentSchema()
	}
	return capability(capabilitySpec{
		ProviderType:         ProviderDingTalkRobot,
		DisplayName:          "DingTalk group robot",
		Category:             "enterprise_robot",
		MessageType:          messageType,
		MessageSchema:        messageSchema,
		CredentialSchema:     rawJSON(`{"type":"object","properties":{"secret":{"type":"string","title":"secret","format":"password","description":"机器人安全设置中加签一栏下 SEC 开头的字符串；为空时不加签。"}}}`),
		ChannelConfigSchema:  rawJSON(`{"type":"object","properties":{"base_url":{"type":"string","title":"API 基础地址","default":"https://oapi.dingtalk.com"},"isAtAll":{"type":"boolean","title":"isAtAll","default":false}}}`),
		RecipientRequired:    true,
		AllowNoRecipient:     false,
		RecipientRequirement: "system_or_channel",
		RecipientFieldName:   "access_token",
		RecipientLocation:    PlacementQuery,
		RecipientPath:        "access_token",
		RecipientFormat:      "string",
		IdentityKind:         "dingtalk_robot_access_token",
		TokenLocation:        PlacementNone,
		TokenStrategy:        rawJSON(`{"strategy":"webhook_access_token","cacheable":false,"placement":{"location":"query","field_name":"access_token"},"signing":{"algorithm":"HMAC-SHA256","fields":["timestamp","sign"],"secret_field":"secret"}}`),
		SendAPI:              rawJSON(`{"method":"POST","url":"https://oapi.dingtalk.com/robot/send?access_token={access_token}","content_type":"application/json","live_test_status":"implemented_but_not_live_tested","notes":"No live DingTalk robot account is configured in this environment."}`),
		SuccessRule:          rawJSON(`{"type":"json_field","status_codes":[200],"field":"errcode","equals":0}`),
		RetryRule:            rawJSON(`{"status_codes":[408,429,500,502,503,504],"network_errors":true,"retryable_json_codes":[88],"non_retryable_json_codes":[310000,300001]}`),
		DefaultRateLimit:     rawJSON(`{"qps":1}`),
		DefaultConcurrency:   1,
		DefaultTimeoutMS:     5000,
		DefaultRetryPolicy:   rawJSON(`{"max_attempts":3,"delay_ms":1500,"backoff":"linear"}`),
		RequestExamples:      rawJSON(`{"msgtype":"markdown","markdown":{"title":"标题","text":"## 标题 \\n - 列表项 \\n [链接](url)"},"at":{"isAtAll":false}}`),
		CustomBodyAllowed:    false,
	})
}

func dingTalkWorkCapability(providerType ProviderType, displayName string, messageType string) Capability {
	messageSchema := dingTalkWorkMarkdownContentSchema()
	if messageType == "sampleText" {
		messageSchema = dingTalkWorkTextContentSchema()
	}
	return capability(capabilitySpec{
		ProviderType:         providerType,
		DisplayName:          displayName,
		Category:             "enterprise_app",
		MessageType:          messageType,
		MessageSchema:        messageSchema,
		CredentialSchema:     rawJSON(`{"type":"object","required":["corp_id","client_id","client_secret"],"properties":{"corp_id":{"type":"string","title":"corpId"},"client_id":{"type":"string","title":"client_id"},"client_secret":{"type":"string","title":"client_secret","format":"password"}}}`),
		ChannelConfigSchema:  rawJSON(`{"type":"object","required":["robot_code"],"properties":{"base_url":{"type":"string","default":"https://api.dingtalk.com"},"robot_code":{"type":"string","title":"robotCode"}}}`),
		RecipientRequired:    true,
		RecipientRequirement: "system",
		RecipientFieldName:   "userIds",
		RecipientLocation:    PlacementBody,
		RecipientPath:        "userIds",
		RecipientFormat:      "array",
		IdentityKind:         "dingtalk_userid",
		TokenLocation:        PlacementHeader,
		TokenFieldName:       "x-acs-dingtalk-access-token",
		TokenStrategy:        rawJSON(`{"strategy":"app_access_token","cacheable":true,"cache_key_fields":["corp_id","client_id","client_secret"],"token_url":"https://api.dingtalk.com/v1.0/oauth2/{corp_id}/token","request":{"method":"POST","body":{"grant_type":"client_credentials"},"body_fields":["client_id","client_secret"]},"response_token_path":"accessToken|access_token","response_expires_in_path":"expireIn|expires_in","placement":{"location":"header","field_name":"x-acs-dingtalk-access-token"},"refresh_on_json_codes":[40001,42001]}`),
		SendAPI:              rawJSON(`{"method":"POST","url":"https://api.dingtalk.com/v1.0/robot/oToMessages/batchSend","content_type":"application/json","live_test_status":"implemented_but_not_live_tested","notes":"No live DingTalk work-message account is configured in this environment."}`),
		SuccessRule:          rawJSON(`{"type":"json_field","status_codes":[200],"field":"errcode","equals":0}`),
		RetryRule:            rawJSON(`{"status_codes":[408,429,500,502,503,504],"network_errors":true,"refresh_token_codes":[40001,42001],"retryable_json_codes":[88],"non_retryable_json_codes":[40035,40036,60020]}`),
		DefaultRateLimit:     rawJSON(`{"qps":20}`),
		DefaultConcurrency:   2,
		DefaultTimeoutMS:     5000,
		DefaultRetryPolicy:   rawJSON(`{"max_attempts":3,"delay_ms":1000,"backoff":"linear"}`),
		RequestExamples:      rawJSON(`{"robotCode":"dingxxxxxx","userIds":["user1","user2"],"msgKey":"sampleMarkdown","msgParam":"{\"title\":\"hello title\",\"text\":\"hello text\"}"}`),
		CustomBodyAllowed:    false,
	})
}

func feishuRobotCapability() Capability {
	return capability(capabilitySpec{
		ProviderType:         ProviderFeishuRobot,
		DisplayName:          "Feishu app robot",
		Category:             "enterprise_app",
		MessageType:          "text",
		MessageSchema:        robotTextContentSchema(),
		CredentialSchema:     rawJSON(`{"type":"object","required":["app_id","app_secret"],"properties":{"app_id":{"type":"string"},"app_secret":{"type":"string","format":"password"}}}`),
		ChannelConfigSchema:  rawJSON(`{"type":"object","properties":{"base_url":{"type":"string","default":"https://open.feishu.cn/open-apis"}}}`),
		RecipientRequired:    true,
		RecipientRequirement: "system",
		RecipientFieldName:   "receive_id",
		RecipientLocation:    PlacementBody,
		RecipientPath:        "receive_id",
		RecipientFormat:      "string",
		IdentityKind:         "feishu_open_id",
		TokenLocation:        PlacementHeader,
		TokenFieldName:       "Authorization",
		TokenStrategy:        rawJSON(`{"strategy":"tenant_access_token","cacheable":true,"token_url":"https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal","request":{"method":"POST","body_fields":["app_id","app_secret"]},"response_token_path":"tenant_access_token","response_expires_in_path":"expire","placement":{"location":"header","field_name":"Authorization","prefix":"Bearer "},"refresh_on_json_codes":[99991663,99991664,99991665]}`),
		SendAPI:              rawJSON(`{"method":"POST","url":"https://open.feishu.cn/open-apis/im/v1/messages","query":{"receive_id_type":"open_id"},"content_type":"application/json","live_test_status":"implemented_but_not_live_tested","notes":"No live Feishu app robot account is configured in this environment."}`),
		SuccessRule:          rawJSON(`{"type":"json_field","status_codes":[200],"field":"code","equals":0}`),
		RetryRule:            rawJSON(`{"status_codes":[408,429,500,502,503,504],"network_errors":true,"refresh_token_codes":[99991663,99991664,99991665],"retryable_json_codes":[99991663,99991664],"non_retryable_json_codes":[19021,19022]}`),
		DefaultRateLimit:     rawJSON(`{"qps":1}`),
		DefaultConcurrency:   1,
		DefaultTimeoutMS:     5000,
		DefaultRetryPolicy:   rawJSON(`{"max_attempts":3,"delay_ms":1500,"backoff":"linear"}`),
		RequestExamples:      rawJSON(`{"receive_id":"ou_xxx","msg_type":"text","content":"{\"text\":\"Disk 95%\"}"}`),
		CustomBodyAllowed:    false,
	})
}

func feishuGroupCapability() Capability {
	return capability(capabilitySpec{
		ProviderType:         ProviderFeishuGroup,
		DisplayName:          "Feishu group message",
		Category:             "enterprise_robot",
		MessageType:          "text",
		MessageSchema:        feishuGroupContentSchema(),
		CredentialSchema:     rawJSON(`{"type":"object","properties":{"sign_secret":{"type":"string","title":"签名密钥","format":"password"}}}`),
		ChannelConfigSchema:  rawJSON(`{"type":"object","properties":{"base_url":{"type":"string","title":"基础 API","default":"https://open.feishu.cn/open-apis"}}}`),
		RecipientRequired:    true,
		RecipientRequirement: "system",
		RecipientFieldName:   "token",
		RecipientLocation:    PlacementPath,
		RecipientPath:        "token",
		RecipientFormat:      "string",
		IdentityKind:         "feishu_webhook_token",
		TokenLocation:        PlacementNone,
		TokenStrategy:        rawJSON(`{"strategy":"webhook_token","cacheable":false,"placement":{"location":"path","field_name":"token"},"signing":{"algorithm":"HMAC-SHA256","fields":["timestamp","sign"],"secret_field":"sign_secret"}}`),
		SendAPI:              rawJSON(`{"method":"POST","url":"https://open.feishu.cn/open-apis/bot/v2/hook/{token}","content_type":"application/json","live_test_status":"implemented_but_not_live_tested","notes":"Feishu group webhook token is resolved from route recipients or personnel platform identity."}`),
		SuccessRule:          rawJSON(`{"type":"json_field","status_codes":[200],"field":"code","equals":0}`),
		RetryRule:            rawJSON(`{"status_codes":[408,429,500,502,503,504],"network_errors":true,"retryable_json_codes":[9499],"non_retryable_json_codes":[19021]}`),
		DefaultRateLimit:     rawJSON(`{"qps":1}`),
		DefaultConcurrency:   1,
		DefaultTimeoutMS:     5000,
		DefaultRetryPolicy:   rawJSON(`{"max_attempts":3,"delay_ms":1500,"backoff":"linear"}`),
		RequestExamples:      rawJSON(`{"msg_type":"text","content":{"text":"request example"}}`),
		CustomBodyAllowed:    false,
	})
}
