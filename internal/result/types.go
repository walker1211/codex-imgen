package result

type ImageResult struct {
	Index  int    `json:"index"`
	Status string `json:"status,omitempty"`
	Path   string `json:"path,omitempty"`
	URI    string `json:"uri,omitempty"`
	Error  string `json:"error,omitempty"`
}

type Result struct {
	OK        bool          `json:"ok"`
	JobID     string        `json:"job_id,omitempty"`
	Prompt    string        `json:"prompt,omitempty"`
	Status    string        `json:"status,omitempty"`
	Count     int           `json:"count,omitempty"`
	Completed int           `json:"completed,omitempty"`
	Failed    int           `json:"failed,omitempty"`
	Images    []ImageResult `json:"images,omitempty"`
	Error     string        `json:"error,omitempty"`
	RawOutput string        `json:"raw_output,omitempty"`
}
