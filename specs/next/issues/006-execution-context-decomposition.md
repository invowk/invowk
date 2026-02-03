# Issue: Decompose ExecutionContext into Focused Types

**Category**: Testability
**Priority**: High
**Effort**: Medium (2-3 days)
**Labels**: `refactoring`, `architecture`, `runtime`

## Summary

`ExecutionContext` in `internal/runtime/runtime.go` has 17 fields serving different concerns, making it a "god object" that's difficult to test and understand. Decompose it into smaller, focused types.

## Problem

**Current Structure** (lines 24-77 of `runtime.go`):

```go
type ExecutionContext struct {
    // I/O (3 fields)
    Stdout io.Writer
    Stderr io.Writer
    Stdin  io.Reader

    // Environment (5 fields)
    ExtraEnv          map[string]string
    RuntimeEnvVars    map[string]string
    RuntimeEnvFiles   []string
    EnvInheritMode    EnvInheritMode
    EnvInheritModeSet bool

    // Execution control (3 fields)
    Context         context.Context
    SelectedRuntime RuntimeMode
    PositionalArgs  []string

    // Configuration (3 fields)
    WorkDir      string
    SelectedImpl *invkfile.Implementation
    Command      *invkfile.Command

    // TUI integration (2 fields)
    TUIServerURL   string
    TUIServerToken string

    // Plus more...
}
```

**Issues**:
1. Hard to construct in tests (many fields to populate)
2. Unclear which fields are required vs optional
3. No grouping of related fields
4. Changes to one concern affect the whole struct

## Solution

Decompose into focused sub-types using composition:

```go
// IOContext holds I/O streams for command execution.
type IOContext struct {
    Stdout io.Writer
    Stderr io.Writer
    Stdin  io.Reader
}

// EnvContext holds environment configuration for command execution.
type EnvContext struct {
    // ExtraEnv provides additional environment variables with highest precedence.
    ExtraEnv map[string]string

    // RuntimeEnvVars are variables defined in the command's runtime section.
    RuntimeEnvVars map[string]string

    // RuntimeEnvFiles are .env files to load for the command.
    RuntimeEnvFiles []string

    // InheritMode controls how system environment is inherited.
    InheritMode EnvInheritMode

    // InheritModeSet indicates if InheritMode was explicitly set.
    InheritModeSet bool
}

// TUIContext holds TUI server connection details for interactive execution.
type TUIContext struct {
    // ServerURL is the TUI server endpoint (empty if not using TUI).
    ServerURL string

    // ServerToken is the authentication token for the TUI server.
    ServerToken string
}

// ExecutionContext holds the complete context for command execution.
type ExecutionContext struct {
    // Context provides cancellation and deadline support.
    Context context.Context

    // Command is the command definition from the invkfile.
    Command *invkfile.Command

    // Invkfile is the parsed invkfile containing the command.
    Invkfile *invkfile.Invkfile

    // SelectedImpl is the specific implementation to execute.
    SelectedImpl *invkfile.Implementation

    // IO holds I/O stream configuration.
    IO IOContext

    // Env holds environment configuration.
    Env EnvContext

    // TUI holds TUI server connection details (optional).
    TUI TUIContext

    // SelectedRuntime specifies which runtime to use.
    SelectedRuntime RuntimeMode

    // PositionalArgs are arguments passed to the command.
    PositionalArgs []string

    // WorkDir is the working directory for execution.
    WorkDir string
}
```

### Helper Methods

Add convenience methods to improve usability:

```go
// HasTUI returns true if TUI server is configured.
func (ctx *ExecutionContext) HasTUI() bool {
    return ctx.TUI.ServerURL != ""
}

// DefaultIO returns IOContext with standard streams.
func DefaultIO() IOContext {
    return IOContext{
        Stdout: os.Stdout,
        Stderr: os.Stderr,
        Stdin:  os.Stdin,
    }
}

// WithEnv returns a copy of EnvContext with additional variables.
func (e EnvContext) WithEnv(extra map[string]string) EnvContext {
    merged := make(map[string]string, len(e.ExtraEnv)+len(extra))
    for k, v := range e.ExtraEnv {
        merged[k] = v
    }
    for k, v := range extra {
        merged[k] = v
    }
    return EnvContext{
        ExtraEnv:        merged,
        RuntimeEnvVars:  e.RuntimeEnvVars,
        RuntimeEnvFiles: e.RuntimeEnvFiles,
        InheritMode:     e.InheritMode,
        InheritModeSet:  e.InheritModeSet,
    }
}
```

## Files to Modify

### Primary Changes

| File | Changes |
|------|---------|
| `internal/runtime/runtime.go` | Add sub-types, refactor ExecutionContext |
| `internal/runtime/env.go` | Update to use `ctx.Env.*` |
| `internal/runtime/native.go` | Update field references |
| `internal/runtime/virtual.go` | Update field references |
| `internal/runtime/container.go` | Update field references |
| `internal/runtime/interactive.go` | Update field references |

### Callers to Update

Search for `ExecutionContext{` and `&ExecutionContext{`:

| File | Changes |
|------|---------|
| `cmd/invowk/cmd_run.go` | Update context construction |
| `cmd/invowk/tui_interactive.go` | Update context construction |
| Test files | Update test context construction |

## Implementation Steps

1. [ ] **Define sub-types** in `runtime.go`:
   - Add `IOContext` type with doc comments
   - Add `EnvContext` type with doc comments
   - Add `TUIContext` type with doc comments

2. [ ] **Update ExecutionContext**:
   - Embed sub-types
   - Add helper methods
   - Update doc comments

3. [ ] **Update internal references** (search and replace):
   - `ctx.Stdout` → `ctx.IO.Stdout`
   - `ctx.Stderr` → `ctx.IO.Stderr`
   - `ctx.Stdin` → `ctx.IO.Stdin`
   - `ctx.ExtraEnv` → `ctx.Env.ExtraEnv`
   - `ctx.RuntimeEnvVars` → `ctx.Env.RuntimeEnvVars`
   - `ctx.RuntimeEnvFiles` → `ctx.Env.RuntimeEnvFiles`
   - `ctx.EnvInheritMode` → `ctx.Env.InheritMode`
   - `ctx.EnvInheritModeSet` → `ctx.Env.InheritModeSet`
   - `ctx.TUIServerURL` → `ctx.TUI.ServerURL`
   - `ctx.TUIServerToken` → `ctx.TUI.ServerToken`

4. [ ] **Update callers** in `cmd/`:
   - Update `ExecutionContext{}` literals
   - Use sub-type constructors where appropriate

5. [ ] **Update tests**:
   - Simplify test context construction
   - Use helper methods

6. [ ] **Verify all builds and tests pass**

## Migration Example

**Before**:
```go
ctx := &ExecutionContext{
    Context:           context.Background(),
    Stdout:            os.Stdout,
    Stderr:            os.Stderr,
    Stdin:             os.Stdin,
    ExtraEnv:          map[string]string{"FOO": "bar"},
    EnvInheritMode:    EnvInheritAll,
    SelectedRuntime:   RuntimeNative,
    Command:           cmd,
    Invkfile:          inv,
}
```

**After**:
```go
ctx := &ExecutionContext{
    Context:         context.Background(),
    IO:              DefaultIO(),
    Env: EnvContext{
        ExtraEnv:     map[string]string{"FOO": "bar"},
        InheritMode:  EnvInheritAll,
    },
    SelectedRuntime: RuntimeNative,
    Command:         cmd,
    Invkfile:        inv,
}
```

## Acceptance Criteria

- [ ] `IOContext`, `EnvContext`, `TUIContext` types created with doc comments
- [ ] `ExecutionContext` uses composition
- [ ] Helper methods added (`HasTUI()`, `DefaultIO()`, etc.)
- [ ] All internal references updated
- [ ] All callers in `cmd/` updated
- [ ] All tests updated and passing
- [ ] `make lint` passes
- [ ] `make test` passes
- [ ] API is clearer and more self-documenting

## Testing

```bash
# Verify compilation
go build ./...

# Run all tests
make test

# Verify no regressions in CLI behavior
make test-cli
```

## Notes

- This is a breaking change for any external code using `ExecutionContext`
- Consider adding deprecation aliases if backward compatibility is needed
- The new structure makes it easier to:
  - Test environment building in isolation
  - Mock I/O for testing
  - Add new context types without growing the main struct
