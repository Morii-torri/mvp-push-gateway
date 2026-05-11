package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"mvp-push-gateway/backend/internal/audit"
	"mvp-push-gateway/backend/internal/auth"
	"mvp-push-gateway/backend/internal/config"
	"mvp-push-gateway/backend/internal/matchgroup"
	"mvp-push-gateway/backend/internal/messagelog"
	"mvp-push-gateway/backend/internal/monitoring"
	"mvp-push-gateway/backend/internal/provider"
	"mvp-push-gateway/backend/internal/recipient"
	"mvp-push-gateway/backend/internal/route"
	"mvp-push-gateway/backend/internal/settings"
	"mvp-push-gateway/backend/internal/source"
	"mvp-push-gateway/backend/internal/statistics"
	msgtemplate "mvp-push-gateway/backend/internal/template"
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
	cfg         config.Config
	auth        authService
	sources     sourceService
	providers   providerService
	recipients  recipientService
	routes      routeService
	templates   templateService
	monitoring  monitoringService
	stats       statisticsService
	matchGroups matchGroupService
	messageLogs messageLogService
	audit       auditService
	settings    settingsService
}

type Option func(*Handler)

type sourceService interface {
	ListSources(context.Context) ([]source.Source, error)
	CreateSource(context.Context, source.CreateSourceInput) (source.Source, error)
	GetSource(context.Context, string) (source.Source, error)
	UpdateSource(context.Context, string, source.UpdateSourceInput) (source.Source, error)
	DeleteSource(context.Context, string) error
	Ingest(context.Context, source.IngestInput) (source.IngestResult, error)
}

type providerService interface {
	SeedProviderCapabilities(context.Context) error
	ListProviderCapabilities(context.Context) ([]provider.Capability, error)
	ListChannels(context.Context) ([]provider.Channel, error)
	CreateChannel(context.Context, provider.CreateChannelInput) (provider.Channel, error)
	GetChannel(context.Context, string) (provider.Channel, error)
	UpdateChannel(context.Context, string, provider.UpdateChannelInput) (provider.Channel, error)
	DeleteChannel(context.Context, string) error
	BuildRequest(context.Context, string, provider.BuildRequestInput) (provider.BuiltRequest, error)
	TestSend(context.Context, string, provider.TestSendInput) (provider.TestSendResult, error)
}

type recipientService interface {
	ListOrgUnits(context.Context) ([]recipient.OrgUnit, error)
	CreateOrgUnit(context.Context, recipient.OrgUnitInput) (recipient.OrgUnit, error)
	GetOrgUnit(context.Context, string) (recipient.OrgUnit, error)
	UpdateOrgUnit(context.Context, string, recipient.OrgUnitInput) (recipient.OrgUnit, error)
	DeleteOrgUnit(context.Context, string) error
	ListUsers(context.Context) ([]recipient.User, error)
	CreateUser(context.Context, recipient.UserInput) (recipient.User, error)
	GetUser(context.Context, string) (recipient.User, error)
	UpdateUser(context.Context, string, recipient.UserInput) (recipient.User, error)
	DeleteUser(context.Context, string) error
	ListUserIdentities(context.Context, string) ([]recipient.UserIdentity, error)
	CreateUserIdentity(context.Context, recipient.UserIdentityInput) (recipient.UserIdentity, error)
	UpdateUserIdentity(context.Context, string, recipient.UserIdentityInput) (recipient.UserIdentity, error)
	DeleteUserIdentity(context.Context, string) error
	FindUserIdentity(context.Context, string, string, string) (recipient.UserIdentity, error)
	ListRecipientGroups(context.Context) ([]recipient.RecipientGroup, error)
	CreateRecipientGroup(context.Context, recipient.RecipientGroupInput) (recipient.RecipientGroup, error)
	GetRecipientGroup(context.Context, string) (recipient.RecipientGroup, error)
	UpdateRecipientGroup(context.Context, string, recipient.RecipientGroupInput) (recipient.RecipientGroup, error)
	DeleteRecipientGroup(context.Context, string) error
}

type templateService interface {
	ListTemplates(context.Context) ([]msgtemplate.Template, error)
	CreateTemplate(context.Context, msgtemplate.TemplateInput) (msgtemplate.Template, error)
	GetTemplate(context.Context, string) (msgtemplate.Template, error)
	UpdateTemplate(context.Context, string, msgtemplate.TemplateInput) (msgtemplate.Template, error)
	DeleteTemplate(context.Context, string) error
	Parse(msgtemplate.VersionInput) (msgtemplate.ValidationResult, error)
	Preview(msgtemplate.VersionInput) (msgtemplate.ValidationResult, error)
	Validate(msgtemplate.VersionInput) msgtemplate.ValidationResult
	Publish(context.Context, string, msgtemplate.VersionInput) (msgtemplate.TemplateVersion, error)
}

type routeService interface {
	ListFlows(context.Context) ([]route.Flow, error)
	CreateFlow(context.Context, route.CreateFlowInput) (route.Flow, error)
	GetFlow(context.Context, string) (route.Flow, error)
	UpdateFlow(context.Context, string, route.UpdateFlowInput) (route.Flow, error)
	DeleteFlow(context.Context, string) error
	ListVersions(context.Context, string) ([]route.Version, error)
	ActivateVersion(context.Context, string, string) (route.Flow, error)
	GetCanvas(context.Context, string) (route.CanvasState, error)
	SaveCanvas(context.Context, string, route.SaveCanvasInput) (route.CanvasState, error)
	GetRules(context.Context, string) (route.RuleSet, error)
	SaveRules(context.Context, string, route.SaveRulesInput) (route.RuleSet, error)
	ReorderRules(context.Context, string, route.ReorderRulesInput) (route.RuleSet, error)
	Validate(context.Context, string) (route.ValidationResult, error)
	Publish(context.Context, string) (route.Version, error)
	Simulate(context.Context, string, route.SimulateInput) (route.SimulationResult, error)
}

type monitoringService interface {
	GetQueueMonitoringSnapshot(context.Context) (monitoring.QueueSnapshot, error)
	RunRetentionCleanup(context.Context, monitoring.RetentionCleanupParams) (monitoring.CleanupStatus, error)
}

type statisticsService interface {
	GetOverview(context.Context) (statistics.Overview, error)
}

type matchGroupService interface {
	ListGroups(context.Context) ([]matchgroup.Group, error)
	CreateGroup(context.Context, matchgroup.GroupInput) (matchgroup.Group, error)
	GetGroup(context.Context, string) (matchgroup.Group, error)
	UpdateGroup(context.Context, string, matchgroup.GroupInput) (matchgroup.Group, error)
	DeleteGroup(context.Context, string) error
	ListItems(context.Context, string) ([]matchgroup.Item, error)
	CreateItem(context.Context, string, matchgroup.ItemInput) (matchgroup.Item, error)
	GetItem(context.Context, string, string) (matchgroup.Item, error)
	UpdateItem(context.Context, string, string, matchgroup.ItemInput) (matchgroup.Item, error)
	DeleteItem(context.Context, string, string) error
}

type messageLogService interface {
	ListMessages(context.Context, messagelog.ListFilter) (messagelog.ListResult, error)
	GetMessage(context.Context, string) (messagelog.MessageDetail, error)
}

type auditService interface {
	ListLogs(context.Context, audit.ListFilter) (audit.ListResult, error)
	GetLog(context.Context, string) (audit.Log, error)
	Record(context.Context, audit.RecordInput) (audit.Log, error)
}

type settingsService interface {
	ListSettings(context.Context) ([]settings.Setting, error)
	UpdateSetting(context.Context, string, settings.UpdateInput) (settings.Setting, error)
}

func WithAuthService(service authService) Option {
	return func(h *Handler) {
		h.auth = service
	}
}

func WithSourceService(service sourceService) Option {
	return func(h *Handler) {
		h.sources = service
	}
}

func WithProviderService(service providerService) Option {
	return func(h *Handler) {
		h.providers = service
	}
}

func WithRecipientService(service recipientService) Option {
	return func(h *Handler) {
		h.recipients = service
	}
}

func WithTemplateService(service templateService) Option {
	return func(h *Handler) {
		h.templates = service
	}
}

func WithRouteService(service routeService) Option {
	return func(h *Handler) {
		h.routes = service
	}
}

func WithMonitoringService(service monitoringService) Option {
	return func(h *Handler) {
		h.monitoring = service
	}
}

func WithStatisticsService(service statisticsService) Option {
	return func(h *Handler) {
		h.stats = service
	}
}

func WithMatchGroupService(service matchGroupService) Option {
	return func(h *Handler) {
		h.matchGroups = service
	}
}

func WithMessageLogService(service messageLogService) Option {
	return func(h *Handler) {
		h.messageLogs = service
	}
}

func WithAuditService(service auditService) Option {
	return func(h *Handler) {
		h.audit = service
	}
}

func WithSettingsService(service settingsService) Option {
	return func(h *Handler) {
		h.settings = service
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
	mux.HandleFunc(cfg.Server.APIPrefix+"/sources", handler.sourcesHandler)
	mux.HandleFunc(cfg.Server.APIPrefix+"/sources/", handler.sourceDetailHandler)
	mux.HandleFunc(cfg.Server.APIPrefix+"/ingest/", handler.ingestHandler)
	mux.HandleFunc(cfg.Server.APIPrefix+"/provider-capabilities", handler.providerCapabilitiesHandler)
	mux.HandleFunc(cfg.Server.APIPrefix+"/channels", handler.channelsHandler)
	mux.HandleFunc(cfg.Server.APIPrefix+"/channels/", handler.channelDetailHandler)
	mux.HandleFunc(cfg.Server.APIPrefix+"/org-units", handler.orgUnitsHandler)
	mux.HandleFunc(cfg.Server.APIPrefix+"/org-units/", handler.orgUnitDetailHandler)
	mux.HandleFunc(cfg.Server.APIPrefix+"/users", handler.usersHandler)
	mux.HandleFunc(cfg.Server.APIPrefix+"/users/", handler.userDetailHandler)
	mux.HandleFunc(cfg.Server.APIPrefix+"/user-identities/lookup", handler.userIdentityLookupHandler)
	mux.HandleFunc(cfg.Server.APIPrefix+"/user-identities/", handler.userIdentityDetailHandler)
	mux.HandleFunc(cfg.Server.APIPrefix+"/recipient-groups", handler.recipientGroupsHandler)
	mux.HandleFunc(cfg.Server.APIPrefix+"/recipient-groups/", handler.recipientGroupDetailHandler)
	mux.HandleFunc(cfg.Server.APIPrefix+"/match-groups", handler.matchGroupsHandler)
	mux.HandleFunc(cfg.Server.APIPrefix+"/match-groups/", handler.matchGroupDetailHandler)
	mux.HandleFunc(cfg.Server.APIPrefix+"/route-flows", handler.routeFlowsHandler)
	mux.HandleFunc(cfg.Server.APIPrefix+"/route-flows/", handler.routeFlowDetailHandler)
	mux.HandleFunc(cfg.Server.APIPrefix+"/messages", handler.messagesHandler)
	mux.HandleFunc(cfg.Server.APIPrefix+"/messages/", handler.messageDetailHandler)
	mux.HandleFunc(cfg.Server.APIPrefix+"/audit-logs", handler.auditLogsHandler)
	mux.HandleFunc(cfg.Server.APIPrefix+"/audit-logs/", handler.auditLogDetailHandler)
	mux.HandleFunc(cfg.Server.APIPrefix+"/settings", handler.settingsHandler)
	mux.HandleFunc(cfg.Server.APIPrefix+"/settings/", handler.settingDetailHandler)
	mux.HandleFunc(cfg.Server.APIPrefix+"/monitoring/queue", handler.queueMonitoringHandler)
	mux.HandleFunc(cfg.Server.APIPrefix+"/monitor/queues", handler.queueMonitoringHandler)
	mux.HandleFunc(cfg.Server.APIPrefix+"/statistics/overview", handler.statisticsOverviewHandler)
	mux.HandleFunc(cfg.Server.APIPrefix+"/stats/overview", handler.statisticsOverviewHandler)
	mux.HandleFunc(cfg.Server.APIPrefix+"/maintenance/retention/cleanup", handler.retentionCleanupHandler)
	mux.HandleFunc(cfg.Server.APIPrefix+"/templates/parse", handler.templateParseHandler)
	mux.HandleFunc(cfg.Server.APIPrefix+"/templates/preview", handler.templatePreviewHandler)
	mux.HandleFunc(cfg.Server.APIPrefix+"/templates/validate", handler.templateValidateHandler)
	mux.HandleFunc(cfg.Server.APIPrefix+"/templates", handler.templatesHandler)
	mux.HandleFunc(cfg.Server.APIPrefix+"/templates/", handler.templateDetailHandler)
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

func (h *Handler) requireSourceService(w http.ResponseWriter) bool {
	if h.sources != nil {
		return true
	}
	writeAPIError(w, http.StatusServiceUnavailable, "MGP-SRC-001", "来源服务未启用，请先配置数据库")
	return false
}

func (h *Handler) requireProviderService(w http.ResponseWriter) bool {
	if h.providers != nil {
		return true
	}
	writeAPIError(w, http.StatusServiceUnavailable, "MGP-CHN-001", "平台服务未启用，请先配置数据库")
	return false
}

func (h *Handler) requireMonitoringService(w http.ResponseWriter) bool {
	if h.monitoring != nil {
		return true
	}
	writeAPIError(w, http.StatusServiceUnavailable, "MGP-MON-001", "监控服务未启用，请先配置数据库")
	return false
}

func (h *Handler) requireStatisticsService(w http.ResponseWriter) bool {
	if h.stats != nil {
		return true
	}
	writeAPIError(w, http.StatusServiceUnavailable, "MGP-STAT-001", "统计服务未启用，请先配置数据库")
	return false
}

func (h *Handler) requireRecipientService(w http.ResponseWriter) bool {
	if h.recipients != nil {
		return true
	}
	writeAPIError(w, http.StatusServiceUnavailable, "MGP-RCP-001", "接收人服务未启用，请先配置数据库")
	return false
}

func (h *Handler) requireTemplateService(w http.ResponseWriter) bool {
	if h.templates != nil {
		return true
	}
	writeAPIError(w, http.StatusServiceUnavailable, "MGP-TPL-001", "模板服务未启用，请先配置数据库")
	return false
}

func (h *Handler) requireRouteService(w http.ResponseWriter) bool {
	if h.routes != nil {
		return true
	}
	writeAPIError(w, http.StatusServiceUnavailable, "MGP-ROUTE-002", "路由服务未启用，请先配置数据库")
	return false
}

func (h *Handler) requireMatchGroupService(w http.ResponseWriter) bool {
	if h.matchGroups != nil {
		return true
	}
	writeAPIError(w, http.StatusServiceUnavailable, "MGP-MATCH-001", "匹配组服务未启用，请先配置数据库")
	return false
}

func (h *Handler) requireMessageLogService(w http.ResponseWriter) bool {
	if h.messageLogs != nil {
		return true
	}
	writeAPIError(w, http.StatusServiceUnavailable, "MGP-MSG-001", "消息日志服务未启用，请先配置数据库")
	return false
}

func (h *Handler) requireAuditService(w http.ResponseWriter) bool {
	if h.audit != nil {
		return true
	}
	writeAPIError(w, http.StatusServiceUnavailable, "MGP-AUDIT-001", "审计服务未启用，请先配置数据库")
	return false
}

func (h *Handler) requireSettingsService(w http.ResponseWriter) bool {
	if h.settings != nil {
		return true
	}
	writeAPIError(w, http.StatusServiceUnavailable, "MGP-SETTINGS-001", "系统设置服务未启用，请先配置数据库")
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
