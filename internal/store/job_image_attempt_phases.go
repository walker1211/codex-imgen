package store

import (
	"context"
	"time"
)

func (s *Store) RecordImageAttemptPhase(ctx context.Context, jobID string, imageIndex int, attempt int, phase string, occurredAt time.Time, elapsedMS int64, detail string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO job_image_attempt_phases (job_id, image_index, attempt, phase, occurred_at_ms, elapsed_ms, detail) VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(job_id, image_index, attempt, phase) DO UPDATE SET
			occurred_at_ms = excluded.occurred_at_ms,
			elapsed_ms = excluded.elapsed_ms,
			detail = excluded.detail`, jobID, imageIndex, attempt, phase, occurredAt.UnixMilli(), elapsedMS, detail)
	return err
}

func (s *Store) ListImageAttemptPhases(ctx context.Context, jobID string) ([]JobImageAttemptPhase, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT job_id, image_index, attempt, phase, occurred_at_ms, elapsed_ms, detail FROM job_image_attempt_phases WHERE job_id = ? ORDER BY image_index, attempt, occurred_at_ms, id`, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var phases []JobImageAttemptPhase
	for rows.Next() {
		var phase JobImageAttemptPhase
		if err := rows.Scan(&phase.JobID, &phase.ImageIndex, &phase.Attempt, &phase.Phase, &phase.OccurredAtMS, &phase.ElapsedMS, &phase.Detail); err != nil {
			return nil, err
		}
		phases = append(phases, phase)
	}
	return phases, rows.Err()
}
