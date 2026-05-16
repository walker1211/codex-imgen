package doctor

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenClawCheckerReportsOKAndWarn(t *testing.T) {
	home := t.TempDir()
	writeOpenClawConfig(t, home, `{
		"tools": {"deny": ["image_generate"]},
		"gateway": {"tools": {"deny": ["image_generate"]}},
		"agents": {"list": [{"id": "main", "tools": {"alsoAllow": ["message"]}}]},
		"surfaces": {"telegram": {"silentReply": {"direct": "allow"}}},
		"plugins": {"active-memory": {"agents": ["main"]}}
	}`)
	checker := checkerWithSyncedRepo(t, home)

	report, err := checker.Check(context.Background())
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if report.Failed() {
		t.Fatalf("expected no failures, got:\n%s", report.Render())
	}
	if !reportHas(report, LevelWarn, "active-memory targets main") {
		t.Fatalf("expected active-memory warning, got:\n%s", report.Render())
	}
	if !reportHas(report, LevelOK, "repository OpenClaw skill mirror matches Claude source") {
		t.Fatalf("expected repository sync OK, got:\n%s", report.Render())
	}
	if !reportHas(report, LevelOK, "installed OpenClaw imgen skill matches Claude source") {
		t.Fatalf("expected installed sync OK, got:\n%s", report.Render())
	}
	if !strings.HasPrefix(report.Render(), "OpenClaw doctor\n") {
		t.Fatalf("rendered report = %q", report.Render())
	}
}

func TestOpenClawCheckerFailsWhenMessageMissing(t *testing.T) {
	home := t.TempDir()
	writeOpenClawConfig(t, home, validOpenClawConfig(`"alsoAllow": []`))
	checker := checkerWithSyncedRepo(t, home)

	report, err := checker.Check(context.Background())
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if !reportHas(report, LevelFail, "main agent does not expose message") {
		t.Fatalf("expected message failure, got:\n%s", report.Render())
	}
}

func TestOpenClawCheckerFailsWhenDirectSilentReplyMissing(t *testing.T) {
	home := t.TempDir()
	writeOpenClawConfig(t, home, `{
		"tools": {"deny": ["image_generate"]},
		"gateway": {"tools": {"deny": ["image_generate"]}},
		"agents": {"list": [{"id": "main", "tools": {"alsoAllow": ["message"]}}]},
		"surfaces": {"telegram": {"silentReply": {"direct": "fallback"}}}
	}`)
	checker := checkerWithSyncedRepo(t, home)

	report, err := checker.Check(context.Background())
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if !reportHas(report, LevelFail, "telegram direct NO_REPLY is not configured as silent") {
		t.Fatalf("expected silent reply failure, got:\n%s", report.Render())
	}
}

func TestOpenClawCheckerFailsWhenSkillMissing(t *testing.T) {
	home := t.TempDir()
	writeOpenClawConfig(t, home, validOpenClawConfig(`"alsoAllow": ["message"]`))
	checker := checkerWithRepoMirror(t, home, validOpenClawSkillText())

	report, err := checker.Check(context.Background())
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if !reportHas(report, LevelFail, "imgen skill is not installed") {
		t.Fatalf("expected skill failure, got:\n%s", report.Render())
	}
}

func TestOpenClawCheckerFailsWhenSkillMarkersMissing(t *testing.T) {
	home := t.TempDir()
	writeOpenClawConfig(t, home, validOpenClawConfig(`"alsoAllow": ["message"]`))
	writeOpenClawSkill(t, home, "Use imgen.")

	checker := checkerWithRepoMirror(t, home, validOpenClawSkillText())

	report, err := checker.Check(context.Background())
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if !reportHas(report, LevelFail, "missing guidance markers") {
		t.Fatalf("expected marker failure, got:\n%s", report.Render())
	}
}

func TestOpenClawCheckerFailsWhenSkillMissingSyncJSONContract(t *testing.T) {
	home := t.TempDir()
	writeOpenClawConfig(t, home, validOpenClawConfig(`"alsoAllow": ["message"]`))
	writeOpenClawSkill(t, home, "Use ./imgen --json. Deny image_generate. Reply NO_REPLY. Use forceDocument or asDocument.")
	checker := checkerWithRepoMirror(t, home, validOpenClawSkillText())

	report, err := checker.Check(context.Background())
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if !reportHas(report, LevelFail, "missing guidance markers") {
		t.Fatalf("expected marker failure, got:\n%s", report.Render())
	}
	if !reportHas(report, LevelFail, "sync JSON success") {
		t.Fatalf("expected sync JSON contract marker failure, got:\n%s", report.Render())
	}
}

func TestOpenClawCheckerFailsWhenSkillUsesGenericImagesAndPathWords(t *testing.T) {
	home := t.TempDir()
	writeOpenClawConfig(t, home, validOpenClawConfig(`"alsoAllow": ["message"]`))
	writeOpenClawSkill(t, home, "Use ./imgen --json. Deny image_generate. Reply NO_REPLY. Use forceDocument or asDocument. Treat synchronous success as ok=true. images may be returned and a path may appear elsewhere. Image status can be done.")
	checker := checkerWithRepoMirror(t, home, validOpenClawSkillText())

	report, err := checker.Check(context.Background())
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if !reportHas(report, LevelFail, "sync JSON success") {
		t.Fatalf("expected sync JSON contract marker failure, got:\n%s", report.Render())
	}
}

func TestOpenClawCheckerReportsMessageSendForceDocumentSupport(t *testing.T) {
	home := t.TempDir()
	writeOpenClawConfig(t, home, validOpenClawConfig(`"alsoAllow": ["message"]`))
	checker := checkerWithSyncedRepo(t, home)

	report, err := checker.Check(context.Background())
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if !reportHas(report, LevelOK, "openclaw CLI message send supports --force-document") {
		t.Fatalf("expected force-document OK, got:\n%s", report.Render())
	}
	if !reportHas(report, LevelOK, "installed OpenClaw imgen skill matches Claude source") {
		t.Fatalf("expected installed sync OK, got:\n%s", report.Render())
	}
}

func TestOpenClawCheckerReportsRepositorySkillMirrorDrift(t *testing.T) {
	home := t.TempDir()
	repoRoot := t.TempDir()
	writeOpenClawConfig(t, home, validOpenClawConfig(`"alsoAllow": ["message"]`))
	writeOpenClawSkill(t, home, validOpenClawSkillText())
	writeRepoSkillFile(t, repoRoot, ".claude", "SKILL.md", validOpenClawSkillText())
	writeRepoSkillFile(t, repoRoot, ".openclaw", "SKILL.md", "changed")
	checker := checkerWithOpenClawSupport(home)
	checker.RepoRoot = repoRoot

	report, err := checker.Check(context.Background())
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if !reportHas(report, LevelFail, "repository OpenClaw skill mirror drift") {
		t.Fatalf("expected repository mirror drift, got:\n%s", report.Render())
	}
}

func TestOpenClawCheckerReportsInstalledOpenClawSkillDrift(t *testing.T) {
	home := t.TempDir()
	repoRoot := t.TempDir()
	writeOpenClawConfig(t, home, validOpenClawConfig(`"alsoAllow": ["message"]`))
	writeOpenClawSkill(t, home, "changed installed skill")
	writeRepoSkillFile(t, repoRoot, ".claude", "SKILL.md", validOpenClawSkillText())
	writeRepoSkillFile(t, repoRoot, ".openclaw", "SKILL.md", validOpenClawSkillText())
	checker := checkerWithOpenClawSupport(home)
	checker.RepoRoot = repoRoot

	report, err := checker.Check(context.Background())
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if !reportHas(report, LevelFail, "installed OpenClaw imgen skill drift") {
		t.Fatalf("expected installed skill drift, got:\n%s", report.Render())
	}
}

func TestOpenClawCheckerFailsWhenMessageSendForceDocumentMissing(t *testing.T) {
	home := t.TempDir()
	writeOpenClawConfig(t, home, validOpenClawConfig(`"alsoAllow": ["message"]`))
	checker := checkerWithSyncedRepo(t, home)
	checker.RunCommand = func(ctx context.Context, name string, args ...string) (string, error) {
		return "Usage: openclaw message send\n  --media <path-or-url>\n", nil
	}

	report, err := checker.Check(context.Background())
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if !reportHas(report, LevelFail, "openclaw CLI message send does not expose --force-document") {
		t.Fatalf("expected force-document failure, got:\n%s", report.Render())
	}
}

func TestOpenClawCheckerWarnsWhenOpenClawCLIMissing(t *testing.T) {
	home := t.TempDir()
	writeOpenClawConfig(t, home, validOpenClawConfig(`"alsoAllow": ["message"]`))
	checker := checkerWithSyncedRepo(t, home)
	checker.LookPath = func(name string) (string, error) {
		return "", exec.ErrNotFound
	}

	report, err := checker.Check(context.Background())
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if report.Failed() {
		t.Fatalf("missing CLI should warn, not fail:\n%s", report.Render())
	}
	if !reportHas(report, LevelWarn, "openclaw CLI not found on PATH") {
		t.Fatalf("expected CLI warning, got:\n%s", report.Render())
	}
	if !reportHas(report, LevelOK, "repository OpenClaw skill mirror matches Claude source") {
		t.Fatalf("expected repository sync OK, got:\n%s", report.Render())
	}
}

func TestOpenClawCheckerWarnsWhenRepositoryUnavailable(t *testing.T) {
	home := t.TempDir()
	writeOpenClawConfig(t, home, validOpenClawConfig(`"alsoAllow": ["message"]`))
	writeOpenClawSkill(t, home, validOpenClawSkillText())
	checker := checkerWithUnavailableRepo(home)

	report, err := checker.Check(context.Background())
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if report.Failed() {
		t.Fatalf("unavailable repository should warn, not fail:\n%s", report.Render())
	}
	if !reportHas(report, LevelWarn, "repository root not checked: current working directory unavailable") {
		t.Fatalf("expected repository root warning, got:\n%s", report.Render())
	}
}

func checkerWithOpenClawSupport(home string) OpenClawChecker {
	const resolvedPath = "/usr/local/bin/openclaw"

	checker := NewOpenClawChecker(home)
	checker.LookPath = func(name string) (string, error) {
		return resolvedPath, nil
	}
	checker.RunCommand = func(ctx context.Context, name string, args ...string) (string, error) {
		if name != resolvedPath || len(args) != 3 || args[0] != "message" || args[1] != "send" || args[2] != "--help" {
			return "", errors.New("unexpected openclaw command")
		}
		return "Usage: openclaw message send\n  --media <path-or-url>\n  --force-document\n", nil
	}
	return checker
}

func checkerWithSyncedRepo(t *testing.T, home string) OpenClawChecker {
	t.Helper()
	writeOpenClawSkill(t, home, validOpenClawSkillText())
	return checkerWithRepoMirror(t, home, validOpenClawSkillText())
}

func checkerWithRepoMirror(t *testing.T, home string, sourceSkill string) OpenClawChecker {
	t.Helper()
	repoRoot := t.TempDir()
	writeRepoSkillFile(t, repoRoot, ".claude", "SKILL.md", sourceSkill)
	writeRepoSkillFile(t, repoRoot, ".openclaw", "SKILL.md", sourceSkill)
	checker := checkerWithOpenClawSupport(home)
	checker.RepoRoot = repoRoot
	return checker
}

func checkerWithUnavailableRepo(home string) OpenClawChecker {
	checker := checkerWithOpenClawSupport(home)
	checker.Getwd = func() (string, error) {
		return "", errors.New("current working directory unavailable")
	}
	return checker
}

func validOpenClawSkillText() string {
	return "Use ./imgen --json. Deny image_generate. Reply NO_REPLY. Use forceDocument or asDocument. Treat synchronous success as ok=true and images[].path; image status can be done."
}

func validOpenClawConfig(mainTools string) string {
	return `{
		"tools": {"deny": ["image_generate"]},
		"gateway": {"tools": {"deny": ["image_generate"]}},
		"agents": {"list": [{"id": "main", "tools": {` + mainTools + `}}]},
		"surfaces": {"telegram": {"silentReply": {"direct": "allow"}}}
	}`
}

func writeOpenClawConfig(t *testing.T, home string, content string) {
	t.Helper()
	path := filepath.Join(home, ".openclaw", "openclaw.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
}

func writeOpenClawSkill(t *testing.T, home string, content string) {
	t.Helper()
	path := filepath.Join(home, ".openclaw", "workspace", "skills", "imgen", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
}

func writeRepoSkillFile(t *testing.T, repoRoot string, root string, rel string, content string) {
	t.Helper()
	path := filepath.Join(repoRoot, root, "skills", "imgen", rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
}

func reportHas(report Report, level Level, fragment string) bool {
	for _, item := range report.Items {
		if item.Level == level && strings.Contains(item.Message, fragment) {
			return true
		}
	}
	return false
}
