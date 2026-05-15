package provider

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestDefaultCapabilitiesExposeProviderMetadata(t *testing.T) {
	capabilities := DefaultCapabilities()
	if len(capabilities) == 0 {
		t.Fatal("expected default capabilities")
	}

	seen := map[ProviderType]bool{}
	for _, capability := range capabilities {
		seen[capability.ProviderType] = true
		if capability.DisplayName == "" {
			t.Fatalf("%s/%s missing display name", capability.ProviderType, capability.MessageType)
		}
		if capability.Category == "" {
			t.Fatalf("%s/%s missing category", capability.ProviderType, capability.MessageType)
		}
		assertJSONObject(t, capability.CredentialSchema, "%s/%s credential schema", capability.ProviderType, capability.MessageType)
		assertJSONObject(t, capability.ChannelConfigSchema, "%s/%s channel config schema", capability.ProviderType, capability.MessageType)
		assertJSONObject(t, capability.MessageSchema, "%s/%s message schema", capability.ProviderType, capability.MessageType)
		assertJSONObject(t, capability.TokenStrategy, "%s/%s token strategy", capability.ProviderType, capability.MessageType)
		assertJSONObject(t, capability.SendAPI, "%s/%s send api", capability.ProviderType, capability.MessageType)
		assertJSONObject(t, capability.SuccessRule, "%s/%s success rule", capability.ProviderType, capability.MessageType)
		assertJSONObject(t, capability.RetryRule, "%s/%s retry rule", capability.ProviderType, capability.MessageType)
		assertJSONObject(t, capability.DefaultRateLimit, "%s/%s rate limit defaults", capability.ProviderType, capability.MessageType)
		assertJSONObject(t, capability.DefaultRetryPolicy, "%s/%s retry defaults", capability.ProviderType, capability.MessageType)
		if capability.DefaultTimeoutMS <= 0 {
			t.Fatalf("%s/%s missing timeout default", capability.ProviderType, capability.MessageType)
		}
		if capability.DefaultConcurrencyLimit <= 0 {
			t.Fatalf("%s/%s missing concurrency default", capability.ProviderType, capability.MessageType)
		}
		assertMessageSchemaHasProperties(t, capability)
	}

	for _, providerType := range []ProviderType{
		ProviderWeCom,
		ProviderFeishu,
		ProviderDingTalk,
		ProviderEmail,
		ProviderSMS,
		ProviderGovCloud,
		ProviderSelf,
		ProviderWebhook,
		ProviderCustomToken,
	} {
		if !seen[providerType] {
			t.Fatalf("missing default capability for provider type %s", providerType)
		}
	}
}

func TestDefaultCapabilitiesExposeFirstBatchBuiltInProviders(t *testing.T) {
	required := []struct {
		providerType ProviderType
		messageType  string
		identityKind string
	}{
		{ProviderWebhook, "json", ""},
		{ProviderSelf, "json", "system_user_id"},
		{ProviderPushPlus, "html", ""},
		{ProviderWxPusher, "html", "wxpusher_uid"},
		{ProviderServerChan, "notice", ""},
		{ProviderEmail, "email", "email"},
		{ProviderAliyunSMS, "sms_template", "mobile"},
		{ProviderTencentSMS, "sms_template", "mobile"},
		{ProviderBaiduSMS, "sms_template", "mobile"},
		{ProviderWeComRobot, "text", "wecom_userid"},
		{ProviderWeComApp, "text", "wecom_userid"},
		{ProviderWeCom, "text", "wecom_userid"},
		{ProviderDingTalkRobot, "text", "mobile"},
		{ProviderDingTalkWork, "text", "dingtalk_userid"},
		{ProviderDingTalk, "text", "dingtalk_userid"},
		{ProviderFeishuRobot, "text", "feishu_open_id"},
		{ProviderGovCloud, "text", "gov_userid"},
	}

	for _, item := range required {
		t.Run(string(item.providerType), func(t *testing.T) {
			capability := findCapability(t, item.providerType, item.messageType)
			if item.identityKind != "" && capability.IdentityKind != item.identityKind {
				t.Fatalf("expected identity kind %q, got %q", item.identityKind, capability.IdentityKind)
			}
			assertCapabilityHasLiveTestMetadata(t, capability)
			if len(capability.RequestExamples) == 0 || string(capability.RequestExamples) == "{}" {
				t.Fatalf("%s/%s missing request example", capability.ProviderType, capability.MessageType)
			}
		})
	}
}

func TestPushPlusCapabilityUsesSendJsonContentFields(t *testing.T) {
	capability := findCapability(t, ProviderPushPlus, "html")

	assertJSONField(t, capability.SendAPI, "url", "https://www.pushplus.plus/send")
	assertJSONField(t, capability.SendAPI, "content_type", "application/json")
	assertJSONField(t, capability.MessageSchema, "properties.content.type", "string")
	assertJSONField(t, capability.MessageSchema, "properties.title.type", "string")
	assertJSONField(t, capability.MessageSchema, "properties.topic.type", "string")

	var messageSchema struct {
		Required []string `json:"required"`
	}
	if err := json.Unmarshal(capability.MessageSchema, &messageSchema); err != nil {
		t.Fatalf("decode pushplus message schema: %v", err)
	}
	if len(messageSchema.Required) != 1 || messageSchema.Required[0] != "content" {
		t.Fatalf("expected only content to be required, got %+v", messageSchema.Required)
	}
	assertJSONField(t, capability.ChannelConfigSchema, "properties.topic", nil)
	assertJSONField(t, capability.ChannelConfigSchema, "properties.template", nil)
	assertJSONField(t, capability.ChannelConfigSchema, "properties.channel", nil)
}

func TestWxPusherCapabilityUsesStandardPostHTMLFields(t *testing.T) {
	capability := findCapability(t, ProviderWxPusher, "html")

	assertJSONField(t, capability.SendAPI, "url", "https://wxpusher.zjiecode.com/api/send/message")
	assertJSONField(t, capability.SendAPI, "content_type", "application/json")
	assertJSONField(t, capability.SendAPI, "simple_url", nil)
	assertJSONField(t, capability.CredentialSchema, "properties.app_token.format", "password")
	assertJSONField(t, capability.CredentialSchema, "properties.spt", nil)
	assertJSONField(t, capability.ChannelConfigSchema, "properties.topic_ids.items.type", "integer")
	assertJSONField(t, capability.ChannelConfigSchema, "properties.mode", nil)
	assertJSONField(t, capability.ChannelConfigSchema, "properties.content_type", nil)
	assertJSONField(t, capability.MessageSchema, "properties.content.type", "string")
	assertJSONField(t, capability.MessageSchema, "properties.summary.type", "string")
	assertJSONField(t, capability.MessageSchema, "properties.url.type", "string")
	assertJSONField(t, capability.MessageSchema, "properties.title", nil)
	assertJSONField(t, capability.MessageSchema, "properties.body", nil)
	assertJSONField(t, capability.MessageSchema, "properties.format", nil)
	assertJSONField(t, capability.SuccessRule, "field", "code")
	assertJSONField(t, capability.SuccessRule, "equals", float64(1000))

	var credentialSchema struct {
		Required []string `json:"required"`
	}
	if err := json.Unmarshal(capability.CredentialSchema, &credentialSchema); err != nil {
		t.Fatalf("decode wxpusher credential schema: %v", err)
	}
	if len(credentialSchema.Required) != 1 || credentialSchema.Required[0] != "app_token" {
		t.Fatalf("expected only app_token credential to be required, got %+v", credentialSchema.Required)
	}
	var messageSchema struct {
		Required []string `json:"required"`
	}
	if err := json.Unmarshal(capability.MessageSchema, &messageSchema); err != nil {
		t.Fatalf("decode wxpusher message schema: %v", err)
	}
	if len(messageSchema.Required) != 1 || messageSchema.Required[0] != "content" {
		t.Fatalf("expected only content to be required, got %+v", messageSchema.Required)
	}
}

func TestDefaultCapabilitiesExposeP2Providers(t *testing.T) {
	required := []struct {
		providerType ProviderType
		messageType  string
		identityKind string
		liveStatus   string
	}{
		{ProviderNtfy, "notice", "", "configuration_dependent"},
		{ProviderGotify, "notice", "", "configuration_dependent"},
		{ProviderBark, "notice", "bark_device_key", "implemented_but_not_live_tested"},
		{ProviderPushMe, "notice", "", "implemented_but_not_live_tested"},
	}

	for _, item := range required {
		t.Run(string(item.providerType), func(t *testing.T) {
			capability := findCapability(t, item.providerType, item.messageType)
			if capability.IdentityKind != item.identityKind {
				t.Fatalf("expected identity kind %q, got %q", item.identityKind, capability.IdentityKind)
			}
			assertCapabilityHasLiveTestMetadata(t, capability)
			assertJSONField(t, capability.SendAPI, "live_test_status", item.liveStatus)
			if len(capability.RequestExamples) == 0 || string(capability.RequestExamples) == "{}" {
				t.Fatalf("%s/%s missing request example", capability.ProviderType, capability.MessageType)
			}
		})
	}
}

func TestGovCloudCapabilityUsesDocumentedTokenAndSendMetadata(t *testing.T) {
	capability := findCapability(t, ProviderGovCloud, "text")
	assertJSONField(t, capability.CredentialSchema, "properties.base_url.default", "https://www.ywxt.sh.cegn.cn/api-gateway/uranus/uranus/cgi-bin/")
	assertJSONField(t, capability.TokenStrategy, "token_url", "https://www.ywxt.sh.cegn.cn/api-gateway/uranus/uranus/cgi-bin/gettoken")
	assertJSONField(t, capability.TokenStrategy, "request.method", "GET")
	assertJSONField(t, capability.TokenStrategy, "request.query_secret_field", "corpsecret")
	assertJSONField(t, capability.TokenStrategy, "expires_in_seconds", float64(3600))
	assertJSONField(t, capability.SendAPI, "url", "https://www.ywxt.sh.cegn.cn/api-gateway/uranus/uranus/cgi-bin/request/message/send")
	assertJSONField(t, capability.SuccessRule, "field", "errcode")
	assertJSONField(t, capability.SuccessRule, "equals", float64(0))

	var retryRule map[string]any
	if err := json.Unmarshal(capability.RetryRule, &retryRule); err != nil {
		t.Fatalf("decode retry rule: %v", err)
	}
	requireJSONListContains(t, retryRule["refresh_token_codes"], float64(401))
	requireJSONListContains(t, retryRule["refresh_token_codes"], float64(40014))
	requireJSONListContains(t, retryRule["refresh_token_codes"], float64(42001))
	requireJSONListContains(t, retryRule["retryable_json_codes"], float64(-1))
	requireJSONListContains(t, retryRule["retryable_json_codes"], float64(523))
	requireJSONListContains(t, retryRule["non_retryable_json_codes"], float64(40031))
	requireJSONListContains(t, retryRule["non_retryable_json_codes"], float64(40032))
	requireJSONListContains(t, retryRule["non_retryable_json_codes"], float64(82001))
}

func TestDefaultCapabilitiesDistinguishWebhookAndBuiltInProviders(t *testing.T) {
	webhook := findCapability(t, ProviderWebhook, "json")
	if webhook.RecipientRequired {
		t.Fatal("webhook should not require a recipient")
	}
	if !webhook.AllowNoRecipient || webhook.RecipientRequirement != "none" {
		t.Fatalf("webhook should allow no recipient with none requirement, got allow=%v requirement=%q", webhook.AllowNoRecipient, webhook.RecipientRequirement)
	}
	if !webhook.CustomBodyAllowed {
		t.Fatal("webhook should allow custom JSON bodies")
	}

	customToken := findCapability(t, ProviderCustomToken, "json")
	if !customToken.CustomBodyAllowed {
		t.Fatal("custom token provider should allow custom JSON bodies")
	}
	if customToken.TokenLocation == PlacementNone {
		t.Fatal("custom token provider should describe token placement")
	}

	wecom := findCapability(t, ProviderWeCom, "text")
	if !wecom.RecipientRequired || wecom.RecipientRequirement != "system" {
		t.Fatalf("wecom should require system recipients, got required=%v requirement=%q", wecom.RecipientRequired, wecom.RecipientRequirement)
	}
	if wecom.CustomBodyAllowed {
		t.Fatal("built-in wecom provider should not allow custom JSON bodies by default")
	}
	if wecom.IdentityKind != "wecom_userid" || wecom.RecipientFormat != "pipe_string" {
		t.Fatalf("unexpected wecom recipient metadata: identity=%q format=%q", wecom.IdentityKind, wecom.RecipientFormat)
	}
}

func TestBuildRequestSupportsTokenAndRecipientPlacement(t *testing.T) {
	for _, tc := range []struct {
		name    string
		channel Channel
		input   BuildRequestInput
		assert  func(t *testing.T, request BuiltRequest)
	}{
		{
			name: "token query and recipient header",
			channel: Channel{
				TokenConfig: json.RawMessage(`{"location":"query","field_name":"access_token"}`),
				SendConfig:  json.RawMessage(`{"method":"POST","url":"https://example.test/send","recipient":{"location":"header","field_name":"X-Recipient"}}`),
			},
			input: BuildRequestInput{Token: "token-a", Recipient: "user-a"},
			assert: func(t *testing.T, request BuiltRequest) {
				if request.Query["access_token"] != "token-a" || request.Headers["X-Recipient"] != "user-a" {
					t.Fatalf("unexpected query/header placement: %+v", request)
				}
				if request.URL != "https://example.test/send?access_token=token-a" {
					t.Fatalf("unexpected url: %s", request.URL)
				}
			},
		},
		{
			name: "token header and recipient body array",
			channel: Channel{
				TokenConfig: json.RawMessage(`{"location":"header","field_name":"Authorization","prefix":"Bearer "}`),
				SendConfig:  json.RawMessage(`{"method":"POST","url":"https://example.test/send","body":{"msgtype":"text"},"recipient":{"location":"body","path":"touser","format":"array"}}`),
			},
			input: BuildRequestInput{Token: "token-b", Recipient: []any{"u1", "u2"}},
			assert: func(t *testing.T, request BuiltRequest) {
				if request.Headers["Authorization"] != "Bearer token-b" {
					t.Fatalf("unexpected auth header: %+v", request.Headers)
				}
				var body map[string]any
				if err := json.Unmarshal(request.Body, &body); err != nil {
					t.Fatalf("decode body: %v", err)
				}
				recipients, ok := body["touser"].([]any)
				if !ok || len(recipients) != 2 {
					t.Fatalf("expected array recipient in body, got %v", body["touser"])
				}
			},
		},
		{
			name: "token body and recipient path",
			channel: Channel{
				TokenConfig: json.RawMessage(`{"location":"body","path":"auth.token"}`),
				SendConfig:  json.RawMessage(`{"method":"POST","url":"https://example.test/send/{recipient}","recipient":{"location":"path","field_name":"recipient"}}`),
			},
			input: BuildRequestInput{Token: "token-c", Recipient: "mobile 1"},
			assert: func(t *testing.T, request BuiltRequest) {
				if request.URL != "https://example.test/send/mobile%201" {
					t.Fatalf("unexpected path placement url: %s", request.URL)
				}
				var body map[string]any
				if err := json.Unmarshal(request.Body, &body); err != nil {
					t.Fatalf("decode body: %v", err)
				}
				auth, ok := body["auth"].(map[string]any)
				if !ok || auth["token"] != "token-c" {
					t.Fatalf("expected nested body token, got %+v", body)
				}
			},
		},
		{
			name: "none placement",
			channel: Channel{
				TokenConfig: json.RawMessage(`{"location":"none"}`),
				SendConfig:  json.RawMessage(`{"method":"POST","url":"https://example.test/send","recipient":{"location":"none"}}`),
			},
			input: BuildRequestInput{Token: "token-d", Recipient: "u3"},
			assert: func(t *testing.T, request BuiltRequest) {
				if len(request.Query) != 0 || len(request.Headers) != 0 || string(request.Body) != "{}" {
					t.Fatalf("expected none placement to omit token/recipient, got %+v body=%s", request, request.Body)
				}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request, err := BuildRequest(tc.channel, tc.input)
			if err != nil {
				t.Fatalf("build request: %v", err)
			}
			tc.assert(t, request)
		})
	}
}

func TestBuildDeliveryRequestUsesBuiltInProviderDefaultsWithoutLegacyURL(t *testing.T) {
	for _, tc := range []struct {
		name       string
		channel    Channel
		token      string
		message    json.RawMessage
		recipients []ResolvedRecipient
		assert     func(t *testing.T, request BuiltRequest)
	}{
		{
			name: "pushplus",
			channel: Channel{
				ProviderType: ProviderPushPlus,
				AuthConfig:   json.RawMessage(`{"token":"push-token"}`),
				SendConfig:   json.RawMessage(`{"topic":"legacy-send-topic","template":"markdown"}`),
			},
			message: json.RawMessage(`{"title":"Disk alert","content":"Disk 95%","topic":"ops","format":"markdown","url":"https://example.test/detail"}`),
			assert: func(t *testing.T, request BuiltRequest) {
				requireRequest(t, request, "POST", "https://www.pushplus.plus/send")
				body := decodeRequestBody(t, request)
				requireBodyField(t, body, "token", "push-token")
				requireBodyField(t, body, "title", "Disk alert")
				requireBodyField(t, body, "content", "Disk 95%")
				requireBodyField(t, body, "topic", "ops")
				requireNoBodyField(t, body, "template")
				requireNoBodyField(t, body, "channel")
			},
		},
		{
			name: "wxpusher",
			channel: Channel{
				ProviderType: ProviderWxPusher,
				AuthConfig:   json.RawMessage(`{"app_token":"wx-app-token"}`),
				SendConfig:   json.RawMessage(`{"topic_ids":[101]}`),
			},
			message:    json.RawMessage(`{"summary":"Deploy","content":"<b>Finished</b>","contentType":3,"url":"https://example.test/detail"}`),
			recipients: []ResolvedRecipient{{PlatformIDs: map[string]string{"wxpusher_uid": "UID_1"}}},
			assert: func(t *testing.T, request BuiltRequest) {
				requireRequest(t, request, "POST", "https://wxpusher.zjiecode.com/api/send/message")
				body := decodeRequestBody(t, request)
				requireBodyField(t, body, "appToken", "wx-app-token")
				requireBodyField(t, body, "summary", "Deploy")
				requireBodyField(t, body, "content", "<b>Finished</b>")
				requireBodyField(t, body, "contentType", float64(2))
				requireBodyField(t, body, "url", "https://example.test/detail")
				requireStringListField(t, body, "uids", []string{"UID_1"})
				requireNumberListField(t, body, "topicIds", []float64{101})
				requireNoBodyField(t, body, "spt")
			},
		},
		{
			name: "serverchan",
			channel: Channel{
				ProviderType: ProviderServerChan,
				AuthConfig:   json.RawMessage(`{"send_key":"SCT123"}`),
				SendConfig:   json.RawMessage(`{"version":"turbo","channel":"9"}`),
			},
			message: json.RawMessage(`{"title":"Build","body":"Failed"}`),
			assert: func(t *testing.T, request BuiltRequest) {
				requireRequest(t, request, "POST", "https://sctapi.ftqq.com/SCT123.send")
				if request.Headers["Content-Type"] != "application/x-www-form-urlencoded" {
					t.Fatalf("expected form content type, got %+v", request.Headers)
				}
				body := decodeRequestBody(t, request)
				requireBodyField(t, body, "title", "Build")
				requireBodyField(t, body, "desp", "Failed")
				requireBodyField(t, body, "channel", "9")
			},
		},
		{
			name: "self cascade",
			channel: Channel{
				ProviderType: ProviderSelf,
				AuthConfig:   json.RawMessage(`{"base_url":"https://upstream.example","source_token":"source-token"}`),
				SendConfig:   json.RawMessage(`{"source_code":"alerts","api_prefix":"/api/v1","payload_mode":"wrapped"}`),
			},
			message:    json.RawMessage(`{"title":"Alert","body":"Disk full"}`),
			recipients: []ResolvedRecipient{{SystemUserID: "user-1"}},
			assert: func(t *testing.T, request BuiltRequest) {
				requireRequest(t, request, "POST", "https://upstream.example/api/v1/ingest/alerts")
				if request.Headers["Authorization"] != "Bearer source-token" {
					t.Fatalf("expected bearer source token, got %+v", request.Headers)
				}
				body := decodeRequestBody(t, request)
				message, ok := body["message"].(map[string]any)
				if !ok || message["title"] != "Alert" {
					t.Fatalf("expected wrapped message, got %+v", body)
				}
				requireStringListField(t, body, "recipients", []string{"user-1"})
			},
		},
		{
			name: "aliyun sms",
			channel: Channel{
				ProviderType: ProviderAliyunSMS,
				AuthConfig:   json.RawMessage(`{"access_key_id":"ak","access_key_secret":"sk"}`),
				SendConfig:   json.RawMessage(`{"sign_name":"Ops","template_id":"SMS_1","region":"cn-hangzhou"}`),
			},
			message:    json.RawMessage(`{"template_params":{"code":"1234"}}`),
			recipients: []ResolvedRecipient{{Mobile: "13800138000"}},
			assert: func(t *testing.T, request BuiltRequest) {
				requireRequest(t, request, "POST", "https://dysmsapi.aliyuncs.com/")
				body := decodeRequestBody(t, request)
				requireBodyField(t, body, "vendor", "aliyun")
				requireStringListField(t, body, "phone_numbers", []string{"13800138000"})
				requireBodyField(t, body, "sign_name", "Ops")
				requireBodyField(t, body, "template_code", "SMS_1")
			},
		},
		{
			name: "tencent sms",
			channel: Channel{
				ProviderType: ProviderTencentSMS,
				AuthConfig:   json.RawMessage(`{"secret_id":"sid","secret_key":"sk"}`),
				SendConfig:   json.RawMessage(`{"sms_sdk_app_id":"1400001","sign_name":"Ops","template_id":"1001","region":"ap-shanghai"}`),
			},
			message:    json.RawMessage(`{"template_params":["1234","5"]}`),
			recipients: []ResolvedRecipient{{Mobile: "13800138000"}},
			assert: func(t *testing.T, request BuiltRequest) {
				requireRequest(t, request, "POST", "https://sms.tencentcloudapi.com/")
				body := decodeRequestBody(t, request)
				requireBodyField(t, body, "vendor", "tencent")
				requireStringListField(t, body, "PhoneNumberSet", []string{"13800138000"})
				requireBodyField(t, body, "SmsSdkAppId", "1400001")
				requireBodyField(t, body, "TemplateId", "1001")
			},
		},
		{
			name: "baidu sms",
			channel: Channel{
				ProviderType: ProviderBaiduSMS,
				AuthConfig:   json.RawMessage(`{"access_key_id":"ak","secret_access_key":"sk"}`),
				SendConfig:   json.RawMessage(`{"signature_id":"sig","template_id":"tpl","region":"bj"}`),
			},
			message:    json.RawMessage(`{"template_params":{"code":"1234"}}`),
			recipients: []ResolvedRecipient{{Mobile: "13800138000"}},
			assert: func(t *testing.T, request BuiltRequest) {
				requireRequest(t, request, "POST", "https://sms.bj.baidubce.com/bce/v2/message")
				body := decodeRequestBody(t, request)
				requireBodyField(t, body, "vendor", "baidu")
				requireStringListField(t, body, "mobile", []string{"13800138000"})
				requireBodyField(t, body, "signature_id", "sig")
				requireBodyField(t, body, "template", "tpl")
			},
		},
		{
			name: "wecom robot",
			channel: Channel{
				ProviderType: ProviderWeComRobot,
				AuthConfig:   json.RawMessage(`{"key":"robot-key"}`),
			},
			message:    json.RawMessage(`{"body":"hello robot"}`),
			recipients: []ResolvedRecipient{{PlatformIDs: map[string]string{"wecom_userid": "u1"}}},
			assert: func(t *testing.T, request BuiltRequest) {
				requireRequest(t, request, "POST", "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=robot-key")
				body := decodeRequestBody(t, request)
				requireBodyField(t, body, "msgtype", "text")
				text := requireObjectField(t, body, "text")
				requireBodyField(t, text, "content", "hello robot")
				requireStringListField(t, text, "mentioned_list", []string{"u1"})
			},
		},
		{
			name: "wecom app",
			channel: Channel{
				ProviderType: ProviderWeComApp,
				AuthConfig:   json.RawMessage(`{"agentid":1000001}`),
			},
			token:      "wecom-token",
			message:    json.RawMessage(`{"body":"hello app"}`),
			recipients: []ResolvedRecipient{{PlatformIDs: map[string]string{"wecom_userid": "u1"}}, {PlatformIDs: map[string]string{"wecom_userid": "u2"}}},
			assert: func(t *testing.T, request BuiltRequest) {
				requireRequest(t, request, "POST", "https://qyapi.weixin.qq.com/cgi-bin/message/send?access_token=wecom-token")
				body := decodeRequestBody(t, request)
				requireBodyField(t, body, "touser", "u1|u2")
				requireBodyField(t, body, "agentid", float64(1000001))
				requireBodyField(t, body, "msgtype", "text")
			},
		},
		{
			name: "dingtalk robot",
			channel: Channel{
				ProviderType: ProviderDingTalkRobot,
				AuthConfig:   json.RawMessage(`{"webhook_url":"https://oapi.dingtalk.com/robot/send?access_token=robot-token"}`),
			},
			message:    json.RawMessage(`{"title":"Notice","body":"hello ding"}`),
			recipients: []ResolvedRecipient{{Mobile: "13800138000"}},
			assert: func(t *testing.T, request BuiltRequest) {
				requireRequest(t, request, "POST", "https://oapi.dingtalk.com/robot/send?access_token=robot-token")
				body := decodeRequestBody(t, request)
				requireBodyField(t, body, "msgtype", "text")
				at := requireObjectField(t, body, "at")
				requireStringListField(t, at, "atMobiles", []string{"13800138000"})
			},
		},
		{
			name: "dingtalk work",
			channel: Channel{
				ProviderType: ProviderDingTalkWork,
				AuthConfig:   json.RawMessage(`{"agent_id":123}`),
			},
			token:      "ding-token",
			message:    json.RawMessage(`{"body":"hello work"}`),
			recipients: []ResolvedRecipient{{PlatformIDs: map[string]string{"dingtalk_userid": "u1"}}, {PlatformIDs: map[string]string{"dingtalk_userid": "u2"}}},
			assert: func(t *testing.T, request BuiltRequest) {
				requireRequest(t, request, "POST", "https://oapi.dingtalk.com/topapi/message/corpconversation/asyncsend_v2?access_token=ding-token")
				body := decodeRequestBody(t, request)
				requireBodyField(t, body, "agent_id", float64(123))
				requireBodyField(t, body, "userid_list", "u1,u2")
				msg := requireObjectField(t, body, "msg")
				requireBodyField(t, msg, "msgtype", "text")
			},
		},
		{
			name: "feishu robot",
			channel: Channel{
				ProviderType: ProviderFeishuRobot,
				AuthConfig:   json.RawMessage(`{"webhook_url":"https://open.feishu.cn/open-apis/bot/v2/hook/hook-token"}`),
			},
			message: json.RawMessage(`{"body":"hello feishu"}`),
			assert: func(t *testing.T, request BuiltRequest) {
				requireRequest(t, request, "POST", "https://open.feishu.cn/open-apis/bot/v2/hook/hook-token")
				body := decodeRequestBody(t, request)
				requireBodyField(t, body, "msg_type", "text")
				content := requireObjectField(t, body, "content")
				requireBodyField(t, content, "text", "hello feishu")
			},
		},
		{
			name: "gov cloud",
			channel: Channel{
				ProviderType: ProviderGovCloud,
				AuthConfig:   json.RawMessage(`{"corpsecret":"secret"}`),
			},
			token:      "gov-token",
			message:    json.RawMessage(`{"description":"gov message"}`),
			recipients: []ResolvedRecipient{{PlatformIDs: map[string]string{"gov_userid": "gov-u1"}}},
			assert: func(t *testing.T, request BuiltRequest) {
				requireRequest(t, request, "POST", "https://www.ywxt.sh.cegn.cn/api-gateway/uranus/uranus/cgi-bin/request/message/send?access_token=gov-token")
				body := decodeRequestBody(t, request)
				requireBodyField(t, body, "touser", "gov-u1")
				requireBodyField(t, body, "msgtype", "text")
				requireBodyField(t, body, "description", "gov message")
			},
		},
		{
			name: "ntfy",
			channel: Channel{
				ProviderType: ProviderNtfy,
				AuthConfig:   json.RawMessage(`{"server_url":"https://ntfy.example","bearer_token":"ntfy-token"}`),
				SendConfig:   json.RawMessage(`{"topic":"ops","priority":"high","tags":["warning","disk"],"markdown":true}`),
			},
			message: json.RawMessage(`{"title":"Disk alert","body":"Disk 95%","url":"https://example.test/detail"}`),
			assert: func(t *testing.T, request BuiltRequest) {
				requireRequest(t, request, "POST", "https://ntfy.example/ops")
				if request.Headers["Authorization"] != "Bearer ntfy-token" {
					t.Fatalf("expected bearer token header, got %+v", request.Headers)
				}
				if request.Headers["Title"] != "Disk alert" || request.Headers["Priority"] != "high" || request.Headers["Tags"] != "warning,disk" || request.Headers["Markdown"] != "yes" || request.Headers["Click"] != "https://example.test/detail" {
					t.Fatalf("unexpected ntfy headers: %+v", request.Headers)
				}
				body := decodeRequestBody(t, request)
				requireBodyField(t, body, "message", "Disk 95%\n\nhttps://example.test/detail")
				requireBodyField(t, body, "mock_protocol", "ntfy_text_body")
			},
		},
		{
			name: "gotify",
			channel: Channel{
				ProviderType: ProviderGotify,
				AuthConfig:   json.RawMessage(`{"server_url":"https://gotify.example","app_token":"gotify-token"}`),
				SendConfig:   json.RawMessage(`{"priority":8,"content_type":"text/markdown"}`),
			},
			message: json.RawMessage(`{"title":"Deploy","body":"Finished","format":"markdown"}`),
			assert: func(t *testing.T, request BuiltRequest) {
				requireRequest(t, request, "POST", "https://gotify.example/message?token=gotify-token")
				body := decodeRequestBody(t, request)
				requireBodyField(t, body, "title", "Deploy")
				requireBodyField(t, body, "message", "Finished")
				requireBodyField(t, body, "priority", float64(8))
				extras := requireObjectField(t, body, "extras")
				display := requireObjectField(t, extras, "client::display")
				requireBodyField(t, display, "contentType", "text/markdown")
			},
		},
		{
			name: "bark",
			channel: Channel{
				ProviderType: ProviderBark,
				AuthConfig:   json.RawMessage(`{"server_url":"https://bark.example","device_key":"channel-key"}`),
				SendConfig:   json.RawMessage(`{"group":"ops","sound":"alarm","level":"critical","icon":"https://example.test/icon.png"}`),
			},
			message:    json.RawMessage(`{"title":"Disk alert","body":"Disk 95%","subtitle":"prod","url":"https://example.test/detail"}`),
			recipients: []ResolvedRecipient{{PlatformIDs: map[string]string{"bark_device_key": "device-key"}}},
			assert: func(t *testing.T, request BuiltRequest) {
				requireRequest(t, request, "POST", "https://bark.example/push")
				body := decodeRequestBody(t, request)
				requireBodyField(t, body, "device_key", "device-key")
				requireBodyField(t, body, "title", "Disk alert")
				requireBodyField(t, body, "subtitle", "prod")
				requireBodyField(t, body, "body", "Disk 95%")
				requireBodyField(t, body, "url", "https://example.test/detail")
				requireBodyField(t, body, "group", "ops")
				requireBodyField(t, body, "sound", "alarm")
				requireBodyField(t, body, "level", "critical")
				requireBodyField(t, body, "icon", "https://example.test/icon.png")
			},
		},
		{
			name: "pushme",
			channel: Channel{
				ProviderType: ProviderPushMe,
				AuthConfig:   json.RawMessage(`{"server_url":"https://push.example","push_key":"push-key"}`),
				SendConfig:   json.RawMessage(`{"type":"markdown"}`),
			},
			message: json.RawMessage(`{"title":"Build","body":"Failed","format":"markdown"}`),
			assert: func(t *testing.T, request BuiltRequest) {
				requireRequest(t, request, "POST", "https://push.example")
				body := decodeRequestBody(t, request)
				requireBodyField(t, body, "push_key", "push-key")
				requireBodyField(t, body, "title", "Build")
				requireBodyField(t, body, "content", "Failed")
				requireBodyField(t, body, "type", "markdown")
				requireBodyField(t, body, "live_test_status", "implemented_but_not_live_tested")
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request, err := BuildDeliveryRequest(tc.channel, BuildDeliveryRequestInput{
				Token: tc.token,
				RenderedMessage: RenderedMessage{
					ProviderType: tc.channel.ProviderType,
					Content:      tc.message,
				},
				ResolvedRecipients: tc.recipients,
				TargetContext: DeliveryTargetContext{
					DeliveryAttemptID: "attempt-1",
					MessageID:         "message-1",
					ChannelID:         "channel-1",
					ChannelName:       "Channel",
					ProviderType:      string(tc.channel.ProviderType),
					TemplateVersionID: "template-version-1",
					JobID:             "job-1",
				},
			})
			if err != nil {
				t.Fatalf("build request: %v", err)
			}
			tc.assert(t, request)
		})
	}
}

func TestBuildDeliveryRequestUsesRenderedMessageRecipientsAndTargetContext(t *testing.T) {
	request, err := BuildDeliveryRequest(Channel{
		ID:           "channel-wecom",
		ProviderType: ProviderWeCom,
		Name:         "WeCom Prod",
		TokenConfig:  json.RawMessage(`{"location":"query","field_name":"access_token"}`),
		SendConfig: json.RawMessage(`{
			"method":"POST",
			"url":"https://example.test/send",
			"body":{"msgtype":"text"},
			"recipient":{"location":"body","path":"touser","format":"pipe_string"}
		}`),
	}, BuildDeliveryRequestInput{
		Token: "token-1",
		RenderedMessage: RenderedMessage{
			ProviderType: ProviderWeCom,
			MessageType:  "text",
			Content:      json.RawMessage(`{"text":{"content":"hello from template"}}`),
		},
		ResolvedRecipients: []ResolvedRecipient{
			{PlatformIDs: map[string]string{"wecom_userid": "u1"}},
			{PlatformIDs: map[string]string{"wecom_userid": "u2"}},
		},
		TargetContext: DeliveryTargetContext{
			DeliveryAttemptID: "attempt-1",
			MessageID:         "message-1",
			ChannelID:         "channel-wecom",
			ChannelName:       "WeCom Prod",
			ProviderType:      string(ProviderWeCom),
			TemplateVersionID: "template-version-1",
			JobID:             "job-1",
		},
	})
	if err != nil {
		t.Fatalf("build delivery request: %v", err)
	}

	if request.URL != "https://example.test/send?access_token=token-1" {
		t.Fatalf("unexpected adapter url: %s", request.URL)
	}
	var body map[string]any
	if err := json.Unmarshal(request.Body, &body); err != nil {
		t.Fatalf("decode adapter body: %v", err)
	}
	if body["msgtype"] != "text" {
		t.Fatalf("expected configured base body to remain, got %+v", body)
	}
	text, ok := body["text"].(map[string]any)
	if !ok || text["content"] != "hello from template" {
		t.Fatalf("expected rendered message content in body, got %+v", body)
	}
	if body["touser"] != "u1|u2" {
		t.Fatalf("expected adapter to format resolved recipients for WeCom, got %+v", body)
	}
}

func TestBuildDeliveryRequestKeepsWebhookAdvancedMapping(t *testing.T) {
	request, err := BuildDeliveryRequest(Channel{
		ID:           "channel-webhook",
		ProviderType: ProviderWebhook,
		Name:         "Webhook",
		TokenConfig:  json.RawMessage(`{"location":"header","field_name":"X-Token","prefix":"Bearer "}`),
		SendConfig: json.RawMessage(`{
			"method":"PATCH",
			"url":"https://example.test/hooks/{recipient}",
			"headers":{"X-Static":"yes"},
			"body":{"kind":"alert"},
			"recipient":{"location":"path","field_name":"recipient"}
		}`),
	}, BuildDeliveryRequestInput{
		Token: "token-2",
		RenderedMessage: RenderedMessage{
			ProviderType: ProviderWebhook,
			MessageType:  "json",
			Content:      json.RawMessage(`{"summary":"disk full"}`),
		},
		ResolvedRecipients: []ResolvedRecipient{{Value: "ops room"}},
		TargetContext: DeliveryTargetContext{
			DeliveryAttemptID: "attempt-2",
			MessageID:         "message-2",
			ChannelID:         "channel-webhook",
			ChannelName:       "Webhook",
			ProviderType:      string(ProviderWebhook),
			TemplateVersionID: "template-version-2",
			JobID:             "job-2",
		},
	})
	if err != nil {
		t.Fatalf("build delivery request: %v", err)
	}
	if request.Method != "PATCH" {
		t.Fatalf("expected custom method to remain, got %s", request.Method)
	}
	if request.URL != "https://example.test/hooks/ops%20room" {
		t.Fatalf("expected path recipient mapping to remain, got %s", request.URL)
	}
	if request.Headers["X-Static"] != "yes" || request.Headers["X-Token"] != "Bearer token-2" {
		t.Fatalf("expected advanced headers to remain, got %+v", request.Headers)
	}
	var body map[string]any
	if err := json.Unmarshal(request.Body, &body); err != nil {
		t.Fatalf("decode adapter body: %v", err)
	}
	if body["kind"] != "alert" || body["summary"] != "disk full" {
		t.Fatalf("expected advanced base body and rendered content to merge, got %+v", body)
	}
}

func TestTestSendDryRunSnapshotIncludesAdapterContext(t *testing.T) {
	service := NewService(singleChannelStore{
		channel: Channel{
			ID:           "channel-webhook",
			ProviderType: ProviderWebhook,
			Name:         "Local Webhook",
			SendConfig: json.RawMessage(`{
				"method":"POST",
				"url":"http://127.0.0.1:18081/webhook",
				"headers":{"X-Test":"dry-run"},
				"body":{"gateway":"mvp-push"},
				"recipient":{"location":"none"}
			}`),
			TimeoutMS: 1000,
		},
	})

	result, err := service.TestSend(context.Background(), "channel-webhook", TestSendInput{
		BuildRequestInput: BuildRequestInput{
			Recipient: map[string]any{"system_user_id": "ops-1"},
			Body:      json.RawMessage(`{"title":"dry run","content":"只生成请求"}`),
		},
	})
	if err != nil {
		t.Fatalf("dry-run test send: %v", err)
	}
	if result.Status != "dry_run" {
		t.Fatalf("expected dry_run status, got %q", result.Status)
	}
	if result.ResponseSnapshot != nil {
		t.Fatalf("dry-run must not include upstream response snapshot: %s", result.ResponseSnapshot)
	}

	var snapshot map[string]any
	if err := json.Unmarshal(result.RequestSnapshot, &snapshot); err != nil {
		t.Fatalf("decode dry-run snapshot: %v", err)
	}
	for _, key := range []string{"final_request", "target_context", "rendered_message", "resolved_recipients"} {
		if _, ok := snapshot[key]; !ok {
			t.Fatalf("dry-run snapshot missing %s: %+v", key, snapshot)
		}
	}
	finalRequest, ok := snapshot["final_request"].(map[string]any)
	if !ok {
		t.Fatalf("expected final_request object, got %+v", snapshot["final_request"])
	}
	if finalRequest["url"] != "http://127.0.0.1:18081/webhook" || finalRequest["method"] != "POST" {
		t.Fatalf("unexpected final request summary: %+v", finalRequest)
	}
}

func TestTestSendRequiresExplicitLiveSendConfirmation(t *testing.T) {
	service := NewService(singleChannelStore{
		channel: Channel{
			ID:           "channel-webhook",
			ProviderType: ProviderWebhook,
			Name:         "Local Webhook",
			SendConfig:   json.RawMessage(`{"method":"POST","url":"http://127.0.0.1:18081/webhook"}`),
			TimeoutMS:    1000,
		},
	})

	_, err := service.TestSend(context.Background(), "channel-webhook", TestSendInput{
		Send: true,
		BuildRequestInput: BuildRequestInput{
			Body: json.RawMessage(`{"title":"live send"}`),
		},
	})
	if err == nil || !strings.Contains(err.Error(), "真实发送需要二次确认") {
		t.Fatalf("expected explicit live send confirmation error, got %v", err)
	}
}

func findCapability(t *testing.T, providerType ProviderType, messageType string) Capability {
	t.Helper()

	for _, capability := range DefaultCapabilities() {
		if capability.ProviderType == providerType && capability.MessageType == messageType {
			return capability
		}
	}
	t.Fatalf("capability %s/%s not found", providerType, messageType)
	return Capability{}
}

type singleChannelStore struct {
	channel Channel
}

func (s singleChannelStore) SeedProviderCapabilities(context.Context, []Capability) error {
	return nil
}

func (s singleChannelStore) ListProviderCapabilities(context.Context) ([]Capability, error) {
	return nil, nil
}

func (s singleChannelStore) ListChannels(context.Context) ([]Channel, error) {
	return []Channel{s.channel}, nil
}

func (s singleChannelStore) CreateChannel(context.Context, CreateChannelParams) (Channel, error) {
	return Channel{}, ErrInvalidInput
}

func (s singleChannelStore) GetChannel(_ context.Context, id string) (Channel, error) {
	if id != s.channel.ID {
		return Channel{}, ErrNotFound
	}
	return s.channel, nil
}

func (s singleChannelStore) UpdateChannel(context.Context, string, UpdateChannelParams) (Channel, error) {
	return Channel{}, ErrInvalidInput
}

func (s singleChannelStore) DeleteChannel(context.Context, string) error {
	return ErrInvalidInput
}

func assertCapabilityHasLiveTestMetadata(t *testing.T, capability Capability) {
	t.Helper()

	var sendAPI map[string]any
	if err := json.Unmarshal(capability.SendAPI, &sendAPI); err != nil {
		t.Fatalf("%s/%s decode send api: %v", capability.ProviderType, capability.MessageType, err)
	}
	if strings.TrimSpace(fmtString(sendAPI["live_test_status"])) == "" && strings.TrimSpace(fmtString(sendAPI["notes"])) == "" {
		t.Fatalf("%s/%s should include live_test_status or notes metadata", capability.ProviderType, capability.MessageType)
	}
}

func assertJSONObject(t *testing.T, raw json.RawMessage, format string, args ...any) {
	t.Helper()

	var object map[string]any
	if err := json.Unmarshal(raw, &object); err != nil {
		t.Fatalf(format+": decode object: %v", append(args, err)...)
	}
	if len(object) == 0 {
		t.Fatalf(format+": expected non-empty object", args...)
	}
}

func assertMessageSchemaHasProperties(t *testing.T, capability Capability) {
	t.Helper()

	var schema struct {
		Type       string         `json:"type"`
		Properties map[string]any `json:"properties"`
	}
	if err := json.Unmarshal(capability.MessageSchema, &schema); err != nil {
		t.Fatalf("%s/%s decode message schema: %v", capability.ProviderType, capability.MessageType, err)
	}
	if schema.Type != "object" || len(schema.Properties) == 0 {
		t.Fatalf("%s/%s should expose an object message schema with properties, got %s", capability.ProviderType, capability.MessageType, capability.MessageSchema)
	}
}

func assertJSONField(t *testing.T, raw json.RawMessage, path string, expected any) {
	t.Helper()

	var object any
	if err := json.Unmarshal(raw, &object); err != nil {
		t.Fatalf("decode json: %v", err)
	}
	actual := jsonPathValue(object, strings.Split(path, "."))
	if actual != expected {
		t.Fatalf("expected %s=%v, got %v in %s", path, expected, actual, raw)
	}
}

func jsonPathValue(value any, parts []string) any {
	if len(parts) == 0 {
		return value
	}
	object, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	return jsonPathValue(object[parts[0]], parts[1:])
}

func requireJSONListContains(t *testing.T, value any, expected any) {
	t.Helper()

	items, ok := value.([]any)
	if !ok {
		t.Fatalf("expected JSON list, got %#v", value)
	}
	for _, item := range items {
		if item == expected {
			return
		}
	}
	t.Fatalf("expected list %#v to contain %#v", items, expected)
}

func requireRequest(t *testing.T, request BuiltRequest, method, requestURL string) {
	t.Helper()

	if request.Method != method || request.URL != requestURL {
		t.Fatalf("expected %s %s, got %s %s", method, requestURL, request.Method, request.URL)
	}
}

func decodeRequestBody(t *testing.T, request BuiltRequest) map[string]any {
	t.Helper()

	var body map[string]any
	if err := json.Unmarshal(request.Body, &body); err != nil {
		t.Fatalf("decode body %s: %v", request.Body, err)
	}
	return body
}

func requireObjectField(t *testing.T, body map[string]any, field string) map[string]any {
	t.Helper()

	object, ok := body[field].(map[string]any)
	if !ok {
		t.Fatalf("expected object field %s, got %#v in %+v", field, body[field], body)
	}
	return object
}

func requireBodyField(t *testing.T, body map[string]any, field string, expected any) {
	t.Helper()

	if body[field] != expected {
		t.Fatalf("expected body[%s]=%#v, got %#v in %+v", field, expected, body[field], body)
	}
}

func requireNoBodyField(t *testing.T, body map[string]any, field string) {
	t.Helper()

	if _, ok := body[field]; ok {
		t.Fatalf("expected body to omit %s, got %+v", field, body)
	}
}

func requireStringListField(t *testing.T, body map[string]any, field string, expected []string) {
	t.Helper()

	items, ok := body[field].([]any)
	if !ok {
		t.Fatalf("expected list field %s, got %#v in %+v", field, body[field], body)
	}
	if len(items) != len(expected) {
		t.Fatalf("expected %s length %d, got %d: %#v", field, len(expected), len(items), items)
	}
	for i, expectedItem := range expected {
		if items[i] != expectedItem {
			t.Fatalf("expected %s[%d]=%q, got %#v", field, i, expectedItem, items[i])
		}
	}
}

func requireNumberListField(t *testing.T, body map[string]any, field string, expected []float64) {
	t.Helper()

	items, ok := body[field].([]any)
	if !ok {
		t.Fatalf("expected list field %s, got %#v in %+v", field, body[field], body)
	}
	if len(items) != len(expected) {
		t.Fatalf("expected %s length %d, got %d: %#v", field, len(expected), len(items), items)
	}
	for i, expectedItem := range expected {
		if items[i] != expectedItem {
			t.Fatalf("expected %s[%d]=%v, got %#v", field, i, expectedItem, items[i])
		}
	}
}

func fmtString(value any) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(strings.ReplaceAll(strings.Trim(strings.TrimSpace(jsonString(value)), `"`), `\"`, `"`))
}

func jsonString(value any) string {
	raw, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(raw)
}
