# Go Style

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

## File Organization

- Keep files focused on a single responsibility.
- Separate test helpers into dedicated functions.
- Use `_test.go` suffix for test files only.
- Schema files use `.cue` extension (e.g., `config_schema.cue`, `invkfile_schema.cue`).
