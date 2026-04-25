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

func (a App) Run(ctx context.Context, args []string) int {
	cmd, err := ParseCommand(args)
	if err != nil {
		if errors.Is(err, ErrHelp) {
			fmt.Fprint(a.Stdout, UsageText())
			return 0
		}
		fmt.Fprintln(a.Stderr, err.Error())
		return 1
	}

	switch cmd.Name {
	case "serve":
		if a.ServerRunner == nil {
			fmt.Fprintln(a.Stderr, "not implemented")
			return 1
		}
		if err := a.ServerRunner.Run(); err != nil {
			fmt.Fprintln(a.Stderr, err.Error())
			return 1
		}
		return 0
	case "submit":
		if a.Client == nil {
			fmt.Fprintln(a.Stderr, "not implemented")
			return 1
		}
		job, err := a.Client.CreateJob(ctx, cmd.Prompt, cmd.Images, cmd.Count, cmd.Concurrency)
		if err != nil {
			fmt.Fprintln(a.Stderr, err.Error())
			return 1
		}
		if cmd.JSON {
			fmt.Fprint(a.Stdout, result.RenderJSON(result.Result{OK: true, JobID: job.JobID, Status: job.Status}))
		} else {
			fmt.Fprintf(a.Stdout, "%s\n", job.JobID)
		}
		return 0
	case "status", "get":
		if a.Client == nil {
			fmt.Fprintln(a.Stderr, "not implemented")
			return 1
		}
		status, err := a.Client.GetJob(ctx, cmd.JobID)
		if err != nil {
			fmt.Fprintln(a.Stderr, err.Error())
			return 1
		}
		res := result.Result{OK: true, JobID: status.JobID, Status: status.Status, Count: status.Count, Completed: status.Completed, Failed: status.Failed}
		for _, image := range status.Images {
			res.Images = append(res.Images, result.ImageResult{Index: image.Index, Status: image.Status, Path: image.Path, URI: image.URI})
		}
		if cmd.JSON {
			fmt.Fprint(a.Stdout, result.RenderJSON(res))
		} else {
			fmt.Fprint(a.Stdout, result.RenderText(res))
		}
		return 0
	case "list":
		if a.Client == nil {
			fmt.Fprintln(a.Stderr, "not implemented")
			return 1
		}
		jobs, err := a.Client.ListJobs(ctx, 20)
		if err != nil {
			fmt.Fprintln(a.Stderr, err.Error())
			return 1
		}
		for _, job := range jobs {
			fmt.Fprintf(a.Stdout, "%s %s\n", job.JobID, job.Status)
		}
		return 0
	case "cancel":
		if a.Client == nil {
			fmt.Fprintln(a.Stderr, "not implemented")
			return 1
		}
		if err := a.Client.CancelJob(ctx, cmd.JobID); err != nil {
			fmt.Fprintln(a.Stderr, err.Error())
			return 1
		}
		fmt.Fprintf(a.Stdout, "%s\n", cmd.JobID)
		return 0
	case "run":
		if a.Engine == nil {
			fmt.Fprintln(a.Stderr, "not implemented")
			return 1
		}
		res, err := a.Engine.RunSync(ctx, SyncRequest{
			Prompt:      cmd.Prompt,
			Images:      cmd.Images,
			Count:       cmd.Count,
			Concurrency: cmd.Concurrency,
		})
		if err != nil {
			if cmd.JSON {
				fmt.Fprint(a.Stdout, result.RenderJSON(result.Result{OK: false, Error: err.Error()}))
			} else {
				fmt.Fprintln(a.Stderr, err.Error())
			}
			return 1
		}
		if cmd.JSON {
			fmt.Fprint(a.Stdout, result.RenderJSON(res))
		} else {
			fmt.Fprint(a.Stdout, result.RenderText(res))
		}
		return 0
	default:
		fmt.Fprintln(a.Stderr, "unknown command")
		return 1
	}
}
