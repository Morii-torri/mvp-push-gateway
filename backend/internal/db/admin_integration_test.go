package db

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"mvp-push-gateway/backend/internal/auth"
)

func TestRepositoryFirstRunSetupLifecycle(t *testing.T) {
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

	service := auth.NewService(NewRepository(pool))

	status, err := service.GetSetupStatus(ctx)
	if err != nil {
		t.Fatalf("get initial setup status: %v", err)
	}
	if !status.SetupOpen {
		t.Fatal("expected setup to be open before admin exists")
	}

	adminUser, err := service.CreateFirstAdmin(ctx, auth.CreateFirstAdminInput{
		Username:    "Admin",
		Password:    "valid-password-123",
		DisplayName: "系统管理员",
	})
	if err != nil {
		t.Fatalf("create first admin: %v", err)
	}
	if adminUser.Username != "admin" || !adminUser.MustChangePassword {
		t.Fatalf("unexpected created admin: %+v", adminUser)
	}

	status, err = service.GetSetupStatus(ctx)
	if err != nil {
		t.Fatalf("get closed setup status: %v", err)
	}
	if status.SetupOpen {
		t.Fatal("expected setup to be closed after admin exists")
	}

	_, err = service.CreateFirstAdmin(ctx, auth.CreateFirstAdminInput{
		Username: "another-admin",
		Password: "valid-password-456",
	})
	if !errors.Is(err, auth.ErrSetupClosed) {
		t.Fatalf("expected second setup attempt to be blocked, got %v", err)
	}

	login, err := service.Login(ctx, auth.LoginInput{
		Username: "admin",
		Password: "valid-password-123",
	})
	if err != nil {
		t.Fatalf("login first admin: %v", err)
	}
	if login.Token == "" {
		t.Fatal("expected login token")
	}

	authenticatedAdmin, err := service.Authenticate(ctx, login.Token)
	if err != nil {
		t.Fatalf("authenticate token: %v", err)
	}
	if authenticatedAdmin.ID != adminUser.ID {
		t.Fatalf("expected authenticated admin %s, got %s", adminUser.ID, authenticatedAdmin.ID)
	}
}

func createMigratedTestSchema(ctx context.Context, t *testing.T, dsn string) string {
	t.Helper()

	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connect postgres: %v", err)
	}
	defer conn.Close(ctx)

	schemaName := "mgp_admin_test_" + strings.ReplaceAll(time.Now().UTC().Format("20060102150405.000000000"), ".", "_")
	if _, err := conn.Exec(ctx, "CREATE SCHEMA "+schemaName); err != nil {
		t.Fatalf("create test schema: %v", err)
	}
	if _, err := conn.Exec(ctx, "SET search_path TO "+schemaName); err != nil {
		t.Fatalf("set search_path: %v", err)
	}
	for _, migration := range readGooseUpMigrations(t) {
		if _, err := conn.Exec(ctx, migration); err != nil {
			t.Fatalf("apply migration: %v", err)
		}
	}
	return schemaName
}

func dropTestSchema(schemaName string) {
	dsn := os.Getenv("MGP_TEST_DATABASE_URL")
	if dsn == "" || schemaName == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return
	}
	defer conn.Close(ctx)
	conn.Exec(ctx, "DROP SCHEMA "+schemaName+" CASCADE")
}
