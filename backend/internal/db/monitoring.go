package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"mvp-push-gateway/backend/internal/monitoring"
	"mvp-push-gateway/backend/internal/statistics"
)

func (r Repository) GetQueueMonitoringSnapshot(ctx context.Context, params monitoring.QueryParams) (monitoring.QueueSnapshot, error) {
	if r.pool == nil {
		return monitoring.QueueSnapshot{}, errors.New("postgres pool is nil")
	}
	now := params.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	window := normalizedQueryWindow(params.Window)
	windowStart := now.Add(-window)
	bucketInterval := bucketIntervalLiteral(window)

	summary, err := r.getQueueSummary(ctx, now, windowStart)
	if err != nil {
		return monitoring.QueueSnapshot{}, err
	}
	platformHealth, err := r.getPlatformHealth(ctx, windowStart)
	if err != nil {
		return monitoring.QueueSnapshot{}, err
	}
	trend, err := r.getQueueTrend(ctx, windowStart, now, bucketInterval)
	if err != nil {
		return monitoring.QueueSnapshot{}, err
	}
	slowRules, err := r.getSlowRules(ctx, windowStart)
	if err != nil {
		return monitoring.QueueSnapshot{}, err
	}
	cleanupStatus, err := r.getCleanupStatus(ctx)
	if err != nil {
		return monitoring.QueueSnapshot{}, err
	}

	return monitoring.QueueSnapshot{
		Summary:        summary,
		PlatformHealth: platformHealth,
		Trend:          trend,
		SlowRules:      slowRules,
		CleanupStatus:  cleanupStatus,
	}, nil
}

func (r Repository) GetOverviewStatistics(ctx context.Context, params statistics.QueryParams) (statistics.Overview, error) {
	if r.pool == nil {
		return statistics.Overview{}, errors.New("postgres pool is nil")
	}
	now := params.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	window := normalizedQueryWindow(params.Window)
	windowStart := now.Add(-window)
	bucketInterval := bucketIntervalLiteral(window)

	overview := statistics.Overview{
		WindowStart: windowStart,
		WindowEnd:   now,
	}

	summary, err := r.getOverviewSummary(ctx, windowStart, now, int(window.Seconds()))
	if err != nil {
		return statistics.Overview{}, err
	}
	overview.Summary = summary

	if overview.Trend, err = r.getOverviewTrend(ctx, windowStart, now, bucketInterval); err != nil {
		return statistics.Overview{}, err
	}
	if overview.PlatformRankings, err = r.getPlatformRankings(ctx, windowStart, now, int(window.Seconds())); err != nil {
		return statistics.Overview{}, err
	}
	if overview.FailureRankings, err = r.getFailureRankings(ctx, windowStart, now); err != nil {
		return statistics.Overview{}, err
	}
	if overview.RecentAnomalies, err = r.getRecentAnomalies(ctx, windowStart, now); err != nil {
		return statistics.Overview{}, err
	}

	return overview, nil
}

func (r Repository) RunRetentionCleanup(ctx context.Context, params monitoring.RetentionCleanupParams) (monitoring.CleanupStatus, error) {
	if r.pool == nil {
		return monitoring.CleanupStatus{}, errors.New("postgres pool is nil")
	}
	if params.RetentionDays <= 0 || params.BatchSize <= 0 {
		return monitoring.CleanupStatus{}, monitoring.ErrInvalidInput
	}

	now := params.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	cutoff := now.Add(-time.Duration(params.RetentionDays) * 24 * time.Hour)

	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return monitoring.CleanupStatus{}, fmt.Errorf("begin retention cleanup transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	status := monitoring.CleanupStatus{
		LastRunAt:     &now,
		RetentionDays: params.RetentionDays,
		BatchSize:     params.BatchSize,
		Completed:     false,
		HasMore:       false,
	}

	if status.DeletedJobs, err = deleteOldJobs(ctx, tx, cutoff, params.BatchSize); err != nil {
		return monitoring.CleanupStatus{}, err
	}
	if status.DeletedDeadLetters, err = deleteOldDeadLetters(ctx, tx, cutoff, params.BatchSize); err != nil {
		return monitoring.CleanupStatus{}, err
	}
	if status.DeletedWorkerMetrics, err = deleteOldWorkerMetrics(ctx, tx, cutoff, params.BatchSize); err != nil {
		return monitoring.CleanupStatus{}, err
	}
	if status.DeletedRouteRuleMetrics, err = deleteOldRouteRuleMetrics(ctx, tx, cutoff, params.BatchSize); err != nil {
		return monitoring.CleanupStatus{}, err
	}
	if status.DeletedDedupeKeys, err = deleteExpiredDedupeKeys(ctx, tx, now, params.BatchSize); err != nil {
		return monitoring.CleanupStatus{}, err
	}
	if status.DeletedMessageRecords, status.DeletedDeliveryAttempts, err = deleteOldMessages(ctx, tx, cutoff, params.BatchSize); err != nil {
		return monitoring.CleanupStatus{}, err
	}
	if status.DeletedAuditLogs, err = deleteOldAuditLogs(ctx, tx, cutoff, params.BatchSize); err != nil {
		return monitoring.CleanupStatus{}, err
	}

	status.LastBatchDeleted = status.DeletedJobs +
		status.DeletedDeadLetters +
		status.DeletedWorkerMetrics +
		status.DeletedRouteRuleMetrics +
		status.DeletedDedupeKeys +
		status.DeletedMessageRecords +
		status.DeletedDeliveryAttempts +
		status.DeletedAuditLogs

	if status.HasMore, err = hasRetentionRowsRemaining(ctx, tx, cutoff, now); err != nil {
		return monitoring.CleanupStatus{}, err
	}
	status.Completed = !status.HasMore

	if err := insertCleanupRun(ctx, tx, now, status); err != nil {
		return monitoring.CleanupStatus{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return monitoring.CleanupStatus{}, fmt.Errorf("commit retention cleanup transaction: %w", err)
	}

	latest, err := r.getCleanupStatus(ctx)
	if err != nil {
		return monitoring.CleanupStatus{}, err
	}
	return latest, nil
}

func (r Repository) getQueueSummary(ctx context.Context, now time.Time, windowStart time.Time) (monitoring.QueueSummary, error) {
	var summary monitoring.QueueSummary
	err := r.pool.QueryRow(ctx, `
		WITH queued_jobs AS (
			SELECT
				count(*) FILTER (WHERE type = 'route_plan' AND status = 'queued')::integer AS route_plan_pending,
				count(*) FILTER (WHERE type = 'send_message' AND status = 'queued')::integer AS send_message_pending,
				COALESCE(max(EXTRACT(EPOCH FROM ($1 - run_at))) FILTER (WHERE status = 'queued' AND run_at <= $1), 0)::bigint AS oldest_job_wait_seconds
			FROM jobs
		),
		planning_metrics AS (
			SELECT
				COALESCE(
					round(sum(COALESCE(avg_duration_ms, 0) * GREATEST(processed, 1))::numeric / NULLIF(sum(GREATEST(processed, 1)), 0)),
					0
				)::integer AS avg_duration_ms,
				COALESCE(max(p95_duration_ms), 0)::integer AS p99_duration_ms
			FROM worker_metrics
			WHERE worker_type = 'planning'
				AND job_type = 'route_plan'
				AND bucket_start >= $2
		),
		sending_metrics AS (
			SELECT
				COALESCE(
					round(sum(COALESCE(avg_duration_ms, 0) * GREATEST(processed, 1))::numeric / NULLIF(sum(GREATEST(processed, 1)), 0)),
					0
				)::integer AS avg_duration_ms,
				COALESCE(max(p95_duration_ms), 0)::integer AS p99_duration_ms,
				COALESCE(sum(failed), 0)::integer AS failed_count,
				COALESCE(sum(success), 0)::integer AS success_count,
				COALESCE(sum(rate_limited), 0)::integer AS rate_limited_count
			FROM worker_metrics
			WHERE worker_type = 'sending'
				AND job_type = 'send_message'
				AND bucket_start >= $2
		),
		dead_letters AS (
			SELECT count(*)::integer AS dead_letter_count
			FROM dead_letter_jobs
			WHERE dead_lettered_at >= $2
		)
		SELECT
			queued_jobs.route_plan_pending,
			queued_jobs.send_message_pending,
			queued_jobs.oldest_job_wait_seconds,
			planning_metrics.avg_duration_ms,
			planning_metrics.p99_duration_ms,
			sending_metrics.avg_duration_ms,
			sending_metrics.p99_duration_ms,
			COALESCE(
				round(
					(sending_metrics.failed_count::numeric * 100.0)
					/ NULLIF((sending_metrics.failed_count + sending_metrics.success_count)::numeric, 0),
					2
				),
				0
			)::float8 AS platform_failure_rate,
			sending_metrics.rate_limited_count,
			dead_letters.dead_letter_count
		FROM queued_jobs, planning_metrics, sending_metrics, dead_letters
	`, now, windowStart).Scan(
		&summary.RoutePlanPending,
		&summary.SendMessagePending,
		&summary.OldestJobWaitSeconds,
		&summary.PlanningAvgDurationMS,
		&summary.PlanningP99DurationMS,
		&summary.SendingAvgDurationMS,
		&summary.SendingP99DurationMS,
		&summary.PlatformFailureRate,
		&summary.RateLimitedCount,
		&summary.DeadLetterCount,
	)
	if err != nil {
		return monitoring.QueueSummary{}, fmt.Errorf("query queue summary: %w", err)
	}
	return summary, nil
}

func (r Repository) getPlatformHealth(ctx context.Context, windowStart time.Time) ([]monitoring.PlatformHealth, error) {
	rows, err := r.pool.Query(ctx, `
		WITH pending AS (
			SELECT channel_id, count(*)::integer AS pending
			FROM jobs
			WHERE type = 'send_message'
				AND status = 'queued'
				AND channel_id IS NOT NULL
			GROUP BY channel_id
		),
		sending_metrics AS (
			SELECT
				channel_id,
				COALESCE(sum(success), 0)::integer AS success_count,
				COALESCE(sum(failed), 0)::integer AS failed_count,
				COALESCE(sum(rate_limited), 0)::integer AS rate_limited_count
			FROM worker_metrics
			WHERE worker_type = 'sending'
				AND job_type = 'send_message'
				AND bucket_start >= $1
				AND channel_id IS NOT NULL
			GROUP BY channel_id
		),
		retries AS (
			SELECT channel_id, count(*)::integer AS retry_count
			FROM delivery_attempts
			WHERE channel_id IS NOT NULL
				AND attempt_no > 1
				AND COALESCE(finished_at, queued_at) >= $1
			GROUP BY channel_id
		),
		dead_letters AS (
			SELECT channel_id, count(*)::integer AS dead_letter_count
			FROM dead_letter_jobs
			WHERE channel_id IS NOT NULL
				AND dead_lettered_at >= $1
			GROUP BY channel_id
		),
		last_errors AS (
			SELECT DISTINCT ON (channel_id)
				channel_id,
				COALESCE(NULLIF(error_message, ''), '-') AS last_error,
				error_at
			FROM (
				SELECT channel_id, error_message, COALESCE(finished_at, queued_at) AS error_at
				FROM delivery_attempts
				WHERE channel_id IS NOT NULL
					AND status = 'failed'
					AND COALESCE(finished_at, queued_at) >= $1
				UNION ALL
				SELECT channel_id, error_message, dead_lettered_at AS error_at
				FROM dead_letter_jobs
				WHERE channel_id IS NOT NULL
					AND dead_lettered_at >= $1
			) AS errors
			ORDER BY channel_id, error_at DESC
		)
		SELECT
			channel.id,
			channel.name,
			channel.provider_type,
			COALESCE(pending.pending, 0)::integer AS pending_count,
			COALESCE(
				round((sending_metrics.failed_count::numeric * 100.0) / NULLIF((sending_metrics.failed_count + sending_metrics.success_count)::numeric, 0), 2),
				0
			)::float8 AS failure_rate,
			COALESCE(sending_metrics.rate_limited_count, 0)::integer AS rate_limited_count,
			COALESCE(retries.retry_count, 0)::integer AS retry_count,
			COALESCE(dead_letters.dead_letter_count, 0)::integer AS dead_letter_count,
			COALESCE(last_errors.last_error, '-') AS last_error
		FROM delivery_channels AS channel
		LEFT JOIN pending ON pending.channel_id = channel.id
		LEFT JOIN sending_metrics ON sending_metrics.channel_id = channel.id
		LEFT JOIN retries ON retries.channel_id = channel.id
		LEFT JOIN dead_letters ON dead_letters.channel_id = channel.id
		LEFT JOIN last_errors ON last_errors.channel_id = channel.id
		WHERE COALESCE(pending.pending, 0) > 0
			OR COALESCE(sending_metrics.success_count, 0) > 0
			OR COALESCE(sending_metrics.failed_count, 0) > 0
			OR COALESCE(dead_letters.dead_letter_count, 0) > 0
		ORDER BY pending_count DESC, channel.name ASC
	`, windowStart)
	if err != nil {
		return nil, fmt.Errorf("query platform health: %w", err)
	}
	defer rows.Close()

	platforms := make([]monitoring.PlatformHealth, 0)
	for rows.Next() {
		var item monitoring.PlatformHealth
		if err := rows.Scan(
			&item.ChannelID,
			&item.Name,
			&item.ProviderType,
			&item.Pending,
			&item.FailureRate,
			&item.RateLimited,
			&item.Retries,
			&item.DeadLetters,
			&item.LastError,
		); err != nil {
			return nil, fmt.Errorf("scan platform health: %w", err)
		}
		item.Health = monitoring.NormalizeHealth(deriveHealth(item.FailureRate, item.DeadLetters, item.RateLimited))
		platforms = append(platforms, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate platform health: %w", err)
	}
	return platforms, nil
}

func (r Repository) getQueueTrend(ctx context.Context, windowStart, now time.Time, bucketInterval string) ([]monitoring.QueueTrendPoint, error) {
	rows, err := r.pool.Query(ctx, `
		WITH buckets AS (
			SELECT generate_series(
				date_bin($3::interval, $1, $4::timestamptz),
				date_bin($3::interval, $2, $4::timestamptz),
				$3::interval
			) AS bucket_start
		),
		metrics AS (
			SELECT
				date_bin($3::interval, bucket_start, $4::timestamptz) AS bucket_start,
				COALESCE(sum(processed) FILTER (WHERE job_type = 'route_plan'), 0)::integer AS route_plan_processed,
				COALESCE(sum(processed) FILTER (WHERE job_type = 'send_message'), 0)::integer AS send_message_processed,
				COALESCE(sum(dead_lettered), 0)::integer AS dead_letters,
				COALESCE(max(p95_duration_ms), 0)::integer AS p99_duration_ms
			FROM worker_metrics
			WHERE bucket_start >= $1
				AND bucket_start <= $2
			GROUP BY date_bin($3::interval, bucket_start, $4::timestamptz)
		)
		SELECT
			buckets.bucket_start,
			COALESCE(metrics.route_plan_processed, 0)::integer,
			COALESCE(metrics.send_message_processed, 0)::integer,
			COALESCE(metrics.dead_letters, 0)::integer,
			COALESCE(metrics.p99_duration_ms, 0)::integer
		FROM buckets
		LEFT JOIN metrics ON metrics.bucket_start = buckets.bucket_start
		ORDER BY buckets.bucket_start ASC
	`, windowStart, now, bucketInterval, time.Unix(0, 0).UTC())
	if err != nil {
		return nil, fmt.Errorf("query queue trend: %w", err)
	}
	defer rows.Close()

	trend := make([]monitoring.QueueTrendPoint, 0)
	for rows.Next() {
		var item monitoring.QueueTrendPoint
		if err := rows.Scan(
			&item.BucketStart,
			&item.RoutePlanProcessed,
			&item.SendMessageProcessed,
			&item.DeadLetters,
			&item.P99DurationMS,
		); err != nil {
			return nil, fmt.Errorf("scan queue trend: %w", err)
		}
		trend = append(trend, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate queue trend: %w", err)
	}
	return trend, nil
}

func (r Repository) getSlowRules(ctx context.Context, windowStart time.Time) ([]monitoring.SlowRule, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT
			rule.id,
			source.name,
			flow.name,
			rule.name,
			COALESCE(sum(metrics.matched), 0)::integer AS hit_count,
			COALESCE(
				round(sum(COALESCE(metrics.avg_duration_ms, 0) * GREATEST(metrics.evaluated, 1))::numeric / NULLIF(sum(GREATEST(metrics.evaluated, 1)), 0)),
				0
			)::integer AS avg_duration_ms,
			COALESCE(max(metrics.p95_duration_ms), 0)::integer AS p99_duration_ms
		FROM route_rule_metrics AS metrics
		JOIN route_rules AS rule ON rule.id = metrics.rule_id
		JOIN route_flows AS flow ON flow.id = metrics.flow_id
		JOIN inbound_sources AS source ON source.id = metrics.source_id
		WHERE metrics.bucket_start >= $1
		GROUP BY rule.id, source.name, flow.name, rule.name
		ORDER BY p99_duration_ms DESC, hit_count DESC, rule.name ASC
		LIMIT 10
	`, windowStart)
	if err != nil {
		return nil, fmt.Errorf("query slow rules: %w", err)
	}
	defer rows.Close()

	items := make([]monitoring.SlowRule, 0)
	for rows.Next() {
		var item monitoring.SlowRule
		if err := rows.Scan(
			&item.RuleID,
			&item.Source,
			&item.RouteGroup,
			&item.Rule,
			&item.HitCount,
			&item.AvgDurationMS,
			&item.P99DurationMS,
		); err != nil {
			return nil, fmt.Errorf("scan slow rule: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate slow rules: %w", err)
	}
	return items, nil
}

func (r Repository) getCleanupStatus(ctx context.Context) (monitoring.CleanupStatus, error) {
	var status monitoring.CleanupStatus
	var lastRunAt *time.Time
	err := r.pool.QueryRow(ctx, `
		WITH totals AS (
			SELECT
				COALESCE(sum(last_batch_deleted), 0)::integer AS total_deleted
			FROM retention_cleanup_runs
		),
		latest AS (
			SELECT
				started_at,
				retention_days,
				batch_size,
				deleted_jobs,
				deleted_dead_letters,
				deleted_message_records,
				deleted_delivery_attempts,
				deleted_dedupe_keys,
				deleted_worker_metrics,
				deleted_route_rule_metrics,
				deleted_audit_logs,
				last_batch_deleted,
				completed,
				has_more
			FROM retention_cleanup_runs
			ORDER BY started_at DESC
			LIMIT 1
		)
		SELECT
			latest.started_at,
			COALESCE(latest.retention_days, 0)::integer,
			COALESCE(latest.batch_size, 0)::integer,
			COALESCE(latest.deleted_jobs, 0)::integer,
			COALESCE(latest.deleted_dead_letters, 0)::integer,
			COALESCE(latest.deleted_message_records, 0)::integer,
			COALESCE(latest.deleted_delivery_attempts, 0)::integer,
			COALESCE(latest.deleted_dedupe_keys, 0)::integer,
			COALESCE(latest.deleted_worker_metrics, 0)::integer,
			COALESCE(latest.deleted_route_rule_metrics, 0)::integer,
			COALESCE(latest.deleted_audit_logs, 0)::integer,
			COALESCE(latest.last_batch_deleted, 0)::integer,
			totals.total_deleted,
			COALESCE(latest.completed, false),
			COALESCE(latest.has_more, false)
		FROM totals
		LEFT JOIN latest ON true
	`).Scan(
		&lastRunAt,
		&status.RetentionDays,
		&status.BatchSize,
		&status.DeletedJobs,
		&status.DeletedDeadLetters,
		&status.DeletedMessageRecords,
		&status.DeletedDeliveryAttempts,
		&status.DeletedDedupeKeys,
		&status.DeletedWorkerMetrics,
		&status.DeletedRouteRuleMetrics,
		&status.DeletedAuditLogs,
		&status.LastBatchDeleted,
		&status.TotalDeleted,
		&status.Completed,
		&status.HasMore,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return monitoring.CleanupStatus{}, nil
		}
		return monitoring.CleanupStatus{}, fmt.Errorf("query cleanup status: %w", err)
	}
	status.LastRunAt = lastRunAt
	return status, nil
}

func (r Repository) getOverviewSummary(ctx context.Context, windowStart, now time.Time, windowSeconds int) (statistics.Summary, error) {
	if windowSeconds <= 0 {
		windowSeconds = int((24 * time.Hour).Seconds())
	}
	var summary statistics.Summary
	err := r.pool.QueryRow(ctx, `
		WITH delivery_summary AS (
			SELECT
				count(*) FILTER (WHERE status IN ('sent', 'failed'))::integer AS total_sent,
				count(*) FILTER (WHERE status = 'sent')::integer AS successful,
				count(*) FILTER (WHERE status = 'failed')::integer AS failed,
				COALESCE(
					round(avg(duration_ms) FILTER (WHERE status IN ('sent', 'failed') AND duration_ms IS NOT NULL))::integer,
					0
				)::integer AS average_duration_ms
			FROM delivery_attempts
			WHERE COALESCE(finished_at, queued_at) >= $1
				AND COALESCE(finished_at, queued_at) <= $2
		),
		received_summary AS (
			SELECT count(*)::integer AS total_received
			FROM message_records
			WHERE received_at >= $1
				AND received_at <= $2
		)
		SELECT
			delivery_summary.total_sent,
			delivery_summary.successful,
			delivery_summary.failed,
			COALESCE(round((delivery_summary.successful::numeric * 100.0) / NULLIF(delivery_summary.total_sent::numeric, 0), 2), 0)::float8 AS success_rate,
			delivery_summary.average_duration_ms,
			COALESCE(round(delivery_summary.total_sent::numeric / $3::numeric, 2), 0)::float8 AS average_qps,
			received_summary.total_received
		FROM delivery_summary, received_summary
	`, windowStart, now, windowSeconds).Scan(
		&summary.TotalSent,
		&summary.Successful,
		&summary.Failed,
		&summary.SuccessRate,
		&summary.AverageDurationMS,
		&summary.AverageQPS,
		&summary.TotalReceived,
	)
	if err != nil {
		return statistics.Summary{}, fmt.Errorf("query overview summary: %w", err)
	}
	return summary, nil
}

func (r Repository) getOverviewTrend(ctx context.Context, windowStart, now time.Time, bucketInterval string) ([]statistics.TrendPoint, error) {
	bucketSeconds := bucketIntervalSeconds(bucketInterval)
	rows, err := r.pool.Query(ctx, `
		WITH buckets AS (
			SELECT generate_series(
				date_bin($3::interval, $1, $4::timestamptz),
				date_bin($3::interval, $2, $4::timestamptz),
				$3::interval
			) AS bucket_start
		),
		attempts AS (
			SELECT
				date_bin($3::interval, COALESCE(finished_at, queued_at), $4::timestamptz) AS bucket_start,
				count(*) FILTER (WHERE status IN ('sent', 'failed'))::integer AS sent,
				count(*) FILTER (WHERE status = 'sent')::integer AS successful,
				count(*) FILTER (WHERE status = 'failed')::integer AS failed,
				COALESCE(
					round(sum(COALESCE(duration_ms, 0))::numeric / NULLIF(count(*) FILTER (WHERE duration_ms IS NOT NULL), 0), 0),
					0
				)::integer AS avg_duration_ms
			FROM delivery_attempts
			WHERE COALESCE(finished_at, queued_at) >= $1
				AND COALESCE(finished_at, queued_at) <= $2
			GROUP BY date_bin($3::interval, COALESCE(finished_at, queued_at), $4::timestamptz)
		)
		SELECT
			buckets.bucket_start,
			COALESCE(attempts.sent, 0)::integer,
			COALESCE(attempts.successful, 0)::integer,
			COALESCE(attempts.failed, 0)::integer,
			COALESCE(round(COALESCE(attempts.sent, 0)::numeric / $5::numeric, 2), 0)::float8 AS qps,
			COALESCE(attempts.avg_duration_ms, 0)::integer
		FROM buckets
		LEFT JOIN attempts ON attempts.bucket_start = buckets.bucket_start
		ORDER BY buckets.bucket_start ASC
	`, windowStart, now, bucketInterval, time.Unix(0, 0).UTC(), bucketSeconds)
	if err != nil {
		return nil, fmt.Errorf("query overview trend: %w", err)
	}
	defer rows.Close()

	trend := make([]statistics.TrendPoint, 0)
	for rows.Next() {
		var item statistics.TrendPoint
		if err := rows.Scan(&item.BucketStart, &item.Sent, &item.Successful, &item.Failed, &item.QPS, &item.AverageDurationMS); err != nil {
			return nil, fmt.Errorf("scan overview trend: %w", err)
		}
		trend = append(trend, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate overview trend: %w", err)
	}
	return trend, nil
}

func (r Repository) getPlatformRankings(ctx context.Context, windowStart, now time.Time, windowSeconds int) ([]statistics.PlatformRanking, error) {
	if windowSeconds <= 0 {
		windowSeconds = int((24 * time.Hour).Seconds())
	}
	rows, err := r.pool.Query(ctx, `
		WITH attempt_stats AS (
			SELECT
				channel_id,
				count(*) FILTER (WHERE status IN ('sent', 'failed'))::integer AS sent,
				count(*) FILTER (WHERE status = 'sent')::integer AS successful,
				count(*) FILTER (WHERE status = 'failed')::integer AS failed,
				COALESCE(
					round(sum(COALESCE(duration_ms, 0))::numeric / NULLIF(count(*) FILTER (WHERE duration_ms IS NOT NULL), 0), 0),
					0
				)::integer AS avg_duration_ms,
				COALESCE(
					round((percentile_cont(0.99) WITHIN GROUP (ORDER BY duration_ms) FILTER (WHERE duration_ms IS NOT NULL))::numeric),
					0
				)::integer AS p99_duration_ms
			FROM delivery_attempts
			WHERE channel_id IS NOT NULL
				AND status IN ('sent', 'failed')
				AND COALESCE(finished_at, queued_at) >= $1
				AND COALESCE(finished_at, queued_at) <= $2
			GROUP BY channel_id
		),
		rate_limits AS (
			SELECT channel_id, COALESCE(sum(rate_limited), 0)::integer AS rate_limited
			FROM worker_metrics
			WHERE worker_type = 'sending'
				AND job_type = 'send_message'
				AND bucket_start >= $1
				AND bucket_start <= $2
				AND channel_id IS NOT NULL
			GROUP BY channel_id
		),
		last_errors AS (
			SELECT DISTINCT ON (channel_id)
				channel_id,
				COALESCE(NULLIF(error_message, ''), '-') AS last_error
			FROM delivery_attempts
			WHERE channel_id IS NOT NULL
				AND status = 'failed'
				AND COALESCE(finished_at, queued_at) >= $1
				AND COALESCE(finished_at, queued_at) <= $2
			ORDER BY channel_id, COALESCE(finished_at, queued_at) DESC
		)
		SELECT
			channel.id,
			channel.name,
			channel.provider_type,
			attempt_stats.sent,
			COALESCE(round((attempt_stats.successful::numeric * 100.0) / NULLIF(attempt_stats.sent::numeric, 0), 2), 0)::float8 AS success_rate,
			COALESCE(round(attempt_stats.sent::numeric / $3::numeric, 2), 0)::float8 AS qps,
			attempt_stats.failed,
			COALESCE(rate_limits.rate_limited, 0)::integer,
			attempt_stats.avg_duration_ms,
			attempt_stats.p99_duration_ms,
			COALESCE(last_errors.last_error, '-') AS last_error
		FROM attempt_stats
		JOIN delivery_channels AS channel ON channel.id = attempt_stats.channel_id
		LEFT JOIN rate_limits ON rate_limits.channel_id = channel.id
		LEFT JOIN last_errors ON last_errors.channel_id = channel.id
		ORDER BY attempt_stats.sent DESC, channel.name ASC
		LIMIT 10
	`, windowStart, now, windowSeconds)
	if err != nil {
		return nil, fmt.Errorf("query platform rankings: %w", err)
	}
	defer rows.Close()

	items := make([]statistics.PlatformRanking, 0)
	for rows.Next() {
		var item statistics.PlatformRanking
		if err := rows.Scan(
			&item.ChannelID,
			&item.Name,
			&item.ProviderType,
			&item.Sent,
			&item.SuccessRate,
			&item.QPS,
			&item.Failures,
			&item.RateLimited,
			&item.AvgDurationMS,
			&item.P99DurationMS,
			&item.LastError,
		); err != nil {
			return nil, fmt.Errorf("scan platform ranking: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate platform rankings: %w", err)
	}
	return items, nil
}

func (r Repository) getFailureRankings(ctx context.Context, windowStart, now time.Time) ([]statistics.FailureRanking, error) {
	rows, err := r.pool.Query(ctx, `
		WITH totals AS (
			SELECT count(*)::integer AS total_failed
			FROM delivery_attempts
			WHERE status = 'failed'
				AND COALESCE(finished_at, queued_at) >= $1
				AND COALESCE(finished_at, queued_at) <= $2
		)
		SELECT
			COALESCE(NULLIF(error_message, ''), '未知错误') AS reason,
			count(*)::integer AS failure_count,
			COALESCE(round((count(*)::numeric * 100.0) / NULLIF(totals.total_failed::numeric, 0), 2), 0)::float8 AS ratio
		FROM delivery_attempts, totals
		WHERE status = 'failed'
			AND COALESCE(finished_at, queued_at) >= $1
			AND COALESCE(finished_at, queued_at) <= $2
		GROUP BY reason, totals.total_failed
		ORDER BY failure_count DESC, reason ASC
		LIMIT 5
	`, windowStart, now)
	if err != nil {
		return nil, fmt.Errorf("query failure rankings: %w", err)
	}
	defer rows.Close()

	items := make([]statistics.FailureRanking, 0)
	for rows.Next() {
		var item statistics.FailureRanking
		if err := rows.Scan(&item.Reason, &item.Count, &item.Ratio); err != nil {
			return nil, fmt.Errorf("scan failure ranking: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate failure rankings: %w", err)
	}
	return items, nil
}

func (r Repository) getRecentAnomalies(ctx context.Context, windowStart, now time.Time) ([]statistics.RecentAnomaly, error) {
	rows, err := r.pool.Query(ctx, `
		WITH totals AS (
			SELECT count(*)::integer AS total_failed
			FROM delivery_attempts
			WHERE status = 'failed'
				AND COALESCE(finished_at, queued_at) >= $1
				AND COALESCE(finished_at, queued_at) <= $2
		)
		SELECT
			CASE
				WHEN count(*) >= 10 THEN '高'
				WHEN count(*) >= 5 THEN '中'
				ELSE '低'
			END AS level,
			COALESCE(NULLIF(error_message, ''), '未知错误') AS title,
			max(COALESCE(finished_at, queued_at)) AS latest_at,
			count(*)::integer AS anomaly_count,
			COALESCE(round((count(*)::numeric * 100.0) / NULLIF(totals.total_failed::numeric, 0), 2), 0)::float8 AS ratio
		FROM delivery_attempts, totals
		WHERE status = 'failed'
			AND COALESCE(finished_at, queued_at) >= $1
			AND COALESCE(finished_at, queued_at) <= $2
		GROUP BY title, totals.total_failed
		ORDER BY latest_at DESC, anomaly_count DESC
		LIMIT 5
	`, windowStart, now)
	if err != nil {
		return nil, fmt.Errorf("query recent anomalies: %w", err)
	}
	defer rows.Close()

	items := make([]statistics.RecentAnomaly, 0)
	for rows.Next() {
		var item statistics.RecentAnomaly
		if err := rows.Scan(&item.Level, &item.Title, &item.Time, &item.Count, &item.Ratio); err != nil {
			return nil, fmt.Errorf("scan recent anomaly: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate recent anomalies: %w", err)
	}
	return items, nil
}

func deleteOldJobs(ctx context.Context, tx pgx.Tx, cutoff time.Time, batchSize int) (int, error) {
	var deleted int
	err := tx.QueryRow(ctx, `
		WITH target_rows AS (
			SELECT id
			FROM jobs
			WHERE status IN ('done', 'failed', 'dead')
				AND COALESCE(finished_at, updated_at) < $1
			ORDER BY COALESCE(finished_at, updated_at) ASC, id ASC
			LIMIT $2
		),
		deleted_rows AS (
			DELETE FROM jobs
			WHERE id IN (SELECT id FROM target_rows)
			RETURNING id
		)
		SELECT count(*)::integer FROM deleted_rows
	`, cutoff, batchSize).Scan(&deleted)
	if err != nil {
		return 0, fmt.Errorf("delete old jobs: %w", err)
	}
	return deleted, nil
}

func deleteOldDeadLetters(ctx context.Context, tx pgx.Tx, cutoff time.Time, batchSize int) (int, error) {
	var deleted int
	err := tx.QueryRow(ctx, `
		WITH target_rows AS (
			SELECT id
			FROM dead_letter_jobs
			WHERE dead_lettered_at < $1
			ORDER BY dead_lettered_at ASC, id ASC
			LIMIT $2
		),
		deleted_rows AS (
			DELETE FROM dead_letter_jobs
			WHERE id IN (SELECT id FROM target_rows)
			RETURNING id
		)
		SELECT count(*)::integer FROM deleted_rows
	`, cutoff, batchSize).Scan(&deleted)
	if err != nil {
		return 0, fmt.Errorf("delete old dead letters: %w", err)
	}
	return deleted, nil
}

func deleteOldWorkerMetrics(ctx context.Context, tx pgx.Tx, cutoff time.Time, batchSize int) (int, error) {
	var deleted int
	err := tx.QueryRow(ctx, `
		WITH target_rows AS (
			SELECT id
			FROM worker_metrics
			WHERE bucket_start < $1
			ORDER BY bucket_start ASC, id ASC
			LIMIT $2
		),
		deleted_rows AS (
			DELETE FROM worker_metrics
			WHERE id IN (SELECT id FROM target_rows)
			RETURNING id
		)
		SELECT count(*)::integer FROM deleted_rows
	`, cutoff, batchSize).Scan(&deleted)
	if err != nil {
		return 0, fmt.Errorf("delete old worker metrics: %w", err)
	}
	return deleted, nil
}

func deleteOldRouteRuleMetrics(ctx context.Context, tx pgx.Tx, cutoff time.Time, batchSize int) (int, error) {
	var deleted int
	err := tx.QueryRow(ctx, `
		WITH target_rows AS (
			SELECT id
			FROM route_rule_metrics
			WHERE bucket_start < $1
			ORDER BY bucket_start ASC, id ASC
			LIMIT $2
		),
		deleted_rows AS (
			DELETE FROM route_rule_metrics
			WHERE id IN (SELECT id FROM target_rows)
			RETURNING id
		)
		SELECT count(*)::integer FROM deleted_rows
	`, cutoff, batchSize).Scan(&deleted)
	if err != nil {
		return 0, fmt.Errorf("delete old route rule metrics: %w", err)
	}
	return deleted, nil
}

func deleteOldMessages(ctx context.Context, tx pgx.Tx, cutoff time.Time, batchSize int) (int, int, error) {
	var deletedMessages int
	var deletedAttempts int
	err := tx.QueryRow(ctx, `
		WITH target_messages AS (
			SELECT id
			FROM message_records
			WHERE received_at < $1
			ORDER BY received_at ASC, id ASC
			LIMIT $2
		),
		attempt_counts AS (
			SELECT count(*)::integer AS deleted_attempts
			FROM delivery_attempts
			WHERE message_id IN (SELECT id FROM target_messages)
		),
		deleted_rows AS (
			DELETE FROM message_records
			WHERE id IN (SELECT id FROM target_messages)
			RETURNING id
		)
		SELECT
			COALESCE((SELECT count(*)::integer FROM deleted_rows), 0)::integer,
			COALESCE((SELECT deleted_attempts FROM attempt_counts), 0)::integer
	`, cutoff, batchSize).Scan(&deletedMessages, &deletedAttempts)
	if err != nil {
		return 0, 0, fmt.Errorf("delete old messages: %w", err)
	}
	return deletedMessages, deletedAttempts, nil
}

func deleteExpiredDedupeKeys(ctx context.Context, tx pgx.Tx, now time.Time, batchSize int) (int, error) {
	var deleted int
	err := tx.QueryRow(ctx, `
		WITH target_rows AS (
			SELECT id
			FROM dedupe_keys
			WHERE expires_at < $1
			ORDER BY expires_at ASC, id ASC
			LIMIT $2
		),
		deleted_rows AS (
			DELETE FROM dedupe_keys
			WHERE id IN (SELECT id FROM target_rows)
			RETURNING id
		)
		SELECT count(*)::integer FROM deleted_rows
	`, now, batchSize).Scan(&deleted)
	if err != nil {
		return 0, fmt.Errorf("delete expired dedupe keys: %w", err)
	}
	return deleted, nil
}

func deleteOldAuditLogs(ctx context.Context, tx pgx.Tx, cutoff time.Time, batchSize int) (int, error) {
	var deleted int
	err := tx.QueryRow(ctx, `
		WITH target_rows AS (
			SELECT id
			FROM audit_logs
			WHERE created_at < $1
			ORDER BY created_at ASC, id ASC
			LIMIT $2
		),
		deleted_rows AS (
			DELETE FROM audit_logs
			WHERE id IN (SELECT id FROM target_rows)
			RETURNING id
		)
		SELECT count(*)::integer FROM deleted_rows
	`, cutoff, batchSize).Scan(&deleted)
	if err != nil {
		return 0, fmt.Errorf("delete old audit logs: %w", err)
	}
	return deleted, nil
}

func hasRetentionRowsRemaining(ctx context.Context, tx pgx.Tx, cutoff, now time.Time) (bool, error) {
	var hasMore bool
	err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM jobs WHERE status IN ('done', 'failed', 'dead') AND COALESCE(finished_at, updated_at) < $1
			UNION ALL
			SELECT 1 FROM dead_letter_jobs WHERE dead_lettered_at < $1
			UNION ALL
			SELECT 1 FROM worker_metrics WHERE bucket_start < $1
			UNION ALL
			SELECT 1 FROM route_rule_metrics WHERE bucket_start < $1
			UNION ALL
			SELECT 1 FROM message_records WHERE received_at < $1
			UNION ALL
			SELECT 1 FROM dedupe_keys WHERE expires_at < $2
			UNION ALL
			SELECT 1 FROM audit_logs WHERE created_at < $1
		)
	`, cutoff, now).Scan(&hasMore)
	if err != nil {
		return false, fmt.Errorf("check remaining retention rows: %w", err)
	}
	return hasMore, nil
}

func insertCleanupRun(ctx context.Context, tx pgx.Tx, now time.Time, status monitoring.CleanupStatus) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO retention_cleanup_runs (
			id, started_at, retention_days, batch_size,
			deleted_jobs, deleted_dead_letters, deleted_message_records, deleted_delivery_attempts,
			deleted_dedupe_keys, deleted_worker_metrics, deleted_route_rule_metrics, deleted_audit_logs,
			last_batch_deleted, completed, has_more
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
	`, uuid.NewString(), now, status.RetentionDays, status.BatchSize, status.DeletedJobs, status.DeletedDeadLetters, status.DeletedMessageRecords, status.DeletedDeliveryAttempts, status.DeletedDedupeKeys, status.DeletedWorkerMetrics, status.DeletedRouteRuleMetrics, status.DeletedAuditLogs, status.LastBatchDeleted, status.Completed, status.HasMore)
	if err != nil {
		return fmt.Errorf("insert retention cleanup run: %w", err)
	}
	return nil
}

func deriveHealth(failureRate float64, deadLetters int, rateLimited int) string {
	switch {
	case failureRate >= 10 || deadLetters >= 5:
		return "critical"
	case failureRate > 0 || deadLetters > 0 || rateLimited > 0:
		return "warning"
	default:
		return "healthy"
	}
}

func normalizedQueryWindow(window time.Duration) time.Duration {
	switch window {
	case 15 * time.Minute, time.Hour, 6 * time.Hour, 24 * time.Hour, 7 * 24 * time.Hour:
		return window
	default:
		if window > 0 && window < 15*time.Minute {
			return 15 * time.Minute
		}
		return 24 * time.Hour
	}
}

func bucketIntervalLiteral(window time.Duration) string {
	switch {
	case window <= 15*time.Minute:
		return "1 minute"
	case window <= time.Hour:
		return "5 minutes"
	case window <= 6*time.Hour:
		return "15 minutes"
	case window <= 24*time.Hour:
		return "1 hour"
	default:
		return "6 hours"
	}
}

func bucketIntervalSeconds(interval string) int {
	switch interval {
	case "1 minute":
		return 60
	case "5 minutes":
		return 5 * 60
	case "15 minutes":
		return 15 * 60
	case "6 hours":
		return 6 * 3600
	default:
		return 3600
	}
}
