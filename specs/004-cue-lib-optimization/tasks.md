# Tasks: CUE Library Usage Optimization

**Input**: Design documents from `/specs/004-cue-lib-optimization/`
**Prerequisites**: plan.md ✓, spec.md ✓, research.md ✓, data-model.md ✓, quickstart.md ✓

**Tests**: Schema sync tests are core to this feature (they ARE the feature for US1).

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3, US4)
- Include exact file paths in descriptions

## Path Conventions

- **Source**: `pkg/`, `internal/` at repository root
- **Documentation**: `.claude/rules/` for rules files
- **Specs**: `specs/004-cue-lib-optimization/` for feature docs

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Create shared helper code and establish constants

- [X] T001 Add `DefaultMaxCUEFileSize` constant (5MB) in pkg/invowkfile/parse.go
- [X] T002 [P] Create `extractCUEFields()` helper function in pkg/invowkfile/sync_test.go
- [X] T003 [P] Create `extractGoJSONTags()` helper function in pkg/invowkfile/sync_test.go

**Checkpoint**: Shared helpers ready for all user stories

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Error formatting infrastructure used by all CUE packages

**⚠️ CRITICAL**: All user stories depend on consistent error formatting

- [X] T004 Create `formatCUEError()` helper in pkg/invowkfile/parse.go with path extraction
- [X] T005 Add file size check to `ParseBytes()` in pkg/invowkfile/parse.go
- [X] T006 Add file size check to `ParseInvowkmodBytes()` in pkg/invowkmod/invowkmod.go

**Checkpoint**: Foundation ready - size guards and error formatting in place

---

## Phase 3: User Story 1 - Reliable Schema Validation (Priority: P1) 🎯 MVP

**Goal**: Schema sync tests catch Go/CUE misalignments at CI time

**Independent Test**: Modify a CUE schema field, run `make test`, verify sync test fails with clear error message

### Implementation for User Story 1

- [X] T007 [US1] Add sync test for `Invowkfile` struct in pkg/invowkfile/sync_test.go
- [X] T008 [US1] Add sync test for `Command` struct in pkg/invowkfile/sync_test.go
- [X] T009 [US1] Add sync test for `Implementation` struct in pkg/invowkfile/sync_test.go
- [X] T010 [US1] Add sync test for `RuntimeConfig` struct in pkg/invowkfile/sync_test.go
- [X] T011 [US1] Add sync test for `DependsOn` struct in pkg/invowkfile/sync_test.go
- [X] T012 [US1] Add sync test for `Flag` and `Argument` structs in pkg/invowkfile/sync_test.go
- [X] T013 [P] [US1] Create sync_test.go for `Invowkmod` struct in pkg/invowkmod/sync_test.go
- [X] T014 [P] [US1] Create sync_test.go for `Config` struct in internal/config/sync_test.go
- [X] T015 [US1] Add CUE schema validation to `loadCUEIntoViper()` in internal/config/config.go (unify parsed config with #Config schema, call Validate(), use formatCUEError for errors)

**Checkpoint**: All sync tests pass, any field mismatch causes test failure

---

## Phase 4: User Story 2 - Minimal Redundant Validation (Priority: P2)

**Goal**: Validation lives in one authoritative location (CUE preferred)

**Independent Test**: Search for duplicate validation patterns; verify each exists in only one layer

### Implementation for User Story 2

- [X] T016 [US2] Audit `validateInterpreter()` in pkg/invowkfile/invowkfile_validation.go for CUE redundancy; remove if redundant, add justification comment if Go-only validation is necessary
- [X] T017 [US2] Audit `validateToolDependency()` in pkg/invowkfile/invowkfile_validation.go for CUE redundancy; remove if redundant, add justification comment if Go-only validation is necessary
- [X] T018 [US2] Audit `validateEnvVar()` in pkg/invowkfile/invowkfile_validation.go for CUE redundancy; remove if redundant, add justification comment if Go-only validation is necessary
- [X] T019 [US2] Add justification comments for Go-only validations (ReDoS, filesystem, cross-field)

**Checkpoint**: Zero redundant validation; Go-only checks have justification comments

---

## Phase 5: User Story 3 - Type-Safe Value Extraction (Priority: P2)

**Goal**: CUE `Decode()` used consistently; manual extraction justified

**Independent Test**: Grep for `String()`, `Int64()` manual extraction; verify justification exists

### Implementation for User Story 3

- [X] T020 [US3] Audit pkg/invowkfile/parse.go for manual CUE value extraction
- [X] T021 [US3] Audit pkg/invowkmod/invowkmod.go for manual CUE value extraction
- [X] T022 [US3] Audit internal/config/config.go for manual CUE value extraction
- [X] T023 [US3] Add justification comments for any necessary manual extraction

**Checkpoint**: All CUE value extraction uses `Decode()` or has documented justification
**Result**: ✅ PASS - All three files already use `Decode()`. No manual extraction found; no justification comments needed.

---

## Phase 6: User Story 4 - Documented Best Practices (Priority: P3)

**Goal**: Comprehensive CUE rules in `.claude/rules/cue.md`

**Independent Test**: New contributor can follow rules file to add CUE-based configuration correctly

### Implementation for User Story 4

- [X] T024 [US4] Document schema compilation pattern (3-step flow) in .claude/rules/cue.md
- [X] T025 [US4] Document validation responsibility matrix in .claude/rules/cue.md
- [X] T026 [US4] Document Decode usage rules in .claude/rules/cue.md
- [X] T027 [US4] Document field naming convention (snake_case → PascalCase) in .claude/rules/cue.md
- [X] T028 [US4] Document error formatting requirements in .claude/rules/cue.md
- [X] T029 [US4] Document CUE library version pinning and upgrade process in .claude/rules/cue.md
- [X] T030 [US4] Add common pitfalls section (unclosed structs, redundant validation) to .claude/rules/cue.md

**Checkpoint**: Rules file covers all 5 key patterns; documentation is standalone comprehensible

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Final verification and cleanup

- [X] T031 Run `make test` and verify all sync tests pass; spot-check CUE schemas for defense-in-depth constraints (regex, MaxRunes, range) per FR-009
- [X] T032 Run `make lint` and verify zero CUE-related warnings
- [X] T033 Verify error messages include full JSON paths (manual testing)
- [X] T034 Update CLAUDE.md Active Technologies section if needed
- [X] T035 Run quickstart.md validation checklist

**Checkpoint**: ✅ All Phase 7 tasks complete - Feature ready for final review

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on T001 constant
- **User Story 1 (Phase 3)**: Depends on T002, T003 helpers
- **User Story 2 (Phase 4)**: Can start after Foundational
- **User Story 3 (Phase 5)**: Can start after Foundational
- **User Story 4 (Phase 6)**: Can start after US1-US3 provide patterns to document
- **Polish (Phase 7)**: Depends on all user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Independent - core schema sync tests
- **User Story 2 (P2)**: Independent - validation audit
- **User Story 3 (P2)**: Independent - extraction audit
- **User Story 4 (P3)**: Should follow US1-US3 to document learned patterns

### Within Each User Story

- Setup → Foundational → User Story implementation
- Audit tasks before code changes
- Justification comments after audits complete

### Parallel Opportunities

**Phase 1 (Setup)**:
- T002 and T003 can run in parallel (different helper functions)

**Phase 3 (US1)**:
- T013 and T014 can run in parallel (different packages: invowkmod, config)

**User Stories 2 & 3**:
- Can run in parallel with each other (different concerns: validation vs extraction)

---

## Parallel Example: User Story 1

```bash
# After T001-T006 complete, launch sync tests in parallel:
Task: "Add sync test for Invowkfile struct"      # T007
Task: "Add sync test for Command struct"       # T008
# (sequential within invowkfile package)

# In parallel with invowkfile sync tests:
Task: "Create sync_test.go for Invowkmod"        # T013 [P]
Task: "Create sync_test.go for Config"         # T014 [P]
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (T001-T003)
2. Complete Phase 2: Foundational (T004-T006)
3. Complete Phase 3: User Story 1 (T007-T015)
4. **STOP and VALIDATE**: Run `make test`, verify sync tests catch mismatches
5. This delivers core value: schema sync verification

### Incremental Delivery

1. Setup + Foundational → Error handling and size guards ready
2. Add User Story 1 → Schema sync tests operational (MVP!)
3. Add User Story 2 → Validation consolidated, cleaner codebase
4. Add User Story 3 → Type-safe extraction verified
5. Add User Story 4 → Documentation complete for contributors

### Single Developer Strategy

1. Complete Setup + Foundational first (T001-T006)
2. User Story 1 is highest value - complete fully (T007-T015)
3. User Stories 2 & 3 are quick audits - can interleave
4. User Story 4 (documentation) best done after patterns are clear

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story should be independently completable and testable
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
- This is primarily a refactoring/testing/documentation effort - no new features
- Total estimated changes: ~250 lines of new test code, ~80 lines of modifications, ~150 lines of documentation
