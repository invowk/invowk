# Quickstart: Go Codebase Quality Audit

**Feature Branch**: `006-go-codebase-audit`
**Date**: 2026-01-30

## Overview

This guide provides a step-by-step implementation sequence for the codebase quality audit. Each step is designed to be atomic and verifiable before moving to the next.

---

## Prerequisites

Before starting implementation:

1. **Verify baseline passes**: `make lint && make test`
2. **Review contracts**: Read files in `specs/006-go-codebase-audit/contracts/`
3. **Understand patterns**: Review `.claude/rules/servers.md` and `.claude/rules/functional-options.md`

---

## Implementation Phases

### Phase 1: Foundation (Extract Shared Packages)

**Goal**: Create new internal packages without breaking existing code.

#### Step 1.1: Create `internal/core/serverbase/`

1. Create package with `State` type and constants
2. Create `Base` struct with common fields
3. Create functional options (`WithErrorChannel`)
4. Create lifecycle helper methods
5. Add comprehensive tests (state transitions, races)

**Verification**: `go test -v -race ./internal/core/serverbase/...`

#### Step 1.2: Create `internal/cueutil/`

1. Create `ParseAndDecode[T]` generic function
2. Create `FormatError` helper
3. Create functional options (`WithMaxFileSize`, `WithConcrete`, `WithFilename`)
4. Add tests for all three target types (Invkfile, Invkmod, Config)

**Verification**: `go test -v ./internal/cueutil/...`

---

### Phase 2: Migrate Server Components

**Goal**: SSH server and TUI server use the shared serverbase.

#### Step 2.1: Migrate SSH Server

1. Import `internal/core/serverbase`
2. Embed `serverbase.Base` in `Server` struct
3. Replace duplicate state machine code with base methods
4. Verify all existing tests pass
5. Run race detection: `go test -v -race ./internal/sshserver/...`

**Verification**: All SSH server tests pass, including `TestServerStartWithCancelledContext`

#### Step 2.2: Migrate TUI Server

1. Import `internal/core/serverbase`
2. Embed `serverbase.Base` in `Server` struct
3. Replace duplicate state machine code with base methods
4. Verify all existing tests pass

**Verification**: All TUI server tests pass

---

### Phase 3: Migrate Container Engine

**Goal**: Docker and Podman engines use shared base implementation.

#### Step 3.1: Create `engine_base.go`

1. Create `BaseCLIEngine` struct
2. Implement `BuildArgs()`, `RunArgs()`, `ExecArgs()` methods
3. Implement `ResolveDockerfilePath()` helper
4. Add functional options for testing (`WithExecCommand`)
5. Add comprehensive tests

**Verification**: `go test -v ./internal/container/...` (new tests)

#### Step 3.2: Migrate Docker Engine

1. Embed `BaseCLIEngine` in `DockerEngine`
2. Replace duplicate code with base method calls
3. Keep only Docker-specific: version format
4. Verify existing tests pass

**Verification**: All Docker engine tests pass

#### Step 3.3: Migrate Podman Engine

1. Embed `BaseCLIEngine` in `PodmanEngine`
2. Replace duplicate code with base method calls
3. Keep only Podman-specific: version format, SELinux labels
4. Verify existing tests pass

**Verification**: All Podman engine tests pass

---

### Phase 4: Migrate CUE Parsing

**Goal**: All CUE parsing uses the shared `cueutil` package.

#### Step 4.1: Migrate `pkg/invkfile/parse.go`

1. Import `internal/cueutil`
2. Replace 3-step parsing with `cueutil.ParseAndDecode[Invkfile]`
3. Keep invkfile-specific error handling
4. Verify schema sync tests pass

**Verification**: `go test -v ./pkg/invkfile/...`

#### Step 4.2: Migrate `pkg/invkmod/invkmod.go`

1. Import `internal/cueutil`
2. Replace 3-step parsing with `cueutil.ParseAndDecode[Invkmod]`
3. Verify schema sync tests pass

**Verification**: `go test -v ./pkg/invkmod/...`

#### Step 4.3: Migrate `internal/config/config.go`

1. Import `internal/cueutil`
2. Replace CUE parsing with `cueutil.ParseAndDecode[Config]`
3. Note: Config uses `cue.Concrete(false)` - use `WithConcrete(false)`
4. Verify schema sync tests pass

**Verification**: `go test -v ./internal/config/...`

---

### Phase 5: File Splitting

**Goal**: Split large files without changing behavior.

#### General Approach for Each File

1. Identify logical concerns in the file
2. Create new files named `<original>_<concern>.go`
3. Move code to new files (preserve order: const → var → type → func)
4. Update imports
5. Run `make lint` to verify declaration ordering
6. Run tests to verify no behavior change

#### Step 5.1: Split `cmd/invowk/module.go` (1,118 lines)

Target files:
- `module.go` - Root command, shared helpers (keep ~200 lines)
- `module_validate.go` - Validation subcommands
- `module_create.go` - Create subcommand
- `module_alias.go` - Alias management
- `module_package.go` - Package/unpackage subcommands

**Verification**: `go test -v ./cmd/invowk/...` (module tests)

#### Step 5.2: Split `cmd/invowk/cmd_validate.go` (920 lines)

Target files:
- `cmd_validate.go` - Main validate command (keep ~200 lines)
- `cmd_validate_runtime.go` - Runtime validation logic
- `cmd_validate_deps.go` - Dependency validation logic
- `cmd_validate_schema.go` - Schema validation logic

**Verification**: `go test -v ./cmd/invowk/...` (validate tests)

#### Step 5.3: Split `internal/runtime/container.go` (917 lines)

Target files:
- `container.go` - ContainerRuntime struct, interface (keep ~200 lines)
- `container_build.go` - Build-related methods
- `container_exec.go` - Execute/ExecuteCapture methods
- `container_provision.go` - Auto-provisioning logic

**Verification**: `go test -v ./internal/runtime/...`

#### Step 5.4: Split `internal/tui/interactive.go` (806 lines)

Target files:
- `interactive.go` - Model definition, Init (keep ~200 lines)
- `interactive_update.go` - Update method and message handling
- `interactive_view.go` - View method and rendering

**Verification**: `go test -v ./internal/tui/...`

#### Step 5.5: Split `pkg/invkmod/operations.go` (827 lines)

Target files:
- `operations.go` - Shared types and helpers (keep ~200 lines)
- `operations_validate.go` - Validation operations
- `operations_create.go` - Create operations
- `operations_package.go` - Package/unpackage operations

**Verification**: `go test -v ./pkg/invkmod/...`

---

### Phase 6: Split Test Files

**Goal**: Test files under 800 lines.

#### Step 6.1: Split `internal/runtime/container_integration_test.go` (847 lines)

Target files:
- `container_build_integration_test.go` - Build tests
- `container_exec_integration_test.go` - Execution tests

#### Step 6.2: Split `pkg/invkmod/operations_packaging_test.go` (817 lines)

Target files:
- `operations_zip_test.go` - Zip/package tests
- `operations_unzip_test.go` - Unzip/extract tests

#### Step 6.3: Split `pkg/invkfile/invkfile_flags_enhanced_test.go` (814 lines)

Target files:
- `invkfile_flags_validation_test.go` - Flag validation tests
- `invkfile_flags_parsing_test.go` - Flag parsing tests

**Verification**: `make test` (all tests pass)

---

### Phase 7: CUE Schema Enhancement

**Goal**: Add missing validation constraints per FR-008/FR-009.

#### Step 7.1: Update `pkg/invkfile/invkfile_schema.cue`

Add constraints:
```cue
// #Command.description - add non-empty-with-content validation
description?: string & =~"^\\s*\\S.*$"

// #RuntimeConfigContainer.image - add length limit
image?: string & strings.MaxRunes(512)

// #RuntimeConfigNative.interpreter - add length limit
interpreter?: string & =~"^\\s*\\S.*$" & strings.MaxRunes(1024)

// #RuntimeConfigContainer.interpreter - add length limit
interpreter?: string & =~"^\\s*\\S.*$" & strings.MaxRunes(1024)

// #Flag.default_value - add length limit
default_value?: string & strings.MaxRunes(4096)

// #Argument.default_value - add length limit
default_value?: string & strings.MaxRunes(4096)
```

#### Step 7.2: Update Schema Sync Tests

1. Add boundary tests for new constraints
2. Verify error messages include CUE paths
3. Run `go test -v ./pkg/invkfile/...` including sync tests

**Verification**: `make lint && make test`

---

### Phase 8: Error Message Standardization

**Goal**: All user-facing errors include operation, resource, and suggestion.

#### Step 8.1: Fix Config Loading Errors (FR-011)

In `internal/config/config.go`:
- Surface config loading errors regardless of verbose mode
- Add actionable error context

#### Step 8.2: Enhance Shell Not Found Errors (FR-013)

In `internal/runtime/native.go`:
- Include operation: "find shell"
- Include resource: list of attempted shells
- Include suggestion: installation hints
- Full chain with --verbose

#### Step 8.3: Enhance Container Build Errors (FR-013)

In `internal/container/`:
- Include operation: "build container"
- Include resource: image/containerfile path
- Include suggestion: check Dockerfile syntax or path

#### Step 8.4: Review and Update Other Error Sites

Search for `fmt.Errorf("failed to` and update key user-facing errors.

**Verification**: Manual testing of error scenarios

---

### Phase 9: Documentation and Validation

**Goal**: Update docs and verify all success criteria.

#### Step 9.1: Update Documentation Sync Map

If any documented file paths changed, update:
- `website/docs/` references
- `.claude/rules/` references
- Sample module references

#### Step 9.2: Validate Success Criteria

| Criteria | Verification |
|----------|--------------|
| SC-001: No file >700 lines | `find . -name "*.go" -exec wc -l {} \; | sort -rn | head -20` |
| SC-002: 40% reduction in 500+ line files | Compare before/after counts |
| SC-003: 15% line reduction in affected packages | Compare before/after |
| SC-004: All schemas have length constraints | Review schema files |
| SC-005: 100% error context | Audit error sites |
| SC-006: All tests pass | `make lint && make test` |
| SC-007: Better navigation | Subjective evaluation |

---

## Checkpoints

After each phase, verify:

1. **Linting**: `make lint`
2. **Unit Tests**: `make test`
3. **CLI Tests**: `make test-cli` (if CLI affected)
4. **License Headers**: `make license-check` (for new files)
5. **Module Validation**: `go run . module validate modules/*.invkmod --deep`

---

## Rollback Strategy

Each phase is designed to be independently revertible:

1. New packages (Phase 1) can be deleted
2. Migration phases can be reverted by uncommitting
3. File splits can be combined back
4. Schema changes can be reverted (with data migration note)

Keep commits atomic and well-described for easy bisect if issues arise.
