package notify

import (
	"testing"

	"github.com/lxzan/gws"
)

func TestWebSocketHubAddAndRemoveUseSameConnID(t *testing.T) {
	hub := NewWebSocketHub()
	conn := &gws.Conn{}
	connID := hub.Add("job_1", conn)
	if len(hub.Subscriptions.List("job_1")) != 1 {
		t.Fatalf("expected 1 subscription")
	}
	hub.Remove("job_1", connID)
	if len(hub.Subscriptions.List("job_1")) != 0 {
		t.Fatalf("expected subscription to be removed")
	}
}
