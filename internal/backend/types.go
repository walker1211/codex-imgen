package backend

import (
	"context"
	"time"
)

type PhaseRecorder func(phase string, occurredAt time.Time, detail string)

type GenerateRequest struct {
	Prompt      string
	Images      []string
	Model       string
	CWD         string
	JobID       string
	ImageIndex  int
	Attempt     int
	RecordPhase PhaseRecorder
}

type GenerateResult struct {
	Path      string
	URI       string
	RawOutput string
}

type Generator interface {
	Generate(context.Context, GenerateRequest) (GenerateResult, error)
}
