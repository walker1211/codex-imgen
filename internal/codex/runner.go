package codex

import (
	"bytes"
	"context"
	"os/exec"
	"time"
)

type Request struct {
	Command string
	Args    []string
	Dir     string
	Env     []string
	Timeout time.Duration
}

type RunResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

type Runner struct{}

func (Runner) Run(ctx context.Context, req Request) (RunResult, error) {
	if req.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, req.Timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, req.Command, req.Args...)
	cmd.Dir = req.Dir
	cmd.Env = req.Env

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result := RunResult{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}
	if err == nil {
		return result, nil
	}
	if ctx.Err() != nil {
		return result, ctx.Err()
	}

	if exitErr, ok := err.(*exec.ExitError); ok {
		result.ExitCode = exitErr.ExitCode()
	}
	return result, err
}
