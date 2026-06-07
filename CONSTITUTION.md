# CONSTITUTION — Talos

> **Versión 1.0 · 2026-06-06**
> Las convenciones que el sistema entero asume y que son caras de cambiar después.
> Esta es la pieza más reutilizable del Kit. Cambiarla requiere aprobación de **ZEUS**.

Estado de las decisiones: **CONGELADAS** (gate HG0). Nada se construye sobre supuestos no escritos acá.

---

## 1. Reparto y cadena de autoridad

**TALOS** es el autómata completo. **ATHENA** lo comanda y delega; **ARGOS**, el de los cien ojos,
vigila todos los módulos; **ZEUS** es la autoridad final.

| Figura | Rol | Sigla | Modelo |
|---|---|---|---|
| **ATHENA** | Orquestador / PM — planifica y reparte | Agentic Task Handling, Execution & Networking Authority | Opus |
| **ATLAS** | Backend — bounded context #1 | Application-Tier Logic & API Service | Sonnet |
| **HEPHAESTUS** | Backend — bounded context #2 (la forja: motores, workers, integraciones) | Hosted Engine for Processing, Handlers & Async Execution Systems | Sonnet |
| **CRONOS** | Backend — bounded context #3 (núcleo de dominio / jobs / scheduling) | Core Runtime, Orchestration, Networking & Operations System | Sonnet |
| **IRIS** | Frontend — la capa visible | Interface Rendering & Interaction System | Sonnet |
| **GAIA** | Datos / base de datos — el cimiento | Global Archive & Index Authority | Sonnet |
| **THEMIS** | QA / testing — verifica que el código obedezca la spec | Test Harness, Evaluation Metrics & Inspection Suite | Sonnet |
| **HERMES** | DevOps / entrega — mueve y despliega entre entornos | Hosted Environments, Release Management & Egress System | Sonnet |
| **ARGOS** | Revisión adversarial (Judgment Day) — **NO es dev** | el de los cien ojos | jueces con routing propio |
| **ZEUS** | El TL humano — autoridad final por encima de ATHENA | — | humano |

**Cadena de autoridad:** ZEUS → ATHENA → devs. ARGOS es transversal (juzga, no implementa).
**Regla de escritura única:** un solo orquestador (ATHENA) y un solo agente escritor por módulo.
Ningún dev toca código fuera de su dominio sin pasar por ATHENA.

> Las cinco especializaciones son un **roster template**. Otros dominios se reasignan figura→rol
> (ej. **NEMESIS** seguridad, **HÉCATE** mobile/multiplataforma) por decisión de ZEUS.

---

## 2. Modelo de ownership

**Un módulo = un único agente escritor.** Todos leen, solo el dueño escribe. Esta es la regla que
previene la colisión en origen.

- La fuente de verdad del mapa módulo→agente es el blackboard `team-context/ownership.md`.
- **Regla dura de los 3 backend:** ATLAS, HEPHAESTUS y CRONOS **no comparten módulo**. El backend
  se descompone en **3 bounded contexts independientes**, corte **vertical por dominio**
  (auth, billing, catálogo…), **NUNCA por capa** (controller/service/repo de la misma feature).
  Si el backend no se parte en 3 contextos genuinamente independientes, **no se corren los 3 en
  paralelo → se serializa**.
- Los contextos concretos los descubre la fase `explore`; esta constitución fija el patrón y los slots.

---

## 3. Naming de ramas

```
agent/<figura>/<JIRA-KEY>
```

Ejemplos: `agent/atlas/TAL-142`, `agent/iris/TAL-150`. Trazable a Jira y al worktree.
El nombre de figura va en minúscula. La rama es la **segunda fuente de verdad** de la atribución
(ver §5).

---

## 4. Esquema de labels de Jira

Como la identidad de agente NO se expresa por assignee (ver §5), **identidad, ownership y detección
de colisión corren sobre labels**.

| Namespace | Ejemplo | Significado | Lo setea |
|---|---|---|---|
| `agent:*` | `agent:atlas` | QUIÉN hizo el trabajo (reemplaza assignee) | ATHENA al crear el issue |
| `module:*` | `module:auth` | QUÉ dominio toca — clave de colisión | ATHENA desde `ownership.md` |
| `change:*` | `change:jira-evidence-loop` | agrupa los issues de un change SDD | ATHENA al crear |
| `phase:*` | `phase:apply` | fase SDD (opcional, para dashboards) | el helper al transicionar |

**Invariante (enforced por ATHENA y CI):** exactamente **un** `agent:*` y **un** `module:*` por
issue de dev; el par `(module:X, agent:Y)` debe cumplir `ownership[X] == Y`, si no se rechaza.

**Detección de colisión por JQL** (`module:*` + `agent:*`). El solape a nivel **archivo** no es
expresable en JQL — se resuelve en el blackboard: los agentes declaran archivos tocados en el body
del issue y ATHENA los cruza entre issues que comparten `module:*`.

---

## 5. Identidad de agente

- **Una sola cuenta de Jira compartida** (la de ZEUS / John). Ambos servidores MCP actúan como ella;
  `currentUser()` **no** distingue agentes.
- La identidad de cada agente se expresa por el **label `agent:*`** + la **rama** `agent/<figura>/…`.
- Costo asumido: el audit log muestra siempre al mismo actor. Mitigación: el invariante de §4 + la
  rama como corroboración. (Migrar a cuentas por-agente es una decisión futura de ZEUS, no de Fase 0–2.)

---

## 6. Definition of Done (DoD)

**DoD = PR linkeado + CI verde + resumen de `verify-report`.** El DoD vive en **evidencia**, no en
estado de Jira a secas.

Regla de evidencia: **linkeá lo vivo, adjuntá lo muerto.**

- **Linkear** (URL-addressable, vive en git/GitHub): PR, run de CI, commit, branch → como *remote
  issue link*.
- **Adjuntar** (evidencia puntual que debe sobrevivir aunque se borre el repo): `verify-report`,
  logs de tests fallidos, coverage, capturas → como *attachment*.

---

## 7. Routing MCP (híbrido)

Dos servidores Atlassian, ambos autenticados contra `tablex.atlassian.net`. Cada llamada tiene su ruta.

| Server | Namespace | Para qué |
|---|---|---|
| **Rovo oficial** | `mcp__claude_ai_Atlassian__*` | Ciclo de vida del issue: create / edit / transition / comment / worklog / búsqueda JQL |
| **Community** | `mcp__atlassian__*` | Lo que Rovo NO puede: **adjuntos** (`jira_update_issue`) + **remote issue links** (`jira_create_remote_issue_link`) |

Mantener el server community en el **set mínimo** de llamadas (adjuntos + remote links). El helper
**falla fuerte** si un adjunto errorea (no se traga el error).

**Límites heredados:** ningún MCP crea proyectos Jira (el proyecto se crea manual). Proyectos
team-managed → leer transiciones con `getTransitionsForJiraIssue`, **nunca** hardcodear IDs.

---

## 8. Model routing

| Fase / rol | Modelo | Por qué |
|---|---|---|
| ATHENA (orquestación, propose, design) | Opus | Decisiones arquitectónicas |
| Devs (apply, spec, tasks, verify) | Sonnet | Implementación / escritura estructurada |
| `explore` | modelo barato | Lectura estructural, no arquitectónica |
| ARGOS (jueces + fix-agent) | routing propio de Judgment Day | Revisión adversarial con contexto fresco |

> En Claude Code el routing es **single-mode** (multi-mode por fase es exclusivo de OpenCode).
> El modelo se pasa por agente en cada invocación.

---

## 9. Store de artefactos SDD

**Híbrido:** `openspec/` committeable (trail versionado, lo que el Kit necesita) + **Engram** para
recovery entre sesiones/compactaciones.

- Artefactos en `openspec/changes/<change-name>/`: `proposal.md`, `specs/`, `design.md`, `tasks.md`,
  `verify-report.md`; al cerrar → `openspec/changes/archive/`.
- **NO** se crea `docs/sdd/` hasta Fase 5: su sola existencia forzaría a engram-only y rompería el
  híbrido (regla `doc-zones.md`).

---

## 10. Gates humanos (ZEUS)

| Gate | Cuándo | Qué aprueba |
|---|---|---|
| **HG0** | Fase 0 | `CONSTITUTION.md` congelado (este documento) |
| **HG1** | pre-Fase 1 | Proyecto Jira **TAL** creado manualmente en la UI |
| **HG2** | tras `tasks`, en cada change | Revisar `tasks.md` antes de que `apply` escriba código |
| **HG3** | en cada change | Aprobar el merge final a `main` |
| **HG4** | fin Fase 1 | Primer change archivado: loop + evidencia válidos antes de escalar |
| **HG5** | Fase 3+ | ARGOS / Judgment Day **APPROVED** (gate duro pre-`sdd-archive`) |
| **HG6** | pre-Fase 4 | OK a escalar 2→5+ agentes tras revisar costo + métricas de colisión |
| **HG7** | Fase 5 | Extracción del Kit + smoke test del `init` en un segundo proyecto |

---

## 11. Concurrencia y merge

- **Worktree por agente + rama por agente** sobre UN solo monorepo (NO clones). `isolation: worktree`.
- Los worktrees aíslan **filesystem + branch**, NO procesos/puertos/DB. Aislamiento real = **`.env`
  por worktree** (puertos disjuntos + schema de DB por agente).
- **Merge ordenado**: de a un worktree contra `main`, o rebase-sobre-`main`-antes-del-PR. El worktree
  sin cambios se limpia solo.
- **Dos issues sobre el mismo archivo → nunca en paralelo.** Se serializa o se re-segmenta.
- **Métrica de salud:** tasa de conflictos de merge **> ~15% = segmentación mala** → re-segmentar
  antes de sumar más agentes.
- **Fallo mid-task:** transición de vuelta a To Do + comentario del error; el worktree se descarta.

---

## 12. Revisión adversarial (ARGOS / Judgment Day)

Desde Fase 3, **obligatoria**: dos jueces ciegos + un fix-agent, con contexto fresco y distinto del
implementor. Veredicto **APPROVED** es **gate duro** antes de `sdd-archive` (`require_judgment_day: true`).
Si los jueces se contradicen o escala, **decide ZEUS** (no hay resolución automática).

---

## 13. No-goals tempranos (Fase 0–2)

Cuentas Jira por-agente · aislamiento de proceso/DB/puertos más allá de `.env`-por-worktree ·
extracción del Kit / `init` · herramienta visual Tier-2 · `docs/sdd/` · detección de colisión a nivel
archivo *dentro de Jira* · resolución automática de contradicción entre jueces.
