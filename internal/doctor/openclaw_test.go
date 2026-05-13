package doctor

import (
	"context"
	"os"
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
	writeOpenClawSkill(t, home, "Use ./imgen --json. Deny image_generate. Reply NO_REPLY. Use forceDocument or asDocument.")

	report, err := NewOpenClawChecker(home).Check(context.Background())
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if report.Failed() {
		t.Fatalf("expected no failures, got:\n%s", report.Render())
	}
	if !reportHas(report, LevelWarn, "active-memory targets main") {
		t.Fatalf("expected active-memory warning, got:\n%s", report.Render())
	}
	if !strings.HasPrefix(report.Render(), "OpenClaw doctor\n") {
		t.Fatalf("rendered report = %q", report.Render())
	}
}

func TestOpenClawCheckerFailsWhenMessageMissing(t *testing.T) {
	home := t.TempDir()
	writeOpenClawConfig(t, home, validOpenClawConfig(`"alsoAllow": []`))
	writeOpenClawSkill(t, home, "Use ./imgen --json. Deny image_generate. Reply NO_REPLY. Use forceDocument or asDocument.")

	report, err := NewOpenClawChecker(home).Check(context.Background())
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
	writeOpenClawSkill(t, home, "Use ./imgen --json. Deny image_generate. Reply NO_REPLY. Use forceDocument or asDocument.")

	report, err := NewOpenClawChecker(home).Check(context.Background())
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

	report, err := NewOpenClawChecker(home).Check(context.Background())
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

	report, err := NewOpenClawChecker(home).Check(context.Background())
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if !reportHas(report, LevelFail, "missing guidance markers") {
		t.Fatalf("expected marker failure, got:\n%s", report.Render())
	}
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

func reportHas(report Report, level Level, fragment string) bool {
	for _, item := range report.Items {
		if item.Level == level && strings.Contains(item.Message, fragment) {
			return true
		}
	}
	return false
}
