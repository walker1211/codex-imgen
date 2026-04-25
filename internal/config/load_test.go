package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDefaultPathUsesConfigsDirectory(t *testing.T) {
	if got, want := DefaultPath(), filepath.Join("configs", "config.yaml"); got != want {
		t.Fatalf("DefaultPath() = %q, want %q", got, want)
	}
}

func TestLoadMissingDefaultPathReturnsDefaults(t *testing.T) {
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldWD); err != nil {
			t.Fatalf("Chdir cleanup failed: %v", err)
		}
	})
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}

	cfg, err := Load(DefaultPath())
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Server.Listen != Default().Server.Listen {
		t.Fatalf("listen = %q", cfg.Server.Listen)
	}
}

func TestLoadReadsDotEnv(t *testing.T) {
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldWD); err != nil {
			t.Fatalf("Chdir cleanup failed: %v", err)
		}
	})
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("EMAIL_SMTP_AUTH_CODE=test-secret\n"), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}
	t.Setenv("EMAIL_SMTP_AUTH_CODE", "")

	if _, err := Load(DefaultPath()); err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if got := os.Getenv("EMAIL_SMTP_AUTH_CODE"); got != "" {
		t.Fatalf("EMAIL_SMTP_AUTH_CODE = %q", got)
	}

	_ = os.Unsetenv("EMAIL_SMTP_AUTH_CODE")
	if _, err := Load(DefaultPath()); err != nil {
		t.Fatalf("Load returned error after unset: %v", err)
	}
	if got := os.Getenv("EMAIL_SMTP_AUTH_CODE"); got != "test-secret" {
		t.Fatalf("EMAIL_SMTP_AUTH_CODE = %q", got)
	}
}

func TestLoadExampleConfig(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	path := filepath.Join(filepath.Dir(filename), "..", "..", "configs", "config.example.yaml")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Server.Listen != "127.0.0.1:18080" {
		t.Fatalf("listen = %q", cfg.Server.Listen)
	}
	if cfg.Backend.Type != "built_in_codex" {
		t.Fatalf("backend.type = %q", cfg.Backend.Type)
	}
	if cfg.Backend.Command != "codex" {
		t.Fatalf("backend.command = %q", cfg.Backend.Command)
	}
	if cfg.Backend.Prompt.Prefix != "$imagegen" {
		t.Fatalf("backend.prompt.prefix = %q", cfg.Backend.Prompt.Prefix)
	}
	if cfg.Scheduler.MaxAttempts != 3 {
		t.Fatalf("max attempts = %d", cfg.Scheduler.MaxAttempts)
	}
}

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
