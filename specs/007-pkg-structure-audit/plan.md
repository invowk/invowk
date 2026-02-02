# Implementation Plan: Go Package Structure & Organization Audit

**Branch**: `007-pkg-structure-audit` | **Date**: 2026-01-30 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/007-pkg-structure-audit/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/commands/plan.md` for the execution workflow.

## Summary

Audit and restructure the Go codebase for better package organization, file size compliance, and agentic coding optimization. Key deliverables: split 6 files exceeding 600 lines (including separate analysis for invkfile_validation.go per research.md—determine whether to merge with validation.go or split standalone), eliminate 4 identified code duplication patterns, add doc.go files to 5 packages, and consolidate shared utilities (styles, clock interface).

## Technical Context

**Language/Version**: Go 1.25+ with CUE v0.15.3
**Primary Dependencies**: Cobra, Viper, Bubble Tea, Lip Gloss, mvdan/sh
**Storage**: N/A (file-based CUE configuration only)
**Testing**: Go stdlib `testing` + `testscript` for CLI integration tests
**Target Platform**: Linux, macOS, Windows (cross-platform CLI)
**Project Type**: Single CLI application with modular internal packages
**Performance Goals**: <100ms startup for `invowk --help`
**Constraints**: Single-binary distribution, no external runtime dependencies
**Scale/Scope**: ~15 internal packages, ~3 pkg packages, ~25k LOC

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Idiomatic Go & Schema-Driven Design | ✅ PASS | Refactoring preserves error handling, declaration order, SPDX headers |
| II. Comprehensive Testing Discipline | ✅ PASS | Each package migration keeps tests passing; coverage preserved |
| III. Consistent User Experience | ✅ PASS | No CLI behavior changes; internal restructuring only |
| IV. Single-Binary Performance | ✅ PASS | No performance impact from file/package reorganization |
| V. Simplicity & Minimalism | ✅ PASS | Reducing duplication and improving organization aligns with simplicity |
| VI. Documentation Synchronization | ✅ PASS | Adding doc.go files improves documentation; no user-facing docs needed |
| VII. Pre-Existing Issue Resolution | ✅ PASS | This audit IS the resolution of pre-existing structural issues |

## Project Structure

### Documentation (this feature)

```text
specs/007-pkg-structure-audit/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output - current state analysis and decisions
├── package-map.md       # Phase 1 output - package responsibilities and boundaries
├── migration-guide.md   # Phase 1 output - step-by-step restructuring sequence
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (existing structure - no new directories)

```text
cmd/invowk/              # CLI commands - add styles.go for shared styles
internal/
├── config/              # Configuration management
├── container/           # Container engine abstraction (already consolidated)
├── core/serverbase/     # ADD: doc.go
├── cueutil/             # CUE parsing utilities
├── discovery/           # Module/command discovery - split discovery.go
├── issue/               # ADD: doc.go
├── runtime/             # Execution runtimes - refactor container_exec.go
├── sshserver/           # SSH server
├── testutil/            # Test utilities (keep Clock here)
├── tui/                 # ADD: doc.go
└── tuiserver/           # ADD: doc.go
pkg/
├── invkfile/            # ADD: doc.go, split validation.go into validation_*.go files
│                        # NOTE: invkfile_validation.go handled separately in Phase 4B
│                        # (analyze whether to merge with validation.go or split standalone)
├── invkmod/             # Module types (has doc.go), split resolver.go
└── platform/            # Platform detection (has doc.go)
tests/cli/               # CLI integration tests
```

**Structure Decision**: This is a refactoring audit; no new top-level directories. Changes are:
1. File splits within existing packages (validation.go, resolver.go, discovery.go, etc.)
2. New doc.go files in 5 packages
3. New styles.go in cmd/invowk for shared style definitions
4. Refactored helper methods to eliminate duplication

## Post-Design Constitution Re-Check

*Re-evaluated after Phase 1 design artifacts completed.*

| Principle | Status | Verification |
|-----------|--------|--------------|
| I. Idiomatic Go | ✅ PASS | All doc.go files include SPDX headers; file splits preserve declaration order |
| II. Testing | ✅ PASS | Migration guide mandates `make test` after each step |
| III. User Experience | ✅ PASS | styles.go consolidation improves consistency (Principle III alignment) |
| IV. Performance | ✅ PASS | No new dependencies; file reorganization has zero runtime impact |
| V. Simplicity | ✅ PASS | Eliminating 4 duplication patterns reduces maintenance complexity |
| VI. Documentation | ✅ PASS | Adding 5 doc.go files + package-map.md improves codebase documentation |
| VII. Pre-Existing Issues | ✅ PASS | This entire audit resolves pre-existing structural issues identified in spec |

**All gates passed. Ready for task generation.**

## Complexity Tracking

> **No Constitution violations identified. This section remains empty.**

N/A - All principles satisfied.
