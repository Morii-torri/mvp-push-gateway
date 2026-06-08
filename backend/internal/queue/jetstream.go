package queue

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

type StreamPublisher interface {
	Publish(ctx context.Context, subject string, messageID string, payload []byte) (PublishResult, error)
}

type StreamMessage struct {
	Data          []byte
	DeliveryCount int
	Ack           func() error
	Nak           func(time.Duration) error
}

type StreamMessageHandler func(context.Context, StreamMessage) error

type StreamSubscriber interface {
	Subscribe(ctx context.Context, subject string, durable string, handler StreamMessageHandler) error
}

type JetStreamBroker struct {
	publisher  StreamPublisher
	subscriber StreamSubscriber
}

func NewJetStreamBroker(publisher StreamPublisher) *JetStreamBroker {
	var subscriber StreamSubscriber
	if candidate, ok := publisher.(StreamSubscriber); ok {
		subscriber = candidate
	}
	return &JetStreamBroker{publisher: publisher, subscriber: subscriber}
}

func NewJetStreamBrokerWithSubscriber(publisher StreamPublisher, subscriber StreamSubscriber) *JetStreamBroker {
	return &JetStreamBroker{publisher: publisher, subscriber: subscriber}
}

func (b *JetStreamBroker) PublishRoutePlan(ctx context.Context, event RoutePlanEvent) (PublishResult, error) {
	if err := event.Validate(); err != nil {
		return PublishResult{}, err
	}
	return b.publish(ctx, event.Subject(), event.MessageIDForDedup(), event)
}

func (b *JetStreamBroker) PublishSend(ctx context.Context, event SendMessageEvent) (PublishResult, error) {
	if err := event.Validate(); err != nil {
		return PublishResult{}, err
	}
	return b.publish(ctx, event.Subject(), event.MessageIDForDedup(), event)
}

func (b *JetStreamBroker) PublishResult(ctx context.Context, event DeliveryResultEvent) (PublishResult, error) {
	if err := event.Validate(); err != nil {
		return PublishResult{}, err
	}
	return b.publish(ctx, event.Subject(), event.MessageIDForDedup(), event)
}

func (b *JetStreamBroker) SubscribeRoutePlan(ctx context.Context, handler RoutePlanHandler) error {
	return b.subscribe(ctx, RoutePlanSubjectPrefix+".*", "route-plan-workers", func(_ context.Context, message StreamMessage) error {
		var event RoutePlanEvent
		if err := json.Unmarshal(message.Data, &event); err != nil {
			return err
		}
		return handler(ctx, RoutePlanMessage{
			Event: event,
			Ack:   ackFunc(message),
			Nak:   nakFunc(message),
		})
	})
}

func (b *JetStreamBroker) SubscribeSend(ctx context.Context, handler SendHandler) error {
	return b.subscribe(ctx, SendSubjectPrefix+".*.*", "send-workers", func(_ context.Context, message StreamMessage) error {
		var event SendMessageEvent
		if err := json.Unmarshal(message.Data, &event); err != nil {
			return err
		}
		return handler(ctx, SendMessage{
			Event:         event,
			DeliveryCount: positiveDeliveryCount(message.DeliveryCount),
			Ack:           ackFunc(message),
			Nak:           nakFunc(message),
		})
	})
}

func (b *JetStreamBroker) SubscribeResult(ctx context.Context, handler ResultHandler) error {
	return b.subscribe(ctx, ResultSubjectPrefix+".*", "result-writers", func(_ context.Context, message StreamMessage) error {
		var event DeliveryResultEvent
		if err := json.Unmarshal(message.Data, &event); err != nil {
			return err
		}
		return handler(ctx, ResultMessage{
			Event: event,
			Ack:   ackFunc(message),
			Nak:   nakFunc(message),
		})
	})
}

func (b *JetStreamBroker) publish(ctx context.Context, subject string, messageID string, event any) (PublishResult, error) {
	if b == nil || b.publisher == nil {
		return PublishResult{}, ErrInvalidInput
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return PublishResult{}, err
	}
	return b.publisher.Publish(ctx, subject, messageID, payload)
}

func (b *JetStreamBroker) subscribe(ctx context.Context, subject string, durable string, handler StreamMessageHandler) error {
	if b == nil || b.subscriber == nil || handler == nil {
		return ErrInvalidInput
	}
	return b.subscriber.Subscribe(ctx, subject, durable, func(ctx context.Context, message StreamMessage) error {
		tracker := &ackTracker{}
		tracked := tracker.wrap(message)
		if err := handler(ctx, tracked); err != nil {
			if tracker.done() {
				return err
			}
			return errors.Join(err, nakMessage(message, time.Second))
		}
		if tracker.done() {
			return nil
		}
		return ackMessage(message)
	})
}

func ackFunc(message StreamMessage) func() error {
	return func() error {
		return ackMessage(message)
	}
}

func nakFunc(message StreamMessage) func(time.Duration) error {
	return func(delay time.Duration) error {
		return nakMessage(message, delay)
	}
}

func ackMessage(message StreamMessage) error {
	if message.Ack == nil {
		return nil
	}
	return message.Ack()
}

func nakMessage(message StreamMessage, delay time.Duration) error {
	if message.Nak == nil {
		return nil
	}
	if delay <= 0 {
		delay = time.Second
	}
	return message.Nak(delay)
}

func positiveDeliveryCount(value int) int {
	if value > 0 {
		return value
	}
	return 1
}

type ackTracker struct {
	doneValue bool
}

func (t *ackTracker) wrap(message StreamMessage) StreamMessage {
	return StreamMessage{
		Data:          message.Data,
		DeliveryCount: message.DeliveryCount,
		Ack: func() error {
			t.doneValue = true
			return ackMessage(message)
		},
		Nak: func(delay time.Duration) error {
			t.doneValue = true
			return nakMessage(message, delay)
		},
	}
}

func (t *ackTracker) done() bool {
	return t != nil && t.doneValue
}
