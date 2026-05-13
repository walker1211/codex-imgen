package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"

	api "github.com/walker1211/codex-imgen/internal/api"
	"github.com/walker1211/codex-imgen/internal/doctor"
	"github.com/walker1211/codex-imgen/internal/result"
)

type stubEngine struct {
	result      result.Result
	err         error
	lastRequest SyncRequest
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}

func (s *stubEngine) RunSync(ctx context.Context, req SyncRequest) (result.Result, error) {
	s.lastRequest = req
	return s.result, s.err
}

type stubClient struct {
	createResult       api.CreateJobResult
	createErr          error
	lastCreatePrompt   string
	lastCreateImages   []string
	lastCreateCount    int
	lastCreateConc     int
	getStatus          api.JobStatus
	getErr             error
	lastGetJobID       string
	listJobs           []api.JobSummary
	listErr            error
	lastListLimit      int
	cancelErr          error
	lastCancelledJobID string
}

func (s *stubClient) CreateJob(ctx context.Context, prompt string, images []string, count int, concurrency int) (api.CreateJobResult, error) {
	s.lastCreatePrompt = prompt
	s.lastCreateImages = append([]string(nil), images...)
	s.lastCreateCount = count
	s.lastCreateConc = concurrency
	return s.createResult, s.createErr
}

func (s *stubClient) GetJob(ctx context.Context, jobID string) (api.JobStatus, error) {
	s.lastGetJobID = jobID
	return s.getStatus, s.getErr
}

func (s *stubClient) ListJobs(ctx context.Context, limit int) ([]api.JobSummary, error) {
	s.lastListLimit = limit
	return s.listJobs, s.listErr
}

func (s *stubClient) CancelJob(ctx context.Context, jobID string) error {
	s.lastCancelledJobID = jobID
	return s.cancelErr
}

type stubOpenClawDoctor struct {
	report doctor.Report
	err    error
	called bool
}

func (s *stubOpenClawDoctor) Check(ctx context.Context) (doctor.Report, error) {
	s.called = true
	return s.report, s.err
}

func TestAppRunPrintsMultiplePaths(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	app := App{
		Stdout: stdout,
		Stderr: stderr,
		Engine: &stubEngine{result: result.Result{
			OK: true,
			Images: []result.ImageResult{
				{Index: 1, Path: "/tmp/1.png"},
				{Index: 2, Path: "/tmp/2.png"},
			},
		}},
	}

	exitCode := app.Run(context.Background(), []string{"--count", "2", "draw a dragon"})
	if exitCode != 0 {
		t.Fatalf("exitCode = %d", exitCode)
	}
	if got := stdout.String(); got != "/tmp/1.png\n/tmp/2.png\n" {
		t.Fatalf("stdout = %q", got)
	}
}

func TestAppRunPrintsHelp(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	app := App{Stdout: stdout, Stderr: stderr}

	exitCode := app.Run(context.Background(), []string{"--help"})
	if exitCode != 0 {
		t.Fatalf("exitCode = %d", exitCode)
	}
	if got := stdout.String(); got == "" {
		t.Fatal("expected help text")
	}
}

func TestAppRunReturnsFailureWhenStdoutWriteFails(t *testing.T) {
	stderr := &bytes.Buffer{}
	app := App{Stdout: failingWriter{}, Stderr: stderr}

	exitCode := app.Run(context.Background(), []string{"--help"})
	if exitCode != 1 {
		t.Fatalf("exitCode = %d", exitCode)
	}
}

type stubServeRunner struct {
	called bool
}

func (s *stubServeRunner) Run() error {
	s.called = true
	return nil
}

func TestAppRunServeStartsServer(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	runner := &stubServeRunner{}
	app := App{Stdout: stdout, Stderr: stderr, ServerRunner: runner}

	exitCode := app.Run(context.Background(), []string{"serve"})
	if exitCode != 0 {
		t.Fatalf("exitCode = %d", exitCode)
	}
	if !runner.called {
		t.Fatal("expected server runner to be called")
	}
}

func TestAppRunJSONPrintsStructuredError(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	engine := &stubEngine{err: errors.New("codex exec failed: signal: killed; stderr: generation failed")}
	app := App{
		Stdout: stdout,
		Stderr: stderr,
		Engine: engine,
	}

	exitCode := app.Run(context.Background(), []string{"--json", "draw a dragon"})
	if exitCode != 1 {
		t.Fatalf("exitCode = %d", exitCode)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
	var got result.Result
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not valid JSON: %v", err)
	}
	if got.OK {
		t.Fatalf("expected error result, got %+v", got)
	}
	if got.Error == "" {
		t.Fatalf("expected error message, got %+v", got)
	}
}

func TestAppRunPassesImagesToEngine(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	engine := &stubEngine{result: result.Result{OK: true, Images: []result.ImageResult{{Index: 1, Path: "/tmp/1.png"}}}}
	app := App{Stdout: stdout, Stderr: stderr, Engine: engine}

	exitCode := app.Run(context.Background(), []string{"--image", "/tmp/1.png", "draw a dragon"})
	if exitCode != 0 {
		t.Fatalf("exitCode = %d stderr=%q", exitCode, stderr.String())
	}
	if len(engine.lastRequest.Images) != 1 || engine.lastRequest.Images[0] != "/tmp/1.png" {
		t.Fatalf("images = %v", engine.lastRequest.Images)
	}
}

func TestAppRunSubmitCreatesJobAndPrintsID(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	client := &stubClient{createResult: api.CreateJobResult{JobID: "job_123", Status: "queued"}}
	app := App{Stdout: stdout, Stderr: stderr, Client: client}

	exitCode := app.Run(context.Background(), []string{"submit", "--count", "4", "--concurrency", "2", "--image", "/tmp/ref.png", "draw a dragon"})
	if exitCode != 0 {
		t.Fatalf("exitCode = %d stderr=%q", exitCode, stderr.String())
	}
	if got := stdout.String(); got != "job_123\n" {
		t.Fatalf("stdout = %q", got)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
	if client.lastCreatePrompt != "draw a dragon" || client.lastCreateCount != 4 || client.lastCreateConc != 2 {
		t.Fatalf("create args = prompt:%q count:%d concurrency:%d", client.lastCreatePrompt, client.lastCreateCount, client.lastCreateConc)
	}
	if len(client.lastCreateImages) != 1 || client.lastCreateImages[0] != "/tmp/ref.png" {
		t.Fatalf("create images = %v", client.lastCreateImages)
	}
}

func TestAppRunSubmitJSONPrintsStructuredResult(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	client := &stubClient{createResult: api.CreateJobResult{JobID: "job_123", Status: "queued"}}
	app := App{Stdout: stdout, Stderr: stderr, Client: client}

	exitCode := app.Run(context.Background(), []string{"submit", "--json", "draw a dragon"})
	if exitCode != 0 {
		t.Fatalf("exitCode = %d stderr=%q", exitCode, stderr.String())
	}
	var got result.Result
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not valid JSON: %v", err)
	}
	if !got.OK || got.JobID != "job_123" || got.Status != "queued" {
		t.Fatalf("result = %+v", got)
	}
}

func TestAppRunStatusPrintsJobStatusAsText(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	client := &stubClient{getStatus: api.JobStatus{JobID: "job_123", Status: "running", Count: 2}}
	app := App{Stdout: stdout, Stderr: stderr, Client: client}

	exitCode := app.Run(context.Background(), []string{"status", "job_123"})
	if exitCode != 0 {
		t.Fatalf("exitCode = %d stderr=%q", exitCode, stderr.String())
	}
	if client.lastGetJobID != "job_123" {
		t.Fatalf("get job id = %q", client.lastGetJobID)
	}
	if got := stdout.String(); got != "job_123 running\n" {
		t.Fatalf("stdout = %q", got)
	}
}

func TestAppRunGetJSONPrintsStructuredStatus(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	client := &stubClient{getStatus: api.JobStatus{
		JobID:     "job_123",
		Status:    "completed",
		Count:     2,
		Completed: 1,
		Failed:    1,
		Images: []api.JobImage{
			{Index: 1, Status: "done", Path: "/tmp/1.png", URI: "file:///tmp/1.png"},
			{Index: 2, Status: "failed"},
		},
	}}
	app := App{Stdout: stdout, Stderr: stderr, Client: client}

	exitCode := app.Run(context.Background(), []string{"get", "--json", "job_123"})
	if exitCode != 0 {
		t.Fatalf("exitCode = %d stderr=%q", exitCode, stderr.String())
	}
	var got result.Result
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not valid JSON: %v", err)
	}
	if !got.OK || got.JobID != "job_123" || got.Status != "completed" || got.Count != 2 || got.Completed != 1 || got.Failed != 1 {
		t.Fatalf("result = %+v", got)
	}
	if len(got.Images) != 2 || got.Images[0].Path != "/tmp/1.png" || got.Images[1].Status != "failed" {
		t.Fatalf("images = %+v", got.Images)
	}
}

func TestAppRunListPrintsJobIDsAndStatuses(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	client := &stubClient{listJobs: []api.JobSummary{{JobID: "job_1", Status: "queued"}, {JobID: "job_2", Status: "completed"}}}
	app := App{Stdout: stdout, Stderr: stderr, Client: client}

	exitCode := app.Run(context.Background(), []string{"list"})
	if exitCode != 0 {
		t.Fatalf("exitCode = %d stderr=%q", exitCode, stderr.String())
	}
	if client.lastListLimit != 20 {
		t.Fatalf("list limit = %d", client.lastListLimit)
	}
	if got := stdout.String(); got != "job_1 queued\njob_2 completed\n" {
		t.Fatalf("stdout = %q", got)
	}
}

func TestAppRunCancelCancelsJobAndPrintsID(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	client := &stubClient{}
	app := App{Stdout: stdout, Stderr: stderr, Client: client}

	exitCode := app.Run(context.Background(), []string{"cancel", "job_123"})
	if exitCode != 0 {
		t.Fatalf("exitCode = %d stderr=%q", exitCode, stderr.String())
	}
	if client.lastCancelledJobID != "job_123" {
		t.Fatalf("cancel job id = %q", client.lastCancelledJobID)
	}
	if got := stdout.String(); got != "job_123\n" {
		t.Fatalf("stdout = %q", got)
	}
}

func TestAppRunClientErrorPrintsStderr(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	client := &stubClient{createErr: errors.New("api unavailable")}
	app := App{Stdout: stdout, Stderr: stderr, Client: client}

	exitCode := app.Run(context.Background(), []string{"submit", "draw a dragon"})
	if exitCode != 1 {
		t.Fatalf("exitCode = %d", exitCode)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout output, got %q", stdout.String())
	}
	if got := stderr.String(); got != "api unavailable\n" {
		t.Fatalf("stderr = %q", got)
	}
}

func TestAppRunDoctorPrintsReport(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	doctorRunner := &stubOpenClawDoctor{report: doctor.Report{Title: "OpenClaw doctor", Items: []doctor.Item{{Level: doctor.LevelOK, Message: "config file"}}}}
	app := App{Stdout: stdout, Stderr: stderr, OpenClawDoctor: doctorRunner}

	exitCode := app.Run(context.Background(), []string{"doctor", "openclaw"})
	if exitCode != 0 {
		t.Fatalf("exitCode = %d stderr=%q", exitCode, stderr.String())
	}
	if !doctorRunner.called {
		t.Fatal("expected doctor to be called")
	}
	if got := stdout.String(); got != "OpenClaw doctor\n[OK] config file\n" {
		t.Fatalf("stdout = %q", got)
	}
}

func TestAppRunDoctorReturnsFailureWhenReportFails(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	doctorRunner := &stubOpenClawDoctor{report: doctor.Report{Title: "OpenClaw doctor", Items: []doctor.Item{{Level: doctor.LevelFail, Message: "missing message"}}}}
	app := App{Stdout: stdout, Stderr: stderr, OpenClawDoctor: doctorRunner}

	exitCode := app.Run(context.Background(), []string{"doctor", "openclaw"})
	if exitCode != 1 {
		t.Fatalf("exitCode = %d", exitCode)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
	if got := stdout.String(); got != "OpenClaw doctor\n[FAIL] missing message\n" {
		t.Fatalf("stdout = %q", got)
	}
}
