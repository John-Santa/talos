package cichecks_test

import (
	"errors"
	"testing"

	"github.com/John-Santa/talos/platform/ci-checks/domain/cichecks"
)

const ownershipFixture = `# Blackboard — Ownership

## Mapa módulo → agente

| ` + "`module:*`" + ` | Dueño (` + "`agent:*`" + `) | Dominio | Estado |
|---|---|---|---|
| ` + "`module:devops`" + ` | HERMES | CI / release | active |
| ` + "`module:jira-loop`" + ` | HERMES | Jira lifecycle | active |
| ` + "`module:qa`" + ` | THEMIS | tests | active |
| ` + "`module:frontend`" + ` | IRIS | UI | slot |

## Mapa archivo → módulo (compartidos)

| Archivo / path | Dueño | Nota |
|---|---|---|
| ` + "`platform/ci-checks/**`" + ` | HERMES | CLI ch |
`

const ownershipMixedCasing = `## Mapa módulo → agente

| ` + "`module:*`" + ` | Dueño |
|---|---|
| ` + "`module:Qa`" + ` | Themis |
| ` + "`module:devops`" + ` | HERMES |
`

const ownershipDuplicate = `## Mapa módulo → agente

| ` + "`module:*`" + ` | Dueño |
|---|---|
| ` + "`module:devops`" + ` | HERMES |
| ` + "`module:devops`" + ` | ATLAS |
`

const ownershipHeaderOnly = `## Mapa módulo → agente

| ` + "`module:*`" + ` | Dueño |
|---|---|
`

func TestParseOwnershipTable_OK(t *testing.T) {
	m, err := cichecks.ParseOwnershipTable(ownershipFixture)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := map[string]string{
		"module:devops":    "hermes",
		"module:jira-loop": "hermes",
		"module:qa":        "themis",
		"module:frontend":  "iris",
	}
	for k, v := range want {
		got, ok := m[k]
		if !ok {
			t.Errorf("key %q missing from result", k)
			continue
		}
		if got != v {
			t.Errorf("m[%q] = %q, want %q", k, got, v)
		}
	}
}

func TestParseOwnershipTable_SkipsHeader(t *testing.T) {
	m, err := cichecks.ParseOwnershipTable(ownershipFixture)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := m["module:*"]; ok {
		t.Error("header row 'module:*' must be skipped but was included")
	}
}

func TestParseOwnershipTable_SkipsSeparator(t *testing.T) {
	m, err := cichecks.ParseOwnershipTable(ownershipFixture)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for k := range m {
		if k == "---|" || k == "---" {
			t.Errorf("separator row included as key: %q", k)
		}
	}
}

func TestParseOwnershipTable_SkipsSecondTable(t *testing.T) {
	m, err := cichecks.ParseOwnershipTable(ownershipFixture)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := m["platform/ci-checks/**"]; ok {
		t.Error("second-table entry must be skipped but was included")
	}
}

func TestParseOwnershipTable_DuplicateModule(t *testing.T) {
	_, err := cichecks.ParseOwnershipTable(ownershipDuplicate)
	if err == nil {
		t.Fatal("expected ErrMalformedOwnership for duplicate, got nil")
	}
	var e *cichecks.ErrMalformedOwnership
	if !errors.As(err, &e) {
		t.Fatalf("error type = %T, want *ErrMalformedOwnership", err)
	}
	if e.Detail == "" {
		t.Error("Detail must name the duplicate module")
	}
}

func TestParseOwnershipTable_EmptyResult(t *testing.T) {
	_, err := cichecks.ParseOwnershipTable(ownershipHeaderOnly)
	if err == nil {
		t.Fatal("expected ErrMalformedOwnership for empty table, got nil")
	}
	var e *cichecks.ErrMalformedOwnership
	if !errors.As(err, &e) {
		t.Fatalf("error type = %T, want *ErrMalformedOwnership", err)
	}
}

func TestParseOwnershipTable_MixedCasing(t *testing.T) {
	m, err := cichecks.ParseOwnershipTable(ownershipMixedCasing)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := m["module:qa"]
	if !ok {
		t.Fatal("expected key 'module:qa' in result")
	}
	if got != "themis" {
		t.Errorf("m[module:qa] = %q, want %q", got, "themis")
	}
	got2, ok2 := m["module:devops"]
	if !ok2 {
		t.Fatal("expected key 'module:devops' in result")
	}
	if got2 != "hermes" {
		t.Errorf("m[module:devops] = %q, want %q", got2, "hermes")
	}
}
