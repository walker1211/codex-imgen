package backend

import (
	"context"
	"os"
	"time"

	"github.com/walker1211/codex-imgen/internal/codex"
	"github.com/walker1211/codex-imgen/internal/parser"
)

type BuiltinCodex struct {
	Command   string
	Model     string
	CWD       string
	Timeout   time.Duration
	CodexHome string
	Runner    codex.Runner
}

func (b BuiltinCodex) Generate(ctx context.Context, req GenerateRequest) (GenerateResult, error) {
	args := []string{"exec", "--json"}
	model := req.Model
	if model == "" {
		model = b.Model
	}
	if model != "" {
		args = append(args, "--model", model)
	}
	cwd := req.CWD
	if cwd == "" {
		cwd = b.CWD
	}
	if cwd != "" {
		args = append(args, "--cd", cwd)
	}
	args = append(args, req.Prompt)

	runner := b.Runner
	result, err := runner.Run(ctx, codex.Request{
		Command: b.command(),
		Args:    args,
		Env:     os.Environ(),
		Timeout: b.timeout(),
	})
	if err != nil {
		return GenerateResult{}, err
	}
	parsed, err := parser.ExtractImageResult(result.Stdout, b.codexHome())
	if err != nil {
		return GenerateResult{}, err
	}
	return GenerateResult{Path: parsed.Path, URI: parsed.URI, RawOutput: result.Stdout + result.Stderr}, nil
}

func (b BuiltinCodex) command() string {
	if b.Command != "" {
		return b.Command
	}
	return "codex"
}

func (b BuiltinCodex) codexHome() string {
	if b.CodexHome != "" {
		return b.CodexHome
	}
	home, _ := os.UserHomeDir()
	return home + "/.codex"
}

func (b BuiltinCodex) timeout() time.Duration {
	if b.Timeout > 0 {
		return b.Timeout
	}
	return 90 * time.Second
}
