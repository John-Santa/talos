// Package tomlstore implements WorkspaceStore using a TOML file in TALOS_HOME.
//
// # TOML dependency decision
//
// This package uses a hand-written TOML encoder/decoder restricted to the exact
// schema of workspaces.toml. We chose this over adding BurntSushi/toml because:
//   - The module currently has zero external dependencies.
//   - The schema is small and stable: one [active] table + repeated [[workspace]] arrays.
//   - Avoiding CGO and network resolution simplifies CI.
//   - The port.WorkspaceStore interface isolates callers; if requirements grow
//     (nested tables, inline tables, datetime types) we can swap to a full TOML library
//     by changing only this adapter without touching domain or service.
//
// Schema (version = 1):
//
//	version = 1
//
//	[active]
//	name = "my-project"
//
//	[[workspace]]
//	name       = "my-project"
//	repo_path  = "/abs/my-project"
//	cred_ref   = "my-project"
//
//	[workspace.jira]
//	site_url        = "https://..."
//	project_key     = "EX"
//	project_id      = "10001"
//	issue_type_name = "Story"
//	state_new           = ["To Do"]
//	state_indeterminate = ["In Progress"]
//	state_done          = ["Done"]
package tomlstore

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/John-Santa/talos/platform/workspaces/domain/workspace"
)

const (
	schemaVersion = 1
	fileName      = "workspaces.toml"
)

// Store implements port.WorkspaceStore on top of a TOML file.
type Store struct {
	path string // full path to workspaces.toml
}

// New creates a Store rooted at talosHome.
// The file is created on first write; reads on a missing file return empty state.
func New(talosHome string) (*Store, error) {
	if err := os.MkdirAll(talosHome, 0700); err != nil {
		return nil, fmt.Errorf("creating talos home %q: %w", talosHome, err)
	}
	return &Store{path: filepath.Join(talosHome, fileName)}, nil
}

// ---- port.WorkspaceStore implementation ----

func (s *Store) List(_ context.Context) ([]workspace.Workspace, error) {
	st, err := s.load()
	if err != nil {
		return nil, err
	}
	return st.Workspaces, nil
}

func (s *Store) Get(_ context.Context, name string) (workspace.Workspace, error) {
	st, err := s.load()
	if err != nil {
		return workspace.Workspace{}, err
	}
	for _, ws := range st.Workspaces {
		if ws.Name == name {
			return ws, nil
		}
	}
	return workspace.Workspace{}, &workspace.ErrNotFound{Name: name}
}

func (s *Store) Add(_ context.Context, ws workspace.Workspace) error {
	st, err := s.load()
	if err != nil {
		return err
	}
	for _, existing := range st.Workspaces {
		if existing.Name == ws.Name {
			return &workspace.ErrDuplicate{Name: ws.Name}
		}
	}
	st.Workspaces = append(st.Workspaces, ws)
	return s.save(st)
}

func (s *Store) Remove(_ context.Context, name string) error {
	st, err := s.load()
	if err != nil {
		return err
	}
	found := false
	filtered := st.Workspaces[:0]
	for _, ws := range st.Workspaces {
		if ws.Name == name {
			found = true
			continue
		}
		filtered = append(filtered, ws)
	}
	if !found {
		return &workspace.ErrNotFound{Name: name}
	}
	st.Workspaces = filtered
	if st.ActiveName == name {
		st.ActiveName = ""
	}
	return s.save(st)
}

func (s *Store) SetActive(_ context.Context, name string) error {
	st, err := s.load()
	if err != nil {
		return err
	}
	found := false
	for _, ws := range st.Workspaces {
		if ws.Name == name {
			found = true
			break
		}
	}
	if !found {
		return &workspace.ErrNotFound{Name: name}
	}
	st.ActiveName = name
	return s.save(st)
}

func (s *Store) Active(_ context.Context) (string, error) {
	st, err := s.load()
	if err != nil {
		return "", err
	}
	return st.ActiveName, nil
}

// ---- internal state ----

type state struct {
	ActiveName string
	Workspaces []workspace.Workspace
}

// load reads and parses the TOML file. Returns empty state if file does not exist.
func (s *Store) load() (state, error) {
	b, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return state{}, nil
		}
		return state{}, fmt.Errorf("reading %q: %w", s.path, err)
	}
	return parse(b)
}

// save serialises state and writes it atomically (temp file + rename).
func (s *Store) save(st state) error {
	b := render(st)
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, fs.FileMode(0600)); err != nil {
		return fmt.Errorf("writing %q: %w", tmp, err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("renaming %q -> %q: %w", tmp, s.path, err)
	}
	return nil
}

// ---- minimal TOML encoder ----

func render(st state) []byte {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "version = %d\n\n", schemaVersion)
	fmt.Fprintf(&buf, "[active]\nname = %q\n\n", st.ActiveName)
	for _, ws := range st.Workspaces {
		fmt.Fprintf(&buf, "[[workspace]]\n")
		fmt.Fprintf(&buf, "name      = %q\n", ws.Name)
		fmt.Fprintf(&buf, "repo_path = %q\n", ws.RepoPath)
		fmt.Fprintf(&buf, "cred_ref  = %q\n", ws.CredRef)
		fmt.Fprintf(&buf, "[workspace.jira]\n")
		fmt.Fprintf(&buf, "site_url        = %q\n", ws.Jira.SiteURL)
		fmt.Fprintf(&buf, "project_key     = %q\n", ws.Jira.ProjectKey)
		fmt.Fprintf(&buf, "project_id      = %q\n", ws.Jira.ProjectID)
		fmt.Fprintf(&buf, "issue_type_name = %q\n", ws.Jira.IssueTypeName)
		fmt.Fprintf(&buf, "state_new           = %s\n", renderStringSlice(ws.Jira.StateMapping[workspace.CategoryNew]))
		fmt.Fprintf(&buf, "state_indeterminate = %s\n", renderStringSlice(ws.Jira.StateMapping[workspace.CategoryIndeterminate]))
		fmt.Fprintf(&buf, "state_done          = %s\n", renderStringSlice(ws.Jira.StateMapping[workspace.CategoryDone]))
		fmt.Fprintf(&buf, "\n")
	}
	return buf.Bytes()
}

func renderStringSlice(ss []string) string {
	if len(ss) == 0 {
		return "[]"
	}
	var parts []string
	for _, s := range ss {
		parts = append(parts, fmt.Sprintf("%q", s))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// ---- minimal TOML decoder ----

// parse decodes the TOML schema into state. It handles only the exact
// schema written by render() above — no general-purpose TOML is needed.
func parse(b []byte) (state, error) {
	var st state
	scanner := bufio.NewScanner(bytes.NewReader(b))

	type section int
	const (
		secTop section = iota
		secActive
		secWorkspace
		secWorkspaceJira
	)

	cur := secTop
	var curWS *workspace.Workspace

	flush := func() {
		if curWS != nil {
			if curWS.Jira.StateMapping == nil {
				curWS.Jira.StateMapping = make(map[workspace.StatusCategory][]string)
			}
			st.Workspaces = append(st.Workspaces, *curWS)
			curWS = nil
		}
	}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// skip blank lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// section headers
		switch {
		case line == "[active]":
			flush()
			cur = secActive
			continue
		case line == "[[workspace]]":
			flush()
			curWS = &workspace.Workspace{
				Jira: workspace.JiraBinding{
					StateMapping: make(map[workspace.StatusCategory][]string),
				},
			}
			cur = secWorkspace
			continue
		case line == "[workspace.jira]":
			cur = secWorkspaceJira
			continue
		case strings.HasPrefix(line, "["):
			// unknown section — skip
			cur = secTop
			continue
		}

		// key = value
		k, v, ok := splitKV(line)
		if !ok {
			continue
		}

		switch cur {
		case secActive:
			if k == "name" {
				st.ActiveName = unquote(v)
			}
		case secWorkspace:
			if curWS == nil {
				break
			}
			switch k {
			case "name":
				curWS.Name = unquote(v)
			case "repo_path":
				curWS.RepoPath = unquote(v)
			case "cred_ref":
				curWS.CredRef = unquote(v)
			}
		case secWorkspaceJira:
			if curWS == nil {
				break
			}
			switch k {
			case "site_url":
				curWS.Jira.SiteURL = unquote(v)
			case "project_key":
				curWS.Jira.ProjectKey = unquote(v)
			case "project_id":
				curWS.Jira.ProjectID = unquote(v)
			case "issue_type_name":
				curWS.Jira.IssueTypeName = unquote(v)
			case "state_new":
				curWS.Jira.StateMapping[workspace.CategoryNew] = parseStringSlice(v)
			case "state_indeterminate":
				curWS.Jira.StateMapping[workspace.CategoryIndeterminate] = parseStringSlice(v)
			case "state_done":
				curWS.Jira.StateMapping[workspace.CategoryDone] = parseStringSlice(v)
			}
		}
	}
	flush()

	if err := scanner.Err(); err != nil {
		return state{}, fmt.Errorf("scanning TOML: %w", err)
	}
	return st, nil
}

// splitKV splits "key = value" into (key, value, true). Whitespace around '=' is trimmed.
func splitKV(line string) (k, v string, ok bool) {
	idx := strings.IndexByte(line, '=')
	if idx < 0 {
		return "", "", false
	}
	return strings.TrimSpace(line[:idx]), strings.TrimSpace(line[idx+1:]), true
}

// unquote removes surrounding double-quotes from a TOML string value.
func unquote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

// parseStringSlice decodes a TOML inline array like ["a", "b", "c"].
func parseStringSlice(s string) []string {
	s = strings.TrimSpace(s)
	if s == "[]" {
		return nil
	}
	s = strings.TrimPrefix(s, "[")
	s = strings.TrimSuffix(s, "]")
	var result []string
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		result = append(result, unquote(part))
	}
	return result
}
