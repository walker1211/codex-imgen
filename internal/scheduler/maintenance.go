package scheduler

import (
	"context"
	"time"

	"github.com/walker1211/codex-imgen/internal/notify"
	"github.com/walker1211/codex-imgen/internal/store"
)

type MaintenanceStore interface {
	MarkExpiredRunningTasks(context.Context, time.Time, time.Duration) error
	FinalizeJobsFromImages(context.Context) error
	ListJobs(context.Context, int) ([]store.Job, error)
	GetJob(context.Context, string) (store.Job, []store.JobImage, error)
	UpdateJobNotification(context.Context, string, string) error
}

type Maintenance struct {
	Store        MaintenanceStore
	Now          func() time.Time
	Mailer       notify.Mailer
	LeaseTimeout time.Duration
}

func (m Maintenance) RunOnce(ctx context.Context) error {
	now := time.Now()
	if m.Now != nil {
		now = m.Now()
	}
	leaseTimeout := m.LeaseTimeout
	if leaseTimeout <= 0 {
		leaseTimeout = 30 * time.Minute
	}
	if err := m.Store.MarkExpiredRunningTasks(ctx, now, leaseTimeout); err != nil {
		return err
	}
	if err := m.Store.FinalizeJobsFromImages(ctx); err != nil {
		return err
	}
	jobs, err := m.Store.ListJobs(ctx, 100)
	if err != nil {
		return err
	}
	for _, job := range jobs {
		if err := notify.NotifyFailureIfNeeded(ctx, m.Store, m.Mailer, job.JobID); err != nil {
			return err
		}
	}
	return nil
}
