# Feature Specification: Test Suite Audit and Improvements

**Feature Branch**: `003-test-suite-audit`
**Created**: 2026-01-29
**Status**: Draft
**Input**: User description: "Inspect all tests from the Go codebase and identify significant improvements/gaps with respect to: test files >800 lines, code/fixture duplication, brittle tests, flaky patterns, low-value tests, and missing test coverage for significant behaviors."

## Clarifications

### Session 2026-01-29

- Q: Should refactoring or new test coverage be prioritized? → A: Refactoring first, then new tests (sequential). Rationale: reduces maintenance burden and makes adding new tests easier.
- Q: What is the acceptable test execution time increase threshold? → A: No threshold - prioritize coverage over speed. Test quality and coverage are more important than execution time.
- Q: What test artifacts are in scope for this audit? → A: Only Go test files (`*_test.go`). Testscript `.txtar` files and VHS demo tapes are out of scope (already well-structured).

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Developer Navigating Test Files (Priority: P1)

A developer (human or AI agent) needs to modify a feature and update its corresponding tests. They navigate to the test file and can quickly understand its scope, find relevant test cases, and make targeted changes without being overwhelmed by unrelated test code.

**Why this priority**: Large monolithic test files (>800 lines) are the biggest barrier to maintainability. When `invowkfile_test.go` is 6,597 lines covering parsing, dependencies, capabilities, flags, args, environment, workdir, and schema validation, finding and updating the right tests becomes time-consuming and error-prone.

**Independent Test**: Can be fully tested by measuring time-to-first-edit for a developer tasked with updating tests for a specific feature. Success is measurable file sizes and clear file naming conventions.

**Acceptance Scenarios**:

1. **Given** a test file exists for a feature, **When** a developer opens it, **Then** they can identify the file's scope from its name within 5 seconds
2. **Given** a large test file (>800 lines), **When** it is refactored, **Then** each resulting file covers a single logical concern and is under 800 lines
3. **Given** multiple test files exist for a package, **When** reviewing the package, **Then** there is a clear naming convention indicating what each file tests

---

### User Story 2 - Developer Reusing Test Utilities (Priority: P1)

A developer writing new tests needs common test fixtures (mock commands, environment setup, temporary directories). They can find and reuse existing test utilities without duplicating code across test files.

**Why this priority**: Duplicated test helpers create maintenance burden. When `testCommand()` is implemented identically in 3 different packages, any change to command structure requires 3 separate updates. This also makes onboarding harder.

**Independent Test**: Can be verified by searching for duplicate function signatures across test files and confirming utilities are centralized in `testutil` package.

**Acceptance Scenarios**:

1. **Given** a common test pattern exists (e.g., creating mock commands), **When** a developer needs that pattern, **Then** they find a single canonical implementation in `testutil`
2. **Given** duplicated test helpers exist across packages, **When** they are consolidated, **Then** only one implementation remains and all usages are updated
3. **Given** a new test utility is needed, **When** it is created, **Then** it is placed in `testutil` with clear documentation

---

### User Story 3 - CI/CD Pipeline Reliability (Priority: P2)

The CI/CD pipeline runs the test suite on every pull request. Tests pass or fail deterministically based solely on code correctness, not timing, system load, or environmental factors.

**Why this priority**: Flaky tests erode confidence in the test suite and waste developer time investigating false failures. Time-dependent tests and race conditions cause intermittent failures that are difficult to diagnose.

**Independent Test**: Can be verified by running the test suite multiple times (10+ runs) on CI and confirming zero flaky failures.

**Acceptance Scenarios**:

1. **Given** a test that relies on time (e.g., token expiration), **When** it is refactored, **Then** it uses deterministic time mocking instead of `time.Sleep()`
2. **Given** the full test suite, **When** run 10 consecutive times on CI, **Then** all runs produce identical pass/fail results
3. **Given** a test with timing assumptions, **When** run on both fast and slow systems, **Then** it passes consistently

---

### User Story 4 - TUI Component Reliability (Priority: P2)

Users interact with TUI components (command chooser, input prompts, file selector, table display). These components handle edge cases correctly: empty inputs, very long inputs, special characters, terminal resize events.

**Why this priority**: TUI components represent ~4,250 lines of code with minimal test coverage. While terminal I/O is difficult to test, component state and text processing logic can and should be unit tested. Bugs in TUI components directly impact user experience.

**Independent Test**: Can be verified by running TUI component unit tests that exercise state transitions and text processing without requiring actual terminal I/O.

**Acceptance Scenarios**:

1. **Given** a TUI component (e.g., `choose`), **When** unit tests exist, **Then** they cover model state transitions without terminal I/O
2. **Given** a text input component, **When** tested with edge cases (empty, very long, unicode, special chars), **Then** all cases are handled correctly
3. **Given** a list component, **When** tested with zero items, one item, and many items, **Then** rendering logic handles all cases

---

### User Story 5 - Container Runtime Testing (Priority: P3)

Developers modify container runtime code (Docker/Podman integration). Unit tests verify argument construction, error handling, and edge cases without requiring actual container engines.

**Why this priority**: Container Build/Run/ImageExists operations currently lack unit tests. Integration tests exist but only run when containers are available. Mock-based unit tests would catch logic errors faster and run on all systems.

**Independent Test**: Can be verified by running container runtime unit tests that use mocked exec.Command and verify expected arguments and error handling.

**Acceptance Scenarios**:

1. **Given** a Docker engine method, **When** unit tested with mocked exec, **Then** argument construction is verified without running Docker
2. **Given** an error scenario (image not found, build failure), **When** unit tested, **Then** error handling paths are exercised
3. **Given** platform-specific logic, **When** unit tested, **Then** both Linux and non-Linux paths are verified

---

### Edge Cases

- What happens when a test file is split but tests share complex setup? Use shared test fixtures and setup functions.
- How to test TUI components without a terminal? Test model/state logic separately from rendering; use Bubble Tea's test utilities.
- What if removing duplicate helpers changes behavior? Ensure consolidated helpers maintain exact existing behavior; run full test suite to verify.

## Requirements *(mandatory)*

### Functional Requirements

#### Test File Organization

- **FR-001**: Test files exceeding 800 lines MUST be refactored into smaller focused files
- **FR-002**: Each test file MUST cover a single logical concern (e.g., parsing, dependencies, flags)
- **FR-003**: Test file names MUST clearly indicate their scope (e.g., `invowkfile_parsing_test.go`, `invowkfile_deps_test.go`)

#### Test Utility Consolidation

- **FR-004**: Duplicated test helper functions MUST be consolidated into the `testutil` package
- **FR-005**: The `testutil` package MUST provide a `NewTestCommand()` builder with options pattern
- **FR-006**: The `testutil` package MUST provide platform-aware environment setup helpers

#### Test Reliability

- **FR-007**: Tests MUST NOT use `time.Sleep()` for verifying time-dependent behavior
- **FR-008**: Time-dependent tests MUST use clock injection or deterministic time mocking
- **FR-009**: Tests MUST use `t.TempDir()` instead of hardcoded paths like `/tmp`

#### Coverage Gaps

- **FR-010**: TUI components MUST have unit tests covering model state transitions
- **FR-011**: TUI text processing functions MUST have unit tests for edge cases
- **FR-012**: Container runtime methods MUST have mock-based unit tests for argument construction
- **FR-013**: Container runtime error paths MUST be tested

#### Test Quality

- **FR-014**: Pure struct field assignment tests (low value) SHOULD be removed or converted to behavior tests
- **FR-015**: Tests MUST verify behavior, not implementation details

### Out of Scope

- **Testscript files** (`.txtar` in `tests/cli/testdata/`): Already comprehensive and well-structured
- **VHS demo tapes** (`vhs/demos/`): Used for documentation GIFs, not CI testing
- **Benchmark tests**: Performance benchmarking is separate from correctness testing
- **Fuzz tests**: Fuzzing infrastructure is a separate initiative

### Key Entities

- **Test File**: A `*_test.go` file containing test functions for a specific package
- **Test Helper**: A reusable function in test code that sets up fixtures or asserts common conditions
- **Test Fixture**: Pre-defined test data (mock commands, configurations, file structures)
- **Test Utility Package**: The `testutil` package providing shared testing infrastructure
- **TUI Component**: A Bubble Tea model implementing interactive terminal UI (choose, confirm, input, etc.)

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: No test file exceeds 800 lines of code after refactoring
- **SC-002**: Duplicated test helpers are reduced from 3+ instances to 1 canonical implementation
- **SC-003**: Test suite passes 10 consecutive runs on CI with zero flaky failures
- **SC-004**: TUI component test coverage increases from ~5% to at least 50% (measured by function coverage)
- **SC-005**: Container runtime unit test coverage for Build/Run/ImageExists reaches at least 80%
- **SC-006**: All time-dependent tests use deterministic mocking (zero `time.Sleep()` in test assertions)
- **SC-007**: Test coverage and quality are prioritized over execution time (no arbitrary time threshold)
- **SC-008**: All existing tests continue to pass after refactoring (zero regressions)

## Appendix: Current State Analysis

### Files Requiring Refactoring (>800 lines)

| File                                        | Lines | Recommended Split                                                               |
| ------------------------------------------- | ----- | ------------------------------------------------------------------------------- |
| `pkg/invowkfile/invowkfile_test.go`             | 6,597 | Split into: parsing, dependencies, flags, args, platforms (incl. capabilities), environment, workdir, schema |
| `cmd/invowk/cmd_test.go`                    | 2,567 | Split into: dependencies, rendering, flags/args, source filtering               |
| `internal/discovery/discovery_test.go`      | 1,842 | Split into: basic discovery, modules, collisions/precedence                     |
| `pkg/invowkmod/operations_test.go`            | 1,683 | Consider split if scope grows                                                   |
| `internal/runtime/runtime_test.go`          | 1,605 | Split into: native, virtual, container, common                                  |
| `internal/runtime/container_integration_test.go` | 847 | At threshold; monitor but acceptable                                            |

### Duplicated Helpers to Consolidate

| Helper                            | Locations                                                  | Target                      |
| --------------------------------- | ---------------------------------------------------------- | --------------------------- |
| `testCommand()` / `testCmd()`     | `invowkfile_test.go`, `cmd_test.go`, `runtime_test.go`       | `testutil.NewTestCommand()` |
| `setHomeDirEnv()`                 | `config_test.go`, `discovery_test.go`, `cmd_test.go`       | `testutil.SetHomeDir()`     |

### TUI Components Lacking Tests

| Component      | Lines | Current Tests |
| -------------- | ----- | ------------- |
| `choose.go`    | 564   | 0             |
| `confirm.go`   | 240   | 0             |
| `file.go`      | 305   | 0             |
| `filter.go`    | 566   | 0             |
| `format.go`    | 270   | 0             |
| `input.go`     | 274   | 0             |
| `pager.go`     | 260   | 0             |
| `table.go`     | 425   | 0             |
| `spin.go`      | 402   | 0             |
| `tui.go`       | 345   | 0             |

### Strengths to Preserve

- Comprehensive testscript-based CLI integration tests
- Table-driven test patterns used consistently
- Proper integration test gating with `testing.Short()`
- Strong server state machine tests (SSH, TUI servers)
- Well-maintained `testutil` package foundation
