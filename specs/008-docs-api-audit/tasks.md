# Tasks: Documentation/API Audit

**Input**: Design documents from `/specs/008-docs-api-audit/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/, quickstart.md

**Tests**: Tests are required and must include unit coverage plus testscript CLI coverage for `invowk docs audit`.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project initialization and CLI scaffolding

- [X] T001 Create audit package entrypoint in `internal/docsaudit/docsaudit.go`
- [X] T002 Create and register `docs` command group in `cmd/invowk/docs.go`
- [X] T003 Implement `docs audit` subcommand wiring and flags (including `--output human|json`) in `cmd/invowk/docs_audit.go`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that MUST be complete before ANY user story can be implemented

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [X] T004 Define core data types matching `specs/008-docs-api-audit/data-model.md` in `internal/docsaudit/types.go`
- [X] T005 [P] Implement repo scanning helpers in `internal/docsaudit/fs.go`
- [X] T006 [P] Implement audit options parsing/defaults and output format parsing in `internal/docsaudit/options.go`
- [X] T007 [P] Implement Markdown report writer skeleton, JSON summary output, and file output in `internal/docsaudit/report.go`
- [X] T008 Implement audit orchestration in `internal/docsaudit/audit.go`

**Checkpoint**: Foundation ready - user story implementation can now begin in parallel

---

## Phase 3: User Story 1 - Coverage & Mismatch Inventory (Priority: P1) 🎯 MVP

**Goal**: Inventory user-facing surfaces and map them to documentation with coverage metrics and docs-only detection.

**Independent Test**: A reviewer can verify that every in-scope user-facing surface is either linked to a specific doc location or flagged as undocumented.

### Implementation for User Story 1

- [X] T009 [P] [US1] Implement documentation source discovery in `internal/docsaudit/sources.go`
- [X] T010 [P] [US1] Implement CLI surface inventory (commands/flags) in `internal/docsaudit/surfaces_cli.go`
- [X] T011 [P] [US1] Implement config/schema surface inventory in `internal/docsaudit/surfaces_config.go`
- [X] T012 [P] [US1] Implement module surface inventory in `internal/docsaudit/surfaces_modules.go`
- [X] T013 [US1] Implement docs-to-surface matching and docs-only detection in `internal/docsaudit/matching.go`
- [X] T014 [US1] Implement coverage metrics aggregation in `internal/docsaudit/metrics.go`
- [X] T015 [US1] Wire surface inventory + metrics into report sections in `internal/docsaudit/report.go`

**Checkpoint**: At this point, User Story 1 should be fully functional and testable independently

---

## Phase 4: User Story 2 - Example Validation Summary (Priority: P2)

**Goal**: Validate documentation examples and identify invalid or misleading snippets with reasons.

**Independent Test**: A reviewer can inspect the report and confirm that every example is marked valid or invalid with a concrete reason.

### Implementation for User Story 2

- [X] T016 [P] [US2] Implement example extraction from docs and samples in `internal/docsaudit/examples_extract.go`
- [X] T017 [US2] Implement example validation against surfaced features in `internal/docsaudit/examples_validate.go`
- [X] T018 [P] [US2] Implement canonical examples location scan in `internal/docsaudit/examples_canonical.go`
- [X] T019 [US2] Update report examples section and canonical path notes in `internal/docsaudit/report.go`

**Checkpoint**: At this point, User Stories 1 AND 2 should both work independently

---

## Phase 5: User Story 3 - Prioritized Fix Plan (Priority: P3)

**Goal**: Categorize findings by severity and provide actionable recommendations.

**Independent Test**: A reviewer can filter the report by severity and see a clear, actionable fix recommendation for each finding.

### Implementation for User Story 3

- [X] T020 [P] [US3] Implement severity mapping by user impact in `internal/docsaudit/severity.go`
- [X] T021 [P] [US3] Implement recommendation generator in `internal/docsaudit/recommendations.go`
- [X] T022 [US3] Integrate severity + recommendations into findings and report output in `internal/docsaudit/matching.go` and `internal/docsaudit/report.go`

**Checkpoint**: All user stories should now be independently functional

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Documentation and final alignment tasks

- [X] T023 [P] Update CLI usage documentation in `README.md`
- [X] T024 [P] Update website CLI reference in `website/docs/reference/cli.mdx`
- [X] T025 [P] Validate and align quickstart command text in `specs/008-docs-api-audit/quickstart.md`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion - BLOCKS all user stories
- **User Stories (Phase 3+)**: All depend on Foundational phase completion
  - User stories can proceed in parallel or sequentially in priority order (P1 → P2 → P3)
- **Polish (Final Phase)**: Depends on all desired user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Foundational (Phase 2) - No dependencies on other stories
- **User Story 2 (P2)**: Can start after Foundational (Phase 2) - No dependencies on other stories
- **User Story 3 (P3)**: Can start after Foundational (Phase 2) - No dependencies on other stories

### Within Each User Story

- Models and core types before matching or report integration
- Surface discovery before example validation
- Report updates after data generation
- Story complete before moving to next priority (if sequential)

### Parallel Opportunities

- Foundational tasks marked [P] can run in parallel after T004
- User Story 1 discovery tasks (T009-T012) can run in parallel
- User Story 2 extraction and canonical scan (T016, T018) can run in parallel
- User Story 3 severity and recommendation tasks (T020, T021) can run in parallel

---

## Parallel Example: User Story 1

```bash
Task: "Implement documentation source discovery in internal/docsaudit/sources.go"
Task: "Implement CLI surface inventory in internal/docsaudit/surfaces_cli.go"
Task: "Implement config/schema surface inventory in internal/docsaudit/surfaces_config.go"
Task: "Implement module surface inventory in internal/docsaudit/surfaces_modules.go"
```

---

## Parallel Example: User Story 2

```bash
Task: "Implement example extraction in internal/docsaudit/examples_extract.go"
Task: "Implement canonical examples location scan in internal/docsaudit/examples_canonical.go"
```

---

## Parallel Example: User Story 3

```bash
Task: "Implement severity mapping in internal/docsaudit/severity.go"
Task: "Implement recommendation generator in internal/docsaudit/recommendations.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (CRITICAL - blocks all stories)
3. Complete Phase 3: User Story 1
4. **STOP and VALIDATE**: Verify coverage inventory and docs-only detection
5. Demo the report output for a single repo snapshot

### Incremental Delivery

1. Complete Setup + Foundational → Foundation ready
2. Add User Story 1 → Verify independently → MVP
3. Add User Story 2 → Validate examples → Release
4. Add User Story 3 → Severity + recommendations → Release
5. Each story adds value without breaking previous stories

### Parallel Team Strategy

With multiple developers:

1. Team completes Setup + Foundational together
2. Once Foundational is done:
   - Developer A: User Story 1
   - Developer B: User Story 2
   - Developer C: User Story 3
3. Stories complete and integrate independently

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story should be independently completable and testable
- Avoid vague tasks; include exact file paths

---

## Phase 7: Testing & Verification

**Purpose**: Unit and CLI test coverage for docs audit

- [X] T026 [P] unit tests for surface inventory, matching, metrics in `internal/docsaudit/*_test.go`
- [X] T027 [P] unit tests for examples extraction/validation in `internal/docsaudit/examples_*_test.go`
- [X] T028 [P] unit tests for severity + recommendations in `internal/docsaudit/*_test.go`
- [X] T029 CLI testscript coverage for docs audit (include JSON output case) in `tests/cli/testdata/docs_audit.txtar`
