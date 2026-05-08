package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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
}

func (f fakeAuthService) GetSetupStatus(context.Context) (auth.SetupStatus, error) {
	return f.status, nil
}

func (fakeAuthService) CreateFirstAdmin(context.Context, auth.CreateFirstAdminInput) (auth.Admin, error) {
	return auth.Admin{}, nil
}

func (fakeAuthService) Login(context.Context, auth.LoginInput) (auth.LoginResult, error) {
	return auth.LoginResult{}, nil
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
