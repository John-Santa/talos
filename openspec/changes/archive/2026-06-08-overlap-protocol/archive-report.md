# Archive Report — overlap-protocol (TAL-4)

**Change:** overlap-protocol · **Jira:** TAL-4 · **Module:** module:qa · **Owner:** THEMIS
**Fase 2 · change #3** · **Archived:** 2026-06-08 · **Store:** hybrid (openspec/ + engram)

## Resultado

Tercera entrega de la Fase 2. CLI Go `ov` (`platform/overlap-guard/`, hexagonal, cero deps,
**autocontenido**) que automatiza la disciplina de `team-context/overlap-protocol.md`: enforcea la
regla dura **"mismo archivo → nunca en paralelo"** (Verdict BLOCK), serializa el solape de módulo
(SERIALIZE), y reporta la tasa de colisión que alimenta el gate **HG6** (< 15% para escalar 2→5+
agentes). Es **prevención** que va ANTES de la demo #5.

Tres subcomandos read-only: `ov check` (T0 — gate pre-asignación vía Jira, JQL `module:*` sobre
issues activos de ≠ dueño + cruce de checklists `files:`), `ov scan` (T1 — solape de archivos
cross-agente sobre worktrees activos), `ov metric` (HG6 — tasa de colisión).

`ov` es **complementario a `mo`**, no solapado: `mo` responde "¿esta rama conflictúa con `develop`
al mergear?" (`git merge-tree` vs base); `ov` responde "¿dos ramas/issues de agentes distintos tocan
el MISMO archivo?" (colisión en el origen, pairwise entre hermanos). El archive-report de `mo` ya
había reservado: "#3 mantiene su dominio".

## Entrega

| PR | Alcance | Estado |
|----|---------|--------|
| #10 | PR1 — domain + ports + mocks + service (puro, cero I/O) | MERGED |
| #11 | PR2 — adapters (jirarest/wtcli/gitcli) + cmd/ov + bootstrap | MERGED |
| #12 | fix-forward — W-01/W-02/W-03 (no llegaron al merge de #11) | MERGED |

`go test ./... -count=1` → **8 paquetes PASS / 0 FAIL** · `go vet` limpio · `go.mod` cero deps.
Verify final (post-fix): **0 CRITICAL · 0 WARNING · 3 SUGGESTION**.

## Decisiones clave (ADRs)

- **ADR-OV1: gitcli autocontenido** — `ov` re-implementa `Fetch`/`RevParse`/`ChangedFiles` (~50 ln)
  y NO importa `platform/merge-order-orchestrator` (single-writer exclusivo de HERMES). Cross-CLI =
  shell-out a `wt list --json`, nunca import Go.
- **ADR-OV2: jirarest pide `description` + aplana ADF** — el `description` de Jira es ADF JSON; se
  declara un `adfNode{Type,Text,Content}` local y se hace DFS de `content[].text` antes de parsear el
  checklist. No acopla al `evidence` de Fase 1.
- **ADR-OV3: atribución de módulo en T1 = opcional** — la regla dura same-file→BLOCK es pairwise
  sobre archivos y NUNCA necesita el módulo; la rama sólo da figura. `--ownership-file` enriquece el
  reporte, no es requerido.
- **ADR-OV4: `Guard` read-only por construcción** — no tiene NINGÚN write-port; "ov no muta Jira ni
  el working dir" es garantía de tipos, no convención (espejo del `Planner`/ADR-M1 de `mo`).
- **ADR-OV5: boundary HG6** — `OverThreshold` cuando rate **strictly > 0.15** (exactamente 0.15 NO
  es over), paridad con REQ-HEALTH-2 de `mo`.

## Incidente de proceso (resuelto, registrado)

El fix-batch de los 3 WARNINGs de verify (commits `c5806c8`, `396c071`) **no entró en el merge de
PR#11** (mergeado en `f65bdfd`, pre-fix); el verify post-fix se había corrido contra la rama local,
no contra develop. Se detectó verificando develop por contenido (`agent_a` presente, `agents[]`
ausente), se re-aplicó vía **PR#12 fix-forward** (cherry-pick limpio, suite verde) y se re-verificó
contra el develop resultante (`da6a566`). Lección en engram
`sdd/overlap-protocol/archive-premature-fix`: **antes de archivar, verificar
`git merge-base --is-ancestor <fix-sha> origin/develop`** — un fix pusheado después de que el PR se
mergeó en un commit anterior no llega.

## Follow-up (3 SUGGESTION — bajo riesgo, no bloqueante)

- **S-01**: ubicación de `ErrWtBinaryNotFound`/`ErrWtOutputMalformed` (en `adapter/wtcli`, no en `domain/overlap`).
- **S-02**: texto de `ErrNoClaims` ambiguo entre files-file vacío y worktrees vacíos.
- **S-03**: comentarios inline en `parse.go`/`guard.go` violan la convención godoc-only.

## Estado del store

- Spec promovido a canónico: `openspec/specs/overlap-guard/functional.md`.
- Artefactos del change archivados en este directorio (`openspec/changes/archive/2026-06-08-overlap-protocol/`).
- Engram: topics `sdd/overlap-protocol/*` (proposal #1866, spec #1867, design #1868, tasks #1869,
  apply-progress #1871, verify-report #1872, archive-report) + `architecture/talos-fase2-roadmap`.

## Qué desbloquea

Tercer pilar de la Fase 2. Habilita el **change #4 `ci-hardening` (TAL-5)** (CI-green readiness, toca
`.yml`; reusa el cliente Jira de Fase 1 para el invariante de labels) y crucialmente el **change #5
`demo 2-devs` (TAL-6)**, donde `ov metric` alimenta el gate **HG6** (tasa de colisión < ~15%).
Secuencia Fase 2: `#1 wt` ✓ → `#2 mo` ✓ → `#3 overlap` ✓ → `#4 ci` → `#5 demo`.
