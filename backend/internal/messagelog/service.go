package messagelog

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"
)

var (
	ErrNotFound     = errors.New("message log not found")
	ErrInvalidInput = errors.New("invalid message log input")
)

type ListFilter struct {
	TraceID   string
	SourceID  string
	Status    string
	ChannelID string
	Limit     int
	Offset    int
}

type ListResult struct {
	Messages []MessageSummary
	Total    int
	Limit    int
	Offset   int
}

type MessageSummary struct {
	ID                  string
	TraceID             string
	SourceID            string
	SourceName          string
	ReceivedAt          time.Time
	Status              string
	InboundStatus       string
	MatchedFlowID       string
	MatchedFlowName     string
	MatchedRuleIDs      []string
	ErrorCode           string
	ErrorMessage        string
	OutboundStatus      string
	FirstOutboundAt     *time.Time
	LastOutboundAt      *time.Time
	AttemptCount        int
	TargetChannelIDs    []string
	TargetChannelNames  []string
	TargetProviderTypes []string
	DurationMS          int
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type MessageDetail struct {
	MessageSummary
	Headers     json.RawMessage
	Payload     json.RawMessage
	PayloadHash string
	Attempts    []DeliveryAttempt
	DeadLetters []DeadLetterEvent
	Timeline    []TimelineEvent
}

type DeliveryAttempt struct {
	ID                 string
	MessageID          string
	ChannelID          string
	ChannelName        string
	ProviderType       string
	TemplateVersionID  string
	RecipientSnapshot  json.RawMessage
	RequestSnapshot    json.RawMessage
	ResponseSnapshot   json.RawMessage
	TargetContext      json.RawMessage
	RenderedMessage    json.RawMessage
	ResolvedRecipients json.RawMessage
	FinalRequest       json.RawMessage
	UpstreamResponse   json.RawMessage
	Status             string
	ErrorCode          string
	ErrorMessage       string
	DurationMS         int
	AttemptNo          int
	NextRetryAt        *time.Time
	DeadLetteredAt     *time.Time
	QueuedAt           *time.Time
	StartedAt          *time.Time
	FinishedAt         *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type DeadLetterEvent struct {
	ID             string
	AttemptID      string
	ErrorCode      string
	ErrorMessage   string
	DeadLetteredAt time.Time
	ReplayedAt     *time.Time
	HandledAt      *time.Time
	HandledReason  string
}

type TimelineEvent struct {
	Stage       string    `json:"stage"`
	At          time.Time `json:"at"`
	Status      string    `json:"status"`
	Description string    `json:"description"`
	DurationMS  int       `json:"duration_ms"`
	ErrorCode   string    `json:"error_code,omitempty"`
}

type Store interface {
	ListMessages(ctx context.Context, filter ListFilter) (ListResult, error)
	GetMessage(ctx context.Context, id string) (MessageDetail, error)
}

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

func (s *Service) ListMessages(ctx context.Context, filter ListFilter) (ListResult, error) {
	filter = normalizeFilter(filter)
	result, err := s.store.ListMessages(ctx, filter)
	if err != nil {
		return ListResult{}, err
	}
	for index := range result.Messages {
		result.Messages[index] = normalizeSummary(result.Messages[index])
	}
	return result, nil
}

func (s *Service) GetMessage(ctx context.Context, id string) (MessageDetail, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return MessageDetail{}, ErrInvalidInput
	}
	detail, err := s.store.GetMessage(ctx, id)
	if err != nil {
		return MessageDetail{}, err
	}
	detail.Attempts = deriveAttemptSnapshots(detail.Attempts)
	detail.MessageSummary = normalizeDetailSummary(detail.MessageSummary, detail.Attempts)
	detail.Timeline = buildTimeline(detail)
	return detail, nil
}

func normalizeFilter(filter ListFilter) ListFilter {
	filter.TraceID = strings.TrimSpace(filter.TraceID)
	filter.SourceID = strings.TrimSpace(filter.SourceID)
	filter.Status = strings.TrimSpace(filter.Status)
	filter.ChannelID = strings.TrimSpace(filter.ChannelID)
	if filter.Limit <= 0 {
		filter.Limit = 50
	}
	if filter.Limit > 200 {
		filter.Limit = 200
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	return filter
}

func buildTimeline(detail MessageDetail) []TimelineEvent {
	events := []TimelineEvent{{
		Stage:       "inbound_received",
		At:          detail.ReceivedAt,
		Status:      detail.InboundStatus,
		Description: "入站请求已接收",
		DurationMS:  0,
		ErrorCode:   detail.ErrorCode,
	}}
	if isTimelineTime(detail.CreatedAt) && detail.CreatedAt.After(detail.ReceivedAt) {
		events = append(events, TimelineEvent{
			Stage:       "inbound_validated",
			At:          detail.CreatedAt,
			Status:      detail.InboundStatus,
			Description: inboundValidationDescription(detail.InboundStatus),
			DurationMS:  timelineDurationMS(detail.ReceivedAt, detail.CreatedAt),
			ErrorCode:   detail.ErrorCode,
		})
	}
	routePlannedAt := routePlannedAt(detail)
	if detail.Status == "no_route" {
		events = append(events, TimelineEvent{
			Stage:       "route_no_match",
			At:          routePlannedAt,
			Status:      detail.Status,
			Description: "路由规划完成，未命中可执行规则",
			ErrorCode:   detail.ErrorCode,
		})
	} else if len(detail.MatchedRuleIDs) > 0 {
		routePlanStartedAt := appendRoutePlanningBreakdown(&events, detail)
		routeDurationMS := 0
		if !routePlanStartedAt.IsZero() {
			routeDurationMS = timelineDurationMS(routePlanStartedAt, routePlannedAt)
		}
		events = append(events, TimelineEvent{
			Stage:       "route_planned",
			At:          routePlannedAt,
			Status:      detail.InboundStatus,
			Description: "路由规划完成，命中规则",
			DurationMS:  routeDurationMS,
		})
	}
	for _, attempt := range detail.Attempts {
		if at, ok := lifecycleTime(attempt.RequestSnapshot, "delivery_created_at"); ok || attempt.QueuedAt != nil {
			if !ok {
				at = *attempt.QueuedAt
			}
			events = append(events, TimelineEvent{
				Stage:       "delivery_created",
				At:          at,
				Status:      attempt.Status,
				Description: deliveryCreatedDescription(attempt),
			})
		}
		if at, ok := lifecycleTime(attempt.RequestSnapshot, "request_started_at"); ok || attempt.StartedAt != nil {
			if !ok {
				at = *attempt.StartedAt
			}
			events = append(events, TimelineEvent{
				Stage:       "upstream_request_sent",
				At:          at,
				Status:      attempt.Status,
				Description: "上级请求已发出",
			})
		}
		if at, ok := lifecycleTime(attempt.ResponseSnapshot, "request_finished_at"); ok || attempt.FinishedAt != nil {
			if !ok {
				at = *attempt.FinishedAt
			}
			durationMS := attempt.DurationMS
			if value, ok := lifecycleDurationMS(attempt.ResponseSnapshot, "request_duration_ms"); ok {
				durationMS = value
			}
			events = append(events, TimelineEvent{
				Stage:       "upstream_call_finished",
				At:          at,
				Status:      attempt.Status,
				Description: upstreamFinishedDescription(attempt),
				DurationMS:  durationMS,
				ErrorCode:   attempt.ErrorCode,
			})
		}
	}
	appendDeadLetterTimelineEvents(&events, detail)
	sort.SliceStable(events, func(i, j int) bool {
		return events[i].At.Before(events[j].At)
	})
	fillTimelineDurations(events)
	return events
}

func appendDeadLetterTimelineEvents(events *[]TimelineEvent, detail MessageDetail) {
	eventByAttempt := map[string]bool{}
	for _, item := range detail.DeadLetters {
		eventByAttempt[item.AttemptID] = true
		if isTimelineTime(item.DeadLetteredAt) {
			*events = append(*events, TimelineEvent{
				Stage:       "delivery_dead_lettered",
				At:          item.DeadLetteredAt,
				Status:      "dead",
				Description: "发送失败，进入死信队列",
				DurationMS:  0,
				ErrorCode:   item.ErrorCode,
			})
		}
		if item.ReplayedAt != nil {
			*events = append(*events, TimelineEvent{
				Stage:       "dead_letter_replayed",
				At:          *item.ReplayedAt,
				Status:      "replayed",
				Description: "死信已重放，已重新进入发送队列",
				DurationMS:  0,
			})
		}
		if item.HandledAt != nil {
			description := "死信已人工处理"
			if strings.TrimSpace(item.HandledReason) != "" {
				description += "：" + strings.TrimSpace(item.HandledReason)
			}
			*events = append(*events, TimelineEvent{
				Stage:       "dead_letter_handled",
				At:          *item.HandledAt,
				Status:      "handled",
				Description: description,
				DurationMS:  0,
			})
		}
	}
	for _, attempt := range detail.Attempts {
		if attempt.DeadLetteredAt == nil || eventByAttempt[attempt.ID] {
			continue
		}
		*events = append(*events, TimelineEvent{
			Stage:       "delivery_dead_lettered",
			At:          *attempt.DeadLetteredAt,
			Status:      "dead",
			Description: "发送失败，进入死信队列",
			DurationMS:  0,
			ErrorCode:   attempt.ErrorCode,
		})
	}
}

func appendRoutePlanningBreakdown(events *[]TimelineEvent, detail MessageDetail) time.Time {
	snapshot, ok := firstRequestSnapshot(detail.Attempts)
	if !ok {
		return time.Time{}
	}
	routePlanStartedAt, hasRoutePlanStartedAt := lifecycleTime(snapshot, "route_plan_started_at")
	if hasRoutePlanStartedAt {
		*events = append(*events, TimelineEvent{
			Stage:       "route_planning_started",
			At:          routePlanStartedAt,
			Status:      detail.InboundStatus,
			Description: "路由规划开始",
		})
	}
	appendLifecycleDurationEvent(events, snapshot, TimelineEvent{
		Stage:       "route_condition_evaluated",
		Status:      detail.InboundStatus,
		Description: "规则判断完成",
	}, "route_condition_finished_at", "route_condition_duration_ms")
	appendLifecycleDurationEvent(events, snapshot, TimelineEvent{
		Stage:       "route_template_rendered",
		Status:      detail.InboundStatus,
		Description: "模板渲染完成",
	}, "template_render_finished_at", "template_render_duration_ms")
	appendLifecycleDurationEvent(events, snapshot, TimelineEvent{
		Stage:       "route_send_event_built",
		Status:      detail.InboundStatus,
		Description: "出站事件已生成",
	}, "send_event_built_at", "send_event_build_duration_ms")
	if hasRoutePlanStartedAt {
		return routePlanStartedAt
	}
	return time.Time{}
}

func appendLifecycleDurationEvent(events *[]TimelineEvent, snapshot json.RawMessage, event TimelineEvent, timeKey string, durationKey string) {
	at, ok := lifecycleTime(snapshot, timeKey)
	if !ok {
		return
	}
	event.At = at
	if durationMS, ok := lifecycleDurationMS(snapshot, durationKey); ok {
		event.DurationMS = durationMS
	}
	*events = append(*events, event)
}

func firstRequestSnapshot(attempts []DeliveryAttempt) (json.RawMessage, bool) {
	for _, attempt := range attempts {
		if len(strings.TrimSpace(string(attempt.RequestSnapshot))) > 0 && string(attempt.RequestSnapshot) != "null" {
			return attempt.RequestSnapshot, true
		}
	}
	return nil, false
}

func fillTimelineDurations(events []TimelineEvent) {
	for index := range events {
		if index == 0 {
			events[index].DurationMS = 0
			continue
		}
		if timelineDurationFixed(events[index].Stage) {
			events[index].DurationMS = 0
			continue
		}
		if events[index].DurationMS > 0 {
			continue
		}
		events[index].DurationMS = timelineDurationMS(events[index-1].At, events[index].At)
	}
}

func timelineDurationFixed(stage string) bool {
	switch stage {
	case "delivery_dead_lettered", "dead_letter_replayed", "dead_letter_handled":
		return true
	default:
		return false
	}
}

func routePlannedAt(detail MessageDetail) time.Time {
	for _, attempt := range detail.Attempts {
		if at, ok := lifecycleTime(attempt.RequestSnapshot, "route_planned_at"); ok {
			return at
		}
	}
	for _, attempt := range detail.Attempts {
		if attempt.QueuedAt != nil && attempt.QueuedAt.After(detail.ReceivedAt) {
			return *attempt.QueuedAt
		}
	}
	return detail.UpdatedAtOrReceivedAt()
}

func lifecycleTime(snapshot json.RawMessage, key string) (time.Time, bool) {
	var fields map[string]json.RawMessage
	if !decodeSnapshotObject(snapshotField(snapshot, "lifecycle"), &fields) {
		return time.Time{}, false
	}
	var value string
	if err := json.Unmarshal(fields[key], &value); err != nil {
		return time.Time{}, false
	}
	at, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(value))
	if err != nil {
		return time.Time{}, false
	}
	return at, true
}

func lifecycleDurationMS(snapshot json.RawMessage, key string) (int, bool) {
	var fields map[string]json.RawMessage
	if !decodeSnapshotObject(snapshotField(snapshot, "lifecycle"), &fields) {
		return 0, false
	}
	var value int
	if err := json.Unmarshal(fields[key], &value); err != nil {
		return 0, false
	}
	return value, true
}

func inboundValidationDescription(status string) string {
	switch strings.TrimSpace(status) {
	case "deduped":
		return "入站校验完成，命中去重"
	case "silenced":
		return "入站校验完成，命中免打扰"
	case "failed":
		return "入站校验完成，校验失败"
	default:
		return "入站校验完成"
	}
}

func deliveryCreatedDescription(attempt DeliveryAttempt) string {
	if strings.TrimSpace(attempt.ChannelName) == "" {
		return "出站投递已创建"
	}
	return "出站投递已创建，目标：" + strings.TrimSpace(attempt.ChannelName)
}

func upstreamFinishedDescription(attempt DeliveryAttempt) string {
	switch strings.TrimSpace(attempt.Status) {
	case "sent":
		return "上级调用结束，成功"
	case "failed", "dead":
		return "上级调用结束，失败"
	default:
		return "上级调用结束"
	}
}

func isTimelineTime(value time.Time) bool {
	return !value.IsZero()
}

func timelineDurationMS(start time.Time, end time.Time) int {
	if start.IsZero() || end.IsZero() || end.Before(start) {
		return 0
	}
	return int(end.Sub(start) / time.Millisecond)
}

func (detail MessageDetail) UpdatedAtOrReceivedAt() time.Time {
	if !detail.UpdatedAt.IsZero() {
		return detail.UpdatedAt
	}
	return detail.ReceivedAt
}

func normalizeDetailSummary(summary MessageSummary, attempts []DeliveryAttempt) MessageSummary {
	summary.OutboundStatus = deriveOutboundStatus(attempts)
	summary.FirstOutboundAt, summary.LastOutboundAt = outboundTimes(attempts)
	if len(attempts) > 0 {
		summary.DurationMS = messageDurationMS(summary.ReceivedAt, summary.LastOutboundAt, summary.DurationMS)
	}
	return normalizeSummary(summary)
}

func deriveOutboundStatus(attempts []DeliveryAttempt) string {
	if len(attempts) == 0 {
		return ""
	}
	allSent := true
	anySent := false
	for _, attempt := range attempts {
		if attempt.DeadLetteredAt != nil {
			return "dead"
		}
		switch attempt.Status {
		case "failed":
			return "failed"
		case "processing":
			return "processing"
		case "queued":
			return "queued"
		case "sent":
			anySent = true
		default:
			allSent = false
		}
	}
	if allSent && anySent {
		return "sent"
	}
	if anySent {
		return "partial_sent"
	}
	return attempts[len(attempts)-1].Status
}

func normalizeSummary(summary MessageSummary) MessageSummary {
	inboundStatus := strings.TrimSpace(summary.InboundStatus)
	if inboundStatus == "" {
		inboundStatus = strings.TrimSpace(summary.Status)
	}
	summary.InboundStatus = inboundStatus
	summary.Status = lifecycleStatus(inboundStatus, summary.OutboundStatus)
	if summary.Status == "no_route" && len(summary.MatchedRuleIDs) == 0 {
		summary.MatchedFlowID = ""
		summary.MatchedFlowName = ""
	}
	return summary
}

func lifecycleStatus(inboundStatus string, outboundStatus string) string {
	outboundStatus = strings.TrimSpace(outboundStatus)
	if outboundStatus != "" {
		switch outboundStatus {
		case "queued":
			return "queued"
		case "processing":
			return "processing"
		case "sent":
			return "sent"
		case "partial_sent":
			return "partial_sent"
		case "failed":
			return "failed"
		case "dead":
			return "dead"
		case "deduped":
			return "deduped"
		case "skipped":
			return "skipped"
		}
	}
	switch strings.TrimSpace(inboundStatus) {
	case "accepted", "deduped", "silenced", "planned", "partial_sent", "sent", "failed", "no_route", "skipped", "queued", "processing", "dead":
		return strings.TrimSpace(inboundStatus)
	default:
		return "accepted"
	}
}

func outboundTimes(attempts []DeliveryAttempt) (*time.Time, *time.Time) {
	var first *time.Time
	var last *time.Time
	for _, attempt := range attempts {
		if attempt.FinishedAt == nil {
			continue
		}
		candidate := attempt.FinishedAt
		if first == nil || candidate.Before(*first) {
			value := candidate.UTC()
			first = &value
		}
		if last == nil || candidate.After(*last) {
			value := candidate.UTC()
			last = &value
		}
	}
	return first, last
}

func messageDurationMS(receivedAt time.Time, lastOutboundAt *time.Time, fallback int) int {
	if receivedAt.IsZero() || lastOutboundAt == nil {
		return fallback
	}
	duration := lastOutboundAt.Sub(receivedAt)
	if duration < 0 {
		return fallback
	}
	return int(duration / time.Millisecond)
}

func deriveAttemptSnapshots(attempts []DeliveryAttempt) []DeliveryAttempt {
	for index := range attempts {
		attempts[index] = deriveAttemptSnapshot(attempts[index])
	}
	return attempts
}

func deriveAttemptSnapshot(attempt DeliveryAttempt) DeliveryAttempt {
	if !hasJSON(attempt.TargetContext) {
		attempt.TargetContext = snapshotField(attempt.RequestSnapshot, "target_context")
		if !hasJSON(attempt.TargetContext) {
			attempt.TargetContext = synthesizedTargetContext(attempt)
		}
	}
	if !hasJSON(attempt.RenderedMessage) {
		attempt.RenderedMessage = snapshotField(attempt.RequestSnapshot, "rendered_message")
		if !hasJSON(attempt.RenderedMessage) {
			attempt.RenderedMessage = nestedSnapshotField(attempt.RequestSnapshot, "send", "body")
		}
	}
	if !hasJSON(attempt.ResolvedRecipients) {
		attempt.ResolvedRecipients = snapshotField(attempt.RequestSnapshot, "resolved_recipients")
		if !hasJSON(attempt.ResolvedRecipients) {
			attempt.ResolvedRecipients = normalizeRecipients(nestedSnapshotField(attempt.RequestSnapshot, "send", "recipient"))
		}
		if !hasJSON(attempt.ResolvedRecipients) {
			attempt.ResolvedRecipients = normalizeRecipients(snapshotField(attempt.RecipientSnapshot, "recipient"))
		}
	}
	if !hasJSON(attempt.FinalRequest) {
		attempt.FinalRequest = snapshotField(attempt.RequestSnapshot, "final_request")
		if !hasJSON(attempt.FinalRequest) {
			attempt.FinalRequest = snapshotField(attempt.RequestSnapshot, "send")
		}
	}
	if !hasJSON(attempt.UpstreamResponse) {
		attempt.UpstreamResponse = snapshotField(attempt.ResponseSnapshot, "upstream_response")
		if !hasJSON(attempt.UpstreamResponse) {
			attempt.UpstreamResponse = snapshotField(attempt.ResponseSnapshot, "send")
		}
	}
	return attempt
}

func snapshotField(snapshot json.RawMessage, key string) json.RawMessage {
	var fields map[string]json.RawMessage
	if !decodeSnapshotObject(snapshot, &fields) {
		return nil
	}
	return cloneRawJSON(fields[key])
}

func nestedSnapshotField(snapshot json.RawMessage, parent string, key string) json.RawMessage {
	parentRaw := snapshotField(snapshot, parent)
	var fields map[string]json.RawMessage
	if !decodeSnapshotObject(parentRaw, &fields) {
		return nil
	}
	return cloneRawJSON(fields[key])
}

func decodeSnapshotObject(snapshot json.RawMessage, target *map[string]json.RawMessage) bool {
	if !hasJSON(snapshot) {
		return false
	}
	if err := json.Unmarshal(snapshot, target); err != nil {
		return false
	}
	return *target != nil
}

func cloneRawJSON(raw json.RawMessage) json.RawMessage {
	if !hasJSON(raw) {
		return nil
	}
	return append(json.RawMessage(nil), raw...)
}

func hasJSON(raw json.RawMessage) bool {
	trimmed := strings.TrimSpace(string(raw))
	return trimmed != "" && trimmed != "null"
}

func synthesizedTargetContext(attempt DeliveryAttempt) json.RawMessage {
	context := map[string]string{}
	if strings.TrimSpace(attempt.ChannelID) != "" {
		context["channel_id"] = attempt.ChannelID
	}
	if strings.TrimSpace(attempt.ChannelName) != "" {
		context["channel_name"] = attempt.ChannelName
	}
	if strings.TrimSpace(attempt.ProviderType) != "" {
		context["provider_type"] = attempt.ProviderType
	}
	if strings.TrimSpace(attempt.TemplateVersionID) != "" {
		context["template_version_id"] = attempt.TemplateVersionID
	}
	if len(context) == 0 {
		return nil
	}
	raw, err := json.Marshal(context)
	if err != nil {
		return nil
	}
	return raw
}

func normalizeRecipients(raw json.RawMessage) json.RawMessage {
	if !hasJSON(raw) {
		return nil
	}
	var array []json.RawMessage
	if err := json.Unmarshal(raw, &array); err == nil {
		return cloneRawJSON(raw)
	}
	wrapped, err := json.Marshal([]json.RawMessage{raw})
	if err != nil {
		return nil
	}
	return wrapped
}
