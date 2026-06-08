package db

import (
	"context"
	"testing"
	"time"

	"mvp-push-gateway/backend/internal/audit"
)

func TestRepositoryRecordAuditPreservesTextResourceID(t *testing.T) {
	pool := openMigratedPool(t)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	repository := NewRepository(pool)
	record, err := repository.Record(ctx, audit.RecordInput{
		ActorUsername:    "admin",
		Action:           "update",
		ResourceType:     "system_setting",
		ResourceID:       "logs.retention_days",
		RequestSnapshot:  []byte(`{"value":45}`),
		ResponseSnapshot: []byte(`{"ok":true}`),
	})
	if err != nil {
		t.Fatalf("record audit log: %v", err)
	}
	if record.ResourceID != "logs.retention_days" {
		t.Fatalf("expected text resource id to be preserved, got %q", record.ResourceID)
	}

	loaded, err := repository.GetLog(ctx, record.ID)
	if err != nil {
		t.Fatalf("get audit log: %v", err)
	}
	if loaded.ResourceID != "logs.retention_days" {
		t.Fatalf("expected loaded text resource id to be preserved, got %q", loaded.ResourceID)
	}
}
