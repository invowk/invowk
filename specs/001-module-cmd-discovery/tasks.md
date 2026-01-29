# Tasks: Module-Aware Command Discovery

**Input**: Design documents from `/specs/001-module-cmd-discovery/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md

**Tests**: CLI integration tests (testscript) and unit tests included per Constitution Principle II.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1, US2, US3, US4)
- Include exact file paths in descriptions

## Path Conventions

Based on plan.md structure:
- CLI commands: `cmd/invowk/`
- Discovery logic: `internal/discovery/`
- Module operations: `pkg/invkmod/`
- CLI tests: `tests/cli/testdata/`

---

## Phase 1: Setup

**Purpose**: Foundation types and reserved name validation

- [ ] T001 Add reserved name check for `invkfile.invkmod` in pkg/invkmod/operations.go Validate() function
- [ ] T002 [P] Add reserved name skip with warning in internal/discovery/discovery.go discoverModulesInDir() function
- [ ] T003 [P] Extend CommandInfo struct with SimpleName, SourceID, ModuleID, IsAmbiguous fields in internal/discovery/discovery.go

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core types and discovery infrastructure that ALL user stories depend on

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [ ] T004 Create DiscoveredCommandSet type with Commands, BySimpleName, AmbiguousNames, BySource, SourceOrder fields in internal/discovery/discovery.go
- [ ] T005 Implement NewDiscoveredCommandSet() constructor in internal/discovery/discovery.go
- [ ] T006 Implement DiscoveredCommandSet.Add(cmd *CommandInfo) method in internal/discovery/discovery.go
- [ ] T007 Implement DiscoveredCommandSet.Analyze() method for conflict detection in internal/discovery/discovery.go
- [ ] T008 Create SourceFilter type with ParseSourceFilter() function in cmd/invowk/cmd.go
- [ ] T009 Add normalizeSourceName() helper to handle foo/foo.invkmod/invkfile/invkfile.cue variants in cmd/invowk/cmd.go

**Checkpoint**: Foundation ready - user story implementation can now begin

---

## Phase 3: User Story 1 - Discover Commands from Multiple Sources (Priority: P1) 🎯 MVP

**Goal**: `invowk cmd` lists all commands from root invkfile AND sibling modules (excluding dependencies)

**Independent Test**: Create directory with invkfile.cue + two modules, run `invowk cmd`, verify all commands appear grouped by source

### Tests for User Story 1

- [ ] T010 [P] [US1] Create CLI test for multi-source discovery in tests/cli/testdata/multi_source.txtar
- [ ] T011 [P] [US1] Create unit test for DiscoveredCommandSet aggregation in internal/discovery/discovery_test.go

### Implementation for User Story 1

- [ ] T012 [US1] Modify DiscoverCommands() to use DiscoveredCommandSet for aggregation in internal/discovery/discovery.go
- [ ] T013 [US1] Populate SimpleName and SourceID fields during command discovery in internal/discovery/discovery.go
- [ ] T014 [US1] Ensure module commands use short names (not full module ID prefix) in internal/discovery/discovery.go
- [ ] T015 [US1] Exclude transitive dependencies (only first-level .invkmod dirs) in internal/discovery/discovery.go
- [ ] T016 [US1] Update listCommands() to group by SourceID with section headers in cmd/invowk/cmd.go
- [ ] T017 [US1] Implement verbose mode output for discovery sources (FR-013) in cmd/invowk/cmd.go

**Checkpoint**: Multi-source discovery works - commands from invkfile + modules all appear in listing

---

## Phase 4: User Story 2 - Transparent Namespace for Unambiguous Commands (Priority: P1)

**Goal**: Unambiguous commands execute with simple names; no namespace required

**Independent Test**: Create setup with unique command names across sources, verify `invowk cmd <name>` works without disambiguation

### Tests for User Story 2

- [ ] T018 [P] [US2] Add CLI tests for unambiguous command execution in tests/cli/testdata/multi_source.txtar
- [ ] T019 [P] [US2] Create unit test for ambiguity detection (no conflicts case) in internal/discovery/discovery_test.go

### Implementation for User Story 2

- [ ] T020 [US2] Modify registerDiscoveredCommands() to use SimpleName for Cobra registration in cmd/invowk/cmd.go
- [ ] T021 [US2] Ensure backward compatibility when only invkfile.cue exists (FR-011) in cmd/invowk/cmd.go
- [ ] T022 [US2] Remove ambiguity annotation from listing when command is unique (FR-005) in cmd/invowk/cmd.go

**Checkpoint**: Unambiguous commands work exactly like before - simple names, no extra syntax

---

## Phase 5: User Story 3 - Canonical Namespace for Ambiguous Commands (Priority: P2)

**Goal**: Ambiguous commands show source annotations and require disambiguation via `@source` or `--from`

**Independent Test**: Create setup with duplicate command names, verify listing shows annotations and execution requires disambiguation

### Tests for User Story 3

- [x] T023 [P] [US3] Create CLI test for ambiguous command listing in tests/cli/testdata/ambiguity.txtar
- [x] T024 [P] [US3] Create CLI test for @source prefix disambiguation in tests/cli/testdata/disambiguation.txtar
- [x] T025 [P] [US3] Create CLI test for --from flag disambiguation in tests/cli/testdata/disambiguation.txtar
- [x] T026 [P] [US3] Create unit test for ambiguity detection (conflicts case) in internal/discovery/discovery_test.go

### Implementation for User Story 3

- [x] T027 [US3] Add --from persistent flag to cmdCmd in cmd/invowk/cmd.go
- [x] T028 [US3] Implement @source prefix detection in args before Cobra matching in cmd/invowk/cmd.go
- [x] T029 [US3] Add source validation (check source exists) with helpful error messages in cmd/invowk/cmd.go
- [x] T030 [US3] Show source annotation (@foo, @invkfile) in listing for ambiguous commands only (FR-006) in cmd/invowk/cmd.go
- [x] T031 [US3] Implement ambiguous command rejection with disambiguation suggestions (FR-008) in cmd/invowk/cmd.go
- [x] T032 [US3] Allow explicit source for non-ambiguous commands without warning (FR-009c) in cmd/invowk/cmd.go
- [x] T033 [US3] Accept both short names (foo) and full names (foo.invkmod) for source (FR-009b) in cmd/invowk/cmd.go

**Checkpoint**: Ambiguous commands are detected, displayed with annotations, and can be disambiguated

---

## Phase 6: User Story 4 - Subcommand Ambiguity Handling (Priority: P2)

**Goal**: Hierarchical commands (with subcommands) handle ambiguity at the appropriate level

**Independent Test**: Create commands with shared subcommand paths, verify disambiguation works at correct hierarchy level

### Tests for User Story 4

- [x] T034 [P] [US4] Create CLI test for subcommand disambiguation in tests/cli/testdata/disambiguation.txtar
- [x] T035 [P] [US4] Create unit test for hierarchical command ambiguity in internal/discovery/discovery_test.go

### Implementation for User Story 4

- [x] T036 [US4] Ensure SimpleName includes full command path (e.g., "deploy staging") for subcommands in internal/discovery/discovery.go
- [x] T037 [US4] Verify ambiguity detection works at subcommand level in internal/discovery/discovery.go
- [x] T038 [US4] Test disambiguation with subcommands (e.g., @foo deploy staging) in cmd/invowk/cmd.go

**Checkpoint**: Hierarchical commands with subcommands handle ambiguity correctly ✅

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Documentation, edge cases, and quality

- [X] T039 [P] Update README.md with multi-source discovery documentation
- [X] T040 [P] Update cmd/invowk/cmd.go help text for `invowk cmd`
- [X] T041 [P] Add edge case tests: invalid module, zero sources, typos in source name in tests/cli/testdata/
- [X] T042 Run `make lint` and fix any issues
- [X] T043 Run `make test` and verify all unit tests pass
- [X] T044 Run `make test-cli` and verify all CLI tests pass
- [X] T045 Run quickstart.md validation manually
- [X] T046 Add SPDX headers to any new Go files (if created)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion - BLOCKS all user stories
- **User Stories 1-4 (Phases 3-6)**: All depend on Foundational phase completion
  - US1 and US2 can proceed in parallel (both P1 priority)
  - US3 and US4 can proceed in parallel after US1/US2 patterns established (both P2 priority)
- **Polish (Phase 7)**: Depends on all user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Foundational - No dependencies on other stories
- **User Story 2 (P1)**: Can start after Foundational - Slight dependency on US1 for registration approach
- **User Story 3 (P2)**: Can start after Foundational - Builds on US1/US2 listing and execution patterns
- **User Story 4 (P2)**: Can start after Foundational - Extends US3 disambiguation to hierarchical commands

### Within Each User Story

- Tests should be written first (fail before implementation)
- Discovery changes before CLI changes
- Core implementation before edge case handling
- Verify story independently before moving to next

### Parallel Opportunities

**Within Phase 1 (Setup):**
```
T002 (reserved name in discovery) || T003 (CommandInfo fields)
```

**Within Phase 2 (Foundational):**
```
After T004-T007: T008 (SourceFilter) || T009 (normalizeSourceName)
```

**Within Phase 3 (US1):**
```
T010 (CLI test) || T011 (unit test)
```

**Within Phase 5 (US3):**
```
T023 (ambiguity test) || T024 (@source test) || T025 (--from test) || T026 (unit test)
```

**User Story Parallelism:**
```
After Foundational:
  US1 || US2 (both P1)
After US1+US2 patterns established:
  US3 || US4 (both P2)
```

---

## Implementation Strategy

### MVP First (User Story 1 + User Story 2)

1. Complete Phase 1: Setup (T001-T003)
2. Complete Phase 2: Foundational (T004-T009)
3. Complete Phase 3: User Story 1 (T010-T017)
4. Complete Phase 4: User Story 2 (T018-T022)
5. **STOP and VALIDATE**: Test multi-source discovery and unambiguous execution
6. Deploy/demo if ready

### Full Feature

1. MVP (above)
2. Complete Phase 5: User Story 3 (T023-T033) - Disambiguation
3. Complete Phase 6: User Story 4 (T034-T038) - Subcommand handling
4. Complete Phase 7: Polish (T039-T046)

### Quality Gates Before Merge

- [X] `make lint` passes
- [X] `make test` passes
- [X] `make test-cli` passes
- [X] `make license-check` passes (if new files)
- [X] README.md updated
- [X] Help text updated

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story should be independently completable and testable
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
- Follow existing patterns in `internal/discovery/` and `cmd/invowk/`
