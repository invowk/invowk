# Implementation Plan: Go Codebase Quality Audit

**Branch**: `006-go-codebase-audit` | **Date**: 2026-01-30 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/006-go-codebase-audit/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/commands/plan.md` for the execution workflow.

## Summary

This audit addresses technical debt in the Invowk Go codebase by:
1. Splitting files over 800 lines into focused, single-responsibility files under 600 lines
2. Extracting duplicate code patterns (server state machine, container engines, CUE parsing) into reusable abstractions
3. Adding missing CUE schema validation constraints for defense-in-depth
4. Standardizing error messages to include operation, resource, and actionable suggestions

## Technical Context

**Language/Version**: Go 1.26+
**Primary Dependencies**: CUE v0.15.3, Cobra, Viper, Bubble Tea, mvdan/sh
**Storage**: N/A (no persistent storage)
**Testing**: Go stdlib `testing`, `testscript` for CLI integration
**Target Platform**: Linux, macOS, Windows (CLI tool)
**Project Type**: Single CLI binary with internal packages
**Performance Goals**: <100ms startup for `invowk --help` (per constitution)
**Constraints**: Single-binary distribution, no external dependencies at runtime
**Scale/Scope**: 62,974 LOC across 202 Go files, ~311 lines average per file

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Idiomatic Go & Schema-Driven Design | ✅ PASS | Follows existing patterns; extractions use functional options |
| II. Comprehensive Testing Discipline | ✅ PASS | Tests required for all extracted code; sync tests for CUE changes |
| III. Consistent User Experience | ✅ PASS | Error message standardization improves UX |
| IV. Single-Binary Performance | ✅ PASS | No new dependencies; refactoring only |
| V. Simplicity & Minimalism | ✅ PASS | Extracting duplicates reduces complexity |
| VI. Documentation Synchronization | ✅ PASS | File path changes require docs sync map updates |
| VII. Pre-Existing Issue Resolution | ⚠️ WATCH | May discover issues during refactoring |

## Project Structure

### Documentation (this feature)

```text
specs/006-go-codebase-audit/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (/speckit.plan command)
├── data-model.md        # Phase 1 output (/speckit.plan command)
├── quickstart.md        # Phase 1 output (/speckit.plan command)
├── contracts/           # Phase 1 output (/speckit.plan command)
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)

```text
# Existing Go codebase structure (will be modified)
cmd/invowk/                    # CLI commands (34 files, 9,021 lines)
├── module.go → module_*.go    # Split: validate, create, alias, package
├── cmd_validate.go → split    # Split by validation concern
└── ...

internal/                      # Private packages
├── core/                      # NEW: Shared types to break import cycles
│   └── serverbase/            # NEW: Server state machine base type
├── container/                 # Container engines (will extract base)
│   ├── engine.go              # Interface definitions (unchanged)
│   ├── engine_base.go         # NEW: Base CLI engine implementation
│   ├── docker.go              # Simplified: embeds base
│   └── podman.go              # Simplified: embeds base
├── runtime/
│   └── container.go → split   # Split container runtime logic
├── cueutil/                   # NEW: Shared CUE parsing utilities
│   └── parse.go               # 3-step CUE parsing helper
└── ...

pkg/                           # Public packages
├── invowkfile/                  # Invowkfile parsing (31 files, 12,671 lines)
│   └── parse.go               # Uses cueutil
└── invowkmod/                   # Module operations
    └── operations.go → split  # Split by operation type
```

**Structure Decision**: Existing structure preserved with targeted additions:
- `internal/core/serverbase/` for extracted server state machine
- `internal/cueutil/` for shared CUE parsing utilities
- File splits within existing packages (no new public packages)

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| New `internal/core/` package | Breaks import cycles when extracting shared types | Inline duplication creates maintenance burden; package boundary enables testability |
| New `internal/cueutil/` package | Consolidates CUE 3-step parsing pattern | Three separate implementations creates inconsistency risk |

---

## Phase 0 Findings (Technical Research)

### Files Requiring Split (>800 lines)

| File | Lines | Split Strategy |
|------|-------|---------------|
| cmd/invowk/module.go | 1,118 | → module_validate.go, module_create.go, module_alias.go, module_package.go |
| cmd/invowk/cmd_validate.go | 920 | → cmd_validate_runtime.go, cmd_validate_deps.go, cmd_validate_schema.go |
| internal/runtime/container.go | 917 | → container_build.go, container_exec.go, container_provision.go |
| internal/tui/interactive.go | 806 | → interactive_model.go, interactive_update.go, interactive_view.go |
| pkg/invowkmod/operations.go | 827 | → operations_validate.go, operations_create.go, operations_package.go |

### Test Files Requiring Split (>800 lines)

| File | Lines | Split Strategy |
|------|-------|---------------|
| internal/runtime/container_integration_test.go | 847 | → container_build_integration_test.go, container_exec_integration_test.go |
| pkg/invowkmod/operations_packaging_test.go | 817 | → operations_zip_test.go, operations_unzip_test.go |
| pkg/invowkfile/invowkfile_flags_enhanced_test.go | 814 | → invowkfile_flags_validation_test.go, invowkfile_flags_parsing_test.go |

### Duplication Patterns to Extract

#### 1. Server State Machine (SSH Server + TUI Server)

Both servers implement identical state machine logic:
- `ServerState` type with 6 states (Created, Starting, Running, Stopping, Stopped, Failed)
- `String()` method for state representation
- `atomic.Int32` for lock-free state reads
- `sync.Mutex` for state transitions
- Lifecycle methods: `Start()`, `Stop()`, `State()`, `IsRunning()`
- Context cancellation, WaitGroup, channel signaling

**Extraction approach**: Create `internal/core/serverbase/` with:
- `State` type and constants
- `Base` struct with common fields
- Lifecycle method helpers that concrete servers can compose

#### 2. Container Engine (Docker + Podman)

Both engines have ~80% identical code:
- `Build()` method argument construction
- `Run()` method with volume, port, env handling
- Dockerfile path resolution logic
- Command execution via `exec.Command`

**Extraction approach**: Create `engine_base.go` with:
- `BaseCLIEngine` struct with common fields
- `BuildArgs()`, `RunArgs()` helper methods
- Concrete engines embed base and override only differences (version format, SELinux labels)

#### 3. CUE 3-Step Parsing

Three implementations in `pkg/invowkfile/parse.go`, `pkg/invowkmod/invowkmod.go`, `internal/config/config.go`:
- Compile embedded schema
- Compile user data and unify
- Validate and decode to Go struct

**Extraction approach**: Create `internal/cueutil/parse.go` with:
```go
type ParseOptions struct {
    MaxFileSize  int64
    SchemaPath   string  // e.g., "#Invowkfile"
    Filename     string
}

func ParseAndDecode[T any](schema, data []byte, opts ParseOptions) (*T, error)
```

### CUE Schema Gaps

| Field | Current | Required by FR-008 |
|-------|---------|-------------------|
| `#Invowkfile.default_shell` | `strings.MaxRunes(1024)` | ✅ Already present |
| `#Command.description` | Optional, no validation | Add `=~"^\\s*\\S.*$"` (non-empty with content) |
| `#RuntimeConfigContainer.image` | No constraint | Add `strings.MaxRunes(512)` |
| `#RuntimeConfigNative.interpreter` | No length limit | Add `strings.MaxRunes(1024)` |
| `#RuntimeConfigContainer.interpreter` | No length limit | Add `strings.MaxRunes(1024)` |
| `#Flag.default_value` | No constraint | Add `strings.MaxRunes(4096)` |
| `#Argument.default_value` | No constraint | Add `strings.MaxRunes(4096)` |

### Error Message Pattern Analysis

**Current pattern**: `fmt.Errorf("failed to X: %w", err)` — lacks actionable guidance

**Required pattern** (per FR-012):
```
Operation failed: <operation>
Resource: <file/path/entity>
Suggestion: <how to fix>
[With --verbose: full error chain]
```

**Key error sites to update**:
1. `internal/config/config.go` - Config loading errors (FR-011 says these are silently ignored)
2. `internal/runtime/native.go` - Shell not found errors (FR-013 example)
3. `internal/container/engine.go` - Container build failures (FR-013 example)
4. `pkg/invowkfile/parse.go` - CUE validation errors (already have paths, need suggestions)

---

## Phase 1 Design Artifacts

✅ **Generated artifacts**:

- [data-model.md](data-model.md) - Entity definitions for extracted types (State, Base, BaseCLIEngine, ParseOptions, ActionableError)
- [contracts/serverbase.go](contracts/serverbase.go) - API contract for `internal/core/serverbase/` package
- [contracts/enginebase.go](contracts/enginebase.go) - API contract for container engine base type
- [contracts/cueutil.go](contracts/cueutil.go) - API contract for `internal/cueutil/` package
- [quickstart.md](quickstart.md) - 9-phase implementation guide with checkpoints

---

## Post-Design Constitution Re-Check

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Idiomatic Go | ✅ PASS | Functional options, composition via embedding, generics for CUE parsing |
| II. Testing | ✅ PASS | Test requirements defined for each extracted package |
| III. UX | ✅ PASS | ActionableError model provides consistent error context |
| IV. Performance | ✅ PASS | Composition has minimal overhead; no new dependencies |
| V. Simplicity | ✅ PASS | ~1500 lines of duplication reduced to shared packages |
| VI. Docs | ✅ PASS | quickstart.md includes docs sync verification step |
| VII. Pre-Existing | ⚠️ WATCH | Implementation may reveal blocking issues |

**Ready for task generation**: Use `/speckit.tasks` to generate actionable tasks.
