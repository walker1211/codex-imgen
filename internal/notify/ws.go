package notify

import (
	"encoding/json"
	"fmt"
	"sort"
	"sync"

	"github.com/lxzan/gws"
)

type Event struct {
	Type    string          `json:"type"`
	JobID   string          `json:"job_id"`
	Index   int             `json:"index,omitempty"`
	Path    string          `json:"path,omitempty"`
	Status  string          `json:"status,omitempty"`
	Message string          `json:"message,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

type SubscriptionStore struct {
	mu         sync.Mutex
	eventConns map[string]map[string]*gws.Conn
}

func NewSubscriptionStore() *SubscriptionStore {
	return &SubscriptionStore{eventConns: map[string]map[string]*gws.Conn{}}
}

func (s *SubscriptionStore) Add(jobID string, connID string, conn *gws.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.eventConns[jobID] == nil {
		s.eventConns[jobID] = map[string]*gws.Conn{}
	}
	s.eventConns[jobID][connID] = conn
}

func (s *SubscriptionStore) Remove(jobID string, connID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.eventConns[jobID] == nil {
		return
	}
	delete(s.eventConns[jobID], connID)
	if len(s.eventConns[jobID]) == 0 {
		delete(s.eventConns, jobID)
	}
}

func (s *SubscriptionStore) List(jobID string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var ids []string
	for id := range s.eventConns[jobID] {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func (s *SubscriptionStore) Connections(jobID string) []*gws.Conn {
	s.mu.Lock()
	defer s.mu.Unlock()
	var conns []*gws.Conn
	for _, conn := range s.eventConns[jobID] {
		if conn != nil {
			conns = append(conns, conn)
		}
	}
	return conns
}

type EventSink interface {
	Send(jobID string, event Event)
}

type Publisher struct {
	Subscriptions *SubscriptionStore
	Sink          EventSink
}

func (p Publisher) Publish(event Event) {
	if p.Subscriptions == nil || p.Sink == nil {
		return
	}
	if len(p.Subscriptions.List(event.JobID)) == 0 {
		return
	}
	p.Sink.Send(event.JobID, event)
}

type WebSocketHub struct {
	Subscriptions *SubscriptionStore
}

func NewWebSocketHub() *WebSocketHub {
	return &WebSocketHub{Subscriptions: NewSubscriptionStore()}
}

func (h *WebSocketHub) Add(jobID string, conn *gws.Conn) string {
	connID := fmt.Sprintf("%p", conn)
	h.Subscriptions.Add(jobID, connID, conn)
	return connID
}

func (h *WebSocketHub) Remove(jobID string, connID string) {
	h.Subscriptions.Remove(jobID, connID)
}

func (h *WebSocketHub) Send(jobID string, event Event) {
	data, err := json.Marshal(event)
	if err != nil {
		return
	}
	for _, conn := range h.Subscriptions.Connections(jobID) {
		_ = conn.WriteMessage(gws.OpcodeText, data)
	}
}

func (h *WebSocketHub) Publish(event Event) {
	Publisher{Subscriptions: h.Subscriptions, Sink: h}.Publish(event)
}
