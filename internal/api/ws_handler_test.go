package api

import (
	"testing"

	"github.com/lxzan/gws"
	"github.com/walker1211/codex-imgen/internal/notify"
)

func TestWSHandlerUsesStoredConnIDOnClose(t *testing.T) {
	hub := notify.NewWebSocketHub()
	conn := &gws.Conn{}
	connID := hub.Add("job_1", conn)
	h := &wsHandler{jobID: "job_1", hub: hub, connID: connID}
	if len(hub.Subscriptions.List("job_1")) != 1 {
		t.Fatalf("expected 1 subscription")
	}
	h.OnClose(conn, nil)
	if len(hub.Subscriptions.List("job_1")) != 0 {
		t.Fatalf("expected subscription to be removed")
	}
}
