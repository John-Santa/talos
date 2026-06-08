package cichecks_test

import (
	"testing"

	"github.com/John-Santa/talos/platform/ci-checks/domain/cichecks"
)

func TestParseLabels(t *testing.T) {
	tests := []struct {
		name   string
		raw    []string
		wantN  int
		wantKV [][2]string
	}{
		{
			name:   "single agent",
			raw:    []string{"agent:hermes"},
			wantN:  1,
			wantKV: [][2]string{{"agent", "hermes"}},
		},
		{
			name:   "agent and module",
			raw:    []string{"agent:hermes", "module:devops"},
			wantN:  2,
			wantKV: [][2]string{{"agent", "hermes"}, {"module", "devops"}},
		},
		{
			name:   "label without colon is skipped",
			raw:    []string{"nocoIon"},
			wantN:  0,
			wantKV: nil,
		},
		{
			name:   "empty list",
			raw:    []string{},
			wantN:  0,
			wantKV: nil,
		},
		{
			name:   "mixed valid and invalid",
			raw:    []string{"agent:hermes", "nocoIon", "module:devops"},
			wantN:  2,
			wantKV: [][2]string{{"agent", "hermes"}, {"module", "devops"}},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := cichecks.ParseLabels(tc.raw)
			if len(got) != tc.wantN {
				t.Fatalf("ParseLabels() len=%d, want %d", len(got), tc.wantN)
			}
			for i, kv := range tc.wantKV {
				if got[i].Key != kv[0] || got[i].Value != kv[1] {
					t.Errorf("got[%d] = {%q,%q}, want {%q,%q}", i, got[i].Key, got[i].Value, kv[0], kv[1])
				}
			}
		})
	}
}

func TestLabelSet_Get(t *testing.T) {
	ls := cichecks.ParseLabels([]string{"agent:hermes", "module:devops"})

	tests := []struct {
		key   string
		want  string
		found bool
	}{
		{"agent", "hermes", true},
		{"module", "devops", true},
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

func TestLabelSet_Validate_OK(t *testing.T) {
	ls := cichecks.ParseLabels([]string{"agent:hermes", "module:devops"})
	if err := ls.Validate([]string{"agent", "module"}); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLabelSet_Validate_MissingAgent(t *testing.T) {
	ls := cichecks.ParseLabels([]string{"module:devops"})
	err := ls.Validate([]string{"agent", "module"})
	if err == nil {
		t.Fatal("expected error for missing agent, got nil")
	}
}

func TestLabelSet_Validate_DuplicateAgent(t *testing.T) {
	ls := cichecks.ParseLabels([]string{"agent:hermes", "agent:atlas", "module:devops"})
	err := ls.Validate([]string{"agent", "module"})
	if err == nil {
		t.Fatal("expected error for duplicate agent, got nil")
	}
}

func TestLabelSet_Validate_MissingModule(t *testing.T) {
	ls := cichecks.ParseLabels([]string{"agent:hermes"})
	err := ls.Validate([]string{"agent", "module"})
	if err == nil {
		t.Fatal("expected error for missing module, got nil")
	}
}

func TestLabelSet_Validate_DuplicateModule(t *testing.T) {
	ls := cichecks.ParseLabels([]string{"agent:hermes", "module:devops", "module:qa"})
	err := ls.Validate([]string{"agent", "module"})
	if err == nil {
		t.Fatal("expected error for duplicate module, got nil")
	}
}
