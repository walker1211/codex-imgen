package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadUsesDefaultsWhenFileMissing(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "missing.yaml"))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.Codex.Command != "codex" {
		t.Fatalf("command = %q", cfg.Codex.Command)
	}
	if cfg.Prompt.Prefix != "$imagegen" {
		t.Fatalf("prefix = %q", cfg.Prompt.Prefix)
	}
	if cfg.Codex.Timeout != 90*time.Second {
		t.Fatalf("timeout = %v", cfg.Codex.Timeout)
	}
}

func TestLoadExpandsHomeDirectory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := []byte("codex:\n  cwd: ~/work/demo\n")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	want := filepath.Join(home, "work", "demo")
	if cfg.Codex.CWD != want {
		t.Fatalf("cwd = %q, want %q", cfg.Codex.CWD, want)
	}
}

func TestLoadRejectsUnknownFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := []byte("codex:\n  unknown_field: true\n")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
