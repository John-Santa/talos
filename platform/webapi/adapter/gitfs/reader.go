// Package gitfs is the native PlatformReader: it produces orchestration inputs
// from the local repo using only `git` and team-context/ownership.md — no
// wt/mo/ov/ch binaries, no network, no Jira. Merge-order and overlap are derived
// (commits-ahead via git rev-list; collisions via shared module ownership)
// rather than computed with mo/ov's full conflict-prediction engines.
package gitfs

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/John-Santa/talos/platform/webapi/domain"
)

type Reader struct {
	root string
	base string
}

// New builds a Reader for the given repo root (base defaults to "develop").
func New(root, base string) *Reader {
	if base == "" {
		base = "develop"
	}
	return &Reader{root: root, base: base}
}

// NewAutodetect resolves the repo root from the current working directory.
func NewAutodetect(base string) (*Reader, error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return nil, fmt.Errorf("detect repo root: %w", err)
	}
	return New(strings.TrimSpace(string(out)), base), nil
}

func (r *Reader) git(ctx context.Context, args ...string) ([]byte, error) {
	full := append([]string{"-C", r.root}, args...)
	return exec.CommandContext(ctx, "git", full...).Output()
}

// Ready verifies the root is a usable git repository.
func (r *Reader) Ready(ctx context.Context) error {
	if _, err := r.git(ctx, "rev-parse", "--git-dir"); err != nil {
		return fmt.Errorf("git repo not ready at %q: %w", r.root, err)
	}
	return nil
}

func figuraFromBranch(branch string) (string, bool) {
	parts := strings.Split(branch, "/")
	if len(parts) >= 3 && parts[0] == "agent" {
		return parts[1], true
	}
	return "", false
}

// Worktrees parses `git worktree list --porcelain`, keeping only agent worktrees.
func (r *Reader) Worktrees(ctx context.Context) ([]domain.WtEntry, error) {
	out, err := r.git(ctx, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("git worktree list: %w", err)
	}
	var entries []domain.WtEntry
	var cur domain.WtEntry
	keep := false
	flush := func() {
		if keep {
			entries = append(entries, cur)
		}
		cur = domain.WtEntry{}
		keep = false
	}
	for _, line := range strings.Split(string(out), "\n") {
		switch {
		case strings.HasPrefix(line, "worktree "):
			flush()
			cur.Path = strings.TrimPrefix(line, "worktree ")
		case strings.HasPrefix(line, "HEAD "):
			head := strings.TrimPrefix(line, "HEAD ")
			if len(head) > 7 {
				head = head[:7]
			}
			cur.Head = head
		case strings.HasPrefix(line, "branch "):
			branch := strings.TrimPrefix(strings.TrimPrefix(line, "branch "), "refs/heads/")
			if fig, ok := figuraFromBranch(branch); ok {
				cur.Branch = branch
				cur.Figura = fig
				cur.Status = "active"
				keep = true
			}
		}
	}
	flush()
	return entries, nil
}

func cleanCell(s string) string {
	return strings.Trim(strings.TrimSpace(s), "`")
}

func parseOwnership(md string) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(md, "\n") {
		if !strings.Contains(line, "|") {
			continue
		}
		cells := strings.Split(line, "|")
		if len(cells) < 3 {
			continue
		}
		module := cleanCell(cells[1])
		agent := domain.NormalizeFigura(cleanCell(cells[2]))
		if !strings.HasPrefix(module, "module:") || strings.Contains(module, "*") {
			continue
		}
		if _, ok := domain.AgentByID(agent); ok {
			out[module] = agent
		}
	}
	return out
}

// Ownership reads and parses team-context/ownership.md (module -> agent).
func (r *Reader) Ownership(_ context.Context) (map[string]string, error) {
	path := filepath.Join(r.root, "team-context", "ownership.md")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read ownership: %w", err)
	}
	return parseOwnership(string(data)), nil
}

func (r *Reader) aheadCount(ctx context.Context, branch string) int {
	out, err := r.git(ctx, "rev-list", "--count", r.base+".."+branch)
	if err != nil {
		return 0
	}
	n, _ := strconv.Atoi(strings.TrimSpace(string(out)))
	return n
}

// MergePlan derives the merge order from the worktrees (ahead via git rev-list;
// "ready" = ahead of base). Conflict prediction is not computed offline.
func (r *Reader) MergePlan(ctx context.Context) (domain.MoPlan, error) {
	wts, err := r.Worktrees(ctx)
	if err != nil {
		return domain.MoPlan{}, err
	}
	steps := make([]domain.MoPlanStep, 0, len(wts))
	for i, w := range wts {
		ahead := r.aheadCount(ctx, w.Branch)
		steps = append(steps, domain.MoPlanStep{
			Position:       i + 1,
			Branch:         w.Branch,
			Figura:         w.Figura,
			CommitsAhead:   ahead,
			PredictedClean: ahead > 0,
		})
	}
	return domain.MoPlan{BaseBranch: r.base, ConflictRate: 0, Threshold: 0.15, Steps: steps}, nil
}

// Overlap derives collisions: two active worktrees collide when they own the
// same module (per ownership.md).
func (r *Reader) Overlap(ctx context.Context) (domain.OvScan, error) {
	wts, err := r.Worktrees(ctx)
	if err != nil {
		return domain.OvScan{}, err
	}
	ownership, _ := r.Ownership(ctx)
	agentModule := make(map[string]string, len(ownership))
	for module, agent := range ownership {
		agentModule[agent] = strings.TrimPrefix(module, "module:")
	}

	agents := make([]string, 0, len(wts))
	for _, w := range wts {
		agents = append(agents, w.Figura)
	}

	total, colliding := 0, 0
	for i := 0; i < len(agents); i++ {
		for j := i + 1; j < len(agents); j++ {
			total++
			mi, mj := agentModule[agents[i]], agentModule[agents[j]]
			if mi != "" && mi == mj {
				colliding++
			}
		}
	}
	rate := 0.0
	if total > 0 {
		rate = float64(colliding) / float64(total)
	}
	verdict := "ok"
	if colliding > 0 {
		verdict = "conflict"
	}
	return domain.OvScan{
		Verdict:        verdict,
		CollisionRate:  rate,
		PairsEvaluated: total,
		CollidingPairs: colliding,
	}, nil
}
