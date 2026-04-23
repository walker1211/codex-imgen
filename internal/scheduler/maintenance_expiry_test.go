package scheduler

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/walker1211/codex-imgen/internal/store"
)

func TestMaintenanceDoesNotFailFreshRunningTask(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "imgen.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer s.Close()

	ctx := context.Background()
	job := store.Job{JobID: "job_1", Prompt: "x", RequestedCount: 1, EffectiveCount: 1, RequestedConcurrency: 1, EffectiveConcurrency: 1, Status: "running", NotificationStatus: "pending"}
	images := []store.JobImage{{JobID: "job_1", ImageIndex: 1, Status: "running", StartedAt: time.Unix(120, 0)}}
	if err := s.CreateJob(ctx, job, images); err != nil {
		t.Fatalf("CreateJob returned error: %v", err)
	}

	m := Maintenance{Store: s, Now: func() time.Time { return time.Unix(123, 0) }, LeaseTimeout: 30 * time.Minute}
	if err := m.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}
	gotJob, gotImages, err := s.GetJob(ctx, "job_1")
	if err != nil {
		t.Fatalf("GetJob returned error: %v", err)
	}
	if gotImages[0].Status != "running" {
		t.Fatalf("image status = %q", gotImages[0].Status)
	}
	if gotJob.Status != "running" {
		t.Fatalf("job status = %q", gotJob.Status)
	}
}
