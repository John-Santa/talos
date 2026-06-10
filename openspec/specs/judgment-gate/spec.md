# Spec: judgment-day-gate — Delta Functional Requirements

**Change:** judgment-day-gate
**Jira:** TAL-A (por crear)
**Module:** module:devops
**Owner:** HERMES
**Phase:** Fase 3 · HG5
**RFC 2119 keywords apply throughout.**

---

## Overview

Este spec describe el comportamiento observable que MUST ser verdadero después de aplicar el change
`judgment-day-gate`. No prescribe implementación ni nombres de paquetes — eso es responsabilidad de la
fase de diseño.

Este change introduce la capability **`judgment-gate`**: el contrato de evidencia del veredicto de
Judgment Day (artefacto `judgment-report.md` + validador `ch judgment` + job de CI `judgment` +
`rules.archive`) como gate duro pre-archive (HG5).

**Capabilities añadidas:** `judgment-gate` (net-new).
**Capabilities modificadas:** ninguna (extensión aditiva del CLI `ch` existente; los requisitos
de `labels`, `ownership`, `changed-modules`, `dod` y `branch-name` son invariantes).

**Fuera de scope (explícito):**
- Thread B (`ov`-en-CI) → change hermano `collision-guard-ci`.
- Edición del Kit global `~/.claude` (skills `judgment-day` / `sdd-archive`, instrucciones de ATHENA).
- Auto-trigger del nodo `verify→judgment-day→archive` en el DAG de ATHENA → Fase 5.
- `ch` ejecutando la review adversarial (ARGOS corre fuera; `ch` sólo valida el artefacto).
- Generación automática del `judgment-report.md` (CI asserta presencia + APPROVED; no genera).

---

## REQ-ARTIFACT — Contrato de evidencia: `judgment-report.md`

### REQ-ARTIFACT-1: Ruta canónica del artefacto

El artefacto de evidencia MUST ubicarse en:

```
openspec/changes/{change-slug}/judgment-report.md
```

donde `{change-slug}` es el slug canónico del change (e.g. `judgment-day-gate`).

**Given** un change activo con slug `{slug}`,
**When** el gate `judgment` se evalúa,
**Then** el archivo `openspec/changes/{slug}/judgment-report.md` MUST existir en el repositorio;
su ausencia MUST causar exit 1 con `ErrNoJudgmentReport`.

### REQ-ARTIFACT-2: Header obligatorio (cinco campos)

**Given** un `judgment-report.md` que existe en la ruta canónica,
**When** su contenido es parseado,
**Then** MUST contener exactamente estas cinco líneas de header en cualquier posición antes de la
línea terminal, cada una en formato `**Campo:** valor`:

| Campo | Descripción |
|-------|-------------|
| `**Change:**` | Slug del change (e.g. `judgment-day-gate`) |
| `**Round:**` | Entero ≥ 1 (e.g. `1`, `2`) |
| `**Judges:**` | Lista de identificadores de jueces (e.g. `ARGOS-1, ARGOS-2`) |
| `**Implementor:**` | Identificador del implementor (e.g. `HERMES`) |
| `**Date:**` | Fecha de la revisión (formato libre; no vacío) |

La ausencia de cualquier campo MUST causar que el report sea considerado malformado y MUST
disparar exit 1 con `ErrJudgmentNotApproved`.

### REQ-ARTIFACT-3: Campo `Round` — valor mínimo

**Given** un `judgment-report.md` con campo `**Round:**` presente,
**When** su valor es parseado,
**Then** el valor numérico MUST ser `>= 1`.
Un `Round: 0` MUST ser tratado como malformado y MUST causar exit 1 con `ErrJudgmentNotApproved`.

### REQ-ARTIFACT-4: Línea terminal — formato y unicidad

**Given** un `judgment-report.md` bien formado en header,
**When** el parser busca el veredicto,
**Then** MUST existir exactamente UNA línea terminal que sea una de las siguientes formas:

```
JUDGMENT: APPROVED
JUDGMENT: APPROVED ✅
JUDGMENT: ESCALATED
JUDGMENT: ESCALATED ⚠️
```

La línea terminal MUST ser la última línea no-vacía del archivo. El sufijo emoji (`✅`, `⚠️`) es
OPTIONAL y MUST ser tolerado sin alterar el veredicto.

Si no existe ninguna línea terminal parseable, el report MUST ser tratado como malformado y MUST
causar exit 1 con `ErrJudgmentNotApproved`.

### REQ-ARTIFACT-5: Campo `Change` matchea el slug del invocador

**Given** `ch judgment --change <slug>` se ejecuta contra un `judgment-report.md`,
**When** el parser lee el campo `**Change:**`,
**Then** el valor MUST ser igual (case-sensitive) al `<slug>` provisto en `--change`.
Una discrepancia MUST causar exit 1 con `ErrJudgmentNotApproved`.

---

## REQ-VALIDATOR — Subcomando `ch judgment`

### REQ-VALIDATOR-1: Interfaz del subcomando

El CLI `ch` MUST exponer el subcomando `judgment` con la siguiente interfaz:

```
ch judgment --change <slug> [--changes-dir openspec/changes] [--json]
```

| Flag | Tipo | Default | Descripción |
|------|------|---------|-------------|
| `--change` | string | (requerido) | Slug del change a validar |
| `--changes-dir` | string | `openspec/changes` relativo al repo root | Directorio base de changes |
| `--json` | bool | false | Emitir resultado como JSON |

### REQ-VALIDATOR-2: Exit code 0 — veredicto APPROVED

**Given** `ch judgment --change <slug>` se ejecuta,
**When** el archivo `openspec/changes/<slug>/judgment-report.md` existe, parsea sin errores,
el `Round` es ≥ 1, el campo `Change` coincide con `<slug>`, y la línea terminal es
`JUDGMENT: APPROVED` (o con sufijo emoji tolerado),
**Then** `ch judgment` MUST retornar exit code **0**.

### REQ-VALIDATOR-3: Exit code 1 — report ausente

**Given** `ch judgment --change <slug>` se ejecuta,
**When** el archivo `openspec/changes/<slug>/judgment-report.md` no existe,
**Then** `ch judgment` MUST retornar exit code **1** con error `ErrNoJudgmentReport`.

### REQ-VALIDATOR-4: Exit code 1 — veredicto ESCALATED

**Given** `ch judgment --change <slug>` se ejecuta,
**When** el archivo existe y parsea correctamente pero la línea terminal es
`JUDGMENT: ESCALATED` (con o sin sufijo emoji),
**Then** `ch judgment` MUST retornar exit code **1** con error `ErrJudgmentNotApproved`.

### REQ-VALIDATOR-5: Exit code 1 — sin línea terminal parseable

**Given** `ch judgment --change <slug>` se ejecuta,
**When** el archivo existe pero no contiene ninguna línea que siga el formato
`JUDGMENT: APPROVED[...] | JUDGMENT: ESCALATED[...]`,
**Then** `ch judgment` MUST retornar exit code **1** con error `ErrJudgmentNotApproved`.

### REQ-VALIDATOR-6: Exit code 1 — header malformado (campo faltante o Round 0)

**Given** `ch judgment --change <slug>` se ejecuta,
**When** el archivo existe pero le falta uno o más campos obligatorios del header, o el campo
`Round` tiene valor 0,
**Then** `ch judgment` MUST retornar exit code **1** con error `ErrJudgmentNotApproved`.

### REQ-VALIDATOR-7: Exit code 1 — mismatch de `Change`

**Given** `ch judgment --change foo-change` se ejecuta,
**When** el archivo existe pero contiene `**Change:** bar-change`,
**Then** `ch judgment` MUST retornar exit code **1** con error `ErrJudgmentNotApproved`.

### REQ-VALIDATOR-8: Tolerancia al sufijo emoji en APPROVED

**Given** `ch judgment --change <slug>` se ejecuta,
**When** la línea terminal del report es `JUDGMENT: APPROVED ✅`,
**Then** `ch judgment` MUST retornar exit code **0** (idéntico a `JUDGMENT: APPROVED` sin emoji).
El emoji es cosmético; no altera el veredicto.

### REQ-VALIDATOR-9: Salida `--json`

**Given** `ch judgment --change <slug> --json` se ejecuta,
**When** el subcomando evalúa el report,
**Then** MUST emitir a stdout un objeto JSON con exactamente estos campos:

```json
{
  "change":     "<slug>",
  "round":      <int>,
  "judges":     "<string>",
  "verdict":    "APPROVED" | "ESCALATED" | "MALFORMED" | "MISSING",
  "violations": ["<string>", ...]
}
```

El campo `violations` MUST ser un array vacío `[]` cuando no hay violaciones. El campo `verdict`
MUST ser `"MISSING"` cuando el archivo no existe, `"MALFORMED"` cuando existe pero no parsea, y
`"APPROVED"` o `"ESCALATED"` cuando parsea correctamente.

La salida `--json` MUST usarse con el exit code definido en REQ-VALIDATOR-2 a REQ-VALIDATOR-7:
`--json` no altera el exit code.

### REQ-VALIDATOR-10: Advertencias de surface (O-1 — no hard-fail en v1)

**Given** `ch judgment --change <slug>` se ejecuta,
**When** el report parsea correctamente pero presenta alguna de estas condiciones:
(a) el número de jueces listados en `**Judges:**` es distinto de 2, o
(b) el valor de `**Implementor:**` aparece también en la lista de `**Judges:**`,
**Then** `ch judgment` MUST incluir esas condiciones en el campo `violations[]` del output
(`--json`) o en stderr (modo texto), pero MUST NOT cambiar el exit code.
En v1 estas son advertencias de superficie; NO son hard-fail.

---

## REQ-CI-GATE — Job `judgment` en CI

> **Revised post Judgment Day Round 1 (C1/C3 hardening)**: The gate is now archive-scoped.
> It fires ONLY when the PR diff includes paths under `openspec/changes/archive/<date>-<slug>/`.
> Feature PRs exit 0 (notice) — this is correct, not a bypass. HG5 is pre-archive; feature PRs
> do not require a judgment report.

### REQ-CI-GATE-1: Job `judgment` declarado en `pr-checks.yml`

**Given** un PR abierto contra `develop`,
**When** el workflow `pr-checks` se ejecuta,
**Then** MUST existir un job llamado `judgment` en `.github/workflows/pr-checks.yml`.

### REQ-CI-GATE-2: Activación condicional — el PR es un archive PR

**Given** el job `judgment` corre,
**When** el diff del PR modifica al menos un archivo bajo `openspec/changes/archive/<date>-<slug>/`,
**Then** el job MUST correr `ch judgment --change <slug> --report openspec/changes/archive/<date>-<slug>/judgment-report.md`
para cada archived change folder detectada en el diff. El `<slug>` se obtiene eliminando el prefijo
`YYYY-MM-DD-` de `<date>-<slug>`.

### REQ-CI-GATE-3: N/A cuando el PR no es un archive PR

**Given** el job `judgment` corre,
**When** el diff del PR no modifica ningún archivo bajo `openspec/changes/archive/`,
**Then** el job MUST retornar exit 0 con un notice `"Not an archive PR — HG5 judgment gate N/A"`.
Esto asegura que PRs de features, infraestructura, docs u otras áreas no sean bloqueados por el
gate. Este exit 0 es CORRECTO — no es un bypass.

### REQ-CI-GATE-4: Rojo ante report faltante o no-APPROVED

**Given** el job `judgment` detecta un archive folder en el diff,
**When** `ch judgment --change <slug> --report <path>` retorna exit 1
(report faltante, malformado, o ESCALATED),
**Then** el job MUST fallar (exit 1) y el PR MUST quedar rojo.

### REQ-CI-GATE-5: Verde ante APPROVED

**Given** el job `judgment` detecta un archive folder en el diff,
**When** `ch judgment --change <slug> --report <path>` retorna exit 0 (veredicto APPROVED),
**Then** el job MUST pasar (exit 0) y no bloquear el merge.

### REQ-CI-GATE-6: Jobs existentes siguen verdes (aditividad)

**Given** el job `judgment` existe en el workflow,
**When** cualquier PR pasa por `pr-checks`,
**Then** los jobs `branch-name`, `labels`, y `dod` MUST seguir comportándose de forma idéntica
al estado anterior a este change. El change es aditivo: no modifica la lógica de ningún job
existente.

### REQ-CI-GATE-7: Derivación del slug desde el diff (archive-scoped)

**Given** el job `judgment` necesita determinar qué changes archivados están presentes en el diff,
**When** el job procesa el diff del PR,
**Then** MUST buscar paths que coincidan con `^openspec/changes/archive/[^/]+/`, extraer el
folder name `<date>-<slug>`, eliminar el prefijo `YYYY-MM-DD-` para obtener `<slug>`, y construir
el `--report` path como `openspec/changes/archive/<date>-<slug>/judgment-report.md`.

### REQ-CI-GATE-8: Flag `--report` en `ch judgment`

**Given** `ch judgment` se ejecuta con el flag `--report <path>`,
**When** el flag está presente,
**Then** `ch judgment` MUST leer el archivo en `<path>` en lugar de derivar
`<changes-dir>/<slug>/judgment-report.md`. El flag `--change` sigue siendo usado para
validar el campo `**Change:**` del header (defense-in-depth). Un `<path>` inexistente MUST
causar exit 1 con `ErrNoJudgmentReport`.

---

## REQ-ARCHIVE-RULE — Segunda línea de defensa: `rules.archive`

### REQ-ARCHIVE-RULE-1: Bloque `rules.archive` en `openspec/config.yaml`

**Given** `openspec/config.yaml` es el archivo de configuración del store SDD,
**When** el archivo es inspeccionado,
**Then** MUST contener un bloque `rules.archive` con al menos los siguientes campos:

```yaml
rules:
  archive:
    require_judgment_report: true
    judgment_verdict: APPROVED
    judgment_report_path: "openspec/changes/{change}/judgment-report.md"
    escalation_authority: ZEUS
```

### REQ-ARCHIVE-RULE-2: `sdd-archive` respeta `require_judgment_report: true`

**Given** `openspec/config.yaml` declara `rules.archive.require_judgment_report: true`
y `rules.archive.judgment_verdict: APPROVED`,
**When** `sdd-archive` intenta archivar un change,
**Then** MUST verificar la existencia y el veredicto del `judgment-report.md` antes de proceder.
Si el archivo no existe o el veredicto no es `APPROVED`, `sdd-archive` MUST NO archivar y MUST
reportar el bloqueo.

### REQ-ARCHIVE-RULE-3: Escalación ESCALATED → ZEUS

**Given** `openspec/config.yaml` declara `rules.archive.escalation_authority: ZEUS`
y el `judgment-report.md` contiene `JUDGMENT: ESCALATED`,
**When** `sdd-archive` evalúa el gate,
**Then** MUST NOT archivar automáticamente. MUST NOT resolver la escalación sin intervención humana.
El bloqueo MUST indicar que la resolución corresponde a `ZEUS` (la autoridad declarada en
`escalation_authority`).

---

## REQ-GOVERNANCE — Protocolo de Judgment Day

### REQ-GOVERNANCE-1: Existencia del documento de protocolo

**Given** el change `judgment-day-gate` es aplicado,
**When** se inspecciona el repositorio,
**Then** MUST existir el archivo `team-context/judgment-day.md`.

### REQ-GOVERNANCE-2: Contenido mínimo del protocolo

**Given** `team-context/judgment-day.md` existe,
**When** su contenido es inspeccionado,
**Then** MUST documentar al menos:
(a) Cuándo corre ARGOS: después de que `sdd-verify` pasa sin CRITICALes y antes de `sdd-archive`.
(b) Qué artefacto deja ARGOS: el archivo `judgment-report.md` con el formato especificado en
    REQ-ARTIFACT-2 a REQ-ARTIFACT-4.
(c) El protocolo de escalación: veredicto `ESCALATED` → decisión de ZEUS (sin resolución
    automática; §12/§13 de CONSTITUTION.md).

### REQ-GOVERNANCE-3: Formato del report documentado en el protocolo

**Given** `team-context/judgment-day.md` documenta el formato del artefacto,
**When** el documento es leído,
**Then** MUST incluir un ejemplo o descripción del formato completo de `judgment-report.md`,
incluyendo los cinco campos del header y las dos formas válidas de línea terminal
(`JUDGMENT: APPROVED` / `JUDGMENT: ESCALATED`), con y sin sufijo emoji.

---

## REQ-ERR — Catálogo de errores

### REQ-ERR-1: Errores introducidos por este change

**Given** `ch judgment` encuentra una condición de error conocida,
**When** retorna el error al llamador,
**Then** MUST usar el identificador correspondiente de este catálogo:

| Identificador | Tipo | Condición de disparo |
|---------------|------|----------------------|
| `ErrNoJudgmentReport` | `var` (sentinel) | El archivo `judgment-report.md` no existe en la ruta canónica |
| `ErrJudgmentNotApproved` | `var` (sentinel) | El report existe pero: header malformado, `Round < 1`, change-mismatch, sin línea terminal parseable, o veredicto `ESCALATED` |

Ambos errores MUST producir exit code 1. El error `ErrNoJudgmentReport` MUST ser distinguible de
`ErrJudgmentNotApproved` para permitir diagnósticos precisos en el output `--json` (`verdict:
"MISSING"` vs `"MALFORMED"` / `"ESCALATED"`).

### REQ-ERR-2: Errores existentes no se modifican

**Given** los errores existentes del paquete `cichecks` (`ErrNoJiraKey`, `ErrLabelInvariant`,
`ErrMalformedOwnership`, `ErrIssueNotFound`),
**When** este change es aplicado,
**Then** sus tipos, mensajes, y comportamientos MUST permanecer sin cambios.

---

## REQ-TEST — Testeabilidad

### REQ-TEST-1: `ParseJudgmentReport` — función pura, sin I/O

**Given** la función de dominio `ParseJudgmentReport` (o equivalente),
**When** los unit tests corren,
**Then** MUST existir tests table-driven en `platform/ci-checks/domain/cichecks/` cubriendo
cada fila de la siguiente tabla:

| Caso | Input (resumen) | Exit esperado | Error esperado |
|------|-----------------|---------------|----------------|
| APPROVED sin emoji | Header completo + `JUDGMENT: APPROVED` | 0 | nil |
| APPROVED con emoji ✅ | Header completo + `JUDGMENT: APPROVED ✅` | 0 | nil |
| ESCALATED sin emoji | Header completo + `JUDGMENT: ESCALATED` | 1 | `ErrJudgmentNotApproved` |
| ESCALATED con emoji ⚠️ | Header completo + `JUDGMENT: ESCALATED ⚠️` | 1 | `ErrJudgmentNotApproved` |
| Archivo faltante | — | 1 | `ErrNoJudgmentReport` |
| Sin línea terminal | Header completo, sin línea `JUDGMENT:` | 1 | `ErrJudgmentNotApproved` |
| `Round: 0` | Header con `Round: 0` + `JUDGMENT: APPROVED` | 1 | `ErrJudgmentNotApproved` |
| Change-mismatch | `Change: other-slug` + `JUDGMENT: APPROVED` | 1 | `ErrJudgmentNotApproved` |
| Header incompleto (campo faltante) | Falta `**Judges:**` | 1 | `ErrJudgmentNotApproved` |

Estos tests MUST NOT tocar el sistema de archivos ni requerir I/O externo.

### REQ-TEST-2: `ch judgment` — integration tests de la capa cmd

**Given** el subcomando `judgment` en `platform/ci-checks/cmd/ch/main_test.go`,
**When** los tests de `run()` corren (estilo `table-driven`, inyectando un directorio temporal),
**Then** MUST existir casos de prueba para: APPROVED→exit 0, ESCALATED→exit 1, falta→exit 1,
sin-terminal→exit 1, round-0→exit 1, change-mismatch→exit 1, trailing ✅ tolerado→exit 0.

### REQ-TEST-3: Rojo→verde→refactor (strict TDD)

**Given** el proyecto tiene `strict_tdd: true` en `openspec/config.yaml`,
**When** se implementa cualquier lógica de `ParseJudgmentReport` o `case "judgment"`,
**Then** los tests correspondientes MUST ser escritos PRIMERO (rojo), luego la implementación
(verde), luego el refactor. `go test ./...` MUST pasar en verde antes de abrir cualquier PR.

### REQ-TEST-4: Tests existentes siguen verdes

**Given** los tests existentes de `ch` (`labels`, `ownership`, `changed-modules`, `dod`),
**When** este change es aplicado,
**Then** todos los tests existentes MUST seguir pasando sin modificación.

---

## REQ-OUT-OF-SCOPE — Deferrals explícitos

Los siguientes ítems NO son requisitos de este change y MUST NOT ser implementados:

| Ítem | Deferido a |
|------|-----------|
| Cableado `ov`-en-CI (Thread B) | Change hermano `collision-guard-ci` |
| Edición de skills/instrucciones globales `~/.claude` (judgment-day, sdd-archive, ATHENA) | Repo-only; auto-trigger del nodo = Fase 5 |
| Auto-trigger `verify→judgment-day→archive` en el DAG de ATHENA | Fase 5 |
| Ejecución de la review adversarial por `ch` | ARGOS corre fuera; `ch` sólo valida el artefacto |
| Generación automática del `judgment-report.md` | CI asserta presencia; no genera |

---

## Open questions (design/dispatch — NO resolver aquí)

| ID | Pregunta | Constraint del spec |
|----|----------|---------------------|
| O-1 | ¿Hard-fail si `judges ≠ 2` o `implementor ∈ judges`? | **En v1: surface-only** (REQ-VALIDATOR-10). `violations[]` los lista; NO altera exit code. La spec lo fija como warning de superficie. |
| O-2 | ¿1 PR = 1 change slug? | El job `judgment` itera todos los slugs activos detectados en el diff (REQ-CI-GATE-7). La spec permite múltiples, pero el caso típico es 1. Diseño define la lógica de iteración. |
| O-3 | ¿El `judgment-report.md` se commitea en la rama del implementor? | Reco: sí (ARGOS no es dueño de módulo; el implementor lo adjunta a su rama). Diseño confirma. |
| O-4 | Label del issue del doc A.3 (`team-context/judgment-day.md`) | Dispatch (tasks). El path `team-context/` pertenece a ATHENA según `ownership.md`; si la fila no existe, se añade una nueva bajo ATHENA. |

---

## Trazabilidad de requisitos

| REQ-ID | Fuente (propuesta) | Decisión / riesgo |
|--------|-------------------|-------------------|
| REQ-ARTIFACT-1 a 5 | §A.1, §Success Criteria | Formato fijo espejo de verify-report |
| REQ-VALIDATOR-1 a 10 | §A.1, §Success Criteria, O-1 | O-1 → surface-only v1 (REQ-VALIDATOR-10) |
| REQ-CI-GATE-1 a 7 | §A.2, §Success Criteria | Espejo del job `dod`; N/A si PR no toca changes |
| REQ-ARCHIVE-RULE-1 a 3 | §A.2, §Resolved decisions #4 | `sdd-archive` lee config línea 145; sin tocar skill |
| REQ-GOVERNANCE-1 a 3 | §A.3, §Success Criteria | Doc mantenido por ATHENA; escalación ESCALATED→ZEUS sin auto-resolución |
| REQ-ERR-1 a 2 | §A.1 (errores) | Nuevos sentinels; errores existentes invariantes |
| REQ-TEST-1 a 4 | CONSTITUTION; strict TDD; §Constraint heredada | Rojo→verde→refactor; sin tocar tests existentes |
| REQ-OUT-OF-SCOPE | §Out of scope (propuesta) | Thread B, Kit global, auto-trigger |
