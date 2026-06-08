# Design: ci-hardening

**Change:** ci-hardening · **Jira:** TAL-5 · **Module:** module:devops · **Owner:** HERMES
**Store:** hybrid (this file + engram `sdd/ci-hardening/design`)
**Reads:** proposal (#1880, 8 resolved decisions) + explore (#1879, 4 enfoques lockeados con evidencia file:line)
**Module path:** `github.com/John-Santa/talos/platform/ci-checks`
**Mirror reference:** `platform/overlap-guard/` + `platform/merge-order-orchestrator/` (hexagonal, cero deps, mock hand-written, `run(args) error` como composition root)

Este documento es el **CÓMO** a nivel arquitectónico. No enumera pasos de tarea — `sdd-tasks`
convierte estos límites en un backlog ordenado por strict-TDD. Los requerimientos funcionales (el
QUÉ) los posee `sdd-spec`. Las firmas de abajo se verificaron compile-plausibles contra el código
real citado: `evidence/issue.go:11-64`, `evidence/ownership.go:5-63`, `overlap-guard/adapter/jirarest/{client.go,auth.go}`,
`overlap/parse.go`, `merge-order/cmd/mo/main.go`, `merge-order/service/config.go`,
`merge-order/port/git_inspector.go`, `overlap-guard/mock/issue_searcher_mock.go`.

Este design FORMALIZA el alcance ya aprobado por ZEUS (D1/D2 + las 8 decisiones resueltas del
proposal). Las forks resueltas se registran como ADRs en §13.

> **Convención de comentarios (heredada — apply DEBE honrarla):** código auto-explicativo, SIN
> comentarios salvo **una única línea godoc** en símbolos **exportados**. Cada tipo/func exportado
> de las firmas de abajo lleva exactamente una línea godoc; nada más. Sin comentarios inline, sin
> bloques de sección decorativos.

---

## 1. Architecture approach

Hexagonal (ports & adapters), el **mismo esqueleto que `ov` y `mo`**: la flecha de dependencia
apunta hacia adentro — `cmd → service → (domain + port) ← adapter` — y el núcleo `domain/cichecks`
tiene **cero I/O y cero imports de terceros** (stdlib only, cero `require`, `go 1.26`).

A diferencia de `mo` (que tenía un split read/write de puertos como invariante central), acá el
centro arquitectónico es **la pureza del invariante §4**: las 4 sub-reglas (a)(b)(c)(d) viven en
`domain/cichecks` como funciones puras, exhaustivamente unit-testeadas SIN red ni filesystem. El
`service` solo orquesta (parsea rama → fetch labels → lee ownership → compone el invariante puro →
traduce `Violations` a error tipado). Los adapters son delgados (HTTP GET a Jira / `os.ReadFile` +
parse) y solo su comportamiento contra Jira/FS reales merece un test de integración
`testing.Short()`-gated.

`ch` es **SOLO el motor del invariante de labels**. Correr tests (`go test`) es trabajo del
YAML/runner, no de `ch` — esto evita el "test-corriendo-tests" (D2, ADR-C2).

```
            ┌─────────────────────────────────────────────────────┐
            │                domain/cichecks                      │  puro, sin deps
            │  Label · LabelSet · ParseLabels · Validate          │  value objects +
            │  ValidateOwnership · ErrOwnershipViolation          │  el invariante §4
            │  ParseAgentBranch · ParseOwnershipTable             │  (a)(b)(c)(d) PURO
            │  CheckLabelInvariant(InvariantInput)→InvariantResult │  (NO http, NO os)
            │  ErrNoJiraKey · ErrLabelInvariant · ErrMalformedOwnership │
            └─────────────────────────────────────────────────────┘
                        ▲                            ▲
              imports   │                            │   imports
        ┌───────────────┴────────┐         ┌─────────┴──────────────────┐
        │ service/Checker        │         │ adapter/jirarest           │
        │ depends on             │         │  Client → IssueLabelReader │
        │  IssueLabelReader +    │         │ adapter/ownershipfile      │
        │  OwnershipReader       │         │  Reader  → OwnershipReader │
        │  + Config              │         │ IMPLEMENTAN los ports      │
        └───────────┬────────────┘         └────────────┬───────────────┘
                    │ depends on                        │ satisfacen
                    ▼                                    │
        ┌───────────────────────────────────────────────────────────────┐
        │  port/  IssueLabelReader (read) · OwnershipReader (read)        │
        │         — el límite mockeable, capability-named, context-first  │
        └───────────────────────────────────────────────────────────────┘
                    ▲                                    ▲
            mock/ ──┘ (hand-written, 2 mocks)   cmd/ch ──┘ (composition root)
```

**Boundary rules:**
- `service` importa `port` + `domain`, nunca `adapter/*`.
- `adapter/jirarest` y `adapter/ownershipfile` importan `port` + `domain` (para satisfacer la
  interfaz), nunca `service`.
- `cmd/ch` es el ÚNICO lugar donde se construyen y cablean los adapters concretos y se leen las
  credenciales del entorno — el composition root.
- Los dos puertos son **read-only** (no hay path de mutación; `ch` no escribe nada en Jira ni en FS).

**Dónde está el valor (honesto):** el invariante §4 puro (`CheckLabelInvariant` + las 4 sub-reglas)
es el corazón unit-testeado. El `service` es secuenciación + traducción de error tipado, testeado
contra los 2 mocks. Los adapters son delgados y su comportamiento contra Jira/FS reales se cubre con
integración `Short`-gated.

---

## 2. Module naming

| Decisión | Valor | Justificación |
|---|---|---|
| Directorio | `platform/ci-checks/` | Nombrado por **lo que ES** (checks de CI), como `overlap-guard`/`worktree-orchestrator`, **no por el change**. Sobrevive al change que lo creó. |
| Módulo Go | `github.com/John-Santa/talos/platform/ci-checks` | Mismo prefijo que los 3 siblings; `go.mod` propio (cero `require`). |
| Binario | `ch` | 2 letras como `wt`/`mo`/`ov`. |
| Package dominio | `cichecks` | Sin guion (identificador Go válido); el package es el sustantivo del dominio. |
| Go version | `go 1.26` | Igual que los siblings; stdlib only. |

`go.mod` espeja `overlap-guard/go.mod`: una sola línea `module …/ci-checks`, `go 1.26`, **sin bloque
`require`**.

---

## 3. Package layout

Cada archivo nombrado con su responsabilidad y su sibling-fuente. Tests `*_test.go` por archivo
(omitidos del árbol salvo donde aclara el RED→GREEN).

```
platform/ci-checks/
  go.mod                                  ← overlap-guard/go.mod (module + go 1.26, sin require)
  cmd/ch/
    main.go                               ← cmd/mo/main.go (dispatch, exitCodeFor, repoRoot, --json structs, env reads)
    main_test.go                          run([])/run(["bogus"])/labels-sin-branch → error; exitCodeFor table; --json shape
  domain/cichecks/
    label.go        ParseLabels + Label + LabelSet + Validate   ← evidence/issue.go:11-64 (reimpl)
    ownership.go    ValidateOwnership + ErrOwnershipViolation     ← evidence/ownership.go:5-63 (reimpl verbatim)
    branch.go       ParseAgentBranch(branch)→(figura,key,err)     ← mo figuraFromBranch + regex TAL (NEW)
    parse.go        ParseOwnershipTable(md)→map[module]agent      ← overlap/parse.go line-scanner style (NEW)
    invariant.go    CheckLabelInvariant + InvariantInput + InvariantResult — compone (a)(b)(c)(d), PURO (NEW)
    errors.go       ErrNoJiraKey · ErrLabelInvariant · ErrMalformedOwnership (NEW)
    label_test.go · ownership_test.go · branch_test.go · parse_test.go · invariant_test.go
  port/
    issue_label_reader.go   IssueLabelReader { LabelsByKey(ctx, key) ([]string, error) }   ← port/git_inspector.go convención
    ownership_reader.go     OwnershipReader  { Ownership(ctx) (map[string]string, error) }
  adapter/jirarest/
    client.go       Client → IssueLabelReader: GET /rest/api/3/issue/{key}?fields=labels   ← overlap-guard/adapter/jirarest/client.go
    auth.go         setBasicAuth (verbatim)                                                ← overlap-guard/adapter/jirarest/auth.go
    client_test.go  httptest (path, fields, Basic, decode, 404, 401) + Integration (Short-gated)
  adapter/ownershipfile/
    reader.go       Reader → OwnershipReader: os.ReadFile(path) + cichecks.ParseOwnershipTable
    reader_test.go  contra testdata/ + Integration (Short-gated)
    testdata/ownership.md   fixture (réplica de team-context/ownership.md, con MAYÚSCULAS)
  service/
    config.go       Config + DefaultTALConfig()                                           ← merge-order/service/config.go
    checker.go      Checker.Check(ctx, branch) — parse → labels → ownership → invariante
    checker_test.go ambos ports mockeados: todos los paths + orden de llamadas
  mock/
    issue_label_reader_mock.go   IssueLabelReaderMock (call recording)                    ← overlap-guard/mock/issue_searcher_mock.go
    ownership_reader_mock.go      OwnershipReaderMock (call recording)
```

Cleanup fuera de `platform/ci-checks/` (todo HERMES-owned): `.github/workflows/pr-checks.yml` (NEW),
borrar `ci/pr-checks.yml` + dir `ci/`, `.gitignore` (append), `team-context/ownership.md` (fila),
opcional `team-context/ci.md`. Detalle en §11 y §12.

---

## 4. Ports

Dos puertos outbound read-only, **capability-named** (nombran la capacidad, no la tecnología) y
**context-first** (mismo patrón que `port/git_inspector.go:10`, donde cada método toma `ctx context.Context`
primero). El package `port` no importa `domain` (los puertos hablan en tipos stdlib: `[]string`,
`map[string]string`) — esto mantiene el adapter desacoplado del value-object y evita ciclos.

```go
// Package port defines the outbound ports for the ci-checks module.
package port

import "context"

// IssueLabelReader is the read-only outbound port for fetching a Jira issue's labels by key.
type IssueLabelReader interface {
	// LabelsByKey returns the raw label strings of the issue identified by key.
	LabelsByKey(ctx context.Context, key string) ([]string, error)
}

// OwnershipReader is the read-only outbound port for reading the module→agent ownership map.
type OwnershipReader interface {
	// Ownership returns the module→agent map parsed from the ownership source.
	Ownership(ctx context.Context) (map[string]string, error)
}
```

- `IssueLabelReader.LabelsByKey` devuelve `[]string` crudos (`["agent:hermes","module:devops"]`),
  no `LabelSet` — el parseo a `LabelSet` es dominio puro (`ParseLabels`), responsabilidad del
  `service`, no del adapter.
- `OwnershipReader.Ownership` devuelve ya el `map[module]agent` **lowercased** — el adapter delega a
  `cichecks.ParseOwnershipTable`, que normaliza (ver §5).

---

## 5. Domain model (puro, sin I/O)

Todo en `domain/cichecks`. Reimplementación (no import — `go.mod` separado, cero-dep; ADR-C6) de las
piezas verificadas, más las 3 piezas nuevas. Cada símbolo exportado con **una** línea godoc.

### 5.1 `label.go` — reimpl de `evidence/issue.go:11-64`

```go
// Label is a typed key:value pair attached to a Jira issue.
type Label struct {
	Key   string
	Value string
}

// LabelSet is an ordered collection of Labels.
type LabelSet []Label

// ParseLabels splits raw Jira label strings ("key:value") into a LabelSet; entries without a colon are kept with an empty Value.
func ParseLabels(raw []string) LabelSet

// Get returns the value for the first Label whose Key matches k, plus a found bool.
func (ls LabelSet) Get(k string) (string, bool)

// Validate checks that each key in requiredKeys appears exactly once in ls.
func (ls LabelSet) Validate(requiredKeys []string) error
```

`Validate` es **verbatim** de `issue.go:49-64` (cuenta por key; `n==0` → "required but missing";
`n>1` → "appears N times, must be exactly 1"). `ParseLabels` es nuevo: `strings.SplitN(s, ":", 2)`
sobre cada string crudo; preserva orden. `Get` se reusa de `issue.go:30-37` para que el service
extraiga `agent`/`module` después de validar.

### 5.2 `ownership.go` — reimpl verbatim de `evidence/ownership.go:5-63`

```go
// ErrOwnershipViolation is returned when the claimed agent does not own the specified module according to the ownership map.
type ErrOwnershipViolation struct {
	Module        string
	ClaimedAgent  string
	ExpectedAgent string
}

// Error implements error with a message distinguishing unregistered module from wrong owner.
func (e *ErrOwnershipViolation) Error() string

// ValidateOwnership checks that the given agent legitimately owns the given module according to the provided ownership map.
func ValidateOwnership(module, agent string, ownership map[string]string) error
```

Verbatim de `ownership.go:35-63`: `module`/`agent` no vacíos, `module` presente en el mapa,
`ownership[module] == agent`. **Es case-sensitive** (`ownership.go:55`: `if expected != agent`) — por
eso `ParseOwnershipTable` y `ParseAgentBranch` deben normalizar TODO a lowercase aguas arriba, así la
comparación final cae en el mismo caso. Esto es (a)(b)(c) del invariante §4 ya diseñado y testeado en
`ownership_test.go` (copiar casos).

### 5.3 `branch.go` — `ParseAgentBranch` (NEW)

```go
// ParseAgentBranch parses an agent branch name into its figura and Jira key; it returns ErrNoJiraKey when the branch does not match ^agent/([a-z]+)/(TAL-[0-9]+)$.
func ParseAgentBranch(branch string) (figura, key string, err error)
```

Regex `^agent/([a-z]+)/(TAL-[0-9]+)$` (CONSTITUTION §3; idéntico al bash de `branch-name`). Más
estricto que el `figuraFromBranch` de `mo` (`main.go:312-318`, que hace `SplitN`/3 tolerante): acá
una rama que no matchea exacto (`develop`, `agent/Hermes/TAL-7` con mayúscula, `agent/hermes/JIRA-7`)
devuelve `ErrNoJiraKey`. `figura` sale del grupo 1 (ya lowercase por `[a-z]+`); `key` del grupo 2.
Usa `regexp` (stdlib).

### 5.4 `parse.go` — `ParseOwnershipTable` (NEW, estilo `overlap/parse.go`)

```go
// ParseOwnershipTable parses the module→agent table from ownership.md markdown, lowercasing both module and agent; it returns ErrMalformedOwnership on a duplicate module.
func ParseOwnershipTable(md string) (map[string]string, error)
```

Line-scanner al estilo `overlap/parse.go:9-42` (`strings.Split(md,"\n")` + `TrimRight(line,"\r")` +
máquina de estados `inSection`):

- **Solo la PRIMERA tabla** (`## Mapa módulo → agente`, `ownership.md:8-18`). El archivo tiene una
  **segunda** tabla (`## Mapa archivo → módulo`, `ownership.md:36`) cuyas filas (`platform/x/** | HERMES | …`)
  NO son pares module→agente y **deben saltearse**. La máquina de estados cierra la sección al ver el
  segundo header `## ` (o el `>` blockquote / línea no-fila tras la primera tabla).
- Cada fila: `| `module:x` | AGENTE | … |`. Extraer col-1 (`module:*`, quitar backticks como
  `parse.go:78`) y col-2 (figura). **Lowercasear ambos** (`strings.ToLower`): `module:devops`/`hermes`.
  Saltear la fila separadora (`|---|---|`) y el header (`| `module:*` | Dueño …`).
- Clave del mapa: el `module:*` **completo** (`"module:devops"` → `"hermes"`), porque el label del
  issue es `module:devops` y el service compara contra eso sin re-prefijar. (Decisión de design: la
  clave es el valor de label crudo, no el sufijo, para que `ValidateOwnership(labelModule, …)` sea
  directo.)
- **Duplicado de module** → `ErrMalformedOwnership` (canario de `ch ownership`). Mapa vacío tras
  parsear (ninguna fila válida) → también `ErrMalformedOwnership`.

> **El hallazgo clave del explore:** `ownership.md:16` guarda la figura en MAYÚSCULA (`HERMES`) pero
> los labels son minúscula (`agent:hermes`). El lowercasing acá NO es cosmético — es lo que hace que
> la comparación case-sensitive de `ValidateOwnership` (§5.2) sea correcta. Es un concern de dominio
> nombrado y table-tested (`parse_test.go` assertea explícitamente `HERMES → hermes`).

### 5.5 `invariant.go` — `CheckLabelInvariant` (NEW, el centro)

```go
// InvariantInput carries everything CheckLabelInvariant needs: the branch-derived figura, the issue labels, and the ownership map.
type InvariantInput struct {
	BranchFigura string
	Labels       LabelSet
	Ownership    map[string]string
	RequiredKeys []string
}

// InvariantResult reports the outcome of the §4 label invariant: the resolved agent/module and any violations found.
type InvariantResult struct {
	Agent      string
	Module     string
	Violations []string
}

// CheckLabelInvariant evaluates the CONSTITUTION §4 label invariant — sub-rules (a)(b)(c)(d) — over input and returns the result; it returns a non-nil error only on a programming-level problem, never for a business violation.
func CheckLabelInvariant(input InvariantInput) (InvariantResult, error)
```

`CheckLabelInvariant` **devuelve `(InvariantResult{Violations}, nil)` aun con violaciones de negocio**
(patrón de `overlap-guard`): acumula, no corta. El `service` mapea `len(Violations) > 0 → ErrLabelInvariant
→ exit 1` (§7). El segundo retorno `error` queda reservado para fallos no-de-negocio (hoy: `nil`
siempre; el design lo deja en la firma para futura extensibilidad y por simetría con el resto del
dominio).

---

## 6. La descomposición del invariante §4

`CheckLabelInvariant` compone **4 sub-reglas puras**, cada una un test independiente en
`invariant_test.go`. Acumula en `Violations []string` (no corta en la primera) para que un PR
mal-formado vea TODAS sus violaciones de una.

| Sub-regla | Qué chequea | Cómo | Fuente |
|---|---|---|---|
| **(a)** | exactamente un `agent:*` | `input.Labels.Validate(["agent"])` | §5.1 / `issue.go:49` |
| **(b)** | exactamente un `module:*` | `input.Labels.Validate(["module"])` | §5.1 / `issue.go:49` |
| **(c)** | `ownership[module] == agent` | `ValidateOwnership(module, agent, input.Ownership)` | §5.2 / `ownership.go:35` |
| **(d)** | `branchFigura == labelAgent` | comparación directa de strings (ambos lowercase) | NEW — §3 × §4, realiza §5 |

Flujo interno de `CheckLabelInvariant`:

1. (a)+(b) juntas: `Validate(RequiredKeys)` con `["agent","module"]`. Si falla → append a `Violations`
   el mensaje (missing/duplicate). Si falta `agent` o `module`, las reglas (c)(d) que dependen de
   ellos se **saltean** (no hay valor que comparar) pero NO se corta el resto.
2. Extraer `agent, _ := Labels.Get("agent")` y `module, _ := Labels.Get("module")` → poblar
   `result.Agent`/`result.Module`.
3. (c): si `module` y `agent` presentes → `ValidateOwnership(module, agent, Ownership)`; error →
   append.
4. (d): si `agent` presente → `if BranchFigura != agent` → append violación `branch≠label`
   (`"branch figura %q does not match label agent %q"`).
5. Return `result, nil`.

Casos de `invariant_test.go` (table-driven, el centro de la suite): OK · 0 agent · 2 agent · 0 module
· 2 module · ownership-mismatch (módulo conocido, agente equivocado) · módulo-desconocido (no en mapa)
· branch≠label · violaciones múltiples (varias acumuladas).

---

## 7. Service

`service/checker.go` orquesta; no contiene lógica de negocio (esa está en `domain`). Espeja la
construcción de `mo` (`Config` + `DefaultTALConfig()` + servicio que toma los ports).

```go
// Config holds the runtime configuration for the ci-checks services.
type Config struct {
	Project       string
	OwnershipPath string
	RequiredKeys  []string
}

// DefaultTALConfig returns a Config seeded with the Talos platform defaults.
func DefaultTALConfig() Config {
	return Config{
		Project:       "TAL",
		OwnershipPath: "team-context/ownership.md",
		RequiredKeys:  []string{"agent", "module"},
	}
}

// Checker evaluates the §4 label invariant for a given agent branch against Jira and the ownership map.
type Checker struct {
	labels    port.IssueLabelReader
	ownership port.OwnershipReader
	cfg       Config
}

// NewChecker wires a Checker with its label reader, ownership reader, and config.
func NewChecker(labels port.IssueLabelReader, ownership port.OwnershipReader, cfg Config) *Checker

// Check parses the branch, fetches the issue labels and ownership map, evaluates the §4 invariant, and returns the result or a typed error.
func (c *Checker) Check(ctx context.Context, branch string) (cichecks.InvariantResult, error)
```

**Flujo de `Check`** (orden testeado con `AssertMethodOrder`/call-recording):

1. `figura, key, err := cichecks.ParseAgentBranch(branch)` → si `err` (`ErrNoJiraKey`) **cortar acá**,
   ANTES de tocar Jira (`AssertNotCalled(LabelsByKey)`). Exit 1.
2. `raw, err := c.labels.LabelsByKey(ctx, key)` → error de Jira (404/HTTP) burbujea tal cual.
3. `own, err := c.ownership.Ownership(ctx)` → error (`ErrMalformedOwnership`) burbujea.
4. `set := cichecks.ParseLabels(raw)`.
5. `res, _ := cichecks.CheckLabelInvariant(InvariantInput{BranchFigura: figura, Labels: set, Ownership: own, RequiredKeys: c.cfg.RequiredKeys})`.
6. `if len(res.Violations) > 0 { return res, &cichecks.ErrLabelInvariant{Violations: res.Violations} }` → exit 1.
7. `return res, nil` → exit 0.

**Credenciales NO van en `Config`** (ADR-C8): `Config` es declarativo/serializable; las creds Jira
(`JIRA_SITE_URL`/`JIRA_EMAIL`/`JIRA_API_TOKEN`) se leen en `cmd/ch` y se pasan a `jirarest.NewClient(...)`.
Los adapters toman args explícitos, igual que `overlap-guard`.

---

## 8. Adapters

### 8.1 `adapter/jirarest` — espeja `overlap-guard/adapter/jirarest`

```go
// HTTPError is returned on any non-2xx Jira API response.
type HTTPError struct {
	StatusCode int
	Body       string
	Op         string
}

// Client implements port.IssueLabelReader against the Jira REST API v3.
type Client struct {
	http    *http.Client
	baseURL string
	email   string
	token   string
}

// NewClient constructs a Client targeting siteURL and authenticating via Basic auth.
func NewClient(siteURL, email, token string) *Client

// LabelsByKey implements port.IssueLabelReader via GET /rest/api/3/issue/{key}?fields=labels.
func (c *Client) LabelsByKey(ctx context.Context, key string) ([]string, error)
```

- `NewClient`/`HTTPError`/`http.Client{Timeout: 30 * time.Second}` **verbatim** de `client.go:18-46`.
- `auth.go` (`setBasicAuth`) **verbatim** de `auth.go:8-11` (Basic, `base64.StdEncoding`).
- **`GET /rest/api/3/issue/{key}?fields=labels`** (NO el `POST .../search` de `ov`; ADR-C3). Decode
  más simple que el de `ov` (sin `issues[]`, sin ADF):

  ```go
  // issueLabelResponse is the minimal decode shape for GET issue ?fields=labels.
  type issueLabelResponse struct {
  	Key    string `json:"key"`
  	Fields struct {
  		Labels []string `json:"labels"`
  	} `json:"fields"`
  }
  ```

- Construcción del request: `http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/rest/api/3/issue/"+key+"?fields=labels", nil)`,
  headers `Accept: application/json` + `setBasicAuth`.
- **404 → `cichecks.ErrIssueNotFound`** (issue no existe = violación real, exit 1; el §4 dice que
  ATHENA labela al crear, ausencia = violación). 401/5xx/cualquier no-2xx → `*HTTPError`
  (`client.go:100-103` pattern). El service NO distingue: ambos burbujean → exit 1.
- `var _ port.IssueLabelReader = (*Client)(nil)` (assertion estática, como `client.go:36`).

> **Decisión de design:** `ErrIssueNotFound` vive en `domain/cichecks/errors.go` (no en el adapter),
> para que el adapter dependa del dominio (capa interior) y no al revés. El adapter mapea
> `resp.StatusCode == 404 → cichecks.ErrIssueNotFound`.

- **Tests:** `httptest.Server` para path (`/rest/api/3/issue/TAL-7`), querystring (`fields=labels`),
  Basic auth header, decode feliz, 404→`ErrIssueNotFound`, 401→`*HTTPError`. Un test de
  **integración** contra Jira real **`testing.Short()`-gated** (skip si `-short`; corre solo con creds).

### 8.2 `adapter/ownershipfile` — leer-de-path (NO `go:embed`; ADR-C5)

```go
// Reader implements port.OwnershipReader by reading and parsing ownership.md from disk.
type Reader struct {
	path string
}

// NewReader constructs a Reader bound to the given ownership.md path.
func NewReader(path string) *Reader

// Ownership implements port.OwnershipReader: it reads the file and delegates to cichecks.ParseOwnershipTable.
func (r *Reader) Ownership(ctx context.Context) (map[string]string, error)
```

- `os.ReadFile(r.path)` → `cichecks.ParseOwnershipTable(string(data))`. Si el archivo no existe →
  el error de `os.ReadFile` burbujea (exit 1). Si está malformado → `ErrMalformedOwnership` (de
  `ParseOwnershipTable`).
- Default `repoRoot/team-context/ownership.md` (resuelto en `cmd/ch`); `--ownership-file` override.
- **NO `go:embed`** (ADR-C5): CI corre en repo checkouteado → el archivo VIVO está presente;
  embeber bakearía un snapshot stale de la fuente-de-verdad §2.
- `var _ port.OwnershipReader = (*Reader)(nil)`.
- **Tests:** contra `testdata/ownership.md` (fixture con MAYÚSCULAS, réplica del real) → assert
  lowercasing + skip de la 2ª tabla + duplicado→error. Test de integración contra el `ownership.md`
  real del repo **`Short`-gated**.

---

## 9. cmd/ch (composition root)

Espeja `cmd/mo/main.go` (`main_test.go` testea `run`). Estructura:

```go
func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "ch: %v\n", err)
		os.Exit(exitCodeFor(err))
	}
}

// run dispatches to the labels or ownership subcommand.
func run(args []string) error
```

- **Dispatch** (`run`, espeja `mo/main.go:36-50`): `args[0]` → `labels` | `ownership`; vacío o
  desconocido → error con uso.
- **`exitCodeFor`** (espeja `mo/main.go:52-61`): `nil → 0`. TODOS los errores de negocio del módulo
  (`ErrNoJiraKey`, `ErrLabelInvariant`, `ErrMalformedOwnership`, `ErrIssueNotFound`, `*HTTPError`) →
  **1**. NO hay código "skip" (3 del plan §45): `branch-name` corre primero con `needs:` y garantiza
  que la key existe; un fallo acá ES una violación.
- **`repoRoot()`** (verbatim de `mo/main.go:63-70`): `git rev-parse --show-toplevel`, fallback `os.Getwd()`.
- **Parsing por subcomando** con `flag.NewFlagSet(..., flag.ContinueOnError)` (como `mo`):
  - `ch labels --branch <b> [--ownership-file p] [--site-url u] [--json]` → requiere `--branch`
    (ausente → error). Lee creds del entorno, construye `jirarest.NewClient` + `ownershipfile.NewReader`
    + `service.NewChecker`, llama `Check(ctx, branch)`.
  - `ch ownership [--ownership-file p] [--json]` → canario **offline** (sin Jira): solo
    `ownershipfile.NewReader(...).Ownership(ctx)`; mapa OK → exit 0, malformado → exit 1. La única
    lógica sin red.
- **Env reads** (en `cmd`, no en adapter): `JIRA_SITE_URL` (override con `--site-url`), `JIRA_EMAIL`,
  `JIRA_API_TOKEN`. Si faltan en `labels` → error claro antes de pegarle a Jira.
- **`--json` structs** (espeja `planJSON`/`planStepJSON` de `mo/main.go:135-151`):

  ```go
  // labelsJSON is the --json output shape for ch labels.
  type labelsJSON struct {
  	Branch     string   `json:"branch"`
  	JiraKey    string   `json:"jira_key"`
  	Figura     string   `json:"figura"`
  	Verdict    string   `json:"verdict"`            // "OK" | "VIOLATION"
  	Labels     []string `json:"labels"`
  	Agent      string   `json:"agent,omitempty"`
  	Module     string   `json:"module,omitempty"`
  	Violations []string `json:"violations,omitempty"`
  }

  // ownershipJSON is the --json output shape for ch ownership.
  type ownershipJSON struct {
  	Map   map[string]string `json:"map"`
  	Valid bool              `json:"valid"`
  }
  ```

  `enc.SetIndent("", "  ")` como `mo/main.go:213-214`. Sin `--json`: salida tabular legible.

---

## 10. Mocks

Dos mocks hand-written en `mock/`, espejando `overlap-guard/mock/issue_searcher_mock.go:11-98`
(call-recording con `Call{Method, Args}` + helpers de aserción). Reusan el tipo `Call` y `ErrSentinel`
en un solo archivo compartido del package `mock`.

```go
// Call records a single invocation of a mock method.
type Call struct {
	Method string
	Args   []any
}

// IssueLabelReaderMock is a hand-written test double implementing port.IssueLabelReader.
type IssueLabelReaderMock struct {
	Calls         []Call
	LabelsByKeyResults map[string][]string
	LabelsByKeyErrs    map[string]error
	DefaultResult []string
	DefaultErr    error
}

// OwnershipReaderMock is a hand-written test double implementing port.OwnershipReader.
type OwnershipReaderMock struct {
	Calls       []Call
	OwnershipMap map[string]string
	OwnershipErr error
}
```

Helpers (espejan `issue_searcher_mock.go:44-76`, más `AssertMethodOrder` requerido para testear el
orden branch→labels→ownership):

- `record(method, args...)` — append a `Calls`.
- `CallsFor(method) []Call`.
- `AssertCallCount(t, method, n)`.
- `AssertNotCalled(t, method)` — clave para testear que `ErrNoJiraKey` corta antes de Jira.
- `AssertMethodOrder(t, methods ...string)` — assert que la secuencia de `Calls` matchea el orden
  esperado (nuevo helper, no en el sibling; trivial sobre `Calls`).
- `var _ port.IssueLabelReader = (*IssueLabelReaderMock)(nil)` y la análoga para `OwnershipReaderMock`.

Programables por clave (`LabelsByKeyResults[key]`) + default, igual que el sibling.

---

## 11. Migración del YAML → `.github/workflows/pr-checks.yml`

Crear `.github/workflows/pr-checks.yml` (`.github/` no existe en el repo). 3 jobs
`on: pull_request: branches: [develop]`.

### Job `branch-name` (verbatim)
Portar el bash existente de `ci/pr-checks.yml:16-27` **sin cambios**: regex
`^agent/[a-z]+/TAL-[0-9]+$` sobre `$GITHUB_HEAD_REF`, `::error::` + `exit 1`. Gate barato, corre primero.

### Job `labels` (`needs: branch-name`)
```yaml
labels:
  needs: branch-name
  runs-on: ubuntu-latest
  steps:
    - uses: actions/checkout@v4
    - uses: actions/setup-go@v5
      with: { go-version: '1.26' }
    - name: Build ch
      working-directory: platform/ci-checks
      run: go build -o /tmp/ch ./cmd/ch
    - name: Enforce §4 label invariant
      run: /tmp/ch labels --branch "$GITHUB_HEAD_REF" --json
      env:
        JIRA_SITE_URL: ${{ secrets.JIRA_SITE_URL }}
        JIRA_EMAIL: ${{ secrets.JIRA_EMAIL }}
        JIRA_API_TOKEN: ${{ secrets.JIRA_API_TOKEN }}
```
El último step corre **desde repo-root** (no `working-directory`) para que `ch` resuelva
`team-context/ownership.md` vía `repoRoot()`. Secrets GitHub = las env vars que lee `cmd/ch`.

### Job `dod` (`needs: branch-name`)
```yaml
dod:
  needs: branch-name
  runs-on: ubuntu-latest
  steps:
    - uses: actions/checkout@v4
      with: { fetch-depth: 0 }
    - uses: actions/setup-go@v5
      with: { go-version: '1.26' }
    - name: Test changed modules
      run: |
        BASE="origin/${GITHUB_BASE_REF}"
        CHANGED=$(git diff --name-only "$BASE"...HEAD | grep '^platform/' || true)
        MODULES=$(echo "$CHANGED" | sed -E 's#(platform/[^/]+)/.*#\1#' | sort -u)
        for m in $MODULES; do
          if [ -f "$m/go.mod" ]; then
            echo "::group::go test -short $m"
            (cd "$m" && go test -short ./...)
            echo "::endgroup::"
          fi
        done
    - name: Frontend tests (placeholder)
      run: echo "vitest placeholder — no hay frontend todavía (deferral documentado)"
    - name: Assert verify-report present
      run: |
        ls openspec/changes/**/verify-report.md >/dev/null 2>&1 \
          || { echo "::error::no verify-report.md found"; exit 1; }
```
`fetch-depth: 0` para que el merge-base contra `$GITHUB_BASE_REF` exista. La lógica de correr tests
vive en el **YAML/runner** (D2/ADR-C2), NO en `ch`. vitest = placeholder no-bloqueante (deferral).
verify-report: assert **presencia** (glob), no lo genera.

> **`actionlint` es advisory** (ADR-C2 caveat): el YAML no es Go-testeable; la superficie no-testeada
> se reduce por diseño a "checkout, build, invocar binario, pasar env". Mitigación: `actionlint`
> advisory + dog-food del propio PR2.

---

## 12. Cleanup in-scope (todo HERMES-owned, sin cross-module write)

1. **`.gitignore`**: append `/platform/ci-checks/ch` (binario compilado; patrón de los 4 siblings en
   `.gitignore:35-38`).
2. **`team-context/ownership.md`**: agregar a la 2ª tabla (archivo→módulo) la fila
   `| platform/ci-checks/** | HERMES | CLI ch, invariante §4 |`. (Esta fila vive en la tabla que
   `ParseOwnershipTable` **saltea** — no afecta el parseo.)
3. **`ci/` → `.github/`**: borrar `ci/pr-checks.yml` + el dir `ci/` (era staging documentado sin
   pointer-stub; ADR-C7); crear `.github/workflows/pr-checks.yml`.
4. **(Opcional) `team-context/ci.md`**: doc de los 3 jobs + los 3 secrets + el contrato de detección
   del `dod`. No-bloqueante.

**Seams con spec:** `sdd-spec` posee el QUÉ (los requerimientos funcionales: exit codes, el contrato
del invariante, el shape del `--json`). Este design posee el CÓMO (los límites de tipo). El punto de
encuentro: cada sub-regla §4 (a)(b)(c)(d) es un requerimiento funcional que mapea 1:1 a un test de
`invariant_test.go`.

---

## 13. ADRs

| # | Decisión | Resolución | Por qué |
|---|---|---|---|
| **ADR-C1** | Motor del invariante de labels | **CLI Go `ch`** (`platform/ci-checks/`, hexagonal, cero-deps), no bash, no extender `ov` | Bash no es testeable bajo strict TDD. `ov` es de THEMIS (`module:qa`) → extenderlo violaría §2 (escritura única). El YAML solo invoca `ch labels`. |
| **ADR-C2** | Alcance del gate `dod` | Correr `go test -short ./...` por módulo cambiado vive en el **YAML/runner**, NO en `ch` | Correr tests es trabajo de CI; mantener `ch` como motor de labels evita el "test-corriendo-tests". El YAML no es Go-testeable → `actionlint` advisory + dog-food. |
| **ADR-C3** | Endpoint Jira | **`GET /rest/api/3/issue/{key}?fields=labels`**, NO el `POST .../search` de `ov` | "Labels de un issue por key" → GET es el fit exacto: decode más simple (sin `issues[]`, sin ADF), 404 distinguible (`ErrIssueNotFound`). |
| **ADR-C4** | Invariante branch-figura == label-agent | **Incluir como sub-regla (d) fuerte** del invariante | Cruza §3 (rama) con §4 (label) y realiza §5 ("la rama como corroboración"); puro, trivial de testear, atrapa el PR mal-labelado vs su rama. |
| **ADR-C5** | Fuente de `ownership.md` | **Leer-de-path** (default `repoRoot/team-context/ownership.md`), NO `go:embed` | CI corre en repo checkouteado → el archivo VIVO está presente; embeber bakearía un snapshot stale de la fuente-de-verdad §2. `--ownership-file` override para tests. |
| **ADR-C6** | Reúso de dominio | **Reimplementar verbatim** (no import) `LabelSet.Validate` + `ValidateOwnership`/`ErrOwnershipViolation` | No se puede importar — `go.mod` separado, **cero-dep** (stdlib only, cero `require`). (a)(b)(c) ya están diseñados/testeados; (c) es **case-sensitive** → el parser lowercasea figura y módulo. |
| **ADR-C7** | Migración del dir `ci/` | **Borrar** `ci/pr-checks.yml` + dir `ci/`, crear `.github/workflows/pr-checks.yml` desde cero | `ci/` era staging documentado sin pointer-stub; `.github/` no existe → movimiento limpio a la ubicación canónica de Actions. |
| **ADR-C8** | Secrets / creds | `JIRA_SITE_URL`/`JIRA_EMAIL`/`JIRA_API_TOKEN` leídos en `cmd/ch`, NO en `Config` ni en el adapter | Mismas creds Basic-auth que `ov`; `Config` es declarativo/serializable; los adapters toman args explícitos (`NewClient(siteURL,email,token)`). |

---

**Próximo:** `sdd-tasks` (lee spec + design) — convierte estos límites en un backlog ordenado por
strict-TDD (RED→GREEN por capa: domain → service → adapter → cmd → YAML), respetando los 2 PRs
encadenados (PR1 lógica pura sin red; PR2 adapters + cmd + YAML + cleanup).
