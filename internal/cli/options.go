package cli

import (
	"errors"
	"flag"
	"io"
	"strings"
	"time"
)

type Options struct {
	JSON    bool
	Model   string
	CWD     string
	Timeout time.Duration
	Raw     bool
	Verbose bool
	Prompt  string
}

func ParseOptions(args []string) (Options, error) {
	fs := flag.NewFlagSet("imgen", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var opts Options
	fs.BoolVar(&opts.JSON, "json", false, "output JSON instead of a plain path")
	fs.StringVar(&opts.Model, "model", "", "forward model to codex --model")
	fs.StringVar(&opts.CWD, "cwd", "", "forward working directory to codex --cd")
	fs.DurationVar(&opts.Timeout, "timeout", 0, "set external command timeout")
	fs.BoolVar(&opts.Raw, "raw", false, "print raw codex output to stderr")
	fs.BoolVar(&opts.Verbose, "verbose", false, "print command summary and key steps")

	if err := fs.Parse(args); err != nil {
		return Options{}, err
	}
	if fs.NArg() != 1 {
		return Options{}, errors.New("expected exactly one prompt argument")
	}
	opts.Prompt = fs.Arg(0)
	return opts, nil
}

func UsageText() string {
	return strings.TrimSpace(`Usage: imgen [flags] "prompt"

Flags:
  --json            output JSON instead of a plain path
  --model string    forward model to codex --model
  --cwd string      forward working directory to codex --cd
  --timeout value   set external command timeout
  --raw             print raw codex output to stderr
  --verbose         print command summary and key steps
  --help            show this help text
`) + "\n"
}
