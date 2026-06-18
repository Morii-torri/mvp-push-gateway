package provider

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
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
		ProviderWeComApp,
		ProviderFeishuRobot,
		ProviderDingTalkWork,
		ProviderEmail,
		ProviderAliyunSMS,
		ProviderSelf,
		ProviderWebhook,
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
		{ProviderPushPlus, "html", "pushplus_token"},
		{ProviderWxPusher, "html", "wxpusher_uid"},
		{ProviderServerChan, "markdown", "serverchan_sendkey"},
		{ProviderEmail, "email", "email"},
		{ProviderAliyunSMS, "sms_template", "mobile"},
		{ProviderTencentSMS, "sms_template", "mobile"},
		{ProviderBaiduSMS, "sms_template", "mobile"},
		{ProviderWeComRobot, "text", "wecom_robot_key"},
		{ProviderWeComApp, "text", "wecom_userid"},
		{ProviderDingTalkRobot, "markdown", "dingtalk_robot_access_token"},
		{ProviderDingTalkWork, "sampleMarkdown", "dingtalk_userid"},
		{ProviderFeishuRobot, "text", "feishu_open_id"},
		{ProviderFeishuGroup, "text", "feishu_webhook_token"},
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

func TestEmailCapabilityUsesProviderPresetsAndEncryptedSecurity(t *testing.T) {
	capability := findCapability(t, ProviderEmail, "email")

	assertJSONField(t, capability.CredentialSchema, "properties.service_provider.default", "custom")
	assertJSONField(t, capability.CredentialSchema, "properties.service_provider.enum_labels.aliyun_qiye", "阿里企业邮箱")
	assertJSONField(t, capability.CredentialSchema, "properties.host.default", "")
	assertJSONField(t, capability.CredentialSchema, "properties.port.default", float64(465))
	assertJSONField(t, capability.CredentialSchema, "properties.security.default", "SSL")
	assertJSONField(t, capability.CredentialSchema, "properties.secure", nil)
	assertJSONField(t, capability.CredentialSchema, "properties.start_tls", nil)
	assertJSONField(t, capability.CredentialSchema, "properties.password.title", "授权码 / 密码")
	assertJSONField(t, capability.ChannelConfigSchema, "properties.from.title", "发件人显示名")
	assertJSONField(t, capability.ChannelConfigSchema, "properties.cc.title", "抄送收件人地址")
	assertJSONField(t, capability.ChannelConfigSchema, "properties.bcc.title", "密送收件人地址")
	assertJSONField(t, capability.ChannelConfigSchema, "properties.reply_to.title", "指定回复地址")

	var schema struct {
		Required   []string `json:"required"`
		Properties map[string]struct {
			Enum []string `json:"enum"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(capability.CredentialSchema, &schema); err != nil {
		t.Fatalf("decode email credential schema: %v", err)
	}
	required := map[string]bool{}
	for _, item := range schema.Required {
		required[item] = true
	}
	for _, key := range []string{"service_provider", "host", "port", "security", "username", "password"} {
		if !required[key] {
			t.Fatalf("expected email credential schema to require %q, got %#v", key, schema.Required)
		}
	}
	if got := schema.Properties["security"].Enum; len(got) != 2 || got[0] != "SSL" || got[1] != "STARTTLS" {
		t.Fatalf("expected encrypted SMTP security options only, got %#v", got)
	}
}

func TestListChannelsReportsExpiredPersistentTokenCache(t *testing.T) {
	now := time.Date(2026, 5, 27, 16, 0, 0, 0, time.UTC)
	channel := Channel{
		ID:           "89a72d77-e0a8-43e5-bc50-edb3a7a114a0",
		ProviderType: ProviderWeComApp,
		AuthConfig:   json.RawMessage(`{"corpid":"corp-1","corpsecret":"secret-1"}`),
		SendConfig:   json.RawMessage(`{}`),
	}
	capability := findCapability(t, ProviderWeComApp, "text")
	resolver, _, err := DecodeCapabilityResolver(capability.TokenStrategy, channel)
	if err != nil {
		t.Fatalf("decode token resolver: %v", err)
	}
	store := channelTokenCacheStore{
		singleChannelStore:    singleChannelStore{channel: channel},
		memoryTokenCacheStore: newMemoryTokenCacheStore(),
	}
	if err := store.StoreTokenCache(context.Background(), StoreTokenCacheParams{
		ProviderType:   ProviderWeComApp,
		Strategy:       "client_credentials",
		CacheKey:       TokenResolverCacheKey(resolver),
		ChannelID:      channel.ID,
		TokenURL:       resolver.Request.URL,
		Token:          "token-from-db",
		ExpiresAt:      now.Add(-time.Minute),
		RefreshAfterAt: now.Add(-6 * time.Minute),
		RefreshedAt:    now.Add(-2 * time.Hour),
		Metadata:       json.RawMessage(`{}`),
	}); err != nil {
		t.Fatalf("store token cache: %v", err)
	}
	service := NewService(store)
	service.tokenManager = NewTokenManager(store, WithTokenManagerNow(func() time.Time { return now }))

	channels, err := service.ListChannels(context.Background())
	if err != nil {
		t.Fatalf("list channels: %v", err)
	}
	if len(channels) != 1 {
		t.Fatalf("expected one channel, got %d", len(channels))
	}
	if channels[0].IsCached {
		t.Fatalf("expired token must not be reported usable")
	}
	if channels[0].TokenCacheStatus != "expired" {
		t.Fatalf("expected expired token cache status, got %q", channels[0].TokenCacheStatus)
	}
	if channels[0].TokenRefreshedAt == "" || channels[0].TokenExpiresAt == "" {
		t.Fatalf("expected persisted token timestamps, got refreshed=%q expires=%q", channels[0].TokenRefreshedAt, channels[0].TokenExpiresAt)
	}
}

func TestCreateAndUpdateChannelPreserveTrimmedDescription(t *testing.T) {
	store := &recordingChannelStore{}
	service := NewService(store)

	created, err := service.CreateChannel(context.Background(), CreateChannelInput{
		ProviderType:     ProviderBark,
		Name:             " Bark ",
		Description:      "  值班告警主通道  ",
		ConcurrencyLimit: 1,
		TimeoutMS:        1000,
	})
	if err != nil {
		t.Fatalf("create channel: %v", err)
	}
	if store.createInput.Description != "值班告警主通道" || created.Description != "值班告警主通道" {
		t.Fatalf("expected trimmed create description to round trip, input=%q created=%q", store.createInput.Description, created.Description)
	}

	updated, err := service.UpdateChannel(context.Background(), "channel-1", UpdateChannelInput{
		ProviderType:     ProviderBark,
		Name:             "Bark",
		Description:      " ",
		ConcurrencyLimit: 1,
		TimeoutMS:        1000,
	})
	if err != nil {
		t.Fatalf("update channel: %v", err)
	}
	if store.updateInput.Description != "" || updated.Description != "" {
		t.Fatalf("expected blank description to remain blank, input=%q updated=%q", store.updateInput.Description, updated.Description)
	}
}

func TestLegacyCompatibilityProviderTypesAreUnsupported(t *testing.T) {
	for _, providerType := range []ProviderType{"sms", "wecom", "dingtalk", "feishu", "gov_cloud", "custom_token"} {
		if validProviderType(providerType) {
			t.Fatalf("expected removed provider type %s to be unsupported", providerType)
		}
	}
}

func TestFeishuRobotCapabilityUsesTenantTokenAndOpenID(t *testing.T) {
	capability := findCapability(t, ProviderFeishuRobot, "text")

	if capability.TokenLocation != PlacementHeader || capability.TokenFieldName != "Authorization" {
		t.Fatalf("expected Feishu token in Authorization header, got location=%q field=%q", capability.TokenLocation, capability.TokenFieldName)
	}
	if !capability.RecipientRequired || capability.AllowNoRecipient || capability.IdentityKind != "feishu_open_id" {
		t.Fatalf("expected required Feishu open_id recipient, got required=%v allow=%v identity=%q", capability.RecipientRequired, capability.AllowNoRecipient, capability.IdentityKind)
	}
	assertJSONField(t, capability.CredentialSchema, "properties.app_id.type", "string")
	assertJSONField(t, capability.CredentialSchema, "properties.app_secret.format", "password")
	assertJSONField(t, capability.ChannelConfigSchema, "properties.base_url.default", "https://open.feishu.cn/open-apis")
	assertJSONField(t, capability.TokenStrategy, "strategy", "tenant_access_token")
	assertJSONField(t, capability.TokenStrategy, "request.method", "POST")
	assertJSONField(t, capability.TokenStrategy, "response_token_path", "tenant_access_token")
	assertJSONField(t, capability.TokenStrategy, "response_expires_in_path", "expire")
	assertJSONField(t, capability.TokenStrategy, "placement.location", "header")
	assertJSONField(t, capability.TokenStrategy, "placement.field_name", "Authorization")
	assertJSONField(t, capability.TokenStrategy, "placement.prefix", "Bearer ")
	assertJSONField(t, capability.SendAPI, "url", "https://open.feishu.cn/open-apis/im/v1/messages")
	var strategy struct {
		Request struct {
			BodyFields []string `json:"body_fields"`
		} `json:"request"`
	}
	if err := json.Unmarshal(capability.TokenStrategy, &strategy); err != nil {
		t.Fatalf("decode token strategy: %v", err)
	}
	if strings.Join(strategy.Request.BodyFields, ",") != "app_id,app_secret" {
		t.Fatalf("expected app_id/app_secret body fields, got %+v", strategy.Request.BodyFields)
	}
}

func TestFeishuGroupCapabilityUsesWebhookTokenRecipientAndOptionalSignSecret(t *testing.T) {
	capability := findCapability(t, ProviderFeishuGroup, "text")

	if !capability.RecipientRequired || capability.AllowNoRecipient || capability.IdentityKind != "feishu_webhook_token" {
		t.Fatalf("expected required Feishu webhook token recipient, got required=%v allow=%v identity=%q", capability.RecipientRequired, capability.AllowNoRecipient, capability.IdentityKind)
	}
	if capability.RecipientFieldName != "token" || capability.RecipientLocation != PlacementPath {
		t.Fatalf("expected Feishu group token in path, got field=%q location=%q", capability.RecipientFieldName, capability.RecipientLocation)
	}
	if capability.TokenLocation != PlacementNone {
		t.Fatalf("expected Feishu group to have no access token placement, got %q", capability.TokenLocation)
	}
	assertJSONField(t, capability.CredentialSchema, "properties.sign_secret.format", "password")
	assertJSONField(t, capability.ChannelConfigSchema, "properties.base_url.default", "https://open.feishu.cn/open-apis")
	assertJSONField(t, capability.MessageSchema, "properties.msgtype.default", "text")
	assertJSONField(t, capability.MessageSchema, "properties.text.type", "string")
	assertJSONField(t, capability.SendAPI, "url", "https://open.feishu.cn/open-apis/bot/v2/hook/{token}")
}

func TestServiceResolvesFeishuOpenIDByMobileWithTenantToken(t *testing.T) {
	var tokenRequests int32
	var resolveRequests int32
	var resolveAuthHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/open-apis/auth/v3/tenant_access_token/internal":
			atomic.AddInt32(&tokenRequests, 1)
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode token body: %v", err)
			}
			if body["app_id"] != "cli_resolve" || body["app_secret"] != "secret-resolve" {
				t.Fatalf("unexpected token body: %+v", body)
			}
			_, _ = w.Write([]byte(`{"code":0,"msg":"ok","tenant_access_token":"tenant-resolve","expire":7200}`))
		case "/open-apis/contact/v3/users/batch_get_id":
			atomic.AddInt32(&resolveRequests, 1)
			resolveAuthHeader = r.Header.Get("Authorization")
			if r.URL.Query().Get("user_id_type") != "open_id" {
				t.Fatalf("expected open_id user_id_type, got %s", r.URL.RawQuery)
			}
			var body struct {
				Mobiles         []string `json:"mobiles"`
				IncludeResigned bool     `json:"include_resigned"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode resolve body: %v", err)
			}
			if strings.Join(body.Mobiles, ",") != "13011111111" || body.IncludeResigned {
				t.Fatalf("unexpected resolve body: %+v", body)
			}
			_, _ = w.Write([]byte(`{"code":0,"msg":"ok","data":{"user_list":[{"mobile":"13011111111","user_id":"ou_resolved"}]}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	service := NewService(singleChannelStore{channel: Channel{
		ID:           "channel-feishu-resolve",
		ProviderType: ProviderFeishuRobot,
		AuthConfig:   json.RawMessage(`{"app_id":"cli_resolve","app_secret":"secret-resolve"}`),
		SendConfig:   json.RawMessage(`{"base_url":"` + server.URL + `/open-apis"}`),
		TimeoutMS:    1000,
	}}, WithProviderHTTPClientFactory(localTestHTTPClientFactory))

	result, err := service.ResolveFeishuOpenID(context.Background(), "channel-feishu-resolve", []string{"13011111111"})
	if err != nil {
		t.Fatalf("resolve Feishu open id: %v", err)
	}
	if tokenRequests != 1 || resolveRequests != 1 {
		t.Fatalf("expected one token request and one resolve request, got token=%d resolve=%d", tokenRequests, resolveRequests)
	}
	if resolveAuthHeader != "Bearer tenant-resolve" {
		t.Fatalf("expected bearer token header, got %q", resolveAuthHeader)
	}
	if !result.Success || len(result.Items) != 1 {
		t.Fatalf("unexpected resolve result: %+v", result)
	}
	if item := result.Items[0]; item.Mobile != "13011111111" || item.OpenID != "ou_resolved" || item.Status != "resolved" {
		t.Fatalf("unexpected resolve item: %+v", item)
	}
}

func TestServiceResolvesDingTalkUserIDByQueryWordWithAppToken(t *testing.T) {
	var tokenRequests int32
	var resolveRequests int32
	var resolveTokenHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1.0/oauth2/ding-corp/token":
			atomic.AddInt32(&tokenRequests, 1)
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode token body: %v", err)
			}
			if body["client_id"] != "ding-client" || body["client_secret"] != "secret-app" || body["grant_type"] != "client_credentials" {
				t.Fatalf("unexpected token body: %+v", body)
			}
			_, _ = w.Write([]byte(`{"accessToken":"ding-resolve-token","expireIn":7200}`))
		case "/v1.0/contact/users/search":
			atomic.AddInt32(&resolveRequests, 1)
			resolveTokenHeader = r.Header.Get("x-acs-dingtalk-access-token")
			var body struct {
				QueryWord      string `json:"queryWord"`
				Offset         int    `json:"offset"`
				Size           int    `json:"size"`
				FullMatchField int    `json:"fullMatchField"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode resolve body: %v", err)
			}
			if body.QueryWord != "张三" || body.Offset != 0 || body.Size != 10 || body.FullMatchField != 1 {
				t.Fatalf("unexpected resolve body: %+v", body)
			}
			_, _ = w.Write([]byte(`{"hasMore":false,"totalCount":1,"list":["093102391140051902"]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	service := NewService(singleChannelStore{channel: Channel{
		ID:           "channel-dingtalk-resolve",
		ProviderType: ProviderDingTalkWork,
		AuthConfig:   json.RawMessage(`{"corp_id":"ding-corp","client_id":"ding-client","client_secret":"secret-app","token_base_url":"` + server.URL + `"}`),
		SendConfig:   json.RawMessage(`{"base_url":"` + server.URL + `","robot_code":"ding_app"}`),
		TimeoutMS:    1000,
	}}, WithProviderHTTPClientFactory(localTestHTTPClientFactory))

	result, err := service.ResolveDingTalkUserID(context.Background(), "channel-dingtalk-resolve", []string{"张三"})
	if err != nil {
		t.Fatalf("resolve DingTalk user id: %v", err)
	}
	if tokenRequests != 1 || resolveRequests != 1 {
		t.Fatalf("expected one token request and one resolve request, got token=%d resolve=%d", tokenRequests, resolveRequests)
	}
	if resolveTokenHeader != "ding-resolve-token" {
		t.Fatalf("expected DingTalk token header, got %q", resolveTokenHeader)
	}
	if !result.Success || len(result.Items) != 1 {
		t.Fatalf("unexpected resolve result: %+v", result)
	}
	if item := result.Items[0]; item.QueryWord != "张三" || item.UserID != "093102391140051902" || item.Status != "resolved" {
		t.Fatalf("unexpected resolve item: %+v", item)
	}
	channels, err := service.ListChannels(context.Background())
	if err != nil {
		t.Fatalf("list channels after resolving DingTalk user id: %v", err)
	}
	if len(channels) != 1 || !channels[0].IsCached || channels[0].TokenCacheStatus != "cached" {
		t.Fatalf("expected DingTalk channel token cache to be reported cached after user id resolve, got %+v", channels)
	}
}

func TestServiceDingTalkUserIDResolveMarksMultipleMatches(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1.0/oauth2/ding-corp/token":
			_, _ = w.Write([]byte(`{"access_token":"ding-resolve-token","expires_in":7200}`))
		case "/v1.0/contact/users/search":
			_, _ = w.Write([]byte(`{"hasMore":false,"totalCount":2,"list":["u1","u2"]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	service := NewService(singleChannelStore{channel: Channel{
		ID:           "channel-dingtalk-multiple",
		ProviderType: ProviderDingTalkWork,
		AuthConfig:   json.RawMessage(`{"corp_id":"ding-corp","client_id":"ding-client","client_secret":"secret-app","token_base_url":"` + server.URL + `"}`),
		SendConfig:   json.RawMessage(`{"base_url":"` + server.URL + `","robot_code":"ding_app"}`),
		TimeoutMS:    1000,
	}}, WithProviderHTTPClientFactory(localTestHTTPClientFactory))

	result, err := service.ResolveDingTalkUserID(context.Background(), "channel-dingtalk-multiple", []string{"张三"})
	if err != nil {
		t.Fatalf("resolve DingTalk user id: %v", err)
	}
	if result.Success || len(result.Items) != 1 {
		t.Fatalf("expected unresolved multiple result, got %+v", result)
	}
	if item := result.Items[0]; item.Status != "multiple" || !strings.Contains(item.Error, "检测到多个用户") {
		t.Fatalf("unexpected multiple item: %+v", item)
	}
}

func TestBuildRequestForTokenManagedProviderIgnoresCallerToken(t *testing.T) {
	var tokenRequests int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenRequests++
		_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok","access_token":"backend-token","expires_in":7200}`))
	}))
	defer server.Close()

	service := NewService(singleChannelStore{channel: Channel{
		ID:           "channel-wecom",
		ProviderType: ProviderWeComApp,
		AuthConfig:   json.RawMessage(`{"corpid":"corp-1","corpsecret":"secret-1","token_url":"` + server.URL + `/token"}`),
		SendConfig:   json.RawMessage(`{"agentid":1000002}`),
		TimeoutMS:    1000,
	}}, WithProviderHTTPClientFactory(localTestHTTPClientFactory))

	request, err := service.BuildRequest(context.Background(), "channel-wecom", BuildRequestInput{
		Token:     "caller-token",
		Recipient: "u1",
		Body:      json.RawMessage(`{"content":"hello"}`),
	})
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if tokenRequests != 1 {
		t.Fatalf("expected backend token resolver to be called once, got %d", tokenRequests)
	}
	parsed, err := url.Parse(request.URL)
	if err != nil {
		t.Fatalf("parse request URL: %v", err)
	}
	if got := parsed.Query().Get("access_token"); got != "backend-token" {
		t.Fatalf("expected backend token to be used, got %q in %s", got, request.URL)
	}
	if strings.Contains(request.URL, "caller-token") {
		t.Fatalf("caller supplied token must not be used in token-managed request: %s", request.URL)
	}
}

func TestPushPlusCapabilityUsesSendJsonContentFields(t *testing.T) {
	capability := findCapability(t, ProviderPushPlus, "html")

	assertJSONField(t, capability.SendAPI, "url", "https://www.pushplus.plus/send")
	assertJSONField(t, capability.SendAPI, "content_type", "application/json")
	assertJSONField(t, capability.MessageSchema, "properties.content.type", "string")
	assertJSONField(t, capability.MessageSchema, "properties.title.type", "string")
	assertJSONField(t, capability.MessageSchema, "properties.topic.type", "string")
	assertJSONField(t, capability.CredentialSchema, "properties.token", nil)
	if capability.IdentityKind != "pushplus_token" {
		t.Fatalf("expected PushPlus recipient identity kind pushplus_token, got %q", capability.IdentityKind)
	}

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

func TestServerChanCapabilityUsesV3URLJsonAndMarkdownFields(t *testing.T) {
	capability := findCapability(t, ProviderServerChan, "markdown")

	assertJSONField(t, capability.SendAPI, "url_pattern", "https://<uid>.push.ft07.com/send/<sendkey>.send")
	assertJSONField(t, capability.SendAPI, "content_type", "application/json")
	assertJSONField(t, capability.ChannelConfigSchema, "properties.url.type", "string")
	assertJSONField(t, capability.ChannelConfigSchema, "properties.url.default", "https://<uid>.push.ft07.com/send/<sendkey>.send")
	assertJSONField(t, capability.MessageSchema, "properties.title.type", "string")
	assertJSONField(t, capability.MessageSchema, "properties.desp.format_hint", "支持 Markdown")
	assertJSONField(t, capability.MessageSchema, "properties.short.type", "string")

	var credentialSchema struct {
		Properties map[string]any `json:"properties"`
	}
	if err := json.Unmarshal(capability.CredentialSchema, &credentialSchema); err != nil {
		t.Fatalf("decode credential schema: %v", err)
	}
	if _, ok := credentialSchema.Properties["send_key"]; ok {
		t.Fatalf("ServerChan v3 should only ask for URL in channel config, got credential schema %s", capability.CredentialSchema)
	}
	if capability.IdentityKind != "serverchan_sendkey" {
		t.Fatalf("expected ServerChan recipient identity kind serverchan_sendkey, got %q", capability.IdentityKind)
	}
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

func TestPushMeCapabilityUsesPostKeyOnlyAndTypedContent(t *testing.T) {
	capability := findCapability(t, ProviderPushMe, "notice")

	assertJSONField(t, capability.SendAPI, "method", "POST")
	assertJSONField(t, capability.SendAPI, "content_type", "application/json")
	assertJSONField(t, capability.CredentialSchema, "properties.server_url.title", "服务地址")
	assertJSONField(t, capability.CredentialSchema, "properties.server_url.default", "https://push.i-i.me")
	assertJSONField(t, capability.CredentialSchema, "properties.push_key", nil)
	assertJSONField(t, capability.CredentialSchema, "properties.temp_key", nil)
	assertJSONField(t, capability.ChannelConfigSchema, "properties.type", nil)
	assertJSONField(t, capability.ChannelConfigSchema, "properties.method", nil)
	assertJSONField(t, capability.MessageSchema, "properties.title.type", "string")
	assertJSONField(t, capability.MessageSchema, "properties.content.type", "string")
	assertJSONField(t, capability.MessageSchema, "properties.body", nil)
	assertJSONField(t, capability.MessageSchema, "properties.format", nil)

	var tokenStrategy struct {
		SupportedFields []string `json:"supported_fields"`
	}
	if err := json.Unmarshal(capability.TokenStrategy, &tokenStrategy); err != nil {
		t.Fatalf("decode pushme token strategy: %v", err)
	}
	if got, want := strings.Join(tokenStrategy.SupportedFields, ","), "push_key"; got != want {
		t.Fatalf("expected PushMe token strategy to support only push_key, got %+v", tokenStrategy.SupportedFields)
	}
	if capability.IdentityKind != "pushme_push_key" {
		t.Fatalf("expected PushMe recipient identity kind pushme_push_key, got %q", capability.IdentityKind)
	}

	var messageSchema struct {
		Required   []string `json:"required"`
		Properties struct {
			Type struct {
				Enum []string `json:"enum"`
			} `json:"type"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(capability.MessageSchema, &messageSchema); err != nil {
		t.Fatalf("decode pushme message schema: %v", err)
	}
	if got, want := strings.Join(messageSchema.Required, ","), "title,content,type"; got != want {
		t.Fatalf("expected title/content/type to be required, got %+v", messageSchema.Required)
	}
	if got, want := strings.Join(messageSchema.Properties.Type.Enum, ","), "text,markdown,html"; got != want {
		t.Fatalf("expected PushMe type enum text/markdown/html, got %+v", messageSchema.Properties.Type.Enum)
	}
}

func TestBarkCapabilityUsesFallbackDeviceKeyAndMessageFields(t *testing.T) {
	capability := findCapability(t, ProviderBark, "notice")

	var credentialSchema struct {
		Required   []string `json:"required"`
		Properties map[string]struct {
			Type string `json:"type"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(capability.CredentialSchema, &credentialSchema); err != nil {
		t.Fatalf("decode bark credential schema: %v", err)
	}
	if got, want := strings.Join(credentialSchema.Required, ","), "server_url"; got != want {
		t.Fatalf("expected only server_url to be required, got %+v", credentialSchema.Required)
	}
	if _, ok := credentialSchema.Properties["device_key"]; ok {
		t.Fatalf("Bark channel config should not expose fallback device_key; recipients provide targets")
	}
	if _, ok := credentialSchema.Properties["device_keys"]; ok {
		t.Fatalf("Bark channel config should not expose device_keys; recipients provide multiple targets")
	}
	if !capability.RecipientRequired || capability.AllowNoRecipient || capability.RecipientRequirement != "system" {
		t.Fatalf("Bark should require system recipients, got required=%v allow_no_recipient=%v requirement=%q", capability.RecipientRequired, capability.AllowNoRecipient, capability.RecipientRequirement)
	}

	var channelSchema struct {
		Properties map[string]any `json:"properties"`
	}
	if err := json.Unmarshal(capability.ChannelConfigSchema, &channelSchema); err != nil {
		t.Fatalf("decode bark channel schema: %v", err)
	}
	if len(channelSchema.Properties) != 0 {
		t.Fatalf("Bark message fields should not be channel config fields, got %+v", channelSchema.Properties)
	}

	var messageSchema struct {
		Properties struct {
			Level struct {
				Enum         []string          `json:"enum"`
				Descriptions map[string]string `json:"enum_descriptions"`
			} `json:"level"`
			Markdown struct {
				Type       string `json:"type"`
				FormatHint string `json:"format_hint"`
			} `json:"markdown"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(capability.MessageSchema, &messageSchema); err != nil {
		t.Fatalf("decode bark message schema: %v", err)
	}
	if got, want := strings.Join(messageSchema.Properties.Level.Enum, ","), "critical,active,timeSensitive,passive"; got != want {
		t.Fatalf("expected Bark level enum active/timeSensitive/passive/critical, got %+v", messageSchema.Properties.Level.Enum)
	}
	if messageSchema.Properties.Level.Descriptions["critical"] == "" ||
		messageSchema.Properties.Level.Descriptions["passive"] == "" {
		t.Fatalf("expected Bark level descriptions, got descriptions=%+v", messageSchema.Properties.Level.Descriptions)
	}
	if messageSchema.Properties.Markdown.Type != "string" || messageSchema.Properties.Markdown.FormatHint == "" {
		t.Fatalf("expected Bark markdown message field with format hint, got %+v", messageSchema.Properties.Markdown)
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
		{ProviderPushMe, "notice", "pushme_push_key", "implemented_but_not_live_tested"},
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

func TestWeComRobotCapabilityUsesWebhookURLAndRecipientKey(t *testing.T) {
	capability := findCapability(t, ProviderWeComRobot, "text")

	assertJSONField(t, capability.CredentialSchema, "properties.webhook_url.default", "https://qyapi.weixin.qq.com/cgi-bin/webhook/send")
	assertJSONField(t, capability.CredentialSchema, "properties.key", nil)
	assertJSONField(t, capability.ChannelConfigSchema, "properties.key", nil)
	assertJSONField(t, capability.MessageSchema, "properties.msgtype.default", "text")
	assertJSONField(t, capability.MessageSchema, "properties.content.type", "string")
	if capability.IdentityKind != "wecom_robot_key" || !capability.RecipientRequired || capability.AllowNoRecipient {
		t.Fatalf("expected WeCom robot key as required recipient identity, got identity=%q required=%v allow=%v", capability.IdentityKind, capability.RecipientRequired, capability.AllowNoRecipient)
	}
	if capability.RecipientFieldName != "key" || capability.RecipientLocation != PlacementQuery {
		t.Fatalf("expected WeCom robot key in query, got field=%q location=%q", capability.RecipientFieldName, capability.RecipientLocation)
	}
	if capability.TokenLocation != PlacementNone {
		t.Fatalf("expected WeCom robot to have no channel token placement, got %q", capability.TokenLocation)
	}
	assertJSONField(t, capability.SendAPI, "url", "https://qyapi.weixin.qq.com/cgi-bin/webhook/send")
}

func TestDingTalkRobotCapabilityUsesMarkdownAndRecipientAccessToken(t *testing.T) {
	capability := findCapability(t, ProviderDingTalkRobot, "markdown")
	textCapability := findCapability(t, ProviderDingTalkRobot, "text")

	assertJSONField(t, capability.CredentialSchema, "properties.webhook_url", nil)
	assertJSONField(t, capability.CredentialSchema, "properties.access_token", nil)
	assertJSONField(t, capability.CredentialSchema, "properties.secret.title", "secret")
	assertJSONField(t, capability.ChannelConfigSchema, "properties.base_url.default", "https://oapi.dingtalk.com")
	assertJSONField(t, capability.ChannelConfigSchema, "properties.isAtAll.default", false)
	assertJSONField(t, capability.MessageSchema, "properties.msgtype", nil)
	assertJSONStringListField(t, capability.MessageSchema, "field_order", []string{"title", "text"})
	assertJSONField(t, capability.MessageSchema, "properties.title.type", "string")
	assertJSONField(t, capability.MessageSchema, "properties.text.type", "string")
	assertJSONField(t, textCapability.MessageSchema, "properties.content.type", "string")
	assertJSONField(t, textCapability.MessageSchema, "properties.content.title", "content")
	if capability.IdentityKind != "dingtalk_robot_access_token" || !capability.RecipientRequired || capability.AllowNoRecipient {
		t.Fatalf("expected DingTalk robot access token as required recipient identity, got identity=%q required=%v allow=%v", capability.IdentityKind, capability.RecipientRequired, capability.AllowNoRecipient)
	}
	if capability.RecipientFieldName != "access_token" || capability.RecipientLocation != PlacementQuery {
		t.Fatalf("expected DingTalk robot access token in query, got field=%q location=%q", capability.RecipientFieldName, capability.RecipientLocation)
	}
}

func TestDingTalkWorkCapabilityUsesRobotBatchSendAndUserIDs(t *testing.T) {
	markdown := findCapability(t, ProviderDingTalkWork, "sampleMarkdown")
	text := findCapability(t, ProviderDingTalkWork, "sampleText")

	assertJSONField(t, markdown.CredentialSchema, "properties.corp_id.type", "string")
	assertJSONField(t, markdown.CredentialSchema, "properties.client_id.type", "string")
	assertJSONField(t, markdown.CredentialSchema, "properties.client_secret.format", "password")
	assertJSONField(t, markdown.ChannelConfigSchema, "properties.robot_code.type", "string")
	assertJSONField(t, markdown.ChannelConfigSchema, "properties.base_url.default", "https://api.dingtalk.com")
	assertJSONStringListField(t, markdown.MessageSchema, "field_order", []string{"title", "text"})
	assertJSONField(t, markdown.MessageSchema, "properties.title.type", "string")
	assertJSONField(t, markdown.MessageSchema, "properties.text.type", "string")
	assertJSONField(t, text.MessageSchema, "properties.content.type", "string")
	if markdown.IdentityKind != "dingtalk_userid" || markdown.RecipientFormat != "array" || markdown.RecipientFieldName != "userIds" {
		t.Fatalf("unexpected DingTalk work recipient metadata: identity=%q format=%q field=%q", markdown.IdentityKind, markdown.RecipientFormat, markdown.RecipientFieldName)
	}
	if markdown.TokenLocation != PlacementHeader || markdown.TokenFieldName != "x-acs-dingtalk-access-token" {
		t.Fatalf("expected DingTalk token in x-acs-dingtalk-access-token header, got location=%q field=%q", markdown.TokenLocation, markdown.TokenFieldName)
	}
	assertJSONField(t, markdown.TokenStrategy, "token_url", "https://api.dingtalk.com/v1.0/oauth2/{corp_id}/token")
	assertJSONField(t, markdown.TokenStrategy, "request.method", "POST")
	assertJSONField(t, markdown.TokenStrategy, "request.body.grant_type", "client_credentials")
	assertJSONField(t, markdown.TokenStrategy, "response_token_path", "accessToken|access_token")
	assertJSONField(t, markdown.TokenStrategy, "response_expires_in_path", "expireIn|expires_in")
	assertJSONField(t, markdown.TokenStrategy, "placement.location", "header")
	assertJSONField(t, markdown.TokenStrategy, "placement.field_name", "x-acs-dingtalk-access-token")
	assertJSONField(t, markdown.SendAPI, "url", "https://api.dingtalk.com/v1.0/robot/oToMessages/batchSend")
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
	assertJSONField(t, webhook.MessageSchema, "properties.body.type", "object")
	assertJSONStringListField(t, webhook.ChannelConfigSchema, "properties.method.enum", []string{"POST", "GET"})
	assertJSONField(t, webhook.SendAPI, "url_template", "{{ channel.url }}")
	assertJSONDoesNotContain(t, webhook.CredentialSchema, "note")
	assertJSONDoesNotContain(t, webhook.CredentialSchema, "secret")
	assertJSONDoesNotContain(t, webhook.MessageSchema, "payload")
	assertJSONDoesNotContain(t, webhook.MessageSchema, "headers")
	assertJSONDoesNotContain(t, webhook.ChannelConfigSchema, "recipient")
	assertJSONDoesNotContain(t, webhook.ChannelConfigSchema, `"body"`)

	wecom := findCapability(t, ProviderWeComApp, "text")
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
				AuthConfig:   json.RawMessage(`{"token":"legacy-channel-token"}`),
				SendConfig:   json.RawMessage(`{"topic":"legacy-send-topic","template":"markdown"}`),
			},
			message:    json.RawMessage(`{"title":"Disk alert","content":"Disk 95%","topic":"ops","format":"markdown","url":"https://example.test/detail"}`),
			recipients: []ResolvedRecipient{{PlatformIDs: map[string]string{"pushplus_token": "recipient-push-token"}}},
			assert: func(t *testing.T, request BuiltRequest) {
				requireRequest(t, request, "POST", "https://www.pushplus.plus/send")
				body := decodeRequestBody(t, request)
				requireBodyField(t, body, "token", "recipient-push-token")
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
				SendConfig:   json.RawMessage(`{"url":"https://<uid>.push.ft07.com/send/<sendkey>.send"}`),
			},
			message:    json.RawMessage(`{"title":"Build","desp":"**Failed**","short":"Failed"}`),
			recipients: []ResolvedRecipient{{PlatformIDs: map[string]string{"serverchan_sendkey": "sctp21329tfauqvvbhe2wpeb5lufz4gz"}}},
			assert: func(t *testing.T, request BuiltRequest) {
				requireRequest(t, request, "POST", "https://21329.push.ft07.com/send/sctp21329tfauqvvbhe2wpeb5lufz4gz.send")
				if request.Headers["Content-Type"] != "application/json" {
					t.Fatalf("expected json content type, got %+v", request.Headers)
				}
				body := decodeRequestBody(t, request)
				requireBodyField(t, body, "title", "Build")
				requireBodyField(t, body, "desp", "**Failed**")
				requireBodyField(t, body, "short", "Failed")
				requireNoBodyField(t, body, "text")
				requireNoBodyField(t, body, "send_key")
				requireNoBodyField(t, body, "channel")
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
			name: "smtp email snapshot",
			channel: Channel{
				ProviderType: ProviderEmail,
				AuthConfig:   json.RawMessage(`{"host":"smtp.qq.com","port":465,"security":"SSL","username":"ops@qq.com","password":"app-password"}`),
				SendConfig:   json.RawMessage(`{"from":"MVP Push <ops@qq.com>","cc":["team@example.com"],"bcc":["audit@example.com"],"reply_to":"reply@example.com"}`),
			},
			message:    json.RawMessage(`{"subject":"测试标题","body":"邮件测试消息"}`),
			recipients: []ResolvedRecipient{{Email: "021120129@sues.edu.cn"}},
			assert: func(t *testing.T, request BuiltRequest) {
				requireRequest(t, request, "SMTP_SEND", "smtp://smtp.qq.com:465")
				body := decodeRequestBody(t, request)
				requireBodyField(t, body, "host", "smtp.qq.com")
				requireBodyField(t, body, "port", float64(465))
				requireBodyField(t, body, "security", "SSL")
				requireBodyField(t, body, "username", "ops@qq.com")
				requireBodyField(t, body, "from", `"MVP Push" <ops@qq.com>`)
				requireStringListField(t, body, "to", []string{"021120129@sues.edu.cn"})
				requireStringListField(t, body, "cc", []string{"team@example.com"})
				requireStringListField(t, body, "bcc", []string{"audit@example.com"})
				requireBodyField(t, body, "reply_to", "reply@example.com")
				requireBodyField(t, body, "subject", "测试标题")
				requireBodyField(t, body, "body", "邮件测试消息")
				requireBodyField(t, body, "format", "text")
				requireBodyField(t, body, "smtp_envelope_from", "ops@qq.com")
				requireBodyField(t, body, "password_configured", true)
				requireNoBodyField(t, body, "password")
				requireNoBodyField(t, body, "html")
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
				requireNoBodyField(t, text, "mentioned_list")
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
			name: "wecom app markdown",
			channel: Channel{
				ProviderType: ProviderWeComApp,
				AuthConfig:   json.RawMessage(`{"agentid":1000001}`),
			},
			token:      "wecom-token",
			message:    json.RawMessage(`{"msgtype":"markdown","markdown":"# hello"}`),
			recipients: []ResolvedRecipient{{PlatformIDs: map[string]string{"wecom_userid": "u1"}}},
			assert: func(t *testing.T, request BuiltRequest) {
				requireRequest(t, request, "POST", "https://qyapi.weixin.qq.com/cgi-bin/message/send?access_token=wecom-token")
				body := decodeRequestBody(t, request)
				requireBodyField(t, body, "msgtype", "markdown")
				markdown := requireObjectField(t, body, "markdown")
				requireBodyField(t, markdown, "content", "# hello")
			},
		},
		{
			name: "wecom app textcard",
			channel: Channel{
				ProviderType: ProviderWeComApp,
				AuthConfig:   json.RawMessage(`{"agentid":1000001}`),
			},
			token:      "wecom-token",
			message:    json.RawMessage(`{"msgtype":"textcard","title":"告警","description":"磁盘 95%","url":"https://example.test/detail","btntxt":"查看"}`),
			recipients: []ResolvedRecipient{{PlatformIDs: map[string]string{"wecom_userid": "u1"}}},
			assert: func(t *testing.T, request BuiltRequest) {
				body := decodeRequestBody(t, request)
				requireBodyField(t, body, "msgtype", "textcard")
				textcard := requireObjectField(t, body, "textcard")
				requireBodyField(t, textcard, "title", "告警")
				requireBodyField(t, textcard, "description", "磁盘 95%")
				requireBodyField(t, textcard, "url", "https://example.test/detail")
				requireBodyField(t, textcard, "btntxt", "查看")
			},
		},
		{
			name: "wecom robot recipient key markdown",
			channel: Channel{
				ProviderType: ProviderWeComRobot,
				AuthConfig:   json.RawMessage(`{"webhook_url":"https://qyapi.weixin.qq.com/cgi-bin/webhook/send"}`),
			},
			message:    json.RawMessage(`{"msgtype":"markdown","content":"# hello"}`),
			recipients: []ResolvedRecipient{{PlatformIDs: map[string]string{"wecom_robot_key": "robot-key-1"}}},
			assert: func(t *testing.T, request BuiltRequest) {
				requireRequest(t, request, "POST", "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=robot-key-1")
				body := decodeRequestBody(t, request)
				requireBodyField(t, body, "msgtype", "markdown")
				markdown := requireObjectField(t, body, "markdown")
				requireBodyField(t, markdown, "content", "# hello")
				requireNoBodyField(t, body, "text")
			},
		},
		{
			name: "dingtalk robot markdown",
			channel: Channel{
				ProviderType: ProviderDingTalkRobot,
				AuthConfig:   json.RawMessage(`{"secret":"SEC123"}`),
				SendConfig:   json.RawMessage(`{"base_url":"https://oapi.dingtalk.com","isAtAll":false}`),
			},
			message:    json.RawMessage(`{"title":"Notice","text":"## hello ding\n - item"}`),
			recipients: []ResolvedRecipient{{PlatformIDs: map[string]string{"dingtalk_robot_access_token": "robot-token"}}},
			assert: func(t *testing.T, request BuiltRequest) {
				if request.Method != "POST" {
					t.Fatalf("expected POST, got %s", request.Method)
				}
				parsed, err := url.Parse(request.URL)
				if err != nil {
					t.Fatalf("parse request url %q: %v", request.URL, err)
				}
				if parsed.Scheme+"://"+parsed.Host+parsed.Path != "https://oapi.dingtalk.com/robot/send" {
					t.Fatalf("unexpected request URL path: %s", request.URL)
				}
				query := parsed.Query()
				if query.Get("access_token") != "robot-token" || query.Get("timestamp") == "" || query.Get("sign") == "" {
					t.Fatalf("expected access_token, timestamp and sign query params, got %s", request.URL)
				}
				body := decodeRequestBody(t, request)
				requireBodyField(t, body, "msgtype", "markdown")
				markdown := requireObjectField(t, body, "markdown")
				requireBodyField(t, markdown, "title", "Notice")
				requireBodyField(t, markdown, "text", "## hello ding\n - item")
				at := requireObjectField(t, body, "at")
				requireBodyField(t, at, "isAtAll", false)
				requireNoBodyField(t, at, "atMobiles")
				requireNoBodyField(t, body, "text")
			},
		},
		{
			name: "dingtalk robot text",
			channel: Channel{
				ProviderType: ProviderDingTalkRobot,
				SendConfig:   json.RawMessage(`{"base_url":"https://oapi.dingtalk.com","isAtAll":true}`),
			},
			message:    json.RawMessage(`{"content":"我就是我"}`),
			recipients: []ResolvedRecipient{{PlatformIDs: map[string]string{"dingtalk_robot_access_token": "robot-token"}}},
			assert: func(t *testing.T, request BuiltRequest) {
				requireRequest(t, request, "POST", "https://oapi.dingtalk.com/robot/send?access_token=robot-token")
				body := decodeRequestBody(t, request)
				requireBodyField(t, body, "msgtype", "text")
				text := requireObjectField(t, body, "text")
				requireBodyField(t, text, "content", "我就是我")
				at := requireObjectField(t, body, "at")
				requireBodyField(t, at, "isAtAll", true)
				requireNoBodyField(t, body, "markdown")
			},
		},
		{
			name: "dingtalk work markdown",
			channel: Channel{
				ProviderType: ProviderDingTalkWork,
				SendConfig:   json.RawMessage(`{"base_url":"https://api.dingtalk.com","robot_code":"dingia0tfyhohgzi1zci"}`),
			},
			token:      "ding-token",
			message:    json.RawMessage(`{"title":"hello title","text":"hello text"}`),
			recipients: []ResolvedRecipient{{PlatformIDs: map[string]string{"dingtalk_userid": "u1"}}, {PlatformIDs: map[string]string{"dingtalk_userid": "u2"}}},
			assert: func(t *testing.T, request BuiltRequest) {
				requireRequest(t, request, "POST", "https://api.dingtalk.com/v1.0/robot/oToMessages/batchSend")
				if request.Headers["x-acs-dingtalk-access-token"] != "ding-token" {
					t.Fatalf("expected DingTalk access token header, got headers=%v", request.Headers)
				}
				body := decodeRequestBody(t, request)
				requireBodyField(t, body, "robotCode", "dingia0tfyhohgzi1zci")
				requireStringListField(t, body, "userIds", []string{"u1", "u2"})
				requireBodyField(t, body, "msgKey", "sampleMarkdown")
				var msgParam map[string]string
				if err := json.Unmarshal([]byte(fmt.Sprint(body["msgParam"])), &msgParam); err != nil {
					t.Fatalf("decode msgParam: %v", err)
				}
				if msgParam["title"] != "hello title" || msgParam["text"] != "hello text" {
					t.Fatalf("unexpected msgParam: %+v", msgParam)
				}
			},
		},
		{
			name: "dingtalk work text",
			channel: Channel{
				ProviderType: ProviderDingTalkWork,
				SendConfig:   json.RawMessage(`{"base_url":"https://api.dingtalk.com","robot_code":"ding-app"}`),
			},
			token:      "ding-token",
			message:    json.RawMessage(`{"msgKey":"sampleText","content":"hello content"}`),
			recipients: []ResolvedRecipient{{PlatformIDs: map[string]string{"dingtalk_userid": "u1"}}},
			assert: func(t *testing.T, request BuiltRequest) {
				requireRequest(t, request, "POST", "https://api.dingtalk.com/v1.0/robot/oToMessages/batchSend")
				body := decodeRequestBody(t, request)
				requireStringListField(t, body, "userIds", []string{"u1"})
				requireBodyField(t, body, "msgKey", "sampleText")
				var msgParam map[string]string
				if err := json.Unmarshal([]byte(fmt.Sprint(body["msgParam"])), &msgParam); err != nil {
					t.Fatalf("decode msgParam: %v", err)
				}
				if msgParam["content"] != "hello content" {
					t.Fatalf("unexpected msgParam: %+v", msgParam)
				}
			},
		},
		{
			name: "feishu robot",
			channel: Channel{
				ProviderType: ProviderFeishuRobot,
				TokenConfig:  json.RawMessage(`{"location":"header","field_name":"Authorization","prefix":"Bearer "}`),
				SendConfig:   json.RawMessage(`{"base_url":"https://open.feishu.cn/open-apis"}`),
			},
			token:      "tenant-token",
			message:    json.RawMessage(`{"body":"hello feishu"}`),
			recipients: []ResolvedRecipient{{PlatformIDs: map[string]string{"feishu_open_id": "ou_123"}}},
			assert: func(t *testing.T, request BuiltRequest) {
				requireRequest(t, request, "POST", "https://open.feishu.cn/open-apis/im/v1/messages?receive_id_type=open_id")
				if request.Headers["Authorization"] != "Bearer tenant-token" {
					t.Fatalf("expected bearer token header, got %+v", request.Headers)
				}
				body := decodeRequestBody(t, request)
				requireBodyField(t, body, "msg_type", "text")
				requireBodyField(t, body, "receive_id", "ou_123")
				contentString, ok := body["content"].(string)
				if !ok {
					t.Fatalf("expected serialized content string, got %#v", body["content"])
				}
				var content map[string]string
				if err := json.Unmarshal([]byte(contentString), &content); err != nil {
					t.Fatalf("decode feishu content string: %v", err)
				}
				if content["text"] != "hello feishu" {
					t.Fatalf("expected text content, got %+v", content)
				}
			},
		},
		{
			name: "feishu group message webhook",
			channel: Channel{
				ProviderType: ProviderFeishuGroup,
				AuthConfig:   json.RawMessage(`{"sign_secret":"secret"}`),
				SendConfig:   json.RawMessage(`{"base_url":"https://open.feishu.cn/open-apis"}`),
			},
			message:    json.RawMessage(`{"msgtype":"text","text":"hello group"}`),
			recipients: []ResolvedRecipient{{PlatformIDs: map[string]string{"feishu_webhook_token": "hook-token-1"}}},
			assert: func(t *testing.T, request BuiltRequest) {
				requireRequest(t, request, "POST", "https://open.feishu.cn/open-apis/bot/v2/hook/hook-token-1")
				body := decodeRequestBody(t, request)
				requireBodyField(t, body, "msg_type", "text")
				content := requireObjectField(t, body, "content")
				requireBodyField(t, content, "text", "hello group")
				if body["timestamp"] == "" || body["sign"] == "" {
					t.Fatalf("expected signed Feishu group request, got %+v", body)
				}
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
				AuthConfig:   json.RawMessage(`{"server_url":"https://bark.example"}`),
				SendConfig:   json.RawMessage(`{"group":"old-channel-group","sound":"old-channel-sound","level":"passive","icon":"https://example.test/old.png"}`),
			},
			message: json.RawMessage(`{
				"title":"Disk alert",
				"markdown":"**Disk 95%**",
				"subtitle":"prod",
				"url":"https://example.test/detail",
				"group":"ops",
				"sound":"alarm",
				"level":"critical",
				"icon":"https://example.test/icon.png",
				"image":"https://example.test/image.png"
			}`),
			recipients: []ResolvedRecipient{
				{PlatformIDs: map[string]string{"bark_device_key": "device-key-1"}},
				{PlatformIDs: map[string]string{"bark_device_key": "device-key-2"}},
			},
			assert: func(t *testing.T, request BuiltRequest) {
				requireRequest(t, request, "POST", "https://bark.example/push")
				body := decodeRequestBody(t, request)
				requireNoBodyField(t, body, "device_key")
				requireStringListField(t, body, "device_keys", []string{"device-key-1", "device-key-2"})
				requireBodyField(t, body, "title", "Disk alert")
				requireBodyField(t, body, "subtitle", "prod")
				requireNoBodyField(t, body, "body")
				requireBodyField(t, body, "markdown", "**Disk 95%**")
				requireBodyField(t, body, "url", "https://example.test/detail")
				requireBodyField(t, body, "group", "ops")
				requireBodyField(t, body, "sound", "alarm")
				requireBodyField(t, body, "level", "critical")
				requireBodyField(t, body, "icon", "https://example.test/icon.png")
				requireBodyField(t, body, "image", "https://example.test/image.png")
			},
		},
		{
			name: "pushme",
			channel: Channel{
				ProviderType: ProviderPushMe,
				AuthConfig:   json.RawMessage(`{"server_url":"https://push.example","push_key":"legacy-channel-key"}`),
				SendConfig:   json.RawMessage(`{"type":"text","method":"GET"}`),
			},
			message:    json.RawMessage(`{"title":"Build","content":"Failed","type":"html"}`),
			recipients: []ResolvedRecipient{{PlatformIDs: map[string]string{"pushme_push_key": "recipient-push-key"}}},
			assert: func(t *testing.T, request BuiltRequest) {
				requireRequest(t, request, "POST", "https://push.example")
				body := decodeRequestBody(t, request)
				if len(body) != 4 {
					t.Fatalf("expected PushMe request body to only contain push_key/title/content/type, got %#v", body)
				}
				requireBodyField(t, body, "push_key", "recipient-push-key")
				requireBodyField(t, body, "title", "Build")
				requireBodyField(t, body, "content", "Failed")
				requireBodyField(t, body, "type", "html")
				if _, ok := body["live_test_status"]; ok {
					t.Fatalf("PushMe request body must not expose internal live test metadata: %#v", body)
				}
				if _, ok := body["temp_key"]; ok {
					t.Fatalf("PushMe request body must not use temp_key: %#v", body)
				}
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

func TestBuildDeliveryRequestFallsBackToResolvedRecipientsWhenLegacyRecipientIsEmpty(t *testing.T) {
	request, err := BuildDeliveryRequest(Channel{
		ProviderType: ProviderWxPusher,
		AuthConfig:   json.RawMessage(`{"app_token":"wx-app-token"}`),
	}, BuildDeliveryRequestInput{
		LegacyRecipientValue: "",
		RenderedMessage: RenderedMessage{
			ProviderType: ProviderWxPusher,
			MessageType:  "html",
			Content:      json.RawMessage(`{"content":"<b>Finished</b>","topicIds":[101,102]}`),
		},
		ResolvedRecipients: []ResolvedRecipient{
			{PlatformIDs: map[string]string{"wxpusher_uid": "UID_1"}},
			{PlatformIDs: map[string]string{"wxpusher_uid": "UID_2"}},
		},
	})
	if err != nil {
		t.Fatalf("build request: %v", err)
	}

	body := decodeRequestBody(t, request)
	requireStringListField(t, body, "uids", []string{"UID_1", "UID_2"})
	requireNumberListField(t, body, "topicIds", []float64{101, 102})
	requireBodyField(t, body, "contentType", float64(2))
}

func TestBuildDeliveryRequestBarkRequiresRecipient(t *testing.T) {
	_, err := BuildDeliveryRequest(Channel{
		ProviderType: ProviderBark,
		AuthConfig:   json.RawMessage(`{"server_url":"https://bark.example","device_key":"channel-key"}`),
	}, BuildDeliveryRequestInput{
		RenderedMessage: RenderedMessage{
			ProviderType: ProviderBark,
			MessageType:  "text",
			Content:      json.RawMessage(`{"body":"Fallback target"}`),
		},
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected invalid input without Bark recipient, got %v", err)
	}
}

func TestBuildDeliveryRequestPersonalProvidersRequireRecipientIdentity(t *testing.T) {
	for _, tc := range []struct {
		name    string
		channel Channel
		message json.RawMessage
	}{
		{
			name:    "pushplus",
			channel: Channel{ProviderType: ProviderPushPlus, AuthConfig: json.RawMessage(`{"token":"legacy-channel-token"}`)},
			message: json.RawMessage(`{"content":"PushPlus message"}`),
		},
		{
			name:    "serverchan",
			channel: Channel{ProviderType: ProviderServerChan, SendConfig: json.RawMessage(`{"url":"https://<uid>.push.ft07.com/send/<sendkey>.send"}`)},
			message: json.RawMessage(`{"title":"ServerChan title"}`),
		},
		{
			name:    "pushme",
			channel: Channel{ProviderType: ProviderPushMe, AuthConfig: json.RawMessage(`{"server_url":"https://push.example","push_key":"legacy-channel-key"}`)},
			message: json.RawMessage(`{"title":"PushMe title","content":"PushMe content","type":"markdown"}`),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := BuildDeliveryRequest(tc.channel, BuildDeliveryRequestInput{
				RenderedMessage: RenderedMessage{
					ProviderType: tc.channel.ProviderType,
					Content:      tc.message,
				},
			})
			if !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("expected invalid input without recipient identity, got %v", err)
			}
		})
	}
}

func TestBuildDeliveryRequestUsesRenderedMessageRecipientsAndTargetContext(t *testing.T) {
	request, err := BuildDeliveryRequest(Channel{
		ID:           "channel-wecom",
		ProviderType: ProviderWeComApp,
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
			ProviderType: ProviderWeComApp,
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
			ProviderType:      string(ProviderWeComApp),
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

func TestBuildDeliveryRequestUsesWebhookIdentityHeadersAndBody(t *testing.T) {
	request, err := BuildDeliveryRequest(Channel{
		ID:           "channel-webhook",
		ProviderType: ProviderWebhook,
		Name:         "Webhook",
		SendConfig: json.RawMessage(`{
			"method":"POST",
			"url":"https://example.test/hooks/{{ identity }}",
			"headers":{"X-App-Id":"app-1","X-Token":"token-1"}
		}`),
	}, BuildDeliveryRequestInput{
		RenderedMessage: RenderedMessage{
			ProviderType: ProviderWebhook,
			MessageType:  "json",
			Content:      json.RawMessage(`{"body":{"title":"告警标题","level":"critical","content":"告警内容","biz_id":"order-10001"}}`),
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
	if request.Method != "POST" {
		t.Fatalf("expected POST method, got %s", request.Method)
	}
	if request.URL != "https://example.test/hooks/ops%20room" {
		t.Fatalf("expected identity placeholder in URL, got %s", request.URL)
	}
	if request.Headers["X-App-Id"] != "app-1" || request.Headers["X-Token"] != "token-1" {
		t.Fatalf("expected configured webhook headers, got %+v", request.Headers)
	}
	if request.Headers["Content-Type"] != "application/json" {
		t.Fatalf("expected JSON content type, got %+v", request.Headers)
	}
	var body map[string]any
	if err := json.Unmarshal(request.Body, &body); err != nil {
		t.Fatalf("decode adapter body: %v", err)
	}
	if body["title"] != "告警标题" || body["level"] != "critical" || body["content"] != "告警内容" || body["biz_id"] != "order-10001" {
		t.Fatalf("expected raw template body object, got %+v", body)
	}
	if _, ok := body["body"]; ok {
		t.Fatalf("webhook request body must not keep the template wrapper: %+v", body)
	}
	if _, ok := body["payload"]; ok {
		t.Fatalf("webhook request body must not use legacy payload wrapper: %+v", body)
	}
	if _, ok := body["headers"]; ok {
		t.Fatalf("webhook request body must not include headers wrapper: %+v", body)
	}
}

func TestBuildDeliveryRequestUsesWebhookGetBodyAsQuery(t *testing.T) {
	request, err := BuildDeliveryRequest(Channel{
		ID:           "channel-webhook",
		ProviderType: ProviderWebhook,
		Name:         "Webhook",
		SendConfig: json.RawMessage(`{
			"method":"GET",
			"url":"https://example.test/hooks/{{ identity }}?fixed=1",
			"headers":{"X-App-Id":"app-1"}
		}`),
	}, BuildDeliveryRequestInput{
		RenderedMessage: RenderedMessage{
			ProviderType: ProviderWebhook,
			MessageType:  "json",
			Content:      json.RawMessage(`{"body":{"title":"告警标题","level":"critical","tags":["ops","db"]}}`),
		},
		ResolvedRecipients: []ResolvedRecipient{{Value: "ops room"}},
	})
	if err != nil {
		t.Fatalf("build webhook GET request: %v", err)
	}
	if request.Method != "GET" {
		t.Fatalf("expected GET method, got %s", request.Method)
	}
	parsed, err := url.Parse(request.URL)
	if err != nil {
		t.Fatalf("parse request URL: %v", err)
	}
	values := parsed.Query()
	if parsed.Scheme != "https" || parsed.Host != "example.test" || parsed.Path != "/hooks/ops room" {
		t.Fatalf("unexpected request URL: %s", request.URL)
	}
	if values.Get("fixed") != "1" || values.Get("title") != "告警标题" || values.Get("level") != "critical" {
		t.Fatalf("expected webhook body fields as query params, got %s", parsed.RawQuery)
	}
	if values.Get("tags") != `["ops","db"]` {
		t.Fatalf("expected array query value to be JSON encoded, got %q", values.Get("tags"))
	}
	if len(bytes.TrimSpace(request.Body)) != 0 {
		t.Fatalf("GET webhook request must not carry a JSON body, got %s", request.Body)
	}
}

func TestBuildDeliveryRequestReplacesURLEncodedWebhookIdentity(t *testing.T) {
	request, err := BuildDeliveryRequest(Channel{
		ID:           "channel-webhook",
		ProviderType: ProviderWebhook,
		Name:         "Webhook",
		SendConfig: json.RawMessage(`{
			"method":"GET",
			"url":"https://21329.push.ft07.com/send/%7B%7B%20identity%20%7D%7D.send"
		}`),
	}, BuildDeliveryRequestInput{
		RenderedMessage: RenderedMessage{
			ProviderType: ProviderWebhook,
			MessageType:  "json",
			Content:      json.RawMessage(`{"body":{"title":"告警标题","content":"告警内容"}}`),
		},
		ResolvedRecipients: []ResolvedRecipient{{Value: "send-key-1"}},
	})
	if err != nil {
		t.Fatalf("build webhook encoded identity request: %v", err)
	}
	if strings.Contains(request.URL, "%7B%7B") || strings.Contains(request.URL, "identity") {
		t.Fatalf("expected encoded identity placeholder to be replaced, got %s", request.URL)
	}
	parsed, err := url.Parse(request.URL)
	if err != nil {
		t.Fatalf("parse request URL: %v", err)
	}
	if parsed.Path != "/send/send-key-1.send" {
		t.Fatalf("expected identity in path, got %s", parsed.Path)
	}
	if parsed.Query().Get("title") != "告警标题" || parsed.Query().Get("content") != "告警内容" {
		t.Fatalf("expected body fields as query params, got %s", parsed.RawQuery)
	}
	if len(bytes.TrimSpace(request.Body)) != 0 {
		t.Fatalf("GET webhook request must not carry body, got %s", request.Body)
	}
}

func TestBuildDeliveryRequestRejectsLegacyWebhookPlaceholders(t *testing.T) {
	_, err := BuildDeliveryRequest(Channel{
		ID:           "channel-webhook",
		ProviderType: ProviderWebhook,
		Name:         "Webhook",
		SendConfig:   json.RawMessage(`{"method":"POST","url":"https://example.test/hooks/{{ recipient }}"}`),
	}, BuildDeliveryRequestInput{
		RenderedMessage: RenderedMessage{
			ProviderType: ProviderWebhook,
			MessageType:  "json",
			Content:      json.RawMessage(`{"body":{"title":"告警标题"}}`),
		},
		ResolvedRecipients: []ResolvedRecipient{{Value: "ops room"}},
	})
	if err == nil {
		t.Fatal("expected legacy webhook placeholder to be rejected")
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
				"headers":{"X-Test":"dry-run"}
			}`),
			TimeoutMS: 1000,
		},
	})

	result, err := service.TestSend(context.Background(), "channel-webhook", TestSendInput{
		BuildRequestInput: BuildRequestInput{
			Body: json.RawMessage(`{"body":{"title":"dry run","content":"只生成请求"}}`),
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

func TestTestSendDryRunBuildsWebhookIdentityRequest(t *testing.T) {
	service := NewService(singleChannelStore{
		channel: Channel{
			ID:           "channel-webhook",
			ProviderType: ProviderWebhook,
			Name:         "Webhook",
			SendConfig: json.RawMessage(`{
				"method":"POST",
				"url":"https://21329.push.ft07.com/send/{{ identity }}.send"
			}`),
			TimeoutMS: 1000,
		},
	})

	result, err := service.TestSend(context.Background(), "channel-webhook", TestSendInput{
		BuildRequestInput: BuildRequestInput{
			Recipient: "send-key-1",
			Body:      json.RawMessage(`{"body":{"title":"告警标题","content":"告警内容"}}`),
		},
	})
	if err != nil {
		t.Fatalf("dry-run webhook test send: %v", err)
	}

	var snapshot map[string]any
	if err := json.Unmarshal(result.RequestSnapshot, &snapshot); err != nil {
		t.Fatalf("decode dry-run snapshot: %v", err)
	}
	finalRequest, ok := snapshot["final_request"].(map[string]any)
	if !ok {
		t.Fatalf("expected final_request object, got %+v", snapshot["final_request"])
	}
	if finalRequest["url"] != "https://21329.push.ft07.com/send/send-key-1.send" || finalRequest["method"] != "POST" {
		t.Fatalf("expected identity-substituted webhook request, got %+v", finalRequest)
	}
	body, ok := finalRequest["body"].(map[string]any)
	if !ok {
		t.Fatalf("expected final_request body object, got %+v", finalRequest["body"])
	}
	if body["title"] != "告警标题" || body["content"] != "告警内容" {
		t.Fatalf("expected unwrapped webhook body, got %+v", body)
	}
	if _, ok := body["body"]; ok {
		t.Fatalf("webhook test-send body must not keep wrapper: %+v", body)
	}
}

func TestTestSendDryRunBuildsEmailSMTPSnapshot(t *testing.T) {
	service := NewService(singleChannelStore{
		channel: Channel{
			ID:           "channel-email",
			ProviderType: ProviderEmail,
			Name:         "QQ email",
			AuthConfig:   json.RawMessage(`{"host":"smtp.qq.com","port":465,"security":"SSL","username":"ops@qq.com","password":"app-password"}`),
			SendConfig:   json.RawMessage(`{"from":"MVP Push <ops@qq.com>"}`),
			TimeoutMS:    1000,
		},
	})

	result, err := service.TestSend(context.Background(), "channel-email", TestSendInput{
		BuildRequestInput: BuildRequestInput{
			Recipient: "021120129@sues.edu.cn",
			Body:      json.RawMessage(`{"subject":"邮件测试标题","body":"邮件测试消息"}`),
		},
	})
	if err != nil {
		t.Fatalf("dry-run email test send: %v", err)
	}
	if result.Status != "dry_run" {
		t.Fatalf("expected dry_run status, got %q", result.Status)
	}
	if result.Request.Method != "SMTP_SEND" || result.Request.URL != "smtp://smtp.qq.com:465" {
		t.Fatalf("expected SMTP dry-run request, got %s %s", result.Request.Method, result.Request.URL)
	}
	body := decodeRequestBody(t, result.Request)
	requireBodyField(t, body, "subject", "邮件测试标题")
	requireBodyField(t, body, "body", "邮件测试消息")
	requireStringListField(t, body, "to", []string{"021120129@sues.edu.cn"})
	requireNoBodyField(t, body, "password")
}

func TestRedactBuiltRequestMasksSensitiveBodyFields(t *testing.T) {
	redacted := RedactBuiltRequest(BuiltRequest{
		Method: "POST",
		URL:    "https://example.test/send?access_token=query-token",
		Headers: map[string]string{
			"Authorization": "Bearer header-token",
		},
		Query: map[string]string{
			"access_token": "query-token",
		},
		Body: json.RawMessage(`{
			"safe":"visible",
			"token":"body-token",
			"nested":{"password":"body-password","items":[{"secret":"body-secret"}]}
		}`),
	})

	body := decodeRequestBody(t, redacted)
	requireBodyField(t, body, "safe", "visible")
	requireBodyField(t, body, "token", "***")
	nested, ok := body["nested"].(map[string]any)
	if !ok {
		t.Fatalf("expected nested object, got %+v", body["nested"])
	}
	if nested["password"] != "***" {
		t.Fatalf("expected nested password to be redacted, got %+v", nested)
	}
	items, ok := nested["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("expected nested items array, got %+v", nested["items"])
	}
	item, ok := items[0].(map[string]any)
	if !ok || item["secret"] != "***" {
		t.Fatalf("expected nested array secret to be redacted, got %+v", items)
	}
}

func TestRefreshTokenUnsupportedProviderReturnsInvalidInput(t *testing.T) {
	service := NewService(singleChannelStore{channel: Channel{
		ID:           "channel-webhook",
		ProviderType: ProviderWebhook,
		Name:         "Webhook",
	}})

	_, err := service.RefreshToken(context.Background(), "channel-webhook")
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected unsupported refresh-token to return ErrInvalidInput, got %v", err)
	}
}

func TestEgressPolicyBlocksLoopbackAndMetadataButAllowsPrivateInterfaceAddress(t *testing.T) {
	for _, addr := range []string{"127.0.0.1", "::1", "169.254.169.254", "100.100.100.200"} {
		if egressAddressAllowed(netip.MustParseAddr(addr)) {
			t.Fatalf("expected egress address %s to be blocked", addr)
		}
	}
	if !egressAddressAllowed(netip.MustParseAddr("172.16.66.30")) {
		t.Fatal("private interface address 172.16.66.30 should be allowed")
	}
}

func TestSendBuiltRequestRejectsLoopbackHTTP(t *testing.T) {
	_, _, _, err := sendBuiltRequest(context.Background(), Channel{
		ProviderType: ProviderWebhook,
		TimeoutMS:    100,
	}, BuiltRequest{
		Method: "POST",
		URL:    "http://127.0.0.1:1/webhook",
		Body:   json.RawMessage(`{"title":"blocked"}`),
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected loopback HTTP send to be rejected as invalid input, got %v", err)
	}
}

func TestSendBuiltRequestRejectsLoopbackSMTP(t *testing.T) {
	_, _, _, err := sendBuiltRequest(context.Background(), Channel{
		ProviderType: ProviderEmail,
		AuthConfig:   json.RawMessage(`{"host":"127.0.0.1","username":"login@example.com","password":"app-password"}`),
		TimeoutMS:    100,
	}, BuiltRequest{
		Method: "SMTP_SEND",
		URL:    "smtp://127.0.0.1:1",
		Body: json.RawMessage(`{
			"from":"sender@example.com",
			"to":["user@example.com"],
			"subject":"blocked",
			"body":"blocked"
		}`),
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected loopback SMTP send to be rejected as invalid input, got %v", err)
	}
}

func TestSendBuiltRequestDeliversEmailOverSMTP(t *testing.T) {
	smtpServer := newFakeSMTPTLSServer(t)
	defer smtpServer.Close()

	originalTLSConfig := smtpTLSConfigForHost
	originalSMTPResolve := smtpResolveEgressAddress
	smtpTLSConfigForHost = func(host string) *tls.Config {
		return &tls.Config{ServerName: host, InsecureSkipVerify: true}
	}
	smtpResolveEgressAddress = func(context.Context, smtpEndpoint) (string, error) {
		return smtpServer.Addr(), nil
	}
	t.Cleanup(func() {
		smtpTLSConfigForHost = originalTLSConfig
		smtpResolveEgressAddress = originalSMTPResolve
	})

	channel := Channel{
		ProviderType: ProviderEmail,
		AuthConfig:   json.RawMessage(`{"host":"127.0.0.1","username":"login@example.com","password":"app-password"}`),
		SendConfig:   json.RawMessage(`{"from":"Gongsy OA-admin <sender-alias@example.com>"}`),
		TimeoutMS:    1000,
	}
	built, err := BuildRequest(channel, BuildRequestInput{
		Recipient: "user@example.com",
		Body:      json.RawMessage(`{"subject":"邮件测试标题","body":"邮件测试消息"}`),
	})
	if err != nil {
		t.Fatalf("build email request: %v", err)
	}
	built.URL = "smtp://" + smtpServer.Addr()

	statusCode, _, response, err := sendBuiltRequest(context.Background(), channel, built)
	if err != nil {
		t.Fatalf("send smtp email: %v", err)
	}
	if statusCode != http.StatusAccepted {
		t.Fatalf("expected accepted status, got %d body=%s", statusCode, response)
	}
	if !strings.Contains(smtpServer.Message(), "Subject: =?utf-8?q?=E9=82=AE=E4=BB=B6=E6=B5=8B=E8=AF=95=E6=A0=87=E9=A2=98?=") {
		t.Fatalf("expected encoded subject, got:\n%s", smtpServer.Message())
	}
	if !strings.Contains(smtpServer.Message(), "邮件测试消息") {
		t.Fatalf("expected message body, got:\n%s", smtpServer.Message())
	}
	if !strings.Contains(smtpServer.Message(), `From: "Gongsy OA-admin" <sender-alias@example.com>`) {
		t.Fatalf("expected display From header, got:\n%s", smtpServer.Message())
	}
	if smtpServer.MailFrom() != "login@example.com" {
		t.Fatalf("expected SMTP MAIL FROM address, got %q", smtpServer.MailFrom())
	}
	if got := smtpServer.Recipients(); len(got) != 1 || got[0] != "user@example.com" {
		t.Fatalf("expected SMTP recipient, got %+v", got)
	}
}

func TestSendBuiltRequestComposesEmailFromDisplayName(t *testing.T) {
	smtpServer := newFakeSMTPTLSServer(t)
	defer smtpServer.Close()

	originalTLSConfig := smtpTLSConfigForHost
	originalSMTPResolve := smtpResolveEgressAddress
	smtpTLSConfigForHost = func(host string) *tls.Config {
		return &tls.Config{ServerName: host, InsecureSkipVerify: true}
	}
	smtpResolveEgressAddress = func(context.Context, smtpEndpoint) (string, error) {
		return smtpServer.Addr(), nil
	}
	t.Cleanup(func() {
		smtpTLSConfigForHost = originalTLSConfig
		smtpResolveEgressAddress = originalSMTPResolve
	})

	channel := Channel{
		ProviderType: ProviderEmail,
		AuthConfig:   json.RawMessage(`{"host":"127.0.0.1","username":"2536209004@qq.com","password":"app-password"}`),
		SendConfig:   json.RawMessage(`{"from":"Gongsy-admin"}`),
		TimeoutMS:    1000,
	}
	built, err := BuildRequest(channel, BuildRequestInput{
		Recipient: "user@example.com",
		Body:      json.RawMessage(`{"subject":"Test","body":"Body"}`),
	})
	if err != nil {
		t.Fatalf("build email request: %v", err)
	}
	built.URL = "smtp://" + smtpServer.Addr()

	if _, _, _, err := sendBuiltRequest(context.Background(), channel, built); err != nil {
		t.Fatalf("send smtp email: %v", err)
	}
	if !strings.Contains(smtpServer.Message(), `From: "Gongsy-admin" <2536209004@qq.com>`) {
		t.Fatalf("expected composed From header, got:\n%s", smtpServer.Message())
	}
	if smtpServer.MailFrom() != "2536209004@qq.com" {
		t.Fatalf("expected SMTP MAIL FROM username, got %q", smtpServer.MailFrom())
	}
}

func TestSendBuiltRequestDeliversEmailHTMLContent(t *testing.T) {
	smtpServer := newFakeSMTPTLSServer(t)
	defer smtpServer.Close()

	originalTLSConfig := smtpTLSConfigForHost
	originalSMTPResolve := smtpResolveEgressAddress
	smtpTLSConfigForHost = func(host string) *tls.Config {
		return &tls.Config{ServerName: host, InsecureSkipVerify: true}
	}
	smtpResolveEgressAddress = func(context.Context, smtpEndpoint) (string, error) {
		return smtpServer.Addr(), nil
	}
	t.Cleanup(func() {
		smtpTLSConfigForHost = originalTLSConfig
		smtpResolveEgressAddress = originalSMTPResolve
	})

	channel := Channel{
		ProviderType: ProviderEmail,
		AuthConfig:   json.RawMessage(`{"host":"127.0.0.1","username":"login@example.com","password":"app-password"}`),
		TimeoutMS:    1000,
	}
	built, err := BuildRequest(channel, BuildRequestInput{
		Recipient: "user@example.com",
		Body:      json.RawMessage(`{"subject":"HTML Test","body":"<strong>Body</strong>","format":"html"}`),
	})
	if err != nil {
		t.Fatalf("build email request: %v", err)
	}
	built.URL = "smtp://" + smtpServer.Addr()

	if _, _, _, err := sendBuiltRequest(context.Background(), channel, built); err != nil {
		t.Fatalf("send smtp html email: %v", err)
	}
	if !strings.Contains(smtpServer.Message(), "Content-Type: text/html; charset=UTF-8") {
		t.Fatalf("expected html content type, got:\n%s", smtpServer.Message())
	}
	if !strings.Contains(smtpServer.Message(), "<strong>Body</strong>") {
		t.Fatalf("expected html body, got:\n%s", smtpServer.Message())
	}
}

func TestTestSendRequiresBarkRecipientEvenWithLegacyChannelDeviceKey(t *testing.T) {
	service := NewService(singleChannelStore{
		channel: Channel{
			ID:           "channel-bark",
			ProviderType: ProviderBark,
			Name:         "Bark",
			AuthConfig:   json.RawMessage(`{"server_url":"https://bark.example","device_key":"channel-key"}`),
			TimeoutMS:    1000,
		},
	})

	_, err := service.TestSend(context.Background(), "channel-bark", TestSendInput{
		BuildRequestInput: BuildRequestInput{
			Body: json.RawMessage(`{"body":"Bark dry run"}`),
		},
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected Bark test send to require recipient, got %v", err)
	}
}

func TestTestSendAllowsBarkResolvedRecipientWithoutChannelDeviceKey(t *testing.T) {
	service := NewService(singleChannelStore{
		channel: Channel{
			ID:           "channel-bark",
			ProviderType: ProviderBark,
			Name:         "Bark",
			AuthConfig:   json.RawMessage(`{"server_url":"https://bark.example"}`),
			TimeoutMS:    1000,
		},
	})

	result, err := service.TestSend(context.Background(), "channel-bark", TestSendInput{
		BuildRequestInput: BuildRequestInput{
			Body: json.RawMessage(`{"body":"Bark dry run"}`),
		},
		ResolvedRecipients: []ResolvedRecipient{{PlatformIDs: map[string]string{"bark_device_key": "device-key"}}},
	})
	if err != nil {
		t.Fatalf("bark dry-run with resolved recipient: %v", err)
	}
	body := decodeRequestBody(t, result.Request)
	requireBodyField(t, body, "device_key", "device-key")
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
			Body: json.RawMessage(`{"body":{"title":"live send"}}`),
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

type channelTokenCacheStore struct {
	singleChannelStore
	*memoryTokenCacheStore
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

type recordingChannelStore struct {
	createInput CreateChannelParams
	updateInput UpdateChannelParams
}

func (s *recordingChannelStore) SeedProviderCapabilities(context.Context, []Capability) error {
	return nil
}

func (s *recordingChannelStore) ListProviderCapabilities(context.Context) ([]Capability, error) {
	return nil, nil
}

func (s *recordingChannelStore) ListChannels(context.Context) ([]Channel, error) {
	return nil, nil
}

func (s *recordingChannelStore) CreateChannel(_ context.Context, params CreateChannelParams) (Channel, error) {
	s.createInput = params
	return Channel{
		ID:               "channel-1",
		ProviderType:     params.ProviderType,
		Name:             params.Name,
		Description:      params.Description,
		Enabled:          params.Enabled,
		ConcurrencyLimit: params.ConcurrencyLimit,
		TimeoutMS:        params.TimeoutMS,
	}, nil
}

func (s *recordingChannelStore) GetChannel(context.Context, string) (Channel, error) {
	return Channel{}, ErrNotFound
}

func (s *recordingChannelStore) UpdateChannel(_ context.Context, id string, params UpdateChannelParams) (Channel, error) {
	s.updateInput = params
	return Channel{
		ID:               id,
		ProviderType:     params.ProviderType,
		Name:             params.Name,
		Description:      params.Description,
		Enabled:          params.Enabled,
		ConcurrencyLimit: params.ConcurrencyLimit,
		TimeoutMS:        params.TimeoutMS,
	}, nil
}

func (s *recordingChannelStore) DeleteChannel(context.Context, string) error {
	return nil
}

func localTestHTTPClientFactory(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout}
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

func assertJSONDoesNotContain(t *testing.T, raw json.RawMessage, text string) {
	t.Helper()

	if strings.Contains(string(raw), text) {
		t.Fatalf("expected JSON not to contain %q, got %s", text, raw)
	}
}

func assertJSONStringListField(t *testing.T, raw json.RawMessage, path string, expected []string) {
	t.Helper()

	var object any
	if err := json.Unmarshal(raw, &object); err != nil {
		t.Fatalf("decode json: %v", err)
	}
	actual, ok := jsonPathValue(object, strings.Split(path, ".")).([]any)
	if !ok {
		t.Fatalf("expected %s to be a string list, got %v in %s", path, actual, raw)
	}
	if len(actual) != len(expected) {
		t.Fatalf("expected %s=%v, got %v in %s", path, expected, actual, raw)
	}
	for index, item := range actual {
		if item != expected[index] {
			t.Fatalf("expected %s=%v, got %v in %s", path, expected, actual, raw)
		}
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

type fakeSMTPTLSServer struct {
	listener   net.Listener
	done       chan struct{}
	mailFrom   atomic.Value
	recipients atomic.Value
	message    atomic.Value
}

func newFakeSMTPTLSServer(t *testing.T) *fakeSMTPTLSServer {
	t.Helper()

	tlsSource := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	certificates := tlsSource.TLS.Certificates
	tlsSource.Close()

	listener, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: certificates})
	if err != nil {
		t.Fatalf("listen fake smtp: %v", err)
	}
	server := &fakeSMTPTLSServer{listener: listener, done: make(chan struct{})}
	go server.serve()
	return server
}

func (s *fakeSMTPTLSServer) Addr() string {
	return s.listener.Addr().String()
}

func (s *fakeSMTPTLSServer) Close() {
	_ = s.listener.Close()
	<-s.done
}

func (s *fakeSMTPTLSServer) MailFrom() string {
	value, _ := s.mailFrom.Load().(string)
	return value
}

func (s *fakeSMTPTLSServer) Recipients() []string {
	value, _ := s.recipients.Load().([]string)
	return value
}

func (s *fakeSMTPTLSServer) Message() string {
	value, _ := s.message.Load().(string)
	return value
}

func (s *fakeSMTPTLSServer) serve() {
	defer close(s.done)
	conn, err := s.listener.Accept()
	if err != nil {
		return
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)
	writeSMTPLine(writer, "220 localhost ESMTP")
	recipients := []string{}
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		command := strings.TrimSpace(line)
		upper := strings.ToUpper(command)
		switch {
		case strings.HasPrefix(upper, "EHLO"):
			writeSMTPLine(writer, "250-localhost")
			writeSMTPLine(writer, "250 AUTH PLAIN")
		case strings.HasPrefix(upper, "AUTH"):
			writeSMTPLine(writer, "235 2.7.0 Authentication successful")
		case strings.HasPrefix(upper, "MAIL FROM:"):
			s.mailFrom.Store(extractSMTPPath(command))
			writeSMTPLine(writer, "250 2.1.0 OK")
		case strings.HasPrefix(upper, "RCPT TO:"):
			recipients = append(recipients, extractSMTPPath(command))
			s.recipients.Store(append([]string(nil), recipients...))
			writeSMTPLine(writer, "250 2.1.5 OK")
		case upper == "DATA":
			writeSMTPLine(writer, "354 End data with <CR><LF>.<CR><LF>")
			message, err := readSMTPData(reader)
			if err != nil {
				return
			}
			s.message.Store(message)
			writeSMTPLine(writer, "250 2.0.0 OK queued")
		case upper == "QUIT":
			writeSMTPLine(writer, "221 2.0.0 Bye")
			return
		default:
			writeSMTPLine(writer, "250 OK")
		}
	}
}

func writeSMTPLine(writer *bufio.Writer, line string) {
	_, _ = writer.WriteString(line + "\r\n")
	_ = writer.Flush()
}

func extractSMTPPath(command string) string {
	start := strings.Index(command, "<")
	end := strings.LastIndex(command, ">")
	if start >= 0 && end > start {
		return command[start+1 : end]
	}
	parts := strings.SplitN(command, ":", 2)
	if len(parts) == 2 {
		return strings.TrimSpace(parts[1])
	}
	return ""
}

func readSMTPData(reader *bufio.Reader) (string, error) {
	var builder strings.Builder
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}
		if strings.TrimRight(line, "\r\n") == "." {
			return builder.String(), nil
		}
		if strings.HasPrefix(line, "..") {
			line = line[1:]
		}
		if _, err := io.WriteString(&builder, line); err != nil {
			return "", err
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
