package backend

import (
	"context"
	"os"
	"path/filepath"
	"testing"
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
