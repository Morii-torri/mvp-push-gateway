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
	if strings.TrimSpace(input.TemplateBody) == "" {
		return ValidationResult{}, ErrInvalidInput
	}
	if _, err := pongo2.FromString(input.TemplateBody); err != nil {
		return ValidationResult{
			Status: "invalid",
			Errors: []ValidationError{{
				Code:    "MGP-TPL-001",
				Message: err.Error(),
			}},
		}, ErrInvalidTemplate
	}
	return ValidationResult{Status: "valid", Variables: ParseVariables(input.TemplateBody)}, nil
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

	tpl, err := pongo2.FromString(input.TemplateBody)
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

	for _, variable := range result.Variables {
		if !hasPayloadPath(payloadMap, variable.Path) {
			result.Status = "invalid"
			result.Errors = append(result.Errors, ValidationError{
				Code:    "MGP-TPL-003",
				Message: "模板变量在 sample_payload 中不存在",
				Path:    variable.Path,
			})
		}
	}
	for _, required := range requiredPayloadFields(input.MessageBodySchema) {
		if !hasPayloadPath(payloadMap, required) {
			result.Status = "invalid"
			result.Errors = append(result.Errors, ValidationError{
				Code:    "MGP-TPL-004",
				Message: "消息体 schema 需要的 payload 字段不存在",
				Path:    required,
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
	if strings.TrimSpace(input.MessageType) == "" || strings.TrimSpace(input.TargetProviderType) == "" || strings.TrimSpace(input.TemplateBody) == "" {
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
