---
paths:
  - "pkg/**"
  - "internal/**"
  - "cmd/**"
---

# Package Design

## Cross-Domain Dependencies

- When two packages have a semantic owner/owned relationship (e.g., `invowkmod` and `invowkfile`), use **interface decoupling**: the lower-level domain defines the interfaces or minimal types it needs, and the higher-level domain implements them. Apply it when the relationship may evolve (1:1 → 1:N or N:M) or one domain must be testable without the other.
- `invowkfile → invowkmod` only; `invowkmod` never imports `invowkfile`. Reference: the `ModuleCommands` interface in `pkg/invowkmod/invowkmod.go` (`GetModule() ModuleID`, `ListCommands() []string`), held by `Module.Commands`.
- If you need a compile-time contract, introduce a small interface in `invowkmod` and implement it in `invowkfile`.
- Avoid "bridge" packages that import both domains only to hold aggregations. A bridge is acceptable only when it adds real orchestration logic.

## Foundation Package for Cross-Cutting Value Types

`pkg/types` imports only the standard library and is imported by multiple domains as a leaf. Use it for value types with identical semantics across domains (e.g., `DescriptionText`), where placing the type in one domain would misrepresent its ownership. Do not use it for single-domain types or for types that need domain imports.

## State Ownership and Mutation Boundaries

- Keep mutable operational state inside short-lived structs/services created by constructors.
- Avoid package-level mutable singletons for runtime behavior (execution state, server instances, dynamic discovery caches, request config).
- If process-wide state is unavoidable (for backward compatibility), isolate it behind a narrow API and provide deterministic reset hooks for tests.
- Prefer immutable configuration snapshots passed into services over late global lookups.

## Boundary-Oriented Responsibilities

- **Boundary packages** (e.g., `cmd/`) handle input parsing, output rendering and transport concerns.
- **Domain packages** (e.g., `internal/discovery`, `internal/config`) implement domain logic and return typed results and diagnostics.
- **Domain packages must not perform terminal rendering** in core flows; return structured diagnostics/errors and let boundary packages decide presentation.

## Tightly-Coupled Multi-Concern Packages

Keep tightly-coupled concerns in one package when splitting would only add indirection: document the combination in `doc.go` (see `internal/discovery/doc.go`) and separate concerns by file.

- **Keep together** when concern B directly consumes concern A's output with no other consumers, splitting would pass large intermediate structures between packages, or the concerns share significant internal state or helpers.
- **Split** when other packages need A without B, the concerns change at different rates or have different maintainers, or a file exceeds 600 lines even with good internal organization.

## Utility Package Boundaries

- Utility packages (`testutil`, `cueutil`, etc.) stay **domain-agnostic**. Signs of domain creep: a helper references domain types (e.g., `invowkfile.Command`), is used by only one domain package, or carries domain terminology in its name. Migrate such helpers to the domain they serve.
- If several domain packages need similar helpers, each gets its own copy: duplication is preferable to hidden coupling in utilities.
- Domain-specific test subpackages such as `testutil/invowkfiletest/` are acceptable when the helpers are reusable across test files and would otherwise cause import cycles (see the `testing` skill).
