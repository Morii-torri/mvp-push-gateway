package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"mvp-push-gateway/backend/internal/auth"
	"mvp-push-gateway/backend/internal/config"
	dbrepo "mvp-push-gateway/backend/internal/db"
	httpapi "mvp-push-gateway/backend/internal/http"
	"mvp-push-gateway/backend/internal/provider"
	"mvp-push-gateway/backend/internal/route"
	"mvp-push-gateway/backend/internal/source"
	msgtemplate "mvp-push-gateway/backend/internal/template"
)

func TestFreshEnvironmentHTTPFlowCoversSetupAuthSourceTemplateRouteAndIngest(t *testing.T) {
	pool := openMigratedPool(t)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	repository := dbrepo.NewRepository(pool)
	now := time.Date(2026, 5, 11, 9, 30, 0, 0, time.UTC)
	providerService := provider.NewService(repository)
	if err := providerService.SeedProviderCapabilities(ctx); err != nil {
		t.Fatalf("seed provider capabilities: %v", err)
	}

	handler := httpapi.NewHandler(
		integrationTestConfig(),
		httpapi.WithAuthService(auth.NewService(repository)),
		httpapi.WithSourceService(source.NewService(
			repository,
			source.WithNow(func() time.Time { return now }),
			source.WithTraceIDGenerator(func() string { return "trace-http-flow-001" }),
			source.WithLatestPayloadFlushInterval(10*time.Millisecond),
		)),
		httpapi.WithProviderService(providerService),
		httpapi.WithTemplateService(msgtemplate.NewService(repository)),
		httpapi.WithRouteService(route.NewService(
			repository,
			route.WithNow(func() time.Time { return now }),
		)),
	)

	setupStatus := doJSON[setupStatusBody](t, handler, http.MethodGet, "/api/v1/setup/status", "", nil, http.StatusOK)
	if !setupStatus.SetupOpen || setupStatus.Initialized || setupStatus.AdminCount != 0 {
		t.Fatalf("expected fresh setup to be open, got %+v", setupStatus)
	}

	adminCreated := doJSON[setupAdminBody](t, handler, http.MethodPost, "/api/v1/setup/admin", "", map[string]any{
		"username":         "Admin",
		"password":         "valid-password-123",
		"confirm_password": "valid-password-123",
		"display_name":     "系统管理员",
	}, http.StatusCreated)
	if adminCreated.Admin.Username != "admin" || !adminCreated.Admin.MustChangePassword {
		t.Fatalf("unexpected created admin: %+v", adminCreated.Admin)
	}

	closedStatus := doJSON[setupStatusBody](t, handler, http.MethodGet, "/api/v1/setup/status", "", nil, http.StatusOK)
	if closedStatus.SetupOpen || !closedStatus.Initialized || closedStatus.AdminCount != 1 {
		t.Fatalf("expected setup to close after first admin creation, got %+v", closedStatus)
	}

	assertStatusCode(t, handler, http.MethodPost, "/api/v1/setup/admin", "", map[string]any{
		"username":         "second-admin",
		"password":         "valid-password-456",
		"confirm_password": "valid-password-456",
	}, http.StatusConflict)

	login, loginRec := doJSONWithResponse[loginBody](t, handler, http.MethodPost, "/api/v1/auth/login", "", map[string]any{
		"username": "admin",
		"password": "valid-password-123",
	}, http.StatusOK)
	if login.Token != "" || login.TokenType != "" {
		t.Fatalf("unexpected login result: %+v", login)
	}
	sessionToken := adminSessionTokenFromResponse(t, loginRec)

	me := doJSON[meBody](t, handler, http.MethodGet, "/api/v1/auth/me", sessionToken, nil, http.StatusOK)
	if me.Admin.ID != adminCreated.Admin.ID || me.Admin.Username != "admin" {
		t.Fatalf("unexpected /auth/me response: %+v", me.Admin)
	}

	changePassword := doJSON[okBody](t, handler, http.MethodPost, "/api/v1/auth/change-password", sessionToken, map[string]any{
		"current_password": "valid-password-123",
		"new_password":     "rotated-password-456",
	}, http.StatusOK)
	if !changePassword.OK {
		t.Fatalf("expected password change success, got %+v", changePassword)
	}
	assertStatusCode(t, handler, http.MethodGet, "/api/v1/auth/me", sessionToken, nil, http.StatusUnauthorized)

	assertStatusCode(t, handler, http.MethodPost, "/api/v1/auth/login", "", map[string]any{
		"username": "admin",
		"password": "valid-password-123",
	}, http.StatusUnauthorized)

	rotatedLogin, rotatedLoginRec := doJSONWithResponse[loginBody](t, handler, http.MethodPost, "/api/v1/auth/login", "", map[string]any{
		"username": "admin",
		"password": "rotated-password-456",
	}, http.StatusOK)
	if rotatedLogin.Token != "" || rotatedLogin.TokenType != "" {
		t.Fatalf("expected rotated password login to keep bearer token out of JSON, got %+v", rotatedLogin)
	}
	rotatedSessionToken := adminSessionTokenFromResponse(t, rotatedLoginRec)

	sourceCreated := doJSON[sourceCreateBody](t, handler, http.MethodPost, "/api/v1/sources", rotatedSessionToken, map[string]any{
		"code":                    "orders",
		"name":                    "Orders",
		"auth_mode":               "token",
		"auth_token":              "sourcetoken001",
		"inbound_dedupe_enabled":  true,
		"inbound_dedupe_strategy": "payload_hash",
		"inbound_dedupe_config":   map[string]any{},
		"rate_limit_config":       map[string]any{},
	}, http.StatusCreated)
	if sourceCreated.Source.Code != "orders" || sourceCreated.Source.AuthToken != "sourcetoken001" {
		t.Fatalf("unexpected created source: %+v", sourceCreated.Source)
	}
	if string(sourceCreated.Source.LatestPayloadSample) != "null" {
		t.Fatalf("expected no latest payload sample before ingest, got %s", sourceCreated.Source.LatestPayloadSample)
	}

	templateCreated := doJSON[templateBody](t, handler, http.MethodPost, "/api/v1/templates", rotatedSessionToken, map[string]any{
		"name":        "Alert Template",
		"description": "Title renderer",
		"source_id":   sourceCreated.Source.ID,
		"enabled":     true,
	}, http.StatusCreated)
	if templateCreated.Template.SourceID != sourceCreated.Source.ID {
		t.Fatalf("unexpected template source: %+v", templateCreated.Template)
	}

	templateVersion := doJSON[templateVersionBody](t, handler, http.MethodPost, "/api/v1/templates/"+templateCreated.Template.ID+"/publish", rotatedSessionToken, map[string]any{
		"message_type":         "json",
		"target_provider_type": "webhook",
		"template_body":        `{"body":{"title":"{{ payload.title }}"}}`,
		"message_body_schema":  map[string]any{},
		"sample_payload":       map[string]any{"title": "critical"},
	}, http.StatusCreated)
	if templateVersion.Version.VersionNo != 1 || templateVersion.Version.ValidationStatus != "valid" {
		t.Fatalf("unexpected template version: %+v", templateVersion.Version)
	}
	if len(templateVersion.Version.UsedVariables) != 1 || templateVersion.Version.UsedVariables[0] != "payload.title" {
		t.Fatalf("expected payload.title variable capture, got %+v", templateVersion.Version.UsedVariables)
	}

	channelCreated := doJSON[channelBody](t, handler, http.MethodPost, "/api/v1/channels", rotatedSessionToken, map[string]any{
		"provider_type":      "webhook",
		"name":               "Test Webhook",
		"enabled":            true,
		"send_config":        map[string]any{"method": "POST", "url": "https://example.test/send", "recipient": map[string]any{"location": "none"}},
		"rate_limit_config":  map[string]any{},
		"concurrency_limit":  1,
		"timeout_ms":         1000,
		"retry_policy":       map[string]any{"max_attempts": 2},
		"dead_letter_policy": map[string]any{},
	}, http.StatusCreated)
	if channelCreated.Channel.ProviderType != "webhook" {
		t.Fatalf("unexpected channel: %+v", channelCreated.Channel)
	}

	flowList := doJSON[routeFlowListBody](t, handler, http.MethodGet, "/api/v1/route-flows", rotatedSessionToken, nil, http.StatusOK)
	flowCreated := routeFlowBody{}
	for _, item := range flowList.Flows {
		if item.SourceID == sourceCreated.Source.ID {
			flowCreated.Flow.ID = item.ID
			flowCreated.Flow.SourceID = item.SourceID
			break
		}
	}
	if flowCreated.Flow.ID == "" {
		t.Fatalf("expected source creation to prepare route flow, got %+v", flowList.Flows)
	}
	ruleKey := "00000000-0000-0000-0000-000000018001"

	savedRules := doJSON[routeRulesBody](t, handler, http.MethodPut, "/api/v1/route-flows/"+flowCreated.Flow.ID+"/rules", rotatedSessionToken, map[string]any{
		"rules": []map[string]any{
			{
				"rule_key":       ruleKey,
				"sort_order":     10,
				"name":           "Critical title",
				"condition_tree": map[string]any{"operator": "equals", "path": "payload.title", "value": "critical"},
				"enabled":        true,
				"action": map[string]any{
					"targets": []map[string]any{
						{
							"channel_id":          channelCreated.Channel.ID,
							"template_version_id": templateVersion.Version.ID,
							"enabled":             true,
						},
					},
					"recipient_strategy": map[string]any{},
					"send_dedupe_config": map[string]any{},
					"failure_policy":     map[string]any{},
				},
			},
		},
	}, http.StatusOK)
	if len(savedRules.Rules) != 1 || len(savedRules.Rules[0].Action.Targets) != 1 {
		t.Fatalf("unexpected saved rules: %+v", savedRules.Rules)
	}
	if savedRules.Rules[0].Action.Targets[0].TemplateVersionID != templateVersion.Version.ID || savedRules.Rules[0].Action.Targets[0].ChannelID != channelCreated.Channel.ID {
		t.Fatalf("unexpected saved rule target: %+v", savedRules.Rules[0].Action.Targets[0])
	}
	if savedRules.Rules[0].Action.TemplateVersionID != templateVersion.Version.ID || len(savedRules.Rules[0].Action.ChannelIDs) != 1 || savedRules.Rules[0].Action.ChannelIDs[0] != channelCreated.Channel.ID {
		t.Fatalf("unexpected saved rule compatibility fields: %+v", savedRules.Rules[0].Action)
	}
	if savedRules.Rules[0].RuleKey != ruleKey {
		t.Fatalf("expected saved rule key %s, got %+v", ruleKey, savedRules.Rules[0])
	}

	validation := doJSON[routeValidationBody](t, handler, http.MethodPost, "/api/v1/route-flows/"+flowCreated.Flow.ID+"/validate", rotatedSessionToken, nil, http.StatusOK)
	if validation.Status != "valid" || validation.VersionID == "" {
		t.Fatalf("unexpected route validation result: %+v", validation)
	}

	published := doJSON[routeVersionBody](t, handler, http.MethodPost, "/api/v1/route-flows/"+flowCreated.Flow.ID+"/publish", rotatedSessionToken, nil, http.StatusOK)
	if published.Version.ValidationStatus != "valid" || published.Version.ID == "" {
		t.Fatalf("unexpected published route version: %+v", published.Version)
	}

	simulated := doJSON[routeSimulationBody](t, handler, http.MethodPost, "/api/v1/route-flows/"+flowCreated.Flow.ID+"/simulate", rotatedSessionToken, map[string]any{
		"payload": map[string]any{"title": "critical"},
	}, http.StatusOK)
	if simulated.StopReason != "first_match_stop" || simulated.MatchedRule == nil || simulated.MatchedRule.RuleKey != ruleKey {
		t.Fatalf("unexpected route simulation result: %+v", simulated)
	}

	logout := doJSON[okBody](t, handler, http.MethodPost, "/api/v1/auth/logout", rotatedSessionToken, nil, http.StatusOK)
	if !logout.OK {
		t.Fatalf("expected logout success, got %+v", logout)
	}
	assertStatusCode(t, handler, http.MethodGet, "/api/v1/auth/me", rotatedSessionToken, nil, http.StatusUnauthorized)

	ingest := doJSON[ingestBody](t, handler, http.MethodPost, "/api/v1/ingest/orders", "sourcetoken001", map[string]any{
		"title":   "critical",
		"content": "paid",
	}, http.StatusAccepted)
	if ingest.TraceID != "trace-http-flow-001" || ingest.Status != "accepted" {
		t.Fatalf("unexpected ingest response: %+v", ingest)
	}

	waitForLatestPayloadSample(t, ctx, pool, sourceCreated.Source.ID)
	var latestPayload string
	var latestPayloadAt *time.Time
	if err := pool.QueryRow(ctx, `
		SELECT latest_payload_sample::text, latest_payload_sample_updated_at
		FROM inbound_sources
		WHERE id = $1
	`, sourceCreated.Source.ID).Scan(&latestPayload, &latestPayloadAt); err != nil {
		t.Fatalf("query latest payload sample: %v", err)
	}
	var latestPayloadValue map[string]any
	if err := json.Unmarshal([]byte(latestPayload), &latestPayloadValue); err != nil {
		t.Fatalf("decode latest payload sample: %v", err)
	}
	if latestPayloadValue["title"] != "critical" || latestPayloadValue["content"] != "paid" || latestPayloadAt == nil {
		t.Fatalf("expected latest payload sample to persist after ingest, got payload=%s updated_at=%v", latestPayload, latestPayloadAt)
	}

	var routePlanJobs int
	var acceptedMessages int
	if err := pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*)::integer FROM jobs WHERE type = 'route_plan'),
			(SELECT count(*)::integer FROM message_records WHERE source_id = $1 AND status = 'accepted')
	`, sourceCreated.Source.ID).Scan(&routePlanJobs, &acceptedMessages); err != nil {
		t.Fatalf("query ingest side effects: %v", err)
	}
	if routePlanJobs != 1 || acceptedMessages != 1 {
		t.Fatalf("expected one accepted message and one route_plan job, got jobs=%d messages=%d", routePlanJobs, acceptedMessages)
	}
}

type setupStatusBody struct {
	Initialized bool `json:"initialized"`
	SetupOpen   bool `json:"setup_open"`
	AdminCount  int  `json:"admin_count"`
}

type adminJSON struct {
	ID                 string `json:"id"`
	Username           string `json:"username"`
	DisplayName        string `json:"display_name"`
	MustChangePassword bool   `json:"must_change_password"`
	Enabled            bool   `json:"enabled"`
}

type setupAdminBody struct {
	Admin adminJSON `json:"admin"`
}

type loginBody struct {
	Token     string    `json:"token"`
	TokenType string    `json:"token_type"`
	ExpiresAt string    `json:"expires_at"`
	Admin     adminJSON `json:"admin"`
}

type meBody struct {
	Admin adminJSON `json:"admin"`
}

type okBody struct {
	OK bool `json:"ok"`
}

type sourceCreateBody struct {
	Source struct {
		ID                  string          `json:"id"`
		Code                string          `json:"code"`
		AuthToken           string          `json:"auth_token"`
		LatestPayloadSample json.RawMessage `json:"latest_payload_sample"`
	} `json:"source"`
}

type templateBody struct {
	Template struct {
		ID       string `json:"id"`
		SourceID string `json:"source_id"`
	} `json:"template"`
}

type templateVersionBody struct {
	Version struct {
		ID               string   `json:"id"`
		VersionNo        int      `json:"version_no"`
		ValidationStatus string   `json:"validation_status"`
		UsedVariables    []string `json:"used_variables"`
	} `json:"version"`
}

type channelBody struct {
	Channel struct {
		ID           string `json:"id"`
		ProviderType string `json:"provider_type"`
	} `json:"channel"`
}

type routeFlowBody struct {
	Flow struct {
		ID       string `json:"id"`
		SourceID string `json:"source_id"`
	} `json:"flow"`
}

type routeFlowListBody struct {
	Flows []struct {
		ID       string `json:"id"`
		SourceID string `json:"source_id"`
	} `json:"flows"`
}

type routeRulesBody struct {
	VersionID string `json:"version_id"`
	Rules     []struct {
		RuleKey string `json:"rule_key"`
		Action  struct {
			TemplateVersionID string   `json:"template_version_id"`
			ChannelIDs        []string `json:"channel_ids"`
			Targets           []struct {
				ChannelID         string `json:"channel_id"`
				TemplateVersionID string `json:"template_version_id"`
				Enabled           bool   `json:"enabled"`
				SortOrder         int    `json:"sort_order"`
			} `json:"targets"`
		} `json:"action"`
	} `json:"rules"`
}

type routeValidationBody struct {
	VersionID string `json:"version_id"`
	Status    string `json:"status"`
	Errors    []struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Path    string `json:"path"`
	} `json:"errors"`
}

type routeVersionBody struct {
	Version struct {
		ID               string `json:"id"`
		ValidationStatus string `json:"validation_status"`
	} `json:"version"`
}

type routeSimulationBody struct {
	StopReason  string `json:"stop_reason"`
	MatchedRule *struct {
		RuleKey string `json:"rule_key"`
	} `json:"matched_rule"`
}

type ingestBody struct {
	TraceID string `json:"trace_id"`
	Status  string `json:"status"`
}

func doJSON[T any](t *testing.T, handler http.Handler, method string, path string, bearerToken string, requestBody any, expectedStatus int) T {
	t.Helper()
	body, _ := doJSONWithResponse[T](t, handler, method, path, bearerToken, requestBody, expectedStatus)
	return body
}

func doJSONWithResponse[T any](t *testing.T, handler http.Handler, method string, path string, bearerToken string, requestBody any, expectedStatus int) (T, *httptest.ResponseRecorder) {
	t.Helper()

	req := newJSONRequest(t, method, path, bearerToken, requestBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != expectedStatus {
		t.Fatalf("expected %s %s to return %d, got %d body=%s", method, path, expectedStatus, rec.Code, rec.Body.String())
	}

	var body T
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode %s %s response: %v body=%s", method, path, err, rec.Body.String())
	}
	return body, rec
}

func adminSessionTokenFromResponse(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == "mgp_admin_session" && cookie.Value != "" && cookie.HttpOnly {
			return cookie.Value
		}
	}
	t.Fatalf("expected login response to set HttpOnly admin session cookie, got %+v", rec.Result().Cookies())
	return ""
}

func assertStatusCode(t *testing.T, handler http.Handler, method string, path string, sessionOrSourceToken string, requestBody any, expectedStatus int) {
	t.Helper()

	req := newJSONRequest(t, method, path, sessionOrSourceToken, requestBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != expectedStatus {
		t.Fatalf("expected %s %s to return %d, got %d body=%s", method, path, expectedStatus, rec.Code, rec.Body.String())
	}
}

func newJSONRequest(t *testing.T, method string, path string, sessionOrSourceToken string, requestBody any) *http.Request {
	t.Helper()

	var body bytes.Buffer
	if requestBody != nil {
		if err := json.NewEncoder(&body).Encode(requestBody); err != nil {
			t.Fatalf("encode request body: %v", err)
		}
	}

	req := httptest.NewRequest(method, path, &body)
	req.RemoteAddr = "127.0.0.1:42310"
	if requestBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if sessionOrSourceToken != "" {
		if strings.HasPrefix(path, "/api/v1/ingest/") {
			req.Header.Set("Authorization", "Bearer "+sessionOrSourceToken)
		} else {
			setAdminSessionCookie(req, sessionOrSourceToken)
		}
	}
	return req
}

func integrationTestConfig() config.Config {
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

func waitForLatestPayloadSample(t *testing.T, ctx context.Context, pool *pgxpool.Pool, sourceID string) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		var exists bool
		if err := pool.QueryRow(ctx, `
			SELECT latest_payload_sample IS NOT NULL
			FROM inbound_sources
			WHERE id = $1
		`, sourceID).Scan(&exists); err != nil {
			t.Fatalf("query latest payload visibility: %v", err)
		}
		if exists {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("latest payload sample was not flushed for source %s", sourceID)
}

func openMigratedPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dsn := os.Getenv("MGP_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("MGP_TEST_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	schemaName := createMigratedTestSchema(ctx, t, dsn)
	t.Cleanup(func() {
		dropTestSchema(schemaName)
	})

	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parse pool config: %v", err)
	}
	poolConfig.ConnConfig.RuntimeParams["search_path"] = schemaName

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		t.Fatalf("open test pool: %v", err)
	}
	return pool
}

func createMigratedTestSchema(ctx context.Context, t *testing.T, dsn string) string {
	t.Helper()

	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connect postgres: %v", err)
	}
	defer conn.Close(ctx)

	schemaName := "mgp_http_test_" + strings.ReplaceAll(time.Now().UTC().Format("20060102150405.000000000"), ".", "_")
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

func readGooseUpMigrations(t *testing.T) []string {
	t.Helper()

	paths, err := filepath.Glob("../../migrations/*.sql")
	if err != nil {
		t.Fatalf("list migrations: %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("expected at least one migration")
	}

	migrations := make([]string, 0, len(paths))
	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read migration %s: %v", path, err)
		}
		migrations = append(migrations, extractGooseUp(string(content)))
	}
	return migrations
}

func extractGooseUp(migration string) string {
	var builder strings.Builder
	for _, line := range strings.Split(migration, "\n") {
		if strings.HasPrefix(line, "-- +goose Down") {
			break
		}
		builder.WriteString(line)
		builder.WriteByte('\n')
	}
	return builder.String()
}
