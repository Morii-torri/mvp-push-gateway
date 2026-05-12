package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

type capabilityProviderService struct {
	capabilities []provider.Capability
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
