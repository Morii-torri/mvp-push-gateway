package queue

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
)

func (p *NATSPublisher) PublishRoutePlanChange(ctx context.Context, sourceID string) error {
	sourceID = strings.TrimSpace(sourceID)
	if p == nil || p.conn == nil || sourceID == "" {
		return ErrInvalidInput
	}
	payload, err := json.Marshal(RoutePlanChangeEvent{SourceID: sourceID})
	if err != nil {
		return err
	}
	msg := &nats.Msg{
		Subject: RouteChangedSubjectPrefix + "." + subjectToken(sourceID, "unknown"),
		Data:    payload,
	}
	if err := p.conn.PublishMsg(msg); err != nil {
		return err
	}
	return waitForNATSFlush(ctx, p.conn)
}

func (p *NATSPublisher) ListenRoutePlanChanges(ctx context.Context, onChange func(string)) error {
	if p == nil || p.conn == nil {
		return ErrInvalidInput
	}
	subscription, err := p.conn.Subscribe(RouteChangedSubjectPrefix+".*", func(message *nats.Msg) {
		sourceID := routePlanChangeSourceID(message)
		if sourceID == "" || onChange == nil {
			return
		}
		onChange(sourceID)
	})
	if err != nil {
		return err
	}
	defer subscription.Unsubscribe()
	if err := waitForNATSFlush(ctx, p.conn); err != nil {
		return err
	}

	<-ctx.Done()
	if errors.Is(ctx.Err(), context.Canceled) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return ctx.Err()
	}
	return nil
}

func routePlanChangeSourceID(message *nats.Msg) string {
	if message == nil {
		return ""
	}
	var event RoutePlanChangeEvent
	if err := json.Unmarshal(message.Data, &event); err == nil {
		if sourceID := strings.TrimSpace(event.SourceID); sourceID != "" {
			return sourceID
		}
	}
	subject := strings.TrimSpace(message.Subject)
	prefix := RouteChangedSubjectPrefix + "."
	if strings.HasPrefix(subject, prefix) {
		return strings.TrimSpace(strings.TrimPrefix(subject, prefix))
	}
	return ""
}

func waitForNATSFlush(ctx context.Context, conn *nats.Conn) error {
	if conn == nil {
		return ErrInvalidInput
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if deadline, ok := ctx.Deadline(); ok {
		timeout := time.Until(deadline)
		if timeout <= 0 {
			return ctx.Err()
		}
		return conn.FlushTimeout(timeout)
	}
	return conn.Flush()
}
