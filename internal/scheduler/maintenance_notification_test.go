package scheduler

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/walker1211/codex-imgen/internal/config"
	"github.com/walker1211/codex-imgen/internal/notify"
	"github.com/walker1211/codex-imgen/internal/store"
)

type maintenanceDialer struct{ called bool }

func (d *maintenanceDialer) Send(from string, to string, subject string, body string) error {
	d.called = true
	return nil
}

func TestMaintenanceRunOnceSendsFailureNotification(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "imgen.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer s.Close()

	ctx := context.Background()
	job := store.Job{JobID: "job_1", Prompt: "x", RequestedCount: 1, EffectiveCount: 1, RequestedConcurrency: 1, EffectiveConcurrency: 1, Status: "failed", NotificationStatus: "pending"}
	images := []store.JobImage{{JobID: "job_1", ImageIndex: 1, Status: "failed", LastError: "all attempts failed", StartedAt: time.Unix(0, 0)}}
	if err := s.CreateJob(ctx, job, images); err != nil {
		t.Fatalf("CreateJob returned error: %v", err)
	}

	dialer := &maintenanceDialer{}
	m := Maintenance{
		Store:  s,
		Now:    func() time.Time { return time.Unix(123, 0) },
		Mailer:       notify.Mailer{Config: config.EmailConfig{From: "from@example.com", To: "to@example.com"}, Dialer: dialer},
		LeaseTimeout: time.Second,
	}

	if err := m.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}
	got, _, err := s.GetJob(ctx, "job_1")
	if err != nil {
		t.Fatalf("GetJob returned error: %v", err)
	}
	if got.NotificationStatus != "sent" {
		t.Fatalf("notification status = %q", got.NotificationStatus)
	}
	if !dialer.called {
		t.Fatal("expected mail to be sent")
	}
}
