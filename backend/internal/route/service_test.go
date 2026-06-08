package route

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestSaveRulesGeneratesRuleKeyAndFirstMatchStopsSimulation(t *testing.T) {
	store := newMemoryStore()
	service := NewService(
		store,
		WithIDGenerator(func() string { return "generated-rule-key" }),
	)

	_, err := service.SaveRules(context.Background(), "flow-1", SaveRulesInput{
		Rules: []RuleInput{
			{
				Name:          "标题命中",
				ConditionTree: json.RawMessage(`{"operator":"equals","path":"payload.title","value":"critical"}`),
				Enabled:       true,
				Action: ActionInput{
					TemplateVersionID: "tpl-1",
					ChannelIDs:        []string{"channel-a"},
				},
			},
			{
				RuleKey:       "rule-second",
				SortOrder:     20,
				Name:          "IP 包含",
				ConditionTree: json.RawMessage(`{"operator":"contains","path":"payload.alert.ip","value":"10."}`),
				Enabled:       true,
				Action: ActionInput{
					TemplateVersionID: "tpl-2",
					ChannelIDs:        []string{"channel-b"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("save rules: %v", err)
	}

	saved, err := service.GetRules(context.Background(), "flow-1")
	if err != nil {
		t.Fatalf("get rules: %v", err)
	}
	if len(saved.Rules) != 2 {
		t.Fatalf("expected 2 saved rules, got %d", len(saved.Rules))
	}
	if saved.Rules[0].RuleKey != "generated-rule-key" {
		t.Fatalf("expected generated rule key to be kept, got %q", saved.Rules[0].RuleKey)
	}

	result, err := service.Simulate(context.Background(), "flow-1", SimulateInput{
		Payload: json.RawMessage(`{"title":"critical","alert":{"ip":"10.1.2.3"}}`),
	})
	if err != nil {
		t.Fatalf("simulate: %v", err)
	}
	if result.StopReason != "first_match_stop" {
		t.Fatalf("expected first_match_stop, got %q", result.StopReason)
	}
	if result.MatchedRule == nil || result.MatchedRule.RuleKey != "generated-rule-key" {
		t.Fatalf("expected first generated rule to match, got %+v", result.MatchedRule)
	}
	if len(result.RuleResults) != 2 {
		t.Fatalf("expected 2 rule results, got %d", len(result.RuleResults))
	}
	if !result.RuleResults[0].Matched || !result.RuleResults[0].Evaluated {
		t.Fatalf("expected first rule to be evaluated and matched, got %+v", result.RuleResults[0])
	}
	if result.RuleResults[1].Evaluated {
		t.Fatalf("expected second rule not to be evaluated after first match stop, got %+v", result.RuleResults[1])
	}
	if result.RuleResults[1].StopReason != "first_match_stop" {
		t.Fatalf("expected second rule stop reason first_match_stop, got %+v", result.RuleResults[1])
	}
}

func TestSimulateCoarseSkipsRulesWhenRequiredPayloadFieldIsMissing(t *testing.T) {
	service := NewService(newMemoryStore())

	_, err := service.SaveRules(context.Background(), "flow-1", SaveRulesInput{
		Rules: []RuleInput{
			{
				RuleKey:       "rule-missing-field",
				SortOrder:     1,
				Name:          "缺字段规则",
				ConditionTree: json.RawMessage(`{"operator":"equals","path":"payload.severity","value":"critical"}`),
				Enabled:       true,
				Action: ActionInput{
					Targets: []ActionTargetInput{{ChannelID: "channel-1", TemplateVersionID: "template-1", Enabled: true}},
				},
			},
			{
				RuleKey:       "rule-fallback",
				SortOrder:     2,
				Name:          "兜底规则",
				ConditionTree: json.RawMessage(`{"operator":"always"}`),
				Enabled:       true,
				Action: ActionInput{
					Targets: []ActionTargetInput{{ChannelID: "channel-1", TemplateVersionID: "template-1", Enabled: true}},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("save rules: %v", err)
	}

	result, err := service.Simulate(context.Background(), "flow-1", SimulateInput{
		Payload: json.RawMessage(`{"title":"critical"}`),
	})
	if err != nil {
		t.Fatalf("simulate: %v", err)
	}
	if len(result.RuleResults) != 2 {
		t.Fatalf("expected 2 rule results, got %d", len(result.RuleResults))
	}
	missing := result.RuleResults[0]
	if !missing.CoarseSkipped || missing.Evaluated || missing.StopReason != "coarse_filter" {
		t.Fatalf("expected first rule to be coarse skipped, got %+v", missing)
	}
	if missing.SkipReason != "missing_field:payload.severity" {
		t.Fatalf("expected missing field skip reason, got %+v", missing)
	}
	if result.MatchedRule == nil || result.MatchedRule.RuleKey != "rule-fallback" {
		t.Fatalf("expected fallback rule to match, got %+v", result.MatchedRule)
	}
}

func TestRouteSaveRulesAcceptsActionTargets(t *testing.T) {
	service := NewService(newMemoryStore())

	saved, err := service.SaveRules(context.Background(), "flow-1", SaveRulesInput{
		Rules: []RuleInput{
			{
				RuleKey:       "rule-a",
				SortOrder:     10,
				Name:          "发送动作组",
				ConditionTree: json.RawMessage(`{"operator":"always"}`),
				Enabled:       true,
				Action: ActionInput{
					Targets: []ActionTargetInput{
						{ChannelID: "channel-a", TemplateVersionID: "tpl-a", Enabled: true},
						{ChannelID: "channel-b", TemplateVersionID: "tpl-b", Enabled: false},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("save rules with targets: %v", err)
	}
	action := saved.Rules[0].Action
	if len(action.Targets) != 2 {
		t.Fatalf("expected 2 action targets, got %+v", action.Targets)
	}
	if action.Targets[0].ChannelID != "channel-a" || action.Targets[0].TemplateVersionID != "tpl-a" || !action.Targets[0].Enabled || action.Targets[0].SortOrder != 10 {
		t.Fatalf("unexpected first target: %+v", action.Targets[0])
	}
	if action.Targets[1].ChannelID != "channel-b" || action.Targets[1].TemplateVersionID != "tpl-b" || action.Targets[1].Enabled || action.Targets[1].SortOrder != 20 {
		t.Fatalf("unexpected second target: %+v", action.Targets[1])
	}
	if action.TemplateVersionID != "tpl-a" {
		t.Fatalf("expected compatibility template tpl-a, got %q", action.TemplateVersionID)
	}
	if got := action.ChannelIDs; len(got) != 2 || got[0] != "channel-a" || got[1] != "channel-b" {
		t.Fatalf("expected compatibility channels from targets, got %+v", got)
	}
}

func TestRouteSaveRulesConvertsLegacyActionFieldsToTargets(t *testing.T) {
	service := NewService(newMemoryStore())

	saved, err := service.SaveRules(context.Background(), "flow-1", SaveRulesInput{
		Rules: []RuleInput{
			{
				RuleKey:       "rule-a",
				SortOrder:     10,
				Name:          "兼容旧动作",
				ConditionTree: json.RawMessage(`{"operator":"always"}`),
				Enabled:       true,
				Action: ActionInput{
					TemplateVersionID: "tpl-legacy",
					ChannelIDs:        []string{"channel-a", "channel-b"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("save rules with legacy action: %v", err)
	}
	targets := saved.Rules[0].Action.Targets
	if len(targets) != 2 {
		t.Fatalf("expected 2 targets converted from legacy fields, got %+v", targets)
	}
	if targets[0].ChannelID != "channel-a" || targets[0].TemplateVersionID != "tpl-legacy" || !targets[0].Enabled {
		t.Fatalf("unexpected first legacy target: %+v", targets[0])
	}
	if targets[1].ChannelID != "channel-b" || targets[1].TemplateVersionID != "tpl-legacy" || !targets[1].Enabled {
		t.Fatalf("unexpected second legacy target: %+v", targets[1])
	}
}

func TestRouteSaveRulesAllowsIncompleteDraftActionTargets(t *testing.T) {
	service := NewService(newMemoryStore())

	saved, err := service.SaveRules(context.Background(), "flow-1", SaveRulesInput{
		Rules: []RuleInput{
			{
				RuleKey:       "rule-a",
				SortOrder:     10,
				Name:          "无发送目标",
				ConditionTree: json.RawMessage(`{"operator":"always"}`),
				Enabled:       true,
				Action:        ActionInput{},
			},
		},
	})
	if err != nil {
		t.Fatalf("save incomplete draft rule: %v", err)
	}
	if len(saved.Rules) != 1 || len(saved.Rules[0].Action.Targets) != 0 {
		t.Fatalf("expected incomplete draft rule to be saved without targets, got %+v", saved.Rules)
	}
	result, err := service.Validate(context.Background(), "flow-1")
	if err != nil {
		t.Fatalf("validate incomplete draft: %v", err)
	}
	if result.Status != "invalid" {
		t.Fatalf("expected incomplete draft to stay invalid until targets are configured, got %+v", result)
	}
}

func TestCheckoutVersionReplacesDraftAndRecordsBaseVersion(t *testing.T) {
	store := newMemoryStore()
	store.checkoutDraft = Draft{
		Version: Version{
			ID:                 "draft-4",
			FlowID:             "flow-1",
			VersionNo:          4,
			DraftBaseVersionID: "version-2",
			DraftBaseVersionNo: 2,
		},
		Rules: []Rule{{
			ID:      "rule-copy",
			RuleKey: "rule-a",
			Name:    "从 v2 检出的规则",
		}},
	}
	service := NewService(store)

	ruleSet, err := service.CheckoutVersion(context.Background(), "flow-1", "version-2")
	if err != nil {
		t.Fatalf("checkout version: %v", err)
	}
	if store.checkoutFlowID != "flow-1" || store.checkoutVersionID != "version-2" {
		t.Fatalf("expected checkout flow/version ids, got flow=%q version=%q", store.checkoutFlowID, store.checkoutVersionID)
	}
	if ruleSet.VersionID != "draft-4" || ruleSet.DraftBaseVersionID != "version-2" || ruleSet.DraftBaseVersionNo != 2 {
		t.Fatalf("expected draft v4 based on v2, got %+v", ruleSet)
	}
	if len(ruleSet.Rules) != 1 || ruleSet.Rules[0].Name != "从 v2 检出的规则" {
		t.Fatalf("expected checked out rules to be returned, got %+v", ruleSet.Rules)
	}
}

func TestRouteSaveRulesDeduplicatesActionTargets(t *testing.T) {
	service := NewService(newMemoryStore())

	saved, err := service.SaveRules(context.Background(), "flow-1", SaveRulesInput{
		Rules: []RuleInput{
			{
				RuleKey:       "rule-a",
				SortOrder:     10,
				Name:          "目标去重",
				ConditionTree: json.RawMessage(`{"operator":"always"}`),
				Enabled:       true,
				Action: ActionInput{
					Targets: []ActionTargetInput{
						{ChannelID: " channel-a ", TemplateVersionID: " tpl-a ", Enabled: true},
						{ChannelID: "channel-a", TemplateVersionID: "tpl-a", Enabled: true},
						{ChannelID: "", TemplateVersionID: "tpl-a", Enabled: true},
						{ChannelID: "channel-a", TemplateVersionID: "tpl-b", Enabled: true},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("save rules with duplicate targets: %v", err)
	}
	targets := saved.Rules[0].Action.Targets
	if len(targets) != 2 {
		t.Fatalf("expected duplicate target pair to collapse to 2 targets, got %+v", targets)
	}
	if targets[0].ChannelID != "channel-a" || targets[0].TemplateVersionID != "tpl-a" {
		t.Fatalf("expected first target to be trimmed channel-a/tpl-a, got %+v", targets[0])
	}
	if targets[1].ChannelID != "channel-a" || targets[1].TemplateVersionID != "tpl-b" {
		t.Fatalf("expected second distinct target channel-a/tpl-b, got %+v", targets[1])
	}
}

func TestReorderRulesChangesSimulationOrder(t *testing.T) {
	store := newMemoryStore()
	service := NewService(store)

	_, err := service.SaveRules(context.Background(), "flow-1", SaveRulesInput{
		Rules: []RuleInput{
			{
				RuleKey:       "rule-a",
				SortOrder:     10,
				Name:          "标题命中",
				ConditionTree: json.RawMessage(`{"operator":"equals","path":"payload.title","value":"critical"}`),
				Enabled:       true,
				Action:        validActionInput(),
			},
			{
				RuleKey:       "rule-b",
				SortOrder:     20,
				Name:          "标题包含",
				ConditionTree: json.RawMessage(`{"operator":"contains","path":"payload.title","value":"crit"}`),
				Enabled:       true,
				Action:        validActionInput(),
			},
		},
	})
	if err != nil {
		t.Fatalf("save rules: %v", err)
	}

	_, err = service.ReorderRules(context.Background(), "flow-1", ReorderRulesInput{
		RuleKeys: []string{"rule-b", "rule-a"},
	})
	if err != nil {
		t.Fatalf("reorder rules: %v", err)
	}

	result, err := service.Simulate(context.Background(), "flow-1", SimulateInput{
		Payload: json.RawMessage(`{"title":"critical"}`),
	})
	if err != nil {
		t.Fatalf("simulate after reorder: %v", err)
	}
	if result.MatchedRule == nil || result.MatchedRule.RuleKey != "rule-b" {
		t.Fatalf("expected reordered first rule to match, got %+v", result.MatchedRule)
	}
	if len(result.RuleResults) != 2 || result.RuleResults[0].RuleKey != "rule-b" {
		t.Fatalf("expected reordered execution order in trace, got %+v", result.RuleResults)
	}
	reordered, err := service.GetRules(context.Background(), "flow-1")
	if err != nil {
		t.Fatalf("get reordered rules: %v", err)
	}
	if got := []int{reordered.Rules[0].SortOrder, reordered.Rules[1].SortOrder}; got[0] != 1 || got[1] != 2 {
		t.Fatalf("expected display-friendly sort orders 1/2 after reorder, got %+v", got)
	}
}

func TestValidateHandlesMissingFieldWithoutPanic(t *testing.T) {
	store := newMemoryStore()
	service := NewService(store)

	_, err := service.SaveRules(context.Background(), "flow-1", SaveRulesInput{
		Rules: []RuleInput{
			{
				RuleKey:       "rule-a",
				SortOrder:     10,
				Name:          "字段存在",
				ConditionTree: json.RawMessage(`{"operator":"exists","path":"payload.alert.ip"}`),
				Enabled:       true,
				Action:        validActionInput(),
			},
			{
				RuleKey:       "rule-b",
				SortOrder:     20,
				Name:          "不包含",
				ConditionTree: json.RawMessage(`{"operator":"not_contains","path":"payload.title","value":"debug"}`),
				Enabled:       true,
				Action:        validActionInput(),
			},
			{
				RuleKey:       "rule-c",
				SortOrder:     30,
				Name:          "集合包含",
				ConditionTree: json.RawMessage(`{"operator":"in","path":"payload.severity","values":["critical","high"]}`),
				Enabled:       true,
				Action:        validActionInput(),
			},
		},
	})
	if err != nil {
		t.Fatalf("save rules: %v", err)
	}

	validation, err := service.Validate(context.Background(), "flow-1")
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if validation.Status != "valid" {
		t.Fatalf("expected validation valid, got %+v", validation)
	}

	result, err := service.Simulate(context.Background(), "flow-1", SimulateInput{
		Payload: json.RawMessage(`{"title":"release","severity":"high"}`),
	})
	if err != nil {
		t.Fatalf("simulate with missing path: %v", err)
	}
	if result.MatchedRule == nil || result.MatchedRule.RuleKey != "rule-b" {
		t.Fatalf("expected rule-b to match when payload.alert.ip is absent, got %+v", result.MatchedRule)
	}
}

func TestEvaluateConditionTreeSupportsDocumentedOperators(t *testing.T) {
	scope := map[string]any{
		"payload": map[string]any{
			"title":    "critical disk alert",
			"severity": "critical",
			"count":    float64(12),
			"ip":       "10.1.2.3",
		},
	}
	tests := []struct {
		name       string
		condition  string
		matchGroup map[string][]string
	}{
		{name: "not equals", condition: `{"operator":"not_equals","path":"payload.severity","value":"info"}`},
		{name: "not exists", condition: `{"operator":"not_exists","path":"payload.missing"}`},
		{name: "regex", condition: `{"operator":"regex","path":"payload.title","value":"disk\\s+alert"}`},
		{name: "greater than", condition: `{"operator":"gt","path":"payload.count","value":10}`},
		{name: "greater than or equal", condition: `{"operator":"gte","path":"payload.count","value":12}`},
		{name: "less than", condition: `{"operator":"lt","path":"payload.count","value":13}`},
		{name: "less than or equal", condition: `{"operator":"lte","path":"payload.count","value":12}`},
		{name: "cidr match group", condition: `{"operator":"in_match_group","path":"payload.ip","match_group_id":"group-ip"}`, matchGroup: map[string][]string{"group-ip": {"10.1.0.0/16"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matched, err := EvaluateConditionTreeWithMatchGroups(json.RawMessage(tt.condition), scope, tt.matchGroup)
			if err != nil {
				t.Fatalf("evaluate condition: %v", err)
			}
			if !matched {
				t.Fatalf("expected %s to match", tt.name)
			}
		})
	}
}

func TestSimulateUsesMatchGroupValuesFromStore(t *testing.T) {
	store := newMemoryStore()
	store.matchGroups = map[string][]string{
		"group-severity": {"critical", "high"},
	}
	service := NewService(store)
	if _, err := service.SaveRules(context.Background(), "flow-1", SaveRulesInput{Rules: []RuleInput{{
		RuleKey:       "rule-severity",
		SortOrder:     10,
		Name:          "严重等级",
		ConditionTree: json.RawMessage(`{"operator":"in_match_group","path":"payload.severity","match_group_id":"group-severity"}`),
		Enabled:       true,
		Action:        validActionInput(),
	}}}); err != nil {
		t.Fatalf("save rules: %v", err)
	}

	result, err := service.Simulate(context.Background(), "flow-1", SimulateInput{
		Payload: json.RawMessage(`{"severity":"critical"}`),
	})
	if err != nil {
		t.Fatalf("simulate: %v", err)
	}
	if result.MatchedRule == nil || result.MatchedRule.RuleKey != "rule-severity" {
		t.Fatalf("expected match group backed rule to match, got %+v", result)
	}
}

func TestValidateRejectsRuleWithNoEnabledTargets(t *testing.T) {
	store := newMemoryStore()
	service := NewService(store)
	if _, err := service.SaveRules(context.Background(), "flow-1", SaveRulesInput{Rules: []RuleInput{{
		RuleKey:       "rule-disabled-target",
		SortOrder:     10,
		Name:          "停用目标",
		ConditionTree: json.RawMessage(`{"operator":"always"}`),
		Enabled:       true,
		Action: ActionInput{Targets: []ActionTargetInput{
			{ChannelID: "channel-a", TemplateVersionID: "tpl-a", Enabled: false},
		}},
	}}}); err != nil {
		t.Fatalf("save rules: %v", err)
	}

	result, err := service.Validate(context.Background(), "flow-1")
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if result.Status != "invalid" {
		t.Fatalf("expected invalid route when no target is enabled, got %+v", result)
	}
}

func TestPublishRejectsReferenceValidationErrors(t *testing.T) {
	store := newMemoryStore()
	store.referenceErrors = []ValidationError{{
		Code:    "MGP-ROUTE-REF",
		Message: "发送目标引用的模板不存在或未发布",
		Path:    "rule-a.targets[0]",
	}}
	service := NewService(store)
	if _, err := service.SaveRules(context.Background(), "flow-1", SaveRulesInput{Rules: []RuleInput{{
		RuleKey:       "rule-a",
		SortOrder:     10,
		Name:          "引用缺失",
		ConditionTree: json.RawMessage(`{"operator":"always"}`),
		Enabled:       true,
		Action:        validActionInput(),
	}}}); err != nil {
		t.Fatalf("save rules: %v", err)
	}

	result, err := service.Validate(context.Background(), "flow-1")
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if result.Status != "invalid" || len(result.Errors) != 1 || result.Errors[0].Code != "MGP-ROUTE-REF" {
		t.Fatalf("expected reference validation error, got %+v", result)
	}
	if _, err := service.Publish(context.Background(), "flow-1"); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("expected publish to reject reference errors with ErrInvalidConfig, got %v", err)
	}
	if store.publishCalls != 0 {
		t.Fatalf("expected publish store call to be skipped, got %d", store.publishCalls)
	}
}

func TestRouteServiceBroadcastsRoutePlanChangeAfterExecutionConfigChanges(t *testing.T) {
	store := newMemoryStore()
	publisher := &recordingChangePublisher{}
	service := NewService(store, WithChangePublisher(publisher))

	if _, err := service.Publish(context.Background(), "flow-1"); err != nil {
		t.Fatalf("publish route: %v", err)
	}
	if _, err := service.ActivateVersion(context.Background(), "flow-1", "version-1"); err != nil {
		t.Fatalf("activate route version: %v", err)
	}
	if _, err := service.UpdateFlow(context.Background(), "flow-1", UpdateFlowInput{
		SourceID: "source-2",
		Name:     "Flow 1",
		Enabled:  true,
		Mode:     ModeTable,
	}); err != nil {
		t.Fatalf("update route flow: %v", err)
	}
	if err := service.DeleteFlow(context.Background(), "flow-1"); err != nil {
		t.Fatalf("delete route flow: %v", err)
	}

	expected := []string{"source-1", "source-1", "source-1", "source-2", "source-2"}
	if !reflect.DeepEqual(publisher.sourceIDs, expected) {
		t.Fatalf("expected route change broadcasts %+v, got %+v", expected, publisher.sourceIDs)
	}
}

func TestRulesFromCompiledPreferPublishedExecutionModel(t *testing.T) {
	persisted := []Rule{{
		ID:            "rule-row-id",
		FlowID:        "flow-1",
		VersionID:     "version-1",
		RuleKey:       "rule-a",
		SortOrder:     10,
		Name:          "关系表规则",
		ConditionTree: json.RawMessage(`{"operator":"equals","path":"payload.kind","value":"table"}`),
		Enabled:       true,
		Action: Action{
			ID:                "action-row-id",
			TemplateVersionID: "tpl-table",
			ChannelIDs:        []string{"channel-table"},
			Targets: []ActionTarget{{
				ID:                "target-row-id",
				ActionID:          "action-row-id",
				ChannelID:         "channel-table",
				TemplateVersionID: "tpl-table",
				Enabled:           true,
				SortOrder:         10,
			}},
		},
	}}
	compiled := json.RawMessage(`{
		"execution_mode": "first_match_stop",
		"rules": [{
			"rule_key": "rule-a",
			"sort_order": 30,
			"name": "发布执行规则",
			"enabled": true,
			"condition_tree": {"operator":"equals","path":"payload.kind","value":"compiled"},
			"action": {
				"targets": [{
					"channel_id": "channel-compiled",
					"template_version_id": "tpl-compiled",
					"enabled": true,
					"sort_order": 10
				}],
				"recipient_strategy": {"mode":"none"},
				"send_dedupe_config": {"strategy":"trace_id"},
				"failure_policy": {"policy":"continue"}
			}
		}]
	}`)

	rules, ok, err := RulesFromCompiled(compiled, persisted)
	if err != nil {
		t.Fatalf("parse compiled rules: %v", err)
	}
	if !ok || len(rules) != 1 {
		t.Fatalf("expected one compiled rule, got ok=%v rules=%+v", ok, rules)
	}
	rule := rules[0]
	if rule.ID != "rule-row-id" || rule.Action.ID != "action-row-id" {
		t.Fatalf("expected persisted identities to be preserved, got %+v", rule)
	}
	if rule.SortOrder != 30 || rule.Name != "发布执行规则" || string(rule.ConditionTree) == string(persisted[0].ConditionTree) {
		t.Fatalf("expected compiled rule data to win, got %+v", rule)
	}
	if len(rule.Action.Targets) != 1 || rule.Action.Targets[0].ChannelID != "channel-compiled" || rule.Action.TemplateVersionID != "tpl-compiled" {
		t.Fatalf("expected compiled action target to win, got %+v", rule.Action)
	}
}

func TestRulesFromCompiledKeepsPersistedFieldsForPartialCompiledRules(t *testing.T) {
	persisted := []Rule{{
		ID:            "rule-row-id",
		FlowID:        "flow-1",
		VersionID:     "version-1",
		RuleKey:       "rule-a",
		SortOrder:     10,
		Name:          "持久化规则",
		ConditionTree: json.RawMessage(`{"operator":"equals","path":"payload.kind","value":"table"}`),
		Enabled:       true,
		Action: Action{
			ID:                "action-row-id",
			RuleID:            "rule-row-id",
			TemplateVersionID: "tpl-table",
			ChannelIDs:        []string{"channel-table"},
			Targets: []ActionTarget{{
				ID:                "target-row-id",
				ActionID:          "action-row-id",
				ChannelID:         "channel-table",
				TemplateVersionID: "tpl-table",
				Enabled:           true,
				SortOrder:         10,
			}},
			RecipientStrategy: json.RawMessage(`{"mode":"payload","path":"payload.to"}`),
			SendDedupeConfig:  json.RawMessage(`{"strategy":"trace_id"}`),
			FailurePolicy:     json.RawMessage(`{"policy":"continue"}`),
		},
	}}
	compiled := json.RawMessage(`{
		"execution_mode": "first_match_stop",
		"rules": [{"rule_key": "rule-a"}]
	}`)

	rules, ok, err := RulesFromCompiled(compiled, persisted)
	if err != nil {
		t.Fatalf("parse compiled rules: %v", err)
	}
	if !ok || len(rules) != 1 {
		t.Fatalf("expected one rule from partial compiled envelope, got ok=%v rules=%+v", ok, rules)
	}
	rule := rules[0]
	if rule.SortOrder != persisted[0].SortOrder || rule.Name != persisted[0].Name || string(rule.ConditionTree) != string(persisted[0].ConditionTree) {
		t.Fatalf("expected persisted rule fields to survive partial compiled envelope, got %+v", rule)
	}
	if len(rule.Action.Targets) != 1 || rule.Action.Targets[0].ChannelID != "channel-table" || string(rule.Action.RecipientStrategy) != string(persisted[0].Action.RecipientStrategy) {
		t.Fatalf("expected persisted action to survive partial compiled envelope, got %+v", rule.Action)
	}
}

func validActionInput() ActionInput {
	return ActionInput{
		Targets: []ActionTargetInput{
			{ChannelID: "channel-a", TemplateVersionID: "tpl-a", Enabled: true},
		},
	}
}

type memoryStore struct {
	flow            Flow
	draft           Draft
	checkoutDraft   Draft
	matchGroups     map[string][]string
	referenceErrors []ValidationError
	publishCalls    int

	checkoutFlowID    string
	checkoutVersionID string
}

type recordingChangePublisher struct {
	sourceIDs []string
}

func (p *recordingChangePublisher) PublishRoutePlanChange(_ context.Context, sourceID string) error {
	p.sourceIDs = append(p.sourceIDs, sourceID)
	return nil
}

func newMemoryStore() *memoryStore {
	return &memoryStore{
		flow: Flow{ID: "flow-1", SourceID: "source-1", Name: "Flow 1", Enabled: true, Mode: ModeTable},
		draft: Draft{
			Version: Version{
				ID:        "draft-1",
				FlowID:    "flow-1",
				VersionNo: 1,
			},
			Rules: []Rule{},
		},
	}
}

func (s *memoryStore) ListFlows(context.Context) ([]Flow, error) {
	return []Flow{s.flow}, nil
}

func (s *memoryStore) CreateFlow(context.Context, CreateFlowParams) (Flow, error) {
	return s.flow, nil
}

func (s *memoryStore) GetFlow(context.Context, string) (Flow, error) {
	return s.flow, nil
}

func (s *memoryStore) UpdateFlow(_ context.Context, _ string, params UpdateFlowParams) (Flow, error) {
	s.flow.SourceID = params.SourceID
	s.flow.Name = params.Name
	s.flow.Enabled = params.Enabled
	s.flow.Mode = params.Mode
	return s.flow, nil
}

func (s *memoryStore) DeleteFlow(context.Context, string) error {
	return nil
}

func (s *memoryStore) GetDraft(context.Context, string) (Draft, error) {
	return s.draft, nil
}

func (s *memoryStore) ListVersions(context.Context, string) ([]Version, error) {
	return []Version{s.draft.Version}, nil
}

func (s *memoryStore) GetVersionRules(_ context.Context, _ string, versionID string) (RuleSet, error) {
	if s.draft.Version.ID != versionID {
		return RuleSet{}, ErrNotFound
	}
	return RuleSet{VersionID: s.draft.Version.ID, Rules: append([]Rule(nil), s.draft.Rules...)}, nil
}

func (s *memoryStore) CheckoutVersion(_ context.Context, flowID string, versionID string) (Draft, error) {
	s.checkoutFlowID = flowID
	s.checkoutVersionID = versionID
	if s.checkoutDraft.Version.ID == "" {
		return Draft{}, ErrNotFound
	}
	s.draft = s.checkoutDraft
	return s.draft, nil
}

func (s *memoryStore) DeleteVersion(context.Context, string, string) error {
	return nil
}

func (s *memoryStore) UpdateCanvas(_ context.Context, _ string, snapshot json.RawMessage, _ FlowMode) (Draft, error) {
	s.draft.Version.CanvasSnapshot = snapshot
	return s.draft, nil
}

func (s *memoryStore) ReplaceRules(_ context.Context, _ string, _ string, rules []Rule) ([]Rule, error) {
	s.draft.Rules = append([]Rule(nil), rules...)
	return s.draft.Rules, nil
}

func (s *memoryStore) ReorderRules(_ context.Context, _ string, _ string, ruleKeys []string) ([]Rule, error) {
	next := make([]Rule, 0, len(ruleKeys))
	for idx, key := range ruleKeys {
		for _, rule := range s.draft.Rules {
			if rule.RuleKey == key {
				rule.SortOrder = idx + 1
				next = append(next, rule)
			}
		}
	}
	s.draft.Rules = next
	return s.draft.Rules, nil
}

func (s *memoryStore) Publish(context.Context, PublishParams) (Version, error) {
	s.publishCalls++
	return s.draft.Version, nil
}

func (s *memoryStore) ActivateVersion(context.Context, string, string) (Flow, error) {
	return s.flow, nil
}

func (s *memoryStore) IncrementRuleCounter(context.Context, string, string, time.Time) error {
	return nil
}

func (s *memoryStore) LoadMatchGroupValues(_ context.Context, _ []Rule) (map[string][]string, error) {
	if s.matchGroups == nil {
		return map[string][]string{}, nil
	}
	return s.matchGroups, nil
}

func (s *memoryStore) ValidateRuleReferences(context.Context, string, string, []Rule) ([]ValidationError, error) {
	return append([]ValidationError(nil), s.referenceErrors...), nil
}
