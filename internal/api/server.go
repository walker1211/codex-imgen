package api

import (
	"net/http"
	"strings"

	"github.com/walker1211/codex-imgen/internal/notify"
)

type Service interface {
	CreateJob(CreateJobRequest) (CreateJobResult, error)
	GetJob(jobID string) (JobStatus, error)
	ListJobs(limit int) ([]JobSummary, error)
	CancelJob(jobID string) error
}

func NewServer(service Service) http.Handler {
	return NewServerWithNotifier(service, nil)
}

func NewServerWithNotifier(service Service, hub *notify.WebSocketHub) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", handleWebSocket(hub))
	mux.HandleFunc("/v1/jobs", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/jobs" {
			handleJobByID(w, r, service)
			return
		}
		switch r.Method {
		case http.MethodPost:
			handleCreateJob(w, r, service)
		case http.MethodGet:
			handleListJobs(w, r, service)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/v1/jobs/", func(w http.ResponseWriter, r *http.Request) {
		handleJobByID(w, r, service)
	})
	return mux
}

func jobIDFromPath(path string) string {
	trimmed := strings.TrimPrefix(path, "/v1/jobs/")
	trimmed = strings.TrimSuffix(trimmed, "/cancel")
	return strings.Trim(trimmed, "/")
}
