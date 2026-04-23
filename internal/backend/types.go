package backend

import "context"

type GenerateRequest struct {
	Prompt string
	Model  string
	CWD    string
}

type GenerateResult struct {
	Path      string
	URI       string
	RawOutput string
}

type Generator interface {
	Generate(context.Context, GenerateRequest) (GenerateResult, error)
}
