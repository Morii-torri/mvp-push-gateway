package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"mvp-push-gateway/backend/internal/auth"
	"mvp-push-gateway/backend/internal/config"
	httpapi "mvp-push-gateway/backend/internal/http"
)

func TestHealthEndpointReturnsServiceMetadata(t *testing.T) {
	cfg := config.Config{
		App: config.AppConfig{
			Name:        "MVP Push Gateway",
			Environment: "test",
		},
		Server: config.ServerConfig{
			APIPrefix: "/api/v1",
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rec := httptest.NewRecorder()

	httpapi.NewHandler(cfg).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected application/json content type, got %q", got)
	}

	var body struct {
		Status      string `json:"status"`
		AppName     string `json:"app_name"`
		Environment string `json:"environment"`
		APIPrefix   string `json:"api_prefix"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode health response: %v", err)
	}

	if body.Status != "ok" {
		t.Fatalf("expected health status ok, got %q", body.Status)
	}
	if body.AppName != cfg.App.Name {
		t.Fatalf("expected app name %q, got %q", cfg.App.Name, body.AppName)
	}
	if body.Environment != cfg.App.Environment {
		t.Fatalf("expected environment %q, got %q", cfg.App.Environment, body.Environment)
	}
	if body.APIPrefix != cfg.Server.APIPrefix {
		t.Fatalf("expected API prefix %q, got %q", cfg.Server.APIPrefix, body.APIPrefix)
	}
}

func TestSetupStatusEndpointReturnsOpenState(t *testing.T) {
	handler := httpapi.NewHandler(testConfig(), httpapi.WithAuthService(fakeAuthService{
		status: auth.SetupStatus{Initialized: false, SetupOpen: true, AdminCount: 0},
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/setup/status", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var body struct {
		Initialized bool `json:"initialized"`
		SetupOpen   bool `json:"setup_open"`
		AdminCount  int  `json:"admin_count"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode setup status: %v", err)
	}
	if body.Initialized || !body.SetupOpen || body.AdminCount != 0 {
		t.Fatalf("unexpected open setup status: %+v", body)
	}
}

func TestSetupStatusEndpointReturnsClosedState(t *testing.T) {
	handler := httpapi.NewHandler(testConfig(), httpapi.WithAuthService(fakeAuthService{
		status: auth.SetupStatus{Initialized: true, SetupOpen: false, AdminCount: 1},
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/setup/status", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var body struct {
		Initialized bool `json:"initialized"`
		SetupOpen   bool `json:"setup_open"`
		AdminCount  int  `json:"admin_count"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode setup status: %v", err)
	}
	if !body.Initialized || body.SetupOpen || body.AdminCount != 1 {
		t.Fatalf("unexpected closed setup status: %+v", body)
	}
}

func TestProfileEndpointUpdatesCurrentAdminDisplayName(t *testing.T) {
	handler := httpapi.NewHandler(testConfig(), httpapi.WithAuthService(fakeAuthService{
		authenticatedToken: "admin-session",
	}))

	req := httptest.NewRequest(http.MethodPut, "/api/v1/auth/profile", strings.NewReader(`{"display_name":"管理员"}`))
	req.Header.Set("Authorization", "Bearer admin-session")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var body struct {
		Admin struct {
			DisplayName string `json:"display_name"`
		} `json:"admin"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode profile response: %v", err)
	}
	if body.Admin.DisplayName != "管理员" {
		t.Fatalf("expected updated display name, got %q", body.Admin.DisplayName)
	}
}

func TestAuthHandlersRecordSecurityAudit(t *testing.T) {
	auditService := &fakeAuditService{}
	handler := httpapi.NewHandler(
		testConfig(),
		httpapi.WithAuthService(fakeAuthService{
			authenticatedToken: "admin-session",
			loginResult: auth.LoginResult{
				Token:     "admin-session",
				ExpiresAt: time.Date(2026, 6, 4, 10, 0, 0, 0, time.UTC),
				Admin: auth.Admin{
					ID:          "00000000-0000-0000-0000-000000000001",
					Username:    "admin",
					DisplayName: "Admin",
					Enabled:     true,
				},
			},
		}),
		httpapi.WithAuditService(auditService),
	)

	loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"username":"admin","password":"ChangeMe2026!"}`))
	loginRec := httptest.NewRecorder()
	handler.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("expected login status 200, got %d body=%s", loginRec.Code, loginRec.Body.String())
	}
	if auditService.recordCalls != 1 || auditService.recordInputs[0].Action != "login" || auditService.recordInputs[0].ResourceType != "admin_session" {
		t.Fatalf("expected login audit record, calls=%d inputs=%+v", auditService.recordCalls, auditService.recordInputs)
	}
	if strings.Contains(string(auditService.recordInputs[0].RequestSnapshot), "ChangeMe2026") {
		t.Fatalf("expected login password to be redacted, got %s", auditService.recordInputs[0].RequestSnapshot)
	}

	logoutReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	logoutReq.Header.Set("Authorization", "Bearer admin-session")
	logoutRec := httptest.NewRecorder()
	handler.ServeHTTP(logoutRec, logoutReq)
	if logoutRec.Code != http.StatusOK {
		t.Fatalf("expected logout status 200, got %d body=%s", logoutRec.Code, logoutRec.Body.String())
	}
	if auditService.recordCalls != 2 || auditService.recordInputs[1].Action != "logout" {
		t.Fatalf("expected logout audit record, calls=%d inputs=%+v", auditService.recordCalls, auditService.recordInputs)
	}
}

func TestLoginFailureRecordsSecurityAudit(t *testing.T) {
	auditService := &fakeAuditService{}
	handler := httpapi.NewHandler(
		testConfig(),
		httpapi.WithAuthService(fakeAuthService{loginErr: auth.ErrInvalidCredentials}),
		httpapi.WithAuditService(auditService),
	)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"username":"admin","password":"wrong"}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected login failure status 401, got %d body=%s", rec.Code, rec.Body.String())
	}
	if auditService.recordCalls != 1 || auditService.recordInputs[0].Action != "login_failed" {
		t.Fatalf("expected login_failed audit record, calls=%d inputs=%+v", auditService.recordCalls, auditService.recordInputs)
	}
	if strings.Contains(string(auditService.recordInputs[0].RequestSnapshot), "wrong") {
		t.Fatalf("expected failed login password to be redacted, got %s", auditService.recordInputs[0].RequestSnapshot)
	}
}

func testConfig() config.Config {
	return config.Config{
		App: config.AppConfig{
			Name:        "MVP Push Gateway",
			Environment: "test",
		},
		Server: config.ServerConfig{
			APIPrefix: "/api/v1",
		},
	}
}

type fakeAuthService struct {
	status             auth.SetupStatus
	authenticatedToken string
	loginResult        auth.LoginResult
	loginErr           error
}

func (f fakeAuthService) GetSetupStatus(context.Context) (auth.SetupStatus, error) {
	return f.status, nil
}

func (fakeAuthService) CreateFirstAdmin(context.Context, auth.CreateFirstAdminInput) (auth.Admin, error) {
	return auth.Admin{}, nil
}

func (f fakeAuthService) Login(context.Context, auth.LoginInput) (auth.LoginResult, error) {
	if f.loginErr != nil {
		return auth.LoginResult{}, f.loginErr
	}
	if f.loginResult.Token != "" {
		return f.loginResult, nil
	}
	return auth.LoginResult{
		Token:     "admin-session",
		ExpiresAt: time.Date(2026, 6, 4, 10, 0, 0, 0, time.UTC),
		Admin: auth.Admin{
			ID:          "00000000-0000-0000-0000-000000000001",
			Username:    "admin",
			DisplayName: "Admin",
			Enabled:     true,
		},
	}, nil
}

func (f fakeAuthService) Authenticate(_ context.Context, token string) (auth.Admin, error) {
	if f.authenticatedToken != "" && token == f.authenticatedToken {
		return auth.Admin{
			ID:          "admin-1",
			Username:    "admin",
			DisplayName: "Admin",
			Enabled:     true,
		}, nil
	}
	return auth.Admin{}, auth.ErrUnauthorized
}

func (fakeAuthService) Logout(context.Context, string) error {
	return nil
}

func (fakeAuthService) ChangePassword(context.Context, auth.ChangePasswordInput) error {
	return nil
}

func (fakeAuthService) UpdateProfile(_ context.Context, input auth.UpdateProfileInput) (auth.Admin, error) {
	return auth.Admin{
		ID:          input.AdminID,
		Username:    "admin",
		DisplayName: input.DisplayName,
		Enabled:     true,
	}, nil
}
