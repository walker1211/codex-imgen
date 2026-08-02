package skillsync

import (
	"bytes"
	"os"
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

func TestRunApplyCopiesOnlyAgentsWhenOptionalRuntimesMissing(t *testing.T) {
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
	assertFile(t, filepath.Join(home, ".agents", "skills", "imgen", "SKILL.md"), "agents skill")
	if _, err := os.Stat(filepath.Join(home, ".claude")); !os.IsNotExist(err) {
		t.Fatalf("Claude directory stat error = %v, want not exist", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".openclaw")); !os.IsNotExist(err) {
		t.Fatalf("OpenClaw directory stat error = %v, want not exist", err)
	}
	if !strings.Contains(stdout.String(), "copied") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunApplyCopiesOpenClawWhenOpenClawDirectoryExists(t *testing.T) {
	repoRoot := t.TempDir()
	home := t.TempDir()
	createSourceSkills(t, repoRoot)
	if err := os.MkdirAll(filepath.Join(home, ".openclaw"), 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
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
	assertFile(t, filepath.Join(home, ".openclaw", "workspace", "skills", "imgen", "SKILL.md"), "agents skill")
}

func TestRunApplyCopiesClaudeWhenClaudeDirectoryExists(t *testing.T) {
	repoRoot := t.TempDir()
	home := t.TempDir()
	createSourceSkills(t, repoRoot)
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
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
	assertFile(t, filepath.Join(home, ".claude", "skills", "imgen", "SKILL.md"), "agents skill")
}

func TestRunApplyAcceptsAgentsDirOverride(t *testing.T) {
	repoRoot := t.TempDir()
	home := t.TempDir()
	agentsParent := filepath.Join(t.TempDir(), "custom-agents", "skills")
	agentsDir := filepath.Join(agentsParent, "imgen")
	createSourceSkills(t, repoRoot)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Run([]string{"--apply", "--repo-root", repoRoot, "--agents-dir", agentsDir}, CommandContext{
		Stdout:      &stdout,
		Stderr:      &stderr,
		Getwd:       func() (string, error) { return repoRoot, nil },
		UserHomeDir: func() (string, error) { return home, nil },
	})

	if exitCode != 0 {
		t.Fatalf("exitCode = %d stdout=%q stderr=%q", exitCode, stdout.String(), stderr.String())
	}
	assertFile(t, filepath.Join(agentsParent, "imgen", "SKILL.md"), "agents skill")
	if !strings.Contains(stdout.String(), agentsDir) {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunApplyAcceptsOpenClawDirOverride(t *testing.T) {
	repoRoot := t.TempDir()
	home := t.TempDir()
	openClawParent := filepath.Join(t.TempDir(), "custom-openclaw", "skills")
	openClawDir := filepath.Join(openClawParent, "imgen")
	createSourceSkills(t, repoRoot)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Run([]string{"--apply", "--repo-root", repoRoot, "--openclaw-dir", openClawDir}, CommandContext{
		Stdout:      &stdout,
		Stderr:      &stderr,
		Getwd:       func() (string, error) { return repoRoot, nil },
		UserHomeDir: func() (string, error) { return home, nil },
	})

	if exitCode != 0 {
		t.Fatalf("exitCode = %d stdout=%q stderr=%q", exitCode, stdout.String(), stderr.String())
	}
	assertFile(t, filepath.Join(openClawParent, "imgen", "SKILL.md"), "agents skill")
	if !strings.Contains(stdout.String(), openClawDir) {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunApplyAcceptsCodexDirCompatibilityAlias(t *testing.T) {
	repoRoot := t.TempDir()
	home := t.TempDir()
	agentsParent := filepath.Join(t.TempDir(), "custom-agents", "skills")
	agentsDir := filepath.Join(agentsParent, "imgen")
	createSourceSkills(t, repoRoot)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Run([]string{"--apply", "--repo-root", repoRoot, "--codex-dir", agentsDir}, CommandContext{
		Stdout:      &stdout,
		Stderr:      &stderr,
		Getwd:       func() (string, error) { return repoRoot, nil },
		UserHomeDir: func() (string, error) { return home, nil },
	})

	if exitCode != 0 {
		t.Fatalf("exitCode = %d stdout=%q stderr=%q", exitCode, stdout.String(), stderr.String())
	}
	assertFile(t, filepath.Join(agentsParent, "imgen", "SKILL.md"), "agents skill")
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

func TestRunRejectsAgentsAndCodexDirTogether(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Run([]string{"--agents-dir", "/tmp/agents/skills/imgen", "--codex-dir", "/tmp/codex/skills/imgen"}, CommandContext{
		Stdout: &stdout,
		Stderr: &stderr,
	})

	if exitCode != 2 {
		t.Fatalf("exitCode = %d", exitCode)
	}
	if !strings.Contains(stderr.String(), "choose either --agents-dir or --codex-dir") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunHelp(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"-h"}, {"help"}} {
		t.Run(args[0], func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer

			exitCode := Run(args, CommandContext{
				Stdout: &stdout,
				Stderr: &stderr,
			})

			if exitCode != 0 {
				t.Fatalf("exitCode = %d", exitCode)
			}
			if !strings.Contains(stderr.String(), "Usage of skill-sync") {
				t.Fatalf("stderr = %q", stderr.String())
			}
		})
	}
}
