package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

type SensitiveLogBackfillStats struct {
	MessagePayloadsUpdated            int `json:"message_payloads_updated"`
	SourceLatestPayloadSamplesUpdated int `json:"source_latest_payload_samples_updated"`
	DeliveryRecipientSnapshotsUpdated int `json:"delivery_recipient_snapshots_updated"`
	DeliveryRequestSnapshotsUpdated   int `json:"delivery_request_snapshots_updated"`
	DeliveryResponseSnapshotsUpdated  int `json:"delivery_response_snapshots_updated"`
	AuditRequestSnapshotsUpdated      int `json:"audit_request_snapshots_updated"`
	AuditResponseSnapshotsUpdated     int `json:"audit_response_snapshots_updated"`
}

type jsonColumnUpdate struct {
	id      string
	values  []json.RawMessage
	changed []bool
}

func (r Repository) BackfillSensitiveLogData(ctx context.Context) (SensitiveLogBackfillStats, error) {
	if r.pool == nil {
		return SensitiveLogBackfillStats{}, errors.New("postgres pool is nil")
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return SensitiveLogBackfillStats{}, fmt.Errorf("begin sensitive log backfill transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var stats SensitiveLogBackfillStats
	if err := backfillMessagePayloads(ctx, tx, &stats); err != nil {
		return SensitiveLogBackfillStats{}, err
	}
	if err := backfillSourceLatestPayloadSamples(ctx, tx, &stats); err != nil {
		return SensitiveLogBackfillStats{}, err
	}
	if err := backfillDeliverySnapshots(ctx, tx, &stats); err != nil {
		return SensitiveLogBackfillStats{}, err
	}
	if err := backfillAuditSnapshots(ctx, tx, &stats); err != nil {
		return SensitiveLogBackfillStats{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return SensitiveLogBackfillStats{}, fmt.Errorf("commit sensitive log backfill transaction: %w", err)
	}
	return stats, nil
}

func backfillMessagePayloads(ctx context.Context, tx pgx.Tx, stats *SensitiveLogBackfillStats) error {
	updates, err := collectSingleJSONColumnUpdates(ctx, tx, `
		SELECT id::text, payload
		FROM message_records
	`, maxStoredMessagePayloadBytes)
	if err != nil {
		return fmt.Errorf("collect message payload backfill rows: %w", err)
	}
	for _, update := range updates {
		if _, err := tx.Exec(ctx, `
			UPDATE message_records
			SET payload = $2
			WHERE id = $1::uuid
		`, update.id, update.values[0]); err != nil {
			return fmt.Errorf("update message %s payload: %w", update.id, err)
		}
		stats.MessagePayloadsUpdated++
	}
	return nil
}

func backfillSourceLatestPayloadSamples(ctx context.Context, tx pgx.Tx, stats *SensitiveLogBackfillStats) error {
	updates, err := collectSingleJSONColumnUpdates(ctx, tx, `
		SELECT id::text, latest_payload_sample
		FROM inbound_sources
		WHERE latest_payload_sample IS NOT NULL
	`, maxStoredLatestPayloadSampleBytes)
	if err != nil {
		return fmt.Errorf("collect source latest payload sample backfill rows: %w", err)
	}
	for _, update := range updates {
		if _, err := tx.Exec(ctx, `
			UPDATE inbound_sources
			SET latest_payload_sample = $2
			WHERE id = $1::uuid
		`, update.id, update.values[0]); err != nil {
			return fmt.Errorf("update source %s latest payload sample: %w", update.id, err)
		}
		stats.SourceLatestPayloadSamplesUpdated++
	}
	return nil
}

func backfillDeliverySnapshots(ctx context.Context, tx pgx.Tx, stats *SensitiveLogBackfillStats) error {
	updates, err := collectTripleJSONColumnUpdates(ctx, tx, `
		SELECT id::text, recipient_snapshot, request_snapshot, response_snapshot
		FROM delivery_attempts
	`, maxStoredMessagePayloadBytes)
	if err != nil {
		return fmt.Errorf("collect delivery snapshot backfill rows: %w", err)
	}
	for _, update := range updates {
		if _, err := tx.Exec(ctx, `
			UPDATE delivery_attempts
			SET recipient_snapshot = $2,
				request_snapshot = $3,
				response_snapshot = $4,
				updated_at = now()
			WHERE id = $1::uuid
		`, update.id, update.values[0], update.values[1], update.values[2]); err != nil {
			return fmt.Errorf("update delivery attempt %s snapshots: %w", update.id, err)
		}
		if update.changed[0] {
			stats.DeliveryRecipientSnapshotsUpdated++
		}
		if update.changed[1] {
			stats.DeliveryRequestSnapshotsUpdated++
		}
		if update.changed[2] {
			stats.DeliveryResponseSnapshotsUpdated++
		}
	}
	return nil
}

func backfillAuditSnapshots(ctx context.Context, tx pgx.Tx, stats *SensitiveLogBackfillStats) error {
	updates, err := collectDoubleJSONColumnUpdates(ctx, tx, `
		SELECT id::text, request_snapshot, response_snapshot
		FROM audit_logs
	`, maxStoredMessagePayloadBytes)
	if err != nil {
		return fmt.Errorf("collect audit snapshot backfill rows: %w", err)
	}
	for _, update := range updates {
		if _, err := tx.Exec(ctx, `
			UPDATE audit_logs
			SET request_snapshot = $2,
				response_snapshot = $3
			WHERE id = $1::uuid
		`, update.id, update.values[0], update.values[1]); err != nil {
			return fmt.Errorf("update audit log %s snapshots: %w", update.id, err)
		}
		if update.changed[0] {
			stats.AuditRequestSnapshotsUpdated++
		}
		if update.changed[1] {
			stats.AuditResponseSnapshotsUpdated++
		}
	}
	return nil
}

func collectSingleJSONColumnUpdates(ctx context.Context, tx pgx.Tx, sql string, maxBytes int) ([]jsonColumnUpdate, error) {
	rows, err := tx.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var updates []jsonColumnUpdate
	for rows.Next() {
		var id string
		var raw []byte
		if err := rows.Scan(&id, &raw); err != nil {
			return nil, err
		}
		minimized, changed := minimizeStoredLogJSON(raw, maxBytes)
		if changed {
			updates = append(updates, jsonColumnUpdate{id: id, values: []json.RawMessage{minimized}, changed: []bool{true}})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return updates, nil
}

func collectDoubleJSONColumnUpdates(ctx context.Context, tx pgx.Tx, sql string, maxBytes int) ([]jsonColumnUpdate, error) {
	rows, err := tx.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var updates []jsonColumnUpdate
	for rows.Next() {
		var id string
		var first []byte
		var second []byte
		if err := rows.Scan(&id, &first, &second); err != nil {
			return nil, err
		}
		minimizedFirst, firstChanged := minimizeStoredLogJSON(first, maxBytes)
		minimizedSecond, secondChanged := minimizeStoredLogJSON(second, maxBytes)
		if firstChanged || secondChanged {
			updates = append(updates, jsonColumnUpdate{
				id:      id,
				values:  []json.RawMessage{minimizedFirst, minimizedSecond},
				changed: []bool{firstChanged, secondChanged},
			})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return updates, nil
}

func collectTripleJSONColumnUpdates(ctx context.Context, tx pgx.Tx, sql string, maxBytes int) ([]jsonColumnUpdate, error) {
	rows, err := tx.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var updates []jsonColumnUpdate
	for rows.Next() {
		var id string
		var first []byte
		var second []byte
		var third []byte
		if err := rows.Scan(&id, &first, &second, &third); err != nil {
			return nil, err
		}
		minimizedFirst, firstChanged := minimizeStoredLogJSON(first, maxBytes)
		minimizedSecond, secondChanged := minimizeStoredLogJSON(second, maxBytes)
		minimizedThird, thirdChanged := minimizeStoredLogJSON(third, maxBytes)
		if firstChanged || secondChanged || thirdChanged {
			updates = append(updates, jsonColumnUpdate{
				id:      id,
				values:  []json.RawMessage{minimizedFirst, minimizedSecond, minimizedThird},
				changed: []bool{firstChanged, secondChanged, thirdChanged},
			})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return updates, nil
}
