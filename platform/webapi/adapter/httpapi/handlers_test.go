package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/John-Santa/talos/platform/webapi/domain"
)

type fakeSvc struct{ notReady bool }

func (s fakeSvc) Ready(context.Context) error {
	if s.notReady {
		return context.DeadlineExceeded
	}
	return nil
}

func (fakeSvc) Orchestration(context.Context) (domain.OrchestrationSnapshot, error) {
	return domain.OrchestrationSnapshot{
		Worktrees: []domain.Worktree{{Agent: "hermes", JiraKey: "TAL-15"}},
		Slots:     domain.Slots{Used: 1, Total: 7},
	}, nil
}

func (fakeSvc) Agents(context.Context) []domain.Agent { return domain.AllAgents() }

func (fakeSvc) Agent(_ context.Context, figura string) (domain.AgentDetail, error) {
	a, _ := domain.AgentByID(figura)
	return domain.AgentDetail{Agent: a, DoD: []domain.DoDItem{}, Activity: []domain.ActivityEntry{}}, nil
}

func (fakeSvc) Judgment(_ context.Context, jiraKey string) (domain.JudgmentReview, error) {
	return domain.JudgmentReview{JiraKey: jiraKey, Gate: "HG5", Verdict: "agree"}, nil
}

func do(h http.Handler, method, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
	return rec
}

func TestHealthz(t *testing.T) {
	rec := do(New(fakeSvc{}, "*", nil), http.MethodGet, "/api/healthz")
	if rec.Code != http.StatusOK {
		t.Fatalf("healthz status = %d, want 200", rec.Code)
	}
}

func TestReadyz(t *testing.T) {
	ok := do(New(fakeSvc{}, "*", nil), http.MethodGet, "/api/readyz")
	if ok.Code != http.StatusOK {
		t.Errorf("readyz (ready) = %d, want 200", ok.Code)
	}
	down := do(New(fakeSvc{notReady: true}, "*", nil), http.MethodGet, "/api/readyz")
	if down.Code != http.StatusServiceUnavailable {
		t.Errorf("readyz (not ready) = %d, want 503", down.Code)
	}
}

func TestOrchestrationRoute(t *testing.T) {
	rec := do(New(fakeSvc{}, "*", nil), http.MethodGet, "/api/orchestration")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("missing CORS header")
	}
	var snap domain.OrchestrationSnapshot
	if err := json.Unmarshal(rec.Body.Bytes(), &snap); err != nil {
		t.Fatal(err)
	}
	if len(snap.Worktrees) != 1 || snap.Slots.Total != 7 {
		t.Errorf("body mapped wrong: %+v", snap)
	}
}

func TestAgentsRoute(t *testing.T) {
	rec := do(New(fakeSvc{}, "*", nil), http.MethodGet, "/api/agents")
	var agents []domain.Agent
	if err := json.Unmarshal(rec.Body.Bytes(), &agents); err != nil {
		t.Fatal(err)
	}
	if len(agents) != 10 {
		t.Errorf("agents = %d, want 10", len(agents))
	}
}

func TestAgentDetailRoute(t *testing.T) {
	rec := do(New(fakeSvc{}, "*", nil), http.MethodGet, "/api/agents/hermes")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var detail domain.AgentDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
		t.Fatal(err)
	}
	if detail.Agent.Name != "Hermes" {
		t.Errorf("agent = %q, want Hermes", detail.Agent.Name)
	}
}

func TestCORSPreflight(t *testing.T) {
	rec := do(New(fakeSvc{}, "*", nil), http.MethodOptions, "/api/orchestration")
	if rec.Code != http.StatusNoContent {
		t.Errorf("preflight status = %d, want 204", rec.Code)
	}
}
