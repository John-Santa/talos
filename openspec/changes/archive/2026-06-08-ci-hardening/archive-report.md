# Archive Report — ci-hardening (TAL-5)

**Change:** ci-hardening · **Jira:** TAL-5 · **Module:** module:devops · **Owner:** HERMES
**Fase 2 · change #4** · **Archived:** 2026-06-08 · **Store:** hybrid (openspec/ + engram)

## Resultado

Cuarta entrega de la Fase 2. CLI Go `ch` (`platform/ci-checks/`, hexagonal, cero deps) que implementa el motor del invariante §4 (CONSTITUTION.md): exactamente un `agent:*` y un `module:*` por issue Jira con ownership validado y rama corroborada. Tres gates CI en `.github/workflows/pr-checks.yml` (`branch-name`, `labels`, `dod`) bloquean PRs a `develop` si el invariante falla, migrando la arquitectura desde `ci/` (scripts) a YAML native.

Consume `wt list --json` y `mo` del `mo` change (TAL-3, archivado) indirectamente vía gate `dod` que verifica módulo Go y tests; **posture HERMETIC**: `ch` solo gate de invariante, no decision-maker (reporte, no escalación).

## Entrega

| PR | Alcance | Estado |
|----|---------|--------|
| #14 | PR1 — domain + service (logic puro, cero red hasta tests) | MERGED |
| #15 | PR2 — adapters + cmd + gates YAML + cleanup (Jira real, file IO) | MERGED |

`go test ./... -count=1` → **59 PASS / 0 FAIL** · short 5 PASS / 1 SKIP · integration 7 PASS · `go vet` limpio · go.mod cero deps.
Verify final: **0 CRITICAL · 2 WARNING · 3 SUGGESTION**.

## Decisiones clave (rationale completo en engram)

- **ADR-C1: Port split hermetic** — `Checker` (orchestration) depende SOLO de `BranchParser` + `LabelsFetcher` + `OwnershipReader` + `InvariantValidator`; NUNCA puede tomar decisiones de Jira (read-only + validation only). Tipos enforzan esta partición.
- **REQ-BRANCH-1 W-1: case-insensitive figura — normalizacion deferred to cmd layer** — regex en `domain/cichecks/branch.go` es lowercase-only; `cmd/ch/main.go:142-149` normaliza pre-parse. Contrato observable satisfecho; implementación auditada.
- **`ch labels` y `ch ownership` separan concerns** — labels valida invariante (Jira + ownership); ownership solo parsea y valida estructura (offline). Fácil de testear por separado.
- **Gates YAML native — no shellscript:** `branch-name` usa regex simple; `labels` invoca `ch labels --json`; `dod` corre tests + verifica verify-report. Shell puro, cero dependencias.
- **Integration tests skipped under -short** — `adapter/jirarest` httptest runs; integration que toca Jira real skipped con flag `-short`. Cumple strict TDD.

## Follow-up documentado (bajo riesgo, no bloqueante)

- **W-1**: REQ-BRANCH-1 advisory para parseo case-insensitive — regex lowercase, normalización deferred a cmd. Implementado end-to-end, no critico. Status: FIXED en PR2.
- **W-2**: REQ-JSON-2 edge case — `ch labels --branch develop --json` DEBE emitir JSON con violation, no mezclar texto. Implementado; test en `cmd/ch/main_test.go:275`.
- **S-1**: REQ-JIRA-1 credentials de env vars — si alguien pasa creds vía arg explícito, `ch` las rechaza. Esperado; la spec solo permite env vars.
- **S-2**: REQ-GATES-4 `needs` dependency — asume que CI runner respeta `needs`; GitHub Actions lo hace. Spec observable, no riesgo.
- **S-3**: REQ-CLEANUP-2 ownership.md entry para `platform/ci-checks/**` — debe existir en la tabla de archivo→módulo post-apply. Verificado en develop (commit c76fdfe).

## Estado del store

- Spec promovido a canónico: `openspec/specs/ci-checks/functional.md`.
- Artefactos del change archivados en este directorio.
- Engram: topics `sdd/ci-hardening/*` (#XXX proposal, #XXX spec, #XXX design, #XXX tasks, #XXX verify-report) + `architecture/talos-fase2-roadmap` + evolving.

## Gotchas de entorno / decisiones de implementación

- **Strict TDD:** 59 top-level tests + 24 subtests, cero skip en full mode; integration skip bajo `-short` por design.
- **cero deps:** incluso stdlib-only en dominio. No `regexp2` ni librerías externas.
- **Coexistencia con #2 merge-order-automation y #3 overlap-protocol:** `ch` es hermética (solo valida, no escala). `mo` rankea por surface; `ov` detecta collisions. Composición limpia.
- **Mock de JiraFetcher como test-seam:** inyectable en `service/Checker` para test de invariante sin Jira real.

## Qué desbloquea

Estrictamente CI. Habilita el bloqueo de PRs que violen invariante §4 **before merging to develop**. Consume outputs de `mo` indirectamente (gate `dod` verifica tests de `mo` y de `ch`). Desbloquea **change #5 `demo 2-devs`** (prueba paralela con gate HG6 collision-rate con `mo` + `ov`). La secuencia Fase 2: `#1 wt` → `#2 mo` → `#3 overlap` → `#4 ci` (THIS) → `#5 demo`.

## Artefactos de especificación

| Artifact | Location | Observability |
|----------|----------|---|
| Proposal | `openspec/changes/archive/2026-06-08-ci-hardening/proposal.md` | Engram topic |
| Spec (delta → canonical) | `openspec/specs/ci-checks/functional.md` (newly promoted) | Engram topic |
| Design | `openspec/changes/archive/2026-06-08-ci-hardening/design.md` | Engram topic |
| Tasks | `openspec/changes/archive/2026-06-08-ci-hardening/tasks.md` | Engram topic |
| Verify Report | `openspec/changes/archive/2026-06-08-ci-hardening/verify-report.md` | Engram topic |
| Archive Report | this file + Engram `sdd/ci-hardening/archive-report` | traceability |

## Roadmap Alignment

Fase 2 roadmap (`[[architecture/talos-fase2-roadmap]]`):
1. **change #1 `worktree-orchestration` (TAL-2)** — archivado 2026-06-08. ✅
2. **change #2 `merge-order-automation` (TAL-3)** — archivado 2026-06-08. ✅
3. **change #3 `overlap-protocol` (TAL-4)** — archivado 2026-06-08. ✅
4. **change #4 `ci-hardening` (TAL-5)** — archivado 2026-06-08 (THIS). ✅
5. **change #5 `demo 2-devs` (TAL-6)** — ready to start (unblocked; requires review of gate config in TAL-5).
