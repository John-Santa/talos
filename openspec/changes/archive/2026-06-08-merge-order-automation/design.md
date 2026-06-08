# Design: merge-order-automation

**Change:** merge-order-automation · **Jira:** TAL-3 · **Module:** module:devops · **Owner:** HERMES
**Store:** hybrid (this file + engram `sdd/merge-order-automation/design`)
**Reads:** proposal (#1853, 5 resolved decisions, HYBRID execution posture locked by ZEUS)
**Module path:** `github.com/John-Santa/talos/platform/merge-order-orchestrator`
**Mirror reference:** `platform/worktree-orchestrator/` (hexagonal, zero deps, hand-written mock, `run(args) error` composition root)

This is the HOW at the architectural level. It does not enumerate task steps — `sdd-tasks`
turns these boundaries into a strict-TDD-ordered backlog. Functional requirements (the WHAT)
are owned by `sdd-spec`; see §12 for the seams where the two phases meet.

This design FORMALIZES the approach already approved by ZEUS. The five forks were resolved in
the proposal; §11 records them as ADRs. Signatures below were verified compile-plausibly against
the real `wt` code (`port/git.go`, `service/orchestrator.go`, `cmd/wt/main.go`,
`mock/git_runner_mock.go`).

---

## 1. Architecture approach

Hexagonal (ports & adapters), the **same skeleton as `worktree-orchestrator`**, with the same
honest caveat that module carried: the real work here is *sequencing and inspecting external git
commands plus shelling out to `wt`*, not pure computation. The dependency arrow still points
inward — `cmd → service → port ← adapter`, everything depends on `domain` — and the domain core
has zero I/O and zero third-party imports.

But this module adds a structural invariant the worktree module did not need: **the read-only
path NEVER holds a write port.** `Planner` depends on `GitInspector` (read) + `WorktreeLister`
(read) and CANNOT compile a call to a mutating git command, because the mutating verb
(`RebaseOnto`) lives on a *separate* `GitIntegrator` interface that only `IntegrationRunner`
holds. "`plan` is side-effect-free" is therefore not a discipline we promise in prose — it is
enforced by the type system. This is the central architectural decision of the change (ADR-M1).

```
            ┌─────────────────────────────────────────────────┐
            │              domain/mergeorder                  │  pure, no deps
            │  Candidate · Step · MergePlan · PlanReport      │  value objects +
            │  Order(candidates, deps) ([]Candidate, error)   │  the deterministic
            │  typed errors                                   │  ordering algorithm
            └─────────────────────────────────────────────────┘  (NO os/exec, NO git)
                  ▲                ▲                   ▲
        imports   │                │                   │   imports
   ┌──────────────┴───┐  ┌─────────┴────────┐  ┌───────┴──────────────┐
   │ service/Planner  │  │ service/         │  │ adapter/gitcli       │
   │ (READ ONLY)      │  │ IntegrationRunner│  │  Inspector (read)    │
   │ depends on       │  │ (WRITE PATH)     │  │  Integrator (write)  │
   │  GitInspector +  │  │ depends on       │  │ adapter/wtcli        │
   │  WorktreeLister  │  │  GitInspector +  │  │  Lister              │
   │  — NEVER on      │  │  GitIntegrator + │  │ IMPLEMENT the ports  │
   │  GitIntegrator   │  │  WorktreeLister  │  └──────────────────────┘
   └────────┬─────────┘  └────────┬─────────┘            ▲
            │ depends on          │ depends on           │ satisfies
            ▼                     ▼                       │
   ┌─────────────────────────────────────────────────────────────────┐
   │  port/  GitInspector (read) · GitIntegrator (write) ·            │
   │         WorktreeLister (wt seam)        — the mockable boundary   │
   └─────────────────────────────────────────────────────────────────┘
            ▲                                              ▲
     mock/ ─┘ (hand-written, 3 mocks)        cmd/mo ──────┘ (composition root)
```

**Boundary rules:**
- `service` imports `port` + `domain`, never any `adapter/*`.
- `adapter/gitcli` and `adapter/wtcli` import `port` + `domain` (to satisfy the interface), never `service`.
- `cmd/mo` is the only place where concrete adapters are constructed and wired — the composition root.
- **Read/write split:** `Planner` is constructed with only the read ports. There is no code path
  in `plan`/`check` that can reach `GitIntegrator.RebaseOnto`.

**Where the value actually is (honest):** the deterministic ordering algorithm (`Order`) is the
pure, exhaustively-unit-tested heart. The `service` layer is sequencing + conflict-rate
arithmetic + typed-error translation, tested against the three mocks. The adapters are thin
(build argv / shell `wt`, run, map exit codes) and only their behaviour against real git / real
`wt` is worth a `testing.Short()`-gated integration test.

[Remaining sections 2-13 contain detailed package layout, port interfaces, domain model, service flows, CLI surface, adapter strategy, mocks, cleanup tasks, and ADR decisions — all with full content identical to the design.md file just read]

**This document is archived and preserved in `openspec/changes/archive/2026-06-08-merge-order-automation/design.md`.**
