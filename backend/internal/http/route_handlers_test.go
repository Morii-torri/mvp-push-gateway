package httpapi_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	httpapi "mvp-push-gateway/backend/internal/http"
	"mvp-push-gateway/backend/internal/route"
)

func TestRouteFlowCRUDRequiresAdminBearerAuthentication(t *testing.T) {
	routeService := &fakeRouteService{
		listResult: []route.Flow{{
			ID:       "flow-1",
			SourceID: "source-1",
			Name:     "Orders",
			Enabled:  true,
			Mode:     route.ModeTable,
		}},
	}
	handler := httpapi.NewHandler(
		testConfig(),
		httpapi.WithAuthService(fakeAuthService{authenticatedToken: "admin-session"}),
		httpapi.WithRouteService(routeService),
	)

	unauthenticated := httptest.NewRequest(http.MethodGet, "/api/v1/route-flows", nil)
	unauthenticatedRec := httptest.NewRecorder()
	handler.ServeHTTP(unauthenticatedRec, unauthenticated)
	if unauthenticatedRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected route list without admin bearer to return 401, got %d", unauthenticatedRec.Code)
	}
	if routeService.listCalls != 0 {
		t.Fatalf("expected route service not to be called without admin auth, got %d calls", routeService.listCalls)
	}

	authenticated := httptest.NewRequest(http.MethodGet, "/api/v1/route-flows", nil)
	authenticated.Header.Set("Authorization", "Bearer admin-session")
	authenticatedRec := httptest.NewRecorder()
	handler.ServeHTTP(authenticatedRec, authenticated)
	if authenticatedRec.Code != http.StatusOK {
		t.Fatalf("expected route list with admin bearer to return 200, got %d body=%s", authenticatedRec.Code, authenticatedRec.Body.String())
	}
	if routeService.listCalls != 1 {
		t.Fatalf("expected one route list call, got %d", routeService.listCalls)
	}
}

func TestDuplicateEnabledRouteFlowReturnsPublishedErrorCode(t *testing.T) {
	handler := httpapi.NewHandler(
		testConfig(),
		httpapi.WithAuthService(fakeAuthService{authenticatedToken: "admin-session"}),
		httpapi.WithRouteService(&fakeRouteService{createErr: route.ErrEnabledFlowExists}),
	)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/route-flows", strings.NewReader(`{"source_id":"source-1","name":"Orders","enabled":true,"mode":"table"}`))
	req.Header.Set("Authorization", "Bearer admin-session")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 conflict, got %d body=%s", rec.Code, rec.Body.String())
	}
	if got := responseErrorCode(t, rec); got != "MGP-ROUTE-003" {
		t.Fatalf("expected MGP-ROUTE-003, got %q", got)
	}
}

func TestSimulateReturnsMatchedRuleAndStopReason(t *testing.T) {
	handler := httpapi.NewHandler(
		testConfig(),
		httpapi.WithAuthService(fakeAuthService{authenticatedToken: "admin-session"}),
		httpapi.WithRouteService(&fakeRouteService{
			simulateResult: route.SimulationResult{
				VersionID:  "draft-1",
				StopReason: "first_match_stop",
				MatchedRule: &route.RuleTrace{
					RuleKey:   "rule-a",
					Name:      "First",
					SortOrder: 10,
					Matched:   true,
					Evaluated: true,
				},
				RuleResults: []route.RuleTrace{
					{RuleKey: "rule-a", Name: "First", SortOrder: 10, Matched: true, Evaluated: true},
					{RuleKey: "rule-b", Name: "Second", SortOrder: 20, Matched: false, Evaluated: false, StopReason: "first_match_stop"},
				},
			},
		}),
	)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/route-flows/flow-1/simulate", strings.NewReader(`{"payload":{"title":"critical"}}`))
	req.Header.Set("Authorization", "Bearer admin-session")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		StopReason string            `json:"stop_reason"`
		Matched    *route.RuleTrace  `json:"matched_rule"`
		RuleTrace  []route.RuleTrace `json:"rule_results"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode simulate response: %v", err)
	}
	if body.StopReason != "first_match_stop" {
		t.Fatalf("expected stop_reason first_match_stop, got %q", body.StopReason)
	}
	if body.Matched == nil || body.Matched.RuleKey != "rule-a" {
		t.Fatalf("expected matched rule-a, got %+v", body.Matched)
	}
	if len(body.RuleTrace) != 2 || body.RuleTrace[1].StopReason != "first_match_stop" {
		t.Fatalf("expected rule trace to include stop reason, got %+v", body.RuleTrace)
	}
}

type fakeRouteService struct {
	listResult     []route.Flow
	getResult      route.Flow
	createResult   route.Flow
	updateResult   route.Flow
	versionsResult []route.Version
	canvasResult   route.CanvasState
	rulesResult    route.RuleSet
	validateResult route.ValidationResult
	publishResult  route.Version
	simulateResult route.SimulationResult

	createErr   error
	getErr      error
	updateErr   error
	deleteErr   error
	validateErr error
	publishErr  error
	simulateErr error

	listCalls int
}

func (f *fakeRouteService) ListFlows(context.Context) ([]route.Flow, error) {
	f.listCalls++
	return f.listResult, nil
}

func (f *fakeRouteService) CreateFlow(context.Context, route.CreateFlowInput) (route.Flow, error) {
	if f.createErr != nil {
		return route.Flow{}, f.createErr
	}
	if f.createResult.ID != "" {
		return f.createResult, nil
	}
	return route.Flow{ID: "flow-1", SourceID: "source-1", Name: "Orders", Enabled: true, Mode: route.ModeTable}, nil
}

func (f *fakeRouteService) GetFlow(context.Context, string) (route.Flow, error) {
	if f.getErr != nil {
		return route.Flow{}, f.getErr
	}
	return f.getResult, nil
}

func (f *fakeRouteService) UpdateFlow(context.Context, string, route.UpdateFlowInput) (route.Flow, error) {
	if f.updateErr != nil {
		return route.Flow{}, f.updateErr
	}
	return f.updateResult, nil
}

func (f *fakeRouteService) DeleteFlow(context.Context, string) error {
	return f.deleteErr
}

func (f *fakeRouteService) ListVersions(context.Context, string) ([]route.Version, error) {
	return f.versionsResult, nil
}

func (f *fakeRouteService) GetCanvas(context.Context, string) (route.CanvasState, error) {
	return f.canvasResult, nil
}

func (f *fakeRouteService) SaveCanvas(context.Context, string, route.SaveCanvasInput) (route.CanvasState, error) {
	return f.canvasResult, nil
}

func (f *fakeRouteService) GetRules(context.Context, string) (route.RuleSet, error) {
	return f.rulesResult, nil
}

func (f *fakeRouteService) SaveRules(context.Context, string, route.SaveRulesInput) (route.RuleSet, error) {
	return f.rulesResult, nil
}

func (f *fakeRouteService) ReorderRules(context.Context, string, route.ReorderRulesInput) (route.RuleSet, error) {
	return f.rulesResult, nil
}

func (f *fakeRouteService) Validate(context.Context, string) (route.ValidationResult, error) {
	if f.validateErr != nil {
		return route.ValidationResult{}, f.validateErr
	}
	return f.validateResult, nil
}

func (f *fakeRouteService) Publish(context.Context, string) (route.Version, error) {
	if f.publishErr != nil {
		return route.Version{}, f.publishErr
	}
	return f.publishResult, nil
}

func (f *fakeRouteService) Simulate(context.Context, string, route.SimulateInput) (route.SimulationResult, error) {
	if f.simulateErr != nil {
		return route.SimulationResult{}, f.simulateErr
	}
	return f.simulateResult, nil
}

func (f *fakeRouteService) ActivateVersion(context.Context, string, string) (route.Flow, error) {
	if f.getErr != nil {
		return route.Flow{}, errors.New("activate failed")
	}
	return route.Flow{ID: "flow-1", CurrentVersionID: "version-1"}, nil
}
