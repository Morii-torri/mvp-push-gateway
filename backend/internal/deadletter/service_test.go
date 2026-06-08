package deadletter

import (
	"context"
	"reflect"
	"testing"
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
	if !store.replayAll {
		t.Fatalf("expected all replay selection to reach store")
	}
	if _, err := service.MarkDeadLettersHandled(context.Background(), HandleInput{All: true}); err != nil {
		t.Fatalf("handle all dead letters: %v", err)
	}
	if !store.handleAll {
		t.Fatalf("expected all handle selection to reach store")
	}
}

type memoryStore struct {
	deleteIDs []string
	replayAll bool
	handleAll bool
}

func (m *memoryStore) ListDeadLetters(context.Context, ListFilter) (ListResult, error) {
	return ListResult{}, nil
}

func (m *memoryStore) ReplayDeadLetters(_ context.Context, input BatchInput) (BatchResult, error) {
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
