package store

import (
	"context"
	"database/sql"
	"time"
)

func (s *Store) StartImageAttempt(ctx context.Context, jobID string, imageIndex int, attempt int, startedAt time.Time) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO job_image_attempts (job_id, image_index, attempt, status, started_at) VALUES (?, ?, ?, 'running', ?)
		ON CONFLICT(job_id, image_index, attempt) DO UPDATE SET
			status = 'running',
			started_at = excluded.started_at,
			finished_at = 0,
			duration_ms = 0,
			path = '',
			uri = '',
			last_error = '',
			stdout_tail = '',
			stderr_tail = ''`, jobID, imageIndex, attempt, startedAt.Unix())
	return err
}

func (s *Store) FinishImageAttempt(ctx context.Context, jobID string, imageIndex int, attempt int, status string, finishedAt time.Time, path string, uri string, lastError string, stdoutTail string, stderrTail string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var startedAt int64
	if err := tx.QueryRowContext(ctx, `SELECT started_at FROM job_image_attempts WHERE job_id = ? AND image_index = ? AND attempt = ?`, jobID, imageIndex, attempt).Scan(&startedAt); err != nil {
		if err == sql.ErrNoRows {
			return ErrNotFound
		}
		return err
	}
	durationMS := finishedAt.Sub(unixTime(startedAt)).Milliseconds()
	if durationMS < 0 {
		durationMS = 0
	}
	result, err := tx.ExecContext(ctx, `UPDATE job_image_attempts SET status = ?, finished_at = ?, duration_ms = ?, path = ?, uri = ?, last_error = ?, stdout_tail = ?, stderr_tail = ? WHERE job_id = ? AND image_index = ? AND attempt = ?`, status, finishedAt.Unix(), durationMS, path, uri, lastError, stdoutTail, stderrTail, jobID, imageIndex, attempt)
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected != 1 {
		return ErrNotFound
	}
	return tx.Commit()
}

func (s *Store) ListImageAttempts(ctx context.Context, jobID string) ([]JobImageAttempt, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT job_id, image_index, attempt, status, started_at, finished_at, duration_ms, path, uri, last_error, stdout_tail, stderr_tail FROM job_image_attempts WHERE job_id = ? ORDER BY image_index, attempt`, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var attempts []JobImageAttempt
	for rows.Next() {
		var attempt JobImageAttempt
		var startedAt int64
		var finishedAt int64
		if err := rows.Scan(&attempt.JobID, &attempt.ImageIndex, &attempt.Attempt, &attempt.Status, &startedAt, &finishedAt, &attempt.DurationMS, &attempt.Path, &attempt.URI, &attempt.LastError, &attempt.StdoutTail, &attempt.StderrTail); err != nil {
			return nil, err
		}
		if startedAt > 0 {
			attempt.StartedAt = unixTime(startedAt)
		}
		if finishedAt > 0 {
			attempt.FinishedAt = unixTime(finishedAt)
		}
		attempts = append(attempts, attempt)
	}
	return attempts, rows.Err()
}
