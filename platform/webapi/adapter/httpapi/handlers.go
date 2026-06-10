// Package httpapi exposes the gateway over HTTP/JSON.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/John-Santa/talos/platform/webapi/domain"
)

// Service is the inbound port the handlers depend on.
type Service interface {
	Ready(ctx context.Context) error
	Orchestration(ctx context.Context) (domain.OrchestrationSnapshot, error)
	Agents(ctx context.Context) []domain.Agent
	Agent(ctx context.Context, figura string) (domain.AgentDetail, error)
	Judgment(ctx context.Context, jiraKey string) (domain.JudgmentReview, error)

	CreateWorktree(ctx context.Context, figura, jiraKey string) error
	TeardownWorktree(ctx context.Context, figura string) error
	MergeWorktree(ctx context.Context, figura, jiraKey string) error
}

// New wires the read-only routes, CORS, and request logging.
func New(svc Service, corsOrigin string, logger *slog.Logger) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	mux.HandleFunc("GET /api/readyz", func(w http.ResponseWriter, r *http.Request) {
		if err := svc.Ready(r.Context()); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable", "error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
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

	mux.HandleFunc("POST /api/worktrees", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Figura  string `json:"figura"`
			JiraKey string `json:"jiraKey"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.Figura == "" || in.JiraKey == "" {
			writeError(w, http.StatusBadRequest, errors.New("figura and jiraKey are required"))
			return
		}
		if err := svc.CreateWorktree(r.Context(), in.Figura, in.JiraKey); err != nil {
			if errors.Is(err, domain.ErrUnknownFigura) {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			writeError(w, http.StatusBadGateway, err)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]string{"status": "created"})
	})

	mux.HandleFunc("DELETE /api/worktrees/{figura}", func(w http.ResponseWriter, r *http.Request) {
		if err := svc.TeardownWorktree(r.Context(), r.PathValue("figura")); err != nil {
			writeError(w, http.StatusBadGateway, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "removed"})
	})

	mux.HandleFunc("POST /api/merge/{jiraKey}", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Figura string `json:"figura"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.Figura == "" {
			writeError(w, http.StatusBadRequest, errors.New("figura is required"))
			return
		}
		if err := svc.MergeWorktree(r.Context(), in.Figura, r.PathValue("jiraKey")); err != nil {
			writeError(w, http.StatusBadGateway, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "merged"})
	})

	return withLogging(logger, withCORS(corsOrigin, mux))
}

func withCORS(origin string, next http.Handler) http.Handler {
	if origin == "" {
		origin = "*"
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func withLogging(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		logger.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", sw.status,
			"dur", time.Since(start).String(),
		)
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
