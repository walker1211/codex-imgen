package skillsync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckReportsMissingInstallTargets(t *testing.T) {
	repoRoot := t.TempDir()
	home := t.TempDir()
	createSourceSkills(t, repoRoot)

	result, err := DefaultPaths(repoRoot, home).Check()
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if len(result.Drift) != 2 {
		t.Fatalf("drift = %v", result.Drift)
	}
	if !containsLine(result.Drift, "claude install missing") {
		t.Fatalf("expected missing Claude install, got %v", result.Drift)
	}
	if !containsLine(result.Drift, "openclaw install missing") {
		t.Fatalf("expected missing OpenClaw install, got %v", result.Drift)
	}
}

func TestApplyCopiesSourcesAndRemovesStaleFiles(t *testing.T) {
	repoRoot := t.TempDir()
	home := t.TempDir()
	createSourceSkills(t, repoRoot)
	paths := DefaultPaths(repoRoot, home)
	writeFile(t, filepath.Join(paths.ClaudeInstallDir, "stale.md"), "old")

	result, err := paths.Apply()
	if err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	if len(result.Applied) != 3 {
		t.Fatalf("applied = %v", result.Applied)
	}
	assertFile(t, filepath.Join(paths.ClaudeInstallDir, "SKILL.md"), "claude skill")
	assertFile(t, filepath.Join(paths.OpenClawInstallDir, "SKILL.md"), "claude skill")
	if _, err := os.Stat(filepath.Join(paths.ClaudeInstallDir, "stale.md")); !os.IsNotExist(err) {
		t.Fatalf("expected stale install file to be removed, stat error = %v", err)
	}

	check, err := paths.Check()
	if err != nil {
		t.Fatalf("Check returned error after Apply: %v", err)
	}
	if len(check.Drift) != 0 {
		t.Fatalf("drift after apply = %v", check.Drift)
	}
}

func TestApplySyncsRepositoryOpenClawFromClaudeSource(t *testing.T) {
	repoRoot := t.TempDir()
	home := t.TempDir()
	createSourceSkills(t, repoRoot)
	paths := DefaultPaths(repoRoot, home)
	writeFile(t, filepath.Join(repoRoot, ".openclaw", "skills", "imgen", "SKILL.md"), "stale openclaw skill")

	result, err := paths.Apply()
	if err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	if !containsLine(result.Applied, filepath.Join(repoRoot, ".openclaw", "skills", "imgen")) {
		t.Fatalf("expected repository openclaw skill to be applied, got %v", result.Applied)
	}
	assertFile(t, filepath.Join(repoRoot, ".openclaw", "skills", "imgen", "SKILL.md"), "claude skill")
	assertFile(t, filepath.Join(paths.OpenClawInstallDir, "SKILL.md"), "claude skill")
}

func TestCheckReportsChangedTargetFile(t *testing.T) {
	repoRoot := t.TempDir()
	home := t.TempDir()
	createSourceSkills(t, repoRoot)
	paths := DefaultPaths(repoRoot, home)
	if _, err := paths.Apply(); err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	writeFile(t, filepath.Join(paths.ClaudeInstallDir, "SKILL.md"), "changed")

	result, err := paths.Check()
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if !containsLine(result.Drift, "claude file differs: SKILL.md") {
		t.Fatalf("expected changed file drift, got %v", result.Drift)
	}
}

func TestCompareSkillTreesReportsDifferences(t *testing.T) {
	source := filepath.Join(t.TempDir(), "source")
	destination := filepath.Join(t.TempDir(), "destination")
	writeFile(t, filepath.Join(source, "SKILL.md"), "source skill")
	writeFile(t, filepath.Join(source, "references", "imgen-usage.md"), "usage")
	writeFile(t, filepath.Join(destination, "SKILL.md"), "changed skill")
	writeFile(t, filepath.Join(destination, "extra.md"), "extra")

	drift, err := CompareSkillTrees(source, destination, "openclaw skill")
	if err != nil {
		t.Fatalf("CompareSkillTrees returned error: %v", err)
	}
	if !containsLine(drift, "openclaw skill file differs: SKILL.md") {
		t.Fatalf("expected changed SKILL.md drift, got %v", drift)
	}
	if !containsLine(drift, "openclaw skill file missing: references/imgen-usage.md") {
		t.Fatalf("expected missing reference drift, got %v", drift)
	}
	if !containsLine(drift, "openclaw skill extra file: extra.md") {
		t.Fatalf("expected extra file drift, got %v", drift)
	}
}

func TestCheckReportsRepositorySkillMismatch(t *testing.T) {
	repoRoot := t.TempDir()
	home := t.TempDir()
	createSourceSkills(t, repoRoot)
	paths := DefaultPaths(repoRoot, home)
	writeFile(t, filepath.Join(repoRoot, ".openclaw", "skills", "imgen", "SKILL.md"), "changed openclaw skill")

	result, err := paths.Check()
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if !containsLine(result.Drift, "repo openclaw file differs: SKILL.md") {
		t.Fatalf("expected repo skill drift, got %v", result.Drift)
	}
}

func TestCheckReportsMissingRepositoryMirrorWithLegacyMessage(t *testing.T) {
	repoRoot := t.TempDir()
	home := t.TempDir()
	createSourceSkills(t, repoRoot)
	paths := DefaultPaths(repoRoot, home)
	if err := os.RemoveAll(filepath.Join(repoRoot, ".openclaw", "skills", "imgen")); err != nil {
		t.Fatalf("RemoveAll returned error: %v", err)
	}

	result, err := paths.Check()
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if !containsLine(result.Drift, "repo openclaw skill missing:") {
		t.Fatalf("expected legacy missing repo mirror drift, got %v", result.Drift)
	}
	if containsLine(result.Drift, "repo openclaw missing:") {
		t.Fatalf("expected missing repo mirror drift to keep legacy wording, got %v", result.Drift)
	}
}

func TestCheckReportsRepositoryReferenceMismatch(t *testing.T) {
	repoRoot := t.TempDir()
	home := t.TempDir()
	createSourceSkills(t, repoRoot)
	paths := DefaultPaths(repoRoot, home)
	writeFile(t, filepath.Join(repoRoot, ".openclaw", "skills", "imgen", "references", "imgen-usage.md"), "changed usage")

	result, err := paths.Check()
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if !containsLine(result.Drift, "repo openclaw file differs: references/imgen-usage.md") {
		t.Fatalf("expected repo reference drift, got %v", result.Drift)
	}
}

func TestCheckReportsMissingRepositoryReference(t *testing.T) {
	repoRoot := t.TempDir()
	home := t.TempDir()
	createSourceSkills(t, repoRoot)
	paths := DefaultPaths(repoRoot, home)
	if err := os.Remove(filepath.Join(repoRoot, ".openclaw", "skills", "imgen", "references", "imgen-usage.md")); err != nil {
		t.Fatalf("Remove returned error: %v", err)
	}

	result, err := paths.Check()
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if !containsLine(result.Drift, "repo openclaw file missing: references/imgen-usage.md") {
		t.Fatalf("expected missing repo reference drift, got %v", result.Drift)
	}
}

func TestCheckReportsExtraRepositoryReference(t *testing.T) {
	repoRoot := t.TempDir()
	home := t.TempDir()
	createSourceSkills(t, repoRoot)
	paths := DefaultPaths(repoRoot, home)
	writeFile(t, filepath.Join(repoRoot, ".openclaw", "skills", "imgen", "references", "extra.md"), "extra")

	result, err := paths.Check()
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if !containsLine(result.Drift, "repo openclaw extra file: references/extra.md") {
		t.Fatalf("expected extra repo reference drift, got %v", result.Drift)
	}
}

func TestCheckReportsMissingAndExtraInstallFiles(t *testing.T) {
	repoRoot := t.TempDir()
	home := t.TempDir()
	createSourceSkills(t, repoRoot)
	paths := DefaultPaths(repoRoot, home)
	if _, err := paths.Apply(); err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	if err := os.Remove(filepath.Join(paths.ClaudeInstallDir, "references", "imgen-usage.md")); err != nil {
		t.Fatalf("Remove returned error: %v", err)
	}
	writeFile(t, filepath.Join(paths.ClaudeInstallDir, "extra.md"), "extra")

	result, err := paths.Check()
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if !containsLine(result.Drift, "claude file missing: references/imgen-usage.md") {
		t.Fatalf("expected missing file drift, got %v", result.Drift)
	}
	if !containsLine(result.Drift, "claude extra install file: extra.md") {
		t.Fatalf("expected extra file drift, got %v", result.Drift)
	}
}

func TestApplyRejectsDestinationOutsideExpectedSkillParent(t *testing.T) {
	repoRoot := t.TempDir()
	home := t.TempDir()
	createSourceSkills(t, repoRoot)
	outside := t.TempDir()
	paths := DefaultPaths(repoRoot, home)
	paths.ClaudeInstallDir = filepath.Join(outside, "skills", "imgen")
	writeFile(t, filepath.Join(paths.ClaudeInstallDir, "keep.md"), "keep")

	_, err := paths.Apply()
	if err == nil {
		t.Fatal("expected Apply to reject destination outside expected skill parent")
	}
	if !strings.Contains(err.Error(), "outside expected install parent") {
		t.Fatalf("error = %v", err)
	}
	assertFile(t, filepath.Join(paths.ClaudeInstallDir, "keep.md"), "keep")
}

func TestApplyRejectsRelativeDestination(t *testing.T) {
	repoRoot := t.TempDir()
	home := t.TempDir()
	createSourceSkills(t, repoRoot)
	paths := DefaultPaths(repoRoot, home)
	paths.ClaudeInstallDir = filepath.Join("relative", "skills", "imgen")

	_, err := paths.Apply()
	if err == nil {
		t.Fatal("expected Apply to reject relative destination")
	}
	if !strings.Contains(err.Error(), "destination must be absolute") {
		t.Fatalf("error = %v", err)
	}
}

func TestApplyAcceptsExplicitInstallDirOverride(t *testing.T) {
	repoRoot := t.TempDir()
	home := t.TempDir()
	createSourceSkills(t, repoRoot)
	claudeParent := filepath.Join(t.TempDir(), "custom-claude", "skills")
	openClawParent := filepath.Join(t.TempDir(), "custom-openclaw", "skills")
	paths := DefaultPaths(repoRoot, home).
		WithClaudeInstallDir(filepath.Join(claudeParent, "imgen")).
		WithOpenClawInstallDir(filepath.Join(openClawParent, "imgen"))

	result, err := paths.Apply()
	if err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	if len(result.Applied) != 3 {
		t.Fatalf("applied = %v", result.Applied)
	}
	assertFile(t, filepath.Join(claudeParent, "imgen", "SKILL.md"), "claude skill")
	assertFile(t, filepath.Join(openClawParent, "imgen", "SKILL.md"), "claude skill")
}

func TestCheckReportsDestinationSymlinkAsDrift(t *testing.T) {
	repoRoot := t.TempDir()
	home := t.TempDir()
	createSourceSkills(t, repoRoot)
	paths := DefaultPaths(repoRoot, home)
	if _, err := paths.Apply(); err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	if err := os.Remove(filepath.Join(paths.ClaudeInstallDir, "SKILL.md")); err != nil {
		t.Fatalf("Remove returned error: %v", err)
	}
	outside := filepath.Join(t.TempDir(), "outside.md")
	writeFile(t, outside, "outside")
	if err := os.Symlink(outside, filepath.Join(paths.ClaudeInstallDir, "SKILL.md")); err != nil {
		t.Fatalf("Symlink returned error: %v", err)
	}

	result, err := paths.Check()
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if !containsLine(result.Drift, "claude file is symlink: SKILL.md") {
		t.Fatalf("expected destination symlink drift, got %v", result.Drift)
	}
}

func TestApplyRejectsSourceSymlink(t *testing.T) {
	repoRoot := t.TempDir()
	home := t.TempDir()
	createSourceSkills(t, repoRoot)
	outside := filepath.Join(t.TempDir(), "secret.txt")
	writeFile(t, outside, "secret")
	link := filepath.Join(repoRoot, ".claude", "skills", "imgen", "linked-secret.txt")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatalf("Symlink returned error: %v", err)
	}

	_, err := DefaultPaths(repoRoot, home).Apply()
	if err == nil {
		t.Fatal("expected Apply to reject source symlink")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("error = %v", err)
	}
}

func TestFindRepositoryRoot(t *testing.T) {
	repoRoot := t.TempDir()
	createSourceSkills(t, repoRoot)
	writeFile(t, filepath.Join(repoRoot, "go.mod"), "module example.test/imgen\n")
	nested := filepath.Join(repoRoot, "cmd", "skill-sync")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}

	found, err := FindRepositoryRoot(nested)
	if err != nil {
		t.Fatalf("FindRepositoryRoot returned error: %v", err)
	}
	if found != repoRoot {
		t.Fatalf("found = %q, want %q", found, repoRoot)
	}
}

func createSourceSkills(t *testing.T, repoRoot string) {
	t.Helper()
	writeFile(t, filepath.Join(repoRoot, ".claude", "skills", "imgen", "SKILL.md"), "claude skill")
	writeFile(t, filepath.Join(repoRoot, ".claude", "skills", "imgen", "references", "imgen-usage.md"), "usage")
	writeFile(t, filepath.Join(repoRoot, ".claude", "skills", "imgen", "evals", "evals.json"), "{}")
	writeFile(t, filepath.Join(repoRoot, ".openclaw", "skills", "imgen", "SKILL.md"), "claude skill")
	writeFile(t, filepath.Join(repoRoot, ".openclaw", "skills", "imgen", "references", "imgen-usage.md"), "usage")
	writeFile(t, filepath.Join(repoRoot, ".openclaw", "skills", "imgen", "evals", "evals.json"), "{}")
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
}

func assertFile(t *testing.T, path string, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if string(got) != want {
		t.Fatalf("%s = %q, want %q", path, string(got), want)
	}
}

func containsLine(lines []string, want string) bool {
	for _, line := range lines {
		if strings.Contains(line, want) {
			return true
		}
	}
	return false
}
