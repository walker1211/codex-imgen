package store

import "time"

type Job struct {
	JobID                string
	Prompt               string
	ImagesJSON           string
	RequestedCount       int
	EffectiveCount       int
	RequestedConcurrency int
	EffectiveConcurrency int
	Status               string
	NotificationStatus   string
}

type JobImage struct {
	JobID      string
	ImageIndex int
	Status     string
	Path       string
	URI        string
	LastError  string
	StartedAt  time.Time
}

type JobImageAttempt struct {
	JobID      string
	ImageIndex int
	Attempt    int
	Status     string
	StartedAt  time.Time
	FinishedAt time.Time
	DurationMS int64
	Path       string
	URI        string
	LastError  string
	StdoutTail string
	StderrTail string
}

type JobEvent struct {
	JobID     string
	EventType string
}
