# Feature Specification: Codebase Cleanup Audit

**Feature Branch**: `002-codebase-cleanup-audit`
**Created**: 2026-01-29
**Status**: Draft
**Input**: Comprehensive analysis of Go codebase to identify dead/unused code, oversized abstractions, and partially implemented features.

## Clarifications

### Session 2026-01-29

- Q: How should `ContainerRuntime.ExecuteCapture()` capture output given containers execute via `docker exec`/`podman exec`? → A: Route container engine's stdout/stderr streams to buffers (mirror native/virtual pattern)
- Q: Should `IsWindowsReservedName()` be moved to a new `pkg/platform/` or duplicated in `pkg/invowkmod/`? → A: Create `pkg/platform/` as new public package
- Q: What logical responsibility groupings should guide `cmd/invowk/cmd.go` decomposition? → A: Discovery, Execution, Validation, Output/Errors (4 files + main cmd.go)

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Developer Navigating Large Files (Priority: P1)

When developers (human or AI agents) work on the invowk codebase, they encounter files that are difficult to comprehend due to their size. The largest files (`cmd/invowk/cmd.go` at 2,927 lines, `internal/runtime/container.go` at 745 lines) mix multiple responsibilities, making it hard to locate specific functionality.

**Why this priority**: Large files directly impact developer productivity and increase the risk of introducing bugs. AI coding agents perform significantly worse on files exceeding 500-700 lines.

**Independent Test**: Can be verified by confirming no single file exceeds 800 lines after refactoring, while all existing tests pass.

**Acceptance Scenarios**:

1. **Given** a developer opens `cmd/invowk/cmd.go`, **When** they need to find command execution logic, **Then** they should find it in a dedicated file (e.g., `cmd_execute.go`) with focused responsibilities.
2. **Given** an AI coding agent analyzes the codebase, **When** it reads any single Go file, **Then** that file should not exceed 800 lines.

---

### User Story 2 - Developer Using Container Runtime Output Capture (Priority: P1)

Developers rely on the `CapturingRuntime` interface to capture command output for dependency validation and programmatic use. Currently, `NativeRuntime` and `VirtualRuntime` implement `ExecuteCapture()`, but `ContainerRuntime` does not.

**Why this priority**: This is missing functionality that breaks API completeness. Users cannot capture output from container-executed commands, affecting dependency validation reliability.

**Independent Test**: Can be verified by implementing `ExecuteCapture()` on `ContainerRuntime` and running existing capture tests with container runtime.

**Acceptance Scenarios**:

1. **Given** a command with container runtime and tool dependencies, **When** invowk validates dependencies, **Then** it should use `ExecuteCapture()` to check tool versions correctly.
2. **Given** a developer calls `ContainerRuntime.ExecuteCapture()`, **When** the command produces output, **Then** `Result.Output` and `Result.ErrOutput` should contain the captured streams.

---

### User Story 3 - External Consumer Using `pkg/invowkmod` (Priority: P2)

External developers importing `pkg/invowkmod` as a library expect it to be a self-contained public API. Currently, `pkg/invowkmod/operations.go` imports `internal/platform`, violating Go's layering conventions.

**Why this priority**: Architectural layering violations make the public API unstable and create hidden dependencies for external consumers.

**Independent Test**: Can be verified by checking that `pkg/invowkmod` has no imports from `internal/` packages after refactoring.

**Acceptance Scenarios**:

1. **Given** an external project imports `pkg/invowkmod`, **When** it builds, **Then** it should not transitively depend on any `internal/` packages.
2. **Given** the codebase is analyzed with `go mod graph`, **When** checking `pkg/invowkmod` imports, **Then** no `internal/*` imports should exist.

---

### User Story 4 - Developer Reducing Code Duplication (Priority: P2)

The codebase contains duplicated patterns, particularly in `internal/runtime/native.go` where four execution functions share ~75% code, and in server lifecycle management where `sshserver` and `tuiserver` implement identical state machines.

**Why this priority**: Duplication increases maintenance burden and bug risk (fixes must be applied in multiple locations).

**Independent Test**: Can be verified by counting deduplicated lines and ensuring all execution paths still work correctly.

**Acceptance Scenarios**:

1. **Given** a developer fixes a bug in shell execution logic, **When** they apply the fix, **Then** they should only need to modify one location, not four.
2. **Given** the codebase duplication analysis, **When** comparing native.go execution functions, **Then** shared logic should be extracted to helper functions.

---

### Edge Cases

- What happens when decomposing a file breaks circular dependencies?
  - Ensure all imports remain unidirectional after file splits.
- How does the system handle if extracted helpers need to be tested in isolation?
  - Extracted functions should be testable with existing test infrastructure.
- What if `ExecuteCapture()` for containers requires different error handling?
  - Follow the patterns established in `NativeRuntime.ExecuteCapture()` and `VirtualRuntime.ExecuteCapture()`.

## Requirements *(mandatory)*

### Functional Requirements

**File Decomposition (Agent Optimization)**

- **FR-001**: `cmd/invowk/cmd.go` MUST be split into files where no single file exceeds 800 lines, using these responsibility groupings:
  - `cmd_discovery.go` - Command discovery and invowkfile parsing
  - `cmd_execute.go` - Execution orchestration and runtime coordination
  - `cmd_validate.go` - Dependency validation (tools, files, capabilities)
  - `cmd_validate_input.go` - User input validation (flags, arguments)
  - `cmd_render.go` - Output formatting and error rendering
  - `cmd.go` - Main command registration and shared types
- **FR-002**: Each extracted file MUST have a single, clear responsibility aligned with the groupings above.
- **FR-003**: ~~`internal/runtime/container.go` MUST be decomposed with image management, SSH integration, and working directory logic in separate files.~~ **DEFERRED** - Container runtime file (745 lines) is under the 800-line target and has focused responsibility. Decomposition deferred to future spec if file grows.

**Missing Functionality**

- **FR-004**: `ContainerRuntime` MUST implement the `CapturingRuntime` interface by adding an `ExecuteCapture()` method.
- **FR-005**: The `ExecuteCapture()` implementation MUST capture both stdout and stderr into `Result.Output` and `Result.ErrOutput` by routing the container engine's streams to buffers (mirroring the native/virtual pattern).

**Architectural Fixes**

- **FR-006**: `pkg/invowkmod` MUST NOT import any packages from `internal/`.
- **FR-007**: The `IsWindowsReservedName()` function MUST be moved to a new `pkg/platform/` public package (establishing a home for cross-platform utilities).

**Code Duplication**

- **FR-008**: Common execution setup logic in `native.go` (env building, working directory setup, exit code handling) MUST be extracted to shared helper functions.
- **FR-009**: ~~The server state machine pattern SHOULD be extracted to a shared `internal/serverbase/` package.~~ **DEFERRED** - Both `sshserver` and `tuiserver` already follow the documented pattern correctly. Extraction deferred until a third server type is added (see plan.md R5 rationale).

**Test Coverage**

- **FR-010**: New unit tests MUST be added for `ContainerRuntime.ExecuteCapture()`.
- **FR-011**: Existing tests MUST continue to pass after all refactoring.

**Constraints**

- **FR-012**: All changes MUST NOT alter any user-facing CLI behavior, flags, or output formats.
- **FR-013**: All changes MUST maintain backward compatibility with existing `invowkfile.cue` and `invowkmod.cue` files.

### Key Entities

- **ContainerRuntime**: Container execution runtime, currently missing `ExecuteCapture()` method.
- **NativeRuntime**: Native shell execution runtime with duplicated execution functions.
- **CapturingRuntime**: Interface defining `ExecuteCapture()` for output capture (defined in `runtime.go:102-106`).
- **ServerLifecycle**: Shared state machine pattern used by SSH and TUI servers.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: No Go source file in the codebase exceeds 800 lines (currently `cmd.go` is 2,927 lines).
- **SC-002**: All three runtime types (`native`, `virtual`, `container`) implement the `CapturingRuntime` interface.
- **SC-003**: `pkg/invowkmod` has zero imports from `internal/` packages.
- **SC-004**: Code duplication in `native.go` execution functions is reduced by at least 50% (from ~4 copies to shared helpers).
- **SC-005**: 100% of existing tests pass after refactoring (`make test`, `make test-cli`).
- **SC-006**: `make lint` passes with no new warnings.
- **SC-007**: License check passes for all files (`make license-check`).

## Assumptions

1. **File size target of 800 lines** is based on typical AI agent context window efficiency - this is a reasonable default based on industry practice for agentic coding optimization.
2. **Server state machine extraction is optional** because both implementations follow the documented pattern in `.claude/rules/servers.md` and may intentionally remain separate for clarity.
3. **Windows reserved name validation** can safely be moved to `pkg/platform/` as it has no dependencies on internal implementation details.
4. **Container ExecuteCapture** follows the same pattern as native/virtual - capturing stdout/stderr to buffers while running the container command.

## Out of Scope

The following items are **partially implemented features** that should be tracked separately in `specs/next/`:

1. **Custom config file path** (`--config` flag) - declared but not implemented (TODO in `root.go:23`)
2. **Force rebuild container images** - TODO comment at `container.go:683`
3. **enable_uroot_utils** - wired in code but functionality is stubbed

These are intentionally excluded from this cleanup audit as they require new feature design, not code cleanup.

4. **Container runtime decomposition** (`internal/runtime/container.go`) - File is 745 lines, under target. Decomposition deferred unless file grows beyond 800 lines.
5. **Server state machine extraction** (`internal/serverbase/`) - Both servers follow documented pattern correctly. Extraction deferred until a third server type is added.
