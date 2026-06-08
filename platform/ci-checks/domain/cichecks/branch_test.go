package cichecks_test

import (
	"errors"
	"testing"

	"github.com/John-Santa/talos/platform/ci-checks/domain/cichecks"
)

func TestParseAgentBranch(t *testing.T) {
	tests := []struct {
		name       string
		branch     string
		wantFigura string
		wantKey    string
		wantErr    bool
	}{
		{
			name:       "canonical lowercase figura",
			branch:     "agent/hermes/TAL-7",
			wantFigura: "hermes",
			wantKey:    "TAL-7",
		},
		{
			name:    "uppercase figura rejected by regex",
			branch:  "agent/Hermes/TAL-7",
			wantErr: true,
		},
		{
			name:    "develop branch",
			branch:  "develop",
			wantErr: true,
		},
		{
			name:    "agent without jira key",
			branch:  "agent/hermes/feature",
			wantErr: true,
		},
		{
			name:    "arbitrary branch",
			branch:  "fix/typo",
			wantErr: true,
		},
		{
			name:    "empty string",
			branch:  "",
			wantErr: true,
		},
		{
			name:    "non-TAL key",
			branch:  "agent/hermes/JIRA-7",
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			figura, key, err := cichecks.ParseAgentBranch(tc.branch)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got figura=%q key=%q", figura, key)
				}
				if !errors.Is(err, cichecks.ErrNoJiraKey) {
					t.Errorf("error = %v, want ErrNoJiraKey", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if figura != tc.wantFigura {
				t.Errorf("figura = %q, want %q", figura, tc.wantFigura)
			}
			if key != tc.wantKey {
				t.Errorf("key = %q, want %q", key, tc.wantKey)
			}
		})
	}
}
