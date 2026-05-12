package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

type ProviderType string

const (
	ProviderWeCom         ProviderType = "wecom"
	ProviderWeComApp      ProviderType = "wecom_app"
	ProviderWeComRobot    ProviderType = "wecom_robot"
	ProviderFeishu        ProviderType = "feishu"
	ProviderFeishuRobot   ProviderType = "feishu_robot"
	ProviderDingTalk      ProviderType = "dingtalk"
	ProviderDingTalkWork  ProviderType = "dingtalk_work"
	ProviderDingTalkRobot ProviderType = "dingtalk_robot"
	ProviderEmail         ProviderType = "email"
	ProviderSMS           ProviderType = "sms"
	ProviderAliyunSMS     ProviderType = "aliyun_sms"
	ProviderTencentSMS    ProviderType = "tencent_sms"
	ProviderBaiduSMS      ProviderType = "baidu_sms"
	ProviderGovCloud      ProviderType = "gov_cloud"
	ProviderSelf          ProviderType = "self"
	ProviderWebhook       ProviderType = "webhook"
	ProviderCustomToken   ProviderType = "custom_token"
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
}

type CreateChannelInput struct {
	ProviderType     ProviderType    `json:"provider_type"`
	Name             string          `json:"name"`
	Enabled          bool            `json:"enabled"`
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
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
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
	return s.store.ListChannels(ctx)
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
	return s.store.GetChannel(ctx, id)
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
	return BuildRequest(channel, input)
}

func (s *Service) BuildDeliveryRequest(ctx context.Context, channelID string, input BuildDeliveryRequestInput) (BuiltRequest, error) {
	channel, err := s.GetChannel(ctx, channelID)
	if err != nil {
		return BuiltRequest{}, err
	}
	return BuildDeliveryRequest(channel, input)
}

func (s *Service) TestSend(ctx context.Context, channelID string, input TestSendInput) (TestSendResult, error) {
	channel, err := s.GetChannel(ctx, channelID)
	if err != nil {
		return TestSendResult{}, err
	}
	built, err := BuildRequest(channel, input.BuildRequestInput)
	if err != nil {
		return TestSendResult{}, err
	}
	result := TestSendResult{
		Status:  "built",
		Request: built,
	}
	requestSnapshot, err := marshalJSON(map[string]any{
		"method":    built.Method,
		"url":       built.URL,
		"headers":   built.Headers,
		"query":     built.Query,
		"recipient": input.Recipient,
		"body":      jsonValue(built.Body),
	})
	if err != nil {
		return TestSendResult{}, err
	}
	result.RequestSnapshot = requestSnapshot
	if !input.Send {
		return result, nil
	}
	if strings.TrimSpace(built.URL) == "" {
		return TestSendResult{}, ErrInvalidInput
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
	return rawJSON(`{"type":"object","required":["subject","html"],"properties":{"subject":{"type":"string","title":"Subject","default":"{{ payload.title }}"},"html":{"type":"string","title":"HTML body","default":"{{ payload.content }}"},"text":{"type":"string","title":"Plain text body"}}}`)
}

func smsContentSchema() json.RawMessage {
	return rawJSON(`{"type":"object","required":["content"],"properties":{"content":{"type":"string","title":"SMS content","default":"{{ payload.title }}"},"template_params":{"type":"object","title":"Template parameters","additionalProperties":true}}}`)
}

func webhookContentSchema() json.RawMessage {
	return rawJSON(`{"type":"object","properties":{"payload":{"type":"object","title":"Payload","additionalProperties":true},"headers":{"type":"object","additionalProperties":{"type":"string"}}},"additionalProperties":true}`)
}

func customTokenContentSchema() json.RawMessage {
	return rawJSON(`{"type":"object","required":["message"],"properties":{"message":{"type":"string","title":"Message","default":"{{ payload.title }}"},"payload":{"type":"object","additionalProperties":true}},"additionalProperties":true}`)
}

func normalizeChannelInput(input CreateChannelInput) (CreateChannelParams, error) {
	input.Name = strings.TrimSpace(input.Name)
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
	case ProviderWeCom,
		ProviderWeComApp,
		ProviderWeComRobot,
		ProviderFeishu,
		ProviderFeishuRobot,
		ProviderDingTalk,
		ProviderDingTalkWork,
		ProviderDingTalkRobot,
		ProviderEmail,
		ProviderSMS,
		ProviderAliyunSMS,
		ProviderTencentSMS,
		ProviderBaiduSMS,
		ProviderGovCloud,
		ProviderSelf,
		ProviderWebhook,
		ProviderCustomToken,
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
	Send bool `json:"send"`
}

type BuiltRequest struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
	Query   map[string]string `json:"query"`
	Body    json.RawMessage   `json:"body"`
}

type TestSendResult struct {
	Status           string          `json:"status"`
	Request          BuiltRequest    `json:"request"`
	RequestSnapshot  json.RawMessage `json:"request_snapshot"`
	ResponseSnapshot json.RawMessage `json:"response_snapshot"`
	StatusCode       int             `json:"status_code"`
	DurationMS       int             `json:"duration_ms"`
	ErrorMessage     string          `json:"error_message"`
}

type requestConfig struct {
	Method            string            `json:"method"`
	URL               string            `json:"url"`
	Headers           map[string]string `json:"headers"`
	Body              json.RawMessage   `json:"body"`
	Token             placementConfig   `json:"token"`
	Recipient         placementConfig   `json:"recipient"`
	SkipRenderedMerge bool              `json:"-"`
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
	if recipientValue == nil {
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

	body, err := json.Marshal(bodyMap)
	if err != nil {
		return BuiltRequest{}, ErrInvalidInput
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
	for _, key := range []string{"wecom_userid", "feishu_open_id", "feishu_user_id", "dingtalk_userid", "wxpusher_uid", "bark_device_key", "gov_userid", "gov_party_id", "gov_tag_id", "userid", "open_id"} {
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
	case ProviderSMS, ProviderAliyunSMS, ProviderTencentSMS, ProviderBaiduSMS, ProviderDingTalkRobot:
		return recipient.Mobile
	case ProviderGovCloud:
		return firstString(recipient.PlatformIDs["gov_userid"], recipient.Mobile)
	case ProviderSelf:
		return recipient.SystemUserID
	default:
		return firstString(recipient.SystemUserID, recipient.Mobile, recipient.Email)
	}
}

func providerIdentityKeys(providerType ProviderType) []string {
	switch providerType {
	case ProviderWeCom, ProviderWeComApp, ProviderWeComRobot:
		return []string{"wecom_userid", "userid"}
	case ProviderFeishu, ProviderFeishuRobot:
		return []string{"feishu_open_id", "feishu_user_id", "open_id", "user_id"}
	case ProviderDingTalk, ProviderDingTalkWork:
		return []string{"dingtalk_userid", "userid", "user_id"}
	case ProviderWxPusher:
		return []string{"wxpusher_uid", "uid"}
	case ProviderBark:
		return []string{"bark_device_key", "device_key"}
	case ProviderGovCloud:
		return []string{"gov_userid", "userid", "user_id"}
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
	body := built.Body
	if len(bytes.TrimSpace(body)) == 0 {
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
	client := &http.Client{Timeout: timeout}
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

func requestConfigFrom(channel Channel, input BuildRequestInput) (requestConfig, error) {
	var config requestConfig
	if err := decodeInto(channel.SendConfig, &config); err != nil {
		return requestConfig{}, err
	}
	if strings.TrimSpace(config.URL) == "" {
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
