package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

type ProviderType string

const (
	ProviderWeCom       ProviderType = "wecom"
	ProviderFeishu      ProviderType = "feishu"
	ProviderDingTalk    ProviderType = "dingtalk"
	ProviderEmail       ProviderType = "email"
	ProviderSMS         ProviderType = "sms"
	ProviderGovCloud    ProviderType = "gov_cloud"
	ProviderSelf        ProviderType = "self"
	ProviderWebhook     ProviderType = "webhook"
	ProviderCustomToken ProviderType = "custom_token"
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
	ID                 string
	ProviderType       ProviderType
	MessageType        string
	MessageSchema      json.RawMessage
	RecipientRequired  bool
	AllowNoRecipient   bool
	RecipientFieldName string
	RecipientLocation  Placement
	RecipientPath      string
	RecipientFormat    string
	IdentityKind       string
	TokenLocation      Placement
	TokenFieldName     string
	RequestExamples    json.RawMessage
	CreatedAt          time.Time
	UpdatedAt          time.Time
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

func DefaultCapabilities() []Capability {
	return []Capability{
		capability(ProviderWeCom, "text", true, false, "touser", PlacementBody, "touser", "pipe_string", "wecom_userid", PlacementQuery, "access_token", `{"text":{"content":"{{ payload.title }}"}}`),
		capability(ProviderFeishu, "text", true, false, "receive_id", PlacementBody, "receive_id", "string", "feishu_open_id", PlacementHeader, "Authorization", `{"msg_type":"text","content":{"text":"{{ payload.title }}"}}`),
		capability(ProviderDingTalk, "text", true, false, "userid_list", PlacementBody, "userid_list", "comma_string", "dingtalk_userid", PlacementQuery, "access_token", `{"msgtype":"text","text":{"content":"{{ payload.title }}"}}`),
		capability(ProviderEmail, "text", true, false, "to", PlacementBody, "to", "array", "email", PlacementNone, "", `{"subject":"{{ payload.title }}","html":"{{ payload.content }}"}`),
		capability(ProviderSMS, "text", true, false, "phones", PlacementBody, "phones", "array", "mobile", PlacementNone, "", `{"content":"{{ payload.title }}"}`),
		capability(ProviderGovCloud, "text", true, false, "touser", PlacementBody, "touser", "string", "mobile", PlacementHeader, "Authorization", `{"title":"{{ payload.title }}","content":"{{ payload.content }}"}`),
		capability(ProviderSelf, "text", true, false, "user_id", PlacementBody, "user_id", "string", "system_user_id", PlacementHeader, "Authorization", `{"title":"{{ payload.title }}","body":"{{ payload.content }}"}`),
		capability(ProviderWebhook, "json", false, true, "", PlacementNone, "", "string", "", PlacementNone, "", `{"payload":"{{ payload }}"}`),
		capability(ProviderCustomToken, "json", true, true, "recipient", PlacementBody, "recipient", "string", "custom", PlacementHeader, "Authorization", `{"message":"{{ payload.title }}"}`),
	}
}

func capability(providerType ProviderType, messageType string, recipientRequired bool, allowNoRecipient bool, recipientFieldName string, recipientLocation Placement, recipientPath string, recipientFormat string, identityKind string, tokenLocation Placement, tokenFieldName string, example string) Capability {
	return Capability{
		ID:                 uuid.NewString(),
		ProviderType:       providerType,
		MessageType:        messageType,
		MessageSchema:      json.RawMessage(`{"type":"object"}`),
		RecipientRequired:  recipientRequired,
		AllowNoRecipient:   allowNoRecipient,
		RecipientFieldName: recipientFieldName,
		RecipientLocation:  recipientLocation,
		RecipientPath:      recipientPath,
		RecipientFormat:    recipientFormat,
		IdentityKind:       identityKind,
		TokenLocation:      tokenLocation,
		TokenFieldName:     tokenFieldName,
		RequestExamples:    json.RawMessage(example),
	}
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
	case ProviderWeCom, ProviderFeishu, ProviderDingTalk, ProviderEmail, ProviderSMS, ProviderGovCloud, ProviderSelf, ProviderWebhook, ProviderCustomToken:
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

type BuiltRequest struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
	Query   map[string]string `json:"query"`
	Body    json.RawMessage   `json:"body"`
}

type requestConfig struct {
	Method    string            `json:"method"`
	URL       string            `json:"url"`
	Headers   map[string]string `json:"headers"`
	Body      json.RawMessage   `json:"body"`
	Token     placementConfig   `json:"token"`
	Recipient placementConfig   `json:"recipient"`
}

type placementConfig struct {
	Location  Placement `json:"location"`
	FieldName string    `json:"field_name"`
	Path      string    `json:"path"`
	Prefix    string    `json:"prefix"`
	Format    string    `json:"format"`
}

func BuildRequest(channel Channel, input BuildRequestInput) (BuiltRequest, error) {
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
	if len(bytes.TrimSpace(input.Body)) > 0 {
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

func requestConfigFrom(channel Channel, input BuildRequestInput) (requestConfig, error) {
	var config requestConfig
	if err := decodeInto(channel.SendConfig, &config); err != nil {
		return requestConfig{}, err
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
