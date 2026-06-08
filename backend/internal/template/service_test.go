package msgtemplate

import (
	"context"
	"encoding/json"
	"testing"
)

func TestParseVariablesUsesCopyFormatAndInternalPath(t *testing.T) {
	result, err := NewService(nil).Parse(VersionInput{
		MessageType:        "text",
		TargetProviderType: "wecom_app",
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
		TargetProviderType: "wecom_app",
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

func TestPreviewUsesGlobalFallbackForMissingPayloadField(t *testing.T) {
	result, err := NewService(nil).Preview(VersionInput{
		MessageType:        "text",
		TargetProviderType: "wecom_app",
		TemplateBody:       `{"msgtype":"text","content":"标题：{{ payload.title }} IP：{{ payload.alert.ip }}"}`,
		SamplePayload:      json.RawMessage(`{"title":"告警"}`),
	})
	if err != nil {
		t.Fatalf("preview template with missing payload field: %v result=%+v", err, result)
	}
	if result.Status != "valid" || result.Preview != `{"msgtype":"text","content":"标题：告警 IP：-"}` {
		t.Fatalf("expected global fallback preview, got %+v", result)
	}
}

func TestPreviewRendersValidTemplate(t *testing.T) {
	result, err := NewService(nil).Preview(VersionInput{
		MessageType:        "text",
		TargetProviderType: "wecom_app",
		TemplateBody:       `{"msgtype":"text","content":"标题：{{ payload.title }}"}`,
		SamplePayload:      json.RawMessage(`{"title":"告警"}`),
	})
	if err != nil {
		t.Fatalf("preview valid template: %v", err)
	}
	if result.Status != "valid" || result.Preview != `{"msgtype":"text","content":"标题：告警"}` {
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
		TargetProviderType: "wecom_app",
		TemplateBody:       `{"msgtype":"text","content":"{{ payload.summary | default('通知') }}"}`,
		SamplePayload:      json.RawMessage(`{}`),
	})
	if err != nil {
		t.Fatalf("preview template with default filter: %v result=%+v", err, result)
	}
	if result.Status != "valid" || result.Preview != `{"msgtype":"text","content":"通知"}` {
		t.Fatalf("expected default filter preview, got %+v", result)
	}
}

func TestTemplateValidateRejectsRecipientLikeFieldsInTemplateBody(t *testing.T) {
	result := NewService(nil).Validate(VersionInput{
		MessageType:        "text",
		TargetProviderType: "wecom_app",
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
		TargetProviderType: "wecom_app",
		TemplateBody:       `{"title":"{{ payload.title }}"}`,
		SamplePayload:      json.RawMessage(`{"title":"告警"}`),
	})
	if result.Status != "invalid" {
		t.Fatalf("expected missing provider schema field to be invalid, got %+v", result)
	}
	assertValidationError(t, result.Errors, "MGP-TPL-REQUIRED", "msgtype")
}

func TestTemplateValidateRejectsRenderedNonJSONBody(t *testing.T) {
	result := NewService(nil).Validate(VersionInput{
		MessageType:        "json",
		TargetProviderType: "webhook",
		TemplateBody:       `plain {{ payload.title }}`,
		SamplePayload:      json.RawMessage(`{"title":"告警"}`),
	})
	if result.Status != "invalid" {
		t.Fatalf("expected rendered non-json template to be invalid, got %+v", result)
	}
	assertValidationError(t, result.Errors, "MGP-TPL-JSON", "template_body")
}

func TestTemplateValidateUsesRenderedJSONForSchemaChecks(t *testing.T) {
	result := NewService(nil).Validate(VersionInput{
		MessageType:        "json",
		TargetProviderType: "webhook",
		TemplateBody:       `{"body":"{{ payload.title }}"}`,
		SamplePayload:      json.RawMessage(`{"title":"告警"}`),
	})
	if result.Status != "invalid" {
		t.Fatalf("expected rendered schema type mismatch to be invalid, got %+v", result)
	}
	assertValidationError(t, result.Errors, "MGP-TPL-SCHEMA", "body")
}

func TestPerformanceWebhookTemplateValidatesRenderedBodyObject(t *testing.T) {
	result := NewService(nil).Validate(VersionInput{
		MessageType:        "json",
		TargetProviderType: "webhook",
		TemplateBody:       `{"body":{"title":"{{ payload.title | default('【模版】性能测试') }}","content":"{{ payload.content | default('【模版】性能测试消息') }}","route_key":"{{ payload.route_key }}","timestamp":"{{ payload.timestamp }}"}}`,
		MessageBodySchema:  json.RawMessage(`{"type":"object","required":["body"],"properties":{"body":{"type":"object"}}}`),
		SamplePayload:      json.RawMessage(`{"timestamp":"2026-06-05T00:00:00+08:00","route_key":"a","title":"性能测试","content":"这是一条性能测试消息，随机消息-000000000000"}`),
	})
	if result.Status != "valid" {
		t.Fatalf("expected performance webhook template to validate, got %+v", result)
	}
	if result.Preview != `{"body":{"title":"性能测试","content":"这是一条性能测试消息，随机消息-000000000000","route_key":"a","timestamp":"2026-06-05T00:00:00+08:00"}}` {
		t.Fatalf("unexpected performance template preview: %+v", result)
	}
}

func TestTemplateValidateRejectsUnsupportedFilter(t *testing.T) {
	result := NewService(nil).Validate(VersionInput{
		MessageType:        "text",
		TargetProviderType: "wecom_app",
		TemplateBody:       `{"msgtype":"text","content":"{{ payload.title | upper }}"}`,
		SamplePayload:      json.RawMessage(`{"title":"告警"}`),
	})
	if result.Status != "invalid" {
		t.Fatalf("expected unsupported filter to be invalid, got %+v", result)
	}
	assertValidationError(t, result.Errors, "MGP-TPL-FILTER", "upper")
}

func TestTemplateValidateRejectsUnsupportedTag(t *testing.T) {
	result := NewService(nil).Validate(VersionInput{
		MessageType:        "text",
		TargetProviderType: "wecom_app",
		TemplateBody:       `{"msgtype":"text","content":"{% include \"secret.tpl\" %}"}`,
		SamplePayload:      json.RawMessage(`{"title":"告警"}`),
	})
	if result.Status != "invalid" {
		t.Fatalf("expected unsupported tag to be invalid, got %+v", result)
	}
	assertValidationError(t, result.Errors, "MGP-TPL-TAG", "include")
}

func TestPublishValidProviderAwareJSONTemplate(t *testing.T) {
	store := &recordingTemplateStore{}
	version, err := NewService(store).Publish(context.Background(), "template-1", VersionInput{
		MessageType:        " text ",
		TargetProviderType: " wecom_app ",
		TemplateBody:       `{"msgtype":"text","content":"{{ payload.summary | default('通知') }}"}`,
		SamplePayload:      json.RawMessage(`{}`),
	})
	if err != nil {
		t.Fatalf("publish provider-aware template: %v", err)
	}
	if version.ValidationStatus != "valid" || store.publishParams.MessageType != "text" || store.publishParams.TargetProviderType != "wecom_app" {
		t.Fatalf("expected normalized valid publish, version=%+v params=%+v", version, store.publishParams)
	}
	if string(store.publishParams.CompiledPreview) != `{"rendered":"{\"msgtype\":\"text\",\"content\":\"通知\"}"}` {
		t.Fatalf("unexpected compiled preview: %s", store.publishParams.CompiledPreview)
	}
	if len(store.publishParams.AllowedFilters) != 1 || store.publishParams.AllowedFilters[0] != "default" {
		t.Fatalf("expected default filter to be persisted, got %+v", store.publishParams.AllowedFilters)
	}
}

func TestRestoreTemplateVersionPublishesCopiedHistoricalVersion(t *testing.T) {
	store := &recordingTemplateStore{
		version: TemplateVersion{
			ID:                 "version-old",
			TemplateID:         "template-1",
			VersionNo:          2,
			MessageType:        "json",
			TargetProviderType: "pushplus",
			TemplateBody:       `{"content":"{{ payload.content | default('-') }}"}`,
			MessageBodySchema:  json.RawMessage(`{"type":"object"}`),
			SamplePayload:      json.RawMessage(`{"content":"历史内容"}`),
		},
	}
	version, err := NewService(store).RestoreTemplateVersion(context.Background(), "template-1", "version-old")
	if err != nil {
		t.Fatalf("restore template version: %v", err)
	}
	if version.ID != "version-1" {
		t.Fatalf("expected restored publish result, got %+v", version)
	}
	if store.requestedTemplateID != "template-1" || store.requestedVersionID != "version-old" {
		t.Fatalf("expected historical version lookup, got template=%s version=%s", store.requestedTemplateID, store.requestedVersionID)
	}
	if store.publishParams.MessageType != "json" ||
		store.publishParams.TargetProviderType != "pushplus" ||
		store.publishParams.TemplateBody != `{"content":"{{ payload.content | default('-') }}"}` ||
		string(store.publishParams.MessageBodySchema) != `{"type":"object"}` ||
		string(store.publishParams.SamplePayload) != `{"content":"历史内容"}` {
		t.Fatalf("restore did not copy historical version into publish params: %+v", store.publishParams)
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
	publishParams       PublishTemplateVersionParams
	version             TemplateVersion
	requestedTemplateID string
	requestedVersionID  string
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

func (s *recordingTemplateStore) ListTemplateVersions(context.Context, string) ([]TemplateVersion, error) {
	return nil, nil
}

func (s *recordingTemplateStore) GetTemplateVersionForRestore(_ context.Context, templateID string, versionID string) (TemplateVersion, error) {
	s.requestedTemplateID = templateID
	s.requestedVersionID = versionID
	return s.version, nil
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
		AllowedFilters:     params.AllowedFilters,
	}, nil
}
