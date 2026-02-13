# Deduplication Design: internal/runtime/native.go

**Date**: 2026-01-29
**Source File**: `internal/runtime/native.go` (578 lines)
**Target**: ~350 lines after extracting shared helpers to `native_helpers.go` (~200 lines)

## Overview

The native runtime contains duplicated code patterns across execute/capture variants and shell/interpreter variants. This document specifies helper functions to extract shared logic.

## Current Duplication Pattern

```
Execute() ─────────────────────────────── ExecuteCapture()
    │                                           │
    ├─ executeWithShell() ─── 95% shared ─── executeCaptureWithShell()
    │
    └─ executeWithInterpreter() ─ 95% shared ─ executeCaptureWithInterpreter()
```

**Root Cause**: Execute and ExecuteCapture differ only in output destination:
- `Execute()`: Writes to `ctx.Stdout`, `ctx.Stderr`
- `ExecuteCapture()`: Writes to `bytes.Buffer`, returns in `Result.Output`/`Result.ErrOutput`

## Repeated Code Blocks

### Block 1: Script Resolution (3 occurrences)
**Lines**: 55-57, 81-85, 122-124

```go
script, err := ctx.SelectedImpl.ResolveScript(ctx.Invowkfile.FilePath)
if err != nil {
    return &Result{ExitCode: 1, Error: err}
}
```

### Block 2: Working Directory Management (5+ occurrences)
**Lines**: 163-168, 276-279, 340-343, 512-515, 562-565

```go
origDir, err := os.Getwd()
if err != nil {
    return &Result{ExitCode: 1, Error: fmt.Errorf("failed to get working directory: %w", err)}
}
if err := os.Chdir(workDir); err != nil {
    return &Result{ExitCode: 1, Error: fmt.Errorf("failed to change to working directory: %w", err)}
}
defer os.Chdir(origDir)
```

### Block 3: Environment Building (6 occurrences)
**Lines**: 172-176, 281-285, 345-349, 518-522, 568-572

```go
env := buildRuntimeEnv(ctx, invowkfile.EnvInheritAll)
cmd.Env = env
```

### Block 4: Exit Code Extraction (4 occurrences)
**Lines**: 184-192, 250-256, 297-305, 355-371

```go
if err != nil {
    if exitErr, ok := err.(*exec.ExitError); ok {
        return &Result{ExitCode: exitErr.ExitCode(), Error: nil}
    }
    return &Result{ExitCode: 1, Error: err}
}
```

---

## Proposed Helper Design

### File: `native_helpers.go`

```go
// SPDX-License-Identifier: MPL-2.0

package runtime

import (
    "bytes"
    "fmt"
    "io"
    "os"
    "os/exec"
)

// executeOutput specifies where execution output should be directed.
type executeOutput struct {
    stdout io.Writer
    stderr io.Writer
    // capture indicates if output should be captured (true) or streamed (false)
    capture bool
}

// capturedOutput holds the results when executeOutput.capture is true.
type capturedOutput struct {
    stdout *bytes.Buffer
    stderr *bytes.Buffer
}

// newStreamingOutput creates output configuration that streams to context.
func newStreamingOutput(ctx *ExecutionContext) executeOutput {
    return executeOutput{
        stdout:  ctx.Stdout,
        stderr:  ctx.Stderr,
        capture: false,
    }
}

// newCapturingOutput creates output configuration that captures to buffers.
func newCapturingOutput() (executeOutput, *capturedOutput) {
    captured := &capturedOutput{
        stdout: &bytes.Buffer{},
        stderr: &bytes.Buffer{},
    }
    return executeOutput{
        stdout:  captured.stdout,
        stderr:  captured.stderr,
        capture: true,
    }, captured
}
```

### Helper Functions

#### 1. Working Directory Manager

```go
// workDirScope manages working directory changes with automatic restoration.
type workDirScope struct {
    original string
    changed  bool
}

// enterWorkDir changes to the specified working directory and returns
// a scope that restores the original directory when Close() is called.
func enterWorkDir(workDir string) (*workDirScope, error) {
    if workDir == "" {
        return &workDirScope{changed: false}, nil
    }

    origDir, err := os.Getwd()
    if err != nil {
        return nil, fmt.Errorf("failed to get working directory: %w", err)
    }

    if err := os.Chdir(workDir); err != nil {
        return nil, fmt.Errorf("failed to change to working directory %q: %w", workDir, err)
    }

    return &workDirScope{original: origDir, changed: true}, nil
}

// Close restores the original working directory.
func (s *workDirScope) Close() error {
    if !s.changed {
        return nil
    }
    return os.Chdir(s.original)
}
```

**Usage**:
```go
wdScope, err := enterWorkDir(workDir)
if err != nil {
    return &Result{ExitCode: 1, Error: err}
}
defer wdScope.Close()
```

#### 2. Exit Code Extractor

```go
// extractExitCode converts an exec error into an exit code and optional error.
// Returns (exitCode, nil) for normal exits, (1, err) for unexpected errors.
func extractExitCode(err error) (int, error) {
    if err == nil {
        return 0, nil
    }
    if exitErr, ok := err.(*exec.ExitError); ok {
        return exitErr.ExitCode(), nil
    }
    return 1, err
}
```

**Usage**:
```go
err := cmd.Run()
exitCode, runErr := extractExitCode(err)
return &Result{ExitCode: exitCode, Error: runErr, ...}
```

#### 3. Command Configuration Helper

```go
// configureCommand sets up common command properties.
func configureCommand(cmd *exec.Cmd, ctx *ExecutionContext, output executeOutput) {
    cmd.Stdin = ctx.Stdin
    cmd.Stdout = output.stdout
    cmd.Stderr = output.stderr
    cmd.Env = buildRuntimeEnv(ctx, invowkfile.EnvInheritAll)
}
```

#### 4. Result Builder

```go
// buildResult creates a Result from execution outcome.
func buildResult(exitCode int, err error, captured *capturedOutput) *Result {
    result := &Result{
        ExitCode: exitCode,
        Error:    err,
    }
    if captured != nil {
        result.Output = captured.stdout.String()
        result.ErrOutput = captured.stderr.String()
    }
    return result
}
```

---

## Refactored Function Signatures

### Current (duplicated)

```go
func (r *NativeRuntime) executeWithShell(ctx *ExecutionContext, script string) *Result
func (r *NativeRuntime) executeCaptureWithShell(ctx *ExecutionContext, script string) *Result
func (r *NativeRuntime) executeWithInterpreter(ctx *ExecutionContext, script string, interpInfo invowkfile.InterpreterInfo) *Result
func (r *NativeRuntime) executeCaptureWithInterpreter(ctx *ExecutionContext, script string, interpInfo invowkfile.InterpreterInfo) *Result
```

### After (unified with options)

```go
// executeWithShellCommon handles both Execute and ExecuteCapture for shell execution.
func (r *NativeRuntime) executeWithShellCommon(ctx *ExecutionContext, script string, output executeOutput) *Result

// executeWithInterpreterCommon handles both Execute and ExecuteCapture for interpreter execution.
func (r *NativeRuntime) executeWithInterpreterCommon(ctx *ExecutionContext, script string, interpInfo invowkfile.InterpreterInfo, output executeOutput) *Result
```

### Simplified Execute/ExecuteCapture

```go
func (r *NativeRuntime) Execute(ctx *ExecutionContext) *Result {
    script, err := ctx.SelectedImpl.ResolveScript(ctx.Invowkfile.FilePath)
    if err != nil {
        return &Result{ExitCode: 1, Error: err}
    }

    output := newStreamingOutput(ctx)

    rtConfig := ctx.SelectedImpl.GetRuntimeConfig(ctx.SelectedRuntime)
    if rtConfig == nil {
        return r.executeWithShellCommon(ctx, script, output)
    }

    interpInfo := rtConfig.ResolveInterpreterFromScript(script)
    if interpInfo.Found {
        return r.executeWithInterpreterCommon(ctx, script, interpInfo, output)
    }

    return r.executeWithShellCommon(ctx, script, output)
}

func (r *NativeRuntime) ExecuteCapture(ctx *ExecutionContext) *Result {
    script, err := ctx.SelectedImpl.ResolveScript(ctx.Invowkfile.FilePath)
    if err != nil {
        return &Result{ExitCode: 1, Error: err}
    }

    output, captured := newCapturingOutput()

    rtConfig := ctx.SelectedImpl.GetRuntimeConfig(ctx.SelectedRuntime)
    if rtConfig == nil {
        result := r.executeWithShellCommon(ctx, script, output)
        result.Output = captured.stdout.String()
        result.ErrOutput = captured.stderr.String()
        return result
    }

    interpInfo := rtConfig.ResolveInterpreterFromScript(script)
    if interpInfo.Found {
        result := r.executeWithInterpreterCommon(ctx, script, interpInfo, output)
        result.Output = captured.stdout.String()
        result.ErrOutput = captured.stderr.String()
        return result
    }

    result := r.executeWithShellCommon(ctx, script, output)
    result.Output = captured.stdout.String()
    result.ErrOutput = captured.stderr.String()
    return result
}
```

---

## Expected Line Reduction

| Component | Before | After | Savings |
|-----------|--------|-------|---------|
| Working dir setup | 5×8 = 40 | 1×15 = 15 | 25 lines |
| Exit code handling | 4×8 = 32 | 1×8 = 8 | 24 lines |
| Output configuration | Inline | Helper funcs | ~20 lines saved |
| Shell variants | 2×45 = 90 | 1×50 = 50 | 40 lines |
| Interpreter variants | 2×65 = 130 | 1×70 = 70 | 60 lines |
| **Total** | 578 | ~350 | **~230 lines (40%)** |

---

## File Structure After Deduplication

```
internal/runtime/
├── runtime.go           # Interfaces (unchanged)
├── native.go            # ~350 lines (reduced from 578)
├── native_helpers.go    # ~200 lines (NEW)
├── virtual.go           # Unchanged
└── container.go         # + ExecuteCapture() (~30 lines added)
```

---

## Implementation Order

1. **Create `native_helpers.go`** with helper types and functions
2. **Add `executeWithShellCommon()`** - unified shell execution
3. **Add `executeWithInterpreterCommon()`** - unified interpreter execution
4. **Update `Execute()`** to use new helpers
5. **Update `ExecuteCapture()`** to use new helpers
6. **Remove old duplicated functions** (4 functions)
7. **Run tests** to verify no regressions

---

## Verification Checklist

After deduplication:

- [ ] `native.go` ≤ 400 lines
- [ ] `native_helpers.go` ≤ 250 lines
- [ ] All 4 duplicated functions removed
- [ ] `Execute()` uses `executeWithShellCommon()` / `executeWithInterpreterCommon()`
- [ ] `ExecuteCapture()` uses same helpers with capturing output
- [ ] `make test` passes (especially `TestNativeRuntime*`)
- [ ] `make lint` passes
- [ ] New file has SPDX header

---

## ContainerRuntime.ExecuteCapture() Addition

This is a separate change (not deduplication) but related to completing the `CapturingRuntime` interface.

### Implementation Pattern

Follow `VirtualRuntime.ExecuteCapture()` pattern:

```go
// ExecuteCapture executes the command in a container and captures stdout/stderr.
func (r *ContainerRuntime) ExecuteCapture(ctx *ExecutionContext) *Result {
    // Same preparation as Execute()
    image, script, err := r.prepareContainerExecution(ctx)
    if err != nil {
        return &Result{ExitCode: 1, Error: err}
    }

    var stdout, stderr bytes.Buffer

    // Run with buffer capture instead of ctx streams
    exitCode, err := r.runInContainer(ctx, image, script, &stdout, &stderr)

    return &Result{
        ExitCode:  exitCode,
        Output:    stdout.String(),
        ErrOutput: stderr.String(),
        Error:     err,
    }
}
```

### Verification

- [ ] `ContainerRuntime` implements `CapturingRuntime` interface
- [ ] Tests added for `ContainerRuntime.ExecuteCapture()`
- [ ] Container dependency validation uses `ExecuteCapture()` when available
