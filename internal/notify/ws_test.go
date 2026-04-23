package notify

import "testing"

func TestEventTypeNames(t *testing.T) {
	event := Event{Type: "job.completed", JobID: "job_1"}
	if event.Type != "job.completed" {
		t.Fatalf("type = %q", event.Type)
	}
}

func TestSubscriptionStoreTracksSubscribers(t *testing.T) {
	store := NewSubscriptionStore()
	store.Add("job_1", "conn_1", nil)
	store.Add("job_1", "conn_2", nil)
	ids := store.List("job_1")
	if len(ids) != 2 {
		t.Fatalf("len = %d", len(ids))
	}
	store.Remove("job_1", "conn_1")
	ids = store.List("job_1")
	if len(ids) != 1 {
		t.Fatalf("len = %d", len(ids))
	}
}
