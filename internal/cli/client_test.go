package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	api "github.com/walker1211/codex-imgen/internal/api"
)

func TestClientSubmitCommandPrintsJobID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"data": map[string]any{
				"job_id": "job_123",
				"status": "queued",
			},
		})
	}))
	defer server.Close()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	app := App{Stdout: stdout, Stderr: stderr, Client: &Client{BaseURL: server.URL}}
	code := app.Run(context.Background(), []string{"submit", "--count", "4", "draw a dragon"})
	if code != 0 {
		t.Fatalf("code = %d", code)
	}
	if got := stdout.String(); got == "" {
		t.Fatal("expected output")
	}
}

func TestClientStatusCommandPrintsJobStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"data": map[string]any{
				"job_id":    "job_123",
				"status":    "completed",
				"count":     1,
				"completed": 1,
				"failed":    0,
				"images":    []map[string]any{{"index": 1, "status": "done", "path": "/tmp/1.png"}},
			},
		})
	}))
	defer server.Close()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	app := App{Stdout: stdout, Stderr: stderr, Client: &Client{BaseURL: server.URL}}
	code := app.Run(context.Background(), []string{"status", "job_123"})
	if code != 0 {
		t.Fatalf("code = %d", code)
	}
	if got := stdout.String(); got != "/tmp/1.png\n" {
		t.Fatalf("stdout = %q", got)
	}
}

func TestClientStatusCommandPrintsJobStateWithoutPaths(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"data": map[string]any{
				"job_id":    "job_123",
				"status":    "running",
				"count":     2,
				"completed": 0,
				"failed":    0,
				"images":    []map[string]any{{"index": 1, "status": "running"}, {"index": 2, "status": "queued"}},
			},
		})
	}))
	defer server.Close()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	app := App{Stdout: stdout, Stderr: stderr, Client: &Client{BaseURL: server.URL}}
	code := app.Run(context.Background(), []string{"status", "job_123"})
	if code != 0 {
		t.Fatalf("code = %d", code)
	}
	if got := stdout.String(); got != "job_123 running\n" {
		t.Fatalf("stdout = %q", got)
	}
}

func TestClientListCommandPrintsJobs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":   true,
			"data": []map[string]any{{"job_id": "job_1", "status": "queued", "count": 2, "completed": 0, "failed": 0}},
		})
	}))
	defer server.Close()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	app := App{Stdout: stdout, Stderr: stderr, Client: &Client{BaseURL: server.URL}}
	code := app.Run(context.Background(), []string{"list"})
	if code != 0 {
		t.Fatalf("code = %d", code)
	}
	if got := stdout.String(); got == "" {
		t.Fatal("expected output")
	}
}

func TestClientCancelCommandPrintsJobID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":   true,
			"data": map[string]any{"job_id": "job_1", "status": "cancelled"},
		})
	}))
	defer server.Close()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	app := App{Stdout: stdout, Stderr: stderr, Client: &Client{BaseURL: server.URL}}
	code := app.Run(context.Background(), []string{"cancel", "job_1"})
	if code != 0 {
		t.Fatalf("code = %d", code)
	}
	if got := stdout.String(); got != "job_1\n" {
		t.Fatalf("stdout = %q", got)
	}
}

func TestClientSubmitJSONCommandPrintsEnvelope(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"data": map[string]any{
				"job_id": "job_123",
				"status": "queued",
			},
		})
	}))
	defer server.Close()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	app := App{Stdout: stdout, Stderr: stderr, Client: &Client{BaseURL: server.URL}}
	code := app.Run(context.Background(), []string{"submit", "--json", "--count", "4", "draw a dragon"})
	if code != 0 {
		t.Fatalf("code = %d", code)
	}
	if got := stdout.String(); got == "job_123\n" || got == "" {
		t.Fatalf("stdout = %q", got)
	}
}

func TestClientStatusCommandPrintsErrorForMissingJob(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":    false,
			"error": map[string]any{"code": "not_found", "message": "job not found"},
		})
	}))
	defer server.Close()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	app := App{Stdout: stdout, Stderr: stderr, Client: &Client{BaseURL: server.URL}}
	code := app.Run(context.Background(), []string{"status", "missing"})
	if code != 1 {
		t.Fatalf("code = %d", code)
	}
	if got := stderr.String(); got == "" {
		t.Fatal("expected stderr output")
	}
}

func TestClientSubmitRequestIncludesImages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req api.CreateJobRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("Decode returned error: %v", err)
		}
		if len(req.Images) != 2 {
			t.Fatalf("images = %v", req.Images)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "data": map[string]any{"job_id": "job_123", "status": "queued"}})
	}))
	defer server.Close()

	_, err := (&Client{BaseURL: server.URL}).CreateJob(context.Background(), "draw a dragon", []string{"/tmp/1.png", "/tmp/2.png"}, 1, 1)
	if err != nil {
		t.Fatalf("CreateJob returned error: %v", err)
	}
}

func TestClientSubmitStagesImagesIntoInputDir(t *testing.T) {
	source := filepath.Join(t.TempDir(), "ref.png")
	if err := os.WriteFile(source, []byte("png"), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	inputDir := t.TempDir()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req api.CreateJobRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("Decode returned error: %v", err)
		}
		if len(req.Images) != 1 {
			t.Fatalf("images = %v", req.Images)
		}
		if !filepath.IsLocal(req.Images[0]) {
			t.Fatalf("image path should be relative to input dir: %v", req.Images)
		}
		if _, err := os.Stat(filepath.Join(inputDir, req.Images[0])); err != nil {
			t.Fatalf("staged image missing: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "data": map[string]any{"job_id": "job_123", "status": "queued"}})
	}))
	defer server.Close()

	_, err := (&Client{BaseURL: server.URL, ImageInputDir: inputDir}).CreateJob(context.Background(), "draw a dragon", []string{source}, 1, 1)
	if err != nil {
		t.Fatalf("CreateJob returned error: %v", err)
	}
}
