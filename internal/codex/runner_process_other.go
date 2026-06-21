//go:build !windows && !unix

package codex

import (
	"errors"
	"os"
	"os/exec"
)

func configureCommandProcess(cmd *exec.Cmd) {}

func killCommandProcess(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	return cmd.Process.Kill()
}

func commandProcessDoneError(err error) bool {
	return errors.Is(err, os.ErrProcessDone)
}
