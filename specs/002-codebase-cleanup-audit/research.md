# Research: Codebase Cleanup Audit

**Date**: 2026-01-29
**Branch**: `002-codebase-cleanup-audit`

This document captures the detailed codebase analysis that informed the implementation plan.

## 1. File Size Analysis

### Files Exceeding Target (800 lines)

| File | Lines | Status |
|------|-------|--------|
| `cmd/invowk/cmd.go` | 2,927 | **CRITICAL** - requires decomposition |
| `cmd/invowk/module.go` | 1,118 | Out of scope (module CLI, not cmd handler) |
| `pkg/invowkmod/operations.go` | 827 | Minor - focused responsibility |
| `internal/tui/interactive.go` | 806 | Minor - TUI handler |

### Files Approaching Target (500-800 lines)

| File | Lines | Status |
|------|-------|--------|
| `internal/runtime/container.go` | 745 | OK - focused responsibility |
| `pkg/invowkfile/validation.go` | 728 | OK - schema validation |
| `pkg/invowkmod/resolver.go` | 726 | OK - dependency resolution |
| `internal/discovery/discovery.go` | 715 | OK - command discovery |
| `internal/sshserver/server.go` | 695 | OK - well-structured server |
| `internal/runtime/native.go` | 578 | **Code duplication** - requires deduplication |

### Summary Statistics

- **Total Go files**: 80 (production)
- **Total lines**: ~47,500 (including tests: ~62,500)
- **Files over 800 lines**: 4
- **Files over 500 lines**: 10

## 2. cmd.go Analysis (2,927 lines)

### Function Inventory

Total: **69 functions** across these responsibility groups:

#### Group 1: Command Registration & Routing (~8 functions)
```go
NewCmdCommand()              // Main command factory
createCommand()              // Cobra command builder
handleCommandFlags()         // Flag setup
setupCommandContext()        // Context initialization
// + 4 helper functions
```

#### Group 2: Command Discovery & Parsing (~12 functions)
```go
registerDiscoveredCommands() // lines 262-370 (109 lines)
listCommands()               // lines 657-822 (166 lines)
discoverCommandSources()     // Multi-source discovery
buildCommandTree()           // Subcommand hierarchy
disambiguateCommand()        // Namespace resolution
// + 7 helper functions
```

#### Group 3: Execution Orchestration (~15 functions)
```go
runCommandWithFlags()        // lines 866-1120 (260 lines) - LARGEST
ensureSSHServer()            // lines 1425-1446
stopSSHServer()              // lines 1448-1462
prepareExecution()           // Runtime selection
executeCommand()             // Actual execution dispatch
// + 10 helper functions
```

#### Group 4: Dependency Validation (~18 functions)
```go
validateDependencies()       // lines 1465-1516 (51 lines)
validateToolDependency()     // Tool version checking
validateFilepathDependency() // File existence checking
validateCapability()         // System capability checking
validateCustomCheck()        // User-defined validation
// + 13 helper functions
```

#### Group 5: Output Formatting & Errors (~16 functions)
```go
RenderArgumentValidationError()   // lines 2514-2580
RenderDependencyError()           // lines 2582-2620
RenderArgsSubcommandConflictError() // lines 2622-2656
formatCommandOutput()             // Output styling
formatErrorMessage()              // Error styling
// + 11 helper functions
```

### Global State

```go
// Line 47-50: SSH server singleton (well-protected with mutex)
var (
    sshServerInstance *sshserver.Server
    sshServerMu       sync.Mutex
)
```

### Import Dependencies (8 internal packages)

```go
import (
    "github.com/invowk/invowk/internal/config"
    "github.com/invowk/invowk/internal/discovery"
    "github.com/invowk/invowk/internal/issue"
    "github.com/invowk/invowk/internal/runtime"
    "github.com/invowk/invowk/internal/sshserver"
    "github.com/invowk/invowk/internal/tui"
    "github.com/invowk/invowk/internal/tuiserver"
    "github.com/invowk/invowk/pkg/invowkfile"
)
```

### Decomposition Challenges

1. **Shared Types**: Several types/vars are used across responsibility groups
   - Must remain in `cmd.go` or move to shared location
2. **Import Cycles**: All files will be in same package, so no cycle risk
3. **Test Files**: `cmd_test.go` may need splitting to match new structure

## 3. Native Runtime Duplication Analysis

### File: `internal/runtime/native.go` (578 lines)

### Duplicated Function Pairs

| Execute Function | Capture Function | Shared % |
|-----------------|------------------|----------|
| `executeWithShell()` (147-193) | `executeCaptureWithShell()` (262-308) | ~95% |
| `executeWithInterpreter()` (196-259) | `executeCaptureWithInterpreter()` (311-372) | ~95% |

### Repeated Code Patterns

#### Pattern 1: Script Resolution (3 occurrences)
```go
// Lines 55-57, 81-85, 122-124
script, err := ctx.SelectedImpl.ResolveScript(ctx.Invowkfile.FilePath)
if err != nil {
    return &Result{ExitCode: 1, Error: err}
}
```

#### Pattern 2: Working Directory Setup (5+ occurrences)
```go
// Lines 163-168, 276-279, 340-343, 512-515, 562-565
origDir, err := os.Getwd()
if err != nil {
    return &Result{ExitCode: 1, Error: fmt.Errorf("failed to get working directory: %w", err)}
}
if err := os.Chdir(workDir); err != nil {
    return &Result{ExitCode: 1, Error: fmt.Errorf("failed to change to working directory: %w", err)}
}
defer os.Chdir(origDir)
```

#### Pattern 3: Environment Building (6 occurrences)
```go
// Lines 172-176, 281-285, 345-349, 518-522, 568-572
env := buildRuntimeEnv(ctx, invowkfile.EnvInheritAll)
cmd.Env = env
```

#### Pattern 4: Exit Code Extraction (4 occurrences)
```go
// Lines 184-192, 250-256, 297-305, 355-371
if err != nil {
    if exitErr, ok := err.(*exec.ExitError); ok {
        return &Result{ExitCode: exitErr.ExitCode(), Error: nil}
    }
    return &Result{ExitCode: 1, Error: err}
}
```

### Counter-Example: Good Consolidation

`PrepareCommand()` (lines 120-144) successfully consolidates shell/interpreter dispatch:

```go
func (r *NativeRuntime) PrepareCommand(ctx *ExecutionContext) (*PreparedCommand, error) {
    script, err := ctx.SelectedImpl.ResolveScript(ctx.Invowkfile.FilePath)
    if err != nil {
        return nil, err
    }

    rtConfig := ctx.SelectedImpl.GetRuntimeConfig(ctx.SelectedRuntime)
    if rtConfig == nil {
        return r.prepareShellCommand(ctx, script)
    }

    interpInfo := rtConfig.ResolveInterpreterFromScript(script)
    if interpInfo.Found {
        return r.prepareInterpreterCommand(ctx, script, interpInfo)
    }

    return r.prepareShellCommand(ctx, script)
}
```

This pattern can be extended to execution with output capture as an optional parameter.

## 4. Interface Implementation Status

### CapturingRuntime Interface

**Definition** (`internal/runtime/runtime.go:102-106`):
```go
type CapturingRuntime interface {
    ExecuteCapture(ctx *ExecutionContext) *Result
}
```

### Implementation Status

| Runtime | Implements | Location |
|---------|------------|----------|
| `NativeRuntime` | ✅ Yes | `native.go:80-104` |
| `VirtualRuntime` | ✅ Yes | `virtual.go:150-216` |
| `ContainerRuntime` | ❌ **No** | Missing - needs implementation |

### VirtualRuntime Reference Implementation

```go
// virtual.go:150-216
func (r *VirtualRuntime) ExecuteCapture(ctx *ExecutionContext) *Result {
    script, err := ctx.SelectedImpl.ResolveScript(ctx.Invowkfile.FilePath)
    if err != nil {
        return &Result{ExitCode: 1, Error: err}
    }

    file, err := syntax.NewParser().Parse(strings.NewReader(script), "")
    if err != nil {
        return &Result{ExitCode: 1, Error: fmt.Errorf("failed to parse script: %w", err)}
    }

    var stdout, stderr bytes.Buffer  // <-- Key difference: buffers instead of ctx streams

    opts := []interp.RunnerOption{
        interp.StdIO(nil, &stdout, &stderr),
        // ... other options
    }

    runner, err := interp.New(opts...)
    if err != nil {
        return &Result{ExitCode: 1, Error: err}
    }

    err = runner.Run(ctx.Context, file)

    return &Result{
        ExitCode:  int(runner.Exited()),
        Output:    stdout.String(),    // <-- Captured stdout
        ErrOutput: stderr.String(),    // <-- Captured stderr
        Error:     err,
    }
}
```

## 5. Architectural Layering Analysis

### Violation: Public Package Imports Internal

**Affected Files**:
1. `pkg/invowkmod/operations.go:14`
2. `pkg/invowkfile/validation.go:23`

Both import: `"github.com/invowk/invowk/internal/platform"`

### Source of Violation

**File**: `internal/platform/windows.go` (lines 18-28)

```go
// WindowsReservedNames contains names reserved by Windows
var WindowsReservedNames = map[string]bool{
    "CON": true, "PRN": true, "AUX": true, "NUL": true,
    "COM1": true, "COM2": true, "COM3": true, "COM4": true,
    "COM5": true, "COM6": true, "COM7": true, "COM8": true,
    "COM9": true, "LPT1": true, "LPT2": true, "LPT3": true,
    "LPT4": true, "LPT5": true, "LPT6": true, "LPT7": true,
    "LPT8": true, "LPT9": true,
}

func IsWindowsReservedName(name string) bool {
    upper := strings.ToUpper(name)
    if idx := strings.LastIndex(upper, "."); idx != -1 {
        upper = upper[:idx]
    }
    return WindowsReservedNames[upper]
}
```

### Usage Sites

1. `pkg/invowkfile/validation.go:571` - Command name validation
2. `pkg/invowkmod/operations.go:243` - Directory name validation

### Consumers of internal/platform

Checking for other internal consumers:
```
internal/platform/ is ONLY consumed by:
- pkg/invowkmod/operations.go
- pkg/invowkfile/validation.go
```

No internal-only consumers exist, so the entire package can be moved to `pkg/platform/`.

## 6. Server State Machine Analysis

### Pattern Compliance

Both servers follow the documented pattern in `.claude/rules/servers.md`:

| Feature | sshserver | tuiserver |
|---------|-----------|-----------|
| State enum | ✅ lines 28-41 | ✅ lines 20-33 |
| Atomic reads | ✅ `atomic.Int32` | ✅ `atomic.Int32` |
| Mutex transitions | ✅ `stateMu sync.Mutex` | ✅ `stateMu sync.Mutex` |
| Blocking Start | ✅ `startedCh chan struct{}` | ✅ `startedCh chan struct{}` |
| WaitGroup tracking | ✅ `wg sync.WaitGroup` | ✅ `wg sync.WaitGroup` |
| Error channel | ✅ `errCh chan error` | ✅ `errCh chan error` |
| Idempotent Stop | ✅ | ✅ |

### Deduplication Assessment

**Conclusion**: Defer extraction to separate spec.

**Rationale**:
1. Both implementations are correct and well-tested
2. Pattern is documented, not just duplicated
3. Extraction would add abstraction without fixing bugs
4. Only 2 servers exist; abstraction justified at 3+

## 7. Container Runtime Analysis

### Current Execute() Implementation

**File**: `internal/runtime/container.go`

The `Execute()` method (approximately lines 200-280) follows this flow:

```
Execute(ctx)
    → prepareContainerExecution(ctx)
    → runInContainer(ctx, image, script, ctx.Stdout, ctx.Stderr)
    → return Result{ExitCode, Error}
```

### ExecuteCapture() Implementation Path

Following the VirtualRuntime pattern:

```go
func (r *ContainerRuntime) ExecuteCapture(ctx *ExecutionContext) *Result {
    // Same preparation as Execute()
    image, script, err := r.prepareContainerExecution(ctx)
    if err != nil {
        return &Result{ExitCode: 1, Error: err}
    }

    // Key difference: buffers instead of ctx streams
    var stdout, stderr bytes.Buffer

    // Pass buffers to runInContainer
    exitCode, err := r.runInContainer(ctx, image, script, &stdout, &stderr)

    return &Result{
        ExitCode:  exitCode,
        Output:    stdout.String(),
        ErrOutput: stderr.String(),
        Error:     err,
    }
}
```

### Required Changes

1. Add `ExecuteCapture()` method (~30 lines)
2. Ensure `runInContainer()` accepts `io.Writer` for stdout/stderr (verify signature)
3. Add tests for capture functionality

## 8. Summary: Changes Required

### New Files to Create

| File | Lines (est) | Purpose |
|------|-------------|---------|
| `cmd/invowk/cmd_discovery.go` | ~500 | Command discovery functions |
| `cmd/invowk/cmd_execute.go` | ~600 | Execution orchestration |
| `cmd/invowk/cmd_validate.go` | ~700 | Dependency validation |
| `cmd/invowk/cmd_output.go` | ~400 | Output formatting |
| `internal/runtime/native_helpers.go` | ~200 | Shared execution helpers |
| `pkg/platform/doc.go` | ~10 | Package documentation |
| `pkg/platform/windows.go` | ~35 | Moved from internal |
| `pkg/platform/windows_test.go` | ~50 | Moved from internal |

### Files to Modify

| File | Change |
|------|--------|
| `cmd/invowk/cmd.go` | Remove moved functions (2,927 → ~600) |
| `internal/runtime/native.go` | Extract shared code (578 → ~350) |
| `internal/runtime/container.go` | Add `ExecuteCapture()` (~30 lines added) |
| `pkg/invowkmod/operations.go` | Update import |
| `pkg/invowkfile/validation.go` | Update import |

### Files to Delete

| File | Reason |
|------|--------|
| `internal/platform/windows.go` | Moved to `pkg/platform/` |
| `internal/platform/windows_test.go` | Moved to `pkg/platform/` |
| `internal/platform/` (directory) | Empty after move |
