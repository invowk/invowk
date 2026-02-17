---
paths:
  - "**/*.go"
---

# Go Patterns

## Import Ordering

Imports should be organized in three groups, separated by blank lines:

```go
import (
    // 1. Standard library
    "context"
    "fmt"
    "os"

    // 2. External dependencies
    "github.com/spf13/cobra"
    "cuelang.org/go/cue"

    // 3. Internal packages
    "github.com/invowk/invowk/internal/config"
    "github.com/invowk/invowk/pkg/invowkfile"
)
```

## Naming Conventions

- **Packages**: lowercase, single word preferred (e.g., `config`, `runtime`, `discovery`).
- **Exported types**: PascalCase (e.g., `Config`, `RuntimeMode`, `ExecutionContext`).
- **Unexported types/vars**: camelCase (e.g., `globalConfig`, `configPath`).
- **Constants**: PascalCase for exported, camelCase for unexported.
- **Interfaces**: Use action-oriented names (e.g., `Engine`, `Runtime`).
- **Error variables**: Prefix with `Err` (e.g., `var ErrNotFound = errors.New("not found")`).
- **Error types**: Suffix with `Error` (e.g., `EngineNotAvailableError`).

## Error Handling

### Basic Pattern

- Always wrap errors with context using `fmt.Errorf("context: %w", err)`.
- Use custom error types for specific error cases that callers need to handle.
- Return early on errors to reduce nesting.

```go
// Good
if err := doSomething(); err != nil {
    return fmt.Errorf("failed to do something: %w", err)
}

// Custom error type example
type EngineNotAvailableError struct {
    Engine string
    Reason string
}

func (e *EngineNotAvailableError) Error() string {
    return fmt.Sprintf("container engine '%s' is not available: %s", e.Engine, e.Reason)
}
```

### Defer Close Pattern with Named Returns

When functions open resources that need closing, use **named return values** to aggregate close errors. This ensures close errors are never silently ignored.

```go
func processFile(path string) (err error) {
    f, err := os.Open(path)
    if err != nil {
        return err
    }
    defer func() {
        if closeErr := f.Close(); closeErr != nil && err == nil {
            err = closeErr
        }
    }()
    // ... work with f ...
    return nil
}
```

For multiple resources, apply the same `defer` pattern to each. The key is `closeErr != nil && err == nil` — close errors only surface when the primary operation succeeded.

**Anti-patterns:** `defer f.Close()` (silently ignored), `defer func() { _ = f.Close() }()` (explicitly discarded without aggregation).

**Exceptions:**
1. **Test code**: Use test helpers (e.g., `testutil.MustClose(t, f)`) instead of named returns.
2. **Terminal operations in SSH sessions**: Where `sess.Exit()` errors cannot be meaningfully handled, use `_ =` with a comment.
3. **Best-effort cleanup after primary error**: Logging the close error may be appropriate rather than overwriting the primary error.
4. **Read-only file handles**: `os.Open()` close failures are exotic edge cases. Use `defer func() { _ = f.Close() }()` with a comment.

## Documentation

- Every exported type, function, and constant MUST have a doc comment.
- Package comments should be in the format `// Package name description.`
- Use complete sentences starting with the item name.

### Semantic Commenting (CRITICAL)

Comments are not optional decoration. They are part of the contract of the code.

- **Document semantics, not syntax**: Explain intent, behavior, invariants, ownership, lifecycle, precedence, and side effects. Do not restate obvious code.
- **Document abstractions and interfaces**: For interfaces, explain what the abstraction means, who implements it, and what callers can rely on.
- **Document function signature meaning**: Clarify non-obvious parameter semantics, required/optional values, return value meaning, error behavior, and context cancellation expectations.
- **Document field purpose**: Add comments for fields that carry domain meaning, constraints, units, ownership, or cross-component contracts.
- **Document subtle body logic**: Add inline comments where behavior is easy to misread (fallback chains, precedence rules, best-effort cleanup, intentionally ignored errors, ordering requirements, race-sensitive logic).
- **Prefer why over what**: If the code already shows what it does, comment why this approach exists.

**When comments are required even for unexported code:**
- Non-trivial orchestration or state transitions
- Security-sensitive behavior or validation rules
- Compatibility behavior and migration shims
- Error handling decisions that are intentional (for example, continue-on-error paths)

**Comment quality rules:**
- Keep comments precise and maintainable; update or remove stale comments when code changes.
- Avoid boilerplate comments that only repeat identifiers.
- If a block is subtle enough to require rereading, add a short comment before it.
- **Guardrail-safe references**: When commenting about deprecated or removed APIs (e.g., explaining what an abstraction replaces), use indirect phrasing rather than the exact prohibited call signature. Pattern guardrail tests like `TestNoGlobalConfigAccess` scan all non-test `.go` files with raw substring matching and will flag comments containing prohibited patterns. For example, write "replaces the previous global config accessor" instead of the literal call expression.

```go
// GOOD: explains semantics and constraints.
// ResolveConfigPath applies precedence: CLI flag > env override > default path.
// It returns an absolute path so downstream checks are deterministic.
func ResolveConfigPath(cliPath string) (string, error) { ... }

// GOOD: documents abstraction contract.
// CommandService executes a resolved command request and returns user-renderable diagnostics.
// Implementations must not write directly to stdout/stderr.
type CommandService interface {
    Execute(ctx context.Context, req ExecuteRequest) (ExecuteResult, []Diagnostic, error)
}

// BAD: restates obvious code.
// Set x to 10
x := 10
```

```go
// Package config handles application configuration using Viper with CUE.
package config

// Config holds the application configuration
type Config struct {
    // ContainerEngine specifies whether to use "podman" or "docker"
    ContainerEngine ContainerEngine `json:"container_engine" mapstructure:"container_engine"`
}

// Load reads and parses the configuration file
func Load() (*Config, error) { ... }
```

### Package Documentation (doc.go files)

For packages with extensive documentation, use a dedicated `doc.go` file:

**When to use `doc.go`:**
- Package has complex APIs requiring multi-paragraph explanation
- Need to document architectural decisions or design rationale
- Package has multiple concerns that should be explained together

**Structure:**
```go
// SPDX-License-Identifier: MPL-2.0

// Package name provides brief summary.
//
// Extended description with multiple paragraphs explaining:
//   - Purpose and key abstractions
//   - Important types and interfaces
//   - Design decisions or limitations
//   - File organization (if package spans multiple files)
package name
```

**CRITICAL: Only one package comment per package.** When using `doc.go`, remove `// Package name ...` comments from other files (keep only `package name`). Go's `go doc` tool concatenates all package comments, causing duplicate documentation.

```go
// doc.go - Has the package documentation
// Package config handles application configuration.
package config

// config.go - NO package comment, just package declaration
package config
```

### Existing doc.go Examples

- `internal/tui/doc.go` (9 lines): Brief, single paragraph
- `pkg/invowkfile/doc.go` (12 lines): Detailed with internal references
- `internal/discovery/doc.go`: Multi-concern package with file organization

## Struct Tags

Use JSON tags with snake_case, add mapstructure tags when using Viper:

```go
type Config struct {
    ContainerEngine ContainerEngine `json:"container_engine" mapstructure:"container_engine"`
    Includes        []IncludeEntry  `json:"includes" mapstructure:"includes"`
}
```

## Interface Design

Define interfaces for testability and extensibility:

```go
// Runtime defines the interface for command execution
type Runtime interface {
    Name() string
    Execute(ctx *ExecutionContext) *Result
    Available() bool
    Validate(ctx *ExecutionContext) error
}
```

## State and Dependency Patterns

### Prefer Explicit Dependencies Over Global Mutable State

Use constructor-injected dependencies and request-scoped data instead of package-level mutable variables.

- **Do not introduce mutable globals** for runtime flags, config paths, singleton servers, discovered command caches, or cross-command execution state.
- **Use composition roots** (`NewApp`, `NewService`, dependency structs, functional options) to wire concrete implementations.
- **Pass request-specific inputs explicitly** (e.g., `ExecuteRequest`, `LoadOptions`) instead of mutating shared package state.
- **Keep global vars immutable** (constants, build metadata) unless there is a compatibility reason that is documented and tested.

### Separate Orchestration from Domain Logic

- **CLI/transport adapters** (Cobra handlers, HTTP handlers, etc.) should parse input, build request structs, call services, and map errors/results to user output.
- **Domain/services** should return typed results/diagnostics and avoid terminal writes (`fmt.Print*`, direct stdout/stderr logging).
- **Rendering policy belongs at the boundary** (CLI layer), not inside discovery/config/runtime domain packages.

```go
// GOOD: explicit dependency injection and request-scoped execution.
type CommandService interface {
    Execute(ctx context.Context, req ExecuteRequest) (ExecuteResult, []Diagnostic, error)
}

// BAD: hidden mutable state shared by all command executions.
var currentConfigPath string
```

## Context Usage

Use `context.Context` for cancellation and timeouts in long-running operations:

```go
func (e *Engine) Run(ctx context.Context, opts RunOptions) (*RunResult, error) {
    // Check context before expensive operations
    select {
    case <-ctx.Done():
        return nil, ctx.Err()
    default:
    }
    // ... implementation
}
```

## Functional Options Pattern

Use functional options for constructors with optional configuration:

```go
type Option func(*Server)

func WithTimeout(d time.Duration) Option {
    return func(s *Server) { s.timeout = d }
}

func NewServer(addr string, opts ...Option) *Server {
    s := &Server{addr: addr, timeout: 30 * time.Second}
    for _, opt := range opts { opt(s) }
    return s
}
```

**When to use:** Constructor has >2-3 optional parameters, you want sensible defaults, or the API may evolve.

**When NOT to use:** All parameters are required, only 1-2 optional parameters, or performance is critical in hot paths.

**Naming conventions:** Option type: `Option` (or `<Type>Option`). Option functions: `With<Name>`. Document defaults in doc comments.

## Code Organization

### Declaration Order (decorder)

**CRITICAL: The `decorder` and `funcorder` linters MUST always remain enabled in `.golangci.toml`.**

Files should follow this structure:
```go
// SPDX-License-Identifier: MPL-2.0

// Package documentation
package mypackage

import (...)

const (...)

var (...)

type (...)

// Exported functions first
func PublicFunction() {}

// Unexported functions after
func privateFunction() {}
```

**Rules:**
- Never disable `decorder` or `funcorder` in `.golangci.toml`
- Never add `//nolint:decorder` or `//nolint:funcorder` directives
- When adding new code, follow the declaration order: `const`, `var`, `type`, `func`
- When adding new functions, place exported functions before unexported ones

### File Organization

- Keep files focused on a single responsibility.
- Separate test helpers into dedicated functions.
- Use `_test.go` suffix for test files only.
- Schema files use `.cue` extension (e.g., `config_schema.cue`, `invowkfile_schema.cue`).

### File Splitting Protocol

When splitting a large file into multiple focused files (e.g., decomposing a 975-line validator into concern-specific files), follow this strict protocol to avoid duplicate declaration errors:

1. **Read the source file** and identify logical groupings of declarations (types, methods, functions, variables).
2. **Create the new target files** with the moved declarations, including SPDX headers and necessary imports.
3. **Remove the moved declarations from the source file** — this step is critical and frequently forgotten. In Go, all files in a package share the same namespace, so duplicate function/method/variable declarations cause compiler errors.
4. **Clean up the source file's imports** — removing declarations often leaves unused imports behind.
5. **Build immediately** (`go build ./...`) to verify no duplicate declarations or missing imports.

**Why this matters:** Go's same-package namespace means the compiler catches duplicates instantly, but the error messages can be confusing when they reference "redeclared in this block" across two different files. The fix is always: ensure each declaration exists in exactly one file.

## Linter Configuration Documentation

**The `.golangci.toml` file must be kept documented when linters, formatters, or settings are added or changed.**

When modifying `.golangci.toml`:
1. **New linters**: Add a comment above each linter entry with its official description
2. **New settings**: Add inline comments explaining non-obvious configuration values
3. **Setting changes**: Update comments if the meaning or rationale changes
4. **Exclusions**: Document why specific rules, paths, or functions are excluded

```toml
[linters]
enable = [
    # modernize: A suite of analyzers that suggest simplifications to Go code.
    "modernize",
]

[linters.settings.example]
setting-name = "value"  # Brief explanation of what this controls
```

## Common Pitfalls

- **Silent close errors** - Use named returns with defer for resource cleanup.
- **Missing defaults documentation** - Document default values in functional options.
- **Wrong declaration order** - Follow const → var → type → func, exported before unexported.
- **`reflect.DeepEqual` for typed slices** - Use `slices.Equal` for `[]string`, `[]int`, etc. It's type-safe, gives better error messages, and avoids importing `reflect` in tests.
- **Duplicate package comments** - Use `doc.go` for package docs, remove `// Package` comments from other files.
- **Prohibited patterns in comments** - Guardrail tests (e.g., `TestNoGlobalConfigAccess`) scan all non-test `.go` files for banned call signatures using raw substring matching. Comments mentioning deprecated APIs must use indirect phrasing to avoid false positives.
- **Duplicate declarations after file splits** - When moving functions, methods, or variables from one file to another within the same package, always delete the originals from the source file and clean up orphaned imports. This is the most common mistake during file-splitting refactors.
- **`exhaustive` linter and enum switch cases** - The `exhaustive` linter requires ALL enum values to be explicitly listed in switch statements, even when a case returns the same value as `default`. Do not remove "redundant" zero-value cases (e.g., `SandboxNone`). The linter guarantees that adding new enum values in the future will be caught as missing cases.
- **`modernize` linter `fmtappendf`** - `[]byte(fmt.Sprintf(...))` should be `fmt.Appendf(nil, ...)`. The `modernize` linter flags this as `fmtappendf`. Same applies to `[]byte(fmt.Sprint(...))` → `fmt.Append(nil, ...)`.
