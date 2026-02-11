# Implementation Plan: Module-Aware Command Discovery

**Branch**: `001-module-cmd-discovery` | **Date**: 2026-01-21 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/001-module-cmd-discovery/spec.md`

## Summary

Extend `invowk cmd` to discover and list commands from multiple sources: the root `invkfile.cue` AND all first-level `*.invkmod` directories in the current directory (excluding module dependencies). Introduce a canonical namespace system that remains transparent when command names are unique but enables disambiguation via `@<source>` prefix or `--from <source>` flag when conflicts occur.

**Key Changes:**
1. Modify discovery to aggregate commands from invkfile + sibling modules
2. Add `CommandSource` tracking to identify command origin
3. Detect and report ambiguous command names
4. Add `@source` and `--from` disambiguation syntax to CLI parsing
5. Update command listing to group by source with headers

## Technical Context

**Language/Version**: Go 1.26+
**Primary Dependencies**:
- `github.com/spf13/cobra` - CLI framework
- `cuelang.org/go` - CUE parsing for invkfile/invkmod
- `github.com/charmbracelet/lipgloss` - Styled terminal output
**Storage**: Filesystem (invkfile.cue, *.invkmod directories)
**Testing**:
- `go test` with table-driven unit tests
- `testscript` (.txtar) for CLI integration tests
**Target Platform**: Linux, macOS, Windows (cross-platform CLI)
**Project Type**: Single CLI binary
**Performance Goals**: List commands from invkfile + 10 modules in <2 seconds (SC-001)
**Constraints**: Backward compatible with existing single-invkfile workflows
**Scale/Scope**: Typical workspace: 0-1 invkfile + 0-10 modules

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Idiomatic Go | PASS | Will follow existing patterns in `internal/discovery/` |
| II. Testing Discipline | PASS | Will add unit tests + testscript CLI tests |
| III. Consistent UX | PASS | New `@source` syntax follows existing flag conventions |
| IV. Single-Binary Performance | PASS | Discovery already lazy-loads; no new external deps |
| V. Simplicity | PASS | Minimal new abstractions; extends existing types |
| VI. Documentation Sync | PENDING | Will update README + help text after implementation |
| VII. Pre-Existing Issues | PENDING | Will halt if blocking issues discovered |

**Quality Gates (pre-merge):**
- `make lint` - linting passes
- `make test` - unit tests pass
- `make test-cli` - CLI integration tests pass (new tests added)
- `make license-check` - SPDX headers on new files

## Project Structure

### Documentation (this feature)

```text
specs/001-module-cmd-discovery/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
└── tasks.md             # Phase 2 output (/speckit.tasks)
```

### Source Code (repository root)

```text
cmd/invowk/
├── cmd.go               # MODIFY: registerDiscoveredCommands(), listCommands(), runCommandWithFlags()
├── cmd_flags.go         # MODIFY: Add --from flag parsing
└── root.go              # Reference only

internal/discovery/
├── discovery.go         # MODIFY: DiscoverCommands() to aggregate sources, track CommandSource
├── validation.go        # MODIFY: Add ambiguity detection
└── types.go             # NEW: CommandSource, DiscoveredCommandSet types

pkg/invkfile/
├── invkfile.go          # Reference: Invkfile, Command types
└── parse.go             # Reference: parsing logic

pkg/invkmod/
├── invkmod.go           # Reference: Module, Invkmod types
└── operations.go        # Reference: Validate(), reserved name check addition

tests/cli/testdata/
├── multi_source.txtar   # NEW: Multi-source discovery tests
├── disambiguation.txtar # NEW: @source and --from tests
└── ambiguity.txtar      # NEW: Conflict detection tests
```

**Structure Decision**: Existing Go CLI project structure. Changes primarily in `internal/discovery/` for core logic and `cmd/invowk/` for CLI integration. New types added to discovery package to track command sources.

## Complexity Tracking

No constitution violations requiring justification. The implementation extends existing patterns without introducing new abstractions beyond what's necessary for source tracking.
