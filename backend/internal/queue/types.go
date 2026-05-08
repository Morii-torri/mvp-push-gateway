package queue

import (
	"encoding/json"
	"errors"
	"time"
)

type JobType string

const (
	JobTypeRoutePlan        JobType = "route_plan"
	JobTypeSendMessage      JobType = "send_message"
	JobTypeStatsAggregate   JobType = "stats_aggregate"
	JobTypeRetentionCleanup JobType = "retention_cleanup"
	JobTypeDeadLetterReplay JobType = "dead_letter_replay"
)

type JobStatus string

const (
	JobStatusQueued     JobStatus = "queued"
	JobStatusProcessing JobStatus = "processing"
	JobStatusDone       JobStatus = "done"
	JobStatusFailed     JobStatus = "failed"
	JobStatusDead       JobStatus = "dead"
)

var (
	ErrInvalidInput = errors.New("invalid queue input")
	ErrNotFound     = errors.New("job not found")
)

type Job struct {
	ID                       string
	Type                     JobType
	Status                   JobStatus
	Payload                  json.RawMessage
	RunAt                    time.Time
	Attempts                 int
	MaxAttempts              int
	LockedBy                 string
	LockedAt                 *time.Time
	HeartbeatAt              *time.Time
	ProcessingTimeoutSeconds *int
	LastError                string
	ChannelID                string
	Priority                 int
	QueueKey                 string
	StartedAt                *time.Time
	FinishedAt               *time.Time
	DurationMS               *int
	CreatedAt                time.Time
	UpdatedAt                time.Time
}

type EnqueueParams struct {
	ID                       string
	Type                     JobType
	Payload                  json.RawMessage
	RunAt                    time.Time
	MaxAttempts              int
	ChannelID                string
	Priority                 int
	QueueKey                 string
	ProcessingTimeoutSeconds *int
}

type ClaimParams struct {
	WorkerID string
	Types    []JobType
	Limit    int
	Now      time.Time
}

type HeartbeatParams struct {
	JobID    string
	WorkerID string
	Now      time.Time
}

type CompleteParams struct {
	JobID      string
	WorkerID   string
	Now        time.Time
	DurationMS int
}

type FailParams struct {
	JobID        string
	WorkerID     string
	ErrorCode    string
	ErrorMessage string
	RetryAt      time.Time
	Now          time.Time
}

type FailResult struct {
	JobID        string
	Status       JobStatus
	Retry        bool
	DeadLettered bool
}

type RecoverParams struct {
	WorkerID              string
	DefaultTimeoutSeconds int
	RetryAt               time.Time
	Now                   time.Time
	Limit                 int
}

type RecoverResult struct {
	Scanned      int
	Requeued     int
	DeadLettered int
}

func AllJobTypes() []JobType {
	return []JobType{
		JobTypeRoutePlan,
		JobTypeSendMessage,
		JobTypeStatsAggregate,
		JobTypeRetentionCleanup,
		JobTypeDeadLetterReplay,
	}
}
