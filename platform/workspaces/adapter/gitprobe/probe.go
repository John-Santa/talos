// Package gitprobe implements RepoProbe using the git CLI.
package gitprobe

import (
	"os"
	"os/exec"
	"path/filepath"

	"github.com/John-Santa/talos/platform/workspaces/domain/workspace"
)

// Probe implements port.RepoProbe via the git CLI.
type Probe struct{}

// New constructs a Probe.
func New() *Probe { return &Probe{} }

// Resolve returns the absolute, symlink-cleaned path.
// Returns ErrRepoNotFound if the path does not exist.
func (p *Probe) Resolve(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", &workspace.ErrRepoNotFound{Path: path}
	}
	abs, err = filepath.EvalSymlinks(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return "", &workspace.ErrRepoNotFound{Path: path}
		}
		return "", &workspace.ErrRepoNotFound{Path: path}
	}
	return abs, nil
}

// IsGitRepo runs `git -C <path> rev-parse --is-inside-work-tree` and returns
// true if git exits 0, false otherwise. Never returns an error for a plain
// non-git directory; only returns an error on unexpected failures.
func (p *Probe) IsGitRepo(path string) (bool, error) {
	cmd := exec.Command("git", "-C", path, "rev-parse", "--is-inside-work-tree")
	cmd.Stdout = nil
	cmd.Stderr = nil
	err := cmd.Run()
	if err != nil {
		// git exits non-zero when not in a work tree — that's not an error.
		if _, ok := err.(*exec.ExitError); ok {
			return false, nil
		}
		// genuine execution failure (e.g., git not in PATH)
		return false, err
	}
	return true, nil
}
