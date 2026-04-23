package notify

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/walker1211/codex-imgen/internal/config"
	"github.com/walker1211/codex-imgen/internal/store"
)

type stubDialer struct{ called bool }

func (d *stubDialer) Send(from string, to string, subject string, body string) error {
	d.called = true
	return nil
}

func TestMailerSendsFailureNotification(t *testing.T) {
	dialer := &stubDialer{}
	mailer := Mailer{Config: config.EmailConfig{From: "from@example.com", To: "to@example.com"}, Dialer: dialer}
	err := mailer.SendFailure(FailureMail{
		To:        "to@example.com",
		JobID:     "job_123",
		Prompt:    "draw a dragon",
		LastError: "all attempts failed",
	})
	if err != nil {
		t.Fatalf("SendFailure returned error: %v", err)
	}
	if !dialer.called {
		t.Fatal("expected dialer to be called")
	}
}

func TestNotifyOnFinalFailureMarksJob(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "imgen.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer s.Close()

	ctx := context.Background()
	job := store.Job{JobID: "job_1", Prompt: "x", RequestedCount: 1, EffectiveCount: 1, RequestedConcurrency: 1, EffectiveConcurrency: 1, Status: "failed", NotificationStatus: "pending"}
	images := []store.JobImage{{JobID: "job_1", ImageIndex: 1, Status: "failed", LastError: "all attempts failed"}}
	if err := s.CreateJob(ctx, job, images); err != nil {
		t.Fatalf("CreateJob returned error: %v", err)
	}

	dialer := &stubDialer{}
	mailer := Mailer{Config: config.EmailConfig{From: "from@example.com", To: "to@example.com"}, Dialer: dialer}
	if err := NotifyFailureIfNeeded(ctx, s, mailer, "job_1"); err != nil {
		t.Fatalf("NotifyFailureIfNeeded returned error: %v", err)
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
