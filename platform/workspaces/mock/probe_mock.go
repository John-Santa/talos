package mock

import "github.com/John-Santa/talos/platform/workspaces/domain/workspace"

// RepoProbe interface mirrors port.RepoProbe to avoid import cycle.
// ProbeMock is a hand-written test double for port.RepoProbe.
type ProbeMock struct {
	Calls      []Call
	AbsPath    string
	IsGit      bool
	ResolveErr error
	IsGitErr   error
}

func NewProbeMock(absPath string, isGit bool) *ProbeMock {
	return &ProbeMock{AbsPath: absPath, IsGit: isGit}
}

func (m *ProbeMock) Resolve(path string) (string, error) {
	m.Calls = append(m.Calls, Call{Method: "Resolve", Args: []any{path}})
	if m.ResolveErr != nil {
		return "", m.ResolveErr
	}
	if m.AbsPath != "" {
		return m.AbsPath, nil
	}
	return path, nil
}

func (m *ProbeMock) IsGitRepo(path string) (bool, error) {
	m.Calls = append(m.Calls, Call{Method: "IsGitRepo", Args: []any{path}})
	return m.IsGit, m.IsGitErr
}

// NotFoundProbe always returns ErrRepoNotFound.
type NotFoundProbe struct {
	Path string
}

func (p *NotFoundProbe) Resolve(_ string) (string, error) {
	return "", &workspace.ErrRepoNotFound{Path: p.Path}
}

func (p *NotFoundProbe) IsGitRepo(_ string) (bool, error) {
	return false, nil
}
