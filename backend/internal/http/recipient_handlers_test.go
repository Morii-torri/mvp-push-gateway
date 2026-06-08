package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	httpapi "mvp-push-gateway/backend/internal/http"
	"mvp-push-gateway/backend/internal/recipient"
)

func TestUserProfileEndpointSavesUserAndIdentitiesInOneCall(t *testing.T) {
	updatedAt := time.Date(2026, 6, 4, 10, 0, 0, 0, time.UTC)
	recipientService := &fakeRecipientService{
		saveProfileResult: recipient.UserProfile{
			User: recipient.User{
				ID:          "user-1",
				DisplayName: "张三",
				Enabled:     true,
				Attributes:  json.RawMessage(`{"email":"zhangsan@example.com"}`),
				UpdatedAt:   updatedAt,
			},
			Identities: []recipient.UserIdentity{
				{
					ID:            "identity-1",
					UserID:        "user-1",
					ProviderType:  "email",
					IdentityKind:  "email",
					IdentityValue: "zhangsan@example.com",
					Verified:      true,
					UpdatedAt:     updatedAt,
				},
			},
		},
	}
	auditService := &fakeAuditService{}
	handler := httpapi.NewHandler(
		testConfig(),
		httpapi.WithAuthService(fakeAuthService{authenticatedToken: "admin-session"}),
		httpapi.WithRecipientService(recipientService),
		httpapi.WithAuditService(auditService),
	)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/users/user-1/profile", strings.NewReader(`{
		"user":{"display_name":" 张三 ","primary_org_id":"","enabled":true,"attributes":{"email":"zhangsan@example.com"}},
		"identities":[{"id":"identity-1","provider_type":"email","identity_kind":"email","identity_value":"zhangsan@example.com","verified":true}],
		"expected_updated_at":"2026-06-04T10:00:00Z"
	}`))
	req.Header.Set("Authorization", "Bearer admin-session")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if recipientService.saveProfileCalls != 1 {
		t.Fatalf("expected one profile save call, got %d", recipientService.saveProfileCalls)
	}
	if recipientService.savedProfileID != "user-1" {
		t.Fatalf("expected profile id user-1, got %q", recipientService.savedProfileID)
	}
	if recipientService.savedProfileInput.User.DisplayName != " 张三 " || len(recipientService.savedProfileInput.Identities) != 1 {
		t.Fatalf("unexpected profile input: %+v", recipientService.savedProfileInput)
	}
	if auditService.recordCalls != 1 || auditService.recordInputs[0].Action != "update" || auditService.recordInputs[0].ResourceType != "user_profile" {
		t.Fatalf("expected user_profile update audit, calls=%d inputs=%+v", auditService.recordCalls, auditService.recordInputs)
	}

	var body struct {
		User       map[string]any   `json:"user"`
		Identities []map[string]any `json:"identities"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode profile response: %v", err)
	}
	if body.User["id"] != "user-1" || len(body.Identities) != 1 || body.Identities[0]["id"] != "identity-1" {
		t.Fatalf("unexpected profile response: %+v", body)
	}
}

type fakeRecipientService struct {
	saveProfileCalls  int
	savedProfileID    string
	savedProfileInput recipient.UserProfileInput
	saveProfileResult recipient.UserProfile
	saveProfileErr    error

	createProfileCalls  int
	createdProfileInput recipient.UserProfileInput
	createProfileResult recipient.UserProfile
	createProfileErr    error
}

func (f *fakeRecipientService) ListOrgUnits(context.Context) ([]recipient.OrgUnit, error) {
	return nil, nil
}

func (f *fakeRecipientService) CreateOrgUnit(context.Context, recipient.OrgUnitInput) (recipient.OrgUnit, error) {
	return recipient.OrgUnit{}, nil
}

func (f *fakeRecipientService) GetOrgUnit(context.Context, string) (recipient.OrgUnit, error) {
	return recipient.OrgUnit{}, nil
}

func (f *fakeRecipientService) UpdateOrgUnit(context.Context, string, recipient.OrgUnitInput) (recipient.OrgUnit, error) {
	return recipient.OrgUnit{}, nil
}

func (f *fakeRecipientService) DeleteOrgUnit(context.Context, string) error {
	return nil
}

func (f *fakeRecipientService) ListUsers(context.Context) ([]recipient.User, error) {
	return nil, nil
}

func (f *fakeRecipientService) CreateUser(context.Context, recipient.UserInput) (recipient.User, error) {
	return recipient.User{}, nil
}

func (f *fakeRecipientService) CreateUserProfile(_ context.Context, input recipient.UserProfileInput) (recipient.UserProfile, error) {
	f.createProfileCalls++
	f.createdProfileInput = input
	return f.createProfileResult, f.createProfileErr
}

func (f *fakeRecipientService) GetUser(context.Context, string) (recipient.User, error) {
	return recipient.User{}, nil
}

func (f *fakeRecipientService) UpdateUser(context.Context, string, recipient.UserInput) (recipient.User, error) {
	return recipient.User{}, nil
}

func (f *fakeRecipientService) SaveUserProfile(_ context.Context, id string, input recipient.UserProfileInput) (recipient.UserProfile, error) {
	f.saveProfileCalls++
	f.savedProfileID = id
	f.savedProfileInput = input
	return f.saveProfileResult, f.saveProfileErr
}

func (f *fakeRecipientService) DeleteUser(context.Context, string) error {
	return nil
}

func (f *fakeRecipientService) ListUserIdentities(context.Context, string) ([]recipient.UserIdentity, error) {
	return nil, nil
}

func (f *fakeRecipientService) CreateUserIdentity(context.Context, recipient.UserIdentityInput) (recipient.UserIdentity, error) {
	return recipient.UserIdentity{}, nil
}

func (f *fakeRecipientService) UpdateUserIdentity(context.Context, string, recipient.UserIdentityInput) (recipient.UserIdentity, error) {
	return recipient.UserIdentity{}, nil
}

func (f *fakeRecipientService) DeleteUserIdentity(context.Context, string) error {
	return nil
}

func (f *fakeRecipientService) FindUserIdentity(context.Context, string, string, string, string) (recipient.UserIdentity, error) {
	return recipient.UserIdentity{}, nil
}

func (f *fakeRecipientService) ListRecipientGroups(context.Context) ([]recipient.RecipientGroup, error) {
	return nil, nil
}

func (f *fakeRecipientService) CreateRecipientGroup(context.Context, recipient.RecipientGroupInput) (recipient.RecipientGroup, error) {
	return recipient.RecipientGroup{}, nil
}

func (f *fakeRecipientService) GetRecipientGroup(context.Context, string) (recipient.RecipientGroup, error) {
	return recipient.RecipientGroup{}, nil
}

func (f *fakeRecipientService) UpdateRecipientGroup(context.Context, string, recipient.RecipientGroupInput) (recipient.RecipientGroup, error) {
	return recipient.RecipientGroup{}, nil
}

func (f *fakeRecipientService) DeleteRecipientGroup(context.Context, string) error {
	return nil
}
