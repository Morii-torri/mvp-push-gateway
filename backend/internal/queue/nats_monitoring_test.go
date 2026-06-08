package queue_test

import (
	"testing"

	"mvp-push-gateway/backend/internal/queue"
)

func TestNATSPublisherImplementsJetStreamStatsProvider(t *testing.T) {
	var _ queue.JetStreamStatsProvider = (*queue.NATSPublisher)(nil)
}
