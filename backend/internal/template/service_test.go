package msgtemplate

import (
	"context"
	"encoding/json"
	"testing"
)

func TestParseVariablesUsesCopyFormatAndInternalPath(t *testing.T) {
	result, err := NewService(nil).Parse(VersionInput{
		MessageType:        "text",
		TargetProviderType: "wecom",
		TemplateBody:       `标题：{{ payload.title }} 内容：{{ payload.alert.ip | default:"-" }}`,
	})
	if err != nil {
		t.Fatalf("parse template variables: %v", err)
	}
	if len(result.Variables) != 2 {
		t.Fatalf("expected 2 variables, got %+v", result.Variables)
	}
	if result.Variables[1].Variable != "{{ payload.title }}" || result.Variables[1].Path != "payload.title" {
		t.Fatalf("expected copy variable {{ payload.title }} and internal path payload.title, got %+v", result.Variables)
	}
}

func TestValidateBlocksInvalidSyntax(t *testing.T) {
	result := NewService(nil).Validate(VersionInput{
		MessageType:        "text",
		TargetProviderType: "wecom",
		TemplateBody:       `标题：{{ payload.title `,
		SamplePayload:      json.RawMessage(`{"title":"告警"}`),
	})
	if result.Status != "invalid" {
		t.Fatalf("expected invalid syntax to be blocked, got %+v", result)
	}
	if len(result.Errors) == 0 || result.Errors[0].Code != "MGP-TPL-001" {
		t.Fatalf("expected syntax error code MGP-TPL-001, got %+v", result.Errors)
	}
}

func TestValidateBlocksMissingPayloadField(t *testing.T) {
	result := NewService(nil).Validate(VersionInput{
		MessageType:        "text",
		TargetProviderType: "wecom",
		TemplateBody:       `标题：{{ payload.title }} IP：{{ payload.alert.ip }}`,
		SamplePayload:      json.RawMessage(`{"title":"告警"}`),
	})
	if result.Status != "invalid" {
		t.Fatalf("expected missing field to be blocked, got %+v", result)
	}
	var hasMissingField bool
	for _, err := range result.Errors {
		if err.Code == "MGP-TPL-003" && err.Path == "payload.alert.ip" {
			hasMissingField = true
		}
	}
	if !hasMissingField {
		t.Fatalf("expected missing field error for payload.alert.ip, got %+v", result.Errors)
	}
}

func TestPreviewRendersValidTemplate(t *testing.T) {
	result, err := NewService(nil).Preview(VersionInput{
		MessageType:        "text",
		TargetProviderType: "wecom",
		TemplateBody:       `标题：{{ payload.title }}`,
		SamplePayload:      json.RawMessage(`{"title":"告警"}`),
	})
	if err != nil {
		t.Fatalf("preview valid template: %v", err)
	}
	if result.Status != "valid" || result.Preview != "标题：告警" {
		t.Fatalf("unexpected preview result: %+v", result)
	}
}

func TestTemplateValidateRequiresProviderAndMessageType(t *testing.T) {
	result := NewService(nil).Validate(VersionInput{
		TemplateBody:  `标题：{{ payload.title }}`,
		SamplePayload: json.RawMessage(`{"title":"告警"}`),
	})
	if result.Status != "invalid" {
		t.Fatalf("expected missing provider/message type to be invalid, got %+v", result)
	}
	assertValidationError(t, result.Errors, "MGP-TPL-REQUIRED", "target_provider_type")
	assertValidationError(t, result.Errors, "MGP-TPL-REQUIRED", "message_type")
}

func TestTemplatePreviewAllowsDefaultFilterFunctionSyntax(t *testing.T) {
	result, err := NewService(nil).Preview(VersionInput{
		MessageType:        "text",
		TargetProviderType: "wecom",
		TemplateBody:       `{{ payload.summary | default('通知') }}`,
		SamplePayload:      json.RawMessage(`{}`),
	})
	if err != nil {
		t.Fatalf("preview template with default filter: %v result=%+v", err, result)
	}
	if result.Status != "valid" || result.Preview != "通知" {
		t.Fatalf("expected default filter preview, got %+v", result)
	}
}

func TestTemplateValidateRejectsRecipientLikeFieldsInTemplateBody(t *testing.T) {
	result := NewService(nil).Validate(VersionInput{
		MessageType:        "text",
		TargetProviderType: "wecom",
		TemplateBody:       `{"touser":"{{ payload.user }}","content":"{{ payload.title }}"}`,
		SamplePayload:      json.RawMessage(`{"user":"zhangsan","title":"告警"}`),
	})
	if result.Status != "invalid" {
		t.Fatalf("expected recipient-like field to be invalid, got %+v", result)
	}
	assertValidationError(t, result.Errors, "MGP-TPL-RECIPIENT", "touser")
}

func TestTemplateValidateUsesProviderDefaultSchemaRequiredFields(t *testing.T) {
	result := NewService(nil).Validate(VersionInput{
		MessageType:        "text",
		TargetProviderType: "wecom",
		TemplateBody:       `{"title":"{{ payload.title }}"}`,
		SamplePayload:      json.RawMessage(`{"title":"告警"}`),
	})
	if result.Status != "invalid" {
		t.Fatalf("expected missing provider schema field to be invalid, got %+v", result)
	}
	assertValidationError(t, result.Errors, "MGP-TPL-REQUIRED", "content")
}

func TestPublishValidProviderAwareJSONTemplate(t *testing.T) {
	store := &recordingTemplateStore{}
	version, err := NewService(store).Publish(context.Background(), "template-1", VersionInput{
		MessageType:        " text ",
		TargetProviderType: " wecom ",
		TemplateBody:       `{"content":"{{ payload.summary | default('通知') }}"}`,
		SamplePayload:      json.RawMessage(`{}`),
	})
	if err != nil {
		t.Fatalf("publish provider-aware template: %v", err)
	}
	if version.ValidationStatus != "valid" || store.publishParams.MessageType != "text" || store.publishParams.TargetProviderType != "wecom" {
		t.Fatalf("expected normalized valid publish, version=%+v params=%+v", version, store.publishParams)
	}
	if string(store.publishParams.CompiledPreview) != `{"rendered":"{\"content\":\"通知\"}"}` {
		t.Fatalf("unexpected compiled preview: %s", store.publishParams.CompiledPreview)
	}
}

func assertValidationError(t *testing.T, errors []ValidationError, code string, path string) {
	t.Helper()
	for _, err := range errors {
		if err.Code == code && err.Path == path {
			return
		}
	}
	t.Fatalf("expected validation error %s at %s, got %+v", code, path, errors)
}

type recordingTemplateStore struct {
	publishParams PublishTemplateVersionParams
}

func (s *recordingTemplateStore) ListTemplates(context.Context) ([]Template, error) {
	return nil, nil
}

func (s *recordingTemplateStore) CreateTemplate(context.Context, CreateTemplateParams) (Template, error) {
	return Template{}, nil
}

func (s *recordingTemplateStore) GetTemplate(context.Context, string) (Template, error) {
	return Template{}, nil
}

func (s *recordingTemplateStore) UpdateTemplate(context.Context, string, UpdateTemplateParams) (Template, error) {
	return Template{}, nil
}

func (s *recordingTemplateStore) DeleteTemplate(context.Context, string) error {
	return nil
}

func (s *recordingTemplateStore) PublishTemplateVersion(_ context.Context, templateID string, params PublishTemplateVersionParams) (TemplateVersion, error) {
	s.publishParams = params
	return TemplateVersion{
		ID:                 "version-1",
		TemplateID:         templateID,
		VersionNo:          1,
		MessageType:        params.MessageType,
		TargetProviderType: params.TargetProviderType,
		TemplateBody:       params.TemplateBody,
		ValidationStatus:   params.ValidationStatus,
		CompiledPreview:    params.CompiledPreview,
		UsedVariables:      params.UsedVariables,
	}, nil
}
