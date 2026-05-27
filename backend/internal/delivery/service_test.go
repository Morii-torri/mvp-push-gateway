package delivery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"mvp-push-gateway/backend/internal/provider"
	"mvp-push-gateway/backend/internal/queue"
)

func TestWorkerProcessBatchScopesSendDedupeByChannel(t *testing.T) {
	var sent int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&sent, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	store := newMemoryRepository()
	channelA := provider.Channel{
		ID:               "channel-a",
		Name:             "Channel A",
		Enabled:          true,
		ConcurrencyLimit: 1,
		TimeoutMS:        500,
		SendConfig:       json.RawMessage(`{"method":"POST","url":"` + server.URL + `/send","recipient":{"location":"body","path":"recipient"}}`),
	}
	channelB := channelA
	channelB.ID = "channel-b"
	channelB.Name = "Channel B"
	store.channels[channelA.ID] = channelA
	store.channels[channelB.ID] = channelB

	store.addAttempt(Attempt{ID: "attempt-a1", MessageID: "message-a1", ChannelID: channelA.ID, Status: StatusQueued})
	store.addAttempt(Attempt{ID: "attempt-a2", MessageID: "message-a2", ChannelID: channelA.ID, Status: StatusQueued})
	store.addAttempt(Attempt{ID: "attempt-b1", MessageID: "message-b1", ChannelID: channelB.ID, Status: StatusQueued})

	store.addJob(newSendJob("job-a1", channelA.ID, 3, time.Now().Add(-time.Second), SendMessageJobPayload{
		DeliveryAttemptID: "attempt-a1",
		DedupeKey:         "order-1001",
		DedupeTTLSeconds:  3600,
		Recipient:         "user-a1",
		Body:              json.RawMessage(`{"title":"hello a1"}`),
	}))
	store.addJob(newSendJob("job-a2", channelA.ID, 3, time.Now().Add(-time.Second), SendMessageJobPayload{
		DeliveryAttemptID: "attempt-a2",
		DedupeKey:         "order-1001",
		DedupeTTLSeconds:  3600,
		Recipient:         "user-a2",
		Body:              json.RawMessage(`{"title":"hello a2"}`),
	}))
	store.addJob(newSendJob("job-b1", channelB.ID, 3, time.Now().Add(-time.Second), SendMessageJobPayload{
		DeliveryAttemptID: "attempt-b1",
		DedupeKey:         "order-1001",
		DedupeTTLSeconds:  3600,
		Recipient:         "user-b1",
		Body:              json.RawMessage(`{"title":"hello b1"}`),
	}))

	worker := NewWorker(store,
		WithWorkerID("sender-1"),
		WithHTTPClientFactory(func(timeout time.Duration) *http.Client {
			client := server.Client()
			client.Timeout = timeout
			return client
		}),
	)

	processed, err := worker.ProcessBatch(context.Background(), 3)
	if err != nil {
		t.Fatalf("process batch: %v", err)
	}
	if processed != 3 {
		t.Fatalf("expected 3 processed jobs, got %d", processed)
	}
	if atomic.LoadInt32(&sent) != 2 {
		t.Fatalf("expected 2 outbound sends after scoped dedupe, got %d", sent)
	}

	sameChannelStatuses := []Status{
		store.attempts["attempt-a1"].Status,
		store.attempts["attempt-a2"].Status,
	}
	sentCount := 0
	dedupedCount := 0
	for _, status := range sameChannelStatuses {
		if status == StatusSent {
			sentCount++
		}
		if status == StatusDeduped {
			dedupedCount++
		}
	}
	if sentCount != 1 || dedupedCount != 1 {
		t.Fatalf("expected same-channel attempts to split sent/deduped, got %+v", sameChannelStatuses)
	}
	if got := store.attempts["attempt-b1"].Status; got != StatusSent {
		t.Fatalf("expected other-channel attempt sent, got %s", got)
	}
}

func TestWorkerProcessBatchScopesSendDedupeByTemplateVersion(t *testing.T) {
	var sent int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&sent, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	store := newMemoryRepository()
	channel := provider.Channel{
		ID:               "channel-templates",
		Name:             "Template Scoped Channel",
		ProviderType:     provider.ProviderWebhook,
		Enabled:          true,
		ConcurrencyLimit: 1,
		TimeoutMS:        500,
		SendConfig:       json.RawMessage(`{"method":"POST","url":"` + server.URL + `/send","recipient":{"location":"none"}}`),
	}
	store.channels[channel.ID] = channel

	store.addAttempt(Attempt{ID: "attempt-template-a", MessageID: "message-template-a", ChannelID: channel.ID, TemplateVersionID: "template-version-a", Status: StatusQueued})
	store.addAttempt(Attempt{ID: "attempt-template-b", MessageID: "message-template-b", ChannelID: channel.ID, TemplateVersionID: "template-version-b", Status: StatusQueued})

	for _, item := range []struct {
		jobID     string
		attemptID string
		title     string
	}{
		{jobID: "job-template-a", attemptID: "attempt-template-a", title: "first"},
		{jobID: "job-template-b", attemptID: "attempt-template-b", title: "second"},
	} {
		store.addJob(newSendJob(item.jobID, channel.ID, 3, time.Now().Add(-time.Second), SendMessageJobPayload{
			DeliveryAttemptID: item.attemptID,
			DedupeKey:         "same-trace-id",
			DedupeTTLSeconds:  3600,
			MessageType:       "json",
			Body:              json.RawMessage(`{"title":"` + item.title + `"}`),
		}))
	}

	worker := NewWorker(store,
		WithWorkerID("sender-1"),
		WithHTTPClientFactory(func(timeout time.Duration) *http.Client {
			client := server.Client()
			client.Timeout = timeout
			return client
		}),
	)

	processed, err := worker.ProcessBatch(context.Background(), 2)
	if err != nil {
		t.Fatalf("process template-scoped dedupe batch: %v", err)
	}
	if processed != 2 {
		t.Fatalf("expected 2 processed jobs, got %d", processed)
	}
	if atomic.LoadInt32(&sent) != 2 {
		t.Fatalf("expected both template-version targets to send, got %d outbound sends", sent)
	}
	for _, attemptID := range []string{"attempt-template-a", "attempt-template-b"} {
		attempt := store.attempts[attemptID]
		if attempt.Status != StatusSent {
			t.Fatalf("expected %s sent, got %s", attemptID, attempt.Status)
		}
		snapshot := decodeSnapshot(t, attempt.RequestSnapshot)
		dedupe, ok := snapshot["dedupe"].(map[string]any)
		if !ok {
			t.Fatalf("expected request snapshot dedupe metadata, got %+v", snapshot)
		}
		if dedupe["configured_key"] != "same-trace-id" {
			t.Fatalf("expected original dedupe key to remain traceable, got %+v", dedupe)
		}
		if dedupe["effective_key"] == "same-trace-id" || !strings.Contains(fmt.Sprint(dedupe["effective_key"]), attempt.TemplateVersionID) {
			t.Fatalf("expected effective dedupe key scoped by template version, got %+v", dedupe)
		}
	}
}

func TestWorkerProcessOneBuildsRequestResolvesTokenAndStoresSnapshots(t *testing.T) {
	var authHeader string
	var requestBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"resolved-token"}`))
		case "/send":
			authHeader = r.Header.Get("Authorization")
			body, _ := io.ReadAll(r.Body)
			requestBody = string(body)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"message":"accepted"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	store := newMemoryRepository()
	store.channels["channel-1"] = provider.Channel{
		ID:               "channel-1",
		ProviderType:     provider.ProviderWebhook,
		Name:             "Webhook",
		Enabled:          true,
		ConcurrencyLimit: 1,
		TimeoutMS:        1000,
		TokenConfig: json.RawMessage(`{
			"request":{"method":"POST","url":"` + server.URL + `/token","headers":{"X-App":"gateway"},"body":{"client_id":"abc","client_secret":"xyz"}},
			"response_path":"access_token",
			"placement":{"location":"header","field_name":"Authorization","prefix":"Bearer "}
		}`),
		SendConfig: json.RawMessage(`{
			"method":"POST",
			"url":"` + server.URL + `/send",
			"body":{"msgtype":"text"},
			"recipient":{"location":"body","path":"touser","format":"array"}
		}`),
	}
	store.addAttempt(Attempt{
		ID:                "attempt-1",
		MessageID:         "message-1",
		ChannelID:         "channel-1",
		TemplateVersionID: "template-version-1",
		Status:            StatusQueued,
	})

	job := newSendJob("job-1", "channel-1", 3, time.Now().Add(-time.Second), SendMessageJobPayload{
		DeliveryAttemptID: "attempt-1",
		Recipient:         []any{"u1", "u2"},
		Body:              json.RawMessage(`{"text":{"content":"hello"}}`),
	})
	store.addJob(job)

	worker := NewWorker(store,
		WithWorkerID("sender-1"),
		WithHTTPClientFactory(func(timeout time.Duration) *http.Client {
			client := server.Client()
			client.Timeout = timeout
			return client
		}),
	)

	if err := worker.ProcessOne(context.Background(), store.jobs[job.ID]); err != nil {
		t.Fatalf("process one: %v", err)
	}

	if authHeader != "Bearer resolved-token" {
		t.Fatalf("expected resolved token in send request, got %q", authHeader)
	}
	if !strings.Contains(requestBody, `"touser":["u1","u2"]`) {
		t.Fatalf("expected recipient array in request body, got %s", requestBody)
	}

	attempt := store.attempts["attempt-1"]
	if attempt.Status != StatusSent {
		t.Fatalf("expected attempt sent, got %s", attempt.Status)
	}
	if store.jobs[job.ID].Status != queue.JobStatusDone {
		t.Fatalf("expected job done, got %s", store.jobs[job.ID].Status)
	}
	if jsonContains(t, attempt.RequestSnapshot, `"resolved-token"`) {
		t.Fatalf("expected request snapshot to redact resolved token, got %s", attempt.RequestSnapshot)
	}
	if jsonContains(t, attempt.RequestSnapshot, `"Authorization":"Bearer resolved-token"`) {
		t.Fatalf("expected request snapshot to redact outbound headers, got %s", attempt.RequestSnapshot)
	}
	var requestSnapshot map[string]any
	if err := json.Unmarshal(attempt.RequestSnapshot, &requestSnapshot); err != nil {
		t.Fatalf("decode request snapshot: %v", err)
	}
	targetContext, ok := requestSnapshot["target_context"].(map[string]any)
	if !ok {
		t.Fatalf("expected request snapshot target_context, got %s", attempt.RequestSnapshot)
	}
	for key, want := range map[string]string{
		"delivery_attempt_id": "attempt-1",
		"message_id":          "message-1",
		"channel_id":          "channel-1",
		"channel_name":        "Webhook",
		"provider_type":       string(provider.ProviderWebhook),
		"template_version_id": "template-version-1",
		"job_id":              "job-1",
	} {
		if got := targetContext[key]; got != want {
			t.Fatalf("expected target_context.%s=%q, got %v in %+v", key, want, got, targetContext)
		}
	}
	renderedMessage, ok := requestSnapshot["rendered_message"].(map[string]any)
	if !ok {
		t.Fatalf("expected request snapshot rendered_message, got %s", attempt.RequestSnapshot)
	}
	text, ok := renderedMessage["text"].(map[string]any)
	if !ok || text["content"] != "hello" {
		t.Fatalf("expected rendered_message to mirror job payload body, got %+v", renderedMessage)
	}
	resolvedRecipients, ok := requestSnapshot["resolved_recipients"].([]any)
	if !ok || len(resolvedRecipients) != 2 {
		t.Fatalf("expected resolved_recipients to mirror job payload recipient, got %+v", requestSnapshot["resolved_recipients"])
	}
	finalRequest, ok := requestSnapshot["final_request"].(map[string]any)
	if !ok {
		t.Fatalf("expected request snapshot final_request, got %s", attempt.RequestSnapshot)
	}
	finalBody, ok := finalRequest["body"].(map[string]any)
	if !ok || finalBody["msgtype"] != "text" {
		t.Fatalf("expected final_request body to include mapped send body, got %+v", finalRequest)
	}
	if _, ok := requestSnapshot["send"].(map[string]any); !ok {
		t.Fatalf("expected request snapshot to keep legacy send field, got %s", attempt.RequestSnapshot)
	}
	if !jsonContains(t, attempt.ResponseSnapshot, `"status_code":202`) || !jsonContains(t, attempt.ResponseSnapshot, `"message":"accepted"`) {
		t.Fatalf("expected response snapshot to keep outbound response, got %s", attempt.ResponseSnapshot)
	}
	var responseSnapshot map[string]any
	if err := json.Unmarshal(attempt.ResponseSnapshot, &responseSnapshot); err != nil {
		t.Fatalf("decode response snapshot: %v", err)
	}
	upstreamResponse, ok := responseSnapshot["upstream_response"].(map[string]any)
	if !ok || upstreamResponse["status_code"] != float64(http.StatusAccepted) {
		t.Fatalf("expected upstream_response with status 202, got %+v", responseSnapshot)
	}
	if _, ok := responseSnapshot["send"].(map[string]any); !ok {
		t.Fatalf("expected response snapshot to keep legacy send field, got %s", attempt.ResponseSnapshot)
	}
}

func TestWorkerUsesCapabilityTokenPlacementWhenChannelHasNoExplicitPlacement(t *testing.T) {
	var tokenQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenQuery = r.URL.Query().Get("access_token")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	store := newMemoryRepository()
	store.channels["channel-capability-token"] = provider.Channel{
		ID:               "channel-capability-token",
		ProviderType:     provider.ProviderWebhook,
		Name:             "Capability Token",
		Enabled:          true,
		ConcurrencyLimit: 1,
		TimeoutMS:        1000,
		SendConfig:       json.RawMessage(`{"method":"POST","url":"` + server.URL + `/send","recipient":{"location":"none"}}`),
	}
	store.capabilities[capabilityKey(provider.ProviderWebhook, "json")] = provider.Capability{
		ProviderType:  provider.ProviderWebhook,
		MessageType:   "json",
		TokenStrategy: json.RawMessage(`{"strategy":"static_token","placement":{"location":"query","field_name":"access_token"}}`),
		SuccessRule:   json.RawMessage(`{"type":"status_code","status_codes":[200]}`),
		RetryRule:     json.RawMessage(`{"status_codes":[408,429,500,502,503,504],"network_errors":true}`),
	}
	store.addAttempt(Attempt{ID: "attempt-capability-token", MessageID: "message-capability-token", ChannelID: "channel-capability-token", TemplateVersionID: "template-token", Status: StatusQueued})
	store.addJob(newSendJob("job-capability-token", "channel-capability-token", 3, time.Now().Add(-time.Second), SendMessageJobPayload{
		DeliveryAttemptID: "attempt-capability-token",
		MessageType:       "json",
		Token:             "capability-token",
		Body:              json.RawMessage(`{"title":"hello"}`),
	}))

	worker := NewWorker(store,
		WithWorkerID("sender-1"),
		WithHTTPClientFactory(func(timeout time.Duration) *http.Client {
			client := server.Client()
			client.Timeout = timeout
			return client
		}),
	)

	if err := worker.ProcessOne(context.Background(), store.jobs["job-capability-token"]); err != nil {
		t.Fatalf("process capability token job: %v", err)
	}
	if tokenQuery != "capability-token" {
		t.Fatalf("expected capability token placement in query, got %q", tokenQuery)
	}
	snapshot := decodeSnapshot(t, store.attempts["attempt-capability-token"].RequestSnapshot)
	tokenBehavior, ok := snapshot["token_behavior"].(map[string]any)
	if !ok || tokenBehavior["source"] != "capability.token_strategy" {
		t.Fatalf("expected token behavior source from capability, got %+v", snapshot)
	}
}

func TestWorkerCachesCapabilityTokenAndRefreshesOnInvalidTokenCode(t *testing.T) {
	var tokenRequests int32
	var sendRequests int32
	var sendTokens []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/token":
			count := atomic.AddInt32(&tokenRequests, 1)
			if r.URL.Query().Get("corpid") != "corp-1" || r.URL.Query().Get("corpsecret") != "secret-1" {
				t.Fatalf("unexpected token query: %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(fmt.Sprintf(`{"errcode":0,"access_token":"token-%d","expires_in":7200}`, count)))
		case "/cgi-bin/message/send":
			count := atomic.AddInt32(&sendRequests, 1)
			sendTokens = append(sendTokens, r.URL.Query().Get("access_token"))
			if count == 2 {
				_, _ = w.Write([]byte(`{"errcode":41001,"errmsg":"access_token missing"}`))
				return
			}
			_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	store := newMemoryRepository()
	store.channels["channel-wecom"] = provider.Channel{
		ID:               "channel-wecom",
		ProviderType:     provider.ProviderWeComApp,
		Name:             "WeCom App",
		Enabled:          true,
		ConcurrencyLimit: 1,
		TimeoutMS:        1000,
		AuthConfig:       json.RawMessage(`{"corpid":"corp-1","corpsecret":"secret-1"}`),
		SendConfig:       json.RawMessage(`{"base_url":"` + server.URL + `","agentid":1000001}`),
	}
	store.capabilities[capabilityKey(provider.ProviderWeComApp, "text")] = provider.Capability{
		ProviderType:     provider.ProviderWeComApp,
		MessageType:      "text",
		TokenStrategy:    json.RawMessage(`{"strategy":"client_credentials","cacheable":true,"token_url":"` + server.URL + `/token","request":{"method":"GET","query_fields":["corpid","corpsecret"]},"response_token_path":"access_token","response_expires_in_path":"expires_in","placement":{"location":"query","field_name":"access_token"},"refresh_on_json_codes":[41001,40014,42001]}`),
		SuccessRule:      json.RawMessage(`{"type":"json_field","status_codes":[200],"field":"errcode","equals":0}`),
		RetryRule:        json.RawMessage(`{"refresh_token_codes":[41001,40014,42001],"retryable_json_codes":[-1]}`),
		DefaultRateLimit: json.RawMessage(`{"qps":20}`),
	}
	for i := 1; i <= 3; i++ {
		attemptID := fmt.Sprintf("attempt-%d", i)
		jobID := fmt.Sprintf("job-%d", i)
		store.addAttempt(Attempt{ID: attemptID, MessageID: fmt.Sprintf("message-%d", i), ChannelID: "channel-wecom", Status: StatusQueued})
		store.addJob(newSendJob(jobID, "channel-wecom", 3, time.Now().Add(-time.Second), SendMessageJobPayload{
			DeliveryAttemptID: attemptID,
			MessageType:       "text",
			Recipient:         []any{"zhangsan"},
			Body:              json.RawMessage(`{"msgtype":"text","content":"hello"}`),
		}))
	}

	worker := NewWorker(store,
		WithWorkerID("sender-1"),
		WithHTTPClientFactory(func(timeout time.Duration) *http.Client {
			client := server.Client()
			client.Timeout = timeout
			return client
		}),
	)
	for i := 1; i <= 3; i++ {
		jobID := fmt.Sprintf("job-%d", i)
		if err := worker.ProcessOne(context.Background(), store.jobs[jobID]); err != nil {
			t.Fatalf("process %s: %v", jobID, err)
		}
		if got := store.attempts[fmt.Sprintf("attempt-%d", i)].Status; got != StatusSent {
			t.Fatalf("expected attempt %d sent, got %s", i, got)
		}
	}
	if tokenRequests != 2 {
		t.Fatalf("expected first token to be cached and refreshed once, got %d token requests", tokenRequests)
	}
	wantTokens := []string{"token-1", "token-1", "token-2", "token-2"}
	if strings.Join(sendTokens, ",") != strings.Join(wantTokens, ",") {
		t.Fatalf("unexpected send tokens: got %v want %v", sendTokens, wantTokens)
	}
	if !jsonContains(t, store.attempts["attempt-2"].ResponseSnapshot, `"token_refreshed":true`) {
		t.Fatalf("expected token refresh snapshot, got %s", store.attempts["attempt-2"].ResponseSnapshot)
	}
}

func TestWorkerSendsFeishuTextWithCachedTenantTokenAndRefreshRetry(t *testing.T) {
	var tokenRequests int32
	var sendRequests int32
	var sendAuthHeaders []string
	var sendBodies []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/open-apis/auth/v3/tenant_access_token/internal":
			count := atomic.AddInt32(&tokenRequests, 1)
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode token body: %v", err)
			}
			if body["app_id"] != "cli_1" || body["app_secret"] != "secret-1" {
				t.Fatalf("unexpected token body: %+v", body)
			}
			_, _ = w.Write([]byte(fmt.Sprintf(`{"code":0,"msg":"ok","tenant_access_token":"tenant-%d","expire":7200}`, count)))
		case "/open-apis/im/v1/messages":
			count := atomic.AddInt32(&sendRequests, 1)
			if r.URL.Query().Get("receive_id_type") != "open_id" {
				t.Fatalf("expected open_id receive_id_type, got %s", r.URL.RawQuery)
			}
			sendAuthHeaders = append(sendAuthHeaders, r.Header.Get("Authorization"))
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode send body: %v", err)
			}
			sendBodies = append(sendBodies, body)
			if count == 2 {
				_, _ = w.Write([]byte(`{"code":99991663,"msg":"token invalid"}`))
				return
			}
			_, _ = w.Write([]byte(`{"code":0,"msg":"ok","data":{"message_id":"om_1"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	store := newMemoryRepository()
	store.channels["channel-feishu"] = provider.Channel{
		ID:               "channel-feishu",
		ProviderType:     provider.ProviderFeishuRobot,
		Name:             "Feishu App Robot",
		Enabled:          true,
		ConcurrencyLimit: 1,
		TimeoutMS:        1000,
		AuthConfig:       json.RawMessage(`{"app_id":"cli_1","app_secret":"secret-1"}`),
		SendConfig:       json.RawMessage(`{"base_url":"` + server.URL + `/open-apis"}`),
	}
	store.capabilities[capabilityKey(provider.ProviderFeishuRobot, "text")] = provider.Capability{
		ProviderType:     provider.ProviderFeishuRobot,
		MessageType:      "text",
		TokenStrategy:    json.RawMessage(`{"strategy":"tenant_access_token","cacheable":true,"token_url":"` + server.URL + `/open-apis/auth/v3/tenant_access_token/internal","request":{"method":"POST","body_fields":["app_id","app_secret"]},"response_token_path":"tenant_access_token","response_expires_in_path":"expire","placement":{"location":"header","field_name":"Authorization","prefix":"Bearer "},"refresh_on_json_codes":[99991663,99991664]}`),
		SuccessRule:      json.RawMessage(`{"type":"json_field","status_codes":[200],"field":"code","equals":0}`),
		RetryRule:        json.RawMessage(`{"refresh_token_codes":[99991663,99991664],"retryable_json_codes":[99991663]}`),
		DefaultRateLimit: json.RawMessage(`{"qps":20}`),
	}
	for i := 1; i <= 3; i++ {
		attemptID := fmt.Sprintf("attempt-feishu-%d", i)
		jobID := fmt.Sprintf("job-feishu-%d", i)
		store.addAttempt(Attempt{ID: attemptID, MessageID: fmt.Sprintf("message-feishu-%d", i), ChannelID: "channel-feishu", Status: StatusQueued})
		store.addJob(newSendJob(jobID, "channel-feishu", 3, time.Now().Add(-time.Second), SendMessageJobPayload{
			DeliveryAttemptID: attemptID,
			MessageType:       "text",
			Recipient:         []any{map[string]any{"platform_ids": map[string]any{"feishu_open_id": "ou_123"}}},
			Body:              json.RawMessage(`{"text":"hello feishu"}`),
		}))
	}

	worker := NewWorker(store,
		WithWorkerID("sender-feishu"),
		WithHTTPClientFactory(func(timeout time.Duration) *http.Client {
			client := server.Client()
			client.Timeout = timeout
			return client
		}),
	)
	for i := 1; i <= 3; i++ {
		jobID := fmt.Sprintf("job-feishu-%d", i)
		if err := worker.ProcessOne(context.Background(), store.jobs[jobID]); err != nil {
			t.Fatalf("process %s: %v", jobID, err)
		}
		if got := store.attempts[fmt.Sprintf("attempt-feishu-%d", i)].Status; got != StatusSent {
			t.Fatalf("expected attempt %d sent, got %s", i, got)
		}
	}
	if tokenRequests != 2 {
		t.Fatalf("expected Feishu tenant token cached and refreshed once, got %d token requests", tokenRequests)
	}
	wantHeaders := []string{"Bearer tenant-1", "Bearer tenant-1", "Bearer tenant-2", "Bearer tenant-2"}
	if strings.Join(sendAuthHeaders, ",") != strings.Join(wantHeaders, ",") {
		t.Fatalf("unexpected auth headers: got %v want %v", sendAuthHeaders, wantHeaders)
	}
	for _, body := range sendBodies {
		if body["receive_id"] != "ou_123" || body["msg_type"] != "text" {
			t.Fatalf("unexpected Feishu send body: %+v", body)
		}
		contentString, ok := body["content"].(string)
		if !ok {
			t.Fatalf("expected serialized Feishu content string, got %#v", body["content"])
		}
		var content map[string]string
		if err := json.Unmarshal([]byte(contentString), &content); err != nil {
			t.Fatalf("decode Feishu content string: %v", err)
		}
		if content["text"] != "hello feishu" {
			t.Fatalf("unexpected Feishu text content: %+v", content)
		}
	}
	if !jsonContains(t, store.attempts["attempt-feishu-2"].ResponseSnapshot, `"token_refreshed":true`) {
		t.Fatalf("expected token refresh snapshot, got %s", store.attempts["attempt-feishu-2"].ResponseSnapshot)
	}
}

func TestWorkerClassifiesSuccessWithCapabilityRules(t *testing.T) {
	t.Run("json_field success records success rule", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"errcode":0,"message":"ok"}`))
		}))
		defer server.Close()

		store := newMemoryRepository()
		store.channels["channel-json-success"] = provider.Channel{
			ID:               "channel-json-success",
			ProviderType:     provider.ProviderWeComApp,
			Name:             "JSON Success",
			Enabled:          true,
			ConcurrencyLimit: 1,
			TimeoutMS:        1000,
			SendConfig:       json.RawMessage(`{"method":"POST","url":"` + server.URL + `/send","recipient":{"location":"none"}}`),
		}
		store.capabilities[capabilityKey(provider.ProviderWeComApp, "text")] = provider.Capability{
			ProviderType: provider.ProviderWeComApp,
			MessageType:  "text",
			SuccessRule:  json.RawMessage(`{"type":"json_field","status_codes":[200],"field":"errcode","equals":0}`),
			RetryRule:    json.RawMessage(`{"status_codes":[408,429,500,502,503,504],"network_errors":true}`),
		}
		store.addAttempt(Attempt{ID: "attempt-json-success", MessageID: "message-json-success", ChannelID: "channel-json-success", TemplateVersionID: "template-json-success", Status: StatusQueued})
		store.addJob(newSendJob("job-json-success", "channel-json-success", 3, time.Now().Add(-time.Second), SendMessageJobPayload{
			DeliveryAttemptID: "attempt-json-success",
			MessageType:       "text",
			Body:              json.RawMessage(`{"content":"hello"}`),
		}))

		worker := NewWorker(store,
			WithWorkerID("sender-1"),
			WithHTTPClientFactory(func(timeout time.Duration) *http.Client {
				client := server.Client()
				client.Timeout = timeout
				return client
			}),
		)

		if err := worker.ProcessOne(context.Background(), store.jobs["job-json-success"]); err != nil {
			t.Fatalf("process json success job: %v", err)
		}
		attempt := store.attempts["attempt-json-success"]
		if attempt.Status != StatusSent {
			t.Fatalf("expected json_field success attempt sent, got %s", attempt.Status)
		}
		snapshot := decodeSnapshot(t, attempt.ResponseSnapshot)
		successRule, ok := snapshot["success_rule"].(map[string]any)
		if !ok || successRule["source"] != "capability.success_rule" {
			t.Fatalf("expected response snapshot success_rule source, got %+v", snapshot)
		}
	})

	t.Run("status_code rule can fail a 2xx response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"queued"}`))
		}))
		defer server.Close()

		store := newMemoryRepository()
		store.channels["channel-status-fail"] = provider.Channel{
			ID:               "channel-status-fail",
			ProviderType:     provider.ProviderWebhook,
			Name:             "Status Fail",
			Enabled:          true,
			ConcurrencyLimit: 1,
			TimeoutMS:        1000,
			RetryPolicy:      json.RawMessage(`{"max_attempts":3,"delay_ms":10}`),
			SendConfig:       json.RawMessage(`{"method":"POST","url":"` + server.URL + `/send","recipient":{"location":"none"}}`),
		}
		store.capabilities[capabilityKey(provider.ProviderWebhook, "json")] = provider.Capability{
			ProviderType: provider.ProviderWebhook,
			MessageType:  "json",
			SuccessRule:  json.RawMessage(`{"type":"status_code","status_codes":[202]}`),
			RetryRule:    json.RawMessage(`{"status_codes":[408,429,500,502,503,504],"network_errors":true}`),
		}
		store.addAttempt(Attempt{ID: "attempt-status-fail", MessageID: "message-status-fail", ChannelID: "channel-status-fail", TemplateVersionID: "template-status-fail", Status: StatusQueued})
		store.addJob(newSendJob("job-status-fail", "channel-status-fail", 3, time.Now().Add(-time.Second), SendMessageJobPayload{
			DeliveryAttemptID: "attempt-status-fail",
			MessageType:       "json",
			Body:              json.RawMessage(`{"title":"hello"}`),
		}))

		worker := NewWorker(store,
			WithWorkerID("sender-1"),
			WithHTTPClientFactory(func(timeout time.Duration) *http.Client {
				client := server.Client()
				client.Timeout = timeout
				return client
			}),
		)

		if err := worker.ProcessOne(context.Background(), store.jobs["job-status-fail"]); err != nil {
			t.Fatalf("process status fail job: %v", err)
		}
		if got := store.attempts["attempt-status-fail"].Status; got != StatusFailed {
			t.Fatalf("expected status_code rule mismatch to fail, got %s", got)
		}
		if store.jobs["job-status-fail"].Status != queue.JobStatusDead {
			t.Fatalf("expected non-retryable 2xx success-rule mismatch to dead-letter, got %s", store.jobs["job-status-fail"].Status)
		}
	})
}

func TestWorkerClassifiesRetryWithCapabilityRules(t *testing.T) {
	t.Run("non-retryable status dead-letters without consuming max attempts", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"errcode":40003}`))
		}))
		defer server.Close()

		store := newMemoryRepository()
		store.channels["channel-nonretry-status"] = provider.Channel{
			ID:               "channel-nonretry-status",
			ProviderType:     provider.ProviderWeComApp,
			Name:             "Non Retry Status",
			Enabled:          true,
			ConcurrencyLimit: 1,
			TimeoutMS:        1000,
			RetryPolicy:      json.RawMessage(`{"max_attempts":3,"delay_ms":10}`),
			SendConfig:       json.RawMessage(`{"method":"POST","url":"` + server.URL + `/send","recipient":{"location":"none"}}`),
		}
		store.capabilities[capabilityKey(provider.ProviderWeComApp, "text")] = provider.Capability{
			ProviderType: provider.ProviderWeComApp,
			MessageType:  "text",
			SuccessRule:  json.RawMessage(`{"type":"json_field","status_codes":[200],"field":"errcode","equals":0}`),
			RetryRule:    json.RawMessage(`{"status_codes":[408,429,500,502,503,504],"network_errors":true,"non_retryable_status_codes":[400]}`),
		}
		store.addAttempt(Attempt{ID: "attempt-nonretry-status", MessageID: "message-nonretry-status", ChannelID: "channel-nonretry-status", TemplateVersionID: "template-nonretry-status", Status: StatusQueued})
		store.addJob(newSendJob("job-nonretry-status", "channel-nonretry-status", 3, time.Now().Add(-time.Second), SendMessageJobPayload{
			DeliveryAttemptID: "attempt-nonretry-status",
			MessageType:       "text",
			Body:              json.RawMessage(`{"content":"hello"}`),
		}))

		worker := NewWorker(store,
			WithWorkerID("sender-1"),
			WithHTTPClientFactory(func(timeout time.Duration) *http.Client {
				client := server.Client()
				client.Timeout = timeout
				return client
			}),
		)

		if err := worker.ProcessOne(context.Background(), store.jobs["job-nonretry-status"]); err != nil {
			t.Fatalf("process non-retry status job: %v", err)
		}
		attempt := store.attempts["attempt-nonretry-status"]
		if attempt.DeadLetteredAt == nil {
			t.Fatalf("expected non-retryable status to dead-letter immediately, got %+v", attempt)
		}
		if store.jobs["job-nonretry-status"].Status != queue.JobStatusDead {
			t.Fatalf("expected non-retryable status job dead, got %s", store.jobs["job-nonretry-status"].Status)
		}
	})

	t.Run("non-retryable json code dead-letters", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"errcode":40003}`))
		}))
		defer server.Close()

		store := newMemoryRepository()
		store.channels["channel-nonretry-json"] = provider.Channel{
			ID:               "channel-nonretry-json",
			ProviderType:     provider.ProviderWeComApp,
			Name:             "Non Retry JSON",
			Enabled:          true,
			ConcurrencyLimit: 1,
			TimeoutMS:        1000,
			RetryPolicy:      json.RawMessage(`{"max_attempts":3,"delay_ms":10}`),
			SendConfig:       json.RawMessage(`{"method":"POST","url":"` + server.URL + `/send","recipient":{"location":"none"}}`),
		}
		store.capabilities[capabilityKey(provider.ProviderWeComApp, "text")] = provider.Capability{
			ProviderType: provider.ProviderWeComApp,
			MessageType:  "text",
			SuccessRule:  json.RawMessage(`{"type":"json_field","status_codes":[200],"field":"errcode","equals":0}`),
			RetryRule:    json.RawMessage(`{"status_codes":[408,429,500,502,503,504],"network_errors":true,"non_retryable_json_codes":[40003]}`),
		}
		store.addAttempt(Attempt{ID: "attempt-nonretry-json", MessageID: "message-nonretry-json", ChannelID: "channel-nonretry-json", TemplateVersionID: "template-nonretry-json", Status: StatusQueued})
		store.addJob(newSendJob("job-nonretry-json", "channel-nonretry-json", 3, time.Now().Add(-time.Second), SendMessageJobPayload{
			DeliveryAttemptID: "attempt-nonretry-json",
			MessageType:       "text",
			Body:              json.RawMessage(`{"content":"hello"}`),
		}))

		worker := NewWorker(store,
			WithWorkerID("sender-1"),
			WithHTTPClientFactory(func(timeout time.Duration) *http.Client {
				client := server.Client()
				client.Timeout = timeout
				return client
			}),
		)

		if err := worker.ProcessOne(context.Background(), store.jobs["job-nonretry-json"]); err != nil {
			t.Fatalf("process non-retry json job: %v", err)
		}
		attempt := store.attempts["attempt-nonretry-json"]
		if attempt.DeadLetteredAt == nil {
			t.Fatalf("expected non-retryable json code to dead-letter, got %+v", attempt)
		}
	})

	t.Run("retryable status enters retry", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"errcode":45009}`))
		}))
		defer server.Close()

		store := newMemoryRepository()
		store.channels["channel-retry-status"] = provider.Channel{
			ID:               "channel-retry-status",
			ProviderType:     provider.ProviderWeComApp,
			Name:             "Retry Status",
			Enabled:          true,
			ConcurrencyLimit: 1,
			TimeoutMS:        1000,
			RetryPolicy:      json.RawMessage(`{"max_attempts":3,"delay_ms":25}`),
			SendConfig:       json.RawMessage(`{"method":"POST","url":"` + server.URL + `/send","recipient":{"location":"none"}}`),
		}
		store.capabilities[capabilityKey(provider.ProviderWeComApp, "text")] = provider.Capability{
			ProviderType: provider.ProviderWeComApp,
			MessageType:  "text",
			SuccessRule:  json.RawMessage(`{"type":"json_field","status_codes":[200],"field":"errcode","equals":0}`),
			RetryRule:    json.RawMessage(`{"status_codes":[408,429,500,502,503,504],"network_errors":true,"retryable_json_codes":[45009],"non_retryable_status_classes":[400]}`),
		}
		store.addAttempt(Attempt{ID: "attempt-retry-status", MessageID: "message-retry-status", ChannelID: "channel-retry-status", TemplateVersionID: "template-retry-status", Status: StatusQueued})
		store.addJob(newSendJob("job-retry-status", "channel-retry-status", 3, time.Now().Add(-time.Second), SendMessageJobPayload{
			DeliveryAttemptID: "attempt-retry-status",
			MessageType:       "text",
			Body:              json.RawMessage(`{"content":"hello"}`),
		}))

		worker := NewWorker(store,
			WithWorkerID("sender-1"),
			WithHTTPClientFactory(func(timeout time.Duration) *http.Client {
				client := server.Client()
				client.Timeout = timeout
				return client
			}),
		)

		if err := worker.ProcessOne(context.Background(), store.jobs["job-retry-status"]); err != nil {
			t.Fatalf("process retry status job: %v", err)
		}
		attempt := store.attempts["attempt-retry-status"]
		if attempt.NextRetryAt == nil || store.jobs["job-retry-status"].Status != queue.JobStatusQueued {
			t.Fatalf("expected retryable status to requeue, attempt=%+v job=%+v", attempt, store.jobs["job-retry-status"])
		}
		snapshot := decodeSnapshot(t, attempt.ResponseSnapshot)
		retryRule, ok := snapshot["retry_rule"].(map[string]any)
		if !ok || retryRule["source"] != "capability.retry_rule" || retryRule["decision"] != "retry" {
			t.Fatalf("expected retry_rule snapshot with retry decision, got %+v", snapshot)
		}
	})
}

func TestWorkerFailuresRetryAndDeadLetter(t *testing.T) {
	t.Run("timeout retries", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(80 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok":true}`))
		}))
		defer server.Close()

		store := newMemoryRepository()
		store.channels["channel-timeout"] = provider.Channel{
			ID:               "channel-timeout",
			Name:             "Timeout Channel",
			Enabled:          true,
			ConcurrencyLimit: 1,
			TimeoutMS:        20,
			RetryPolicy:      json.RawMessage(`{"max_attempts":2,"delay_ms":25}`),
			SendConfig:       json.RawMessage(`{"method":"POST","url":"` + server.URL + `/send"}`),
		}
		store.addAttempt(Attempt{ID: "attempt-timeout", MessageID: "message-timeout", ChannelID: "channel-timeout", Status: StatusQueued})
		store.addJob(newSendJob("job-timeout", "channel-timeout", 2, time.Now().Add(-time.Second), SendMessageJobPayload{
			DeliveryAttemptID: "attempt-timeout",
			Body:              json.RawMessage(`{"title":"slow"}`),
		}))

		worker := NewWorker(store,
			WithWorkerID("sender-1"),
			WithHTTPClientFactory(func(timeout time.Duration) *http.Client {
				client := server.Client()
				client.Timeout = timeout
				return client
			}),
		)

		if _, err := worker.ProcessBatch(context.Background(), 1); err != nil {
			t.Fatalf("process timeout batch: %v", err)
		}

		attempt := store.attempts["attempt-timeout"]
		if attempt.Status != StatusFailed {
			t.Fatalf("expected timeout attempt failed for retry, got %s", attempt.Status)
		}
		if attempt.NextRetryAt == nil {
			t.Fatalf("expected timeout attempt retry schedule")
		}
		if store.jobs["job-timeout"].Status != queue.JobStatusQueued {
			t.Fatalf("expected timeout job requeued, got %s", store.jobs["job-timeout"].Status)
		}
	})

	t.Run("non-2xx exhausts into dead letter", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`{"error":"bad gateway"}`))
		}))
		defer server.Close()

		store := newMemoryRepository()
		store.channels["channel-fail"] = provider.Channel{
			ID:               "channel-fail",
			Name:             "Fail Channel",
			Enabled:          true,
			ConcurrencyLimit: 1,
			TimeoutMS:        200,
			RetryPolicy:      json.RawMessage(`{"max_attempts":2,"delay_ms":10}`),
			SendConfig:       json.RawMessage(`{"method":"POST","url":"` + server.URL + `/send"}`),
		}
		store.addAttempt(Attempt{ID: "attempt-fail", MessageID: "message-fail", ChannelID: "channel-fail", Status: StatusQueued})
		store.addJob(newSendJob("job-fail", "channel-fail", 2, time.Now().Add(-time.Second), SendMessageJobPayload{
			DeliveryAttemptID: "attempt-fail",
			Body:              json.RawMessage(`{"title":"fail"}`),
		}))

		worker := NewWorker(store,
			WithWorkerID("sender-1"),
			WithHTTPClientFactory(func(timeout time.Duration) *http.Client {
				client := server.Client()
				client.Timeout = timeout
				return client
			}),
		)

		if _, err := worker.ProcessBatch(context.Background(), 1); err != nil {
			t.Fatalf("process first failure batch: %v", err)
		}
		if store.jobs["job-fail"].Status != queue.JobStatusQueued {
			t.Fatalf("expected first non-2xx to requeue, got %s", store.jobs["job-fail"].Status)
		}

		job := store.jobs["job-fail"]
		job.RunAt = time.Now().Add(-time.Second)
		store.jobs["job-fail"] = job
		if _, err := worker.ProcessBatch(context.Background(), 1); err != nil {
			t.Fatalf("process second failure batch: %v", err)
		}

		attempt := store.attempts["attempt-fail"]
		if attempt.DeadLetteredAt == nil {
			t.Fatalf("expected dead-letter timestamp after retry exhaustion")
		}
		if store.jobs["job-fail"].Status != queue.JobStatusDead {
			t.Fatalf("expected exhausted job dead, got %s", store.jobs["job-fail"].Status)
		}
		if len(store.deadLetters) != 1 {
			t.Fatalf("expected one dead-letter record, got %d", len(store.deadLetters))
		}
	})
}

func TestWorkerPerChannelIsolationForConcurrencyAndRateLimit(t *testing.T) {
	type marker struct {
		name string
		at   time.Time
	}

	var markersMu sync.Mutex
	markers := []marker{}
	record := func(name string) {
		markersMu.Lock()
		defer markersMu.Unlock()
		markers = append(markers, marker{name: name, at: time.Now()})
	}

	var slowCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/slow":
			order := atomic.AddInt32(&slowCount, 1)
			record("slow-start-" + string(rune('0'+order)))
			time.Sleep(80 * time.Millisecond)
			record("slow-done-" + string(rune('0'+order)))
		case "/fast":
			record("fast-start")
			record("fast-done")
		default:
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	store := newMemoryRepository()
	store.channels["channel-slow"] = provider.Channel{
		ID:               "channel-slow",
		Name:             "Slow Channel",
		Enabled:          true,
		ConcurrencyLimit: 1,
		TimeoutMS:        1000,
		RateLimitConfig:  json.RawMessage(`{"enabled":true,"qps":4}`),
		SendConfig:       json.RawMessage(`{"method":"POST","url":"` + server.URL + `/slow"}`),
	}
	store.channels["channel-fast"] = provider.Channel{
		ID:               "channel-fast",
		Name:             "Fast Channel",
		Enabled:          true,
		ConcurrencyLimit: 1,
		TimeoutMS:        1000,
		SendConfig:       json.RawMessage(`{"method":"POST","url":"` + server.URL + `/fast"}`),
	}

	store.addAttempt(Attempt{ID: "attempt-slow-1", MessageID: "message-slow-1", ChannelID: "channel-slow", Status: StatusQueued})
	store.addAttempt(Attempt{ID: "attempt-slow-2", MessageID: "message-slow-2", ChannelID: "channel-slow", Status: StatusQueued})
	store.addAttempt(Attempt{ID: "attempt-fast-1", MessageID: "message-fast-1", ChannelID: "channel-fast", Status: StatusQueued})

	store.addJob(newSendJob("job-slow-1", "channel-slow", 3, time.Now().Add(-time.Second), SendMessageJobPayload{DeliveryAttemptID: "attempt-slow-1", Body: json.RawMessage(`{"title":"slow-1"}`)}))
	store.addJob(newSendJob("job-slow-2", "channel-slow", 3, time.Now().Add(-time.Second), SendMessageJobPayload{DeliveryAttemptID: "attempt-slow-2", Body: json.RawMessage(`{"title":"slow-2"}`)}))
	store.addJob(newSendJob("job-fast-1", "channel-fast", 3, time.Now().Add(-time.Second), SendMessageJobPayload{DeliveryAttemptID: "attempt-fast-1", Body: json.RawMessage(`{"title":"fast-1"}`)}))

	worker := NewWorker(store,
		WithWorkerID("sender-1"),
		WithHTTPClientFactory(func(timeout time.Duration) *http.Client {
			client := server.Client()
			client.Timeout = timeout
			return client
		}),
	)

	if _, err := worker.ProcessBatch(context.Background(), 3); err != nil {
		t.Fatalf("process isolation batch: %v", err)
	}

	indexOf := func(name string) int {
		markersMu.Lock()
		defer markersMu.Unlock()
		for idx, marker := range markers {
			if marker.name == name {
				return idx
			}
		}
		return -1
	}

	if indexOf("fast-done") == -1 || indexOf("slow-start-2") == -1 {
		t.Fatalf("expected fast and second slow markers, got %+v", markers)
	}
	if indexOf("fast-done") > indexOf("slow-start-2") {
		t.Fatalf("expected fast channel to finish before slow channel second send starts, got %+v", markers)
	}
}

func TestWorkerProcessBatchFairlyClaimsAcrossChannels(t *testing.T) {
	type marker struct {
		name string
		at   time.Time
	}

	var markersMu sync.Mutex
	markers := []marker{}
	record := func(name string) {
		markersMu.Lock()
		defer markersMu.Unlock()
		markers = append(markers, marker{name: name, at: time.Now()})
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/slow":
			record("slow-start")
			time.Sleep(80 * time.Millisecond)
			record("slow-done")
		case "/fast":
			record("fast-start")
			record("fast-done")
		default:
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	store := newMemoryRepository()
	store.channels["channel-slow"] = provider.Channel{
		ID:               "channel-slow",
		Name:             "Slow Channel",
		Enabled:          true,
		ConcurrencyLimit: 1,
		TimeoutMS:        1000,
		SendConfig:       json.RawMessage(`{"method":"POST","url":"` + server.URL + `/slow"}`),
	}
	store.channels["channel-fast"] = provider.Channel{
		ID:               "channel-fast",
		Name:             "Fast Channel",
		Enabled:          true,
		ConcurrencyLimit: 1,
		TimeoutMS:        1000,
		SendConfig:       json.RawMessage(`{"method":"POST","url":"` + server.URL + `/fast"}`),
	}

	for i := 1; i <= 4; i++ {
		attemptID := "attempt-slow-" + string(rune('0'+i))
		jobID := "job-slow-" + string(rune('0'+i))
		store.addAttempt(Attempt{ID: attemptID, MessageID: "message-slow-" + string(rune('0'+i)), ChannelID: "channel-slow", Status: StatusQueued})
		store.addJob(newSendJob(jobID, "channel-slow", 3, time.Now().Add(-time.Second), SendMessageJobPayload{
			DeliveryAttemptID: attemptID,
			Body:              json.RawMessage(`{"title":"slow"}`),
		}))
	}
	store.addAttempt(Attempt{ID: "attempt-fast-1", MessageID: "message-fast-1", ChannelID: "channel-fast", Status: StatusQueued})
	store.addJob(newSendJob("job-fast-1", "channel-fast", 3, time.Now().Add(-time.Second), SendMessageJobPayload{
		DeliveryAttemptID: "attempt-fast-1",
		Body:              json.RawMessage(`{"title":"fast"}`),
	}))

	worker := NewWorker(store,
		WithWorkerID("sender-1"),
		WithHTTPClientFactory(func(timeout time.Duration) *http.Client {
			client := server.Client()
			client.Timeout = timeout
			return client
		}),
	)

	processed, err := worker.ProcessBatch(context.Background(), 4)
	if err != nil {
		t.Fatalf("process fair claim batch: %v", err)
	}
	if processed != 4 {
		t.Fatalf("expected 4 processed jobs, got %d", processed)
	}
	if store.attempts["attempt-fast-1"].Status != StatusSent {
		t.Fatalf("expected fast-channel job to be included in the same batch, got %s", store.attempts["attempt-fast-1"].Status)
	}
	if store.jobs["job-slow-4"].Status != queue.JobStatusQueued {
		t.Fatalf("expected one slow-channel job to remain queued after fair claim, got %s", store.jobs["job-slow-4"].Status)
	}

	indexOf := func(name string) int {
		markersMu.Lock()
		defer markersMu.Unlock()
		for idx, marker := range markers {
			if marker.name == name {
				return idx
			}
		}
		return -1
	}
	if indexOf("fast-done") == -1 {
		t.Fatalf("expected fast-channel send to execute in the claimed batch, got %+v", markers)
	}
}

func TestRateLimitFromRequiresQPS(t *testing.T) {
	cfg := rateLimitFrom(json.RawMessage(`{"enabled":true}`))

	if cfg.Enabled {
		t.Fatalf("rate limiting without qps must stay disabled: %+v", cfg)
	}
}

func jsonContains(t *testing.T, raw json.RawMessage, needle string) bool {
	t.Helper()
	return strings.Contains(string(raw), needle)
}

func decodeSnapshot(t *testing.T, raw json.RawMessage) map[string]any {
	t.Helper()
	var snapshot map[string]any
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		t.Fatalf("decode snapshot: %v; raw=%s", err, raw)
	}
	return snapshot
}

func capabilityKey(providerType provider.ProviderType, messageType string) string {
	return string(providerType) + "::" + strings.TrimSpace(messageType)
}

type memoryRepository struct {
	mu           sync.Mutex
	jobs         map[string]queue.Job
	jobOrder     []string
	channels     map[string]provider.Channel
	capabilities map[string]provider.Capability
	attempts     map[string]Attempt
	dedupe       map[string]string
	deadLetters  []DeadLetterRecord
}

func newMemoryRepository() *memoryRepository {
	return &memoryRepository{
		jobs:         map[string]queue.Job{},
		channels:     map[string]provider.Channel{},
		capabilities: map[string]provider.Capability{},
		attempts:     map[string]Attempt{},
		dedupe:       map[string]string{},
	}
}

func (m *memoryRepository) addAttempt(attempt Attempt) {
	m.attempts[attempt.ID] = attempt
}

func (m *memoryRepository) addJob(job queue.Job) {
	m.jobs[job.ID] = job
	m.jobOrder = append(m.jobOrder, job.ID)
}

func (m *memoryRepository) ClaimSendJobs(_ context.Context, params queue.ClaimParams) ([]queue.Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	byChannel := map[string][]string{}
	for _, jobID := range m.jobOrder {
		job := m.jobs[jobID]
		if job.Status != queue.JobStatusQueued || job.RunAt.After(params.Now) || job.Type != queue.JobTypeSendMessage {
			continue
		}
		channelID := job.ChannelID
		if channelID == "" {
			channelID = job.ID
		}
		byChannel[channelID] = append(byChannel[channelID], jobID)
	}

	claimed := make([]queue.Job, 0, params.Limit)
	for round := 0; len(claimed) < params.Limit; round++ {
		progressed := false
		for _, jobID := range m.jobOrder {
			if len(claimed) >= params.Limit {
				break
			}
			job := m.jobs[jobID]
			channelID := job.ChannelID
			if channelID == "" {
				channelID = job.ID
			}
			channelJobs := byChannel[channelID]
			if round >= len(channelJobs) || channelJobs[round] != jobID {
				continue
			}
			progressed = true
			job.Status = queue.JobStatusProcessing
			job.Attempts++
			job.LockedBy = params.WorkerID
			now := params.Now
			job.LockedAt = &now
			job.HeartbeatAt = &now
			m.jobs[jobID] = job
			claimed = append(claimed, job)
		}
		if !progressed {
			break
		}
	}
	return claimed, nil
}

func (m *memoryRepository) ClaimJobs(_ context.Context, _ queue.ClaimParams) ([]queue.Job, error) {
	return nil, errors.New("ClaimJobs should not be used by delivery worker")
}

func (m *memoryRepository) GetChannel(_ context.Context, id string) (provider.Channel, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	channel, ok := m.channels[id]
	if !ok {
		return provider.Channel{}, provider.ErrNotFound
	}
	return channel, nil
}

func (m *memoryRepository) GetProviderCapability(_ context.Context, providerType provider.ProviderType, messageType string) (provider.Capability, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	capability, ok := m.capabilities[capabilityKey(providerType, messageType)]
	if !ok {
		return provider.Capability{}, provider.ErrNotFound
	}
	return capability, nil
}

func (m *memoryRepository) GetAttempt(_ context.Context, attemptID string) (Attempt, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	attempt, ok := m.attempts[attemptID]
	if !ok {
		return Attempt{}, errors.New("attempt not found")
	}
	return attempt, nil
}

func (m *memoryRepository) MarkAttemptProcessing(_ context.Context, params MarkAttemptProcessingParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	attempt := m.attempts[params.AttemptID]
	attempt.Status = StatusProcessing
	attempt.AttemptNo = params.AttemptNo
	attempt.StartedAt = &params.StartedAt
	m.attempts[params.AttemptID] = attempt
	return nil
}

func (m *memoryRepository) InsertSendDedupeKey(_ context.Context, params SendDedupeParams) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := params.ChannelID + "::" + params.DedupeKey
	if _, ok := m.dedupe[key]; ok {
		return false, nil
	}
	m.dedupe[key] = params.MessageID
	return true, nil
}

func (m *memoryRepository) CompleteDelivery(_ context.Context, params CompleteDeliveryParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	attempt := m.attempts[params.AttemptID]
	attempt.Status = params.Status
	attempt.RequestSnapshot = append(json.RawMessage(nil), params.RequestSnapshot...)
	attempt.ResponseSnapshot = append(json.RawMessage(nil), params.ResponseSnapshot...)
	attempt.DurationMS = params.DurationMS
	attempt.FinishedAt = &params.FinishedAt
	m.attempts[params.AttemptID] = attempt

	job := m.jobs[params.JobID]
	job.Status = queue.JobStatusDone
	job.LockedBy = ""
	job.LockedAt = nil
	job.HeartbeatAt = nil
	job.FinishedAt = &params.FinishedAt
	duration := params.DurationMS
	job.DurationMS = &duration
	m.jobs[params.JobID] = job
	return nil
}

func (m *memoryRepository) RetryDelivery(_ context.Context, params RetryDeliveryParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	attempt := m.attempts[params.AttemptID]
	attempt.Status = StatusFailed
	attempt.ErrorCode = params.ErrorCode
	attempt.ErrorMessage = params.ErrorMessage
	attempt.RequestSnapshot = append(json.RawMessage(nil), params.RequestSnapshot...)
	attempt.ResponseSnapshot = append(json.RawMessage(nil), params.ResponseSnapshot...)
	attempt.DurationMS = params.DurationMS
	attempt.NextRetryAt = &params.RetryAt
	attempt.FinishedAt = &params.FinishedAt
	m.attempts[params.AttemptID] = attempt

	job := m.jobs[params.JobID]
	job.Status = queue.JobStatusQueued
	job.RunAt = params.RetryAt
	job.LastError = params.ErrorMessage
	job.LockedBy = ""
	job.LockedAt = nil
	job.HeartbeatAt = nil
	m.jobs[params.JobID] = job
	return nil
}

func (m *memoryRepository) DeadLetterDelivery(_ context.Context, params DeadLetterDeliveryParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	attempt := m.attempts[params.AttemptID]
	attempt.Status = StatusFailed
	attempt.ErrorCode = params.ErrorCode
	attempt.ErrorMessage = params.ErrorMessage
	attempt.RequestSnapshot = append(json.RawMessage(nil), params.RequestSnapshot...)
	attempt.ResponseSnapshot = append(json.RawMessage(nil), params.ResponseSnapshot...)
	attempt.DurationMS = params.DurationMS
	attempt.DeadLetteredAt = &params.FinishedAt
	attempt.FinishedAt = &params.FinishedAt
	m.attempts[params.AttemptID] = attempt

	job := m.jobs[params.JobID]
	job.Status = queue.JobStatusDead
	job.LastError = params.ErrorMessage
	job.LockedBy = ""
	job.LockedAt = nil
	job.HeartbeatAt = nil
	finishedAt := params.FinishedAt
	job.FinishedAt = &finishedAt
	m.jobs[params.JobID] = job

	m.deadLetters = append(m.deadLetters, DeadLetterRecord{
		JobID:        params.JobID,
		ChannelID:    job.ChannelID,
		ErrorCode:    params.ErrorCode,
		ErrorMessage: params.ErrorMessage,
	})
	return nil
}

func newSendJob(id string, channelID string, maxAttempts int, runAt time.Time, payload SendMessageJobPayload) queue.Job {
	raw, _ := json.Marshal(payload)
	return queue.Job{
		ID:          id,
		Type:        queue.JobTypeSendMessage,
		Status:      queue.JobStatusQueued,
		Payload:     raw,
		RunAt:       runAt,
		MaxAttempts: maxAttempts,
		ChannelID:   channelID,
	}
}
