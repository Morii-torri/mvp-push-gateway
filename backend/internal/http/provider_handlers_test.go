package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	httpapi "mvp-push-gateway/backend/internal/http"
	"mvp-push-gateway/backend/internal/provider"
)

func TestProviderCapabilitiesResponseIncludesCapabilityMetadata(t *testing.T) {
	service := &capabilityProviderService{
		capabilities: []provider.Capability{{
			ID:                      "cap-1",
			ProviderType:            provider.ProviderWebhook,
			DisplayName:             "通用 Webhook",
			Category:                "advanced",
			MessageType:             "json",
			MessageSchema:           json.RawMessage(`{"type":"object","properties":{"payload":{"type":"object"}}}`),
			CredentialSchema:        json.RawMessage(`{"type":"object","properties":{"secret":{"type":"string"}}}`),
			ChannelConfigSchema:     json.RawMessage(`{"type":"object","properties":{"url":{"type":"string"}}}`),
			CustomBodyAllowed:       true,
			RecipientRequired:       false,
			AllowNoRecipient:        true,
			RecipientRequirement:    "none",
			RecipientLocation:       provider.PlacementNone,
			RecipientFormat:         "string",
			TokenLocation:           provider.PlacementNone,
			TokenStrategy:           json.RawMessage(`{"strategy":"none"}`),
			SendAPI:                 json.RawMessage(`{"method":"POST","url_template":"{{ channel.url }}"}`),
			SuccessRule:             json.RawMessage(`{"type":"status_code","status_codes":[200,201,202]}`),
			RetryRule:               json.RawMessage(`{"status_codes":[429,500,502,503,504]}`),
			DefaultRateLimit:        json.RawMessage(`{"qps":10}`),
			DefaultTimeoutMS:        8000,
			DefaultConcurrencyLimit: 4,
			DefaultRetryPolicy:      json.RawMessage(`{"max_attempts":2,"delay_ms":500}`),
			RequestExamples:         json.RawMessage(`{"payload":"{{ payload }}"}`),
		}},
	}
	handler := httpapi.NewHandler(
		testConfig(),
		httpapi.WithAuthService(fakeAuthService{authenticatedToken: "admin-session"}),
		httpapi.WithProviderService(service),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/provider-capabilities", nil)
	req.Header.Set("Authorization", "Bearer admin-session")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var body struct {
		Capabilities []map[string]any `json:"capabilities"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode provider capabilities response: %v", err)
	}
	if len(body.Capabilities) != 1 {
		t.Fatalf("expected one capability, got %d", len(body.Capabilities))
	}
	capability := body.Capabilities[0]
	for _, key := range []string{
		"display_name",
		"category",
		"credential_schema",
		"channel_config_schema",
		"custom_body_allowed",
		"recipient_requirement",
		"token_strategy",
		"send_api",
		"success_rule",
		"retry_rule",
		"default_rate_limit",
		"default_timeout_ms",
		"default_concurrency_limit",
		"default_retry_policy",
	} {
		if _, ok := capability[key]; !ok {
			t.Fatalf("response missing key %q: %+v", key, capability)
		}
	}
	if capability["display_name"] != "通用 Webhook" || capability["custom_body_allowed"] != true {
		t.Fatalf("unexpected capability metadata: %+v", capability)
	}
	if capability["message_schema"] == nil || capability["request_examples"] == nil {
		t.Fatalf("legacy schema/example fields should remain present: %+v", capability)
	}
}

func TestChannelDescriptionRoundTripsThroughHTTPHandlers(t *testing.T) {
	service := &capabilityProviderService{
		channel: provider.Channel{
			ID:               "channel-1",
			ProviderType:     provider.ProviderBark,
			Name:             "bark-webhook",
			Enabled:          true,
			Description:      "值班告警主通道",
			ConcurrencyLimit: 1,
			TimeoutMS:        1000,
		},
	}
	handler := httpapi.NewHandler(
		testConfig(),
		httpapi.WithAuthService(fakeAuthService{authenticatedToken: "admin-session"}),
		httpapi.WithProviderService(service),
	)

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/channels/channel-1", nil)
	getReq.Header.Set("Authorization", "Bearer admin-session")
	getRec := httptest.NewRecorder()
	handler.ServeHTTP(getRec, getReq)

	if getRec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", getRec.Code, getRec.Body.String())
	}
	var getBody struct {
		Channel map[string]any `json:"channel"`
	}
	if err := json.NewDecoder(getRec.Body).Decode(&getBody); err != nil {
		t.Fatalf("decode get response: %v", err)
	}
	if getBody.Channel["description"] != "值班告警主通道" {
		t.Fatalf("expected description in get response, got %+v", getBody.Channel)
	}

	putReq := httptest.NewRequest(http.MethodPut, "/api/v1/channels/channel-1", strings.NewReader(`{
		"provider_type":"bark",
		"name":"bark-webhook",
		"enabled":true,
		"description":"",
		"auth_config":{},
		"token_config":{},
		"send_config":{},
		"rate_limit_config":{},
		"concurrency_limit":1,
		"timeout_ms":1000,
		"retry_policy":{"max_attempts":1},
		"dead_letter_policy":{}
	}`))
	putReq.Header.Set("Authorization", "Bearer admin-session")
	putReq.Header.Set("Content-Type", "application/json")
	putRec := httptest.NewRecorder()
	handler.ServeHTTP(putRec, putReq)

	if putRec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", putRec.Code, putRec.Body.String())
	}
	if service.updateInput.Description != "" {
		t.Fatalf("expected blank description to be passed through update, got %q", service.updateInput.Description)
	}
}

func TestFeishuResolveOpenIDEndpointDelegatesToProviderService(t *testing.T) {
	service := &capabilityProviderService{
		resolveResult: provider.FeishuOpenIDResolveResult{
			Success: true,
			Items: []provider.FeishuOpenIDResolveItem{{
				Mobile: "13011111111",
				OpenID: "ou_resolved",
				Status: "resolved",
			}},
		},
	}
	handler := httpapi.NewHandler(
		testConfig(),
		httpapi.WithAuthService(fakeAuthService{authenticatedToken: "admin-session"}),
		httpapi.WithProviderService(service),
	)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/channels/channel-feishu/feishu/resolve-open-id", strings.NewReader(`{"mobiles":["13011111111"]}`))
	req.Header.Set("Authorization", "Bearer admin-session")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if service.resolveChannelID != "channel-feishu" || strings.Join(service.resolveMobiles, ",") != "13011111111" {
		t.Fatalf("unexpected resolve input channel=%q mobiles=%v", service.resolveChannelID, service.resolveMobiles)
	}
	var body struct {
		Success bool                               `json:"success"`
		Items   []provider.FeishuOpenIDResolveItem `json:"items"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !body.Success || len(body.Items) != 1 || body.Items[0].OpenID != "ou_resolved" {
		t.Fatalf("unexpected response: %+v", body)
	}
}

func TestDingTalkResolveUserIDEndpointDelegatesToProviderService(t *testing.T) {
	service := &capabilityProviderService{
		dingTalkResolveResult: provider.DingTalkUserIDResolveResult{
			Success: true,
			Items: []provider.DingTalkUserIDResolveItem{{
				QueryWord: "张三",
				UserID:    "093102391140051902",
				Status:    "resolved",
			}},
		},
	}
	handler := httpapi.NewHandler(
		testConfig(),
		httpapi.WithAuthService(fakeAuthService{authenticatedToken: "admin-session"}),
		httpapi.WithProviderService(service),
	)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/channels/channel-dingtalk/dingtalk/resolve-user-id", strings.NewReader(`{"query_words":["张三"]}`))
	req.Header.Set("Authorization", "Bearer admin-session")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if service.dingTalkResolveChannelID != "channel-dingtalk" || strings.Join(service.dingTalkResolveWords, ",") != "张三" {
		t.Fatalf("unexpected resolve input channel=%q words=%v", service.dingTalkResolveChannelID, service.dingTalkResolveWords)
	}
	var body struct {
		Success bool                                 `json:"success"`
		Items   []provider.DingTalkUserIDResolveItem `json:"items"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !body.Success || len(body.Items) != 1 || body.Items[0].UserID != "093102391140051902" {
		t.Fatalf("unexpected response: %+v", body)
	}
}

func TestPatchChannelEnabledPreservesRuntimeConfiguration(t *testing.T) {
	service := &capabilityProviderService{
		channel: provider.Channel{
			ID:               "channel-1",
			ProviderType:     provider.ProviderWebhook,
			Name:             "Webhook",
			Enabled:          true,
			AuthConfig:       json.RawMessage(`{"token":"secret"}`),
			SendConfig:       json.RawMessage(`{"url":"https://example.test/send"}`),
			RateLimitConfig:  json.RawMessage(`{"enabled":true,"qps":9}`),
			ConcurrencyLimit: 6,
			TimeoutMS:        2500,
			RetryPolicy:      json.RawMessage(`{"max_attempts":4,"delay_ms":800}`),
			DeadLetterPolicy: json.RawMessage(`{"policy":"retry_exhausted_or_upstream_error","retention_days":30,"replay":false}`),
		},
	}
	handler := httpapi.NewHandler(
		testConfig(),
		httpapi.WithAuthService(fakeAuthService{authenticatedToken: "admin-session"}),
		httpapi.WithProviderService(service),
	)

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/channels/channel-1", strings.NewReader(`{"enabled":false}`))
	req.Header.Set("Authorization", "Bearer admin-session")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if service.updateChannelID != "channel-1" {
		t.Fatalf("expected channel-1 update, got %q", service.updateChannelID)
	}
	input := service.updateInput
	if input.Enabled {
		t.Fatal("expected enabled=false in patch update")
	}
	if input.ConcurrencyLimit != 6 || input.TimeoutMS != 2500 {
		t.Fatalf("expected runtime settings to be preserved, got concurrency=%d timeout=%d", input.ConcurrencyLimit, input.TimeoutMS)
	}
	if string(input.DeadLetterPolicy) != `{"policy":"retry_exhausted_or_upstream_error","retention_days":30,"replay":false}` {
		t.Fatalf("expected dead letter policy to be preserved, got %s", input.DeadLetterPolicy)
	}
}

func TestSensitiveChannelActionsWriteAuditRecords(t *testing.T) {
	for _, tc := range []struct {
		name   string
		path   string
		body   string
		action string
	}{
		{name: "build request", path: "/api/v1/channels/channel-1/build-request", body: `{"body":{"title":"preview"}}`, action: "build_request"},
		{name: "test send", path: "/api/v1/channels/channel-1/test-send", body: `{"send":false,"body":{"title":"preview"}}`, action: "test_send"},
		{name: "refresh token", path: "/api/v1/channels/channel-1/refresh-token", body: `{}`, action: "refresh_token"},
		{name: "feishu resolve", path: "/api/v1/channels/channel-1/feishu/resolve-open-id", body: `{"mobiles":["13011111111"]}`, action: "resolve_feishu_open_id"},
		{name: "dingtalk resolve", path: "/api/v1/channels/channel-1/dingtalk/resolve-user-id", body: `{"query_words":["张三"]}`, action: "resolve_dingtalk_user_id"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			auditService := &fakeAuditService{}
			handler := httpapi.NewHandler(
				testConfig(),
				httpapi.WithAuthService(fakeAuthService{authenticatedToken: "admin-session"}),
				httpapi.WithProviderService(&capabilityProviderService{
					resolveResult:         provider.FeishuOpenIDResolveResult{Success: true},
					dingTalkResolveResult: provider.DingTalkUserIDResolveResult{Success: true},
				}),
				httpapi.WithAuditService(auditService),
			)

			req := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(tc.body))
			req.Header.Set("Authorization", "Bearer admin-session")
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
			}
			if auditService.recordCalls != 1 {
				t.Fatalf("expected one audit record, got %d", auditService.recordCalls)
			}
			if auditService.recordInput.Action != tc.action || auditService.recordInput.ResourceType != "channel" || auditService.recordInput.ResourceID != "channel-1" {
				t.Fatalf("unexpected audit record: %+v", auditService.recordInput)
			}
		})
	}
}

type capabilityProviderService struct {
	capabilities             []provider.Capability
	channel                  provider.Channel
	updateChannelID          string
	updateInput              provider.UpdateChannelInput
	resolveChannelID         string
	resolveMobiles           []string
	resolveResult            provider.FeishuOpenIDResolveResult
	dingTalkResolveChannelID string
	dingTalkResolveWords     []string
	dingTalkResolveResult    provider.DingTalkUserIDResolveResult
}

func (f *capabilityProviderService) SeedProviderCapabilities(context.Context) error {
	return nil
}

func (f *capabilityProviderService) ListProviderCapabilities(context.Context) ([]provider.Capability, error) {
	return f.capabilities, nil
}

func (f *capabilityProviderService) ListChannels(context.Context) ([]provider.Channel, error) {
	return nil, nil
}

func (f *capabilityProviderService) CreateChannel(context.Context, provider.CreateChannelInput) (provider.Channel, error) {
	return provider.Channel{}, nil
}

func (f *capabilityProviderService) GetChannel(context.Context, string) (provider.Channel, error) {
	return f.channel, nil
}

func (f *capabilityProviderService) UpdateChannel(_ context.Context, id string, input provider.UpdateChannelInput) (provider.Channel, error) {
	f.updateChannelID = id
	f.updateInput = input
	return provider.Channel{
		ID:               id,
		ProviderType:     input.ProviderType,
		Name:             input.Name,
		Enabled:          input.Enabled,
		Description:      input.Description,
		ConcurrencyLimit: input.ConcurrencyLimit,
		TimeoutMS:        input.TimeoutMS,
	}, nil
}

func (f *capabilityProviderService) DeleteChannel(context.Context, string) error {
	return nil
}

func (f *capabilityProviderService) BuildRequest(context.Context, string, provider.BuildRequestInput) (provider.BuiltRequest, error) {
	return provider.BuiltRequest{}, nil
}

func (f *capabilityProviderService) TestSend(context.Context, string, provider.TestSendInput) (provider.TestSendResult, error) {
	return provider.TestSendResult{}, nil
}

func (f *capabilityProviderService) RefreshToken(context.Context, string) (provider.TokenCacheStatus, error) {
	return provider.TokenCacheStatus{IsCached: true}, nil
}

func (f *capabilityProviderService) ResolveFeishuOpenID(_ context.Context, channelID string, mobiles []string) (provider.FeishuOpenIDResolveResult, error) {
	f.resolveChannelID = channelID
	f.resolveMobiles = append([]string(nil), mobiles...)
	return f.resolveResult, nil
}

func (f *capabilityProviderService) ResolveDingTalkUserID(_ context.Context, channelID string, queryWords []string) (provider.DingTalkUserIDResolveResult, error) {
	f.dingTalkResolveChannelID = channelID
	f.dingTalkResolveWords = append([]string(nil), queryWords...)
	return f.dingTalkResolveResult, nil
}
