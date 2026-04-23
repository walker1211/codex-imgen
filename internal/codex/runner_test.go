package codex

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRunnerRunReturnsDeadlineExceededOnTimeout(t *testing.T) {
	_, err := (Runner{}).Run(context.Background(), Request{
		Command: "sh",
		Args:    []string{"-c", "sleep 0.2"},
		Timeout: 10 * time.Millisecond,
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}
}
