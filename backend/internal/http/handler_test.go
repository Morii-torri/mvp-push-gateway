package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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
