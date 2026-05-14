package api

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRealtimeStartRequestJSONTags(t *testing.T) {
	payload := RealtimeStartRequest{
		Type:            "generate.start",
		ClientRequestID: "client-123",
		Items: []RealtimeItem{{
			ID:     "item-1",
			Prompt: "draw a dragon",
			Images: []string{"/tmp/input.png"},
			Count:  2,
		}},
		MaxConcurrency: 2,
		TimeoutMS:      30000,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	body := string(data)
	for _, field := range []string{"type", "client_request_id", "items", "id", "prompt", "images", "count", "max_concurrency", "timeout_ms"} {
		if !strings.Contains(body, field) {
			t.Fatalf("marshaled body %q does not contain %q", body, field)
		}
	}
	for _, field := range []string{"ClientRequestID", "MaxConcurrency", "TimeoutMS", "ID", "Prompt", "Images", "Count"} {
		if strings.Contains(body, field) {
			t.Fatalf("marshaled body %q unexpectedly contains Go field %q", body, field)
		}
	}
}

func TestRealtimeEventJSONTags(t *testing.T) {
	payload := RealtimeEvent{
		Type:            "image.completed",
		SessionID:       "session-1",
		ClientRequestID: "client-123",
		ItemID:          "item-1",
		Index:           0,
		Path:            "/tmp/out.png",
		URI:             "file:///tmp/out.png",
		MIME:            "image/png",
		Error:           "backend failed",
		Retryable:       true,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	body := string(data)
	for _, field := range []string{"session_id", "item_id", "index", "path", "uri", "mime"} {
		if !strings.Contains(body, field) {
			t.Fatalf("marshaled body %q does not contain %q", body, field)
		}
	}
	for _, field := range []string{"client_request_id", "error", "retryable", "total_items", "max_concurrency", "image_uri", "image_index"} {
		if strings.Contains(body, field) {
			t.Fatalf("marshaled body %q unexpectedly contains non-spec field %q", body, field)
		}
	}
}
