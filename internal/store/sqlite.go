package store

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	schema := `
CREATE TABLE IF NOT EXISTS jobs (
  job_id TEXT PRIMARY KEY,
  prompt TEXT NOT NULL DEFAULT '',
  images_json TEXT NOT NULL DEFAULT '[]',
  requested_count INTEGER NOT NULL DEFAULT 1,
  effective_count INTEGER NOT NULL DEFAULT 1,
  requested_concurrency INTEGER NOT NULL DEFAULT 1,
  effective_concurrency INTEGER NOT NULL DEFAULT 1,
  status TEXT NOT NULL,
  notification_status TEXT NOT NULL DEFAULT 'pending'
);
CREATE TABLE IF NOT EXISTS job_images (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  job_id TEXT NOT NULL,
  image_index INTEGER NOT NULL,
  status TEXT NOT NULL,
  path TEXT NOT NULL DEFAULT '',
  uri TEXT NOT NULL DEFAULT '',
  last_error TEXT NOT NULL DEFAULT '',
  started_at INTEGER NOT NULL DEFAULT 0,
  UNIQUE(job_id, image_index)
);
CREATE TABLE IF NOT EXISTS job_events (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  job_id TEXT NOT NULL,
  event_type TEXT NOT NULL,
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS job_image_attempts (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  job_id TEXT NOT NULL,
  image_index INTEGER NOT NULL,
  attempt INTEGER NOT NULL,
  status TEXT NOT NULL,
  started_at INTEGER NOT NULL DEFAULT 0,
  finished_at INTEGER NOT NULL DEFAULT 0,
  duration_ms INTEGER NOT NULL DEFAULT 0,
  path TEXT NOT NULL DEFAULT '',
  uri TEXT NOT NULL DEFAULT '',
  last_error TEXT NOT NULL DEFAULT '',
  stdout_tail TEXT NOT NULL DEFAULT '',
  stderr_tail TEXT NOT NULL DEFAULT '',
  UNIQUE(job_id, image_index, attempt)
);
CREATE TABLE IF NOT EXISTS job_image_attempt_phases (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  job_id TEXT NOT NULL,
  image_index INTEGER NOT NULL,
  attempt INTEGER NOT NULL,
  phase TEXT NOT NULL,
  occurred_at_ms INTEGER NOT NULL,
  elapsed_ms INTEGER NOT NULL DEFAULT 0,
  detail TEXT NOT NULL DEFAULT '',
  UNIQUE(job_id, image_index, attempt, phase)
);
`
	if _, err := db.Exec(schema); err != nil {
		return nil, err
	}
	migrations := []struct {
		table string
		name  string
		sql   string
	}{
		{"jobs", "notification_status", `ALTER TABLE jobs ADD COLUMN notification_status TEXT NOT NULL DEFAULT 'pending'`},
		{"jobs", "images_json", `ALTER TABLE jobs ADD COLUMN images_json TEXT NOT NULL DEFAULT '[]'`},
		{"job_images", "last_error", `ALTER TABLE job_images ADD COLUMN last_error TEXT NOT NULL DEFAULT ''`},
		{"job_images", "started_at", `ALTER TABLE job_images ADD COLUMN started_at INTEGER NOT NULL DEFAULT 0`},
	}
	for _, m := range migrations {
		has, err := hasColumn(db, m.table, m.name)
		if err != nil {
			return nil, err
		}
		if has {
			continue
		}
		if _, err := db.Exec(m.sql); err != nil {
			return nil, fmt.Errorf("migrate %s.%s: %w", m.table, m.name, err)
		}
	}
	return &Store{db: db}, nil
}

func hasColumn(db *sql.DB, table string, column string) (bool, error) {
	rows, err := db.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, ctype string
		var notNull int
		var dfltValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &ctype, &notNull, &dfltValue, &pk); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}
	return false, rows.Err()
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) HasTable(name string) bool {
	var got string
	err := s.db.QueryRow(`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`, name).Scan(&got)
	return err == nil && got == name
}
