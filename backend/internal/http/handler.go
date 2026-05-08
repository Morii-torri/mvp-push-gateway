package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"mvp-push-gateway/backend/internal/auth"
	"mvp-push-gateway/backend/internal/config"
)

type authService interface {
	GetSetupStatus(context.Context) (auth.SetupStatus, error)
	CreateFirstAdmin(context.Context, auth.CreateFirstAdminInput) (auth.Admin, error)
	Login(context.Context, auth.LoginInput) (auth.LoginResult, error)
	Authenticate(context.Context, string) (auth.Admin, error)
	Logout(context.Context, string) error
	ChangePassword(context.Context, auth.ChangePasswordInput) error
}

type Handler struct {
	cfg  config.Config
	auth authService
}

type Option func(*Handler)

func WithAuthService(service authService) Option {
	return func(h *Handler) {
		h.auth = service
	}
}

type healthResponse struct {
	Status      string `json:"status"`
	AppName     string `json:"app_name"`
	Environment string `json:"environment"`
	APIPrefix   string `json:"api_prefix"`
}

func NewHandler(cfg config.Config, options ...Option) http.Handler {
	handler := &Handler{cfg: cfg}
	for _, option := range options {
		option(handler)
	}

	mux := http.NewServeMux()
	mux.HandleFunc(cfg.Server.APIPrefix+"/health", handler.healthHandler)
	mux.HandleFunc(cfg.Server.APIPrefix+"/setup/status", handler.setupStatusHandler)
	mux.HandleFunc(cfg.Server.APIPrefix+"/setup/admin", handler.setupAdminHandler)
	mux.HandleFunc(cfg.Server.APIPrefix+"/auth/login", handler.loginHandler)
	mux.HandleFunc(cfg.Server.APIPrefix+"/auth/logout", handler.logoutHandler)
	mux.HandleFunc(cfg.Server.APIPrefix+"/auth/me", handler.meHandler)
	mux.HandleFunc(cfg.Server.APIPrefix+"/auth/change-password", handler.changePasswordHandler)
	return mux
}

func (h *Handler) healthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}

	writeJSON(w, http.StatusOK, healthResponse{
		Status:      "ok",
		AppName:     h.cfg.App.Name,
		Environment: h.cfg.App.Environment,
		APIPrefix:   h.cfg.Server.APIPrefix,
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type errorResponse struct {
	Error apiError `json:"error"`
}

func writeAPIError(w http.ResponseWriter, status int, code string, message string) {
	writeJSON(w, status, errorResponse{
		Error: apiError{
			Code:    code,
			Message: message,
		},
	})
}

func methodNotAllowed(w http.ResponseWriter, allowed string) {
	w.Header().Set("Allow", allowed)
	http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
}

func (h *Handler) requireAuthService(w http.ResponseWriter) bool {
	if h.auth != nil {
		return true
	}
	writeAPIError(w, http.StatusServiceUnavailable, "MGP-SETUP-000", "认证服务未启用，请先配置数据库")
	return false
}

func authErrorStatus(err error) (int, string, string) {
	switch {
	case errors.Is(err, auth.ErrInvalidInput):
		return http.StatusBadRequest, "MGP-REQ-001", "请求参数不合法"
	case errors.Is(err, auth.ErrSetupClosed):
		return http.StatusConflict, "MGP-SETUP-001", "初始化入口已关闭"
	case errors.Is(err, auth.ErrInvalidCredentials):
		return http.StatusUnauthorized, "MGP-AUTH-002", "用户名或密码错误"
	case errors.Is(err, auth.ErrUnauthorized):
		return http.StatusUnauthorized, "MGP-AUTH-003", "未登录或登录已过期"
	default:
		return http.StatusInternalServerError, "MGP-AUTH-999", "认证服务内部错误"
	}
}
