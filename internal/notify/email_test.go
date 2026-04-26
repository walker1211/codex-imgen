package notify

import (
	"context"
	"errors"
	"io"
	"net/smtp"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/walker1211/codex-imgen/internal/config"
	"github.com/walker1211/codex-imgen/internal/store"
)

type stubDialer struct {
	called  bool
	err     error
	subject string
	body    string
}

func (d *stubDialer) Send(from string, to string, subject string, body string) error {
	d.called = true
	d.subject = subject
	d.body = body
	return d.err
}

func TestMailerSendsFailureNotification(t *testing.T) {
	dialer := &stubDialer{}
	mailer := Mailer{Config: config.EmailConfig{Enabled: true, From: "from@example.com", To: "to@example.com"}, Dialer: dialer}
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

func TestMailerSkipsWhenDisabled(t *testing.T) {
	dialer := &stubDialer{}
	mailer := Mailer{Config: config.EmailConfig{Enabled: false, From: "from@example.com", To: "to@example.com"}, Dialer: dialer}
	if err := mailer.SendFailure(FailureMail{To: "to@example.com", JobID: "job_123"}); err != nil {
		t.Fatalf("SendFailure returned error: %v", err)
	}
	if dialer.called {
		t.Fatal("did not expect dialer to be called")
	}
}

func TestMailerReturnsErrorWhenEnabledWithoutDialer(t *testing.T) {
	mailer := Mailer{Config: config.EmailConfig{Enabled: true, From: "from@example.com", To: "to@example.com"}}
	if err := mailer.SendFailure(FailureMail{To: "to@example.com", JobID: "job_123"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestNotifyOnFinalFailureSkipsWhenEmailDisabled(t *testing.T) {
	s := openNotificationStore(t)
	ctx := context.Background()
	createFailedJob(t, ctx, s)

	if err := NotifyFailureIfNeeded(ctx, s, Mailer{Config: config.EmailConfig{Enabled: false}}, "job_1"); err != nil {
		t.Fatalf("NotifyFailureIfNeeded returned error: %v", err)
	}
	got, _, err := s.GetJob(ctx, "job_1")
	if err != nil {
		t.Fatalf("GetJob returned error: %v", err)
	}
	if got.NotificationStatus != "pending" {
		t.Fatalf("notification status = %q", got.NotificationStatus)
	}
}

func TestNotifyOnFinalFailureMarksJob(t *testing.T) {
	s := openNotificationStore(t)
	ctx := context.Background()
	createFailedJob(t, ctx, s)

	dialer := &stubDialer{}
	mailer := Mailer{Config: config.EmailConfig{Enabled: true, From: "from@example.com", To: "to@example.com"}, Dialer: dialer}
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
	if dialer.subject != "[codex-imgen] job failed: job_1" {
		t.Fatalf("subject = %q", dialer.subject)
	}
}

func TestNotifyOnFinalFailureMarksFailedWhenSendFails(t *testing.T) {
	s := openNotificationStore(t)
	ctx := context.Background()
	createFailedJob(t, ctx, s)

	mailer := Mailer{Config: config.EmailConfig{Enabled: true, From: "from@example.com", To: "to@example.com"}, Dialer: &stubDialer{err: errors.New("smtp secret failure")}}
	stderr := captureStderr(t, func() {
		if err := NotifyFailureIfNeeded(ctx, s, mailer, "job_1"); err != nil {
			t.Fatalf("NotifyFailureIfNeeded returned error: %v", err)
		}
	})
	got, _, err := s.GetJob(ctx, "job_1")
	if err != nil {
		t.Fatalf("GetJob returned error: %v", err)
	}
	if got.NotificationStatus != "failed" {
		t.Fatalf("notification status = %q", got.NotificationStatus)
	}
	if !strings.Contains(stderr, "maintenance notification failed job_id=job_1 error_len=") {
		t.Fatalf("stderr = %q", stderr)
	}
	if strings.Contains(stderr, "smtp secret failure") {
		t.Fatalf("stderr leaked raw smtp error: %q", stderr)
	}
}

func TestSMTPDialerRetriesUntilSuccess(t *testing.T) {
	attempts := 0
	dialer := SMTPDialer{
		Config: config.EmailConfig{Enabled: true, SMTPHost: "smtp.example.com", SMTPPort: 587, From: "from@example.com", To: "to@example.com", Timeout: time.Second, RetryTimes: 3, RetryWaitTime: time.Millisecond, SMTPAuthCode: "secret"},
		SendFunc: func(addr string, auth smtp.Auth, from string, to []string, msg []byte) error {
			attempts++
			if attempts < 2 {
				return errors.New("temporary failure")
			}
			if addr != "smtp.example.com:587" {
				t.Fatalf("addr = %q", addr)
			}
			if from != "from@example.com" {
				t.Fatalf("from = %q", from)
			}
			if len(to) != 1 || to[0] != "to@example.com" {
				t.Fatalf("to = %v", to)
			}
			if len(msg) == 0 {
				t.Fatal("expected message")
			}
			return nil
		},
		Sleep: func(time.Duration) {},
	}
	if err := dialer.Send("from@example.com", "to@example.com", "subject", "body"); err != nil {
		t.Fatalf("Send returned error: %v", err)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d", attempts)
	}
}

func TestSMTPDialerReturnsLastErrorAfterRetries(t *testing.T) {
	attempts := 0
	sleeps := 0
	dialer := SMTPDialer{
		Config: config.EmailConfig{Enabled: true, SMTPHost: "smtp.example.com", SMTPPort: 587, From: "from@example.com", To: "to@example.com", Timeout: time.Second, RetryTimes: 3, RetryWaitTime: time.Millisecond, SMTPAuthCode: "secret"},
		SendFunc: func(addr string, auth smtp.Auth, from string, to []string, msg []byte) error {
			attempts++
			return errors.New("smtp failed")
		},
		Sleep: func(time.Duration) { sleeps++ },
	}
	if err := dialer.Send("from@example.com", "to@example.com", "subject", "body"); err == nil {
		t.Fatal("expected error")
	}
	if attempts != 3 {
		t.Fatalf("attempts = %d", attempts)
	}
	if sleeps != 2 {
		t.Fatalf("sleeps = %d", sleeps)
	}
}

func TestValidateEmailConfigRequiresAuthCode(t *testing.T) {
	err := ValidateEmailConfig(config.EmailConfig{Enabled: true, SMTPHost: "smtp.example.com", SMTPPort: 465, From: "from@example.com", To: "to@example.com", Timeout: time.Second, RetryTimes: 1})
	if err == nil {
		t.Fatal("expected error")
	}
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	originalStderr := os.Stderr
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe returned error: %v", err)
	}
	defer reader.Close()
	os.Stderr = writer
	defer func() { os.Stderr = originalStderr }()
	fn()
	if err := writer.Close(); err != nil {
		t.Fatalf("closing stderr writer returned error: %v", err)
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("reading stderr returned error: %v", err)
	}
	return string(data)
}

func openNotificationStore(t *testing.T) *store.Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "imgen.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func createFailedJob(t *testing.T, ctx context.Context, s *store.Store) {
	t.Helper()
	job := store.Job{JobID: "job_1", Prompt: "x", RequestedCount: 1, EffectiveCount: 1, RequestedConcurrency: 1, EffectiveConcurrency: 1, Status: "failed", NotificationStatus: "pending"}
	images := []store.JobImage{{JobID: "job_1", ImageIndex: 1, Status: "failed", LastError: "all attempts failed"}}
	if err := s.CreateJob(ctx, job, images); err != nil {
		t.Fatalf("CreateJob returned error: %v", err)
	}
}
