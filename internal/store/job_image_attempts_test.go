package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestStoreRecordsImageAttempts(t *testing.T) {
	s := openAttemptTestStore(t)
	ctx := context.Background()
	started := time.Unix(100, 0)
	finished := time.Unix(103, 0)

	if err := s.StartImageAttempt(ctx, "job_1", 1, 1, started); err != nil {
		t.Fatalf("StartImageAttempt first attempt returned error: %v", err)
	}
	if err := s.FinishImageAttempt(ctx, "job_1", 1, 1, "failed", finished, "", "", "temporary failure", "stdout tail", "stderr tail"); err != nil {
		t.Fatalf("FinishImageAttempt first attempt returned error: %v", err)
	}
	if err := s.StartImageAttempt(ctx, "job_1", 1, 2, finished); err != nil {
		t.Fatalf("StartImageAttempt second attempt returned error: %v", err)
	}
	if err := s.FinishImageAttempt(ctx, "job_1", 1, 2, "done", finished.Add(2*time.Second), "/tmp/out.png", "file:///tmp/out.png", "", "raw output", ""); err != nil {
		t.Fatalf("FinishImageAttempt second attempt returned error: %v", err)
	}

	attempts, err := s.ListImageAttempts(ctx, "job_1")
	if err != nil {
		t.Fatalf("ListImageAttempts returned error: %v", err)
	}
	if len(attempts) != 2 {
		t.Fatalf("expected 2 attempts, got %d", len(attempts))
	}
	if attempts[0].Attempt != 1 || attempts[0].Status != "failed" || attempts[0].LastError != "temporary failure" {
		t.Fatalf("unexpected first attempt: %+v", attempts[0])
	}
	if attempts[0].DurationMS != 3000 {
		t.Fatalf("expected first attempt duration 3000ms, got %d", attempts[0].DurationMS)
	}
	if attempts[1].Attempt != 2 || attempts[1].Status != "done" || attempts[1].Path != "/tmp/out.png" {
		t.Fatalf("unexpected second attempt: %+v", attempts[1])
	}
}

func TestFinishImageAttemptReturnsNotFoundForMissingAttempt(t *testing.T) {
	s := openAttemptTestStore(t)
	err := s.FinishImageAttempt(context.Background(), "job_1", 1, 1, "done", time.Unix(100, 0), "", "", "", "", "")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestStartImageAttemptResetsExistingAttempt(t *testing.T) {
	s := openAttemptTestStore(t)
	ctx := context.Background()
	started := time.Unix(100, 0)
	restarted := time.Unix(200, 0)

	if err := s.StartImageAttempt(ctx, "job_1", 1, 1, started); err != nil {
		t.Fatalf("StartImageAttempt returned error: %v", err)
	}
	if err := s.FinishImageAttempt(ctx, "job_1", 1, 1, "done", started.Add(time.Second), "/tmp/out.png", "file:///tmp/out.png", "error", "stdout", "stderr"); err != nil {
		t.Fatalf("FinishImageAttempt returned error: %v", err)
	}
	if err := s.StartImageAttempt(ctx, "job_1", 1, 1, restarted); err != nil {
		t.Fatalf("StartImageAttempt reset returned error: %v", err)
	}

	attempts, err := s.ListImageAttempts(ctx, "job_1")
	if err != nil {
		t.Fatalf("ListImageAttempts returned error: %v", err)
	}
	if len(attempts) != 1 {
		t.Fatalf("expected 1 attempt, got %d", len(attempts))
	}
	got := attempts[0]
	if got.Status != "running" {
		t.Fatalf("expected status running, got %q", got.Status)
	}
	if !got.StartedAt.Equal(restarted) {
		t.Fatalf("expected started_at %v, got %v", restarted, got.StartedAt)
	}
	if !got.FinishedAt.IsZero() {
		t.Fatalf("expected zero finished_at, got %v", got.FinishedAt)
	}
	if got.DurationMS != 0 {
		t.Fatalf("expected duration 0, got %d", got.DurationMS)
	}
	if got.Path != "" || got.URI != "" || got.LastError != "" || got.StdoutTail != "" || got.StderrTail != "" {
		t.Fatalf("expected reset attempt fields to be empty, got %+v", got)
	}
}

func TestFinishImageAttemptClampsNegativeDuration(t *testing.T) {
	s := openAttemptTestStore(t)
	ctx := context.Background()
	if err := s.StartImageAttempt(ctx, "job_1", 1, 1, time.Unix(200, 0)); err != nil {
		t.Fatalf("StartImageAttempt returned error: %v", err)
	}
	if err := s.FinishImageAttempt(ctx, "job_1", 1, 1, "failed", time.Unix(100, 0), "", "", "", "", ""); err != nil {
		t.Fatalf("FinishImageAttempt returned error: %v", err)
	}

	attempts, err := s.ListImageAttempts(ctx, "job_1")
	if err != nil {
		t.Fatalf("ListImageAttempts returned error: %v", err)
	}
	if len(attempts) != 1 {
		t.Fatalf("expected 1 attempt, got %d", len(attempts))
	}
	if attempts[0].DurationMS != 0 {
		t.Fatalf("expected duration 0, got %d", attempts[0].DurationMS)
	}
}

func openAttemptTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "imgen.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	})
	return s
}
