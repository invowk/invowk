# Issue: Extract EnvBuilder Interface

**Category**: Architecture
**Priority**: Medium
**Effort**: Medium (2-3 days)
**Labels**: `architecture`, `refactoring`, `runtime`
**Depends On**: #006 (ExecutionContext decomposition)

## Summary

Extract environment building logic from scattered files (`env.go`, `dotenv.go`, `runtime.go`) into a well-defined `EnvBuilder` interface. This clarifies the complex 10-level precedence hierarchy and improves testability.

## Problem

Environment building logic is currently:
1. **Scattered** across 3+ files with complex interactions
2. **Hard to understand** - 10-level precedence is implicit in code flow
3. **Difficult to test** - can't easily verify precedence behavior
4. **Tightly coupled** - all runtimes directly call `buildRuntimeEnv()`

**Current files**:
- `internal/runtime/env.go` - `buildRuntimeEnv()` with 10 precedence levels (lines 19-81)
- `internal/runtime/dotenv.go` - dotenv file loading
- `internal/runtime/runtime.go` - env filtering (remove INVOWK_* vars)

## Solution

Create explicit `EnvBuilder` interface with documented precedence:

### Interface Definition

```go
// internal/runtime/env_builder.go

// SPDX-License-Identifier: MPL-2.0

package runtime

import (
    "context"
)

// EnvBuilder constructs the environment for command execution.
type EnvBuilder interface {
    // Build returns the environment map for the given context.
    // The returned map follows the precedence rules defined by the implementation.
    Build(ctx context.Context, execCtx *ExecutionContext) (map[string]string, error)
}

// DotenvLoader loads environment variables from .env files.
type DotenvLoader interface {
    // Load reads environment variables from the specified files.
    // Files are processed in order; later files override earlier ones.
    Load(paths []string, workDir string) (map[string]string, error)
}
```

### Default Implementation

```go
// DefaultEnvBuilder implements the standard environment building logic.
//
// Precedence (highest to lowest):
//  1. ExtraEnv from ExecutionContext
//  2. RuntimeEnvVars from command definition
//  3. RuntimeEnvFiles (.env files from command)
//  4. Command-level env from invowkfile
//  5. Invowkfile-level env
//  6. Module-level env (if in module context)
//  7. User config env
//  8. System environment (if inherit mode allows)
//  9. Default values from schema
// 10. Filtered INVOWK_* variables (excluded unless explicitly allowed)
type DefaultEnvBuilder struct {
    dotenvLoader DotenvLoader
    configEnv    map[string]string // From user config
}

// NewDefaultEnvBuilder creates an EnvBuilder with the standard precedence rules.
func NewDefaultEnvBuilder(dotenvLoader DotenvLoader, configEnv map[string]string) *DefaultEnvBuilder {
    return &DefaultEnvBuilder{
        dotenvLoader: dotenvLoader,
        configEnv:    configEnv,
    }
}

// Build constructs the environment following the 10-level precedence.
func (b *DefaultEnvBuilder) Build(ctx context.Context, execCtx *ExecutionContext) (map[string]string, error) {
    env := make(map[string]string)

    // Level 10: Start with filtered system env (if inherit mode allows)
    if execCtx.Env.InheritMode == EnvInheritAll {
        for _, e := range os.Environ() {
            if k, v, ok := strings.Cut(e, "="); ok {
                if !isInvowkVar(k) { // Filter INVOWK_* vars
                    env[k] = v
                }
            }
        }
    }

    // Level 9: Default values from schema (if any)
    // ...

    // Level 8: User config env
    for k, v := range b.configEnv {
        env[k] = v
    }

    // Level 7: Module-level env (if in module context)
    if execCtx.Module != nil && execCtx.Module.Env != nil {
        for k, v := range execCtx.Module.Env {
            env[k] = v
        }
    }

    // Level 6: Invowkfile-level env
    if execCtx.Invowkfile != nil && execCtx.Invowkfile.Env != nil {
        for k, v := range execCtx.Invowkfile.Env {
            env[k] = v
        }
    }

    // Level 5: Command-level env
    if execCtx.Command != nil && execCtx.Command.Env != nil {
        for k, v := range execCtx.Command.Env {
            env[k] = v
        }
    }

    // Level 4: Implementation-level env
    if execCtx.SelectedImpl != nil && execCtx.SelectedImpl.Env != nil {
        for k, v := range execCtx.SelectedImpl.Env {
            env[k] = v
        }
    }

    // Level 3: RuntimeEnvFiles (.env files)
    if len(execCtx.Env.RuntimeEnvFiles) > 0 {
        dotenv, err := b.dotenvLoader.Load(execCtx.Env.RuntimeEnvFiles, execCtx.WorkDir)
        if err != nil {
            return nil, fmt.Errorf("loading env files: %w", err)
        }
        for k, v := range dotenv {
            env[k] = v
        }
    }

    // Level 2: RuntimeEnvVars from command definition
    for k, v := range execCtx.Env.RuntimeEnvVars {
        env[k] = v
    }

    // Level 1: ExtraEnv (highest precedence)
    for k, v := range execCtx.Env.ExtraEnv {
        env[k] = v
    }

    return env, nil
}

func isInvowkVar(key string) bool {
    return strings.HasPrefix(key, "INVOWK_")
}
```

### Dotenv Loader Implementation

```go
// internal/runtime/dotenv_loader.go

// DefaultDotenvLoader loads .env files using godotenv.
type DefaultDotenvLoader struct{}

func (l *DefaultDotenvLoader) Load(paths []string, workDir string) (map[string]string, error) {
    result := make(map[string]string)

    for _, path := range paths {
        fullPath := path
        if !filepath.IsAbs(path) {
            fullPath = filepath.Join(workDir, path)
        }

        envMap, err := godotenv.Read(fullPath)
        if err != nil {
            return nil, fmt.Errorf("reading %s: %w", path, err)
        }

        for k, v := range envMap {
            result[k] = v
        }
    }

    return result, nil
}
```

### Mock for Testing

```go
// internal/runtime/env_builder_test.go

// MockEnvBuilder for testing runtimes without real env building.
type MockEnvBuilder struct {
    Env map[string]string
    Err error
}

func (m *MockEnvBuilder) Build(_ context.Context, _ *ExecutionContext) (map[string]string, error) {
    return m.Env, m.Err
}

// MockDotenvLoader for testing without filesystem.
type MockDotenvLoader struct {
    Envs map[string]map[string]string // path -> env vars
    Err  error
}

func (m *MockDotenvLoader) Load(paths []string, _ string) (map[string]string, error) {
    if m.Err != nil {
        return nil, m.Err
    }
    result := make(map[string]string)
    for _, path := range paths {
        if envMap, ok := m.Envs[path]; ok {
            for k, v := range envMap {
                result[k] = v
            }
        }
    }
    return result, nil
}
```

## Files to Modify

| File | Changes |
|------|---------|
| `internal/runtime/env_builder.go` | NEW - Interface and default implementation |
| `internal/runtime/dotenv_loader.go` | NEW - Dotenv loader interface and implementation |
| `internal/runtime/env.go` | Refactor to use EnvBuilder (may be removed) |
| `internal/runtime/dotenv.go` | Refactor to use DotenvLoader (may be removed) |
| `internal/runtime/native.go` | Accept EnvBuilder in constructor, use in Execute |
| `internal/runtime/virtual.go` | Accept EnvBuilder in constructor, use in Execute |
| `internal/runtime/container.go` | Accept EnvBuilder in constructor, use in Execute |
| `internal/runtime/runtime.go` | Update NewRuntime to create/inject EnvBuilder |

## Implementation Steps

1. [ ] Create `env_builder.go` with interface definitions
2. [ ] Create `dotenv_loader.go` with loader interface
3. [ ] Implement `DefaultEnvBuilder` with documented precedence
4. [ ] Implement `DefaultDotenvLoader`
5. [ ] Add comprehensive tests for all 10 precedence levels
6. [ ] Create mock implementations for testing
7. [ ] Update `NativeRuntime` to accept and use `EnvBuilder`
8. [ ] Update `VirtualRuntime` to accept and use `EnvBuilder`
9. [ ] Update `ContainerRuntime` to accept and use `EnvBuilder`
10. [ ] Update `NewRuntime()` to wire up dependencies
11. [ ] Remove or refactor old `env.go` and `dotenv.go`
12. [ ] Update all tests

## Acceptance Criteria

- [ ] `EnvBuilder` interface defined with clear contract
- [ ] `DotenvLoader` interface defined
- [ ] `DefaultEnvBuilder` implements all 10 precedence levels
- [ ] Precedence levels documented in code comments
- [ ] Comprehensive tests cover each precedence level
- [ ] Mock implementations available for testing
- [ ] All runtimes use `EnvBuilder` interface
- [ ] Old scattered code refactored or removed
- [ ] `make lint` passes
- [ ] `make test` passes

## Testing

```bash
# Run env builder tests
go test -v ./internal/runtime/... -run EnvBuilder

# Run all runtime tests
go test -v ./internal/runtime/...

# Verify precedence with integration tests
make test-cli
```

## Notes

- This is a prerequisite for easier runtime testing
- The interface allows injecting mock builders in tests
- Precedence documentation becomes authoritative (in code, not just docs)
- Consider using functional options for `DefaultEnvBuilder` configuration
