package api

type CreateJobRequest struct {
	Prompt      string   `json:"prompt"`
	Images      []string `json:"images,omitempty"`
	Count       int      `json:"count"`
	Concurrency int      `json:"concurrency"`
}

type RealtimeStartRequest struct {
	Type            string         `json:"type"`
	ClientRequestID string         `json:"client_request_id"`
	Items           []RealtimeItem `json:"items"`
	MaxConcurrency  int            `json:"max_concurrency"`
	TimeoutMS       int            `json:"timeout_ms"`
}

type RealtimeItem struct {
	ID     string   `json:"id"`
	Prompt string   `json:"prompt"`
	Images []string `json:"images,omitempty"`
	Count  int      `json:"count"`
}

type RealtimeEvent struct {
	Type            string `json:"type"`
	SessionID       string `json:"session_id,omitempty"`
	ClientRequestID string `json:"client_request_id,omitempty"`
	ItemID          string `json:"item_id,omitempty"`
	Index           int    `json:"index,omitempty"`
	Path            string `json:"path,omitempty"`
	URI             string `json:"uri,omitempty"`
	MIME            string `json:"mime,omitempty"`
	Error           string `json:"error,omitempty"`
	Retryable       bool   `json:"retryable,omitempty"`
	TotalItems      int    `json:"total_items,omitempty"`
	MaxConcurrency  int    `json:"max_concurrency,omitempty"`
	Completed       int    `json:"completed,omitempty"`
	Failed          int    `json:"failed,omitempty"`
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
	JobID     string     `json:"job_id"`
	Status    string     `json:"status"`
	Count     int        `json:"count"`
	Completed int        `json:"completed"`
	Failed    int        `json:"failed"`
	Images    []JobImage `json:"images,omitempty"`
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
