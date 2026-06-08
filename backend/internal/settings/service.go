package settings

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	KeyConsolePollingIntervalSeconds = "console.polling_interval_seconds"
	KeyLogsRetentionDays             = "logs.retention_days"
	KeyAdminSingleAccountMode        = "admin.single_account_mode"
	KeyIngestMaxPayloadBytes         = "ingest.max_payload_bytes"
	KeyRuntimeDeliveryConcurrency    = "runtime.delivery_global_concurrency"
	KeyDeadLetterProcessingMode      = "dead_letter.processing_mode"
	DefaultIngestMaxPayloadBytes     = 5 << 20
	DefaultDeliveryGlobalConcurrency = 10
	MaxRuntimeDeliveryConcurrency    = 100000
	PerformanceWorkerModeSystem      = "system"
	PerformanceWorkerModeConcurrency = "concurrency"
)

var (
	ErrNotFound     = errors.New("setting not found")
	ErrInvalidInput = errors.New("invalid setting input")
)

type Setting struct {
	Key         string
	Value       json.RawMessage
	Description string
	Category    string
	UpdatedAt   time.Time
	CreatedAt   time.Time
}

type UpdateInput struct {
	Value json.RawMessage `json:"value"`
}

type PerformanceTestInput struct {
	MessageCount           int                                 `json:"message_count"`
	SourceCount            int                                 `json:"source_count"`
	PayloadVariantCount    int                                 `json:"payload_variant_count"`
	SourceAuthMode         string                              `json:"auth_mode"`
	MaxConcurrency         int                                 `json:"max_concurrency"`
	ConcurrencyRange       string                              `json:"concurrency_range"`
	ConcurrencyStart       int                                 `json:"concurrency_start"`
	ConcurrencyEnd         int                                 `json:"concurrency_end"`
	ConcurrencyCandidates  []int                               `json:"concurrency_candidates"`
	WorkerMode             string                              `json:"worker_mode"`
	GeneratedSourceCode    string                              `json:"-"`
	GeneratedRouteName     string                              `json:"-"`
	GeneratedChannelName   string                              `json:"-"`
	Observations           []PerformanceTestObservation        `json:"-"`
	Diagnostics            PerformanceRuntimeDiagnostics       `json:"-"`
	ConcurrencyDiagnostics []PerformanceConcurrencyDiagnostics `json:"-"`
	DBTimings              map[string][]int                    `json:"-"`
}

type PerformanceTestObservation struct {
	SampleID                           string
	TraceID                            string
	StartedAt                          time.Time
	Concurrency                        int
	InboundDurationMS                  int
	RouteDurationMS                    int
	TemplateRenderDurationMS           int
	DispatchDurationMS                 int
	ReceiveDurationMS                  int
	EndToEndDurationMS                 int
	CompletionEndToEndDurationMS       int
	AcceptedRunDurationMS              int
	DispatchRunDurationMS              int
	ConcurrencyRunDurationMS           int
	DBPoolWaitDurationMS               int
	SourceLookupDurationMS             int
	LatestPayloadUpdateDurationMS      int
	EnqueueInboundDurationMS           int
	InsertMessageRecordDurationMS      int
	InsertInboundDedupeKeyDurationMS   int
	InsertRoutePlanJobDurationMS       int
	CommitInboundTransactionDurationMS int
	PlanningClaimDurationMS            int
	RoutePlanLookupDurationMS          int
	RouteConditionDurationMS           int
	PlanningTemplateRenderDurationMS   int
	PlanningCompleteDurationMS         int
	DeliveryClaimDurationMS            int
	DeliveryDispatchDurationMS         int
	DeliverySendDurationMS             int
	DeliveryCompleteDurationMS         int
	DBTimings                          map[string][]int
	WorkerCount                        int
	Success                            bool
	SlowRuleCount                      int
}

type PerformanceTestStageResult struct {
	Key        string  `json:"key"`
	Label      string  `json:"label"`
	Count      int     `json:"count"`
	AvgMS      float64 `json:"avg_ms"`
	P99MS      float64 `json:"p99_ms"`
	DurationMS int     `json:"duration_ms"`
}

type PerformanceTestConcurrencyResult struct {
	Concurrency         int                           `json:"concurrency"`
	MessageCount        int                           `json:"message_count"`
	ActualWorkerCount   int                           `json:"actual_worker_count"`
	SuccessRate         float64                       `json:"success_rate"`
	AcceptedQPS         float64                       `json:"accepted_qps"`
	DispatchQPS         float64                       `json:"dispatch_qps"`
	CompletionQPS       float64                       `json:"completion_qps"`
	SendQPS             float64                       `json:"send_qps"`
	DispatchP99MS       float64                       `json:"dispatch_p99_ms"`
	CompletionP99MS     float64                       `json:"completion_p99_ms"`
	RouteP99MS          float64                       `json:"route_p99_ms"`
	TemplateRenderP99MS float64                       `json:"template_render_p99_ms"`
	InboundWriteP99MS   float64                       `json:"inbound_write_p99_ms"`
	EndToEndP99MS       float64                       `json:"end_to_end_p99_ms"`
	WallClockMS         int                           `json:"wall_clock_ms"`
	Recommended         bool                          `json:"recommended"`
	Diagnostics         PerformanceRuntimeDiagnostics `json:"diagnostics"`
	StageResults        []PerformanceTestStageResult  `json:"stage_results"`
}

type PerformanceConcurrencyDiagnostics struct {
	Concurrency int                           `json:"concurrency"`
	Diagnostics PerformanceRuntimeDiagnostics `json:"diagnostics"`
}

type PerformanceRuntimeDiagnostics struct {
	DBPoolAcquireCountDelta   int64  `json:"db_pool_acquire_count_delta"`
	DBPoolWaitCountDelta      int64  `json:"db_pool_wait_count_delta"`
	DBPoolWaitDurationDeltaMS int64  `json:"db_pool_wait_duration_delta_ms"`
	DBPoolAcquiredConnsBefore int32  `json:"db_pool_acquired_conns_before"`
	DBPoolAcquiredConnsAfter  int32  `json:"db_pool_acquired_conns_after"`
	DBPoolTotalConnsBefore    int32  `json:"db_pool_total_conns_before"`
	DBPoolTotalConnsAfter     int32  `json:"db_pool_total_conns_after"`
	PostgresMaxConnections    int32  `json:"postgres_max_connections"`
	PostgresBlocksRead        int64  `json:"postgres_blocks_read"`
	PostgresBlocksHit         int64  `json:"postgres_blocks_hit"`
	PostgresTempBytes         int64  `json:"postgres_temp_bytes"`
	PostgresBlocksReadDelta   int64  `json:"postgres_blocks_read_delta"`
	PostgresBlocksHitDelta    int64  `json:"postgres_blocks_hit_delta"`
	PostgresTempBytesDelta    int64  `json:"postgres_temp_bytes_delta"`
	CPUCount                  int    `json:"cpu_count"`
	GoMaxProcs                int    `json:"go_max_procs"`
	QueueBacklogBefore        int    `json:"queue_backlog_before"`
	QueueBacklogAfter         int    `json:"queue_backlog_after"`
	QueueBacklogDelta         int    `json:"queue_backlog_delta"`
	QueueRoutePlanBefore      int    `json:"queue_route_plan_before"`
	QueueRoutePlanAfter       int    `json:"queue_route_plan_after"`
	QueueRoutePlanDelta       int    `json:"queue_route_plan_delta"`
	QueueSendMessageBefore    int    `json:"queue_send_message_before"`
	QueueSendMessageAfter     int    `json:"queue_send_message_after"`
	QueueSendMessageDelta     int    `json:"queue_send_message_delta"`
	QueueOldestWaitBefore     int64  `json:"queue_oldest_wait_before"`
	QueueOldestWaitAfter      int64  `json:"queue_oldest_wait_after"`
	GoroutinesBefore          int    `json:"goroutines_before"`
	GoroutinesAfter           int    `json:"goroutines_after"`
	GoroutinesDelta           int    `json:"goroutines_delta"`
	GoroutineGrowthWarning    bool   `json:"goroutine_growth_warning"`
	MemoryAllocBytesBefore    uint64 `json:"memory_alloc_bytes_before"`
	MemoryAllocBytesAfter     uint64 `json:"memory_alloc_bytes_after"`
	MemoryAllocDeltaBytes     int64  `json:"memory_alloc_delta_bytes"`
	MemorySysBytesBefore      uint64 `json:"memory_sys_bytes_before"`
	MemorySysBytesAfter       uint64 `json:"memory_sys_bytes_after"`
	GCCountDelta              uint32 `json:"gc_count_delta"`
	GCPauseTotalDeltaMS       uint64 `json:"gc_pause_total_delta_ms"`
}

type PerformanceTestResult struct {
	MessageCount                 int                                `json:"message_count"`
	SourceCount                  int                                `json:"source_count"`
	PayloadVariantCount          int                                `json:"payload_variant_count"`
	ConcurrencyRange             string                             `json:"concurrency_range"`
	GeneratedSourceCode          string                             `json:"generated_source_code"`
	GeneratedRouteName           string                             `json:"generated_route_name"`
	GeneratedChannelName         string                             `json:"generated_channel_name"`
	AcceptedCount                int                                `json:"accepted_count"`
	FailedCount                  int                                `json:"failed_count"`
	SuccessRate                  float64                            `json:"success_rate"`
	AvgInboundMS                 float64                            `json:"avg_inbound_ms"`
	P99InboundMS                 float64                            `json:"p99_inbound_ms"`
	AvgRouteMS                   float64                            `json:"avg_route_ms"`
	RouteP99MS                   float64                            `json:"route_p99_ms"`
	AvgTemplateRenderMS          float64                            `json:"avg_template_render_ms"`
	TemplateRenderP99MS          float64                            `json:"template_render_p99_ms"`
	AvgEndToEndMS                float64                            `json:"avg_end_to_end_ms"`
	EndToEndP99MS                float64                            `json:"end_to_end_p99_ms"`
	SlowRuleCount                int                                `json:"slow_rule_count"`
	RecommendedGlobalConcurrency int                                `json:"recommended_global_concurrency"`
	EstimatedAcceptedQPS         float64                            `json:"estimated_accepted_qps"`
	EstimatedDispatchQPS         float64                            `json:"estimated_dispatch_qps"`
	EstimatedCompletionQPS       float64                            `json:"estimated_completion_qps"`
	EstimatedSendQPS             float64                            `json:"estimated_send_qps"`
	CompletionEndToEndAvgMS      float64                            `json:"completion_end_to_end_avg_ms"`
	CompletionEndToEndP99MS      float64                            `json:"completion_end_to_end_p99_ms"`
	DurationMS                   int                                `json:"duration_ms"`
	RecommendationReason         string                             `json:"recommendation_reason"`
	UpdatedSettingKey            string                             `json:"updated_setting_key"`
	StageResults                 []PerformanceTestStageResult       `json:"stage_results"`
	ConcurrencyResults           []PerformanceTestConcurrencyResult `json:"concurrency_results"`
	Diagnostics                  PerformanceRuntimeDiagnostics      `json:"diagnostics"`
}

type Store interface {
	ListSettings(ctx context.Context) ([]Setting, error)
	GetSetting(ctx context.Context, key string) (Setting, error)
	UpdateSetting(ctx context.Context, key string, input UpdateInput) (Setting, error)
	EnsureDefaultSettings(ctx context.Context, defaults []Setting) error
}

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

func (s *Service) EnsureDefaults(ctx context.Context) error {
	return s.store.EnsureDefaultSettings(ctx, DefaultSettings())
}

func (s *Service) ListSettings(ctx context.Context) ([]Setting, error) {
	return s.store.ListSettings(ctx)
}

func (s *Service) UpdateSetting(ctx context.Context, key string, input UpdateInput) (Setting, error) {
	key = strings.TrimSpace(key)
	if !isAllowedKey(key) {
		return Setting{}, ErrInvalidInput
	}
	value, err := normalizeJSON(input.Value)
	if err != nil {
		return Setting{}, err
	}
	if err := validateSettingValue(key, value); err != nil {
		return Setting{}, err
	}
	input.Value = value
	return s.store.UpdateSetting(ctx, key, input)
}

func (s *Service) GetSetting(ctx context.Context, key string) (Setting, error) {
	key = strings.TrimSpace(key)
	if !isAllowedKey(key) {
		return Setting{}, ErrInvalidInput
	}
	return s.store.GetSetting(ctx, key)
}

func (s *Service) IntSetting(ctx context.Context, key string, fallback int) int {
	item, err := s.GetSetting(ctx, key)
	if err != nil {
		return fallback
	}
	var value int
	if err := json.Unmarshal(item.Value, &value); err != nil || value <= 0 {
		return fallback
	}
	return value
}

func (s *Service) RunPerformanceTest(ctx context.Context, input PerformanceTestInput) (PerformanceTestResult, error) {
	result, err := s.BuildPerformanceTestResult(input)
	if err != nil {
		return PerformanceTestResult{}, err
	}
	raw, err := json.Marshal(result.RecommendedGlobalConcurrency)
	if err != nil {
		return PerformanceTestResult{}, err
	}
	if _, err := s.UpdateSetting(ctx, KeyRuntimeDeliveryConcurrency, UpdateInput{Value: raw}); err != nil {
		return PerformanceTestResult{}, err
	}
	return result, nil
}

func (s *Service) BuildPerformanceTestResult(input PerformanceTestInput) (PerformanceTestResult, error) {
	targetMessageCount := PerformanceMessageCount(input)
	sourceCount := normalizeSmallPositive(input.SourceCount, 1, 5)
	payloadVariantCount := normalizeSmallPositive(input.PayloadVariantCount, 3, 12)
	candidates := PerformanceConcurrencyCandidates(input)
	observations := normalizedPerformanceObservations(input.Observations, targetMessageCount, candidates)
	stats := summarizePerformanceObservations(observations)
	messageCount := len(observations)
	concurrencyResults, recommended := buildConcurrencyResults(candidates, observations, stats, input.ConcurrencyDiagnostics)
	durationMS := performanceWallClockDuration(observations)
	acceptedQPS := qpsFromDuration(messageCount, performanceAcceptedWallClockDuration(observations))
	dispatchQPS := qpsFromDuration(performanceDispatchCount(observations), performanceDispatchWallClockDuration(observations))
	completionQPS := qpsFromDuration(messageCount, durationMS)
	diagnostics := normalizedPerformanceDiagnostics(input.Diagnostics)
	return PerformanceTestResult{
		MessageCount:                 messageCount,
		SourceCount:                  sourceCount,
		PayloadVariantCount:          payloadVariantCount,
		ConcurrencyRange:             normalizedConcurrencyRangeLabel(candidates),
		GeneratedSourceCode:          firstNonEmpty(input.GeneratedSourceCode, "perftestsource"),
		GeneratedRouteName:           firstNonEmpty(input.GeneratedRouteName, "性能测试路由"),
		GeneratedChannelName:         firstNonEmpty(input.GeneratedChannelName, "性能测试本地上级"),
		AcceptedCount:                stats.AcceptedCount,
		FailedCount:                  stats.FailedCount,
		SuccessRate:                  stats.SuccessRate,
		AvgInboundMS:                 stats.InboundAvgMS,
		P99InboundMS:                 stats.InboundP99MS,
		AvgRouteMS:                   stats.RouteAvgMS,
		RouteP99MS:                   stats.RouteP99MS,
		AvgTemplateRenderMS:          stats.TemplateRenderAvgMS,
		TemplateRenderP99MS:          stats.TemplateRenderP99MS,
		AvgEndToEndMS:                stats.EndToEndAvgMS,
		EndToEndP99MS:                stats.EndToEndP99MS,
		SlowRuleCount:                stats.SlowRuleCount,
		RecommendedGlobalConcurrency: recommended,
		EstimatedAcceptedQPS:         acceptedQPS,
		EstimatedDispatchQPS:         dispatchQPS,
		EstimatedCompletionQPS:       completionQPS,
		EstimatedSendQPS:             dispatchQPS,
		CompletionEndToEndAvgMS:      stats.CompletionEndToEndAvgMS,
		CompletionEndToEndP99MS:      stats.CompletionEndToEndP99MS,
		DurationMS:                   durationMS,
		RecommendationReason:         recommendationReason(recommended, stats),
		UpdatedSettingKey:            KeyRuntimeDeliveryConcurrency,
		StageResults:                 buildStageResults(observations, input.DBTimings),
		ConcurrencyResults:           concurrencyResults,
		Diagnostics:                  diagnostics,
	}, nil
}

func DefaultSettings() []Setting {
	return []Setting{
		{Key: KeyConsolePollingIntervalSeconds, Value: json.RawMessage(`5`), Description: "管理台轮询刷新间隔秒数", Category: "console"},
		{Key: KeyLogsRetentionDays, Value: json.RawMessage(`30`), Description: "消息日志和运行记录保留天数", Category: "logs"},
		{Key: KeyAdminSingleAccountMode, Value: json.RawMessage(`true`), Description: "一期管理员单账户模式", Category: "admin"},
		{Key: KeyIngestMaxPayloadBytes, Value: json.RawMessage(`5242880`), Description: "入站 Payload 最大字节数", Category: "performance"},
		{Key: KeyRuntimeDeliveryConcurrency, Value: json.RawMessage(`10`), Description: "当前系统实例并发上限", Category: "performance"},
		{Key: KeyDeadLetterProcessingMode, Value: json.RawMessage(`"manual"`), Description: "死信处理模式：manual 手动处理，auto 自动重放", Category: "dead_letter"},
	}
}

func isAllowedKey(key string) bool {
	for _, item := range DefaultSettings() {
		if item.Key == key {
			return true
		}
	}
	return false
}

func validateSettingValue(key string, raw json.RawMessage) error {
	switch key {
	case KeyConsolePollingIntervalSeconds:
		return validateIntRange(raw, 1, 300)
	case KeyLogsRetentionDays:
		return validateIntRange(raw, 1, 3650)
	case KeyIngestMaxPayloadBytes:
		return validateIntRange(raw, 1024, 50<<20)
	case KeyRuntimeDeliveryConcurrency:
		return validateIntRange(raw, 1, MaxRuntimeDeliveryConcurrency)
	case KeyAdminSingleAccountMode:
		var value bool
		if err := json.Unmarshal(raw, &value); err != nil {
			return ErrInvalidInput
		}
		return nil
	case KeyDeadLetterProcessingMode:
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return ErrInvalidInput
		}
		switch value {
		case "manual", "auto":
			return nil
		default:
			return ErrInvalidInput
		}
	default:
		return ErrInvalidInput
	}
}

func validateIntRange(raw json.RawMessage, minValue int, maxValue int) error {
	var value int
	if err := json.Unmarshal(raw, &value); err != nil {
		return ErrInvalidInput
	}
	if value < minValue || value > maxValue {
		return ErrInvalidInput
	}
	return nil
}

func normalizeJSON(raw json.RawMessage) (json.RawMessage, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, ErrInvalidInput
	}
	if !json.Valid(raw) {
		return nil, ErrInvalidInput
	}
	return append(json.RawMessage(nil), bytes.TrimSpace(raw)...), nil
}

func normalizeMessageCount(value int) int {
	if value <= 0 {
		return 200
	}
	return value
}

func PerformanceMessageCount(input PerformanceTestInput) int {
	if input.MessageCount > 0 {
		return normalizeMessageCount(input.MessageCount)
	}
	candidates := PerformanceConcurrencyCandidates(input)
	return normalizeMessageCount(candidates[len(candidates)-1])
}

func PerformanceMessageCountForConcurrency(input PerformanceTestInput, concurrency int) int {
	if input.MessageCount > 0 {
		return normalizeMessageCount(input.MessageCount)
	}
	if concurrency <= 0 {
		concurrency = 1
	}
	return normalizeMessageCount(concurrency)
}

func PerformanceWorkerMode(input PerformanceTestInput) string {
	if strings.TrimSpace(input.WorkerMode) == PerformanceWorkerModeConcurrency {
		return PerformanceWorkerModeConcurrency
	}
	return PerformanceWorkerModeSystem
}

type performanceObservationStats struct {
	AcceptedCount           int
	FailedCount             int
	SuccessRate             float64
	InboundAvgMS            float64
	InboundP99MS            float64
	RouteAvgMS              float64
	RouteP99MS              float64
	TemplateRenderAvgMS     float64
	TemplateRenderP99MS     float64
	EndToEndAvgMS           float64
	EndToEndP99MS           float64
	CompletionEndToEndAvgMS float64
	CompletionEndToEndP99MS float64
	SlowRuleCount           int
}

func normalizeSmallPositive(value int, fallback int, maxValue int) int {
	if value <= 0 {
		return fallback
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func normalizedPerformanceObservations(observations []PerformanceTestObservation, messageCount int, candidates []int) []PerformanceTestObservation {
	if len(observations) == 0 {
		recommended := recommendedConcurrency(messageCount)
		duration := estimatedBenchmarkDurationMS(messageCount, recommended)
		synthetic := make([]PerformanceTestObservation, 0, messageCount)
		if len(candidates) == 0 {
			candidates = []int{recommended}
		}
		for index := 0; index < messageCount; index++ {
			inbound := 2 + index%3
			route := 1 + index%5
			templateRender := 1 + index%3
			receive := maxInt(1, duration/messageCount)
			synthetic = append(synthetic, PerformanceTestObservation{
				Concurrency:                  candidates[index%len(candidates)],
				InboundDurationMS:            inbound,
				RouteDurationMS:              route,
				TemplateRenderDurationMS:     templateRender,
				DispatchDurationMS:           receive,
				ReceiveDurationMS:            receive,
				EndToEndDurationMS:           inbound + route + templateRender + receive,
				CompletionEndToEndDurationMS: inbound + route + templateRender + receive,
				AcceptedRunDurationMS:        duration,
				DispatchRunDurationMS:        duration,
				ConcurrencyRunDurationMS:     duration,
				Success:                      true,
			})
		}
		return synthetic
	}
	normalized := make([]PerformanceTestObservation, 0, len(observations))
	for _, item := range observations {
		if item.InboundDurationMS < 0 {
			item.InboundDurationMS = 0
		}
		if item.RouteDurationMS < 0 {
			item.RouteDurationMS = 0
		}
		if item.TemplateRenderDurationMS < 0 {
			item.TemplateRenderDurationMS = 0
		}
		if item.DispatchDurationMS < 0 {
			item.DispatchDurationMS = 0
		}
		if item.ReceiveDurationMS < 0 {
			item.ReceiveDurationMS = 0
		}
		if item.DispatchDurationMS <= 0 {
			item.DispatchDurationMS = item.ReceiveDurationMS
		}
		if item.EndToEndDurationMS <= 0 {
			item.EndToEndDurationMS = maxInt(1, item.InboundDurationMS+item.RouteDurationMS+item.TemplateRenderDurationMS+item.DispatchDurationMS)
		}
		if item.CompletionEndToEndDurationMS <= 0 {
			item.CompletionEndToEndDurationMS = maxInt(item.EndToEndDurationMS, item.InboundDurationMS+item.RouteDurationMS+item.TemplateRenderDurationMS+item.ReceiveDurationMS)
		}
		if item.AcceptedRunDurationMS < 0 {
			item.AcceptedRunDurationMS = 0
		}
		if item.DispatchRunDurationMS < 0 {
			item.DispatchRunDurationMS = 0
		}
		if item.ConcurrencyRunDurationMS < 0 {
			item.ConcurrencyRunDurationMS = 0
		}
		if item.AcceptedRunDurationMS <= 0 {
			item.AcceptedRunDurationMS = firstPositiveInt(item.DispatchRunDurationMS, item.ConcurrencyRunDurationMS)
		}
		if item.DispatchRunDurationMS <= 0 {
			item.DispatchRunDurationMS = firstPositiveInt(item.ConcurrencyRunDurationMS, item.AcceptedRunDurationMS)
		}
		if item.DBPoolWaitDurationMS < 0 {
			item.DBPoolWaitDurationMS = 0
		}
		if item.SourceLookupDurationMS < 0 {
			item.SourceLookupDurationMS = 0
		}
		if item.LatestPayloadUpdateDurationMS < 0 {
			item.LatestPayloadUpdateDurationMS = 0
		}
		if item.EnqueueInboundDurationMS < 0 {
			item.EnqueueInboundDurationMS = 0
		}
		if item.InsertMessageRecordDurationMS < 0 {
			item.InsertMessageRecordDurationMS = 0
		}
		if item.InsertInboundDedupeKeyDurationMS < 0 {
			item.InsertInboundDedupeKeyDurationMS = 0
		}
		if item.InsertRoutePlanJobDurationMS < 0 {
			item.InsertRoutePlanJobDurationMS = 0
		}
		if item.CommitInboundTransactionDurationMS < 0 {
			item.CommitInboundTransactionDurationMS = 0
		}
		if item.PlanningClaimDurationMS < 0 {
			item.PlanningClaimDurationMS = 0
		}
		if item.RoutePlanLookupDurationMS < 0 {
			item.RoutePlanLookupDurationMS = 0
		}
		if item.RouteConditionDurationMS < 0 {
			item.RouteConditionDurationMS = 0
		}
		if item.PlanningTemplateRenderDurationMS < 0 {
			item.PlanningTemplateRenderDurationMS = 0
		}
		if item.PlanningCompleteDurationMS < 0 {
			item.PlanningCompleteDurationMS = 0
		}
		if item.DeliveryClaimDurationMS < 0 {
			item.DeliveryClaimDurationMS = 0
		}
		if item.DeliveryDispatchDurationMS < 0 {
			item.DeliveryDispatchDurationMS = 0
		}
		if item.DeliverySendDurationMS < 0 {
			item.DeliverySendDurationMS = 0
		}
		if item.DeliveryCompleteDurationMS < 0 {
			item.DeliveryCompleteDurationMS = 0
		}
		normalized = append(normalized, item)
	}
	return normalized
}

func summarizePerformanceObservations(observations []PerformanceTestObservation) performanceObservationStats {
	inbound := make([]int, 0, len(observations))
	routeDurations := make([]int, 0, len(observations))
	templateDurations := make([]int, 0, len(observations))
	endToEnd := make([]int, 0, len(observations))
	completionEndToEnd := make([]int, 0, len(observations))
	stats := performanceObservationStats{}
	for _, item := range observations {
		inbound = append(inbound, item.InboundDurationMS)
		routeDurations = append(routeDurations, item.RouteDurationMS)
		templateDurations = append(templateDurations, item.TemplateRenderDurationMS)
		endToEnd = append(endToEnd, item.EndToEndDurationMS)
		completionEndToEnd = append(completionEndToEnd, item.CompletionEndToEndDurationMS)
		stats.SlowRuleCount += item.SlowRuleCount
		if item.Success {
			stats.AcceptedCount++
		} else {
			stats.FailedCount++
		}
	}
	total := len(observations)
	if total > 0 {
		stats.SuccessRate = roundFloat(float64(stats.AcceptedCount)/float64(total)*100, 2)
	}
	stats.InboundAvgMS = averageInt(inbound)
	stats.InboundP99MS = percentileInt(inbound, 0.99)
	stats.RouteAvgMS = averageInt(routeDurations)
	stats.RouteP99MS = percentileInt(routeDurations, 0.99)
	stats.TemplateRenderAvgMS = averageInt(templateDurations)
	stats.TemplateRenderP99MS = percentileInt(templateDurations, 0.99)
	stats.EndToEndAvgMS = averageInt(endToEnd)
	stats.EndToEndP99MS = percentileInt(endToEnd, 0.99)
	stats.CompletionEndToEndAvgMS = averageInt(completionEndToEnd)
	stats.CompletionEndToEndP99MS = percentileInt(completionEndToEnd, 0.99)
	return stats
}

func PerformanceConcurrencyCandidates(input PerformanceTestInput) []int {
	if candidates := normalizeConcurrencyStartEnd(input.ConcurrencyStart, input.ConcurrencyEnd); len(candidates) > 0 {
		return candidates
	}
	if candidates := parseConcurrencyRange(input.ConcurrencyRange); len(candidates) > 0 {
		return candidates
	}
	if input.MaxConcurrency > 0 {
		maxConcurrency := input.MaxConcurrency
		candidates := make([]int, 0, maxConcurrency)
		for value := 1; value <= maxConcurrency; value++ {
			candidates = append(candidates, value)
		}
		return candidates
	}
	return normalizeConcurrencyCandidates(input.ConcurrencyCandidates)
}

func normalizeConcurrencyStartEnd(start int, end int) []int {
	if start <= 0 && end <= 0 {
		return nil
	}
	if start <= 0 {
		start = 1
	}
	if end <= 0 {
		end = start
	}
	if start > end {
		start, end = end, start
	}
	return intRange(start, end)
}

func normalizeConcurrencyCandidates(values []int) []int {
	if len(values) == 0 {
		return intRange(1, 16)
	}
	seen := map[int]bool{}
	candidates := make([]int, 0, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if !seen[value] {
			seen[value] = true
			candidates = append(candidates, value)
		}
	}
	if len(candidates) == 0 {
		return intRange(1, 16)
	}
	sort.Ints(candidates)
	return candidates
}

func parseConcurrencyRange(value string) []int {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	value = strings.ReplaceAll(value, " ", "")
	if single, err := strconv.Atoi(value); err == nil {
		if single <= 0 {
			return nil
		}
		return intRange(1, single)
	}
	parts := strings.Split(value, "-")
	if len(parts) != 2 {
		return nil
	}
	start, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil
	}
	end, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil
	}
	if start <= 0 || end <= 0 {
		return nil
	}
	if start > end {
		start, end = end, start
	}
	return intRange(start, end)
}

func intRange(start int, end int) []int {
	if start <= 0 || end <= 0 || start > end {
		return nil
	}
	values := make([]int, 0, end-start+1)
	for value := start; value <= end; value++ {
		values = append(values, value)
	}
	return values
}

func recommendConcurrencyFromMetrics(p99MS float64, successRate float64, candidates []int) int {
	target := 16
	switch {
	case successRate < 95:
		target = 4
	case p99MS <= 25:
		target = 64
	case p99MS <= 50:
		target = 32
	case p99MS <= 120:
		target = 16
	case p99MS <= 250:
		target = 8
	default:
		target = 4
	}
	recommended := candidates[0]
	for _, candidate := range candidates {
		if candidate <= target {
			recommended = candidate
		}
	}
	return recommended
}

func buildConcurrencyResults(candidates []int, observations []PerformanceTestObservation, stats performanceObservationStats, diagnostics []PerformanceConcurrencyDiagnostics) ([]PerformanceTestConcurrencyResult, int) {
	if len(candidates) == 0 {
		candidates = intRange(1, 16)
	}
	diagnosticsByConcurrency := map[int]PerformanceRuntimeDiagnostics{}
	for _, item := range diagnostics {
		diagnosticsByConcurrency[item.Concurrency] = normalizedPerformanceDiagnostics(item.Diagnostics)
	}
	hasBucketedObservations := false
	for _, item := range observations {
		if item.Concurrency > 0 {
			hasBucketedObservations = true
			break
		}
	}
	if !hasBucketedObservations {
		recommended := recommendConcurrencyFromMetrics(stats.EndToEndP99MS, stats.SuccessRate, candidates)
		results := buildEstimatedConcurrencyResults(candidates, recommended, len(observations), stats)
		for index := range results {
			results[index].Diagnostics = diagnosticsByConcurrency[results[index].Concurrency]
		}
		return results, recommended
	}
	results := make([]PerformanceTestConcurrencyResult, 0, len(candidates))
	for _, candidate := range candidates {
		bucket := make([]PerformanceTestObservation, 0)
		for _, item := range observations {
			if item.Concurrency == candidate {
				bucket = append(bucket, item)
			}
		}
		result := concurrencyResultFromBucket(candidate, bucket)
		result.Diagnostics = diagnosticsByConcurrency[candidate]
		results = append(results, result)
	}
	recommended := recommendConcurrencyFromResults(results, candidates)
	for index := range results {
		results[index].Recommended = results[index].Concurrency == recommended
	}
	return results, recommended
}

func buildEstimatedConcurrencyResults(candidates []int, recommended int, messageCount int, stats performanceObservationStats) []PerformanceTestConcurrencyResult {
	results := make([]PerformanceTestConcurrencyResult, 0, len(candidates))
	baseAvg := math.Max(stats.EndToEndAvgMS, 1)
	for _, candidate := range candidates {
		pressure := 1.0
		if candidate > recommended && recommended > 0 {
			pressure += float64(candidate-recommended) / float64(recommended) * 0.45
		}
		dispatchQPS := roundFloat(float64(candidate)*1000/baseAvg, 1)
		completionP99 := stats.CompletionEndToEndP99MS
		if completionP99 <= 0 {
			completionP99 = stats.EndToEndP99MS
		}
		results = append(results, PerformanceTestConcurrencyResult{
			Concurrency:         candidate,
			MessageCount:        messageCount,
			ActualWorkerCount:   minInt(candidate, messageCount),
			SuccessRate:         stats.SuccessRate,
			AcceptedQPS:         dispatchQPS,
			DispatchQPS:         dispatchQPS,
			CompletionQPS:       dispatchQPS,
			SendQPS:             dispatchQPS,
			DispatchP99MS:       roundFloat(stats.EndToEndP99MS*pressure, 2),
			CompletionP99MS:     roundFloat(completionP99*pressure, 2),
			RouteP99MS:          roundFloat(stats.RouteP99MS*pressure, 2),
			TemplateRenderP99MS: roundFloat(stats.TemplateRenderP99MS*pressure, 2),
			InboundWriteP99MS:   roundFloat(stats.InboundP99MS*pressure, 2),
			EndToEndP99MS:       roundFloat(stats.EndToEndP99MS*pressure, 2),
			WallClockMS:         performanceWallClockDurationFromCount(messageCount, candidate, stats.EndToEndAvgMS),
			Recommended:         candidate == recommended,
			StageResults:        estimatedStageResults(stats, pressure, messageCount),
		})
	}
	return results
}

func concurrencyResultFromBucket(concurrency int, observations []PerformanceTestObservation) PerformanceTestConcurrencyResult {
	stats := summarizePerformanceObservations(observations)
	completionDurationMS := performanceWallClockDuration(observations)
	dispatchDurationMS := performanceDispatchWallClockDuration(observations)
	acceptedDurationMS := performanceAcceptedWallClockDuration(observations)
	messageCount := len(observations)
	acceptedQPS := qpsFromDuration(messageCount, acceptedDurationMS)
	dispatchQPS := qpsFromDuration(performanceDispatchCount(observations), dispatchDurationMS)
	completionQPS := qpsFromDuration(messageCount, completionDurationMS)
	workerCount := 0
	for _, observation := range observations {
		if observation.WorkerCount > workerCount {
			workerCount = observation.WorkerCount
		}
	}
	if workerCount <= 0 {
		workerCount = minInt(concurrency, len(observations))
	}
	return PerformanceTestConcurrencyResult{
		Concurrency:         concurrency,
		MessageCount:        len(observations),
		ActualWorkerCount:   workerCount,
		SuccessRate:         stats.SuccessRate,
		AcceptedQPS:         acceptedQPS,
		DispatchQPS:         dispatchQPS,
		CompletionQPS:       completionQPS,
		SendQPS:             dispatchQPS,
		DispatchP99MS:       stats.EndToEndP99MS,
		CompletionP99MS:     stats.CompletionEndToEndP99MS,
		RouteP99MS:          stats.RouteP99MS,
		TemplateRenderP99MS: stats.TemplateRenderP99MS,
		InboundWriteP99MS:   stats.InboundP99MS,
		EndToEndP99MS:       stats.EndToEndP99MS,
		WallClockMS:         dispatchDurationMS,
		StageResults:        buildStageResults(observations, nil),
	}
}

func estimatedStageResults(stats performanceObservationStats, pressure float64, messageCount int) []PerformanceTestStageResult {
	if pressure <= 0 {
		pressure = 1
	}
	return []PerformanceTestStageResult{
		{
			Key:        "ingest",
			Label:      "入站写库",
			Count:      messageCount,
			AvgMS:      roundFloat(stats.InboundAvgMS*pressure, 2),
			P99MS:      roundFloat(stats.InboundP99MS*pressure, 2),
			DurationMS: int(math.Round(stats.InboundAvgMS * pressure * float64(messageCount))),
		},
		{
			Key:        "dispatch",
			Label:      "出站链路",
			Count:      messageCount,
			AvgMS:      roundFloat(stats.EndToEndAvgMS*pressure, 2),
			P99MS:      roundFloat(stats.EndToEndP99MS*pressure, 2),
			DurationMS: int(math.Round(stats.EndToEndAvgMS * pressure * float64(messageCount))),
		},
		{
			Key:        "completion",
			Label:      "完整链路",
			Count:      messageCount,
			AvgMS:      roundFloat(stats.CompletionEndToEndAvgMS*pressure, 2),
			P99MS:      roundFloat(stats.CompletionEndToEndP99MS*pressure, 2),
			DurationMS: int(math.Round(stats.CompletionEndToEndAvgMS * pressure * float64(messageCount))),
		},
	}
}

func recommendConcurrencyFromResults(results []PerformanceTestConcurrencyResult, candidates []int) int {
	if len(candidates) == 0 {
		return 1
	}
	recommended := candidates[0]
	bestQPS := -1.0
	bestP99 := math.MaxFloat64
	for _, item := range results {
		if item.MessageCount == 0 || item.SuccessRate < 95 {
			continue
		}
		if item.SendQPS > bestQPS || (item.SendQPS == bestQPS && item.EndToEndP99MS < bestP99) {
			recommended = item.Concurrency
			bestQPS = item.SendQPS
			bestP99 = item.EndToEndP99MS
		}
	}
	return recommended
}

func buildStageResults(observations []PerformanceTestObservation, globalDBTimings map[string][]int) []PerformanceTestStageResult {
	results := []PerformanceTestStageResult{
		stageResult("prepare", "准备测试资源", []int{1}),
		stageResult("template", "请求前模板预览", collectDurations(observations, func(item PerformanceTestObservation) int {
			return item.TemplateRenderDurationMS
		})),
		stageResult("ingest", "入站写库", collectDurations(observations, func(item PerformanceTestObservation) int {
			return item.InboundDurationMS
		})),
		stageResult("dispatch", "出站链路", collectDurations(observations, func(item PerformanceTestObservation) int {
			return item.EndToEndDurationMS
		})),
		stageResult("completion", "完整链路", collectDurations(observations, func(item PerformanceTestObservation) int {
			return item.CompletionEndToEndDurationMS
		})),
	}
	results = appendOptionalStageResult(results, "source_lookup", "来源配置查询", observations, func(item PerformanceTestObservation) int {
		return item.SourceLookupDurationMS
	})
	results = appendOptionalStageResult(results, "latest_payload", "最近 Payload 更新", observations, func(item PerformanceTestObservation) int {
		return item.LatestPayloadUpdateDurationMS
	})
	results = appendOptionalStageResult(results, "enqueue_inbound", "入站接收写入", observations, func(item PerformanceTestObservation) int {
		return item.EnqueueInboundDurationMS
	})
	results = appendOptionalStageResult(results, "insert_message_record", "写入消息主记录", observations, func(item PerformanceTestObservation) int {
		return item.InsertMessageRecordDurationMS
	})
	results = appendOptionalStageResult(results, "insert_inbound_dedupe_key", "写入入站去重键", observations, func(item PerformanceTestObservation) int {
		return item.InsertInboundDedupeKeyDurationMS
	})
	results = appendOptionalStageResult(results, "insert_route_plan_job", "写入路由规划任务", observations, func(item PerformanceTestObservation) int {
		return item.InsertRoutePlanJobDurationMS
	})
	results = appendOptionalStageResult(results, "commit_inbound", "提交入站事务", observations, func(item PerformanceTestObservation) int {
		return item.CommitInboundTransactionDurationMS
	})
	results = appendOptionalStageResult(results, "planning_claim", "路由任务领取", observations, func(item PerformanceTestObservation) int {
		return item.PlanningClaimDurationMS
	})
	results = appendOptionalStageResult(results, "route_plan_lookup", "路由缓存/加载", observations, func(item PerformanceTestObservation) int {
		return item.RoutePlanLookupDurationMS
	})
	results = appendOptionalStageResult(results, "route_condition", "条件判断", observations, func(item PerformanceTestObservation) int {
		return item.RouteConditionDurationMS
	})
	results = appendOptionalStageResult(results, "planning_template_render", "规划模板渲染", observations, func(item PerformanceTestObservation) int {
		return item.PlanningTemplateRenderDurationMS
	})
	results = appendOptionalStageResult(results, "planning_complete", "写入投递任务", observations, func(item PerformanceTestObservation) int {
		return item.PlanningCompleteDurationMS
	})
	results = appendOptionalStageResult(results, "delivery_claim", "发送任务领取", observations, func(item PerformanceTestObservation) int {
		return item.DeliveryClaimDurationMS
	})
	results = appendOptionalStageResult(results, "delivery_send", "上级请求往返", observations, func(item PerformanceTestObservation) int {
		return item.DeliverySendDurationMS
	})
	results = appendOptionalStageResult(results, "delivery_complete", "发送结果落库", observations, func(item PerformanceTestObservation) int {
		return item.DeliveryCompleteDurationMS
	})
	results = appendDBStageResults(results, observations, globalDBTimings)
	return results
}

func appendDBStageResults(results []PerformanceTestStageResult, observations []PerformanceTestObservation, global map[string][]int) []PerformanceTestStageResult {
	for _, definition := range dbStageDefinitions() {
		durations := make([]int, 0, len(global[definition.key]))
		durations = append(durations, global[definition.key]...)
		for _, observation := range observations {
			if observation.DBTimings == nil {
				continue
			}
			durations = append(durations, observation.DBTimings[definition.key]...)
		}
		if len(durations) == 0 {
			continue
		}
		results = append(results, stageResult(definition.key, definition.label, durations))
	}
	return results
}

func dbStageDefinitions() []struct {
	key   string
	label string
} {
	return []struct {
		key   string
		label string
	}{
		{key: "db.acquire.enqueue_inbound", label: "DB 等待：入站入队"},
		{key: "db.query.enqueue_inbound_fast", label: "SQL 执行：入站快速入队"},
		{key: "db.query.insert_message_record", label: "SQL 执行：写入消息主记录"},
		{key: "db.query.insert_inbound_dedupe_key", label: "SQL 执行：写入入站去重键"},
		{key: "db.query.insert_route_plan_job", label: "SQL 执行：写入路由规划任务"},
		{key: "db.query.commit_inbound", label: "SQL 执行：提交入站事务"},
		{key: "db.acquire.claim_route_jobs", label: "DB 等待：路由任务领取"},
		{key: "db.query.claim_route_jobs", label: "SQL 执行：路由任务领取"},
		{key: "db.acquire.claim_send_jobs", label: "DB 等待：发送任务领取"},
		{key: "db.query.claim_send_jobs", label: "SQL 执行：发送任务领取"},
		{key: "db.query.claim_send_jobs_fast_path", label: "SQL 执行：发送任务领取 fast path"},
		{key: "db.acquire.complete_planning", label: "DB 等待：写入投递任务"},
		{key: "db.query.complete_planning", label: "SQL 执行：写入投递任务"},
		{key: "db.acquire.complete_delivery", label: "DB 等待：发送结果落库"},
		{key: "db.query.complete_delivery", label: "SQL 执行：发送结果落库"},
		{key: "db.query.complete_delivery_batch", label: "SQL 执行：批量发送结果落库"},
	}
}

func appendOptionalStageResult(results []PerformanceTestStageResult, key string, label string, observations []PerformanceTestObservation, pick func(PerformanceTestObservation) int) []PerformanceTestStageResult {
	durations := collectPositiveDurations(observations, pick)
	if len(durations) == 0 {
		return results
	}
	return append(results, stageResult(key, label, durations))
}

func stageResult(key string, label string, durations []int) PerformanceTestStageResult {
	return PerformanceTestStageResult{
		Key:        key,
		Label:      label,
		Count:      len(durations),
		AvgMS:      averageInt(durations),
		P99MS:      percentileInt(durations, 0.99),
		DurationMS: int(sumInts(durations)),
	}
}

func recommendationReason(recommended int, stats performanceObservationStats) string {
	if stats.SuccessRate < 95 {
		return "成功率偏低，建议先使用较小并发观察失败原因"
	}
	return "基于端到端 P99、路由选择耗时和成功率给出推荐并发"
}

func collectDurations(observations []PerformanceTestObservation, pick func(PerformanceTestObservation) int) []int {
	values := make([]int, 0, len(observations))
	for _, item := range observations {
		values = append(values, maxInt(0, pick(item)))
	}
	return values
}

func collectPositiveDurations(observations []PerformanceTestObservation, pick func(PerformanceTestObservation) int) []int {
	values := make([]int, 0, len(observations))
	for _, item := range observations {
		value := maxInt(0, pick(item))
		if value > 0 {
			values = append(values, value)
		}
	}
	return values
}

func sumDurations(observations []PerformanceTestObservation, pick func(PerformanceTestObservation) int) float64 {
	return sumInts(collectDurations(observations, pick))
}

func qpsFromDuration(count int, durationMS int) float64 {
	if count <= 0 {
		return 0
	}
	return roundFloat((float64(count)/float64(maxInt(durationMS, 1)))*1000, 1)
}

func performanceDispatchCount(observations []PerformanceTestObservation) int {
	if len(observations) == 0 {
		return 0
	}
	count := 0
	hasTrace := false
	for _, item := range observations {
		if strings.TrimSpace(item.TraceID) != "" {
			hasTrace = true
		}
		if item.DeliveryDispatchDurationMS > 0 {
			count++
		}
	}
	if count > 0 || hasTrace {
		return count
	}
	return len(observations)
}

func performanceAcceptedWallClockDuration(observations []PerformanceTestObservation) int {
	return performanceWallClockDurationByStage(observations, func(item PerformanceTestObservation) int {
		return item.AcceptedRunDurationMS
	}, func(item PerformanceTestObservation) int {
		return item.InboundDurationMS
	})
}

func performanceDispatchWallClockDuration(observations []PerformanceTestObservation) int {
	return performanceWallClockDurationByStage(observations, func(item PerformanceTestObservation) int {
		return item.DispatchRunDurationMS
	}, func(item PerformanceTestObservation) int {
		return item.EndToEndDurationMS
	})
}

func performanceWallClockDuration(observations []PerformanceTestObservation) int {
	return performanceWallClockDurationByStage(observations, func(item PerformanceTestObservation) int {
		return item.ConcurrencyRunDurationMS
	}, func(item PerformanceTestObservation) int {
		return item.CompletionEndToEndDurationMS
	})
}

func performanceWallClockDurationByStage(observations []PerformanceTestObservation, pickRunDuration func(PerformanceTestObservation) int, pickFallbackDuration func(PerformanceTestObservation) int) int {
	if len(observations) == 0 {
		return 1
	}
	hasRunDurations := false
	groupDurations := map[int]int{}
	for index, item := range observations {
		duration := pickRunDuration(item)
		if duration <= 0 {
			continue
		}
		hasRunDurations = true
		key := item.Concurrency
		if key <= 0 {
			key = index + 1
		}
		if duration > groupDurations[key] {
			groupDurations[key] = duration
		}
	}
	if hasRunDurations {
		total := 0
		for _, value := range groupDurations {
			total += value
		}
		return maxInt(1, total)
	}
	return maxInt(1, int(math.Round(sumDurations(observations, func(item PerformanceTestObservation) int {
		return pickFallbackDuration(item)
	}))))
}

func performanceWallClockDurationFromCount(messageCount int, concurrency int, avgEndToEndMS float64) int {
	if messageCount <= 0 {
		return 1
	}
	if concurrency <= 0 {
		concurrency = 1
	}
	avg := math.Max(avgEndToEndMS, 1)
	return maxInt(1, int(math.Ceil(float64(messageCount)/float64(concurrency)*avg)))
}

func normalizedPerformanceDiagnostics(input PerformanceRuntimeDiagnostics) PerformanceRuntimeDiagnostics {
	input.QueueBacklogDelta = input.QueueBacklogAfter - input.QueueBacklogBefore
	input.QueueRoutePlanDelta = input.QueueRoutePlanAfter - input.QueueRoutePlanBefore
	input.QueueSendMessageDelta = input.QueueSendMessageAfter - input.QueueSendMessageBefore
	input.GoroutinesDelta = input.GoroutinesAfter - input.GoroutinesBefore
	if input.DBPoolAcquireCountDelta < 0 {
		input.DBPoolAcquireCountDelta = 0
	}
	if input.DBPoolWaitCountDelta < 0 {
		input.DBPoolWaitCountDelta = 0
	}
	if input.DBPoolWaitDurationDeltaMS < 0 {
		input.DBPoolWaitDurationDeltaMS = 0
	}
	if input.PostgresBlocksReadDelta < 0 {
		input.PostgresBlocksReadDelta = 0
	}
	if input.PostgresBlocksHitDelta < 0 {
		input.PostgresBlocksHitDelta = 0
	}
	if input.PostgresTempBytesDelta < 0 {
		input.PostgresTempBytesDelta = 0
	}
	if input.GoroutinesDelta > 100 {
		input.GoroutineGrowthWarning = true
	}
	input.MemoryAllocDeltaBytes = int64(input.MemoryAllocBytesAfter) - int64(input.MemoryAllocBytesBefore)
	return input
}

func normalizedConcurrencyRangeLabel(candidates []int) string {
	if len(candidates) == 0 {
		return ""
	}
	if len(candidates) == 1 {
		return strconv.Itoa(candidates[0])
	}
	return strconv.Itoa(candidates[0]) + "-" + strconv.Itoa(candidates[len(candidates)-1])
}

func sumInts(values []int) float64 {
	total := 0
	for _, value := range values {
		total += value
	}
	return float64(total)
}

func averageInt(values []int) float64 {
	if len(values) == 0 {
		return 0
	}
	return roundFloat(sumInts(values)/float64(len(values)), 2)
}

func percentileInt(values []int, percentile float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]int(nil), values...)
	sort.Ints(sorted)
	index := int(math.Ceil(float64(len(sorted))*percentile)) - 1
	if index < 0 {
		index = 0
	}
	if index >= len(sorted) {
		index = len(sorted) - 1
	}
	return float64(sorted[index])
}

func roundFloat(value float64, precision int) float64 {
	factor := math.Pow10(precision)
	return math.Round(value*factor) / factor
}

func maxInt(left int, right int) int {
	if left > right {
		return left
	}
	return right
}

func minInt(left int, right int) int {
	if left < right {
		return left
	}
	return right
}

func firstPositiveInt(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func recommendedConcurrency(messageCount int) int {
	recommended := int(math.Ceil(math.Sqrt(float64(messageCount))))
	if recommended < 1 {
		return 1
	}
	if recommended > 64 {
		return 64
	}
	return recommended
}

func estimatedBenchmarkDurationMS(messageCount int, concurrency int) int {
	if concurrency <= 0 {
		concurrency = 1
	}
	durationMS := int(math.Ceil(float64(messageCount) / float64(concurrency) * 5))
	if durationMS < 1 {
		return 1
	}
	return durationMS
}

func firstNonEmpty(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}
