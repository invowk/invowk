---
name: go-hexagonal-ddd
description: Hexagonal Architecture and Domain-Driven Design guidance for Invowk's Go codebase. Use when Codex designs, reviews, refactors, or implements Go package boundaries, application services, ports/adapters, aggregates, value objects, repositories, domain services, module/discovery/runtime architecture, or when deciding how much DDD structure is warranted without sacrificing Go readability.
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
