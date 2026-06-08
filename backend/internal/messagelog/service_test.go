package messagelog

import (
	"context"
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
	queuedAt := receivedAt.Add(2 * time.Second)
	startedAt := receivedAt.Add(3 * time.Second)
	finishedAt := receivedAt.Add(5 * time.Second)
	store := &memoryStore{
		detail: MessageDetail{
			MessageSummary: MessageSummary{
				ID:              "message-1",
				ReceivedAt:      receivedAt,
				Status:          "planned",
				MatchedFlowID:   "flow-1",
				MatchedFlowName: "生产路由",
				MatchedRuleIDs:  []string{"rule-1"},
			},
			Attempts: []DeliveryAttempt{
				{
					ID:         "attempt-1",
					Status:     "sent",
					QueuedAt:   &queuedAt,
					StartedAt:  &startedAt,
					FinishedAt: &finishedAt,
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
	if detail.FirstOutboundAt == nil || !detail.FirstOutboundAt.Equal(queuedAt) {
		t.Fatalf("expected first outbound time %s, got %+v", queuedAt, detail.FirstOutboundAt)
	}
	if detail.LastOutboundAt == nil || !detail.LastOutboundAt.Equal(finishedAt) {
		t.Fatalf("expected last outbound time %s, got %+v", finishedAt, detail.LastOutboundAt)
	}
	if !hasTimelineStage(detail.Timeline, "route_matched") || !hasTimelineStage(detail.Timeline, "delivery_finished") {
		t.Fatalf("expected route and delivery timeline events, got %+v", detail.Timeline)
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
