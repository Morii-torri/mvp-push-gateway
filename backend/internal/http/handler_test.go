package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
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
	setAdminSessionCookie(req, "admin-session")
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

func TestLoginSetsHttpOnlySessionCookieAndDoesNotReturnBearerToken(t *testing.T) {
	handler := httpapi.NewHandler(
		testConfig(),
		httpapi.WithLoginCaptchaAnswerForTesting("ABC234"),
		httpapi.WithAuthService(fakeAuthService{
			loginResult: auth.LoginResult{
				Token:     "admin-session",
				ExpiresAt: time.Date(2026, 6, 4, 10, 0, 0, 0, time.UTC),
				Admin: auth.Admin{
					ID:       "00000000-0000-0000-0000-000000000001",
					Username: "admin",
					Enabled:  true,
				},
			},
		}),
	)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(loginJSON(t, handler, "admin", "ChangeMe2026!", "ABC234")))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected login status 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	cookies := rec.Result().Cookies()
	sessionCookie := cookieByName(cookies, "mgp_admin_session")
	if sessionCookie == nil || sessionCookie.Value == "" || !sessionCookie.HttpOnly || sessionCookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("expected HttpOnly SameSite session cookie, got %+v all=%+v", sessionCookie, cookies)
	}
	csrfCookie := cookieByName(cookies, "mgp_csrf_token")
	if csrfCookie == nil || csrfCookie.Value == "" || csrfCookie.HttpOnly || csrfCookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("expected readable SameSite CSRF cookie, got %+v all=%+v", csrfCookie, cookies)
	}
	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	if token, ok := body["token"].(string); ok && token != "" {
		t.Fatalf("expected login response not to expose bearer token, got %q", token)
	}
}

func TestLoginCaptchaEndpointReturnsNoStorePNGDataURL(t *testing.T) {
	handler := httpapi.NewHandler(
		testConfig(),
		httpapi.WithLoginCaptchaAnswerForTesting("ABC234"),
		httpapi.WithAuthService(fakeAuthService{}),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/captcha", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected captcha status 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if cacheControl := rec.Header().Get("Cache-Control"); !strings.Contains(cacheControl, "no-store") {
		t.Fatalf("expected no-store captcha cache control, got %q", cacheControl)
	}
	var body struct {
		CaptchaID        string `json:"captcha_id"`
		ImageDataURL     string `json:"image_data_url"`
		ExpiresInSeconds int    `json:"expires_in_seconds"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode captcha response: %v", err)
	}
	if body.CaptchaID == "" || !strings.HasPrefix(body.ImageDataURL, "data:image/png;base64,") || body.ExpiresInSeconds <= 0 {
		t.Fatalf("unexpected captcha response: %+v", body)
	}
}

func TestLoginRejectsInvalidCaptchaBeforePasswordCheck(t *testing.T) {
	loginCalls := 0
	handler := httpapi.NewHandler(
		testConfig(),
		httpapi.WithLoginCaptchaAnswerForTesting("ABC234"),
		httpapi.WithAuthService(fakeAuthService{loginCalls: &loginCalls}),
	)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(loginJSON(t, handler, "admin", "ChangeMe2026!", "WRONG1")))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid captcha to return 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	if got := authErrorCode(t, rec); got != "MGP-AUTH-006" {
		t.Fatalf("expected invalid captcha code MGP-AUTH-006, got %q", got)
	}
	if loginCalls != 0 {
		t.Fatalf("expected invalid captcha not to call auth login, got %d calls", loginCalls)
	}
}

func TestLoginCaptchaIsSingleUse(t *testing.T) {
	loginCalls := 0
	handler := httpapi.NewHandler(
		testConfig(),
		httpapi.WithLoginCaptchaAnswerForTesting("ABC234"),
		httpapi.WithAuthService(fakeAuthService{loginCalls: &loginCalls}),
	)
	body := loginJSON(t, handler, "admin", "ChangeMe2026!", "ABC234")

	firstReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
	firstRec := httptest.NewRecorder()
	handler.ServeHTTP(firstRec, firstReq)
	if firstRec.Code != http.StatusOK {
		t.Fatalf("expected first login to return 200, got %d body=%s", firstRec.Code, firstRec.Body.String())
	}

	secondReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
	secondRec := httptest.NewRecorder()
	handler.ServeHTTP(secondRec, secondReq)
	if secondRec.Code != http.StatusBadRequest {
		t.Fatalf("expected reused captcha to return 400, got %d body=%s", secondRec.Code, secondRec.Body.String())
	}
	if got := authErrorCode(t, secondRec); got != "MGP-AUTH-006" {
		t.Fatalf("expected reused captcha code MGP-AUTH-006, got %q", got)
	}
	if loginCalls != 1 {
		t.Fatalf("expected reused captcha not to call auth login again, got %d calls", loginCalls)
	}
}

func TestLoginCaptchaStateCanBeSharedAcrossHandlers(t *testing.T) {
	shared := newSharedCaptchaStateStore()
	issuer := httpapi.NewHandler(
		testConfig(),
		httpapi.WithLoginCaptchaAnswerForTesting("ABC234"),
		httpapi.WithLoginCaptchaStateStore(shared),
		httpapi.WithAuthService(fakeAuthService{}),
	)
	consumer := httpapi.NewHandler(
		testConfig(),
		httpapi.WithLoginCaptchaStateStore(shared),
		httpapi.WithAuthService(fakeAuthService{}),
	)

	captchaReq := httptest.NewRequest(http.MethodGet, "/api/v1/auth/captcha", nil)
	captchaRec := httptest.NewRecorder()
	issuer.ServeHTTP(captchaRec, captchaReq)
	if captchaRec.Code != http.StatusOK {
		t.Fatalf("expected captcha status 200, got %d body=%s", captchaRec.Code, captchaRec.Body.String())
	}
	var captcha struct {
		CaptchaID string `json:"captcha_id"`
	}
	if err := json.NewDecoder(captchaRec.Body).Decode(&captcha); err != nil {
		t.Fatalf("decode captcha response: %v", err)
	}
	body, err := json.Marshal(map[string]string{
		"username":     "admin",
		"password":     "ChangeMe2026!",
		"captcha_id":   captcha.CaptchaID,
		"captcha_code": "ABC234",
	})
	if err != nil {
		t.Fatalf("marshal login body: %v", err)
	}

	loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(string(body)))
	loginRec := httptest.NewRecorder()
	consumer.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("expected login through second handler to return 200, got %d body=%s", loginRec.Code, loginRec.Body.String())
	}
}

func TestCookieAuthenticatedMutationRequiresCSRFHeader(t *testing.T) {
	handler := httpapi.NewHandler(testConfig(), httpapi.WithAuthService(fakeAuthService{
		authenticatedToken: "admin-session",
	}))

	missing := httptest.NewRequest(http.MethodPut, "/api/v1/auth/profile", strings.NewReader(`{"display_name":"管理员"}`))
	missing.AddCookie(&http.Cookie{Name: "mgp_admin_session", Value: "admin-session"})
	missing.AddCookie(&http.Cookie{Name: "mgp_csrf_token", Value: "csrf-token"})
	missingRec := httptest.NewRecorder()
	handler.ServeHTTP(missingRec, missing)
	if missingRec.Code != http.StatusForbidden {
		t.Fatalf("expected cookie mutation without CSRF to return 403, got %d body=%s", missingRec.Code, missingRec.Body.String())
	}

	allowed := httptest.NewRequest(http.MethodPut, "/api/v1/auth/profile", strings.NewReader(`{"display_name":"管理员"}`))
	allowed.AddCookie(&http.Cookie{Name: "mgp_admin_session", Value: "admin-session"})
	allowed.AddCookie(&http.Cookie{Name: "mgp_csrf_token", Value: "csrf-token"})
	allowed.Header.Set("X-MGP-CSRF-Token", "csrf-token")
	allowedRec := httptest.NewRecorder()
	handler.ServeHTTP(allowedRec, allowed)
	if allowedRec.Code != http.StatusOK {
		t.Fatalf("expected cookie mutation with CSRF to return 200, got %d body=%s", allowedRec.Code, allowedRec.Body.String())
	}
}

func TestAdminBearerHeaderAuthenticationIsRejected(t *testing.T) {
	handler := httpapi.NewHandler(testConfig(), httpapi.WithAuthService(fakeAuthService{
		authenticatedToken: "admin-session",
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.Header.Set("Authorization", "Bearer admin-session")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected admin bearer header auth to return 401, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAuthHandlersRecordSecurityAudit(t *testing.T) {
	auditService := &fakeAuditService{}
	handler := httpapi.NewHandler(
		testConfig(),
		httpapi.WithLoginCaptchaAnswerForTesting("ABC234"),
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

	loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(loginJSON(t, handler, "admin", "ChangeMe2026!", "ABC234")))
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
	setAdminSessionCookie(logoutReq, "admin-session")
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
		httpapi.WithLoginCaptchaAnswerForTesting("ABC234"),
		httpapi.WithAuthService(fakeAuthService{loginErr: auth.ErrInvalidCredentials}),
		httpapi.WithAuditService(auditService),
	)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(loginJSON(t, handler, "admin", "wrong", "ABC234")))
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

func TestLoginFailureDoesNotRevealUsernameByErrorCode(t *testing.T) {
	for _, username := range []string{"admin", "missing-admin"} {
		handler := httpapi.NewHandler(
			testConfig(),
			httpapi.WithLoginCaptchaAnswerForTesting("ABC234"),
			httpapi.WithAuthService(fakeAuthService{loginErr: auth.ErrInvalidCredentials}),
		)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(loginJSON(t, handler, username, "wrong", "ABC234")))
		req.RemoteAddr = "203.0.113.10:1234"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected %s login failure to return 401, got %d body=%s", username, rec.Code, rec.Body.String())
		}
		if got := authErrorCode(t, rec); got != "MGP-AUTH-002" {
			t.Fatalf("expected %s login failure code MGP-AUTH-002, got %q", username, got)
		}
	}
}

func TestLoginRateLimitDoesNotRevealUsernameByErrorCode(t *testing.T) {
	handler := httpapi.NewHandler(
		testConfig(),
		httpapi.WithLoginCaptchaAnswerForTesting("ABC234"),
		httpapi.WithAuthService(fakeAuthService{loginErr: auth.ErrInvalidCredentials}),
	)

	for _, username := range []string{"admin", "missing-admin"} {
		var rec *httptest.ResponseRecorder
		for index := 0; index < 6; index++ {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(loginJSON(t, handler, username, "wrong", "ABC234")))
			req.RemoteAddr = "203.0.113.10:1234"
			rec = httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
		}
		if rec.Code != http.StatusTooManyRequests {
			t.Fatalf("expected %s repeated failures to return 429, got %d body=%s", username, rec.Code, rec.Body.String())
		}
		if got := authErrorCode(t, rec); got != "MGP-AUTH-004" {
			t.Fatalf("expected %s rate limit code MGP-AUTH-004, got %q", username, got)
		}
	}
}

func TestLoginFailuresAreRateLimited(t *testing.T) {
	handler := httpapi.NewHandler(
		testConfig(),
		httpapi.WithLoginCaptchaAnswerForTesting("ABC234"),
		httpapi.WithAuthService(fakeAuthService{loginErr: auth.ErrInvalidCredentials}),
	)

	var rec *httptest.ResponseRecorder
	for index := 0; index < 6; index++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(loginJSON(t, handler, "admin", "wrong", "ABC234")))
		req.RemoteAddr = "203.0.113.10:1234"
		rec = httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected repeated login failures to return 429, got %d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode rate limit response: %v", err)
	}
	if body.Error.Code != "MGP-AUTH-004" {
		t.Fatalf("expected login rate limit code MGP-AUTH-004, got %q", body.Error.Code)
	}
}

func authErrorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode auth error response: %v", err)
	}
	return body.Error.Code
}

func loginJSON(t *testing.T, handler http.Handler, username string, password string, captchaCode string) string {
	t.Helper()
	captchaReq := httptest.NewRequest(http.MethodGet, "/api/v1/auth/captcha", nil)
	captchaRec := httptest.NewRecorder()
	handler.ServeHTTP(captchaRec, captchaReq)
	if captchaRec.Code != http.StatusOK {
		t.Fatalf("expected captcha status 200, got %d body=%s", captchaRec.Code, captchaRec.Body.String())
	}
	var captcha struct {
		CaptchaID string `json:"captcha_id"`
	}
	if err := json.NewDecoder(captchaRec.Body).Decode(&captcha); err != nil {
		t.Fatalf("decode captcha response: %v", err)
	}
	body, err := json.Marshal(map[string]string{
		"username":     username,
		"password":     password,
		"captcha_id":   captcha.CaptchaID,
		"captcha_code": captchaCode,
	})
	if err != nil {
		t.Fatalf("marshal login body: %v", err)
	}
	return string(body)
}

func cookieByName(cookies []*http.Cookie, name string) *http.Cookie {
	for _, cookie := range cookies {
		if cookie.Name == name {
			return cookie
		}
	}
	return nil
}

func setAdminSessionCookie(req *http.Request, token string) {
	req.AddCookie(&http.Cookie{Name: "mgp_admin_session", Value: token})
	req.AddCookie(&http.Cookie{Name: "mgp_csrf_token", Value: "test-csrf-token"})
	req.Header.Set("X-MGP-CSRF-Token", "test-csrf-token")
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
	loginCalls         *int
}

type sharedCaptchaStateStore struct {
	mu         sync.Mutex
	challenges map[string]sharedCaptchaState
}

type sharedCaptchaState struct {
	answerHash [32]byte
	expiresAt  time.Time
}

func newSharedCaptchaStateStore() *sharedCaptchaStateStore {
	return &sharedCaptchaStateStore{challenges: map[string]sharedCaptchaState{}}
}

func (s *sharedCaptchaStateStore) StoreLoginCaptcha(ctx context.Context, id string, answerHash [32]byte, expiresAt time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.challenges[id] = sharedCaptchaState{answerHash: answerHash, expiresAt: expiresAt}
	return nil
}

func (s *sharedCaptchaStateStore) ConsumeLoginCaptcha(ctx context.Context, id string, answerHash [32]byte, now time.Time) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	challenge, ok := s.challenges[id]
	if !ok {
		return false, nil
	}
	delete(s.challenges, id)
	if !now.Before(challenge.expiresAt) {
		return false, nil
	}
	return challenge.answerHash == answerHash, nil
}

func (f fakeAuthService) GetSetupStatus(context.Context) (auth.SetupStatus, error) {
	return f.status, nil
}

func (fakeAuthService) CreateFirstAdmin(context.Context, auth.CreateFirstAdminInput) (auth.Admin, error) {
	return auth.Admin{}, nil
}

func (f fakeAuthService) Login(context.Context, auth.LoginInput) (auth.LoginResult, error) {
	if f.loginCalls != nil {
		*f.loginCalls = *f.loginCalls + 1
	}
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
