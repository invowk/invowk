# Tasks: u-root Utils Integration

**Input**: Design documents from `/specs/005-uroot-utils/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/

**Tests**: Included as specified in plan.md (unit tests for each utility + CLI integration tests)

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Path Conventions

- **Go project**: `internal/` for private packages, `pkg/` for public
- **Tests**: Co-located `*_test.go` files + `tests/cli/testdata/` for CLI integration tests

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Add u-root dependency and create package structure

- [X] T001 Add u-root dependency with `go get github.com/u-root/u-root@v0.15.0` and run `make tidy`
- [X] T002 Create `internal/uroot/doc.go` with package documentation and SPDX header

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that MUST be complete before ANY user story can be implemented

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [X] T003 Implement `Command` interface and `FlagInfo` struct in `internal/uroot/command.go`
- [X] T004 Implement `HandlerContext` extraction from mvdan/sh context in `internal/uroot/handler.go`
- [X] T005 Implement `Registry` struct with `Register`, `Lookup`, `Names`, and `Run` methods in `internal/uroot/registry.go`
- [X] T006 Write unit tests for `Registry` in `internal/uroot/registry_test.go`
- [X] T007 Wire registry into `VirtualRuntime.tryUrootBuiltin()` in `internal/runtime/virtual.go`

**Checkpoint**: Foundation ready - user story implementation can now begin in parallel

---

## Phase 3: User Story 1 - Execute File Operations Without External Dependencies (Priority: P1) 🎯 MVP

**Goal**: Enable invowkfiles with file operations (`cp`, `mv`, `cat`, `ls`, `mkdir`, `rm`, `touch`) to execute on systems without those binaries installed when u-root is enabled.

**Independent Test**: Configure `enable_uroot_utils: true`, write an invowkfile using `cp`, `mv`, `cat`, and execute it on a minimal system without coreutils binaries.

### Tests for User Story 1

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [X] T008 [P] [US1] Write unit tests for `cat` wrapper in `internal/uroot/cat_test.go`
- [X] T009 [P] [US1] Write unit tests for `mkdir` wrapper in `internal/uroot/mkdir_test.go`
- [X] T010 [P] [US1] Write unit tests for `rm` wrapper in `internal/uroot/rm_test.go`
- [X] T011 [P] [US1] Write unit tests for `touch` wrapper in `internal/uroot/touch_test.go`
- [X] T012 [P] [US1] Write unit tests for `cp` wrapper in `internal/uroot/cp_test.go`
- [X] T013 [P] [US1] Write unit tests for `mv` wrapper in `internal/uroot/mv_test.go`
- [X] T014 [P] [US1] Write unit tests for `ls` wrapper in `internal/uroot/ls_test.go`

### Implementation for User Story 1 - pkg/core Wrappers

- [X] T015 [P] [US1] Implement base wrapper helper function in `internal/uroot/wrapper.go`
- [X] T016 [P] [US1] Implement `cat` wrapper in `internal/uroot/cat.go`
- [X] T017 [P] [US1] Implement `mkdir` wrapper in `internal/uroot/mkdir.go`
- [X] T018 [P] [US1] Implement `rm` wrapper in `internal/uroot/rm.go`
- [X] T019 [P] [US1] Implement `touch` wrapper in `internal/uroot/touch.go`
- [X] T020 [US1] Implement `cp` wrapper in `internal/uroot/cp.go` (verify streaming I/O)
- [X] T021 [US1] Implement `mv` wrapper in `internal/uroot/mv.go`
- [X] T022 [US1] Implement `ls` wrapper in `internal/uroot/ls.go`
- [X] T023 [US1] Register all pkg/core wrappers in `internal/uroot/registry.go`

### CLI Integration Tests for User Story 1

- [X] T024 [US1] Write CLI integration test for basic file operations in `tests/cli/testdata/uroot_basic.txtar`
- [X] T025 [US1] Write CLI integration test for file operations (cp, mv, rm) in `tests/cli/testdata/uroot_file_ops.txtar`

**Checkpoint**: At this point, User Story 1 should be fully functional and testable independently. Commands `cat`, `cp`, `ls`, `mkdir`, `mv`, `rm`, `touch` work without system binaries.

---

## Phase 4: User Story 2 - Predictable Cross-Platform Behavior (Priority: P2)

**Goal**: Enable consistent behavior for text processing operations (`head`, `tail`, `wc`, `grep`, `sort`, `uniq`, `cut`, `tr`) across Linux, macOS, and Windows.

**Independent Test**: Run the same invowkfile with `cat file | grep pattern | wc -l` on Linux, macOS, and Windows with u-root enabled; verify identical results.

### Tests for User Story 2

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [X] T026 [P] [US2] Write unit tests for `head` in `internal/uroot/head_test.go`
- [X] T027 [P] [US2] Write unit tests for `tail` in `internal/uroot/tail_test.go`
- [X] T028 [P] [US2] Write unit tests for `wc` in `internal/uroot/wc_test.go`
- [X] T029 [P] [US2] Write unit tests for `grep` in `internal/uroot/grep_test.go`
- [X] T030 [P] [US2] Write unit tests for `sort` in `internal/uroot/sort_test.go`
- [X] T031 [P] [US2] Write unit tests for `uniq` in `internal/uroot/uniq_test.go`
- [X] T032 [P] [US2] Write unit tests for `cut` in `internal/uroot/cut_test.go`
- [X] T033 [P] [US2] Write unit tests for `tr` in `internal/uroot/tr_test.go`

### Implementation for User Story 2 - Custom Implementations

- [X] T034 [P] [US2] Implement `head` command (line counter with early exit) in `internal/uroot/head.go`
- [X] T035 [P] [US2] Implement `tail` command (ring buffer for N lines) in `internal/uroot/tail.go`
- [X] T036 [P] [US2] Implement `wc` command (streaming counters) in `internal/uroot/wc.go`
- [X] T037 [US2] Implement `grep` command (line-by-line regex matching) in `internal/uroot/grep.go`
- [X] T038 [US2] Implement `sort` command (may use temp files for large inputs) in `internal/uroot/sort.go`
- [X] T039 [P] [US2] Implement `uniq` command (adjacent line comparison) in `internal/uroot/uniq.go`
- [X] T040 [P] [US2] Implement `cut` command (field delimiter parsing) in `internal/uroot/cut.go`
- [X] T041 [P] [US2] Implement `tr` command (character mapping) in `internal/uroot/tr.go`
- [X] T042 [US2] Register all custom implementations in `internal/uroot/registry.go`

### CLI Integration Tests for User Story 2

- [X] T043 [US2] Write CLI integration test for text operations (head, tail, grep) in `tests/cli/testdata/uroot_text_ops.txtar`

**Checkpoint**: At this point, User Stories 1 AND 2 should both work independently. All 15 utilities function without system binaries.

---

## Phase 5: User Story 3 - Gradual Adoption with Fallback (Priority: P3)

**Goal**: Ensure u-root utils handle supported commands while unsupported commands fall back to system binaries, and failed u-root commands do NOT silently fall back.

**Independent Test**: Enable u-root, run `cp file1 file2 && git status` - verify `cp` uses u-root while `git` falls back to system binary. Also verify that if u-root `cp` fails, it returns an error (no fallback).

### Tests for User Story 3

- [X] T044 [P] [US3] Write unit tests for fallback behavior (unregistered commands use system) in `internal/uroot/registry_test.go`
- [X] T045 [P] [US3] Write unit tests for no-silent-fallback (u-root errors are propagated) in `internal/uroot/registry_test.go`

### Implementation for User Story 3

- [X] T046 [US3] Verify `tryUrootBuiltin` returns `handled=false` for unregistered commands in `internal/runtime/virtual.go`
- [X] T047 [US3] Verify u-root command errors are propagated with `[uroot]` prefix (no fallback) in `internal/runtime/virtual.go`

**Checkpoint**: All user stories should now be independently functional. Feature is complete.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Documentation, validation, and quality improvements

- [X] T048 [P] Update `website/docs/runtime-modes/virtual.mdx` with u-root utils documentation
- [X] T049 [P] Update `README.md` to mention u-root utils support
- [X] T050 [P] Update config documentation in `website/docs/configuration/options.mdx` for `enable_uroot_utils`
- [X] T051 Run `make lint` and fix any linting issues
- [X] T052 Run `make test` and verify all tests pass
- [X] T053 Run `make test-cli` and verify CLI integration tests pass
- [X] T054 Run `make license-check` and ensure all new .go files have SPDX headers
- [X] T055 Verify streaming I/O compliance: audit all file operations for `io.ReadAll` or `os.ReadFile` misuse
- [X] T056 Verify error prefix format: audit all error returns for `[uroot]` prefix compliance
- [X] T057 Run quickstart.md validation: execute the test invowkfile from quickstart.md

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion - BLOCKS all user stories
- **User Stories (Phase 3+)**: All depend on Foundational phase completion
  - User stories can then proceed in parallel (if staffed)
  - Or sequentially in priority order (P1 → P2 → P3)
- **Polish (Final Phase)**: Depends on all desired user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Foundational (Phase 2) - No dependencies on other stories
- **User Story 2 (P2)**: Can start after Foundational (Phase 2) - No dependencies on US1 (independent text utilities)
- **User Story 3 (P3)**: Can start after Foundational (Phase 2) - Validates behavior from US1 and US2, but is primarily verification/testing

### Within Each User Story

- Tests MUST be written and FAIL before implementation
- Wrappers/implementations before registry registration
- Registry registration enables CLI integration tests
- Story complete before moving to next priority

### Parallel Opportunities

- **Phase 2 (Foundational)**: T003, T004, T005 can run in parallel (different files)
- **US1 Tests**: All T008-T014 can run in parallel (different test files)
- **US1 Implementation**: T015-T019 can run in parallel (different utility files)
- **US2 Tests**: All T026-T033 can run in parallel (different test files)
- **US2 Implementation**: T034-T036, T039-T041 can run in parallel (different utility files)
- **US3**: T044-T045 can run in parallel (same test file but different test functions)
- **Polish**: T048-T050 can run in parallel (different doc files)

---

## Parallel Example: User Story 1 Tests

```bash
# Launch all US1 tests together:
Task: "Write unit tests for cat wrapper in internal/uroot/cat_test.go"
Task: "Write unit tests for mkdir wrapper in internal/uroot/mkdir_test.go"
Task: "Write unit tests for rm wrapper in internal/uroot/rm_test.go"
Task: "Write unit tests for touch wrapper in internal/uroot/touch_test.go"
Task: "Write unit tests for cp wrapper in internal/uroot/cp_test.go"
Task: "Write unit tests for mv wrapper in internal/uroot/mv_test.go"
Task: "Write unit tests for ls wrapper in internal/uroot/ls_test.go"
```

## Parallel Example: User Story 2 Implementations

```bash
# Launch simpler custom implementations together:
Task: "Implement head command in internal/uroot/head.go"
Task: "Implement tail command in internal/uroot/tail.go"
Task: "Implement wc command in internal/uroot/wc.go"
Task: "Implement uniq command in internal/uroot/uniq.go"
Task: "Implement cut command in internal/uroot/cut.go"
Task: "Implement tr command in internal/uroot/tr.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (T001-T002)
2. Complete Phase 2: Foundational (T003-T007) - CRITICAL
3. Complete Phase 3: User Story 1 (T008-T025)
4. **STOP and VALIDATE**: Test file operations independently
5. Deploy/demo if ready - basic file utilities work!

### Incremental Delivery

1. Complete Setup + Foundational → Foundation ready
2. Add User Story 1 → Test independently → Deploy/Demo (MVP! 7 utilities)
3. Add User Story 2 → Test independently → Deploy/Demo (Full feature! 15 utilities)
4. Add User Story 3 → Verify fallback behavior → Complete!
5. Each story adds value without breaking previous stories

### Parallel Team Strategy

With multiple developers:

1. Team completes Setup + Foundational together
2. Once Foundational is done:
   - Developer A: User Story 1 (pkg/core wrappers)
   - Developer B: User Story 2 (custom implementations)
3. Stories complete and integrate independently
4. Developer A or B: User Story 3 (verification)

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story should be independently completable and testable
- Verify tests fail before implementing
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
- **Streaming I/O**: All file operations MUST use `io.Copy()` or equivalent - never `io.ReadAll()` or `os.ReadFile()` for arbitrary user files
- **Error prefix**: All errors MUST be prefixed with `[uroot] <cmd>:`
- Avoid: vague tasks, same file conflicts, cross-story dependencies that break independence
