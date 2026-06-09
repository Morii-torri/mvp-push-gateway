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
		Username:        "admin",
		Password:        "valid-password-123",
		ConfirmPassword: "valid-password-123",
	})
	if !errors.Is(err, ErrSetupClosed) {
		t.Fatalf("expected ErrSetupClosed, got %v", err)
	}
}

func TestCreateFirstAdminRequiresPasswordConfirmation(t *testing.T) {
	store := &fakeStore{status: SetupStatus{Initialized: false, AdminCount: 0}}
	service := NewService(store)

	_, err := service.CreateFirstAdmin(context.Background(), CreateFirstAdminInput{
		Username:        "admin",
		Password:        "valid-password-123",
		ConfirmPassword: "different-password-123",
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for mismatched password confirmation, got %v", err)
	}
	if store.created.PasswordHash != "" {
		t.Fatal("expected mismatched password confirmation not to create admin")
	}
}

func TestCreateFirstAdminHashesPassword(t *testing.T) {
	store := &fakeStore{status: SetupStatus{Initialized: false, AdminCount: 0}}
	service := NewService(store)

	admin, err := service.CreateFirstAdmin(context.Background(), CreateFirstAdminInput{
		Username:        "Admin",
		Password:        "valid-password-123",
		ConfirmPassword: "valid-password-123",
		DisplayName:     "Administrator",
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

func TestLoginUsesStoreBackedRateLimitBeforePasswordVerification(t *testing.T) {
	store := &fakeStore{loginReserveBlocked: true}
	service := NewService(store)

	_, err := service.Login(context.Background(), LoginInput{
		Username: "admin",
		Password: "valid-password-123",
		Meta:     SessionMeta{IPAddress: "203.0.113.10"},
	})
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("expected ErrRateLimited, got %v", err)
	}
	if store.reserveLoginAttemptCalls != 1 {
		t.Fatalf("expected one persistent rate limit reservation, got %d", store.reserveLoginAttemptCalls)
	}
	if store.findAdminByUsernameCalls != 0 {
		t.Fatalf("expected rate limit to stop before password lookup, got %d lookups", store.findAdminByUsernameCalls)
	}
}

func TestLoginClearsStoreBackedRateLimitOnSuccess(t *testing.T) {
	hash, err := HashPassword("valid-password-123")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	store := &fakeStore{
		admin: StoredAdmin{
			Admin: Admin{
				ID:       "admin-id",
				Username: "admin",
				Enabled:  true,
			},
			PasswordHash: hash,
		},
	}
	service := NewService(store)

	_, err = service.Login(context.Background(), LoginInput{
		Username: "admin",
		Password: "valid-password-123",
		Meta:     SessionMeta{IPAddress: "203.0.113.10"},
	})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if store.clearLoginAttemptCalls != 1 {
		t.Fatalf("expected successful login to clear persistent rate limit state, got %d", store.clearLoginAttemptCalls)
	}
}

func TestLoginForMissingUserPerformsPasswordVerificationWork(t *testing.T) {
	store := &fakeStore{}
	service := NewService(store)

	startedAt := time.Now()
	_, err := service.Login(context.Background(), LoginInput{
		Username: "missing-admin",
		Password: "wrong-password-123",
		Meta:     SessionMeta{IPAddress: "203.0.113.10"},
	})
	elapsed := time.Since(startedAt)

	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
	if elapsed < 10*time.Millisecond {
		t.Fatalf("expected missing-user login to perform password verification work, finished in %s", elapsed)
	}
}

func TestChangePasswordRevokesAdminSessions(t *testing.T) {
	hash, err := HashPassword("current-password-123")
	if err != nil {
		t.Fatalf("hash current password: %v", err)
	}
	store := &fakeStore{
		admin: StoredAdmin{
			Admin: Admin{
				ID:       "admin-id",
				Username: "admin",
				Enabled:  true,
			},
			PasswordHash: hash,
		},
	}
	service := NewService(store)

	if err := service.ChangePassword(context.Background(), ChangePasswordInput{
		AdminID:         "admin-id",
		CurrentPassword: "current-password-123",
		NewPassword:     "rotated-password-456",
	}); err != nil {
		t.Fatalf("change password: %v", err)
	}
	if store.revokedAdminID != "admin-id" {
		t.Fatalf("expected admin sessions to be revoked, got %q", store.revokedAdminID)
	}
}

func TestUpdateProfileTrimsDisplayName(t *testing.T) {
	store := &fakeStore{}
	service := NewService(store)

	admin, err := service.UpdateProfile(context.Background(), UpdateProfileInput{
		AdminID:     "admin-id",
		DisplayName: "  管理员  ",
	})
	if err != nil {
		t.Fatalf("update profile: %v", err)
	}
	if store.updatedProfileDisplayName != "管理员" {
		t.Fatalf("expected trimmed display name, got %q", store.updatedProfileDisplayName)
	}
	if admin.DisplayName != "管理员" {
		t.Fatalf("expected updated admin display name, got %q", admin.DisplayName)
	}
}

func TestUpdateProfileRejectsEmptyDisplayName(t *testing.T) {
	service := NewService(&fakeStore{})

	_, err := service.UpdateProfile(context.Background(), UpdateProfileInput{
		AdminID:     "admin-id",
		DisplayName: " ",
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

type fakeStore struct {
	status                    SetupStatus
	created                   CreateFirstAdminParams
	admin                     StoredAdmin
	session                   Session
	updatedProfileDisplayName string
	revokedAdminID            string
	loginReserveBlocked       bool
	reserveLoginAttemptCalls  int
	clearLoginAttemptCalls    int
	findAdminByUsernameCalls  int
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
	f.findAdminByUsernameCalls++
	if f.admin.ID == "" {
		return StoredAdmin{}, ErrNotFound
	}
	return f.admin, nil
}

func (f *fakeStore) ReserveLoginAttempt(context.Context, LoginAttemptParams) (bool, error) {
	f.reserveLoginAttemptCalls++
	return !f.loginReserveBlocked, nil
}

func (f *fakeStore) ClearLoginAttempts(context.Context, string, string) error {
	f.clearLoginAttemptCalls++
	return nil
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

func (f *fakeStore) UpdateAdminProfile(_ context.Context, adminID string, displayName string) (Admin, error) {
	f.updatedProfileDisplayName = displayName
	return Admin{
		ID:          adminID,
		Username:    "admin",
		DisplayName: displayName,
		Enabled:     true,
	}, nil
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

func (f *fakeStore) RevokeAdminSessionsByAdminID(_ context.Context, adminID string) error {
	f.revokedAdminID = adminID
	return nil
}
