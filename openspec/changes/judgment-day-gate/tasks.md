# Tasks: judgment-day-gate

**Change:** judgment-day-gate · **Jira:** TAL-A (por crear) · **Module:** module:devops · **Owner:** HERMES
**Branch:** `agent/hermes/TAL-A` (A1) → stacked A2 → independent A3
**Store:** hybrid (`openspec/changes/judgment-day-gate/tasks.md` + engram `sdd/judgment-day-gate/tasks`)
**TDD mode:** Strict — RED before GREEN, L1 domain → L2 cmd → L3 CI.
**Status: A1 DONE — A2, A3 PENDING**

---

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~520–650 LOC |
| 400-line budget risk | High (A1 alone ~300–380; A2 ~100–130; A3 ~70–100) |
| Chained PRs recommended | Yes |
| Suggested split | A1 → A2 (stacked) → A3 (independent) |
| Delivery strategy | auto-chain |
| Chain strategy | stacked-to-main |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: High

### Suggested Work Units

| Unit | Goal | Likely PR | Notes |
|------|------|-----------|-------|
| A1 | domain `judgment.go` + `errors.go` + cmd `cmdJudgment` | PR A1 | base: `develop`; all tests included |
| A2 | CI job `judgment` in `pr-checks.yml` + `rules.archive` block in `config.yaml` + dog-food bootstrap | PR A2 | base: PR A1 branch (stacked); gate self-proves on A2 |
| A3 | `team-context/judgment-day.md` + ownership row (ATHENA-shared-file) | PR A3 | independent; base: `develop`; no code; ZEUS sign-off on O-4 label |

---

## Execution Summary

25 tasks across 3 PRs — A1 (17 tasks, platform/ci-checks/**) → A2 (5 tasks, CI + config, stacked on A1) → A3 (3 tasks, governance doc, independent).

| PR | Tasks | Status |
|----|-------|--------|
| A1 | 17 | DONE |
| A2 | 5 | PENDING |
| A3 | 3 | PENDING |
| **Total** | **25** | **8/25 DONE** |

### Strict TDD Layer Order

```
L1 domain (pure strings, zero I/O) → L2 cmd (I/O boundary, temp dir fixture) → L3 CI (self-gated, D-5)
```

Within each layer: `_test.go` (RED) written first → implementation (GREEN) → refactor.

---

## Key Decisions Locked by Design

1. **D-1 Validator lives in `ch`:** `platform/ci-checks/` is the gate engine; `judgment` is another gate of the same module. No new binary.
2. **D-2 I/O boundary:** `cmd` does `os.ReadFile`; domain is pure on `string`. No fs port — mirrors `ParseOwnershipTable` approach.
3. **D-3 O-1 surface-only:** `judges≠2` or `implementor∈judges` → append to `Violations[]`, NOT a hard error in v1. Verdict `APPROVED` is the only gate.
4. **D-4 Implementor commits `judgment-report.md`:** ARGOS owns no module; implementor attaches to `agent/<figura>/TAL-A` branch.
5. **D-5 Dog-food bootstrap (A2):** The introducing PR A2 ships its own APPROVED `openspec/changes/judgment-day-gate/judgment-report.md` committed on the implementor branch before opening the PR. Gate proves itself on first real run — no "first PR exempt" hole.
6. **D-6 ESCALATED:** parses fine; `ApprovedFor` returns `ErrJudgmentNotApproved` → exit 1. NOT a parse error.
7. **O-4 resolved:** `team-context/judgment-day.md` is a shared-governance file maintained by ATHENA. Label for its issue: `module:devops` (HERMES is owner of `platform/ci-checks/**` and the gate tooling). Add a `team-context/judgment-day.md` row to the second table of `ownership.md` under HERMES + note "governance doc; shared with ATHENA".

---

## PR A1 — domain + errors + cmd (platform/ci-checks/**)

> **Base branch:** `develop`
> **Files:** `domain/cichecks/errors.go` (modify), `domain/cichecks/judgment_test.go` (create RED), `domain/cichecks/judgment.go` (create GREEN), `cmd/ch/main_test.go` (modify RED), `cmd/ch/main.go` (modify GREEN)
> **REQ-IDs:** REQ-ARTIFACT-1..5, REQ-VALIDATOR-1..10, REQ-ERR-1..2, REQ-TEST-1..4

### L1 — Domain (RED → GREEN)

- [x] **A1-1 — `errors.go` (modify, L1)** · `platform/ci-checks/domain/cichecks/errors.go`
  Add `ErrNoJudgmentReport` (var sentinel), `ErrJudgmentNotApproved` struct `{Change, Verdict string}` with `.Error()`, and `ErrMalformedJudgment` struct `{Detail string}` with `.Error()`. Match existing style (`ErrIssueNotFound` pattern). No behavior test — compile check only.
  _REQ-ERR-1_

- [x] **A1-2 — `judgment_test.go` (RED)** · `platform/ci-checks/domain/cichecks/judgment_test.go`
  Write full table-driven tests for `ParseJudgmentReport` + `ApprovedFor` BEFORE `judgment.go` exists. Confirm `go test ./domain/cichecks/...` FAILS (RED). Cases (REQ-TEST-1):
  - `APPROVED no emoji` → nil error, `Verdict=="APPROVED"`
  - `APPROVED with ✅` → nil error (emoji stripped)
  - `ESCALATED no emoji` → `ErrJudgmentNotApproved`
  - `ESCALATED with ⚠️` → `ErrJudgmentNotApproved`
  - `no terminal line` → `ErrJudgmentNotApproved`
  - `Round: 0` → `ErrJudgmentNotApproved`
  - `change-mismatch` → `ErrJudgmentNotApproved` (via `ApprovedFor`)
  - `missing Judges field` → `ErrJudgmentNotApproved`
  - `judges≠2` → `Violations[]` populated, NO error (O-1 surface-only)
  - `implementor∈judges` → `Violations[]` populated, NO error
  All tests must NOT touch the filesystem or require I/O.
  _REQ-TEST-1, REQ-TEST-3, REQ-VALIDATOR-10_

- [x] **A1-3 — `judgment.go` (GREEN)** · `platform/ci-checks/domain/cichecks/judgment.go`
  Implement `JudgmentReport` struct (`Change, Implementor, Verdict, RawVerdictLine string; Round int; Judges, Violations []string`), `ParseJudgmentReport(md string) (JudgmentReport, error)`, and `(r JudgmentReport) ApprovedFor(change string) error`.
  Parse rules (line-scan via `strings.Split(md, "\n")`, mirroring `ParseOwnershipTable`):
  - Header fields: `**Field:** value` prefix → populate `Change`, `Round`, `Judges` (comma-split), `Implementor`, skip `Date` (parsed for presence only).
  - Terminal line: `strings.HasPrefix(trimmed, "JUDGMENT:")` → strip trailing ` ✅`/` ⚠️`; Verdict ∈ `{APPROVED, ESCALATED}` else `ErrMalformedJudgment`.
  - Missing terminal or empty `Change` → `ErrMalformedJudgment`.
  - O-1: `len(Judges)!=2` or `Implementor∈Judges` → append to `Violations`, NOT an error.
  `ApprovedFor(change)`: nil iff `Verdict==APPROVED && Change==change`; else `ErrJudgmentNotApproved{Change, Verdict}`.
  Confirm `go test ./domain/cichecks/...` PASSES (GREEN).
  _REQ-ARTIFACT-2..5, REQ-VALIDATOR-2..10, REQ-ERR-1_

### L2 — Cmd (RED → GREEN)

- [x] **A1-4 — `main_test.go` (modify, RED)** · `platform/ci-checks/cmd/ch/main_test.go`
  Extend existing test harness with `judgment` subcommand cases (inject temp dir fixture — write `judgment-report.md` in temp dir and pass `--changes-dir`). Cases (REQ-TEST-2):
  - `APPROVED → exit 0`
  - `ESCALATED → exit 1`
  - `missing file → exit 1` (error `ErrNoJudgmentReport`)
  - `no terminal line → exit 1`
  - `Round: 0 → exit 1`
  - `change-mismatch → exit 1`
  - `trailing ✅ tolerado → exit 0`
  - `--json shape: APPROVED` → JSON fields `{change, round, judges, implementor, verdict:"APPROVED", violations:[]}`
  - `--json shape: MISSING` → `verdict:"MISSING"` field present
  Extend `TestExitCodeFor_Table` with `ErrNoJudgmentReport→1` and `ErrJudgmentNotApproved→1` rows.
  Confirm `go test ./cmd/ch/...` FAILS (RED) on the new cases.
  _REQ-TEST-2, REQ-TEST-3, REQ-VALIDATOR-1..9_

- [x] **A1-5 — `main.go` (modify, GREEN)** · `platform/ci-checks/cmd/ch/main.go`
  Add `cmdJudgment(args []string, out io.Writer) error`: flags `--change` (required), `--changes-dir` (default `openspec/changes`), `--json`.
  Reads `os.ReadFile(<changesDir>/<slug>/judgment-report.md)`; if not found → `ErrNoJudgmentReport` → exit 1.
  Calls `ParseJudgmentReport(md)` → if `ErrMalformedJudgment` → `ErrJudgmentNotApproved`.
  Calls `report.ApprovedFor(slug)`.
  `--json` output shape: `{change, round, judges[]string, implementor, verdict, violations[]}` (violations never nil).
  Verdict mapping: file absent → `"MISSING"`, parse error → `"MALFORMED"`, else `report.Verdict`.
  Add `judgmentJSON` struct with JSON tags.
  Add `case "judgment"` to `run()` switch; update usage docstring; update unknown-subcommand error to include `"judgment"`.
  `exitCodeFor` does not change (any non-nil → 1).
  Confirm `go test ./cmd/ch/...` PASSES (GREEN) — all new and existing tests green.
  _REQ-VALIDATOR-1..9, REQ-ERR-1, REQ-TEST-4_

- [x] **A1-6 — Verify PR A1** (gate task)
  `cd platform/ci-checks && go test ./...` → PASS (0 FAIL). Existing tests for `labels`, `ownership`, `changed-modules`, `dod` remain green (REQ-TEST-4). `go build -o /dev/null ./cmd/ch` compiles cleanly.

---

## PR A2 — CI job + config + dog-food bootstrap (stacked on A1)

> **Base branch:** PR A1 branch
> **Files:** `.github/workflows/pr-checks.yml` (modify), `openspec/config.yaml` (modify), `openspec/changes/judgment-day-gate/judgment-report.md` (create — D-5)
> **REQ-IDs:** REQ-CI-GATE-1..7, REQ-ARCHIVE-RULE-1..3

- [ ] **A2-1 — CI job `judgment`** · `.github/workflows/pr-checks.yml`
  Add job `judgment` mirroring `dod` job structure:
  ```yaml
  judgment:
    name: Judgment Day gate (ch judgment)
    runs-on: ubuntu-latest
    needs: branch-name
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - uses: actions/setup-go@v5
        with:
          go-version: '1.26'
      - name: Build ch
        working-directory: platform/ci-checks
        run: go build -o /tmp/ch ./cmd/ch
      - name: Derive active change slugs
        id: slugs
        run: |
          SLUGS=$(git diff --name-only origin/${{ github.base_ref }}...HEAD \
            | grep '^openspec/changes/' \
            | grep -v '^openspec/changes/archive/' \
            | sed -E 's#^openspec/changes/([^/]+)/.*#\1#' \
            | sort -u)
          echo "slugs=$SLUGS" >> "$GITHUB_OUTPUT"
      - name: Run judgment gate
        run: |
          SLUGS="${{ steps.slugs.outputs.slugs }}"
          if [ -z "$SLUGS" ]; then
            echo "::notice::No active changes in diff — judgment gate N/A"
            exit 0
          fi
          for slug in $SLUGS; do
            /tmp/ch judgment --change "$slug"
          done
  ```
  Verify existing jobs `branch-name`, `labels`, `dod` are untouched (REQ-CI-GATE-6).
  _REQ-CI-GATE-1..7_

- [ ] **A2-2 — `rules.archive` block** · `openspec/config.yaml`
  Append additive block after `paths:` section:
  ```yaml
  rules:
    archive:
      require_judgment_report: true
      judgment_verdict: APPROVED
      judgment_report_path: "openspec/changes/{change}/judgment-report.md"
      escalation_authority: ZEUS
  ```
  _REQ-ARCHIVE-RULE-1..3_

- [ ] **A2-3 — Dog-food bootstrap (D-5)** · `openspec/changes/judgment-day-gate/judgment-report.md`
  **[GATE: ZEUS pre-open-PR]** Generate and commit the `judgment-report.md` for this very change on the implementor branch BEFORE opening PR A2. File must have header (`Change: judgment-day-gate`, `Round: 1`, `Judges: ARGOS-1, ARGOS-2`, `Implementor: HERMES`, `Date: <review-date>`) and terminal line `JUDGMENT: APPROVED ✅`. The CI `judgment` job must pass green on A2's own PR — gate proves itself. ZEUS approves before opening PR.
  _D-5, REQ-ARTIFACT-1..4_

- [ ] **A2-4 — Smoke-test A2 gate locally**
  On the A2 branch, run: `/tmp/ch judgment --change judgment-day-gate` → exit 0. Confirms dog-food bootstrap is valid before pushing.

- [ ] **A2-5 — Verify PR A2** (gate task)
  `cd platform/ci-checks && go test ./...` → PASS (A1 code still green). PR A2 CI is green including the new `judgment` job. `openspec/changes/judgment-day-gate/judgment-report.md` present and APPROVED in diff.

---

## PR A3 — Governance doc + ownership row (independent)

> **Base branch:** `develop` (independent — no dependency on A1 or A2)
> **Files:** `team-context/judgment-day.md` (create), `team-context/ownership.md` (modify — second table)
> **REQ-IDs:** REQ-GOVERNANCE-1..3
> **O-4 resolved:** `module:devops` label (HERMES). Add `team-context/judgment-day.md` row to ownership table's second section (shared-file rows) with note "governance doc; ATHENA maintains content".

- [ ] **A3-1 — `team-context/judgment-day.md`** (create, L4 doc) · `team-context/judgment-day.md`
  Create governance document covering (REQ-GOVERNANCE-2..3):
  - **Propósito:** ARGOS = 2 blind judges + evidence contract.
  - **Cuándo corre ARGOS:** after `sdd-verify` passes with 0 CRITICALs, before `sdd-archive` (HG5 gate).
  - **Artefacto que deja ARGOS:** `openspec/changes/{slug}/judgment-report.md` — path, header format (5 fields: Change, Round, Judges, Implementor, Date), terminal line options (`JUDGMENT: APPROVED` / `JUDGMENT: APPROVED ✅` / `JUDGMENT: ESCALATED` / `JUDGMENT: ESCALATED ⚠️`). Include a full example block.
  - **Quién commitea:** implementor on `agent/<figura>/TAL-N` branch (ARGOS owns no module — D-4).
  - **Escalación:** `ESCALATED` → ZEUS decision required; no auto-resolution; re-review increments `Round`.
  - **Quorum (advisory v1):** 2 judges distinct from implementor; violations surface-only (O-1).
  - **Mantenido por:** ATHENA (content) / HERMES (gate tooling).
  _REQ-GOVERNANCE-1..3_

- [ ] **A3-2 — ownership.md second table row** · `team-context/ownership.md`
  Add row to the second table (file→module map): `| \`team-context/judgment-day.md\` | HERMES | governance doc; ATHENA maintains content |`.
  Confirm `ParseOwnershipTable` skips the second table (existing behavior — no test change needed).
  _O-4 resolved, REQ-GOVERNANCE-1_

- [ ] **A3-3 — Verify PR A3** (gate task)
  `team-context/judgment-day.md` exists with all required sections. `team-context/ownership.md` second table updated. No Go code changed → no `go test` needed. Cross-ref: add a note to `team-context/merge-order.md` (if it references governance gates) pointing to `judgment-day.md`.

---

## Completion Checklist

**Status: A1 DONE — A2, A3 PENDING**

- [x] `cd platform/ci-checks && go test ./...` → PASS (0 FAIL) — domain + cmd all green
- [x] `go build -o /dev/null ./cmd/ch` in `platform/ci-checks` compiles cleanly
- [ ] `ch judgment --change judgment-day-gate` → exit 0 (with dog-food `judgment-report.md`)
- [ ] `.github/workflows/pr-checks.yml` has `judgment` job; existing jobs untouched
- [ ] `openspec/config.yaml` has `rules.archive` block
- [ ] `openspec/changes/judgment-day-gate/judgment-report.md` present and APPROVED (D-5)
- [ ] `team-context/judgment-day.md` exists with required content
- [ ] `team-context/ownership.md` second table has `judgment-day.md` row
- [ ] PR A1 open and green; PR A2 stacked on A1 and green (self-gated); PR A3 open independently and green
- [ ] ZEUS pre-open sign-off recorded for PR A2 (dog-food bootstrap gate — D-5)
- [ ] verify-report: 0 CRITICALs
