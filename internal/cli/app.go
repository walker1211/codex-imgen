package cli

import (
	"context"
	"errors"
	"fmt"
	"io"

	api "github.com/walker1211/codex-imgen/internal/api"
	"github.com/walker1211/codex-imgen/internal/result"
)

type SyncRequest struct {
	Prompt      string
	Images      []string
	Count       int
	Concurrency int
}

type Engine interface {
	RunSync(context.Context, SyncRequest) (result.Result, error)
}

type ClientAPI interface {
	CreateJob(context.Context, string, []string, int, int) (api.CreateJobResult, error)
	GetJob(context.Context, string) (api.JobStatus, error)
	ListJobs(context.Context, int) ([]api.JobSummary, error)
	CancelJob(context.Context, string) error
}

type ServerRunner interface {
	Run() error
}

type App struct {
	Stdout       io.Writer
	Stderr       io.Writer
	Engine       Engine
	Client       ClientAPI
	ServerRunner ServerRunner
}

func (a App) writeStdout(value string) error {
	_, err := fmt.Fprint(a.Stdout, value)
	return err
}

func (a App) writeStdoutf(format string, args ...any) error {
	_, err := fmt.Fprintf(a.Stdout, format, args...)
	return err
}

func (a App) writeStderrln(value string) error {
	_, err := fmt.Fprintln(a.Stderr, value)
	return err
}

func (a App) failWithStderr(value string) int {
	if err := a.writeStderrln(value); err != nil {
		return 1
	}
	return 1
}

func (a App) Run(ctx context.Context, args []string) int {
	cmd, err := ParseCommand(args)
	if err != nil {
		if errors.Is(err, ErrHelp) {
			if err := a.writeStdout(UsageText()); err != nil {
				return 1
			}
			return 0
		}
		return a.failWithStderr(err.Error())
	}

	switch cmd.Name {
	case "serve":
		if a.ServerRunner == nil {
			return a.failWithStderr("not implemented")
		}
		if err := a.ServerRunner.Run(); err != nil {
			return a.failWithStderr(err.Error())
		}
		return 0
	case "submit":
		if a.Client == nil {
			return a.failWithStderr("not implemented")
		}
		job, err := a.Client.CreateJob(ctx, cmd.Prompt, cmd.Images, cmd.Count, cmd.Concurrency)
		if err != nil {
			return a.failWithStderr(err.Error())
		}
		if cmd.JSON {
			if err := a.writeStdout(result.RenderJSON(result.Result{OK: true, JobID: job.JobID, Status: job.Status})); err != nil {
				return 1
			}
		} else {
			if err := a.writeStdoutf("%s\n", job.JobID); err != nil {
				return 1
			}
		}
		return 0
	case "status", "get":
		if a.Client == nil {
			return a.failWithStderr("not implemented")
		}
		status, err := a.Client.GetJob(ctx, cmd.JobID)
		if err != nil {
			return a.failWithStderr(err.Error())
		}
		res := result.Result{OK: true, JobID: status.JobID, Status: status.Status, Count: status.Count, Completed: status.Completed, Failed: status.Failed}
		for _, image := range status.Images {
			res.Images = append(res.Images, result.ImageResult{Index: image.Index, Status: image.Status, Path: image.Path, URI: image.URI})
		}
		if cmd.JSON {
			if err := a.writeStdout(result.RenderJSON(res)); err != nil {
				return 1
			}
		} else {
			if err := a.writeStdout(result.RenderText(res)); err != nil {
				return 1
			}
		}
		return 0
	case "list":
		if a.Client == nil {
			return a.failWithStderr("not implemented")
		}
		jobs, err := a.Client.ListJobs(ctx, 20)
		if err != nil {
			return a.failWithStderr(err.Error())
		}
		for _, job := range jobs {
			if err := a.writeStdoutf("%s %s\n", job.JobID, job.Status); err != nil {
				return 1
			}
		}
		return 0
	case "cancel":
		if a.Client == nil {
			return a.failWithStderr("not implemented")
		}
		if err := a.Client.CancelJob(ctx, cmd.JobID); err != nil {
			return a.failWithStderr(err.Error())
		}
		if err := a.writeStdoutf("%s\n", cmd.JobID); err != nil {
			return 1
		}
		return 0
	case "run":
		if a.Engine == nil {
			return a.failWithStderr("not implemented")
		}
		res, err := a.Engine.RunSync(ctx, SyncRequest{
			Prompt:      cmd.Prompt,
			Images:      cmd.Images,
			Count:       cmd.Count,
			Concurrency: cmd.Concurrency,
		})
		if err != nil {
			if cmd.JSON {
				if err := a.writeStdout(result.RenderJSON(result.Result{OK: false, Error: err.Error()})); err != nil {
					return 1
				}
				return 1
			}
			return a.failWithStderr(err.Error())
		}
		if cmd.JSON {
			if err := a.writeStdout(result.RenderJSON(res)); err != nil {
				return 1
			}
		} else {
			if err := a.writeStdout(result.RenderText(res)); err != nil {
				return 1
			}
		}
		return 0
	default:
		return a.failWithStderr("unknown command")
	}
}
