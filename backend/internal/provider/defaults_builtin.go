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
		smsVendorCapability(ProviderAliyunSMS, "Aliyun SMS", "aliyun", rawJSON(`{"method":"POST","url":"https://dysmsapi.aliyuncs.com/","content_type":"application/json","adapter":"mock_http","live_test_status":"implemented_but_not_live_tested","notes":"No SDK or live SMS account is used in this build-request adapter."}`), rawJSON(`{"vendor":"aliyun","phone_numbers":["13800138000"],"sign_name":"Ops","template_code":"SMS_001","template_param":{"code":"1234"}}`)),
		smsVendorCapability(ProviderTencentSMS, "Tencent Cloud SMS", "tencent", rawJSON(`{"method":"POST","url":"https://sms.tencentcloudapi.com/","content_type":"application/json","adapter":"mock_http","live_test_status":"implemented_but_not_live_tested","notes":"No SDK or live SMS account is used in this build-request adapter."}`), rawJSON(`{"vendor":"tencent","PhoneNumberSet":["13800138000"],"SmsSdkAppId":"1400001","SignName":"Ops","TemplateId":"1001","TemplateParamSet":["1234"]}`)),
		smsVendorCapability(ProviderBaiduSMS, "Baidu Cloud SMS", "baidu", rawJSON(`{"method":"POST","url":"https://sms.bj.baidubce.com/bce/v2/message","content_type":"application/json","adapter":"mock_http","live_test_status":"implemented_but_not_live_tested","notes":"No SDK or live SMS account is used in this build-request adapter."}`), rawJSON(`{"vendor":"baidu","mobile":["13800138000"],"signature_id":"sig","template":"tpl","content_var":{"code":"1234"}}`)),
		weComRobotCapability(),
		weComAppCapability(ProviderWeComApp, "WeCom application message"),
		dingTalkRobotCapability("text"),
		dingTalkRobotCapability("markdown"),
		dingTalkWorkCapability(ProviderDingTalkWork, "DingTalk work message", "sampleMarkdown"),
		dingTalkWorkCapability(ProviderDingTalkWork, "DingTalk work message", "sampleText"),
		feishuRobotCapability(),
		feishuGroupCapability(),
	}
}

func noticeContentSchema() json.RawMessage {
	return rawJSON(`{"type":"object","required":["title","body"],"properties":{"title":{"type":"string","title":"Title","default":"{{ payload.title }}"},"body":{"type":"string","title":"Body","default":"{{ payload.content }}"},"format":{"type":"string","enum":["text","markdown","html","json"],"default":"markdown"},"url":{"type":"string"}}}`)
}

func barkContentSchema() json.RawMessage {
	return rawJSON(`{"type":"object","field_order":["title","subtitle","body","markdown","group","sound","level","icon","url","image"],"properties":{"title":{"type":"string","title":"title","default":"{{ payload.title }}"},"subtitle":{"type":"string","title":"subtitle","default":"{{ payload.subtitle }}"},"body":{"type":"string","title":"body","default":"{{ payload.content }}"},"markdown":{"type":"string","title":"markdown","default":"{{ payload.markdown }}","format_hint":"支持 Markdown"},"group":{"type":"string","title":"group","default":"{{ payload.group }}"},"sound":{"type":"string","title":"sound","default":"{{ payload.sound }}"},"level":{"type":"string","title":"level","enum":["critical","active","timeSensitive","passive"],"enum_descriptions":{"critical":"重要警告，在静音模式下也会响铃","active":"默认值，系统会立即亮屏显示通知","timeSensitive":"时效性通知，可在专注状态下显示通知","passive":"仅将通知添加到通知列表，不会亮屏提醒"}},"icon":{"type":"string","title":"icon","default":"{{ payload.icon }}"},"url":{"type":"string","title":"url","default":"{{ payload.url }}"},"image":{"type":"string","title":"image","default":"{{ payload.image }}"}}}`)
}

func cascadeContentSchema() json.RawMessage {
	return rawJSON(`{"type":"object","properties":{"title":{"type":"string","default":"{{ payload.title }}"},"body":{"type":"string","default":"{{ payload.content }}"},"payload":{"type":"object","additionalProperties":true}},"additionalProperties":true}`)
}

func robotTextContentSchema() json.RawMessage {
	return rawJSON(`{"type":"object","required":["content"],"properties":{"title":{"type":"string","default":"{{ payload.title }}"},"body":{"type":"string","default":"{{ payload.content }}"},"content":{"type":"string","default":"{{ payload.content }}"},"markdown":{"type":"string"}}}`)
}

func dingTalkRobotTextContentSchema() json.RawMessage {
	return rawJSON(`{"type":"object","required":["content"],"properties":{"content":{"type":"string","title":"content","default":"{{ payload.content }}"}}}`)
}

func dingTalkRobotMarkdownContentSchema() json.RawMessage {
	return rawJSON(`{"type":"object","required":["title","text"],"field_order":["title","text"],"properties":{"title":{"type":"string","title":"Markdown 标题","default":"{{ payload.title }}"},"text":{"type":"string","title":"Markdown 内容","default":"{{ payload.content }}","format_hint":"支持标准 Markdown；换行用 \\n，空格可用 &nbsp;"}}}`)
}

func dingTalkWorkTextContentSchema() json.RawMessage {
	return rawJSON(`{"type":"object","required":["content"],"properties":{"content":{"type":"string","title":"content","default":"{{ payload.content }}"}}}`)
}

func dingTalkWorkMarkdownContentSchema() json.RawMessage {
	return rawJSON(`{"type":"object","required":["title","text"],"field_order":["title","text"],"properties":{"title":{"type":"string","title":"title","default":"{{ payload.title }}"},"text":{"type":"string","title":"text","default":"{{ payload.content }}","format_hint":"支持标准 Markdown；换行用 \\n，空格可用 &nbsp;"}}}`)
}

func feishuGroupContentSchema() json.RawMessage {
	return rawJSON(`{"type":"object","required":["msgtype","text"],"properties":{"msgtype":{"type":"string","enum":["text"],"default":"text"},"text":{"type":"string","title":"text","default":"{{ payload.content }}"}}}`)
}

func weComRobotContentSchema() json.RawMessage {
	return rawJSON(`{"type":"object","required":["msgtype","content"],"properties":{"msgtype":{"type":"string","enum":["text","markdown"],"enum_descriptions":{"text":"文本消息","markdown":"Markdown 消息"},"default":"text"},"content":{"type":"string","title":"content","default":"{{ payload.content }}"}}}`)
}

func weComAppContentSchema() json.RawMessage {
	return rawJSON(`{"type":"object","required":["msgtype"],"properties":{"msgtype":{"type":"string","enum":["text","markdown","textcard"],"enum_descriptions":{"text":"文本消息","markdown":"Markdown 消息","textcard":"文本卡片消息"},"default":"text"},"content":{"type":"string","title":"Text content","default":"{{ payload.content }}"},"markdown":{"type":"string","title":"Markdown content","default":"{{ payload.content }}"},"title":{"type":"string","title":"Card title","default":"{{ payload.title }}"},"description":{"type":"string","title":"Card description","default":"{{ payload.content }}"},"url":{"type":"string","title":"Card URL","default":"{{ payload.url }}"},"btntxt":{"type":"string","title":"Button text","default":"详情"}}}`)
}

func smsTemplateContentSchema() json.RawMessage {
	return rawJSON(`{"type":"object","required":["template_params"],"properties":{"template_params":{"type":["object","array"],"title":"Template parameters"},"out_id":{"type":"string"},"sign_name":{"type":"string"},"template_id":{"type":"string"}}}`)
}
