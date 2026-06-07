package worktree

import (
	"fmt"
	"strings"
)

// WorktreeInfo holds the parsed fields from a single block in `git worktree
// list --porcelain` output.
type WorktreeInfo struct {
	Path     string
	Head     string
	Branch   string // refs/heads/ prefix stripped
	Detached bool
	Bare     bool
}

// ParseWorktreeList parses the output of `git worktree list --porcelain` and
// returns the subset of entries that correspond to agent/* branches.
//
// Non-agent entries (main/bare worktrees, develop, etc.) are silently skipped.
// Empty input is valid and returns an empty slice.
// Malformed input (e.g. null bytes, or a worktree block without a HEAD line)
// returns a non-nil error.
//
// The refs/heads/ prefix is stripped from the Branch field on output.
func ParseWorktreeList(stdout string) ([]WorktreeInfo, error) {
	// Null bytes are never valid in porcelain output.
	if strings.ContainsRune(stdout, 0) {
		return nil, fmt.Errorf("porcelain: input contains null bytes (malformed)")
	}

	var result []WorktreeInfo

	// Split on blank-line delimiters to get one block per worktree entry.
	// A block looks like:
	//   worktree <path>
	//   HEAD <sha>
	//   branch refs/heads/<name>   -- OR --
	//   detached                   -- OR --
	//   bare
	blocks := splitBlocks(stdout)

	for _, block := range blocks {
		if block == "" {
			continue
		}
		info, skip, err := parseBlock(block)
		if err != nil {
			return nil, err
		}
		if skip {
			continue
		}
		// Only include agent/* branch entries or detached heads that live in an
		// agent worktree path (path contains "agent-").
		if !info.Detached && !strings.HasPrefix(info.Branch, "agent/") {
			continue
		}
		if info.Detached && !strings.Contains(info.Path, "agent-") {
			continue
		}
		result = append(result, info)
	}

	if result == nil {
		result = []WorktreeInfo{}
	}
	return result, nil
}

// splitBlocks splits the raw porcelain stdout into individual worktree blocks
// separated by one or more blank lines.
func splitBlocks(s string) []string {
	lines := strings.Split(s, "\n")
	var blocks []string
	var current []string

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			if len(current) > 0 {
				blocks = append(blocks, strings.Join(current, "\n"))
				current = current[:0]
			}
			continue
		}
		current = append(current, line)
	}
	if len(current) > 0 {
		blocks = append(blocks, strings.Join(current, "\n"))
	}
	return blocks
}

// parseBlock parses a single worktree block. Returns (info, skip=true, nil) for
// bare worktrees or blocks without a "worktree " header (invalid partial data
// mid-stream is treated as a skip rather than an error unless structure is
// fundamentally broken). Returns (_, _, err) on hard malformed input.
func parseBlock(block string) (WorktreeInfo, bool, error) {
	lines := strings.Split(block, "\n")
	if len(lines) == 0 {
		return WorktreeInfo{}, true, nil
	}

	var info WorktreeInfo
	var hasWorktree, hasHEAD bool

	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "worktree "):
			info.Path = strings.TrimPrefix(line, "worktree ")
			hasWorktree = true
		case strings.HasPrefix(line, "HEAD "):
			info.Head = strings.TrimPrefix(line, "HEAD ")
			hasHEAD = true
		case strings.HasPrefix(line, "branch "):
			raw := strings.TrimPrefix(line, "branch ")
			info.Branch = strings.TrimPrefix(raw, "refs/heads/")
		case line == "detached":
			info.Detached = true
		case line == "bare":
			info.Bare = true
		}
	}

	// A block with a worktree line but no HEAD is malformed.
	if hasWorktree && !hasHEAD {
		return WorktreeInfo{}, false, fmt.Errorf("porcelain: block for %q is missing HEAD line", info.Path)
	}

	// Blocks without a worktree header are skipped (they shouldn't appear in
	// well-formed output but we tolerate them gracefully).
	if !hasWorktree {
		return WorktreeInfo{}, true, nil
	}

	// Bare worktrees are always skipped.
	if info.Bare {
		return WorktreeInfo{}, true, nil
	}

	return info, false, nil
}
