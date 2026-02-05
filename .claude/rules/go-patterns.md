---
description: Go style guide covering imports, naming, errors, interfaces, functional options, code organization
globs:
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
    "invowk-cli/internal/config"
    "invowk-cli/pkg/invkfile"
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

When functions open resources that need closing (files, connections, readers, writers), use **named return values** to aggregate close errors with the primary operation's error. This ensures close errors are never silently ignored.

```go
// CORRECT: Use named return to capture close error
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

// CORRECT: Multiple resources
func copyFile(src, dst string) (err error) {
    srcFile, err := os.Open(src)
    if err != nil {
        return err
    }
    defer func() {
        if closeErr := srcFile.Close(); closeErr != nil && err == nil {
            err = closeErr
        }
    }()

    dstFile, err := os.Create(dst)
    if err != nil {
        return err
    }
    defer func() {
        if closeErr := dstFile.Close(); closeErr != nil && err == nil {
            err = closeErr
        }
    }()

    _, err = io.Copy(dstFile, srcFile)
    return err
}
```

**Anti-patterns to avoid:**

```go
// WRONG: Error silently ignored
defer f.Close()

// WRONG: Error explicitly discarded without aggregation
defer func() { _ = f.Close() }()

// WRONG: Only logging (acceptable in rare cases with justification)
defer func() {
    if err := f.Close(); err != nil {
        log.Printf("close error: %v", err)
    }
}()
```

**When this pattern applies** - Use for any `io.Closer` or similar resource:
- `*os.File`, `*zip.ReadCloser`, `*zip.Writer`
- `net.Conn`, `*http.Response.Body`, `*sql.Rows`
- Custom types implementing `Close() error`

**Exceptions:**
1. **Test code**: Use test helpers (e.g., `testutil.MustClose(t, f)`) instead of named returns.
2. **Terminal operations in SSH sessions**: Where `sess.Exit()` errors cannot be meaningfully handled, use `_ =` with a comment explaining why.
3. **Best-effort cleanup after primary error**: When the function already has an error, logging the close error may be appropriate rather than overwriting the primary error.

## Documentation

- Every exported type, function, and constant MUST have a doc comment.
- Package comments should be in the format `// Package name description.`
- Use complete sentences starting with the item name.

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
- `pkg/invkfile/doc.go` (12 lines): Detailed with internal references
- `internal/discovery/doc.go`: Multi-concern package with file organization

## Struct Tags

Use JSON tags with snake_case, add mapstructure tags when using Viper:

```go
type Config struct {
    ContainerEngine ContainerEngine `json:"container_engine" mapstructure:"container_engine"`
    SearchPaths     []string        `json:"search_paths" mapstructure:"search_paths"`
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

Use the functional options pattern for constructors that accept optional configuration. This pattern provides sensible defaults, self-documenting APIs, and backward-compatible evolution.

Reference: [Dave Cheney - Functional options for friendly APIs](https://dave.cheney.net/2014/10/17/functional-options-for-friendly-apis)

### Pattern Structure

```go
// 1. Define the Option Type
type Option func(*Server)

// 2. Create With* Functions
func WithTimeout(d time.Duration) Option {
    return func(s *Server) {
        s.timeout = d
    }
}

func WithLogger(logger *slog.Logger) Option {
    return func(s *Server) {
        s.logger = logger
    }
}

// 3. Constructor with Variadic Options
func NewServer(addr string, opts ...Option) *Server {
    s := &Server{
        addr:     addr,
        timeout:  30 * time.Second,  // sensible default
        maxConns: 100,               // sensible default
        logger:   slog.Default(),    // sensible default
    }
    for _, opt := range opts {
        opt(s)
    }
    return s
}
```

**Usage:**

```go
// Default configuration
server := NewServer("localhost:8080")

// With custom configuration
server := NewServer("localhost:8080",
    WithTimeout(60 * time.Second),
    WithLogger(customLogger),
)

// Conditional options
opts := []Option{WithTimeout(60 * time.Second)}
if enableTLS {
    opts = append(opts, WithTLS(certPath, keyPath))
}
server := NewServer(addr, opts...)
```

**When to use:** Constructor has >2-3 optional parameters, you want sensible defaults, or the API may evolve.

**When NOT to use:** All parameters are required, only 1-2 optional parameters, or performance is critical in hot paths.

**Naming conventions:**
- Option type: `Option` (or `<Type>Option` if multiple exist)
- Option functions: `With<OptionName>` (e.g., `WithTimeout`, `WithLogger`)
- Document default values in doc comments

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
- Schema files use `.cue` extension (e.g., `config_schema.cue`, `invkfile_schema.cue`).

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
- **Duplicate package comments** - Use `doc.go` for package docs, remove `// Package` comments from other files.
