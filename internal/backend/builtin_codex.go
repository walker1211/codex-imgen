package backend

import (
	"context"
	"fmt"
	"os"
	"strings"
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
		return GenerateResult{}, fmt.Errorf("codex exec failed: %s", formatCommandError(err, result))
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

func formatCommandError(err error, result codex.RunResult) string {
	parts := []string{"execution failed"}
	if err == context.DeadlineExceeded {
		parts = append(parts, "deadline exceeded")
	} else if err != nil {
		parts = append(parts, err.Error())
	}
	if result.ExitCode != 0 {
		parts = append(parts, fmt.Sprintf("exit_code=%d", result.ExitCode))
	}
	stderr := summarizeOutput(result.Stderr)
	if stderr != "" {
		parts = append(parts, fmt.Sprintf("stderr: %s", stderr))
	}
	stdout := summarizeOutput(result.Stdout)
	if stdout != "" {
		parts = append(parts, fmt.Sprintf("stdout: %s", stdout))
	}
	return strings.Join(parts, "; ")
}

func summarizeOutput(output string) string {
	output = strings.TrimSpace(output)
	if output == "" {
		return ""
	}
	output = strings.ReplaceAll(output, "\n", " | ")
	if len(output) <= 200 {
		return output
	}
	return output[:200] + "..."
}
