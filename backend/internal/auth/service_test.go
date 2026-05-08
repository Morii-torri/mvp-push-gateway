package auth

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestSetupStatusOpenWhenNoAdminExists(t *testing.T) {
	store := &fakeStore{status: SetupStatus{Initialized: false, AdminCount: 0}}
	service := NewService(store)

	status, err := service.GetSetupStatus(context.Background())
	if err != nil {
		t.Fatalf("get setup status: %v", err)
	}
	if !status.SetupOpen {
		t.Fatal("expected setup to be open")
	}
}

func TestSetupStatusClosedWhenAdminExists(t *testing.T) {
	store := &fakeStore{status: SetupStatus{Initialized: true, AdminCount: 1}}
	service := NewService(store)

	status, err := service.GetSetupStatus(context.Background())
	if err != nil {
		t.Fatalf("get setup status: %v", err)
	}
	if status.SetupOpen {
		t.Fatal("expected setup to be closed")
	}
}

func TestCreateFirstAdminRejectsClosedSetup(t *testing.T) {
	store := &fakeStore{status: SetupStatus{Initialized: true, AdminCount: 1}}
	service := NewService(store)

	_, err := service.CreateFirstAdmin(context.Background(), CreateFirstAdminInput{
		Username: "admin",
		Password: "valid-password-123",
	})
	if !errors.Is(err, ErrSetupClosed) {
		t.Fatalf("expected ErrSetupClosed, got %v", err)
	}
}

func TestCreateFirstAdminHashesPassword(t *testing.T) {
	store := &fakeStore{status: SetupStatus{Initialized: false, AdminCount: 0}}
	service := NewService(store)

	admin, err := service.CreateFirstAdmin(context.Background(), CreateFirstAdminInput{
		Username:    "Admin",
		Password:    "valid-password-123",
		DisplayName: "Administrator",
	})
	if err != nil {
		t.Fatalf("create first admin: %v", err)
	}
	if admin.Username != "admin" {
		t.Fatalf("expected normalized username admin, got %q", admin.Username)
	}
	if store.created.PasswordHash == "" || store.created.PasswordHash == "valid-password-123" {
		t.Fatal("expected stored password hash, not raw password")
	}
	ok, err := VerifyPassword("valid-password-123", store.created.PasswordHash)
	if err != nil {
		t.Fatalf("verify stored hash: %v", err)
	}
	if !ok {
		t.Fatal("expected stored hash to verify")
	}
}

type fakeStore struct {
	status  SetupStatus
	created CreateFirstAdminParams
	admin   StoredAdmin
	session Session
}

func (f *fakeStore) GetSetupStatus(context.Context) (SetupStatus, error) {
	return f.status, nil
}

func (f *fakeStore) CreateFirstAdmin(_ context.Context, params CreateFirstAdminParams) (Admin, error) {
	f.created = params
	return Admin{
		ID:                 "admin-id",
		Username:           params.Username,
		DisplayName:        params.DisplayName,
		MustChangePassword: params.MustChangePassword,
		Enabled:            true,
	}, nil
}

func (f *fakeStore) FindAdminByUsername(context.Context, string) (StoredAdmin, error) {
	if f.admin.ID == "" {
		return StoredAdmin{}, ErrNotFound
	}
	return f.admin, nil
}

func (f *fakeStore) FindAdminByID(context.Context, string) (StoredAdmin, error) {
	if f.admin.ID == "" {
		return StoredAdmin{}, ErrNotFound
	}
	return f.admin, nil
}

func (f *fakeStore) UpdateAdminPassword(context.Context, string, string, bool) error {
	return nil
}

func (f *fakeStore) CreateAdminSession(context.Context, CreateSessionParams) error {
	return nil
}

func (f *fakeStore) FindAdminBySessionTokenHash(context.Context, string, time.Time) (Session, error) {
	if f.session.ID == "" {
		return Session{}, ErrNotFound
	}
	return f.session, nil
}

func (f *fakeStore) RevokeAdminSession(context.Context, string) error {
	return nil
}
