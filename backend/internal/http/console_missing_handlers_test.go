package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"mvp-push-gateway/backend/internal/audit"
	httpapi "mvp-push-gateway/backend/internal/http"
	"mvp-push-gateway/backend/internal/matchgroup"
	"mvp-push-gateway/backend/internal/messagelog"
	"mvp-push-gateway/backend/internal/provider"
	"mvp-push-gateway/backend/internal/settings"
)

func TestMatchGroupHandlersSupportGroupAndItemCRUD(t *testing.T) {
	service := &fakeMatchGroupService{
		listResult: []matchgroup.Group{{
			ID:          "group-1",
			Name:        "Urgent",
			GroupType:   "business",
			Description: "urgent words",
			Enabled:     true,
			ItemCount:   1,
			Items: []matchgroup.Item{{
				ID:        "item-1",
				GroupID:   "group-1",
				Value:     "urgent",
				ValueType: "text",
			}},
		}},
		getResult: matchgroup.Group{
			ID:        "group-1",
			Name:      "Urgent",
			GroupType: "business",
			Enabled:   true,
		},
		itemResult: matchgroup.Item{
			ID:        "item-1",
			GroupID:   "group-1",
			Value:     "urgent",
			ValueType: "text",
		},
	}
	handler := httpapi.NewHandler(
		testConfig(),
		httpapi.WithAuthService(fakeAuthService{authenticatedToken: "admin-session"}),
		httpapi.WithMatchGroupService(service),
	)

	for _, tc := range []struct {
		name           string
		method         string
		path           string
		body           string
		expectedStatus int
	}{
		{name: "list groups", method: http.MethodGet, path: "/api/v1/match-groups", expectedStatus: http.StatusOK},
		{name: "create group", method: http.MethodPost, path: "/api/v1/match-groups", body: `{"name":"Urgent","group_type":"business","description":"urgent words","enabled":true}`, expectedStatus: http.StatusCreated},
		{name: "get group", method: http.MethodGet, path: "/api/v1/match-groups/group-1", expectedStatus: http.StatusOK},
		{name: "update group", method: http.MethodPut, path: "/api/v1/match-groups/group-1", body: `{"name":"Urgent Updated","group_type":"business","description":"urgent words","enabled":true}`, expectedStatus: http.StatusOK},
		{name: "list items", method: http.MethodGet, path: "/api/v1/match-groups/group-1/items", expectedStatus: http.StatusOK},
		{name: "create item", method: http.MethodPost, path: "/api/v1/match-groups/group-1/items", body: `{"value":"urgent","value_type":"text","metadata":{"label":"Urgent"}}`, expectedStatus: http.StatusCreated},
		{name: "get item", method: http.MethodGet, path: "/api/v1/match-groups/group-1/items/item-1", expectedStatus: http.StatusOK},
		{name: "update item", method: http.MethodPut, path: "/api/v1/match-groups/group-1/items/item-1", body: `{"value":"critical","value_type":"text","metadata":{}}`, expectedStatus: http.StatusOK},
		{name: "delete item", method: http.MethodDelete, path: "/api/v1/match-groups/group-1/items/item-1", expectedStatus: http.StatusOK},
		{name: "delete group", method: http.MethodDelete, path: "/api/v1/match-groups/group-1", expectedStatus: http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			req.Header.Set("Authorization", "Bearer admin-session")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != tc.expectedStatus {
				t.Fatalf("expected status %d, got %d body=%s", tc.expectedStatus, rec.Code, rec.Body.String())
			}
		})
	}

	if service.listCalls != 1 || service.createCalls != 1 || service.getCalls != 1 || service.updateCalls != 1 || service.deleteCalls != 1 {
		t.Fatalf("unexpected group calls: list=%d create=%d get=%d update=%d delete=%d", service.listCalls, service.createCalls, service.getCalls, service.updateCalls, service.deleteCalls)
	}
	if service.listItemCalls != 1 || service.createItemCalls != 1 || service.getItemCalls != 1 || service.updateItemCalls != 1 || service.deleteItemCalls != 1 {
		t.Fatalf("unexpected item calls: list=%d create=%d get=%d update=%d delete=%d", service.listItemCalls, service.createItemCalls, service.getItemCalls, service.updateItemCalls, service.deleteItemCalls)
	}
}

func TestMessageLogHandlersReturnListAndDetailWithAttempts(t *testing.T) {
	now := time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)
	service := &fakeMessageLogService{
		listResult: messagelog.ListResult{
			Messages: []messagelog.MessageSummary{{
				ID:              "message-1",
				TraceID:         "trace-1",
				SourceID:        "source-1",
				SourceName:      "Orders",
				ReceivedAt:      now,
				Status:          "sent",
				MatchedFlowID:   "flow-1",
				MatchedFlowName: "Orders Flow",
				OutboundStatus:  "sent",
				AttemptCount:    1,
				DurationMS:      128,
			}},
			Total: 1,
			Limit: 50,
		},
		detailResult: messagelog.MessageDetail{
			MessageSummary: messagelog.MessageSummary{
				ID:         "message-1",
				TraceID:    "trace-1",
				SourceID:   "source-1",
				SourceName: "Orders",
				ReceivedAt: now,
				Status:     "sent",
			},
			Headers: json.RawMessage(`{"Authorization":["Bearer token"]}`),
			Payload: json.RawMessage(`{"title":"paid"}`),
			Attempts: []messagelog.DeliveryAttempt{{
				ID:               "attempt-1",
				MessageID:        "message-1",
				ChannelID:        "channel-1",
				ChannelName:      "Webhook",
				RequestSnapshot:  json.RawMessage(`{"send":{"url":"https://example.test"}}`),
				ResponseSnapshot: json.RawMessage(`{"send":{"status_code":200}}`),
				Status:           "sent",
				DurationMS:       128,
			}},
			Timeline: []messagelog.TimelineEvent{{Stage: "inbound_received", At: now, Status: "sent"}},
		},
	}
	handler := httpapi.NewHandler(
		testConfig(),
		httpapi.WithAuthService(fakeAuthService{authenticatedToken: "admin-session"}),
		httpapi.WithMessageLogService(service),
	)

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/messages?trace_id=trace-1", nil)
	listReq.Header.Set("Authorization", "Bearer admin-session")
	listRec := httptest.NewRecorder()
	handler.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected list status 200, got %d body=%s", listRec.Code, listRec.Body.String())
	}
	var listBody struct {
		Messages []struct {
			TraceID        string `json:"trace_id"`
			OutboundStatus string `json:"outbound_status"`
		} `json:"messages"`
		Total int `json:"total"`
	}
	if err := json.NewDecoder(listRec.Body).Decode(&listBody); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if listBody.Total != 1 || len(listBody.Messages) != 1 || listBody.Messages[0].TraceID != "trace-1" || listBody.Messages[0].OutboundStatus != "sent" {
		t.Fatalf("unexpected list body: %+v", listBody)
	}

	detailReq := httptest.NewRequest(http.MethodGet, "/api/v1/messages/message-1", nil)
	detailReq.Header.Set("Authorization", "Bearer admin-session")
	detailRec := httptest.NewRecorder()
	handler.ServeHTTP(detailRec, detailReq)
	if detailRec.Code != http.StatusOK {
		t.Fatalf("expected detail status 200, got %d body=%s", detailRec.Code, detailRec.Body.String())
	}
	var detailBody struct {
		Message struct {
			TraceID  string `json:"trace_id"`
			Payload  any    `json:"payload"`
			Attempts []struct {
				ID               string `json:"id"`
				RequestSnapshot  any    `json:"request_snapshot"`
				ResponseSnapshot any    `json:"response_snapshot"`
			} `json:"attempts"`
			Timeline []struct {
				Stage string `json:"stage"`
			} `json:"timeline"`
		} `json:"message"`
	}
	if err := json.NewDecoder(detailRec.Body).Decode(&detailBody); err != nil {
		t.Fatalf("decode detail response: %v", err)
	}
	if detailBody.Message.TraceID != "trace-1" || len(detailBody.Message.Attempts) != 1 || len(detailBody.Message.Timeline) != 1 {
		t.Fatalf("unexpected detail body: %+v", detailBody)
	}
}

func TestAuditAndSettingsHandlersSupportListDetailAndUpdate(t *testing.T) {
	now := time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)
	auditService := &fakeAuditService{
		listResult: audit.ListResult{
			Logs: []audit.Log{{
				ID:            "audit-1",
				ActorUsername: "admin",
				Action:        "update",
				ResourceType:  "source",
				ResourceID:    "source-1",
				IPAddress:     "127.0.0.1",
				CreatedAt:     now,
			}},
			Total: 1,
			Limit: 50,
		},
		getResult: audit.Log{
			ID:               "audit-1",
			ActorUsername:    "admin",
			Action:           "update",
			ResourceType:     "source",
			ResourceID:       "source-1",
			RequestSnapshot:  json.RawMessage(`{"name":"before"}`),
			ResponseSnapshot: json.RawMessage(`{"name":"after"}`),
			CreatedAt:        now,
		},
	}
	settingsService := &fakeSettingsService{
		listResult: []settings.Setting{{
			Key:         "logs.retention_days",
			Value:       json.RawMessage(`30`),
			Description: "日志保留天数",
			Category:    "logs",
			UpdatedAt:   now,
		}},
		getResult: settings.Setting{
			Key:         "logs.retention_days",
			Value:       json.RawMessage(`45`),
			Description: "日志保留天数",
			Category:    "logs",
			UpdatedAt:   now,
		},
	}
	handler := httpapi.NewHandler(
		testConfig(),
		httpapi.WithAuthService(fakeAuthService{authenticatedToken: "admin-session"}),
		httpapi.WithAuditService(auditService),
		httpapi.WithSettingsService(settingsService),
	)

	for _, tc := range []struct {
		name           string
		method         string
		path           string
		body           string
		expectedStatus int
	}{
		{name: "audit list", method: http.MethodGet, path: "/api/v1/audit-logs", expectedStatus: http.StatusOK},
		{name: "audit detail", method: http.MethodGet, path: "/api/v1/audit-logs/audit-1", expectedStatus: http.StatusOK},
		{name: "settings list", method: http.MethodGet, path: "/api/v1/settings", expectedStatus: http.StatusOK},
		{name: "settings update", method: http.MethodPut, path: "/api/v1/settings/logs.retention_days", body: `{"value":45}`, expectedStatus: http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			req.Header.Set("Authorization", "Bearer admin-session")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != tc.expectedStatus {
				t.Fatalf("expected status %d, got %d body=%s", tc.expectedStatus, rec.Code, rec.Body.String())
			}
		})
	}

	if auditService.listCalls != 1 || auditService.getCalls != 1 {
		t.Fatalf("unexpected audit calls: list=%d get=%d", auditService.listCalls, auditService.getCalls)
	}
	if settingsService.listCalls != 1 || settingsService.updateCalls != 1 || string(settingsService.updateInput.Value) != "45" {
		t.Fatalf("unexpected settings calls: list=%d update=%d value=%s", settingsService.listCalls, settingsService.updateCalls, settingsService.updateInput.Value)
	}
}

func TestChannelTestSendBuildsOrSendsScopedRequest(t *testing.T) {
	providerService := &fakeProviderService{
		testSendResult: provider.TestSendResult{
			Status:     "sent",
			StatusCode: 200,
			Request: provider.BuiltRequest{
				Method: "POST",
				URL:    "https://example.test/send",
				Body:   json.RawMessage(`{"title":"paid"}`),
			},
			ResponseSnapshot: json.RawMessage(`{"status_code":200}`),
			DurationMS:       12,
		},
	}
	handler := httpapi.NewHandler(
		testConfig(),
		httpapi.WithAuthService(fakeAuthService{authenticatedToken: "admin-session"}),
		httpapi.WithProviderService(providerService),
	)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/channels/channel-1/test-send", strings.NewReader(`{
		"send": true,
		"live_send_confirmed": true,
		"token": "token",
		"recipient": "user-1",
		"body": {"title":"paid"}
	}`))
	req.Header.Set("Authorization", "Bearer admin-session")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if providerService.testSendCalls != 1 || !providerService.testSendInput.Send || !providerService.testSendInput.LiveSendConfirmed {
		t.Fatalf("expected scoped test-send to be called once with confirmed send=true, calls=%d input=%+v", providerService.testSendCalls, providerService.testSendInput)
	}
}

func TestSourceMutationWritesAuditRecord(t *testing.T) {
	auditService := &fakeAuditService{}
	handler := httpapi.NewHandler(
		testConfig(),
		httpapi.WithAuthService(fakeAuthService{authenticatedToken: "admin-session"}),
		httpapi.WithSourceService(&fakeSourceService{}),
		httpapi.WithAuditService(auditService),
	)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/sources", strings.NewReader(`{"code":"orders","name":"Orders","auth_mode":"token","auth_token":"sourceToken"}`))
	req.Header.Set("Authorization", "Bearer admin-session")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected source create status 201, got %d body=%s", rec.Code, rec.Body.String())
	}
	if auditService.recordCalls != 1 {
		t.Fatalf("expected one audit record, got %d", auditService.recordCalls)
	}
	if auditService.recordInput.Action != "create" || auditService.recordInput.ResourceType != "source" || auditService.recordInput.ActorUsername != "admin" {
		t.Fatalf("unexpected audit input: %+v", auditService.recordInput)
	}
}

type fakeMatchGroupService struct {
	listResult []matchgroup.Group
	getResult  matchgroup.Group
	itemResult matchgroup.Item

	listCalls       int
	createCalls     int
	getCalls        int
	updateCalls     int
	deleteCalls     int
	listItemCalls   int
	createItemCalls int
	getItemCalls    int
	updateItemCalls int
	deleteItemCalls int
}

func (f *fakeMatchGroupService) ListGroups(context.Context) ([]matchgroup.Group, error) {
	f.listCalls++
	return f.listResult, nil
}

func (f *fakeMatchGroupService) CreateGroup(_ context.Context, input matchgroup.GroupInput) (matchgroup.Group, error) {
	f.createCalls++
	return matchgroup.Group{ID: "group-1", Name: input.Name, GroupType: input.GroupType, Description: input.Description, Enabled: input.Enabled}, nil
}

func (f *fakeMatchGroupService) GetGroup(context.Context, string) (matchgroup.Group, error) {
	f.getCalls++
	return f.getResult, nil
}

func (f *fakeMatchGroupService) UpdateGroup(_ context.Context, id string, input matchgroup.GroupInput) (matchgroup.Group, error) {
	f.updateCalls++
	return matchgroup.Group{ID: id, Name: input.Name, GroupType: input.GroupType, Description: input.Description, Enabled: input.Enabled}, nil
}

func (f *fakeMatchGroupService) DeleteGroup(context.Context, string) error {
	f.deleteCalls++
	return nil
}

func (f *fakeMatchGroupService) ListItems(context.Context, string) ([]matchgroup.Item, error) {
	f.listItemCalls++
	return []matchgroup.Item{f.itemResult}, nil
}

func (f *fakeMatchGroupService) CreateItem(_ context.Context, groupID string, input matchgroup.ItemInput) (matchgroup.Item, error) {
	f.createItemCalls++
	return matchgroup.Item{ID: "item-1", GroupID: groupID, Value: input.Value, ValueType: input.ValueType, Metadata: input.Metadata}, nil
}

func (f *fakeMatchGroupService) GetItem(context.Context, string, string) (matchgroup.Item, error) {
	f.getItemCalls++
	return f.itemResult, nil
}

func (f *fakeMatchGroupService) UpdateItem(_ context.Context, groupID string, itemID string, input matchgroup.ItemInput) (matchgroup.Item, error) {
	f.updateItemCalls++
	return matchgroup.Item{ID: itemID, GroupID: groupID, Value: input.Value, ValueType: input.ValueType, Metadata: input.Metadata}, nil
}

func (f *fakeMatchGroupService) DeleteItem(context.Context, string, string) error {
	f.deleteItemCalls++
	return nil
}

type fakeMessageLogService struct {
	listResult   messagelog.ListResult
	detailResult messagelog.MessageDetail
}

func (f *fakeMessageLogService) ListMessages(context.Context, messagelog.ListFilter) (messagelog.ListResult, error) {
	return f.listResult, nil
}

func (f *fakeMessageLogService) GetMessage(context.Context, string) (messagelog.MessageDetail, error) {
	return f.detailResult, nil
}

type fakeAuditService struct {
	listResult  audit.ListResult
	getResult   audit.Log
	recordInput audit.RecordInput
	listCalls   int
	getCalls    int
	recordCalls int
}

func (f *fakeAuditService) ListLogs(context.Context, audit.ListFilter) (audit.ListResult, error) {
	f.listCalls++
	return f.listResult, nil
}

func (f *fakeAuditService) GetLog(context.Context, string) (audit.Log, error) {
	f.getCalls++
	return f.getResult, nil
}

func (f *fakeAuditService) Record(_ context.Context, input audit.RecordInput) (audit.Log, error) {
	f.recordCalls++
	f.recordInput = input
	return audit.Log{ID: "audit-new", ActorUsername: input.ActorUsername, Action: input.Action, ResourceType: input.ResourceType, ResourceID: input.ResourceID}, nil
}

type fakeSettingsService struct {
	listResult           []settings.Setting
	getResult            settings.Setting
	updateInput          settings.UpdateInput
	performanceTestInput settings.PerformanceTestInput
	intValues            map[string]int
	performanceTestErr   error
	listCalls            int
	updateCalls          int
	intCalls             int
	performanceTestCalls int
}

func (f *fakeSettingsService) ListSettings(context.Context) ([]settings.Setting, error) {
	f.listCalls++
	return f.listResult, nil
}

func (f *fakeSettingsService) UpdateSetting(_ context.Context, _ string, input settings.UpdateInput) (settings.Setting, error) {
	f.updateCalls++
	f.updateInput = input
	return f.getResult, nil
}

func (f *fakeSettingsService) IntSetting(_ context.Context, key string, fallback int) int {
	f.intCalls++
	if f.intValues == nil {
		return fallback
	}
	value, ok := f.intValues[key]
	if !ok || value <= 0 {
		return fallback
	}
	return value
}

func (f *fakeSettingsService) RunPerformanceTest(_ context.Context, input settings.PerformanceTestInput) (settings.PerformanceTestResult, error) {
	f.performanceTestCalls++
	f.performanceTestInput = input
	if f.performanceTestErr != nil {
		return settings.PerformanceTestResult{}, f.performanceTestErr
	}
	return settings.PerformanceTestResult{
		MessageCount:                 input.MessageCount,
		GeneratedSourceCode:          input.GeneratedSourceCode,
		GeneratedRouteName:           input.GeneratedRouteName,
		GeneratedChannelName:         input.GeneratedChannelName,
		RecommendedGlobalConcurrency: 10,
		UpdatedSettingKey:            settings.KeyRuntimeDeliveryConcurrency,
	}, nil
}

type fakeProviderService struct {
	testSendResult     provider.TestSendResult
	testSendInput      provider.TestSendInput
	testSendCalls      int
	createChannelCalls int
	deleteChannelCalls int
}

func (f *fakeProviderService) SeedProviderCapabilities(context.Context) error {
	return nil
}

func (f *fakeProviderService) ListProviderCapabilities(context.Context) ([]provider.Capability, error) {
	return nil, nil
}

func (f *fakeProviderService) ListChannels(context.Context) ([]provider.Channel, error) {
	return nil, nil
}

func (f *fakeProviderService) CreateChannel(_ context.Context, input provider.CreateChannelInput) (provider.Channel, error) {
	f.createChannelCalls++
	return provider.Channel{ID: "channel-1", ProviderType: input.ProviderType, Name: input.Name, Enabled: input.Enabled}, nil
}

func (f *fakeProviderService) GetChannel(context.Context, string) (provider.Channel, error) {
	return provider.Channel{ID: "channel-1", ProviderType: provider.ProviderWebhook, Name: "Webhook", Enabled: true}, nil
}

func (f *fakeProviderService) UpdateChannel(_ context.Context, id string, input provider.UpdateChannelInput) (provider.Channel, error) {
	return provider.Channel{ID: id, ProviderType: input.ProviderType, Name: input.Name, Enabled: input.Enabled}, nil
}

func (f *fakeProviderService) DeleteChannel(context.Context, string) error {
	f.deleteChannelCalls++
	return nil
}

func (f *fakeProviderService) BuildRequest(context.Context, string, provider.BuildRequestInput) (provider.BuiltRequest, error) {
	return provider.BuiltRequest{Method: "POST", URL: "https://example.test/send"}, nil
}

func (f *fakeProviderService) TestSend(_ context.Context, _ string, input provider.TestSendInput) (provider.TestSendResult, error) {
	f.testSendCalls++
	f.testSendInput = input
	return f.testSendResult, nil
}

func (f *fakeProviderService) RefreshToken(context.Context, string) (provider.TokenCacheStatus, error) {
	return provider.TokenCacheStatus{IsCached: true, TokenRefreshed: time.Now().Format(time.RFC3339)}, nil
}

func (f *fakeProviderService) ResolveFeishuOpenID(context.Context, string, []string) (provider.FeishuOpenIDResolveResult, error) {
	return provider.FeishuOpenIDResolveResult{}, nil
}
