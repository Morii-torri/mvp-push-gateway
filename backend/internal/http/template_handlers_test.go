package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	httpapi "mvp-push-gateway/backend/internal/http"
	msgtemplate "mvp-push-gateway/backend/internal/template"
)

func TestTemplateValidationHandlersReturnTemplateValidationErrors(t *testing.T) {
	handler := httpapi.NewHandler(
		testConfig(),
		httpapi.WithAuthService(fakeAuthService{authenticatedToken: "admin-session"}),
		httpapi.WithTemplateService(msgtemplate.NewService(&httpTemplateStore{})),
	)

	for _, tc := range []struct {
		path       string
		wantStatus int
	}{
		{path: "/api/v1/templates/parse", wantStatus: http.StatusBadRequest},
		{path: "/api/v1/templates/preview", wantStatus: http.StatusBadRequest},
		{path: "/api/v1/templates/validate", wantStatus: http.StatusOK},
	} {
		t.Run(tc.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(`{
				"message_type":"text",
				"target_provider_type":"wecom_app",
				"template_body":"{\"touser\":\"{{ payload.user }}\",\"content\":\"{{ payload.title }}\"}",
				"sample_payload":{"user":"zhangsan","title":"告警"}
			}`))
			setAdminSessionCookie(req, "admin-session")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("expected status %d, got %d body=%s", tc.wantStatus, rec.Code, rec.Body.String())
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
		"target_provider_type":" wecom_app ",
		"template_body":"{\"msgtype\":\"text\",\"content\":\"{{ payload.summary | default('通知') }}\"}",
		"sample_payload":{}
	}`))
	setAdminSessionCookie(req, "admin-session")
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
	if body.Version.MessageType != "text" || body.Version.TargetProviderType != "wecom_app" || body.Version.ValidationStatus != "valid" {
		t.Fatalf("unexpected template version response: %+v", body.Version)
	}
	if string(body.Version.CompiledPreview) != `{"rendered":"{\"msgtype\":\"text\",\"content\":\"通知\"}"}` {
		t.Fatalf("unexpected compiled preview: %s", body.Version.CompiledPreview)
	}
}

func TestTemplatesHandlerIncludesCurrentVersionMetadata(t *testing.T) {
	now := time.Date(2026, 5, 12, 9, 0, 0, 0, time.UTC)
	version := msgtemplate.TemplateVersion{
		ID:                    "version-1",
		TemplateID:            "template-1",
		VersionNo:             1,
		MessageType:           "json",
		TargetProviderType:    "webhook",
		TemplateEngine:        "pongo2",
		TemplateSyntaxVersion: "jinja-like-v1",
		TemplateBody:          `{"title":"{{ payload.title }}"}`,
		MessageBodySchema:     json.RawMessage(`{"type":"object"}`),
		SamplePayload:         json.RawMessage(`{"title":"Smoke"}`),
		CompiledPreview:       json.RawMessage(`{"rendered":"{}"}`),
		UsedVariables:         []string{"payload.title"},
		ValidationStatus:      "valid",
		ValidationErrors:      json.RawMessage(`[]`),
		CreatedAt:             now,
		UpdatedAt:             now,
	}
	store := &httpTemplateStore{
		templates: []msgtemplate.Template{{
			ID:               "template-1",
			Name:             "Smoke 模板",
			SourceID:         "source-1",
			Enabled:          true,
			CurrentVersionID: version.ID,
			CurrentVersion:   &version,
			CreatedAt:        now,
			UpdatedAt:        now,
		}},
	}
	handler := httpapi.NewHandler(
		testConfig(),
		httpapi.WithAuthService(fakeAuthService{authenticatedToken: "admin-session"}),
		httpapi.WithTemplateService(msgtemplate.NewService(store)),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/templates", nil)
	setAdminSessionCookie(req, "admin-session")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Templates []struct {
			CurrentVersionID   string `json:"current_version_id"`
			MessageType        string `json:"message_type"`
			TargetProviderType string `json:"target_provider_type"`
			CurrentVersion     struct {
				ID                 string `json:"id"`
				TargetProviderType string `json:"target_provider_type"`
			} `json:"current_version"`
		} `json:"templates"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode templates response: %v", err)
	}
	if len(body.Templates) != 1 {
		t.Fatalf("expected one template, got %d", len(body.Templates))
	}
	got := body.Templates[0]
	if got.CurrentVersionID != "version-1" || got.MessageType != "json" || got.TargetProviderType != "webhook" || got.CurrentVersion.ID != "version-1" || got.CurrentVersion.TargetProviderType != "webhook" {
		t.Fatalf("current version metadata missing from response: %+v", got)
	}
}

func TestTemplateVersionHandlersListAndRestoreHistoricalVersions(t *testing.T) {
	now := time.Date(2026, 5, 15, 8, 0, 0, 0, time.UTC)
	store := &httpTemplateStore{
		versions: []msgtemplate.TemplateVersion{{
			ID:                 "version-2",
			TemplateID:         "template-1",
			VersionNo:          2,
			MessageType:        "json",
			TargetProviderType: "pushplus",
			TemplateBody:       `{"content":"{{ payload.content | default('-') }}"}`,
			MessageBodySchema:  json.RawMessage(`{"type":"object"}`),
			SamplePayload:      json.RawMessage(`{"content":"历史内容"}`),
			UsedVariables:      []string{"payload.content"},
			ValidationStatus:   "valid",
			ValidationErrors:   json.RawMessage(`[]`),
			CreatedAt:          now,
			UpdatedAt:          now,
		}},
	}
	handler := httpapi.NewHandler(
		testConfig(),
		httpapi.WithAuthService(fakeAuthService{authenticatedToken: "admin-session"}),
		httpapi.WithTemplateService(msgtemplate.NewService(store)),
	)

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/templates/template-1/versions", nil)
	setAdminSessionCookie(listReq, "admin-session")
	listRec := httptest.NewRecorder()
	handler.ServeHTTP(listRec, listReq)

	if listRec.Code != http.StatusOK {
		t.Fatalf("expected list status 200, got %d body=%s", listRec.Code, listRec.Body.String())
	}
	var listBody struct {
		Versions []struct {
			ID           string `json:"id"`
			VersionNo    int    `json:"version_no"`
			TemplateBody string `json:"template_body"`
		} `json:"versions"`
	}
	if err := json.NewDecoder(listRec.Body).Decode(&listBody); err != nil {
		t.Fatalf("decode list versions response: %v", err)
	}
	if len(listBody.Versions) != 1 || listBody.Versions[0].ID != "version-2" || listBody.Versions[0].VersionNo != 2 || listBody.Versions[0].TemplateBody == "" {
		t.Fatalf("unexpected versions response: %+v", listBody.Versions)
	}

	restoreReq := httptest.NewRequest(http.MethodPost, "/api/v1/templates/template-1/versions/version-2/restore", nil)
	setAdminSessionCookie(restoreReq, "admin-session")
	restoreRec := httptest.NewRecorder()
	handler.ServeHTTP(restoreRec, restoreReq)

	if restoreRec.Code != http.StatusCreated {
		t.Fatalf("expected restore status 201, got %d body=%s", restoreRec.Code, restoreRec.Body.String())
	}
	if store.requestedTemplateID != "template-1" || store.requestedVersionID != "version-2" || store.publishCalls != 1 {
		t.Fatalf("restore did not copy old version into new publish: template=%s version=%s publishCalls=%d", store.requestedTemplateID, store.requestedVersionID, store.publishCalls)
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
	templates           []msgtemplate.Template
	versions            []msgtemplate.TemplateVersion
	publishCalls        int
	publishParams       msgtemplate.PublishTemplateVersionParams
	requestedTemplateID string
	requestedVersionID  string
}

func (s *httpTemplateStore) ListTemplates(context.Context) ([]msgtemplate.Template, error) {
	return s.templates, nil
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

func (s *httpTemplateStore) ListTemplateVersions(context.Context, string) ([]msgtemplate.TemplateVersion, error) {
	return s.versions, nil
}

func (s *httpTemplateStore) GetTemplateVersionForRestore(_ context.Context, templateID string, versionID string) (msgtemplate.TemplateVersion, error) {
	s.requestedTemplateID = templateID
	s.requestedVersionID = versionID
	for _, version := range s.versions {
		if version.ID == versionID {
			return version, nil
		}
	}
	return msgtemplate.TemplateVersion{}, msgtemplate.ErrNotFound
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
