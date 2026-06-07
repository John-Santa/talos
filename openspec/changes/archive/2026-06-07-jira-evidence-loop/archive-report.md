# Archive Report — jira-evidence-loop

**Archivado:** 2026-06-07 · **Estado:** CERRADO · **Issue:** TAL-1 · **Owner:** HERMES (`module:jira-loop`)

## Qué se entregó

Helper Go hexagonal (`platform/jira-evidence-loop/`) que ejecuta el loop de evidencia de Jira de 7
pasos contra el REST v3, con auth Basic, **cero dependencias transitivas** y arquitectura
domain/port/adapter/service. Primer artefacto extraíble al Kit.

## Entrega (PRs encadenados)

| PR | Scope | Estado |
|---|---|---|
| [#1](https://github.com/John-Santa/talos/pull/1) | domain + port + service + mock + unit tests | **merged → develop** |
| [#2](https://github.com/John-Santa/talos/pull/2) | adapter/rest + cmd + integration smoke + ownership | **merged → develop** |

## Verificación

- **59 tests verdes** (27 adapter httptest · 21 domain · 11 service) · `go vet` limpio ·
  `go build -tags=integration` compila.
- **31/31 REQs** del spec cubiertas. Verify: **0 CRITICAL**.
- Seguridad: Basic auth en las 8 llamadas · `X-Atlassian-Token: no-check` · multipart · `globalId` upsert.
- Decisiones lockeadas implementadas: auth env-only · idempotencia por tupla `(change,phase,agent)` ·
  transición por `statusCategory.key` + `OrderedStates` (3 estados `indeterminate` reales de TAL) ·
  `net/http` a mano (sin go-jira).

## Loop de evidencia (validado en vivo, dogfooding)

TAL-1: create + 4 labels (Rovo) · transiciones En curso → En revisión (Rovo) · 2 remote-links
PR#1/#2 (community) · 2 adjuntos verify-report (community) · 2 comentarios. **Confirma la ruta
híbrida Rovo+community de punta a punta** (Rovo no sube binarios; community sí).

## Diferido a Fase 2 (verify WARNINGs, no bloqueantes)

- Profundizar asserts de valores en el wire test de remote-link.
- Smoke de integración de los pasos 5-6 (worklog/remote-link/attachment) contra Jira real (requiere token).

## Spec promovida

`openspec/specs/jira-loop/functional.md` (capacidad canónica establecida).

## Cierre

Fase 1 COMPLETA. Desbloquea Fase 2 (worktrees + 2 devs en paralelo sobre módulos no solapados).
