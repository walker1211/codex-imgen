package scripts_test

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReleaseWorkflowUsesReleaseNotesFile(t *testing.T) {
	content, err := os.ReadFile(filepath.Join(repoRoot(t), ".github", "workflows", "release.yml"))
	if err != nil {
		t.Fatalf("ReadFile release workflow failed: %v", err)
	}
	text := string(content)

	assertContains(t, text, `notes_file="docs/releases/${GITHUB_REF_NAME}.md"`)
	assertContains(t, text, `--notes-file "$notes_file"`)
	assertContains(t, text, "gh release edit")
	assertNotContains(t, text, "--generate-notes")
}
