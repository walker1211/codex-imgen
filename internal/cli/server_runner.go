package cli

import (
	"net/http"
	"time"
)

type MaintenanceRunner interface {
	RunOnce() error
}

type HTTPServerRunner struct {
	Server              *http.Server
	Maintenance         MaintenanceRunner
	MaintenanceInterval time.Duration
}

func (r HTTPServerRunner) Run() error {
	stop := make(chan struct{})
	defer close(stop)
	if r.Maintenance != nil && r.MaintenanceInterval > 0 {
		go r.startMaintenance(stop)
	}
	return r.Server.ListenAndServe()
}

func (r HTTPServerRunner) startMaintenance(stop <-chan struct{}) {
	ticker := time.NewTicker(r.MaintenanceInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			_ = r.Maintenance.RunOnce()
		case <-stop:
			return
		}
	}
}
