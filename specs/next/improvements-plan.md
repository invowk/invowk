# Invowk Improvements Plan

This document provides a detailed implementation plan for codebase improvements identified during architectural analysis. Items are organized by category and priority.

> **Last updated**: 2026-02-06.
> Several items were completed by the spec-008 stateless composition refactoring and spec-005 u-root implementation. Completed items are marked with **(COMPLETED)** in their headings.

**Legend:**
- **Effort**: Low (< 1 day), Medium (1-3 days), High (> 3 days)
- **Impact**: Low (internal cleanup), Medium (improves DX/maintainability), High (enables new capabilities)

---

## Category 1: Quick Wins (Error Handling & Code Quality)

These items have low implementation effort but high value in terms of code consistency and adherence to project standards.

### 1.1 Fix Error Handling in `internal/uroot/` Commands

**Priority**: High
**Effort**: Low
**Impact**: Medium (consistency with project standards)

**Problem**: All 7 utility commands use naked `f.Close()` calls without error handling, violating `.claude/rules/go-patterns.md` patterns.

**Files to Modify**:

| File | Lines | Current Pattern |
|------|-------|-----------------|
| `internal/uroot/head.go` | 83, 86 | `f.Close()` |
| `internal/uroot/tail.go` | 87, 90 | `f.Close()` |
| `internal/uroot/grep.go` | 118 | `f.Close()` |
| `internal/uroot/cut.go` | 107 | `f.Close()` |
| `internal/uroot/sort.go` | 97 | `f.Close()` |
| `internal/uroot/wc.go` | 112 | `f.Close()` |
| `internal/uroot/uniq.go` | 77 | `f.Close()` |

**Implementation Steps**:

1. **Decide on pattern** - Since these are read-only file operations, use the explicit discard pattern with comment:
   ```go
   defer func() { _ = f.Close() }() // Read-only file; close error non-critical
   ```

2. **Update each file**:
   - Replace `f.Close()` with deferred close pattern
   - Add justification comment explaining why error is discarded
   - Ensure consistent placement (immediately after successful `os.Open()`)

3. **Verify consistency** with existing patterns:
   - `cmd/invowk/tui_table.go:72` - Uses `defer func() { _ = f.Close() }()` with comment
   - `internal/runtime/provision.go:98,172` - Same pattern

**Acceptance Criteria**:
- [ ] All 7 files use consistent close pattern
- [ ] Each close has explanatory comment
- [ ] `make lint` passes
- [ ] `make test` passes

---

### 1.2 Extract File Processing Helper in `internal/uroot/`

**Priority**: Medium
**Effort**: Low
**Impact**: Medium (reduces code duplication)

**Problem**: All 7 utility commands repeat identical file opening/processing boilerplate (~30 lines each).

**Current Duplicated Pattern** (in all 7 files):
```go
files := fs.Args()
if len(files) == 0 {
    // Handle stdin
    return processReader(ctx.Stdin, ...)
} else {
    for _, file := range files {
        path := file
        if !filepath.IsAbs(path) {
            path = filepath.Join(hc.Dir, path)
        }
        f, err := os.Open(path)
        if err != nil {
            return wrapError(c.name, err)
        }
        // Process file
        f.Close()
    }
}
return nil
```

**Implementation Steps**:

1. **Create new helper file** `internal/uroot/files.go`:
   ```go
   // SPDX-License-Identifier: MPL-2.0

   package uroot

   import (
       "io"
       "os"
       "path/filepath"
   )

   // FileProcessor processes a single reader and returns any error.
   type FileProcessor func(r io.Reader, filename string) error

   // ProcessFilesOrStdin processes files from args or stdin if no files given.
   // The processor is called for each file (or stdin) with the reader and filename.
   // For stdin, filename is "-".
   func ProcessFilesOrStdin(
       args []string,
       stdin io.Reader,
       workDir string,
       cmdName string,
       processor FileProcessor,
   ) error {
       if len(args) == 0 {
           return processor(stdin, "-")
       }

       for _, file := range args {
           path := file
           if !filepath.IsAbs(path) {
               path = filepath.Join(workDir, path)
           }

           f, err := os.Open(path)
           if err != nil {
               return wrapError(cmdName, err)
           }

           processErr := processor(f, file)
           _ = f.Close() // Read-only file; close error non-critical

           if processErr != nil {
               return processErr
           }
       }

       return nil
   }
   ```

2. **Refactor each command** to use the helper:
   ```go
   // Before (head.go)
   files := fs.Args()
   if len(files) == 0 { ... } else { for _, file := range files { ... } }

   // After (head.go)
   return ProcessFilesOrStdin(fs.Args(), ctx.Stdin, hc.Dir, c.name, func(r io.Reader, _ string) error {
       return c.processHead(r, ctx.Stdout, lines)
   })
   ```

3. **Update each file**:
   - `head.go`: Extract `processHead()` method
   - `tail.go`: Extract `processTail()` method
   - `grep.go`: Extract `processGrep()` method (note: needs filename for output)
   - `cut.go`: Extract `processCut()` method
   - `wc.go`: Extract `processWc()` method (note: aggregates counts across files)
   - `sort.go`: Extract `processSort()` method
   - `uniq.go`: Extract `processUniq()` method

4. **Handle special cases**:
   - `grep`: Needs filename in output format, use the `filename` parameter
   - `wc`: Aggregates totals across files, may need modified approach

**Acceptance Criteria**:
- [ ] New `files.go` helper created with tests
- [ ] All 7 commands refactored to use helper
- [ ] No functionality changes (verified by existing tests)
- [ ] `make lint` passes
- [ ] `make test` passes
- [ ] Lines of code reduced by ~150 lines total

---

### 1.3 Add Missing Close Error Comments in `internal/tuiserver/`

**Priority**: Low
**Effort**: Low
**Impact**: Low (documentation consistency)

**Problem**: Close errors are discarded without explanatory comments, inconsistent with `internal/sshserver/` patterns.

**Files to Modify**:
- `internal/tuiserver/server.go` lines 127, 141

**Current Code**:
```go
_ = s.httpServer.Close()  // No comment
_ = s.listener.Close()    // No comment
```

**Reference Pattern** (from `internal/sshserver/server_lifecycle.go:73`):
```go
_ = listener.Close() // Best-effort cleanup on error
```

**Implementation Steps**:

1. **Add comments** explaining why errors are discarded:
   ```go
   _ = s.httpServer.Close() // Best-effort cleanup during state transition
   _ = s.listener.Close()   // Best-effort cleanup; server already stopping
   ```

**Acceptance Criteria**:
- [ ] All discarded close errors have explanatory comments
- [ ] Comments follow established pattern from sshserver

---

## Category 2: Testability Improvements

These items improve the ability to test components in isolation and increase overall test coverage.

### 2.1 Add Unit Tests for Runtime Package

**Priority**: High
**Effort**: High
**Impact**: High (critical code path coverage)

**Problem**: 9 implementation files in `internal/runtime/` lack corresponding test files.

**Files Needing Tests**:

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

**Implementation Steps**:

1. **Phase 1: Core Runtime Tests** (`runtime_test.go`)
   - Test `NewRuntime()` registry behavior
   - Test `Execute()` dispatching to correct runtime
   - Test runtime selection logic
   - Test `Available()` checks

2. **Phase 2: Environment Building Tests** (`env_test.go`)
   - Test 10-level precedence hierarchy
   - Test `EnvInheritMode` options (inherit, none, explicit)
   - Test dotenv file loading
   - Test Invowk env var filtering
   - Test `ExtraEnv` merging

3. **Phase 3: Native Runtime Tests** (`native_test.go`)
   - Test script resolution from Implementation
   - Test interpreter detection (bash, sh, zsh, fish)
   - Test shell command construction
   - Test output capture vs streaming
   - Test exit code extraction
   - Mock `exec.Command` for isolation

4. **Phase 4: Virtual Runtime Tests** (`virtual_test.go`)
   - Test mvdan/sh interpreter integration
   - Test builtin command handling
   - Test script execution
   - Test environment inheritance

5. **Phase 5: Container Runtime Tests** (`container_test.go`)
   - Create mock `Engine` interface implementation
   - Test image preparation logic
   - Test provisioning layer creation
   - Test run argument construction
   - Test SSH callback setup

**Test Patterns to Use**:
```go
// Use table-driven tests
func TestBuildRuntimeEnv(t *testing.T) {
    tests := []struct {
        name     string
        ctx      *ExecutionContext
        wantEnv  map[string]string
        wantErr  bool
    }{
        {
            name: "inherit all with extra env",
            ctx: &ExecutionContext{
                EnvInheritMode: EnvInheritAll,
                ExtraEnv:       map[string]string{"FOO": "bar"},
            },
            wantEnv: map[string]string{"FOO": "bar", /* inherited */},
        },
        // ... more cases
    }
    // ...
}
```

**Acceptance Criteria**:
- [ ] `runtime_test.go` covers dispatcher and registry
- [ ] `env_test.go` covers all 10 precedence levels
- [ ] `native_test.go` covers script resolution and execution
- [ ] `virtual_test.go` covers interpreter behavior
- [ ] `container_test.go` covers with mocked engine
- [ ] Test coverage for runtime package > 70%
- [ ] All tests pass on Linux, macOS, and Windows

---

### 2.2 Add Unit Tests for Container Engine Package

**Priority**: Medium
**Effort**: Medium
**Impact**: Medium (Docker/Podman abstraction coverage)

**Problem**: 3 implementation files in `internal/container/` lack tests.

**Files Needing Tests**:
- `docker.go` - Docker CLI wrapper
- `podman.go` - Podman CLI wrapper
- `engine_base.go` - Base CLI engine functionality

**Implementation Steps**:

1. **Create `engine_test.go`**:
   - Test `NewEngine()` detection logic
   - Test `Available()` for both engines
   - Test command construction for common operations

2. **Create `docker_test.go`**:
   - Test Docker-specific command flags
   - Test image name formatting
   - Test volume mount argument construction

3. **Create `podman_test.go`**:
   - Test Podman-specific command flags
   - Test rootless vs rootful detection
   - Test cgroup handling differences

4. **Create mock exec helper**:
   ```go
   // internal/container/testutil_test.go
   type mockExecRunner struct {
       commands [][]string
       outputs  map[string]string
       errors   map[string]error
   }
   ```

**Acceptance Criteria**:
- [ ] Engine detection tested
- [ ] Command construction tested for both engines
- [ ] Error handling tested
- [ ] Test coverage for container package > 60%

---

### 2.3 Decompose ExecutionContext into Focused Types -- (COMPLETED)

**Priority**: High
**Effort**: Medium
**Impact**: High (improves testability and API clarity)

> **Completed by**: spec-008 stateless composition refactoring.
> The monolithic `ExecutionContext` orchestration pattern was replaced by the `App`/`ExecuteRequest`/`CommandService` architecture in `cmd/invowk/app.go`. CLI handlers now build immutable `ExecuteRequest` structs and delegate to injected services, eliminating the "god object" problem through a different (and more thorough) decomposition than originally proposed here.

**Original Problem**: `ExecutionContext` in `internal/runtime/runtime.go` has 17 fields serving different concerns, making it a "god object" that's hard to test and understand.

**Current Structure** (lines 24-77):
```go
type ExecutionContext struct {
    // I/O
    Stdout, Stderr io.Writer
    Stdin          io.Reader

    // Environment
    ExtraEnv           map[string]string
    RuntimeEnvVars     map[string]string
    RuntimeEnvFiles    []string
    EnvInheritMode     EnvInheritMode
    EnvInheritModeSet  bool

    // Execution control
    Context          context.Context
    SelectedRuntime  RuntimeMode
    PositionalArgs   []string

    // Configuration
    WorkDir       string
    SelectedImpl  *invkfile.Implementation

    // TUI integration
    TUIServerURL   string
    TUIServerToken string

    // ... more fields
}
```

**Proposed Structure**:
```go
// IOContext holds I/O streams for command execution.
type IOContext struct {
    Stdout io.Writer
    Stderr io.Writer
    Stdin  io.Reader
}

// EnvContext holds environment configuration for command execution.
type EnvContext struct {
    ExtraEnv        map[string]string
    RuntimeEnvVars  map[string]string
    RuntimeEnvFiles []string
    InheritMode     EnvInheritMode
    InheritModeSet  bool
}

// TUIContext holds TUI server connection details.
type TUIContext struct {
    ServerURL   string
    ServerToken string
}

// ExecutionContext holds the complete context for command execution.
type ExecutionContext struct {
    Context context.Context

    // Core execution
    Command      *invkfile.Command
    Invkfile     *invkfile.Invkfile
    SelectedImpl *invkfile.Implementation

    // Sub-contexts
    IO  IOContext
    Env EnvContext
    TUI TUIContext

    // Runtime selection
    SelectedRuntime RuntimeMode
    PositionalArgs  []string
    WorkDir         string
}
```

**Implementation Steps**:

1. **Create sub-context types** in `runtime.go`:
   - Add `IOContext`, `EnvContext`, `TUIContext` types
   - Add doc comments explaining each type's purpose

2. **Update ExecutionContext**:
   - Embed sub-context types
   - Update constructor/factory functions

3. **Update all callers** (search for `ExecutionContext{`):
   - `cmd/invowk/cmd_run.go`
   - `cmd/invowk/tui_interactive.go`
   - `internal/runtime/*.go`
   - Test files

4. **Update runtime implementations**:
   - Change `ctx.Stdout` to `ctx.IO.Stdout`
   - Change `ctx.ExtraEnv` to `ctx.Env.ExtraEnv`
   - etc.

5. **Consider helper methods**:
   ```go
   // HasTUI returns true if TUI server is configured.
   func (ctx *ExecutionContext) HasTUI() bool {
       return ctx.TUI.ServerURL != ""
   }
   ```

**Acceptance Criteria**:
- [ ] Sub-context types created with doc comments
- [ ] ExecutionContext uses composition
- [ ] All callers updated
- [ ] All tests pass
- [ ] API is clearer and more self-documenting

---

### 2.4 Add Integration Tests for Container Runtime

**Priority**: Medium
**Effort**: High
**Impact**: High (validates Docker/Podman execution paths)

**Problem**: No integration tests for container runtime execution.

**Test Scenarios Needed**:

1. **Basic container execution**:
   - Run simple command in container
   - Verify output capture
   - Verify exit code handling

2. **Provisioning**:
   - Test binary mounting works
   - Test module directory mounting
   - Test invkfile availability in container

3. **SSH callback**:
   - Test host access from container
   - Test token authentication

4. **Image handling**:
   - Test custom Dockerfile
   - Test base image specification
   - Test image caching

**Implementation Steps**:

1. **Create testscript tests** in `tests/cli/testdata/`:
   - `container_basic.txtar` - Basic container execution
   - `container_provision.txtar` - Provisioning verification
   - `container_callback.txtar` - SSH callback tests

2. **Guard tests** for CI environment:
   ```
   [!exec:docker] [!exec:podman] skip 'no container runtime'
   ```

3. **Add to Makefile**:
   ```makefile
   test-container:
       go test -v -tags=integration ./tests/cli/... -run Container
   ```

**Acceptance Criteria**:
- [ ] Container execution tests pass locally
- [ ] Tests properly skip in CI (no Docker/Podman)
- [ ] Provisioning verified
- [ ] SSH callback tested

---

## Category 3: Architectural Improvements

These items improve the overall architecture, making the codebase more maintainable and extensible.

### 3.1 Extract EnvBuilder Interface

**Priority**: Medium
**Effort**: Medium
**Impact**: Medium (clarifies environment contract)

**Problem**: Environment building logic is scattered across `env.go`, `dotenv.go`, and `runtime.go` with a complex 10-level precedence hierarchy that's hard to understand and test.

**Current State**:
- `env.go:19-81` - `buildRuntimeEnv()` with 10 precedence levels
- `dotenv.go` - dotenv file loading
- `runtime.go` - env filtering (remove INVOWK_* vars)

**Proposed Design**:
```go
// EnvBuilder constructs the environment for command execution.
type EnvBuilder interface {
    // Build returns the environment map for the given context.
    // The returned map follows the precedence rules defined by the implementation.
    Build(ctx *ExecutionContext) (map[string]string, error)
}

// DefaultEnvBuilder implements the standard 10-level precedence.
type DefaultEnvBuilder struct {
    dotenvLoader DotenvLoader
}

// DotenvLoader loads environment variables from .env files.
type DotenvLoader interface {
    Load(paths []string) (map[string]string, error)
}
```

**Implementation Steps**:

1. **Create `env_builder.go`**:
   - Define `EnvBuilder` interface
   - Define `DotenvLoader` interface
   - Implement `DefaultEnvBuilder`

2. **Document precedence levels** clearly:
   ```go
   // DefaultEnvBuilder precedence (highest to lowest):
   // 1. ExtraEnv from ExecutionContext
   // 2. RuntimeEnvVars from command definition
   // 3. RuntimeEnvFiles (.env files from command)
   // 4. Command-level env from invkfile
   // 5. Invkfile-level env
   // 6. Module-level env (if in module context)
   // 7. User config env
   // 8. System environment (if inherit mode allows)
   // 9. Default values from schema
   // 10. Filtered INVOWK_* variables
   ```

3. **Update runtime implementations**:
   - Accept `EnvBuilder` in constructor
   - Call `envBuilder.Build(ctx)` instead of `buildRuntimeEnv(ctx)`

4. **Create test implementation**:
   ```go
   // MockEnvBuilder for testing
   type MockEnvBuilder struct {
       Env map[string]string
       Err error
   }
   func (m *MockEnvBuilder) Build(_ *ExecutionContext) (map[string]string, error) {
       return m.Env, m.Err
   }
   ```

**Acceptance Criteria**:
- [ ] `EnvBuilder` interface defined
- [ ] `DefaultEnvBuilder` implements current behavior
- [ ] Precedence is documented in code
- [ ] Runtimes use interface (can inject mock)
- [ ] Tests cover all precedence levels

---

### 3.2 Extract Provisioning to Separate Package -- (COMPLETED)

**Priority**: Medium
**Effort**: Medium
**Impact**: Medium (improves separation of concerns)

> **Completed by**: spec-008 stateless composition refactoring.
> Provisioning logic was extracted to `internal/provision/` as a separate package, with the `ForceRebuild` option wired through `ExecuteRequest.ForceRebuild` in `cmd/invowk/app.go:62`.

**Original Problem**: Container provisioning logic (~300 lines) is embedded in `internal/runtime/`, mixing execution and preparation concerns.

**Original Files**:
- `internal/runtime/provision.go` - Core provisioning logic
- `internal/runtime/provision_layer.go` - Layer utilities
- `internal/runtime/container_provision.go` - Container-specific provisioning

**Proposed Structure**:
```
internal/provision/
├── doc.go                  # Package documentation
├── provisioner.go          # Provisioner interface
├── layer_provisioner.go    # LayerProvisioner implementation
├── layer_provisioner_test.go
├── hash.go                 # Content hashing for caching
├── hash_test.go
└── config.go               # ProvisionConfig type
```

**Implementation Steps**:

1. **Create package structure**:
   ```go
   // internal/provision/doc.go

   // Package provision handles resource provisioning for container execution.
   //
   // This package provides the Provisioner interface and implementations for
   // preparing container images with the necessary resources (invowk binary,
   // modules, invkfiles) for command execution.
   package provision
   ```

2. **Define Provisioner interface**:
   ```go
   // Provisioner prepares resources for container execution.
   type Provisioner interface {
       // Provision prepares the container image with necessary resources.
       // Returns the image ID to use for execution.
       Provision(ctx context.Context, opts *ProvisionOptions) (imageID string, err error)

       // Cleanup removes provisioned resources.
       Cleanup(ctx context.Context, imageID string) error
   }

   type ProvisionOptions struct {
       BaseImage      string
       Containerfile  string
       BinaryPath     string
       Modules        []ModuleMount
       Invkfiles      []InvkfileMount
       ForceRebuild   bool  // Addresses TODO in container_provision.go
   }
   ```

3. **Move existing code**:
   - `provision.go` → `provision/layer_provisioner.go`
   - `provision_layer.go` → `provision/layer.go`
   - `container_provision.go` → `provision/container.go`

4. **Update imports** in `internal/runtime/`:
   - Import `internal/provision`
   - Use `provision.Provisioner` interface

5. **Add `ForceRebuild` option** (addresses existing TODO):
   ```go
   if opts.ForceRebuild {
       // Skip cache check, always rebuild
   }
   ```

**Acceptance Criteria**:
- [ ] New `internal/provision/` package created
- [ ] `Provisioner` interface defined
- [ ] Existing functionality preserved
- [ ] `ForceRebuild` option implemented (TODO resolved)
- [ ] Tests moved and passing
- [ ] Runtime package uses interface

---

### 3.3 Create Validator Interface for Invkfile Validation

**Priority**: Low
**Effort**: Medium
**Impact**: Medium (enables custom validation)

**Problem**: 5 validation files in `pkg/invkfile/` have similar patterns but no shared interface, making it hard to add custom validators.

**Current Files**:
- `validation_container.go` - Container-specific validation
- `validation_filesystem.go` - File path validation
- `validation_primitives.go` - Basic type validation
- `invkfile_validation_deps.go` - Dependency validation
- `invkfile_validation_struct.go` - Structure validation

**Proposed Design**:
```go
// Validator validates an Invkfile and returns any errors.
type Validator interface {
    // Validate checks the invkfile and returns validation errors.
    // The context provides access to filesystem and environment.
    Validate(ctx *ValidationContext, inv *Invkfile) []ValidationError
}

// ValidationContext provides context for validation.
type ValidationContext struct {
    // WorkDir is the directory containing the invkfile
    WorkDir string
    // FileSystem allows validators to check file existence
    FileSystem FileSystem
    // Platform indicates the target platform
    Platform string
}

// ValidationError represents a single validation error.
type ValidationError struct {
    Field   string // JSON path to field, e.g., "cmds.build.container.image"
    Message string
    Severity ValidationSeverity // Error, Warning
}

// CompositeValidator runs multiple validators.
type CompositeValidator struct {
    validators []Validator
}
```

**Implementation Steps**:

1. **Create `validation.go`** with interface definitions

2. **Refactor existing validators** to implement interface:
   - `ContainerValidator` from `validation_container.go`
   - `FilesystemValidator` from `validation_filesystem.go`
   - `DependencyValidator` from `invkfile_validation_deps.go`
   - `StructureValidator` from `invkfile_validation_struct.go`

3. **Create `CompositeValidator`**:
   ```go
   func DefaultValidators() []Validator {
       return []Validator{
           &StructureValidator{},
           &DependencyValidator{},
           &ContainerValidator{},
           &FilesystemValidator{},
       }
   }
   ```

4. **Update `Validate()` method** on `Invkfile`:
   ```go
   func (inv *Invkfile) Validate(opts ...ValidateOption) []ValidationError {
       ctx := &ValidationContext{...}
       validators := DefaultValidators()
       // Apply options to customize validators

       var errors []ValidationError
       for _, v := range validators {
           errors = append(errors, v.Validate(ctx, inv)...)
       }
       return errors
   }
   ```

**Acceptance Criteria**:
- [ ] `Validator` interface defined
- [ ] Existing validators refactored
- [ ] `CompositeValidator` implemented
- [ ] Custom validators can be added
- [ ] All existing tests pass

---

## Category 4: Documentation & Cleanup

These items improve documentation and resolve tracked technical debt.

### 4.1 Add Package Documentation

**Priority**: Low
**Effort**: Low
**Impact**: Low (developer experience)

**Problem**: Several internal packages lack `doc.go` files.

**Packages Needing `doc.go`**:

1. **`internal/container/`**:
   ```go
   // SPDX-License-Identifier: MPL-2.0

   // Package container provides an abstraction layer for container engines.
   //
   // This package supports both Docker and Podman through a unified Engine
   // interface, allowing the rest of the codebase to work with containers
   // without coupling to a specific implementation.
   //
   // The package automatically detects available container engines and
   // selects the appropriate one based on configuration or availability.
   package container
   ```

2. **`internal/runtime/`**:
   ```go
   // SPDX-License-Identifier: MPL-2.0

   // Package runtime provides command execution runtimes for invowk.
   //
   // Three runtime implementations are available:
   //   - NativeRuntime: Executes commands using the host's shell
   //   - VirtualRuntime: Executes commands using mvdan/sh pure-Go shell
   //   - ContainerRuntime: Executes commands in Docker/Podman containers
   //
   // All runtimes implement the Runtime interface and can be extended with
   // CapturingRuntime (for output capture) and InteractiveRuntime (for PTY).
   package runtime
   ```

**Implementation Steps**:

1. Create `doc.go` in each package
2. Document package purpose and key types
3. Cross-reference related packages

**Acceptance Criteria**:
- [ ] All internal packages have `doc.go`
- [ ] Documentation explains package purpose
- [ ] Key types and patterns documented

---

### 4.2 Resolve Existing TODOs -- (PARTIALLY COMPLETED)

**Priority**: Medium
**Effort**: Low-Medium
**Impact**: Medium (completes planned features)

> **Partially completed by**: spec-008 stateless composition refactoring.
> Both original TODOs (custom config path and force rebuild) are now resolved:
> - Custom config file path: `--config` flag wired through `rootFlags.configPath` into `ConfigProvider.Load()` via `config.LoadOptions.ConfigFilePath`.
> - Force rebuild: `--force-rebuild` flag wired through `ExecuteRequest.ForceRebuild` into `internal/provision/`.
>
> Remaining work: verify no other stale TODOs remain in the codebase.

**Original TODOs** (both resolved):

1. **Custom Config File Path** (`cmd/invowk/root.go`):
   - ~~Already documented in `specs/next/pending-features.md`~~ Archived to `specs/completed/pending-features.md`.
   - Implementation requires config package changes -- **DONE** via `ConfigProvider` interface.

2. **Force Rebuild Option** (`internal/provision/`):
   - ~~Will be addressed by 3.2 (Extract Provisioning Package)~~ **DONE** via `ExecuteRequest.ForceRebuild`.

**Acceptance Criteria**:
- [x] Custom config path working
- [x] Force rebuild flag added
- [ ] Verify no other stale TODOs remain in code
- [ ] Features documented on website

---

### 4.3 Update Website Documentation

**Priority**: Low
**Effort**: Medium
**Impact**: Medium (user experience)

**Missing Documentation Sections**:

1. **Architecture Overview** - How invowk works internally
2. **Troubleshooting Guide** - Common issues and solutions
3. **Contributing Guide** - How to contribute to the project
4. **Performance Tuning** - Optimization tips

**Implementation Steps**:

1. **Create `website/docs/architecture.md`**:
   - Explain CUE → Parser → Runtime flow
   - Document runtime selection logic
   - Explain module resolution

2. **Create `website/docs/troubleshooting.md`**:
   - Common container issues
   - Path resolution problems
   - Environment variable debugging

3. **Create `website/docs/contributing.md`**:
   - Development setup
   - Testing requirements
   - Code style (link to `.claude/rules/`)

**Acceptance Criteria**:
- [ ] Architecture overview written
- [ ] Troubleshooting guide covers common issues
- [ ] Contributing guide helps new contributors
- [ ] `npm run build` passes

---

## Implementation Order (Recommended)

> **Note**: Items marked with ~~strikethrough~~ were completed by the spec-008 stateless composition refactoring.

### Phase 1: Quick Wins (1-2 days)
1. 1.1 - Fix error handling in uroot (2 hours)
2. 1.3 - Add close error comments (30 minutes)
3. 4.1 - Add package documentation (2 hours)

### Phase 2: Core Testability (1 week)
4. ~~2.3 - Decompose ExecutionContext (2 days)~~ **COMPLETED** (spec-008)
5. 2.1 - Add runtime tests (env.go first) (3 days)

### Phase 3: Architecture (1 week)
6. 3.1 - Extract EnvBuilder interface (2 days)
7. 1.2 - Extract file processing helper (1 day)
8. ~~3.2 - Extract provisioning package (2 days)~~ **COMPLETED** (spec-008)

### Phase 4: Extended Testing (1 week)
9. 2.2 - Add container engine tests (2 days)
10. 2.4 - Add container integration tests (3 days)

### Phase 5: Polish (3 days)
11. ~~4.2 - Resolve TODOs (2 days)~~ **PARTIALLY COMPLETED** (spec-008: config path + force rebuild done)
12. 4.3 - Update website docs (1 day)
13. 3.3 - Create Validator interface (if time permits)

---

## Dependencies Between Items

> Dependencies involving completed items (2.3, 3.2, 4.2) are now unblocked.

```
1.1 (error handling) ─────────────────────────────────┐
                                                      │
1.2 (file helper) ────────────────────────────────────┤
                                                      ├──► 2.1 (runtime tests)
[DONE] 2.3 (ExecutionContext decomposition) ──────────┤
                                                      │
3.1 (EnvBuilder) ─────────────────────────────────────┘

[DONE] 3.2 (provisioning package) ──► [DONE] 4.2 (resolve TODOs)

2.1 (runtime tests) ──► 2.4 (container integration tests)

2.2 (container engine tests) ──► 2.4 (container integration tests)
```

---

## Success Metrics

| Metric | Pre-spec-008 | Post-spec-008 | Target |
|--------|--------------|---------------|--------|
| Runtime package test coverage | ~10% | ~10% | >70% |
| Container package test coverage | ~20% | ~20% | >60% |
| Packages with doc.go | ~60% | ~60% | 100% |
| Open TODOs in code | 2 | 0 (config path + force rebuild resolved) | 0 |
| Integration test scenarios | 19 | 19 | 25+ |
| Linter warnings | 0 | 0 | 0 |
| Global config state in execution paths | present | removed (ConfigProvider) | removed |
| Untyped module command storage | `any` | `ModuleCommands` interface | typed |
