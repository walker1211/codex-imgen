package main

import (
	"os"

	"github.com/walker1211/codex-imgen/internal/skillsync"
)

func main() {
	os.Exit(skillsync.Run(os.Args[1:], skillsync.CommandContext{
		Stdout:      os.Stdout,
		Stderr:      os.Stderr,
		Getwd:       os.Getwd,
		UserHomeDir: os.UserHomeDir,
	}))
}
