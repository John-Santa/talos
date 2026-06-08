# Verify Report — overlap-protocol (TAL-4)

**Change:** overlap-protocol · **Jira:** TAL-4 · **Module:** module:qa · **Owner:** THEMIS
**Verdict (final, post-fix re-verify): PASS — 0 CRITICAL / 0 WARNING / 3 SUGGESTION**

> Reconstruido tras incidente de merge-timing (ver nota al pie): el `verify-report.md` original
> quedó en una rama local y no llegó a develop. Este refleja el estado real post-fix verificado
> contra `origin/develop`.

## Suite (contra develop, post PR#12)

| Suite | Resultado |
|-------|-----------|
| `go test ./... -count=1` | PASS — 0 FAIL, 8 paquetes |
| `go test -short ./... -count=1` | PASS — 0 FAIL (integración skippeada por diseño) |
| `go vet ./...` | limpio |
| `go.mod` deps externas | 0 (stdlib only) |
| Single-writer | `rg "merge-order-orchestrator" platform/overlap-guard` → vacío (gitcli autocontenido) |

## Primer verify: 0 CRITICAL / 3 WARNING / 3 SUGGESTION

### WARNING (los 3 → RESUELTOS por fix-batch, mergeados en PR#12)

**W-01 — `advisories[]` nunca se emitía (REQ-CHECKLIST-4 / REQ-OUTPUT-1)** — RESOLVED
`Report.Advisories []string` agregado; `Guard.CheckPreAssignment` acumula un advisory por
`ErrChecklistMissing` y lo setea; `cmd/ov` emite `advisoriesSlice(report)` (ya no `[]string{}`).
Invariante mantenido: issue sin checklist sigue contribuyendo a `module_overlaps[]`.
Tests: `TestGuard_CheckPreAssignment_MissingChecklist_PopulatesAdvisories`,
`TestReportToCheckJSON_AdvisoriesFromReport`.

**W-02 — campos scan-only en `check --json` + `cmdMetric` sin test (REQ-OUTPUT-1)** — RESOLVED
Structs separados por subcomando: `checkOnlyJSON` (sin campos de métrica), `scanOnlyJSON`,
`metricOnlyJSON` (sólo métrica). Test de `cmdMetric` agregado.
Tests: `TestReportToCheckJSON_OmitsScanOnlyFields`, `TestReportToMetricJSON_OnlyMetricFields`,
`TestCmdMetric_RequiresNoArgsForHelp`.

**W-03 — schema `agent_a`/`agent_b` en vez de `agents[]` (REQ-OUTPUT-1)** — RESOLVED
`file_collisions[]` y `module_overlaps[]` emiten `agents: []string` (ordenado, determinista).
Dominio (`FileCollision`/`ModuleOverlap`) intacto — cambio sólo en la capa de serialización.
Tests: `TestReportToCheckJSON_FileCollisions_AgentsArray`,
`TestReportToCheckJSON_ModuleOverlaps_AgentsArray`.

## SUGGESTION (3 — no bloqueantes, follow-up)

- **S-01** — `ErrWtBinaryNotFound`/`ErrWtOutputMalformed` viven en `adapter/wtcli`, no en
  `domain/overlap` donde el catálogo de errores los implica.
- **S-02** — `ErrNoClaims` se usa para dos escenarios (files-file vacío y worktrees vacíos); el exit
  code es correcto (0) en ambos pero el texto del mensaje es ambiguo en el caso worktrees.
- **S-03** — comentarios inline de rationale en cuerpos de `parse.go`/`guard.go` violan la convención
  del proyecto (godoc sólo en exportados, cuerpos autoexplicativos).

## TDD Compliance

37/37 tasks completas y marcadas `[x]`. RED→GREEN confirmado contra la evidencia de apply-progress.
5 tests RED nuevos en el fix-batch. Sin tautologías ni ghost-loops detectados.

## Nota: incidente de merge-timing (resuelto)

El fix-batch de los 3 WARNINGs (commits `c5806c8`, `396c071`) NO entró en PR#11 (mergeado en
`f65bdfd`, pre-fix). Se detectó verificando develop por contenido, se re-aplicó vía **PR#12
fix-forward** (cherry-pick limpio), y este verify final corre contra el develop resultante
(`da6a566`). Lección registrada en engram (`sdd/overlap-protocol/archive-premature-fix`):
verificar `git merge-base --is-ancestor <fix-sha> origin/develop` antes de archivar.
