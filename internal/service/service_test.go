package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	api "github.com/walker1211/codex-imgen/internal/api"
	"github.com/walker1211/codex-imgen/internal/backend"
	"github.com/walker1211/codex-imgen/internal/store"
)

type fakeGenerator struct {
	mu     sync.Mutex
	calls  int
	images [][]string
}

func (g *fakeGenerator) Generate(ctx context.Context, req backend.GenerateRequest) (backend.GenerateResult, error) {
	g.mu.Lock()
	g.calls++
	g.images = append(g.images, append([]string(nil), req.Images...))
	idx := g.calls
	g.mu.Unlock()
	path := fmt.Sprintf("/tmp/%d.png", idx)
	return backend.GenerateResult{Path: path, URI: "file://" + path}, nil
}

type phaseGenerator struct{}

func (g *phaseGenerator) Generate(ctx context.Context, req backend.GenerateRequest) (backend.GenerateResult, error) {
	req.RecordPhase("process.started", time.Unix(100, 100_000_000), "")
	req.RecordPhase("stdout.turn_started", time.Unix(100, 250_000_000), "")
	return backend.GenerateResult{Path: "/tmp/phase.png", URI: "file:///tmp/phase.png", RawOutput: "phase output"}, nil
}

type phaseFailStore struct {
	*store.Store
}

func (s phaseFailStore) RecordImageAttemptPhase(ctx context.Context, jobID string, imageIndex int, attempt int, phase string, occurredAt time.Time, elapsedMS int64, detail string) error {
	return errors.New("phase write failed")
}

type attemptFinishFailStore struct {
	*store.Store
}

func (s attemptFinishFailStore) FinishImageAttempt(ctx context.Context, jobID string, imageIndex int, attempt int, status string, finishedAt time.Time, path string, uri string, lastError string, rawOutput string, finalMessage string) error {
	return errors.New("attempt finish failed")
}

func waitForJobStatus(t *testing.T, svc *Service, jobID string, status string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		got, err := svc.GetJob(jobID)
		if err != nil {
			t.Fatalf("GetJob returned error: %v", err)
		}
		if got.Status == status {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("job %s did not reach status %s", jobID, status)
}

func waitForImageAttempts(t *testing.T, s *store.Store, jobID string, ready func([]store.JobImageAttempt) bool) []store.JobImageAttempt {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	var last []store.JobImageAttempt
	for time.Now().Before(deadline) {
		attempts, err := s.ListImageAttempts(context.Background(), jobID)
		if err != nil {
			t.Fatalf("ListImageAttempts returned error: %v", err)
		}
		last = attempts
		if ready(attempts) {
			return attempts
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("image attempts did not reach expected state: %+v", last)
	return nil
}

func requireJobOrImageCancelled(t *testing.T, svc *Service, jobID string) {
	t.Helper()
	got, err := svc.GetJob(jobID)
	if err != nil {
		t.Fatalf("GetJob returned error: %v", err)
	}
	if got.Status == "cancelled" || len(got.Images) == 1 && got.Images[0].Status == "cancelled" {
		return
	}
	t.Fatalf("job/image status = %+v", got)
}

func createRunningJobAttempt(t *testing.T, s *store.Store, jobID string) {
	t.Helper()
	ctx := context.Background()
	if err := s.CreateJob(ctx, store.Job{JobID: jobID, Prompt: "x", RequestedCount: 1, EffectiveCount: 1, RequestedConcurrency: 1, EffectiveConcurrency: 1, Status: "running"}, []store.JobImage{{JobID: jobID, ImageIndex: 1, Status: "running"}}); err != nil {
		t.Fatalf("CreateJob returned error: %v", err)
	}
	if err := s.StartImageAttempt(ctx, jobID, 1, 1, time.Unix(100, 0)); err != nil {
		t.Fatalf("StartImageAttempt returned error: %v", err)
	}
}

func TestTailStringKeepsValidUTF8(t *testing.T) {
	limit := 10
	got := tailString(strings.Repeat("前缀", 20)+"中文🙂tail", limit)
	if !utf8.ValidString(got) {
		t.Fatalf("tailString returned invalid UTF-8: %q", got)
	}
	if len(got) > limit {
		t.Fatalf("len(tailString) = %d, want <= %d", len(got), limit)
	}
	if got := tailString("中文🙂", 0); got != "" {
		t.Fatalf("tailString zero limit = %q", got)
	}
	if got := tailString("中文🙂", -1); got != "" {
		t.Fatalf("tailString negative limit = %q", got)
	}
}

func TestServiceCreateAndGetJob(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "imgen.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer s.Close()

	gen := &fakeGenerator{}
	svc := Service{Store: s, Generator: gen, PromptPrefix: "$imagegen", PromptPrelude: "使用内置 imagegen 技能。", RetryDelays: []time.Duration{time.Millisecond, time.Millisecond}}
	created, err := svc.CreateJob(api.CreateJobRequest{Prompt: "draw a dragon", Count: 2, Concurrency: 5})
	if err != nil {
		t.Fatalf("CreateJob returned error: %v", err)
	}
	if created.JobID == "" {
		t.Fatal("expected job id")
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		got, err := svc.GetJob(created.JobID)
		if err != nil {
			t.Fatalf("GetJob returned error: %v", err)
		}
		if got.Status == "completed" {
			if got.Count != 2 {
				t.Fatalf("count = %d", got.Count)
			}
			if len(got.Images) != 2 {
				t.Fatalf("images = %d", len(got.Images))
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("job did not complete in time")
}

type flakyGenerator struct {
	mu        sync.Mutex
	calls     int
	failCalls int
}

func (g *flakyGenerator) Generate(ctx context.Context, req backend.GenerateRequest) (backend.GenerateResult, error) {
	g.mu.Lock()
	g.calls++
	calls := g.calls
	failCalls := g.failCalls
	g.mu.Unlock()
	if failCalls == 0 {
		failCalls = 1
	}
	if calls <= failCalls {
		return backend.GenerateResult{}, errors.New("temporary codex failure")
	}
	return backend.GenerateResult{Path: "/tmp/retry-success.png", URI: "file:///tmp/retry-success.png", RawOutput: "thread output"}, nil
}

type alwaysFailGenerator struct{}

func (g *alwaysFailGenerator) Generate(ctx context.Context, req backend.GenerateRequest) (backend.GenerateResult, error) {
	return backend.GenerateResult{}, errors.New("codex permanently failed")
}

type cancellableGenerator struct {
	started chan struct{}
	once    sync.Once
}

func (g *cancellableGenerator) Generate(ctx context.Context, req backend.GenerateRequest) (backend.GenerateResult, error) {
	g.once.Do(func() { close(g.started) })
	<-ctx.Done()
	return backend.GenerateResult{}, ctx.Err()
}

type signalFailGenerator struct {
	failed chan struct{}
	once   sync.Once
}

func (g *signalFailGenerator) Generate(ctx context.Context, req backend.GenerateRequest) (backend.GenerateResult, error) {
	g.once.Do(func() { close(g.failed) })
	return backend.GenerateResult{}, errors.New("first attempt failed")
}

type successAfterCancelGenerator struct {
	started chan struct{}
	release chan struct{}
}

func (g *successAfterCancelGenerator) Generate(ctx context.Context, req backend.GenerateRequest) (backend.GenerateResult, error) {
	close(g.started)
	<-g.release
	return backend.GenerateResult{Path: "/tmp/cancel-race.png", URI: "file:///tmp/cancel-race.png"}, nil
}

func TestStoreCancelJobUpdatesStatus(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "imgen.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer s.Close()

	ctx := context.Background()
	job := store.Job{JobID: "job_1", Prompt: "x", RequestedCount: 1, EffectiveCount: 1, RequestedConcurrency: 1, EffectiveConcurrency: 1, Status: "queued"}
	images := []store.JobImage{{JobID: "job_1", ImageIndex: 1, Status: "queued"}}
	if err := s.CreateJob(ctx, job, images); err != nil {
		t.Fatalf("CreateJob returned error: %v", err)
	}
	if err := s.CancelJob(ctx, "job_1"); err != nil {
		t.Fatalf("CancelJob returned error: %v", err)
	}
	got, _, err := s.GetJob(ctx, "job_1")
	if err != nil {
		t.Fatalf("GetJob returned error: %v", err)
	}
	if got.Status != "cancelled" {
		t.Fatalf("status = %q", got.Status)
	}
}

func TestServiceRecordsAttemptPhases(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "imgen.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer s.Close()

	now := time.Unix(100, 0)
	svc := Service{Store: s, Generator: &phaseGenerator{}, PromptPrefix: "$imagegen", Now: func() time.Time { return now }}
	created, err := svc.CreateJob(api.CreateJobRequest{Prompt: "draw a dragon", Count: 1, Concurrency: 1})
	if err != nil {
		t.Fatalf("CreateJob returned error: %v", err)
	}
	waitForJobStatus(t, &svc, created.JobID, "completed")

	phases, err := s.ListImageAttemptPhases(context.Background(), created.JobID)
	if err != nil {
		t.Fatalf("ListImageAttemptPhases returned error: %v", err)
	}
	if len(phases) != 2 {
		t.Fatalf("expected 2 phases, got %d: %+v", len(phases), phases)
	}
	if phases[0].Phase != "process.started" || phases[0].ElapsedMS != 100 {
		t.Fatalf("unexpected first phase: %+v", phases[0])
	}
	if phases[1].Phase != "stdout.turn_started" || phases[1].ElapsedMS != 250 {
		t.Fatalf("unexpected second phase: %+v", phases[1])
	}
}

func TestServiceFinishCancelledImageAttemptRecordsAttemptAndCancelsImage(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "imgen.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer s.Close()

	ctx := context.Background()
	jobID := "job_1"
	createRunningJobAttempt(t, s, jobID)

	pub := &capturePublisher{}
	svc := Service{Store: s, Publisher: pub}
	svc.finishCancelledImageAttempt(jobID, 1, 1, time.Unix(101, 0), "cancelled error")

	attempts, err := s.ListImageAttempts(ctx, jobID)
	if err != nil {
		t.Fatalf("ListImageAttempts returned error: %v", err)
	}
	if len(attempts) != 1 {
		t.Fatalf("attempts = %d", len(attempts))
	}
	if attempts[0].Status != "cancelled" || attempts[0].LastError != "cancelled error" || attempts[0].Path != "" || attempts[0].URI != "" {
		t.Fatalf("attempt = %+v", attempts[0])
	}
	_, images, err := s.GetJob(ctx, jobID)
	if err != nil {
		t.Fatalf("GetJob returned error: %v", err)
	}
	if len(images) != 1 || images[0].Status != "cancelled" {
		t.Fatalf("images = %+v", images)
	}
	events := pub.Events()
	if len(events) != 1 || events[0].Type != "image.cancelled" || events[0].JobID != jobID || events[0].Index != 1 || events[0].Status != "cancelled" {
		t.Fatalf("events = %+v", events)
	}
}

func TestServiceFinishFailedImageAttemptRecordsAttemptWithoutFailingImage(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "imgen.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer s.Close()

	ctx := context.Background()
	jobID := "job_1"
	createRunningJobAttempt(t, s, jobID)

	pub := &capturePublisher{}
	svc := Service{Store: s, Publisher: pub}
	if !svc.finishFailedImageAttempt(jobID, 1, 1, time.Unix(101, 0), "failed error") {
		t.Fatal("finishFailedImageAttempt returned false")
	}

	attempts, err := s.ListImageAttempts(ctx, jobID)
	if err != nil {
		t.Fatalf("ListImageAttempts returned error: %v", err)
	}
	if len(attempts) != 1 {
		t.Fatalf("attempts = %d", len(attempts))
	}
	if attempts[0].Status != "failed" || attempts[0].LastError != "failed error" || attempts[0].Path != "" || attempts[0].URI != "" {
		t.Fatalf("attempt = %+v", attempts[0])
	}
	_, images, err := s.GetJob(ctx, jobID)
	if err != nil {
		t.Fatalf("GetJob returned error: %v", err)
	}
	if len(images) != 1 || images[0].Status != "running" {
		t.Fatalf("images = %+v", images)
	}
	if events := pub.Events(); len(events) != 0 {
		t.Fatalf("events = %+v", events)
	}
}

func TestServiceFinishFailedImageAttemptFailsImageWhenAttemptWriteFails(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "imgen.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer s.Close()

	ctx := context.Background()
	jobID := "job_1"
	createRunningJobAttempt(t, s, jobID)

	pub := &capturePublisher{}
	svc := Service{Store: attemptFinishFailStore{Store: s}, Publisher: pub}
	if svc.finishFailedImageAttempt(jobID, 1, 1, time.Unix(101, 0), "failed error") {
		t.Fatal("finishFailedImageAttempt returned true")
	}

	_, images, err := s.GetJob(ctx, jobID)
	if err != nil {
		t.Fatalf("GetJob returned error: %v", err)
	}
	if len(images) != 1 || images[0].Status != "failed" {
		t.Fatalf("images = %+v", images)
	}
	events := pub.Events()
	if len(events) != 1 || events[0].Type != "image.failed" || events[0].JobID != jobID || events[0].Index != 1 || events[0].Status != "failed" {
		t.Fatalf("events = %+v", events)
	}
}

func TestServiceRecordImageAttemptPhaseClampsNegativeElapsedAndTailsDetail(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "imgen.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer s.Close()

	svc := Service{Store: s}
	detail := strings.Repeat("前缀", 200) + "tail"
	svc.recordImageAttemptPhase("job_1", 1, 1, time.Unix(100, 0), "process.started", time.Unix(99, 0), detail)

	phases, err := s.ListImageAttemptPhases(context.Background(), "job_1")
	if err != nil {
		t.Fatalf("ListImageAttemptPhases returned error: %v", err)
	}
	if len(phases) != 1 {
		t.Fatalf("phases = %d", len(phases))
	}
	if phases[0].ElapsedMS != 0 {
		t.Fatalf("elapsed ms = %d", phases[0].ElapsedMS)
	}
	if len(phases[0].Detail) > 500 {
		t.Fatalf("detail len = %d", len(phases[0].Detail))
	}
	if !strings.HasSuffix(phases[0].Detail, "tail") {
		t.Fatalf("detail = %q", phases[0].Detail)
	}
}

func TestServiceIgnoresAttemptPhaseWriteFailure(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "imgen.db")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer st.Close()

	now := time.Unix(100, 0)
	svc := Service{Store: phaseFailStore{Store: st}, Generator: &phaseGenerator{}, PromptPrefix: "$imagegen", Now: func() time.Time { return now }}
	created, err := svc.CreateJob(api.CreateJobRequest{Prompt: "draw a dragon", Count: 1, Concurrency: 1})
	if err != nil {
		t.Fatalf("CreateJob returned error: %v", err)
	}
	waitForJobStatus(t, &svc, created.JobID, "completed")
}

func TestServiceRecordsAttemptHistoryWhenRetrySucceeds(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "imgen.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer s.Close()

	svc := Service{
		Store:                 s,
		Generator:             &flakyGenerator{},
		RetryDelays:           []time.Duration{time.Millisecond},
		MaxAttempts:           2,
		DefaultJobConcurrency: 1,
		MaxJobConcurrency:     1,
		MaxCountPerJob:        1,
	}
	created, err := svc.CreateJob(api.CreateJobRequest{Prompt: "draw a dragon", Count: 1, Concurrency: 1})
	if err != nil {
		t.Fatalf("CreateJob returned error: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		got, err := svc.GetJob(created.JobID)
		if err != nil {
			t.Fatalf("GetJob returned error: %v", err)
		}
		if got.Status == "completed" {
			attempts, err := s.ListImageAttempts(context.Background(), created.JobID)
			if err != nil {
				t.Fatalf("ListImageAttempts returned error: %v", err)
			}
			if len(attempts) != 2 {
				t.Fatalf("attempts = %d", len(attempts))
			}
			if attempts[0].Attempt != 1 || attempts[0].Status != "failed" || !strings.Contains(attempts[0].LastError, "temporary codex failure") {
				t.Fatalf("attempt 1 = %+v", attempts[0])
			}
			if attempts[1].Attempt != 2 || attempts[1].Status != "done" || attempts[1].Path != "/tmp/retry-success.png" {
				t.Fatalf("attempt 2 = %+v", attempts[1])
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("job did not complete in time")
}

func TestServiceRecordsAllAttemptsWhenGenerationFails(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "imgen.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer s.Close()

	svc := Service{
		Store:                 s,
		Generator:             &alwaysFailGenerator{},
		RetryDelays:           []time.Duration{time.Millisecond},
		MaxAttempts:           2,
		DefaultJobConcurrency: 1,
		MaxJobConcurrency:     1,
		MaxCountPerJob:        1,
	}
	created, err := svc.CreateJob(api.CreateJobRequest{Prompt: "draw a dragon", Count: 1, Concurrency: 1})
	if err != nil {
		t.Fatalf("CreateJob returned error: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		got, err := svc.GetJob(created.JobID)
		if err != nil {
			t.Fatalf("GetJob returned error: %v", err)
		}
		if got.Status == "failed" {
			attempts, err := s.ListImageAttempts(context.Background(), created.JobID)
			if err != nil {
				t.Fatalf("ListImageAttempts returned error: %v", err)
			}
			if len(attempts) != 2 {
				t.Fatalf("attempts = %d", len(attempts))
			}
			for _, attempt := range attempts {
				if attempt.Status != "failed" || !strings.Contains(attempt.LastError, "codex permanently failed") {
					t.Fatalf("attempt = %+v", attempt)
				}
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("job did not fail in time")
}

func TestServiceRecordsCancelledAttemptWhenGeneratorContextCancelled(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "imgen.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer s.Close()

	gen := &cancellableGenerator{started: make(chan struct{})}
	svc := Service{
		Store:                 s,
		Generator:             gen,
		RetryDelays:           []time.Duration{time.Millisecond},
		MaxAttempts:           2,
		DefaultJobConcurrency: 1,
		MaxJobConcurrency:     1,
		MaxCountPerJob:        1,
	}
	created, err := svc.CreateJob(api.CreateJobRequest{Prompt: "draw a dragon", Count: 1, Concurrency: 1})
	if err != nil {
		t.Fatalf("CreateJob returned error: %v", err)
	}

	select {
	case <-gen.started:
	case <-time.After(2 * time.Second):
		t.Fatal("generator did not start in time")
	}
	if err := svc.CancelJob(created.JobID); err != nil {
		t.Fatalf("CancelJob returned error: %v", err)
	}

	attempts := waitForImageAttempts(t, s, created.JobID, func(attempts []store.JobImageAttempt) bool {
		return len(attempts) == 1 && attempts[0].Status == "cancelled"
	})
	if attempts[0].Path != "" || attempts[0].URI != "" {
		t.Fatalf("attempt = %+v", attempts[0])
	}
	requireJobOrImageCancelled(t, &svc, created.JobID)
}

func TestServiceRecordsCancelledAttemptAfterGenerateReturnsSuccess(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "imgen.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer s.Close()

	gen := &successAfterCancelGenerator{started: make(chan struct{}), release: make(chan struct{})}
	svc := Service{
		Store:                 s,
		Generator:             gen,
		RetryDelays:           []time.Duration{time.Millisecond},
		MaxAttempts:           1,
		DefaultJobConcurrency: 1,
		MaxJobConcurrency:     1,
		MaxCountPerJob:        1,
	}
	created, err := svc.CreateJob(api.CreateJobRequest{Prompt: "draw a dragon", Count: 1, Concurrency: 1})
	if err != nil {
		t.Fatalf("CreateJob returned error: %v", err)
	}

	select {
	case <-gen.started:
	case <-time.After(2 * time.Second):
		t.Fatal("generator did not start in time")
	}
	if err := svc.CancelJob(created.JobID); err != nil {
		t.Fatalf("CancelJob returned error: %v", err)
	}
	close(gen.release)

	attempts := waitForImageAttempts(t, s, created.JobID, func(attempts []store.JobImageAttempt) bool {
		return len(attempts) == 1 && attempts[0].Status == "cancelled"
	})
	if attempts[0].Path != "" || attempts[0].URI != "" {
		t.Fatalf("attempt = %+v", attempts[0])
	}
	requireJobOrImageCancelled(t, &svc, created.JobID)
}

func TestServiceRecordsCancelledAttemptDuringRetryDelay(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "imgen.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer s.Close()

	gen := &signalFailGenerator{failed: make(chan struct{})}
	svc := Service{
		Store:                 s,
		Generator:             gen,
		RetryDelays:           []time.Duration{500 * time.Millisecond},
		MaxAttempts:           2,
		DefaultJobConcurrency: 1,
		MaxJobConcurrency:     1,
		MaxCountPerJob:        1,
	}
	created, err := svc.CreateJob(api.CreateJobRequest{Prompt: "draw a dragon", Count: 1, Concurrency: 1})
	if err != nil {
		t.Fatalf("CreateJob returned error: %v", err)
	}

	select {
	case <-gen.failed:
	case <-time.After(2 * time.Second):
		t.Fatal("first attempt did not fail in time")
	}
	if err := svc.CancelJob(created.JobID); err != nil {
		t.Fatalf("CancelJob returned error: %v", err)
	}

	attempts := waitForImageAttempts(t, s, created.JobID, func(attempts []store.JobImageAttempt) bool {
		return len(attempts) == 2 && attempts[1].Status == "cancelled"
	})
	if attempts[0].Attempt != 1 || attempts[0].Status != "failed" || !strings.Contains(attempts[0].LastError, "first attempt failed") {
		t.Fatalf("attempt 1 = %+v", attempts[0])
	}
	if attempts[1].Attempt != 2 || attempts[1].Path != "" || attempts[1].URI != "" {
		t.Fatalf("attempt 2 = %+v", attempts[1])
	}
	requireJobOrImageCancelled(t, &svc, created.JobID)
}

func TestServiceReusesLastRetryDelayWhenAttemptsExceedDelays(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "imgen.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer s.Close()

	svc := Service{
		Store:                 s,
		Generator:             &flakyGenerator{failCalls: 2},
		RetryDelays:           []time.Duration{time.Millisecond},
		MaxAttempts:           3,
		DefaultJobConcurrency: 1,
		MaxJobConcurrency:     1,
		MaxCountPerJob:        1,
	}
	created, err := svc.CreateJob(api.CreateJobRequest{Prompt: "draw a dragon", Count: 1, Concurrency: 1})
	if err != nil {
		t.Fatalf("CreateJob returned error: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		got, err := svc.GetJob(created.JobID)
		if err != nil {
			t.Fatalf("GetJob returned error: %v", err)
		}
		if got.Status == "completed" {
			attempts, err := s.ListImageAttempts(context.Background(), created.JobID)
			if err != nil {
				t.Fatalf("ListImageAttempts returned error: %v", err)
			}
			if len(attempts) != 3 {
				t.Fatalf("attempts = %d", len(attempts))
			}
			if attempts[0].Status != "failed" || attempts[1].Status != "failed" || attempts[2].Status != "done" {
				t.Fatalf("attempts = %+v", attempts)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("job did not complete in time")
}

func TestServiceCreateJobClampsCountAndConcurrency(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "imgen.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer s.Close()

	svc := Service{Store: s, MaxCountPerJob: 10, MaxJobConcurrency: 10, DefaultJobConcurrency: 2}
	created, err := svc.CreateJob(api.CreateJobRequest{Prompt: "draw a dragon", Count: 99, Concurrency: 99})
	if err != nil {
		t.Fatalf("CreateJob returned error: %v", err)
	}
	got, err := svc.GetJob(created.JobID)
	if err != nil {
		t.Fatalf("GetJob returned error: %v", err)
	}
	if got.Count != 10 {
		t.Fatalf("count = %d", got.Count)
	}
}

func TestServiceRunsQueuedJobToCompletion(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "imgen.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer s.Close()

	gen := &fakeGenerator{}
	svc := Service{
		Store:                 s,
		Generator:             gen,
		PromptPrefix:          "$imagegen",
		PromptPrelude:         "使用内置 imagegen 技能。",
		RetryDelays:           []time.Duration{time.Millisecond, time.Millisecond},
		DefaultJobConcurrency: 2,
		MaxJobConcurrency:     10,
		MaxCountPerJob:        10,
	}
	created, err := svc.CreateJob(api.CreateJobRequest{Prompt: "draw a dragon", Count: 2, Concurrency: 2})
	if err != nil {
		t.Fatalf("CreateJob returned error: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		got, err := svc.GetJob(created.JobID)
		if err != nil {
			t.Fatalf("GetJob returned error: %v", err)
		}
		if got.Status == "completed" {
			if got.Completed != 2 {
				t.Fatalf("completed = %d", got.Completed)
			}
			if got.Images[0].Path == "" || got.Images[1].Path == "" {
				t.Fatalf("images = %+v", got.Images)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("job did not complete in time")
}

func TestServiceQueuesAllJobImagesToInjectedGenerator(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "imgen.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer s.Close()

	fake := newBlockingServiceQueueTestGenerator(2)
	svc := Service{
		Store:                 s,
		Generator:             backend.NewQueuedGenerator(fake, 10),
		PromptPrefix:          "$imagegen",
		RetryDelays:           []time.Duration{time.Millisecond},
		MaxAttempts:           1,
		DefaultJobConcurrency: 1,
		MaxJobConcurrency:     1,
		MaxCountPerJob:        2,
	}
	created, err := svc.CreateJob(api.CreateJobRequest{Prompt: "draw a dragon", Count: 2, Concurrency: 1})
	if err != nil {
		t.Fatalf("CreateJob returned error: %v", err)
	}

	waitForServiceQueueTestEntry(t, fake.entered)
	waitForServiceQueueTestEntry(t, fake.entered)

	close(fake.release)
	waitForJobStatus(t, &svc, created.JobID, "completed")
}

type blockingServiceQueueTestGenerator struct {
	entered chan backend.GenerateRequest
	release chan struct{}
}

func newBlockingServiceQueueTestGenerator(enteredBuffer int) *blockingServiceQueueTestGenerator {
	return &blockingServiceQueueTestGenerator{
		entered: make(chan backend.GenerateRequest, enteredBuffer),
		release: make(chan struct{}),
	}
}

func (g *blockingServiceQueueTestGenerator) Generate(ctx context.Context, req backend.GenerateRequest) (backend.GenerateResult, error) {
	select {
	case g.entered <- req:
	case <-ctx.Done():
		return backend.GenerateResult{}, ctx.Err()
	}
	select {
	case <-g.release:
		path := fmt.Sprintf("/tmp/service-queue-%d.png", req.ImageIndex)
		return backend.GenerateResult{Path: path, URI: "file://" + path}, nil
	case <-ctx.Done():
		return backend.GenerateResult{}, ctx.Err()
	}
}

func waitForServiceQueueTestEntry(t *testing.T, entered <-chan backend.GenerateRequest) backend.GenerateRequest {
	t.Helper()
	select {
	case req := <-entered:
		return req
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for backend Generate call")
		return backend.GenerateRequest{}
	}
}

func TestServiceCreateJobStoresInputDirImagePaths(t *testing.T) {
	inputDir := t.TempDir()
	img := filepath.Join(inputDir, "1.png")
	if err := os.WriteFile(img, []byte("png"), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	dbPath := filepath.Join(t.TempDir(), "imgen.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer s.Close()

	svc := Service{Store: s, ImageInputDir: inputDir}
	created, err := svc.CreateJob(api.CreateJobRequest{Prompt: "draw a dragon", Images: []string{"1.png"}})
	if err != nil {
		t.Fatalf("CreateJob returned error: %v", err)
	}
	job, _, err := s.GetJob(context.Background(), created.JobID)
	if err != nil {
		t.Fatalf("GetJob returned error: %v", err)
	}
	if job.ImagesJSON == "[]" || !strings.Contains(job.ImagesJSON, img) {
		t.Fatalf("images_json = %q", job.ImagesJSON)
	}
}

func TestServiceCreateJobRejectsImagePathOutsideInputDir(t *testing.T) {
	outside := filepath.Join(t.TempDir(), "secret.png")
	if err := os.WriteFile(outside, []byte("png"), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	dbPath := filepath.Join(t.TempDir(), "imgen.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer s.Close()

	svc := Service{Store: s, ImageInputDir: t.TempDir()}
	_, err = svc.CreateJob(api.CreateJobRequest{Prompt: "draw a dragon", Images: []string{outside}})
	if err == nil || !strings.Contains(err.Error(), "image path must stay within image input directory") {
		t.Fatalf("err = %v", err)
	}
	if strings.Contains(err.Error(), outside) {
		t.Fatalf("error leaked rejected path: %v", err)
	}
}

func TestServiceCreateJobRejectsMissingImagePath(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "imgen.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer s.Close()

	svc := Service{Store: s, ImageInputDir: t.TempDir()}
	_, err = svc.CreateJob(api.CreateJobRequest{Prompt: "draw a dragon", Images: []string{"missing.png"}})
	if err == nil || !strings.Contains(err.Error(), "image path not found") {
		t.Fatalf("err = %v", err)
	}
}

func TestServiceRunJobPassesImagesToGenerator(t *testing.T) {
	inputDir := t.TempDir()
	img := filepath.Join(inputDir, "1.png")
	if err := os.WriteFile(img, []byte("png"), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	gen := &fakeGenerator{}
	dbPath := filepath.Join(t.TempDir(), "imgen.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer s.Close()
	svc := Service{Store: s, Generator: gen, PromptPrefix: "$imagegen", RetryDelays: []time.Duration{time.Millisecond}, ImageInputDir: inputDir}
	created, err := svc.CreateJob(api.CreateJobRequest{Prompt: "draw a dragon", Images: []string{"1.png"}})
	if err != nil {
		t.Fatalf("CreateJob returned error: %v", err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		got, err := svc.GetJob(created.JobID)
		if err != nil {
			t.Fatalf("GetJob returned error: %v", err)
		}
		if got.Status == "completed" || got.Status == "partial_success" || got.Status == "failed" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(gen.images) == 0 || len(gen.images[0]) != 1 || gen.images[0][0] != img {
		t.Fatalf("images = %v", gen.images)
	}
}
