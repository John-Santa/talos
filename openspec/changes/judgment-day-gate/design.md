# Design: judgment-day-gate (Thread A · HG5)

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

## Data Flow

    git diff --name-only origin/<base>...HEAD
        └─→ grep ^openspec/changes/ (excl. archive/) → unique slug(s)
              └─→ ch judgment --change <slug>
                    └─ os.ReadFile(<changes-dir>/<slug>/judgment-report.md)
                         └─ ParseJudgmentReport(md) → JudgmentReport
                              └─ report.ApprovedFor(slug) → nil | err
                                   └─ exitCodeFor(err) → 0 APPROVED / 1 else

No change touched (no `openspec/changes/` slug in diff) → job prints notice, exit 0.

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

Parse rules (line-scan over `strings.Split(md,"\n")`, mirroring `ParseOwnershipTable`):
- Header fields via `**Field:**` prefix → `Change`, `Round`, `Judges` (comma-split), `Implementor`.
- Terminal line: `strings.HasPrefix(trimmed,"JUDGMENT:")`; strip trailing ` ✅`/` ⚠️`; `Verdict ∈ {APPROVED, ESCALATED}`, else `ErrMalformedJudgment`.
- Missing `JUDGMENT:` line or empty `Change` → malformed (fail-closed).
- O-1: `len(Judges)!=2` or `Implementor ∈ Judges` → append to `Violations`, NOT an error.

`ApprovedFor(change)`: nil iff `Verdict=="APPROVED"` AND `Change==change`; else `*ErrJudgmentNotApproved{Change, Verdict}`.

Errors (match existing struct style in `errors.go`): `ErrNoJudgmentReport` (`var ... = errors.New`), `ErrJudgmentNotApproved struct{Change, Verdict string}`, `ErrMalformedJudgment struct{Detail string}`. All map to exit 1 via the existing catch-all `exitCodeFor` (returns 1 for any non-nil) — confirmed, no change needed; add table rows only.

`judgmentJSON`: `{change, round, judges[], implementor, verdict, approved bool, violations[]}` (violations never nil → `[]string{}`).

## CI job (exact)

```yaml
  judgment:
    name: Judgment Day verdict (ch judgment)
    runs-on: ubuntu-latest
    needs: branch-name
    steps:
      - uses: actions/checkout@v4
        with: { fetch-depth: 0 }
      - uses: actions/setup-go@v5
        with: { go-version: '1.26' }
      - name: Build ch
        working-directory: platform/ci-checks
        run: go build -o /tmp/ch ./cmd/ch
      - name: Assert APPROVED judgment for touched change(s)
        run: |
          SLUGS=$(git diff --name-only origin/${{ github.base_ref }}...HEAD \
            | grep '^openspec/changes/' | grep -v '^openspec/changes/archive/' \
            | sed -E 's#^openspec/changes/([^/]+)/.*#\1#' | sort -u)
          if [ -z "$SLUGS" ]; then
            echo "::notice::No openspec change touched — judgment gate skipped"; exit 0
          fi
          for slug in $SLUGS; do
            echo "Checking judgment for $slug"
            /tmp/ch judgment --change "$slug"
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

- [ ] O-4: `module:*` label of the A.3 governance doc issue → tasks/dispatch.
- [ ] Header `Date` field: captured into struct or ignored by parser? Reco: ignored in v1 (not gate-relevant).
