package parser

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractImageResultFromInlineSavedTo(t *testing.T) {
	output := "Saved to: file:///Users/demo/.codex/generated/image.png\n"

	result, err := ExtractImageResult(output, "")
	if err != nil {
		t.Fatalf("ExtractImageResult returned error: %v", err)
	}

	if result.URI != "file:///Users/demo/.codex/generated/image.png" {
		t.Fatalf("URI = %q", result.URI)
	}

	if result.Path != "/Users/demo/.codex/generated/image.png" {
		t.Fatalf("Path = %q", result.Path)
	}
}

func TestExtractImageResultFromNextLineSavedTo(t *testing.T) {
	output := "Saved to:\nfile:///Users/demo/.codex/generated/dragon.png\n"

	result, err := ExtractImageResult(output, "")
	if err != nil {
		t.Fatalf("ExtractImageResult returned error: %v", err)
	}

	if result.Path != "/Users/demo/.codex/generated/dragon.png" {
		t.Fatalf("Path = %q", result.Path)
	}
}

func TestExtractImageResultFromThreadDirectory(t *testing.T) {
	codexHome := t.TempDir()
	threadID := "019db517-05e0-77b2-aff1-a90de1fee1ea"
	imagePath := filepath.Join(codexHome, "generated_images", threadID, "ig_test.png")
	if err := os.MkdirAll(filepath.Dir(imagePath), 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(imagePath, []byte("png"), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	output := "{\"type\":\"thread.started\",\"thread_id\":\"019db517-05e0-77b2-aff1-a90de1fee1ea\"}\n"
	result, err := ExtractImageResult(output, codexHome)
	if err != nil {
		t.Fatalf("ExtractImageResult returned error: %v", err)
	}

	if result.Path != imagePath {
		t.Fatalf("Path = %q, want %q", result.Path, imagePath)
	}
}

func TestExtractImageResultReturnsErrorWhenPathMissing(t *testing.T) {
	_, err := ExtractImageResult("generation finished without final path\n", "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if err.Error() != "image path not found in codex output" {
		t.Fatalf("error = %q", err.Error())
	}
}

func TestExtractImageResultIncludesAgentMessageWhenPathMissing(t *testing.T) {
	output := `{"type":"thread.started","thread_id":"019e9705-aab1-7720-aa88-63ca772101b1"}
{"type":"item.completed","item":{"type":"agent_message","text":"image_gen returned TooManyRequests; retry later"}}
`

	_, err := ExtractImageResult(output, t.TempDir())
	if !errors.Is(err, ErrImagePathNotFound) {
		t.Fatalf("error = %v, want ErrImagePathNotFound", err)
	}
	if !strings.Contains(err.Error(), "TooManyRequests") {
		t.Fatalf("error = %q, want TooManyRequests detail", err.Error())
	}
}
