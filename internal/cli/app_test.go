package cli

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
)

type stubRunner struct {
	stdout string
	stderr string
	err    error
}

func (s stubRunner) Run(ctx context.Context, req RunRequest) (RunResponse, error) {
	return RunResponse{Stdout: s.stdout, Stderr: s.stderr}, s.err
}

func TestAppRunPrintsPathOnSuccess(t *testing.T) {
	app := App{
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
		Runner: stubRunner{stdout: "Saved to: file:///tmp/generated/dragon.png\n"},
		ConfigPath: writeConfigFile(t, "prompt:\n  prefix: $imagegen\n"),
		CodexHome: t.TempDir(),
	}

	exitCode := app.Run(context.Background(), []string{"draw a dragon"})
	if exitCode != 0 {
		t.Fatalf("exitCode = %d", exitCode)
	}

	if got := app.Stdout.(*bytes.Buffer).String(); got != "/tmp/generated/dragon.png\n" {
		t.Fatalf("stdout = %q", got)
	}
}

func TestAppRunPrintsJSONOnSuccess(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	app := App{
		Stdout: stdout,
		Stderr: stderr,
		Runner: stubRunner{stdout: "Saved to: file:///tmp/generated/dragon.png\n"},
		ConfigPath: writeConfigFile(t, "prompt:\n  prefix: $imagegen\n"),
		CodexHome: t.TempDir(),
	}

	exitCode := app.Run(context.Background(), []string{"--json", "draw a dragon"})
	if exitCode != 0 {
		t.Fatalf("exitCode = %d", exitCode)
	}

	got := stdout.String()
	if !strings.Contains(got, "\"ok\":true") {
		t.Fatalf("stdout = %q", got)
	}
	if !strings.Contains(got, "\"path\":\"/tmp/generated/dragon.png\"") {
		t.Fatalf("stdout = %q", got)
	}
}

func TestAppRunPrintsStructuredErrorOnJSONFailure(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	app := App{
		Stdout: stdout,
		Stderr: stderr,
		Runner: stubRunner{stdout: "no saved path\n"},
		ConfigPath: writeConfigFile(t, "prompt:\n  prefix: $imagegen\n"),
		CodexHome: t.TempDir(),
	}

	exitCode := app.Run(context.Background(), []string{"--json", "draw a dragon"})
	if exitCode != 1 {
		t.Fatalf("exitCode = %d", exitCode)
	}

	got := stdout.String()
	if !strings.Contains(got, "\"ok\":false") {
		t.Fatalf("stdout = %q", got)
	}
	if !strings.Contains(got, "image path not found in codex output") {
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

	got := stdout.String()
	if !strings.Contains(got, "Usage: imgen") {
		t.Fatalf("stdout = %q", got)
	}
	if !strings.Contains(got, "--json") {
		t.Fatalf("stdout = %q", got)
	}
}

func TestAppRunUsesConfigJSONFormatOnFailure(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	app := App{
		Stdout: stdout,
		Stderr: stderr,
		Runner: stubRunner{stdout: "no saved path\n"},
		ConfigPath: writeConfigFile(t, "prompt:\n  prefix: $imagegen\noutput:\n  format: json\n"),
	}

	exitCode := app.Run(context.Background(), []string{"draw a dragon"})
	if exitCode != 1 {
		t.Fatalf("exitCode = %d", exitCode)
	}

	if got := stdout.String(); !strings.Contains(got, "\"ok\":false") {
		t.Fatalf("stdout = %q", got)
	}
	if got := stderr.String(); got != "" {
		t.Fatalf("stderr = %q", got)
	}
}

func TestAppRunIncludesRawOutputInJSONWithoutRawFlag(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	app := App{
		Stdout: stdout,
		Stderr: stderr,
		Runner: stubRunner{stdout: "Saved to: file:///tmp/generated/dragon.png\n", stderr: "debug line\n"},
		ConfigPath: writeConfigFile(t, "prompt:\n  prefix: $imagegen\n"),
		CodexHome: t.TempDir(),
	}

	exitCode := app.Run(context.Background(), []string{"--json", "draw a dragon"})
	if exitCode != 0 {
		t.Fatalf("exitCode = %d", exitCode)
	}

	got := stdout.String()
	if !strings.Contains(got, "\"raw_output\":") {
		t.Fatalf("stdout = %q", got)
	}
	if stderr.String() != "" {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestAppRunExtractsImageFromCodexExecThread(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	codexHome := t.TempDir()
	threadID := "019db517-05e0-77b2-aff1-a90de1fee1ea"
	imagePath := codexHome + "/generated_images/" + threadID + "/ig_test.png"
	if err := os.MkdirAll(codexHome+"/generated_images/"+threadID, 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(imagePath, []byte("png"), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	app := App{
		Stdout: stdout,
		Stderr: stderr,
		Runner: stubRunner{stdout: "{\"type\":\"thread.started\",\"thread_id\":\"019db517-05e0-77b2-aff1-a90de1fee1ea\"}\n"},
		ConfigPath: writeConfigFile(t, "prompt:\n  prefix: $imagegen\n"),
		CodexHome: codexHome,
	}

	exitCode := app.Run(context.Background(), []string{"draw a dragon"})
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, stderr = %q", exitCode, stderr.String())
	}
	if got := stdout.String(); got != imagePath+"\n" {
		t.Fatalf("stdout = %q", got)
	}
}

func writeConfigFile(t *testing.T, content string) string {
	t.Helper()
	path := t.TempDir() + "/config.yaml"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	return path
}
