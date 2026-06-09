package httpapi

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"mvp-push-gateway/backend/internal/audit"
	"mvp-push-gateway/backend/internal/auth"
)

const (
	loginFailureLimit  = 5
	loginFailureWindow = 5 * time.Minute

	adminSessionCookieName = "mgp_admin_session"
	csrfTokenCookieName    = "mgp_csrf_token"
	csrfTokenHeaderName    = "X-MGP-CSRF-Token"
)

type loginFailureLimiter struct {
	mu       sync.Mutex
	attempts map[string]loginFailureState
}

type loginFailureState struct {
	count   int
	started time.Time
}

func newLoginFailureLimiter() *loginFailureLimiter {
	return &loginFailureLimiter{attempts: map[string]loginFailureState{}}
}

type setupStatusResponse struct {
	Initialized bool `json:"initialized"`
	SetupOpen   bool `json:"setup_open"`
	AdminCount  int  `json:"admin_count"`
}

type adminResponse struct {
	ID                 string `json:"id"`
	Username           string `json:"username"`
	DisplayName        string `json:"display_name"`
	MustChangePassword bool   `json:"must_change_password"`
	Enabled            bool   `json:"enabled"`
}

type setupAdminRequest struct {
	Username        string `json:"username"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirm_password"`
	DisplayName     string `json:"display_name"`
}

type setupAdminResponse struct {
	Admin adminResponse `json:"admin"`
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginResponse struct {
	Token     string        `json:"token,omitempty"`
	TokenType string        `json:"token_type,omitempty"`
	ExpiresAt string        `json:"expires_at"`
	Admin     adminResponse `json:"admin"`
}

type meResponse struct {
	Admin adminResponse `json:"admin"`
}

type updateProfileRequest struct {
	DisplayName string `json:"display_name"`
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

type okResponse struct {
	OK bool `json:"ok"`
}

func (h *Handler) setupStatusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	if !h.requireAuthService(w) {
		return
	}

	status, err := h.auth.GetSetupStatus(r.Context())
	if err != nil {
		statusCode, code, message := authErrorStatus(err)
		writeAPIError(w, statusCode, code, message)
		return
	}
	writeJSON(w, http.StatusOK, setupStatusResponse{
		Initialized: status.Initialized,
		SetupOpen:   status.SetupOpen,
		AdminCount:  status.AdminCount,
	})
}

func (h *Handler) setupAdminHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	if !h.requireAuthService(w) {
		return
	}

	var request setupAdminRequest
	if err := decodeJSON(r, &request); err != nil {
		writeAPIError(w, http.StatusBadRequest, "MGP-REQ-001", "请求 JSON 不合法")
		return
	}

	adminUser, err := h.auth.CreateFirstAdmin(r.Context(), auth.CreateFirstAdminInput{
		Username:        request.Username,
		Password:        request.Password,
		ConfirmPassword: request.ConfirmPassword,
		DisplayName:     request.DisplayName,
	})
	if err != nil {
		statusCode, code, message := authErrorStatus(err)
		writeAPIError(w, statusCode, code, message)
		return
	}
	writeJSON(w, http.StatusCreated, setupAdminResponse{Admin: toAdminResponse(adminUser)})
}

func (h *Handler) loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	if !h.requireAuthService(w) {
		return
	}

	var request loginRequest
	if err := decodeJSON(r, &request); err != nil {
		writeAPIError(w, http.StatusBadRequest, "MGP-REQ-001", "请求 JSON 不合法")
		return
	}

	loginIP := h.clientIP(r)
	if h.loginFailures != nil && !h.loginFailures.allow(request.Username, loginIP, time.Now()) {
		h.recordLoginFailureAudit(r, request.Username, http.StatusTooManyRequests, "MGP-AUTH-004")
		writeAPIError(w, http.StatusTooManyRequests, "MGP-AUTH-004", "登录失败次数过多，请稍后重试")
		return
	}

	result, err := h.auth.Login(r.Context(), auth.LoginInput{
		Username: request.Username,
		Password: request.Password,
		Meta: auth.SessionMeta{
			UserAgent: r.UserAgent(),
			IPAddress: loginIP,
		},
	})
	if err != nil {
		statusCode, code, message := authErrorStatus(err)
		if h.loginFailures != nil && errors.Is(err, auth.ErrInvalidCredentials) {
			h.loginFailures.recordFailure(request.Username, loginIP, time.Now())
		}
		h.recordLoginFailureAudit(r, request.Username, statusCode, code)
		writeAPIError(w, statusCode, code, message)
		return
	}
	if h.loginFailures != nil {
		h.loginFailures.reset(request.Username, loginIP)
	}

	expiresAt := result.ExpiresAt.UTC().Format("2006-01-02T15:04:05Z")
	csrfToken, err := auth.NewSessionToken()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "MGP-AUTH-999", "认证服务内部错误")
		return
	}
	h.setAuthCookies(w, result.Token, csrfToken, result.ExpiresAt)
	h.recordAudit(r, result.Admin, "login", "admin_session", result.Admin.ID, map[string]string{
		"username": request.Username,
	}, map[string]any{
		"admin_id":   result.Admin.ID,
		"expires_at": expiresAt,
	})
	writeJSON(w, http.StatusOK, loginResponse{
		ExpiresAt: expiresAt,
		Admin:     toAdminResponse(result.Admin),
	})
}

func (h *Handler) logoutHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	if !h.requireAuthService(w) {
		return
	}

	requestToken, err := authTokenFromRequest(r)
	if err != nil {
		writeAPIError(w, http.StatusUnauthorized, "MGP-AUTH-003", "未登录或登录已过期")
		return
	}
	if requestToken.FromCookie && !validCSRFCookieHeader(r) {
		writeAPIError(w, http.StatusForbidden, "MGP-AUTH-005", "CSRF 校验失败")
		return
	}
	adminUser, err := h.auth.Authenticate(r.Context(), requestToken.Value)
	if err != nil {
		statusCode, code, message := authErrorStatus(err)
		writeAPIError(w, statusCode, code, message)
		return
	}
	if err := h.auth.Logout(r.Context(), requestToken.Value); err != nil {
		statusCode, code, message := authErrorStatus(err)
		writeAPIError(w, statusCode, code, message)
		return
	}
	h.clearAuthCookies(w)
	h.recordAudit(r, adminUser, "logout", "admin_session", adminUser.ID, map[string]string{
		"operation": "logout",
	}, okResponse{OK: true})
	writeJSON(w, http.StatusOK, okResponse{OK: true})
}

func (h *Handler) recordLoginFailureAudit(r *http.Request, username string, status int, code string) {
	if h.audit == nil {
		return
	}
	_, _ = h.audit.Record(r.Context(), audit.RecordInput{
		ActorUsername: strings.TrimSpace(username),
		Action:        "login_failed",
		ResourceType:  "admin_session",
		RequestSnapshot: mustMarshalAuditSnapshot(map[string]string{
			"username": strings.TrimSpace(username),
		}),
		ResponseSnapshot: mustMarshalAuditSnapshot(map[string]any{
			"status":     status,
			"error_code": code,
		}),
		IPAddress: h.clientIP(r),
		UserAgent: r.UserAgent(),
	})
}

func (l *loginFailureLimiter) allow(username string, ip string, now time.Time) bool {
	if l == nil {
		return true
	}
	key := loginFailureKey(username, ip)
	l.mu.Lock()
	defer l.mu.Unlock()
	state, ok := l.attempts[key]
	if !ok {
		return true
	}
	if now.Sub(state.started) >= loginFailureWindow {
		delete(l.attempts, key)
		return true
	}
	return state.count < loginFailureLimit
}

func (l *loginFailureLimiter) recordFailure(username string, ip string, now time.Time) {
	if l == nil {
		return
	}
	key := loginFailureKey(username, ip)
	l.mu.Lock()
	defer l.mu.Unlock()
	state, ok := l.attempts[key]
	if !ok || now.Sub(state.started) >= loginFailureWindow {
		l.attempts[key] = loginFailureState{count: 1, started: now}
		return
	}
	state.count++
	l.attempts[key] = state
}

func (l *loginFailureLimiter) reset(username string, ip string) {
	if l == nil {
		return
	}
	key := loginFailureKey(username, ip)
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.attempts, key)
}

func loginFailureKey(username string, ip string) string {
	username = strings.ToLower(strings.TrimSpace(username))
	if username == "" {
		username = "-"
	}
	ip = strings.TrimSpace(ip)
	if ip == "" {
		ip = "-"
	}
	return username + "\x00" + ip
}

func (h *Handler) meHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	adminUser, ok := h.authenticateRequest(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, meResponse{Admin: toAdminResponse(adminUser)})
}

func (h *Handler) profileHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		methodNotAllowed(w, http.MethodPut)
		return
	}
	adminUser, ok := h.authenticateRequest(w, r)
	if !ok {
		return
	}

	var request updateProfileRequest
	if err := decodeJSON(r, &request); err != nil {
		writeAPIError(w, http.StatusBadRequest, "MGP-REQ-001", "请求 JSON 不合法")
		return
	}

	updated, err := h.auth.UpdateProfile(r.Context(), auth.UpdateProfileInput{
		AdminID:     adminUser.ID,
		DisplayName: request.DisplayName,
	})
	if err != nil {
		statusCode, code, message := authErrorStatus(err)
		writeAPIError(w, statusCode, code, message)
		return
	}
	writeJSON(w, http.StatusOK, meResponse{Admin: toAdminResponse(updated)})
}

func (h *Handler) changePasswordHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	adminUser, ok := h.authenticateRequest(w, r)
	if !ok {
		return
	}

	var request changePasswordRequest
	if err := decodeJSON(r, &request); err != nil {
		writeAPIError(w, http.StatusBadRequest, "MGP-REQ-001", "请求 JSON 不合法")
		return
	}

	if err := h.auth.ChangePassword(r.Context(), auth.ChangePasswordInput{
		AdminID:         adminUser.ID,
		CurrentPassword: request.CurrentPassword,
		NewPassword:     request.NewPassword,
	}); err != nil {
		statusCode, code, message := authErrorStatus(err)
		writeAPIError(w, statusCode, code, message)
		return
	}
	h.clearAuthCookies(w)
	writeJSON(w, http.StatusOK, okResponse{OK: true})
}

func (h *Handler) authenticateRequest(w http.ResponseWriter, r *http.Request) (auth.Admin, bool) {
	if !h.requireAuthService(w) {
		return auth.Admin{}, false
	}
	requestToken, err := authTokenFromRequest(r)
	if err != nil {
		writeAPIError(w, http.StatusUnauthorized, "MGP-AUTH-003", "未登录或登录已过期")
		return auth.Admin{}, false
	}
	if requestToken.FromCookie && csrfRequired(r) && !validCSRFCookieHeader(r) {
		writeAPIError(w, http.StatusForbidden, "MGP-AUTH-005", "CSRF 校验失败")
		return auth.Admin{}, false
	}
	adminUser, err := h.auth.Authenticate(r.Context(), requestToken.Value)
	if err != nil {
		statusCode, code, message := authErrorStatus(err)
		writeAPIError(w, statusCode, code, message)
		return auth.Admin{}, false
	}
	return adminUser, true
}

type requestAuthToken struct {
	Value      string
	FromCookie bool
}

func authTokenFromRequest(r *http.Request) (requestAuthToken, error) {
	cookie, err := r.Cookie(adminSessionCookieName)
	if err != nil {
		return requestAuthToken{}, auth.ErrUnauthorized
	}
	token := strings.TrimSpace(cookie.Value)
	if token == "" {
		return requestAuthToken{}, auth.ErrUnauthorized
	}
	return requestAuthToken{Value: token, FromCookie: true}, nil
}

func decodeJSON(r *http.Request, destination any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	var extra struct{}
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("unexpected trailing json")
	}
	return nil
}

func csrfRequired(r *http.Request) bool {
	switch r.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	default:
		return true
	}
}

func validCSRFCookieHeader(r *http.Request) bool {
	cookie, err := r.Cookie(csrfTokenCookieName)
	if err != nil {
		return false
	}
	cookieValue := strings.TrimSpace(cookie.Value)
	headerValue := strings.TrimSpace(r.Header.Get(csrfTokenHeaderName))
	if cookieValue == "" || headerValue == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(cookieValue), []byte(headerValue)) == 1
}

func (h *Handler) setAuthCookies(w http.ResponseWriter, sessionToken string, csrfToken string, expiresAt time.Time) {
	secure := strings.EqualFold(h.cfg.App.Environment, "production")
	http.SetCookie(w, &http.Cookie{
		Name:     adminSessionCookieName,
		Value:    sessionToken,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     csrfTokenCookieName,
		Value:    csrfToken,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: false,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (h *Handler) clearAuthCookies(w http.ResponseWriter) {
	secure := strings.EqualFold(h.cfg.App.Environment, "production")
	expiredAt := time.Unix(0, 0).UTC()
	for _, name := range []string{adminSessionCookieName, csrfTokenCookieName} {
		http.SetCookie(w, &http.Cookie{
			Name:     name,
			Value:    "",
			Path:     "/",
			Expires:  expiredAt,
			MaxAge:   -1,
			HttpOnly: name == adminSessionCookieName,
			Secure:   secure,
			SameSite: http.SameSiteLaxMode,
		})
	}
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return ""
}

func (h *Handler) clientIP(r *http.Request) string {
	return clientIPFromRequest(r, h.cfg.Server.TrustedProxies)
}

func toAdminResponse(adminUser auth.Admin) adminResponse {
	return adminResponse{
		ID:                 adminUser.ID,
		Username:           adminUser.Username,
		DisplayName:        adminUser.DisplayName,
		MustChangePassword: adminUser.MustChangePassword,
		Enabled:            adminUser.Enabled,
	}
}
