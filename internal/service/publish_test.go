package service

import (
	"context"
	"encoding/json"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/walker1211/codex-imgen/internal/api"
	"github.com/walker1211/codex-imgen/internal/backend"
	"github.com/walker1211/codex-imgen/internal/notify"
	"github.com/walker1211/codex-imgen/internal/store"
)

type capturePublisher struct {
	mu     sync.Mutex
	events []notify.Event
}

func (p *capturePublisher) Publish(event notify.Event) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.events = append(p.events, event)
}

func (p *capturePublisher) Events() []notify.Event {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]notify.Event(nil), p.events...)
}

type immediateGenerator struct{}

func (immediateGenerator) Generate(ctx context.Context, req backend.GenerateRequest) (backend.GenerateResult, error) {
	return backend.GenerateResult{Path: "/tmp/test.png", URI: "file:///tmp/test.png"}, nil
}

func countEvents(events []notify.Event, eventType string) int {
	count := 0
	for _, event := range events {
		if event.Type == eventType {
			count++
		}
	}
	return count
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
	events := pub.Events()
	if len(events) == 0 {
		t.Fatal("expected published events")
	}
	last := events[len(events)-1]
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

func TestServicePublishesSingleJobCancelledEvent(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "imgen.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer s.Close()

	pub := &capturePublisher{}
	gen := &cancellableGenerator{started: make(chan struct{})}
	svc := Service{
		Store:                 s,
		Generator:             gen,
		PromptPrefix:          "$imagegen",
		DefaultJobConcurrency: 1,
		MaxJobConcurrency:     1,
		MaxCountPerJob:        1,
		Publisher:             pub,
	}
	created, err := svc.CreateJob(api.CreateJobRequest{Prompt: "draw a dragon", Count: 1, Concurrency: 1})
	if err != nil {
		t.Fatalf("CreateJob returned error: %v", err)
	}

	select {
	case <-gen.started:
	case <-time.After(2 * time.Second):
		t.Fatal("generator did not start in time")
	}
	if err := svc.CancelJob(created.JobID); err != nil {
		t.Fatalf("CancelJob returned error: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		events := pub.Events()
		if countEvents(events, "job.cancelled") == 1 {
			return
		}
		if countEvents(events, "job.cancelled") > 1 {
			t.Fatalf("job.cancelled events = %d", countEvents(events, "job.cancelled"))
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("job.cancelled events = %d", countEvents(pub.Events(), "job.cancelled"))
}
