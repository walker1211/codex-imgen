package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lxzan/gws"
	api "github.com/walker1211/codex-imgen/internal/api"
	"github.com/walker1211/codex-imgen/internal/backend"
	"github.com/walker1211/codex-imgen/internal/service"
	"github.com/walker1211/codex-imgen/internal/store"
)

type serveQueueBlockingGenerator struct {
	mu       sync.Mutex
	started  chan string
	releases map[string]chan struct{}
}

func newServeQueueBlockingGenerator(keys ...string) *serveQueueBlockingGenerator {
	releases := make(map[string]chan struct{}, len(keys))
	for _, key := range keys {
		releases[key] = make(chan struct{})
	}
	return &serveQueueBlockingGenerator{started: make(chan string, len(keys)), releases: releases}
}

func (g *serveQueueBlockingGenerator) Generate(ctx context.Context, req backend.GenerateRequest) (backend.GenerateResult, error) {
	key := g.keyForPrompt(req.Prompt)
	g.started <- key
	select {
	case <-g.releases[key]:
		path := fmt.Sprintf("/tmp/%s.png", key)
		return backend.GenerateResult{Path: path, URI: "file://" + path}, nil
	case <-ctx.Done():
		return backend.GenerateResult{}, ctx.Err()
	}
}

func (g *serveQueueBlockingGenerator) keyForPrompt(prompt string) string {
	g.mu.Lock()
	defer g.mu.Unlock()
	for key := range g.releases {
		if strings.Contains(prompt, key) {
			return key
		}
	}
	return prompt
}

func (g *serveQueueBlockingGenerator) release(key string) {
	close(g.releases[key])
}

type serveQueueRealtimeClientHandler struct {
	messages chan api.RealtimeEvent
}

func (h *serveQueueRealtimeClientHandler) OnOpen(socket *gws.Conn)             {}
func (h *serveQueueRealtimeClientHandler) OnClose(socket *gws.Conn, err error) {}
func (h *serveQueueRealtimeClientHandler) OnPing(socket *gws.Conn, payload []byte) {
	_ = socket.WritePong(payload)
}
func (h *serveQueueRealtimeClientHandler) OnPong(socket *gws.Conn, payload []byte) {}
func (h *serveQueueRealtimeClientHandler) OnMessage(socket *gws.Conn, message *gws.Message) {
	defer message.Close()
	var event api.RealtimeEvent
	if err := json.Unmarshal(message.Bytes(), &event); err != nil {
		panic(err)
	}
	h.messages <- event
}

func TestServeSubmitAndRealtimeShareGenerationQueue(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "imgen.db")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer st.Close()

	fake := newServeQueueBlockingGenerator("submit", "realtime")
	queued := backend.NewQueuedGenerator(fake, 1)
	svc := &service.Service{Store: st, Generator: queued, PromptPrefix: "$imagegen", RetryDelays: []time.Duration{time.Millisecond}, MaxAttempts: 1, DefaultJobConcurrency: 1, MaxJobConcurrency: 1, MaxCountPerJob: 1}
	server := httptest.NewServer(api.NewServerWithOptions(svc, api.ServerOptions{Realtime: api.RealtimeOptions{Enabled: true, Generator: queued, PromptPrefix: "$imagegen", DefaultItemTimeout: time.Second, MaxSessions: 2, MaxItemsPerSession: 2, MaxCountPerItem: 1}}))
	defer server.Close()

	client := Client{BaseURL: server.URL}
	created, err := client.CreateJob(context.Background(), "submit prompt", nil, 1, 1)
	if err != nil {
		t.Fatalf("CreateJob returned error: %v", err)
	}
	if created.JobID == "" {
		t.Fatal("expected job id")
	}
	assertServeQueueGeneratorStarted(t, fake.started, "submit")

	realtimeHandler := &serveQueueRealtimeClientHandler{messages: make(chan api.RealtimeEvent, 8)}
	conn, _, err := gws.NewClient(realtimeHandler, &gws.ClientOption{Addr: strings.Replace(server.URL, "http://", "ws://", 1) + "/v1/realtime/generate/ws"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}
	defer conn.NetConn().Close()
	go conn.ReadLoop()

	writeServeQueueRealtimeStart(t, conn, api.RealtimeStartRequest{
		Type:           "generate.start",
		MaxConcurrency: 1,
		Items:          []api.RealtimeItem{{ID: "realtime", Prompt: "realtime prompt", Count: 1}},
	})
	assertServeQueueRealtimeEventType(t, readServeQueueRealtimeEvent(t, realtimeHandler.messages), "session.started")
	assertServeQueueRealtimeEventType(t, readServeQueueRealtimeEvent(t, realtimeHandler.messages), "item.started")
	assertServeQueueNoGeneratorStart(t, fake.started)

	fake.release("submit")
	assertServeQueueGeneratorStarted(t, fake.started, "realtime")
	fake.release("realtime")
	assertServeQueueRealtimeEventType(t, readServeQueueRealtimeEvent(t, realtimeHandler.messages), "image.completed")
	assertServeQueueRealtimeEventType(t, readServeQueueRealtimeEvent(t, realtimeHandler.messages), "session.completed")
}

func writeServeQueueRealtimeStart(t *testing.T, conn *gws.Conn, req api.RealtimeStartRequest) {
	t.Helper()
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	if err := conn.WriteMessage(gws.OpcodeText, data); err != nil {
		t.Fatalf("WriteMessage returned error: %v", err)
	}
}

func readServeQueueRealtimeEvent(t *testing.T, events <-chan api.RealtimeEvent) api.RealtimeEvent {
	t.Helper()
	select {
	case event := <-events:
		return event
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for realtime event")
		return api.RealtimeEvent{}
	}
}

func assertServeQueueRealtimeEventType(t *testing.T, event api.RealtimeEvent, want string) {
	t.Helper()
	if event.Type != want {
		t.Fatalf("event type = %q, want %q (event=%+v)", event.Type, want, event)
	}
}

func assertServeQueueGeneratorStarted(t *testing.T, started <-chan string, want string) {
	t.Helper()
	select {
	case got := <-started:
		if got != want {
			t.Fatalf("generator started = %q, want %q", got, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for generator start %q", want)
	}
}

func assertServeQueueNoGeneratorStart(t *testing.T, started <-chan string) {
	t.Helper()
	select {
	case got := <-started:
		t.Fatalf("unexpected generator start %q", got)
	case <-time.After(100 * time.Millisecond):
	}
}
