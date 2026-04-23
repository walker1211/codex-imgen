package files

import (
	"path/filepath"
	"testing"
)

func TestJobImagePath(t *testing.T) {
	layout := Layout{DataDir: t.TempDir()}
	path := layout.ImagePath("job_123", 2)
	want := filepath.Join(layout.DataDir, "jobs", "job_123", "images", "2.png")
	if path != want {
		t.Fatalf("path = %q, want %q", path, want)
	}
}
