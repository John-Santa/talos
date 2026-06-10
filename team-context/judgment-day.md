# Blackboard — Judgment Day (ARGOS / gate HG5)

> Revisión adversarial ciega. Desde **Fase 3** es **gate duro pre-`sdd-archive`** (CONSTITUTION §10 HG5, §12).
> ARGOS **juzga, no implementa** (§1). Decide ZEUS cuando escala.

## Cuándo corre

Después de `sdd-verify` PASS (0 CRITICAL) y **antes** de `sdd-archive`. El nodo del flujo es
`verify → judgment-day → archive`. El veredicto **APPROVED** es condición para archivar el change.

## Quién

- **Dos jueces ciegos** + un **fix-agent**, con **contexto fresco** y **distinto del implementor**
  (`reviewer_distinct_from_implementor: true`). ARGOS no es dueño de módulo: revisa todos, no escribe dominio.
- Si los jueces se contradicen o el caso escala → **decide ZEUS**. No hay resolución automática (§12/§13).

## Qué deja (el contrato de evidencia)

ARGOS deja un `judgment-report.md` committeado en la carpeta del change
(`openspec/changes/<change>/judgment-report.md`). Formato que el validador `ch judgment` exige:

```
**Change:** <slug>            # debe matchear el change
**Round:** <N>                # >= 1
**Judges:** <a>, <b>          # dos jueces; el implementor no puede ser juez (O-1, surface)
**Implementor:** <figura>
**Date:** YYYY-MM-DD
... cuerpo: hallazgos, rondas, fixes ...

JUDGMENT: APPROVED            # o JUDGMENT: ESCALATED — ÚLTIMA línea no-vacía, exacta, columna 0
```

Regla dura del parser (endurecida tras el dog-food de TAL-7): el verdicto se toma **solo** de la
última línea no-vacía, match exacto (opcional un emoji ✅/⚠️), y **una sola** línea estricta en todo el
documento. Cualquier `JUDGMENT:` en prosa/indentado/fenced → **malformed** (falla cerrado).

## Dónde se enforcea

- **Gate duro = CI (archive-scoped).** El job `judgment` de `.github/workflows/pr-checks.yml` dispara
  **solo** cuando un PR mueve un change a `openspec/changes/archive/<date>-<slug>/`, y exige que su
  `judgment-report.md` diga APPROVED (`ch judgment --change <slug> --report <ruta-archivada>`). Los
  feature PRs **no** se gatean acá: HG5 es **pre-archive**, no pre-feature-merge.
- **Segunda línea de defensa (agent-side):** `rules.archive` en `openspec/config.yaml` — `sdd-archive`
  no archiva sin un judgment-report APPROVED.

## Evidencia (§6: linkeá lo vivo, adjuntá lo muerto)

- El `judgment-report.md` es **evidencia muerta** → adjuntar al issue Jira (`evidence ... --attach`).
- El PR / run de CI es **evidencia viva** → remote-link al issue.

## Límites v1 (aceptados explícitamente por ZEUS)

- **Forjabilidad:** el report es Markdown committeado; su procedencia es por **protocolo** (lo produce
  ARGOS) + el check implementor∉judges (O-1, surface-only), **no** criptográfica.
- **Branch protection / `develop→main`:** hacer `judgment` un *required check* y gatear el path a `main`
  es config de GitHub de ZEUS, fuera del scope de código del repo.

> Ver también [merge-order.md](merge-order.md) (un change no entra al orden de merge de archive sin
> Judgment Day APPROVED) y [overlap-protocol.md](overlap-protocol.md).
