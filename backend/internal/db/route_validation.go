package db

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mvp-push-gateway/backend/internal/route"
)

const routeReferenceValidationCode = "MGP-ROUTE-REF"

type routeValidationTarget struct {
	RuleKey           string
	Index             int
	ChannelID         string
	TemplateVersionID string
}

type routeValidationChannel struct {
	ProviderType string
	Enabled      bool
}

type routeValidationTemplate struct {
	TemplateEnabled  bool
	CurrentVersionID string
	MessageType      string
	ProviderType     string
	ValidationStatus string
	Published        bool
}

type routeValidationCapability struct {
	RecipientRequired bool
	AllowNoRecipient  bool
}

type routeValidationRecipientStrategy struct {
	Mode                 string   `json:"mode"`
	Type                 string   `json:"type"`
	RecipientMode        string   `json:"recipient_mode"`
	Path                 string   `json:"path"`
	PayloadPath          string   `json:"payload_path"`
	PayloadRecipientPath string   `json:"payload_recipient_path"`
	UserIDs              []string `json:"user_ids"`
	OrgIDs               []string `json:"org_ids"`
	RecipientGroupIDs    []string `json:"recipient_group_ids"`
	GroupIDs             []string `json:"group_ids"`
	IdentityValues       []string `json:"identity_values"`
	Recipients           any      `json:"recipients"`
}

func (r Repository) ValidateRuleReferences(ctx context.Context, _ string, _ string, rules []route.Rule) ([]route.ValidationError, error) {
	validationErrors := make([]route.ValidationError, 0)
	targets := make([]routeValidationTarget, 0)
	channelIDs := map[string]struct{}{}
	templateVersionIDs := map[string]struct{}{}
	matchGroupIDs := map[string]struct{}{}
	invalidChannelIDs := map[string]bool{}
	invalidTemplateVersionIDs := map[string]bool{}

	for _, ruleItem := range rules {
		if !ruleItem.Enabled {
			continue
		}
		extractedMatchGroupIDs, err := route.ExtractMatchGroupIDs(ruleItem.ConditionTree)
		if err != nil {
			return nil, err
		}
		for _, matchGroupID := range extractedMatchGroupIDs {
			id := strings.TrimSpace(matchGroupID)
			if id == "" {
				continue
			}
			if _, err := uuid.Parse(id); err != nil {
				validationErrors = append(validationErrors, route.ValidationError{
					Code:    routeReferenceValidationCode,
					Message: "条件引用的匹配组标识不合法",
					Path:    ruleItem.RuleKey,
				})
				continue
			}
			matchGroupIDs[id] = struct{}{}
		}
		for index, target := range routeValidationTargets(ruleItem.Action) {
			channelID := strings.TrimSpace(target.ChannelID)
			templateVersionID := strings.TrimSpace(target.TemplateVersionID)
			if !target.Enabled || channelID == "" || templateVersionID == "" {
				continue
			}
			path := routeTargetValidationPath(ruleItem.RuleKey, index)
			if _, err := uuid.Parse(channelID); err != nil {
				invalidChannelIDs[channelID] = true
				validationErrors = append(validationErrors, route.ValidationError{
					Code:    routeReferenceValidationCode,
					Message: "发送目标引用的推送渠道实例标识不合法",
					Path:    path,
				})
			} else {
				channelIDs[channelID] = struct{}{}
			}
			if _, err := uuid.Parse(templateVersionID); err != nil {
				invalidTemplateVersionIDs[templateVersionID] = true
				validationErrors = append(validationErrors, route.ValidationError{
					Code:    routeReferenceValidationCode,
					Message: "发送目标引用的模板版本标识不合法",
					Path:    path,
				})
			} else {
				templateVersionIDs[templateVersionID] = struct{}{}
			}
			targets = append(targets, routeValidationTarget{
				RuleKey:           ruleItem.RuleKey,
				Index:             index,
				ChannelID:         channelID,
				TemplateVersionID: templateVersionID,
			})
		}
	}

	matchGroups, err := r.loadRouteValidationMatchGroups(ctx, setKeys(matchGroupIDs))
	if err != nil {
		return nil, err
	}
	for matchGroupID := range matchGroupIDs {
		if !matchGroups[matchGroupID] {
			validationErrors = append(validationErrors, route.ValidationError{
				Code:    routeReferenceValidationCode,
				Message: "条件引用的匹配组不存在或已停用",
				Path:    matchGroupID,
			})
		}
	}

	channels, err := r.loadRouteValidationChannels(ctx, setKeys(channelIDs))
	if err != nil {
		return nil, err
	}
	templates, err := r.loadRouteValidationTemplates(ctx, setKeys(templateVersionIDs))
	if err != nil {
		return nil, err
	}
	capabilities, err := r.loadRouteValidationCapabilities(ctx, targets, channels, templates)
	if err != nil {
		return nil, err
	}
	rulesByKey := map[string]route.Rule{}
	for _, ruleItem := range rules {
		rulesByKey[ruleItem.RuleKey] = ruleItem
	}

	for _, target := range targets {
		path := routeTargetValidationPath(target.RuleKey, target.Index)
		if invalidChannelIDs[target.ChannelID] || invalidTemplateVersionIDs[target.TemplateVersionID] {
			continue
		}
		channel, ok := channels[target.ChannelID]
		if !ok || !channel.Enabled {
			validationErrors = append(validationErrors, route.ValidationError{
				Code:    routeReferenceValidationCode,
				Message: "发送目标引用的推送渠道实例不存在或已停用",
				Path:    path,
			})
			continue
		}
		template, ok := templates[target.TemplateVersionID]
		if !ok || !template.TemplateEnabled || template.CurrentVersionID == "" || !template.Published || template.ValidationStatus != "valid" {
			validationErrors = append(validationErrors, route.ValidationError{
				Code:    routeReferenceValidationCode,
				Message: "发送目标引用的模板不存在、未发布或校验无效",
				Path:    path,
			})
			continue
		}
		if template.ProviderType != "" && template.ProviderType != channel.ProviderType {
			validationErrors = append(validationErrors, route.ValidationError{
				Code:    routeReferenceValidationCode,
				Message: "发送目标的模板与推送渠道类型不兼容",
				Path:    path,
			})
			continue
		}
		capability, ok := capabilities[routeCapabilityKey(channel.ProviderType, template.MessageType)]
		if !ok {
			validationErrors = append(validationErrors, route.ValidationError{
				Code:    routeReferenceValidationCode,
				Message: "推送渠道不支持模板消息类型",
				Path:    path,
			})
			continue
		}
		if recipientError := validateRouteRecipientRequirement(rulesByKey[target.RuleKey].Action.RecipientStrategy, capability); recipientError != "" {
			validationErrors = append(validationErrors, route.ValidationError{
				Code:    routeReferenceValidationCode,
				Message: recipientError,
				Path:    target.RuleKey,
			})
		}
	}
	return validationErrors, nil
}

func routeValidationTargets(action route.Action) []route.ActionTarget {
	if len(action.Targets) > 0 {
		return action.Targets
	}
	targets := make([]route.ActionTarget, 0, len(action.ChannelIDs))
	templateVersionID := strings.TrimSpace(action.TemplateVersionID)
	if templateVersionID == "" {
		return targets
	}
	for index, channelID := range action.ChannelIDs {
		channelID = strings.TrimSpace(channelID)
		if channelID == "" {
			continue
		}
		targets = append(targets, route.ActionTarget{
			ChannelID:         channelID,
			TemplateVersionID: templateVersionID,
			Enabled:           true,
			SortOrder:         (index + 1) * 10,
		})
	}
	return targets
}

func (r Repository) loadRouteValidationMatchGroups(ctx context.Context, ids []string) (map[string]bool, error) {
	result := map[string]bool{}
	if len(ids) == 0 {
		return result, nil
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id::text, enabled
		FROM match_groups
		WHERE id = ANY($1::uuid[])
	`, ids)
	if err != nil {
		return nil, fmt.Errorf("load route validation match groups: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var enabled bool
		if err := rows.Scan(&id, &enabled); err != nil {
			return nil, fmt.Errorf("scan route validation match group: %w", err)
		}
		result[id] = enabled
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("route validation match group rows: %w", err)
	}
	return result, nil
}

func (r Repository) loadRouteValidationChannels(ctx context.Context, ids []string) (map[string]routeValidationChannel, error) {
	result := map[string]routeValidationChannel{}
	if len(ids) == 0 {
		return result, nil
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id::text, provider_type, enabled
		FROM delivery_channels
		WHERE id = ANY($1::uuid[])
	`, ids)
	if err != nil {
		return nil, fmt.Errorf("load route validation channels: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var item routeValidationChannel
		if err := rows.Scan(&id, &item.ProviderType, &item.Enabled); err != nil {
			return nil, fmt.Errorf("scan route validation channel: %w", err)
		}
		result[id] = item
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("route validation channel rows: %w", err)
	}
	return result, nil
}

func (r Repository) loadRouteValidationTemplates(ctx context.Context, ids []string) (map[string]routeValidationTemplate, error) {
	result := map[string]routeValidationTemplate{}
	if len(ids) == 0 {
		return result, nil
	}
	rows, err := r.pool.Query(ctx, `
		SELECT
			referenced_version.id::text,
			template.enabled,
			COALESCE(template.current_version_id::text, ''),
			referenced_version.message_type,
			referenced_version.target_provider_type,
			referenced_version.validation_status,
			referenced_version.published_at
		FROM template_versions AS referenced_version
		JOIN templates AS template ON template.id = referenced_version.template_id
		WHERE referenced_version.id = ANY($1::uuid[])
	`, ids)
	if err != nil {
		return nil, fmt.Errorf("load route validation templates: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var publishedAt pgtype.Timestamptz
		var item routeValidationTemplate
		if err := rows.Scan(
			&id,
			&item.TemplateEnabled,
			&item.CurrentVersionID,
			&item.MessageType,
			&item.ProviderType,
			&item.ValidationStatus,
			&publishedAt,
		); err != nil {
			return nil, fmt.Errorf("scan route validation template: %w", err)
		}
		item.Published = publishedAt.Valid
		result[id] = item
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("route validation template rows: %w", err)
	}
	return result, nil
}

func (r Repository) loadRouteValidationCapabilities(
	ctx context.Context,
	targets []routeValidationTarget,
	channels map[string]routeValidationChannel,
	templates map[string]routeValidationTemplate,
) (map[string]routeValidationCapability, error) {
	providerTypes := map[string]struct{}{}
	messageTypes := map[string]struct{}{}
	for _, target := range targets {
		channel, hasChannel := channels[target.ChannelID]
		template, hasTemplate := templates[target.TemplateVersionID]
		if !hasChannel || !hasTemplate || !channel.Enabled || template.MessageType == "" {
			continue
		}
		providerTypes[channel.ProviderType] = struct{}{}
		messageTypes[template.MessageType] = struct{}{}
	}
	result := map[string]routeValidationCapability{}
	if len(providerTypes) == 0 || len(messageTypes) == 0 {
		return result, nil
	}
	rows, err := r.pool.Query(ctx, `
		SELECT provider_type, message_type, recipient_required, allow_no_recipient
		FROM provider_capabilities
		WHERE provider_type = ANY($1::text[])
			AND message_type = ANY($2::text[])
	`, setKeys(providerTypes), setKeys(messageTypes))
	if err != nil {
		return nil, fmt.Errorf("load route validation provider capabilities: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var providerType string
		var messageType string
		var item routeValidationCapability
		if err := rows.Scan(&providerType, &messageType, &item.RecipientRequired, &item.AllowNoRecipient); err != nil {
			return nil, fmt.Errorf("scan route validation provider capability: %w", err)
		}
		result[routeCapabilityKey(providerType, messageType)] = item
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("route validation provider capability rows: %w", err)
	}
	return result, nil
}

func validateRouteRecipientRequirement(raw json.RawMessage, capability routeValidationCapability) string {
	if !capability.RecipientRequired || capability.AllowNoRecipient {
		return ""
	}
	strategy := decodeRouteValidationRecipientStrategy(raw)
	switch strategy.mode() {
	case "payload":
		if strings.TrimSpace(firstNonEmpty(strategy.Path, strategy.PayloadPath, strategy.PayloadRecipientPath)) == "" {
			return "Payload 接收人模式需要填写接收人路径"
		}
	case "system", "":
		if !strategy.hasSystemSelectors() {
			return "接收策略缺少必需接收人"
		}
	case "none":
		return "当前推送渠道要求接收人，不能使用无接收人策略"
	}
	return ""
}

func decodeRouteValidationRecipientStrategy(raw json.RawMessage) routeValidationRecipientStrategy {
	var strategy routeValidationRecipientStrategy
	if len(strings.TrimSpace(string(raw))) == 0 {
		return strategy
	}
	_ = json.Unmarshal(raw, &strategy)
	return strategy
}

func (s routeValidationRecipientStrategy) mode() string {
	mode := strings.ToLower(strings.TrimSpace(firstNonEmpty(s.Mode, s.Type, s.RecipientMode)))
	if mode != "" {
		return mode
	}
	if firstNonEmpty(s.Path, s.PayloadPath, s.PayloadRecipientPath) != "" {
		return "payload"
	}
	if s.hasSystemSelectors() {
		return "system"
	}
	return ""
}

func (s routeValidationRecipientStrategy) hasSystemSelectors() bool {
	return len(s.UserIDs) > 0 ||
		len(s.OrgIDs) > 0 ||
		len(s.RecipientGroupIDs) > 0 ||
		len(s.GroupIDs) > 0 ||
		len(s.IdentityValues) > 0 ||
		!routeValidationEmptyValue(s.Recipients)
}

func routeValidationEmptyValue(value any) bool {
	if value == nil {
		return true
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed) == ""
	case []any:
		return len(typed) == 0
	case map[string]any:
		return len(typed) == 0
	default:
		return false
	}
}

func routeTargetValidationPath(ruleKey string, index int) string {
	return fmt.Sprintf("%s.targets[%d]", ruleKey, index)
}

func routeCapabilityKey(providerType string, messageType string) string {
	return providerType + ":" + messageType
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func setKeys[T any](items map[string]T) []string {
	keys := make([]string, 0, len(items))
	for key := range items {
		keys = append(keys, key)
	}
	return keys
}
