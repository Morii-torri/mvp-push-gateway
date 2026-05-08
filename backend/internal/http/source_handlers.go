package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"mvp-push-gateway/backend/internal/source"
)

type sourceListResponse struct {
	Sources []sourceResponse `json:"sources"`
}

type sourceResponse struct {
	ID                           string          `json:"id"`
	Code                         string          `json:"code"`
	Name                         string          `json:"name"`
	Enabled                      bool            `json:"enabled"`
	AuthMode                     source.AuthMode `json:"auth_mode"`
	AuthToken                    string          `json:"auth_token"`
	HMACSecret                   string          `json:"hmac_secret"`
	IPAllowlist                  []string        `json:"ip_allowlist"`
	CompatMode                   string          `json:"compat_mode"`
	InboundDedupeEnabled         bool            `json:"inbound_dedupe_enabled"`
	InboundDedupeStrategy        string          `json:"inbound_dedupe_strategy"`
	InboundDedupeConfig          json.RawMessage `json:"inbound_dedupe_config"`
	RateLimitConfig              json.RawMessage `json:"rate_limit_config"`
	LatestPayloadSample          json.RawMessage `json:"latest_payload_sample"`
	LatestPayloadSampleUpdatedAt *string         `json:"latest_payload_sample_updated_at"`
	CreatedAt                    string          `json:"created_at"`
	UpdatedAt                    string          `json:"updated_at"`
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
	if _, ok := h.authenticateRequest(w, r); !ok {
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
		for _, configuredSource := range sources {
			responses = append(responses, toSourceResponse(configuredSource))
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
		writeJSON(w, http.StatusCreated, sourceCreateResponse{Source: toSourceResponse(created)})
	default:
		methodNotAllowed(w, http.MethodGet+", "+http.MethodPost)
	}
}

func (h *Handler) sourceDetailHandler(w http.ResponseWriter, r *http.Request) {
	if !h.requireSourceService(w) {
		return
	}
	if _, ok := h.authenticateRequest(w, r); !ok {
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
		writeJSON(w, http.StatusOK, sourceGetResponse{Source: toSourceResponse(configuredSource)})
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
		writeJSON(w, http.StatusOK, sourceGetResponse{Source: toSourceResponse(updated)})
	case http.MethodDelete:
		if err := h.sources.DeleteSource(r.Context(), id); err != nil {
			status, code, message := sourceErrorStatus(err)
			writeAPIError(w, status, code, message)
			return
		}
		writeJSON(w, http.StatusOK, okResponse{OK: true})
	default:
		methodNotAllowed(w, http.MethodGet+", "+http.MethodPut+", "+http.MethodDelete)
	}
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

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, source.DefaultMaxPayloadBytes+1))
	if err != nil || int64(len(body)) > source.DefaultMaxPayloadBytes {
		writeAPIError(w, http.StatusRequestEntityTooLarge, "MGP-PAYLOAD-002", "Payload 超过大小限制")
		return
	}
	result, err := h.sources.Ingest(r.Context(), source.IngestInput{
		SourceCode: sourceCode,
		Method:     r.Method,
		Path:       r.URL.Path,
		Headers:    r.Header.Clone(),
		RemoteAddr: clientIPForIngest(r),
		Body:       body,
	})
	if err != nil {
		status, code, message := sourceErrorStatus(err)
		writeAPIError(w, status, code, message)
		return
	}
	writeJSON(w, http.StatusAccepted, ingestResponse{
		TraceID: result.TraceID,
		Status:  result.Status,
		Message: result.Message,
	})
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
	}, nil
}

func (r sourceRequest) toUpdateInput() (source.UpdateSourceInput, error) {
	return r.toCreateInput()
}

func toSourceResponse(configuredSource source.Source) sourceResponse {
	latestUpdatedAt := formatOptionalTime(configuredSource.LatestPayloadSampleUpdatedAt)
	return sourceResponse{
		ID:                           configuredSource.ID,
		Code:                         configuredSource.Code,
		Name:                         configuredSource.Name,
		Enabled:                      configuredSource.Enabled,
		AuthMode:                     configuredSource.AuthMode,
		AuthToken:                    configuredSource.AuthToken,
		HMACSecret:                   configuredSource.HMACSecret,
		IPAllowlist:                  configuredSource.IPAllowlist,
		CompatMode:                   configuredSource.CompatMode,
		InboundDedupeEnabled:         configuredSource.InboundDedupeEnabled,
		InboundDedupeStrategy:        string(configuredSource.InboundDedupeStrategy),
		InboundDedupeConfig:          defaultRawJSON(configuredSource.InboundDedupeConfig),
		RateLimitConfig:              defaultRawJSON(configuredSource.RateLimitConfig),
		LatestPayloadSample:          nullableRawJSON(configuredSource.LatestPayloadSample),
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

func clientIPForIngest(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

func sourceErrorStatus(err error) (int, string, string) {
	switch {
	case errors.Is(err, source.ErrInvalidInput):
		return http.StatusBadRequest, "MGP-REQ-001", "请求参数不合法"
	case errors.Is(err, source.ErrInvalidJSON):
		return http.StatusBadRequest, "MGP-PAYLOAD-001", "请求 JSON 不合法"
	case errors.Is(err, source.ErrInvalidDedupeConfig):
		return http.StatusBadRequest, "MGP-DEDUPE-001", "入站去重配置不合法"
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
