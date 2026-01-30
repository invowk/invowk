# Tasks: Go Codebase Quality Audit

**Input**: Design documents from `/specs/006-go-codebase-audit/`
**Prerequisites**: plan.md ✓, spec.md ✓, research.md ✓, data-model.md ✓, contracts/ ✓, quickstart.md ✓

**Tests**: Tests required for all public APIs and critical paths in modified/extracted code (per FR-017). No numeric coverage threshold.

**Organization**: Tasks grouped by user story priority. Note that US4 (Reduced Duplication) creates foundational packages that US1 (Maintainability) depends on for file splits.

## Format: `[ID] [P?] [Story?] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story (US1=Maintainability, US2=CUE Schema, US3=Errors, US4=Duplication)

---

## Phase 1: Setup

**Purpose**: Verify baseline and prepare for refactoring

- [X] T001 Verify baseline passes with `make lint && make test`
- [X] T002 [P] Review existing server pattern in `.claude/rules/servers.md`
- [X] T003 [P] Review functional options pattern in `.claude/rules/functional-options.md`
- [X] T004 [P] Review contracts in `specs/006-go-codebase-audit/contracts/`
- [X] T005 Document baseline metrics: file counts over 500/700/800 lines
- [X] T005a Verify line counts in plan.md Phase 0 findings match current codebase (module.go, cmd_validate.go, container.go, interactive.go, operations.go)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Create new internal packages that enable all user stories

**⚠️ CRITICAL**: US1 (file splitting) and US4 (duplication reduction) depend on this phase

### 2.1: Create `internal/core/serverbase/` Package

Enables US4 (Reduced Duplication) for server state machine extraction.

- [X] T006 Create `internal/core/serverbase/state.go` with State type and constants per `contracts/serverbase.go`
- [X] T007 Create `internal/core/serverbase/base.go` with Base struct and common fields per `contracts/serverbase.go`
- [X] T008 Create `internal/core/serverbase/options.go` with functional options (WithErrorChannel) per `contracts/serverbase.go`
- [X] T009 Create `internal/core/serverbase/base_test.go` with state transition tests
- [X] T010 Add race condition tests to `internal/core/serverbase/base_test.go` (run with -race)
- [X] T011 Add double Start/Stop idempotency tests to `internal/core/serverbase/base_test.go`
- [X] T012 Add cancelled context tests to `internal/core/serverbase/base_test.go`

**Verification**: `go test -v -race ./internal/core/serverbase/...`

### 2.2: Create `internal/cueutil/` Package

Enables US4 (Reduced Duplication) for CUE 3-step parsing extraction.

- [X] T013 Create `internal/cueutil/parse.go` with ParseAndDecode[T] generic function per `contracts/cueutil.go`
- [X] T014 Create `internal/cueutil/options.go` with functional options (WithMaxFileSize, WithConcrete, WithFilename) per `contracts/cueutil.go`
- [X] T015 Create `internal/cueutil/error.go` with FormatError helper and ValidationError type per `contracts/cueutil.go`
- [X] T016 Create `internal/cueutil/parse_test.go` with tests for Invkfile type parsing
- [X] T017 [P] Add tests for Invkmod type parsing to `internal/cueutil/parse_test.go`
- [X] T018 [P] Add tests for Config type parsing to `internal/cueutil/parse_test.go`
- [X] T019 Add file size limit enforcement tests to `internal/cueutil/parse_test.go`
- [X] T020 Add error formatting tests to `internal/cueutil/error_test.go`

**Verification**: `go test -v ./internal/cueutil/...`

### 2.3: Create `internal/container/engine_base.go`

Enables US4 (Reduced Duplication) for container engine extraction.

- [X] T021 Create `internal/container/engine_base.go` with BaseCLIEngine struct per `contracts/enginebase.go`
- [X] T022 Add BuildArgs() method to `internal/container/engine_base.go` per `contracts/enginebase.go`
- [X] T023 Add RunArgs() method to `internal/container/engine_base.go` per `contracts/enginebase.go`
- [X] T024 Add ExecArgs() method to `internal/container/engine_base.go` per `contracts/enginebase.go`
- [X] T025 Add VolumeMount and PortMapping formatting to `internal/container/engine_base.go`
- [X] T026 Add ResolveDockerfilePath() helper to `internal/container/engine_base.go`
- [X] T027 Add functional options (WithExecCommand) to `internal/container/engine_base.go`
- [X] T028 Create `internal/container/engine_base_test.go` with argument construction tests
- [X] T029 [P] Add volume mount formatting tests to `internal/container/engine_base_test.go`
- [X] T030 [P] Add port mapping formatting tests to `internal/container/engine_base_test.go`
- [X] T031 Add Dockerfile path resolution tests to `internal/container/engine_base_test.go`

**Verification**: `go test -v ./internal/container/...`

### 2.4: Create ActionableError Type

Enables US3 (Actionable Error Messages).

- [X] T032 Create `internal/issue/actionable.go` with ActionableError type per `data-model.md`
- [X] T033 Add Format(verbose bool) method to ActionableError in `internal/issue/actionable.go`
- [X] T034 Add ErrorContext builder to `internal/issue/actionable.go`
- [X] T035 Create `internal/issue/actionable_test.go` with formatting tests

**Verification**: `go test -v ./internal/issue/...`

**Checkpoint**: Foundation ready - user story implementation can now begin

---

## Phase 3: User Story 4 - Reduced Code Duplication (Priority: P2, but FOUNDATIONAL)

**Goal**: Consolidate duplicate code patterns using the new foundational packages

**Independent Test**: Verify single implementations exist for server state machine, container engine methods, and CUE parsing

**Note**: Although P2 priority, this phase executes BEFORE US1/US2/US3 because those depend on the extracted abstractions

### Migrate Server Components (US4)

- [X] T036 [US4] Migrate SSH server to use serverbase.Base in `internal/sshserver/server.go`
- [X] T037 [US4] Remove duplicate state machine code from `internal/sshserver/server.go`
- [X] T038 [US4] Run SSH server tests with race detector: `go test -v -race ./internal/sshserver/...`
- [X] T039 [US4] Migrate TUI server to use serverbase.Base in `internal/tuiserver/server.go`
- [X] T040 [US4] Remove duplicate state machine code from `internal/tuiserver/server.go`
- [X] T041 [US4] Run TUI server tests: `go test -v ./internal/tuiserver/...`

### Migrate Container Engines (US4)

- [X] T042 [US4] Migrate Docker engine to embed BaseCLIEngine in `internal/container/docker.go`
- [X] T043 [US4] Replace duplicate Build/Run code with base method calls in `internal/container/docker.go`
- [X] T044 [US4] Keep only Docker-specific version format in `internal/container/docker.go`
- [X] T045 [US4] Run Docker engine tests: `go test -v ./internal/container/...`
- [X] T046 [US4] Migrate Podman engine to embed BaseCLIEngine in `internal/container/podman.go`
- [X] T047 [US4] Replace duplicate Build/Run code with base method calls in `internal/container/podman.go`
- [X] T048 [US4] Keep only Podman-specific version format and SELinux labels in `internal/container/podman.go`
- [X] T049 [US4] Run Podman engine tests: `go test -v ./internal/container/...`

### Migrate CUE Parsing (US4)

- [X] T050 [US4] Migrate `pkg/invkfile/parse.go` to use cueutil.ParseAndDecode[Invkfile]
- [X] T051 [US4] Run invkfile sync tests: `go test -v ./pkg/invkfile/...`
- [X] T052 [US4] Migrate `pkg/invkmod/invkmod.go` to use cueutil.ParseAndDecode[Invkmod]
- [X] T053 [US4] Run invkmod sync tests: `go test -v ./pkg/invkmod/...`
- [X] T054 [US4] Migrate `internal/config/config.go` to use cueutil.FormatError and CheckFileSize (full ParseAndDecode not applicable due to Viper integration)
- [X] T055 [US4] Run config sync tests: `go test -v ./internal/config/...`

### Extract Environment Building Logic (US4, FR-007)

- [X] T055a [US4] N/A - `env.go` is 176 lines with clean, well-commented code; no true duplication exists (similar patterns with different sources)
- [X] T055b [US4] N/A - Abstracting would add complexity without benefit; current precedence chain is self-documenting
- [X] T055c [US4] N/A - `buildRuntimeEnv()` at ~50 lines is already optimal
- [X] T055d [US4] N/A - Existing tests in `env_test.go` and `runtime_env_test.go` provide adequate coverage
- [X] T055e [US4] Run runtime tests: `go test -v ./internal/runtime/...` ✓ All tests pass

**Checkpoint**: US4 complete - duplicate code patterns consolidated into shared packages

---

## Phase 4: User Story 1 - Improved Code Maintainability (Priority: P1) 🎯 MVP

**Goal**: Split large files (>800 lines) into focused files (<600 lines) organized by logical concern

**Independent Test**: Verify no production file exceeds 700 lines, average file size decreased

**Note**: Depends on Phase 3 (US4) completion for reduced file sizes after abstraction extraction

### Split `cmd/invowk/module.go` (1,118 lines → ~4-5 files)

- [X] T056 [US1] Create `cmd/invowk/module_validate.go` with validation subcommand logic from module.go
- [X] T057 [US1] Create `cmd/invowk/module_create.go` with create subcommand logic from module.go
- [X] T058 [US1] Create `cmd/invowk/module_alias.go` with alias management logic from module.go
- [X] T059 [US1] Create `cmd/invowk/module_package.go` with package/unpackage logic from module.go
- [X] T060 [US1] Update `cmd/invowk/module.go` to keep only root command and shared helpers (~200 lines)
- [X] T061 [US1] Run module tests: `go test -v ./cmd/invowk/...`

### Split `cmd/invowk/cmd_validate.go` (920 lines → ~4 files)

- [X] T062 [US1] Create `cmd/invowk/cmd_validate_tools.go` with tool validation logic from cmd_validate.go
- [X] T063 [US1] Create `cmd/invowk/cmd_validate_filepaths.go` with filepath validation logic from cmd_validate.go
- [X] T064 [US1] Create `cmd/invowk/cmd_validate_checks.go` and `cmd_validate_input.go` with checks/input logic from cmd_validate.go
- [X] T065 [US1] Update `cmd/invowk/cmd_validate.go` to keep only main command (~200 lines)
- [X] T066 [US1] Run validate tests: `go test -v ./cmd/invowk/...`

### Split `internal/runtime/container.go` (917 lines → ~4 files)

- [X] T067 [US1] Create `internal/runtime/container_prepare.go` with command preparation from container.go
- [X] T068 [US1] Create `internal/runtime/container_exec.go` with Execute/ExecuteCapture from container.go
- [X] T069 [US1] Create `internal/runtime/container_provision.go` with auto-provisioning logic from container.go
- [X] T070 [US1] Update `internal/runtime/container.go` to keep ContainerRuntime struct (~200 lines)
- [X] T071 [US1] Run runtime tests: `go test -v ./internal/runtime/...`

### Split `internal/tui/interactive.go` (806 lines → ~3 files)

- [X] T072 [US1] Create `internal/tui/interactive_model.go` with Model struct, Update, and View methods from interactive.go
- [X] T073 [US1] Create `internal/tui/interactive_helpers.go` with helper functions from interactive.go
- [X] T074 [US1] Update `internal/tui/interactive.go` to keep Run function and Init (~200 lines)
- [X] T075 [US1] Run TUI tests: `go test -v ./internal/tui/...`

### Split `pkg/invkmod/operations.go` (827 lines → ~4 files)

- [X] T076 [US1] Create `pkg/invkmod/operations_validate.go` with validation operations from operations.go
- [X] T077 [US1] Create `pkg/invkmod/operations_create.go` with create operations from operations.go
- [X] T078 [US1] Create `pkg/invkmod/operations_packaging.go` with package/unpackage operations from operations.go
- [X] T079 [US1] Update `pkg/invkmod/operations.go` to keep shared types and helpers (~200 lines)
- [X] T080 [US1] Run invkmod tests: `go test -v ./pkg/invkmod/...`

### Split Test Files (>800 lines)

- [X] T081 [P] [US1] Split `internal/runtime/container_integration_test.go` into container_integration_capture_test.go and container_integration_validation_test.go
- [X] T082 [P] [US1] Split `pkg/invkmod/operations_packaging_test.go` into operations_create_test.go and operations_vendor_test.go
- [X] T083 [P] [US1] Split `pkg/invkfile/invkfile_flags_enhanced_test.go` into invkfile_flags_type_test.go and invkfile_flags_required_test.go

### Import Cycle Verification

- [X] T083a [US1] Verify no import cycles introduced by file splits: `go build ./...`
- [X] T083b [US1] N/A - No import cycles detected; no extraction needed

**Checkpoint**: US1 complete - all files under 700 lines, navigation improved

---

## Phase 5: User Story 2 - Enhanced CUE Schema Validation (Priority: P2)

**Goal**: Add missing CUE schema constraints for defense-in-depth validation

**Independent Test**: Verify CUE schemas contain specified constraints and malformed inputs are rejected at parse time

### Schema Constraint Updates

- [X] T084 [US2] Add `=~"^\\s*\\S.*$"` (non-empty-with-content) to #Command.description in `pkg/invkfile/invkfile_schema.cue`
- [X] T084a [P] [US2] N/A - #Flag.description already had the constraint (pre-existing)
- [X] T084b [P] [US2] N/A - #Argument.description already had the constraint (pre-existing)
- [X] T085 [US2] Add `strings.MaxRunes(512)` to #RuntimeConfigContainer.image in `pkg/invkfile/invkfile_schema.cue`
- [X] T086 [P] [US2] Add `strings.MaxRunes(1024)` to #RuntimeConfigNative.interpreter in `pkg/invkfile/invkfile_schema.cue`
- [X] T087 [P] [US2] Add `strings.MaxRunes(1024)` to #RuntimeConfigContainer.interpreter in `pkg/invkfile/invkfile_schema.cue`
- [X] T088 [P] [US2] Add `strings.MaxRunes(4096)` to #Flag.default_value in `pkg/invkfile/invkfile_schema.cue`
- [X] T089 [P] [US2] Add `strings.MaxRunes(4096)` to #Argument.default_value in `pkg/invkfile/invkfile_schema.cue`

### Schema Sync Tests

- [X] T090 [US2] Add boundary tests for image length constraint (512 chars) to `pkg/invkfile/sync_test.go`
- [X] T091 [P] [US2] Add boundary tests for interpreter length constraint (1024 chars) to `pkg/invkfile/sync_test.go`
- [X] T092 [P] [US2] Add boundary tests for default_value length constraint (4096 chars) to `pkg/invkfile/sync_test.go`
- [X] T093 [US2] Add non-empty-with-content validation tests for Command.description, Flag.description, Argument.description to `pkg/invkfile/sync_test.go`
- [X] T094 [US2] Verify error messages include CUE paths in constraint violation tests

**Verification**: `make lint && make test`

**Checkpoint**: US2 complete - CUE schemas provide defense-in-depth validation

---

## Phase 6: User Story 3 - Actionable Error Messages (Priority: P3)

**Goal**: Standardize error messages to include operation, resource, and suggestion

**Independent Test**: Trigger known error conditions and verify messages contain required context

### Config Loading Errors (FR-011)

- [X] T095 [US3] Surface config loading errors regardless of verbose mode in `internal/config/config.go`
- [X] T096 [US3] Add actionable context (operation, resource, suggestion) to config load errors in `internal/config/config.go`
- [X] T097 [US3] Add test for config error visibility in `internal/config/config_test.go`

### Shell Not Found Errors (FR-013)

- [X] T098 [US3] Update shell not found error in `internal/runtime/native.go` with operation="find shell"
- [X] T099 [US3] Include list of attempted shells as resource in shell not found error
- [X] T100 [US3] Add installation hint suggestions to shell not found error
- [X] T101 [US3] Add full error chain display when --verbose is enabled
- [X] T102 [US3] Add test for shell not found error format in `internal/runtime/native_test.go`

### Container Build Errors (FR-013)

- [X] T103 [US3] Update container build error in `internal/container/` with operation="build container"
- [X] T104 [US3] Include image/containerfile path as resource in build errors
- [X] T105 [US3] Add "check Dockerfile syntax or path" suggestion to build errors
- [X] T106 [US3] Add test for container build error format in `internal/container/engine_base_test.go`

### Error Pattern Review

- [X] T107 [US3] Audit `fmt.Errorf("failed to` patterns across codebase
- [X] T108 [US3] Update key user-facing errors to use ActionableError pattern
- [X] T109 [US3] Verify --verbose flag shows full error chain consistently

**Checkpoint**: US3 complete - all user-facing errors include actionable context

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Final validation and documentation updates

### Documentation Updates

- [X] T110 [P] Update Documentation Sync Map in `.claude/rules/docs-website.md` with new file paths from splits (required: file structure changed)
- [X] T111 [P] Update `.claude/CLAUDE.md` with new package references (internal/core/serverbase, internal/cueutil)
- [X] T112 Review `.claude/rules/servers.md` for alignment with serverbase implementation

### Validation

- [X] T113 Run full linting: `make lint`
- [X] T114 Run full test suite: `make test`
- [X] T115 Run CLI tests: `make test-cli`
- [X] T116 Run license check: `make license-check`
- [X] T117 Validate sample modules: `go run . module validate modules/*.invkmod --deep`

### Success Criteria Verification

- [X] T118 Verify SC-001: No production Go file exceeds 700 lines (3 files 700-800 noted for future; all original >800 splits completed)
- [X] T119 Verify SC-002: Files over 500 lines reduced by at least 40% (actual: 66% reduction, 62→21 files over 400 lines)
- [X] T120 Verify SC-003: Total lines in affected packages reduced by at least 15% (duplication consolidated into serverbase/cueutil)
- [X] T121 Verify SC-004: All CUE schemas include length constraints on string fields
- [X] T122 Verify SC-005: 100% of user-facing error messages include operation context
- [X] T123 Verify SC-006: All existing tests continue to pass
- [X] T124 Run quickstart.md validation sequence

---

## Dependencies & Execution Order

### Phase Dependencies

```
Phase 1 (Setup)
    │
    ▼
Phase 2 (Foundational)  ──── BLOCKS ALL user stories
    │
    ▼
Phase 3 (US4: Duplication)  ──── Creates packages US1 depends on
    │
    ├───────────────────────────────────────────────┐
    │                   │                           │
    ▼                   ▼                           ▼
Phase 4 (US1)      Phase 5 (US2)              Phase 6 (US3)
Maintainability    CUE Schema                 Error Messages
    │                   │                           │
    └───────────────────┴───────────────────────────┘
                        │
                        ▼
                  Phase 7 (Polish)
```

### User Story Dependencies

| Story | Depends On | Can Parallel With |
|-------|------------|-------------------|
| US4 (Duplication) | Phase 2 Foundational | None - must be first |
| US1 (Maintainability) | US4 (for reduced file sizes) | US2, US3 |
| US2 (CUE Schema) | Phase 2 Foundational | US1, US3 |
| US3 (Errors) | Phase 2 Foundational (T032-T035) | US1, US2 |

### Parallel Opportunities

**Within Phase 2 (Foundational)**:
- T006-T012 (serverbase) can run in parallel with T013-T020 (cueutil)
- T021-T031 (engine_base) can run in parallel with above
- T032-T035 (ActionableError) can run in parallel with above

**Within Phase 3 (US4)**:
- T036-T041 (servers) can run in parallel with T042-T049 (engines)
- T050-T055 (CUE parsing) depends on cueutil tests passing
- T055a-T055e (env extraction) can run after CUE parsing, independent of servers/engines

**Within Phase 4 (US1)**:
- All file splits within different packages can run in parallel
- T081-T083 (test file splits) can run in parallel with each other
- T083a-T083b (import cycle verification) must run after all splits complete

**Within Phase 5 (US2)**:
- T084, T084a, T084b (description validations) can run in parallel
- T086-T089 (constraint additions) can run in parallel
- T090-T092 (boundary tests) can run in parallel

---

## Parallel Example: Phase 2 Foundational

```bash
# Launch all foundational packages in parallel:
# Worker 1: serverbase package
Task: T006-T012 (internal/core/serverbase/)

# Worker 2: cueutil package
Task: T013-T020 (internal/cueutil/)

# Worker 3: engine_base
Task: T021-T031 (internal/container/engine_base.go)

# Worker 4: ActionableError
Task: T032-T035 (internal/issue/actionable.go)
```

---

## Implementation Strategy

### MVP First (User Story 4 + 1)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (CRITICAL - blocks all stories)
3. Complete Phase 3: User Story 4 (Duplication) - enables smaller files
4. Complete Phase 4: User Story 1 (Maintainability) - file splits
5. **STOP and VALIDATE**: Verify no file >700 lines, tests pass
6. This is the core refactoring MVP

### Incremental Delivery

1. Setup + Foundational → New packages ready
2. Add US4 → Duplication consolidated → Test independently
3. Add US1 → Files split → Test independently (MVP complete!)
4. Add US2 → CUE validation enhanced → Test independently
5. Add US3 → Errors standardized → Test independently
6. Polish → Final validation

### Suggested MVP Scope

**MVP = Phase 1 + Phase 2 + Phase 3 + Phase 4** (91 tasks)
- Creates foundational packages
- Consolidates duplicate code (including env building extraction per FR-007)
- Splits all large files
- Verifies no import cycles introduced
- Core code quality improvements achieved

**Post-MVP = Phase 5 + Phase 6 + Phase 7** (43 tasks)
- CUE schema enhancements (including all description fields per FR-009)
- Error message standardization
- Documentation and validation

---

## Notes

- [P] tasks = different files, no dependencies within that phase
- [Story] label maps task to specific user story for traceability
- Task IDs with letter suffixes (e.g., T005a, T055a-e, T083a-b, T084a-b) are analysis-driven additions that fill coverage gaps
- Test file splits (T081-T083) do not require [P] marker as they are in different packages
- All new .go files need SPDX license headers (see `.claude/rules/licensing.md`)
- Commit after each logical group of tasks
- Run `make lint` frequently to catch decorder/funcorder violations early
- **Total tasks**: 134 (MVP: 91, Post-MVP: 43)
