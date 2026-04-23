package service

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	api "github.com/walker1211/codex-imgen/internal/api"
	"github.com/walker1211/codex-imgen/internal/backend"
	"github.com/walker1211/codex-imgen/internal/store"
)

type blockingGenerator struct {
	started chan struct{}
	release chan struct{}
}

func (g *blockingGenerator) Generate(ctx context.Context, req backend.GenerateRequest) (backend.GenerateResult, error) {
	close(g.started)
	<-g.release
	return backend.GenerateResult{Path: "/tmp/test.png", URI: "file:///tmp/test.png"}, nil
}

func TestServiceRefreshesStartedAtWhenTaskBeginsRunning(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "imgen.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer s.Close()

	gen := &blockingGenerator{started: make(chan struct{}), release: make(chan struct{})}
	svc := Service{Store: s, Generator: gen, PromptPrefix: "$imagegen", DefaultJobConcurrency: 1, MaxJobConcurrency: 10, MaxCountPerJob: 10}
	created, err := svc.CreateJob(api.CreateJobRequest{Prompt: "draw a dragon", Count: 1, Concurrency: 1})
	if err != nil {
		t.Fatalf("CreateJob returned error: %v", err)
	}
	<-gen.started

	_, images, err := s.GetJob(context.Background(), created.JobID)
	if err != nil {
		t.Fatalf("GetJob returned error: %v", err)
	}
	if images[0].StartedAt.IsZero() {
		t.Fatal("expected started_at to be refreshed when task enters running")
	}
	close(gen.release)
	_ = time.Second
}
