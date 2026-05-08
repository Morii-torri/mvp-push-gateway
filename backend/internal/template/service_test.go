package msgtemplate

import (
	"encoding/json"
	"testing"
)

func TestParseVariablesUsesCopyFormatAndInternalPath(t *testing.T) {
	result, err := NewService(nil).Parse(VersionInput{
		TemplateBody: `标题：{{ payload.title }} 内容：{{ payload.alert.ip | default:"-" }}`,
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
		TemplateBody:  `标题：{{ payload.title `,
		SamplePayload: json.RawMessage(`{"title":"告警"}`),
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
		TemplateBody:  `标题：{{ payload.title }} IP：{{ payload.alert.ip }}`,
		SamplePayload: json.RawMessage(`{"title":"告警"}`),
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
		TemplateBody:  `标题：{{ payload.title }}`,
		SamplePayload: json.RawMessage(`{"title":"告警"}`),
	})
	if err != nil {
		t.Fatalf("preview valid template: %v", err)
	}
	if result.Status != "valid" || result.Preview != "标题：告警" {
		t.Fatalf("unexpected preview result: %+v", result)
	}
}
