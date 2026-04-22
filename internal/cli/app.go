package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/walker1211/codex-imgen/internal/codex"
	"github.com/walker1211/codex-imgen/internal/config"
	"github.com/walker1211/codex-imgen/internal/parser"
	"github.com/walker1211/codex-imgen/internal/result"
)

type RunRequest = codex.Request
type RunResponse = codex.RunResult

type Runner interface {
	Run(context.Context, RunRequest) (RunResponse, error)
}

type App struct {
	Stdout     io.Writer
	Stderr     io.Writer
	Runner     Runner
	ConfigPath string
	CodexHome  string
}

func (a App) configPath() string {
	if a.ConfigPath != "" {
		return a.ConfigPath
	}
	return filepath.Join(os.Getenv("HOME"), ".config", "codex-imgen", "config.yaml")
}

func (a App) codexHome() string {
	if a.CodexHome != "" {
		return a.CodexHome
	}
	return filepath.Join(os.Getenv("HOME"), ".codex")
}

func (a App) Run(ctx context.Context, args []string) int {
	opts, err := ParseOptions(args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprint(a.Stdout, UsageText())
			return 0
		}
		fmt.Fprintln(a.Stderr, err.Error())
		return 1
	}

	cfg, err := config.Load(a.configPath())
	if err != nil {
		fmt.Fprintln(a.Stderr, err.Error())
		return 1
	}

	command := cfg.Codex.Command
	if command == "" {
		command = "codex"
	}
	cwd := cfg.Codex.CWD
	if opts.CWD != "" {
		cwd = opts.CWD
	}
	model := cfg.Codex.Model
	if opts.Model != "" {
		model = opts.Model
	}
	timeout := cfg.Codex.Timeout
	if opts.Timeout > 0 {
		timeout = opts.Timeout
	}
	if timeout == 0 {
		timeout = 90 * time.Second
	}

	assembledPrompt := codex.BuildPrompt(cfg.Prompt.Prelude, cfg.Prompt.Prefix, opts.Prompt)
	cmdArgs := []string{"exec", "--json"}
	cmdArgs = append(cmdArgs, cfg.Codex.ExtraFlags...)
	if model != "" {
		cmdArgs = append(cmdArgs, "--model", model)
	}
	if cwd != "" {
		cmdArgs = append(cmdArgs, "--cd", cwd)
	}
	cmdArgs = append(cmdArgs, assembledPrompt)

	if opts.Verbose {
		fmt.Fprintf(a.Stderr, "codex command: %s %v\n", command, cmdArgs[:len(cmdArgs)-1])
		fmt.Fprintf(a.Stderr, "working directory: %s\n", cwd)
		fmt.Fprintf(a.Stderr, "codex home: %s\n", a.codexHome())
	}

	runner := a.Runner
	if runner == nil {
		runner = codex.Runner{}
	}
	runResult, err := runner.Run(ctx, RunRequest{
		Command: command,
		Args:    cmdArgs,
		Dir:     cwd,
		Env:     os.Environ(),
		Timeout: timeout,
	})
	rawOutput := runResult.Stdout + runResult.Stderr
	if opts.Raw {
		fmt.Fprint(a.Stderr, runResult.Stdout)
		fmt.Fprint(a.Stderr, runResult.Stderr)
	}
	jsonMode := opts.JSON || cfg.Output.Format == "json"
	if err != nil {
		return a.writeFailure(opts, rawOutput, jsonMode, err)
	}

	parsed, err := parser.ExtractImageResult(runResult.Stdout, a.codexHome())
	if err != nil {
		return a.writeFailure(opts, rawOutput, jsonMode, err)
	}

	res := result.Result{
		OK:              true,
		Prompt:          opts.Prompt,
		AssembledPrompt: assembledPrompt,
		URI:             parsed.URI,
		Path:            parsed.Path,
		RawOutput:       rawOutput,
	}
	if jsonMode {
		fmt.Fprint(a.Stdout, result.RenderJSON(res))
	} else {
		fmt.Fprint(a.Stdout, result.RenderText(res))
	}
	return 0
}

func (a App) writeFailure(opts Options, rawOutput string, jsonMode bool, err error) int {
	res := result.Result{
		OK:        false,
		Prompt:    opts.Prompt,
		Error:     err.Error(),
		RawOutput: rawOutput,
	}
	if jsonMode {
		fmt.Fprint(a.Stdout, result.RenderJSON(res))
	} else {
		fmt.Fprint(a.Stderr, result.RenderText(res))
	}
	return 1
}
