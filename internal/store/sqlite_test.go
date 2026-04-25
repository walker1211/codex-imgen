package store

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func hasColumnForTest(t *testing.T, db *sql.DB, table string, column string) bool {
	t.Helper()
	has, err := hasColumn(db, table, column)
	if err != nil {
		t.Fatalf("hasColumn returned error: %v", err)
	}
	return has
}

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

func TestOpenCreatesSchemaWithImagesJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "imgen.db")
	store, err := Open(path)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer store.Close()
	if !hasColumnForTest(t, store.db, "jobs", "images_json") {
		t.Fatal("expected jobs.images_json column")
	}
}
