# Judgment Report: judgment-day-gate (TAL-7)

**Change:** judgment-day-gate
**Round:** 2
**Judges:** judge-a, judge-b
**Implementor:** hermes
**Date:** 2026-06-09

## Resumen

Revisión adversarial ciega (Judgment Day) del gate HG5 de Talos. Dos jueces independientes
(opus, contexto fresco, distintos del implementor). Dos rondas: la primera versión fue
**rechazada**; la endurecida fue **aprobada**.

## Ronda 1 — ISSUES (rechazado)

Ambos jueces, de forma independiente y convergente, encontraron agujeros reales:

| ID | Severidad (consenso) | Hallazgo |
|----|----|----|
| C1 | CRÍTICO (A+B) | El job corría en cada PR a develop; un PR que no tocaba ningún `openspec/changes/<slug>/` daba CHANGES vacío y salía 0 → código sin gate mergeaba. Bypass. |
| C2 | CRÍTICO (A) / real (B) | El parser matcheaba la marca de verdicto en cualquier lado (HasPrefix). 6 vectores: decoy + verdicto citado, trailing tokens, indentado, fenced, in-body, duplicados evadiendo el contador. |
| C3 | real (A+B) | El gate corría en feature-merge, no en archive; y el archive mueve la carpeta a `archive/`, dejando el report fuera de ruta → self-deadlock del PR de archive. |

## Fixes aplicados

- **C1 + C3** — Re-scope a **archive-time** (constitución §12: gate duro pre-`sdd-archive`). El job
  dispara solo cuando el diff toca `openspec/changes/archive/<date>-<slug>/`; los feature PRs son
  N/A (correcto, no bypass). Flag `--report` para pasar la ruta del report archivado; `--change`
  sigue validando el header del change (defensa en profundidad).
- **C2** — Parser de dos pasos: el verdicto debe ser la **última línea no-vacía**, match exacto en
  columna 0, con unicidad doc-wide (más de una línea estricta → malformed). Los 6 vectores tienen
  tests RED→GREEN.

## Ronda 2 — verificación (APPROVED)

Ambos jueces reconstruyeron `ch`, corrieron la suite fresca y reprodujeron cada ataque de Ronda 1
más evasiones nuevas (CRLF, tabs, NBSP, look-alikes Unicode, fences, trailing whitespace/blank
lines). Resultado:

- **C1 — RESUELTO** (ambos, con evidencia): archive-scoped; el store hybrid siempre escribe
  `archive/<date>-<slug>/`, así que todo archive real dispara el gate; omitir el report da exit 1.
- **C2 — RESUELTO** (ambos): los 6 vectores y todas las evasiones nuevas fallan-cerrado; los reports
  válidos siguen correctos.
- **C3 — RESUELTO** (ambos): `--report` desacopla la ruta del match de slug; header mismatch bloquea.

Un WARNING-real compartido (fail-**cerrado**, no bypass): un archivo suelto directo bajo `archive/`
causaba un falso-positivo que bloqueaba un PR legítimo. Fix aplicado: anclar el grep a un subfolder
(`^openspec/changes/archive/[^/]+/`). Verificado: archivos sueltos ya no matchean, folders reales sí.

## Límites v1 aceptados (documentados)

- **Forjabilidad**: el report es Markdown committeado; su procedencia es por protocolo ARGOS más el
  check implementor-no-es-juez (O-1, surface-only), no criptográfica.
- **Branch protection / main path**: hacer `judgment` un required check y gatear `develop→main` es
  config de GitHub de ZEUS, fuera del scope de código del repo.

## Tabla de resolución

| ID | Severidad | Resolución |
|----|----|----|
| C1 | CRÍTICO | Gate archive-scoped; feature PRs N/A |
| C2 | CRÍTICO | Parser estricto última-línea + unicidad |
| C3 | real | Flag `--report` con ruta de archive explícita |

JUDGMENT: APPROVED ✅
