package cichecks_test

import (
	"testing"

	"github.com/John-Santa/talos/platform/ci-checks/domain/cichecks"
)

func TestCheckLabelInvariant_OK(t *testing.T) {
	input := cichecks.InvariantInput{
		Labels:       cichecks.ParseLabels([]string{"agent:hermes", "module:devops"}),
		Ownership:    map[string]string{"module:devops": "hermes"},
		BranchFigura: "hermes",
	}
	result, err := cichecks.CheckLabelInvariant(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Violations) != 0 {
		t.Errorf("expected no violations, got: %v", result.Violations)
	}
}

func TestCheckLabelInvariant_ZeroAgent(t *testing.T) {
	input := cichecks.InvariantInput{
		Labels:       cichecks.ParseLabels([]string{"module:devops"}),
		Ownership:    map[string]string{"module:devops": "hermes"},
		BranchFigura: "hermes",
	}
	result, _ := cichecks.CheckLabelInvariant(input)
	if len(result.Violations) == 0 {
		t.Fatal("expected at least one violation for zero agent labels")
	}
	found := false
	for _, v := range result.Violations {
		if containsAll(v, "0", "agent") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("violations %v do not mention '0 labels agent:*'", result.Violations)
	}
}

func TestCheckLabelInvariant_TwoAgents(t *testing.T) {
	input := cichecks.InvariantInput{
		Labels:       cichecks.ParseLabels([]string{"agent:hermes", "agent:atlas", "module:devops"}),
		Ownership:    map[string]string{"module:devops": "hermes"},
		BranchFigura: "hermes",
	}
	result, _ := cichecks.CheckLabelInvariant(input)
	if len(result.Violations) == 0 {
		t.Fatal("expected at least one violation for two agent labels")
	}
	found := false
	for _, v := range result.Violations {
		if containsAll(v, "2", "agent") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("violations %v do not mention '2 labels agent:*'", result.Violations)
	}
}

func TestCheckLabelInvariant_ZeroModule(t *testing.T) {
	input := cichecks.InvariantInput{
		Labels:       cichecks.ParseLabels([]string{"agent:hermes"}),
		Ownership:    map[string]string{"module:devops": "hermes"},
		BranchFigura: "hermes",
	}
	result, _ := cichecks.CheckLabelInvariant(input)
	if len(result.Violations) == 0 {
		t.Fatal("expected at least one violation for zero module labels")
	}
	found := false
	for _, v := range result.Violations {
		if containsAll(v, "0", "module") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("violations %v do not mention '0 labels module:*'", result.Violations)
	}
}

func TestCheckLabelInvariant_TwoModules(t *testing.T) {
	input := cichecks.InvariantInput{
		Labels:       cichecks.ParseLabels([]string{"agent:hermes", "module:devops", "module:qa"}),
		Ownership:    map[string]string{"module:devops": "hermes"},
		BranchFigura: "hermes",
	}
	result, _ := cichecks.CheckLabelInvariant(input)
	if len(result.Violations) == 0 {
		t.Fatal("expected at least one violation for two module labels")
	}
	found := false
	for _, v := range result.Violations {
		if containsAll(v, "2", "module") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("violations %v do not mention '2 labels module:*'", result.Violations)
	}
}

func TestCheckLabelInvariant_OwnershipMismatch(t *testing.T) {
	input := cichecks.InvariantInput{
		Labels:       cichecks.ParseLabels([]string{"agent:atlas", "module:devops"}),
		Ownership:    map[string]string{"module:devops": "hermes"},
		BranchFigura: "atlas",
	}
	result, _ := cichecks.CheckLabelInvariant(input)
	if len(result.Violations) == 0 {
		t.Fatal("expected ownership violation")
	}
}

func TestCheckLabelInvariant_UnknownModule(t *testing.T) {
	input := cichecks.InvariantInput{
		Labels:       cichecks.ParseLabels([]string{"agent:hermes", "module:unknown"}),
		Ownership:    map[string]string{"module:devops": "hermes"},
		BranchFigura: "hermes",
	}
	result, _ := cichecks.CheckLabelInvariant(input)
	if len(result.Violations) == 0 {
		t.Fatal("expected violation for unknown module")
	}
}

func TestCheckLabelInvariant_BranchFiguraMismatch(t *testing.T) {
	input := cichecks.InvariantInput{
		Labels:       cichecks.ParseLabels([]string{"agent:atlas", "module:devops"}),
		Ownership:    map[string]string{"module:devops": "atlas"},
		BranchFigura: "hermes",
	}
	result, _ := cichecks.CheckLabelInvariant(input)
	if len(result.Violations) == 0 {
		t.Fatal("expected violation for branch figura != label agent")
	}
	found := false
	for _, v := range result.Violations {
		if containsAll(v, "hermes", "atlas") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("violations %v do not identify branch figura vs label agent mismatch", result.Violations)
	}
}

func TestCheckLabelInvariant_MultipleViolations(t *testing.T) {
	input := cichecks.InvariantInput{
		Labels:       cichecks.ParseLabels([]string{}),
		Ownership:    map[string]string{"module:devops": "hermes"},
		BranchFigura: "hermes",
	}
	result, _ := cichecks.CheckLabelInvariant(input)
	if len(result.Violations) < 2 {
		t.Errorf("expected ≥2 violations, got %d: %v", len(result.Violations), result.Violations)
	}
}

func TestCheckLabelInvariant_AllViolations(t *testing.T) {
	input := cichecks.InvariantInput{
		Labels:       cichecks.ParseLabels([]string{}),
		Ownership:    map[string]string{"module:devops": "hermes"},
		BranchFigura: "hermes",
	}
	result, _ := cichecks.CheckLabelInvariant(input)

	hasAgent := false
	hasModule := false
	for _, v := range result.Violations {
		if containsAll(v, "0", "agent") {
			hasAgent = true
		}
		if containsAll(v, "0", "module") {
			hasModule = true
		}
	}
	if !hasAgent {
		t.Errorf("missing agent violation in: %v", result.Violations)
	}
	if !hasModule {
		t.Errorf("missing module violation in: %v", result.Violations)
	}
}

func TestCheckLabelInvariant_Verdict_OK_Exit0(t *testing.T) {
	input := cichecks.InvariantInput{
		Labels:       cichecks.ParseLabels([]string{"agent:hermes", "module:devops"}),
		Ownership:    map[string]string{"module:devops": "hermes"},
		BranchFigura: "hermes",
	}
	result, err := cichecks.CheckLabelInvariant(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Violations) != 0 {
		t.Errorf("expected 0 violations (exit 0 condition), got: %v", result.Violations)
	}
}

// containsAll returns true when s contains all the given substrings.
func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !containsIgnoreCase(s, sub) {
			return false
		}
	}
	return true
}

func containsIgnoreCase(s, sub string) bool {
	sl := len(sub)
	if sl == 0 {
		return true
	}
	for i := 0; i <= len(s)-sl; i++ {
		if equalFold(s[i:i+sl], sub) {
			return true
		}
	}
	return false
}

func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}
