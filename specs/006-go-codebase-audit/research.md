# Research: Go Codebase Quality Audit

**Feature Branch**: `006-go-codebase-audit`
**Date**: 2026-01-30

## Executive Summary

This research documents findings from the technical analysis of the Invowk Go codebase for the quality audit feature. All "NEEDS CLARIFICATION" items from the Technical Context have been resolved through codebase exploration and spec clarifications.

---

## 1. File Size Analysis

### Decision

Files exceeding 800 lines will be split into focused files under 600 lines, organized by logical concern.

### Rationale

- Large monolithic files are difficult for both humans and AI coding agents to reason about effectively
- The 800-line threshold is specified in FR-001
- The 600-line target provides headroom for future additions
- Split strategy follows existing patterns in the codebase (e.g., `invowkfile_deps_*.go` splits)

### Alternatives Considered

1. **700-line threshold**: Rejected - too aggressive, would affect 33 additional files
2. **No splitting, just comments**: Rejected - doesn't address navigation and cognitive load
3. **Splitting by function**: Rejected - better to split by logical concern for cohesion

### Findings

**Production Files Over 800 Lines (5 files)**:

| File | Lines | Logical Concerns |
|------|-------|-----------------|
| cmd/invowk/module.go | 1,118 | validate, create, alias, package operations |
| cmd/invowk/cmd_validate.go | 920 | runtime validation, dep validation, schema validation |
| internal/runtime/container.go | 917 | build, exec, provisioning |
| internal/tui/interactive.go | 806 | model definition, update logic, view rendering |
| pkg/invowkmod/operations.go | 827 | validate, create, package operations |

**Test Files Over 800 Lines (3 files)**:

| File | Lines | Split Strategy |
|------|-------|---------------|
| internal/runtime/container_integration_test.go | 847 | By tested functionality |
| pkg/invowkmod/operations_packaging_test.go | 817 | Zip vs unzip operations |
| pkg/invowkfile/invowkfile_flags_enhanced_test.go | 814 | Validation vs parsing tests |

---

## 2. Server State Machine Pattern

### Decision

Extract shared state machine logic into `internal/core/serverbase/` using composition via struct embedding.

### Rationale

- Both SSH server and TUI server implement identical state machine logic
- The pattern is documented in `.claude/rules/servers.md` as the canonical approach
- Composition avoids inheritance complexity while enabling code reuse
- Functional options pattern for constructors aligns with clarified requirements

### Alternatives Considered

1. **Interface-only extraction**: Rejected - both servers need the same concrete implementation, not just the contract
2. **Code generation**: Rejected - adds build complexity for minimal benefit
3. **Keep duplication, document well**: Rejected - creates maintenance burden when fixing bugs

### Shared Code Analysis

**Identical in both servers**:
```go
// State type and constants
type ServerState int32
const (
    StateCreated ServerState = iota
    StateStarting
    StateRunning
    StateStopping
    StateStopped
    StateFailed
)

// String() method
func (s ServerState) String() string { ... }

// Common fields
state     atomic.Int32
stateMu   sync.Mutex
ctx       context.Context
cancel    context.CancelFunc
wg        sync.WaitGroup
startedCh chan struct{}
errCh     chan error
lastErr   error
```

**Server-specific code** (remains in concrete implementations):
- SSH: Token management, key generation, connection handling
- TUI: HTTP server setup, request routing, Bubble Tea integration

---

## 3. Container Engine Pattern

### Decision

Extract shared CLI engine logic into `internal/container/engine_base.go` using composition via struct embedding.

### Rationale

- Docker and Podman implementations share ~80% identical code
- Only differences are: version format strings, SELinux label handling
- Embedding a base struct reduces ~500 lines of duplicated code
- Functional options pattern for base engine configuration

### Alternatives Considered

1. **Function extraction only**: Rejected - doesn't provide struct composition benefits
2. **Generic engine with type parameter**: Rejected - Go generics not suited for this pattern
3. **Keep separate, use shared helper functions**: Rejected - still duplicates struct fields and boilerplate

### Shared Code Analysis

**Methods with identical logic**:
- `Build()`: Argument construction for `docker build` / `podman build`
- `Run()`: Volume mounts, port mappings, environment variables
- `Exec()`: Execute command in running container

**Engine-specific differences**:
- Docker: `--format '{{.Server.Version}}'` for version
- Podman: `--format '{{.Version.Version}}'` for version
- Podman: SELinux `:Z` labels on volume mounts

---

## 4. CUE 3-Step Parsing Pattern

### Decision

Create `internal/cueutil/parse.go` with a generic `ParseAndDecode[T]` function.

### Rationale

- Three implementations repeat identical logic with only type differences
- Generic function eliminates boilerplate while maintaining type safety
- Consolidation ensures consistent error formatting and file size limits
- Package is internal, so API changes don't affect external consumers

### Alternatives Considered

1. **Interface-based approach**: Rejected - loses compile-time type safety
2. **Code generation**: Rejected - Go generics solve this elegantly
3. **Shared helper functions, separate Decode calls**: Rejected - still duplicates unify/validate logic

### API Design

```go
// internal/cueutil/parse.go

// ParseOptions configures CUE parsing behavior.
type ParseOptions struct {
    MaxFileSize  int64   // Default: 5MB
    SchemaPath   string  // e.g., "#Invowkfile", "#Invowkmod", "#Config"
    Filename     string  // For error messages
    Concrete     bool    // Whether to require concrete values (default: true)
}

// WithMaxFileSize sets the maximum allowed file size.
func WithMaxFileSize(size int64) Option

// WithConcrete sets whether values must be concrete.
func WithConcrete(concrete bool) Option

// ParseAndDecode compiles schema, unifies with data, validates, and decodes to Go struct.
func ParseAndDecode[T any](schema, data []byte, schemaPath string, opts ...Option) (*T, error)
```

---

## 5. CUE Schema Constraints

### Decision

Add missing `strings.MaxRunes()` constraints and non-empty-with-content validation per FR-008/FR-009.

### Rationale

- Defense-in-depth: catch errors at parse time rather than runtime
- Consistent with existing patterns in the schema
- Clear error messages from CUE validation

### Alternatives Considered

1. **Go-only validation**: Rejected - delays error reporting, duplicates validation responsibility matrix
2. **Larger limits**: Rejected - spec specifies reasonable limits based on use cases
3. **No length limits**: Rejected - allows abuse and potential DoS via extremely long strings

### Specific Changes

```cue
// #Command.description - add non-empty-with-content validation
description?: string & =~"^\\s*\\S.*$"

// #RuntimeConfigContainer.image - add length limit
image?: string & strings.MaxRunes(512)

// #RuntimeConfigNative.interpreter - add length limit (regex already exists)
interpreter?: string & =~"^\\s*\\S.*$" & strings.MaxRunes(1024)

// #RuntimeConfigContainer.interpreter - add length limit (regex already exists)
interpreter?: string & =~"^\\s*\\S.*$" & strings.MaxRunes(1024)

// #Flag.default_value - add length limit
default_value?: string & strings.MaxRunes(4096)

// #Argument.default_value - add length limit
default_value?: string & strings.MaxRunes(4096)
```

---

## 6. Error Message Standardization

### Decision

Implement error context pattern with operation, resource, and suggestion. Use existing `issue.Issue` system for rendering.

### Rationale

- FR-012 requires all user-facing errors include actionable context
- `internal/issue/` package already provides styled error rendering
- The `--verbose` flag controls error chain inclusion (FR-013)
- Consistent error format improves user experience

### Alternatives Considered

1. **New error package**: Rejected - `issue.Issue` already exists and is styled
2. **Structured logging only**: Rejected - doesn't help end users
3. **Error codes**: Rejected - over-engineering for CLI tool

### Error Context Structure

```go
// Extend issue.Issue or create wrapper
type ActionableError struct {
    Operation   string   // What was being attempted
    Resource    string   // File, path, or entity involved
    Suggestions []string // How to fix
    Cause       error    // Wrapped error (shown with --verbose)
}

func (e *ActionableError) Error() string {
    // Format: "failed to <operation>: <resource>"
}

func (e *ActionableError) Format(verbose bool) string {
    // Full format with suggestions and optional error chain
}
```

### Key Error Sites

| Location | Operation | Resource | Suggestion |
|----------|-----------|----------|------------|
| config.go:Load() | "load configuration" | config file path | "Check file exists and has valid CUE syntax" |
| native.go:findShell() | "find shell" | list of attempted shells | "Install bash or specify --shell flag" |
| container.go:Build() | "build container" | image/containerfile path | "Check Dockerfile syntax or verify path exists" |
| parse.go:ParseBytes() | "parse invowkfile" | invowkfile path + CUE path | Specific CUE constraint violated |

---

## 7. Import Cycle Resolution

### Decision

Create `internal/core/` package hierarchy for shared types that would otherwise cause import cycles.

### Rationale

- Clarified in spec: "Create a new `internal/core` or `internal/shared` package for extracted types"
- Breaking cycles by centralizing shared dependencies is idiomatic Go
- `internal/` scope prevents accidental external dependencies

### Alternatives Considered

1. **Duplicate types**: Rejected - violates DRY and FR-004
2. **Move to pkg/**: Rejected - these are implementation details, not public API
3. **Accept cycles with build tags**: Rejected - complicates build and testing

### Package Structure

```
internal/core/
├── serverbase/          # Server state machine base type
│   ├── state.go         # State type, constants, String()
│   ├── base.go          # Base struct with common fields
│   └── options.go       # Functional options
└── README.md            # Package documentation
```

---

## 8. Test Coverage Strategy

### Decision

No numeric coverage threshold; require tests for all public APIs and critical paths in modified/extracted code.

### Rationale

- Clarified in spec clarifications
- Numeric thresholds can be gamed with trivial tests
- Focus on meaningful coverage of behavior and edge cases
- Sync tests verify interface contracts between CUE and Go

### Test Requirements for Extracted Code

1. **serverbase package**:
   - State transitions (Created → Running → Stopped)
   - Double Start/Stop idempotency
   - Cancelled context handling
   - Race condition testing with `-race`

2. **engine_base package**:
   - Argument construction with various options
   - Override mechanism for engine-specific behavior
   - Mock exec.Command for unit testing

3. **cueutil package**:
   - Valid schema parsing
   - Invalid schema error messages
   - File size limit enforcement
   - Generic instantiation for different types

---

## Open Questions Resolved

All questions from the spec clarifications have been addressed:

| Question | Resolution |
|----------|------------|
| Import cycle resolution | Create `internal/core/` package |
| Extraction pattern | Composition via struct embedding |
| Error context elements | Operation + resource + suggestion (full chain with --verbose) |
| Backward compatibility for CUE | Strict enforcement immediately |
| Test coverage threshold | No numeric threshold; API and critical path coverage |

---

## Dependencies and Risks

### Dependencies

- None external; all changes are internal refactoring
- CUE library v0.15.3 supports generics in schema validation

### Risks

| Risk | Mitigation |
|------|------------|
| Breaking existing tests | Run full test suite after each file split |
| Subtle behavior changes in extracted code | Maintain identical behavior; add tests before extraction |
| Import cycle during extraction | Plan extraction order carefully; start with leaf dependencies |
| Performance regression from abstraction | Benchmark critical paths; composition has minimal overhead |

---

## Next Steps

1. Generate `data-model.md` with entity definitions for extracted types
2. Generate `contracts/` with API specifications for new internal packages
3. Generate `quickstart.md` with implementation sequence
4. Proceed to `/speckit.tasks` for task generation
