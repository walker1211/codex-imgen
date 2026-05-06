package api_test

import (
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/lxzan/gws"
	api "github.com/walker1211/codex-imgen/internal/api"
	"github.com/walker1211/codex-imgen/internal/notify"
)

type wsClientHandler struct {
	messages chan string
}

func (h *wsClientHandler) OnOpen(socket *gws.Conn)                 {}
func (h *wsClientHandler) OnClose(socket *gws.Conn, err error)     {}
func (h *wsClientHandler) OnPing(socket *gws.Conn, payload []byte) { _ = socket.WritePong(payload) }
func (h *wsClientHandler) OnPong(socket *gws.Conn, payload []byte) {}
func (h *wsClientHandler) OnMessage(socket *gws.Conn, message *gws.Message) {
	defer message.Close()
	h.messages <- string(message.Bytes())
}

type stubService struct{}

func (stubService) CreateJob(req api.CreateJobRequest) (api.CreateJobResult, error) {
	return api.CreateJobResult{}, nil
}
func (stubService) GetJob(jobID string) (api.JobStatus, error)   { return api.JobStatus{}, nil }
func (stubService) ListJobs(limit int) ([]api.JobSummary, error) { return nil, nil }
func (stubService) CancelJob(jobID string) error                 { return nil }

func TestWebSocketHubDeliversPublishedEvent(t *testing.T) {
	hub := notify.NewWebSocketHub()
	server := api.NewServerWithNotifier(stubService{}, hub)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen returned error: %v", err)
	}
	defer ln.Close()
	httpServer := &http.Server{Handler: server}
	go httpServer.Serve(ln)
	defer httpServer.Close()

	handler := &wsClientHandler{messages: make(chan string, 1)}
	conn, _, err := gws.NewClient(handler, &gws.ClientOption{Addr: "ws://" + ln.Addr().String() + "/ws?job_id=job_1"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}
	defer conn.NetConn().Close()
	go conn.ReadLoop()

	time.Sleep(100 * time.Millisecond)
	hub.Publish(notify.Event{Type: "job.completed", JobID: "job_1", Status: "completed"})

	select {
	case msg := <-handler.messages:
		if msg == "" {
			t.Fatal("expected non-empty message")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for websocket message")
	}
}
