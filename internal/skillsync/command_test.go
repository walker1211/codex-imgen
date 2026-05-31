package skillsync

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunDefaultsToCheck(t *testing.T) {
	repoRoot := t.TempDir()
	home := t.TempDir()
	createSourceSkills(t, repoRoot)
	writeFile(t, filepath.Join(repoRoot, "go.mod"), "module example.test/imgen\n")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Run(nil, CommandContext{
		Stdout:      &stdout,
		Stderr:      &stderr,
		Getwd:       func() (string, error) { return repoRoot, nil },
		UserHomeDir: func() (string, error) { return home, nil },
	})

	if exitCode != 1 {
		t.Fatalf("exitCode = %d", exitCode)
	}
	if !strings.Contains(stdout.String(), "skill installs are out of sync") {
		t.Fatalf("stdout = %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunApplyCopiesSkills(t *testing.T) {
	repoRoot := t.TempDir()
	home := t.TempDir()
	createSourceSkills(t, repoRoot)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Run([]string{"--apply", "--repo-root", repoRoot}, CommandContext{
		Stdout:      &stdout,
		Stderr:      &stderr,
		Getwd:       func() (string, error) { return repoRoot, nil },
		UserHomeDir: func() (string, error) { return home, nil },
	})

	if exitCode != 0 {
		t.Fatalf("exitCode = %d stdout=%q stderr=%q", exitCode, stdout.String(), stderr.String())
	}
	assertFile(t, filepath.Join(home, ".claude", "skills", "imgen", "SKILL.md"), "claude skill")
	assertFile(t, filepath.Join(home, ".openclaw", "workspace", "skills", "imgen", "SKILL.md"), "claude skill")
	assertFile(t, filepath.Join(home, ".codex", "skills", "imgen", "SKILL.md"), "claude skill")
	if !strings.Contains(stdout.String(), "copied") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunApplyAcceptsCodexDirOverride(t *testing.T) {
	repoRoot := t.TempDir()
	home := t.TempDir()
	codexParent := filepath.Join(t.TempDir(), "custom-codex", "skills")
	codexDir := filepath.Join(codexParent, "imgen")
	createSourceSkills(t, repoRoot)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Run([]string{"--apply", "--repo-root", repoRoot, "--codex-dir", codexDir}, CommandContext{
		Stdout:      &stdout,
		Stderr:      &stderr,
		Getwd:       func() (string, error) { return repoRoot, nil },
		UserHomeDir: func() (string, error) { return home, nil },
	})

	if exitCode != 0 {
		t.Fatalf("exitCode = %d stdout=%q stderr=%q", exitCode, stdout.String(), stderr.String())
	}
	assertFile(t, filepath.Join(codexParent, "imgen", "SKILL.md"), "claude skill")
	if !strings.Contains(stdout.String(), codexDir) {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunRejectsApplyAndCheckTogether(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Run([]string{"--apply", "--check"}, CommandContext{
		Stdout:      &stdout,
		Stderr:      &stderr,
		Getwd:       func() (string, error) { return t.TempDir(), nil },
		UserHomeDir: func() (string, error) { return t.TempDir(), nil },
	})

	if exitCode != 2 {
		t.Fatalf("exitCode = %d", exitCode)
	}
	if !strings.Contains(stderr.String(), "choose either --check or --apply") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}
