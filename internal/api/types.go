package api

type CreateJobRequest struct {
	Prompt      string `json:"prompt"`
	Count       int    `json:"count"`
	Concurrency int    `json:"concurrency"`
}

type CreateJobResult struct {
	JobID  string `json:"job_id"`
	Status string `json:"status"`
}

type JobImage struct {
	Index  int    `json:"index"`
	Status string `json:"status"`
	Path   string `json:"path,omitempty"`
	URI    string `json:"uri,omitempty"`
}

type JobStatus struct {
	JobID       string     `json:"job_id"`
	Status      string     `json:"status"`
	Count       int        `json:"count"`
	Completed   int        `json:"completed"`
	Failed      int        `json:"failed"`
	Images      []JobImage `json:"images,omitempty"`
}

type JobSummary struct {
	JobID     string `json:"job_id"`
	Status    string `json:"status"`
	Count     int    `json:"count"`
	Completed int    `json:"completed"`
	Failed    int    `json:"failed"`
}

type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Envelope struct {
	OK    bool      `json:"ok"`
	Data  any       `json:"data,omitempty"`
	Error *APIError `json:"error,omitempty"`
}
