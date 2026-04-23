package service

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	api "github.com/walker1211/codex-imgen/internal/api"
	"github.com/walker1211/codex-imgen/internal/backend"
	"github.com/walker1211/codex-imgen/internal/store"
)

type fakeGenerator struct {
	mu    sync.Mutex
	calls int
}

func (g *fakeGenerator) Generate(ctx context.Context, req backend.GenerateRequest) (backend.GenerateResult, error) {
	g.mu.Lock()
	g.calls++
	idx := g.calls
	g.mu.Unlock()
	path := fmt.Sprintf("/tmp/%d.png", idx)
	return backend.GenerateResult{Path: path, URI: "file://" + path}, nil
}

func TestServiceCreateAndGetJob(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "imgen.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer s.Close()

	gen := &fakeGenerator{}
	svc := Service{Store: s, Generator: gen, PromptPrefix: "$imagegen", PromptPrelude: "使用内置 imagegen 技能。", RetryDelays: []time.Duration{time.Millisecond, time.Millisecond}}
	created, err := svc.CreateJob(api.CreateJobRequest{Prompt: "draw a dragon", Count: 2, Concurrency: 5})
	if err != nil {
		t.Fatalf("CreateJob returned error: %v", err)
	}
	if created.JobID == "" {
		t.Fatal("expected job id")
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		got, err := svc.GetJob(created.JobID)
		if err != nil {
			t.Fatalf("GetJob returned error: %v", err)
		}
		if got.Status == "completed" {
			if got.Count != 2 {
				t.Fatalf("count = %d", got.Count)
			}
			if len(got.Images) != 2 {
				t.Fatalf("images = %d", len(got.Images))
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("job did not complete in time")
}

func TestStoreCancelJobUpdatesStatus(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "imgen.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer s.Close()

	ctx := context.Background()
	job := store.Job{JobID: "job_1", Prompt: "x", RequestedCount: 1, EffectiveCount: 1, RequestedConcurrency: 1, EffectiveConcurrency: 1, Status: "queued"}
	images := []store.JobImage{{JobID: "job_1", ImageIndex: 1, Status: "queued"}}
	if err := s.CreateJob(ctx, job, images); err != nil {
		t.Fatalf("CreateJob returned error: %v", err)
	}
	if err := s.CancelJob(ctx, "job_1"); err != nil {
		t.Fatalf("CancelJob returned error: %v", err)
	}
	got, _, err := s.GetJob(ctx, "job_1")
	if err != nil {
		t.Fatalf("GetJob returned error: %v", err)
	}
	if got.Status != "cancelled" {
		t.Fatalf("status = %q", got.Status)
	}
}

func TestServiceCreateJobClampsCountAndConcurrency(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "imgen.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer s.Close()

	svc := Service{Store: s, MaxCountPerJob: 10, MaxJobConcurrency: 10, DefaultJobConcurrency: 2}
	created, err := svc.CreateJob(api.CreateJobRequest{Prompt: "draw a dragon", Count: 99, Concurrency: 99})
	if err != nil {
		t.Fatalf("CreateJob returned error: %v", err)
	}
	got, err := svc.GetJob(created.JobID)
	if err != nil {
		t.Fatalf("GetJob returned error: %v", err)
	}
	if got.Count != 10 {
		t.Fatalf("count = %d", got.Count)
	}
}

func TestServiceRunsQueuedJobToCompletion(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "imgen.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer s.Close()

	gen := &fakeGenerator{}
	svc := Service{
		Store:                 s,
		Generator:             gen,
		PromptPrefix:          "$imagegen",
		PromptPrelude:         "使用内置 imagegen 技能。",
		RetryDelays:           []time.Duration{time.Millisecond, time.Millisecond},
		DefaultJobConcurrency: 2,
		MaxJobConcurrency:     10,
		MaxCountPerJob:        10,
	}
	created, err := svc.CreateJob(api.CreateJobRequest{Prompt: "draw a dragon", Count: 2, Concurrency: 2})
	if err != nil {
		t.Fatalf("CreateJob returned error: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		got, err := svc.GetJob(created.JobID)
		if err != nil {
			t.Fatalf("GetJob returned error: %v", err)
		}
		if got.Status == "completed" {
			if got.Completed != 2 {
				t.Fatalf("completed = %d", got.Completed)
			}
			if got.Images[0].Path == "" || got.Images[1].Path == "" {
				t.Fatalf("images = %+v", got.Images)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("job did not complete in time")
}
