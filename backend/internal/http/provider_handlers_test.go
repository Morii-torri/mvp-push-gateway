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

type capabilityProviderService struct {
	capabilities             []provider.Capability
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
	return provider.Channel{}, nil
}

func (f *capabilityProviderService) UpdateChannel(context.Context, string, provider.UpdateChannelInput) (provider.Channel, error) {
	return provider.Channel{}, nil
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
