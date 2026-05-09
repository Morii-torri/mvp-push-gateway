package monitoring

import (
	"context"
	"errors"
	"strings"
	"time"
)

const (
	DefaultRetentionDays = 30
	DefaultBatchSize     = 200
)

var ErrInvalidInput = errors.New("invalid monitoring input")

type QueryParams struct {
	Now time.Time
}

type QueueSummary struct {
	RoutePlanPending      int     `json:"route_plan_pending"`
	SendMessagePending    int     `json:"send_message_pending"`
	OldestJobWaitSeconds  int64   `json:"oldest_job_wait_seconds"`
	PlanningAvgDurationMS int     `json:"planning_avg_duration_ms"`
	PlanningP95DurationMS int     `json:"planning_p95_duration_ms"`
	SendingAvgDurationMS  int     `json:"sending_avg_duration_ms"`
	SendingP95DurationMS  int     `json:"sending_p95_duration_ms"`
	PlatformFailureRate   float64 `json:"platform_failure_rate"`
	RateLimitedCount      int     `json:"rate_limited_count"`
	DeadLetterCount       int     `json:"dead_letter_count"`
}

type PlatformHealth struct {
	ChannelID    string  `json:"channel_id"`
	Name         string  `json:"name"`
	ProviderType string  `json:"provider_type"`
	Health       string  `json:"health"`
	Pending      int     `json:"pending"`
	FailureRate  float64 `json:"failure_rate"`
	RateLimited  int     `json:"rate_limited"`
	Retries      int     `json:"retries"`
	DeadLetters  int     `json:"dead_letters"`
	LastError    string  `json:"last_error"`
}

type SlowRule struct {
	RuleID        string `json:"rule_id"`
	Source        string `json:"source"`
	RouteGroup    string `json:"route_group"`
	Rule          string `json:"rule"`
	HitCount      int    `json:"hit_count"`
	AvgDurationMS int    `json:"avg_duration_ms"`
	P95DurationMS int    `json:"p95_duration_ms"`
}

type CleanupStatus struct {
	LastRunAt               *time.Time `json:"last_run_at"`
	RetentionDays           int        `json:"retention_days"`
	BatchSize               int        `json:"batch_size"`
	LastBatchDeleted        int        `json:"last_batch_deleted"`
	TotalDeleted            int        `json:"total_deleted"`
	DeletedJobs             int        `json:"deleted_jobs"`
	DeletedDeadLetters      int        `json:"deleted_dead_letters"`
	DeletedMessageRecords   int        `json:"deleted_message_records"`
	DeletedDeliveryAttempts int        `json:"deleted_delivery_attempts"`
	DeletedDedupeKeys       int        `json:"deleted_dedupe_keys"`
	DeletedWorkerMetrics    int        `json:"deleted_worker_metrics"`
	DeletedRouteRuleMetrics int        `json:"deleted_route_rule_metrics"`
	Completed               bool       `json:"completed"`
	HasMore                 bool       `json:"has_more"`
}

type QueueSnapshot struct {
	Summary        QueueSummary     `json:"summary"`
	PlatformHealth []PlatformHealth `json:"platform_health"`
	SlowRules      []SlowRule       `json:"slow_rules"`
	CleanupStatus  CleanupStatus    `json:"cleanup_status"`
}

type RetentionCleanupParams struct {
	Now           time.Time `json:"-"`
	RetentionDays int       `json:"retention_days"`
	BatchSize     int       `json:"batch_size"`
}

type readerStore interface {
	GetQueueMonitoringSnapshot(context.Context, QueryParams) (QueueSnapshot, error)
}

type cleanupStore interface {
	RunRetentionCleanup(context.Context, RetentionCleanupParams) (CleanupStatus, error)
}

type Service struct {
	reader  readerStore
	cleaner cleanupStore
	now     func() time.Time
}

type Option func(*Service)

func WithNow(now func() time.Time) Option {
	return func(s *Service) {
		if now != nil {
			s.now = now
		}
	}
}

func NewService(reader readerStore, cleaner cleanupStore, options ...Option) *Service {
	service := &Service{
		reader:  reader,
		cleaner: cleaner,
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
	for _, option := range options {
		option(service)
	}
	return service
}

func (s *Service) GetQueueMonitoringSnapshot(ctx context.Context) (QueueSnapshot, error) {
	if s == nil || s.reader == nil {
		return QueueSnapshot{}, ErrInvalidInput
	}
	return s.reader.GetQueueMonitoringSnapshot(ctx, QueryParams{Now: s.now()})
}

func (s *Service) RunRetentionCleanup(ctx context.Context, params RetentionCleanupParams) (CleanupStatus, error) {
	if s == nil || s.cleaner == nil {
		return CleanupStatus{}, ErrInvalidInput
	}
	params = normalizeCleanupParams(params, s.now)
	if params.RetentionDays <= 0 || params.BatchSize <= 0 {
		return CleanupStatus{}, ErrInvalidInput
	}
	return s.cleaner.RunRetentionCleanup(ctx, params)
}

func normalizeCleanupParams(params RetentionCleanupParams, now func() time.Time) RetentionCleanupParams {
	if params.Now.IsZero() && now != nil {
		params.Now = now()
	}
	if params.RetentionDays <= 0 {
		params.RetentionDays = DefaultRetentionDays
	}
	if params.BatchSize <= 0 {
		params.BatchSize = DefaultBatchSize
	}
	return params
}

func NormalizeHealth(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "healthy":
		return "healthy"
	case "critical":
		return "critical"
	default:
		return "warning"
	}
}
