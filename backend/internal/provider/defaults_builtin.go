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
		ntfyCapability(),
		gotifyCapability(),
		barkCapability(),
		pushMeCapability(),
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

func noticeContentSchema() json.RawMessage {
	return rawJSON(`{"type":"object","required":["title","body"],"properties":{"title":{"type":"string","title":"Title","default":"{{ payload.title }}"},"body":{"type":"string","title":"Body","default":"{{ payload.content }}"},"format":{"type":"string","enum":["text","markdown","html","json"],"default":"markdown"},"url":{"type":"string"}}}`)
}

func barkContentSchema() json.RawMessage {
	return rawJSON(`{"type":"object","required":["title","body"],"properties":{"title":{"type":"string","title":"Title","default":"{{ payload.title }}"},"subtitle":{"type":"string","title":"Subtitle"},"body":{"type":"string","title":"Body","default":"{{ payload.content }}"},"url":{"type":"string"},"level":{"type":"string","enum":["active","timeSensitive","passive","critical"]}}}`)
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
