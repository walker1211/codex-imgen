package cli

import (
	"bytes"
	"context"
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
