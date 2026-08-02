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

func TestBuiltinCodexGenerateBuildsExecArgsWithReasoningEffort(t *testing.T) {
	runner := &recordingRunner{}
	backend := BuiltinCodex{Command: "codex", Model: "gpt-5.6-terra", ReasoningEffort: "high", Runner: runner}
	_, _ = backend.Generate(context.Background(), GenerateRequest{Prompt: "$imagegen draw a dragon"})
	want := []string{"exec", "--json", "--config", `model_reasoning_effort="high"`, "--model", "gpt-5.6-terra", "--", "$imagegen draw a dragon"}
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

func TestBuiltinCodexGenerateCopiesImageToDeliveryDir(t *testing.T) {
	sourceDir := t.TempDir()
	sourcePath := filepath.Join(sourceDir, "ig_test.png")
	if err := os.WriteFile(sourcePath, []byte("png"), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	deliveryDir := filepath.Join(t.TempDir(), "openclaw", "workspace", "imgen")
	backend := BuiltinCodex{
		Command:     "codex",
		DeliveryDir: deliveryDir,
		Runner: commandRunnerFunc(func(ctx context.Context, req codex.Request) (codex.RunResult, error) {
			return codex.RunResult{Stdout: "Saved to: file://" + sourcePath + "\n"}, nil
		}),
	}

	result, err := backend.Generate(context.Background(), GenerateRequest{Prompt: "$imagegen draw a dragon"})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	if filepath.Dir(result.Path) != deliveryDir {
		t.Fatalf("Path dir = %q, want %q", filepath.Dir(result.Path), deliveryDir)
	}
	if result.Path == sourcePath {
		t.Fatalf("Path = source path %q, want copied delivery path", result.Path)
	}
	if !strings.HasPrefix(filepath.Base(result.Path), filepath.Base(sourceDir)+"-") || filepath.Ext(result.Path) != ".png" {
		t.Fatalf("Path = %q, want copied png with source directory prefix", result.Path)
	}
	if result.URI != "file://"+result.Path {
		t.Fatalf("URI = %q, want file URI for %q", result.URI, result.Path)
	}
	content, err := os.ReadFile(result.Path)
	if err != nil {
		t.Fatalf("ReadFile copied image failed: %v", err)
	}
	if string(content) != "png" {
		t.Fatalf("copied content = %q, want png", content)
	}
}

func TestBuiltinCodexGenerateKeepsSourceThreadDirByDefault(t *testing.T) {
	codexHome := t.TempDir()
	sourcePath := writeSourceImage(t, codexHome, "thread-default", "ig_test.png", "png")
	sourceThreadDir := filepath.Dir(sourcePath)
	deliveryDir := t.TempDir()
	backend := BuiltinCodex{
		Command:     "codex",
		CodexHome:   codexHome,
		DeliveryDir: deliveryDir,
		Runner: commandRunnerFunc(func(ctx context.Context, req codex.Request) (codex.RunResult, error) {
			return codex.RunResult{Stdout: "Saved to: file://" + sourcePath + "\n"}, nil
		}),
	}

	result, err := backend.Generate(context.Background(), GenerateRequest{Prompt: "$imagegen draw a dragon"})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	if _, err := os.Stat(result.Path); err != nil {
		t.Fatalf("delivery image missing: %v", err)
	}
	if _, err := os.Stat(sourceThreadDir); err != nil {
		t.Fatalf("source thread dir should remain by default: %v", err)
	}
}

func TestBuiltinCodexGenerateCleansSourceThreadDirWhenEnabled(t *testing.T) {
	codexHome := t.TempDir()
	sourcePath := writeSourceImage(t, codexHome, "thread-clean", "ig_test.png", "png")
	sourceThreadDir := filepath.Dir(sourcePath)
	generatedImagesDir := filepath.Dir(sourceThreadDir)
	deliveryDir := t.TempDir()
	backend := BuiltinCodex{
		Command:                "codex",
		CodexHome:              codexHome,
		DeliveryDir:            deliveryDir,
		CleanupSourceThreadDir: true,
		Runner: commandRunnerFunc(func(ctx context.Context, req codex.Request) (codex.RunResult, error) {
			return codex.RunResult{Stdout: "Saved to: file://" + sourcePath + "\n"}, nil
		}),
	}

	result, err := backend.Generate(context.Background(), GenerateRequest{Prompt: "$imagegen draw a dragon"})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	if _, err := os.Stat(result.Path); err != nil {
		t.Fatalf("delivery image missing: %v", err)
	}
	if _, err := os.Stat(sourceThreadDir); !os.IsNotExist(err) {
		t.Fatalf("source thread dir stat err = %v, want not exist", err)
	}
	if _, err := os.Stat(generatedImagesDir); err != nil {
		t.Fatalf("generated_images root should remain: %v", err)
	}
}

func TestBuiltinCodexGenerateDoesNotCleanupSourceThreadDirWithoutDeliveryDir(t *testing.T) {
	codexHome := t.TempDir()
	sourcePath := writeSourceImage(t, codexHome, "thread-no-delivery", "ig_test.png", "png")
	sourceThreadDir := filepath.Dir(sourcePath)
	backend := BuiltinCodex{
		Command:                "codex",
		CodexHome:              codexHome,
		CleanupSourceThreadDir: true,
		Runner: commandRunnerFunc(func(ctx context.Context, req codex.Request) (codex.RunResult, error) {
			return codex.RunResult{Stdout: "Saved to: file://" + sourcePath + "\n"}, nil
		}),
	}

	result, err := backend.Generate(context.Background(), GenerateRequest{Prompt: "$imagegen draw a dragon"})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	if result.Path != sourcePath {
		t.Fatalf("Path = %q, want source path %q", result.Path, sourcePath)
	}
	if _, err := os.Stat(sourceThreadDir); err != nil {
		t.Fatalf("source thread dir should remain without delivery_dir: %v", err)
	}
}

func TestCleanupSourceThreadDirIgnoresUnsafePaths(t *testing.T) {
	codexHome := t.TempDir()
	generatedImagesDir := filepath.Join(codexHome, "generated_images")
	if err := os.MkdirAll(generatedImagesDir, 0o755); err != nil {
		t.Fatalf("MkdirAll generated_images failed: %v", err)
	}
	externalDir := t.TempDir()
	externalPath := filepath.Join(externalDir, "ig_external.png")
	if err := os.WriteFile(externalPath, []byte("external"), 0o644); err != nil {
		t.Fatalf("WriteFile external failed: %v", err)
	}
	rootImagePath := filepath.Join(generatedImagesDir, "ig_root.png")
	if err := os.WriteFile(rootImagePath, []byte("root"), 0o644); err != nil {
		t.Fatalf("WriteFile root image failed: %v", err)
	}
	nestedPath := writeSourceImage(t, codexHome, filepath.Join("thread", "nested"), "ig_nested.png", "nested")

	for _, path := range []string{externalPath, rootImagePath, nestedPath} {
		if err := cleanupSourceThreadDir(path, codexHome); err != nil {
			t.Fatalf("cleanupSourceThreadDir(%q) returned error: %v", path, err)
		}
	}

	if _, err := os.Stat(externalDir); err != nil {
		t.Fatalf("external dir should remain: %v", err)
	}
	if _, err := os.Stat(generatedImagesDir); err != nil {
		t.Fatalf("generated_images root should remain: %v", err)
	}
	if _, err := os.Stat(filepath.Dir(nestedPath)); err != nil {
		t.Fatalf("nested source dir should remain: %v", err)
	}
}

func TestBuiltinCodexGeneratePrunesDeliveryDirToMaxFiles(t *testing.T) {
	sourceDir := t.TempDir()
	sourcePath := filepath.Join(sourceDir, "ig_new.png")
	if err := os.WriteFile(sourcePath, []byte("new"), 0o644); err != nil {
		t.Fatalf("WriteFile source failed: %v", err)
	}
	deliveryDir := t.TempDir()
	oldPath := writeDeliveryFile(t, deliveryDir, "old.png", "old", time.Now().Add(-3*time.Hour))
	keepPath := writeDeliveryFile(t, deliveryDir, "keep.png", "keep", time.Now().Add(-1*time.Hour))
	backend := BuiltinCodex{
		Command:          "codex",
		DeliveryDir:      deliveryDir,
		DeliveryMaxFiles: 2,
		Runner: commandRunnerFunc(func(ctx context.Context, req codex.Request) (codex.RunResult, error) {
			return codex.RunResult{Stdout: "Saved to: file://" + sourcePath + "\n"}, nil
		}),
	}

	result, err := backend.Generate(context.Background(), GenerateRequest{Prompt: "$imagegen draw a dragon"})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	if _, err := os.Stat(result.Path); err != nil {
		t.Fatalf("new delivery file missing: %v", err)
	}
	if _, err := os.Stat(keepPath); err != nil {
		t.Fatalf("newer existing delivery file missing: %v", err)
	}
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Fatalf("old delivery file stat err = %v, want not exist", err)
	}
}

func TestCopyImageToDeliveryDirAvoidsSameBasenameCollisions(t *testing.T) {
	sourceRoot := t.TempDir()
	firstSource := filepath.Join(sourceRoot, "thread-a", "image.png")
	secondSource := filepath.Join(sourceRoot, "thread-b", "image.png")
	if err := os.MkdirAll(filepath.Dir(firstSource), 0o755); err != nil {
		t.Fatalf("MkdirAll first failed: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(secondSource), 0o755); err != nil {
		t.Fatalf("MkdirAll second failed: %v", err)
	}
	if err := os.WriteFile(firstSource, []byte("first"), 0o644); err != nil {
		t.Fatalf("WriteFile first failed: %v", err)
	}
	if err := os.WriteFile(secondSource, []byte("second"), 0o644); err != nil {
		t.Fatalf("WriteFile second failed: %v", err)
	}
	deliveryDir := t.TempDir()

	firstTarget, err := copyImageToDeliveryDir(firstSource, deliveryDir)
	if err != nil {
		t.Fatalf("copy first failed: %v", err)
	}
	secondTarget, err := copyImageToDeliveryDir(secondSource, deliveryDir)
	if err != nil {
		t.Fatalf("copy second failed: %v", err)
	}

	if firstTarget == secondTarget {
		t.Fatalf("targets should differ, both were %q", firstTarget)
	}
	firstContent, err := os.ReadFile(firstTarget)
	if err != nil {
		t.Fatalf("ReadFile first target failed: %v", err)
	}
	secondContent, err := os.ReadFile(secondTarget)
	if err != nil {
		t.Fatalf("ReadFile second target failed: %v", err)
	}
	if string(firstContent) != "first" || string(secondContent) != "second" {
		t.Fatalf("copied content = %q/%q, want first/second", firstContent, secondContent)
	}
}

func TestCopyImageToDeliveryDirAvoidsSameParentDirectoryCollisions(t *testing.T) {
	sourceRoot := t.TempDir()
	firstSource := filepath.Join(sourceRoot, "left", "thread-x", "image.png")
	secondSource := filepath.Join(sourceRoot, "right", "thread-x", "image.png")
	if err := os.MkdirAll(filepath.Dir(firstSource), 0o755); err != nil {
		t.Fatalf("MkdirAll first failed: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(secondSource), 0o755); err != nil {
		t.Fatalf("MkdirAll second failed: %v", err)
	}
	if err := os.WriteFile(firstSource, []byte("first"), 0o644); err != nil {
		t.Fatalf("WriteFile first failed: %v", err)
	}
	if err := os.WriteFile(secondSource, []byte("second"), 0o644); err != nil {
		t.Fatalf("WriteFile second failed: %v", err)
	}
	deliveryDir := t.TempDir()

	firstTarget, err := copyImageToDeliveryDir(firstSource, deliveryDir)
	if err != nil {
		t.Fatalf("copy first failed: %v", err)
	}
	secondTarget, err := copyImageToDeliveryDir(secondSource, deliveryDir)
	if err != nil {
		t.Fatalf("copy second failed: %v", err)
	}

	if firstTarget == secondTarget {
		t.Fatalf("targets should differ, both were %q", firstTarget)
	}
	firstContent, err := os.ReadFile(firstTarget)
	if err != nil {
		t.Fatalf("ReadFile first target failed: %v", err)
	}
	secondContent, err := os.ReadFile(secondTarget)
	if err != nil {
		t.Fatalf("ReadFile second target failed: %v", err)
	}
	if string(firstContent) != "first" || string(secondContent) != "second" {
		t.Fatalf("copied content = %q/%q, want first/second", firstContent, secondContent)
	}
}

func writeSourceImage(t *testing.T, codexHome string, threadID string, name string, content string) string {
	t.Helper()
	path := filepath.Join(codexHome, "generated_images", threadID, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll source failed: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile source failed: %v", err)
	}
	return path
}

func writeDeliveryFile(t *testing.T, dir string, name string, content string, modTime time.Time) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile delivery file failed: %v", err)
	}
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatalf("Chtimes delivery file failed: %v", err)
	}
	return path
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
