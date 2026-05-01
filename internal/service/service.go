package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	api "github.com/walker1211/codex-imgen/internal/api"
	"github.com/walker1211/codex-imgen/internal/backend"
	"github.com/walker1211/codex-imgen/internal/codex"
	"github.com/walker1211/codex-imgen/internal/logutil"
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
	StartImageAttempt(context.Context, string, int, int, time.Time) error
	FinishImageAttempt(context.Context, string, int, int, string, time.Time, string, string, string, string, string) error
	RecordImageAttemptPhase(context.Context, string, int, int, string, time.Time, int64, string) error
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

func (s *Service) retryDelay(attempt int) time.Duration {
	delays := s.retryDelays()
	if attempt <= 0 {
		return delays[0]
	}
	index := attempt - 1
	if index >= len(delays) {
		index = len(delays) - 1
	}
	return delays[index]
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

func normalizeImagePaths(paths []string) ([]string, error) {
	var result []string
	for _, path := range paths {
		trimmed := strings.TrimSpace(path)
		if trimmed == "" {
			continue
		}
		abs, err := filepath.Abs(trimmed)
		if err != nil {
			return nil, err
		}
		if _, err := os.Stat(abs); err != nil {
			if os.IsNotExist(err) {
				return nil, fmt.Errorf("image path not found: %s", abs)
			}
			return nil, err
		}
		result = append(result, abs)
	}
	return result, nil
}

func decodeJobImages(job store.Job) ([]string, error) {
	if strings.TrimSpace(job.ImagesJSON) == "" {
		return nil, nil
	}
	var images []string
	if err := json.Unmarshal([]byte(job.ImagesJSON), &images); err != nil {
		return nil, err
	}
	return images, nil
}

func tailString(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	if len(value) <= limit {
		return value
	}
	start := len(value) - limit
	for start < len(value) && !utf8.RuneStart(value[start]) {
		start++
	}
	return value[start:]
}

func (s *Service) CreateJob(req api.CreateJobRequest) (api.CreateJobResult, error) {
	if strings.TrimSpace(req.Prompt) == "" {
		return api.CreateJobResult{}, errors.New("prompt is required")
	}
	normalizedImages, err := normalizeImagePaths(req.Images)
	if err != nil {
		return api.CreateJobResult{}, err
	}
	imagesJSON, err := json.Marshal(normalizedImages)
	if err != nil {
		return api.CreateJobResult{}, err
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
		JobID:                jobID,
		Prompt:               req.Prompt,
		ImagesJSON:           string(imagesJSON),
		RequestedCount:       req.Count,
		EffectiveCount:       count,
		RequestedConcurrency: req.Concurrency,
		EffectiveConcurrency: concurrency,
		Status:               "queued",
		NotificationStatus:   "pending",
	}
	var images []store.JobImage
	for i := 1; i <= count; i++ {
		images = append(images, store.JobImage{JobID: jobID, ImageIndex: i, Status: "queued"})
	}
	if err := s.Store.CreateJob(context.Background(), job, images); err != nil {
		return api.CreateJobResult{}, err
	}
	logutil.Printf("job created job_id=%s count=%d concurrency=%d images=%d", jobID, count, concurrency, len(normalizedImages))
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

func (s *Service) failImage(jobID string, index int) {
	_ = s.Store.UpdateImageStatus(context.Background(), jobID, index, "failed")
	logutil.Warnf("image failed job_id=%s image_index=%d", jobID, index)
	s.publish(notify.Event{Type: "image.failed", JobID: jobID, Index: index, Status: "failed"})
}

func (s *Service) cancelImage(jobID string, index int) {
	_ = s.Store.UpdateImageStatus(context.Background(), jobID, index, "cancelled")
	logutil.Printf("image cancelled job_id=%s image_index=%d", jobID, index)
	s.publish(notify.Event{Type: "image.cancelled", JobID: jobID, Index: index, Status: "cancelled"})
}

func (s *Service) runJob(ctx context.Context, job store.Job) error {
	_ = s.Store.UpdateJobStatus(ctx, job.JobID, "running")
	logutil.Printf("job started job_id=%s count=%d concurrency=%d", job.JobID, job.EffectiveCount, job.EffectiveConcurrency)
	s.publish(notify.Event{Type: "job.started", JobID: job.JobID, Status: "running"})
	prompt := codex.BuildPrompt(s.PromptPrelude, s.PromptPrefix, job.Prompt)
	jobImages, jobImagesErr := decodeJobImages(job)
	sem := make(chan struct{}, job.EffectiveConcurrency)
	var wg sync.WaitGroup
	for i := 1; i <= job.EffectiveCount; i++ {
		index := i
		wg.Go(func() {
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				s.cancelImage(job.JobID, index)
				return
			}
			defer func() { <-sem }()
			s.runJobImage(ctx, job, index, prompt, jobImages, jobImagesErr)
		})
	}
	wg.Wait()
	return s.finishJob(job)
}

func (s *Service) runJobImage(ctx context.Context, job store.Job, index int, prompt string, jobImages []string, jobImagesErr error) {
	_ = s.Store.StartImageRun(ctx, job.JobID, index, s.now())
	logutil.Printf("image started job_id=%s image_index=%d", job.JobID, index)
	s.publish(notify.Event{Type: "image.started", JobID: job.JobID, Index: index, Status: "running"})
	if jobImagesErr != nil {
		s.failImage(job.JobID, index)
		return
	}
	attempts := s.maxAttempts()
	for attempt := 1; attempt <= attempts; attempt++ {
		attemptStarted := s.now()
		if err := s.Store.StartImageAttempt(ctx, job.JobID, index, attempt, attemptStarted); err != nil {
			if ctx.Err() != nil {
				s.cancelImage(job.JobID, index)
				return
			}
			s.failImage(job.JobID, index)
			return
		}
		logutil.Printf("image attempt started job_id=%s image_index=%d attempt=%d", job.JobID, index, attempt)
		generated, err := s.Generator.Generate(ctx, backend.GenerateRequest{
			Prompt:     prompt,
			Images:     jobImages,
			JobID:      job.JobID,
			ImageIndex: index,
			Attempt:    attempt,
			RecordPhase: func(phase string, occurredAt time.Time, detail string) {
				s.recordImageAttemptPhase(job.JobID, index, attempt, attemptStarted, phase, occurredAt, detail)
			},
		})
		finishedAt := s.now()
		if err == nil {
			if ctx.Err() != nil {
				s.finishCancelledImageAttempt(job.JobID, index, attempt, finishedAt, tailString(ctx.Err().Error(), 2000))
				return
			}
			if err := s.Store.FinishImageAttempt(context.Background(), job.JobID, index, attempt, "done", finishedAt, generated.Path, generated.URI, "", tailString(generated.RawOutput, 2000), ""); err != nil {
				s.failImage(job.JobID, index)
				return
			}
			logutil.Printf("image attempt succeeded job_id=%s image_index=%d attempt=%d path=%s", job.JobID, index, attempt, generated.Path)
			if err := s.Store.UpdateImageResult(context.Background(), job.JobID, index, "done", generated.Path, generated.URI); err != nil {
				if ctx.Err() != nil {
					s.cancelImage(job.JobID, index)
					return
				}
				s.failImage(job.JobID, index)
				return
			}
			logutil.Printf("image completed job_id=%s image_index=%d path=%s", job.JobID, index, generated.Path)
			payload, _ := json.Marshal(map[string]any{"status": "done", "path": generated.Path, "uri": generated.URI})
			s.publish(notify.Event{Type: "image.completed", JobID: job.JobID, Index: index, Status: "done", Path: generated.Path, Payload: payload})
			return
		}
		lastError := tailString(err.Error(), 2000)
		if ctx.Err() != nil {
			s.finishCancelledImageAttempt(job.JobID, index, attempt, finishedAt, lastError)
			return
		}
		if !s.finishFailedImageAttempt(job.JobID, index, attempt, finishedAt, lastError) {
			return
		}
		if attempt < attempts {
			delay := s.retryDelay(attempt)
			logutil.Printf("image retry scheduled job_id=%s image_index=%d next_attempt=%d delay=%s", job.JobID, index, attempt+1, delay)
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				nextAttempt := attempt + 1
				cancelledAt := s.now()
				if err := s.Store.StartImageAttempt(context.Background(), job.JobID, index, nextAttempt, cancelledAt); err != nil {
					s.cancelImage(job.JobID, index)
					return
				}
				s.finishCancelledImageAttempt(job.JobID, index, nextAttempt, cancelledAt, tailString(ctx.Err().Error(), 2000))
				return
			}
		}
	}
	logutil.Warnf("final image failed job_id=%s image_index=%d attempts=%d", job.JobID, index, attempts)
	s.failImage(job.JobID, index)
}

func (s *Service) finishFailedImageAttempt(jobID string, index int, attempt int, finishedAt time.Time, lastError string) bool {
	if err := s.Store.FinishImageAttempt(context.Background(), jobID, index, attempt, "failed", finishedAt, "", "", lastError, "", ""); err != nil {
		s.failImage(jobID, index)
		return false
	}
	logutil.Warnf("image attempt failed job_id=%s image_index=%d attempt=%d error_len=%d", jobID, index, attempt, len(lastError))
	return true
}

func (s *Service) finishCancelledImageAttempt(jobID string, index int, attempt int, finishedAt time.Time, lastError string) {
	if err := s.Store.FinishImageAttempt(context.Background(), jobID, index, attempt, "cancelled", finishedAt, "", "", lastError, "", ""); err != nil {
		s.cancelImage(jobID, index)
		return
	}
	logutil.Printf("image attempt cancelled job_id=%s image_index=%d attempt=%d", jobID, index, attempt)
	s.cancelImage(jobID, index)
}

func (s *Service) recordImageAttemptPhase(jobID string, index int, attempt int, attemptStarted time.Time, phase string, occurredAt time.Time, detail string) {
	elapsedMS := max(int64(0), occurredAt.Sub(attemptStarted).Milliseconds())
	if err := s.Store.RecordImageAttemptPhase(context.Background(), jobID, index, attempt, phase, occurredAt, elapsedMS, tailString(detail, 500)); err != nil {
		logutil.Warnf("codex phase record failed job_id=%s image_index=%d attempt=%d phase=%s error_len=%d", jobID, index, attempt, phase, len(err.Error()))
		return
	}
	logutil.Printf("codex phase job_id=%s image_index=%d attempt=%d phase=%s elapsed_ms=%d", jobID, index, attempt, phase, elapsedMS)
}

func (s *Service) finishJob(job store.Job) error {
	latest, images, err := s.Store.GetJob(context.Background(), job.JobID)
	if err != nil {
		return err
	}
	if latest.Status == "cancelled" {
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
	logutil.Printf("job %s job_id=%s completed=%d count=%d", status, job.JobID, completed, job.EffectiveCount)
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
	logutil.Printf("job cancellation requested job_id=%s", jobID)
	if err := s.Store.CancelOutstandingImages(context.Background(), jobID); err != nil {
		return err
	}
	logutil.Printf("job cancelled job_id=%s", jobID)
	s.publish(notify.Event{Type: "job.cancelled", JobID: jobID, Status: "cancelled"})
	return nil
}
