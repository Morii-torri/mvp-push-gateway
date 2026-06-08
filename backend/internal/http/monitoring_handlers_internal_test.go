package httpapi

import (
	"context"
	"testing"
	"time"
)

type fakeNotificationIntervalReader struct {
	value int
}

func (f fakeNotificationIntervalReader) IntSetting(context.Context, string, int) int {
	return f.value
}

func TestNotificationStreamIntervalUsesSettings(t *testing.T) {
	if got := notificationStreamInterval(context.Background(), fakeNotificationIntervalReader{value: 1}); got != time.Second {
		t.Fatalf("expected notification stream interval to use settings value, got %s", got)
	}
	if got := notificationStreamInterval(context.Background(), fakeNotificationIntervalReader{value: 0}); got != defaultNotificationStreamInterval {
		t.Fatalf("expected invalid interval to fall back to default, got %s", got)
	}
	if got := notificationStreamInterval(context.Background(), nil); got != defaultNotificationStreamInterval {
		t.Fatalf("expected missing settings reader to fall back to default, got %s", got)
	}
}
