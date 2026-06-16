package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type stubService struct {
	createResult CreateJobResult
	jobResult    JobStatus
	listResult   []JobSummary
}

func (s stubService) CreateJob(req CreateJobRequest) (CreateJobResult, error) {
	return s.createResult, nil
}
func (s stubService) GetJob(jobID string) (JobStatus, error)   { return s.jobResult, nil }
func (s stubService) ListJobs(limit int) ([]JobSummary, error) { return s.listResult, nil }
func (s stubService) CancelJob(jobID string) error             { return nil }

func TestCreateJobReturnsJobID(t *testing.T) {
	server := NewServer(stubService{createResult: CreateJobResult{JobID: "job_123", Status: "queued"}})
	req := httptest.NewRequest(http.MethodPost, "/v1/jobs", strings.NewReader(`{"prompt":"draw a dragon","count":4,"concurrency":2}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	server.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d", resp.Code)
	}
	if !strings.Contains(resp.Body.String(), "job_123") {
		t.Fatalf("body = %q", resp.Body.String())
	}
}

func TestCreateJobReturnsJobIDWithImages(t *testing.T) {
	server := NewServer(stubService{createResult: CreateJobResult{JobID: "job_123", Status: "queued"}})
	req := httptest.NewRequest(http.MethodPost, "/v1/jobs", strings.NewReader(`{"prompt":"draw a dragon","images":["/tmp/1.png","/tmp/2.png"]}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	server.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d", resp.Code)
	}
}

func TestGetJobReturnsStatus(t *testing.T) {
	server := NewServer(stubService{jobResult: JobStatus{JobID: "job_123", Status: "running", Count: 2, Images: []JobImage{{Index: 1, Status: "done", Path: "/tmp/1.png"}}}})
	req := httptest.NewRequest(http.MethodGet, "/v1/jobs/job_123", nil)
	resp := httptest.NewRecorder()

	server.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d", resp.Code)
	}
	var env Envelope
	if err := json.Unmarshal(resp.Body.Bytes(), &env); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if !strings.Contains(resp.Body.String(), "running") {
		t.Fatalf("body = %q", resp.Body.String())
	}
}

func TestListJobsReturnsItems(t *testing.T) {
	server := NewServer(stubService{listResult: []JobSummary{{JobID: "job_1", Status: "queued", Count: 2}}})
	req := httptest.NewRequest(http.MethodGet, "/v1/jobs?limit=20", nil)
	resp := httptest.NewRecorder()

	server.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d", resp.Code)
	}
	if !strings.Contains(resp.Body.String(), "job_1") {
		t.Fatalf("body = %q", resp.Body.String())
	}
}

func TestCancelJobReturnsCancelledStatus(t *testing.T) {
	server := NewServer(stubService{})
	req := httptest.NewRequest(http.MethodPost, "/v1/jobs/job_1/cancel", nil)
	resp := httptest.NewRecorder()

	server.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d", resp.Code)
	}
	if !strings.Contains(resp.Body.String(), "cancelled") {
		t.Fatalf("body = %q", resp.Body.String())
	}
}

type invalidPromptService struct{}

func (invalidPromptService) CreateJob(req CreateJobRequest) (CreateJobResult, error) {
	return CreateJobResult{}, errors.New("prompt is required")
}
func (invalidPromptService) GetJob(jobID string) (JobStatus, error)   { return JobStatus{}, nil }
func (invalidPromptService) ListJobs(limit int) ([]JobSummary, error) { return nil, nil }
func (invalidPromptService) CancelJob(jobID string) error             { return nil }

type invalidImageService struct{}

func (invalidImageService) CreateJob(req CreateJobRequest) (CreateJobResult, error) {
	return CreateJobResult{}, errors.New("image path not found")
}
func (invalidImageService) GetJob(jobID string) (JobStatus, error)   { return JobStatus{}, nil }
func (invalidImageService) ListJobs(limit int) ([]JobSummary, error) { return nil, nil }
func (invalidImageService) CancelJob(jobID string) error             { return nil }

func TestCreateJobReturnsBadRequestForEmptyPrompt(t *testing.T) {
	server := NewServer(invalidPromptService{})
	req := httptest.NewRequest(http.MethodPost, "/v1/jobs", strings.NewReader(`{"prompt":"","count":1}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	server.ServeHTTP(resp, req)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", resp.Code)
	}
}

func TestCreateJobReturnsBadRequestForInvalidImagePath(t *testing.T) {
	server := NewServer(invalidImageService{})
	req := httptest.NewRequest(http.MethodPost, "/v1/jobs", strings.NewReader(`{"prompt":"draw a dragon","images":["/tmp/missing.png"]}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	server.ServeHTTP(resp, req)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%q", resp.Code, resp.Body.String())
	}
}

type notFoundService struct{}

func (notFoundService) CreateJob(req CreateJobRequest) (CreateJobResult, error) {
	return CreateJobResult{}, nil
}
func (notFoundService) GetJob(jobID string) (JobStatus, error) {
	return JobStatus{}, errors.New("job not found")
}
func (notFoundService) ListJobs(limit int) ([]JobSummary, error) { return nil, nil }
func (notFoundService) CancelJob(jobID string) error             { return errors.New("job not found") }

func TestGetJobReturnsNotFound(t *testing.T) {
	server := NewServer(notFoundService{})
	req := httptest.NewRequest(http.MethodGet, "/v1/jobs/missing", nil)
	resp := httptest.NewRecorder()

	server.ServeHTTP(resp, req)
	if resp.Code != http.StatusNotFound {
		t.Fatalf("status = %d", resp.Code)
	}
}

func TestCancelJobReturnsNotFound(t *testing.T) {
	server := NewServer(notFoundService{})
	req := httptest.NewRequest(http.MethodPost, "/v1/jobs/missing/cancel", nil)
	resp := httptest.NewRecorder()

	server.ServeHTTP(resp, req)
	if resp.Code != http.StatusNotFound {
		t.Fatalf("status = %d", resp.Code)
	}
}
