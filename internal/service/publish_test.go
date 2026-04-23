package service

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/walker1211/codex-imgen/internal/api"
	"github.com/walker1211/codex-imgen/internal/backend"
	"github.com/walker1211/codex-imgen/internal/notify"
	"github.com/walker1211/codex-imgen/internal/store"
)

type capturePublisher struct {
	events []notify.Event
}

func (p *capturePublisher) Publish(event notify.Event) {
	p.events = append(p.events, event)
}

type immediateGenerator struct{}

func (immediateGenerator) Generate(ctx context.Context, req backend.GenerateRequest) (backend.GenerateResult, error) {
	return backend.GenerateResult{Path: "/tmp/test.png", URI: "file:///tmp/test.png"}, nil
}

func TestServicePublishesCompletionEvents(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "imgen.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer s.Close()

	pub := &capturePublisher{}
	svc := Service{
		Store:                 s,
		Generator:             immediateGenerator{},
		PromptPrefix:          "$imagegen",
		DefaultJobConcurrency: 1,
		MaxJobConcurrency:     10,
		MaxCountPerJob:        10,
		Publisher:             pub,
	}
	created, err := svc.CreateJob(api.CreateJobRequest{Prompt: "draw a dragon", Count: 1, Concurrency: 1})
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
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(pub.events) == 0 {
		t.Fatal("expected published events")
	}
	last := pub.events[len(pub.events)-1]
	if last.Type != "job.completed" {
		t.Fatalf("type = %q", last.Type)
	}
	if last.JobID != created.JobID {
		t.Fatalf("job id = %q", last.JobID)
	}
	if len(last.Payload) == 0 {
		t.Fatal("expected payload")
	}
	var payload map[string]any
	if err := json.Unmarshal(last.Payload, &payload); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
}
