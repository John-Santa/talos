// Package httpapi exposes the gateway over HTTP/JSON.
package httpapi

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/John-Santa/talos/platform/webapi/domain"
)

// Service is the inbound port the handlers depend on.
type Service interface {
	Orchestration(ctx context.Context) (domain.OrchestrationSnapshot, error)
	Agents(ctx context.Context) []domain.Agent
	Agent(ctx context.Context, figura string) (domain.AgentDetail, error)
	Judgment(ctx context.Context, jiraKey string) (domain.JudgmentReview, error)
}

// New wires the read-only routes and CORS for the given allowed origin
// (empty = "*").
func New(svc Service, corsOrigin string) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	mux.HandleFunc("GET /api/orchestration", func(w http.ResponseWriter, r *http.Request) {
		snap, err := svc.Orchestration(r.Context())
		if err != nil {
			writeError(w, http.StatusBadGateway, err)
			return
		}
		writeJSON(w, http.StatusOK, snap)
	})

	mux.HandleFunc("GET /api/agents", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, svc.Agents(r.Context()))
	})

	mux.HandleFunc("GET /api/agents/{figura}", func(w http.ResponseWriter, r *http.Request) {
		detail, err := svc.Agent(r.Context(), r.PathValue("figura"))
		if err != nil {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeJSON(w, http.StatusOK, detail)
	})

	mux.HandleFunc("GET /api/judgment/{jiraKey}", func(w http.ResponseWriter, r *http.Request) {
		review, err := svc.Judgment(r.Context(), r.PathValue("jiraKey"))
		if err != nil {
			writeError(w, http.StatusBadGateway, err)
			return
		}
		writeJSON(w, http.StatusOK, review)
	})

	return withCORS(corsOrigin, mux)
}

func withCORS(origin string, next http.Handler) http.Handler {
	if origin == "" {
		origin = "*"
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
