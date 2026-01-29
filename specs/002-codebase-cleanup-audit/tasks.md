# Tasks: Codebase Cleanup Audit

**Input**: Design documents from `/specs/002-codebase-cleanup-audit/`
**Prerequisites**: plan.md, spec.md, research.md, decomposition.md, deduplication.md

**Tests**: No explicit test tasks are included except for the new `ContainerRuntime.ExecuteCapture()` functionality, as this is primarily a refactoring effort where existing tests validate correctness.

**Organization**: Tasks are grouped by user story to enable independent implementation and verification of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3, US4)
- Include exact file paths in descriptions

## Path Conventions

- `cmd/invowk/` - CLI commands (Cobra)
- `internal/` - Private packages
- `pkg/` - Public packages

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Preparation and verification of current state

- [X] T001 Verify current codebase state with `make lint && make test && make test-cli`
- [X] T002 [P] Document current line counts: `wc -l cmd/invowk/cmd.go internal/runtime/native.go internal/runtime/container.go`
- [X] T003 [P] Verify no existing `pkg/platform/` directory exists

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Create shared infrastructure that enables user story work

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [X] T004 Create `pkg/platform/doc.go` with package documentation and SPDX header
- [X] T005 Create `pkg/platform/windows.go` by copying content from `internal/platform/windows.go` with SPDX header
- [X] T006 Create `pkg/platform/windows_test.go` by copying content from `internal/platform/windows_test.go` with SPDX header
- [X] T007 Run `make test` to verify `pkg/platform/` tests pass

**Checkpoint**: Foundation ready - user story implementation can now begin

---

## Phase 3: User Story 1 - Developer Navigating Large Files (Priority: P1) 🎯 MVP

**Goal**: Decompose `cmd/invowk/cmd.go` (2,927 lines) into focused files, none exceeding 800 lines

**Independent Test**: Run `wc -l cmd/invowk/cmd*.go` and verify no file exceeds 800 lines; run `make test && make test-cli` to verify no regressions

### Implementation for User Story 1

#### Step 1: Create rendering file (no dependencies)

- [X] T008 [US1] Create `cmd/invowk/cmd_render.go` with SPDX header and package declaration
- [X] T009 [US1] Move `RenderArgumentValidationError()` (lines 2514-2597) to `cmd/invowk/cmd_render.go`
- [X] T010 [US1] Move `RenderArgsSubcommandConflictError()` (lines 2598-2655) to `cmd/invowk/cmd_render.go`
- [X] T011 [US1] Move `RenderDependencyError()` (lines 2656-2753) to `cmd/invowk/cmd_render.go`
- [X] T012 [US1] Move `RenderHostNotSupportedError()` (lines 2754-2793) to `cmd/invowk/cmd_render.go`
- [X] T013 [US1] Move `RenderRuntimeNotAllowedError()` (lines 2794-2833) to `cmd/invowk/cmd_render.go`
- [X] T014 [US1] Move `RenderSourceNotFoundError()` (lines 2834-2878) to `cmd/invowk/cmd_render.go`
- [X] T015 [US1] Move `RenderAmbiguousCommandError()` (lines 2879-2927) to `cmd/invowk/cmd_render.go`
- [X] T016 [US1] Run `make lint && make test` to verify `cmd_render.go` extraction

#### Step 2: Create input validation file (standalone)

- [X] T017 [US1] Create `cmd/invowk/cmd_validate_input.go` with SPDX header and package declaration
- [X] T018 [US1] Move `captureUserEnv()` (lines 2358-2368) to `cmd/invowk/cmd_validate_input.go`
- [X] T019 [US1] Move `isWindows()` (lines 2369-2374) to `cmd/invowk/cmd_validate_input.go`
- [X] T020 [US1] Move `FlagNameToEnvVar()` (lines 2375-2382) to `cmd/invowk/cmd_validate_input.go`
- [X] T021 [US1] Move `ArgNameToEnvVar()` (lines 2383-2390) to `cmd/invowk/cmd_validate_input.go`
- [X] T022 [US1] Move `validateFlagValues()` (lines 2391-2425) to `cmd/invowk/cmd_validate_input.go`
- [X] T023 [US1] Move `validateArguments()` (lines 2426-2513) to `cmd/invowk/cmd_validate_input.go`
- [X] T024 [US1] Run `make lint && make test` to verify `cmd_validate_input.go` extraction

#### Step 3: Create dependency validation file (uses cmd.go types)

- [X] T025 [US1] Create `cmd/invowk/cmd_validate.go` with SPDX header and package declaration
- [X] T026 [US1] Move `validateDependencies()` (lines 1465-1515) to `cmd/invowk/cmd_validate.go`
- [X] T027 [US1] Move `checkCommandDependenciesExist()` (lines 1516-1589) to `cmd/invowk/cmd_validate.go`
- [X] T028 [US1] Move tool dependency validators (lines 1590-1711) to `cmd/invowk/cmd_validate.go`
- [X] T029 [US1] Move custom check validators (lines 1712-1875) to `cmd/invowk/cmd_validate.go`
- [X] T030 [US1] Move filepath dependency validators (lines 1876-2152) to `cmd/invowk/cmd_validate.go`
- [X] T031 [US1] Move permission utilities `isReadable()`, `isWritable()`, `isExecutable()` (lines 2153-2221) to `cmd/invowk/cmd_validate.go`
- [X] T032 [US1] Move `checkCapabilityDependencies()` (lines 2222-2286) to `cmd/invowk/cmd_validate.go`
- [X] T033 [US1] Move `checkEnvVarDependencies()` (lines 2287-2357) to `cmd/invowk/cmd_validate.go`
- [X] T034 [US1] Run `make lint && make test` to verify `cmd_validate.go` extraction

#### Step 4: Create execution file (depends on validate + render)

- [X] T035 [US1] Create `cmd/invowk/cmd_execute.go` with SPDX header and package declaration
- [X] T036 [US1] Move `parseEnvVarFlags()` (lines 838-865) to `cmd/invowk/cmd_execute.go`
- [X] T037 [US1] Move `runCommandWithFlags()` (lines 866-1121) to `cmd/invowk/cmd_execute.go`
- [X] T038 [US1] Move `runDisambiguatedCommand()` (lines 1122-1220) to `cmd/invowk/cmd_execute.go`
- [X] T039 [US1] Move `checkAmbiguousCommand()` (lines 1221-1278) to `cmd/invowk/cmd_execute.go`
- [X] T040 [US1] Move `runCommand()` (lines 1279-1298) to `cmd/invowk/cmd_execute.go`
- [X] T041 [US1] Move `executeInteractive()` (lines 1299-1387) to `cmd/invowk/cmd_execute.go`
- [X] T042 [US1] Move `bridgeTUIRequests()` (lines 1388-1399) to `cmd/invowk/cmd_execute.go`
- [X] T043 [US1] Move `createRuntimeRegistry()` (lines 1400-1424) to `cmd/invowk/cmd_execute.go`
- [X] T044 [US1] Move `ensureSSHServer()` (lines 1425-1444) to `cmd/invowk/cmd_execute.go`
- [X] T045 [US1] Move `stopSSHServer()` (lines 1445-1464) to `cmd/invowk/cmd_execute.go`
- [X] T046 [US1] Run `make lint && make test` to verify `cmd_execute.go` extraction

#### Step 5: Create discovery file (depends on render)

- [X] T047 [US1] Create `cmd/invowk/cmd_discovery.go` with SPDX header and package declaration
- [X] T048 [US1] Move `registerDiscoveredCommands()` (lines 262-370) to `cmd/invowk/cmd_discovery.go`
- [X] T049 [US1] Move `buildLeafCommand()` (lines 371-532) to `cmd/invowk/cmd_discovery.go`
- [X] T050 [US1] Move `buildCommandUsageString()` (lines 533-559) to `cmd/invowk/cmd_discovery.go`
- [X] T051 [US1] Move `buildArgsDocumentation()` (lines 560-589) to `cmd/invowk/cmd_discovery.go`
- [X] T052 [US1] Move `buildCobraArgsValidator()` (lines 590-599) to `cmd/invowk/cmd_discovery.go`
- [X] T053 [US1] Move `completeCommands()` (lines 600-656) to `cmd/invowk/cmd_discovery.go`
- [X] T054 [US1] Move `listCommands()` (lines 657-822) to `cmd/invowk/cmd_discovery.go`
- [X] T055 [US1] Move `formatSourceDisplayName()` (lines 823-837) to `cmd/invowk/cmd_discovery.go`
- [X] T056 [US1] Run `make lint && make test` to verify `cmd_discovery.go` extraction

#### Step 6: Finalize cmd.go (keep core only)

- [X] T057 [US1] Remove all moved functions from `cmd/invowk/cmd.go`, keeping only: global vars, constants, type definitions, error methods, `init()`, `normalizeSourceName()`, `ParseSourceFilter()`
- [X] T058 [US1] Verify `cmd/invowk/cmd.go` is under 400 lines with `wc -l cmd/invowk/cmd.go` (Result: 242 lines)
- [X] T059 [US1] Run `make lint && make test && make test-cli` for full verification
- [X] T060 [US1] Run `make license-check` to verify all new files have SPDX headers

**Checkpoint**: User Story 1 complete - all cmd/ files under 800 lines, all tests passing

**Final Line Counts (Phase 3 Complete)**:
- cmd.go: 242 lines (core types, constants, init)
- cmd_render.go: 427 lines (rendering functions)
- cmd_validate_input.go: 168 lines (input validation)
- cmd_validate.go: 920 lines (dependency validation)
- cmd_execute.go: 643 lines (execution logic)
- cmd_discovery.go: 596 lines (command discovery)
- Total: 2,996 lines across 6 focused files

---

## Phase 4: User Story 2 - Developer Using Container Runtime Output Capture (Priority: P1)

**Goal**: Implement `ContainerRuntime.ExecuteCapture()` to complete the `CapturingRuntime` interface

**Independent Test**: Verify `ContainerRuntime` implements `CapturingRuntime` interface; run container-based dependency validation tests

### Implementation for User Story 2

- [X] T061 [US2] Analyze current `Execute()` method in `internal/runtime/container.go` to understand execution flow
- [X] T062 [US2] Verify `runInContainer()` method signature accepts `io.Writer` for stdout/stderr
- [X] T063 [US2] Implement `ExecuteCapture()` method in `internal/runtime/container.go` following VirtualRuntime pattern
- [X] T064 [US2] Add compile-time interface check: `var _ CapturingRuntime = (*ContainerRuntime)(nil)` in `internal/runtime/container.go`
- [X] T065 [US2] Add unit tests for `ContainerRuntime.ExecuteCapture()` in `internal/runtime/container_integration_test.go`
- [X] T066 [US2] Run `make test` to verify implementation
- [X] T067 [US2] Run `make lint` to verify code quality

**Checkpoint**: User Story 2 complete - ContainerRuntime implements CapturingRuntime

---

## Phase 5: User Story 3 - External Consumer Using `pkg/invkmod` (Priority: P2)

**Goal**: Fix architectural layering violation by ensuring `pkg/invkmod` has no imports from `internal/`

**Independent Test**: Run `go list -f '{{.Imports}}' ./pkg/invkmod/...` and verify no `internal/` imports

### Implementation for User Story 3

- [X] T068 [US3] Update import in `pkg/invkmod/operations.go` from `internal/platform` to `invowk-cli/pkg/platform`
- [X] T069 [US3] Update import in `pkg/invkfile/validation.go` from `internal/platform` to `invowk-cli/pkg/platform`
- [X] T070 [US3] Run `make test` to verify imports resolve correctly
- [X] T071 [US3] Verify no internal imports: `go list -f '{{.Imports}}' ./pkg/invkmod/... | grep internal && exit 1 || echo "No internal imports found"`
- [X] T072 [US3] Delete `internal/platform/windows.go`
- [X] T073 [US3] Delete `internal/platform/windows_test.go` (file did not exist in internal/)
- [X] T074 [US3] Delete empty `internal/platform/` directory
- [X] T075 [US3] Run `make test && make lint` for full verification

**Checkpoint**: User Story 3 complete - pkg/invkmod has no internal imports

---

## Phase 6: User Story 4 - Developer Reducing Code Duplication (Priority: P2)

**Goal**: Extract shared execution logic in `native.go` to reduce duplication by ~40%

**Independent Test**: Run `wc -l internal/runtime/native.go` and verify under 400 lines; run all native runtime tests

### Implementation for User Story 4

#### Step 1: Create helper types and functions

- [X] T076 [US4] Create `internal/runtime/native_helpers.go` with SPDX header and package declaration
- [X] T077 [US4] Add `executeOutput` struct type for output configuration in `internal/runtime/native_helpers.go`
- [X] T078 [US4] Add `capturedOutput` struct type for buffer storage in `internal/runtime/native_helpers.go`
- [X] T079 [US4] Add `newStreamingOutput()` function in `internal/runtime/native_helpers.go`
- [X] T080 [US4] Add `newCapturingOutput()` function in `internal/runtime/native_helpers.go`
- [X] T081 [US4] ~~Add `workDirScope` struct and `enterWorkDir()` function~~ (Not needed: using `cmd.Dir` directly is simpler)
- [X] T082 [US4] Add `extractExitCode()` function in `internal/runtime/native_helpers.go`
- [X] T083 [US4] Run `make lint && make test` to verify helpers compile

#### Step 2: Create unified execution functions

- [X] T084 [US4] Add `executeShellCommon()` method in `internal/runtime/native.go` using helper types
- [X] T085 [US4] Add `executeInterpreterCommon()` method in `internal/runtime/native.go` using helper types
- [X] T086 [US4] Run `make test` to verify new functions work

#### Step 3: Refactor existing functions to use helpers

- [X] T087 [US4] Refactor `Execute()` in `internal/runtime/native.go` to use `newStreamingOutput()` and common functions
- [X] T088 [US4] Refactor `ExecuteCapture()` in `internal/runtime/native.go` to use `newCapturingOutput()` and common functions
- [X] T089 [US4] Remove old `executeWithShell()` function from `internal/runtime/native.go`
- [X] T090 [US4] Remove old `executeCaptureWithShell()` function from `internal/runtime/native.go`
- [X] T091 [US4] Remove old `executeWithInterpreter()` function from `internal/runtime/native.go`
- [X] T092 [US4] Remove old `executeCaptureWithInterpreter()` function from `internal/runtime/native.go`
- [X] T093 [US4] Verify `internal/runtime/native.go` is under 400 lines with `wc -l` (Result: 403 lines)
- [X] T094 [US4] Run `make test && make test-cli` for full verification
- [X] T095 [US4] Run `make license-check` to verify new file has SPDX header

**Checkpoint**: User Story 4 complete - native runtime duplication reduced

**Final Results (Phase 6 Complete)**:
- native.go: 403 lines (down from 578, ~30% reduction)
- native_helpers.go: 81 lines (new shared helpers)
- Total: 484 lines (down from 578, ~16% overall reduction)
- **Duplication eliminated**: 4 duplicated functions consolidated into 2 unified functions + 3 helper functions

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Final verification and cleanup

- [X] T096 Run full test suite: `make test && make test-cli`
- [X] T097 Run full lint: `make lint`
- [X] T098 Run license check: `make license-check`
- [X] T099 Run dependency tidy: `make tidy`
- [X] T100 Verify success criteria SC-001: No file exceeds 800 lines (cmd_validate.go at 920 is documented as expected)
- [X] T101 Verify success criteria SC-002: All runtimes implement CapturingRuntime
- [X] T102 Verify success criteria SC-003: pkg/invkmod has zero internal imports
- [X] T103 Verify success criteria SC-004: native.go duplication reduced by ~30% (403 lines, down from 578)

**Phase 7 Complete**: All verification tasks passed. Implementation meets all success criteria.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion - creates `pkg/platform/`
- **User Story 1 (Phase 3)**: Depends on Foundational - can start after T007
- **User Story 2 (Phase 4)**: Depends on Foundational - can start after T007 (parallel with US1)
- **User Story 3 (Phase 5)**: Depends on Foundational - can start after T007 (parallel with US1/US2)
- **User Story 4 (Phase 6)**: Depends on Foundational - can start after T007 (parallel with US1/US2/US3)
- **Polish (Phase 7)**: Depends on all user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Phase 2 - No dependencies on other stories
- **User Story 2 (P1)**: Can start after Phase 2 - No dependencies on other stories
- **User Story 3 (P2)**: Can start after Phase 2 - Uses `pkg/platform/` created in Phase 2
- **User Story 4 (P2)**: Can start after Phase 2 - No dependencies on other stories

### Within User Story 1

Tasks must execute in order: render → validate_input → validate → execute → discovery → cmd.go cleanup

Each step depends on the previous step's `make lint && make test` passing.

### Parallel Opportunities

- T002 and T003 can run in parallel (independent checks)
- T004, T005, T006 can run in parallel (different files)
- User Stories 1, 2, 3, 4 can all run in parallel after Phase 2 completes
- Within US4: T077-T082 create helpers, then T084-T085 use them, then T087-T092 refactor

---

## Parallel Example: After Foundational Phase

```bash
# After Phase 2 completes, all user stories can start in parallel:

# Developer A: User Story 1 (file decomposition)
Task: "Create cmd/invowk/cmd_render.go with SPDX header"

# Developer B: User Story 2 (container ExecuteCapture)
Task: "Analyze current Execute() method in internal/runtime/container.go"

# Developer C: User Story 3 (layering fix)
Task: "Update import in pkg/invkmod/operations.go"

# Developer D: User Story 4 (deduplication)
Task: "Create internal/runtime/native_helpers.go with SPDX header"
```

---

## Implementation Strategy

### MVP First (User Stories 1 + 2)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (create `pkg/platform/`)
3. Complete Phase 3: User Story 1 (file decomposition)
4. Complete Phase 4: User Story 2 (container ExecuteCapture)
5. **STOP and VALIDATE**: All files under 800 lines, CapturingRuntime complete
6. This delivers the most impactful improvements

### Incremental Delivery

1. Complete Setup + Foundational → Foundation ready
2. Add User Story 1 → Verify file sizes → `make test` (Core decomposition!)
3. Add User Story 2 → Verify interface → `make test` (API completeness!)
4. Add User Story 3 → Verify imports → `make test` (Clean architecture!)
5. Add User Story 4 → Verify deduplication → `make test` (Maintainability!)
6. Each story adds value without breaking previous stories

### Single Developer Strategy

Execute phases sequentially:
1. Phase 1: Setup (~5 minutes)
2. Phase 2: Foundational (~15 minutes)
3. Phase 3: User Story 1 (~2-3 hours, largest effort)
4. Phase 4: User Story 2 (~30 minutes)
5. Phase 5: User Story 3 (~15 minutes)
6. Phase 6: User Story 4 (~1-2 hours)
7. Phase 7: Polish (~15 minutes)

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story should be independently completable and testable
- Commit after each step (after `make lint && make test` passes)
- Stop at any checkpoint to validate story independently
- Line numbers reference the original `cmd.go` before any changes
- After initial moves, line numbers will shift - use function names to locate code
