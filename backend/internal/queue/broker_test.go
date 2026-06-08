package queue_test

import (
	"testing"

	"mvp-push-gateway/backend/internal/queue"
)

func TestBrokerEventsExposeStableSubjectsAndMessageIDs(t *testing.T) {
	route := queue.RoutePlanEvent{
		MessageID: "msg-1",
		SourceID:  "source-1",
		TraceID:   "trace-1",
	}
	if route.Subject() != "mgp.route_plan.source-1" {
		t.Fatalf("expected source-scoped route plan subject, got %q", route.Subject())
	}
	if route.MessageIDForDedup() != "trace-1" {
		t.Fatalf("expected route plan dedupe ID to prefer trace ID, got %q", route.MessageIDForDedup())
	}

	send := queue.SendMessageEvent{
		DeliveryAttemptID: "attempt-1",
		ChannelID:         "channel-1",
		ProviderType:      "webhook",
	}
	if send.Subject() != "mgp.send.webhook.channel-1" {
		t.Fatalf("expected provider/channel send subject, got %q", send.Subject())
	}
	if send.MessageIDForDedup() != "attempt-1" {
		t.Fatalf("expected send dedupe ID to use delivery attempt ID, got %q", send.MessageIDForDedup())
	}

	result := queue.DeliveryResultEvent{
		DeliveryAttemptID: "attempt-1",
		AttemptNo:         2,
		Status:            "sent",
	}
	if result.Subject() != "mgp.result.sent" {
		t.Fatalf("expected status-scoped result subject, got %q", result.Subject())
	}
	if result.MessageIDForDedup() != "attempt-1:2" {
		t.Fatalf("expected result dedupe ID to include attempt number, got %q", result.MessageIDForDedup())
	}
}

func TestBrokerEventsValidateRequiredMessageIDs(t *testing.T) {
	if err := (queue.RoutePlanEvent{SourceID: "source-1"}).Validate(); err == nil {
		t.Fatal("expected route plan event without message ID to be invalid")
	}
	if err := (queue.SendMessageEvent{ChannelID: "channel-1"}).Validate(); err == nil {
		t.Fatal("expected send event without delivery attempt ID to be invalid")
	}
	if err := (queue.DeliveryResultEvent{DeliveryAttemptID: "attempt-1"}).Validate(); err == nil {
		t.Fatal("expected result event without attempt number to be invalid")
	}
}
