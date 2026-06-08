package mergeorder_test

import (
	"errors"
	"math/rand"
	"testing"
	"time"

	"github.com/John-Santa/talos/platform/merge-order-orchestrator/domain/mergeorder"
)

func makeCandidate(branch string, changedFiles []string, createdAt time.Time) mergeorder.Candidate {
	return mergeorder.Candidate{
		Branch:       branch,
		ChangedFiles: changedFiles,
		CreatedAt:    createdAt,
	}
}

var t0 = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func TestOrder_EmptyInput(t *testing.T) {
	t.Parallel()
	_, err := mergeorder.Order(nil, nil)
	if err == nil {
		t.Fatal("expected ErrNoCandidates, got nil")
	}
	var e *mergeorder.ErrNoCandidates
	if !errors.As(err, &e) {
		t.Errorf("error type = %T, want *ErrNoCandidates", err)
	}
}

func TestOrder_SingleCandidateNoDeps(t *testing.T) {
	t.Parallel()
	c := makeCandidate("feat/a", []string{"a.go"}, t0)
	result, err := mergeorder.Order([]mergeorder.Candidate{c}, nil)
	if err != nil {
		t.Fatalf("Order() unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("len(result) = %d, want 1", len(result))
	}
	if result[0].Branch != "feat/a" {
		t.Errorf("result[0].Branch = %q, want %q", result[0].Branch, "feat/a")
	}
}

func TestOrder_TiebreakByChangedFilesAscending(t *testing.T) {
	t.Parallel()
	now := t0
	a := makeCandidate("feat/a", []string{"x.go", "y.go", "z.go"}, now)
	b := makeCandidate("feat/b", []string{"m.go"}, now)
	result, err := mergeorder.Order([]mergeorder.Candidate{a, b}, nil)
	if err != nil {
		t.Fatalf("Order() unexpected error: %v", err)
	}
	if result[0].Branch != "feat/b" {
		t.Errorf("expected feat/b (fewer changed files) first, got %q", result[0].Branch)
	}
}

func TestOrder_ConflictSurfaceTieBreakFIFO(t *testing.T) {
	t.Parallel()
	earlier := t0
	later := t0.Add(time.Hour)
	a := makeCandidate("feat/a", []string{"x.go"}, later)
	b := makeCandidate("feat/b", []string{"x.go"}, earlier)
	result, err := mergeorder.Order([]mergeorder.Candidate{a, b}, nil)
	if err != nil {
		t.Fatalf("Order() unexpected error: %v", err)
	}
	if result[0].Branch != "feat/b" {
		t.Errorf("expected feat/b (earlier CreatedAt) first, got %q", result[0].Branch)
	}
}

func TestOrder_CreatedAtTie_BranchLexicographic(t *testing.T) {
	t.Parallel()
	now := t0
	a := makeCandidate("feat/z", []string{"x.go"}, now)
	b := makeCandidate("feat/a", []string{"x.go"}, now)
	result, err := mergeorder.Order([]mergeorder.Candidate{a, b}, nil)
	if err != nil {
		t.Fatalf("Order() unexpected error: %v", err)
	}
	if result[0].Branch != "feat/a" {
		t.Errorf("expected feat/a (lexicographically first) first, got %q", result[0].Branch)
	}
}

func TestOrder_SimpleChainDep(t *testing.T) {
	t.Parallel()
	a := makeCandidate("feat/a", nil, t0)
	b := makeCandidate("feat/b", nil, t0)
	// b depends on a → a must come first
	deps := map[string][]string{
		"feat/b": {"feat/a"},
	}
	result, err := mergeorder.Order([]mergeorder.Candidate{b, a}, deps)
	if err != nil {
		t.Fatalf("Order() unexpected error: %v", err)
	}
	if result[0].Branch != "feat/a" {
		t.Errorf("expected feat/a (dependency) first, got %q", result[0].Branch)
	}
	if result[1].Branch != "feat/b" {
		t.Errorf("expected feat/b second, got %q", result[1].Branch)
	}
}

func TestOrder_DiamondDependency(t *testing.T) {
	t.Parallel()
	// A→C, B→C means C must come before A and B
	a := makeCandidate("feat/a", nil, t0)
	b := makeCandidate("feat/b", nil, t0)
	c := makeCandidate("feat/c", nil, t0)
	deps := map[string][]string{
		"feat/a": {"feat/c"},
		"feat/b": {"feat/c"},
	}
	result, err := mergeorder.Order([]mergeorder.Candidate{a, b, c}, deps)
	if err != nil {
		t.Fatalf("Order() unexpected error: %v", err)
	}
	// feat/c must appear before feat/a and feat/b
	cIdx, aIdx, bIdx := -1, -1, -1
	for i, r := range result {
		switch r.Branch {
		case "feat/c":
			cIdx = i
		case "feat/a":
			aIdx = i
		case "feat/b":
			bIdx = i
		}
	}
	if cIdx > aIdx || cIdx > bIdx {
		t.Errorf("feat/c must come before feat/a and feat/b; got order: %v", branchOrder(result))
	}
}

func TestOrder_CycleAB(t *testing.T) {
	t.Parallel()
	a := makeCandidate("feat/a", nil, t0)
	b := makeCandidate("feat/b", nil, t0)
	deps := map[string][]string{
		"feat/a": {"feat/b"},
		"feat/b": {"feat/a"},
	}
	_, err := mergeorder.Order([]mergeorder.Candidate{a, b}, deps)
	if err == nil {
		t.Fatal("expected ErrDependencyCycle, got nil")
	}
	var e *mergeorder.ErrDependencyCycle
	if !errors.As(err, &e) {
		t.Errorf("error type = %T, want *ErrDependencyCycle", err)
	}
	if len(e.Branches) == 0 {
		t.Errorf("ErrDependencyCycle.Branches is empty, want cycle participants")
	}
}

func TestOrder_CycleLongerChain(t *testing.T) {
	t.Parallel()
	a := makeCandidate("feat/a", nil, t0)
	b := makeCandidate("feat/b", nil, t0)
	c := makeCandidate("feat/c", nil, t0)
	deps := map[string][]string{
		"feat/a": {"feat/b"},
		"feat/b": {"feat/c"},
		"feat/c": {"feat/a"},
	}
	_, err := mergeorder.Order([]mergeorder.Candidate{a, b, c}, deps)
	if err == nil {
		t.Fatal("expected ErrDependencyCycle, got nil")
	}
	var e *mergeorder.ErrDependencyCycle
	if !errors.As(err, &e) {
		t.Errorf("error type = %T, want *ErrDependencyCycle", err)
	}
}

func TestOrder_Determinism(t *testing.T) {
	t.Parallel()
	candidates := []mergeorder.Candidate{
		makeCandidate("feat/a", []string{"a.go"}, t0.Add(2*time.Hour)),
		makeCandidate("feat/b", []string{"b.go", "c.go"}, t0.Add(time.Hour)),
		makeCandidate("feat/c", []string{"d.go"}, t0),
		makeCandidate("feat/d", []string{"e.go", "f.go", "g.go"}, t0.Add(3*time.Hour)),
	}
	first, err := mergeorder.Order(candidates, nil)
	if err != nil {
		t.Fatalf("first Order() error: %v", err)
	}
	firstOrder := branchOrder(first)

	rng := rand.New(rand.NewSource(42))
	for i := 0; i < 10; i++ {
		shuffled := make([]mergeorder.Candidate, len(candidates))
		copy(shuffled, candidates)
		rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })

		result, err := mergeorder.Order(shuffled, nil)
		if err != nil {
			t.Fatalf("Order() shuffle %d error: %v", i, err)
		}
		got := branchOrder(result)
		for j, b := range got {
			if b != firstOrder[j] {
				t.Errorf("shuffle %d: result[%d] = %q, want %q (not deterministic)", i, j, b, firstOrder[j])
			}
		}
	}
}

func TestOrder_DisjointChangedFilesRankedFirst(t *testing.T) {
	t.Parallel()
	// Within same topo layer: disjoint files (no overlap) ranked before overlapping
	// Both have 2 changed files, same CreatedAt, same Branch alphabetical later
	// Branch feat/a has unique files, feat/b has files overlapping with some future branch
	// In practice: fewer changed files → lower conflict surface → ranked first
	now := t0
	// feat/a: 1 file (disjoint from everything)
	// feat/b: 3 files (higher conflict surface)
	a := makeCandidate("feat/a", []string{"unique.go"}, now)
	b := makeCandidate("feat/b", []string{"shared1.go", "shared2.go", "shared3.go"}, now)
	result, err := mergeorder.Order([]mergeorder.Candidate{b, a}, nil)
	if err != nil {
		t.Fatalf("Order() unexpected error: %v", err)
	}
	if result[0].Branch != "feat/a" {
		t.Errorf("expected feat/a (fewer changed files = lower conflict surface) first, got %q", result[0].Branch)
	}
}

func branchOrder(cs []mergeorder.Candidate) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.Branch
	}
	return out
}
