---
description: Package boundaries, interface decoupling, tightly-coupled concerns, utility package guidelines
globs:
  - "pkg/**"
  - "internal/**"
---

# Package Design

## Cross-Domain Dependencies

When two packages have a semantic relationship (e.g., `invkmod` and `invkfile`), use **interface decoupling** to avoid circular dependencies and support future extensibility.

### Pattern: Lower-Level Defines Interfaces

The lower-level domain defines interfaces or minimal data types for what it needs. The higher-level domain implements them.

```
pkg/
├── invkmod/           # Lower-level: defines minimal types/contracts
│   └── invkmod.go     # Core types and module metadata
└── invkfile/          # Higher-level: defines commands
    └── invkfile.go    # Command definitions
```

### Example

```go
// pkg/invkmod/invkmod.go
package invkmod

// Module represents a loaded invowk module, ready for use.
// Commands holds parsed command definitions without importing pkg/invkfile.
type Module struct {
    // Commands is stored as any to avoid a dependency cycle.
    // The concrete type is *invkfile.Invkfile.
    Commands any
}
```

If you need a compile-time contract, introduce a small interface in `invkmod` and implement it in `invkfile`.

### Why This Pattern

1. **No circular dependencies**: `invkfile → invkmod` only; `invkmod` never imports `invkfile`.
2. **Future extensibility**: Supports 1:N relationships (one Module, many Invkfiles) without restructuring.
3. **Testability**: Interfaces enable mocks when you need isolated testing.
4. **Clear contracts**: Interfaces document exactly what the lower-level domain needs.

### When to Apply

Use interface decoupling when:
- Two packages have a semantic parent-child or owner-owned relationship.
- The relationship may evolve (1:1 → 1:N or N:M).
- You want to test one domain independently of the other.

### Anti-Pattern: Bridge Package

Avoid creating third "bridge" packages (e.g., `invkbridge`) that import both domains just to hold aggregations. This adds navigation complexity without providing meaningful abstraction.

**Exception**: A bridge package is acceptable when it genuinely adds orchestration logic beyond simple aggregation.

## Tightly-Coupled Multi-Concern Packages

When a package has multiple tightly-coupled concerns that would create artificial indirection if split:

1. **Keep them together** in one package.
2. **Document explicitly** in `doc.go` that the package handles multiple related concerns.
3. **Separate internally** by file (e.g., `discovery_files.go`, `discovery_commands.go`).

### Example

```go
// internal/discovery/doc.go

// Package discovery handles invkfile and invkmod discovery and command aggregation.
//
// This package intentionally combines two related concerns:
//   - File discovery: locating invkfile.cue and invkmod directories
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

- Helper function references domain types (e.g., `invkfile.Command`, `invkmod.Invkmod`).
- Helper is only used by one domain package.
- Helper name includes domain terminology (e.g., `buildTestCommand` in generic `testutil`).

### Resolution

Migrate to the domain package. If multiple domain packages need similar helpers, each gets its own copy—duplication is preferable to hidden coupling in utilities.

### Exception: Domain-Specific Test Subpackages

Creating `testutil/invkfiletest/` is acceptable when:
- The helpers are genuinely reusable across multiple test files.
- Import cycles would otherwise occur (test package can't import the package it tests).

This is already established in the `testing` skill (`.claude/skills/testing/`) for `invkfiletest`.
