# Tasks: Go Package Structure & Organization Audit

**Input**: Design documents from `/specs/007-pkg-structure-audit/`
**Prerequisites**: plan.md, spec.md, research.md, migration-guide.md, package-map.md

**Tests**: No test tasks included - this is a refactoring audit with existing tests.

**Organization**: Tasks are grouped by concern to enable atomic migrations. Each phase is designed to fit in a single context window and leverage subagents for parallel file operations.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task addresses (US1=Agentic Navigation, US2=Package Responsibilities, US3=Feature Placement, US4=Test Coverage)
- Exact file paths included in descriptions

## Context & Subagent Strategy

Per CLAUDE.md agentic policy:
- **Use subagents (model=opus)** for multi-file reads/edits, launching 1 subagent per file
- **Main agent orchestrates** decisions and validates results
- Each phase is sized to fit in a single context window

---

## Phase 1: Documentation - Package doc.go Files

**Purpose**: Add doc.go to 5 packages missing documentation. Zero risk of breaking anything.

**Subagent Strategy**: Launch 5 parallel subagents, one per doc.go file creation.

**Independent Test**: `make lint && make test && make license-check`

- [x] T001 [P] [US2] Create doc.go in internal/core/serverbase/ with serverbase package documentation
- [x] T002 [P] [US2] Create doc.go in internal/issue/ with issue package documentation
- [x] T003 [P] [US2] Create doc.go in internal/tui/ with tui package documentation
- [x] T004 [P] [US2] Create doc.go in internal/tuiserver/ with tuiserver package documentation
- [x] T005 [P] [US2] Create doc.go in pkg/invowkfile/ with invowkfile package documentation

**Content Reference** (from research.md):
- serverbase: Server state machine and lifecycle infrastructure
- issue: Actionable error handling with user-friendly messages
- tui: Terminal UI components (Bubble Tea, huh, lipgloss)
- tuiserver: HTTP server for child process TUI requests
- invowkfile: Invowkfile types/parsing with internal/cueutil note

**Checkpoint**: All packages have doc.go or inline package comment. Verify: `grep -l "^// Package" internal/*/*.go pkg/*/*.go | wc -l`

---

## Phase 2: Style Consolidation - Eliminate Hex Duplication

**Purpose**: Create cmd/invowk/styles.go with shared color palette and base styles. Update 5 files to use shared styles.

**Subagent Strategy**:
1. Main agent creates styles.go (single file, orchestration work)
2. Launch 5 parallel subagents to update each consumer file

**Independent Test**: `make lint && make test`

### Create Shared Styles

- [x] T006 [US1] Create cmd/invowk/styles.go with color palette (ColorPrimary, ColorMuted, ColorSuccess, ColorError, ColorWarning, ColorHighlight, ColorVerbose) and base styles (TitleStyle, SubtitleStyle, SuccessStyle, ErrorStyle, WarningStyle, CmdStyle, VerboseStyle, VerboseHighlightStyle)

### Update Consumer Files

- [x] T007 [P] [US1] Update cmd/invowk/root.go to use shared styles from styles.go
- [x] T008 [P] [US1] Update cmd/invowk/module.go to use shared styles, derive module-specific variants (moduleTitleStyle, moduleIssueStyle, moduleDetailStyle)
- [x] T009 [P] [US1] Update cmd/invowk/config.go to use shared styles from styles.go
- [x] T010 [P] [US1] Update cmd/invowk/cmd_discovery.go to use shared styles from styles.go
- [x] T011 [P] [US1] Update cmd/invowk/cmd_render.go to use shared styles from styles.go

> **Additional files updated**: completion.go, cmd_execute.go, init.go, module_alias.go, module_create.go, module_deps.go, module_package.go, module_validate.go - these files also referenced the previous lowercase style names and were updated to use the exported PascalCase style names.

**Checkpoint**: No magic hex strings remain in cmd/invowk/ except in styles.go. Verify: `grep -rn "#[0-9A-Fa-f]\{6\}" cmd/invowk/*.go | grep -v styles.go`

---

## Phase 3: Code Duplication - Discovery & Container

**Purpose**: Eliminate 2 code duplication patterns identified in research.md.

**Subagent Strategy**: Launch 2 parallel subagents (different packages, no dependencies)

**Independent Test**: `make lint && make test`

- [x] T012 [P] [US1] Refactor internal/discovery/discovery.go: Make DiscoverCommands() call DiscoverCommandSet() and extract/sort results. Eliminates ~50 lines of duplicated logic.
- [x] T013 [P] [US1] Refactor internal/runtime/container_exec.go: Extract prepareContainerExecution() helper method (containerExecPrep struct) used by both Execute() and ExecuteCapture(). Eliminates ~130 lines of duplicated logic.

**Checkpoint**: No duplicate command discovery or container prep logic. Tests pass.

---

## Phase 4A: File Split - pkg/invowkfile/validation.go (753 lines)

**Purpose**: Split largest file into focused validation files.

**Subagent Strategy**: Single subagent - requires analyzing entire file to determine split points.

**Independent Test**: `make lint && go test -v ./pkg/invowkfile/...`

### Analysis Step (main agent)

- [x] T014 [US1] Analyze pkg/invowkfile/validation.go: Split into validation_primitives.go (constants, regex), validation_container.go (container validation), validation_filesystem.go (path/file validation). Original validation.go DELETED.

### Execution Steps

- [x] T015 [P] [US1] Create pkg/invowkfile/validation_primitives.go: Constants and regex validation (263 lines)
- [x] T016 [P] [US1] Create pkg/invowkfile/validation_container.go: Container image/volume/port validation (290 lines)
- [x] T017 [US1] Create pkg/invowkfile/validation_filesystem.go: Path and filename validation (218 lines). Original validation.go DELETED (code fully distributed).

**Checkpoint**: All files under 600 lines. `wc -l pkg/invowkfile/validation*.go`

---

## Phase 4B: File Split - pkg/invowkfile/invowkfile_validation.go (631 lines)

**Purpose**: Address second validation file - merge or split as appropriate.

**Subagent Strategy**: Single subagent - requires understanding relationship to validation.go.

**Independent Test**: `make lint && go test -v ./pkg/invowkfile/...`

- [x] T018 [US1] Analyze pkg/invowkfile/invowkfile_validation.go: Split into invowkfile_validation_struct.go (struct validation methods) and invowkfile_validation_deps.go (dependency validation). Original DELETED.
- [x] T019 [US1] Execute: Created invowkfile_validation_struct.go (466 lines) and invowkfile_validation_deps.go (173 lines). Original invowkfile_validation.go DELETED.

**Checkpoint**: All validation-related files under 600 lines.

---

## Phase 4C: File Split - pkg/invowkmod/resolver.go (726 lines)

**Purpose**: Split resolver into phase-focused files.

**Subagent Strategy**: Single subagent for analysis, parallel subagents for file creation.

**Independent Test**: `make lint && go test -v ./pkg/invowkmod/...`

### Analysis Step

- [x] T020 [US1] Analyze pkg/invowkmod/resolver.go: Split into resolver_deps.go (dependency resolution) and resolver_cache.go (cache management).

### Execution Steps

- [x] T021 [P] [US1] Create pkg/invowkmod/resolver_deps.go: Dependency resolution logic (248 lines)
- [x] T022 [P] [US1] Create pkg/invowkmod/resolver_cache.go: Cache management logic (162 lines)
- [x] T023 [US1] Refactor pkg/invowkmod/resolver.go: Resolution orchestration (337 lines).

**Checkpoint**: All resolver files under 600 lines. `wc -l pkg/invowkmod/resolver*.go`

---

## Phase 4D: File Split - internal/discovery/discovery.go (715 lines)

**Purpose**: Split discovery into type-focused files.

**Subagent Strategy**: Single subagent for analysis, parallel subagents for file creation.

**Independent Test**: `make lint && go test -v ./internal/discovery/...`

### Analysis Step

- [x] T024 [US1] Analyze internal/discovery/discovery.go: Split into discovery_files.go (file discovery) and discovery_commands.go (command aggregation).

### Execution Steps

- [x] T025 [P] [US1] Create internal/discovery/discovery_files.go: File-level discovery logic (220 lines)
- [x] T026 [P] [US1] Create internal/discovery/discovery_commands.go: Command aggregation logic (294 lines)
- [x] T027 [US1] Refactor internal/discovery/discovery.go: Core discovery orchestration (182 lines).

**Checkpoint**: All discovery files under 600 lines. `wc -l internal/discovery/discovery*.go`

---

## Phase 4E: File Split - cmd/invowk/cmd_execute.go (643 lines)

**Purpose**: Split execute command into main + helpers.

**Subagent Strategy**: Single subagent - straightforward split.

**Independent Test**: `make lint && make test`

- [x] T028 [US1] Analyze cmd/invowk/cmd_execute.go: Extract helpers (parseEnvVarFlags, runDisambiguatedCommand, checkAmbiguousCommand, createRuntimeRegistry, ensureSSHServer, stopSSHServer, bridgeTUIRequests).
- [x] T029 [P] [US1] Create cmd/invowk/cmd_execute_helpers.go: Helper functions (276 lines)
- [x] T030 [US1] Refactor cmd/invowk/cmd_execute.go: Command entry points (386 lines).

**Checkpoint**: Both files under 600 lines. `wc -l cmd/invowk/cmd_execute*.go`

---

## Phase 4F: File Split - internal/sshserver/server.go (627 lines)

**Purpose**: Split server into lifecycle-focused files.

**Subagent Strategy**: Single subagent for analysis, parallel subagents for file creation.

**Independent Test**: `make lint && go test -v ./internal/sshserver/...`

### Analysis Step

- [x] T031 [US1] Analyze internal/sshserver/server.go: Split into server_lifecycle.go (Start/Stop/Wait) and server_auth.go (auth/keys).

### Execution Steps

- [x] T032 [P] [US1] Create internal/sshserver/server_lifecycle.go: Start/Stop/Wait methods (258 lines)
- [x] T033 [P] [US1] Create internal/sshserver/server_auth.go: Authentication and key management (150 lines)
- [x] T034 [US1] Refactor internal/sshserver/server.go: Core server implementation (255 lines).

**Checkpoint**: All server files under 600 lines. `wc -l internal/sshserver/server*.go`

---

## Phase 5: Final Verification & Validation

**Purpose**: Comprehensive verification that all success criteria are met.

**Subagent Strategy**: Main agent orchestrates; parallel verification commands.

### Coverage Baseline

> **Note**: T049 should be run BEFORE Phase 4 file splits begin to establish baseline.

- [x] T049 [US4] Capture coverage baseline: `go test -coverprofile=coverage-before.out ./... && go tool cover -func=coverage-before.out | tail -1` → **53.0%**

### Verification Tasks (run in parallel for speed)

- [x] T035 [P] Run full lint suite: `make lint` → **0 issues**
- [x] T036 [P] Run full test suite: `make test` → **All pass**
- [x] T037 [P] Run CLI integration tests: `make test-cli` → **All pass**
- [x] T038 [P] Run license check: `make license-check` → **All files have SPDX headers**
- [x] T039 [P] Run tidy: `make tidy` → **Complete**
- [x] T040 [P] Run module validation: `go run . module validate modules/*.invowkmod --deep` → **Valid**

> **Note**: These are intentionally separate tasks to enable parallel execution via subagents. A combined `make all` would run sequentially.

### Coverage Comparison

- [x] T050 [US4] Compare coverage after refactoring: `go test -coverprofile=coverage-after.out ./... && go tool cover -func=coverage-after.out | tail -1` → **53.0%** (stable, matches baseline)

### File Size Verification

- [x] T041 Verify all source files under 600 lines: `find . -name "*.go" -not -name "*_test.go" -not -path "./vendor/*" -exec wc -l {} \; | awk '$1 > 600 {print}' | sort -rn` → **No violations**
- [x] T042 Verify all test files under 800 lines (SC-002): `find . -name "*_test.go" -not -path "./vendor/*" -exec wc -l {} \; | awk '$1 > 800 {print}' | sort -rn` → **No violations** (split `engine_base_test.go` into 3 files)

### Documentation Verification

- [x] T043 Verify all packages have documentation: List all package directories and check for doc.go or inline package comment. → **All 17 packages documented**

### Success Criteria Checklist

- [x] T044 [US1] SC-001: All source files under 600 lines (verified by T041)
- [x] T051 [US4] SC-002: All test files under 800 lines (verified by T042)
- [x] T045 [US2] SC-004: Every package has doc.go or inline package comment (verified by T043)
- [x] T046 [US1] SC-003: Zero identified code duplication patterns remain
- [x] T047 [US3] SC-006: No circular dependencies (verify with `go mod graph`) → **No cycles detected**
- [x] T048 [US4] SC-007/SC-008: All tests pass, coverage stable (verified by T049/T050 comparison)

**Checkpoint**: All success criteria verified. Ready for commit/squash-merge.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Documentation)**: No dependencies - can start immediately
- **Phase 2 (Styles)**: No dependencies - can run parallel with Phase 1
- **Phase 3 (Duplication)**: No dependencies - can run parallel with Phase 1/2
- **Phase 4A-4F (File Splits)**: Can run in parallel with each other (different packages)
- **Phase 5 (Verification)**: Depends on ALL previous phases completing

### Parallel Opportunities by Phase

| Phase | Parallel Tasks | Sequential Tasks |
|-------|----------------|------------------|
| 1 | T001-T005 (5 parallel) | None |
| 2 | T007-T011 (5 parallel) | T006 first |
| 3 | T012-T013 (2 parallel) | None |
| Pre-4 | None | T049 (coverage baseline) |
| 4A | T015-T016 (2 parallel) | T014 first, T017 last |
| 4B | None | T018-T019 sequential |
| 4C | T021-T022 (2 parallel) | T020 first, T023 last |
| 4D | T025-T026 (2 parallel) | T024 first, T027 last |
| 4E | T029 (1 parallel) | T028 first, T030 last |
| 4F | T032-T033 (2 parallel) | T031 first, T034 last |
| 5 | T035-T040 (6 parallel) | T041-T043 sequential, T050 after tests, T044-T048/T051 sequential |

### User Story Dependencies

- **US1 (Agentic Navigation)**: All file splits (Phase 4) + duplication fixes (Phase 3)
- **US2 (Package Responsibilities)**: All doc.go files (Phase 1)
- **US3 (Feature Placement)**: Depends on US1 + US2 being complete
- **US4 (Test Coverage)**: Depends on all phases; verified in Phase 5

---

## Parallel Execution Examples

### Phase 1: All doc.go files in parallel

```bash
# Launch 5 subagents concurrently:
Subagent 1: "Create doc.go in internal/core/serverbase/"
Subagent 2: "Create doc.go in internal/issue/"
Subagent 3: "Create doc.go in internal/tui/"
Subagent 4: "Create doc.go in internal/tuiserver/"
Subagent 5: "Create doc.go in pkg/invowkfile/"
```

### Phase 2: Style updates in parallel (after T006)

```bash
# After T006 (styles.go) completes, launch 5 subagents:
Subagent 1: "Update root.go to use shared styles"
Subagent 2: "Update module.go to use shared styles"
Subagent 3: "Update config.go to use shared styles"
Subagent 4: "Update cmd_discovery.go to use shared styles"
Subagent 5: "Update cmd_render.go to use shared styles"
```

### Phase 4: All file splits can run in parallel (different packages)

```bash
# Launch 6 subagents for Phase 4A-4F concurrently:
Subagent 4A: "Split pkg/invowkfile/validation.go"
Subagent 4B: "Split pkg/invowkfile/invowkfile_validation.go"
Subagent 4C: "Split pkg/invowkmod/resolver.go"
Subagent 4D: "Split internal/discovery/discovery.go"
Subagent 4E: "Split cmd/invowk/cmd_execute.go"
Subagent 4F: "Split internal/sshserver/server.go"
```

---

## Implementation Strategy

### MVP First (Phase 1 + Phase 5 Verification)

1. Complete Phase 1: Add all doc.go files
2. Run Phase 5 verification subset: `make lint && make test`
3. **STOP and VALIDATE**: All packages documented

### Incremental Delivery

1. Phase 1 (Documentation) → Commit
2. Phase 2 (Styles) → Commit
3. Phase 3 (Duplication) → Commit
4. Phase 4A-4F (File Splits) → One commit per phase (6 commits)
5. Phase 5 (Verification) → Final validation
6. Squash merge to main per project git rules

### Commit Message Pattern

Per `.claude/rules/git.md`:
```
refactor(<scope>): <summary>

- Bullet 1
- Bullet 2

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
```

---

## Notes

- [P] tasks = different files, no dependencies, can run via parallel subagents
- Each phase is sized for single context window (~10-15 tasks max)
- Analysis tasks (T014, T018, T020, T024, T028, T031) must complete before extraction tasks
- **Analysis task outputs**: Document proposed splits as inline comments in the task checkbox (e.g., `- [x] T014 ... → Split into validation_runtime.go (funcs A, B, C) and validation_deps.go (funcs D, E, F)`). No separate artifact needed.
- All file splits follow the pattern: analyze → extract parallel → refactor original
- Verification in Phase 5 catches any issues before final commit
- Per migration-guide.md: atomic per-package migrations, tests pass after each step
- **Coverage baseline timing**: T049 should be run BEFORE Phase 4 begins; T050 runs after Phase 5 verification tasks
