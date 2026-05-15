package planning

import (
	"encoding/json"
	"testing"
	"time"

	msgtemplate "mvp-push-gateway/backend/internal/template"
)

func TestRenderTemplateUsesGatewayTemplateEngineDefaultFilterSyntax(t *testing.T) {
	body, err := renderTemplate(
		msgtemplate.TemplateVersion{
			TemplateBody: `{"content":"{{ payload.summary | default('通知') }}"}`,
		},
		MessageRecord{
			ID:       "message-1",
			TraceID:  "trace-1",
			SourceID: "source-1",
		},
		map[string]any{},
		time.Date(2026, 5, 14, 10, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("render template with default filter syntax: %v", err)
	}

	var rendered map[string]string
	if err := json.Unmarshal(body, &rendered); err != nil {
		t.Fatalf("decode rendered template: %v", err)
	}
	if rendered["content"] != "通知" {
		t.Fatalf("expected default content, got %q", rendered["content"])
	}
}

func TestRenderTemplateUsesGatewayTemplateEngineGlobalMissingPayloadFallback(t *testing.T) {
	body, err := renderTemplate(
		msgtemplate.TemplateVersion{
			TemplateBody: `{"content":"{{ payload.summary }}"}`,
		},
		MessageRecord{
			ID:       "message-1",
			TraceID:  "trace-1",
			SourceID: "source-1",
		},
		map[string]any{},
		time.Date(2026, 5, 14, 10, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("render template with global missing payload fallback: %v", err)
	}

	var rendered map[string]string
	if err := json.Unmarshal(body, &rendered); err != nil {
		t.Fatalf("decode rendered template: %v", err)
	}
	if rendered["content"] != "-" {
		t.Fatalf("expected global fallback content, got %q", rendered["content"])
	}
}
