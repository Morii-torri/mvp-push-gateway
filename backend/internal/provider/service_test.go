package provider

import (
	"encoding/json"
	"testing"
)

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
