package delivery

import (
	"context"
	"errors"
)

type ResultWriteRepository interface {
	CompleteDeliveries(context.Context, []CompleteDeliveryParams) error
	RetryDelivery(context.Context, RetryDeliveryParams) error
	DeadLetterDelivery(context.Context, DeadLetterDeliveryParams) error
}

type ResultWriter interface {
	Process(context.Context, []DeliveryResultEvent) error
	Flush(context.Context) error
}

type BatchedResultWriter struct {
	repo ResultWriteRepository
}

func NewResultWriter(repo ResultWriteRepository) *BatchedResultWriter {
	return &BatchedResultWriter{repo: repo}
}

func (w *BatchedResultWriter) Process(ctx context.Context, events []DeliveryResultEvent) error {
	if len(events) == 0 {
		return nil
	}
	if w == nil || w.repo == nil {
		return errors.New("result writer repository is required")
	}
	completeParams := make([]CompleteDeliveryParams, 0, len(events))
	var err error
	for _, event := range events {
		if err := event.Validate(); err != nil {
			return err
		}
		switch event.Action {
		case "", ResultActionComplete:
			completeParams = append(completeParams, event.CompleteDeliveryParams())
		case ResultActionRetry:
			err = errors.Join(err, w.repo.RetryDelivery(ctx, event.RetryDeliveryParams()))
		case ResultActionDeadLetter:
			err = errors.Join(err, w.repo.DeadLetterDelivery(ctx, event.DeadLetterDeliveryParams()))
		}
	}
	if len(completeParams) > 0 {
		err = errors.Join(err, w.repo.CompleteDeliveries(ctx, completeParams))
	}
	return err
}

func (w *BatchedResultWriter) Flush(context.Context) error {
	return nil
}
