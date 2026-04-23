package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestServerRegistersWebSocketRoute(t *testing.T) {
	server := NewServer(stubService{})
	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	resp := httptest.NewRecorder()

	server.ServeHTTP(resp, req)
	if resp.Code == http.StatusNotFound {
		t.Fatalf("expected /ws to be registered")
	}
}
