package store

import "context"

func (s *Store) AppendJobEvent(ctx context.Context, event JobEvent) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO job_events (job_id, event_type) VALUES (?, ?)`, event.JobID, event.EventType)
	return err
}
