package delivery

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"mvp-push-gateway/backend/internal/queue"
)

func TestResultQueueWorkerConsumesResultMessagesAndPersistsThem(t *testing.T) {
	store := newMemoryRepository()
	finishedAt := time.Date(2026, 6, 7, 11, 15, 0, 0, time.UTC)
	store.addAttempt(Attempt{
		ID:        "attempt-result-queue",
		MessageID: "message-result-queue",
		ChannelID: "channel-result-queue",
		Status:    StatusProcessing,
	})
	store.addJob(queue.Job{
		ID:        "job-result-queue",
		Type:      queue.JobTypeSendMessage,
		Status:    queue.JobStatusProcessing,
		LockedBy:  "sender-result-queue",
		ChannelID: "channel-result-queue",
	})

	deliveryEvent := DeliveryResultEvent{
		JobID:            "job-result-queue",
		WorkerID:         "sender-result-queue",
		AttemptID:        "attempt-result-queue",
		TraceID:          "trace-result-queue",
		AttemptNo:        1,
		Status:           StatusSent,
		RequestSnapshot:  json.RawMessage(`{"final_request":{"url":"https://example.test/send"}}`),
		ResponseSnapshot: json.RawMessage(`{"upstream_response":{"status_code":200}}`),
		DurationMS:       18,
		FinishedAt:       finishedAt,
	}
	queueEvent, err := deliveryEvent.QueueEvent()
	if err != nil {
		t.Fatalf("build queue event: %v", err)
	}
	subscriber := &recordingResultSubscriber{events: []queue.DeliveryResultEvent{queueEvent}}
	worker := NewResultQueueWorker(subscriber, NewResultWriter(store))

	if err := worker.Run(context.Background()); err != nil {
		t.Fatalf("run result queue worker: %v", err)
	}
	if store.completeDeliveryBatchCalls != 1 {
		t.Fatalf("expected result queue worker to batch persist once, got %d", store.completeDeliveryBatchCalls)
	}
	if got := store.attempts["attempt-result-queue"].Status; got != StatusSent {
		t.Fatalf("expected attempt sent after result queue processing, got %s", got)
	}
	if subscriber.handled != 1 {
		t.Fatalf("expected one result message handled, got %d", subscriber.handled)
	}
}

type recordingResultSubscriber struct {
	events  []queue.DeliveryResultEvent
	handled int
}

func (s *recordingResultSubscriber) SubscribeResult(ctx context.Context, handler queue.ResultHandler) error {
	for _, event := range s.events {
		s.handled++
		if err := handler(ctx, queue.ResultMessage{Event: event}); err != nil {
			return err
		}
	}
	return nil
}
