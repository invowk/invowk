---
name: go-hexagonal-ddd
description: Hexagonal Architecture and Domain-Driven Design guidance for Invowk's Go codebase. Use when Codex designs, reviews, refactors, or implements Go package boundaries, application services, ports/adapters, aggregates, value objects, repositories, domain services, module/discovery/runtime architecture, or when explicitly invoked for a full codebase architecture review with deterministic subagent passes and high-confidence design findings.
---

# Go Hexagonal DDD

Use this skill to keep Invowk's Go architecture domain-centered without turning
simple code into ceremony. Pair it with `.agents/skills/go/SKILL.md` for lint
and implementation rules, and `.agents/skills/invowk-typesystem/SKILL.md` when
adding or changing value types.

For source grounding, read `references/source-guide.md` when the task involves
new boundaries, port choices, aggregate decisions, or an architecture review.

## Operating Model

1. Start from the current code and repository rules. `AGENTS.md` and
   `.agents/rules/*.md` outrank this skill.
2. Name the capability in Invowk language before naming a package or pattern:
   command execution, dependency validation, module sync, discovery, runtime
   selection, audit scanning, TUI session management, and so on.
3. Identify the application boundary. Keep user/transport concerns in driving
   adapters, use-case orchestration in application services, invariants in
   domain types, and technology details in driven adapters.
4. Add DDD structure only where it pays for itself: persistent invariants,
   complicated policies, substitutable external devices, multiple frontends, or
   test isolation across technology boundaries.
5. Keep Go idiomatic: prefer small packages, concrete structs, plain functions,
   explicit constructors, and caller-owned interfaces. Avoid Java-style layers,
   abstract factories, and one-interface-per-struct habits.
6. Verify the design with tests that drive the inner application without the
   real outside device when practical.

## Boundary Map

Use this as a starting hypothesis, then confirm with package docs and call
sites:

| Role | Invowk examples | Responsibility |
| --- | --- | --- |
| Driving adapters | `cmd/invowk/`, TUI entrypoints, testscript fixtures | Parse input, select use case, render output, handle transport/UI details |
| Application services | `internal/app/commandsvc/`, `internal/app/deps/`, `internal/app/execute/` | Coordinate use cases, enforce workflow order, return typed results/errors |
| Domain model | `pkg/invowkfile/`, `pkg/invowkmod/`, `pkg/types/`, focused internal domain packages | Represent ubiquitous language, invariants, policies, value objects |
| Driven adapters | container engines, filesystem/process/network/runtime integrations | Implement external-device conversations behind narrow contracts |
| Foundation packages | `pkg/types/`, domain-agnostic helpers | Provide leaf value types or utilities without importing domains |

Do not force every package into one role. If a package intentionally combines
tightly coupled concerns, keep it together and separate by files unless a split
removes real dependency pressure.

## Port Decisions

Create or preserve a port when at least one is true:

- The inside needs to talk to an external technology or device: process runner,
  container engine, filesystem, SSH server, terminal UI, network, clock, or
  persistent storage.
- More than one adapter is expected or already exists: real vs in-memory,
  native vs virtual vs container, CLI vs TUI vs test harness.
- The dependency blocks deterministic tests or local development.
- The conversation has stable business meaning independent of technology.

Avoid a port when:

- It only wraps a single in-process helper with no boundary behavior.
- It exists because every use case "should have an interface."
- It hides concrete code from the only package that can ever call it.
- It makes names less domain-specific or forces data into an anemic DTO shape.

Place interfaces where the caller owns the need. Keep them tiny and name them
after the conversation, not the implementation.

## DDD Modeling Rules

- Use ubiquitous-language names already present in Invowk docs and schemas:
  `cmd`, `module`, `runtime`, `dependency`, `scope`, `lock`, `audit finding`.
- Treat bounded contexts as semantic boundaries, not necessarily services or
  repositories. In this repo they usually map to package families.
- Prefer value objects for data with validation, normalization, or comparison
  semantics. Follow `invowk-typesystem` for constructors, `Validate()`, sentinel
  errors, and catalogs.
- Use aggregates only around consistency boundaries that must change together.
  Keep them small; do not build object graphs for read convenience.
- Use domain services for policies that do not naturally belong to one value
  object or aggregate.
- Use repositories or query ports for persistence/retrieval conversations, not
  as generic data-access wrappers around every collection.
- Keep application services orchestration-focused. If they are making business
  decisions by inspecting primitive fields, move the decision toward the domain
  model.

## Go Simplicity Guardrails

- A package split must reduce coupling, clarify ownership, or unblock testing.
  Do not split just to mirror a diagram.
- A new interface must have a consumer-side reason. Testability alone is enough
  only when the dependency is a real outside actor or slow/nondeterministic.
- Value objects should improve meaning and validation; do not wrap every string
  in low-risk adapter code.
- Domain packages should return typed errors/diagnostics and let adapters
  choose presentation.
- Keep request-scoped configuration explicit. Avoid hidden globals for dynamic
  command execution, discovery, runtime, or server state.
- Prefer one clear use-case method over a generic service with flags that encode
  unrelated workflows.

## Manual Full-Codebase Review

When the user explicitly invokes `$go-hexagonal-ddd` for a full, repo-wide, or
codebase architecture review, run a deterministic review instead of a loose
brainstorm. Do not auto-run this full workflow for ordinary package design or
implementation tasks.

### Coordinator Duties

1. Read `AGENTS.md`, `.agents/rules/package-design.md`, and
   `references/source-guide.md`.
2. Capture a stable baseline: branch/HEAD, `git status --short`, and
   `go list ./cmd/... ./internal/... ./pkg/... ./tools/...`.
3. Create a task list and launch subagents for the review surfaces below. Use no
   more than six live subagents. If fewer slots are available, queue the
   remaining surfaces and launch them only as slots free up.
4. Tell every subagent this is a read-only review. They must not edit files, run
   broad formatters, or propose speculative rewrites.
5. Do not substitute coordinator analysis for a queued surface. The coordinator
   may inspect shared metadata, deduplicate results, and verify reported
   evidence, but each owned surface should be reviewed by its assigned subagent.
6. Merge subagent reports by evidence quality, not by volume. Prefer no finding
   over a low-confidence architecture opinion.

### Review Surfaces

Use these deterministic surfaces unless the user narrows scope:

| Surface | Primary paths | Focus |
| --- | --- | --- |
| SA-1 CLI and app services | `cmd/invowk/`, `internal/app/commandsvc/`, `internal/app/execute/`, `internal/app/deps/` | Driving adapter boundaries, use-case orchestration, domain policy placement |
| SA-2 Discovery and modules | `internal/discovery/`, `pkg/invowkmod/`, `modules/`, module CLI tests | Dependency graph semantics, scope rules, module aggregate boundaries |
| SA-3 Runtime and outside devices | `internal/runtime/`, `internal/container/`, `internal/provision/`, `internal/watch/`, `internal/uroot/` | Ports/adapters, host process/container/filesystem boundaries, deterministic test seams |
| SA-4 Schemas and value types | `pkg/invowkfile/`, `pkg/types/`, `internal/config/`, `pkg/cueutil/` | Invariants, value-object placement, schema/domain language drift |
| SA-5 Audit and security domains | `internal/audit/`, `internal/issue/`, lock-file and module-security call sites | Finding model, trust boundaries, policy services, error/diagnostic ownership |
| SA-6 UI/server adapters and tools | `internal/tui/`, `internal/tuiserver/`, `internal/sshserver/`, `internal/core/serverbase/`, `tools/goplint/` | Adapter leakage, server lifecycle boundaries, analyzer/domain contract fit |

### Subagent Prompt Shape

Give each subagent the same constraints plus its assigned surface:

```text
Use $go-hexagonal-ddd in /var/home/danilo/Workspace/github/invowk/invowk for a
read-only architecture review of <surface>. Read AGENTS.md, the relevant
.agents/rules files, and .agents/skills/go-hexagonal-ddd/references/source-guide.md.
Inspect current code, imports, call sites, and tests for this surface only.
Return only high-confidence Hexagonal Architecture or DDD findings. For each
finding include: evidence files/symbols, why the current boundary causes real
design pressure, the smallest proposed design adjustment, and verification to
run. If nothing meets that bar, say so.
```

### High-Confidence Gate

Report a finding only when all are true:

- It is grounded in current files, imports, call sites, or tests with concrete
  symbols and paths.
- It names the Invowk capability or policy being misplaced, duplicated, hidden
  in an adapter, or coupled to an outside device.
- It explains real pressure: blocked deterministic tests, repeated policy
  logic, dependency direction mismatch, schema/domain language drift,
  nondeterministic outside actor coupling, or a security/trust-boundary leak.
- It proposes the smallest design adjustment that would improve ownership,
  coupling, invariants, or testability without adding ceremony.
- It includes a verification path such as narrow Go tests, CLI testscript,
  schema sync checks, or package import checks.

Omit items based only on naming taste, preferred layering style, hypothetical
future adapters, generic "clean architecture" rules, or value-object wrapping
with no validation/normalization leverage.

### Final Report Shape

Lead with findings, ordered by impact and confidence. For each finding use this
shape:

- **Finding:** concise title with affected surface.
- **Evidence:** concrete files/symbols/call sites.
- **Why it matters:** the current design pressure in Invowk terms.
- **Design adjustment:** the smallest proposed package/type/port movement.
- **Verification:** exact checks or tests that would confirm the adjustment.

After findings, add "No High-Confidence Finding" surfaces so the user can see
which areas were reviewed without noise. Keep speculative follow-ups separate
and clearly labeled, or omit them if the user asked for findings only.

## Review Checklist

Ask these questions before proposing or editing architecture:

1. What use case or policy is this code serving in Invowk terms?
2. Which side of the boundary owns each dependency?
3. Can the application behavior be tested without the real UI, shell, container,
   network, or filesystem device?
4. Are invariants expressed in named types or are they scattered through
   adapters and tests?
5. Is the proposed abstraction smaller and clearer than the concrete code it
   replaces?
6. What repo skill or rule must be paired with this change: `go`,
   `invowk-typesystem`, `cue`, `container`, `shell`, `module-security`, or
   `testing`?

## Verification

For architecture-only reviews, cite concrete packages, imports, call sites, and
tests. For code changes, run the narrow relevant tests plus the checks required
by the paired skills. If `AGENTS.md` or `.agents/skills/*` changes, run
`make check-agent-docs`.
