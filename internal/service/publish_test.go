package service

import (
	"context"
	"encoding/json"
	"errors"
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

type sequenceGenerateResult struct {
	result backend.GenerateResult
	err    error
}

type sequenceGenerator struct {
	mu      sync.Mutex
	results []sequenceGenerateResult
}

func (g *sequenceGenerator) Generate(ctx context.Context, req backend.GenerateRequest) (backend.GenerateResult, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if len(g.results) == 0 {
		return backend.GenerateResult{}, errors.New("unexpected generate call")
	}
	next := g.results[0]
	g.results = g.results[1:]
	return next.result, next.err
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

func waitForJobEvent(t *testing.T, pub *capturePublisher, jobID string, eventType string) notify.Event {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		events := pub.Events()
		for i := len(events) - 1; i >= 0; i-- {
			event := events[i]
			if event.JobID == jobID && event.Type == eventType {
				return event
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("event %s for job %s was not published", eventType, jobID)
	return notify.Event{}
}

func assertJobEventPayload(t *testing.T, event notify.Event, status string, completed int, count int) {
	t.Helper()
	var payload struct {
		Status    string `json:"status"`
		Completed int    `json:"completed"`
		Count     int    `json:"count"`
	}
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if payload.Status != status || payload.Completed != completed || payload.Count != count {
		t.Fatalf("payload = %+v", payload)
	}
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

func TestServicePublishesPartialSuccessJobEventPayload(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "imgen.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer s.Close()

	pub := &capturePublisher{}
	gen := &sequenceGenerator{results: []sequenceGenerateResult{
		{result: backend.GenerateResult{Path: "/tmp/success.png", URI: "file:///tmp/success.png"}},
		{err: errors.New("codex failed")},
	}}
	svc := Service{
		Store:                 s,
		Generator:             gen,
		PromptPrefix:          "$imagegen",
		DefaultJobConcurrency: 1,
		MaxJobConcurrency:     10,
		MaxCountPerJob:        10,
		MaxAttempts:           1,
		Publisher:             pub,
	}
	created, err := svc.CreateJob(api.CreateJobRequest{Prompt: "draw a dragon", Count: 2, Concurrency: 1})
	if err != nil {
		t.Fatalf("CreateJob returned error: %v", err)
	}

	event := waitForJobEvent(t, pub, created.JobID, "job.partial_success")
	if event.Status != "partial_success" {
		t.Fatalf("status = %q", event.Status)
	}
	assertJobEventPayload(t, event, "partial_success", 1, 2)
}

func TestServicePublishesFailedJobEventPayload(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "imgen.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer s.Close()

	pub := &capturePublisher{}
	svc := Service{
		Store:                 s,
		Generator:             &alwaysFailGenerator{},
		PromptPrefix:          "$imagegen",
		DefaultJobConcurrency: 1,
		MaxJobConcurrency:     10,
		MaxCountPerJob:        10,
		MaxAttempts:           1,
		Publisher:             pub,
	}
	created, err := svc.CreateJob(api.CreateJobRequest{Prompt: "draw a dragon", Count: 1, Concurrency: 1})
	if err != nil {
		t.Fatalf("CreateJob returned error: %v", err)
	}

	event := waitForJobEvent(t, pub, created.JobID, "job.failed")
	if event.Status != "failed" {
		t.Fatalf("status = %q", event.Status)
	}
	assertJobEventPayload(t, event, "failed", 0, 1)
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
