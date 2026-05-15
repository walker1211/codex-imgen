package api

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lxzan/gws"
	"github.com/walker1211/codex-imgen/internal/backend"
)

type realtimeClientHandler struct {
	messages chan RealtimeEvent
}

func (h *realtimeClientHandler) OnOpen(socket *gws.Conn)             {}
func (h *realtimeClientHandler) OnClose(socket *gws.Conn, err error) {}
func (h *realtimeClientHandler) OnPing(socket *gws.Conn, payload []byte) {
	_ = socket.WritePong(payload)
}
func (h *realtimeClientHandler) OnPong(socket *gws.Conn, payload []byte) {}
func (h *realtimeClientHandler) OnMessage(socket *gws.Conn, message *gws.Message) {
	defer message.Close()
	var event RealtimeEvent
	if err := json.Unmarshal(message.Bytes(), &event); err != nil {
		panic(err)
	}
	h.messages <- event
}

type realtimeRawClientHandler struct {
	messages chan map[string]json.RawMessage
}

func (h *realtimeRawClientHandler) OnOpen(socket *gws.Conn)             {}
func (h *realtimeRawClientHandler) OnClose(socket *gws.Conn, err error) {}
func (h *realtimeRawClientHandler) OnPing(socket *gws.Conn, payload []byte) {
	_ = socket.WritePong(payload)
}
func (h *realtimeRawClientHandler) OnPong(socket *gws.Conn, payload []byte) {}
func (h *realtimeRawClientHandler) OnMessage(socket *gws.Conn, message *gws.Message) {
	defer message.Close()
	var event map[string]json.RawMessage
	if err := json.Unmarshal(message.Bytes(), &event); err != nil {
		panic(err)
	}
	h.messages <- event
}

type panicCreateJobService struct {
	mu             sync.Mutex
	createJobCalls int
}

func (s *panicCreateJobService) CreateJob(req CreateJobRequest) (CreateJobResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.createJobCalls++
	panic("CreateJob must not be called by realtime websocket")
}
func (s *panicCreateJobService) GetJob(jobID string) (JobStatus, error)   { return JobStatus{}, nil }
func (s *panicCreateJobService) ListJobs(limit int) ([]JobSummary, error) { return nil, nil }
func (s *panicCreateJobService) CancelJob(jobID string) error             { return nil }
func (s *panicCreateJobService) calls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.createJobCalls
}

type blockingRealtimeGenerator struct {
	mu       sync.Mutex
	calls    []backend.GenerateRequest
	started  chan string
	releases map[string]chan generateOutcome
}

type generateOutcome struct {
	result backend.GenerateResult
	err    error
}

func newBlockingRealtimeGenerator(prompts ...string) *blockingRealtimeGenerator {
	releases := make(map[string]chan generateOutcome, len(prompts))
	for _, prompt := range prompts {
		releases[prompt] = make(chan generateOutcome, 1)
	}
	return &blockingRealtimeGenerator{started: make(chan string, len(prompts)), releases: releases}
}

func (g *blockingRealtimeGenerator) Generate(ctx context.Context, req backend.GenerateRequest) (backend.GenerateResult, error) {
	key := g.keyForPrompt(req.Prompt)
	g.mu.Lock()
	g.calls = append(g.calls, req)
	g.mu.Unlock()
	g.started <- key
	select {
	case outcome := <-g.releases[key]:
		return outcome.result, outcome.err
	case <-ctx.Done():
		return backend.GenerateResult{}, ctx.Err()
	}
}

func (g *blockingRealtimeGenerator) keyForPrompt(prompt string) string {
	for key := range g.releases {
		if strings.Contains(prompt, key) {
			return key
		}
	}
	return prompt
}

func (g *blockingRealtimeGenerator) release(prompt string, result backend.GenerateResult, err error) {
	g.releases[prompt] <- generateOutcome{result: result, err: err}
}

func (g *blockingRealtimeGenerator) callPrompts() []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	prompts := make([]string, 0, len(g.calls))
	for _, call := range g.calls {
		prompts = append(prompts, call.Prompt)
	}
	return prompts
}

type cancellableRealtimeGenerator struct {
	started   chan string
	cancelled chan string
	release   chan string
}

func newCancellableRealtimeGenerator() *cancellableRealtimeGenerator {
	return &cancellableRealtimeGenerator{
		started:   make(chan string, 4),
		cancelled: make(chan string, 4),
		release:   make(chan string, 4),
	}
}

func (g *cancellableRealtimeGenerator) Generate(ctx context.Context, req backend.GenerateRequest) (backend.GenerateResult, error) {
	prompt := req.Prompt
	g.started <- prompt
	select {
	case <-g.release:
		return backend.GenerateResult{Path: "/tmp/" + prompt + ".png", URI: "file:///tmp/" + prompt + ".png"}, nil
	case <-ctx.Done():
		g.cancelled <- prompt
		return backend.GenerateResult{}, ctx.Err()
	}
}

type deadlineCheckingRealtimeGenerator struct {
	deadlineRemaining chan time.Duration
}

func (g *deadlineCheckingRealtimeGenerator) Generate(ctx context.Context, req backend.GenerateRequest) (backend.GenerateResult, error) {
	deadline, ok := ctx.Deadline()
	if !ok {
		g.deadlineRemaining <- 0
	} else {
		g.deadlineRemaining <- time.Until(deadline)
	}
	<-ctx.Done()
	return backend.GenerateResult{}, ctx.Err()
}

func TestRealtimeWebSocketUsesBackendDirectlyAndStreamsImagesImmediately(t *testing.T) {
	service := &panicCreateJobService{}
	generator := newBlockingRealtimeGenerator("theme-1 prompt", "theme-2 prompt")
	server := NewServerWithOptions(service, ServerOptions{Realtime: RealtimeOptions{Enabled: true, Generator: generator, PromptPrefix: "$imagegen", PromptPrelude: "prelude", DefaultItemTimeout: time.Second}})
	conn, events, cleanup := dialRealtimeWebSocket(t, server)
	defer cleanup()

	writeRealtimeStart(t, conn, RealtimeStartRequest{
		Type:            "generate.start",
		ClientRequestID: "client-123",
		MaxConcurrency:  2,
		TimeoutMS:       1000,
		Items: []RealtimeItem{
			{ID: "theme-1", Prompt: "theme-1 prompt", Count: 1},
			{ID: "theme-2", Prompt: "theme-2 prompt", Count: 1},
		},
	})

	assertEventType(t, readRealtimeEvent(t, events), "session.started")
	started := []RealtimeEvent{readRealtimeEvent(t, events), readRealtimeEvent(t, events)}
	assertItemStartedIndexes(t, started, map[string]int{"theme-1": 0, "theme-2": 1})
	assertGeneratorStartedSet(t, generator.started, "theme-1 prompt", "theme-2 prompt")

	generator.release("theme-2 prompt", backend.GenerateResult{Path: "/tmp/theme-2.png", URI: "file:///tmp/theme-2.png"}, nil)
	event := readRealtimeEvent(t, events)
	assertEventType(t, event, "image.completed")
	if event.ItemID != "theme-2" || event.Index != 0 || event.Path != "/tmp/theme-2.png" || event.URI != "file:///tmp/theme-2.png" || event.MIME != "image/png" {
		t.Fatalf("theme-2 image.completed = %+v", event)
	}

	generator.release("theme-1 prompt", backend.GenerateResult{Path: "/tmp/theme-1.png", URI: "file:///tmp/theme-1.png"}, nil)
	event = readRealtimeEvent(t, events)
	assertEventType(t, event, "image.completed")
	if event.ItemID != "theme-1" {
		t.Fatalf("expected theme-1 image after releasing theme-1, got %+v", event)
	}

	event = readRealtimeEvent(t, events)
	assertEventType(t, event, "session.completed")
	if event.Completed != 2 || event.Failed != 0 {
		t.Fatalf("session.completed summary = %+v", event)
	}
	if service.calls() != 0 {
		t.Fatalf("CreateJob calls = %d, want 0", service.calls())
	}
	for _, prompt := range generator.callPrompts() {
		if !strings.Contains(prompt, "prelude") || !strings.Contains(prompt, "$imagegen") {
			t.Fatalf("Generate prompt %q did not include prompt prelude and prefix", prompt)
		}
	}
}

func TestRealtimeWebSocketEmitsSpecJSONKeys(t *testing.T) {
	generator := newBlockingRealtimeGenerator("spec prompt")
	server := NewServerWithOptions(stubService{}, ServerOptions{Realtime: RealtimeOptions{Enabled: true, Generator: generator, MaxItemsPerSession: 8, MaxCountPerItem: 1, DefaultItemTimeout: time.Second}})
	conn, rawEvents, cleanup := dialRealtimeWebSocketRaw(t, server)
	defer cleanup()

	writeRealtimeStart(t, conn, RealtimeStartRequest{
		Type:            "generate.start",
		ClientRequestID: "client-should-not-leak",
		MaxConcurrency:  1,
		TimeoutMS:       1000,
		Items:           []RealtimeItem{{ID: "spec", Prompt: "spec prompt", Count: 1}},
	})

	assertRawEventKeys(t, readRawRealtimeEvent(t, rawEvents), "session.started", "type", "session_id", "total_items", "max_concurrency")
	assertRawEventKeys(t, readRawRealtimeEvent(t, rawEvents), "item.started", "type", "session_id", "item_id", "index")
	select {
	case <-generator.started:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for generator start")
	}
	generator.release("spec prompt", backend.GenerateResult{Path: "/tmp/spec.png", URI: "file:///tmp/spec.png"}, nil)
	assertRawEventKeys(t, readRawRealtimeEvent(t, rawEvents), "image.completed", "type", "session_id", "item_id", "index", "path", "uri", "mime")
	assertRawEventKeys(t, readRawRealtimeEvent(t, rawEvents), "session.completed", "type", "session_id", "completed", "failed")
}

func TestRealtimeWebSocketRejectsTimeoutAboveConfiguredMaxWithoutGenerating(t *testing.T) {
	generator := newBlockingRealtimeGenerator("slow")
	server := NewServerWithOptions(stubService{}, ServerOptions{Realtime: RealtimeOptions{Enabled: true, Generator: generator, DefaultItemTimeout: time.Second, MaxItemTimeout: time.Second}})
	conn, events, cleanup := dialRealtimeWebSocket(t, server)
	defer cleanup()

	writeRealtimeStart(t, conn, RealtimeStartRequest{
		Type:           "generate.start",
		MaxConcurrency: 1,
		TimeoutMS:      int((2 * time.Second) / time.Millisecond),
		Items:          []RealtimeItem{{ID: "slow", Prompt: "slow", Count: 1}},
	})

	event := readRealtimeEvent(t, events)
	assertEventType(t, event, "session.failed")
	if event.Error == "" || event.Retryable {
		t.Fatalf("session.failed = %+v", event)
	}
	assertNoGeneratorStart(t, generator.started)
}

func TestRealtimeWebSocketUsesMaxItemTimeoutWhenRequestAndDefaultTimeoutOmitted(t *testing.T) {
	generator := &deadlineCheckingRealtimeGenerator{deadlineRemaining: make(chan time.Duration, 1)}
	server := NewServerWithOptions(stubService{}, ServerOptions{Realtime: RealtimeOptions{
		Enabled:            true,
		Generator:          generator,
		DefaultItemTimeout: 0,
		MaxItemTimeout:     50 * time.Millisecond,
		MaxItemsPerSession: 8,
		MaxCountPerItem:    1,
	}})
	conn, events, cleanup := dialRealtimeWebSocket(t, server)
	defer cleanup()

	writeRealtimeStart(t, conn, RealtimeStartRequest{
		Type:           "generate.start",
		MaxConcurrency: 1,
		Items:          []RealtimeItem{{ID: "bounded", Prompt: "bounded", Count: 1}},
	})

	assertEventType(t, readRealtimeEvent(t, events), "session.started")
	assertEventType(t, readRealtimeEvent(t, events), "item.started")

	select {
	case remaining := <-generator.deadlineRemaining:
		if remaining <= 0 || remaining > time.Second {
			t.Fatalf("deadline remaining = %s, want bounded positive timeout", remaining)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for generator deadline check")
	}

	event := readRealtimeEvent(t, events)
	assertEventType(t, event, "item.failed")
	if event.Error != "generation timed out" {
		t.Fatalf("item.failed error = %q, want sanitized timeout", event.Error)
	}
}

func TestRealtimeWebSocketClampsDefaultItemTimeoutToConfiguredMax(t *testing.T) {
	generator := &deadlineCheckingRealtimeGenerator{deadlineRemaining: make(chan time.Duration, 1)}
	server := NewServerWithOptions(stubService{}, ServerOptions{Realtime: RealtimeOptions{
		Enabled:            true,
		Generator:          generator,
		DefaultItemTimeout: 250 * time.Millisecond,
		MaxItemTimeout:     50 * time.Millisecond,
		MaxItemsPerSession: 8,
		MaxCountPerItem:    1,
	}})
	conn, events, cleanup := dialRealtimeWebSocket(t, server)
	defer cleanup()

	writeRealtimeStart(t, conn, RealtimeStartRequest{
		Type:           "generate.start",
		MaxConcurrency: 1,
		Items:          []RealtimeItem{{ID: "bounded-default", Prompt: "bounded-default", Count: 1}},
	})

	assertEventType(t, readRealtimeEvent(t, events), "session.started")
	assertEventType(t, readRealtimeEvent(t, events), "item.started")

	select {
	case remaining := <-generator.deadlineRemaining:
		if remaining <= 0 || remaining > 100*time.Millisecond {
			t.Fatalf("deadline remaining = %s, want default timeout clamped to max", remaining)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for generator deadline check")
	}
}

func TestRealtimeHandlerCancelsSessionRegisteredAfterClose(t *testing.T) {
	handler := &realtimeWSHandler{}
	token, reason := handler.beginSession()
	if reason != "" {
		t.Fatalf("beginSession returned reason %q", reason)
	}
	handler.OnClose(nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	handler.setCancel(token, cancel)
	select {
	case <-ctx.Done():
	case <-time.After(100 * time.Millisecond):
		t.Fatal("session context registered after close was not cancelled")
	}
}

func TestRealtimeWebSocketRejectsEmptyItemsWithoutGenerating(t *testing.T) {
	generator := newBlockingRealtimeGenerator()
	server := NewServerWithOptions(stubService{}, ServerOptions{Realtime: RealtimeOptions{Enabled: true, Generator: generator, MaxItemsPerSession: 2, MaxCountPerItem: 1}})
	conn, events, cleanup := dialRealtimeWebSocket(t, server)
	defer cleanup()

	writeRealtimeStart(t, conn, RealtimeStartRequest{
		Type:           "generate.start",
		MaxConcurrency: 1,
		Items:          nil,
	})

	event := readRealtimeEvent(t, events)
	assertEventType(t, event, "session.failed")
	if event.Error == "" || event.Retryable {
		t.Fatalf("session.failed = %+v", event)
	}
	assertNoGeneratorStart(t, generator.started)
}

func TestRealtimeWebSocketRejectsTooManyItemsWithoutGenerating(t *testing.T) {
	generator := newBlockingRealtimeGenerator("one", "two", "three")
	server := NewServerWithOptions(stubService{}, ServerOptions{Realtime: RealtimeOptions{Enabled: true, Generator: generator, MaxItemsPerSession: 2, MaxCountPerItem: 1}})
	conn, events, cleanup := dialRealtimeWebSocket(t, server)
	defer cleanup()

	writeRealtimeStart(t, conn, RealtimeStartRequest{
		Type:           "generate.start",
		MaxConcurrency: 2,
		Items: []RealtimeItem{
			{ID: "one", Prompt: "one", Count: 1},
			{ID: "two", Prompt: "two", Count: 1},
			{ID: "three", Prompt: "three", Count: 1},
		},
	})

	event := readRealtimeEvent(t, events)
	assertEventType(t, event, "session.failed")
	if event.Error == "" || event.Retryable {
		t.Fatalf("session.failed = %+v", event)
	}
	assertNoGeneratorStart(t, generator.started)
}

func TestRealtimeWebSocketRejectsTooHighCountWithoutGenerating(t *testing.T) {
	generator := newBlockingRealtimeGenerator("too-many")
	server := NewServerWithOptions(stubService{}, ServerOptions{Realtime: RealtimeOptions{Enabled: true, Generator: generator, MaxItemsPerSession: 8, MaxCountPerItem: 1}})
	conn, events, cleanup := dialRealtimeWebSocket(t, server)
	defer cleanup()

	writeRealtimeStart(t, conn, RealtimeStartRequest{
		Type:           "generate.start",
		MaxConcurrency: 1,
		Items:          []RealtimeItem{{ID: "too-many", Prompt: "too-many", Count: 2}},
	})

	event := readRealtimeEvent(t, events)
	assertEventType(t, event, "session.failed")
	if event.Error == "" || event.Retryable {
		t.Fatalf("session.failed = %+v", event)
	}
	assertNoGeneratorStart(t, generator.started)
}

func TestRealtimeWebSocketQueuesAllItemsToInjectedGenerator(t *testing.T) {
	generator := newBlockingRealtimeGenerator("first", "second")
	server := NewServerWithOptions(stubService{}, ServerOptions{Realtime: RealtimeOptions{Enabled: true, Generator: backend.NewQueuedGenerator(generator, 10), MaxItemsPerSession: 8, MaxCountPerItem: 1, DefaultItemTimeout: time.Second}})
	conn, events, cleanup := dialRealtimeWebSocket(t, server)
	defer cleanup()

	writeRealtimeStart(t, conn, RealtimeStartRequest{
		Type:           "generate.start",
		MaxConcurrency: 1,
		TimeoutMS:      1000,
		Items: []RealtimeItem{
			{ID: "first", Prompt: "first", Count: 1},
			{ID: "second", Prompt: "second", Count: 1},
		},
	})

	event := readRealtimeEvent(t, events)
	assertEventType(t, event, "session.started")
	if event.MaxConcurrency != 2 {
		t.Fatalf("session.started MaxConcurrency = %d, want 2", event.MaxConcurrency)
	}
	assertItemStartedIndexes(t, []RealtimeEvent{readRealtimeEvent(t, events), readRealtimeEvent(t, events)}, map[string]int{"first": 0, "second": 1})
	assertGeneratorStartedSet(t, generator.started, "first", "second")

	generator.release("first", backend.GenerateResult{Path: "/tmp/first.png", URI: "file:///tmp/first.png"}, nil)
	generator.release("second", backend.GenerateResult{Path: "/tmp/second.png", URI: "file:///tmp/second.png"}, nil)
}

func TestRealtimeWebSocketCancelsBlockedGeneratorsAfterClientDisconnect(t *testing.T) {
	generator := newCancellableRealtimeGenerator()
	server := NewServerWithOptions(stubService{}, ServerOptions{Realtime: RealtimeOptions{Enabled: true, Generator: generator, MaxItemsPerSession: 8, MaxCountPerItem: 1, DefaultItemTimeout: time.Minute}})
	conn, events, cleanup := dialRealtimeWebSocket(t, server)
	defer cleanup()

	writeRealtimeStart(t, conn, RealtimeStartRequest{
		Type:           "generate.start",
		MaxConcurrency: 2,
		Items: []RealtimeItem{
			{ID: "first", Prompt: "first", Count: 1},
			{ID: "second", Prompt: "second", Count: 1},
		},
	})

	assertEventType(t, readRealtimeEvent(t, events), "session.started")
	assertItemStartedIndexes(t, []RealtimeEvent{readRealtimeEvent(t, events), readRealtimeEvent(t, events)}, map[string]int{"first": 0, "second": 1})
	for range 2 {
		select {
		case <-generator.started:
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for generator start")
		}
	}
	_ = conn.NetConn().Close()
	generator.release <- "first"
	select {
	case <-generator.cancelled:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for blocked generator cancellation")
	}
}

func TestRealtimeWebSocketRejectsSessionAboveConfiguredGlobalLimit(t *testing.T) {
	generator := newBlockingRealtimeGenerator("first", "second")
	server := NewServerWithOptions(stubService{}, ServerOptions{Realtime: RealtimeOptions{Enabled: true, Generator: generator, MaxSessions: 1, MaxItemsPerSession: 8, MaxCountPerItem: 1, DefaultItemTimeout: time.Minute}})
	firstConn, firstEvents, firstCleanup := dialRealtimeWebSocket(t, server)
	defer firstCleanup()
	secondConn, secondEvents, secondCleanup := dialRealtimeWebSocket(t, server)
	defer secondCleanup()

	writeRealtimeStart(t, firstConn, RealtimeStartRequest{
		Type:            "generate.start",
		ClientRequestID: "client-first",
		MaxConcurrency:  1,
		Items:           []RealtimeItem{{ID: "first", Prompt: "first", Count: 1}},
	})
	assertEventType(t, readRealtimeEvent(t, firstEvents), "session.started")
	assertEventType(t, readRealtimeEvent(t, firstEvents), "item.started")
	select {
	case prompt := <-generator.started:
		if prompt != "first" {
			t.Fatalf("first generator prompt = %q, want first", prompt)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for first generator start")
	}

	writeRealtimeStart(t, secondConn, RealtimeStartRequest{
		Type:            "generate.start",
		ClientRequestID: "client-second",
		MaxConcurrency:  1,
		Items:           []RealtimeItem{{ID: "second", Prompt: "second", Count: 1}},
	})

	event := readRealtimeEvent(t, secondEvents)
	assertEventType(t, event, "session.failed")
	if event.ClientRequestID != "client-second" || event.Error == "" || !event.Retryable {
		t.Fatalf("session.failed for max sessions = %+v", event)
	}
	assertNoGeneratorStart(t, generator.started)
}

func TestRealtimeWebSocketUsesInjectedGeneratorQueueAcrossSessions(t *testing.T) {
	generator := newBlockingRealtimeGenerator("first", "second")
	server := NewServerWithOptions(stubService{}, ServerOptions{Realtime: RealtimeOptions{Enabled: true, Generator: backend.NewQueuedGenerator(generator, 1), MaxSessions: 2, MaxItemsPerSession: 8, MaxCountPerItem: 1, DefaultItemTimeout: time.Minute}})
	firstConn, firstEvents, firstCleanup := dialRealtimeWebSocket(t, server)
	defer firstCleanup()
	secondConn, secondEvents, secondCleanup := dialRealtimeWebSocket(t, server)
	defer secondCleanup()

	writeRealtimeStart(t, firstConn, RealtimeStartRequest{
		Type:           "generate.start",
		MaxConcurrency: 1,
		Items:          []RealtimeItem{{ID: "first", Prompt: "first", Count: 1}},
	})
	assertEventType(t, readRealtimeEvent(t, firstEvents), "session.started")
	assertEventType(t, readRealtimeEvent(t, firstEvents), "item.started")
	select {
	case prompt := <-generator.started:
		if prompt != "first" {
			t.Fatalf("first generator prompt = %q, want first", prompt)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for first generator start")
	}

	writeRealtimeStart(t, secondConn, RealtimeStartRequest{
		Type:           "generate.start",
		MaxConcurrency: 1,
		Items:          []RealtimeItem{{ID: "second", Prompt: "second", Count: 1}},
	})
	assertEventType(t, readRealtimeEvent(t, secondEvents), "session.started")
	assertEventType(t, readRealtimeEvent(t, secondEvents), "item.started")
	assertNoGeneratorStart(t, generator.started)

	generator.release("first", backend.GenerateResult{Path: "/tmp/first.png", URI: "file:///tmp/first.png"}, nil)
	select {
	case prompt := <-generator.started:
		if prompt != "second" {
			t.Fatalf("second generator prompt = %q, want second", prompt)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for second generator start after first release")
	}
}

func TestRealtimeWebSocketRejectsSecondStartWhileSessionActive(t *testing.T) {
	generator := newBlockingRealtimeGenerator("first", "second")
	server := NewServerWithOptions(stubService{}, ServerOptions{Realtime: RealtimeOptions{Enabled: true, Generator: generator, MaxItemsPerSession: 8, MaxCountPerItem: 1, DefaultItemTimeout: time.Minute}})
	conn, events, cleanup := dialRealtimeWebSocket(t, server)
	defer cleanup()

	writeRealtimeStart(t, conn, RealtimeStartRequest{
		Type:            "generate.start",
		ClientRequestID: "client-first",
		MaxConcurrency:  1,
		Items:           []RealtimeItem{{ID: "first", Prompt: "first", Count: 1}},
	})
	assertEventType(t, readRealtimeEvent(t, events), "session.started")
	assertEventType(t, readRealtimeEvent(t, events), "item.started")
	select {
	case prompt := <-generator.started:
		if prompt != "first" {
			t.Fatalf("first generator prompt = %q, want first", prompt)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for first generator start")
	}

	writeRealtimeStart(t, conn, RealtimeStartRequest{
		Type:            "generate.start",
		ClientRequestID: "client-second",
		MaxConcurrency:  1,
		Items:           []RealtimeItem{{ID: "second", Prompt: "second", Count: 1}},
	})

	event := readRealtimeEvent(t, events)
	assertEventType(t, event, "session.failed")
	if event.ClientRequestID != "client-second" || event.Error == "" || event.Retryable {
		t.Fatalf("session.failed for second start = %+v", event)
	}
	assertNoGeneratorStart(t, generator.started)
}

func TestRealtimeWebSocketRejectsNonGenerateStartFrame(t *testing.T) {
	generator := newBlockingRealtimeGenerator("ignored prompt")
	server := NewServerWithOptions(stubService{}, ServerOptions{Realtime: RealtimeOptions{Enabled: true, Generator: generator, DefaultItemTimeout: time.Second}})
	conn, events, cleanup := dialRealtimeWebSocket(t, server)
	defer cleanup()

	writeRealtimeStart(t, conn, RealtimeStartRequest{
		Type:            "noop",
		ClientRequestID: "client-noop",
		MaxConcurrency:  1,
		TimeoutMS:       1000,
		Items: []RealtimeItem{
			{ID: "ignored", Prompt: "ignored prompt", Count: 1},
		},
	})

	event := readRealtimeEvent(t, events)
	if event.Type == "session.started" {
		t.Fatalf("unexpected session.started for unsupported frame type: %+v", event)
	}
	assertEventType(t, event, "session.failed")
	if event.Error == "" || event.Retryable {
		t.Fatalf("session.failed = %+v", event)
	}
	select {
	case prompt := <-generator.started:
		t.Fatalf("Generate was called for unsupported frame type with prompt %q", prompt)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestRealtimeWebSocketEmitsItemFailedAndSessionSummary(t *testing.T) {
	generator := newBlockingRealtimeGenerator("good prompt", "bad prompt")
	server := NewServerWithOptions(stubService{}, ServerOptions{Realtime: RealtimeOptions{Enabled: true, Generator: generator, DefaultItemTimeout: time.Second}})
	conn, events, cleanup := dialRealtimeWebSocket(t, server)
	defer cleanup()

	writeRealtimeStart(t, conn, RealtimeStartRequest{
		Type:            "generate.start",
		ClientRequestID: "client-456",
		MaxConcurrency:  2,
		TimeoutMS:       1000,
		Items: []RealtimeItem{
			{ID: "good", Prompt: "good prompt", Count: 1},
			{ID: "bad", Prompt: "bad prompt", Count: 1},
		},
	})

	assertEventType(t, readRealtimeEvent(t, events), "session.started")
	assertItemStartedIndexes(t, []RealtimeEvent{readRealtimeEvent(t, events), readRealtimeEvent(t, events)}, map[string]int{"good": 0, "bad": 1})
	assertGeneratorStartedSet(t, generator.started, "good prompt", "bad prompt")

	generator.release("bad prompt", backend.GenerateResult{}, errors.New("backend exploded --token secret --path /Users/private/out.png"))
	event := readRealtimeEvent(t, events)
	assertEventType(t, event, "item.failed")
	if event.ItemID != "bad" || event.Index != 1 || event.Error != "generation failed" || !event.Retryable {
		t.Fatalf("item.failed = %+v", event)
	}
	if strings.Contains(event.Error, "secret") || strings.Contains(event.Error, "/Users/private") {
		t.Fatalf("item.failed error leaked backend details: %q", event.Error)
	}

	generator.release("good prompt", backend.GenerateResult{Path: "/tmp/good.png", URI: "file:///tmp/good.png"}, nil)
	event = readRealtimeEvent(t, events)
	assertEventType(t, event, "image.completed")
	if event.ItemID != "good" || event.Path == "" || event.URI == "" || event.MIME == "" {
		t.Fatalf("image.completed = %+v", event)
	}

	event = readRealtimeEvent(t, events)
	assertEventType(t, event, "session.completed")
	if event.Completed != 1 || event.Failed != 1 {
		t.Fatalf("session.completed = %+v", event)
	}
}

func dialRealtimeWebSocket(t *testing.T, handler http.Handler) (*gws.Conn, <-chan RealtimeEvent, func()) {
	t.Helper()
	clientHandler := &realtimeClientHandler{messages: make(chan RealtimeEvent, 16)}
	conn, cleanup := dialRealtimeWebSocketWithHandler(t, handler, clientHandler)
	return conn, clientHandler.messages, cleanup
}

func dialRealtimeWebSocketRaw(t *testing.T, handler http.Handler) (*gws.Conn, <-chan map[string]json.RawMessage, func()) {
	t.Helper()
	clientHandler := &realtimeRawClientHandler{messages: make(chan map[string]json.RawMessage, 16)}
	conn, cleanup := dialRealtimeWebSocketWithHandler(t, handler, clientHandler)
	return conn, clientHandler.messages, cleanup
}

func dialRealtimeWebSocketWithHandler(t *testing.T, handler http.Handler, eventHandler gws.Event) (*gws.Conn, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen returned error: %v", err)
	}
	httpServer := &http.Server{Handler: handler}
	go func() { _ = httpServer.Serve(ln) }()

	conn, _, err := gws.NewClient(eventHandler, &gws.ClientOption{Addr: "ws://" + ln.Addr().String() + "/v1/realtime/generate/ws"})
	if err != nil {
		_ = httpServer.Close()
		_ = ln.Close()
		t.Fatalf("NewClient returned error: %v", err)
	}
	go conn.ReadLoop()

	cleanup := func() {
		_ = conn.NetConn().Close()
		_ = httpServer.Close()
		_ = ln.Close()
	}
	return conn, cleanup
}

func writeRealtimeStart(t *testing.T, conn *gws.Conn, req RealtimeStartRequest) {
	t.Helper()
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	if err := conn.WriteMessage(gws.OpcodeText, data); err != nil {
		t.Fatalf("WriteMessage returned error: %v", err)
	}
}

func readRealtimeEvent(t *testing.T, events <-chan RealtimeEvent) RealtimeEvent {
	t.Helper()
	select {
	case event := <-events:
		return event
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for realtime event")
		return RealtimeEvent{}
	}
}

func readRawRealtimeEvent(t *testing.T, events <-chan map[string]json.RawMessage) map[string]json.RawMessage {
	t.Helper()
	select {
	case event := <-events:
		return event
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for raw realtime event")
		return nil
	}
}

func assertEventType(t *testing.T, event RealtimeEvent, want string) {
	t.Helper()
	if event.Type != want {
		t.Fatalf("event type = %q, want %q (event=%+v)", event.Type, want, event)
	}
}

func assertRawEventKeys(t *testing.T, event map[string]json.RawMessage, eventType string, keys ...string) {
	t.Helper()
	var gotType string
	if err := json.Unmarshal(event["type"], &gotType); err != nil {
		t.Fatalf("raw event type unmarshal failed: %v event=%v", err, event)
	}
	if gotType != eventType {
		t.Fatalf("raw event type = %q, want %q event=%v", gotType, eventType, event)
	}
	want := map[string]bool{}
	for _, key := range keys {
		want[key] = true
	}
	if len(event) != len(want) {
		t.Fatalf("%s keys = %v, want exactly %v", eventType, rawEventKeys(event), keys)
	}
	for key := range event {
		if !want[key] {
			t.Fatalf("%s has unexpected key %q; keys=%v want=%v", eventType, key, rawEventKeys(event), keys)
		}
	}
}

func rawEventKeys(event map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(event))
	for key := range event {
		keys = append(keys, key)
	}
	return keys
}

func assertItemStartedIndexes(t *testing.T, events []RealtimeEvent, want map[string]int) {
	t.Helper()
	seen := map[string]int{}
	for _, event := range events {
		assertEventType(t, event, "item.started")
		seen[event.ItemID] = event.Index
	}
	for itemID, index := range want {
		if seenIndex, ok := seen[itemID]; !ok || seenIndex != index {
			t.Fatalf("item.started indexes = %v, want %q index %d", seen, itemID, index)
		}
	}
}

func assertNoGeneratorStart(t *testing.T, started <-chan string) {
	t.Helper()
	select {
	case prompt := <-started:
		t.Fatalf("unexpected generator start for prompt %q", prompt)
	case <-time.After(100 * time.Millisecond):
	}
}

func assertGeneratorStartedSet(t *testing.T, started <-chan string, wantA, wantB string) {
	t.Helper()
	seen := map[string]bool{}
	for range 2 {
		select {
		case key := <-started:
			seen[key] = true
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for generator starts; seen=%v", seen)
		}
	}
	if !seen[wantA] || !seen[wantB] {
		t.Fatalf("generator starts = %v, want %q and %q", seen, wantA, wantB)
	}
}
