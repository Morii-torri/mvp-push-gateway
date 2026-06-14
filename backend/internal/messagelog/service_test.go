package messagelog

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestListMessagesDerivesLifecycleStatusAndClearsNoRouteMatch(t *testing.T) {
	firstOutboundAt := time.Date(2026, 6, 4, 10, 1, 0, 0, time.UTC)
	store := &memoryStore{
		listResult: ListResult{
			Messages: []MessageSummary{
				{
					ID:              "message-no-route",
					Status:          "no_route",
					MatchedFlowID:   "flow-1",
					MatchedFlowName: "来源1组",
				},
				{
					ID:              "message-queued",
					Status:          "planned",
					MatchedFlowID:   "flow-2",
					MatchedFlowName: "生产路由",
					MatchedRuleIDs:  []string{"rule-1"},
					OutboundStatus:  "queued",
					FirstOutboundAt: &firstOutboundAt,
				},
			},
		},
	}
	service := NewService(store)

	result, err := service.ListMessages(context.Background(), ListFilter{})
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if result.Messages[0].Status != "no_route" || result.Messages[0].InboundStatus != "no_route" {
		t.Fatalf("expected no_route lifecycle and inbound status, got %+v", result.Messages[0])
	}
	if result.Messages[0].MatchedFlowID != "" || result.Messages[0].MatchedFlowName != "" {
		t.Fatalf("expected no_route message not to expose evaluated route group as matched route, got %+v", result.Messages[0])
	}
	if result.Messages[1].Status != "queued" || result.Messages[1].InboundStatus != "planned" {
		t.Fatalf("expected outbound queued lifecycle status with planned inbound status, got %+v", result.Messages[1])
	}
	if result.Messages[1].MatchedFlowName != "生产路由" {
		t.Fatalf("expected real matched route to remain, got %+v", result.Messages[1])
	}
}

func TestGetMessageBuildsLifecycleTimeline(t *testing.T) {
	receivedAt := time.Date(2026, 6, 4, 10, 0, 0, 0, time.UTC)
	validatedAt := receivedAt.Add(time.Millisecond)
	routeStartedAt := receivedAt.Add(3 * time.Millisecond)
	conditionFinishedAt := receivedAt.Add(6 * time.Millisecond)
	templateRenderedAt := receivedAt.Add(9 * time.Millisecond)
	sendEventBuiltAt := receivedAt.Add(11 * time.Millisecond)
	routePlannedAt := receivedAt.Add(12 * time.Millisecond)
	queuedAt := receivedAt.Add(2 * time.Second)
	startedAt := receivedAt.Add(3 * time.Second)
	finishedAt := receivedAt.Add(5 * time.Second)
	updatedAt := receivedAt.Add(4 * time.Second)
	requestSnapshot := mustJSON(t, map[string]any{
		"lifecycle": map[string]any{
			"route_plan_started_at":        routeStartedAt.Format(time.RFC3339Nano),
			"route_condition_finished_at":  conditionFinishedAt.Format(time.RFC3339Nano),
			"route_condition_duration_ms":  2,
			"template_render_finished_at":  templateRenderedAt.Format(time.RFC3339Nano),
			"template_render_duration_ms":  3,
			"send_event_built_at":          sendEventBuiltAt.Format(time.RFC3339Nano),
			"send_event_build_duration_ms": 2,
			"route_planned_at":             routePlannedAt.Format(time.RFC3339Nano),
			"delivery_created_at":          queuedAt.Format(time.RFC3339Nano),
			"request_started_at":           startedAt.Format(time.RFC3339Nano),
		},
	})
	responseSnapshot := mustJSON(t, map[string]any{
		"lifecycle": map[string]any{
			"request_finished_at": finishedAt.Format(time.RFC3339Nano),
			"request_duration_ms": 2000,
		},
	})
	store := &memoryStore{
		detail: MessageDetail{
			MessageSummary: MessageSummary{
				ID:              "message-1",
				ReceivedAt:      receivedAt,
				CreatedAt:       validatedAt,
				UpdatedAt:       updatedAt,
				Status:          "planned",
				MatchedFlowID:   "flow-1",
				MatchedFlowName: "生产路由",
				MatchedRuleIDs:  []string{"rule-1"},
			},
			Attempts: []DeliveryAttempt{
				{
					ID:               "attempt-1",
					Status:           "sent",
					QueuedAt:         &queuedAt,
					StartedAt:        &startedAt,
					FinishedAt:       &finishedAt,
					RequestSnapshot:  requestSnapshot,
					ResponseSnapshot: responseSnapshot,
				},
			},
		},
	}
	service := NewService(store)

	detail, err := service.GetMessage(context.Background(), "message-1")
	if err != nil {
		t.Fatalf("get message: %v", err)
	}
	if detail.Status != "sent" || detail.InboundStatus != "planned" || detail.OutboundStatus != "sent" {
		t.Fatalf("expected derived lifecycle status, got %+v", detail.MessageSummary)
	}
	if detail.FirstOutboundAt == nil || !detail.FirstOutboundAt.Equal(finishedAt) {
		t.Fatalf("expected first outbound time %s, got %+v", finishedAt, detail.FirstOutboundAt)
	}
	if detail.LastOutboundAt == nil || !detail.LastOutboundAt.Equal(finishedAt) {
		t.Fatalf("expected last outbound time %s, got %+v", finishedAt, detail.LastOutboundAt)
	}
	if !hasTimelineStage(detail.Timeline, "inbound_validated") ||
		!hasTimelineStage(detail.Timeline, "route_planning_started") ||
		!hasTimelineStage(detail.Timeline, "route_condition_evaluated") ||
		!hasTimelineStage(detail.Timeline, "route_template_rendered") ||
		!hasTimelineStage(detail.Timeline, "route_send_event_built") ||
		!hasTimelineStage(detail.Timeline, "route_planned") ||
		!hasTimelineStage(detail.Timeline, "upstream_call_finished") {
		t.Fatalf("expected route and delivery timeline events, got %+v", detail.Timeline)
	}
	if !hasTimelineEvent(detail.Timeline, "route_planning_started", routeStartedAt, 2) {
		t.Fatalf("expected route planning wait event, got %+v", detail.Timeline)
	}
	if !hasTimelineEvent(detail.Timeline, "route_condition_evaluated", conditionFinishedAt, 2) {
		t.Fatalf("expected route condition duration event, got %+v", detail.Timeline)
	}
	if !hasTimelineEvent(detail.Timeline, "route_template_rendered", templateRenderedAt, 3) {
		t.Fatalf("expected template render duration event, got %+v", detail.Timeline)
	}
	if !hasTimelineEvent(detail.Timeline, "route_send_event_built", sendEventBuiltAt, 2) {
		t.Fatalf("expected send event build duration event, got %+v", detail.Timeline)
	}
	if !hasTimelineEvent(detail.Timeline, "route_planned", routePlannedAt, 9) {
		t.Fatalf("expected route_planned at %s with duration 9ms, got %+v", routePlannedAt, detail.Timeline)
	}
	if !hasTimelineEvent(detail.Timeline, "upstream_call_finished", finishedAt, 2000) {
		t.Fatalf("expected upstream_call_finished at %s with duration 2000ms, got %+v", finishedAt, detail.Timeline)
	}
	for i := 1; i < len(detail.Timeline); i++ {
		if detail.Timeline[i].At.Before(detail.Timeline[i-1].At) {
			t.Fatalf("expected timeline to be sorted by time, got %+v", detail.Timeline)
		}
	}
}

func TestGetMessageAddsNoRouteTimelineEvent(t *testing.T) {
	receivedAt := time.Date(2026, 6, 4, 10, 0, 0, 0, time.UTC)
	store := &memoryStore{
		detail: MessageDetail{
			MessageSummary: MessageSummary{
				ID:              "message-no-route",
				ReceivedAt:      receivedAt,
				Status:          "no_route",
				MatchedFlowID:   "flow-1",
				MatchedFlowName: "来源1组",
			},
		},
	}
	service := NewService(store)

	detail, err := service.GetMessage(context.Background(), "message-no-route")
	if err != nil {
		t.Fatalf("get message: %v", err)
	}
	if detail.MatchedFlowID != "" || detail.MatchedFlowName != "" {
		t.Fatalf("expected no_route detail not to expose evaluated route group as matched route, got %+v", detail.MessageSummary)
	}
	if !hasTimelineStage(detail.Timeline, "route_no_match") {
		t.Fatalf("expected no_route timeline event, got %+v", detail.Timeline)
	}
}

func hasTimelineStage(events []TimelineEvent, stage string) bool {
	for _, event := range events {
		if event.Stage == stage {
			return true
		}
	}
	return false
}

func hasTimelineEvent(events []TimelineEvent, stage string, at time.Time, durationMS int) bool {
	for _, event := range events {
		if event.Stage == stage && event.At.Equal(at) && event.DurationMS == durationMS {
			return true
		}
	}
	return false
}

func mustJSON(t *testing.T, value any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}
	return raw
}

type memoryStore struct {
	listResult ListResult
	detail     MessageDetail
}

func (m *memoryStore) ListMessages(context.Context, ListFilter) (ListResult, error) {
	return m.listResult, nil
}

func (m *memoryStore) GetMessage(context.Context, string) (MessageDetail, error) {
	return m.detail, nil
}
