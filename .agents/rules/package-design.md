---
paths:
  - "pkg/**"
  - "internal/**"
---

# Package Design

## Cross-Domain Dependencies

When two packages have a semantic relationship (e.g., `invowkmod` and `invowkfile`), use **interface decoupling** to avoid circular dependencies and support future extensibility.

### Pattern: Lower-Level Defines Interfaces

The lower-level domain defines interfaces or minimal data types for what it needs. The higher-level domain implements them.

```
pkg/
├── invowkmod/           # Lower-level: defines minimal types/contracts
│   └── invowkmod.go     # Core types and module metadata
└── invowkfile/          # Higher-level: defines commands
    └── invowkfile.go    # Command definitions
```

### Example

```go
// pkg/invowkmod/invowkmod.go
package invowkmod

type ModuleCommands interface {
    GetModule() string
    ListCommands() []string
}

// Module represents a loaded invowk module, ready for use.
// Commands holds typed command definitions without importing pkg/invowkfile.
type Module struct {
	Commands ModuleCommands
}
```

If you need a compile-time contract, introduce a small interface in `invowkmod` and implement it in `invowkfile`.

### Why This Pattern

1. **No circular dependencies**: `invowkfile → invowkmod` only; `invowkmod` never imports `invowkfile`.
2. **Future extensibility**: Supports 1:N relationships (one Module, many Invowkfiles) without restructuring.
3. **Testability**: Interfaces enable mocks when you need isolated testing.
4. **Clear contracts**: Interfaces document exactly what the lower-level domain needs.

### When to Apply

Use interface decoupling when:
- Two packages have a semantic parent-child or owner-owned relationship.
- The relationship may evolve (1:1 → 1:N or N:M).
- You want to test one domain independently of the other.

### Anti-Pattern: Aggregation Bridge Package

Avoid creating third "bridge" packages (e.g., `ivkbridge`) that **import both domains** just to hold aggregations. This adds navigation complexity without providing meaningful abstraction.

**Exception**: A bridge package is acceptable when it genuinely adds orchestration logic beyond simple aggregation.

### Pattern: Foundation Package for Cross-Cutting Value Types

A **foundation package** (e.g., `pkg/types`) that defines standalone value types and is **imported by** multiple domain packages is explicitly allowed. This is the opposite of a bridge — it imports neither domain, only the standard library. Both domains depend on it as a leaf.

```
pkg/types/            ← imports only stdlib (errors, fmt, strings)
  ↑          ↑
invowkfile  invowkmod  ← both import types; types imports neither
```

**Use `pkg/types` when:**
- A DDD Value Type has identical semantics across multiple domain packages (e.g., `DescriptionText` used by commands, flags, arguments, and modules).
- Placing the type in one domain and aliasing from the other would misrepresent its ownership (e.g., `DescriptionText` is not module-specific, so placing it in `invowkmod` is semantically dishonest).
- The type has no domain-specific dependencies — it depends only on the standard library.

**Do not use `pkg/types` for:**
- Types that belong to a single domain (keep them in that domain's package).
- Types that require importing domain packages (that would create a bridge, not a foundation).

## State Ownership and Mutation Boundaries

Design package APIs so state ownership is explicit and mutable state is localized.

### Rules

- Keep mutable operational state inside short-lived structs/services created by constructors.
- Avoid package-level mutable singletons for runtime behavior (execution state, server instances, dynamic discovery caches, request config).
- If process-wide state is unavoidable (for backward compatibility), isolate it behind a narrow API and provide deterministic reset hooks for tests.
- Prefer immutable configuration snapshots passed into services over late global lookups.

### Why

1. **Predictability**: Commands become deterministic across repeated invocations.
2. **Test isolation**: No hidden cross-test coupling via leaked global state.
3. **Concurrency safety**: Fewer TOCTOU and race-prone shared writes.
4. **Composability**: Different frontends (CLI/TUI/tests) can wire different implementations cleanly.

## Boundary-Oriented Responsibilities

Separate package responsibilities by boundary:

- **Boundary packages** (e.g., `cmd/`) handle input parsing, output rendering, and transport concerns.
- **Domain packages** (e.g., `internal/discovery`, `internal/config`) implement business/domain logic and return typed results/diagnostics.
- **Domain packages must not perform terminal rendering** as part of core flows; return structured diagnostics/errors and let boundary packages decide presentation.

This keeps package APIs reusable across CLI execution, completion, tests, and future frontends.

## Tightly-Coupled Multi-Concern Packages

When a package has multiple tightly-coupled concerns that would create artificial indirection if split:

1. **Keep them together** in one package.
2. **Document explicitly** in `doc.go` that the package handles multiple related concerns.
3. **Separate internally** by file (e.g., `discovery_files.go`, `discovery_commands.go`).

### Example

```go
// internal/discovery/doc.go

// Package discovery handles invowkfile and invowkmod discovery and command aggregation.
//
// This package intentionally combines two related concerns:
//   - File discovery: locating invowkfile.cue and invowkmod directories
//   - Command aggregation: building the unified command tree from discovered files
//
// These concerns are tightly coupled because command aggregation depends directly
// on discovery results and ordering. Splitting them would create unnecessary
// indirection without meaningful abstraction benefit.
package discovery
```

### When to Keep vs Split

**Keep together** when:
- Concern B directly consumes the output of Concern A with no intermediate consumers.
- Splitting would require passing large intermediate data structures between packages.
- The concerns share significant internal state or helpers.

**Split** when:
- Other packages need Concern A's output without Concern B.
- The concerns have different rates of change or different maintainers.
- File size exceeds 600 lines even with good internal organization.

## Utility Package Boundaries

Utility packages (`testutil`, `cueutil`, etc.) must remain **domain-agnostic**. When domain-specific logic appears in a utility package, migrate it to the domain it serves.

### Signs of Domain Creep

- Helper function references domain types (e.g., `invowkfile.Command`, `invowkmod.Invowkmod`).
- Helper is only used by one domain package.
- Helper name includes domain terminology (e.g., `buildTestCommand` in generic `testutil`).

### Resolution

Migrate to the domain package. If multiple domain packages need similar helpers, each gets its own copy—duplication is preferable to hidden coupling in utilities.

### Exception: Domain-Specific Test Subpackages

Creating `testutil/invowkfiletest/` is acceptable when:
- The helpers are genuinely reusable across multiple test files.
- Import cycles would otherwise occur (test package can't import the package it tests).

This is already established in the `testing` skill (`.claude/skills/testing/`) for `invowkfiletest`.
