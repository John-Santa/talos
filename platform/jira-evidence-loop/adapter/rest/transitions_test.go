package rest

import (
	"testing"

	"github.com/John-Santa/talos/platform/jira-evidence-loop/domain/evidence"
)

// TAL board transitions used across tests (mirrors real board from #1818):
//   11 → "Por hacer"   new
//   21 → "En curso"    indeterminate
//   31 → "Bloqueado"   indeterminate
//   41 → "En revisión" indeterminate
//   51 → "Listo"       done

var talTransitions = []evidence.Transition{
	{ID: "11", ToName: "Por hacer", ToCategory: "new"},
	{ID: "21", ToName: "En curso", ToCategory: "indeterminate"},
	{ID: "31", ToName: "Bloqueado", ToCategory: "indeterminate"},
	{ID: "41", ToName: "En revisión", ToCategory: "indeterminate"},
	{ID: "51", ToName: "Listo", ToCategory: "done"},
}

var talOrderedStates = map[string][]string{
	"new":           {"Por hacer"},
	"indeterminate": {"En curso", "En revisión", "Bloqueado"},
	"done":          {"Listo"},
}

// TestResolveTransition_SingleCandidate verifies that when exactly one
// transition matches the target category, it is returned directly.
func TestResolveTransition_SingleCandidate(t *testing.T) {
	id, err := resolveTransition(talTransitions, "done", 0, talOrderedStates)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "51" {
		t.Errorf("got id %q, want %q", id, "51")
	}
}

// TestResolveTransition_SingleNew verifies the single-candidate path for the
// "new" category.
func TestResolveTransition_SingleNew(t *testing.T) {
	id, err := resolveTransition(talTransitions, "new", 0, talOrderedStates)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "11" {
		t.Errorf("got id %q, want %q", id, "11")
	}
}

// TestResolveTransition_MultiIndeterminate_Ordinal0 verifies that with 3
// indeterminate candidates and ordinal=0, the first OrderedStates name
// ("En curso") is selected.
func TestResolveTransition_MultiIndeterminate_Ordinal0(t *testing.T) {
	id, err := resolveTransition(talTransitions, "indeterminate", 0, talOrderedStates)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "21" {
		t.Errorf("got id %q, want %q (En curso)", id, "21")
	}
}

// TestResolveTransition_MultiIndeterminate_Ordinal1 verifies ordinal=1 picks
// the second ordered name ("En revisión").
func TestResolveTransition_MultiIndeterminate_Ordinal1(t *testing.T) {
	id, err := resolveTransition(talTransitions, "indeterminate", 1, talOrderedStates)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "41" {
		t.Errorf("got id %q, want %q (En revisión)", id, "41")
	}
}

// TestResolveTransition_MultiIndeterminate_Ordinal2 verifies ordinal=2 picks
// the third ordered name ("Bloqueado").
func TestResolveTransition_MultiIndeterminate_Ordinal2(t *testing.T) {
	id, err := resolveTransition(talTransitions, "indeterminate", 2, talOrderedStates)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "31" {
		t.Errorf("got id %q, want %q (Bloqueado)", id, "31")
	}
}

// TestResolveTransition_NameFallbackToOrdinalPosition verifies that when
// OrderedStates name doesn't match any candidate, it falls back to
// candidates[ordinal] by position.
func TestResolveTransition_NameFallbackToOrdinalPosition(t *testing.T) {
	// OrderedStates has a name that doesn't exist in the transitions.
	orderedStates := map[string][]string{
		"indeterminate": {"NonExistentState", "En revisión", "Bloqueado"},
	}
	candidates := []evidence.Transition{
		{ID: "21", ToName: "En curso", ToCategory: "indeterminate"},
		{ID: "31", ToName: "Bloqueado", ToCategory: "indeterminate"},
		{ID: "41", ToName: "En revisión", ToCategory: "indeterminate"},
	}
	// ordinal=0, name "NonExistentState" doesn't match → fallback to candidates[0]
	id, err := resolveTransition(candidates, "indeterminate", 0, orderedStates)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "21" {
		t.Errorf("got id %q, want %q (fallback to candidates[0])", id, "21")
	}
}

// TestResolveTransition_NoCandidates verifies that when no transition exists
// for the target category, resolveTransition returns ("", nil) to signal a
// no-op (issue assumed already in target state).
func TestResolveTransition_NoCandidates(t *testing.T) {
	// Only "new" and "done" transitions — no indeterminate.
	transitions := []evidence.Transition{
		{ID: "11", ToName: "Por hacer", ToCategory: "new"},
		{ID: "51", ToName: "Listo", ToCategory: "done"},
	}
	id, err := resolveTransition(transitions, "indeterminate", 0, talOrderedStates)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "" {
		t.Errorf("got id %q, want empty string (no-op)", id)
	}
}

// TestResolveTransition_CaseInsensitiveNameMatch verifies the name comparison
// is case-insensitive.
func TestResolveTransition_CaseInsensitiveNameMatch(t *testing.T) {
	transitions := []evidence.Transition{
		{ID: "21", ToName: "EN CURSO", ToCategory: "indeterminate"},
		{ID: "31", ToName: "Bloqueado", ToCategory: "indeterminate"},
	}
	orderedStates := map[string][]string{
		"indeterminate": {"en curso", "Bloqueado"},
	}
	id, err := resolveTransition(transitions, "indeterminate", 0, orderedStates)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "21" {
		t.Errorf("got id %q, want %q (case-insensitive match)", id, "21")
	}
}

// TestResolveTransition_OrdinalOutOfBounds verifies that when ordinal exceeds
// both the ordered names list and the candidates slice length, an error is
// returned (never guess between writes). Requires 2+ candidates so the
// multi-candidate code path is reached.
func TestResolveTransition_OrdinalOutOfBounds(t *testing.T) {
	candidates := []evidence.Transition{
		{ID: "21", ToName: "En curso", ToCategory: "indeterminate"},
		{ID: "31", ToName: "Bloqueado", ToCategory: "indeterminate"},
	}
	orderedStates := map[string][]string{
		// Only 1 ordered name; ordinal=5 exceeds both orderedNames (len=1) and
		// candidates (len=2), so no name match and no positional fallback.
		"indeterminate": {"En curso"},
	}
	// ordinal=5, 2 candidates, 1 ordered name — out of bounds for both paths
	_, err := resolveTransition(candidates, "indeterminate", 5, orderedStates)
	if err == nil {
		t.Error("expected error for out-of-bounds ordinal, got nil")
	}
}
