package route

import (
	"context"
	"encoding/json"
	"errors"
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

func TestRouteSaveRulesRejectsActionWithoutTargets(t *testing.T) {
	service := NewService(newMemoryStore())

	_, err := service.SaveRules(context.Background(), "flow-1", SaveRulesInput{
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
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for action without targets, got %v", err)
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

func validActionInput() ActionInput {
	return ActionInput{
		Targets: []ActionTargetInput{
			{ChannelID: "channel-a", TemplateVersionID: "tpl-a", Enabled: true},
		},
	}
}

type memoryStore struct {
	flow  Flow
	draft Draft
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

func (s *memoryStore) UpdateFlow(context.Context, string, UpdateFlowParams) (Flow, error) {
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
				rule.SortOrder = (idx + 1) * 10
				next = append(next, rule)
			}
		}
	}
	s.draft.Rules = next
	return s.draft.Rules, nil
}

func (s *memoryStore) Publish(context.Context, PublishParams) (Version, error) {
	return s.draft.Version, nil
}

func (s *memoryStore) ActivateVersion(context.Context, string, string) (Flow, error) {
	return s.flow, nil
}

func (s *memoryStore) IncrementRuleCounter(context.Context, string, string, time.Time) error {
	return nil
}
