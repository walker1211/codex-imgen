package result

type Result struct {
	OK              bool   `json:"ok"`
	Prompt          string `json:"prompt,omitempty"`
	AssembledPrompt string `json:"assembled_prompt,omitempty"`
	URI             string `json:"uri,omitempty"`
	Path            string `json:"path,omitempty"`
	Error           string `json:"error,omitempty"`
	RawOutput       string `json:"raw_output,omitempty"`
}
