package cli

import (
	"testing"
	"time"
)

type stubMaintenanceRunner struct{ called bool }

func (s *stubMaintenanceRunner) RunOnce() error {
	s.called = true
	return nil
}

func TestHTTPServerRunnerStartsMaintenanceLoop(t *testing.T) {
	runner := HTTPServerRunner{Maintenance: &stubMaintenanceRunner{}, MaintenanceInterval: 10 * time.Millisecond}
	stop := make(chan struct{})
	go runner.startMaintenance(stop)
	time.Sleep(30 * time.Millisecond)
	close(stop)
	if !runner.Maintenance.(*stubMaintenanceRunner).called {
		t.Fatal("expected maintenance runner to be called")
	}
}
