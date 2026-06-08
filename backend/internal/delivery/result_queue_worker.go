package delivery

import (
	"context"
	"errors"

	"mvp-push-gateway/backend/internal/queue"
)

type ResultSubscriber interface {
	SubscribeResult(context.Context, queue.ResultHandler) error
}

type ResultQueueWorker struct {
	subscriber ResultSubscriber
	writer     ResultWriter
}

func NewResultQueueWorker(subscriber ResultSubscriber, writer ResultWriter) *ResultQueueWorker {
	return &ResultQueueWorker{subscriber: subscriber, writer: writer}
}

func (w *ResultQueueWorker) Run(ctx context.Context) error {
	if w == nil || w.subscriber == nil || w.writer == nil {
		return errors.New("result queue worker requires subscriber and writer")
	}
	return w.subscriber.SubscribeResult(ctx, func(ctx context.Context, message queue.ResultMessage) error {
		event, err := DeliveryResultEventFromQueue(message.Event)
		if err != nil {
			return err
		}
		return w.writer.Process(ctx, []DeliveryResultEvent{event})
	})
}
