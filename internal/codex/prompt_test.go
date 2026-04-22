package codex

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBuildPromptWithPrelude(t *testing.T) {
	prompt := BuildPrompt("使用内置 imagegen 技能。\n输出单张图片。", "$imagegen", "生成一张小龙")

	want := "使用内置 imagegen 技能。\n输出单张图片。\n\n$imagegen 生成一张小龙"
	if prompt != want {
		t.Fatalf("prompt = %q, want %q", prompt, want)
	}
}

func TestBuildPromptWithoutPrelude(t *testing.T) {
	prompt := BuildPrompt("", "$imagegen", "生成一张小龙")

	want := "$imagegen 生成一张小龙"
	if prompt != want {
		t.Fatalf("prompt = %q, want %q", prompt, want)
	}
}

func TestBuildPromptWithoutPrefix(t *testing.T) {
	prompt := BuildPrompt("", "", "生成一张小龙")

	want := "生成一张小龙"
	if prompt != want {
		t.Fatalf("prompt = %q, want %q", prompt, want)
	}
}

func TestRunnerRunCapturesStdoutAndStderr(t *testing.T) {
	runner := Runner{}
	script := filepath.Join("..", "..", "testdata", "codex-success.sh")

	result, err := runner.Run(context.Background(), Request{
		Command: script,
		Args:    []string{"prompt text"},
		Timeout: 2 * time.Second,
		Env:     os.Environ(),
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if result.Stdout == "" {
		t.Fatal("expected stdout to be captured")
	}
	if result.Stderr == "" {
		t.Fatal("expected stderr to be captured")
	}
}
