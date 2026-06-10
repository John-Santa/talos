// Package gitfs is the native PlatformReader/Writer: it produces orchestration
// inputs and performs worktree write actions using only `git` and
// team-context/ownership.md — no wt/mo/ov/ch binaries, no network, no Jira.
// Merge-order and overlap are derived (commits-ahead via git rev-list;
// collisions via shared module ownership).
package gitfs

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/John-Santa/talos/platform/webapi/domain"
)

// gitRunner runs `git -C <dir> <args...>` and returns stdout. Injectable for tests.
type gitRunner func(ctx context.Context, dir string, args ...string) ([]byte, error)

// chRunner runs an external command and returns stdout. Injectable for tests.
type chRunner func(ctx context.Context, name string, args ...string) ([]byte, error)

func execGit(ctx context.Context, dir string, args ...string) ([]byte, error) {
	full := append([]string{"-C", dir}, args...)
	return exec.CommandContext(ctx, "git", full...).Output()
}

func execCh(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).Output()
}

type Reader struct {
	root  string
	base  string
	run   gitRunner
	chRun chRunner
}

// New builds a Reader for the given repo root (base defaults to "develop").
func New(root, base string) *Reader {
	if base == "" {
		base = "develop"
	}
	return &Reader{root: root, base: base, run: execGit, chRun: execCh}
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
	return r.run(ctx, r.root, args...)
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

type rawWorktree struct {
	Path   string
	Branch string
	Head   string
}

func (r *Reader) rawWorktrees(ctx context.Context) ([]rawWorktree, error) {
	out, err := r.git(ctx, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("git worktree list: %w", err)
	}
	var list []rawWorktree
	var cur rawWorktree
	started := false
	flush := func() {
		if started {
			list = append(list, cur)
		}
		cur = rawWorktree{}
		started = false
	}
	for _, line := range strings.Split(string(out), "\n") {
		switch {
		case strings.HasPrefix(line, "worktree "):
			flush()
			cur.Path = strings.TrimPrefix(line, "worktree ")
			started = true
		case strings.HasPrefix(line, "HEAD "):
			head := strings.TrimPrefix(line, "HEAD ")
			if len(head) > 7 {
				head = head[:7]
			}
			cur.Head = head
		case strings.HasPrefix(line, "branch "):
			cur.Branch = strings.TrimPrefix(strings.TrimPrefix(line, "branch "), "refs/heads/")
		}
	}
	flush()
	return list, nil
}

// Worktrees returns the agent worktrees (branches matching agent/<figura>/...).
func (r *Reader) Worktrees(ctx context.Context) ([]domain.WtEntry, error) {
	raws, err := r.rawWorktrees(ctx)
	if err != nil {
		return nil, err
	}
	entries := make([]domain.WtEntry, 0, len(raws))
	for _, w := range raws {
		fig, ok := figuraFromBranch(w.Branch)
		if !ok {
			continue
		}
		entries = append(entries, domain.WtEntry{
			Figura: fig,
			Branch: w.Branch,
			Path:   w.Path,
			Head:   w.Head,
			Status: "active",
		})
	}
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

// MergePlan derives the merge order from the worktrees (ahead via git rev-list).
// For each branch with commits ahead, mergeTreeConflict is called to compute
// a real conflict prediction. ConflictRate = C/N where N = steps with ahead>0,
// C = steps with conflicts. N=0 → rate 0.
func (r *Reader) MergePlan(ctx context.Context) (domain.MoPlan, error) {
	wts, err := r.Worktrees(ctx)
	if err != nil {
		return domain.MoPlan{}, err
	}
	steps := make([]domain.MoPlanStep, 0, len(wts))
	candidates, conflicting := 0, 0
	for i, w := range wts {
		ahead := r.aheadCount(ctx, w.Branch)
		var conflictFiles []string
		predictedClean := false
		if ahead > 0 {
			candidates++
			var clean bool
			conflictFiles, clean = r.mergeTreeConflict(ctx, r.base, w.Branch)
			if clean {
				predictedClean = true
			} else {
				conflicting++
			}
		}
		steps = append(steps, domain.MoPlanStep{
			Position:       i + 1,
			Branch:         w.Branch,
			Figura:         w.Figura,
			CommitsAhead:   ahead,
			PredictedClean: predictedClean,
			ConflictFiles:  conflictFiles,
		})
	}
	rate := 0.0
	if candidates > 0 {
		rate = float64(conflicting) / float64(candidates)
	}
	return domain.MoPlan{BaseBranch: r.base, ConflictRate: rate, Threshold: 0.15, Steps: steps}, nil
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

// --- write actions (native git) ---------------------------------------------

// CreateWorktree adds an isolated worktree+branch for a figura off the base.
func (r *Reader) CreateWorktree(ctx context.Context, figura, jiraKey string) error {
	branch := fmt.Sprintf("agent/%s/%s", figura, jiraKey)
	path := fmt.Sprintf("talos.wt/agent-%s", figura)
	if _, err := r.git(ctx, "worktree", "add", path, "-b", branch, r.base); err != nil {
		return fmt.Errorf("create worktree %s: %w", branch, err)
	}
	return nil
}

// TeardownWorktree removes the worktree owned by a figura.
func (r *Reader) TeardownWorktree(ctx context.Context, figura string) error {
	raws, err := r.rawWorktrees(ctx)
	if err != nil {
		return err
	}
	for _, w := range raws {
		if fig, ok := figuraFromBranch(w.Branch); ok && fig == figura {
			if _, err := r.git(ctx, "worktree", "remove", "--force", w.Path); err != nil {
				return fmt.Errorf("teardown %s: %w", figura, err)
			}
			return nil
		}
	}
	return fmt.Errorf("no worktree for figura %q", figura)
}

// mergeTreeConflict runs a non-destructive git merge-tree check for branch
// against r.base. Returns conflict file paths (if any) and whether the merge
// would be clean. Uses --name-only (git ≥ 2.38) to list conflicting files.
func (r *Reader) mergeTreeConflict(ctx context.Context, base, branch string) (conflictFiles []string, clean bool) {
	out, err := r.git(ctx, "merge-tree", "--write-tree", "--name-only", base, branch)
	if err != nil {
		// Non-zero exit = conflicts exist; parse stdout for file names.
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		// First line is the OID of the tree; skip it. Remaining non-empty lines
		// before the informational messages block are conflicting file paths.
		files := make([]string, 0)
		for i, line := range lines {
			if i == 0 {
				continue // skip tree OID
			}
			line = strings.TrimSpace(line)
			if line == "" {
				break // blank line separates sections
			}
			files = append(files, line)
		}
		return files, false
	}
	return nil, true
}

// Labels calls `ch labels --branch <branch> --json` with a short timeout and
// returns the parsed result. This is the ONLY method in this package that
// invokes an external binary other than git. It degrades gracefully: if ch is
// absent, exits non-zero, or returns malformed JSON the method returns an empty
// ChLabels without an error, so callers always get a best-effort result.
func (r *Reader) Labels(ctx context.Context, branch string) (domain.ChLabels, error) {
	tctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	out, err := r.chRun(tctx, "ch", "labels", "--branch", branch, "--json")
	if err != nil {
		// ch absent or failed — degrade cleanly, no error returned.
		return domain.ChLabels{}, nil
	}
	var cl domain.ChLabels
	if err := json.Unmarshal(out, &cl); err != nil {
		return domain.ChLabels{}, nil
	}
	return cl, nil
}

// Merge merges the worktree's branch into base, guarded by a non-destructive
// conflict check (git merge-tree). The branch is matched exactly as
// agent/<figura>/<jiraKey> to avoid collisions between figuras sharing a jiraKey.
func (r *Reader) Merge(ctx context.Context, figura, jiraKey string) error {
	raws, err := r.rawWorktrees(ctx)
	if err != nil {
		return err
	}
	target := fmt.Sprintf("agent/%s/%s", domain.NormalizeFigura(figura), jiraKey)
	branch := ""
	developPath := ""
	for _, w := range raws {
		if w.Branch == r.base {
			developPath = w.Path
		}
		if w.Branch == target {
			branch = w.Branch
		}
	}
	if branch == "" {
		return fmt.Errorf("no worktree for %q", target)
	}
	// Non-destructive conflict check reusing shared helper.
	if _, clean := r.mergeTreeConflict(ctx, r.base, branch); !clean {
		return fmt.Errorf("merge of %s into %s would conflict — aborted", branch, r.base)
	}
	if developPath == "" {
		return fmt.Errorf("%s is not checked out in any worktree", r.base)
	}
	if _, err := r.run(ctx, developPath, "merge", "--no-edit", branch); err != nil {
		return fmt.Errorf("merge %s into %s failed: %w", branch, r.base, err)
	}
	return nil
}
