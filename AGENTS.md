# AGENTS.md - Coding Agent Guidelines for invowk-cli

This document provides instructions for AI coding agents working in this repository.

## Project Overview

Invowk is a dynamically extensible command runner (like `just`) written in Go 1.25+. It supports multiple execution runtimes: native shell, virtual shell (mvdan/sh), and containerized execution (Docker/Podman). Commands are defined in `invowkfile` files using CUE format.

## Build Commands

```bash
# Build the binary (default, stripped)
make build

# Build with debug symbols for development
make build-dev

# Build with UPX compression (smallest size, requires UPX)
make build-upx

# Build all variants
make build-all

# Install to $GOPATH/bin
make install

# Clean build artifacts
make clean

# Tidy dependencies
make tidy
```

## Test Commands

```bash
# Run all tests (verbose)
make test

# Run tests in short mode (skips integration tests)
make test-short

# Run integration tests only
make test-integration

# Run a single test by name
go test -v -run TestFunctionName ./path/to/package/...

# Run a single test file
go test -v ./internal/config/config_test.go ./internal/config/config.go

# Run tests with coverage
go test -v -cover ./...

# Run tests for a specific package
go test -v ./internal/runtime/...
go test -v ./pkg/invowkfile/...
```

## Code Style Guidelines

### Package Structure

- `cmd/invowk/` - CLI commands using Cobra
- `internal/` - Private packages (config, container, discovery, issue, runtime, sshserver, tui)
- `pkg/` - Public packages (invowkfile)

### Import Ordering

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
    "invowk-cli/pkg/invowkfile"
)
```

### Naming Conventions

- **Packages**: lowercase, single word preferred (e.g., `config`, `runtime`, `discovery`)
- **Exported types**: PascalCase (e.g., `Config`, `RuntimeMode`, `ExecutionContext`)
- **Unexported types/vars**: camelCase (e.g., `globalConfig`, `configPath`)
- **Constants**: PascalCase for exported, camelCase for unexported
- **Interfaces**: Use action-oriented names (e.g., `Engine`, `Runtime`)
- **Error types**: Prefix with `Err` (e.g., `ErrEngineNotAvailable`)

### Error Handling

- Always wrap errors with context using `fmt.Errorf("context: %w", err)`
- Use custom error types for specific error cases that callers need to handle
- Return early on errors to reduce nesting

```go
// Good
if err := doSomething(); err != nil {
    return fmt.Errorf("failed to do something: %w", err)
}

// Custom error type example
type ErrEngineNotAvailable struct {
    Engine string
    Reason string
}

func (e *ErrEngineNotAvailable) Error() string {
    return fmt.Sprintf("container engine '%s' is not available: %s", e.Engine, e.Reason)
}
```

### Documentation

- Every exported type, function, and constant MUST have a doc comment
- Package comments should be in the format `// Package name description.`
- Use complete sentences starting with the item name

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

### Struct Tags

Use JSON tags with snake_case, add mapstructure tags when using Viper:

```go
type Config struct {
    ContainerEngine ContainerEngine `json:"container_engine" mapstructure:"container_engine"`
    SearchPaths     []string        `json:"search_paths" mapstructure:"search_paths"`
}
```

### Testing Patterns

- Test files are named `*_test.go` in the same package
- Use `t.TempDir()` for temporary directories (auto-cleaned)
- Use table-driven tests for multiple cases
- Skip integration tests with `if testing.Short() { t.Skip(...) }`
- Reset global state in tests using cleanup functions

```go
func TestExample(t *testing.T) {
    // Setup
    tmpDir := t.TempDir()
    originalEnv := os.Getenv("VAR")
    defer os.Setenv("VAR", originalEnv)

    // Test
    result, err := DoSomething()
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }

    // Assert
    if result != expected {
        t.Errorf("got %v, want %v", result, expected)
    }
}
```

## Built-in Examples of Invowk Commands (invowkfile.cue at project root)

- Always update the example file when there are any invowkfile definition changes or features added/modified/removed.
- All commands should be idempotent and not cause any side effects on the host.
- No commands should be related to building invowk itself or manipulating any of its source code.
- Examples should range from simple (e.g.: native 'hello-world') to complex (e.g.: container 'hello-world' with the enable_host_ssh feature).
- Examples should illustrate the use of different features of Invowk, such as:
  - Native vs. Container execution
  - Volume mounts for Container execution
  - Environment variables
  - Host SSH access enabled vs. disabled
  - Capabilities checks (with and without alternatives)
  - Tools checks (with and without alternatives)
  - Custom checks (with and without alternatives)

## Key Guidelines

- In all planning and design decisions, always consider that the code must be highly testable, maintainable, and extensible.
- Always add unit and integration tests to new code.
- Always document the code (functions, structs, etc.) with comments.
- Always use descriptive variable names.
- Always adjust the README and other documentation as needed when making significant changes to the codebase.
- Always refactor unit and integration tests when needed after code changes, considering both the design and semantics of the code changes.
- After you finish code design and implementation changes, always double-check for leftovers that were not removed or changed after refactoring (e.g.: tests, CUE type definitions, README or documentation instructions, etc.).
- Always follow the best practices for the programming language being used.

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

- Keep files focused on a single responsibility
- Separate test helpers into dedicated functions
- Use `_test.go` suffix for test files only
- Schema files use `.cue` extension (e.g., `config_schema.cue`, `invowkfile_schema.cue`)

## Dependencies

Key dependencies:
- `github.com/spf13/cobra` - CLI framework
- `github.com/spf13/viper` - Configuration management
- `cuelang.org/go` - CUE language support for configuration/schema
- `github.com/charmbracelet/*` - TUI components (lipgloss, bubbletea, huh)
- `mvdan.cc/sh/v3` - Virtual shell implementation
