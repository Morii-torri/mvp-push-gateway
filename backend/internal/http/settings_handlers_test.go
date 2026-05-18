package httpapi_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	httpapi "mvp-push-gateway/backend/internal/http"
	"mvp-push-gateway/backend/internal/route"
	msgtemplate "mvp-push-gateway/backend/internal/template"
)

func TestSettingsPerformanceTestLeavesRuleKeyForRouteServiceGenerator(t *testing.T) {
	settingsService := &fakeSettingsService{}
	routeService := &fakeRouteService{}
	handler := httpapi.NewHandler(
		testConfig(),
		httpapi.WithAuthService(fakeAuthService{authenticatedToken: "admin-session"}),
		httpapi.WithSettingsService(settingsService),
		httpapi.WithSourceService(&fakeSourceService{}),
		httpapi.WithProviderService(&fakeProviderService{}),
		httpapi.WithTemplateService(&fakePerformanceTemplateService{}),
		httpapi.WithRouteService(routeService),
	)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/settings/performance-test", strings.NewReader(`{"message_count":10}`))
	req.Header.Set("Authorization", "Bearer admin-session")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected performance test to return 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if routeService.saveRulesCalls != 1 {
		t.Fatalf("expected one route rule save, got %d", routeService.saveRulesCalls)
	}
	if got := routeService.saveRulesInput.Rules[0].RuleKey; got != "" {
		t.Fatalf("expected performance test to let route service generate UUID rule_key, got %q", got)
	}
	if settingsService.performanceTestInput.GeneratedSourceCode == "" ||
		settingsService.performanceTestInput.GeneratedRouteName == "" ||
		settingsService.performanceTestInput.GeneratedChannelName == "" {
		t.Fatalf("expected generated resource names to be passed to performance test, got %+v", settingsService.performanceTestInput)
	}
}

func TestSettingsPerformanceTestCleansGeneratedResourcesWhenRuleSaveFails(t *testing.T) {
	settingsService := &fakeSettingsService{}
	sourceService := &fakeSourceService{}
	providerService := &fakeProviderService{}
	templateService := &fakePerformanceTemplateService{}
	routeService := &fakeRouteService{saveRulesErr: route.ErrInvalidInput}
	handler := httpapi.NewHandler(
		testConfig(),
		httpapi.WithAuthService(fakeAuthService{authenticatedToken: "admin-session"}),
		httpapi.WithSettingsService(settingsService),
		httpapi.WithSourceService(sourceService),
		httpapi.WithProviderService(providerService),
		httpapi.WithTemplateService(templateService),
		httpapi.WithRouteService(routeService),
	)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/settings/performance-test", strings.NewReader(`{"message_count":10}`))
	req.Header.Set("Authorization", "Bearer admin-session")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected resource generation failure to return 500, got %d body=%s", rec.Code, rec.Body.String())
	}
	if got := responseErrorCode(t, rec); got != "MGP-SETTINGS-999" {
		t.Fatalf("expected MGP-SETTINGS-999, got %q", got)
	}
	if settingsService.performanceTestCalls != 0 {
		t.Fatalf("expected performance runner not to start after resource generation failure, got %d calls", settingsService.performanceTestCalls)
	}
	if routeService.deleteCalls != 1 || templateService.deleteCalls != 1 || providerService.deleteChannelCalls != 1 || sourceService.deleteCalls != 1 {
		t.Fatalf("expected generated resources to be cleaned, got route=%d template=%d channel=%d source=%d",
			routeService.deleteCalls,
			templateService.deleteCalls,
			providerService.deleteChannelCalls,
			sourceService.deleteCalls,
		)
	}
}

func TestSettingsPerformanceTestCleansGeneratedResourcesWhenRunnerFails(t *testing.T) {
	settingsService := &fakeSettingsService{performanceTestErr: errors.New("runner failed")}
	sourceService := &fakeSourceService{}
	providerService := &fakeProviderService{}
	templateService := &fakePerformanceTemplateService{}
	routeService := &fakeRouteService{}
	handler := httpapi.NewHandler(
		testConfig(),
		httpapi.WithAuthService(fakeAuthService{authenticatedToken: "admin-session"}),
		httpapi.WithSettingsService(settingsService),
		httpapi.WithSourceService(sourceService),
		httpapi.WithProviderService(providerService),
		httpapi.WithTemplateService(templateService),
		httpapi.WithRouteService(routeService),
	)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/settings/performance-test", strings.NewReader(`{"message_count":10}`))
	req.Header.Set("Authorization", "Bearer admin-session")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected runner failure to return 500, got %d body=%s", rec.Code, rec.Body.String())
	}
	if got := responseErrorCode(t, rec); got != "MGP-SETTINGS-999" {
		t.Fatalf("expected MGP-SETTINGS-999, got %q", got)
	}
	if routeService.deleteCalls != 1 || templateService.deleteCalls != 1 || providerService.deleteChannelCalls != 1 || sourceService.deleteCalls != 1 {
		t.Fatalf("expected generated resources to be cleaned after runner failure, got route=%d template=%d channel=%d source=%d",
			routeService.deleteCalls,
			templateService.deleteCalls,
			providerService.deleteChannelCalls,
			sourceService.deleteCalls,
		)
	}
}

type fakePerformanceTemplateService struct {
	createCalls  int
	publishCalls int
	deleteCalls  int

	createErr  error
	publishErr error
	deleteErr  error
}

func (f *fakePerformanceTemplateService) ListTemplates(context.Context) ([]msgtemplate.Template, error) {
	return nil, nil
}

func (f *fakePerformanceTemplateService) CreateTemplate(_ context.Context, input msgtemplate.TemplateInput) (msgtemplate.Template, error) {
	f.createCalls++
	if f.createErr != nil {
		return msgtemplate.Template{}, f.createErr
	}
	return msgtemplate.Template{ID: "template-1", Name: input.Name, SourceID: input.SourceID, Enabled: input.Enabled}, nil
}

func (f *fakePerformanceTemplateService) GetTemplate(context.Context, string) (msgtemplate.Template, error) {
	return msgtemplate.Template{}, nil
}

func (f *fakePerformanceTemplateService) UpdateTemplate(context.Context, string, msgtemplate.TemplateInput) (msgtemplate.Template, error) {
	return msgtemplate.Template{}, nil
}

func (f *fakePerformanceTemplateService) DeleteTemplate(context.Context, string) error {
	f.deleteCalls++
	if f.deleteErr != nil {
		return f.deleteErr
	}
	return nil
}

func (f *fakePerformanceTemplateService) ListTemplateVersions(context.Context, string) ([]msgtemplate.TemplateVersion, error) {
	return nil, nil
}

func (f *fakePerformanceTemplateService) Parse(msgtemplate.VersionInput) (msgtemplate.ValidationResult, error) {
	return msgtemplate.ValidationResult{}, nil
}

func (f *fakePerformanceTemplateService) Preview(msgtemplate.VersionInput) (msgtemplate.ValidationResult, error) {
	return msgtemplate.ValidationResult{}, nil
}

func (f *fakePerformanceTemplateService) Validate(msgtemplate.VersionInput) msgtemplate.ValidationResult {
	return msgtemplate.ValidationResult{Status: "valid"}
}

func (f *fakePerformanceTemplateService) Publish(_ context.Context, templateID string, _ msgtemplate.VersionInput) (msgtemplate.TemplateVersion, error) {
	f.publishCalls++
	if f.publishErr != nil {
		return msgtemplate.TemplateVersion{}, f.publishErr
	}
	return msgtemplate.TemplateVersion{ID: "template-version-1", TemplateID: templateID, VersionNo: 1}, nil
}

func (f *fakePerformanceTemplateService) RestoreTemplateVersion(context.Context, string, string) (msgtemplate.TemplateVersion, error) {
	return msgtemplate.TemplateVersion{}, nil
}
