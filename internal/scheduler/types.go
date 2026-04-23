package scheduler

type Config struct {
	GlobalMaxConcurrency int
}

type Job struct {
	JobID       string
	Count       int
	Concurrency int
}

type ImageTask struct {
	Index int
}

type PlanResult struct {
	Started []ImageTask
}
