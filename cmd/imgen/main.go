package main

import (
	"context"
	"os"

	"github.com/walker1211/codex-imgen/internal/cli"
)

func main() {
	app := cli.App{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
	os.Exit(app.Run(context.Background(), os.Args[1:]))
}
