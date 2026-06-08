package worktree_test

import (
	"testing"

	"github.com/John-Santa/talos/platform/worktree-orchestrator/domain/worktree"
)

func TestParseWorktreeList(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    []worktree.WorktreeInfo
		wantErr bool
	}{
		{
			name:  "empty_string",
			input: "",
			want:  []worktree.WorktreeInfo{},
		},
		{
			name: "main_only_no_agent_entries",
			input: "worktree /path/to/repo\n" +
				"HEAD abc123\n" +
				"branch refs/heads/main\n" +
				"\n",
			want: []worktree.WorktreeInfo{},
		},
		{
			name: "bare_worktree_only",
			input: "worktree /path/to/repo\n" +
				"HEAD abc123\n" +
				"bare\n" +
				"\n",
			want: []worktree.WorktreeInfo{},
		},
		{
			name: "one_agent_worktree",
			input: "worktree /repo\n" +
				"HEAD abc123\n" +
				"branch refs/heads/main\n" +
				"\n" +
				"worktree /repo/talos.wt/agent-atlas\n" +
				"HEAD def456\n" +
				"branch refs/heads/agent/atlas/TAL-1\n" +
				"\n",
			want: []worktree.WorktreeInfo{
				{
					Path:   "/repo/talos.wt/agent-atlas",
					Head:   "def456",
					Branch: "agent/atlas/TAL-1",
				},
			},
		},
		{
			name: "multiple_agent_worktrees",
			input: "worktree /repo\n" +
				"HEAD abc123\n" +
				"branch refs/heads/develop\n" +
				"\n" +
				"worktree /repo/talos.wt/agent-atlas\n" +
				"HEAD aaa111\n" +
				"branch refs/heads/agent/atlas/TAL-1\n" +
				"\n" +
				"worktree /repo/talos.wt/agent-hermes\n" +
				"HEAD bbb222\n" +
				"branch refs/heads/agent/hermes/TAL-2\n" +
				"\n",
			want: []worktree.WorktreeInfo{
				{
					Path:   "/repo/talos.wt/agent-atlas",
					Head:   "aaa111",
					Branch: "agent/atlas/TAL-1",
				},
				{
					Path:   "/repo/talos.wt/agent-hermes",
					Head:   "bbb222",
					Branch: "agent/hermes/TAL-2",
				},
			},
		},
		{
			name: "detached_head_entry",
			input: "worktree /repo\n" +
				"HEAD abc123\n" +
				"branch refs/heads/develop\n" +
				"\n" +
				"worktree /repo/talos.wt/agent-iris\n" +
				"HEAD ccc333\n" +
				"detached\n" +
				"\n",
			want: []worktree.WorktreeInfo{
				{
					Path:     "/repo/talos.wt/agent-iris",
					Head:     "ccc333",
					Branch:   "",
					Detached: true,
				},
			},
		},
		{
			name: "leading_trailing_blank_lines",
			input: "\n\n" +
				"worktree /repo\n" +
				"HEAD abc123\n" +
				"branch refs/heads/develop\n" +
				"\n" +
				"worktree /repo/talos.wt/agent-gaia\n" +
				"HEAD ddd444\n" +
				"branch refs/heads/agent/gaia/TAL-3\n" +
				"\n\n",
			want: []worktree.WorktreeInfo{
				{
					Path:   "/repo/talos.wt/agent-gaia",
					Head:   "ddd444",
					Branch: "agent/gaia/TAL-3",
				},
			},
		},
		{
			name: "refs_heads_prefix_stripped",
			input: "worktree /repo/talos.wt/agent-cronos\n" +
				"HEAD eee555\n" +
				"branch refs/heads/agent/cronos/TAL-5\n" +
				"\n",
			want: []worktree.WorktreeInfo{
				{
					Path:   "/repo/talos.wt/agent-cronos",
					Head:   "eee555",
					Branch: "agent/cronos/TAL-5",
				},
			},
		},
		{
			name:    "malformed_block_returns_error",
			input:   "not a valid porcelain format\x00garbage",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := worktree.ParseWorktreeList(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("ParseWorktreeList() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseWorktreeList() unexpected error: %v", err)
			}
			// Normalize nil vs empty slice for comparison
			if len(got) == 0 && len(tc.want) == 0 {
				return
			}
			if len(got) != len(tc.want) {
				t.Fatalf("ParseWorktreeList() returned %d entries, want %d\ngot: %+v", len(got), len(tc.want), got)
			}
			for i, w := range tc.want {
				g := got[i]
				if g.Path != w.Path {
					t.Errorf("entry[%d].Path = %q, want %q", i, g.Path, w.Path)
				}
				if g.Head != w.Head {
					t.Errorf("entry[%d].Head = %q, want %q", i, g.Head, w.Head)
				}
				if g.Branch != w.Branch {
					t.Errorf("entry[%d].Branch = %q, want %q", i, g.Branch, w.Branch)
				}
				if g.Detached != w.Detached {
					t.Errorf("entry[%d].Detached = %v, want %v", i, g.Detached, w.Detached)
				}
				if g.Bare != w.Bare {
					t.Errorf("entry[%d].Bare = %v, want %v", i, g.Bare, w.Bare)
				}
			}
		})
	}
}
