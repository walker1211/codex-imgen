package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
)

type Client struct {
	BaseURL string
	HTTP    *http.Client
}

func (c Client) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return http.DefaultClient
}

func (c Client) CreateJob(ctx context.Context, req CreateJobRequest) (CreateJobResult, error) {
	var result CreateJobResult
	if err := c.doJSON(ctx, http.MethodPost, "/v1/jobs", req, &result); err != nil {
		return CreateJobResult{}, err
	}
	return result, nil
}

func (c Client) GetJob(ctx context.Context, jobID string) (JobStatus, error) {
	var result JobStatus
	if err := c.doJSON(ctx, http.MethodGet, "/v1/jobs/"+jobID, nil, &result); err != nil {
		return JobStatus{}, err
	}
	return result, nil
}

func (c Client) ListJobs(ctx context.Context, limit int) ([]JobSummary, error) {
	var result []JobSummary
	path := "/v1/jobs"
	if limit > 0 {
		path += "?limit=" + strconv.Itoa(limit)
	}
	if err := c.doJSON(ctx, http.MethodGet, path, nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c Client) CancelJob(ctx context.Context, jobID string) error {
	var result map[string]any
	return c.doJSON(ctx, http.MethodPost, "/v1/jobs/"+jobID+"/cancel", nil, &result)
}

func (c Client) doJSON(ctx context.Context, method string, path string, request any, out any) error {
	var body *bytes.Reader
	if request == nil {
		body = bytes.NewReader(nil)
	} else {
		payload, err := json.Marshal(request)
		if err != nil {
			return err
		}
		body = bytes.NewReader(payload)
	}
	httpReq, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, body)
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient().Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var envelope Envelope
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return err
	}
	if resp.StatusCode >= 400 || !envelope.OK {
		if envelope.Error != nil {
			return errors.New(envelope.Error.Message)
		}
		return errors.New("request failed")
	}
	payload, err := json.Marshal(envelope.Data)
	if err != nil {
		return err
	}
	return json.Unmarshal(payload, out)
}
