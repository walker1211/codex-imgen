package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	api "github.com/walker1211/codex-imgen/internal/api"
	"github.com/walker1211/codex-imgen/internal/backend"
	"github.com/walker1211/codex-imgen/internal/cli"
	"github.com/walker1211/codex-imgen/internal/config"
	"github.com/walker1211/codex-imgen/internal/doctor"
	"github.com/walker1211/codex-imgen/internal/logutil"
	"github.com/walker1211/codex-imgen/internal/notify"
	"github.com/walker1211/codex-imgen/internal/scheduler"
	"github.com/walker1211/codex-imgen/internal/service"
	"github.com/walker1211/codex-imgen/internal/store"
)

func main() {
	ctx := context.Background()
	cmd, _ := cli.ParseCommand(os.Args[1:])
	home, err := os.UserHomeDir()
	if err != nil {
		_, _ = os.Stderr.WriteString(err.Error() + "\n")
		os.Exit(1)
	}
	cfg, err := config.Load(config.DefaultPath())
	if err != nil {
		_, _ = os.Stderr.WriteString(err.Error() + "\n")
		os.Exit(1)
	}
	if cfg.Server.Listen == "" {
		cfg.Server.Listen = "127.0.0.1:18080"
	}

	generator := backend.BuiltinCodex{
		Command:   cfg.Backend.Command,
		Model:     cfg.Backend.Model,
		CWD:       cfg.Backend.CWD,
		Timeout:   cfg.Backend.Timeout,
		CodexHome: filepath.Join(home, ".codex"),
	}

	app := cli.App{Stdout: os.Stdout, Stderr: os.Stderr}

	switch cmd.Name {
	case "run":
		app.Engine = cli.LocalEngine{Generator: generator, Prefix: cfg.Backend.Prompt.Prefix, Prelude: cfg.Backend.Prompt.Prelude}
	case "serve":
		dataDir, dbPath := storagePaths(home, cfg)
		st := mustOpenStore(dbPath)
		defer st.Close()
		hub := notify.NewWebSocketHub()
		svc := &service.Service{Store: st, Generator: generator, PromptPrefix: cfg.Backend.Prompt.Prefix, PromptPrelude: cfg.Backend.Prompt.Prelude, DefaultJobConcurrency: cfg.Scheduler.DefaultJobConcurrency, MaxJobConcurrency: cfg.Scheduler.MaxJobConcurrency, MaxCountPerJob: cfg.Scheduler.MaxCountPerJob, MaxAttempts: cfg.Scheduler.MaxAttempts, Publisher: hub}
		handler := api.NewServerWithNotifier(svc, hub)
		server := &http.Server{Addr: cfg.Server.Listen, Handler: handler, ReadTimeout: cfg.Server.ReadTimeout, WriteTimeout: cfg.Server.WriteTimeout}
		logutil.Printf("service starting listen=%s data_dir=%s sqlite_path=%s", cfg.Server.Listen, dataDir, dbPath)
		if err := notify.ValidateEmailConfig(cfg.Email); err != nil {
			_, _ = os.Stderr.WriteString(err.Error() + "\n")
			os.Exit(1)
		}
		maintenance := scheduler.Maintenance{Store: st, Mailer: notify.NewMailer(cfg.Email), LeaseTimeout: cfg.Scheduler.TaskLeaseTimeout}
		app.ServerRunner = cli.HTTPServerRunner{Server: server, Maintenance: cli.MaintenanceAdapter{Maintenance: maintenance}, MaintenanceInterval: cfg.Scheduler.MaintenanceInterval}
	case "submit", "status", "get", "list", "cancel":
		app.Client = &cli.Client{BaseURL: "http://" + cfg.Server.Listen}
	case "doctor":
		app.OpenClawDoctor = doctor.NewOpenClawChecker(home)
	default:
		app.Engine = cli.LocalEngine{Generator: generator, Prefix: cfg.Backend.Prompt.Prefix, Prelude: cfg.Backend.Prompt.Prelude}
	}

	os.Exit(app.Run(ctx, os.Args[1:]))
}

func storagePaths(home string, cfg config.Config) (string, string) {
	dataDir := cfg.Storage.DataDir
	if dataDir == "" {
		dataDir = filepath.Join(home, ".local", "share", "codex-imgen")
	}
	dbPath := cfg.Storage.SQLitePath
	if dbPath == "" {
		dbPath = filepath.Join(dataDir, "imgen.db")
	}
	return dataDir, dbPath
}

func mustOpenStore(dbPath string) *store.Store {
	_ = os.MkdirAll(filepath.Dir(dbPath), 0o755)
	st, err := store.Open(dbPath)
	if err != nil {
		panic(fmt.Sprintf("open store: %v", err))
	}
	return st
}
