package backend

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
