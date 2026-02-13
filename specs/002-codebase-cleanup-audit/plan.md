# Implementation Plan: Codebase Cleanup Audit

**Branch**: `002-codebase-cleanup-audit` | **Date**: 2026-01-29 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/002-codebase-cleanup-audit/spec.md`

## Summary

This is an internal refactoring effort to improve codebase maintainability without changing user-facing behavior. The primary goals are:

1. **Decompose `cmd/invowk/cmd.go`** (2,927 lines) into focused files under 800 lines each
2. **Implement `ContainerRuntime.ExecuteCapture()`** to complete the `CapturingRuntime` interface
3. **Fix architectural layering violation** by moving `IsWindowsReservedName()` to `pkg/platform/`
4. **Reduce code duplication** in `internal/runtime/native.go` execution functions

## Technical Context

**Language/Version**: Go 1.26+
**Primary Dependencies**: Cobra (CLI), Viper (config), CUE (schemas), Charm libraries (TUI), mvdan/sh (virtual shell)
**Storage**: N/A (CLI tool, file-based configuration)
**Testing**: Go test with testscript for CLI integration tests
**Target Platform**: Linux, macOS, Windows (cross-platform CLI)
**Project Type**: Single CLI binary
**Performance Goals**: <100ms startup for `invowk --help`
**Constraints**: No user-facing behavior changes, backward compatibility with existing invowkfiles/invowkmods
**Scale/Scope**: ~47,500 lines of Go code across 80 production files

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Idiomatic Go & Schema-Driven Design | ✅ PASS | Refactoring will follow existing patterns; SPDX headers required for new files |
| II. Comprehensive Testing Discipline | ✅ PASS | Existing tests must pass; no new behavior = no new tests except for `ExecuteCapture()` |
| III. Consistent User Experience | ✅ PASS | No user-facing changes by design (FR-012, FR-013) |
| IV. Single-Binary Performance | ✅ PASS | Refactoring should not impact startup time |
| V. Simplicity & Minimalism | ✅ PASS | Reducing file sizes and duplication improves simplicity |
| VI. Documentation Synchronization | ✅ PASS | No user-facing changes = no documentation updates required |
| VII. Pre-Existing Issue Resolution | ⚠️ MONITOR | This spec IS the pre-existing issue resolution; no new blockers expected |

**Gate Status**: ✅ PASS - Proceed to Phase 0

## Project Structure

### Documentation (this feature)

```text
specs/002-codebase-cleanup-audit/
├── spec.md              # Feature specification (completed)
├── plan.md              # This file
├── research.md          # Phase 0 output - codebase analysis findings
├── decomposition.md     # Phase 1 output - file split design (replaces data-model.md)
├── deduplication.md     # Phase 1 output - shared helper design (replaces contracts/)
└── tasks.md             # Phase 2 output (/speckit.tasks command)
```

### Source Code (affected areas)

```text
cmd/invowk/
├── cmd.go               # 2,927 → ~350 lines (main command registration + shared types)
├── cmd_discovery.go     # NEW: Command discovery and invowkfile parsing (~625 lines)
├── cmd_execute.go       # NEW: Execution orchestration and runtime coordination (~555 lines)
├── cmd_validate.go      # NEW: Dependency validation (tools, files, capabilities) (~770 lines)
├── cmd_validate_input.go # NEW: User input validation (flags, arguments) (~160 lines)
├── cmd_render.go        # NEW: Output formatting and error rendering (~420 lines)
└── [existing files unchanged]

internal/runtime/
├── native.go            # 578 → ~350 lines after extracting shared helpers
├── native_helpers.go    # NEW: Shared execution setup/teardown helpers (~200 lines)
├── container.go         # 745 lines - add ExecuteCapture() method (~30 lines added)
└── [other files unchanged]

pkg/platform/            # NEW public package
└── windows.go           # Moved from internal/platform/windows.go

pkg/invowkmod/
└── operations.go        # Update import from internal/platform → pkg/platform

pkg/invowkfile/
└── validation.go        # Update import from internal/platform → pkg/platform
```

**Structure Decision**: Existing Go project structure preserved. Changes are file-level decomposition within existing packages, plus one new public package (`pkg/platform/`).

## Complexity Tracking

> No constitution violations requiring justification. This refactoring reduces complexity.

| Change | Complexity Impact | Rationale |
|--------|-------------------|-----------|
| 5 new files in `cmd/invowk/` | Neutral | Splitting 1 monolithic file into focused files |
| 1 new file in `internal/runtime/` | Reduction | Extracting shared code reduces duplication |
| 1 new package `pkg/platform/` | Minor increase | Required to fix layering violation; minimal surface area |

---

## Phase 0: Research Findings

### R1: cmd.go Decomposition Strategy

**Decision**: Split by functional responsibility into 5 files

**Current Function Distribution** (2,927 lines, 69 functions):

| Responsibility | Functions | Lines (approx) | Target File |
|---------------|-----------|----------------|-------------|
| Command registration & routing | 8 | ~400 | `cmd.go` |
| Command discovery & parsing | 12 | ~600 | `cmd_discovery.go` |
| Execution orchestration | 15 | ~700 | `cmd_execute.go` |
| Dependency validation | 18 | ~800 | `cmd_validate.go` |
| Output formatting & errors | 16 | ~400 | `cmd_output.go` |

**Key Functions to Move**:
- `cmd_discovery.go`: `registerDiscoveredCommands()`, `listCommands()`, discovery helpers
- `cmd_execute.go`: `runCommandWithFlags()`, `ensureSSHServer()`, `stopSSHServer()`, execution helpers
- `cmd_validate.go`: `validateDependencies()`, tool/file/capability validators
- `cmd_output.go`: `Render*Error()` functions, output formatters

**Alternatives Considered**:
1. ❌ Single package split (too granular, import cycles)
2. ❌ Separate packages (unnecessary, adds complexity)
3. ✅ Same package, multiple files (Go-idiomatic, no import changes)

### R2: Native Runtime Deduplication

**Decision**: Extract shared execution logic into helper functions

**Current Duplication Pattern**:
```
executeWithShell()        ─┬─ 95% shared with ─── executeCaptureWithShell()
executeWithInterpreter()  ─┴─ 95% shared with ─── executeCaptureWithInterpreter()
```

**Shared Code Blocks** (each appears 4-6 times):
1. Script resolution: `ctx.SelectedImpl.ResolveScript()`
2. Working directory setup: `os.Chdir()` + cleanup
3. Environment building: `buildRuntimeEnv()`
4. Exit code extraction: `exec.ExitError` handling

**Proposed Helpers**:
```go
// executeCommon handles shared setup for shell/interpreter execution
type executeOptions struct {
    capture bool           // true for ExecuteCapture, false for Execute
    stdout  io.Writer      // ctx.Stdout or &bytes.Buffer{}
    stderr  io.Writer      // ctx.Stderr or &bytes.Buffer{}
}

func (r *NativeRuntime) executeWithShellCommon(ctx *ExecutionContext, opts executeOptions) *Result
func (r *NativeRuntime) executeWithInterpreterCommon(ctx *ExecutionContext, opts executeOptions) *Result
```

**Alternatives Considered**:
1. ❌ Keep duplication (violates DRY, maintenance risk)
2. ❌ Merge into single function with many parameters (harder to read)
3. ✅ Options pattern with shared helpers (clean, testable)

### R3: ContainerRuntime.ExecuteCapture() Implementation

**Decision**: Mirror NativeRuntime/VirtualRuntime pattern with buffer capture

**Reference Implementation** (`virtual.go:150-216`):
```go
func (r *VirtualRuntime) ExecuteCapture(ctx *ExecutionContext) *Result {
    var stdout, stderr bytes.Buffer
    opts := []interp.RunnerOption{
        interp.StdIO(nil, &stdout, &stderr),
        // ...
    }
    // execute...
    return &Result{Output: stdout.String(), ErrOutput: stderr.String(), ...}
}
```

**Container-Specific Consideration**:
- Container execution uses `docker exec` / `podman exec`
- Stdout/stderr are already separate streams from container engine
- Implementation: Route engine's stdout/stderr to `bytes.Buffer` instead of `os.Stdout/Stderr`

**Affected Code** (`container.go`):
- Current `Execute()` passes `ctx.Stdout`, `ctx.Stderr` to `runInContainer()`
- New `ExecuteCapture()` will pass `&bytes.Buffer{}` instances instead

### R4: Platform Package Migration

**Decision**: Create `pkg/platform/` with minimal surface area

**Files to Create**:
```
pkg/platform/
├── doc.go           # Package documentation
├── windows.go       # IsWindowsReservedName() + WindowsReservedNames map
└── windows_test.go  # Tests (move from internal/platform/)
```

**Import Updates Required**:
1. `pkg/invowkmod/operations.go:14` - Change `internal/platform` → `pkg/platform`
2. `pkg/invowkfile/validation.go:23` - Change `internal/platform` → `pkg/platform`

**Migration Strategy**:
1. Create `pkg/platform/` with copied content
2. Update imports in `pkg/invowkmod/` and `pkg/invowkfile/`
3. Update any internal imports to use `pkg/platform/`
4. Delete `internal/platform/` (after verifying no internal-only consumers)

### R5: Server State Machine Extraction (DEFERRED)

**Decision**: Defer to separate spec (FR-009 marked as SHOULD)

**Rationale**:
- Both `sshserver` and `tuiserver` already follow the documented pattern excellently
- Extraction adds abstraction without clear benefit (they're already correct)
- Risk of breaking well-tested code outweighs marginal deduplication gain
- Can be revisited if a third server type is added

**Alternatives Considered**:
1. ❌ Extract to `internal/serverbase/` (over-engineering for 2 servers)
2. ❌ Use generics (Go 1.26 generics don't improve this pattern)
3. ✅ Defer (servers work correctly, documented pattern is sufficient)

---

## Phase 1: Design Artifacts

### Decomposition Design (replaces data-model.md)

See `decomposition.md` for detailed file-by-file breakdown including:
- Function assignments per file
- Import dependencies (ensuring no cycles)
- Shared type definitions remaining in `cmd.go`
- Test file organization

### Deduplication Design (replaces contracts/)

See `deduplication.md` for detailed helper function signatures including:
- `executeOptions` struct definition
- `executeCommon()` helper interface
- Working directory manager
- Exit code extractor

---

## Constitution Re-Check (Post Phase 1)

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Idiomatic Go & Schema-Driven Design | ✅ PASS | File split follows Go conventions; new files get SPDX headers |
| II. Comprehensive Testing Discipline | ✅ PASS | All existing tests pass; new tests for `ExecuteCapture()` |
| III. Consistent User Experience | ✅ PASS | No user-facing changes |
| IV. Single-Binary Performance | ✅ PASS | No performance impact from refactoring |
| V. Simplicity & Minimalism | ✅ PASS | Complexity reduced, not increased |
| VI. Documentation Synchronization | ✅ PASS | No user-facing changes = no docs needed |
| VII. Pre-Existing Issue Resolution | ✅ PASS | This spec addresses the pre-existing issues |

**Final Gate Status**: ✅ PASS - Ready for task generation

---

## Next Steps

1. Run `/speckit.tasks` to generate implementation tasks
2. Tasks will be ordered: decomposition → deduplication → new functionality → cleanup
3. Each task verifiable via `make test` and file line counts
