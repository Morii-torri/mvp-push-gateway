package deadletter

import (
	"context"
	"reflect"
	"testing"
	"time"

	"mvp-push-gateway/backend/internal/queue"
)

func TestServiceDeleteDeadLettersNormalizesIDs(t *testing.T) {
	store := &memoryStore{}
	service := NewService(store)

	result, err := service.DeleteDeadLetters(context.Background(), BatchInput{
		IDs: []string{" dead-1 ", "", "dead-1", "dead-2"},
	})
	if err != nil {
		t.Fatalf("delete dead letters: %v", err)
	}

	expected := []string{"dead-1", "dead-2"}
	if !reflect.DeepEqual(store.deleteIDs, expected) {
		t.Fatalf("expected normalized delete ids %+v, got %+v", expected, store.deleteIDs)
	}
	if result.Processed != 2 {
		t.Fatalf("expected processed 2, got %d", result.Processed)
	}
}

func TestServiceAllowsAllDeadLetterBatchSelection(t *testing.T) {
	store := &memoryStore{}
	service := NewService(store)

	if _, err := service.ReplayDeadLetters(context.Background(), BatchInput{All: true}); err != nil {
		t.Fatalf("replay all dead letters: %v", err)
	}
	if store.legacyReplayCalled {
		t.Fatalf("expected all replay selection not to reach legacy PostgreSQL job replay")
	}
	if _, err := service.MarkDeadLettersHandled(context.Background(), HandleInput{All: true}); err != nil {
		t.Fatalf("handle all dead letters: %v", err)
	}
	if !store.handleAll {
		t.Fatalf("expected all handle selection to reach store")
	}
}

func TestServiceReplaysExternalDeadLettersThroughSendPublisher(t *testing.T) {
	store := &externalMemoryStore{
		events: []ExternalReplayEvent{{
			ID: "dead-external-1",
			Event: queue.SendMessageEvent{
				DeliveryAttemptID: "attempt-1",
				ChannelID:         "channel-1",
				ProviderType:      "webhook",
			},
		}},
	}
	publisher := &recordingSendPublisher{}
	service := NewService(store, WithSendPublisher(publisher))

	result, err := service.ReplayDeadLetters(context.Background(), BatchInput{All: true, Status: "pending"})
	if err != nil {
		t.Fatalf("replay external dead letters: %v", err)
	}
	if result.Processed != 1 || !reflect.DeepEqual(result.IDs, []string{"dead-external-1"}) {
		t.Fatalf("expected one external replay result, got %+v", result)
	}
	if len(publisher.events) != 1 || publisher.events[0].DeliveryAttemptID != "attempt-1" {
		t.Fatalf("expected send publisher to receive external replay event, got %+v", publisher.events)
	}
	if !reflect.DeepEqual(store.markedIDs, []string{"dead-external-1"}) {
		t.Fatalf("expected external dead letter to be marked replayed, got %+v", store.markedIDs)
	}
}

func TestServiceReplayDoesNotUseLegacyPostgresJobReplay(t *testing.T) {
	store := &externalMemoryStore{
		events: []ExternalReplayEvent{{
			ID: "dead-external-1",
			Event: queue.SendMessageEvent{
				DeliveryAttemptID: "attempt-1",
				ChannelID:         "channel-1",
			},
		}},
	}
	publisher := &recordingSendPublisher{}
	service := NewService(store, WithSendPublisher(publisher))

	if _, err := service.ReplayDeadLetters(context.Background(), BatchInput{All: true, Status: "pending"}); err != nil {
		t.Fatalf("replay dead letters: %v", err)
	}

	if store.legacyReplayCalled {
		t.Fatal("expected JetStream replay not to call legacy PostgreSQL job replay")
	}
}

type memoryStore struct {
	deleteIDs          []string
	replayAll          bool
	handleAll          bool
	legacyReplayCalled bool
}

func (m *memoryStore) ListDeadLetters(context.Context, ListFilter) (ListResult, error) {
	return ListResult{}, nil
}

func (m *memoryStore) ReplayDeadLetters(_ context.Context, input BatchInput) (BatchResult, error) {
	m.legacyReplayCalled = true
	m.replayAll = input.All
	return BatchResult{}, nil
}

func (m *memoryStore) MarkDeadLettersHandled(_ context.Context, input HandleInput) (BatchResult, error) {
	m.handleAll = input.All
	return BatchResult{}, nil
}

func (m *memoryStore) DeleteDeadLetters(_ context.Context, input BatchInput) (BatchResult, error) {
	m.deleteIDs = input.IDs
	return BatchResult{Processed: len(input.IDs), IDs: input.IDs}, nil
}

type externalMemoryStore struct {
	memoryStore
	events    []ExternalReplayEvent
	markedIDs []string
}

func (m *externalMemoryStore) ListExternalReplayEvents(context.Context, BatchInput) ([]ExternalReplayEvent, error) {
	return m.events, nil
}

func (m *externalMemoryStore) MarkExternalDeadLettersReplayed(_ context.Context, ids []string, _ time.Time) (BatchResult, error) {
	m.markedIDs = append([]string(nil), ids...)
	return BatchResult{Processed: len(ids), IDs: append([]string(nil), ids...)}, nil
}

type recordingSendPublisher struct {
	events []queue.SendMessageEvent
}

func (p *recordingSendPublisher) PublishSend(_ context.Context, event queue.SendMessageEvent) (queue.PublishResult, error) {
	p.events = append(p.events, event)
	return queue.PublishResult{}, nil
}
