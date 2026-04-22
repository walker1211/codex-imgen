package cli

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAppRunWithFakeCodexSuccess(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	script := fakeCodexPath(t, "codex-success.sh")
	mustMakeExecutable(t, script)

	app := App{
		Stdout: stdout,
		Stderr: stderr,
		ConfigPath: writeConfigFile(t, fmt.Sprintf("codex:\n  command: %q\nprompt:\n  prefix: \"$imagegen\"\n", script)),
	}

	exitCode := app.Run(context.Background(), []string{"dragon"})
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, stderr = %q", exitCode, stderr.String())
	}
	if got := stdout.String(); got != "/tmp/generated/image.png\n" {
		t.Fatalf("stdout = %q", got)
	}
}

func TestAppRunWithFakeCodexMissingPath(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	script := fakeCodexPath(t, "codex-missing-path.sh")
	mustMakeExecutable(t, script)

	app := App{
		Stdout: stdout,
		Stderr: stderr,
		ConfigPath: writeConfigFile(t, fmt.Sprintf("codex:\n  command: %q\nprompt:\n  prefix: \"$imagegen\"\n", script)),
	}

	exitCode := app.Run(context.Background(), []string{"--json", "dragon"})
	if exitCode != 1 {
		t.Fatalf("exitCode = %d", exitCode)
	}
	if got := stdout.String(); !strings.Contains(got, "image path not found in codex output") {
		t.Fatalf("stdout = %q", got)
	}
}

func TestAppRunWithFakeCodexExitError(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	script := fakeCodexPath(t, "codex-exit-9.sh")
	mustMakeExecutable(t, script)

	app := App{
		Stdout: stdout,
		Stderr: stderr,
		ConfigPath: writeConfigFile(t, fmt.Sprintf("codex:\n  command: %q\nprompt:\n  prefix: \"$imagegen\"\n", script)),
	}

	exitCode := app.Run(context.Background(), []string{"--json", "dragon"})
	if exitCode != 1 {
		t.Fatalf("exitCode = %d", exitCode)
	}
	if got := stdout.String(); !strings.Contains(got, "exit status 9") {
		t.Fatalf("stdout = %q", got)
	}
}

func fakeCodexPath(t *testing.T, name string) string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("..", "..", "testdata", name))
	if err != nil {
		t.Fatalf("Abs failed: %v", err)
	}
	return path
}

func mustMakeExecutable(t *testing.T, path string) {
	t.Helper()
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatalf("Chmod failed: %v", err)
	}
}
