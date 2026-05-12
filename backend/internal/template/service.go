package msgtemplate

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/flosch/pongo2/v6"

	"mvp-push-gateway/backend/internal/provider"
)

var (
	ErrNotFound        = errors.New("template not found")
	ErrInvalidInput    = errors.New("invalid template input")
	ErrInvalidTemplate = errors.New("invalid template")
)

type Template struct {
	ID               string
	Name             string
	Description      string
	SourceID         string
	Enabled          bool
	CurrentVersionID string
	CurrentVersion   *TemplateVersion
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type TemplateVersion struct {
	ID                    string
	TemplateID            string
	VersionNo             int
	MessageType           string
	TargetProviderType    string
	TemplateEngine        string
	TemplateSyntaxVersion string
	TemplateBody          string
	MessageBodySchema     json.RawMessage
	SamplePayload         json.RawMessage
	CompiledPreview       json.RawMessage
	UsedVariables         []string
	AllowedFilters        []string
	ValidationStatus      string
	ValidationErrors      json.RawMessage
	PublishedAt           *time.Time
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

type TemplateInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	SourceID    string `json:"source_id"`
	Enabled     bool   `json:"enabled"`
}

type VersionInput struct {
	MessageType        string          `json:"message_type"`
	TargetProviderType string          `json:"target_provider_type"`
	TemplateBody       string          `json:"template_body"`
	MessageBodySchema  json.RawMessage `json:"message_body_schema"`
	SamplePayload      json.RawMessage `json:"sample_payload"`
}

type VariableRef struct {
	Variable string `json:"variable"`
	Path     string `json:"path"`
}

type ValidationError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Path    string `json:"path,omitempty"`
}

type ValidationResult struct {
	Status    string            `json:"status"`
	Variables []VariableRef     `json:"variables"`
	Preview   string            `json:"preview"`
	Errors    []ValidationError `json:"errors"`
}

type CreateTemplateParams = TemplateInput
type UpdateTemplateParams = TemplateInput

type PublishTemplateVersionParams struct {
	VersionInput
	CompiledPreview  json.RawMessage
	UsedVariables    []string
	ValidationStatus string
	ValidationErrors json.RawMessage
}

type Store interface {
	ListTemplates(ctx context.Context) ([]Template, error)
	CreateTemplate(ctx context.Context, params CreateTemplateParams) (Template, error)
	GetTemplate(ctx context.Context, id string) (Template, error)
	UpdateTemplate(ctx context.Context, id string, params UpdateTemplateParams) (Template, error)
	DeleteTemplate(ctx context.Context, id string) error
	PublishTemplateVersion(ctx context.Context, templateID string, params PublishTemplateVersionParams) (TemplateVersion, error)
}

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

func (s *Service) ListTemplates(ctx context.Context) ([]Template, error) {
	return s.store.ListTemplates(ctx)
}

func (s *Service) CreateTemplate(ctx context.Context, input TemplateInput) (Template, error) {
	params, err := normalizeTemplateInput(input)
	if err != nil {
		return Template{}, err
	}
	return s.store.CreateTemplate(ctx, params)
}

func (s *Service) GetTemplate(ctx context.Context, id string) (Template, error) {
	if strings.TrimSpace(id) == "" {
		return Template{}, ErrInvalidInput
	}
	return s.store.GetTemplate(ctx, id)
}

func (s *Service) UpdateTemplate(ctx context.Context, id string, input TemplateInput) (Template, error) {
	if strings.TrimSpace(id) == "" {
		return Template{}, ErrInvalidInput
	}
	params, err := normalizeTemplateInput(input)
	if err != nil {
		return Template{}, err
	}
	return s.store.UpdateTemplate(ctx, id, params)
}

func (s *Service) DeleteTemplate(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return ErrInvalidInput
	}
	return s.store.DeleteTemplate(ctx, id)
}

func (s *Service) Parse(input VersionInput) (ValidationResult, error) {
	input = normalizeVersionInput(input)
	result := ValidationResult{Status: "valid", Variables: ParseVariables(input.TemplateBody)}
	addRequiredVersionFieldErrors(&result, input)
	addRecipientFieldErrors(&result, input.TemplateBody)
	if result.Status != "valid" {
		return result, ErrInvalidTemplate
	}
	if _, err := parsePongoTemplate(input.TemplateBody); err != nil {
		result.Status = "invalid"
		result.Errors = append(result.Errors, ValidationError{
			Code:    "MGP-TPL-001",
			Message: err.Error(),
		})
		return result, ErrInvalidTemplate
	}
	return result, nil
}

func (s *Service) Preview(input VersionInput) (ValidationResult, error) {
	result := s.Validate(input)
	if result.Status != "valid" {
		return result, ErrInvalidTemplate
	}
	return result, nil
}

func (s *Service) Validate(input VersionInput) ValidationResult {
	input = normalizeVersionInput(input)
	result := ValidationResult{
		Status:    "valid",
		Variables: ParseVariables(input.TemplateBody),
	}
	addRequiredVersionFieldErrors(&result, input)
	addRecipientFieldErrors(&result, input.TemplateBody)
	if result.Status != "valid" {
		return result
	}

	tpl, err := parsePongoTemplate(input.TemplateBody)
	if err != nil {
		result.Status = "invalid"
		result.Errors = append(result.Errors, ValidationError{Code: "MGP-TPL-001", Message: err.Error()})
		return result
	}

	payloadMap, err := decodeJSONObject(input.SamplePayload)
	if err != nil {
		result.Status = "invalid"
		result.Errors = append(result.Errors, ValidationError{Code: "MGP-TPL-002", Message: "sample_payload 必须是 JSON 对象"})
		return result
	}

	defaultedPaths := defaultedPayloadPaths(input.TemplateBody)
	for _, variable := range result.Variables {
		if defaultedPaths[variable.Path] {
			continue
		}
		if !hasPayloadPath(payloadMap, variable.Path) {
			result.Status = "invalid"
			result.Errors = append(result.Errors, ValidationError{
				Code:    "MGP-TPL-003",
				Message: "模板变量在 sample_payload 中不存在",
				Path:    variable.Path,
			})
		}
	}
	schema, schemaFound, err := effectiveMessageSchema(input)
	if err != nil {
		result.Status = "invalid"
		result.Errors = append(result.Errors, ValidationError{
			Code:    "MGP-TPL-SCHEMA",
			Message: err.Error(),
			Path:    "message_body_schema",
		})
		return result
	}
	for _, required := range requiredPayloadFields(schema) {
		if !hasPayloadPath(payloadMap, required) {
			result.Status = "invalid"
			result.Errors = append(result.Errors, ValidationError{
				Code:    "MGP-TPL-004",
				Message: "消息体 schema 需要的 payload 字段不存在",
				Path:    required,
			})
		}
	}
	if schemaFound {
		if err := validateTemplateBodySchema(&result, input.TemplateBody, schema); err != nil {
			result.Status = "invalid"
			result.Errors = append(result.Errors, ValidationError{
				Code:    "MGP-TPL-SCHEMA",
				Message: err.Error(),
				Path:    "message_body_schema",
			})
		}
	}
	if result.Status != "valid" {
		return result
	}

	preview, err := tpl.Execute(pongo2.Context{"payload": payloadMap})
	if err != nil {
		result.Status = "invalid"
		result.Errors = append(result.Errors, ValidationError{Code: "MGP-TPL-005", Message: err.Error()})
		return result
	}
	result.Preview = preview
	return result
}

func (s *Service) Publish(ctx context.Context, templateID string, input VersionInput) (TemplateVersion, error) {
	if strings.TrimSpace(templateID) == "" {
		return TemplateVersion{}, ErrInvalidInput
	}
	input = normalizeVersionInput(input)
	if strings.TrimSpace(input.TemplateBody) == "" {
		return TemplateVersion{}, ErrInvalidInput
	}
	result := s.Validate(input)
	if result.Status != "valid" {
		return TemplateVersion{}, ErrInvalidTemplate
	}
	previewJSON, _ := json.Marshal(map[string]string{"rendered": result.Preview})
	errorsJSON, _ := json.Marshal(result.Errors)
	return s.store.PublishTemplateVersion(ctx, templateID, PublishTemplateVersionParams{
		VersionInput:     input,
		CompiledPreview:  previewJSON,
		UsedVariables:    variablePaths(result.Variables),
		ValidationStatus: "valid",
		ValidationErrors: errorsJSON,
	})
}

var payloadVariablePattern = regexp.MustCompile(`\{\{\s*([^{}]+?)\s*\}\}`)
var payloadPathPattern = regexp.MustCompile(`\bpayload(?:\.[A-Za-z_][A-Za-z0-9_]*)+\b`)
var defaultFilterPattern = regexp.MustCompile(`(?i)\|\s*default\s*(?:[:(])`)
var defaultFilterSingleQuotePattern = regexp.MustCompile(`(?i)\|\s*default\(\s*'((?:\\.|[^\\'])*)'\s*\)`)
var defaultFilterDoubleQuotePattern = regexp.MustCompile(`(?i)\|\s*default\(\s*"((?:\\.|[^\\"])*)"\s*\)`)
var recipientFieldPattern = regexp.MustCompile(`(?i)["']?(touser|toparty|totag|mobile|phone|email|open_id|openid|userid|user_id|dingtalk_userid|wecom_userid|feishu_open_id|recipient|recipients)["']?\s*:`)

func ParseVariables(templateBody string) []VariableRef {
	seen := map[string]bool{}
	var variables []VariableRef
	for _, match := range payloadVariablePattern.FindAllStringSubmatch(templateBody, -1) {
		if len(match) < 2 {
			continue
		}
		expr := strings.Split(match[1], "|")[0]
		path := payloadPathPattern.FindString(expr)
		if path == "" || seen[path] {
			continue
		}
		seen[path] = true
		variables = append(variables, VariableRef{
			Variable: "{{ " + path + " }}",
			Path:     path,
		})
	}
	sort.Slice(variables, func(i, j int) bool {
		return variables[i].Path < variables[j].Path
	})
	return variables
}

func addRequiredVersionFieldErrors(result *ValidationResult, input VersionInput) {
	if input.TargetProviderType == "" {
		appendValidationError(result, ValidationError{
			Code:    "MGP-TPL-REQUIRED",
			Message: "target_provider_type 不能为空",
			Path:    "target_provider_type",
		})
	}
	if input.MessageType == "" {
		appendValidationError(result, ValidationError{
			Code:    "MGP-TPL-REQUIRED",
			Message: "message_type 不能为空",
			Path:    "message_type",
		})
	}
	if input.TemplateBody == "" {
		appendValidationError(result, ValidationError{
			Code:    "MGP-TPL-REQUIRED",
			Message: "template_body 不能为空",
			Path:    "template_body",
		})
	}
}

func addRecipientFieldErrors(result *ValidationResult, templateBody string) {
	for _, path := range recipientFieldPaths(templateBody) {
		appendValidationError(result, ValidationError{
			Code:    "MGP-TPL-RECIPIENT",
			Message: "模板内容不能包含接收人字段，接收人应由路由和平台适配器处理",
			Path:    path,
		})
	}
}

func appendValidationError(result *ValidationResult, err ValidationError) {
	result.Status = "invalid"
	result.Errors = append(result.Errors, err)
}

func recipientFieldPaths(templateBody string) []string {
	found := map[string]bool{}
	if object, ok := decodeTemplateBodyObject(templateBody); ok {
		collectRecipientFieldPaths("", object, found)
	}
	for _, match := range recipientFieldPattern.FindAllStringSubmatch(templateBody, -1) {
		if len(match) >= 2 {
			found[strings.ToLower(match[1])] = true
		}
	}
	paths := make([]string, 0, len(found))
	for path := range found {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

func collectRecipientFieldPaths(prefix string, value any, found map[string]bool) {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			path := key
			if prefix != "" {
				path = prefix + "." + key
			}
			if isRecipientFieldName(key) {
				found[path] = true
			}
			collectRecipientFieldPaths(path, child, found)
		}
	case []any:
		for _, child := range typed {
			collectRecipientFieldPaths(prefix, child, found)
		}
	}
}

func isRecipientFieldName(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "touser",
		"toparty",
		"totag",
		"mobile",
		"phone",
		"email",
		"open_id",
		"openid",
		"userid",
		"user_id",
		"dingtalk_userid",
		"wecom_userid",
		"feishu_open_id",
		"recipient",
		"recipients":
		return true
	default:
		return false
	}
}

func parsePongoTemplate(templateBody string) (*pongo2.Template, error) {
	return pongo2.FromString(normalizeDefaultFilterSyntax(templateBody))
}

func normalizeDefaultFilterSyntax(templateBody string) string {
	templateBody = defaultFilterSingleQuotePattern.ReplaceAllStringFunc(templateBody, func(match string) string {
		parts := defaultFilterSingleQuotePattern.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		return defaultFilterReplacement(parts[1])
	})
	return defaultFilterDoubleQuotePattern.ReplaceAllStringFunc(templateBody, func(match string) string {
		parts := defaultFilterDoubleQuotePattern.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		return defaultFilterReplacement(parts[1])
	})
}

func defaultFilterReplacement(value string) string {
	value = strings.ReplaceAll(value, `"`, `\"`)
	return `| default:"` + value + `"`
}

func defaultedPayloadPaths(templateBody string) map[string]bool {
	paths := map[string]bool{}
	for _, match := range payloadVariablePattern.FindAllStringSubmatch(templateBody, -1) {
		if len(match) < 2 || !defaultFilterPattern.MatchString(match[1]) {
			continue
		}
		if path := payloadPathPattern.FindString(match[1]); path != "" {
			paths[path] = true
		}
	}
	return paths
}

func effectiveMessageSchema(input VersionInput) (json.RawMessage, bool, error) {
	if hasExplicitMessageSchema(input.MessageBodySchema) {
		if !json.Valid(input.MessageBodySchema) {
			return nil, true, fmt.Errorf("message_body_schema 必须是合法 JSON")
		}
		return append(json.RawMessage(nil), input.MessageBodySchema...), true, nil
	}
	for _, capability := range provider.DefaultCapabilities() {
		if string(capability.ProviderType) == input.TargetProviderType && capability.MessageType == input.MessageType {
			return append(json.RawMessage(nil), capability.MessageSchema...), true, nil
		}
	}
	return nil, false, nil
}

func hasExplicitMessageSchema(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte(`{}`)) || bytes.Equal(trimmed, []byte(`null`)) {
		return false
	}
	return true
}

func validateTemplateBodySchema(result *ValidationResult, templateBody string, schema json.RawMessage) error {
	required, expectsObject, err := requiredContentFields(schema)
	if err != nil {
		return err
	}
	object, parsed := decodeTemplateBodyJSONObject(templateBody)
	if !parsed {
		return nil
	}
	if object == nil {
		if expectsObject {
			appendValidationError(result, ValidationError{
				Code:    "MGP-TPL-SCHEMA",
				Message: "template_body 必须是 JSON 对象",
				Path:    "template_body",
			})
		}
		return nil
	}
	for _, field := range required {
		if _, ok := object[field]; !ok {
			appendValidationError(result, ValidationError{
				Code:    "MGP-TPL-REQUIRED",
				Message: "模板内容缺少消息 schema 必填字段",
				Path:    field,
			})
		}
	}
	return nil
}

func requiredContentFields(raw json.RawMessage) ([]string, bool, error) {
	var schema struct {
		Type       string                     `json:"type"`
		Required   []string                   `json:"required"`
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, false, nil
	}
	if err := json.Unmarshal(raw, &schema); err != nil {
		return nil, false, err
	}
	fields := make([]string, 0, len(schema.Required))
	for _, field := range schema.Required {
		field = strings.TrimSpace(field)
		if field != "" {
			fields = append(fields, field)
		}
	}
	expectsObject := schema.Type == "object" || len(schema.Properties) > 0 || len(fields) > 0
	return fields, expectsObject, nil
}

func normalizeTemplateInput(input TemplateInput) (CreateTemplateParams, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	input.SourceID = strings.TrimSpace(input.SourceID)
	if input.Name == "" {
		return CreateTemplateParams{}, ErrInvalidInput
	}
	return input, nil
}

func normalizeVersionInput(input VersionInput) VersionInput {
	input.MessageType = strings.TrimSpace(input.MessageType)
	input.TargetProviderType = strings.TrimSpace(input.TargetProviderType)
	input.TemplateBody = strings.TrimSpace(input.TemplateBody)
	input.MessageBodySchema = normalizeJSON(input.MessageBodySchema)
	input.SamplePayload = normalizeJSON(input.SamplePayload)
	return input
}

func normalizeJSON(raw json.RawMessage) json.RawMessage {
	if len(bytes.TrimSpace(raw)) == 0 {
		return json.RawMessage(`{}`)
	}
	return append(json.RawMessage(nil), bytes.TrimSpace(raw)...)
}

func decodeJSONObject(raw json.RawMessage) (map[string]any, error) {
	var payload map[string]any
	if err := json.Unmarshal(normalizeJSON(raw), &payload); err != nil {
		return nil, err
	}
	if payload == nil {
		payload = map[string]any{}
	}
	return payload, nil
}

func decodeTemplateBodyObject(templateBody string) (map[string]any, bool) {
	object, parsed := decodeTemplateBodyJSONObject(templateBody)
	return object, parsed && object != nil
}

func decodeTemplateBodyJSONObject(templateBody string) (map[string]any, bool) {
	trimmed := strings.TrimSpace(templateBody)
	if !strings.HasPrefix(trimmed, "{") && !strings.HasPrefix(trimmed, "[") {
		return nil, false
	}
	var value any
	if err := json.Unmarshal([]byte(trimmed), &value); err != nil {
		return nil, false
	}
	object, ok := value.(map[string]any)
	if !ok {
		return nil, true
	}
	return object, true
}

func hasPayloadPath(payload map[string]any, path string) bool {
	parts := strings.Split(path, ".")
	if len(parts) < 2 || parts[0] != "payload" {
		return false
	}
	var current any = payload
	for _, part := range parts[1:] {
		obj, ok := current.(map[string]any)
		if !ok {
			return false
		}
		current, ok = obj[part]
		if !ok {
			return false
		}
	}
	return true
}

func requiredPayloadFields(raw json.RawMessage) []string {
	var schema struct {
		RequiredPayloadFields []string `json:"required_payload_fields"`
	}
	if err := json.Unmarshal(normalizeJSON(raw), &schema); err != nil {
		return nil
	}
	fields := make([]string, 0, len(schema.RequiredPayloadFields))
	for _, field := range schema.RequiredPayloadFields {
		field = strings.TrimSpace(field)
		if field != "" {
			fields = append(fields, field)
		}
	}
	return fields
}

func variablePaths(variables []VariableRef) []string {
	paths := make([]string, 0, len(variables))
	for _, variable := range variables {
		paths = append(paths, variable.Path)
	}
	return paths
}

func ValidationErrorsJSON(result ValidationResult) json.RawMessage {
	raw, err := json.Marshal(result.Errors)
	if err != nil {
		return json.RawMessage(`[]`)
	}
	return raw
}

func ErrorSummary(result ValidationResult) error {
	if result.Status == "valid" {
		return nil
	}
	return fmt.Errorf("%w: %d validation errors", ErrInvalidTemplate, len(result.Errors))
}
