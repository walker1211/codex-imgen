package api

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lxzan/gws"
)

func TestWebSocketRouteRejectsPlainHTTPWithUpgradeExpectation(t *testing.T) {
	server := NewServer(stubService{})
	req := httptest.NewRequest("GET", "/ws?job_id=job_1", nil)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	resp := httptest.NewRecorder()

	server.ServeHTTP(resp, req)
	if resp.Code == 501 {
		t.Fatalf("expected real websocket upgrade path, got placeholder response")
	}
	if strings.Contains(resp.Body.String(), "reserved") {
		t.Fatalf("body = %q", resp.Body.String())
	}
}

func TestRealtimeWebSocketRouteRejectsWhenDisabled(t *testing.T) {
	generator := newBlockingRealtimeGenerator("disabled")
	server := NewServerWithOptions(stubService{}, ServerOptions{Realtime: RealtimeOptions{Enabled: false, Generator: generator}})
	req := httptest.NewRequest("GET", "/v1/realtime/generate/ws", nil)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	resp := httptest.NewRecorder()

	server.ServeHTTP(resp, req)

	if resp.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d body=%q", resp.Code, http.StatusServiceUnavailable, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), "realtime disabled") {
		t.Fatalf("body = %q, want disabled response", resp.Body.String())
	}
	assertNoGeneratorStart(t, generator.started)
}

func TestRealtimeWebSocketRouteRejectsWhenDisabledWithoutGenerator(t *testing.T) {
	server := NewServerWithOptions(stubService{}, ServerOptions{Realtime: RealtimeOptions{Enabled: false}})
	req := httptest.NewRequest("GET", "/v1/realtime/generate/ws", nil)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	resp := httptest.NewRecorder()

	server.ServeHTTP(resp, req)

	if resp.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d body=%q", resp.Code, http.StatusServiceUnavailable, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), "realtime disabled") {
		t.Fatalf("body = %q, want disabled response", resp.Body.String())
	}
}

func TestRealtimeWebSocketRouteUpgradesWithoutJobID(t *testing.T) {
	server := NewServer(stubService{})
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen returned error: %v", err)
	}
	defer ln.Close()
	httpServer := &http.Server{Handler: server}
	go httpServer.Serve(ln)
	defer httpServer.Close()

	conn, _, err := gws.NewClient(&gws.BuiltinEventHandler{}, &gws.ClientOption{Addr: "ws://" + ln.Addr().String() + "/v1/realtime/generate/ws"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}
	defer conn.NetConn().Close()
}
