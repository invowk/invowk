# Implementation Plan: Test Suite Audit and Improvements

**Branch**: `003-test-suite-audit` | **Date**: 2026-01-29 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/003-test-suite-audit/spec.md`

## Summary

Comprehensive audit and refactoring of Go test files to improve maintainability, reliability, and coverage. Primary work streams: (1) split large monolithic test files exceeding 800 lines into focused single-concern files, (2) consolidate duplicated test helpers into the `testutil` package, (3) fix flaky time-dependent tests with clock injection, (4) add TUI component unit tests for model state transitions, and (5) add container runtime mock-based unit tests.

## Technical Context

**Language/Version**: Go 1.26+
**Primary Dependencies**: stdlib `testing`, `github.com/charmbracelet/bubbletea` (TUI), `mvdan.cc/sh/v3` (virtual shell), `github.com/rogpeppe/go-internal/testscript` (CLI tests)
**Storage**: N/A (test infrastructure only)
**Testing**: Go's built-in `testing` package, testscript for CLI integration tests
**Target Platform**: Linux, macOS, Windows (cross-platform)
**Project Type**: Single - CLI tool with internal packages
**Performance Goals**: N/A (coverage prioritized over execution time per spec clarification)
**Constraints**: Zero regressions - all existing tests must continue to pass
**Scale/Scope**: 27 test files totaling ~21,725 lines; 6 files over 800 lines; 10 TUI components without tests

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Idiomatic Go & Schema-Driven Design | ✅ PASS | Test refactoring follows Go idioms; no schema changes |
| II. Comprehensive Testing Discipline | ✅ PASS | This feature IS about improving testing discipline |
| III. Consistent User Experience | ✅ PASS | No user-facing changes |
| IV. Single-Binary Performance | ✅ PASS | No runtime changes |
| V. Simplicity & Minimalism | ✅ PASS | Simplifying test organization, not adding complexity |
| VI. Documentation Synchronization | ✅ PASS | Will update `.claude/rules/testing.md` with new patterns |
| VII. Pre-Existing Issue Resolution | 🟡 MONITOR | May discover issues during audit; will follow protocol |

**Gate Decision**: PROCEED to Phase 0

## Project Structure

### Documentation (this feature)

```text
specs/003-test-suite-audit/
├── plan.md              # This file
├── research.md          # Phase 0 output - research findings
├── data-model.md        # Phase 1 output - test helper types
├── quickstart.md        # Phase 1 output - helper usage guide
├── contracts/           # Phase 1 output - helper function signatures
│   ├── testutil_time.go # Clock interface contract
│   ├── testutil_command.go # Command builder contract
│   └── testutil_home.go # Home directory helper contract
└── tasks.md             # Phase 2 output (created by /speckit.tasks)
```

### Source Code (repository root)

```text
internal/testutil/
├── testutil.go          # EXISTING - core helpers (MustChdir, MustSetenv, etc.)
├── testutil_test.go     # EXISTING - tests for helpers
├── clock.go             # NEW - Clock interface + FakeClock for time mocking
├── clock_test.go        # NEW - Clock tests
├── command.go           # NEW - TestCommand builder (consolidated)
├── command_test.go      # NEW - Command builder tests
└── home.go              # NEW - SetHomeDir helper (consolidated)
                         # (already uses MustSetenv, just platform logic)

pkg/invowkfile/
├── invowkfile_test.go     # REFACTOR → split into:
├── invowkfile_parsing_test.go    # Script parsing, resolution, caching
├── invowkfile_deps_test.go       # Dependency parsing and generation
├── invowkfile_flags_test.go      # Flag validation, mapping
├── invowkfile_args_test.go       # Positional arguments
├── invowkfile_platforms_test.go  # Platform filtering, capabilities
├── invowkfile_env_test.go        # Environment variables, isolation
├── invowkfile_workdir_test.go    # Working directory handling
└── invowkfile_schema_test.go     # Schema validation edge cases

cmd/invowk/
├── cmd_test.go          # REFACTOR → split into:
├── cmd_deps_test.go            # Tool, command, filepath, capability deps
├── cmd_flags_test.go           # Flag handling, environment mapping
├── cmd_args_test.go            # Positional argument validation
├── cmd_runtime_test.go         # Runtime selection, platform checking
└── cmd_source_test.go          # Source filtering, discovery integration

internal/discovery/
├── discovery_test.go    # REFACTOR → split into:
├── discovery_core_test.go      # Basic discovery, command info
├── discovery_modules_test.go   # Module discovery, requirements
└── discovery_collisions_test.go # Collision handling, precedence

internal/runtime/
├── runtime_test.go      # REFACTOR → split into:
├── runtime_native_test.go      # Native shell execution
├── runtime_virtual_test.go     # Virtual shell execution (mvdan/sh)
└── runtime_env_test.go         # Environment and interpreter handling

internal/tui/
├── choose_test.go       # NEW - chooseModel state transitions
├── confirm_test.go      # NEW - confirmModel state transitions
├── input_test.go        # NEW - inputModel state transitions
├── filter_test.go       # NEW - filterModel state transitions
├── table_test.go        # NEW - tableModel state transitions
├── format_test.go       # NEW - Text formatting utilities
├── pager_test.go        # NEW - pagerModel state transitions
├── spin_test.go         # NEW - spinModel state transitions
└── file_test.go         # NEW - fileModel state transitions

internal/container/
├── engine_test.go       # EXISTING - add more mock-based unit tests
└── engine_mock_test.go  # NEW - Mock exec.Command infrastructure

internal/sshserver/
└── server_test.go       # FIX - TestExpiredToken uses time.Sleep()
```

**Structure Decision**: Single project layout matches existing structure. No new directories; only new files within existing packages.

## Complexity Tracking

> **No violations to justify** - This refactoring reduces complexity rather than adding it.

| Aspect | Justification |
|--------|---------------|
| Test file splitting | Reduces cognitive load; each file is single-concern |
| New testutil helpers | Consolidates 3 identical implementations into 1 |
| Clock interface | Standard pattern for deterministic time testing |
| TUI component tests | Tests model/state logic only, not terminal I/O |

---

## Post-Design Constitution Check

*Re-evaluated after Phase 1 design completion.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Idiomatic Go & Schema-Driven Design | ✅ PASS | Clock interface follows stdlib patterns; options pattern is idiomatic |
| II. Comprehensive Testing Discipline | ✅ PASS | This feature directly improves testing discipline |
| III. Consistent User Experience | ✅ PASS | No user-facing changes |
| IV. Single-Binary Performance | ✅ PASS | Test-only changes; no runtime impact |
| V. Simplicity & Minimalism | ✅ PASS | Reducing duplication is simplification |
| VI. Documentation Synchronization | ✅ PASS | Will update `.claude/rules/testing.md`; quickstart.md created |
| VII. Pre-Existing Issue Resolution | ✅ PASS | No blockers discovered; one flaky test (time.Sleep) is part of the fix scope |

**Final Gate Decision**: ✅ APPROVED - Ready for Phase 2 task generation

---

## Artifacts Generated

| Artifact | Path | Purpose |
|----------|------|---------|
| Plan | `specs/003-test-suite-audit/plan.md` | This file |
| Research | `specs/003-test-suite-audit/research.md` | Research findings and decisions |
| Data Model | `specs/003-test-suite-audit/data-model.md` | Type definitions for new helpers |
| Quickstart | `specs/003-test-suite-audit/quickstart.md` | Usage guide for new helpers |
| Clock Contract | `specs/003-test-suite-audit/contracts/testutil_time.go` | Clock interface API |
| Command Contract | `specs/003-test-suite-audit/contracts/testutil_command.go` | Command builder API |
| Home Contract | `specs/003-test-suite-audit/contracts/testutil_home.go` | SetHomeDir helper API |

**Next Step**: Run `/speckit.tasks` to generate implementation tasks.
