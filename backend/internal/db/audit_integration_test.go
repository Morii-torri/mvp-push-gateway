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

func TestRepositoryListAuditLogsFiltersByResourceName(t *testing.T) {
	pool := openMigratedPool(t)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	repository := NewRepository(pool)
	if _, err := repository.Record(ctx, audit.RecordInput{
		ActorUsername:    "admin",
		Action:           "update",
		ResourceType:     "source",
		ResourceID:       "source-alpha",
		RequestSnapshot:  []byte(`{"name":"alpha"}`),
		ResponseSnapshot: []byte(`{"ok":true}`),
	}); err != nil {
		t.Fatalf("record first audit log: %v", err)
	}
	if _, err := repository.Record(ctx, audit.RecordInput{
		ActorUsername:    "admin",
		Action:           "update",
		ResourceType:     "source",
		ResourceID:       "source-beta",
		RequestSnapshot:  []byte(`{"name":"beta"}`),
		ResponseSnapshot: []byte(`{"ok":true}`),
	}); err != nil {
		t.Fatalf("record second audit log: %v", err)
	}

	result, err := repository.ListLogs(ctx, audit.ListFilter{
		Actor:        "admin",
		Action:       "update",
		ResourceName: "alpha",
		Limit:        50,
	})
	if err != nil {
		t.Fatalf("list audit logs by resource name: %v", err)
	}
	if result.Total != 1 || len(result.Logs) != 1 || result.Logs[0].ResourceID != "source-alpha" {
		t.Fatalf("expected one matching audit log, got total=%d rows=%+v", result.Total, result.Logs)
	}
}
