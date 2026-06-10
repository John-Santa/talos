# Design: judgment-day-gate (Thread A · HG5) — Revised post Round 1

## Technical Approach

Mirror the proven `verify-report → job dod` mechanism for the Judgment Day verdict. Pure domain parser (`ParseJudgmentReport`, same line-scan style as `ParseOwnershipTable`), a `cmd/ch case "judgment"` that does the file I/O (no fs port — domain stays pure on strings), a CI `judgment` job that derives the change slug from the diff and runs `ch judgment` fail-closed, plus an additive `rules.archive` block honored by `sdd-archive` (skill line 145, unchanged). Strict TDD: domain RED→GREEN first, then cmd, then CI.

## Architecture Decisions

| # | Decision | Choice | Rejected | Rationale |
|---|----------|--------|----------|-----------|
| D-1 | Where validation lives | Extend `ch` (`platform/ci-checks`) | New binary | `ch` is already the gate engine (`labels`/`dod`); `judgment` is one more gate of the same module (HERMES) |
| D-2 | I/O boundary | `cmd` does `os.ReadFile`; domain pure on `string` | fs port in domain | Module has NO fs port today; `ParseOwnershipTable` is pure — keep it that way |
| D-3 | O-1 judges≠2 / impl∈judges | Surface-only → append to `Violations[]`, NOT hard error | hard-fail v1 | Verdict APPROVED is the gate; quorum hygiene is advisory until ARGOS protocol stabilizes |
| D-4 | O-3 artifact location | Implementor commits `judgment-report.md` on `agent/<figura>/TAL-N` | ARGOS owns a path | ARGOS owns no module; CI must see the file in the PR diff, same as `verify-report.md` today |
| D-5 | Dog-food bootstrap | The introducing PR ships its OWN `APPROVED` judgment-report.md | bootstrap allow-list / skip-if-absent | Keeps the gate honest — no "first PR is exempt" hole; the gate proves itself by passing on itself |
| D-6 | ESCALATED handling | Parses fine, `ApprovedFor` returns `ErrJudgmentNotApproved` → exit 1 | treat as malformed | ESCALATED is a valid verdict that must fail the gate, not a parse error |
| D-7 | Gate scope | Archive-scoped: CI job fires ONLY when diff includes `openspec/changes/archive/<date>-<slug>/` | Feature-merge gate | C1/C3 hardening: non-archive PRs exit 0 (notice); archive moves folder so `--report` explicit path prevents self-deadlock |
| D-8 | Strict verdict parser | Two-pass: (1) header scan, (2) last-non-empty-line exact regex + uniqueness | `HasPrefix` anywhere | C2 hardening: eliminates decoy lines, fenced blocks, indented lines, trailing-text bypass, prose mid-body |
| D-9 | `--report` flag | `ch judgment --report <path>` reads that exact file; `--change` still validates header | Derive path only | Archive path changes after `sdd-archive` moves folder; `--report` passes explicit path, slug for defense-in-depth match |

## Data Flow

### Feature PRs (non-archive)

    git diff → no openspec/changes/archive/ path → notice "Not an archive PR" → exit 0
    (gate is N/A for feature merges; this is CORRECT, not a bypass)

### Archive PRs (sdd-archive moves folder to archive/)

    git diff --name-only origin/<base>...HEAD
        └─→ grep '^openspec/changes/archive/' → unique <date>-<slug> folders
              └─→ strip YYYY-MM-DD- prefix → bare <slug>
                    └─→ ch judgment --change "<slug>" --report "openspec/changes/archive/<date>-<slug>/judgment-report.md"
                          └─ os.ReadFile(<report-path>)  ← explicit, not derived
                               └─ ParseJudgmentReport(md) [strict two-pass]
                                    └─ Pass 1: header scan (Change/Round/Judges/Implementor/Date)
                                    └─ Pass 2: last-non-empty-line strict regex + uniqueness count
                                         └─ report.ApprovedFor(slug) → nil | err
                                              └─ exitCodeFor(err) → 0 APPROVED / 1 else

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `domain/cichecks/judgment.go` | Create | `JudgmentReport` + `ParseJudgmentReport` + `ApprovedFor` |
| `domain/cichecks/judgment_test.go` | Create | Table-driven parse + verdict cases |
| `domain/cichecks/errors.go` | Modify | Add `ErrNoJudgmentReport`, `ErrJudgmentNotApproved` |
| `cmd/ch/main.go` | Modify | `cmdJudgment`, `case "judgment"`, usage doc, unknown-list, `judgmentJSON` |
| `cmd/ch/main_test.go` | Modify | `cmdJudgment` cases + `exitCodeFor` rows |
| `.github/workflows/pr-checks.yml` | Modify | New `judgment` job (needs `branch-name`) |
| `openspec/config.yaml` | Modify | Additive `rules.archive` block |
| `team-context/judgment-day.md` | Create | Governance + escalation protocol |
| `team-context/ownership.md` | Modify | Add `team-context/judgment-day.md` shared-file row (gated by ATHENA) |

## Interfaces / Contracts

```go
type JudgmentReport struct {
    Change, Implementor, Verdict, RawVerdictLine string
    Round                                        int
    Judges, Violations                           []string
}
func ParseJudgmentReport(md string) (JudgmentReport, error)
func (r JudgmentReport) ApprovedFor(change string) error
```

Parse rules (strict two-pass, C2 hardening):

**Pass 1 — Header scan** (collects fields, ignores JUDGMENT lines entirely):
- Lines matching `**Field:** value` → `Change`, `Round`, `Judges` (comma-split), `Implementor`, `Date`.
- JUDGMENT lines in prose, fenced blocks, quotes, etc. are invisible to pass 1.

**Pass 2 — Last non-empty line strict check** (verdict):
- Scan all lines (raw, not TrimSpace) to find the last non-empty line.
- Apply `verdictLineRe = ^JUDGMENT: (APPROVED|ESCALATED)(suffix)?$` where suffix ∈ `{" ✅", " ⚠️"}` or absent.
- Also count all lines matching this strict pattern in the full document; if count > 1 → `ErrMalformedJudgment` (prevents decoy+real dual-line bypass).
- If last non-empty line does not match → `ErrMalformedJudgment`.
- Leading whitespace on the verdict line disqualifies it (column-0 requirement).

**Rejected cases (all now malformed)**:
- `JUDGMENT: APPROVED but with caveats` (trailing text beyond the optional emoji)
- `    JUDGMENT: APPROVED` (indented)
- `` `JUDGMENT: APPROVED` `` inside a fenced block (not at column 0 or not last non-empty line)
- Two JUDGMENT lines anywhere in the document
- `JUDGMENT: APPROVED` mid-prose where last line is body text

O-1: `len(Judges)!=2` or `Implementor ∈ Judges` → append to `Violations`, NOT an error.

`ApprovedFor(change)`: nil iff `Verdict=="APPROVED"` AND `Change==change`; else `*ErrJudgmentNotApproved{Change, Verdict}`.

Errors (match existing struct style in `errors.go`): `ErrNoJudgmentReport` (`var ... = errors.New`), `ErrJudgmentNotApproved struct{Change, Verdict string}`, `ErrMalformedJudgment struct{Detail string}`. All map to exit 1 via the existing catch-all `exitCodeFor` (returns 1 for any non-nil) — confirmed, no change needed; add table rows only.

`judgmentJSON`: `{change, round, judges[], implementor, verdict, approved bool, violations[]}` (violations never nil → `[]string{}`).

## CI job (exact — revised, archive-scoped)

```yaml
  judgment:
    # Archive-scoped gate (HG5): fires ONLY when this PR moves a change folder
    # into openspec/changes/archive/<date>-<slug>/. Feature PRs are NOT gated
    # here — the gate applies at archive-time, not at feature-merge time.
    name: Judgment Day archive gate (HG5)
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
      - name: Assert judgment-report APPROVED for archived change(s)
        run: |
          ARCHIVE_FOLDERS=$(git diff --name-only origin/${{ github.base_ref }}...HEAD \
            | grep '^openspec/changes/archive/' \
            | sed -E 's#^openspec/changes/archive/([^/]+)/.*#\1#' \
            | sort -u)
          if [ -z "$ARCHIVE_FOLDERS" ]; then
            echo "::notice::Not an archive PR — HG5 judgment gate N/A"
            exit 0
          fi
          for dated_slug in $ARCHIVE_FOLDERS; do
            slug=$(echo "$dated_slug" | sed -E 's/^[0-9]{4}-[0-9]{2}-[0-9]{2}-//')
            report_path="openspec/changes/archive/${dated_slug}/judgment-report.md"
            echo "Checking judgment gate for archived change: $slug (report: $report_path)"
            /tmp/ch judgment --change "$slug" --report "$report_path"
          done
```

## config.yaml rules.archive (exact)

```yaml
# --- Reglas de gate (honradas por sdd-archive, línea 145) ---
rules:
  archive:
    require_judgment_report: true     # judgment-report.md con JUDGMENT: APPROVED (HG5)
    judgment_verdict: APPROVED
```

## team-context/judgment-day.md (outline)

- **Propósito**: protocolo de Judgment Day (ARGOS, 2 jueces LLM ciegos) y su contrato de evidencia.
- **Cuándo corre**: post-`verify`, pre-`archive` (HG5).
- **Artefacto**: `openspec/changes/{change}/judgment-report.md` — header (`Change`/`Round`/`Judges`/`Implementor`/`Date`) + línea terminal `JUDGMENT: APPROVED|ESCALATED`. Formato = fuente de verdad (anti-drift).
- **Quién lo commitea**: el implementor, en `agent/<figura>/TAL-N` (D-4).
- **Escalación**: `ESCALATED → ZEUS` decide; re-review incrementa `Round`.
- **Quorum (O-1)**: 2 jueces distintos del implementor; violación = warning (surface-only v1).
- **Mantenido por**: ATHENA.

## Testing Strategy

| Layer | What | Approach |
|-------|------|----------|
| Domain | parse header/verdict, ESCALATED→err, malformed, O-1 violations, `ApprovedFor` match/mismatch | table-driven `judgment_test.go`, RED→GREEN |
| Cmd | flag parse, missing file→`ErrNoJudgmentReport`, text + JSON shape, `exitCodeFor` rows | `main_test.go` in-process `run([]string,...)` + temp dir fixture |
| CI | slug derivation, no-change→exit 0, APPROVED→green, ESCALATED/missing→red | real `judgment` job (self-gated, D-5) |

## Migration / Rollout

No migration. Three chained PRs (auto-chain): **A.1** domain+errors+cmd (`platform/ci-checks/**`); **A.2** CI job + `rules.archive` (stacked on A.1); **A.3** governance doc + ownership row (independent). A.2's introducing PR ships its own APPROVED `judgment-report.md` (D-5).

## Open Questions

- [x] O-4: `module:*` label of the A.3 governance doc issue → `module:devops` (HERMES is gate-tooling owner). Resolved in tasks.
- [x] Header `Date` field: captured into struct or ignored by parser? → Ignored in v1 (presence required, value not gate-relevant). Resolved in tasks.

---

## Judgment Day Round 1 — Resolved Findings

ARGOS Round 1 identified three CRITICAL issues in the original A2 design. All have been resolved in this revised A2 implementation.

### C1 — Empty CHANGES bypass (RESOLVED)

**Original flaw**: The CI job fired on every develop PR. PRs touching no `openspec/changes/<slug>/` file produced `CHANGES=""` → `exit 0` → ungated code could merge.

**Root cause**: Gate scope was feature-merge time, not archive-time. A feature PR that doesn't touch a change dir would skip the gate entirely.

**Resolution**: Gate is now archive-scoped. The job fires ONLY when the diff includes paths under `openspec/changes/archive/<date>-<slug>/`. Non-archive PRs print a notice and exit 0 — this is correct behavior, not a bypass. HG5 is a pre-archive gate; feature PRs don't need a judgment report.

### C2 — Parser bypass via `HasPrefix` anywhere (RESOLVED)

**Original flaw**: `strings.HasPrefix(trimmed, "JUDGMENT:")` matched JUDGMENT lines anywhere in the document. Attack vectors:
- Decoy `JUDGMENT: APPROVED` before a quoted/prose `JUDGMENT: ESCALATED`
- `JUDGMENT: APPROVED but with caveats` — `fields[0]` extracted `APPROVED`
- Indented `    JUDGMENT: APPROVED` — TrimSpace made it match
- `JUDGMENT: APPROVED` inside a fenced code block
- `JUDGMENT: APPROVED` mid-prose where the last line is body text
- Duplicate guard only fired AFTER the scan, so a decoy+real combo could still pass

**Resolution**: Two-pass strict parser:
1. Header pass: collects fields, never looks for JUDGMENT lines.
2. Last-non-empty-line pass: only the final non-empty line of the document can be the verdict. It must match `^JUDGMENT: (APPROVED|ESCALATED)(suffix)?$` exactly at column 0 (raw line, no TrimSpace). Additionally, the entire document is scanned for strict-pattern matches; if count > 1, the report is malformed (prevents decoy+real bypass).

All six attack vector cases have RED→GREEN tests in `judgment_test.go`.

### C3 — Archive self-deadlock (RESOLVED)

**Original flaw**: The original gate fired at feature-merge time and derived the report path as `openspec/changes/<slug>/judgment-report.md`. When `sdd-archive` ran, it would move the folder to `openspec/changes/archive/<date>-<slug>/` — so the report path in the live change dir would vanish, causing the gate to fail on its own archive PR.

**Resolution**: 
1. Gate is now archive-scoped (C1 fix), so it fires at archive-time when the folder is already in `archive/`.
2. The `--report <path>` flag allows CI to pass the exact archive path explicitly: `--report openspec/changes/archive/<date>-<slug>/judgment-report.md`.
3. `--change` is still used for the `**Change:**` header match, providing defense-in-depth.

### Accepted v1 Limits (not bugs — deliberate deferrals)

**Forgeability**: The `judgment-report.md` is a committed Markdown file. Provenance is established by the ARGOS protocol (two blind LLM judges, documented in `team-context/judgment-day.md`) and the O-1 surface check (judges ≠ implementor). Cryptographic signing of the report is deferred (out of v1 scope).

**Branch-protection / main path**: Making `judgment` a required CI check and gating `develop → main` requires GitHub repository settings configuration. This is a ZEUS (human) concern, not repo code scope. The implementation provides the gate mechanism; ZEUS enables it in GitHub settings.
