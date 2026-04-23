package cli

import (
	"context"

	api "github.com/walker1211/codex-imgen/internal/api"
)

type Client struct {
	BaseURL string
}

func (c Client) CreateJob(ctx context.Context, prompt string, count int, concurrency int) (api.CreateJobResult, error) {
	return api.Client{BaseURL: c.BaseURL}.CreateJob(ctx, api.CreateJobRequest{
		Prompt:      prompt,
		Count:       count,
		Concurrency: concurrency,
	})
}

func (c Client) GetJob(ctx context.Context, jobID string) (api.JobStatus, error) {
	return api.Client{BaseURL: c.BaseURL}.GetJob(ctx, jobID)
}

func (c Client) ListJobs(ctx context.Context, limit int) ([]api.JobSummary, error) {
	return api.Client{BaseURL: c.BaseURL}.ListJobs(ctx, limit)
}

func (c Client) CancelJob(ctx context.Context, jobID string) error {
	return api.Client{BaseURL: c.BaseURL}.CancelJob(ctx, jobID)
}
