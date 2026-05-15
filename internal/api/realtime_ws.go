package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lxzan/gws"
	"github.com/walker1211/codex-imgen/internal/backend"
	"github.com/walker1211/codex-imgen/internal/codex"
)

const (
	defaultRealtimeMaxSessions        = 4
	defaultRealtimeMaxItemsPerSession = 8
	defaultRealtimeMaxCountPerItem    = 1
	defaultRealtimeMaxItemTimeout     = 5 * time.Minute
)

var realtimeSessionCounter atomic.Uint64

type realtimeWSHandler struct {
	gws.BuiltinEventHandler
	options     RealtimeOptions
	state       *realtimeSharedState
	mu          sync.Mutex
	writeMu     sync.Mutex
	active      bool
	activeToken uint64
	closed      bool
	cancel      context.CancelFunc
}

type realtimeSharedState struct {
	sessions chan struct{}
}

type realtimeWorkerEvent struct {
	event     RealtimeEvent
	complete  bool
	succeeded bool
}

func (h *realtimeWSHandler) OnOpen(socket *gws.Conn) {}

func (h *realtimeWSHandler) OnClose(socket *gws.Conn, err error) {
	h.mu.Lock()
	h.closed = true
	cancel := h.cancel
	h.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (h *realtimeWSHandler) OnMessage(socket *gws.Conn, message *gws.Message) {
	defer message.Close()
	if socket == nil {
		return
	}
	var req RealtimeStartRequest
	if err := json.Unmarshal(message.Bytes(), &req); err != nil {
		_ = h.writeRealtimeEvent(socket, RealtimeEvent{Type: "session.failed", Error: err.Error(), Retryable: false})
		return
	}
	if req.Type != "generate.start" {
		_ = h.writeRealtimeEvent(socket, RealtimeEvent{Type: "session.failed", ClientRequestID: req.ClientRequestID, Error: "unsupported realtime frame type", Retryable: false})
		return
	}
	token, reason := h.beginSession()
	if reason != "" {
		retryable := reason == "realtime max sessions reached"
		_ = h.writeRealtimeEvent(socket, RealtimeEvent{Type: "session.failed", ClientRequestID: req.ClientRequestID, Error: reason, Retryable: retryable})
		return
	}
	go h.runSession(socket, req, token)
}

func (h *realtimeWSHandler) runSession(socket *gws.Conn, req RealtimeStartRequest, token uint64) {
	defer h.finishSession(token)
	sessionID := newRealtimeSessionID()
	limits := h.realtimeLimits()
	if err := validateRealtimeRequest(req, limits); err != nil {
		_ = h.writeRealtimeEvent(socket, RealtimeEvent{Type: "session.failed", SessionID: sessionID, ClientRequestID: req.ClientRequestID, Error: err.Error(), Retryable: false})
		return
	}
	maxConcurrency := len(req.Items)
	ctx, cancel := context.WithCancel(context.Background())
	h.setCancel(token, cancel)
	defer cancel()

	started := RealtimeEvent{
		Type:           "session.started",
		SessionID:      sessionID,
		TotalItems:     len(req.Items),
		MaxConcurrency: maxConcurrency,
	}
	if err := h.writeRealtimeEvent(socket, started); err != nil {
		return
	}
	if h.options.Generator == nil {
		_ = h.writeRealtimeEvent(socket, RealtimeEvent{Type: "session.completed", SessionID: sessionID, Failed: len(req.Items)})
		return
	}

	events := make(chan realtimeWorkerEvent, len(req.Items)*2+1)
	var wg sync.WaitGroup
	go func() {
		for itemIndex, item := range req.Items {
			select {
			case <-ctx.Done():
				wg.Wait()
				close(events)
				return
			default:
			}
			itemIndex := itemIndex
			item := item
			wg.Go(func() {
				h.runItem(ctx, req, limits, sessionID, itemIndex, item, events)
			})
		}
		wg.Wait()
		close(events)
	}()

	completed := 0
	failed := 0
	for workerEvent := range events {
		if workerEvent.complete {
			if workerEvent.succeeded {
				completed++
			} else {
				failed++
			}
		}
		if workerEvent.event.Type == "" {
			continue
		}
		if err := h.writeRealtimeEvent(socket, workerEvent.event); err != nil {
			cancel()
			for range events {
			}
			return
		}
	}
	_ = h.writeRealtimeEvent(socket, RealtimeEvent{
		Type:      "session.completed",
		SessionID: sessionID,
		Completed: completed,
		Failed:    failed,
	})
}

func (h *realtimeWSHandler) runItem(ctx context.Context, req RealtimeStartRequest, limits RealtimeOptions, sessionID string, itemIndex int, item RealtimeItem, events chan<- realtimeWorkerEvent) {
	if !sendRealtimeWorkerEvent(ctx, events, realtimeWorkerEvent{event: RealtimeEvent{Type: "item.started", SessionID: sessionID, ItemID: item.ID, Index: itemIndex}}) {
		return
	}
	count := item.Count
	if count <= 0 {
		count = 1
	}
	for index := range count {
		generateCtx, cancel := h.itemContext(ctx, req, limits)
		result, err := h.options.Generator.Generate(generateCtx, backend.GenerateRequest{
			Prompt:     codex.BuildPrompt(h.options.PromptPrelude, h.options.PromptPrefix, item.Prompt),
			Images:     item.Images,
			ImageIndex: index,
		})
		cancel()
		if err != nil {
			sendRealtimeWorkerEvent(ctx, events, realtimeWorkerEvent{event: RealtimeEvent{Type: "item.failed", SessionID: sessionID, ItemID: item.ID, Index: itemIndex, Error: realtimeItemFailureMessage(err), Retryable: true}, complete: true})
			return
		}
		if !sendRealtimeWorkerEvent(ctx, events, realtimeWorkerEvent{event: RealtimeEvent{Type: "image.completed", SessionID: sessionID, ItemID: item.ID, Index: index, Path: result.Path, URI: result.URI, MIME: mimeForPath(result.Path)}}) {
			return
		}
	}
	sendRealtimeWorkerEvent(ctx, events, realtimeWorkerEvent{complete: true, succeeded: true})
}

func (h *realtimeWSHandler) beginSession() (uint64, string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.active || h.closed {
		return 0, "realtime session already active"
	}
	if !h.state.acquireSession() {
		return 0, "realtime max sessions reached"
	}
	h.active = true
	h.activeToken++
	return h.activeToken, ""
}

func (h *realtimeWSHandler) setCancel(token uint64, cancel context.CancelFunc) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.active && h.activeToken == token {
		if h.closed {
			cancel()
			return
		}
		h.cancel = cancel
	}
}

func (h *realtimeWSHandler) finishSession(token uint64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.active && h.activeToken == token {
		h.active = false
		h.cancel = nil
		h.state.releaseSession()
	}
}

func (h *realtimeWSHandler) writeRealtimeEvent(socket *gws.Conn, event RealtimeEvent) error {
	h.writeMu.Lock()
	defer h.writeMu.Unlock()
	return writeRealtimeEvent(socket, event)
}

func (h *realtimeWSHandler) itemContext(parent context.Context, req RealtimeStartRequest, limits RealtimeOptions) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, h.itemTimeout(req, limits))
}

func (h *realtimeWSHandler) itemTimeout(req RealtimeStartRequest, limits RealtimeOptions) time.Duration {
	timeout := limits.MaxItemTimeout
	if req.TimeoutMS > 0 {
		timeout = time.Duration(req.TimeoutMS) * time.Millisecond
	} else if h.options.DefaultItemTimeout > 0 {
		timeout = h.options.DefaultItemTimeout
	}
	if timeout > limits.MaxItemTimeout {
		return limits.MaxItemTimeout
	}
	return timeout
}

func (h *realtimeWSHandler) maxItemTimeout() time.Duration {
	if h.options.MaxItemTimeout > 0 {
		return h.options.MaxItemTimeout
	}
	if h.options.DefaultItemTimeout > 0 {
		return h.options.DefaultItemTimeout
	}
	return defaultRealtimeMaxItemTimeout
}

func (h *realtimeWSHandler) realtimeLimits() RealtimeOptions {
	limits := h.options
	if limits.MaxItemsPerSession <= 0 {
		limits.MaxItemsPerSession = defaultRealtimeMaxItemsPerSession
	}
	if limits.MaxCountPerItem <= 0 {
		limits.MaxCountPerItem = defaultRealtimeMaxCountPerItem
	}
	if limits.MaxItemTimeout <= 0 {
		limits.MaxItemTimeout = h.maxItemTimeout()
	}
	return limits
}

func validateRealtimeRequest(req RealtimeStartRequest, limits RealtimeOptions) error {
	if req.TimeoutMS > 0 && time.Duration(req.TimeoutMS)*time.Millisecond > limits.MaxItemTimeout {
		return fmt.Errorf("realtime timeout_ms exceeds limit: %s > %s", time.Duration(req.TimeoutMS)*time.Millisecond, limits.MaxItemTimeout)
	}
	if len(req.Items) == 0 {
		return fmt.Errorf("realtime items must not be empty")
	}
	if len(req.Items) > limits.MaxItemsPerSession {
		return fmt.Errorf("too many realtime items: %d > %d", len(req.Items), limits.MaxItemsPerSession)
	}
	for _, item := range req.Items {
		count := item.Count
		if count <= 0 {
			count = 1
		}
		if count > limits.MaxCountPerItem {
			return fmt.Errorf("realtime item %q count exceeds limit: %d > %d", item.ID, count, limits.MaxCountPerItem)
		}
	}
	return nil
}

func newRealtimeSharedState(options RealtimeOptions) *realtimeSharedState {
	maxSessions := options.MaxSessions
	if maxSessions <= 0 {
		maxSessions = defaultRealtimeMaxSessions
	}
	return &realtimeSharedState{sessions: make(chan struct{}, maxSessions)}
}

func (s *realtimeSharedState) acquireSession() bool {
	if s == nil {
		return true
	}
	select {
	case s.sessions <- struct{}{}:
		return true
	default:
		return false
	}
}

func (s *realtimeSharedState) releaseSession() {
	if s == nil {
		return
	}
	select {
	case <-s.sessions:
	default:
	}
}

func sendRealtimeWorkerEvent(ctx context.Context, events chan<- realtimeWorkerEvent, event realtimeWorkerEvent) bool {
	select {
	case events <- event:
		return true
	case <-ctx.Done():
		return false
	}
}

func realtimeItemFailureMessage(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return "generation timed out"
	}
	if errors.Is(err, context.Canceled) {
		return "generation cancelled"
	}
	return "generation failed"
}

func handleRealtimeWebSocket(options RealtimeOptions) http.HandlerFunc {
	state := newRealtimeSharedState(options)
	return func(w http.ResponseWriter, r *http.Request) {
		if !options.Enabled {
			http.Error(w, "realtime disabled", http.StatusServiceUnavailable)
			return
		}
		handler := &realtimeWSHandler{options: options, state: state}
		upgrader := gws.NewUpgrader(handler, &gws.ServerOption{})
		conn, err := upgrader.Upgrade(w, r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		go conn.ReadLoop()
	}
}

func (event RealtimeEvent) MarshalJSON() ([]byte, error) {
	return json.Marshal(realtimeEventPayload(event))
}

func writeRealtimeEvent(socket *gws.Conn, event RealtimeEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return socket.WriteMessage(gws.OpcodeText, data)
}

func realtimeEventPayload(event RealtimeEvent) map[string]any {
	payload := map[string]any{"type": event.Type}
	if event.SessionID != "" {
		payload["session_id"] = event.SessionID
	}
	switch event.Type {
	case "session.started":
		payload["total_items"] = event.TotalItems
		payload["max_concurrency"] = event.MaxConcurrency
	case "item.started":
		payload["item_id"] = event.ItemID
		payload["index"] = event.Index
	case "image.completed":
		payload["item_id"] = event.ItemID
		payload["index"] = event.Index
		payload["path"] = event.Path
		payload["uri"] = event.URI
		payload["mime"] = event.MIME
	case "item.failed":
		payload["item_id"] = event.ItemID
		payload["index"] = event.Index
		payload["error"] = event.Error
		payload["retryable"] = event.Retryable
	case "session.completed":
		payload["completed"] = event.Completed
		payload["failed"] = event.Failed
	case "session.failed":
		if event.ClientRequestID != "" {
			payload["client_request_id"] = event.ClientRequestID
		}
		payload["error"] = event.Error
		payload["retryable"] = event.Retryable
	}
	return payload
}

func newRealtimeSessionID() string {
	counter := realtimeSessionCounter.Add(1)
	return fmt.Sprintf("rt_%s_%s", strconv.FormatInt(time.Now().UnixNano(), 36), strconv.FormatUint(counter, 36))
}

func mimeForPath(path string) string {
	if path == "" {
		return ""
	}
	if value := mime.TypeByExtension(filepath.Ext(path)); value != "" {
		return value
	}
	return "application/octet-stream"
}
