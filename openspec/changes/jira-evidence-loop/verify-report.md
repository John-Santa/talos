# Verify Report — jira-evidence-loop · PR1 (domain + service)

**Veredicto: PR1 LISTO.** 0 CRITICAL · 1 WARNING (cerrada) · 2 SUGGESTIONS.
Branch: `agent/hermes/TAL-1-domain-service` → `develop`. Issue: TAL-1.

## Tests
- `go test ./... -count=1` → **32 PASS, 0 fallos** (domain/evidence 17+4 · service 11).
- `go vet ./...` → limpio.

## Cobertura de requisitos (scope PR1)
Satisfechos: REQ-CFG-1/2/3 · REQ-LABEL-1/2/3 · REQ-IDEM-1/2/3/4 · REQ-TRANS-3 · REQ-ERR-1/2/3 · REQ-TEST-1/2/3/4.
Diferido a PR2 (no es falla): REQ-AUTH-3 (header HTTP), REQ-ATTACH multipart, ADF wire format, transitions REST, CLI, smoke de integración.

## Integridad hexagonal
- `domain/evidence`: importa solo `fmt`. `service`: depende solo de `port` (cero refs a adapter). `go.mod`: **cero deps transitivas**. ✅

## OrderedStates (valores reales TAL)
`new:["Por hacer"]` · `indeterminate:["En curso","En revisión","Bloqueado"]` · `done:["Listo"]`. ✅

## Hallazgos
- **WARNING (CERRADA):** la rama de desempate multi-candidato de transición no tenía test → se agregaron 4 tests (`TestDisambiguation_*`); lógica confirmada correcta, sin bug. Commit `da38b3c`.
- **SUGGESTION-1:** `mock` usa un check de interfaz anónimo en vez de `port.JiraClient` directo (trade-off para mantener `mock` libre de dep a `port`). Documentado.
- **SUGGESTION-2:** `.env.example` (resuelto — renombrado del `env.example` inicial).

## Loop de evidencia (manual, ATHENA — dogfooding Fase 1)
- Paso 1 create + labels ✅ (TAL-1, labels con dos puntos OK)
- Paso 2 → En curso ✅ (transición 21)
- Pasos 4-7 (PR link, adjunto, comentario, → En revisión): este PR.
