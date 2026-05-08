package db

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"mvp-push-gateway/backend/internal/recipient"
)

func TestRecipientIdentityLookupAndCRUD(t *testing.T) {
	dsn := os.Getenv("MGP_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("MGP_TEST_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	schemaName := createMigratedTestSchema(ctx, t, dsn)
	defer dropTestSchema(schemaName)

	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parse pool config: %v", err)
	}
	poolConfig.ConnConfig.RuntimeParams["search_path"] = schemaName

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		t.Fatalf("open test pool: %v", err)
	}
	defer pool.Close()

	repository := NewRepository(pool)
	org, err := repository.CreateOrgUnit(ctx, recipient.CreateOrgUnitParams{
		Code: "ops",
		Name: "运维部",
	})
	if err != nil {
		t.Fatalf("create org unit: %v", err)
	}
	user, err := repository.CreateUser(ctx, recipient.CreateUserParams{
		DisplayName:  "张三",
		PrimaryOrgID: org.ID,
		Enabled:      true,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	identity, err := repository.CreateUserIdentity(ctx, recipient.CreateUserIdentityParams{
		UserID:        user.ID,
		ProviderType:  "wecom",
		IdentityKind:  "wecom_userid",
		IdentityValue: "zhangsan",
		Verified:      true,
	})
	if err != nil {
		t.Fatalf("create identity: %v", err)
	}

	found, err := repository.FindUserIdentity(ctx, "wecom", "wecom_userid", "zhangsan")
	if err != nil {
		t.Fatalf("find identity: %v", err)
	}
	if found.ID != identity.ID || found.UserID != user.ID {
		t.Fatalf("unexpected identity lookup result: %+v", found)
	}

	updated, err := repository.UpdateUserIdentity(ctx, identity.ID, recipient.UpdateUserIdentityParams{
		UserID:        user.ID,
		ProviderType:  "wecom",
		IdentityKind:  "mobile",
		IdentityValue: "13800000000",
		Verified:      false,
	})
	if err != nil {
		t.Fatalf("update identity: %v", err)
	}
	if updated.IdentityKind != "mobile" || updated.IdentityValue != "13800000000" || updated.Verified {
		t.Fatalf("unexpected updated identity: %+v", updated)
	}

	if err := repository.DeleteUserIdentity(ctx, identity.ID); err != nil {
		t.Fatalf("delete identity: %v", err)
	}
	if _, err := repository.FindUserIdentity(ctx, "wecom", "mobile", "13800000000"); !errors.Is(err, recipient.ErrNotFound) {
		t.Fatalf("expected missing identity after delete, got %v", err)
	}
}
