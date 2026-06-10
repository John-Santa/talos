// Package jsonlstore implements RunStore using an append-only JSONL file.
//
// # Design decision
//
// Events are stored as one JSON object per line in ~/.talos/runs/runs.jsonl.
// Append is O(1) (O_APPEND write, no re-serialization).
// Query is a linear scan with in-memory filtering.
// A corrupt line (partial write after crash) is silently discarded; the rest of the
// file is unaffected.
// The port.RunStore interface isolates this adapter; migrating to SQLite in cut 2
// requires only replacing this package.
//
// Schema: each line is a RunEvent with V=1.
// Large-line safety: bufio.Scanner buffer is set to 1 MiB to handle events
// with long Judges/Violations slices.
package jsonlstore

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/John-Santa/talos/platform/runs/domain/run"
)

const (
	runsDir  = "runs"
	fileName = "runs.jsonl"
	// scanBuf is the scanner buffer size. Default (64 KiB) is too small for
	// events with large slice fields; 1 MiB is generous.
	scanBuf = 1 << 20 // 1 MiB
)

// Store implements port.RunStore on top of an append-only JSONL file.
type Store struct {
	path string // full path to runs.jsonl
}

// New creates a Store rooted at talosHome.
// The directory and file are created on first write; reads on a missing file return empty results.
func New(talosHome string) (*Store, error) {
	dir := filepath.Join(talosHome, runsDir)
	if err := os.MkdirAll(dir, fs.FileMode(0700)); err != nil {
		return nil, fmt.Errorf("creating runs dir %q: %w", dir, err)
	}
	return &Store{path: filepath.Join(dir, fileName)}, nil
}

// Append appends a single RunEvent as a JSON line. O(1).
func (s *Store) Append(_ context.Context, e run.RunEvent) error {
	f, err := os.OpenFile(s.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, fs.FileMode(0600))
	if err != nil {
		return fmt.Errorf("opening runs log %q: %w", s.path, err)
	}
	defer f.Close()

	line, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("encoding run event: %w", err)
	}
	line = append(line, '\n')

	if _, err := f.Write(line); err != nil {
		return fmt.Errorf("writing run event: %w", err)
	}
	return nil
}

// Query scans the JSONL file and returns events matching the filter.
// Corrupt lines (invalid JSON) are silently discarded.
func (s *Store) Query(_ context.Context, f run.Filter) ([]run.RunEvent, error) {
	file, err := os.Open(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return []run.RunEvent{}, nil
		}
		return nil, fmt.Errorf("opening runs log %q: %w", s.path, err)
	}
	defer file.Close()

	sc := bufio.NewScanner(file)
	buf := make([]byte, scanBuf)
	sc.Buffer(buf, scanBuf)

	var result []run.RunEvent
	for sc.Scan() {
		line := sc.Bytes()
		var e run.RunEvent
		if err := json.Unmarshal(line, &e); err != nil {
			// corrupt line — discard silently
			continue
		}
		if !matchesFilter(e, f) {
			continue
		}
		result = append(result, e)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("scanning runs log: %w", err)
	}
	return result, nil
}

// matchesFilter returns true when event e satisfies every non-zero criterion in f.
func matchesFilter(e run.RunEvent, f run.Filter) bool {
	if f.JiraKey != "" && e.JiraKey != f.JiraKey {
		return false
	}
	if f.Agent != "" && e.Agent != f.Agent {
		return false
	}
	if f.Kind != "" && e.Kind != f.Kind {
		return false
	}
	if !f.Since.IsZero() && e.At.Before(f.Since) {
		return false
	}
	return true
}
