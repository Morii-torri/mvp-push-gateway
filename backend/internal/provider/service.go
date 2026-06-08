package provider

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/mail"
	"net/smtp"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

type ProviderType string

const (
	ProviderWeComApp      ProviderType = "wecom_app"
	ProviderWeComRobot    ProviderType = "wecom_robot"
	ProviderFeishuRobot   ProviderType = "feishu_robot"
	ProviderDingTalkWork  ProviderType = "dingtalk_work"
	ProviderDingTalkRobot ProviderType = "dingtalk_robot"
	ProviderFeishuGroup   ProviderType = "feishu_group"
	ProviderEmail         ProviderType = "email"
	ProviderAliyunSMS     ProviderType = "aliyun_sms"
	ProviderTencentSMS    ProviderType = "tencent_sms"
	ProviderBaiduSMS      ProviderType = "baidu_sms"
	ProviderSelf          ProviderType = "self"
	ProviderWebhook       ProviderType = "webhook"
	ProviderPushPlus      ProviderType = "pushplus"
	ProviderWxPusher      ProviderType = "wxpusher"
	ProviderServerChan    ProviderType = "serverchan"
	ProviderNtfy          ProviderType = "ntfy"
	ProviderGotify        ProviderType = "gotify"
	ProviderBark          ProviderType = "bark"
	ProviderPushMe        ProviderType = "pushme"
)

type Placement string

const (
	PlacementQuery  Placement = "query"
	PlacementHeader Placement = "header"
	PlacementBody   Placement = "body"
	PlacementPath   Placement = "path"
	PlacementNone   Placement = "none"
)

var (
	ErrNotFound     = errors.New("provider resource not found")
	ErrAlreadyExist = errors.New("provider resource already exists")
	ErrInvalidInput = errors.New("invalid provider input")
)

type Capability struct {
	ID                      string
	ProviderType            ProviderType
	DisplayName             string
	Category                string
	MessageType             string
	MessageSchema           json.RawMessage
	CredentialSchema        json.RawMessage
	ChannelConfigSchema     json.RawMessage
	CustomBodyAllowed       bool
	RecipientRequired       bool
	AllowNoRecipient        bool
	RecipientRequirement    string
	RecipientFieldName      string
	RecipientLocation       Placement
	RecipientPath           string
	RecipientFormat         string
	IdentityKind            string
	TokenLocation           Placement
	TokenFieldName          string
	TokenStrategy           json.RawMessage
	SendAPI                 json.RawMessage
	SuccessRule             json.RawMessage
	RetryRule               json.RawMessage
	DefaultRateLimit        json.RawMessage
	DefaultTimeoutMS        int
	DefaultConcurrencyLimit int
	DefaultRetryPolicy      json.RawMessage
	RequestExamples         json.RawMessage
	CreatedAt               time.Time
	UpdatedAt               time.Time
}

type Channel struct {
	ID               string
	ProviderType     ProviderType
	Name             string
	Enabled          bool
	Description      string
	AuthConfig       json.RawMessage
	TokenConfig      json.RawMessage
	SendConfig       json.RawMessage
	RateLimitConfig  json.RawMessage
	ConcurrencyLimit int
	TimeoutMS        int
	RetryPolicy      json.RawMessage
	DeadLetterPolicy json.RawMessage
	CreatedAt        time.Time
	UpdatedAt        time.Time
	IsCached         bool
	TokenCacheStatus string
	TokenRefreshedAt string
	TokenExpiresAt   string
	TokenLastError   string
}

type CreateChannelInput struct {
	ProviderType     ProviderType    `json:"provider_type"`
	Name             string          `json:"name"`
	Enabled          bool            `json:"enabled"`
	Description      string          `json:"description"`
	AuthConfig       json.RawMessage `json:"auth_config"`
	TokenConfig      json.RawMessage `json:"token_config"`
	SendConfig       json.RawMessage `json:"send_config"`
	RateLimitConfig  json.RawMessage `json:"rate_limit_config"`
	ConcurrencyLimit int             `json:"concurrency_limit"`
	TimeoutMS        int             `json:"timeout_ms"`
	RetryPolicy      json.RawMessage `json:"retry_policy"`
	DeadLetterPolicy json.RawMessage `json:"dead_letter_policy"`
}

type UpdateChannelInput = CreateChannelInput
type CreateChannelParams = CreateChannelInput
type UpdateChannelParams = UpdateChannelInput

type FeishuOpenIDResolveItem struct {
	Mobile string `json:"mobile"`
	OpenID string `json:"open_id"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

type FeishuOpenIDResolveResult struct {
	Success bool                      `json:"success"`
	Items   []FeishuOpenIDResolveItem `json:"items"`
	Errors  []string                  `json:"errors,omitempty"`
}

type DingTalkUserIDResolveItem struct {
	QueryWord string `json:"query_word"`
	UserID    string `json:"user_id"`
	Status    string `json:"status"`
	Error     string `json:"error,omitempty"`
}

type DingTalkUserIDResolveResult struct {
	Success bool                        `json:"success"`
	Items   []DingTalkUserIDResolveItem `json:"items"`
	Errors  []string                    `json:"errors,omitempty"`
}

type Store interface {
	SeedProviderCapabilities(ctx context.Context, capabilities []Capability) error
	ListProviderCapabilities(ctx context.Context) ([]Capability, error)
	ListChannels(ctx context.Context) ([]Channel, error)
	CreateChannel(ctx context.Context, params CreateChannelParams) (Channel, error)
	GetChannel(ctx context.Context, id string) (Channel, error)
	UpdateChannel(ctx context.Context, id string, params UpdateChannelParams) (Channel, error)
	DeleteChannel(ctx context.Context, id string) error
}

type Service struct {
	store             Store
	tokenManager      *TokenManager
	httpClientFactory func(time.Duration) *http.Client
}

type ServiceOption func(*Service)

func WithProviderHTTPClientFactory(factory func(time.Duration) *http.Client) ServiceOption {
	return func(s *Service) {
		if factory != nil {
			s.httpClientFactory = factory
			if s.tokenManager != nil {
				s.tokenManager.httpClientFactory = factory
			}
		}
	}
}

func NewService(store Store, options ...ServiceOption) *Service {
	var tokenStore TokenCacheStore
	if candidate, ok := store.(TokenCacheStore); ok {
		tokenStore = candidate
	}
	service := &Service{
		store:             store,
		httpClientFactory: newEgressHTTPClient,
	}
	service.tokenManager = NewTokenManager(tokenStore, WithTokenManagerHTTPClientFactory(service.httpClientFactory))
	for _, option := range options {
		option(service)
	}
	return service
}

func (s *Service) SeedProviderCapabilities(ctx context.Context) error {
	if s.store == nil {
		return ErrNotFound
	}
	return s.store.SeedProviderCapabilities(ctx, DefaultCapabilities())
}

func (s *Service) ListProviderCapabilities(ctx context.Context) ([]Capability, error) {
	if s.store == nil {
		return nil, ErrNotFound
	}
	return s.store.ListProviderCapabilities(ctx)
}

func (s *Service) ListChannels(ctx context.Context) ([]Channel, error) {
	if s.store == nil {
		return nil, ErrNotFound
	}
	channels, err := s.store.ListChannels(ctx)
	if err != nil {
		return nil, err
	}
	for i := range channels {
		channels[i] = s.withTokenStatus(ctx, channels[i])
	}
	return channels, nil
}

func (s *Service) CreateChannel(ctx context.Context, input CreateChannelInput) (Channel, error) {
	params, err := normalizeChannelInput(input)
	if err != nil {
		return Channel{}, err
	}
	return s.store.CreateChannel(ctx, params)
}

func (s *Service) GetChannel(ctx context.Context, id string) (Channel, error) {
	if strings.TrimSpace(id) == "" {
		return Channel{}, ErrInvalidInput
	}
	channel, err := s.store.GetChannel(ctx, id)
	if err != nil {
		return Channel{}, err
	}
	return s.withTokenStatus(ctx, channel), nil
}

func (s *Service) UpdateChannel(ctx context.Context, id string, input UpdateChannelInput) (Channel, error) {
	if strings.TrimSpace(id) == "" {
		return Channel{}, ErrInvalidInput
	}
	params, err := normalizeChannelInput(input)
	if err != nil {
		return Channel{}, err
	}
	return s.store.UpdateChannel(ctx, id, params)
}

func (s *Service) DeleteChannel(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return ErrInvalidInput
	}
	return s.store.DeleteChannel(ctx, id)
}

func (s *Service) BuildRequest(ctx context.Context, channelID string, input BuildRequestInput) (BuiltRequest, error) {
	channel, err := s.GetChannel(ctx, channelID)
	if err != nil {
		return BuiltRequest{}, err
	}
	if token, err := s.managedToken(ctx, channel); err != nil {
		return BuiltRequest{}, err
	} else if token != "" {
		input.Token = token
	}
	return BuildRequest(channel, input)
}

func (s *Service) BuildDeliveryRequest(ctx context.Context, channelID string, input BuildDeliveryRequestInput) (BuiltRequest, error) {
	channel, err := s.GetChannel(ctx, channelID)
	if err != nil {
		return BuiltRequest{}, err
	}
	if token, err := s.managedToken(ctx, channel); err != nil {
		return BuiltRequest{}, err
	} else if token != "" {
		input.Token = token
	}
	return BuildDeliveryRequest(channel, input)
}

func (s *Service) TestSend(ctx context.Context, channelID string, input TestSendInput) (TestSendResult, error) {
	channel, err := s.GetChannel(ctx, channelID)
	if err != nil {
		return TestSendResult{}, err
	}
	if token, err := s.managedToken(ctx, channel); err != nil {
		return TestSendResult{}, err
	} else if token != "" {
		input.Token = token
	}
	deliveryInput := testSendDeliveryInput(channel, input)
	built, err := BuildDeliveryRequest(channel, deliveryInput)
	if err != nil {
		return TestSendResult{}, err
	}
	if err := validateTestSendPrerequisites(channel, deliveryInput, built); err != nil {
		return TestSendResult{}, err
	}
	result := TestSendResult{
		Status:             "dry_run",
		Request:            RedactBuiltRequest(built),
		TargetContext:      deliveryInput.TargetContext,
		RenderedMessage:    deliveryInput.RenderedMessage,
		ResolvedRecipients: deliveryInput.ResolvedRecipients,
	}
	requestSnapshot, err := marshalJSON(map[string]any{
		"final_request":       redactedRequestSnapshot(built),
		"target_context":      deliveryInput.TargetContext,
		"rendered_message":    deliveryInput.RenderedMessage,
		"resolved_recipients": deliveryInput.ResolvedRecipients,
	})
	if err != nil {
		return TestSendResult{}, err
	}
	result.RequestSnapshot = requestSnapshot
	if !input.Send {
		return result, nil
	}
	if !input.LiveSendConfirmed {
		return TestSendResult{}, fmt.Errorf("%w: 真实发送需要二次确认，并确认会调用真实推送渠道", ErrInvalidInput)
	}

	started := time.Now().UTC()
	statusCode, headers, responseBody, sendErr := sendBuiltRequest(ctx, channel, built)
	result.StatusCode = statusCode
	result.DurationMS = int(time.Since(started).Milliseconds())
	if sendErr != nil {
		result.Status = "failed"
		result.ErrorMessage = sendErr.Error()
	} else {
		result.Status = "sent"
	}
	responseSnapshot, err := marshalJSON(map[string]any{
		"status_code": statusCode,
		"headers":     headers,
		"body":        jsonValue(responseBody),
		"error":       result.ErrorMessage,
	})
	if err != nil {
		return TestSendResult{}, err
	}
	result.ResponseSnapshot = responseSnapshot
	if sendErr != nil {
		return result, nil
	}
	if statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices {
		result.Status = "failed"
		result.ErrorMessage = fmt.Sprintf("upstream returned status %d", statusCode)
	}
	return result, nil
}

func testSendDeliveryInput(channel Channel, input TestSendInput) BuildDeliveryRequestInput {
	rendered := input.RenderedMessage
	if len(bytes.TrimSpace(rendered.Content)) == 0 {
		rendered = RenderedMessage{
			ProviderType: channel.ProviderType,
			Content:      input.Body,
		}
	}
	if rendered.ProviderType == "" {
		rendered.ProviderType = channel.ProviderType
	}
	resolvedRecipients := input.ResolvedRecipients
	if resolvedRecipients == nil {
		resolvedRecipients = ResolvedRecipientsFromValue(input.Recipient)
	}
	targetContext := input.TargetContext
	if targetContext.ChannelID == "" {
		targetContext.ChannelID = channel.ID
	}
	if targetContext.ChannelName == "" {
		targetContext.ChannelName = channel.Name
	}
	if targetContext.ProviderType == "" {
		targetContext.ProviderType = string(channel.ProviderType)
	}
	if targetContext.MessageType == "" {
		targetContext.MessageType = rendered.MessageType
	}
	return BuildDeliveryRequestInput{
		Token:                input.Token,
		RenderedMessage:      rendered,
		ResolvedRecipients:   resolvedRecipients,
		TargetContext:        targetContext,
		LegacyRecipientValue: input.Recipient,
	}
}

func validateTestSendPrerequisites(channel Channel, input BuildDeliveryRequestInput, built BuiltRequest) error {
	if strings.TrimSpace(built.URL) == "" {
		return fmt.Errorf("%w: 缺少发送 URL，请检查推送渠道发送配置", ErrInvalidInput)
	}
	if testSendRequiresRecipient(channel.ProviderType) &&
		!hasUsableRecipient(channel.ProviderType, input.ResolvedRecipients) &&
		!builtRequestHasProviderTarget(channel.ProviderType, built) {
		return fmt.Errorf("%w: 缺少测试接收人，请在路由策略接收人配置或测试接收人中提供必要身份", ErrInvalidInput)
	}
	if missing := missingCredentialFields(channel, input.Token); len(missing) > 0 {
		return fmt.Errorf("%w: 缺少推送渠道凭证或必要配置：%s", ErrInvalidInput, strings.Join(missing, "、"))
	}
	return nil
}

func testSendRequiresRecipient(providerType ProviderType) bool {
	switch providerType {
	case ProviderEmail,
		ProviderAliyunSMS,
		ProviderTencentSMS,
		ProviderBaiduSMS,
		ProviderWeComApp,
		ProviderFeishuRobot,
		ProviderDingTalkWork,
		ProviderWeComRobot,
		ProviderPushPlus,
		ProviderWxPusher,
		ProviderServerChan,
		ProviderBark,
		ProviderPushMe:
		return true
	default:
		return false
	}
}

func hasUsableRecipient(providerType ProviderType, recipients []ResolvedRecipient) bool {
	for _, recipient := range recipients {
		if !isEmptyValue(recipientIdentityValue(providerType, recipient)) {
			return true
		}
	}
	return false
}

func missingCredentialFields(channel Channel, token string) []string {
	auth := rawObject(channel.AuthConfig)
	send := rawObject(channel.SendConfig)
	missing := []string{}
	requireAny := func(label string, values ...any) {
		for _, value := range values {
			if !isEmptyValue(value) {
				return
			}
		}
		missing = append(missing, label)
	}

	switch channel.ProviderType {
	case ProviderSelf:
		requireAny("级联 base_url", auth["base_url"], send["base_url"])
		requireAny("级联 source_code", auth["source_code"], send["source_code"])
		requireAny("级联 source_token/HMAC", auth["source_token"], auth["hmac_secret"], token)
	case ProviderWxPusher:
		requireAny("WxPusher appToken", auth["app_token"], auth["appToken"], send["app_token"], send["appToken"], token)
	case ProviderEmail:
		requireAny("SMTP host", auth["host"], send["host"])
		requireAny("SMTP username", auth["username"], send["username"])
		requireAny("SMTP password", auth["password"], send["password"])
	case ProviderAliyunSMS:
		requireAny("阿里云 access_key_id", auth["access_key_id"])
		requireAny("阿里云 access_key_secret", auth["access_key_secret"])
		requireAny("短信模板 ID", send["template_id"], send["template_code"])
	case ProviderTencentSMS:
		requireAny("腾讯云 secret_id", auth["secret_id"])
		requireAny("腾讯云 secret_key", auth["secret_key"])
		requireAny("SmsSdkAppId", send["sms_sdk_app_id"])
		requireAny("短信模板 ID", send["template_id"])
	case ProviderBaiduSMS:
		requireAny("百度云 access_key_id", auth["access_key_id"])
		requireAny("百度云 secret_access_key", auth["secret_access_key"])
		requireAny("短信模板 ID", send["template_id"], send["template"])
	case ProviderWeComApp:
		requireAny("企业微信 access_token 或 corpid/corpsecret", token, auth["access_token"], auth["corpid"])
		if isEmptyValue(token) && isEmptyValue(auth["access_token"]) {
			requireAny("企业微信 corpsecret", auth["corpsecret"])
		}
		requireAny("企业微信 agentid", auth["agentid"], auth["agent_id"], send["agentid"], send["agent_id"])
	case ProviderWeComRobot:
		requireAny("企业微信机器人 Webhook 地址", auth["webhook_url"], send["webhook_url"])
	case ProviderDingTalkWork:
		requireAny("钉钉 access_token 或 corp_id/client_id/client_secret", token, auth["access_token"], auth["corp_id"])
		if isEmptyValue(token) && isEmptyValue(auth["access_token"]) {
			requireAny("钉钉 client_id", auth["client_id"])
			requireAny("钉钉 client_secret", auth["client_secret"])
		}
		requireAny("钉钉 robotCode", send["robot_code"], send["robotCode"], auth["robot_code"], auth["robotCode"])
	case ProviderDingTalkRobot:
		// Access token is resolved from the recipient identity for DingTalk group robots.
	case ProviderFeishuRobot:
		requireAny("飞书 app_id", auth["app_id"])
		requireAny("飞书 app_secret", auth["app_secret"])
	case ProviderNtfy:
		requireAny("ntfy server_url", auth["server_url"], send["server_url"])
		requireAny("ntfy topic", auth["topic"], send["topic"])
	case ProviderGotify:
		requireAny("Gotify server_url", auth["server_url"], send["server_url"])
		requireAny("Gotify app_token", auth["app_token"], send["app_token"], token)
	case ProviderBark:
		requireAny("Bark server_url", auth["server_url"], send["server_url"])
	case ProviderPushMe:
		requireAny("PushMe server_url", auth["server_url"], send["server_url"])
	}
	return missing
}

func builtRequestHasProviderTarget(providerType ProviderType, built BuiltRequest) bool {
	body := rawObject(built.Body)
	switch providerType {
	case ProviderPushPlus:
		return stringConfig(body, "token") != ""
	case ProviderWxPusher:
		return len(listConfig(body, "uids")) > 0 || len(rawListConfig(body, "topicIds", "topic_ids")) > 0
	case ProviderServerChan:
		return strings.Contains(built.URL, ".push.ft07.com/send/") && strings.HasSuffix(strings.TrimSpace(built.URL), ".send")
	case ProviderWeComRobot:
		parsed, err := url.Parse(built.URL)
		return err == nil && strings.TrimSpace(parsed.Query().Get("key")) != ""
	case ProviderBark:
		return stringConfig(body, "device_key") != "" || len(listConfig(body, "device_keys")) > 0
	case ProviderPushMe:
		return stringConfig(body, "push_key") != ""
	default:
		return false
	}
}

func rawObject(raw json.RawMessage) map[string]any {
	if len(bytes.TrimSpace(raw)) == 0 {
		return map[string]any{}
	}
	var object map[string]any
	if err := json.Unmarshal(raw, &object); err != nil || object == nil {
		return map[string]any{}
	}
	return object
}

func DefaultCapabilities() []Capability {
	return builtInCapabilities()
}

type capabilitySpec struct {
	ProviderType         ProviderType
	DisplayName          string
	Category             string
	MessageType          string
	MessageSchema        json.RawMessage
	CredentialSchema     json.RawMessage
	ChannelConfigSchema  json.RawMessage
	CustomBodyAllowed    bool
	RecipientRequired    bool
	AllowNoRecipient     bool
	RecipientRequirement string
	RecipientFieldName   string
	RecipientLocation    Placement
	RecipientPath        string
	RecipientFormat      string
	IdentityKind         string
	TokenLocation        Placement
	TokenFieldName       string
	TokenStrategy        json.RawMessage
	SendAPI              json.RawMessage
	SuccessRule          json.RawMessage
	RetryRule            json.RawMessage
	DefaultRateLimit     json.RawMessage
	DefaultTimeoutMS     int
	DefaultConcurrency   int
	DefaultRetryPolicy   json.RawMessage
	RequestExamples      json.RawMessage
}

func capability(spec capabilitySpec) Capability {
	if spec.RecipientRequirement == "" {
		spec.RecipientRequirement = "system"
		if !spec.RecipientRequired && spec.AllowNoRecipient {
			spec.RecipientRequirement = "none"
		}
	}
	if spec.DefaultTimeoutMS == 0 {
		spec.DefaultTimeoutMS = 5000
	}
	if spec.DefaultConcurrency == 0 {
		spec.DefaultConcurrency = 5
	}
	return Capability{
		ID:                      uuid.NewString(),
		ProviderType:            spec.ProviderType,
		DisplayName:             spec.DisplayName,
		Category:                spec.Category,
		MessageType:             spec.MessageType,
		MessageSchema:           spec.MessageSchema,
		CredentialSchema:        spec.CredentialSchema,
		ChannelConfigSchema:     spec.ChannelConfigSchema,
		CustomBodyAllowed:       spec.CustomBodyAllowed,
		RecipientRequired:       spec.RecipientRequired,
		AllowNoRecipient:        spec.AllowNoRecipient,
		RecipientRequirement:    spec.RecipientRequirement,
		RecipientFieldName:      spec.RecipientFieldName,
		RecipientLocation:       spec.RecipientLocation,
		RecipientPath:           spec.RecipientPath,
		RecipientFormat:         spec.RecipientFormat,
		IdentityKind:            spec.IdentityKind,
		TokenLocation:           spec.TokenLocation,
		TokenFieldName:          spec.TokenFieldName,
		TokenStrategy:           spec.TokenStrategy,
		SendAPI:                 spec.SendAPI,
		SuccessRule:             spec.SuccessRule,
		RetryRule:               spec.RetryRule,
		DefaultRateLimit:        spec.DefaultRateLimit,
		DefaultTimeoutMS:        spec.DefaultTimeoutMS,
		DefaultConcurrencyLimit: spec.DefaultConcurrency,
		DefaultRetryPolicy:      spec.DefaultRetryPolicy,
		RequestExamples:         spec.RequestExamples,
	}
}

func rawJSON(value string) json.RawMessage {
	return json.RawMessage(value)
}

func textContentSchema() json.RawMessage {
	return rawJSON(`{"type":"object","required":["content"],"properties":{"content":{"type":"string","title":"Content","default":"{{ payload.title }}"},"title":{"type":"string","title":"Title","default":"{{ payload.title }}"}}}`)
}

func titleContentSchema() json.RawMessage {
	return rawJSON(`{"type":"object","required":["title","content"],"properties":{"title":{"type":"string","title":"Title","default":"{{ payload.title }}"},"content":{"type":"string","title":"Content","default":"{{ payload.content }}"}}}`)
}

func emailContentSchema() json.RawMessage {
	return rawJSON(`{"type":"object","required":["subject","body","format"],"field_order":["subject","body","format"],"properties":{"subject":{"type":"string","title":"主题","default":"{{ payload.title }}"},"body":{"type":"string","title":"正文","default":"{{ payload.content }}"},"format":{"type":"string","title":"内容格式","default":"text","enum":["text","html"],"enum_labels":{"text":"纯文本","html":"HTML"}}}}`)
}

func smsContentSchema() json.RawMessage {
	return rawJSON(`{"type":"object","required":["content"],"properties":{"content":{"type":"string","title":"SMS content","default":"{{ payload.title }}"},"template_params":{"type":"object","title":"Template parameters","additionalProperties":true}}}`)
}

func webhookContentSchema() json.RawMessage {
	return rawJSON(`{"type":"object","required":["body"],"field_order":["body"],"properties":{"body":{"type":"object","title":"Body","additionalProperties":true,"default":{"title":"告警标题","level":"critical","content":"告警内容","biz_id":"order-10001","timestamp":"2026-06-02T10:00:00+08:00"}}},"additionalProperties":false}`)
}

func normalizeChannelInput(input CreateChannelInput) (CreateChannelParams, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	if input.Name == "" || !validProviderType(input.ProviderType) {
		return CreateChannelParams{}, ErrInvalidInput
	}
	var err error
	if input.AuthConfig, err = normalizeJSONConfig(input.AuthConfig); err != nil {
		return CreateChannelParams{}, err
	}
	if input.TokenConfig, err = normalizeJSONConfig(input.TokenConfig); err != nil {
		return CreateChannelParams{}, err
	}
	if input.SendConfig, err = normalizeJSONConfig(input.SendConfig); err != nil {
		return CreateChannelParams{}, err
	}
	if input.RateLimitConfig, err = normalizeJSONConfig(input.RateLimitConfig); err != nil {
		return CreateChannelParams{}, err
	}
	if input.RetryPolicy, err = normalizeJSONConfigWithDefault(input.RetryPolicy, `{"max_attempts":3}`); err != nil {
		return CreateChannelParams{}, err
	}
	if input.DeadLetterPolicy, err = normalizeJSONConfig(input.DeadLetterPolicy); err != nil {
		return CreateChannelParams{}, err
	}
	if input.ConcurrencyLimit == 0 {
		input.ConcurrencyLimit = 5
	}
	if input.TimeoutMS == 0 {
		input.TimeoutMS = 5000
	}
	if input.ConcurrencyLimit < 1 || input.TimeoutMS < 1 {
		return CreateChannelParams{}, ErrInvalidInput
	}
	return input, nil
}

func validProviderType(providerType ProviderType) bool {
	switch providerType {
	case ProviderWeComApp,
		ProviderWeComRobot,
		ProviderFeishuRobot,
		ProviderFeishuGroup,
		ProviderDingTalkWork,
		ProviderDingTalkRobot,
		ProviderEmail,
		ProviderAliyunSMS,
		ProviderTencentSMS,
		ProviderBaiduSMS,
		ProviderSelf,
		ProviderWebhook,
		ProviderPushPlus,
		ProviderWxPusher,
		ProviderServerChan,
		ProviderNtfy,
		ProviderGotify,
		ProviderBark,
		ProviderPushMe:
		return true
	default:
		return false
	}
}

func normalizeJSONConfig(raw json.RawMessage) (json.RawMessage, error) {
	return normalizeJSONConfigWithDefault(raw, `{}`)
}

func normalizeJSONConfigWithDefault(raw json.RawMessage, fallback string) (json.RawMessage, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return json.RawMessage(fallback), nil
	}
	if !json.Valid(raw) {
		return nil, ErrInvalidInput
	}
	return append(json.RawMessage(nil), bytes.TrimSpace(raw)...), nil
}

type BuildRequestInput struct {
	Token     string          `json:"token"`
	Recipient any             `json:"recipient"`
	Body      json.RawMessage `json:"body"`
}

type RenderedMessage struct {
	ProviderType ProviderType    `json:"provider_type,omitempty"`
	MessageType  string          `json:"message_type,omitempty"`
	Content      json.RawMessage `json:"content,omitempty"`
}

type ResolvedRecipient struct {
	SystemUserID string            `json:"system_user_id,omitempty"`
	Mobile       string            `json:"mobile,omitempty"`
	Email        string            `json:"email,omitempty"`
	PlatformIDs  map[string]string `json:"platform_ids,omitempty"`
	Value        any               `json:"value,omitempty"`
}

type DeliveryTargetContext struct {
	DeliveryAttemptID string `json:"delivery_attempt_id"`
	MessageID         string `json:"message_id"`
	ChannelID         string `json:"channel_id"`
	ChannelName       string `json:"channel_name"`
	ProviderType      string `json:"provider_type"`
	MessageType       string `json:"message_type"`
	TemplateVersionID string `json:"template_version_id"`
	JobID             string `json:"job_id"`
}

type BuildDeliveryRequestInput struct {
	Token                string                `json:"token"`
	RenderedMessage      RenderedMessage       `json:"rendered_message"`
	ResolvedRecipients   []ResolvedRecipient   `json:"resolved_recipients"`
	TargetContext        DeliveryTargetContext `json:"target_context"`
	LegacyRecipientValue any                   `json:"-"`
}

type TestSendInput struct {
	BuildRequestInput
	RenderedMessage    RenderedMessage       `json:"rendered_message"`
	ResolvedRecipients []ResolvedRecipient   `json:"resolved_recipients"`
	TargetContext      DeliveryTargetContext `json:"target_context"`
	Send               bool                  `json:"send"`
	LiveSendConfirmed  bool                  `json:"live_send_confirmed"`
}

type BuiltRequest struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
	Query   map[string]string `json:"query"`
	Body    json.RawMessage   `json:"body"`
}

type TestSendResult struct {
	Status             string                `json:"status"`
	Request            BuiltRequest          `json:"request"`
	TargetContext      DeliveryTargetContext `json:"target_context"`
	RenderedMessage    RenderedMessage       `json:"rendered_message"`
	ResolvedRecipients []ResolvedRecipient   `json:"resolved_recipients"`
	RequestSnapshot    json.RawMessage       `json:"request_snapshot"`
	ResponseSnapshot   json.RawMessage       `json:"response_snapshot"`
	StatusCode         int                   `json:"status_code"`
	DurationMS         int                   `json:"duration_ms"`
	ErrorMessage       string                `json:"error_message"`
}

type requestConfig struct {
	Method            string            `json:"method"`
	URL               string            `json:"url"`
	Headers           map[string]string `json:"headers"`
	Body              json.RawMessage   `json:"body"`
	Token             placementConfig   `json:"token"`
	Recipient         placementConfig   `json:"recipient"`
	SkipRenderedMerge bool              `json:"-"`
	OmitBody          bool              `json:"-"`
}

type placementConfig struct {
	Location  Placement `json:"location"`
	FieldName string    `json:"field_name"`
	Path      string    `json:"path"`
	Prefix    string    `json:"prefix"`
	Format    string    `json:"format"`
}

func BuildRequest(channel Channel, input BuildRequestInput) (BuiltRequest, error) {
	return BuildDeliveryRequest(channel, BuildDeliveryRequestInput{
		Token: input.Token,
		RenderedMessage: RenderedMessage{
			ProviderType: channel.ProviderType,
			Content:      input.Body,
		},
		ResolvedRecipients:   ResolvedRecipientsFromValue(input.Recipient),
		LegacyRecipientValue: input.Recipient,
	})
}

func BuildDeliveryRequest(channel Channel, input BuildDeliveryRequestInput) (BuiltRequest, error) {
	recipientValue := input.LegacyRecipientValue
	if isEmptyValue(recipientValue) {
		recipientValue = recipientValueFromResolved(channel.ProviderType, input.ResolvedRecipients)
	}
	return buildRequest(channel, BuildRequestInput{
		Token:     input.Token,
		Recipient: recipientValue,
		Body:      input.RenderedMessage.Content,
	})
}

func buildRequest(channel Channel, input BuildRequestInput) (BuiltRequest, error) {
	config, err := requestConfigFrom(channel, input)
	if err != nil {
		return BuiltRequest{}, err
	}
	if strings.TrimSpace(config.Method) == "" {
		config.Method = "POST"
	}
	config.Method = strings.ToUpper(strings.TrimSpace(config.Method))
	if config.Headers == nil {
		config.Headers = map[string]string{}
	}

	bodyMap, err := decodeBody(config.Body)
	if err != nil {
		return BuiltRequest{}, err
	}
	if !config.SkipRenderedMerge && len(bytes.TrimSpace(input.Body)) > 0 {
		inputBody, err := decodeBody(input.Body)
		if err != nil {
			return BuiltRequest{}, err
		}
		for key, value := range inputBody {
			bodyMap[key] = value
		}
	}

	query := map[string]string{}
	requestURL := strings.TrimSpace(config.URL)
	if err := applyPlacement(&requestURL, config.Headers, query, bodyMap, config.Token, config.Token.Prefix+input.Token); err != nil {
		return BuiltRequest{}, err
	}
	if err := applyPlacement(&requestURL, config.Headers, query, bodyMap, config.Recipient, formatRecipient(input.Recipient, config.Recipient.Format)); err != nil {
		return BuiltRequest{}, err
	}

	if len(query) > 0 {
		parsed, err := url.Parse(requestURL)
		if err != nil {
			return BuiltRequest{}, ErrInvalidInput
		}
		values := parsed.Query()
		keys := make([]string, 0, len(query))
		for key := range query {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			values.Set(key, query[key])
		}
		parsed.RawQuery = values.Encode()
		requestURL = parsed.String()
	}

	var body json.RawMessage
	if !config.OmitBody {
		body, err = json.Marshal(bodyMap)
		if err != nil {
			return BuiltRequest{}, ErrInvalidInput
		}
	}
	return BuiltRequest{
		Method:  config.Method,
		URL:     requestURL,
		Headers: config.Headers,
		Query:   query,
		Body:    body,
	}, nil
}

func ResolvedRecipientsFromValue(value any) []ResolvedRecipient {
	switch typed := value.(type) {
	case nil:
		return nil
	case []ResolvedRecipient:
		recipients := make([]ResolvedRecipient, len(typed))
		copy(recipients, typed)
		return recipients
	case []string:
		recipients := make([]ResolvedRecipient, 0, len(typed))
		for _, item := range typed {
			recipients = append(recipients, ResolvedRecipient{Value: item})
		}
		return recipients
	case []any:
		recipients := make([]ResolvedRecipient, 0, len(typed))
		for _, item := range typed {
			recipients = append(recipients, resolvedRecipientFromValue(item))
		}
		return recipients
	case map[string]any:
		return []ResolvedRecipient{resolvedRecipientFromMap(typed)}
	case map[string]string:
		return []ResolvedRecipient{resolvedRecipientFromStringMap(typed)}
	default:
		return []ResolvedRecipient{{Value: typed}}
	}
}

func resolvedRecipientFromValue(value any) ResolvedRecipient {
	if object, ok := value.(map[string]any); ok {
		return resolvedRecipientFromMap(object)
	}
	if object, ok := value.(map[string]string); ok {
		return resolvedRecipientFromStringMap(object)
	}
	return ResolvedRecipient{Value: value}
}

func resolvedRecipientFromStringMap(value map[string]string) ResolvedRecipient {
	object := make(map[string]any, len(value))
	for key, item := range value {
		object[key] = item
	}
	return resolvedRecipientFromMap(object)
}

func resolvedRecipientFromMap(value map[string]any) ResolvedRecipient {
	recipient := ResolvedRecipient{
		SystemUserID: stringFromMap(value, "system_user_id", "user_id"),
		Mobile:       stringFromMap(value, "mobile", "phone"),
		Email:        stringFromMap(value, "email"),
		PlatformIDs:  map[string]string{},
	}
	if nested, ok := value["platform_ids"].(map[string]any); ok {
		for key, item := range nested {
			if stringValue := strings.TrimSpace(fmt.Sprint(item)); stringValue != "" {
				recipient.PlatformIDs[key] = stringValue
			}
		}
	}
	if nested, ok := value["platform_ids"].(map[string]string); ok {
		for key, item := range nested {
			if stringValue := strings.TrimSpace(item); stringValue != "" {
				recipient.PlatformIDs[key] = stringValue
			}
		}
	}
	for _, key := range []string{"wecom_robot_key", "wecom_userid", "feishu_open_id", "feishu_user_id", "dingtalk_userid", "dingtalk_robot_access_token", "wxpusher_uid", "bark_device_key", "userid", "open_id"} {
		if stringValue := stringFromMap(value, key); stringValue != "" {
			recipient.PlatformIDs[key] = stringValue
		}
	}
	if recipient.SystemUserID == "" && recipient.Mobile == "" && recipient.Email == "" && len(recipient.PlatformIDs) == 0 {
		recipient.Value = value
	}
	return recipient
}

func recipientValueFromResolved(providerType ProviderType, recipients []ResolvedRecipient) any {
	if len(recipients) == 0 {
		return nil
	}
	values := make([]any, 0, len(recipients))
	for _, recipient := range recipients {
		value := recipientIdentityValue(providerType, recipient)
		if isEmptyValue(value) {
			continue
		}
		values = append(values, value)
	}
	if len(values) == 0 {
		return nil
	}
	if len(values) == 1 {
		return values[0]
	}
	return values
}

func recipientIdentityValue(providerType ProviderType, recipient ResolvedRecipient) any {
	for _, key := range providerIdentityKeys(providerType) {
		if value := strings.TrimSpace(recipient.PlatformIDs[key]); value != "" {
			return value
		}
	}
	if !isEmptyValue(recipient.Value) {
		return recipient.Value
	}
	switch providerType {
	case ProviderEmail:
		return recipient.Email
	case ProviderAliyunSMS, ProviderTencentSMS, ProviderBaiduSMS:
		return recipient.Mobile
	case ProviderSelf:
		return recipient.SystemUserID
	default:
		return firstString(recipient.SystemUserID, recipient.Mobile, recipient.Email)
	}
}

func providerIdentityKeys(providerType ProviderType) []string {
	switch providerType {
	case ProviderWeComApp:
		return []string{"wecom_userid", "userid"}
	case ProviderWeComRobot:
		return []string{"wecom_robot_key", "robot_key", "key"}
	case ProviderFeishuRobot:
		return []string{"feishu_open_id", "feishu_user_id", "open_id", "user_id"}
	case ProviderFeishuGroup:
		return []string{"feishu_webhook_token", "feishu_hook_token", "hook_token", "token"}
	case ProviderDingTalkWork:
		return []string{"dingtalk_userid", "userid", "user_id"}
	case ProviderDingTalkRobot:
		return []string{"dingtalk_robot_access_token", "access_token", "token"}
	case ProviderWxPusher:
		return []string{"wxpusher_uid", "uid"}
	case ProviderPushPlus:
		return []string{"pushplus_token", "token"}
	case ProviderServerChan:
		return []string{"serverchan_sendkey", "sendkey", "send_key"}
	case ProviderBark:
		return []string{"bark_device_key", "device_key"}
	case ProviderPushMe:
		return []string{"pushme_push_key", "push_key"}
	default:
		return nil
	}
}

func stringFromMap(value map[string]any, keys ...string) string {
	for _, key := range keys {
		item, ok := value[key]
		if !ok || item == nil {
			continue
		}
		stringValue := strings.TrimSpace(fmt.Sprint(item))
		if stringValue != "" {
			return stringValue
		}
	}
	return ""
}

func firstString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func sendBuiltRequest(ctx context.Context, channel Channel, built BuiltRequest) (int, map[string][]string, []byte, error) {
	if channel.ProviderType == ProviderEmail || strings.EqualFold(built.Method, "SMTP_SEND") {
		return sendSMTPBuiltRequest(ctx, channel, built)
	}
	body := built.Body
	if len(bytes.TrimSpace(body)) == 0 && !strings.EqualFold(built.Method, "GET") {
		body = json.RawMessage(`{}`)
	}
	req, err := http.NewRequestWithContext(ctx, built.Method, built.URL, bytes.NewReader(body))
	if err != nil {
		return 0, nil, nil, err
	}
	for key, value := range built.Headers {
		req.Header.Set(key, value)
	}
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	timeout := time.Duration(channel.TimeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	client := newEgressHTTPClient(timeout)
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, nil, err
	}
	defer resp.Body.Close()
	responseBody, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return resp.StatusCode, resp.Header, responseBody, readErr
	}
	return resp.StatusCode, resp.Header, responseBody, nil
}

var smtpTLSConfigForHost = func(host string) *tls.Config {
	return &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}
}

var smtpResolveEgressAddress = func(ctx context.Context, endpoint smtpEndpoint) (string, error) {
	return resolveEgressDialAddress(ctx, endpoint.host, endpoint.port)
}

func sendSMTPBuiltRequest(ctx context.Context, channel Channel, built BuiltRequest) (int, map[string][]string, []byte, error) {
	body, err := decodeObjectConfig(built.Body)
	if err != nil {
		return 0, nil, nil, err
	}
	auth, err := decodeObjectConfig(channel.AuthConfig)
	if err != nil {
		return 0, nil, nil, err
	}
	send, err := decodeObjectConfig(channel.SendConfig)
	if err != nil {
		return 0, nil, nil, err
	}
	endpoint, err := smtpEndpointFromRequest(built.URL, body)
	if err != nil {
		return 0, nil, nil, err
	}
	username := firstString(stringConfig(auth, "username"), stringConfig(send, "username"), stringConfig(body, "username"))
	password := firstString(stringConfig(auth, "password"), stringConfig(send, "password"))
	fromHeader, err := smtpFromHeader(username, firstString(stringConfig(body, "from"), stringConfig(send, "from")))
	if err != nil {
		return 0, nil, nil, err
	}
	envelopeFrom := firstString(stringConfig(body, "smtp_envelope_from"), smtpEnvelopeFrom(username, fromHeader))
	if envelopeFrom == "" {
		return 0, nil, nil, fmt.Errorf("%w: 缺少邮件发件人地址", ErrInvalidInput)
	}
	to := stringListFromAny(body["to"])
	if len(to) == 0 {
		return 0, nil, nil, fmt.Errorf("%w: 缺少邮件收件人地址", ErrInvalidInput)
	}
	cc := stringListFromAny(body["cc"])
	bcc := stringListFromAny(body["bcc"])
	recipients := append(append(append([]string{}, to...), cc...), bcc...)
	subject := firstString(stringConfig(body, "subject"), "通知")
	messageBody := firstString(stringConfig(body, "body"), stringConfig(body, "content"), stringConfig(body, "text"))
	format := normalizedEmailContentFormat(firstString(stringConfig(body, "format"), stringConfig(body, "content_type")))
	message := buildSMTPMessage(fromHeader, to, cc, firstString(stringConfig(body, "reply_to"), stringConfig(send, "reply_to")), subject, messageBody, format)

	timeout := time.Duration(channel.TimeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	client, err := dialSMTPClient(ctx, endpoint, timeout)
	if err != nil {
		return 0, nil, nil, err
	}
	defer client.Close()
	if username != "" || password != "" {
		if err := client.Auth(smtp.PlainAuth("", username, password, endpoint.host)); err != nil {
			return 0, nil, nil, err
		}
	}
	if err := client.Mail(envelopeFrom); err != nil {
		return 0, nil, nil, err
	}
	for _, recipient := range recipients {
		address, err := emailAddressOnly(recipient)
		if err != nil {
			return 0, nil, nil, err
		}
		if err := client.Rcpt(address); err != nil {
			return 0, nil, nil, err
		}
	}
	writer, err := client.Data()
	if err != nil {
		return 0, nil, nil, err
	}
	if _, err := writer.Write(message); err != nil {
		_ = writer.Close()
		return 0, nil, nil, err
	}
	if err := writer.Close(); err != nil {
		return 0, nil, nil, err
	}
	if err := client.Quit(); err != nil {
		return 0, nil, nil, err
	}
	response, err := marshalJSON(map[string]any{
		"accepted":   true,
		"recipients": recipients,
		"protocol":   "smtp",
	})
	if err != nil {
		return 0, nil, nil, err
	}
	return http.StatusAccepted, map[string][]string{"Protocol": {"smtp"}}, response, nil
}

type smtpEndpoint struct {
	host     string
	port     string
	address  string
	security string
}

func smtpEndpointFromRequest(rawURL string, body map[string]any) (smtpEndpoint, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Host == "" {
		return smtpEndpoint{}, ErrInvalidInput
	}
	host := parsed.Hostname()
	port := parsed.Port()
	if port == "" {
		port = "465"
	}
	security := strings.ToUpper(firstString(stringConfig(body, "security"), "SSL"))
	if security != "SSL" && security != "STARTTLS" {
		return smtpEndpoint{}, fmt.Errorf("%w: SMTP 仅支持 SSL 或 STARTTLS", ErrInvalidInput)
	}
	return smtpEndpoint{host: host, port: port, address: net.JoinHostPort(host, port), security: security}, nil
}

func dialSMTPClient(ctx context.Context, endpoint smtpEndpoint, timeout time.Duration) (*smtp.Client, error) {
	dialAddress, err := smtpResolveEgressAddress(ctx, endpoint)
	if err != nil {
		return nil, err
	}
	dialer := &net.Dialer{Timeout: timeout}
	if endpoint.security == "STARTTLS" {
		conn, err := dialer.DialContext(ctx, "tcp", dialAddress)
		if err != nil {
			return nil, err
		}
		client, err := smtp.NewClient(conn, endpoint.host)
		if err != nil {
			_ = conn.Close()
			return nil, err
		}
		if err := client.StartTLS(smtpTLSConfigForHost(endpoint.host)); err != nil {
			_ = client.Close()
			return nil, err
		}
		return client, nil
	}
	conn, err := tls.DialWithDialer(dialer, "tcp", dialAddress, smtpTLSConfigForHost(endpoint.host))
	if err != nil {
		return nil, err
	}
	client, err := smtp.NewClient(conn, endpoint.host)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	return client, nil
}

func buildSMTPMessage(from string, to, cc []string, replyTo, subject, body, format string) []byte {
	contentType := "text/plain"
	if normalizedEmailContentFormat(format) == "html" {
		contentType = "text/html"
	}
	headers := []string{
		"From: " + from,
		"To: " + strings.Join(to, ", "),
		"Subject: " + mime.QEncoding.Encode("utf-8", subject),
		"Date: " + time.Now().Format(time.RFC1123Z),
		"MIME-Version: 1.0",
		"Content-Type: " + contentType + "; charset=UTF-8",
		"Content-Transfer-Encoding: 8bit",
	}
	if len(cc) > 0 {
		headers = append(headers, "Cc: "+strings.Join(cc, ", "))
	}
	if strings.TrimSpace(replyTo) != "" {
		headers = append(headers, "Reply-To: "+replyTo)
	}
	return []byte(strings.Join(headers, "\r\n") + "\r\n\r\n" + normalizeSMTPBody(body))
}

func normalizeSMTPBody(body string) string {
	normalized := strings.ReplaceAll(body, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	return strings.ReplaceAll(normalized, "\n", "\r\n")
}

func normalizedEmailContentFormat(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "html", "text/html":
		return "html"
	default:
		return "text"
	}
}

func emailAddressOnly(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", fmt.Errorf("%w: 缺少邮件地址", ErrInvalidInput)
	}
	address, err := mail.ParseAddress(trimmed)
	if err != nil {
		return "", err
	}
	return address.Address, nil
}

func smtpFromHeader(username, fromValue string) (string, error) {
	usernameAddress, usernameErr := emailAddressOnly(username)
	trimmedFrom := strings.TrimSpace(fromValue)
	if trimmedFrom == "" {
		if usernameErr != nil {
			return "", fmt.Errorf("%w: 缺少邮件发件人地址", ErrInvalidInput)
		}
		return (&mail.Address{Address: usernameAddress}).String(), nil
	}
	if address, err := mail.ParseAddress(trimmedFrom); err == nil {
		return address.String(), nil
	}
	if usernameErr == nil && !strings.Contains(trimmedFrom, "@") && !strings.ContainsAny(trimmedFrom, "<>") {
		return (&mail.Address{Name: trimmedFrom, Address: usernameAddress}).String(), nil
	}
	return "", fmt.Errorf("%w: 发件人显示名或地址格式无效", ErrInvalidInput)
}

func smtpEnvelopeFrom(username, fromHeader string) string {
	if address, err := emailAddressOnly(username); err == nil {
		return address
	}
	if address, err := emailAddressOnly(fromHeader); err == nil {
		return address
	}
	return ""
}

func marshalJSON(value any) (json.RawMessage, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return raw, nil
}

func jsonValue(raw []byte) any {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return string(raw)
	}
	return value
}

func RedactBuiltRequest(request BuiltRequest) BuiltRequest {
	return BuiltRequest{
		Method:  request.Method,
		URL:     redactURL(request.URL),
		Headers: redactHeaders(request.Headers),
		Query:   redactQuery(request.Query),
		Body:    redactBody(request.Body),
	}
}

func redactedRequestSnapshot(request BuiltRequest) map[string]any {
	redacted := RedactBuiltRequest(request)
	return map[string]any{
		"method":  redacted.Method,
		"url":     redacted.URL,
		"headers": redacted.Headers,
		"query":   redacted.Query,
		"body":    jsonValue(redacted.Body),
	}
}

func redactHeaders(headers map[string]string) map[string]string {
	redacted := map[string]string{}
	for key, value := range headers {
		if sensitiveTokenField(key) {
			redacted[key] = "***"
			continue
		}
		if strings.EqualFold(key, "Authorization") && strings.TrimSpace(value) != "" {
			redacted[key] = "Bearer ***"
			continue
		}
		redacted[key] = value
	}
	return redacted
}

func redactQuery(query map[string]string) map[string]string {
	redacted := map[string]string{}
	for key, value := range query {
		if sensitiveTokenField(key) {
			redacted[key] = "***"
			continue
		}
		redacted[key] = value
	}
	return redacted
}

func redactBody(raw json.RawMessage) json.RawMessage {
	if len(bytes.TrimSpace(raw)) == 0 {
		return raw
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return raw
	}
	redacted, err := json.Marshal(redactSensitiveJSON(value, ""))
	if err != nil {
		return raw
	}
	return redacted
}

func redactSensitiveJSON(value any, key string) any {
	if sensitiveTokenField(key) {
		return "***"
	}
	switch typed := value.(type) {
	case map[string]any:
		redacted := make(map[string]any, len(typed))
		for itemKey, itemValue := range typed {
			redacted[itemKey] = redactSensitiveJSON(itemValue, itemKey)
		}
		return redacted
	case []any:
		redacted := make([]any, len(typed))
		for i, item := range typed {
			redacted[i] = redactSensitiveJSON(item, "")
		}
		return redacted
	default:
		return value
	}
}

func sensitiveTokenField(key string) bool {
	normalized := strings.ToLower(strings.TrimSpace(key))
	normalized = strings.NewReplacer("_", "", "-", "", ".", "").Replace(normalized)
	return strings.Contains(normalized, "token") ||
		strings.Contains(normalized, "secret") ||
		strings.Contains(normalized, "password") ||
		strings.Contains(normalized, "authorization")
}

func requestConfigFrom(channel Channel, input BuildRequestInput) (requestConfig, error) {
	var config requestConfig
	if err := decodeInto(channel.SendConfig, &config); err != nil {
		return requestConfig{}, err
	}
	if channel.ProviderType == ProviderWebhook || channel.ProviderType == ProviderServerChan || strings.TrimSpace(config.URL) == "" {
		defaultConfig, ok, err := builtInRequestConfig(channel, input)
		if err != nil {
			return requestConfig{}, err
		}
		if ok {
			config = defaultConfig
		}
	}
	tokenPlacement, err := decodePlacement(channel.TokenConfig, "token")
	if err != nil {
		return requestConfig{}, err
	}
	if tokenPlacement.Location != "" {
		config.Token = tokenPlacement
	}
	if strings.TrimSpace(input.Token) == "" {
		config.Token.Location = PlacementNone
	}
	if config.Token.Location == "" {
		config.Token.Location = PlacementNone
	}
	if config.Recipient.Location == "" {
		config.Recipient.Location = PlacementNone
	}
	return config, nil
}

func decodeInto(raw json.RawMessage, dest any) error {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, dest); err != nil {
		return ErrInvalidInput
	}
	return nil
}

func decodePlacement(raw json.RawMessage, field string) (placementConfig, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return placementConfig{}, nil
	}
	var wrapped map[string]json.RawMessage
	if err := json.Unmarshal(raw, &wrapped); err != nil {
		return placementConfig{}, ErrInvalidInput
	}
	var placement placementConfig
	if nested, ok := wrapped[field]; ok {
		if err := json.Unmarshal(nested, &placement); err != nil {
			return placementConfig{}, ErrInvalidInput
		}
		return placement, nil
	}
	if err := json.Unmarshal(raw, &placement); err != nil {
		return placementConfig{}, ErrInvalidInput
	}
	return placement, nil
}

func decodeBody(raw json.RawMessage) (map[string]any, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return map[string]any{}, nil
	}
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil, ErrInvalidInput
	}
	if body == nil {
		body = map[string]any{}
	}
	return body, nil
}

func applyPlacement(requestURL *string, headers map[string]string, query map[string]string, body map[string]any, placement placementConfig, value any) error {
	if isEmptyValue(value) {
		return nil
	}
	location := placement.Location
	if location == "" {
		location = PlacementNone
	}
	fieldName := strings.TrimSpace(placement.FieldName)
	switch location {
	case PlacementQuery:
		if fieldName == "" {
			return ErrInvalidInput
		}
		query[fieldName] = stringifyValue(value)
	case PlacementHeader:
		if fieldName == "" {
			return ErrInvalidInput
		}
		headers[fieldName] = stringifyValue(value)
	case PlacementBody:
		path := strings.TrimSpace(placement.Path)
		if path == "" {
			path = fieldName
		}
		if path == "" {
			return ErrInvalidInput
		}
		setNested(body, path, value)
	case PlacementPath:
		if fieldName == "" {
			return ErrInvalidInput
		}
		escaped := url.PathEscape(stringifyValue(value))
		*requestURL = strings.ReplaceAll(*requestURL, "{"+fieldName+"}", escaped)
	case PlacementNone:
		return nil
	default:
		return ErrInvalidInput
	}
	return nil
}

func setNested(body map[string]any, path string, value any) {
	parts := strings.Split(path, ".")
	current := body
	for i, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if i == len(parts)-1 {
			current[part] = value
			return
		}
		next, ok := current[part].(map[string]any)
		if !ok {
			next = map[string]any{}
			current[part] = next
		}
		current = next
	}
}

func formatRecipient(recipient any, format string) any {
	switch typed := recipient.(type) {
	case nil:
		return ""
	case string:
		if format == "array" {
			if strings.TrimSpace(typed) == "" {
				return []string{}
			}
			return []string{typed}
		}
		return typed
	case []string:
		switch format {
		case "array":
			return typed
		case "pipe_string":
			return strings.Join(typed, "|")
		case "comma_string":
			return strings.Join(typed, ",")
		default:
			return strings.Join(typed, ",")
		}
	case []any:
		values := make([]string, 0, len(typed))
		for _, item := range typed {
			values = append(values, fmt.Sprint(item))
		}
		switch format {
		case "array":
			return typed
		case "pipe_string":
			return strings.Join(values, "|")
		case "comma_string":
			return strings.Join(values, ",")
		default:
			return strings.Join(values, ",")
		}
	default:
		return fmt.Sprint(typed)
	}
}

func stringifyValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case []string:
		return strings.Join(typed, ",")
	case []any:
		values := make([]string, 0, len(typed))
		for _, item := range typed {
			values = append(values, fmt.Sprint(item))
		}
		return strings.Join(values, ",")
	default:
		return fmt.Sprint(typed)
	}
}

func isEmptyValue(value any) bool {
	switch typed := value.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(typed) == ""
	case []string:
		return len(typed) == 0
	case []any:
		return len(typed) == 0
	default:
		return false
	}
}

func (s *Service) autoResolveToken(ctx context.Context, channel Channel) (string, error) {
	capabilities := DefaultCapabilities()
	for _, capability := range capabilities {
		if capability.ProviderType == channel.ProviderType {
			resolution, err := s.tokenManager.Resolve(ctx, capability, channel, false)
			return resolution.Token, err
		}
	}
	return "", nil
}

func (s *Service) managedToken(ctx context.Context, channel Channel) (string, error) {
	if !RequiresTokenResolution(channel.ProviderType) {
		return "", nil
	}
	if token := credentialValue(channel.AuthConfig, "access_token"); token != "" {
		return token, nil
	}
	return s.autoResolveToken(ctx, channel)
}

func (s *Service) RefreshToken(ctx context.Context, id string) (TokenCacheStatus, error) {
	channel, err := s.GetChannel(ctx, id)
	if err != nil {
		return TokenCacheStatus{}, err
	}
	if !RequiresTokenResolution(channel.ProviderType) {
		return TokenCacheStatus{}, fmt.Errorf("%w: channel type does not require token resolution", ErrInvalidInput)
	}
	capabilities := DefaultCapabilities()
	for _, capability := range capabilities {
		if capability.ProviderType == channel.ProviderType {
			resolution, err := s.tokenManager.Resolve(ctx, capability, channel, true)
			if err != nil {
				return TokenCacheStatus{}, err
			}
			status := TokenCacheStatus{IsCached: true, Status: "cached"}
			if !resolution.ExpiresAt.IsZero() {
				status.ExpiresAt = resolution.ExpiresAt.Format(time.RFC3339)
			}
			current, err := s.tokenManager.Status(ctx, capability, channel)
			if err != nil {
				return status, nil
			}
			return current.withFallback(status), nil
		}
	}
	return TokenCacheStatus{}, fmt.Errorf("%w: no capability found for channel provider type", ErrInvalidInput)
}

func (s *Service) ResolveFeishuOpenID(ctx context.Context, channelID string, mobiles []string) (FeishuOpenIDResolveResult, error) {
	channel, err := s.GetChannel(ctx, channelID)
	if err != nil {
		return FeishuOpenIDResolveResult{}, err
	}
	if channel.ProviderType != ProviderFeishuRobot {
		return FeishuOpenIDResolveResult{}, fmt.Errorf("%w: channel is not feishu_robot", ErrInvalidInput)
	}
	cleanMobiles := cleanResolveMobiles(mobiles)
	if len(cleanMobiles) == 0 {
		return FeishuOpenIDResolveResult{}, fmt.Errorf("%w: mobiles is required", ErrInvalidInput)
	}
	if len(cleanMobiles) > 50 {
		return FeishuOpenIDResolveResult{}, fmt.Errorf("%w: mobiles supports at most 50 items", ErrInvalidInput)
	}
	capability := findDefaultCapability(channel.ProviderType, "text")
	if capability.ProviderType == "" {
		return FeishuOpenIDResolveResult{}, fmt.Errorf("%w: feishu capability not found", ErrNotFound)
	}
	resolver := feishuTenantAccessTokenResolver(channel)
	resolution, err := s.tokenManager.ResolveWithResolver(ctx, TokenResolveInput{
		Capability: capability,
		Channel:    channel,
		Resolver:   resolver,
		Strategy:   "tenant_access_token",
	})
	if err != nil {
		return FeishuOpenIDResolveResult{}, err
	}
	result, code, err := s.requestFeishuOpenID(ctx, channel, resolution.Token, cleanMobiles)
	if err != nil {
		return FeishuOpenIDResolveResult{}, err
	}
	if !feishuTokenRefreshCode(code) {
		return result, nil
	}
	_ = s.tokenManager.Invalidate(ctx, resolution.CacheKey, "feishu open id resolve token refresh code")
	refreshed, refreshErr := s.tokenManager.ResolveWithResolver(ctx, TokenResolveInput{
		Capability:   capability,
		Channel:      channel,
		Resolver:     resolver,
		Strategy:     "tenant_access_token",
		ForceRefresh: true,
	})
	if refreshErr != nil {
		return FeishuOpenIDResolveResult{}, refreshErr
	}
	result, _, err = s.requestFeishuOpenID(ctx, channel, refreshed.Token, cleanMobiles)
	return result, err
}

func (s *Service) ResolveDingTalkUserID(ctx context.Context, channelID string, queryWords []string) (DingTalkUserIDResolveResult, error) {
	channel, err := s.GetChannel(ctx, channelID)
	if err != nil {
		return DingTalkUserIDResolveResult{}, err
	}
	if channel.ProviderType != ProviderDingTalkWork {
		return DingTalkUserIDResolveResult{}, fmt.Errorf("%w: channel is not dingtalk_work", ErrInvalidInput)
	}
	cleanWords := cleanResolveWords(queryWords)
	if len(cleanWords) == 0 {
		return DingTalkUserIDResolveResult{}, fmt.Errorf("%w: query_words is required", ErrInvalidInput)
	}
	if len(cleanWords) > 50 {
		return DingTalkUserIDResolveResult{}, fmt.Errorf("%w: query_words supports at most 50 items", ErrInvalidInput)
	}
	capability := findDefaultCapability(channel.ProviderType, "sampleMarkdown")
	if capability.ProviderType == "" {
		return DingTalkUserIDResolveResult{}, fmt.Errorf("%w: dingtalk capability not found", ErrNotFound)
	}
	resolver := dingTalkAppAccessTokenResolver(channel)
	resolution, err := s.tokenManager.ResolveWithResolver(ctx, TokenResolveInput{
		Capability: capability,
		Channel:    channel,
		Resolver:   resolver,
		Strategy:   "app_access_token",
	})
	if err != nil {
		return DingTalkUserIDResolveResult{}, err
	}
	result, code, err := s.requestDingTalkUserID(ctx, channel, resolution.Token, cleanWords)
	if err != nil {
		return DingTalkUserIDResolveResult{}, err
	}
	if !dingTalkTokenRefreshCode(code) {
		return result, nil
	}
	_ = s.tokenManager.Invalidate(ctx, resolution.CacheKey, "dingtalk user id resolve token refresh code")
	refreshed, refreshErr := s.tokenManager.ResolveWithResolver(ctx, TokenResolveInput{
		Capability:   capability,
		Channel:      channel,
		Resolver:     resolver,
		Strategy:     "app_access_token",
		ForceRefresh: true,
	})
	if refreshErr != nil {
		return DingTalkUserIDResolveResult{}, refreshErr
	}
	result, _, err = s.requestDingTalkUserID(ctx, channel, refreshed.Token, cleanWords)
	return result, err
}

func (s *Service) requestDingTalkUserID(ctx context.Context, channel Channel, token string, queryWords []string) (DingTalkUserIDResolveResult, int, error) {
	baseURL := firstString(stringConfig(rawObject(channel.SendConfig), "base_url"), stringConfig(rawObject(channel.AuthConfig), "base_url"), "https://api.dingtalk.com")
	requestURL := joinURL(baseURL, "/v1.0/contact/users/search")
	timeout := time.Duration(channel.TimeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	client := s.httpClientFactory(timeout)
	result := DingTalkUserIDResolveResult{Success: true, Items: make([]DingTalkUserIDResolveItem, 0, len(queryWords))}
	for _, queryWord := range queryWords {
		body := map[string]any{
			"queryWord":      queryWord,
			"offset":         0,
			"size":           10,
			"fullMatchField": 1,
		}
		rawBody, err := json.Marshal(body)
		if err != nil {
			return DingTalkUserIDResolveResult{}, 0, err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, bytes.NewReader(rawBody))
		if err != nil {
			return DingTalkUserIDResolveResult{}, 0, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-acs-dingtalk-access-token", strings.TrimSpace(token))
		resp, err := client.Do(req)
		if err != nil {
			return DingTalkUserIDResolveResult{}, 0, err
		}
		responseBody, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			return DingTalkUserIDResolveResult{}, 0, readErr
		}
		if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
			return DingTalkUserIDResolveResult{}, 0, fmt.Errorf("dingtalk resolve user_id returned status %d", resp.StatusCode)
		}
		var decoded struct {
			Code       int      `json:"code"`
			ErrCode    int      `json:"errcode"`
			Message    string   `json:"message"`
			ErrMsg     string   `json:"errmsg"`
			HasMore    bool     `json:"hasMore"`
			TotalCount int      `json:"totalCount"`
			List       []string `json:"list"`
		}
		if err := json.Unmarshal(responseBody, &decoded); err != nil {
			return DingTalkUserIDResolveResult{}, 0, err
		}
		code := firstNonZero(decoded.Code, decoded.ErrCode)
		if code != 0 {
			result.Success = false
			result.Items = append(result.Items, DingTalkUserIDResolveItem{
				QueryWord: queryWord,
				Status:    "failed",
				Error:     firstString(decoded.Message, decoded.ErrMsg, "钉钉用户搜索失败"),
			})
			return result, code, nil
		}
		switch {
		case len(decoded.List) == 1:
			result.Items = append(result.Items, DingTalkUserIDResolveItem{
				QueryWord: queryWord,
				UserID:    decoded.List[0],
				Status:    "resolved",
			})
		case len(decoded.List) > 1 || decoded.TotalCount > 1:
			result.Success = false
			result.Items = append(result.Items, DingTalkUserIDResolveItem{
				QueryWord: queryWord,
				Status:    "multiple",
				Error:     "检测到多个用户，请重试或手动输入",
			})
		default:
			result.Success = false
			result.Items = append(result.Items, DingTalkUserIDResolveItem{
				QueryWord: queryWord,
				Status:    "failed",
				Error:     "未匹配到钉钉用户",
			})
		}
	}
	return result, 0, nil
}

func (s *Service) requestFeishuOpenID(ctx context.Context, channel Channel, token string, mobiles []string) (FeishuOpenIDResolveResult, int, error) {
	baseURL := firstString(stringConfig(rawObject(channel.SendConfig), "base_url"), stringConfig(rawObject(channel.AuthConfig), "base_url"), "https://open.feishu.cn/open-apis")
	requestURL := joinURL(baseURL, "/contact/v3/users/batch_get_id")
	parsed, err := url.Parse(requestURL)
	if err != nil {
		return FeishuOpenIDResolveResult{}, 0, ErrInvalidInput
	}
	values := parsed.Query()
	values.Set("user_id_type", "open_id")
	parsed.RawQuery = values.Encode()
	body := map[string]any{
		"mobiles":          mobiles,
		"include_resigned": false,
	}
	rawBody, err := json.Marshal(body)
	if err != nil {
		return FeishuOpenIDResolveResult{}, 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, parsed.String(), bytes.NewReader(rawBody))
	if err != nil {
		return FeishuOpenIDResolveResult{}, 0, err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(token))
	timeout := time.Duration(channel.TimeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	resp, err := s.httpClientFactory(timeout).Do(req)
	if err != nil {
		return FeishuOpenIDResolveResult{}, 0, err
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return FeishuOpenIDResolveResult{}, 0, err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return FeishuOpenIDResolveResult{}, 0, fmt.Errorf("feishu resolve open_id returned status %d", resp.StatusCode)
	}
	var decoded struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			UserList []struct {
				Mobile string `json:"mobile"`
				UserID string `json:"user_id"`
				OpenID string `json:"open_id"`
			} `json:"user_list"`
		} `json:"data"`
	}
	if err := json.Unmarshal(responseBody, &decoded); err != nil {
		return FeishuOpenIDResolveResult{}, 0, err
	}
	if decoded.Code != 0 {
		return FeishuOpenIDResolveResult{Success: false, Errors: []string{decoded.Msg}}, decoded.Code, nil
	}
	itemsByMobile := map[string]FeishuOpenIDResolveItem{}
	for _, item := range decoded.Data.UserList {
		openID := firstString(item.UserID, item.OpenID)
		if strings.TrimSpace(item.Mobile) == "" || strings.TrimSpace(openID) == "" {
			continue
		}
		itemsByMobile[item.Mobile] = FeishuOpenIDResolveItem{
			Mobile: item.Mobile,
			OpenID: openID,
			Status: "resolved",
		}
	}
	result := FeishuOpenIDResolveResult{Success: true, Items: make([]FeishuOpenIDResolveItem, 0, len(mobiles))}
	for _, mobile := range mobiles {
		if item, ok := itemsByMobile[mobile]; ok {
			result.Items = append(result.Items, item)
			continue
		}
		result.Success = false
		result.Items = append(result.Items, FeishuOpenIDResolveItem{
			Mobile: mobile,
			Status: "failed",
			Error:  "手机号未匹配到飞书用户",
		})
	}
	return result, decoded.Code, nil
}

func dingTalkAppAccessTokenResolver(channel Channel) TokenResolverConfig {
	auth := rawObject(channel.AuthConfig)
	tokenURL := firstString(stringConfig(rawObject(channel.TokenConfig), "token_url"), stringConfig(auth, "token_url"))
	if tokenURL == "" {
		baseURL := firstString(stringConfig(auth, "token_base_url"), "https://api.dingtalk.com")
		tokenURL = joinURL(baseURL, "/v1.0/oauth2/"+url.PathEscape(credentialValue(channel.AuthConfig, "corp_id"))+"/token")
	}
	body := mustJSON(map[string]any{
		"client_id":     credentialValue(channel.AuthConfig, "client_id"),
		"client_secret": credentialValue(channel.AuthConfig, "client_secret"),
		"grant_type":    "client_credentials",
	})
	return TokenResolverConfig{
		Request: TokenRequestConfig{
			Method: http.MethodPost,
			URL:    tokenURL,
			Body:   body,
		},
		ResponsePath:  "accessToken|access_token",
		ExpiresInPath: "expireIn|expires_in",
		Placement:     rawJSON(`{"location":"header","field_name":"x-acs-dingtalk-access-token"}`),
		Cacheable:     true,
		RefreshCodes:  []any{40001, 42001},
	}
}

func feishuTenantAccessTokenResolver(channel Channel) TokenResolverConfig {
	baseURL := firstString(stringConfig(rawObject(channel.SendConfig), "base_url"), stringConfig(rawObject(channel.AuthConfig), "base_url"), "https://open.feishu.cn/open-apis")
	return TokenResolverConfig{
		Request: TokenRequestConfig{
			Method: http.MethodPost,
			URL:    joinURL(baseURL, "/auth/v3/tenant_access_token/internal"),
			Body: mustJSON(map[string]any{
				"app_id":     credentialValue(channel.AuthConfig, "app_id"),
				"app_secret": credentialValue(channel.AuthConfig, "app_secret"),
			}),
		},
		ResponsePath:  "tenant_access_token",
		ExpiresInPath: "expire",
		Placement:     rawJSON(`{"location":"header","field_name":"Authorization","prefix":"Bearer "}`),
		Cacheable:     true,
		RefreshCodes:  []any{99991663, 99991664, 99991665},
	}
}

func cleanResolveMobiles(values []string) []string {
	return cleanResolveWords(values)
}

func cleanResolveWords(values []string) []string {
	seen := map[string]bool{}
	cleaned := make([]string, 0, len(values))
	for _, value := range values {
		word := strings.TrimSpace(value)
		if word == "" || seen[word] {
			continue
		}
		seen[word] = true
		cleaned = append(cleaned, word)
	}
	return cleaned
}

func feishuTokenRefreshCode(code int) bool {
	switch code {
	case 99991663, 99991664, 99991665:
		return true
	default:
		return false
	}
}

func dingTalkTokenRefreshCode(code int) bool {
	switch code {
	case 40001, 42001:
		return true
	default:
		return false
	}
}

func firstNonZero(values ...int) int {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}

func findDefaultCapability(providerType ProviderType, messageType string) Capability {
	for _, capability := range DefaultCapabilities() {
		if capability.ProviderType == providerType && capability.MessageType == messageType {
			return capability
		}
	}
	return Capability{}
}

func (s *Service) withTokenStatus(ctx context.Context, channel Channel) Channel {
	if s == nil || s.tokenManager == nil || !RequiresTokenResolution(channel.ProviderType) {
		return channel
	}
	for _, capability := range DefaultCapabilities() {
		if capability.ProviderType != channel.ProviderType {
			continue
		}
		status, err := s.tokenManager.Status(ctx, capability, channel)
		if err != nil {
			return channel
		}
		channel.IsCached = status.IsCached
		channel.TokenCacheStatus = status.Status
		channel.TokenRefreshedAt = status.TokenRefreshed
		channel.TokenExpiresAt = status.ExpiresAt
		channel.TokenLastError = status.LastError
		return channel
	}
	return channel
}

func (status TokenCacheStatus) withFallback(fallback TokenCacheStatus) TokenCacheStatus {
	if status.IsCached {
		return status
	}
	if fallback.IsCached {
		return fallback
	}
	if strings.TrimSpace(status.Status) == "" {
		status.Status = fallback.Status
	}
	return status
}
