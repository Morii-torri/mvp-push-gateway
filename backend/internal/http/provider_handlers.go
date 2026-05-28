package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"mvp-push-gateway/backend/internal/provider"
)

type providerCapabilitiesResponse struct {
	Capabilities []capabilityResponse `json:"capabilities"`
}

type capabilityResponse struct {
	ID                      string                `json:"id"`
	ProviderType            provider.ProviderType `json:"provider_type"`
	DisplayName             string                `json:"display_name"`
	Category                string                `json:"category"`
	MessageType             string                `json:"message_type"`
	MessageSchema           json.RawMessage       `json:"message_schema"`
	CredentialSchema        json.RawMessage       `json:"credential_schema"`
	ChannelConfigSchema     json.RawMessage       `json:"channel_config_schema"`
	CustomBodyAllowed       bool                  `json:"custom_body_allowed"`
	RecipientRequired       bool                  `json:"recipient_required"`
	AllowNoRecipient        bool                  `json:"allow_no_recipient"`
	RecipientRequirement    string                `json:"recipient_requirement"`
	RecipientFieldName      string                `json:"recipient_field_name"`
	RecipientLocation       provider.Placement    `json:"recipient_location"`
	RecipientPath           string                `json:"recipient_path"`
	RecipientFormat         string                `json:"recipient_format"`
	IdentityKind            string                `json:"identity_kind"`
	TokenLocation           provider.Placement    `json:"token_location"`
	TokenFieldName          string                `json:"token_field_name"`
	TokenStrategy           json.RawMessage       `json:"token_strategy"`
	SendAPI                 json.RawMessage       `json:"send_api"`
	SuccessRule             json.RawMessage       `json:"success_rule"`
	RetryRule               json.RawMessage       `json:"retry_rule"`
	DefaultRateLimit        json.RawMessage       `json:"default_rate_limit"`
	DefaultTimeoutMS        int                   `json:"default_timeout_ms"`
	DefaultConcurrencyLimit int                   `json:"default_concurrency_limit"`
	DefaultRetryPolicy      json.RawMessage       `json:"default_retry_policy"`
	RequestExamples         json.RawMessage       `json:"request_examples"`
	CreatedAt               string                `json:"created_at"`
	UpdatedAt               string                `json:"updated_at"`
}

type channelsResponse struct {
	Channels []channelResponse `json:"channels"`
}

type channelResponse struct {
	ID               string                `json:"id"`
	ProviderType     provider.ProviderType `json:"provider_type"`
	Name             string                `json:"name"`
	Enabled          bool                  `json:"enabled"`
	AuthConfig       json.RawMessage       `json:"auth_config"`
	TokenConfig      json.RawMessage       `json:"token_config"`
	SendConfig       json.RawMessage       `json:"send_config"`
	RateLimitConfig  json.RawMessage       `json:"rate_limit_config"`
	ConcurrencyLimit int                   `json:"concurrency_limit"`
	TimeoutMS        int                   `json:"timeout_ms"`
	RetryPolicy      json.RawMessage       `json:"retry_policy"`
	DeadLetterPolicy json.RawMessage       `json:"dead_letter_policy"`
	CreatedAt        string                `json:"created_at"`
	UpdatedAt        string                `json:"updated_at"`
	IsCached         bool                  `json:"is_cached"`
	TokenCacheStatus string                `json:"token_cache_status,omitempty"`
	TokenRefreshedAt string                `json:"token_refreshed_at,omitempty"`
	TokenExpiresAt   string                `json:"token_expires_at,omitempty"`
}

type channelRequest struct {
	ProviderType     provider.ProviderType `json:"provider_type"`
	Name             string                `json:"name"`
	Enabled          *bool                 `json:"enabled"`
	AuthConfig       json.RawMessage       `json:"auth_config"`
	TokenConfig      json.RawMessage       `json:"token_config"`
	SendConfig       json.RawMessage       `json:"send_config"`
	RateLimitConfig  json.RawMessage       `json:"rate_limit_config"`
	ConcurrencyLimit int                   `json:"concurrency_limit"`
	TimeoutMS        int                   `json:"timeout_ms"`
	RetryPolicy      json.RawMessage       `json:"retry_policy"`
	DeadLetterPolicy json.RawMessage       `json:"dead_letter_policy"`
}

type channelResponseBody struct {
	Channel channelResponse `json:"channel"`
}

type buildRequestResponse struct {
	Request provider.BuiltRequest `json:"request"`
}

type testSendResponse struct {
	Result provider.TestSendResult `json:"result"`
}

type feishuResolveOpenIDRequest struct {
	Mobiles []string `json:"mobiles"`
}

func (h *Handler) providerCapabilitiesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	if !h.requireProviderService(w) {
		return
	}
	if _, ok := h.authenticateRequest(w, r); !ok {
		return
	}

	capabilities, err := h.providers.ListProviderCapabilities(r.Context())
	if err != nil {
		status, code, message := providerErrorStatus(err)
		writeAPIError(w, status, code, message)
		return
	}
	response := providerCapabilitiesResponse{Capabilities: make([]capabilityResponse, 0, len(capabilities))}
	for _, capability := range capabilities {
		response.Capabilities = append(response.Capabilities, toCapabilityResponse(capability))
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) channelsHandler(w http.ResponseWriter, r *http.Request) {
	if !h.requireProviderService(w) {
		return
	}
	adminUser, ok := h.authenticateRequest(w, r)
	if !ok {
		return
	}

	switch r.Method {
	case http.MethodGet:
		channels, err := h.providers.ListChannels(r.Context())
		if err != nil {
			status, code, message := providerErrorStatus(err)
			writeAPIError(w, status, code, message)
			return
		}
		response := channelsResponse{Channels: make([]channelResponse, 0, len(channels))}
		for _, channel := range channels {
			response.Channels = append(response.Channels, toChannelResponse(channel))
		}
		writeJSON(w, http.StatusOK, response)
	case http.MethodPost:
		var request channelRequest
		if err := decodeJSON(r, &request); err != nil {
			writeAPIError(w, http.StatusBadRequest, "MGP-REQ-001", "请求 JSON 不合法")
			return
		}
		created, err := h.providers.CreateChannel(r.Context(), request.toInput())
		if err != nil {
			status, code, message := providerErrorStatus(err)
			writeAPIError(w, status, code, message)
			return
		}
		response := channelResponseBody{Channel: toChannelResponse(created)}
		h.recordAudit(r, adminUser, "create", "channel", created.ID, request, response)
		writeJSON(w, http.StatusCreated, response)
	default:
		methodNotAllowed(w, http.MethodGet+", "+http.MethodPost)
	}
}

func (h *Handler) channelDetailHandler(w http.ResponseWriter, r *http.Request) {
	if !h.requireProviderService(w) {
		return
	}
	adminUser, ok := h.authenticateRequest(w, r)
	if !ok {
		return
	}

	rest := strings.TrimPrefix(r.URL.Path, h.cfg.Server.APIPrefix+"/channels/")
	parts := strings.Split(strings.Trim(rest, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		writeAPIError(w, http.StatusNotFound, "MGP-CHN-001", "平台实例不存在")
		return
	}
	channelID := parts[0]
	if len(parts) == 2 && parts[1] == "build-request" {
		h.channelBuildRequestHandler(w, r, channelID)
		return
	}
	if len(parts) == 2 && parts[1] == "test-send" {
		h.channelTestSendHandler(w, r, channelID)
		return
	}
	if len(parts) == 2 && parts[1] == "refresh-token" {
		h.channelRefreshTokenHandler(w, r, channelID)
		return
	}
	if len(parts) == 3 && parts[1] == "feishu" && parts[2] == "resolve-open-id" {
		h.channelFeishuResolveOpenIDHandler(w, r, channelID)
		return
	}
	if len(parts) != 1 {
		writeAPIError(w, http.StatusNotFound, "MGP-CHN-001", "平台实例不存在")
		return
	}

	switch r.Method {
	case http.MethodGet:
		channel, err := h.providers.GetChannel(r.Context(), channelID)
		if err != nil {
			status, code, message := providerErrorStatus(err)
			writeAPIError(w, status, code, message)
			return
		}
		writeJSON(w, http.StatusOK, channelResponseBody{Channel: toChannelResponse(channel)})
	case http.MethodPut:
		var request channelRequest
		if err := decodeJSON(r, &request); err != nil {
			writeAPIError(w, http.StatusBadRequest, "MGP-REQ-001", "请求 JSON 不合法")
			return
		}
		updated, err := h.providers.UpdateChannel(r.Context(), channelID, request.toInput())
		if err != nil {
			status, code, message := providerErrorStatus(err)
			writeAPIError(w, status, code, message)
			return
		}
		response := channelResponseBody{Channel: toChannelResponse(updated)}
		h.recordAudit(r, adminUser, "update", "channel", channelID, request, response)
		writeJSON(w, http.StatusOK, response)
	case http.MethodDelete:
		if err := h.providers.DeleteChannel(r.Context(), channelID); err != nil {
			status, code, message := providerErrorStatus(err)
			writeAPIError(w, status, code, message)
			return
		}
		response := okResponse{OK: true}
		h.recordAudit(r, adminUser, "delete", "channel", channelID, nil, response)
		writeJSON(w, http.StatusOK, response)
	default:
		methodNotAllowed(w, http.MethodGet+", "+http.MethodPut+", "+http.MethodDelete)
	}
}

func (h *Handler) channelBuildRequestHandler(w http.ResponseWriter, r *http.Request, channelID string) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	var request provider.BuildRequestInput
	if err := decodeJSON(r, &request); err != nil {
		writeAPIError(w, http.StatusBadRequest, "MGP-REQ-001", "请求 JSON 不合法")
		return
	}
	built, err := h.providers.BuildRequest(r.Context(), channelID, request)
	if err != nil {
		status, code, message := providerErrorStatus(err)
		writeAPIError(w, status, code, message)
		return
	}
	writeJSON(w, http.StatusOK, buildRequestResponse{Request: provider.RedactBuiltRequest(built)})
}

func (h *Handler) channelTestSendHandler(w http.ResponseWriter, r *http.Request, channelID string) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	var request provider.TestSendInput
	if err := decodeJSON(r, &request); err != nil {
		writeAPIError(w, http.StatusBadRequest, "MGP-REQ-001", "请求 JSON 不合法")
		return
	}
	result, err := h.providers.TestSend(r.Context(), channelID, request)
	if err != nil {
		status, code, message := providerErrorStatus(err)
		writeAPIError(w, status, code, message)
		return
	}
	writeJSON(w, http.StatusOK, testSendResponse{Result: result})
}

func (h *Handler) channelFeishuResolveOpenIDHandler(w http.ResponseWriter, r *http.Request, channelID string) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	var request feishuResolveOpenIDRequest
	if err := decodeJSON(r, &request); err != nil {
		writeAPIError(w, http.StatusBadRequest, "MGP-REQ-001", "请求 JSON 不合法")
		return
	}
	result, err := h.providers.ResolveFeishuOpenID(r.Context(), channelID, request.Mobiles)
	if err != nil {
		status, code, message := providerErrorStatus(err)
		writeAPIError(w, status, code, message)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) channelRefreshTokenHandler(w http.ResponseWriter, r *http.Request, channelID string) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	status, err := h.providers.RefreshToken(r.Context(), channelID)
	if err != nil {
		statusErr, code, message := providerErrorStatus(err)
		writeAPIError(w, statusErr, code, message)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":             "ok",
		"is_cached":          status.IsCached,
		"token_cache_status": status.Status,
		"token_refreshed_at": status.TokenRefreshed,
		"token_expires_at":   status.ExpiresAt,
	})
}

func (r channelRequest) toInput() provider.CreateChannelInput {
	enabled := true
	if r.Enabled != nil {
		enabled = *r.Enabled
	}
	return provider.CreateChannelInput{
		ProviderType:     r.ProviderType,
		Name:             r.Name,
		Enabled:          enabled,
		AuthConfig:       r.AuthConfig,
		TokenConfig:      r.TokenConfig,
		SendConfig:       r.SendConfig,
		RateLimitConfig:  r.RateLimitConfig,
		ConcurrencyLimit: r.ConcurrencyLimit,
		TimeoutMS:        r.TimeoutMS,
		RetryPolicy:      r.RetryPolicy,
		DeadLetterPolicy: r.DeadLetterPolicy,
	}
}

func toCapabilityResponse(capability provider.Capability) capabilityResponse {
	return capabilityResponse{
		ID:                      capability.ID,
		ProviderType:            capability.ProviderType,
		DisplayName:             capability.DisplayName,
		Category:                capability.Category,
		MessageType:             capability.MessageType,
		MessageSchema:           defaultRawJSON(capability.MessageSchema),
		CredentialSchema:        defaultRawJSON(capability.CredentialSchema),
		ChannelConfigSchema:     defaultRawJSON(capability.ChannelConfigSchema),
		CustomBodyAllowed:       capability.CustomBodyAllowed,
		RecipientRequired:       capability.RecipientRequired,
		AllowNoRecipient:        capability.AllowNoRecipient,
		RecipientRequirement:    capability.RecipientRequirement,
		RecipientFieldName:      capability.RecipientFieldName,
		RecipientLocation:       capability.RecipientLocation,
		RecipientPath:           capability.RecipientPath,
		RecipientFormat:         capability.RecipientFormat,
		IdentityKind:            capability.IdentityKind,
		TokenLocation:           capability.TokenLocation,
		TokenFieldName:          capability.TokenFieldName,
		TokenStrategy:           defaultRawJSON(capability.TokenStrategy),
		SendAPI:                 defaultRawJSON(capability.SendAPI),
		SuccessRule:             defaultRawJSON(capability.SuccessRule),
		RetryRule:               defaultRawJSON(capability.RetryRule),
		DefaultRateLimit:        defaultRawJSON(capability.DefaultRateLimit),
		DefaultTimeoutMS:        capability.DefaultTimeoutMS,
		DefaultConcurrencyLimit: capability.DefaultConcurrencyLimit,
		DefaultRetryPolicy:      defaultRawJSON(capability.DefaultRetryPolicy),
		RequestExamples:         defaultRawJSON(capability.RequestExamples),
		CreatedAt:               formatTime(capability.CreatedAt),
		UpdatedAt:               formatTime(capability.UpdatedAt),
	}
}

func toChannelResponse(channel provider.Channel) channelResponse {
	return channelResponse{
		ID:               channel.ID,
		ProviderType:     channel.ProviderType,
		Name:             channel.Name,
		Enabled:          channel.Enabled,
		AuthConfig:       defaultRawJSON(channel.AuthConfig),
		TokenConfig:      defaultRawJSON(channel.TokenConfig),
		SendConfig:       defaultRawJSON(channel.SendConfig),
		RateLimitConfig:  defaultRawJSON(channel.RateLimitConfig),
		ConcurrencyLimit: channel.ConcurrencyLimit,
		TimeoutMS:        channel.TimeoutMS,
		RetryPolicy:      defaultRawJSON(channel.RetryPolicy),
		DeadLetterPolicy: defaultRawJSON(channel.DeadLetterPolicy),
		CreatedAt:        formatTime(channel.CreatedAt),
		UpdatedAt:        formatTime(channel.UpdatedAt),
		IsCached:         channel.IsCached,
		TokenCacheStatus: channel.TokenCacheStatus,
		TokenRefreshedAt: channel.TokenRefreshedAt,
		TokenExpiresAt:   channel.TokenExpiresAt,
	}
}

func providerErrorStatus(err error) (int, string, string) {
	switch {
	case errors.Is(err, provider.ErrInvalidInput):
		if message := providerInvalidInputMessage(err); message != "" {
			return http.StatusBadRequest, "MGP-REQ-001", message
		}
		return http.StatusBadRequest, "MGP-REQ-001", "请求参数不合法"
	case errors.Is(err, provider.ErrNotFound):
		return http.StatusNotFound, "MGP-CHN-001", "平台实例不存在"
	case errors.Is(err, provider.ErrAlreadyExist):
		return http.StatusConflict, "MGP-CHN-001", "平台实例已存在"
	default:
		return http.StatusInternalServerError, "MGP-CHN-999", "平台服务内部错误"
	}
}

func providerInvalidInputMessage(err error) string {
	message := strings.TrimSpace(err.Error())
	if message == "" || message == provider.ErrInvalidInput.Error() {
		return ""
	}
	message = strings.TrimPrefix(message, provider.ErrInvalidInput.Error())
	message = strings.TrimPrefix(message, ":")
	return strings.TrimSpace(message)
}
