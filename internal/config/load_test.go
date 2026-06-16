package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
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

func TestDefaultRealtimeConfig(t *testing.T) {
	cfg := Default()

	if !cfg.Realtime.Enabled {
		t.Fatal("expected realtime enabled by default")
	}
	if cfg.Realtime.MaxSessions != 4 {
		t.Fatalf("realtime.max_sessions = %d", cfg.Realtime.MaxSessions)
	}
	if cfg.Realtime.MaxItemsPerSession != 8 {
		t.Fatalf("realtime.max_items_per_session = %d", cfg.Realtime.MaxItemsPerSession)
	}
	if cfg.Realtime.MaxCountPerItem != 1 {
		t.Fatalf("realtime.max_count_per_item = %d", cfg.Realtime.MaxCountPerItem)
	}
	if cfg.Realtime.ItemTimeout != 300*time.Second {
		t.Fatalf("realtime.item_timeout = %s", cfg.Realtime.ItemTimeout)
	}
	if cfg.Realtime.MaxItemTimeout != 300*time.Second {
		t.Fatalf("realtime.max_item_timeout = %s", cfg.Realtime.MaxItemTimeout)
	}
}

func TestLoadParsesRealtimeConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := []byte(`realtime:
  enabled: false
  max_sessions: 6
  max_items_per_session: 9
  max_count_per_item: 2
  item_timeout: 45s
  max_item_timeout: 90s
`)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.Realtime.Enabled {
		t.Fatal("expected realtime disabled")
	}
	if cfg.Realtime.MaxSessions != 6 {
		t.Fatalf("realtime.max_sessions = %d", cfg.Realtime.MaxSessions)
	}
	if cfg.Realtime.MaxItemsPerSession != 9 {
		t.Fatalf("realtime.max_items_per_session = %d", cfg.Realtime.MaxItemsPerSession)
	}
	if cfg.Realtime.MaxCountPerItem != 2 {
		t.Fatalf("realtime.max_count_per_item = %d", cfg.Realtime.MaxCountPerItem)
	}
	if cfg.Realtime.ItemTimeout != 45*time.Second {
		t.Fatalf("realtime.item_timeout = %s", cfg.Realtime.ItemTimeout)
	}
	if cfg.Realtime.MaxItemTimeout != 90*time.Second {
		t.Fatalf("realtime.max_item_timeout = %s", cfg.Realtime.MaxItemTimeout)
	}
}

func TestLoadParsesSchedulerJobConcurrencyFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := []byte(`scheduler:
  default_job_concurrency: 2
  max_job_concurrency: 10
`)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Scheduler.DefaultJobConcurrency != 2 {
		t.Fatalf("scheduler.default_job_concurrency = %d", cfg.Scheduler.DefaultJobConcurrency)
	}
	if cfg.Scheduler.MaxJobConcurrency != 10 {
		t.Fatalf("scheduler.max_job_concurrency = %d", cfg.Scheduler.MaxJobConcurrency)
	}
}

func TestLoadParsesStorageImageInputDir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := []byte(`storage:
  image_input_dir: ~/imgen-inputs
`)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir returned error: %v", err)
	}
	want := filepath.Join(home, "imgen-inputs")
	if cfg.Storage.ImageInputDir != want {
		t.Fatalf("storage.image_input_dir = %q, want %q", cfg.Storage.ImageInputDir, want)
	}
}

func TestLoadParsesBackendDeliveryDir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := []byte(`backend:
  delivery_dir: ./openclaw-media
`)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Backend.DeliveryDir != "./openclaw-media" {
		t.Fatalf("backend.delivery_dir = %q", cfg.Backend.DeliveryDir)
	}
}

func TestLoadParsesBackendDeliveryMaxFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := []byte(`backend:
  delivery_max_files: 42
`)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Backend.DeliveryMaxFiles != 42 {
		t.Fatalf("backend.delivery_max_files = %d", cfg.Backend.DeliveryMaxFiles)
	}
}

func TestLoadParsesBackendCleanupSourceThreadDir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := []byte(`backend:
  cleanup_source_thread_dir: true
`)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if !cfg.Backend.CleanupSourceThreadDir {
		t.Fatal("backend.cleanup_source_thread_dir = false, want true")
	}
}

func TestDefaultDoesNotCleanupSourceThreadDir(t *testing.T) {
	cfg := Default()
	if cfg.Backend.CleanupSourceThreadDir {
		t.Fatal("backend.cleanup_source_thread_dir = true, want false")
	}
}

func TestDefaultLimitsDeliveryDirFiles(t *testing.T) {
	cfg := Default()
	if cfg.Backend.DeliveryMaxFiles != 200 {
		t.Fatalf("backend.delivery_max_files = %d, want 200", cfg.Backend.DeliveryMaxFiles)
	}
}

func TestLoadReadsDeliveryDirEnv(t *testing.T) {
	deliveryDir := filepath.Join(t.TempDir(), "media")
	t.Setenv("IMGEN_DELIVERY_DIR", deliveryDir)

	cfg, err := Load(filepath.Join(t.TempDir(), "missing.yaml"))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Backend.DeliveryDir != deliveryDir {
		t.Fatalf("backend.delivery_dir = %q, want %q", cfg.Backend.DeliveryDir, deliveryDir)
	}
}

func TestLoadRejectsDeprecatedRealtimeConcurrencyFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := []byte(`realtime:
  max_concurrency_per_session: 3
  global_concurrency: 5
`)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected deprecated realtime concurrency fields to be rejected")
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
	cfg, err := Load(DefaultPath())
	if err != nil {
		t.Fatalf("Load returned error after unset: %v", err)
	}
	if got := os.Getenv("EMAIL_SMTP_AUTH_CODE"); got != "test-secret" {
		t.Fatalf("EMAIL_SMTP_AUTH_CODE = %q", got)
	}
	if cfg.Email.SMTPAuthCode != "test-secret" {
		t.Fatalf("cfg.Email.SMTPAuthCode = %q", cfg.Email.SMTPAuthCode)
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
	if !cfg.Realtime.Enabled {
		t.Fatal("expected realtime enabled")
	}
	if cfg.Scheduler.GlobalMaxConcurrency != 10 {
		t.Fatalf("scheduler.global_max_concurrency = %d", cfg.Scheduler.GlobalMaxConcurrency)
	}
	if cfg.Realtime.MaxSessions != 4 {
		t.Fatalf("realtime.max_sessions = %d", cfg.Realtime.MaxSessions)
	}
	if cfg.Realtime.ItemTimeout != 300*time.Second {
		t.Fatalf("realtime.item_timeout = %s", cfg.Realtime.ItemTimeout)
	}
	if cfg.Realtime.MaxItemTimeout != 300*time.Second {
		t.Fatalf("realtime.max_item_timeout = %s", cfg.Realtime.MaxItemTimeout)
	}
}

func TestLoadParsesServiceConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := []byte(`server:
  listen: 127.0.0.1:18080
scheduler:
  global_max_concurrency: 10
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
