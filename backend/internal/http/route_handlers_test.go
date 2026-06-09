package httpapi_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	httpapi "mvp-push-gateway/backend/internal/http"
	"mvp-push-gateway/backend/internal/route"
)

func TestRouteFlowCRUDRequiresAdminSessionAuthentication(t *testing.T) {
	routeService := &fakeRouteService{
		listResult: []route.Flow{{
			ID:            "flow-1",
			SourceID:      "source-1",
			Name:          "Orders",
			Enabled:       true,
			Mode:          route.ModeTable,
			RuleCount:     3,
			TotalHitCount: 12,
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
	setAdminSessionCookie(authenticated, "admin-session")
	authenticatedRec := httptest.NewRecorder()
	handler.ServeHTTP(authenticatedRec, authenticated)
	if authenticatedRec.Code != http.StatusOK {
		t.Fatalf("expected route list with admin bearer to return 200, got %d body=%s", authenticatedRec.Code, authenticatedRec.Body.String())
	}
	if routeService.listCalls != 1 {
		t.Fatalf("expected one route list call, got %d", routeService.listCalls)
	}
	var body struct {
		Flows []struct {
			RuleCount     int `json:"rule_count"`
			TotalHitCount int `json:"total_hit_count"`
		} `json:"flows"`
	}
	if err := json.NewDecoder(authenticatedRec.Body).Decode(&body); err != nil {
		t.Fatalf("decode route flow list response: %v", err)
	}
	if len(body.Flows) != 1 || body.Flows[0].RuleCount != 3 || body.Flows[0].TotalHitCount != 12 {
		t.Fatalf("expected flow stats in list response, got %+v", body.Flows)
	}
}

func TestDuplicateEnabledRouteFlowReturnsPublishedErrorCode(t *testing.T) {
	handler := httpapi.NewHandler(
		testConfig(),
		httpapi.WithAuthService(fakeAuthService{authenticatedToken: "admin-session"}),
		httpapi.WithRouteService(&fakeRouteService{createErr: route.ErrEnabledFlowExists}),
	)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/route-flows", strings.NewReader(`{"source_id":"source-1","name":"Orders","enabled":true,"mode":"table"}`))
	setAdminSessionCookie(req, "admin-session")
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
	setAdminSessionCookie(req, "admin-session")
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

func TestRouteRulesAcceptsAndReturnsActionTargets(t *testing.T) {
	routeService := &fakeRouteService{
		rulesResult: route.RuleSet{
			VersionID: "draft-1",
			Rules: []route.Rule{
				{
					ID:        "rule-id",
					RuleKey:   "00000000-0000-0000-0000-000000000301",
					SortOrder: 10,
					Name:      "Critical",
					Enabled:   true,
					Action: route.Action{
						ID:                "action-id",
						TemplateVersionID: "tpl-a",
						ChannelIDs:        []string{"channel-a"},
						Targets: []route.ActionTarget{
							{
								ID:                "target-a",
								ChannelID:         "channel-a",
								TemplateVersionID: "tpl-a",
								Enabled:           true,
								SortOrder:         10,
							},
						},
					},
				},
			},
		},
	}
	handler := httpapi.NewHandler(
		testConfig(),
		httpapi.WithAuthService(fakeAuthService{authenticatedToken: "admin-session"}),
		httpapi.WithRouteService(routeService),
	)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/route-flows/flow-1/rules", strings.NewReader(`{
		"rules": [{
			"rule_key": "00000000-0000-0000-0000-000000000301",
			"sort_order": 10,
			"name": "Critical",
			"condition_tree": {"operator":"always"},
			"enabled": true,
			"action": {
				"targets": [{
					"channel_id": "channel-a",
					"template_version_id": "tpl-a",
					"enabled": true
				}],
				"recipient_strategy": {},
				"send_dedupe_config": {},
				"failure_policy": {}
			}
		}]
	}`))
	setAdminSessionCookie(req, "admin-session")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected save rules with targets to return 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if routeService.saveRulesCalls != 1 || routeService.saveRulesFlowID != "flow-1" {
		t.Fatalf("expected one save rules call for flow-1, got calls=%d flow=%s", routeService.saveRulesCalls, routeService.saveRulesFlowID)
	}
	savedTargets := routeService.saveRulesInput.Rules[0].Action.Targets
	if len(savedTargets) != 1 || savedTargets[0].ChannelID != "channel-a" || savedTargets[0].TemplateVersionID != "tpl-a" || !savedTargets[0].Enabled {
		t.Fatalf("expected request targets to map into route input, got %+v", savedTargets)
	}

	var savedBody routeRulesJSONBody
	if err := json.NewDecoder(rec.Body).Decode(&savedBody); err != nil {
		t.Fatalf("decode saved route rules response: %v", err)
	}
	assertRouteRuleTargetResponse(t, savedBody)

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/route-flows/flow-1/rules", nil)
	setAdminSessionCookie(getReq, "admin-session")
	getRec := httptest.NewRecorder()
	handler.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected get rules to return 200, got %d body=%s", getRec.Code, getRec.Body.String())
	}
	var getBody routeRulesJSONBody
	if err := json.NewDecoder(getRec.Body).Decode(&getBody); err != nil {
		t.Fatalf("decode get route rules response: %v", err)
	}
	assertRouteRuleTargetResponse(t, getBody)
}

func TestRouteRulesAcceptsLegacyActionFields(t *testing.T) {
	routeService := &fakeRouteService{rulesResult: route.RuleSet{VersionID: "draft-1", Rules: []route.Rule{}}}
	handler := httpapi.NewHandler(
		testConfig(),
		httpapi.WithAuthService(fakeAuthService{authenticatedToken: "admin-session"}),
		httpapi.WithRouteService(routeService),
	)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/route-flows/flow-1/rules", strings.NewReader(`{
		"rules": [{
			"rule_key": "00000000-0000-0000-0000-000000000301",
			"sort_order": 10,
			"name": "Critical",
			"condition_tree": {"operator":"always"},
			"enabled": true,
			"action": {
				"template_version_id": "tpl-legacy",
				"channel_ids": ["channel-a"],
				"recipient_strategy": {},
				"send_dedupe_config": {},
				"failure_policy": {}
			}
		}]
	}`))
	setAdminSessionCookie(req, "admin-session")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected save rules with legacy action fields to return 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	action := routeService.saveRulesInput.Rules[0].Action
	if action.TemplateVersionID != "tpl-legacy" || len(action.ChannelIDs) != 1 || action.ChannelIDs[0] != "channel-a" {
		t.Fatalf("expected legacy action fields to map into route input, got %+v", action)
	}
	if len(action.Targets) != 0 {
		t.Fatalf("expected handler to leave legacy payload normalization to route service, got targets %+v", action.Targets)
	}
}

type routeRulesJSONBody struct {
	VersionID string `json:"version_id"`
	Rules     []struct {
		Action struct {
			TemplateVersionID string   `json:"template_version_id"`
			ChannelIDs        []string `json:"channel_ids"`
			Targets           []struct {
				ID                string `json:"id"`
				ChannelID         string `json:"channel_id"`
				TemplateVersionID string `json:"template_version_id"`
				Enabled           bool   `json:"enabled"`
				SortOrder         int    `json:"sort_order"`
			} `json:"targets"`
		} `json:"action"`
	} `json:"rules"`
}

func assertRouteRuleTargetResponse(t *testing.T, body routeRulesJSONBody) {
	t.Helper()

	if body.VersionID != "draft-1" || len(body.Rules) != 1 {
		t.Fatalf("unexpected route rules response: %+v", body)
	}
	action := body.Rules[0].Action
	if action.TemplateVersionID != "tpl-a" || len(action.ChannelIDs) != 1 || action.ChannelIDs[0] != "channel-a" {
		t.Fatalf("expected response to keep legacy action fields, got %+v", action)
	}
	if len(action.Targets) != 1 {
		t.Fatalf("expected response action targets, got %+v", action.Targets)
	}
	target := action.Targets[0]
	if target.ID != "target-a" || target.ChannelID != "channel-a" || target.TemplateVersionID != "tpl-a" || !target.Enabled || target.SortOrder != 10 {
		t.Fatalf("unexpected response action target: %+v", target)
	}
}

func TestRouteVersionRulesReturnsReadOnlyHistoricalRules(t *testing.T) {
	routeService := &fakeRouteService{
		versionRulesResult: route.RuleSet{
			VersionID: "version-1",
			Rules: []route.Rule{
				{
					ID:            "rule-id",
					RuleKey:       "00000000-0000-0000-0000-000000000301",
					SortOrder:     1,
					Name:          "Published Critical",
					ConditionTree: json.RawMessage(`{"operator":"always"}`),
					Enabled:       true,
					Action: route.Action{
						Targets: []route.ActionTarget{
							{
								ID:                "target-a",
								ChannelID:         "channel-a",
								TemplateVersionID: "tpl-a",
								Enabled:           true,
								SortOrder:         1,
							},
						},
					},
				},
			},
		},
	}
	handler := httpapi.NewHandler(
		testConfig(),
		httpapi.WithAuthService(fakeAuthService{authenticatedToken: "admin-session"}),
		httpapi.WithRouteService(routeService),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/route-flows/flow-1/versions/version-1/rules", nil)
	setAdminSessionCookie(req, "admin-session")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		VersionID string `json:"version_id"`
		Rules     []struct {
			Name string `json:"name"`
		} `json:"rules"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode version rules response: %v", err)
	}
	if body.VersionID != "version-1" || len(body.Rules) != 1 || body.Rules[0].Name != "Published Critical" {
		t.Fatalf("unexpected version rule response: %+v", body)
	}
	if routeService.versionRulesFlowID != "flow-1" || routeService.versionRulesVersionID != "version-1" {
		t.Fatalf("expected service to receive flow/version ids, got flow=%q version=%q", routeService.versionRulesFlowID, routeService.versionRulesVersionID)
	}
}

func TestRouteVersionCheckoutCopiesHistoricalVersionIntoWorkingDraft(t *testing.T) {
	routeService := &fakeRouteService{
		checkoutResult: route.RuleSet{
			VersionID:          "draft-4",
			DraftBaseVersionID: "version-2",
			DraftBaseVersionNo: 2,
			Rules: []route.Rule{{
				ID:        "rule-copy",
				RuleKey:   "00000000-0000-0000-0000-000000000301",
				SortOrder: 1,
				Name:      "从 v2 检出的规则",
				Enabled:   true,
			}},
		},
	}
	handler := httpapi.NewHandler(
		testConfig(),
		httpapi.WithAuthService(fakeAuthService{authenticatedToken: "admin-session"}),
		httpapi.WithRouteService(routeService),
	)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/route-flows/flow-1/versions/version-2/checkout", nil)
	setAdminSessionCookie(req, "admin-session")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if routeService.checkoutFlowID != "flow-1" || routeService.checkoutVersionID != "version-2" {
		t.Fatalf("expected service to checkout flow/version ids, got flow=%q version=%q", routeService.checkoutFlowID, routeService.checkoutVersionID)
	}
	var body struct {
		VersionID          string `json:"version_id"`
		DraftBaseVersionID string `json:"draft_base_version_id"`
		DraftBaseVersionNo int    `json:"draft_base_version_no"`
		Rules              []struct {
			Name string `json:"name"`
		} `json:"rules"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode checkout response: %v", err)
	}
	if body.VersionID != "draft-4" || body.DraftBaseVersionID != "version-2" || body.DraftBaseVersionNo != 2 {
		t.Fatalf("unexpected checkout response metadata: %+v", body)
	}
	if len(body.Rules) != 1 || body.Rules[0].Name != "从 v2 检出的规则" {
		t.Fatalf("unexpected checkout rules: %+v", body.Rules)
	}
}

func TestDeleteRouteVersionRemovesHistoricalVersion(t *testing.T) {
	routeService := &fakeRouteService{}
	handler := httpapi.NewHandler(
		testConfig(),
		httpapi.WithAuthService(fakeAuthService{authenticatedToken: "admin-session"}),
		httpapi.WithRouteService(routeService),
	)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/route-flows/flow-1/versions/version-2", nil)
	setAdminSessionCookie(req, "admin-session")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if routeService.deleteVersionFlowID != "flow-1" || routeService.deleteVersionVersionID != "version-2" {
		t.Fatalf("expected service to delete flow/version ids, got flow=%q version=%q", routeService.deleteVersionFlowID, routeService.deleteVersionVersionID)
	}
}

type fakeRouteService struct {
	mu sync.Mutex

	listResult          []route.Flow
	getResult           route.Flow
	createResult        route.Flow
	updateResult        route.Flow
	versionsResult      []route.Version
	canvasResult        route.CanvasState
	rulesResult         route.RuleSet
	versionRulesResult  route.RuleSet
	checkoutResult      route.RuleSet
	validateResult      route.ValidationResult
	publishResult       route.Version
	simulateResult      route.SimulationResult
	simulateStarted     chan struct{}
	simulateBlock       chan struct{}
	simulateStartedOnce sync.Once

	saveRulesInput         route.SaveRulesInput
	saveRulesInputs        []route.SaveRulesInput
	saveRulesFlowID        string
	versionRulesFlowID     string
	versionRulesVersionID  string
	checkoutFlowID         string
	checkoutVersionID      string
	deleteVersionFlowID    string
	deleteVersionVersionID string
	createInput            route.CreateFlowInput
	simulateInputs         []route.SimulateInput
	deletedFlowIDs         []string

	createErr        error
	getErr           error
	updateErr        error
	deleteErr        error
	deleteVersionErr error
	saveRulesErr     error
	validateErr      error
	publishErr       error
	simulateErr      error

	listCalls      int
	createCalls    int
	deleteCalls    int
	saveRulesCalls int
	simulateCalls  int
}

func (f *fakeRouteService) ListFlows(context.Context) ([]route.Flow, error) {
	f.listCalls++
	return f.listResult, nil
}

func (f *fakeRouteService) CreateFlow(_ context.Context, input route.CreateFlowInput) (route.Flow, error) {
	f.createCalls++
	f.createInput = input
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

func (f *fakeRouteService) DeleteFlow(_ context.Context, id string) error {
	f.deleteCalls++
	f.deletedFlowIDs = append(f.deletedFlowIDs, id)
	return f.deleteErr
}

func (f *fakeRouteService) DeleteVersion(_ context.Context, flowID string, versionID string) error {
	f.deleteVersionFlowID = flowID
	f.deleteVersionVersionID = versionID
	return f.deleteVersionErr
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

func (f *fakeRouteService) GetVersionRules(_ context.Context, flowID string, versionID string) (route.RuleSet, error) {
	f.versionRulesFlowID = flowID
	f.versionRulesVersionID = versionID
	return f.versionRulesResult, nil
}

func (f *fakeRouteService) CheckoutVersion(_ context.Context, flowID string, versionID string) (route.RuleSet, error) {
	f.checkoutFlowID = flowID
	f.checkoutVersionID = versionID
	return f.checkoutResult, nil
}

func (f *fakeRouteService) SaveRules(_ context.Context, flowID string, input route.SaveRulesInput) (route.RuleSet, error) {
	f.saveRulesCalls++
	f.saveRulesFlowID = flowID
	f.saveRulesInput = input
	f.saveRulesInputs = append(f.saveRulesInputs, input)
	if f.saveRulesErr != nil {
		return route.RuleSet{}, f.saveRulesErr
	}
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

func (f *fakeRouteService) Publish(context.Context, string, ...string) (route.Version, error) {
	if f.publishErr != nil {
		return route.Version{}, f.publishErr
	}
	return f.publishResult, nil
}

func (f *fakeRouteService) Simulate(ctx context.Context, _ string, input route.SimulateInput) (route.SimulationResult, error) {
	f.mu.Lock()
	f.simulateCalls++
	f.simulateInputs = append(f.simulateInputs, input)
	if f.simulateErr != nil {
		f.mu.Unlock()
		return route.SimulationResult{}, f.simulateErr
	}
	if f.simulateStarted != nil {
		f.simulateStartedOnce.Do(func() {
			close(f.simulateStarted)
		})
	}
	f.mu.Unlock()
	if f.simulateBlock != nil {
		select {
		case <-ctx.Done():
			return route.SimulationResult{}, ctx.Err()
		case <-f.simulateBlock:
		}
	}
	return f.simulateResult, nil
}

func (f *fakeRouteService) ActivateVersion(context.Context, string, string) (route.Flow, error) {
	if f.getErr != nil {
		return route.Flow{}, errors.New("activate failed")
	}
	return route.Flow{ID: "flow-1", CurrentVersionID: "version-1"}, nil
}
