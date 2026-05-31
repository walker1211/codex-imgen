package skillsync

import (
	"flag"
	"fmt"
	"io"
)

type CommandContext struct {
	Stdout      io.Writer
	Stderr      io.Writer
	Getwd       func() (string, error)
	UserHomeDir func() (string, error)
}

func Run(args []string, ctx CommandContext) int {
	stdout := ctx.Stdout
	stderr := ctx.Stderr
	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}

	fs := flag.NewFlagSet("skill-sync", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	apply := fs.Bool("apply", false, "copy repository skill sources to local installs")
	check := fs.Bool("check", false, "check whether local installs match repository skill sources")
	repoRootFlag := fs.String("repo-root", "", "repository root; defaults to nearest codex-imgen root")
	claudeDir := fs.String("claude-dir", "", "Claude skill install directory")
	openClawDir := fs.String("openclaw-dir", "", "OpenClaw workspace skill install directory")
	codexDir := fs.String("codex-dir", "", "Codex skill install directory")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(stderr, "unexpected positional arguments")
		return 2
	}
	if *apply && *check {
		fmt.Fprintln(stderr, "choose either --check or --apply")
		return 2
	}

	repoRoot := *repoRootFlag
	if repoRoot == "" {
		if ctx.Getwd == nil {
			fmt.Fprintln(stderr, "current working directory lookup is not configured")
			return 2
		}
		cwd, err := ctx.Getwd()
		if err != nil {
			fmt.Fprintln(stderr, err.Error())
			return 1
		}
		found, err := FindRepositoryRoot(cwd)
		if err != nil {
			fmt.Fprintln(stderr, err.Error())
			return 1
		}
		repoRoot = found
	}

	if ctx.UserHomeDir == nil {
		fmt.Fprintln(stderr, "home directory lookup is not configured")
		return 2
	}
	home, err := ctx.UserHomeDir()
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 1
	}
	paths := DefaultPaths(repoRoot, home)
	if *claudeDir != "" {
		paths = paths.WithClaudeInstallDir(*claudeDir)
	}
	if *openClawDir != "" {
		paths = paths.WithOpenClawInstallDir(*openClawDir)
	}
	if *codexDir != "" {
		paths = paths.WithCodexInstallDir(*codexDir)
	}

	if *apply {
		result, err := paths.Apply()
		if err != nil {
			fmt.Fprintln(stderr, err.Error())
			return 1
		}
		for _, path := range result.Applied {
			fmt.Fprintf(stdout, "copied %s\n", path)
		}
		return 0
	}

	result, err := paths.Check()
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 1
	}
	if len(result.Drift) > 0 {
		fmt.Fprintln(stdout, "skill installs are out of sync:")
		for _, line := range result.Drift {
			fmt.Fprintf(stdout, "- %s\n", line)
		}
		return 1
	}
	fmt.Fprintln(stdout, "skill installs are up to date")
	return 0
}
