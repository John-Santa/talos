// Package repoctx provides pure helpers for resolving the current repository
// identity from git metadata (remote URL + toplevel path).
package repoctx

import (
	"path/filepath"
	"strings"
)

// ParseRepoSlug derives a human-readable "owner/repo" slug from the git remote
// URL and the repo's toplevel directory path.
//
// Supported URL forms:
//
//	git@github.com:John-Santa/talos.git  → "John-Santa/talos"
//	https://github.com/John-Santa/talos.git → "John-Santa/talos"
//	https://github.com/John-Santa/talos     → "John-Santa/talos"
//
// If remoteURL is empty or cannot be parsed into an "owner/repo" pair, the
// function falls back to filepath.Base(toplevel). If toplevel is also empty,
// an empty string is returned.
func ParseRepoSlug(remoteURL, toplevel string) string {
	if slug := parseSlugFromURL(remoteURL); slug != "" {
		return slug
	}
	// Fallback: basename of the toplevel directory.
	if toplevel == "" {
		return ""
	}
	return filepath.Base(toplevel)
}

// parseSlugFromURL attempts to extract "owner/repo" from a git remote URL.
// Returns "" if the URL is empty or not in a recognisable form.
func parseSlugFromURL(u string) string {
	if u == "" {
		return ""
	}

	var path string

	switch {
	// SSH form: git@github.com:owner/repo.git
	case strings.HasPrefix(u, "git@"):
		// Find the colon that separates host from path.
		idx := strings.Index(u, ":")
		if idx < 0 {
			return ""
		}
		path = u[idx+1:]

	// HTTPS form: https://host/owner/repo[.git]
	case strings.HasPrefix(u, "https://") || strings.HasPrefix(u, "http://"):
		// Strip the scheme.
		rest := u[strings.Index(u, "//")+2:]
		// Strip the host (everything up to the first slash).
		slashIdx := strings.Index(rest, "/")
		if slashIdx < 0 {
			return ""
		}
		path = rest[slashIdx+1:]

	default:
		return ""
	}

	// Strip trailing .git if present.
	path = strings.TrimSuffix(path, ".git")

	// Validate: we expect exactly "owner/repo" (two non-empty parts).
	parts := strings.SplitN(path, "/", 3)
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return ""
	}

	return parts[0] + "/" + parts[1]
}
