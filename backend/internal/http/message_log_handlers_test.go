package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	httpapi "mvp-push-gateway/backend/internal/http"
	"mvp-push-gateway/backend/internal/messagelog"
)

func TestMessageLogDetailReturnsDerivedAttemptSnapshots(t *testing.T) {
	now := time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC)
	store := fakeMessageLogStore{
		detail: messagelog.MessageDetail{
			MessageSummary: messagelog.MessageSummary{
				ID:              "message-1",
				TraceID:         "trace-1",
				SourceID:        "source-1",
				SourceName:      "Orders",
				ReceivedAt:      now,
				Status:          "sent",
				MatchedFlowID:   "flow-1",
				MatchedFlowName: "订单告警路由",
			},
			Payload: json.RawMessage(`{"title":"paid"}`),
			Attempts: []messagelog.DeliveryAttempt{
				{
					ID:                "attempt-wecom",
					MessageID:         "message-1",
					ChannelID:         "channel-wecom",
					ChannelName:       "企业微信生产",
					ProviderType:      "wecom_app",
					TemplateVersionID: "tpl-wecom-v1",
					RequestSnapshot: json.RawMessage(`{
						"target_context":{"channel_id":"channel-wecom","provider_type":"wecom_app","message_type":"markdown","template_version_id":"tpl-wecom-v1"},
						"rendered_message":{"message_type":"markdown","content":{"markdown":"## paid"}},
						"resolved_recipients":[{"user_id":"user-1","wecom_userid":"zhangsan"}],
						"final_request":{"method":"POST","url":"https://wecom.test/send","body":{"touser":"zhangsan","markdown":{"content":"## paid"}}}
					}`),
					ResponseSnapshot: json.RawMessage(`{"upstream_response":{"status_code":200,"body":{"errcode":0}}}`),
					Status:           "sent",
					DurationMS:       120,
					AttemptNo:        1,
					CreatedAt:        now,
					UpdatedAt:        now,
				},
				{
					ID:                "attempt-legacy",
					MessageID:         "message-1",
					ChannelID:         "channel-legacy",
					ChannelName:       "旧 Webhook",
					ProviderType:      "webhook",
					TemplateVersionID: "tpl-legacy-v1",
					RequestSnapshot:   json.RawMessage(`{"send":{"method":"POST","url":"https://legacy.test/send","recipient":"ops","body":{"text":"legacy"}}}`),
					ResponseSnapshot:  json.RawMessage(`{"send":{"status_code":202,"body":{"ok":true}}}`),
					Status:            "sent",
					DurationMS:        80,
					AttemptNo:         1,
					CreatedAt:         now,
					UpdatedAt:         now,
				},
			},
		},
	}
	handler := httpapi.NewHandler(
		testConfig(),
		httpapi.WithAuthService(fakeAuthService{authenticatedToken: "admin-session"}),
		httpapi.WithMessageLogService(messagelog.NewService(store)),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/messages/message-1", nil)
	req.Header.Set("Authorization", "Bearer admin-session")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected detail status 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var body struct {
		Message struct {
			Attempts []struct {
				ID                 string         `json:"id"`
				TargetContext      map[string]any `json:"target_context"`
				RenderedMessage    map[string]any `json:"rendered_message"`
				ResolvedRecipients []any          `json:"resolved_recipients"`
				FinalRequest       map[string]any `json:"final_request"`
				UpstreamResponse   map[string]any `json:"upstream_response"`
			} `json:"attempts"`
		} `json:"message"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode detail response: %v", err)
	}
	if len(body.Message.Attempts) != 2 {
		t.Fatalf("expected two attempts, got %+v", body.Message.Attempts)
	}

	first := body.Message.Attempts[0]
	if first.TargetContext["provider_type"] != "wecom_app" || first.RenderedMessage["message_type"] != "markdown" || len(first.ResolvedRecipients) != 1 {
		t.Fatalf("expected derived new attempt fields, got %+v", first)
	}
	if first.FinalRequest["url"] != "https://wecom.test/send" || first.UpstreamResponse["status_code"] != float64(200) {
		t.Fatalf("expected final request and upstream response, got %+v", first)
	}

	legacy := body.Message.Attempts[1]
	if legacy.FinalRequest["url"] != "https://legacy.test/send" || legacy.UpstreamResponse["status_code"] != float64(202) {
		t.Fatalf("expected legacy send fallback fields, got %+v", legacy)
	}
}

type fakeMessageLogStore struct {
	detail messagelog.MessageDetail
}

func (f fakeMessageLogStore) ListMessages(context.Context, messagelog.ListFilter) (messagelog.ListResult, error) {
	return messagelog.ListResult{}, nil
}

func (f fakeMessageLogStore) GetMessage(context.Context, string) (messagelog.MessageDetail, error) {
	return f.detail, nil
}
