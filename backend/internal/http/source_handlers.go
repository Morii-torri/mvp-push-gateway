package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"mvp-push-gateway/backend/internal/route"
	"mvp-push-gateway/backend/internal/settings"
	"mvp-push-gateway/backend/internal/source"
)

type sourceListResponse struct {
	Sources []sourceResponse `json:"sources"`
}

type sourceResponse struct {
	ID                           string           `json:"id"`
	Code                         string           `json:"code"`
	Name                         string           `json:"name"`
	Enabled                      bool             `json:"enabled"`
	AuthMode                     source.AuthMode  `json:"auth_mode"`
	AuthToken                    string           `json:"auth_token"`
	AuthTokenSet                 bool             `json:"auth_token_set"`
	HMACSecret                   string           `json:"hmac_secret"`
	HMACSecretSet                bool             `json:"hmac_secret_set"`
	IPAllowlist                  []string         `json:"ip_allowlist"`
	CompatMode                   string           `json:"compat_mode"`
	InboundDedupeEnabled         bool             `json:"inbound_dedupe_enabled"`
	InboundDedupeStrategy        string           `json:"inbound_dedupe_strategy"`
	InboundDedupeConfig          json.RawMessage  `json:"inbound_dedupe_config"`
	RateLimitConfig              json.RawMessage  `json:"rate_limit_config"`
	DoNotDisturbConfig           json.RawMessage  `json:"do_not_disturb_config"`
	LatestPayloadSample          *json.RawMessage `json:"latest_payload_sample,omitempty"`
	LatestPayloadSampleUpdatedAt *string          `json:"latest_payload_sample_updated_at"`
	CreatedAt                    string           `json:"created_at"`
	UpdatedAt                    string           `json:"updated_at"`
}

type sourceRequest struct {
	Code                         string          `json:"code"`
	Name                         string          `json:"name"`
	Enabled                      *bool           `json:"enabled"`
	AuthMode                     source.AuthMode `json:"auth_mode"`
	AuthToken                    string          `json:"auth_token"`
	HMACSecret                   string          `json:"hmac_secret"`
	IPAllowlist                  []string        `json:"ip_allowlist"`
	CompatMode                   string          `json:"compat_mode"`
	InboundDedupeEnabled         bool            `json:"inbound_dedupe_enabled"`
	InboundDedupeStrategy        string          `json:"inbound_dedupe_strategy"`
	InboundDedupeConfig          json.RawMessage `json:"inbound_dedupe_config"`
	RateLimitConfig              json.RawMessage `json:"rate_limit_config"`
	DoNotDisturbConfig           json.RawMessage `json:"do_not_disturb_config"`
	LatestPayloadSample          json.RawMessage `json:"latest_payload_sample"`
	LatestPayloadSampleUpdatedAt *string         `json:"latest_payload_sample_updated_at"`
}

type sourceCreateResponse struct {
	Source sourceResponse `json:"source"`
}

type sourceGetResponse = sourceCreateResponse

type ingestResponse struct {
	TraceID string `json:"trace_id"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

func (h *Handler) sourcesHandler(w http.ResponseWriter, r *http.Request) {
	if !h.requireSourceService(w) {
		return
	}
	adminUser, ok := h.authenticateRequest(w, r)
	if !ok {
		return
	}

	switch r.Method {
	case http.MethodGet:
		sources, err := h.sources.ListSources(r.Context())
		if err != nil {
			status, code, message := sourceErrorStatus(err)
			writeAPIError(w, status, code, message)
			return
		}
		responses := make([]sourceResponse, 0, len(sources))
		reveal := revealSecrets(r)
		for _, configuredSource := range sources {
			responses = append(responses, toSourceListResponse(configuredSource, reveal))
		}
		if reveal {
			h.recordSecretRevealAudit(r, adminUser, "source", "*", []string{"auth_token", "hmac_secret"})
		}
		writeJSON(w, http.StatusOK, sourceListResponse{Sources: responses})
	case http.MethodPost:
		var request sourceRequest
		if err := decodeJSON(r, &request); err != nil {
			writeAPIError(w, http.StatusBadRequest, "MGP-REQ-001", "请求 JSON 不合法")
			return
		}
		input, err := request.toCreateInput()
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, "MGP-REQ-001", "请求参数不合法")
			return
		}
		created, err := h.sources.CreateSource(r.Context(), input)
		if err != nil {
			status, code, message := sourceErrorStatus(err)
			writeAPIError(w, status, code, message)
			return
		}
		autoFlow, autoCreated, err := h.createDefaultRouteFlowForSource(r.Context(), created)
		if err != nil {
			_ = h.sources.DeleteSource(r.Context(), created.ID)
			status, code, message := routeErrorStatus(err)
			writeAPIError(w, status, code, message)
			return
		}
		response := sourceCreateResponse{Source: toSourceResponse(created, false)}
		h.recordAudit(r, adminUser, "create", "source", created.ID, input, response)
		if autoCreated {
			h.recordAudit(r, adminUser, "create", "route_flow", autoFlow.ID, map[string]string{"source_id": created.ID, "reason": "auto_create_with_source"}, routeFlowDetailResponse{Flow: toRouteFlowResponse(autoFlow)})
		}
		writeJSON(w, http.StatusCreated, response)
	default:
		methodNotAllowed(w, http.MethodGet+", "+http.MethodPost)
	}
}

func (h *Handler) sourceDetailHandler(w http.ResponseWriter, r *http.Request) {
	if !h.requireSourceService(w) {
		return
	}
	adminUser, ok := h.authenticateRequest(w, r)
	if !ok {
		return
	}

	id := strings.TrimPrefix(r.URL.Path, h.cfg.Server.APIPrefix+"/sources/")
	id = strings.TrimSpace(id)
	if id == "" || strings.Contains(id, "/") {
		writeAPIError(w, http.StatusNotFound, "MGP-SRC-001", "来源不存在")
		return
	}

	switch r.Method {
	case http.MethodGet:
		configuredSource, err := h.sources.GetSource(r.Context(), id)
		if err != nil {
			status, code, message := sourceErrorStatus(err)
			writeAPIError(w, status, code, message)
			return
		}
		if revealSecrets(r) {
			h.recordSecretRevealAudit(r, adminUser, "source", id, revealedSourceFields(configuredSource))
		}
		writeJSON(w, http.StatusOK, sourceGetResponse{Source: toSourceResponse(configuredSource, revealSecrets(r))})
	case http.MethodPut:
		var request sourceRequest
		if err := decodeJSON(r, &request); err != nil {
			writeAPIError(w, http.StatusBadRequest, "MGP-REQ-001", "请求 JSON 不合法")
			return
		}
		input, err := request.toUpdateInput()
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, "MGP-REQ-001", "请求参数不合法")
			return
		}
		updated, err := h.sources.UpdateSource(r.Context(), id, input)
		if err != nil {
			status, code, message := sourceErrorStatus(err)
			writeAPIError(w, status, code, message)
			return
		}
		response := sourceGetResponse{Source: toSourceResponse(updated, false)}
		h.recordAudit(r, adminUser, "update", "source", id, input, response)
		writeJSON(w, http.StatusOK, response)
	case http.MethodDelete:
		if err := h.sources.DeleteSource(r.Context(), id); err != nil {
			status, code, message := sourceErrorStatus(err)
			writeAPIError(w, status, code, message)
			return
		}
		response := okResponse{OK: true}
		h.recordAudit(r, adminUser, "delete", "source", id, nil, response)
		writeJSON(w, http.StatusOK, response)
	default:
		methodNotAllowed(w, http.MethodGet+", "+http.MethodPut+", "+http.MethodDelete)
	}
}

func (h *Handler) createDefaultRouteFlowForSource(ctx context.Context, created source.Source) (route.Flow, bool, error) {
	if h.routes == nil {
		return route.Flow{}, false, nil
	}
	flow, err := h.routes.CreateFlow(ctx, route.CreateFlowInput{
		SourceID: created.ID,
		Name:     strings.TrimSpace(created.Name) + " 路由组",
		Enabled:  true,
		Mode:     route.ModeTable,
	})
	if err != nil {
		return route.Flow{}, false, err
	}
	return flow, true, nil
}

func (h *Handler) ingestHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	if !h.requireSourceService(w) {
		return
	}

	sourceCode := strings.TrimPrefix(r.URL.Path, h.cfg.Server.APIPrefix+"/ingest/")
	sourceCode = strings.TrimSpace(sourceCode)
	if sourceCode == "" || strings.Contains(sourceCode, "/") {
		writeAPIError(w, http.StatusNotFound, "MGP-SRC-001", "来源不存在")
		return
	}

	maxPayloadBytes := h.ingestMaxPayloadBytes(r.Context())
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxPayloadBytes+1))
	if err != nil || int64(len(body)) > maxPayloadBytes {
		h.recordSourceRejectAudit(r, sourceCode, source.ErrPayloadTooLarge, http.StatusRequestEntityTooLarge, "MGP-PAYLOAD-002")
		writeAPIError(w, http.StatusRequestEntityTooLarge, "MGP-PAYLOAD-002", "Payload 超过大小限制")
		return
	}
	result, err := h.sources.Ingest(r.Context(), source.IngestInput{
		SourceCode:        sourceCode,
		Method:            r.Method,
		Path:              r.URL.Path,
		Headers:           r.Header.Clone(),
		RemoteAddr:        h.clientIPForIngest(r),
		Body:              body,
		PersistBeforePlan: isConsoleIngestTest(r),
	})
	if err != nil {
		status, code, message := sourceErrorStatus(err)
		h.recordSourceRejectAudit(r, sourceCode, err, status, code)
		writeAPIError(w, status, code, message)
		return
	}
	writeJSON(w, http.StatusAccepted, ingestResponse{
		TraceID: result.TraceID,
		Status:  result.Status,
		Message: result.Message,
	})
}

func isConsoleIngestTest(r *http.Request) bool {
	return strings.EqualFold(strings.TrimSpace(r.Header.Get("X-MGP-Console-Ingest-Test")), "true")
}

func (h *Handler) ingestMaxPayloadBytes(ctx context.Context) int64 {
	if h.settings == nil {
		return source.DefaultMaxPayloadBytes
	}
	value := h.settings.IntSetting(ctx, settings.KeyIngestMaxPayloadBytes, int(settings.DefaultIngestMaxPayloadBytes))
	if value <= 0 {
		return source.DefaultMaxPayloadBytes
	}
	return int64(value)
}

func (r sourceRequest) toCreateInput() (source.CreateSourceInput, error) {
	enabled := true
	if r.Enabled != nil {
		enabled = *r.Enabled
	}
	return source.CreateSourceInput{
		Code:                  r.Code,
		Name:                  r.Name,
		Enabled:               enabled,
		AuthMode:              r.AuthMode,
		AuthToken:             r.AuthToken,
		HMACSecret:            r.HMACSecret,
		IPAllowlist:           r.IPAllowlist,
		CompatMode:            r.CompatMode,
		InboundDedupeEnabled:  r.InboundDedupeEnabled,
		InboundDedupeStrategy: source.DedupeStrategy(r.InboundDedupeStrategy),
		InboundDedupeConfig:   r.InboundDedupeConfig,
		RateLimitConfig:       r.RateLimitConfig,
		QuietHoursConfig:      r.DoNotDisturbConfig,
	}, nil
}

func (r sourceRequest) toUpdateInput() (source.UpdateSourceInput, error) {
	return r.toCreateInput()
}

func revealSecrets(r *http.Request) bool {
	return strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("reveal_secrets")), "true")
}

func revealedSourceFields(configuredSource source.Source) []string {
	fields := []string{}
	if strings.TrimSpace(configuredSource.AuthToken) != "" {
		fields = append(fields, "auth_token")
	}
	if strings.TrimSpace(configuredSource.HMACSecret) != "" {
		fields = append(fields, "hmac_secret")
	}
	return fields
}

func toSourceResponse(configuredSource source.Source, reveal bool) sourceResponse {
	return toSourceResponseWithPayload(configuredSource, true, reveal)
}

func toSourceListResponse(configuredSource source.Source, reveal bool) sourceResponse {
	return toSourceResponseWithPayload(configuredSource, false, reveal)
}

func toSourceResponseWithPayload(configuredSource source.Source, includePayload bool, reveal bool) sourceResponse {
	latestUpdatedAt := formatOptionalTime(configuredSource.LatestPayloadSampleUpdatedAt)
	var latestPayloadSample *json.RawMessage
	if includePayload {
		payload := nullableRawJSON(configuredSource.LatestPayloadSample)
		latestPayloadSample = &payload
	}
	authToken := ""
	hmacSecret := ""
	if reveal {
		authToken = configuredSource.AuthToken
		hmacSecret = configuredSource.HMACSecret
	}
	return sourceResponse{
		ID:                           configuredSource.ID,
		Code:                         configuredSource.Code,
		Name:                         configuredSource.Name,
		Enabled:                      configuredSource.Enabled,
		AuthMode:                     configuredSource.AuthMode,
		AuthToken:                    authToken,
		AuthTokenSet:                 strings.TrimSpace(configuredSource.AuthToken) != "",
		HMACSecret:                   hmacSecret,
		HMACSecretSet:                strings.TrimSpace(configuredSource.HMACSecret) != "",
		IPAllowlist:                  configuredSource.IPAllowlist,
		CompatMode:                   configuredSource.CompatMode,
		InboundDedupeEnabled:         configuredSource.InboundDedupeEnabled,
		InboundDedupeStrategy:        string(configuredSource.InboundDedupeStrategy),
		InboundDedupeConfig:          defaultRawJSON(configuredSource.InboundDedupeConfig),
		RateLimitConfig:              defaultRawJSON(configuredSource.RateLimitConfig),
		DoNotDisturbConfig:           defaultRawJSON(configuredSource.QuietHoursConfig),
		LatestPayloadSample:          latestPayloadSample,
		LatestPayloadSampleUpdatedAt: latestUpdatedAt,
		CreatedAt:                    formatTime(configuredSource.CreatedAt),
		UpdatedAt:                    formatTime(configuredSource.UpdatedAt),
	}
}

func defaultRawJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage(`{}`)
	}
	return raw
}

func nullableRawJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage(`null`)
	}
	return raw
}

func formatOptionalTime(value *time.Time) *string {
	if value == nil || value.IsZero() {
		return nil
	}
	formatted := value.UTC().Format(time.RFC3339)
	return &formatted
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

func (h *Handler) clientIPForIngest(r *http.Request) string {
	return clientIPFromRequest(r, h.cfg.Server.TrustedProxies)
}

func clientIPForIngest(r *http.Request) string {
	return clientIPFromRequest(r, nil)
}

func clientIPFromRequest(r *http.Request, trustedProxies []string) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = strings.TrimSpace(r.RemoteAddr)
	}
	if host == "" {
		return ""
	}
	remoteIP, err := netip.ParseAddr(host)
	if err != nil || !trustedProxyContains(trustedProxies, remoteIP) {
		return host
	}
	if forwarded := clientIPFromForwardedFor(r.Header.Get("X-Forwarded-For"), trustedProxies); forwarded != "" {
		return forwarded
	}
	realIP := strings.TrimSpace(r.Header.Get("X-Real-IP"))
	if parsed, err := netip.ParseAddr(realIP); err == nil {
		return parsed.String()
	}
	return host
}

func clientIPFromForwardedFor(value string, trustedProxies []string) string {
	parts := strings.Split(value, ",")
	chain := make([]netip.Addr, 0, len(parts))
	for _, part := range parts {
		ip, err := netip.ParseAddr(strings.TrimSpace(part))
		if err != nil {
			continue
		}
		chain = append(chain, ip.Unmap())
	}
	if len(chain) == 0 {
		return ""
	}
	for index := len(chain) - 1; index >= 0; index-- {
		if !trustedProxyContains(trustedProxies, chain[index]) {
			return chain[index].String()
		}
	}
	return chain[0].String()
}

func trustedProxyContains(entries []string, ip netip.Addr) bool {
	ip = ip.Unmap()
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if prefix, err := netip.ParsePrefix(entry); err == nil {
			if prefix.Contains(ip) {
				return true
			}
			continue
		}
		if exact, err := netip.ParseAddr(entry); err == nil && exact.Unmap() == ip {
			return true
		}
	}
	return false
}

func sourceErrorStatus(err error) (int, string, string) {
	switch {
	case errors.Is(err, source.ErrInvalidInput):
		return http.StatusBadRequest, "MGP-REQ-001", "请求参数不合法"
	case errors.Is(err, source.ErrInvalidJSON):
		return http.StatusBadRequest, "MGP-PAYLOAD-001", "请求 JSON 不合法"
	case errors.Is(err, source.ErrInvalidDedupeConfig):
		return http.StatusBadRequest, "MGP-DEDUPE-001", "入站去重配置不合法"
	case errors.Is(err, source.ErrDedupeStoreFailed):
		return http.StatusServiceUnavailable, "MGP-DEDUPE-002", "入站去重服务暂不可用"
	case errors.Is(err, source.ErrHMACNonceStoreFailed):
		return http.StatusServiceUnavailable, "MGP-HMAC-002", "HMAC 重放防护服务暂不可用"
	case errors.Is(err, source.ErrAlreadyExists):
		return http.StatusConflict, "MGP-SRC-001", "来源编码已存在"
	case errors.Is(err, source.ErrNotFound):
		return http.StatusNotFound, "MGP-SRC-001", "来源不存在"
	case errors.Is(err, source.ErrDisabled):
		return http.StatusForbidden, "MGP-SRC-001", "来源已停用"
	case errors.Is(err, source.ErrUnauthorized):
		return http.StatusUnauthorized, "MGP-AUTH-001", "来源鉴权失败"
	case errors.Is(err, source.ErrIPNotAllowed):
		return http.StatusForbidden, "MGP-SRC-002", "来源 IP 不在白名单内"
	case errors.Is(err, source.ErrPayloadTooLarge):
		return http.StatusRequestEntityTooLarge, "MGP-PAYLOAD-002", "Payload 超过大小限制"
	case errors.Is(err, source.ErrRateLimited):
		return http.StatusTooManyRequests, "MGP-RATE-001", "来源入站已限流"
	case errors.Is(err, source.ErrDuplicateInbound):
		return http.StatusConflict, "MGP-DEDUPE-001", "重复入站消息"
	default:
		return http.StatusInternalServerError, "MGP-REQ-001", "来源服务内部错误"
	}
}
