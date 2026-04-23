package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	api "github.com/walker1211/codex-imgen/internal/api"
	"github.com/walker1211/codex-imgen/internal/backend"
	"github.com/walker1211/codex-imgen/internal/codex"
	"github.com/walker1211/codex-imgen/internal/notify"
	"github.com/walker1211/codex-imgen/internal/store"
)

type JobStore interface {
	CreateJob(context.Context, store.Job, []store.JobImage) error
	GetJob(context.Context, string) (store.Job, []store.JobImage, error)
	ListJobs(context.Context, int) ([]store.Job, error)
	CancelJob(context.Context, string) error
	UpdateJobStatus(context.Context, string, string) error
	StartImageRun(context.Context, string, int, time.Time) error
	UpdateImageStatus(context.Context, string, int, string) error
	UpdateImageResult(context.Context, string, int, string, string, string) error
	CancelOutstandingImages(context.Context, string) error
}

type Service struct {
	Store                 JobStore
	Generator             backend.Generator
	PromptPrefix          string
	PromptPrelude         string
	DefaultJobConcurrency int
	MaxJobConcurrency     int
	MaxCountPerJob        int
	Now                   func() time.Time
	RetryDelays           []time.Duration
	MaxAttempts           int
	Publisher             interface{ Publish(notify.Event) }
	mu                    sync.Mutex
	cancels               map[string]context.CancelFunc
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *Service) retryDelays() []time.Duration {
	if len(s.RetryDelays) > 0 {
		return s.RetryDelays
	}
	return []time.Duration{5 * time.Second, 15 * time.Second}
}

func (s *Service) maxAttempts() int {
	if s.MaxAttempts > 0 {
		return s.MaxAttempts
	}
	return 3
}

func (s *Service) publish(event notify.Event) {
	if s.Publisher != nil {
		s.Publisher.Publish(event)
	}
}

func (s *Service) CreateJob(req api.CreateJobRequest) (api.CreateJobResult, error) {
	if strings.TrimSpace(req.Prompt) == "" {
		return api.CreateJobResult{}, errors.New("prompt is required")
	}
	jobID := fmt.Sprintf("job_%d", s.now().UnixNano())
	count := req.Count
	if count <= 0 {
		count = 1
	}
	maxCount := s.MaxCountPerJob
	if maxCount <= 0 {
		maxCount = 10
	}
	if count > maxCount {
		count = maxCount
	}
	concurrency := req.Concurrency
	if concurrency <= 0 {
		concurrency = s.DefaultJobConcurrency
		if concurrency <= 0 {
			concurrency = 2
		}
	}
	maxJobConcurrency := s.MaxJobConcurrency
	if maxJobConcurrency <= 0 {
		maxJobConcurrency = 10
	}
	if concurrency > maxJobConcurrency {
		concurrency = maxJobConcurrency
	}
	if concurrency > count {
		concurrency = count
	}
	job := store.Job{
		JobID:                 jobID,
		Prompt:                req.Prompt,
		RequestedCount:        req.Count,
		EffectiveCount:        count,
		RequestedConcurrency:  req.Concurrency,
		EffectiveConcurrency:  concurrency,
		Status:                "queued",
		NotificationStatus:    "pending",
	}
	var images []store.JobImage
	for i := 1; i <= count; i++ {
		images = append(images, store.JobImage{JobID: jobID, ImageIndex: i, Status: "queued"})
	}
	if err := s.Store.CreateJob(context.Background(), job, images); err != nil {
		return api.CreateJobResult{}, err
	}
	s.publish(notify.Event{Type: "job.created", JobID: jobID, Status: "queued"})
	if s.Generator != nil {
		s.launchJob(job)
	}
	return api.CreateJobResult{JobID: jobID, Status: "queued"}, nil
}

func (s *Service) launchJob(job store.Job) {
	ctx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	if s.cancels == nil {
		s.cancels = map[string]context.CancelFunc{}
	}
	s.cancels[job.JobID] = cancel
	s.mu.Unlock()
	go func() {
		defer func() {
			s.mu.Lock()
			delete(s.cancels, job.JobID)
			s.mu.Unlock()
		}()
		_ = s.runJob(ctx, job)
	}()
}

func (s *Service) runJob(ctx context.Context, job store.Job) error {
	_ = s.Store.UpdateJobStatus(ctx, job.JobID, "running")
	s.publish(notify.Event{Type: "job.started", JobID: job.JobID, Status: "running"})
	sem := make(chan struct{}, job.EffectiveConcurrency)
	var wg sync.WaitGroup
	for i := 1; i <= job.EffectiveCount; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }()
			_ = s.Store.StartImageRun(ctx, job.JobID, index, s.now())
			s.publish(notify.Event{Type: "image.started", JobID: job.JobID, Index: index, Status: "running"})
			prompt := codex.BuildPrompt(s.PromptPrelude, s.PromptPrefix, job.Prompt)
			attempts := s.maxAttempts()
			for attempt := 1; attempt <= attempts; attempt++ {
				generated, err := s.Generator.Generate(ctx, backend.GenerateRequest{Prompt: prompt})
				if err == nil {
					_ = s.Store.UpdateImageResult(ctx, job.JobID, index, "done", generated.Path, generated.URI)
					payload, _ := json.Marshal(map[string]any{"status": "done", "path": generated.Path, "uri": generated.URI})
					s.publish(notify.Event{Type: "image.completed", JobID: job.JobID, Index: index, Status: "done", Path: generated.Path, Payload: payload})
					return
				}
				if ctx.Err() != nil {
					_ = s.Store.UpdateImageStatus(context.Background(), job.JobID, index, "cancelled")
					s.publish(notify.Event{Type: "image.cancelled", JobID: job.JobID, Index: index, Status: "cancelled"})
					return
				}
				if attempt < attempts {
					delay := s.retryDelays()[attempt-1]
					select {
					case <-time.After(delay):
					case <-ctx.Done():
						_ = s.Store.UpdateImageStatus(context.Background(), job.JobID, index, "cancelled")
						s.publish(notify.Event{Type: "image.cancelled", JobID: job.JobID, Index: index, Status: "cancelled"})
						return
					}
				}
			}
			_ = s.Store.UpdateImageStatus(context.Background(), job.JobID, index, "failed")
			s.publish(notify.Event{Type: "image.failed", JobID: job.JobID, Index: index, Status: "failed"})
		}(i)
	}
	wg.Wait()
	latest, images, err := s.Store.GetJob(context.Background(), job.JobID)
	if err != nil {
		return err
	}
	if latest.Status == "cancelled" {
		s.publish(notify.Event{Type: "job.cancelled", JobID: job.JobID, Status: "cancelled"})
		return nil
	}
	completed := 0
	for _, image := range images {
		if image.Status == "done" {
			completed++
		}
	}
	status := "failed"
	if completed == job.EffectiveCount {
		status = "completed"
	} else if completed > 0 {
		status = "partial_success"
	}
	if err := s.Store.UpdateJobStatus(context.Background(), job.JobID, status); err != nil {
		return err
	}
	payload, _ := json.Marshal(map[string]any{"status": status, "completed": completed, "count": job.EffectiveCount})
	s.publish(notify.Event{Type: "job." + status, JobID: job.JobID, Status: status, Payload: payload})
	return nil
}

func (s *Service) GetJob(jobID string) (api.JobStatus, error) {
	job, images, err := s.Store.GetJob(context.Background(), jobID)
	if err != nil {
		return api.JobStatus{}, err
	}
	result := api.JobStatus{JobID: job.JobID, Status: job.Status, Count: job.EffectiveCount}
	for _, image := range images {
		result.Images = append(result.Images, api.JobImage{Index: image.ImageIndex, Status: image.Status, Path: image.Path, URI: image.URI})
		if image.Status == "done" {
			result.Completed++
		}
		if image.Status == "failed" || image.Status == "cancelled" {
			result.Failed++
		}
	}
	return result, nil
}

func (s *Service) ListJobs(limit int) ([]api.JobSummary, error) {
	jobs, err := s.Store.ListJobs(context.Background(), limit)
	if err != nil {
		return nil, err
	}
	var result []api.JobSummary
	for _, job := range jobs {
		result = append(result, api.JobSummary{JobID: job.JobID, Status: job.Status, Count: job.EffectiveCount})
	}
	return result, nil
}

func (s *Service) CancelJob(jobID string) error {
	s.mu.Lock()
	cancel := s.cancels[jobID]
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if err := s.Store.CancelJob(context.Background(), jobID); err != nil {
		return err
	}
	s.publish(notify.Event{Type: "job.cancelled", JobID: jobID, Status: "cancelled"})
	return s.Store.CancelOutstandingImages(context.Background(), jobID)
}
