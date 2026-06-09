# Archive Report — jira-portability (TAL-6)

**Change:** jira-portability · **Jira:** TAL-6 · **Módulos:** module:devops + module:qa (slice ov)
**Owners:** HERMES + THEMIS · **Fase 2 · change #5** · **Archived:** 2026-06-09 · **Store:** hybrid (openspec/ + engram)

---

## Resultado

Quinta entrega de la Fase 2. Kit-PREP: matanza de dos acoplamientos duros (ausencia de carga local-first de secrets + identidad de proyecto hardcodeada) que impedían clonar Talos a un proyecto Jira distinto sin tocar Go. Las cuatro CLIs (`evidence`, `ch`, `ov`, `wt`) ahora operan contra **cualquier proyecto Jira** llenando `.talos/project.env` (identidad no-secreta, committeado) + `.env` (secrets, gitignoreado).

Entrega cross-cutting: HERMES escribió en 3 módulos (evidence, ci-checks, worktree-orchestrator); THEMIS slice (overlap-guard). Unified implementation: parser zero-dep `internal/envfile/` (canónico en evidence, copia verbatim en los otros); envfile loader hoisted en `run()` antes del switch; regex parametrizada con `regexp.QuoteMeta` (sin injection); 3 scalar overrides en composition roots. REQ-AUTH-1 enmendado en `openspec/specs/jira-loop/functional.md`.

## Entrega

| PR | Alcance | Estado |
|----|---------|--------|
| #17 | PR1-4 (HERMES 4 commits) — envfile canónico + 3 overrides + parametrizaciones branch/naming + hoist + identity | MERGED on develop |
| #18 | PR5 (THEMIS 1 commit) — ov envfile copia + 2 overrides + identity | MERGED on develop |

`go test ./... -count=1` (4 módulos) → **439 PASS / 0 FAIL** · `go vet` limpio · go.mod cero deps.
Verify final: **PASS — 0 CRITICAL / 0 WARNING / 2 SUGGESTION** (post fix-batch para resolver 3 WARNINGs iniciales).

---

## Decisiones clave (rationale completo en engram)

- **D2: Formato KEY=VALUE en `.talos/project.env`** — cero-dep, cero parser nuevo; mismo `envfile.Parse` compartido entre secretos y identidad.
- **D3: Hoist `LoadInto` al tope de `run()`** — carga ANTES del switch, ANTES de que `flag` evalúe defaults con `os.Getenv`. Crítico (R8).
- **D4: Loader NO fail-fast genérico** — solo PUEBLA el env; cada CLI mantiene SU validación (`ch` creds-opcional, `ov check` sí, `ov scan`/`metric` no, `evidence` fuerte).
- **D5: NO renombrar `DefaultTALConfig`** — 9 call sites; override escalares en composition roots (R1).
- **D6: Regex param + `regexp.QuoteMeta`** — dominio puro, `projectKey` por parámetro; sin injection.
- **D7: Creds en worktrees = env heredado + fallback `.env` principal** — `.env` gitignoreado no existe en worktree nuevo; `.talos/project.env` tracked sí.
- **D8: Reúso `envfile` — canónico evidence, copia verbatim ch/ov (zero-dep prohíbe cross-import; mismo patrón `ValidateOwnership`).

---

## Fix-Batch (post-verify, pre-archive)

Verify inicial: PASS WITH WARNINGS (0 CRITICAL / 3 WARNING / 2 SUGGESTION). Los 3 WARNINGs fueron gatillos para tight fix-batch bajo strict TDD, aprobado por ZEUS:

| ID | What | Commit | Rama | Resolved |
|----|------|--------|------|----------|
| **W-3** | `ErrInvalidKey` hardcodeaba "TAL-<n>"; refactored a struct que transporta `projectKey`, mensaje dinámico | `213f7d6` | agent/hermes/TAL-6 (PR #17) | ✅ |
| **W-2** | REQ-BRANCH-5 test case explícito ausente (`FOO/agent/hermes/TAL-7 → ErrNoJiraKey`); agregado en `branch_test.go` | `213f7d6` | agent/hermes/TAL-6 (PR #17) | ✅ |
| **W-1** | REQ-IDENT-1 override (`JIRA_PROJECT_KEY`) ausente en `cmdScan`/`cmdMetric` de ov; agregado tras `DefaultTALConfig()` en líneas ~336/374 | `1dc8beb` | agent/themis/TAL-6 (PR #18) | ✅ |

Re-verificación post-fix en árbol integrado de `agent/hermes/TAL-6 + agent/themis/TAL-6` sobre develop: **439 PASS**, `go vet` limpio. Las 2 SUGGESTIONs quedan como follow-ups no-bloqueantes (S-1: comentarios inline con literales `TAL-N` en wt/main.go + errors.go; S-2: test case para clave vacía tras trim en envfile_test.go).

---

## Test Results

### `go test ./... -count=1` (full suite, 4 módulos)

| Módulo | Tests | PASS | FAIL |
|--------|-------|------|------|
| `jira-evidence-loop` | 5 paquetes | 111 | 0 |
| `ci-checks` | 6 paquetes | 118 | 0 |
| `worktree-orchestrator` | 5 paquetes | 101 | 0 |
| `overlap-guard` | 7 paquetes | 109 | 0 |
| **Total** | **23** | **439** | **0** |

`go test -short ./... -count=1` — idéntico (0 skips).
`go vet ./...` — LIMPIO, 0 advertencias.

---

## REQ Coverage — verificación final

| REQ-ID | Status | Verificación |
|--------|--------|---|
| REQ-ENV-1..7 | ✅ PASS | `LoadInto` hoisted en los 4 binarios antes del switch; precedencia + if-unset + error manejo correcto |
| REQ-PARSE-1..7 | ✅ PASS | `Parse` puro sin I/O, 11+ test cases table-driven (blank, comment, first-=, trim, CRLF, empty, etc.) |
| REQ-IDENT-1 | ✅ PASS (post W-1 fix) | Override en evidence/ch/cmdCheck/ov-cmdCheck/cmdScan/cmdMetric/wt-newOrch (uniforme tras fix) |
| REQ-IDENT-2..4 | ✅ PASS | `DefaultTALConfig()` fallback; `.talos/project.env` confirmado no-secret |
| REQ-BRANCH-1..5 | ✅ PASS (post W-2 fix) | Lógica parametrizada + QuoteMeta; tests completos incluyendo FOO/TAL-7 → error |
| REQ-WTKEY-1..4 | ✅ PASS | `ValidateJiraKey` parametrizado + QuoteMeta; tests TAL/TAL-1, TAL/TAL-0, FOO/FOO-7, FOO/TAL-5 |
| REQ-AUTH-1 (enmienda) | ✅ PASS | `openspec/specs/jira-loop/functional.md` línea 26-31; REQ-AUTH-1 enmendado permitiendo `.talos/project.env` non-secret |
| REQ-WT-1..5 | ✅ PASS | `mainWorktreeRoot` con `git rev-parse --git-common-dir`; wt carga solo `project.env` (no `.env`); golden env_test.go intacto (sin `JIRA_`) |
| REQ-COMPAT-1..3 | ✅ PASS | Sin archivos → valores por defecto (back-compat); CI env no pisado; golden invariant preserved |
| REQ-ERR-1..2 | ✅ PASS (post W-3 fix) | `ErrInvalidKey` ahora struct con `projectKey`, mensaje dinámico `must match <key>-<n>` |
| REQ-TEST-1..5 | ✅ PASS | Parse + LoadInto inyectables; branch/naming table-driven con proyectos alternos; back-compat tests PASS sin cambios |

**Veredicto:** PASS — todos los requisitos cubiertos. W-1, W-2, W-3 resueltos pre-archive.

---

## Zero-Dep Validation

Los 4 módulos no introducen dependencias externas:

```
github.com/John-Santa/talos/platform/jira-evidence-loop     go 1.26
github.com/John-Santa/talos/platform/ci-checks             go 1.26
github.com/John-Santa/talos/platform/worktree-orchestrator go 1.26
github.com/John-Santa/talos/platform/overlap-guard         go 1.26
```

Cada `go.mod`: 2 líneas (module + go version). Sin `require`, sin `go.sum`. ✅

---

## Portability Proof

**Escenario:** clonar Talos a proyecto `FOO` con Jira `https://foo.atlassian.net`:

```bash
# 1. Llenar .talos/project.env (committeado)
export JIRA_SITE_URL="https://foo.atlassian.net"
export JIRA_PROJECT_KEY="FOO"
export JIRA_PROJECT_ID=20001

# 2. Llenar .env (gitignoreado)
export JIRA_EMAIL="dev@foo.com"
export JIRA_API_TOKEN="<token>"

# 3. CLIs corren contra FOO sin cambios Go
./ch labels --branch agent/hermes/FOO-7 # ✅ branch parsea FOO-7
./wt create hermes FOO-1 --no-fetch     # ✅ clave FOO-1 válida, project.env loaded
./ov check --branch agent/iris/FOO-99   # ✅ valida contra FOO-99
./evidence run --project-key FOO --project-id 20001  # ✅ corre contra FOO
```

**Back-compat (TAL con CI env):**

```bash
# Sin .talos/project.env, sin override JIRA_PROJECT_KEY
./ch labels --branch agent/hermes/TAL-7   # ✅ DefaultTALConfig() → TAL, identidad por defecto
# CI env (JIRA_EMAIL, JIRA_API_TOKEN, JIRA_SITE_URL) siempre gana (REQ-ENV-2)
```

---

## What Unlocks

**Habilitador directo:**
- **Fase 5 Kit extraction** — acoplamientos Go resueltos; módulos ya son portables con config externa.
- **Onboarding fluído a nuevos proyectos Jira** — cero edición de Go, solo `.env` + `.talos/project.env`.

**Bloqueos resueltos:**
- **Proposal D1: portabilidad TOTAL** — HERMES D6 (regex pura parametrizada con `QuoteMeta`) + D7 (env heredado en wt) + D3 (hoist) + D5 (override) dan solución cross-module.
- **Composición de módulos cross-cutting** — THEMIS slice (overlap-guard) en PR #18 demuestra ownership §2: mismo patrón reusable sin duplicación de decisiones.

**Fase 2 roadmap (post-TAL-6):**
1. **change #1 `worktree-orchestration` (TAL-2)** — ✅ archivado.
2. **change #2 `merge-order-automation` (TAL-3)** — ✅ archivado.
3. **change #3 `overlap-protocol` (TAL-4)** — ✅ archivado.
4. **change #4 `ci-hardening` (TAL-5)** — ✅ archivado.
5. **change #5 `jira-portability` (TAL-6)** — ✅ archivado (THIS). **Fase 2 COMPLETE.**

---

## Follow-up documentado (no-bloqueante)

| ID | Ítem | Severidad | Tipo |
|----|------|-----------|------|
| S-1 | Comentarios inline en wt/main.go + errors.go con literales `TAL-N` → actualizar a `<KEY-N>` | SUGGESTION | Polish |
| S-2 | Test case para clave vacía tras trim (`"  = value"`) en envfile_test.go | SUGGESTION | Coverage |
| F-1 | Comentario `mirror — mantener en sync` en `.talos/project.env` ↔ `openspec/config.yaml:jira` | Follow-up | Documentation |
| F-2 | `IssueTypeName` multilingüe + `OrderedStates` en idiomas alternativos | Follow-up | Extensión |

---

## Artefactos de especificación

| Artifact | Location | Observability |
|----------|----------|---|
| Proposal | `openspec/changes/archive/2026-06-09-jira-portability/proposal.md` | Engram topic |
| Spec (delta → **canonical NEW**) | `openspec/specs/jira-portability/functional.md` (newly created) | File + Engram |
| Spec jira-loop (REQ-AUTH-1 enmendado) | `openspec/specs/jira-loop/functional.md` (updated) | File + Engram |
| Design | `openspec/changes/archive/2026-06-09-jira-portability/design.md` | Engram topic |
| Tasks | `openspec/changes/archive/2026-06-09-jira-portability/tasks.md` | Engram topic |
| Verify Report | `openspec/changes/archive/2026-06-09-jira-portability/verify-report.md` | Engram topic |
| Archive Report | this file + Engram `sdd/jira-portability/archive-report` | Traceability |

---

## Estado del store

- **Canonical specs promocionados:** `openspec/specs/jira-portability/functional.md` (NEW) + `openspec/specs/jira-loop/functional.md` (enmendado REQ-AUTH-1).
- **Change folder archivado:** `openspec/changes/archive/2026-06-09-jira-portability/` (todos 5 artifacts tracked).
- **Engram:** topics `sdd/jira-portability/*` (proposal, spec, design, tasks, verify-report, archive-report) + `architecture/talos-portability` (cross-cutting decisions).

---

## Gotchas de entorno / decisiones de implementación

- **Strict TDD:** 439 tests full suite, cero skip en full mode.
- **Cero deps:** incluso stdlib-only; `regexp.QuoteMeta` evita injection sin librerías.
- **Byte-identidad envfile:** diff de las 4 copias confirma identidad exacta (mismo patrón `ValidateOwnership`).
- **Hoist ordering crítico (R8):** loader antes del switch, antes de flag defaults con `os.Getenv`. Auditado en los 4 binarios.
- **Parametrizacion + dominio puro:** regex arma en composition root con `projectKey`; dominio recibe como param (no env/config inyectado).
- **IF-UNSET no overwrite:** (REQ-ENV-2) env heredado siempre gana; CI sin cambios.

---

## Roadmap Alignment

**Fase 2 completada:** 5 changes archivados (TAL-2, TAL-3, TAL-4, TAL-5, TAL-6). La plataforma Talos es ahora **por-proyecto configurable sin tocar Go** — Kit-PREP habilitada. Siguiente: Fase 3 (onboarding kit, docs) o Fase 5 (Kit extraction a repo/módulo propio).
