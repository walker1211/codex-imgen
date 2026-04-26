package backend

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/walker1211/codex-imgen/internal/codex"
)

func TestBuiltinCodexGenerateOneImage(t *testing.T) {
	codexHome := t.TempDir()
	threadID := "019db517-05e0-77b2-aff1-a90de1fee1ea"
	imageDir := filepath.Join(codexHome, "generated_images", threadID)
	if err := os.MkdirAll(imageDir, 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(imageDir, "ig_test.png"), []byte("png"), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	backend := BuiltinCodex{
		Command:   filepath.Join("..", "..", "testdata", "codex-thread-success.sh"),
		CodexHome: codexHome,
	}
	result, err := backend.Generate(context.Background(), GenerateRequest{Prompt: "$imagegen draw a dragon"})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	if result.Path == "" {
		t.Fatal("expected path to be set")
	}
}

func TestBuiltinCodexGenerateWrapsCommandError(t *testing.T) {
	backend := BuiltinCodex{Command: filepath.Join("..", "..", "testdata", "codex-exit-9.sh")}

	_, err := backend.Generate(context.Background(), GenerateRequest{Prompt: "$imagegen draw a dragon"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "execution failed") {
		t.Fatalf("expected wrapped execution error, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "codex failed") {
		t.Fatalf("expected stderr summary in error, got %q", err.Error())
	}
}

func TestBuiltinCodexGenerateReportsDeadlineExceeded(t *testing.T) {
	backend := BuiltinCodex{
		Command: filepath.Join("..", "..", "testdata", "codex-thread-success.sh"),
		Timeout: time.Nanosecond,
	}

	_, err := backend.Generate(context.Background(), GenerateRequest{Prompt: "$imagegen draw a dragon"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "deadline exceeded") {
		t.Fatalf("expected deadline exceeded in error, got %q", err.Error())
	}
}

type recordingRunner struct{ req codex.Request }

func (r *recordingRunner) Run(ctx context.Context, req codex.Request) (codex.RunResult, error) {
	r.req = req
	return codex.RunResult{Stdout: "Saved to: file:///tmp/1.png\n"}, nil
}

func TestBuiltinCodexGenerateBuildsExecArgsWithImages(t *testing.T) {
	runner := &recordingRunner{}
	backend := BuiltinCodex{Command: "codex", Runner: runner}
	_, _ = backend.Generate(context.Background(), GenerateRequest{Prompt: "$imagegen draw a dragon", Images: []string{"/tmp/1.png", "/tmp/2.png"}})
	want := []string{"exec", "--json", "--image", "/tmp/1.png", "--image", "/tmp/2.png", "--", "$imagegen draw a dragon"}
	if !reflect.DeepEqual(runner.req.Args, want) {
		t.Fatalf("args = %#v, want %#v", runner.req.Args, want)
	}
}

func TestBuiltinCodexGeneratePassesCodexHome(t *testing.T) {
	runner := &recordingRunner{}
	codexHome := t.TempDir()
	backend := BuiltinCodex{Command: "codex", CodexHome: codexHome, Runner: runner}
	_, _ = backend.Generate(context.Background(), GenerateRequest{Prompt: "$imagegen draw a dragon"})
	if runner.req.CodexHome != codexHome {
		t.Fatalf("CodexHome = %q, want %q", runner.req.CodexHome, codexHome)
	}
}

func TestBuiltinCodexGenerateRecordsParserPhases(t *testing.T) {
	var phases []string
	backend := BuiltinCodex{Command: "codex", Runner: commandRunnerFunc(func(ctx context.Context, req codex.Request) (codex.RunResult, error) {
		return codex.RunResult{Stdout: "Saved to: file:///tmp/1.png\n"}, nil
	})}

	_, err := backend.Generate(context.Background(), GenerateRequest{
		Prompt: "$imagegen draw a dragon",
		RecordPhase: func(phase string, occurredAt time.Time, detail string) {
			phases = append(phases, phase)
		},
	})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	want := []string{"parser.started", "parser.completed"}
	if !reflect.DeepEqual(phases, want) {
		t.Fatalf("phases = %#v, want %#v", phases, want)
	}
}

func TestBuiltinCodexGenerateRecordsParserFailed(t *testing.T) {
	var phases []string
	backend := BuiltinCodex{Command: "codex", Runner: commandRunnerFunc(func(ctx context.Context, req codex.Request) (codex.RunResult, error) {
		return codex.RunResult{Stdout: "no image path\n"}, nil
	})}

	_, err := backend.Generate(context.Background(), GenerateRequest{
		Prompt: "$imagegen draw a dragon",
		RecordPhase: func(phase string, occurredAt time.Time, detail string) {
			phases = append(phases, phase)
		},
	})
	if err == nil {
		t.Fatal("expected parser error")
	}

	want := []string{"parser.started", "parser.failed"}
	if !reflect.DeepEqual(phases, want) {
		t.Fatalf("phases = %#v, want %#v", phases, want)
	}
}

type commandRunnerFunc func(context.Context, codex.Request) (codex.RunResult, error)

func (f commandRunnerFunc) Run(ctx context.Context, req codex.Request) (codex.RunResult, error) {
	return f(ctx, req)
}
