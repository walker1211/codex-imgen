package api

import (
	"net/http"
	"time"

	"github.com/lxzan/gws"
	"github.com/walker1211/codex-imgen/internal/notify"
)

type wsHandler struct {
	gws.BuiltinEventHandler
	jobID  string
	hub    *notify.WebSocketHub
	connID string
}

func (h *wsHandler) OnOpen(socket *gws.Conn) {
	if socket != nil {
		_ = socket.SetDeadline(time.Now().Add(30 * time.Second))
	}
	h.connID = h.hub.Add(h.jobID, socket)
}

func (h *wsHandler) OnClose(socket *gws.Conn, err error) {
	if h.connID != "" {
		h.hub.Remove(h.jobID, h.connID)
	}
}

func handleWebSocket(hub *notify.WebSocketHub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		jobID := r.URL.Query().Get("job_id")
		if jobID == "" {
			http.Error(w, "job_id is required", http.StatusBadRequest)
			return
		}
		if hub == nil {
			http.Error(w, "websocket hub is not configured", http.StatusServiceUnavailable)
			return
		}
		handler := &wsHandler{jobID: jobID, hub: hub}
		upgrader := gws.NewUpgrader(handler, &gws.ServerOption{})
		conn, err := upgrader.Upgrade(w, r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		go conn.ReadLoop()
	}
}
