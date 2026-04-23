package scheduler

type Scheduler struct {
	globalMax int
}

func NewScheduler(cfg Config) *Scheduler {
	return &Scheduler{globalMax: cfg.GlobalMaxConcurrency}
}

func (s *Scheduler) Plan(job Job, queued []ImageTask) PlanResult {
	limit := job.Concurrency
	if limit > len(queued) {
		limit = len(queued)
	}
	if limit > s.globalMax {
		limit = s.globalMax
	}
	return PlanResult{Started: queued[:limit]}
}
