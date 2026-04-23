package notify

import (
	"encoding/json"
	"testing"
)

type captureSink struct {
	messages []Event
}

func (s *captureSink) Send(jobID string, event Event) {
	s.messages = append(s.messages, event)
}

func TestPublisherBroadcastsEventToSubscribedJob(t *testing.T) {
	store := NewSubscriptionStore()
	store.Add("job_1", "conn_1", nil)
	sink := &captureSink{}
	publisher := Publisher{Subscriptions: store, Sink: sink}

	event := Event{Type: "job.completed", JobID: "job_1", Payload: json.RawMessage(`{"status":"completed"}`)}
	publisher.Publish(event)

	if len(sink.messages) != 1 {
		t.Fatalf("messages = %d", len(sink.messages))
	}
	if sink.messages[0].Type != "job.completed" {
		t.Fatalf("type = %q", sink.messages[0].Type)
	}
}
