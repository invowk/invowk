# Issue: Add Unit Tests for Runtime Package

**Category**: Testability
**Priority**: High
**Effort**: High (3-5 days)
**Labels**: `testing`, `runtime`

## Summary

The `internal/runtime/` package has 9 implementation files without corresponding test files. This is a critical package that handles all command execution, and lack of tests makes refactoring risky.

## Problem

**Files without tests**:

| File | Lines | Complexity | Test Priority |
|------|-------|------------|---------------|
| `runtime.go` | ~150 | Medium | High - Core dispatcher |
| `native.go` | ~430 | High | High - Primary runtime |
| `virtual.go` | ~400 | High | High - Alternative runtime |
| `container.go` | ~400 | High | Medium - Requires mocks |
| `env.go` | ~100 | Medium | High - Complex precedence |
| `native_helpers.go` | ~50 | Low | Low |
| `container_exec.go` | ~150 | Medium | Medium |
| `container_prepare.go` | ~200 | Medium | Medium |
| `container_provision.go` | ~250 | High | Medium |
| `provision_layer.go` | ~100 | Medium | Low |

**Current test coverage**: ~10% (estimated)
**Target test coverage**: >70%

## Solution

Create comprehensive unit tests in phases, starting with the most critical and testable components.

### Phase 1: Environment Building (`env_test.go`)

Test the 10-level precedence hierarchy in `buildRuntimeEnv()`:

```go
func TestBuildRuntimeEnv(t *testing.T) {
    tests := []struct {
        name    string
        ctx     *ExecutionContext
        wantEnv map[string]string
        wantErr bool
    }{
        {
            name: "empty context inherits system env",
            ctx: &ExecutionContext{
                EnvInheritMode: EnvInheritAll,
            },
            wantEnv: os.Environ(), // Simplified
        },
        {
            name: "ExtraEnv overrides inherited",
            ctx: &ExecutionContext{
                EnvInheritMode: EnvInheritAll,
                ExtraEnv:       map[string]string{"PATH": "/custom"},
            },
            wantEnv: map[string]string{"PATH": "/custom"},
        },
        {
            name: "EnvInheritNone excludes system env",
            ctx: &ExecutionContext{
                EnvInheritMode: EnvInheritNone,
                ExtraEnv:       map[string]string{"FOO": "bar"},
            },
            wantEnv: map[string]string{"FOO": "bar"},
        },
        // Test all 10 precedence levels...
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := buildRuntimeEnv(tt.ctx)
            if (err != nil) != tt.wantErr {
                t.Errorf("buildRuntimeEnv() error = %v, wantErr %v", err, tt.wantErr)
            }
            // Assert environment matches expected
        })
    }
}
```

**Test cases for env.go**:
- [ ] `EnvInheritAll` includes system environment
- [ ] `EnvInheritNone` excludes system environment
- [ ] `EnvInheritExplicit` with whitelist
- [ ] `ExtraEnv` overrides all other sources
- [ ] `RuntimeEnvVars` from command definition
- [ ] `RuntimeEnvFiles` (.env file loading)
- [ ] Command-level env from invowkfile
- [ ] Invowkfile-level env
- [ ] INVOWK_* variables are filtered correctly
- [ ] Precedence order is correct (higher overrides lower)

### Phase 2: Core Runtime (`runtime_test.go`)

```go
func TestNewRuntime(t *testing.T) {
    tests := []struct {
        name        string
        mode        RuntimeMode
        wantType    string
        wantAvail   bool
    }{
        {"native runtime", RuntimeNative, "*NativeRuntime", true},
        {"virtual runtime", RuntimeVirtual, "*VirtualRuntime", true},
        {"container runtime", RuntimeContainer, "*ContainerRuntime", true}, // May vary
    }
    // ...
}

func TestRuntimeExecute(t *testing.T) {
    // Test dispatcher routes to correct runtime
}
```

**Test cases for runtime.go**:
- [ ] `NewRuntime()` returns correct type for each mode
- [ ] `Execute()` dispatches to correct runtime
- [ ] `Available()` returns correct status
- [ ] Runtime selection with fallback

### Phase 3: Native Runtime (`native_test.go`)

```go
func TestNativeRuntime_ScriptResolution(t *testing.T) {
    tests := []struct {
        name       string
        impl       *invowkfile.Implementation
        wantScript string
        wantErr    bool
    }{
        {
            name: "inline script",
            impl: &invowkfile.Implementation{
                Script: "echo hello",
            },
            wantScript: "echo hello",
        },
        {
            name: "file reference",
            impl: &invowkfile.Implementation{
                File: "scripts/build.sh",
            },
            wantScript: "#!/bin/bash\necho building...",
        },
    }
    // ...
}

func TestNativeRuntime_InterpreterDetection(t *testing.T) {
    tests := []struct {
        name       string
        shebang    string
        wantInterp string
    }{
        {"bash shebang", "#!/bin/bash", "bash"},
        {"sh shebang", "#!/bin/sh", "sh"},
        {"env bash", "#!/usr/bin/env bash", "bash"},
        {"zsh shebang", "#!/bin/zsh", "zsh"},
        {"no shebang defaults to sh", "", "sh"},
    }
    // ...
}
```

**Test cases for native.go**:
- [ ] Script resolution from `Implementation.Script`
- [ ] Script resolution from `Implementation.File`
- [ ] Interpreter detection from shebang
- [ ] Shell command construction
- [ ] Output streaming mode
- [ ] Output capture mode
- [ ] Exit code extraction
- [ ] Error handling for missing scripts

### Phase 4: Virtual Runtime (`virtual_test.go`)

**Test cases for virtual.go**:
- [ ] mvdan/sh interpreter initialization
- [ ] Builtin command execution
- [ ] Script execution
- [ ] Environment inheritance
- [ ] Exit code handling

### Phase 5: Container Runtime (`container_test.go`)

Requires mocking the `Engine` interface:

```go
type mockEngine struct {
    availableResult bool
    runResult       *RunResult
    runError        error
    buildCalled     bool
    runArgs         []string
}

func (m *mockEngine) Available() bool { return m.availableResult }
func (m *mockEngine) Run(ctx context.Context, opts RunOptions) (*RunResult, error) {
    m.runArgs = opts.Args
    return m.runResult, m.runError
}
// ... other methods

func TestContainerRuntime_Execute(t *testing.T) {
    engine := &mockEngine{
        availableResult: true,
        runResult:       &RunResult{ExitCode: 0},
    }
    runtime := NewContainerRuntime(engine)

    ctx := &ExecutionContext{...}
    result := runtime.Execute(ctx)

    // Assert engine.Run was called with correct arguments
}
```

**Test cases for container.go**:
- [ ] Image preparation from base image
- [ ] Image preparation from Containerfile
- [ ] Provisioning layer creation
- [ ] Run argument construction
- [ ] SSH callback setup
- [ ] Exit code handling
- [ ] Error handling for unavailable engine

## Implementation Steps

1. [ ] **Phase 1**: Create `env_test.go` with precedence tests
2. [ ] **Phase 2**: Create `runtime_test.go` with dispatcher tests
3. [ ] **Phase 3**: Create `native_test.go` with execution tests
4. [ ] **Phase 4**: Create `virtual_test.go` with interpreter tests
5. [ ] **Phase 5**: Create `container_test.go` with mocked engine
6. [ ] Add test helpers for common setup patterns
7. [ ] Verify coverage meets target (>70%)

## Acceptance Criteria

- [ ] `env_test.go` covers all 10 precedence levels
- [ ] `runtime_test.go` covers dispatcher and registry
- [ ] `native_test.go` covers script resolution and execution
- [ ] `virtual_test.go` covers interpreter behavior
- [ ] `container_test.go` covers with mocked engine
- [ ] Test coverage for runtime package > 70%
- [ ] All tests pass on Linux, macOS, and Windows
- [ ] `make test` passes

## Testing

```bash
# Run runtime tests
go test -v ./internal/runtime/...

# Check coverage
go test -cover ./internal/runtime/...

# Generate coverage report
go test -coverprofile=coverage.out ./internal/runtime/...
go tool cover -html=coverage.out
```

## Notes

- Start with `env_test.go` as it has the clearest inputs/outputs
- Container tests require careful mocking to avoid actual Docker/Podman calls
- Consider using `testify/mock` or hand-rolled mocks for Engine interface
- Some native/virtual tests may need platform-specific handling (Windows)
