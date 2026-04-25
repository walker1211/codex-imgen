package config

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
)

func DefaultPath() string {
	return filepath.Join("configs", "config.yaml")
}

func Load(path string) (Config, error) {
	_ = godotenv.Load()
	cfg := Default()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return Config{}, err
	}

	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil {
		return Config{}, err
	}
	cfg.Storage.DataDir = expandHome(cfg.Storage.DataDir)
	cfg.Storage.SQLitePath = expandHome(cfg.Storage.SQLitePath)
	cfg.Backend.CWD = expandHome(cfg.Backend.CWD)
	return cfg, nil
}

func expandHome(path string) string {
	if !strings.HasPrefix(path, "~/") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, strings.TrimPrefix(path, "~/"))
}
