package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"mvp-push-gateway/backend/internal/deadletter"
	httpapi "mvp-push-gateway/backend/internal/http"
)

func TestDeadLetterHandlersListAndBatchActions(t *testing.T) {
	now := time.Date(2026, 6, 3, 10, 0, 0, 0, time.UTC)
	service := &fakeDeadLetterService{
		listResult: deadletter.ListResult{
			Items: []deadletter.Job{
				{
					ID:             "dead-1",
					JobID:          "job-1",
					TraceID:        "trace-dead-1",
					Type:           "send_message",
					ChannelName:    "Webhook 生产",
					ErrorCode:      "MGP-SEND-004",
					ErrorMessage:   "upstream timeout",
					Attempts:       3,
					DeadLetteredAt: now,
				},
			},
			Total: 1,
			Limit: 50,
		},
		replayResult: deadletter.BatchResult{Processed: 1, IDs: []string{"dead-1"}},
		handleResult: deadletter.BatchResult{Processed: 1, IDs: []string{"dead-1"}},
		deleteResult: deadletter.BatchResult{Processed: 1, IDs: []string{"dead-1"}},
	}
	handler := httpapi.NewHandler(
		testConfig(),
		httpapi.WithAuthService(fakeAuthService{authenticatedToken: "admin-session"}),
		httpapi.WithDeadLetterService(service),
	)

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/dead-letters?keyword=trace-dead-1", nil)
	setAdminSessionCookie(listReq, "admin-session")
	listRec := httptest.NewRecorder()
	handler.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected list status 200, got %d body=%s", listRec.Code, listRec.Body.String())
	}
	var listBody struct {
		DeadLetters []struct {
			ID           string `json:"id"`
			TraceID      string `json:"trace_id"`
			ChannelName  string `json:"channel_name"`
			ErrorMessage string `json:"error_message"`
		} `json:"dead_letters"`
	}
	if err := json.NewDecoder(listRec.Body).Decode(&listBody); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(listBody.DeadLetters) != 1 || listBody.DeadLetters[0].ID != "dead-1" || listBody.DeadLetters[0].ChannelName != "Webhook 生产" {
		t.Fatalf("unexpected list response: %+v", listBody)
	}
	if listBody.DeadLetters[0].TraceID != "trace-dead-1" || service.listFilter.Keyword != "trace-dead-1" {
		t.Fatalf("expected trace id response and keyword filter, body=%+v filter=%+v", listBody, service.listFilter)
	}

	replayReq := deadLetterJSONRequest(t, http.MethodPost, "/api/v1/dead-letters/batch-replay", map[string]any{"ids": []string{"dead-1"}})
	setAdminSessionCookie(replayReq, "admin-session")
	replayRec := httptest.NewRecorder()
	handler.ServeHTTP(replayRec, replayReq)
	if replayRec.Code != http.StatusOK {
		t.Fatalf("expected replay status 200, got %d body=%s", replayRec.Code, replayRec.Body.String())
	}
	if service.replayIDs[0] != "dead-1" {
		t.Fatalf("expected replay id to reach service, got %+v", service.replayIDs)
	}

	handleReq := deadLetterJSONRequest(t, http.MethodPost, "/api/v1/dead-letters/batch-handle", map[string]any{"ids": []string{"dead-1"}, "reason": "manual"})
	setAdminSessionCookie(handleReq, "admin-session")
	handleRec := httptest.NewRecorder()
	handler.ServeHTTP(handleRec, handleReq)
	if handleRec.Code != http.StatusOK {
		t.Fatalf("expected handle status 200, got %d body=%s", handleRec.Code, handleRec.Body.String())
	}
	if service.handleReason != "manual" || service.handleIDs[0] != "dead-1" {
		t.Fatalf("expected handle input to reach service, ids=%+v reason=%q", service.handleIDs, service.handleReason)
	}

	deleteReq := deadLetterJSONRequest(t, http.MethodPost, "/api/v1/dead-letters/batch-delete", map[string]any{"ids": []string{"dead-1"}})
	setAdminSessionCookie(deleteReq, "admin-session")
	deleteRec := httptest.NewRecorder()
	handler.ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusOK {
		t.Fatalf("expected delete status 200, got %d body=%s", deleteRec.Code, deleteRec.Body.String())
	}
	if service.deleteIDs[0] != "dead-1" {
		t.Fatalf("expected delete id to reach service, got %+v", service.deleteIDs)
	}
}

func deadLetterJSONRequest(t *testing.T, method string, path string, body any) *http.Request {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	return req
}

type fakeDeadLetterService struct {
	listResult   deadletter.ListResult
	replayResult deadletter.BatchResult
	handleResult deadletter.BatchResult
	deleteResult deadletter.BatchResult
	replayIDs    []string
	handleIDs    []string
	deleteIDs    []string
	handleReason string
	listFilter   deadletter.ListFilter
}

func (f *fakeDeadLetterService) ListDeadLetters(_ context.Context, filter deadletter.ListFilter) (deadletter.ListResult, error) {
	f.listFilter = filter
	return f.listResult, nil
}

func (f *fakeDeadLetterService) ReplayDeadLetters(_ context.Context, input deadletter.BatchInput) (deadletter.BatchResult, error) {
	f.replayIDs = input.IDs
	return f.replayResult, nil
}

func (f *fakeDeadLetterService) MarkDeadLettersHandled(_ context.Context, input deadletter.HandleInput) (deadletter.BatchResult, error) {
	f.handleIDs = input.IDs
	f.handleReason = input.Reason
	return f.handleResult, nil
}

func (f *fakeDeadLetterService) DeleteDeadLetters(_ context.Context, input deadletter.BatchInput) (deadletter.BatchResult, error) {
	f.deleteIDs = input.IDs
	return f.deleteResult, nil
}
