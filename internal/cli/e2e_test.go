package cli

import (
	"bytes"
	"context"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	api "github.com/walker1211/codex-imgen/internal/api"
	"github.com/walker1211/codex-imgen/internal/backend"
	"github.com/walker1211/codex-imgen/internal/service"
	"github.com/walker1211/codex-imgen/internal/store"
)

type e2eGenerator struct{}

func (e2eGenerator) Generate(ctx context.Context, req backend.GenerateRequest) (backend.GenerateResult, error) {
	return backend.GenerateResult{Path: "/tmp/test.png", URI: "file:///tmp/test.png"}, nil
}

func TestCLIEndToEndServiceCommands(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "imgen.db")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer st.Close()

	svc := &service.Service{Store: st, Generator: e2eGenerator{}, PromptPrefix: "$imagegen", DefaultJobConcurrency: 2, MaxJobConcurrency: 10, MaxCountPerJob: 10}
	handler := api.NewServer(svc)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen returned error: %v", err)
	}
	defer ln.Close()
	server := &http.Server{Handler: handler}
	go server.Serve(ln)
	defer server.Close()

	baseURL := "http://" + ln.Addr().String()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	app := App{Stdout: stdout, Stderr: stderr, Client: &Client{BaseURL: baseURL}}

	code := app.Run(context.Background(), []string{"submit", "--count", "2", "draw a dragon"})
	if code != 0 {
		t.Fatalf("submit code = %d", code)
	}
	jobID := strings.TrimSpace(stdout.String())
	if jobID == "" {
		t.Fatal("expected job id")
	}

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		stdout.Reset()
		stderr.Reset()
		code = app.Run(context.Background(), []string{"status", jobID})
		if code != 0 {
			t.Fatalf("status code = %d stderr=%q", code, stderr.String())
		}
		if strings.Contains(stdout.String(), "/tmp/test.png") {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !strings.Contains(stdout.String(), "/tmp/test.png") {
		t.Fatalf("status output = %q", stdout.String())
	}

	stdout.Reset()
	code = app.Run(context.Background(), []string{"list"})
	if code != 0 || !strings.Contains(stdout.String(), jobID) {
		t.Fatalf("list output = %q", stdout.String())
	}

	stdout.Reset()
	code = app.Run(context.Background(), []string{"cancel", jobID})
	if code != 0 || strings.TrimSpace(stdout.String()) != jobID {
		t.Fatalf("cancel output = %q", stdout.String())
	}
}
