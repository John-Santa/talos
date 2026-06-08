package overlap_test

import (
	"testing"

	"github.com/John-Santa/talos/platform/overlap-guard/domain/overlap"
)

func TestNewClaim_NormalizesFiles(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		agent         string
		module        string
		ref           string
		files         []string
		source        overlap.ClaimSource
		wantFiles     []string
		wantAgent     string
		wantModule    string
		wantRef       string
		wantSource    overlap.ClaimSource
	}{
		{
			name:       "trims whitespace and deduplicates",
			agent:      "atlas",
			module:     "mod:core",
			ref:        "branch-a",
			files:      []string{"  b.go  ", "a.go", "b.go", " a.go"},
			source:     overlap.SourceDeclared,
			wantFiles:  []string{"a.go", "b.go"},
			wantAgent:  "atlas",
			wantModule: "mod:core",
			wantRef:    "branch-a",
			wantSource: overlap.SourceDeclared,
		},
		{
			name:       "sorts files lexicographically",
			agent:      "hermes",
			module:     "mod:qa",
			ref:        "branch-b",
			files:      []string{"z.go", "a.go", "m.go"},
			source:     overlap.SourceActual,
			wantFiles:  []string{"a.go", "m.go", "z.go"},
			wantAgent:  "hermes",
			wantModule: "mod:qa",
			wantRef:    "branch-b",
			wantSource: overlap.SourceActual,
		},
		{
			name:       "empty files list",
			agent:      "cronos",
			module:     "mod:data",
			ref:        "branch-c",
			files:      []string{},
			source:     overlap.SourceDeclared,
			wantFiles:  []string{},
			wantAgent:  "cronos",
			wantModule: "mod:data",
			wantRef:    "branch-c",
			wantSource: overlap.SourceDeclared,
		},
		{
			name:       "SourceActual is preserved",
			agent:      "iris",
			module:     "mod:ui",
			ref:        "branch-d",
			files:      []string{"component.tsx"},
			source:     overlap.SourceActual,
			wantFiles:  []string{"component.tsx"},
			wantAgent:  "iris",
			wantModule: "mod:ui",
			wantRef:    "branch-d",
			wantSource: overlap.SourceActual,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := overlap.NewClaim(tc.agent, tc.module, tc.ref, tc.files, tc.source)

			if c.Agent != tc.wantAgent {
				t.Errorf("Agent = %q, want %q", c.Agent, tc.wantAgent)
			}
			if c.Module != tc.wantModule {
				t.Errorf("Module = %q, want %q", c.Module, tc.wantModule)
			}
			if c.Ref != tc.wantRef {
				t.Errorf("Ref = %q, want %q", c.Ref, tc.wantRef)
			}
			if c.Source != tc.wantSource {
				t.Errorf("Source = %v, want %v", c.Source, tc.wantSource)
			}
			if len(c.Files) != len(tc.wantFiles) {
				t.Errorf("Files length = %d, want %d; got %v", len(c.Files), len(tc.wantFiles), c.Files)
				return
			}
			for i, f := range c.Files {
				if f != tc.wantFiles[i] {
					t.Errorf("Files[%d] = %q, want %q", i, f, tc.wantFiles[i])
				}
			}
		})
	}
}
