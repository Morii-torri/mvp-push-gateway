package delivery

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"mvp-push-gateway/backend/internal/queue"
)

func TestResultWriterPersistsCompletionEventsInBatch(t *testing.T) {
	store := newMemoryRepository()
	finishedAt := time.Date(2026, 6, 7, 10, 30, 0, 0, time.UTC)
	store.addAttempt(Attempt{
		ID:        "attempt-result-writer",
		MessageID: "message-result-writer",
		ChannelID: "channel-result-writer",
		Status:    StatusProcessing,
	})
	store.addJob(queue.Job{
		ID:        "job-result-writer",
		Type:      queue.JobTypeSendMessage,
		Status:    queue.JobStatusProcessing,
		LockedBy:  "sender-result-writer",
		ChannelID: "channel-result-writer",
	})

	writer := NewResultWriter(store)
	err := writer.Process(context.Background(), []DeliveryResultEvent{{
		JobID:            "job-result-writer",
		WorkerID:         "sender-result-writer",
		AttemptID:        "attempt-result-writer",
		TraceID:          "trace-result-writer",
		AttemptNo:        2,
		Status:           StatusSent,
		RequestSnapshot:  json.RawMessage(`{"final_request":{"url":"https://example.test/send"}}`),
		ResponseSnapshot: json.RawMessage(`{"upstream_response":{"status_code":200}}`),
		DurationMS:       37,
		FinishedAt:       finishedAt,
	}})
	if err != nil {
		t.Fatalf("process result event: %v", err)
	}
	if err := writer.Flush(context.Background()); err != nil {
		t.Fatalf("flush result writer: %v", err)
	}
	if store.completeDeliveryBatchCalls != 1 || store.completeDeliveryCalls != 0 {
		t.Fatalf("expected one batch persistence call, batch=%d single=%d", store.completeDeliveryBatchCalls, store.completeDeliveryCalls)
	}

	attempt := store.attempts["attempt-result-writer"]
	if attempt.Status != StatusSent || attempt.DurationMS != 37 {
		t.Fatalf("expected result writer to persist final attempt status, got %+v", attempt)
	}
	if attempt.FinishedAt == nil || !attempt.FinishedAt.Equal(finishedAt) {
		t.Fatalf("expected finished_at %s, got %v", finishedAt, attempt.FinishedAt)
	}
	if string(attempt.ResponseSnapshot) != `{"upstream_response":{"status_code":200}}` {
		t.Fatalf("unexpected response snapshot: %s", attempt.ResponseSnapshot)
	}
	if got := store.jobs["job-result-writer"].Status; got != queue.JobStatusDone {
		t.Fatalf("expected result writer to mark backing job done, got %s", got)
	}
}

func TestResultWriterPersistsRetryEvents(t *testing.T) {
	store := newMemoryRepository()
	finishedAt := time.Date(2026, 6, 7, 10, 35, 0, 0, time.UTC)
	retryAt := finishedAt.Add(2 * time.Second)
	store.addAttempt(Attempt{
		ID:        "attempt-result-retry",
		MessageID: "message-result-retry",
		ChannelID: "channel-result-retry",
		Status:    StatusProcessing,
	})
	store.addJob(queue.Job{
		ID:        "job-result-retry",
		Type:      queue.JobTypeSendMessage,
		Status:    queue.JobStatusProcessing,
		LockedBy:  "sender-result-retry",
		ChannelID: "channel-result-retry",
	})

	writer := NewResultWriter(store)
	err := writer.Process(context.Background(), []DeliveryResultEvent{{
		Action:           ResultActionRetry,
		JobID:            "job-result-retry",
		WorkerID:         "sender-result-retry",
		AttemptID:        "attempt-result-retry",
		AttemptNo:        1,
		Status:           StatusFailed,
		ErrorCode:        "MGP-SEND-004",
		ErrorMessage:     "temporary upstream failure",
		RequestSnapshot:  json.RawMessage(`{"final_request":{"url":"https://example.test/send"}}`),
		ResponseSnapshot: json.RawMessage(`{"upstream_response":{"status_code":500}}`),
		DurationMS:       42,
		RetryAt:          &retryAt,
		FinishedAt:       finishedAt,
	}})
	if err != nil {
		t.Fatalf("process retry event: %v", err)
	}
	if store.retryDeliveryCalls != 1 {
		t.Fatalf("expected one retry persistence call, got %d", store.retryDeliveryCalls)
	}
	attempt := store.attempts["attempt-result-retry"]
	if attempt.Status != StatusFailed || attempt.NextRetryAt == nil || !attempt.NextRetryAt.Equal(retryAt) {
		t.Fatalf("expected retry state on attempt, got %+v", attempt)
	}
}

func TestResultWriterPersistsDeadLetterEvents(t *testing.T) {
	store := newMemoryRepository()
	finishedAt := time.Date(2026, 6, 7, 10, 40, 0, 0, time.UTC)
	store.addAttempt(Attempt{
		ID:        "attempt-result-dead",
		MessageID: "message-result-dead",
		ChannelID: "channel-result-dead",
		Status:    StatusProcessing,
	})
	store.addJob(queue.Job{
		ID:        "job-result-dead",
		Type:      queue.JobTypeSendMessage,
		Status:    queue.JobStatusProcessing,
		LockedBy:  "sender-result-dead",
		ChannelID: "channel-result-dead",
	})

	writer := NewResultWriter(store)
	err := writer.Process(context.Background(), []DeliveryResultEvent{{
		Action:           ResultActionDeadLetter,
		JobID:            "job-result-dead",
		WorkerID:         "sender-result-dead",
		AttemptID:        "attempt-result-dead",
		ChannelID:        "channel-result-dead",
		AttemptNo:        1,
		Status:           StatusFailed,
		ErrorCode:        "MGP-SEND-004",
		ErrorMessage:     "permanent upstream failure",
		RequestSnapshot:  json.RawMessage(`{"final_request":{"url":"https://example.test/send"}}`),
		ResponseSnapshot: json.RawMessage(`{"upstream_response":{"status_code":400}}`),
		DurationMS:       55,
		FinishedAt:       finishedAt,
	}})
	if err != nil {
		t.Fatalf("process dead-letter event: %v", err)
	}
	if store.deadLetterDeliveryCalls != 1 {
		t.Fatalf("expected one dead-letter persistence call, got %d", store.deadLetterDeliveryCalls)
	}
	attempt := store.attempts["attempt-result-dead"]
	if attempt.Status != StatusFailed || attempt.DeadLetteredAt == nil || !attempt.DeadLetteredAt.Equal(finishedAt) {
		t.Fatalf("expected dead-letter state on attempt, got %+v", attempt)
	}
}
