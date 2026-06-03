package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"

	"mvp-push-gateway/backend/internal/auth"
)

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
	Username    string `json:"username"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
}

type setupAdminResponse struct {
	Admin adminResponse `json:"admin"`
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginResponse struct {
	Token     string        `json:"token"`
	TokenType string        `json:"token_type"`
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
		Username:    request.Username,
		Password:    request.Password,
		DisplayName: request.DisplayName,
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

	result, err := h.auth.Login(r.Context(), auth.LoginInput{
		Username: request.Username,
		Password: request.Password,
		Meta: auth.SessionMeta{
			UserAgent: r.UserAgent(),
			IPAddress: clientIP(r),
		},
	})
	if err != nil {
		statusCode, code, message := authErrorStatus(err)
		writeAPIError(w, statusCode, code, message)
		return
	}

	writeJSON(w, http.StatusOK, loginResponse{
		Token:     result.Token,
		TokenType: "Bearer",
		ExpiresAt: result.ExpiresAt.UTC().Format("2006-01-02T15:04:05Z"),
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

	token, err := bearerToken(r)
	if err != nil {
		writeAPIError(w, http.StatusUnauthorized, "MGP-AUTH-003", "未登录或登录已过期")
		return
	}
	if _, err := h.auth.Authenticate(r.Context(), token); err != nil {
		statusCode, code, message := authErrorStatus(err)
		writeAPIError(w, statusCode, code, message)
		return
	}
	if err := h.auth.Logout(r.Context(), token); err != nil {
		statusCode, code, message := authErrorStatus(err)
		writeAPIError(w, statusCode, code, message)
		return
	}
	writeJSON(w, http.StatusOK, okResponse{OK: true})
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
	writeJSON(w, http.StatusOK, okResponse{OK: true})
}

func (h *Handler) authenticateRequest(w http.ResponseWriter, r *http.Request) (auth.Admin, bool) {
	if !h.requireAuthService(w) {
		return auth.Admin{}, false
	}
	token, err := bearerToken(r)
	if err != nil {
		writeAPIError(w, http.StatusUnauthorized, "MGP-AUTH-003", "未登录或登录已过期")
		return auth.Admin{}, false
	}
	adminUser, err := h.auth.Authenticate(r.Context(), token)
	if err != nil {
		statusCode, code, message := authErrorStatus(err)
		writeAPIError(w, statusCode, code, message)
		return auth.Admin{}, false
	}
	return adminUser, true
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

func bearerToken(r *http.Request) (string, error) {
	value := strings.TrimSpace(r.Header.Get("Authorization"))
	if value == "" {
		return "", auth.ErrUnauthorized
	}
	prefix := "Bearer "
	if !strings.HasPrefix(value, prefix) {
		return "", auth.ErrUnauthorized
	}
	token := strings.TrimSpace(strings.TrimPrefix(value, prefix))
	if token == "" {
		return "", auth.ErrUnauthorized
	}
	return token, nil
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
