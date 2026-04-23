package store

import (
	"database/sql"

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
`
	if _, err := db.Exec(schema); err != nil {
		return nil, err
	}
	return &Store{db: db}, nil
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
