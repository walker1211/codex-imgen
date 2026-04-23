package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
)

func handleCreateJob(w http.ResponseWriter, r *http.Request, service Service) {
	var req CreateJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(Envelope{OK: false, Error: &APIError{Code: "invalid_argument", Message: err.Error()}})
		return
	}
	result, err := service.CreateJob(req)
	if err != nil {
		status := http.StatusInternalServerError
		code := "internal_error"
		if err.Error() == "prompt is required" {
			status = http.StatusBadRequest
			code = "invalid_argument"
		}
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(Envelope{OK: false, Error: &APIError{Code: code, Message: err.Error()}})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(Envelope{OK: true, Data: result})
}

func handleListJobs(w http.ResponseWriter, r *http.Request, service Service) {
	limit := 20
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	result, err := service.ListJobs(limit)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(Envelope{OK: false, Error: &APIError{Code: "internal_error", Message: err.Error()}})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(Envelope{OK: true, Data: result})
}

func handleJobByID(w http.ResponseWriter, r *http.Request, service Service) {
	jobID := jobIDFromPath(r.URL.Path)
	if strings.HasSuffix(r.URL.Path, "/cancel") {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if err := service.CancelJob(jobID); err != nil {
			status := http.StatusInternalServerError
			code := "internal_error"
			if errors.Is(err, ErrNotFound) || err.Error() == ErrNotFound.Error() {
				status = http.StatusNotFound
				code = "not_found"
			}
			w.WriteHeader(status)
			_ = json.NewEncoder(w).Encode(Envelope{OK: false, Error: &APIError{Code: code, Message: err.Error()}})
			return
		}
		_ = json.NewEncoder(w).Encode(Envelope{OK: true, Data: map[string]string{"job_id": jobID, "status": "cancelled"}})
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	result, err := service.GetJob(jobID)
	if err != nil {
		status := http.StatusInternalServerError
		code := "internal_error"
		if errors.Is(err, ErrNotFound) || err.Error() == ErrNotFound.Error() {
			status = http.StatusNotFound
			code = "not_found"
		}
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(Envelope{OK: false, Error: &APIError{Code: code, Message: err.Error()}})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(Envelope{OK: true, Data: result})
}
