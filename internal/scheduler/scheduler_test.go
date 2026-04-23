package scheduler

import "testing"

func TestSchedulerRespectsGlobalAndJobConcurrency(t *testing.T) {
	s := NewScheduler(Config{GlobalMaxConcurrency: 10})
	job := Job{JobID: "job1", Count: 4, Concurrency: 2}

	state := s.Plan(job, []ImageTask{{Index: 1}, {Index: 2}, {Index: 3}, {Index: 4}})
	if len(state.Started) != 2 {
		t.Fatalf("started = %d", len(state.Started))
	}
}
