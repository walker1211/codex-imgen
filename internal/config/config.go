package config

import "time"

type Config struct {
	Codex  CodexConfig  `yaml:"codex"`
	Prompt PromptConfig `yaml:"prompt"`
	Output OutputConfig `yaml:"output"`
}

type CodexConfig struct {
	Command    string        `yaml:"command"`
	CWD        string        `yaml:"cwd"`
	Model      string        `yaml:"model"`
	Timeout    time.Duration `yaml:"timeout"`
	ExtraFlags []string      `yaml:"extra_flags"`
}

type PromptConfig struct {
	Prefix  string `yaml:"prefix"`
	Prelude string `yaml:"prelude"`
}

type OutputConfig struct {
	Format string `yaml:"format"`
}

func Default() Config {
	return Config{
		Codex: CodexConfig{
			Command: "codex",
			Timeout: 90 * time.Second,
		},
		Prompt: PromptConfig{
			Prefix: "$imagegen",
		},
		Output: OutputConfig{
			Format: "text",
		},
	}
}
