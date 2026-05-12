package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	httpapi "mvp-push-gateway/backend/internal/http"
	msgtemplate "mvp-push-gateway/backend/internal/template"
)

func TestTemplateValidationHandlersReturnTemplateValidationErrors(t *testing.T) {
	handler := httpapi.NewHandler(
		testConfig(),
		httpapi.WithAuthService(fakeAuthService{authenticatedToken: "admin-session"}),
		httpapi.WithTemplateService(msgtemplate.NewService(&httpTemplateStore{})),
	)

	for _, path := range []string{"/api/v1/templates/parse", "/api/v1/templates/preview", "/api/v1/templates/validate"} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{
				"message_type":"text",
				"target_provider_type":"wecom",
				"template_body":"{\"touser\":\"{{ payload.user }}\",\"content\":\"{{ payload.title }}\"}",
				"sample_payload":{"user":"zhangsan","title":"告警"}
			}`))
			req.Header.Set("Authorization", "Bearer admin-session")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected status 400, got %d body=%s", rec.Code, rec.Body.String())
			}

			var body struct {
				Result msgtemplate.ValidationResult `json:"result"`
			}
			if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
				t.Fatalf("decode validation response: %v", err)
			}
			if body.Result.Status != "invalid" || !hasTemplateError(body.Result.Errors, "MGP-TPL-RECIPIENT", "touser") {
				t.Fatalf("expected recipient validation error, got %+v", body.Result)
			}
		})
	}
}

func TestTemplatePublishHandlerPublishesProviderAwareTemplate(t *testing.T) {
	store := &httpTemplateStore{}
	handler := httpapi.NewHandler(
		testConfig(),
		httpapi.WithAuthService(fakeAuthService{authenticatedToken: "admin-session"}),
		httpapi.WithTemplateService(msgtemplate.NewService(store)),
	)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/templates/template-1/publish", strings.NewReader(`{
		"message_type":" text ",
		"target_provider_type":" wecom ",
		"template_body":"{\"content\":\"{{ payload.summary | default('通知') }}\"}",
		"sample_payload":{}
	}`))
	req.Header.Set("Authorization", "Bearer admin-session")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d body=%s", rec.Code, rec.Body.String())
	}
	if store.publishCalls != 1 {
		t.Fatalf("expected publish store call, got %d", store.publishCalls)
	}

	var body struct {
		Version struct {
			MessageType        string          `json:"message_type"`
			TargetProviderType string          `json:"target_provider_type"`
			CompiledPreview    json.RawMessage `json:"compiled_preview"`
			ValidationStatus   string          `json:"validation_status"`
		} `json:"version"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode publish response: %v", err)
	}
	if body.Version.MessageType != "text" || body.Version.TargetProviderType != "wecom" || body.Version.ValidationStatus != "valid" {
		t.Fatalf("unexpected template version response: %+v", body.Version)
	}
	if string(body.Version.CompiledPreview) != `{"rendered":"{\"content\":\"通知\"}"}` {
		t.Fatalf("unexpected compiled preview: %s", body.Version.CompiledPreview)
	}
}

func hasTemplateError(errors []msgtemplate.ValidationError, code string, path string) bool {
	for _, err := range errors {
		if err.Code == code && err.Path == path {
			return true
		}
	}
	return false
}

type httpTemplateStore struct {
	publishCalls  int
	publishParams msgtemplate.PublishTemplateVersionParams
}

func (s *httpTemplateStore) ListTemplates(context.Context) ([]msgtemplate.Template, error) {
	return nil, nil
}

func (s *httpTemplateStore) CreateTemplate(context.Context, msgtemplate.CreateTemplateParams) (msgtemplate.Template, error) {
	return msgtemplate.Template{}, nil
}

func (s *httpTemplateStore) GetTemplate(context.Context, string) (msgtemplate.Template, error) {
	return msgtemplate.Template{}, nil
}

func (s *httpTemplateStore) UpdateTemplate(context.Context, string, msgtemplate.UpdateTemplateParams) (msgtemplate.Template, error) {
	return msgtemplate.Template{}, nil
}

func (s *httpTemplateStore) DeleteTemplate(context.Context, string) error {
	return nil
}

func (s *httpTemplateStore) PublishTemplateVersion(_ context.Context, templateID string, params msgtemplate.PublishTemplateVersionParams) (msgtemplate.TemplateVersion, error) {
	s.publishCalls++
	s.publishParams = params
	return msgtemplate.TemplateVersion{
		ID:                    "version-1",
		TemplateID:            templateID,
		VersionNo:             1,
		MessageType:           params.MessageType,
		TargetProviderType:    params.TargetProviderType,
		TemplateEngine:        "pongo2",
		TemplateSyntaxVersion: "jinja-like-v1",
		TemplateBody:          params.TemplateBody,
		MessageBodySchema:     params.MessageBodySchema,
		SamplePayload:         params.SamplePayload,
		CompiledPreview:       params.CompiledPreview,
		UsedVariables:         params.UsedVariables,
		ValidationStatus:      params.ValidationStatus,
		ValidationErrors:      params.ValidationErrors,
	}, nil
}
