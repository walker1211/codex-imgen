package scripts_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestTagReleaseRequiresReleaseNotesByDefault(t *testing.T) {
	root := writeTempTagReleaseScript(t)

	_, stderr, err := runTagRelease(t, root, nil, "v0.0.1")
	if err == nil {
		t.Fatal("tag-release.sh succeeded without release notes")
	}
	assertContains(t, stderr, "release notes file missing: docs/releases/v0.0.1.md")
	assertContains(t, stderr, "create and commit it before tagging")
}

func TestTagReleaseAllowsMissingNotesWithExplicitOptOut(t *testing.T) {
	root := writeTempTagReleaseScript(t)
	initGitRepo(t, root)

	_, stderr, err := runTagRelease(t, root, nil, "v0.0.1", "--allow-missing-notes")
	if err == nil {
		t.Fatal("tag-release.sh succeeded with dirty working tree")
	}
	assertNotContains(t, stderr, "release notes file missing")
	assertContains(t, stderr, "working tree has uncommitted changes")
}

func TestTagReleaseAllowsMissingNotesWithEnvOptOut(t *testing.T) {
	root := writeTempTagReleaseScript(t)
	initGitRepo(t, root)

	_, stderr, err := runTagRelease(t, root, []string{"ALLOW_MISSING_RELEASE_NOTES=1"}, "v0.0.1")
	if err == nil {
		t.Fatal("tag-release.sh succeeded with dirty working tree")
	}
	assertNotContains(t, stderr, "release notes file missing")
	assertContains(t, stderr, "working tree has uncommitted changes")
}

func TestTagReleaseDoesNotTreatEnvZeroAsOptOut(t *testing.T) {
	root := writeTempTagReleaseScript(t)

	_, stderr, err := runTagRelease(t, root, []string{"ALLOW_MISSING_RELEASE_NOTES=0"}, "v0.0.1")
	if err == nil {
		t.Fatal("tag-release.sh succeeded without release notes")
	}
	assertContains(t, stderr, "release notes file missing: docs/releases/v0.0.1.md")
}

func TestTagReleaseRequiresCommittedReleaseNotes(t *testing.T) {
	root := writeTempTagReleaseScript(t)
	initGitRepo(t, root)
	writeReleaseNotes(t, root, "v0.0.1")

	_, stderr, err := runTagRelease(t, root, nil, "v0.0.1")
	if err == nil {
		t.Fatal("tag-release.sh succeeded with uncommitted release notes")
	}
	assertNotContains(t, stderr, "release notes file missing")
	assertContains(t, stderr, "working tree has uncommitted changes")
}

func writeTempTagReleaseScript(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	scriptsDir := filepath.Join(root, "scripts")
	if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll scripts failed: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(repoRoot(t), "scripts", "tag-release.sh"))
	if err != nil {
		t.Fatalf("ReadFile tag-release.sh failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(scriptsDir, "tag-release.sh"), content, 0o755); err != nil {
		t.Fatalf("WriteFile tag-release.sh failed: %v", err)
	}

	return root
}

func repoRoot(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Dir(filepath.Dir(file))
}

func runTagRelease(t *testing.T, root string, env []string, args ...string) (string, string, error) {
	t.Helper()

	cmd := exec.Command("bash", append([]string{filepath.Join(root, "scripts", "tag-release.sh")}, args...)...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), env...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

func initGitRepo(t *testing.T, root string) {
	t.Helper()

	cmd := exec.Command("git", "-C", root, "init")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v\n%s", err, output)
	}
}

func writeReleaseNotes(t *testing.T, root string, version string) {
	t.Helper()

	dir := filepath.Join(root, "docs", "releases")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll release notes failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, version+".md"), []byte("## Test release\n"), 0o644); err != nil {
		t.Fatalf("WriteFile release notes failed: %v", err)
	}
}

func assertContains(t *testing.T, got string, want string) {
	t.Helper()

	if !strings.Contains(got, want) {
		t.Fatalf("output %q does not contain %q", got, want)
	}
}

func assertNotContains(t *testing.T, got string, want string) {
	t.Helper()

	if strings.Contains(got, want) {
		t.Fatalf("output %q contains %q", got, want)
	}
}
