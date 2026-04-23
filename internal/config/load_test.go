package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadParsesServiceConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := []byte(`server:
  listen: 127.0.0.1:18080
scheduler:
  global_max_concurrency: 10
  default_job_concurrency: 2
backend:
  type: built_in_codex
  command: codex
email:
  enabled: true
  smtp_host: smtp.example.com
  smtp_port: 465
  from: from@example.com
  to: to@example.com
`)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.Server.Listen != "127.0.0.1:18080" {
		t.Fatalf("listen = %q", cfg.Server.Listen)
	}
	if cfg.Scheduler.GlobalMaxConcurrency != 10 {
		t.Fatalf("global max = %d", cfg.Scheduler.GlobalMaxConcurrency)
	}
	if cfg.Backend.Type != "built_in_codex" {
		t.Fatalf("backend.type = %q", cfg.Backend.Type)
	}
	if !cfg.Email.Enabled {
		t.Fatal("expected email enabled")
	}
}
