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
	if !store.HasTable("job_image_attempts") {
		t.Fatal("expected job_image_attempts table to be created")
	}
	assertJobImageAttemptsSchema(t, store)
	assertJobImageAttemptsUniqueConstraint(t, store)
	assertJobImageAttemptPhasesTable(t, store)
}

func assertJobImageAttemptsSchema(t *testing.T, store *Store) {
	t.Helper()

	rows, err := store.db.Query(`PRAGMA table_info(job_image_attempts)`)
	if err != nil {
		t.Fatalf("PRAGMA table_info returned error: %v", err)
	}
	defer rows.Close()

	columns := map[string]bool{}
	for rows.Next() {
		var cid int
		var name, ctype string
		var notNull int
		var dfltValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &ctype, &notNull, &dfltValue, &pk); err != nil {
			t.Fatalf("Scan returned error: %v", err)
		}
		columns[name] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows returned error: %v", err)
	}

	for _, name := range []string{"job_id", "image_index", "attempt", "status", "started_at", "finished_at", "duration_ms", "path", "uri", "last_error", "stdout_tail", "stderr_tail"} {
		if !columns[name] {
			t.Fatalf("expected job_image_attempts.%s column", name)
		}
	}
}

func assertJobImageAttemptsUniqueConstraint(t *testing.T, store *Store) {
	t.Helper()

	_, err := store.db.Exec(`INSERT INTO job_image_attempts (job_id, image_index, attempt, status) VALUES ('job_unique', 1, 1, 'running')`)
	if err != nil {
		t.Fatalf("insert attempt returned error: %v", err)
	}
	_, err = store.db.Exec(`INSERT INTO job_image_attempts (job_id, image_index, attempt, status) VALUES ('job_unique', 1, 1, 'running')`)
	if err == nil {
		t.Fatal("expected duplicate job_image_attempts insert to fail")
	}
}

func assertJobImageAttemptPhasesTable(t *testing.T, store *Store) {
	t.Helper()
	if !store.HasTable("job_image_attempt_phases") {
		t.Fatal("expected job_image_attempt_phases table to be created")
	}
	assertJobImageAttemptPhasesSchema(t, store)
	assertJobImageAttemptPhasesUniqueConstraint(t, store)
}

func assertJobImageAttemptPhasesSchema(t *testing.T, store *Store) {
	t.Helper()

	rows, err := store.db.Query(`PRAGMA table_info(job_image_attempt_phases)`)
	if err != nil {
		t.Fatalf("PRAGMA table_info phases returned error: %v", err)
	}
	defer rows.Close()

	columns := map[string]bool{}
	for rows.Next() {
		var cid int
		var name, ctype string
		var notNull int
		var dfltValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &ctype, &notNull, &dfltValue, &pk); err != nil {
			t.Fatalf("Scan phase column returned error: %v", err)
		}
		columns[name] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("phase rows returned error: %v", err)
	}

	for _, name := range []string{"id", "job_id", "image_index", "attempt", "phase", "occurred_at_ms", "elapsed_ms", "detail"} {
		if !columns[name] {
			t.Fatalf("expected job_image_attempt_phases.%s column", name)
		}
	}
}

func assertJobImageAttemptPhasesUniqueConstraint(t *testing.T, store *Store) {
	t.Helper()

	_, err := store.db.Exec(`INSERT INTO job_image_attempt_phases (job_id, image_index, attempt, phase, occurred_at_ms) VALUES ('job_unique_phase', 1, 1, 'process.started', 1000)`)
	if err != nil {
		t.Fatalf("insert phase returned error: %v", err)
	}
	_, err = store.db.Exec(`INSERT INTO job_image_attempt_phases (job_id, image_index, attempt, phase, occurred_at_ms) VALUES ('job_unique_phase', 1, 1, 'process.started', 1000)`)
	if err == nil {
		t.Fatal("expected duplicate job_image_attempt_phases insert to fail")
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
	if !store.HasTable("job_image_attempts") {
		t.Fatal("expected job_image_attempts table to be created")
	}
	assertJobImageAttemptsSchema(t, store)
	assertJobImageAttemptsUniqueConstraint(t, store)
	assertJobImageAttemptPhasesTable(t, store)
}
