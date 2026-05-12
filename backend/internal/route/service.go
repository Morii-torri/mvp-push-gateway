package route

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

type FlowMode string

const (
	ModeCanvas FlowMode = "canvas"
	ModeTable  FlowMode = "table"
)

var (
	ErrNotFound          = errors.New("route flow not found")
	ErrInvalidInput      = errors.New("invalid route input")
	ErrEnabledFlowExists = errors.New("enabled route flow already exists")
	ErrInvalidConfig     = errors.New("invalid route config")
)

type Flow struct {
	ID               string
	SourceID         string
	Name             string
	Enabled          bool
	Mode             FlowMode
	CurrentVersionID string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type Version struct {
	ID               string
	FlowID           string
	VersionNo        int
	CanvasSnapshot   json.RawMessage
	CompiledRules    json.RawMessage
	ValidationStatus string
	ValidationErrors json.RawMessage
	PublishedAt      *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type ActionTarget struct {
	ID                string
	ActionID          string
	ChannelID         string
	TemplateVersionID string
	Enabled           bool
	SortOrder         int
	CreatedAt         time.Time
}

type Action struct {
	ID      string
	RuleID  string
	Targets []ActionTarget
	// Deprecated: use Targets. Kept during API migration.
	TemplateVersionID string
	// Deprecated: use Targets. Kept during API migration.
	ChannelIDs        []string
	RecipientStrategy json.RawMessage
	SendDedupeConfig  json.RawMessage
	FailurePolicy     json.RawMessage
	CreatedAt         time.Time
}

type Rule struct {
	ID            string
	FlowID        string
	VersionID     string
	RuleKey       string
	SortOrder     int
	Name          string
	ConditionTree json.RawMessage
	Enabled       bool
	Action        Action
	HitCount      int
	LastHitAt     *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type Draft struct {
	Version Version
	Rules   []Rule
}

type CanvasState struct {
	VersionID      string          `json:"version_id"`
	CanvasSnapshot json.RawMessage `json:"canvas_snapshot"`
}

type RuleSet struct {
	VersionID string `json:"version_id"`
	Rules     []Rule `json:"rules"`
}

type ValidationError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Path    string `json:"path,omitempty"`
}

type ValidationResult struct {
	VersionID string            `json:"version_id"`
	Status    string            `json:"status"`
	Errors    []ValidationError `json:"errors"`
}

type RuleTrace struct {
	RuleKey       string `json:"rule_key"`
	Name          string `json:"name"`
	SortOrder     int    `json:"sort_order"`
	CoarseSkipped bool   `json:"coarse_skipped"`
	Matched       bool   `json:"matched"`
	Evaluated     bool   `json:"evaluated"`
	DurationMS    int64  `json:"duration_ms"`
	StopReason    string `json:"stop_reason,omitempty"`
}

type SimulationResult struct {
	VersionID   string      `json:"version_id"`
	StopReason  string      `json:"stop_reason"`
	MatchedRule *RuleTrace  `json:"matched_rule"`
	RuleResults []RuleTrace `json:"rule_results"`
}

type CreateFlowInput struct {
	ID       string   `json:"id"`
	SourceID string   `json:"source_id"`
	Name     string   `json:"name"`
	Enabled  bool     `json:"enabled"`
	Mode     FlowMode `json:"mode"`
}

type UpdateFlowInput = CreateFlowInput

type CreateFlowParams = CreateFlowInput
type UpdateFlowParams = UpdateFlowInput

type SaveCanvasInput struct {
	CanvasSnapshot json.RawMessage `json:"canvas_snapshot"`
}

type ActionTargetInput struct {
	ChannelID         string `json:"channel_id"`
	TemplateVersionID string `json:"template_version_id"`
	Enabled           bool   `json:"enabled"`
}

type ActionInput struct {
	Targets []ActionTargetInput `json:"targets"`
	// Deprecated: use Targets. Kept during API migration.
	TemplateVersionID string `json:"template_version_id"`
	// Deprecated: use Targets. Kept during API migration.
	ChannelIDs        []string        `json:"channel_ids"`
	RecipientStrategy json.RawMessage `json:"recipient_strategy"`
	SendDedupeConfig  json.RawMessage `json:"send_dedupe_config"`
	FailurePolicy     json.RawMessage `json:"failure_policy"`
}

type RuleInput struct {
	RuleKey       string          `json:"rule_key"`
	SortOrder     int             `json:"sort_order"`
	Name          string          `json:"name"`
	ConditionTree json.RawMessage `json:"condition_tree"`
	Enabled       bool            `json:"enabled"`
	Action        ActionInput     `json:"action"`
}

type SaveRulesInput struct {
	Rules []RuleInput `json:"rules"`
}

type ReorderRulesInput struct {
	RuleKeys []string `json:"rule_keys"`
}

type SimulateInput struct {
	Payload json.RawMessage `json:"payload"`
}

type PublishParams struct {
	FlowID           string
	DraftVersionID   string
	CompiledRules    json.RawMessage
	ValidationStatus string
	ValidationErrors json.RawMessage
	PublishedAt      time.Time
}

type Store interface {
	ListFlows(ctx context.Context) ([]Flow, error)
	CreateFlow(ctx context.Context, params CreateFlowParams) (Flow, error)
	GetFlow(ctx context.Context, id string) (Flow, error)
	UpdateFlow(ctx context.Context, id string, params UpdateFlowParams) (Flow, error)
	DeleteFlow(ctx context.Context, id string) error
	GetDraft(ctx context.Context, flowID string) (Draft, error)
	ListVersions(ctx context.Context, flowID string) ([]Version, error)
	UpdateCanvas(ctx context.Context, flowID string, snapshot json.RawMessage, mode FlowMode) (Draft, error)
	ReplaceRules(ctx context.Context, flowID string, versionID string, rules []Rule) ([]Rule, error)
	ReorderRules(ctx context.Context, flowID string, versionID string, ruleKeys []string) ([]Rule, error)
	Publish(ctx context.Context, params PublishParams) (Version, error)
	ActivateVersion(ctx context.Context, flowID string, versionID string) (Flow, error)
	IncrementRuleCounter(ctx context.Context, flowID string, ruleKey string, hitAt time.Time) error
}

type Service struct {
	store Store
	now   func() time.Time
	newID func() string
}

type Option func(*Service)

func WithNow(now func() time.Time) Option {
	return func(s *Service) {
		if now != nil {
			s.now = now
		}
	}
}

func WithIDGenerator(generator func() string) Option {
	return func(s *Service) {
		if generator != nil {
			s.newID = generator
		}
	}
}

func NewService(store Store, options ...Option) *Service {
	service := &Service{
		store: store,
		now:   time.Now,
		newID: uuid.NewString,
	}
	for _, option := range options {
		option(service)
	}
	return service
}

func (s *Service) ListFlows(ctx context.Context) ([]Flow, error) {
	return s.store.ListFlows(ctx)
}

func (s *Service) CreateFlow(ctx context.Context, input CreateFlowInput) (Flow, error) {
	params, err := normalizeFlowInput(input, s.newID)
	if err != nil {
		return Flow{}, err
	}
	return s.store.CreateFlow(ctx, params)
}

func (s *Service) GetFlow(ctx context.Context, id string) (Flow, error) {
	if strings.TrimSpace(id) == "" {
		return Flow{}, ErrInvalidInput
	}
	return s.store.GetFlow(ctx, id)
}

func (s *Service) UpdateFlow(ctx context.Context, id string, input UpdateFlowInput) (Flow, error) {
	if strings.TrimSpace(id) == "" {
		return Flow{}, ErrInvalidInput
	}
	params, err := normalizeFlowInput(input, s.newID)
	if err != nil {
		return Flow{}, err
	}
	params.ID = id
	return s.store.UpdateFlow(ctx, id, params)
}

func (s *Service) DeleteFlow(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return ErrInvalidInput
	}
	return s.store.DeleteFlow(ctx, id)
}

func (s *Service) ListVersions(ctx context.Context, flowID string) ([]Version, error) {
	if strings.TrimSpace(flowID) == "" {
		return nil, ErrInvalidInput
	}
	return s.store.ListVersions(ctx, flowID)
}

func (s *Service) ActivateVersion(ctx context.Context, flowID string, versionID string) (Flow, error) {
	if strings.TrimSpace(flowID) == "" || strings.TrimSpace(versionID) == "" {
		return Flow{}, ErrInvalidInput
	}
	return s.store.ActivateVersion(ctx, flowID, versionID)
}

func (s *Service) GetCanvas(ctx context.Context, flowID string) (CanvasState, error) {
	draft, err := s.getDraft(ctx, flowID)
	if err != nil {
		return CanvasState{}, err
	}
	return CanvasState{
		VersionID:      draft.Version.ID,
		CanvasSnapshot: defaultCanvasJSON(draft.Version.CanvasSnapshot),
	}, nil
}

func (s *Service) SaveCanvas(ctx context.Context, flowID string, input SaveCanvasInput) (CanvasState, error) {
	if strings.TrimSpace(flowID) == "" {
		return CanvasState{}, ErrInvalidInput
	}
	draft, err := s.store.UpdateCanvas(ctx, flowID, defaultCanvasJSON(input.CanvasSnapshot), ModeCanvas)
	if err != nil {
		return CanvasState{}, err
	}
	return CanvasState{
		VersionID:      draft.Version.ID,
		CanvasSnapshot: defaultCanvasJSON(draft.Version.CanvasSnapshot),
	}, nil
}

func (s *Service) GetRules(ctx context.Context, flowID string) (RuleSet, error) {
	draft, err := s.getDraft(ctx, flowID)
	if err != nil {
		return RuleSet{}, err
	}
	return RuleSet{VersionID: draft.Version.ID, Rules: sortRules(draft.Rules)}, nil
}

func (s *Service) SaveRules(ctx context.Context, flowID string, input SaveRulesInput) (RuleSet, error) {
	draft, err := s.getDraft(ctx, flowID)
	if err != nil {
		return RuleSet{}, err
	}
	rules, err := normalizeRuleInputs(flowID, draft.Version.ID, input.Rules, s.newID)
	if err != nil {
		return RuleSet{}, err
	}
	saved, err := s.store.ReplaceRules(ctx, flowID, draft.Version.ID, rules)
	if err != nil {
		return RuleSet{}, err
	}
	return RuleSet{VersionID: draft.Version.ID, Rules: sortRules(saved)}, nil
}

func (s *Service) ReorderRules(ctx context.Context, flowID string, input ReorderRulesInput) (RuleSet, error) {
	draft, err := s.getDraft(ctx, flowID)
	if err != nil {
		return RuleSet{}, err
	}
	if err := validateReorderInput(draft.Rules, input.RuleKeys); err != nil {
		return RuleSet{}, err
	}
	reordered, err := s.store.ReorderRules(ctx, flowID, draft.Version.ID, append([]string(nil), input.RuleKeys...))
	if err != nil {
		return RuleSet{}, err
	}
	return RuleSet{VersionID: draft.Version.ID, Rules: sortRules(reordered)}, nil
}

func (s *Service) Validate(ctx context.Context, flowID string) (ValidationResult, error) {
	draft, err := s.getDraft(ctx, flowID)
	if err != nil {
		return ValidationResult{}, err
	}
	errors := validateRules(draft.Rules)
	status := "valid"
	if len(errors) > 0 {
		status = "invalid"
	}
	return ValidationResult{
		VersionID: draft.Version.ID,
		Status:    status,
		Errors:    errors,
	}, nil
}

func (s *Service) Publish(ctx context.Context, flowID string) (Version, error) {
	draft, err := s.getDraft(ctx, flowID)
	if err != nil {
		return Version{}, err
	}
	validationErrors := validateRules(draft.Rules)
	if len(validationErrors) > 0 {
		return Version{}, ErrInvalidConfig
	}
	compiledRules, err := compileRules(draft, s.now())
	if err != nil {
		return Version{}, ErrInvalidConfig
	}
	validationJSON, _ := json.Marshal(validationErrors)
	return s.store.Publish(ctx, PublishParams{
		FlowID:           flowID,
		DraftVersionID:   draft.Version.ID,
		CompiledRules:    compiledRules,
		ValidationStatus: "valid",
		ValidationErrors: validationJSON,
		PublishedAt:      s.now().UTC(),
	})
}

func (s *Service) Simulate(ctx context.Context, flowID string, input SimulateInput) (SimulationResult, error) {
	draft, err := s.getDraft(ctx, flowID)
	if err != nil {
		return SimulationResult{}, err
	}
	payload, err := decodeJSONObject(input.Payload)
	if err != nil {
		return SimulationResult{}, ErrInvalidInput
	}
	validationErrors := validateRules(draft.Rules)
	if len(validationErrors) > 0 {
		return SimulationResult{}, ErrInvalidConfig
	}

	scope := map[string]any{"payload": payload}
	ruleResults := make([]RuleTrace, 0, len(draft.Rules))
	var matchedRule *RuleTrace
	stopReason := "no_match"
	stopped := false

	for _, rule := range sortRules(draft.Rules) {
		trace := RuleTrace{
			RuleKey:       rule.RuleKey,
			Name:          rule.Name,
			SortOrder:     rule.SortOrder,
			CoarseSkipped: false,
		}
		if stopped {
			trace.StopReason = "first_match_stop"
			ruleResults = append(ruleResults, trace)
			continue
		}
		if !rule.Enabled {
			trace.StopReason = "disabled"
			ruleResults = append(ruleResults, trace)
			continue
		}

		startedAt := time.Now()
		matched, evalErr := EvaluateConditionTree(rule.ConditionTree, scope)
		trace.DurationMS = time.Since(startedAt).Milliseconds()
		trace.Evaluated = true
		if evalErr != nil {
			return SimulationResult{}, ErrInvalidConfig
		}
		trace.Matched = matched
		if matched {
			trace.StopReason = "first_match_stop"
			copyTrace := trace
			matchedRule = &copyTrace
			stopReason = "first_match_stop"
			stopped = true
		}
		ruleResults = append(ruleResults, trace)
	}

	return SimulationResult{
		VersionID:   draft.Version.ID,
		StopReason:  stopReason,
		MatchedRule: matchedRule,
		RuleResults: ruleResults,
	}, nil
}

func (s *Service) getDraft(ctx context.Context, flowID string) (Draft, error) {
	if strings.TrimSpace(flowID) == "" {
		return Draft{}, ErrInvalidInput
	}
	return s.store.GetDraft(ctx, flowID)
}

func normalizeFlowInput(input CreateFlowInput, idGenerator func() string) (CreateFlowParams, error) {
	mode := input.Mode
	if mode == "" {
		mode = ModeTable
	}
	if mode != ModeCanvas && mode != ModeTable {
		return CreateFlowParams{}, ErrInvalidInput
	}
	if strings.TrimSpace(input.SourceID) == "" || strings.TrimSpace(input.Name) == "" {
		return CreateFlowParams{}, ErrInvalidInput
	}
	id := strings.TrimSpace(input.ID)
	if id == "" {
		id = idGenerator()
	}
	return CreateFlowParams{
		ID:       id,
		SourceID: strings.TrimSpace(input.SourceID),
		Name:     strings.TrimSpace(input.Name),
		Enabled:  input.Enabled,
		Mode:     mode,
	}, nil
}

func normalizeRuleInputs(flowID string, versionID string, inputs []RuleInput, idGenerator func() string) ([]Rule, error) {
	if len(inputs) == 0 {
		return []Rule{}, nil
	}
	seenRuleKeys := make(map[string]bool, len(inputs))
	rules := make([]Rule, 0, len(inputs))
	for idx, input := range inputs {
		ruleKey := strings.TrimSpace(input.RuleKey)
		if ruleKey == "" {
			ruleKey = idGenerator()
		}
		if seenRuleKeys[ruleKey] {
			return nil, ErrInvalidInput
		}
		seenRuleKeys[ruleKey] = true

		name := strings.TrimSpace(input.Name)
		if name == "" {
			return nil, ErrInvalidInput
		}
		ruleID := idGenerator()
		actionID := idGenerator()
		targets := normalizeActionTargets(input.Action)
		if len(targets) == 0 {
			return nil, ErrInvalidInput
		}
		actionTargets := make([]ActionTarget, 0, len(targets))
		channelIDs := make([]string, 0, len(targets))
		templateVersionID := ""
		for index, target := range targets {
			if templateVersionID == "" {
				templateVersionID = target.TemplateVersionID
			}
			channelIDs = append(channelIDs, target.ChannelID)
			actionTargets = append(actionTargets, ActionTarget{
				ID:                idGenerator(),
				ActionID:          actionID,
				ChannelID:         target.ChannelID,
				TemplateVersionID: target.TemplateVersionID,
				Enabled:           target.Enabled,
				SortOrder:         (index + 1) * 10,
			})
		}
		sortOrder := input.SortOrder
		if sortOrder <= 0 {
			sortOrder = (idx + 1) * 10
		}
		rules = append(rules, Rule{
			ID:            ruleID,
			FlowID:        flowID,
			VersionID:     versionID,
			RuleKey:       ruleKey,
			SortOrder:     sortOrder,
			Name:          name,
			ConditionTree: defaultObjectJSON(input.ConditionTree),
			Enabled:       input.Enabled,
			Action: Action{
				ID:                actionID,
				RuleID:            ruleID,
				Targets:           actionTargets,
				TemplateVersionID: templateVersionID,
				ChannelIDs:        channelIDs,
				RecipientStrategy: defaultObjectJSON(input.Action.RecipientStrategy),
				SendDedupeConfig:  defaultObjectJSON(input.Action.SendDedupeConfig),
				FailurePolicy:     defaultObjectJSON(input.Action.FailurePolicy),
			},
		})
	}
	return sortRules(rules), nil
}

func normalizeActionTargets(input ActionInput) []ActionTargetInput {
	targets := make([]ActionTargetInput, 0, len(input.Targets))
	seen := map[string]struct{}{}
	for _, target := range input.Targets {
		channelID := strings.TrimSpace(target.ChannelID)
		templateVersionID := strings.TrimSpace(target.TemplateVersionID)
		if channelID == "" || templateVersionID == "" {
			continue
		}
		key := channelID + ":" + templateVersionID
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		targets = append(targets, ActionTargetInput{
			ChannelID:         channelID,
			TemplateVersionID: templateVersionID,
			Enabled:           target.Enabled,
		})
	}
	if len(input.Targets) > 0 {
		return targets
	}

	legacyTemplateID := strings.TrimSpace(input.TemplateVersionID)
	if legacyTemplateID == "" {
		return nil
	}
	for _, channelID := range input.ChannelIDs {
		channelID = strings.TrimSpace(channelID)
		if channelID == "" {
			continue
		}
		key := channelID + ":" + legacyTemplateID
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		targets = append(targets, ActionTargetInput{
			ChannelID:         channelID,
			TemplateVersionID: legacyTemplateID,
			Enabled:           true,
		})
	}
	return targets
}

func validateReorderInput(currentRules []Rule, ruleKeys []string) error {
	if len(currentRules) != len(ruleKeys) {
		return ErrInvalidInput
	}
	expected := make(map[string]bool, len(currentRules))
	for _, rule := range currentRules {
		expected[rule.RuleKey] = true
	}
	seen := make(map[string]bool, len(ruleKeys))
	for _, key := range ruleKeys {
		key = strings.TrimSpace(key)
		if key == "" || !expected[key] || seen[key] {
			return ErrInvalidInput
		}
		seen[key] = true
	}
	return nil
}

func validateRules(rules []Rule) []ValidationError {
	errors := make([]ValidationError, 0)
	seenRuleKeys := map[string]bool{}
	seenSortOrders := map[int]bool{}
	for _, rule := range sortRules(rules) {
		if strings.TrimSpace(rule.RuleKey) == "" || strings.TrimSpace(rule.Name) == "" {
			errors = append(errors, ValidationError{Code: "MGP-ROUTE-002", Message: "策略名称和标识不能为空"})
		}
		if seenRuleKeys[rule.RuleKey] {
			errors = append(errors, ValidationError{Code: "MGP-ROUTE-002", Message: "策略标识重复", Path: rule.RuleKey})
		}
		seenRuleKeys[rule.RuleKey] = true
		if seenSortOrders[rule.SortOrder] {
			errors = append(errors, ValidationError{Code: "MGP-ROUTE-002", Message: "策略顺序重复", Path: rule.RuleKey})
		}
		seenSortOrders[rule.SortOrder] = true
		if _, err := parseConditionNode(rule.ConditionTree); err != nil {
			errors = append(errors, ValidationError{Code: "MGP-ROUTE-002", Message: "条件树不合法", Path: rule.RuleKey})
		}
	}
	return errors
}

type compiledActionTarget struct {
	ChannelID         string `json:"channel_id"`
	TemplateVersionID string `json:"template_version_id"`
	Enabled           bool   `json:"enabled"`
	SortOrder         int    `json:"sort_order"`
}

func compileRules(draft Draft, compiledAt time.Time) (json.RawMessage, error) {
	type compiledAction struct {
		Targets           []compiledActionTarget `json:"targets"`
		TemplateVersionID string                 `json:"template_version_id"`
		ChannelIDs        []string               `json:"channel_ids"`
		RecipientStrategy json.RawMessage        `json:"recipient_strategy"`
		SendDedupeConfig  json.RawMessage        `json:"send_dedupe_config"`
		FailurePolicy     json.RawMessage        `json:"failure_policy"`
	}
	type compiledRule struct {
		RuleKey           string          `json:"rule_key"`
		SortOrder         int             `json:"sort_order"`
		Name              string          `json:"name"`
		Enabled           bool            `json:"enabled"`
		ConditionTree     json.RawMessage `json:"condition_tree"`
		FieldDependencies []string        `json:"field_dependencies"`
		MatchGroupIDs     []string        `json:"match_group_ids"`
		CoarseFilter      map[string]any  `json:"coarse_filter"`
		Action            compiledAction  `json:"action"`
	}
	rules := make([]compiledRule, 0, len(draft.Rules))
	for _, rule := range sortRules(draft.Rules) {
		node, err := parseConditionNode(rule.ConditionTree)
		if err != nil {
			return nil, err
		}
		dependencies := sortedUniqueStrings(node.dependencies())
		matchGroupIDs := sortedUniqueStrings(node.matchGroupIDs())
		rules = append(rules, compiledRule{
			RuleKey:           rule.RuleKey,
			SortOrder:         rule.SortOrder,
			Name:              rule.Name,
			Enabled:           rule.Enabled,
			ConditionTree:     defaultObjectJSON(rule.ConditionTree),
			FieldDependencies: dependencies,
			MatchGroupIDs:     matchGroupIDs,
			CoarseFilter: map[string]any{
				"skipped":            false,
				"field_dependencies": dependencies,
			},
			Action: compiledAction{
				Targets:           compiledActionTargets(rule.Action.Targets),
				TemplateVersionID: rule.Action.TemplateVersionID,
				ChannelIDs:        append([]string(nil), rule.Action.ChannelIDs...),
				RecipientStrategy: defaultObjectJSON(rule.Action.RecipientStrategy),
				SendDedupeConfig:  defaultObjectJSON(rule.Action.SendDedupeConfig),
				FailurePolicy:     defaultObjectJSON(rule.Action.FailurePolicy),
			},
		})
	}

	compiled := map[string]any{
		"execution_mode":   "first_match_stop",
		"compiler_version": "route-compiler-v1",
		"compiled_at":      compiledAt.UTC().Format(time.RFC3339),
		"rules":            rules,
	}
	return json.Marshal(compiled)
}

func compiledActionTargets(items []ActionTarget) []compiledActionTarget {
	targets := make([]compiledActionTarget, 0, len(items))
	for _, item := range items {
		targets = append(targets, compiledActionTarget{
			ChannelID:         item.ChannelID,
			TemplateVersionID: item.TemplateVersionID,
			Enabled:           item.Enabled,
			SortOrder:         item.SortOrder,
		})
	}
	return targets
}

type conditionNode struct {
	Operator     string            `json:"operator"`
	Path         string            `json:"path"`
	MatchGroupID string            `json:"match_group_id"`
	Value        json.RawMessage   `json:"value"`
	Values       []json.RawMessage `json:"values"`
	Conditions   []conditionNode   `json:"conditions"`
}

func parseConditionNode(raw json.RawMessage) (conditionNode, error) {
	if len(raw) == 0 || string(raw) == "{}" {
		return conditionNode{Operator: "always"}, nil
	}
	var node conditionNode
	if err := json.Unmarshal(raw, &node); err != nil {
		return conditionNode{}, err
	}
	node.Operator = strings.TrimSpace(strings.ToLower(node.Operator))
	switch node.Operator {
	case "always":
		return node, nil
	case "and", "or":
		if len(node.Conditions) == 0 {
			return conditionNode{}, ErrInvalidConfig
		}
		for _, child := range node.Conditions {
			if err := child.validate(); err != nil {
				return conditionNode{}, err
			}
		}
		return node, nil
	case "equals", "contains", "not_contains", "in", "exists", "in_match_group", "not_in_match_group", "match_group", "not_match_group":
		return node, node.validate()
	default:
		return conditionNode{}, ErrInvalidConfig
	}
}

func (n conditionNode) validate() error {
	switch n.Operator {
	case "always":
		return nil
	case "and", "or":
		if len(n.Conditions) == 0 {
			return ErrInvalidConfig
		}
		for _, child := range n.Conditions {
			if err := child.validate(); err != nil {
				return err
			}
		}
		return nil
	case "equals", "contains", "not_contains", "in", "exists":
		if strings.TrimSpace(n.Path) == "" {
			return ErrInvalidConfig
		}
		if n.Operator == "in" && len(n.Values) == 0 && strings.TrimSpace(n.MatchGroupID) == "" {
			return ErrInvalidConfig
		}
		if n.Operator != "exists" && n.Operator != "in" && len(n.Value) == 0 {
			return ErrInvalidConfig
		}
		return nil
	case "in_match_group", "not_in_match_group", "match_group", "not_match_group":
		if strings.TrimSpace(n.Path) == "" || strings.TrimSpace(n.MatchGroupID) == "" {
			return ErrInvalidConfig
		}
		return nil
	default:
		return ErrInvalidConfig
	}
}

func (n conditionNode) dependencies() []string {
	if strings.TrimSpace(n.Path) != "" {
		return []string{n.Path}
	}
	dependencies := make([]string, 0)
	for _, child := range n.Conditions {
		dependencies = append(dependencies, child.dependencies()...)
	}
	return dependencies
}

func (n conditionNode) matchGroupIDs() []string {
	ids := make([]string, 0)
	if strings.TrimSpace(n.MatchGroupID) != "" {
		ids = append(ids, strings.TrimSpace(n.MatchGroupID))
	}
	for _, child := range n.Conditions {
		ids = append(ids, child.matchGroupIDs()...)
	}
	return ids
}

func ExtractMatchGroupIDs(raw json.RawMessage) ([]string, error) {
	node, err := parseConditionNode(raw)
	if err != nil {
		return nil, err
	}
	return sortedUniqueStrings(node.matchGroupIDs()), nil
}

func EvaluateConditionTree(raw json.RawMessage, payload map[string]any) (bool, error) {
	return EvaluateConditionTreeWithMatchGroups(raw, payload, nil)
}

func EvaluateConditionTreeWithMatchGroups(raw json.RawMessage, payload map[string]any, matchGroups map[string][]string) (bool, error) {
	node, err := parseConditionNode(raw)
	if err != nil {
		return false, err
	}
	return node.evaluate(payload, matchGroups)
}

func evaluateConditionTree(raw json.RawMessage, payload map[string]any) (bool, error) {
	return EvaluateConditionTree(raw, payload)
}

func (n conditionNode) evaluate(scope map[string]any, matchGroups map[string][]string) (bool, error) {
	switch n.Operator {
	case "always":
		return true, nil
	case "and":
		for _, child := range n.Conditions {
			matched, err := child.evaluate(scope, matchGroups)
			if err != nil {
				return false, err
			}
			if !matched {
				return false, nil
			}
		}
		return true, nil
	case "or":
		for _, child := range n.Conditions {
			matched, err := child.evaluate(scope, matchGroups)
			if err != nil {
				return false, err
			}
			if matched {
				return true, nil
			}
		}
		return false, nil
	}

	value, exists := lookupPath(scope, n.Path)
	switch n.Operator {
	case "exists":
		return exists, nil
	case "equals":
		if !exists {
			return false, nil
		}
		expected, err := decodeAny(n.Value)
		if err != nil {
			return false, err
		}
		return reflect.DeepEqual(value, expected), nil
	case "contains":
		if !exists {
			return false, nil
		}
		needle, err := decodeAny(n.Value)
		if err != nil {
			return false, err
		}
		return containsValue(value, needle), nil
	case "not_contains":
		if !exists {
			return true, nil
		}
		needle, err := decodeAny(n.Value)
		if err != nil {
			return false, err
		}
		return !containsValue(value, needle), nil
	case "in":
		if !exists {
			return false, nil
		}
		if strings.TrimSpace(n.MatchGroupID) != "" {
			return valueInMatchGroup(value, matchGroups[strings.TrimSpace(n.MatchGroupID)]), nil
		}
		for _, candidateRaw := range n.Values {
			candidate, err := decodeAny(candidateRaw)
			if err != nil {
				return false, err
			}
			if reflect.DeepEqual(value, candidate) {
				return true, nil
			}
		}
		return false, nil
	case "in_match_group", "match_group":
		if !exists {
			return false, nil
		}
		return valueInMatchGroup(value, matchGroups[strings.TrimSpace(n.MatchGroupID)]), nil
	case "not_in_match_group", "not_match_group":
		if !exists {
			return true, nil
		}
		return !valueInMatchGroup(value, matchGroups[strings.TrimSpace(n.MatchGroupID)]), nil
	default:
		return false, ErrInvalidConfig
	}
}

func lookupPath(scope map[string]any, path string) (any, bool) {
	parts := strings.Split(strings.TrimSpace(path), ".")
	if len(parts) == 0 {
		return nil, false
	}
	var current any = scope
	for _, part := range parts {
		mapped, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = mapped[part]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func containsValue(value any, needle any) bool {
	switch typed := value.(type) {
	case string:
		needleText, ok := needle.(string)
		return ok && strings.Contains(typed, needleText)
	case []any:
		for _, item := range typed {
			if reflect.DeepEqual(item, needle) {
				return true
			}
		}
	}
	return false
}

func valueInMatchGroup(value any, groupValues []string) bool {
	if len(groupValues) == 0 {
		return false
	}
	valueSet := make(map[string]bool, len(groupValues))
	for _, groupValue := range groupValues {
		valueSet[strings.TrimSpace(groupValue)] = true
	}
	switch typed := value.(type) {
	case []any:
		for _, item := range typed {
			if valueSet[strings.TrimSpace(stringifyConditionValue(item))] {
				return true
			}
		}
		return false
	case []string:
		for _, item := range typed {
			if valueSet[strings.TrimSpace(item)] {
				return true
			}
		}
		return false
	default:
		return valueSet[strings.TrimSpace(stringifyConditionValue(value))]
	}
}

func stringifyConditionValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case json.Number:
		return typed.String()
	default:
		return strings.TrimSpace(strings.Trim(fmt.Sprint(typed), "\""))
	}
}

func decodeJSONObject(raw json.RawMessage) (map[string]any, error) {
	if len(raw) == 0 {
		return map[string]any{}, nil
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	if payload == nil {
		return map[string]any{}, nil
	}
	return payload, nil
}

func decodeAny(raw json.RawMessage) (any, error) {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	return value, nil
}

func sortRules(rules []Rule) []Rule {
	sorted := append([]Rule(nil), rules...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].SortOrder == sorted[j].SortOrder {
			return sorted[i].RuleKey < sorted[j].RuleKey
		}
		return sorted[i].SortOrder < sorted[j].SortOrder
	})
	return sorted
}

func sortedUniqueStrings(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func defaultObjectJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage(`{}`)
	}
	return raw
}

func defaultCanvasJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage(`{}`)
	}
	return raw
}
