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
		projectKey string
		wantFigura string
		wantKey    string
		wantErr    bool
	}{
		{
			name:       "canonical lowercase figura",
			branch:     "agent/hermes/TAL-7",
			projectKey: "TAL",
			wantFigura: "hermes",
			wantKey:    "TAL-7",
		},
		{
			name:       "uppercase figura rejected by regex",
			branch:     "agent/Hermes/TAL-7",
			projectKey: "TAL",
			wantErr:    true,
		},
		{
			name:       "develop branch",
			branch:     "develop",
			projectKey: "TAL",
			wantErr:    true,
		},
		{
			name:       "agent without jira key",
			branch:     "agent/hermes/feature",
			projectKey: "TAL",
			wantErr:    true,
		},
		{
			name:       "arbitrary branch",
			branch:     "fix/typo",
			projectKey: "TAL",
			wantErr:    true,
		},
		{
			name:       "empty string",
			branch:     "",
			projectKey: "TAL",
			wantErr:    true,
		},
		{
			name:       "non-TAL key rejected under TAL projectKey",
			branch:     "agent/hermes/JIRA-7",
			projectKey: "TAL",
			wantErr:    true,
		},
		{
			name:       "different project key accepted",
			branch:     "agent/hermes/FOO-7",
			projectKey: "FOO",
			wantFigura: "hermes",
			wantKey:    "FOO-7",
		},
		{
			name:       "FOO branch rejected under TAL projectKey",
			branch:     "agent/hermes/FOO-7",
			projectKey: "TAL",
			wantErr:    true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			figura, key, err := cichecks.ParseAgentBranch(tc.branch, tc.projectKey)
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
