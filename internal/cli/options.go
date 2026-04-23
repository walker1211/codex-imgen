package cli

import (
	"errors"
	"flag"
	"io"
	"strings"
)

var ErrHelp = flag.ErrHelp

type Command struct {
	Name        string
	Prompt      string
	Count       int
	Concurrency int
	JobID       string
	ConfigPath  string
	JSON        bool
}

func ParseCommand(args []string) (Command, error) {
	if len(args) == 0 {
		return Command{Name: "run", Count: 1, Concurrency: 2}, nil
	}
	switch args[0] {
	case "serve":
		return parseServe(args[1:])
	case "submit":
		return parseSubmit(args[1:])
	case "status", "get", "cancel":
		return parseJobCommand(args[0], args[1:])
	case "list":
		return Command{Name: "list"}, nil
	default:
		return parseRun(args)
	}
}

func parseServe(args []string) (Command, error) {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var cmd Command
	cmd.Name = "serve"
	if err := fs.Parse(args); err != nil {
		return Command{}, err
	}
	return cmd, nil
}

func parseSubmit(args []string) (Command, error) {
	fs := flag.NewFlagSet("submit", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	cmd := Command{Name: "submit", Count: 1, Concurrency: 2}
	fs.IntVar(&cmd.Count, "count", 1, "")
	fs.IntVar(&cmd.Concurrency, "concurrency", 2, "")
	fs.BoolVar(&cmd.JSON, "json", false, "")
	if err := fs.Parse(args); err != nil {
		return Command{}, err
	}
	if fs.NArg() != 1 {
		return Command{}, errors.New("expected exactly one prompt argument")
	}
	cmd.Prompt = fs.Arg(0)
	return cmd, nil
}

func parseRun(args []string) (Command, error) {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	cmd := Command{Name: "run", Count: 1, Concurrency: 2}
	fs.IntVar(&cmd.Count, "count", 1, "")
	fs.IntVar(&cmd.Concurrency, "concurrency", 2, "")
	fs.BoolVar(&cmd.JSON, "json", false, "")
	if err := fs.Parse(args); err != nil {
		return Command{}, err
	}
	if fs.NArg() != 1 {
		return Command{}, errors.New("expected exactly one prompt argument")
	}
	cmd.Prompt = fs.Arg(0)
	return cmd, nil
}

func parseJobCommand(name string, args []string) (Command, error) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	cmd := Command{Name: name}
	fs.BoolVar(&cmd.JSON, "json", false, "")
	if err := fs.Parse(args); err != nil {
		return Command{}, err
	}
	if fs.NArg() != 1 {
		return Command{}, errors.New("expected exactly one job id argument")
	}
	cmd.JobID = fs.Arg(0)
	return cmd, nil
}

func UsageText() string {
	return strings.TrimSpace(`Usage: imgen [flags] "prompt"
       imgen submit [flags] "prompt"
       imgen serve
       imgen status <job-id>
       imgen get <job-id>
       imgen list
       imgen cancel <job-id>

Flags:
  --count int          target image count
  --concurrency int    per-job max concurrency
  --json               output JSON instead of plain text
  --help               show this help text
`) + "\n"
}

