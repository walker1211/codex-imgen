package store

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestOpenMigratesLegacySchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "imgen.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("sql.Open returned error: %v", err)
	}
	defer db.Close()

	legacySchema := `
CREATE TABLE jobs (
  job_id TEXT PRIMARY KEY,
  prompt TEXT NOT NULL DEFAULT '',
  requested_count INTEGER NOT NULL DEFAULT 1,
  effective_count INTEGER NOT NULL DEFAULT 1,
  requested_concurrency INTEGER NOT NULL DEFAULT 1,
  effective_concurrency INTEGER NOT NULL DEFAULT 1,
  status TEXT NOT NULL
);
CREATE TABLE job_images (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  job_id TEXT NOT NULL,
  image_index INTEGER NOT NULL,
  status TEXT NOT NULL,
  path TEXT NOT NULL DEFAULT '',
  uri TEXT NOT NULL DEFAULT '',
  UNIQUE(job_id, image_index)
);
`
	if _, err := db.Exec(legacySchema); err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO jobs (job_id, prompt, requested_count, effective_count, requested_concurrency, effective_concurrency, status) VALUES ('job_1', 'x', 1, 1, 1, 1, 'queued')`); err != nil {
		t.Fatalf("insert job returned error: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO job_images (job_id, image_index, status, path, uri) VALUES ('job_1', 1, 'queued', '', '')`); err != nil {
		t.Fatalf("insert image returned error: %v", err)
	}
	db.Close()

	store, err := Open(path)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer store.Close()

	job, images, err := store.GetJob(t.Context(), "job_1")
	if err != nil {
		t.Fatalf("GetJob returned error: %v", err)
	}
	if job.NotificationStatus != "pending" {
		t.Fatalf("notification status = %q", job.NotificationStatus)
	}
	if images[0].LastError != "" {
		t.Fatalf("last error = %q", images[0].LastError)
	}
}

func TestOpenMigratesLegacySchemaWithImagesJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "imgen.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("sql.Open returned error: %v", err)
	}
	defer db.Close()

	legacySchema := `
CREATE TABLE jobs (
  job_id TEXT PRIMARY KEY,
  prompt TEXT NOT NULL DEFAULT '',
  requested_count INTEGER NOT NULL DEFAULT 1,
  effective_count INTEGER NOT NULL DEFAULT 1,
  requested_concurrency INTEGER NOT NULL DEFAULT 1,
  effective_concurrency INTEGER NOT NULL DEFAULT 1,
  status TEXT NOT NULL
);
CREATE TABLE job_images (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  job_id TEXT NOT NULL,
  image_index INTEGER NOT NULL,
  status TEXT NOT NULL,
  path TEXT NOT NULL DEFAULT '',
  uri TEXT NOT NULL DEFAULT '',
  UNIQUE(job_id, image_index)
);
`
	if _, err := db.Exec(legacySchema); err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO jobs (job_id, prompt, requested_count, effective_count, requested_concurrency, effective_concurrency, status) VALUES ('job_1', 'x', 1, 1, 1, 1, 'queued')`); err != nil {
		t.Fatalf("insert job returned error: %v", err)
	}
	db.Close()

	store, err := Open(path)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer store.Close()

	job, _, err := store.GetJob(t.Context(), "job_1")
	if err != nil {
		t.Fatalf("GetJob returned error: %v", err)
	}
	if job.ImagesJSON != "[]" {
		t.Fatalf("images_json = %q", job.ImagesJSON)
	}
}
