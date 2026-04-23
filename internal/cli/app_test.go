package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/walker1211/codex-imgen/internal/result"
)

type stubEngine struct {
	result result.Result
	err    error
}

func (s stubEngine) RunSync(ctx context.Context, req SyncRequest) (result.Result, error) {
	return s.result, s.err
}

func TestAppRunPrintsMultiplePaths(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	app := App{
		Stdout: stdout,
		Stderr: stderr,
		Engine: stubEngine{result: result.Result{
			OK: true,
			Images: []result.ImageResult{
				{Index: 1, Path: "/tmp/1.png"},
				{Index: 2, Path: "/tmp/2.png"},
			},
		}},
	}

	exitCode := app.Run(context.Background(), []string{"--count", "2", "draw a dragon"})
	if exitCode != 0 {
		t.Fatalf("exitCode = %d", exitCode)
	}
	if got := stdout.String(); got != "/tmp/1.png\n/tmp/2.png\n" {
		t.Fatalf("stdout = %q", got)
	}
}

func TestAppRunPrintsHelp(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	app := App{Stdout: stdout, Stderr: stderr}

	exitCode := app.Run(context.Background(), []string{"--help"})
	if exitCode != 0 {
		t.Fatalf("exitCode = %d", exitCode)
	}
	if got := stdout.String(); got == "" {
		t.Fatal("expected help text")
	}
}

type stubServeRunner struct {
	called bool
}

func (s *stubServeRunner) Run() error {
	s.called = true
	return nil
}

func TestAppRunServeStartsServer(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	runner := &stubServeRunner{}
	app := App{Stdout: stdout, Stderr: stderr, ServerRunner: runner}

	exitCode := app.Run(context.Background(), []string{"serve"})
	if exitCode != 0 {
		t.Fatalf("exitCode = %d", exitCode)
	}
	if !runner.called {
		t.Fatal("expected server runner to be called")
	}
}

func TestAppRunJSONPrintsStructuredError(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	app := App{
		Stdout: stdout,
		Stderr: stderr,
		Engine: stubEngine{err: errors.New("codex exec failed: signal: killed; stderr: generation failed")},
	}

	exitCode := app.Run(context.Background(), []string{"--json", "draw a dragon"})
	if exitCode != 1 {
		t.Fatalf("exitCode = %d", exitCode)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
	var got result.Result
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not valid JSON: %v", err)
	}
	if got.OK {
		t.Fatalf("expected error result, got %+v", got)
	}
	if got.Error == "" {
		t.Fatalf("expected error message, got %+v", got)
	}
}
