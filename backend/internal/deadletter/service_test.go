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

type memoryStore struct {
	deleteIDs []string
}

func (m *memoryStore) ListDeadLetters(context.Context, ListFilter) (ListResult, error) {
	return ListResult{}, nil
}

func (m *memoryStore) ReplayDeadLetters(context.Context, BatchInput) (BatchResult, error) {
	return BatchResult{}, nil
}

func (m *memoryStore) MarkDeadLettersHandled(context.Context, HandleInput) (BatchResult, error) {
	return BatchResult{}, nil
}

func (m *memoryStore) DeleteDeadLetters(_ context.Context, input BatchInput) (BatchResult, error) {
	m.deleteIDs = input.IDs
	return BatchResult{Processed: len(input.IDs), IDs: input.IDs}, nil
}
