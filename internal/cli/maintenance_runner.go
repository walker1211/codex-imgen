package cli

import (
	"context"
	"github.com/walker1211/codex-imgen/internal/scheduler"
)

type MaintenanceAdapter struct {
	Maintenance scheduler.Maintenance
}

func (a MaintenanceAdapter) RunOnce() error {
	return a.Maintenance.RunOnce(context.Background())
}
