package evidence_test

import (
	"testing"

	"github.com/John-Santa/talos/platform/jira-evidence-loop/domain/evidence"
)

// ---------------------------------------------------------------------------
// Label
// ---------------------------------------------------------------------------

func TestLabel_String(t *testing.T) {
	tests := []struct {
		name  string
		label evidence.Label
		want  string
	}{
		{"simple", evidence.Label{Key: "agent", Value: "hermes"}, "agent:hermes"},
		{"change", evidence.Label{Key: "change", Value: "TAL-1"}, "change:TAL-1"},
		{"phase", evidence.Label{Key: "phase", Value: "apply"}, "phase:apply"},
		{"module", evidence.Label{Key: "module", Value: "jira-loop"}, "module:jira-loop"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.label.String()
			if got != tc.want {
				t.Errorf("Label.String() = %q, want %q", got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// LabelSet
// ---------------------------------------------------------------------------

func TestLabelSet_Get(t *testing.T) {
	ls := evidence.LabelSet{
		evidence.Label{Key: "agent", Value: "hermes"},
		evidence.Label{Key: "module", Value: "jira-loop"},
		evidence.Label{Key: "change", Value: "TAL-1"},
		evidence.Label{Key: "phase", Value: "apply"},
	}

	tests := []struct {
		key   string
		want  string
		found bool
	}{
		{"agent", "hermes", true},
		{"module", "jira-loop", true},
		{"change", "TAL-1", true},
		{"phase", "apply", true},
		{"missing", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.key, func(t *testing.T) {
			got, ok := ls.Get(tc.key)
			if ok != tc.found {
				t.Fatalf("Get(%q) found=%v, want %v", tc.key, ok, tc.found)
			}
			if ok && got != tc.want {
				t.Errorf("Get(%q) = %q, want %q", tc.key, got, tc.want)
			}
		})
	}
}

func TestLabelSet_Strings(t *testing.T) {
	ls := evidence.LabelSet{
		evidence.Label{Key: "agent", Value: "hermes"},
		evidence.Label{Key: "change", Value: "TAL-1"},
	}
	got := ls.Strings()
	want := []string{"agent:hermes", "change:TAL-1"}
	if len(got) != len(want) {
		t.Fatalf("Strings() len=%d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Strings()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// ---------------------------------------------------------------------------
// LabelSet validation: exactly-one per key
// ---------------------------------------------------------------------------

func TestLabelSet_Validate(t *testing.T) {
	requiredKeys := []string{"agent", "module", "change", "phase"}

	makeValid := func() evidence.LabelSet {
		return evidence.LabelSet{
			evidence.Label{Key: "agent", Value: "hermes"},
			evidence.Label{Key: "module", Value: "jira-loop"},
			evidence.Label{Key: "change", Value: "TAL-1"},
			evidence.Label{Key: "phase", Value: "apply"},
		}
	}

	t.Run("valid set passes", func(t *testing.T) {
		if err := makeValid().Validate(requiredKeys); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("missing required key", func(t *testing.T) {
		ls := evidence.LabelSet{
			evidence.Label{Key: "agent", Value: "hermes"},
			// module missing
			evidence.Label{Key: "change", Value: "TAL-1"},
			evidence.Label{Key: "phase", Value: "apply"},
		}
		if err := ls.Validate(requiredKeys); err == nil {
			t.Error("expected error for missing key, got nil")
		}
	})

	t.Run("duplicate key", func(t *testing.T) {
		ls := evidence.LabelSet{
			evidence.Label{Key: "agent", Value: "hermes"},
			evidence.Label{Key: "agent", Value: "zeus"}, // duplicate
			evidence.Label{Key: "module", Value: "jira-loop"},
			evidence.Label{Key: "change", Value: "TAL-1"},
			evidence.Label{Key: "phase", Value: "apply"},
		}
		if err := ls.Validate(requiredKeys); err == nil {
			t.Error("expected error for duplicate key, got nil")
		}
	})
}

// ---------------------------------------------------------------------------
// ADFDocument
// ---------------------------------------------------------------------------

func TestADFDocument_NotEmpty(t *testing.T) {
	doc := evidence.NewADFDocument("Hello world")
	if doc.Version != 1 {
		t.Errorf("Version = %d, want 1", doc.Version)
	}
	if doc.Type != "doc" {
		t.Errorf("Type = %q, want \"doc\"", doc.Type)
	}
	if len(doc.Content) == 0 {
		t.Error("Content is empty")
	}
}

func TestADFDocument_Empty(t *testing.T) {
	doc := evidence.NewADFDocument("")
	if len(doc.Content) == 0 {
		t.Error("even empty text should produce a paragraph node")
	}
}

// ---------------------------------------------------------------------------
// Issue (value object)
// ---------------------------------------------------------------------------

func TestIssue_Key(t *testing.T) {
	issue := evidence.Issue{Key: "TAL-1", Summary: "test"}
	if issue.Key != "TAL-1" {
		t.Errorf("Key = %q, want \"TAL-1\"", issue.Key)
	}
}

// ---------------------------------------------------------------------------
// Transition (value object)
// ---------------------------------------------------------------------------

func TestTransition_Fields(t *testing.T) {
	tr := evidence.Transition{ID: "21", ToName: "En curso", ToCategory: "indeterminate"}
	if tr.ID != "21" {
		t.Errorf("ID = %q, want \"21\"", tr.ID)
	}
	if tr.ToCategory != "indeterminate" {
		t.Errorf("ToCategory = %q, want \"indeterminate\"", tr.ToCategory)
	}
}

// ---------------------------------------------------------------------------
// Worklog (value object)
// ---------------------------------------------------------------------------

func TestWorklog_DurationString(t *testing.T) {
	tests := []struct {
		seconds int
		want    string
	}{
		{3600, "1h"},
		{7200, "2h"},
		{1800, "30m"},
		{5400, "1h 30m"},
		{60, "1m"},
		{3661, "1h 1m"},
	}
	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			w := evidence.Worklog{TimeSpentSeconds: tc.seconds}
			got := w.DurationString()
			if got != tc.want {
				t.Errorf("DurationString() = %q, want %q", got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// RemoteLink (value object)
// ---------------------------------------------------------------------------

func TestRemoteLink_GlobalID(t *testing.T) {
	rl := evidence.RemoteLink{PRURL: "https://github.com/org/repo/pull/1"}
	want := "pr=https://github.com/org/repo/pull/1"
	if rl.GlobalID() != want {
		t.Errorf("GlobalID() = %q, want %q", rl.GlobalID(), want)
	}
}
