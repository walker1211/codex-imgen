package store

import (
	"path/filepath"
	"testing"
)

func TestOpenCreatesSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "imgen.db")
	store, err := Open(path)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer store.Close()

	for _, table := range []string{"jobs", "job_images", "job_events"} {
		if !store.HasTable(table) {
			t.Fatalf("expected table %q to exist", table)
		}
	}
}
