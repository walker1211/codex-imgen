package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestStoreRecordsImageAttemptPhases(t *testing.T) {
	s := openPhaseTestStore(t)
	ctx := context.Background()
	occurred := time.Unix(100, 123_000_000)

	if err := s.RecordImageAttemptPhase(ctx, "job_1", 2, 1, "process.started", occurred, 123, "pid=set"); err != nil {
		t.Fatalf("RecordImageAttemptPhase returned error: %v", err)
	}
	if err := s.RecordImageAttemptPhase(ctx, "job_1", 2, 1, "stdout.thread_started", occurred.Add(2*time.Second), 2123, "thread_id_len=36"); err != nil {
		t.Fatalf("RecordImageAttemptPhase second phase returned error: %v", err)
	}

	phases, err := s.ListImageAttemptPhases(ctx, "job_1")
	if err != nil {
		t.Fatalf("ListImageAttemptPhases returned error: %v", err)
	}
	if len(phases) != 2 {
		t.Fatalf("expected 2 phases, got %d", len(phases))
	}
	if phases[0].JobID != "job_1" || phases[0].ImageIndex != 2 || phases[0].Attempt != 1 || phases[0].Phase != "process.started" {
		t.Fatalf("unexpected first phase: %+v", phases[0])
	}
	if phases[0].OccurredAtMS != occurred.UnixMilli() || phases[0].ElapsedMS != 123 || phases[0].Detail != "pid=set" {
		t.Fatalf("unexpected first phase timing/detail: %+v", phases[0])
	}
	if phases[1].Phase != "stdout.thread_started" || phases[1].ElapsedMS != 2123 {
		t.Fatalf("unexpected second phase: %+v", phases[1])
	}
}

func TestStoreUpdatesExistingImageAttemptPhase(t *testing.T) {
	s := openPhaseTestStore(t)
	ctx := context.Background()
	first := time.Unix(100, 0)
	second := time.Unix(101, 500_000_000)

	if err := s.RecordImageAttemptPhase(ctx, "job_1", 1, 1, "stdout.first_line", first, 1000, "first"); err != nil {
		t.Fatalf("RecordImageAttemptPhase first returned error: %v", err)
	}
	if err := s.RecordImageAttemptPhase(ctx, "job_1", 1, 1, "stdout.first_line", second, 2500, "updated"); err != nil {
		t.Fatalf("RecordImageAttemptPhase update returned error: %v", err)
	}

	phases, err := s.ListImageAttemptPhases(ctx, "job_1")
	if err != nil {
		t.Fatalf("ListImageAttemptPhases returned error: %v", err)
	}
	if len(phases) != 1 {
		t.Fatalf("expected 1 phase, got %d", len(phases))
	}
	if phases[0].OccurredAtMS != second.UnixMilli() || phases[0].ElapsedMS != 2500 || phases[0].Detail != "updated" {
		t.Fatalf("expected updated phase, got %+v", phases[0])
	}
}

func openPhaseTestStore(t *testing.T) *Store {
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
