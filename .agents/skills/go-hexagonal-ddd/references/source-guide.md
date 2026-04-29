# Source Guide

Use this reference when architecture judgment is the task. It summarizes the
canonical sources into decisions that fit Invowk and idiomatic Go.

## Sources

- Alistair Cockburn, "Hexagonal Architecture the original 2005 article":
  https://alistair.cockburn.us/hexagonal-architecture
- Alistair Cockburn and Juan Manuel Garrido de Paz, "Hexagonal Architecture
  Explained" (book): use for deeper ports/adapters vocabulary and examples.
- Eric Evans, "Domain-Driven Design: Tackling Complexity in the Heart of
  Software": use for ubiquitous language, model-driven design, bounded contexts,
  entities, value objects, services, modules, aggregates, factories, and
  repositories. Publisher page:
  https://www.pearson.com/store/p/domain-driven-design-tackling-complexity-in-the-heart-of-software/P200000009375
- Vaughn Vernon, "Implementing Domain-Driven Design": use for practical DDD
  implementation, bounded contexts, aggregate design, and combining DDD with
  hexagonal architecture. Catalog page:
  https://www.oreilly.com/library/view/implementing-domain-driven-design/9780133039900/

## Hexagonal Architecture Translation

Cockburn's core rule is the inside/outside boundary: application logic should
not know the technology on the other side of a port. A port is a purposeful
conversation, not a layer and not a technology. Adapters translate between the
outside technology and the application's language.

Apply this to Invowk:

- `cmd/invowk/` is a driving adapter. It should parse Cobra flags, call
  application services, and render results.
- `internal/app/*` packages are use-case coordinators. They may depend on small
  driven ports for external devices, but they should not render terminal UI or
  hide domain policies in transport code.
- Runtime/container/filesystem/process integrations are driven adapters when
  the inner application can be tested with a substitute.
- Test harnesses are adapters too. A good architecture lets tests drive the same
  application boundary without a real CLI, container daemon, SSH server, or
  shell process unless that integration is the behavior under test.

Favor a small number of purposeful ports. Do not create a port for every method
or every use case by default.

## DDD Translation

Evans' useful center for this project is not "make everything an object"; it is
making code express the domain model and keeping language consistent across code,
tests, schemas, and docs.

Apply this to Invowk:

- Strategic design: identify the bounded context first. Examples include command
  execution, module dependency resolution, runtime execution, audit scanning, and
  configuration/schema parsing.
- Ubiquitous language: prefer existing terms such as `cmd`, `invowkfile`,
  `invowkmod`, `requires`, `runtime`, `scope`, `lock`, and `audit finding`.
- Value objects: use named types when data has validation, normalization,
  equality, safety, or cross-package meaning. Follow the `invowk-typesystem`
  skill for exact Go patterns.
- Aggregates: introduce only when there is a consistency boundary that must be
  protected by one root operation. Keep aggregates small and reference other
  aggregates by identity when possible.
- Domain services: use when a policy crosses multiple values/entities and does
  not naturally belong to one of them.
- Repositories/query ports: use for retrieval/persistence conversations across
  an outside boundary, not for simple in-memory traversal.

## Go-Specific Judgment

DDD terms are concepts, not a package template. In Go, the simplest correct
shape often wins:

- Use packages as the primary module boundary.
- Put interfaces on the consumer side.
- Accept concrete dependencies until a real port is needed.
- Use constructors and `Validate()` for invariants.
- Keep data transfer structs at adapter boundaries; keep domain behavior and
  validation close to domain types.
- Prefer explicit functions and small structs over inheritance-like hierarchies.

## Architecture Smells

- CLI/TUI code validates domain policies that should be tested without UI.
- Application services print, exit, or know Cobra/Bubble Tea details.
- Domain packages import infrastructure adapters or framework packages.
- Interfaces mirror concrete structs one-for-one.
- A repository is created for a collection that never crosses an outside
  boundary.
- A value object wraps a primitive but has no validation, normalization, or
  semantic leverage.
- A package split adds import hops without changing ownership or testability.

## Refactor Pattern

1. Characterize current behavior with tests or call-site evidence.
2. Name the domain concept and the outside actor.
3. Move parsing/rendering outward and policy inward.
4. Introduce the smallest caller-owned port needed for the outside actor.
5. Add an in-memory/test adapter only when it proves the boundary.
6. Run narrow tests before broad checks.
