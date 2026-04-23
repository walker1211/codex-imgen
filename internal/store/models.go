package store

import "time"

type Job struct {
	JobID                string
	Prompt               string
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

type JobEvent struct {
	JobID     string
	EventType string
}
