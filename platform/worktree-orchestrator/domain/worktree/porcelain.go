package worktree

import (
	"fmt"
	"strings"
)

// WorktreeInfo holds the parsed fields from a single `git worktree list --porcelain` block.
type WorktreeInfo struct {
	Path     string
	Head     string
	Branch   string
	Detached bool
	Bare     bool
}

// ParseWorktreeList parses `git worktree list --porcelain` output and returns only agent/* entries.
func ParseWorktreeList(stdout string) ([]WorktreeInfo, error) {
	if strings.ContainsRune(stdout, 0) {
		return nil, fmt.Errorf("porcelain: input contains null bytes (malformed)")
	}

	var result []WorktreeInfo

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

	if hasWorktree && !hasHEAD {
		return WorktreeInfo{}, false, fmt.Errorf("porcelain: block for %q is missing HEAD line", info.Path)
	}

	if !hasWorktree {
		return WorktreeInfo{}, true, nil
	}

	if info.Bare {
		return WorktreeInfo{}, true, nil
	}

	return info, false, nil
}
