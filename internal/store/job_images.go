package store

import (
	"context"
	"time"
)

func (s *Store) MarkExpiredRunningTasks(ctx context.Context, now time.Time, leaseTimeout time.Duration) error {
	cutoff := now.Add(-leaseTimeout).Unix()
	_, err := s.db.ExecContext(ctx, `UPDATE job_images SET status = 'queued', last_error = 'timeout' WHERE status = 'running' AND started_at IS NOT NULL AND started_at <= ?`, cutoff)
	return err
}

func (s *Store) FinalizeJobsFromImages(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `SELECT job_id FROM jobs WHERE status = 'running'`)
	if err != nil {
		return err
	}
	defer rows.Close()
	var jobIDs []string
	for rows.Next() {
		var jobID string
		if err := rows.Scan(&jobID); err != nil {
			return err
		}
		jobIDs = append(jobIDs, jobID)
	}
	for _, jobID := range jobIDs {
		var doneCount, failedCount, runningCount, queuedCount int
		if err := s.db.QueryRowContext(ctx, `SELECT
			SUM(CASE WHEN status = 'done' THEN 1 ELSE 0 END),
			SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END),
			SUM(CASE WHEN status = 'running' THEN 1 ELSE 0 END),
			SUM(CASE WHEN status = 'queued' THEN 1 ELSE 0 END)
		FROM job_images WHERE job_id = ?`, jobID).Scan(&doneCount, &failedCount, &runningCount, &queuedCount); err != nil {
			return err
		}
		if runningCount == 0 && queuedCount == 0 {
			status := "failed"
			if doneCount > 0 && failedCount > 0 {
				status = "partial_success"
			} else if doneCount > 0 && failedCount == 0 {
				status = "completed"
			}
			if _, err := s.db.ExecContext(ctx, `UPDATE jobs SET status = ? WHERE job_id = ?`, status, jobID); err != nil {
				return err
			}
		}
	}
	return rows.Err()
}
