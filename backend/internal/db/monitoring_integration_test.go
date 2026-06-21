package db

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"mvp-push-gateway/backend/internal/monitoring"
	"mvp-push-gateway/backend/internal/queue"
	"mvp-push-gateway/backend/internal/statistics"
)

func TestRepositoryGetQueueMonitoringSnapshotAggregatesOperationalMetrics(t *testing.T) {
	pool := openMigratedPool(t)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	repository := NewRepository(pool)
	now := time.Date(2026, 5, 9, 10, 0, 0, 0, time.UTC)
	channel := createTestChannel(t, ctx, repository, "monitoring-webhook")

	if _, err := repository.EnqueueJob(ctx, queue.EnqueueParams{
		ID:          testUUID(12001),
		Type:        queue.JobTypeRoutePlan,
		Payload:     json.RawMessage(`{}`),
		RunAt:       now.Add(-18 * time.Minute),
		Priority:    10,
		QueueKey:    "route-plan",
		MaxAttempts: 3,
	}); err != nil {
		t.Fatalf("enqueue route plan job: %v", err)
	}
	if _, err := repository.EnqueueJob(ctx, queue.EnqueueParams{
		ID:          testUUID(12002),
		Type:        queue.JobTypeSendMessage,
		Payload:     json.RawMessage(`{}`),
		RunAt:       now.Add(-9 * time.Minute),
		Priority:    20,
		QueueKey:    channel.ID,
		ChannelID:   channel.ID,
		MaxAttempts: 3,
	}); err != nil {
		t.Fatalf("enqueue send job: %v", err)
	}

	insertWorkerMetric(t, ctx, pool, workerMetricRow{
		ID:            testUUID(12003),
		BucketStart:   now.Add(-20 * time.Minute),
		WorkerType:    "planning",
		JobType:       "route_plan",
		Processed:     12,
		Success:       12,
		AvgDurationMS: 140,
		P99DurationMS: 260,
	})
	insertWorkerMetric(t, ctx, pool, workerMetricRow{
		ID:            testUUID(12004),
		BucketStart:   now.Add(-15 * time.Minute),
		WorkerType:    "sending",
		JobType:       "send_message",
		ChannelID:     channel.ID,
		Processed:     20,
		Success:       18,
		Failed:        2,
		RateLimited:   4,
		DeadLettered:  1,
		AvgDurationMS: 320,
		P99DurationMS: 900,
	})
	insertDeadLetterJob(t, ctx, pool, deadLetterRow{
		ID:             testUUID(12005),
		JobID:          testUUID(12002),
		Type:           "send_message",
		ChannelID:      channel.ID,
		ErrorCode:      "MGP-SEND-500",
		ErrorMessage:   "目标平台超时",
		Attempts:       3,
		DeadLetteredAt: now.Add(-5 * time.Minute),
	})
	insertDeliveryAttemptForStats(t, ctx, pool, deliveryAttemptRow{
		SourceID:     testUUID(12006),
		MessageID:    testUUID(12007),
		AttemptID:    testUUID(12008),
		ChannelID:    channel.ID,
		Status:       "failed",
		ErrorMessage: "目标平台超时",
		AttemptNo:    2,
		QueuedAt:     now.Add(-8 * time.Minute),
		StartedAt:    now.Add(-8*time.Minute + 2*time.Second),
		FinishedAt:   now.Add(-8*time.Minute + 4*time.Second),
		DurationMS:   2000,
	})

	snapshot, err := repository.GetQueueMonitoringSnapshot(ctx, monitoring.QueryParams{Now: now})
	if err != nil {
		t.Fatalf("get queue monitoring snapshot: %v", err)
	}

	if snapshot.Summary.RoutePlanPending != 1 {
		t.Fatalf("expected route_plan pending 1, got %d", snapshot.Summary.RoutePlanPending)
	}
	if snapshot.Summary.SendMessagePending != 1 {
		t.Fatalf("expected send_message pending 1, got %d", snapshot.Summary.SendMessagePending)
	}
	if snapshot.Summary.OldestJobWaitSeconds != int64((18 * time.Minute).Seconds()) {
		t.Fatalf("expected oldest wait 18 minutes, got %d seconds", snapshot.Summary.OldestJobWaitSeconds)
	}
	if snapshot.Summary.RoutePlanOldestQueuedAt == nil || !snapshot.Summary.RoutePlanOldestQueuedAt.Equal(now.Add(-18*time.Minute)) {
		t.Fatalf("unexpected route plan oldest queued at: %v", snapshot.Summary.RoutePlanOldestQueuedAt)
	}
	if snapshot.Summary.SendMessageOldestQueuedAt == nil || !snapshot.Summary.SendMessageOldestQueuedAt.Equal(now.Add(-9*time.Minute)) {
		t.Fatalf("unexpected send message oldest queued at: %v", snapshot.Summary.SendMessageOldestQueuedAt)
	}
	if snapshot.Summary.PlanningAvgDurationMS != 140 || snapshot.Summary.PlanningP99DurationMS != 260 {
		t.Fatalf("unexpected planning durations: %+v", snapshot.Summary)
	}
	if snapshot.Summary.SendingAvgDurationMS != 320 || snapshot.Summary.SendingP99DurationMS != 900 {
		t.Fatalf("unexpected sending durations: %+v", snapshot.Summary)
	}
	if snapshot.Summary.RateLimitedCount != 4 {
		t.Fatalf("expected rate limited count 4, got %d", snapshot.Summary.RateLimitedCount)
	}
	if snapshot.Summary.RateLimitedLatestAt == nil || !snapshot.Summary.RateLimitedLatestAt.Equal(now.Add(-15*time.Minute)) {
		t.Fatalf("unexpected rate limited latest at: %v", snapshot.Summary.RateLimitedLatestAt)
	}
	if snapshot.Summary.DeadLetterCount != 1 {
		t.Fatalf("expected dead letter count 1, got %d", snapshot.Summary.DeadLetterCount)
	}
	if snapshot.Summary.DeadLetterLatestAt == nil || !snapshot.Summary.DeadLetterLatestAt.Equal(now.Add(-5*time.Minute)) {
		t.Fatalf("unexpected dead letter latest at: %v", snapshot.Summary.DeadLetterLatestAt)
	}
	if snapshot.Summary.PlatformFailureRate != 10 {
		t.Fatalf("expected platform failure rate 10%%, got %v", snapshot.Summary.PlatformFailureRate)
	}
	if len(snapshot.PlatformHealth) != 1 {
		t.Fatalf("expected one platform health row, got %d", len(snapshot.PlatformHealth))
	}
	if snapshot.PlatformHealth[0].Pending != 1 || snapshot.PlatformHealth[0].RateLimited != 4 || snapshot.PlatformHealth[0].DeadLetters != 1 {
		t.Fatalf("unexpected platform health row: %+v", snapshot.PlatformHealth[0])
	}
	if len(snapshot.Trend) == 0 {
		t.Fatalf("expected queue trend points from worker metrics")
	}
	hasWorkerMetricTrend := false
	for _, point := range snapshot.Trend {
		if point.RoutePlanProcessed > 0 || point.SendMessageProcessed > 0 {
			hasWorkerMetricTrend = true
			break
		}
	}
	if !hasWorkerMetricTrend {
		t.Fatalf("expected queue trend to include real worker metric counts, got %+v", snapshot.Trend)
	}
	if snapshot.CleanupStatus.LastRunAt != nil {
		t.Fatalf("expected cleanup status to be empty before cleanup, got %+v", snapshot.CleanupStatus)
	}
}

func TestRepositoryGetQueueTrendCountsCompletedDeliveryAttemptsWithoutWorkerMetrics(t *testing.T) {
	pool := openMigratedPool(t)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	repository := NewRepository(pool)
	now := time.Date(2036, 5, 9, 10, 0, 0, 0, time.UTC)
	channel := createTestChannel(t, ctx, repository, "monitoring-attempt-trend")

	insertDeliveryAttemptForStats(t, ctx, pool, deliveryAttemptRow{
		SourceID:   testUUID(12051),
		MessageID:  testUUID(12052),
		AttemptID:  testUUID(12053),
		ChannelID:  channel.ID,
		Status:     "sent",
		AttemptNo:  1,
		QueuedAt:   now.Add(-12 * time.Minute),
		StartedAt:  now.Add(-12*time.Minute + time.Second),
		FinishedAt: now.Add(-12*time.Minute + 2*time.Second),
		DurationMS: 1000,
	})

	snapshot, err := repository.GetQueueMonitoringSnapshot(ctx, monitoring.QueryParams{
		Now:    now,
		Window: time.Hour,
	})
	if err != nil {
		t.Fatalf("get queue monitoring snapshot: %v", err)
	}

	totalSendProcessed := 0
	for _, point := range snapshot.Trend {
		totalSendProcessed += point.SendMessageProcessed
	}
	if totalSendProcessed != 1 {
		t.Fatalf("expected trend to count completed delivery attempt, got %d", totalSendProcessed)
	}
}

func TestRepositoryGetOverviewStatisticsBuildsStable24hDashboard(t *testing.T) {
	pool := openMigratedPool(t)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	repository := NewRepository(pool)
	now := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	channelA := createTestChannel(t, ctx, repository, "overview-webhook-a")
	channelB := createTestChannel(t, ctx, repository, "overview-webhook-b")
	channelC := createTestChannel(t, ctx, repository, "overview-webhook-c")

	insertWorkerMetric(t, ctx, pool, workerMetricRow{
		ID:            testUUID(12101),
		BucketStart:   now.Add(-2 * time.Hour),
		WorkerType:    "sending",
		JobType:       "send_message",
		ChannelID:     channelA.ID,
		Processed:     2,
		Success:       1,
		Failed:        1,
		RateLimited:   3,
		AvgDurationMS: 240,
		P99DurationMS: 600,
	})
	insertWorkerMetric(t, ctx, pool, workerMetricRow{
		ID:            testUUID(12102),
		BucketStart:   now.Add(-1 * time.Hour),
		WorkerType:    "sending",
		JobType:       "send_message",
		ChannelID:     channelB.ID,
		Processed:     1,
		Success:       1,
		Failed:        0,
		RateLimited:   0,
		AvgDurationMS: 180,
		P99DurationMS: 300,
	})
	insertDeliveryAttemptForStats(t, ctx, pool, deliveryAttemptRow{
		SourceID:   testUUID(12103),
		MessageID:  testUUID(12104),
		AttemptID:  testUUID(12105),
		ChannelID:  channelA.ID,
		Status:     "sent",
		AttemptNo:  1,
		QueuedAt:   now.Add(-2 * time.Hour),
		StartedAt:  now.Add(-2*time.Hour + time.Second),
		FinishedAt: now.Add(-2*time.Hour + 3*time.Second),
		DurationMS: 2000,
	})
	insertDeliveryAttemptForStats(t, ctx, pool, deliveryAttemptRow{
		SourceID:     testUUID(12106),
		MessageID:    testUUID(12107),
		AttemptID:    testUUID(12108),
		ChannelID:    channelA.ID,
		Status:       "failed",
		ErrorMessage: "目标平台超时",
		AttemptNo:    2,
		QueuedAt:     now.Add(-90 * time.Minute),
		StartedAt:    now.Add(-90*time.Minute + time.Second),
		FinishedAt:   now.Add(-90*time.Minute + 4*time.Second),
		DurationMS:   3000,
	})
	insertDeliveryAttemptForStats(t, ctx, pool, deliveryAttemptRow{
		SourceID:   testUUID(12109),
		MessageID:  testUUID(12110),
		AttemptID:  testUUID(12111),
		ChannelID:  channelB.ID,
		Status:     "sent",
		AttemptNo:  1,
		QueuedAt:   now.Add(-45 * time.Minute),
		StartedAt:  now.Add(-45*time.Minute + time.Second),
		FinishedAt: now.Add(-45*time.Minute + 2*time.Second),
		DurationMS: 1000,
	})
	insertDeliveryAttemptForStats(t, ctx, pool, deliveryAttemptRow{
		SourceID:   testUUID(12112),
		MessageID:  testUUID(12113),
		AttemptID:  testUUID(12114),
		ChannelID:  channelA.ID,
		Status:     "deduped",
		AttemptNo:  1,
		QueuedAt:   now.Add(-30 * time.Minute),
		StartedAt:  now.Add(-30*time.Minute + time.Second),
		FinishedAt: now.Add(-30*time.Minute + 2*time.Second),
		DurationMS: 1000,
	})
	insertDeliveryAttemptForStats(t, ctx, pool, deliveryAttemptRow{
		SourceID:   testUUID(12115),
		MessageID:  testUUID(12116),
		AttemptID:  testUUID(12117),
		ChannelID:  channelA.ID,
		Status:     "skipped",
		AttemptNo:  1,
		QueuedAt:   now.Add(-15 * time.Minute),
		StartedAt:  now.Add(-15*time.Minute + time.Second),
		FinishedAt: now.Add(-15*time.Minute + 2*time.Second),
		DurationMS: 1000,
	})
	insertDeliveryAttemptForStats(t, ctx, pool, deliveryAttemptRow{
		SourceID:   testUUID(12118),
		MessageID:  testUUID(12119),
		AttemptID:  testUUID(12120),
		ChannelID:  channelC.ID,
		Status:     "skipped",
		AttemptNo:  1,
		QueuedAt:   now.Add(-10 * time.Minute),
		StartedAt:  now.Add(-10*time.Minute + time.Second),
		FinishedAt: now.Add(-10*time.Minute + 2*time.Second),
		DurationMS: 1000,
	})
	insertMessageRecordForOverviewStats(t, ctx, pool, messageRecordStatsRow{
		SourceID:     testUUID(12121),
		MessageID:    testUUID(12122),
		TraceID:      "trace-planning-failed",
		Status:       "failed",
		ErrorMessage: "recipient resolution failed",
		ReceivedAt:   now.Add(-5 * time.Minute),
	})
	insertMessageRecordForOverviewStats(t, ctx, pool, messageRecordStatsRow{
		SourceID:     testUUID(12123),
		MessageID:    testUUID(12124),
		TraceID:      "trace-no-route",
		Status:       "no_route",
		ErrorMessage: "no published route for source",
		ReceivedAt:   now.Add(-4 * time.Minute),
	})

	overview, err := repository.GetOverviewStatistics(ctx, statistics.QueryParams{Now: now})
	if err != nil {
		t.Fatalf("get overview statistics: %v", err)
	}

	var totalAttempts int
	var auxiliaryAttempts int
	if err := pool.QueryRow(ctx, `
		SELECT
			count(*)::integer,
			count(*) FILTER (WHERE status IN ('deduped', 'skipped'))::integer
		FROM delivery_attempts
	`).Scan(&totalAttempts, &auxiliaryAttempts); err != nil {
		t.Fatalf("count delivery attempts: %v", err)
	}
	if totalAttempts != 6 || auxiliaryAttempts != 3 {
		t.Fatalf("expected 6 attempts including 3 deduped/skipped rows, got total=%d auxiliary=%d", totalAttempts, auxiliaryAttempts)
	}

	if overview.Summary.TotalSent != 3 {
		t.Fatalf("expected total sent to exclude deduped/skipped attempts, got %d", overview.Summary.TotalSent)
	}
	if overview.Summary.TotalReceived != 8 {
		t.Fatalf("expected total received 8, got %d", overview.Summary.TotalReceived)
	}
	if overview.Summary.SuccessRate != 40 {
		t.Fatalf("expected success rate 40 with planning/no-route failures included, got %v", overview.Summary.SuccessRate)
	}
	if overview.Summary.AverageDurationMS != 2000 {
		t.Fatalf("expected average delivery duration 2000ms, got %d", overview.Summary.AverageDurationMS)
	}
	if overview.Summary.Failed != 3 {
		t.Fatalf("expected failed 3 including planning/no-route failures, got %d", overview.Summary.Failed)
	}
	if len(overview.Trend) != 25 {
		t.Fatalf("expected 25 trend buckets for inclusive 24h window, got %d", len(overview.Trend))
	}
	totalTrendSent := 0
	totalTrendSuccessful := 0
	totalTrendFailed := 0
	for _, point := range overview.Trend {
		totalTrendSent += point.Sent
		totalTrendSuccessful += point.Successful
		totalTrendFailed += point.Failed
	}
	if totalTrendSent != 3 || totalTrendSuccessful != 2 || totalTrendFailed != 3 {
		t.Fatalf("expected trend totals to ignore deduped/skipped, got sent=%d successful=%d failed=%d", totalTrendSent, totalTrendSuccessful, totalTrendFailed)
	}
	if len(overview.PlatformRankings) != 2 {
		t.Fatalf("expected 2 platform ranking rows, got %d", len(overview.PlatformRankings))
	}
	if overview.PlatformRankings[0].ChannelID != channelA.ID || overview.PlatformRankings[0].Sent != 2 || overview.PlatformRankings[0].Failures != 1 || overview.PlatformRankings[0].SuccessRate != 50 {
		t.Fatalf("expected channel A to rank by 2 real sent/failed attempts, got %+v", overview.PlatformRankings[0])
	}
	if overview.PlatformRankings[1].ChannelID != channelB.ID || overview.PlatformRankings[1].Sent != 1 || overview.PlatformRankings[1].Failures != 0 || overview.PlatformRankings[1].SuccessRate != 100 {
		t.Fatalf("expected channel B to rank by 1 real sent/failed attempt, got %+v", overview.PlatformRankings[1])
	}
	for _, ranking := range overview.PlatformRankings {
		if ranking.ChannelID == channelC.ID {
			t.Fatalf("expected auxiliary-only channel C to be excluded from platform rankings, got %+v", overview.PlatformRankings)
		}
	}
	if len(overview.FailureRankings) == 0 || overview.FailureRankings[0].Reason != "目标平台超时" {
		t.Fatalf("expected timeout in failure rankings, got %+v", overview.FailureRankings)
	}
	if len(overview.RecentAnomalies) == 0 || overview.RecentAnomalies[0].TraceID != "trace-2107" {
		t.Fatalf("expected recent anomaly to carry latest failed trace id, got %+v", overview.RecentAnomalies)
	}
}

func TestRepositoryRunRetentionCleanupDeletesSmallBatchesAndPersistsLatestStatus(t *testing.T) {
	pool := openMigratedPool(t)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	repository := NewRepository(pool)
	now := time.Date(2026, 5, 9, 13, 0, 0, 0, time.UTC)
	cutoff := now.Add(-31 * 24 * time.Hour)
	channel := createTestChannel(t, ctx, repository, "cleanup-webhook")

	for i := 0; i < 3; i++ {
		insertTerminalJob(t, ctx, pool, testUUID(12200+i), "send_message", cutoff.Add(-time.Duration(i)*time.Hour), channel.ID)
		insertDeadLetterJob(t, ctx, pool, deadLetterRow{
			ID:             testUUID(12210 + i),
			Type:           "send_message",
			ChannelID:      channel.ID,
			ErrorCode:      "MGP-SEND-500",
			ErrorMessage:   "cleanup-old",
			Attempts:       3,
			DeadLetteredAt: cutoff.Add(-time.Duration(i) * time.Hour),
		})
		insertWorkerMetric(t, ctx, pool, workerMetricRow{
			ID:            testUUID(12220 + i),
			BucketStart:   cutoff.Add(-time.Duration(i) * time.Hour),
			WorkerType:    "sending",
			JobType:       "send_message",
			ChannelID:     channel.ID,
			Processed:     1,
			Success:       1,
			AvgDurationMS: 100,
			P99DurationMS: 150,
		})
		insertRouteRuleMetric(t, ctx, pool, routeRuleMetricRow{
			ID:            testUUID(12225 + i),
			SourceID:      testUUID(12230 + i),
			FlowID:        testUUID(12240 + i),
			VersionID:     testUUID(12250 + i),
			RuleID:        testUUID(12260 + i),
			BucketStart:   cutoff.Add(-time.Duration(i) * time.Hour),
			Evaluated:     5,
			Matched:       2,
			AvgDurationMS: 210,
			P99DurationMS: 420,
		})
		insertDeliveryAttemptForStats(t, ctx, pool, deliveryAttemptRow{
			SourceID:     testUUID(12270 + i),
			MessageID:    testUUID(12280 + i),
			AttemptID:    testUUID(12290 + i),
			ChannelID:    channel.ID,
			Status:       "failed",
			ErrorMessage: "cleanup-old",
			AttemptNo:    1,
			QueuedAt:     cutoff.Add(-time.Duration(i) * time.Hour),
			StartedAt:    cutoff.Add(-time.Duration(i)*time.Hour + time.Second),
			FinishedAt:   cutoff.Add(-time.Duration(i)*time.Hour + 2*time.Second),
			DurationMS:   1000,
		})
		insertDedupeKey(t, ctx, pool, testUUID(12320+i), testUUID(12270+i), testUUID(12280+i), "cleanup-key-"+twoDigits(i), cutoff.Add(-time.Duration(i)*time.Hour))
	}

	insertTerminalJob(t, ctx, pool, testUUID(12301), "send_message", now.Add(-time.Hour), channel.ID)

	first, err := repository.RunRetentionCleanup(ctx, monitoring.RetentionCleanupParams{
		Now:           now,
		RetentionDays: 30,
		BatchSize:     2,
	})
	if err != nil {
		t.Fatalf("run first retention cleanup: %v", err)
	}
	if first.LastBatchDeleted != 14 {
		t.Fatalf("expected first batch to delete 14 rows, got %d", first.LastBatchDeleted)
	}
	if first.DeletedDedupeKeys != 2 {
		t.Fatalf("expected first batch to delete 2 dedupe keys, got %d", first.DeletedDedupeKeys)
	}
	if !first.HasMore || first.Completed {
		t.Fatalf("expected first cleanup to report remaining rows, got %+v", first)
	}

	second, err := repository.RunRetentionCleanup(ctx, monitoring.RetentionCleanupParams{
		Now:           now.Add(time.Minute),
		RetentionDays: 30,
		BatchSize:     2,
	})
	if err != nil {
		t.Fatalf("run second retention cleanup: %v", err)
	}
	if second.TotalDeleted != 21 {
		t.Fatalf("expected cumulative deleted rows 21, got %d", second.TotalDeleted)
	}
	if second.HasMore || !second.Completed {
		t.Fatalf("expected cleanup to finish on second run, got %+v", second)
	}

	snapshot, err := repository.GetQueueMonitoringSnapshot(ctx, monitoring.QueryParams{Now: now.Add(2 * time.Minute)})
	if err != nil {
		t.Fatalf("get queue monitoring snapshot after cleanup: %v", err)
	}
	if snapshot.CleanupStatus.TotalDeleted != 21 || snapshot.CleanupStatus.RetentionDays != 30 {
		t.Fatalf("unexpected cleanup status from monitoring snapshot: %+v", snapshot.CleanupStatus)
	}

	assertCountEquals(t, ctx, pool, "jobs", 1)
	assertCountEquals(t, ctx, pool, "dead_letter_jobs", 0)
	assertCountEquals(t, ctx, pool, "worker_metrics", 0)
	assertCountEquals(t, ctx, pool, "route_rule_metrics", 0)
	assertCountEquals(t, ctx, pool, "message_records", 0)
	assertCountEquals(t, ctx, pool, "delivery_attempts", 0)
	assertCountEquals(t, ctx, pool, "dedupe_keys", 0)
}

func TestRepositoryRunRetentionCleanupDeletesAuditLogs(t *testing.T) {
	pool := openMigratedPool(t)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	repository := NewRepository(pool)
	now := time.Date(2026, 5, 9, 13, 0, 0, 0, time.UTC)
	cutoff := now.Add(-31 * 24 * time.Hour)

	for i := 0; i < 3; i++ {
		if _, err := pool.Exec(ctx, `
			INSERT INTO audit_logs (
				id, actor_username, action, resource_type, resource_id,
				request_snapshot, response_snapshot, created_at
			)
			VALUES ($1, 'admin', 'update', 'system_setting', $2, '{}'::jsonb, '{}'::jsonb, $3)
		`, testUUID(12400+i), "logs.retention_days", cutoff.Add(-time.Duration(i)*time.Hour)); err != nil {
			t.Fatalf("insert old audit log: %v", err)
		}
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO audit_logs (
			id, actor_username, action, resource_type, resource_id,
			request_snapshot, response_snapshot, created_at
		)
		VALUES ($1, 'admin', 'login', 'admin_session', $2, '{}'::jsonb, '{}'::jsonb, $3)
	`, testUUID(12410), "admin-session", now.Add(-time.Hour)); err != nil {
		t.Fatalf("insert recent audit log: %v", err)
	}

	first, err := repository.RunRetentionCleanup(ctx, monitoring.RetentionCleanupParams{
		Now:           now,
		RetentionDays: 30,
		BatchSize:     2,
	})
	if err != nil {
		t.Fatalf("run first retention cleanup: %v", err)
	}
	if first.DeletedAuditLogs != 2 || !first.HasMore {
		t.Fatalf("expected first cleanup to delete 2 audit logs and report remaining rows, got %+v", first)
	}

	second, err := repository.RunRetentionCleanup(ctx, monitoring.RetentionCleanupParams{
		Now:           now.Add(time.Minute),
		RetentionDays: 30,
		BatchSize:     2,
	})
	if err != nil {
		t.Fatalf("run second retention cleanup: %v", err)
	}
	if second.DeletedAuditLogs != 1 || second.TotalDeleted != 3 || second.HasMore || !second.Completed {
		t.Fatalf("expected second cleanup to finish audit log cleanup, got %+v", second)
	}

	var remaining int
	if err := pool.QueryRow(ctx, `SELECT count(*)::integer FROM audit_logs`).Scan(&remaining); err != nil {
		t.Fatalf("count audit logs: %v", err)
	}
	if remaining != 1 {
		t.Fatalf("expected only recent audit log to remain, got %d", remaining)
	}
}

type workerMetricRow struct {
	ID            string
	BucketStart   time.Time
	WorkerType    string
	JobType       string
	ChannelID     string
	Processed     int
	Success       int
	Failed        int
	RateLimited   int
	DeadLettered  int
	AvgDurationMS int
	P99DurationMS int
}

func insertWorkerMetric(t *testing.T, ctx context.Context, pool *pgxpool.Pool, row workerMetricRow) {
	t.Helper()
	if _, err := pool.Exec(ctx, `
		INSERT INTO worker_metrics (
			id, bucket_start, worker_type, job_type, channel_id,
			processed, success, failed, rate_limited, dead_lettered,
			avg_duration_ms, p95_duration_ms
		)
		VALUES ($1, $2, $3, $4, NULLIF($5, '')::uuid, $6, $7, $8, $9, $10, $11, $12)
	`, row.ID, row.BucketStart, row.WorkerType, row.JobType, row.ChannelID, row.Processed, row.Success, row.Failed, row.RateLimited, row.DeadLettered, row.AvgDurationMS, row.P99DurationMS); err != nil {
		t.Fatalf("insert worker metric: %v", err)
	}
}

type deadLetterRow struct {
	ID             string
	JobID          string
	Type           string
	ChannelID      string
	ErrorCode      string
	ErrorMessage   string
	Attempts       int
	DeadLetteredAt time.Time
}

func insertDeadLetterJob(t *testing.T, ctx context.Context, pool *pgxpool.Pool, row deadLetterRow) {
	t.Helper()
	if _, err := pool.Exec(ctx, `
		INSERT INTO dead_letter_jobs (
			id, job_id, type, payload, channel_id, error_code, error_message, attempts, dead_lettered_at
		)
		VALUES ($1, NULLIF($2, '')::uuid, $3, '{}'::jsonb, NULLIF($4, '')::uuid, NULLIF($5, ''), $6, $7, $8)
	`, row.ID, row.JobID, row.Type, row.ChannelID, row.ErrorCode, row.ErrorMessage, row.Attempts, row.DeadLetteredAt); err != nil {
		t.Fatalf("insert dead letter job: %v", err)
	}
}

type deliveryAttemptRow struct {
	SourceID     string
	MessageID    string
	AttemptID    string
	ChannelID    string
	Status       string
	ErrorMessage string
	AttemptNo    int
	QueuedAt     time.Time
	StartedAt    time.Time
	FinishedAt   time.Time
	DurationMS   int
}

type messageRecordStatsRow struct {
	SourceID     string
	MessageID    string
	TraceID      string
	Status       string
	ErrorMessage string
	ReceivedAt   time.Time
}

func insertMessageRecordForOverviewStats(t *testing.T, ctx context.Context, pool *pgxpool.Pool, row messageRecordStatsRow) {
	t.Helper()

	if _, err := pool.Exec(ctx, `
		INSERT INTO inbound_sources (id, code, name, auth_mode)
		VALUES ($1, $2, $3, 'none')
		ON CONFLICT (id) DO NOTHING
	`, row.SourceID, "source-"+row.SourceID[len(row.SourceID)-4:], "Source "+row.SourceID[len(row.SourceID)-4:]); err != nil {
		t.Fatalf("insert source for message record: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO message_records (
			id, trace_id, source_id, received_at, headers, payload, payload_hash,
			status, error_message, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, '{}'::jsonb, '{}'::jsonb, 'hash', $5, NULLIF($6, ''), $4, $4)
	`, row.MessageID, row.TraceID, row.SourceID, row.ReceivedAt, row.Status, row.ErrorMessage); err != nil {
		t.Fatalf("insert message record: %v", err)
	}
}

func insertDeliveryAttemptForStats(t *testing.T, ctx context.Context, pool *pgxpool.Pool, row deliveryAttemptRow) {
	t.Helper()

	if _, err := pool.Exec(ctx, `
		INSERT INTO inbound_sources (id, code, name, auth_mode)
		VALUES ($1, $2, $3, 'none')
		ON CONFLICT (id) DO NOTHING
	`, row.SourceID, "source-"+row.SourceID[len(row.SourceID)-4:], "Source "+row.SourceID[len(row.SourceID)-4:]); err != nil {
		t.Fatalf("insert source for delivery attempt: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO message_records (id, trace_id, source_id, received_at, headers, payload, payload_hash, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, '{}'::jsonb, '{}'::jsonb, 'hash', 'accepted', $4, $4)
	`, row.MessageID, "trace-"+row.MessageID[len(row.MessageID)-4:], row.SourceID, row.QueuedAt); err != nil {
		t.Fatalf("insert message record for delivery attempt: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO delivery_attempts (
			id, message_id, channel_id, recipient_snapshot, request_snapshot, response_snapshot,
			status, error_message, duration_ms, attempt_no, queued_at, started_at, finished_at, created_at, updated_at
		)
		VALUES ($1, $2, $3, '{}'::jsonb, '{}'::jsonb, '{}'::jsonb, $4, NULLIF($5, ''), $6, $7, $8, $9, $10, $8, $10)
	`, row.AttemptID, row.MessageID, row.ChannelID, row.Status, row.ErrorMessage, row.DurationMS, row.AttemptNo, row.QueuedAt, row.StartedAt, row.FinishedAt); err != nil {
		t.Fatalf("insert delivery attempt: %v", err)
	}
}

func insertDedupeKey(t *testing.T, ctx context.Context, pool *pgxpool.Pool, id, sourceID, messageID, key string, expiresAt time.Time) {
	t.Helper()
	if _, err := pool.Exec(ctx, `
		INSERT INTO dedupe_keys (id, scope, source_id, dedupe_key, expires_at, message_id)
		VALUES ($1, 'inbound', $2, $3, $4, $5)
	`, id, sourceID, key, expiresAt, messageID); err != nil {
		t.Fatalf("insert dedupe key: %v", err)
	}
}

func insertTerminalJob(t *testing.T, ctx context.Context, pool *pgxpool.Pool, jobID string, jobType string, finishedAt time.Time, channelID string) {
	t.Helper()
	if _, err := pool.Exec(ctx, `
		INSERT INTO jobs (
			id, type, status, payload, run_at, attempts, max_attempts, channel_id,
			priority, queue_key, started_at, finished_at, duration_ms, created_at, updated_at
		)
		VALUES ($1, $2, 'done', '{}'::jsonb, $3, 1, 3, NULLIF($4, '')::uuid, 100, $2, $3, $3, 1000, $3, $3)
	`, jobID, jobType, finishedAt, channelID); err != nil {
		t.Fatalf("insert terminal job: %v", err)
	}
}

type routeRuleMetricRow struct {
	ID            string
	SourceID      string
	FlowID        string
	VersionID     string
	RuleID        string
	BucketStart   time.Time
	Evaluated     int
	Matched       int
	AvgDurationMS int
	P99DurationMS int
}

func insertRouteRuleMetric(t *testing.T, ctx context.Context, pool *pgxpool.Pool, row routeRuleMetricRow) {
	t.Helper()
	if _, err := pool.Exec(ctx, `
		INSERT INTO inbound_sources (id, code, name, auth_mode)
		VALUES ($1, $2, $3, 'none')
		ON CONFLICT (id) DO NOTHING
	`, row.SourceID, "source-"+row.SourceID[len(row.SourceID)-4:], "Source "+row.SourceID[len(row.SourceID)-4:]); err != nil {
		t.Fatalf("insert source for route rule metric: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO route_flows (id, source_id, name, enabled, mode)
		VALUES ($1, $2, $3, false, 'table')
	`, row.FlowID, row.SourceID, "Flow "+row.FlowID[len(row.FlowID)-4:]); err != nil {
		t.Fatalf("insert flow for route rule metric: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO route_versions (id, flow_id, version_no, validation_status, created_at, updated_at)
		VALUES ($1, $2, 1, 'valid', $3, $3)
	`, row.VersionID, row.FlowID, row.BucketStart); err != nil {
		t.Fatalf("insert version for route rule metric: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO route_rules (id, flow_id, version_id, rule_key, sort_order, name, condition_tree, enabled, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 1, $5, '{}'::jsonb, true, $6, $6)
	`, row.RuleID, row.FlowID, row.VersionID, row.RuleID, "Rule "+row.RuleID[len(row.RuleID)-4:], row.BucketStart); err != nil {
		t.Fatalf("insert rule for route rule metric: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO route_rule_metrics (
			id, bucket_start, source_id, flow_id, route_version_id, rule_id,
			evaluated, matched, avg_duration_ms, p95_duration_ms
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, row.ID, row.BucketStart, row.SourceID, row.FlowID, row.VersionID, row.RuleID, row.Evaluated, row.Matched, row.AvgDurationMS, row.P99DurationMS); err != nil {
		t.Fatalf("insert route rule metric: %v", err)
	}
}

func assertCountEquals(t *testing.T, ctx context.Context, pool *pgxpool.Pool, table string, expected int) {
	t.Helper()
	var count int
	if err := pool.QueryRow(ctx, "SELECT count(*)::integer FROM "+table).Scan(&count); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	if count != expected {
		t.Fatalf("expected %s count %d, got %d", table, expected, count)
	}
}
