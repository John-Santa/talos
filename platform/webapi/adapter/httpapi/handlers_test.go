package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/John-Santa/talos/platform/webapi/domain"
)

// erringSvc overrides specific methods to return errors.
type fakeSvc struct {
	notReady       bool
	createErr      error
	mergeErr       error
}

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

func (s fakeSvc) CreateWorktree(_ context.Context, figura, jiraKey string) error {
	_ = figura
	_ = jiraKey
	return s.createErr
}
func (fakeSvc) TeardownWorktree(context.Context, string) error { return nil }
func (s fakeSvc) MergeWorktree(_ context.Context, figura, jiraKey string) error {
	_ = figura
	_ = jiraKey
	return s.mergeErr
}

func do(h http.Handler, method, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
	return rec
}

func doBody(h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(body)))
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

func TestCreateWorktreeRoute(t *testing.T) {
	rec := doBody(
		New(fakeSvc{}, "*", nil),
		http.MethodPost,
		"/api/worktrees",
		`{"figura":"atlas","jiraKey":"TAL-99"}`,
	)
	if rec.Code != http.StatusCreated {
		t.Errorf("create status = %d, want 201", rec.Code)
	}
}

func TestCreateWorktreeValidation(t *testing.T) {
	rec := doBody(New(fakeSvc{}, "*", nil), http.MethodPost, "/api/worktrees", `{}`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("invalid create status = %d, want 400", rec.Code)
	}
}

func TestTeardownRoute(t *testing.T) {
	rec := do(New(fakeSvc{}, "*", nil), http.MethodDelete, "/api/worktrees/atlas")
	if rec.Code != http.StatusOK {
		t.Errorf("teardown status = %d, want 200", rec.Code)
	}
}

// --- PR2 handler tests -------------------------------------------------------

func TestCreateWorktreeUnknownFigura400(t *testing.T) {
	svc := fakeSvc{createErr: domain.ErrUnknownFigura}
	rec := doBody(New(svc, "*", nil), http.MethodPost, "/api/worktrees",
		`{"figura":"bogus","jiraKey":"TAL-42"}`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("unknown figura status = %d, want 400", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["error"] == "" {
		t.Errorf("expected error field in body, got %v", body)
	}
}

func TestCreateWorktreeServiceError502(t *testing.T) {
	svc := fakeSvc{createErr: errors.New("git broken")}
	rec := doBody(New(svc, "*", nil), http.MethodPost, "/api/worktrees",
		`{"figura":"iris","jiraKey":"TAL-42"}`)
	if rec.Code != http.StatusBadGateway {
		t.Errorf("service error status = %d, want 502", rec.Code)
	}
}

func TestMergeRouteWithFiguraBody(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		mergeErr   error
		wantStatus int
	}{
		{
			name:       "valid merge with figura body → 200",
			body:       `{"figura":"iris"}`,
			wantStatus: http.StatusOK,
		},
		{
			name:       "missing figura in body → 400",
			body:       `{}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "service error → 502",
			body:       `{"figura":"iris"}`,
			mergeErr:   errors.New("no worktree"),
			wantStatus: http.StatusBadGateway,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := fakeSvc{mergeErr: tt.mergeErr}
			rec := doBody(New(svc, "*", nil), http.MethodPost, "/api/merge/TAL-15", tt.body)
			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d (body: %s)", rec.Code, tt.wantStatus, rec.Body.String())
			}
		})
	}
}
