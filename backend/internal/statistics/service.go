package statistics

import (
	"context"
	"time"
)

type QueryParams struct {
	Now    time.Time
	Window time.Duration
}

type Summary struct {
	TotalSent         int     `json:"total_sent"`
	Successful        int     `json:"successful"`
	Failed            int     `json:"failed"`
	SuccessRate       float64 `json:"success_rate"`
	AverageDurationMS int     `json:"average_duration_ms"`
	AverageQPS        float64 `json:"average_qps"`
	TotalReceived     int     `json:"total_received"`
}

type TrendPoint struct {
	BucketStart time.Time `json:"bucket_start"`
	Sent        int       `json:"sent"`
	Successful  int       `json:"successful"`
	Failed      int       `json:"failed"`
	QPS         float64   `json:"qps"`
}

type PlatformRanking struct {
	ChannelID     string  `json:"channel_id"`
	Name          string  `json:"name"`
	ProviderType  string  `json:"provider_type"`
	Sent          int     `json:"sent"`
	SuccessRate   float64 `json:"success_rate"`
	QPS           float64 `json:"qps"`
	Failures      int     `json:"failures"`
	RateLimited   int     `json:"rate_limited"`
	AvgDurationMS int     `json:"avg_duration_ms"`
	P99DurationMS int     `json:"p99_duration_ms"`
	LastError     string  `json:"last_error"`
}

type FailureRanking struct {
	Reason string  `json:"reason"`
	Count  int     `json:"count"`
	Ratio  float64 `json:"ratio"`
}

type RecentAnomaly struct {
	Level string    `json:"level"`
	Title string    `json:"title"`
	Time  time.Time `json:"time"`
	Count int       `json:"count"`
	Ratio float64   `json:"ratio"`
}

type Overview struct {
	WindowStart      time.Time         `json:"window_start"`
	WindowEnd        time.Time         `json:"window_end"`
	Summary          Summary           `json:"summary"`
	Trend            []TrendPoint      `json:"trend"`
	PlatformRankings []PlatformRanking `json:"platform_rankings"`
	FailureRankings  []FailureRanking  `json:"failure_rankings"`
	RecentAnomalies  []RecentAnomaly   `json:"recent_anomalies"`
}

type store interface {
	GetOverviewStatistics(context.Context, QueryParams) (Overview, error)
}

type Service struct {
	store store
	now   func() time.Time
}

type Option func(*Service)

func WithNow(now func() time.Time) Option {
	return func(s *Service) {
		if now != nil {
			s.now = now
		}
	}
}

func NewService(store store, options ...Option) *Service {
	service := &Service{
		store: store,
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
	for _, option := range options {
		option(service)
	}
	return service
}

func (s *Service) GetOverview(ctx context.Context, params QueryParams) (Overview, error) {
	if params.Now.IsZero() {
		params.Now = s.now()
	}
	return s.store.GetOverviewStatistics(ctx, params)
}
