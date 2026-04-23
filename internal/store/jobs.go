package store

import (
	"context"
	"database/sql"
	"time"
)

func (s *Store) CreateJob(ctx context.Context, job Job, images []JobImage) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO jobs (job_id, prompt, requested_count, effective_count, requested_concurrency, effective_concurrency, status, notification_status) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, job.JobID, job.Prompt, job.RequestedCount, job.EffectiveCount, job.RequestedConcurrency, job.EffectiveConcurrency, job.Status, job.NotificationStatus); err != nil {
		return err
	}
	for _, image := range images {
		if _, err := tx.ExecContext(ctx, `INSERT INTO job_images (job_id, image_index, status, path, uri, last_error, started_at) VALUES (?, ?, ?, ?, ?, ?, ?)`, image.JobID, image.ImageIndex, image.Status, image.Path, image.URI, image.LastError, image.StartedAt.Unix()); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) GetJob(ctx context.Context, jobID string) (Job, []JobImage, error) {
	var job Job
	err := s.db.QueryRowContext(ctx, `SELECT job_id, prompt, requested_count, effective_count, requested_concurrency, effective_concurrency, status, notification_status FROM jobs WHERE job_id = ?`, jobID).Scan(&job.JobID, &job.Prompt, &job.RequestedCount, &job.EffectiveCount, &job.RequestedConcurrency, &job.EffectiveConcurrency, &job.Status, &job.NotificationStatus)
	if err != nil {
		if err == sql.ErrNoRows {
			return Job{}, nil, ErrNotFound
		}
		return Job{}, nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT job_id, image_index, status, path, uri, last_error, started_at FROM job_images WHERE job_id = ? ORDER BY image_index`, jobID)
	if err != nil {
		return Job{}, nil, err
	}
	defer rows.Close()
	var images []JobImage
	for rows.Next() {
		var image JobImage
		var startedAt int64
		if err := rows.Scan(&image.JobID, &image.ImageIndex, &image.Status, &image.Path, &image.URI, &image.LastError, &startedAt); err != nil {
			return Job{}, nil, err
		}
		if startedAt > 0 {
			image.StartedAt = unixTime(startedAt)
		}
		images = append(images, image)
	}
	return job, images, rows.Err()
}

func (s *Store) ListJobs(ctx context.Context, limit int) ([]Job, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.QueryContext(ctx, `SELECT job_id, prompt, requested_count, effective_count, requested_concurrency, effective_concurrency, status, notification_status FROM jobs ORDER BY rowid DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var jobs []Job
	for rows.Next() {
		var job Job
		if err := rows.Scan(&job.JobID, &job.Prompt, &job.RequestedCount, &job.EffectiveCount, &job.RequestedConcurrency, &job.EffectiveConcurrency, &job.Status, &job.NotificationStatus); err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

func (s *Store) CancelJob(ctx context.Context, jobID string) error {
	result, err := s.db.ExecContext(ctx, `UPDATE jobs SET status = 'cancelled' WHERE job_id = ?`, jobID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err == nil && affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) UpdateJobStatus(ctx context.Context, jobID string, status string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE jobs SET status = ? WHERE job_id = ?`, status, jobID)
	return err
}

func (s *Store) StartImageRun(ctx context.Context, jobID string, imageIndex int, now time.Time) error {
	_, err := s.db.ExecContext(ctx, `UPDATE job_images SET status = 'running', started_at = ? WHERE job_id = ? AND image_index = ?`, now.Unix(), jobID, imageIndex)
	return err
}

func (s *Store) UpdateImageStatus(ctx context.Context, jobID string, imageIndex int, status string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE job_images SET status = ? WHERE job_id = ? AND image_index = ?`, status, jobID, imageIndex)
	return err
}

func (s *Store) UpdateImageResult(ctx context.Context, jobID string, imageIndex int, status string, path string, uri string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE job_images SET status = ?, path = ?, uri = ? WHERE job_id = ? AND image_index = ?`, status, path, uri, jobID, imageIndex)
	return err
}

func (s *Store) CancelOutstandingImages(ctx context.Context, jobID string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE job_images SET status = 'cancelled' WHERE job_id = ? AND status != 'done'`, jobID)
	return err
}

func (s *Store) UpdateJobNotification(ctx context.Context, jobID string, status string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE jobs SET notification_status = ? WHERE job_id = ?`, status, jobID)
	return err
}
