package api

import (
	"net/http/httptest"
	"strings"
	"testing"
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
