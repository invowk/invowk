# Research: Go Package Structure & Organization Audit

**Date**: 2026-01-30
**Branch**: `007-pkg-structure-audit`
**Status**: Complete

## Overview

This document consolidates the research findings from the codebase analysis phase. All "NEEDS CLARIFICATION" items from the Technical Context have been resolved.

---

## Finding 1: Files Exceeding 600 Lines

### Current State

| File | Lines | Priority | Concern Split Strategy |
|------|-------|----------|------------------------|
| `pkg/invowkfile/validation.go` | 753 | High | Split by validation category |
| `pkg/invowkmod/resolver.go` | 726 | High | Split by resolution phase |
| `internal/discovery/discovery.go` | 715 | High | Split by discovery type |
| `cmd/invowk/cmd_execute.go` | 643 | Medium | Split by execution phase |
| `pkg/invowkfile/invowkfile_validation.go` | 631 | Medium | Merge with validation.go, then split |
| `internal/sshserver/server.go` | 627 | Medium | Split by lifecycle phase |

### Files Approaching Threshold (500-600 lines, no action required)

- `cmd/invowk/cmd_discovery.go` (596 lines)
- `internal/tui/filter.go` (566 lines)
- `internal/tui/choose.go` (564 lines)
- `pkg/invowkfile/generate.go` (539 lines)
- `internal/issue/issue.go` (528 lines)
- `internal/tui/embeddable.go` (517 lines)

### Decision

**Split the 6 files exceeding 600 lines.** Files approaching the threshold are left as-is per YAGNI principle (Principle V) - only address actual violations.

### Rationale

- Constitution Principle II requires manageable file sizes for testing
- Spec constraint FR-010 mandates 600-line limit
- Smaller files improve agentic navigation (SC-005)

---

## Finding 2: Code Duplication Patterns

### Pattern A: Container Execute/ExecuteCapture (container_exec.go)

**Location**: `internal/runtime/container_exec.go` lines 16-279

**Duplicated Logic** (95% overlap between `Execute()` and `ExecuteCapture()`):
- Container runtime config retrieval
- Script content resolution
- Image determination with provisioning
- Windows container check
- Environment building
- Host SSH setup
- Volume preparation
- Interpreter resolution
- Working directory determination
- Extra hosts building

**Only Difference**: I/O handling (streaming vs capturing)

**Decision**: Extract `prepareContainerExecution()` helper method

**Rationale**: Reduces 260+ lines of duplication to ~30 lines. Follows DRY without over-abstracting.

---

### Pattern B: DiscoverCommands/DiscoverCommandSet (discovery.go)

**Location**: `internal/discovery/discovery.go` lines 329-474

**Duplicated Logic** (90% overlap):
- Both call `LoadAll()`
- Both create `NewDiscoveredCommandSet()`
- Both iterate with identical filtering
- Both determine `sourceID`/`moduleID` identically
- Both iterate `FlattenCommands()` identically
- Both call `commandSet.Add()` and `commandSet.Analyze()`

**Only Difference**: Return type (`[]*CommandInfo` vs `*DiscoveredCommandSet`)

**Decision**: Refactor `DiscoverCommands()` to call `DiscoverCommandSet()` and extract/sort

**Rationale**: Trivial refactor, eliminates ~70 lines of duplication, improves maintainability.

---

### Pattern C: Clock Interface (testutil vs sshserver)

**Locations**:
- `internal/testutil/clock.go`: Full interface (`Now()`, `After()`, `Since()`)
- `internal/sshserver/server.go`: Minimal interface (`Now()` only)

**Current State**: `testutil.FakeClock` already satisfies `sshserver.Clock` (verified by test usage)

**Decision**: Keep current design - no consolidation needed

**Rationale**:
- sshserver's minimal interface follows Interface Segregation Principle
- testutil.FakeClock works with both interfaces
- Creating a shared clock package adds complexity without benefit
- Current approach already works (tests prove compatibility)

---

### Pattern D: Style Definitions (cmd/invowk/*.go)

**Locations**:
- `cmd/invowk/root.go` (lines 34-48)
- `cmd/invowk/module.go` (lines 21-39)
- `cmd/invowk/config.go` (lines 102-104)
- `cmd/invowk/cmd_discovery.go` (lines 482-535)
- `cmd/invowk/cmd_render.go` (multiple blocks)

**Duplicated Elements**:
| Color | Hex Code | Purpose | Usage Count |
|-------|----------|---------|-------------|
| Purple | `#7C3AED` | Title/Header | 4+ files |
| Gray | `#6B7280` | Subtitle/Muted | 4+ files |
| Green | `#10B981` | Success | 3+ files |
| Red | `#EF4444` | Error | 4+ files |
| Amber | `#F59E0B` | Warning | 2+ files |
| Blue | `#3B82F6` | Command/Highlight | 4+ files |

**Decision**: Create `cmd/invowk/styles.go` with shared color palette and base styles

**Rationale**:
- Eliminates magic strings repeated across 5+ files
- Enables consistent theming
- Derived styles (with margins/padding) can use base styles
- Aligns with Principle III (Consistent User Experience)

---

### Pattern E: Container Engines (docker.go vs podman.go)

**Analysis Result**: Already well-consolidated

Both engines embed `*BaseCLIEngine` which provides all shared logic. Per-engine code is minimal and necessary (version format differences, image inspection commands).

**Decision**: No action required

---

## Finding 3: pkg/ Importing internal/

### Current State

| File | Internal Import | Classification |
|------|-----------------|----------------|
| `pkg/invowkfile/parse.go` | `internal/cueutil` | Production code |
| `pkg/invowkmod/invowkmod.go` | `internal/cueutil` | Production code |
| `pkg/invowkmod/operations_packaging_test.go` | `internal/testutil` | Test code |
| `pkg/invowkmod/resolver_test.go` | `internal/testutil` | Test code |

### Go Visibility Rules

**This is valid Go.** The `internal/` visibility rule applies to **external modules**, not within the same module. Since `pkg/` and `internal/` are both in `invowk-cli` module, they can cross-reference.

### Decision

**Accept current pattern with documentation**. Add explicit note to affected packages' doc.go:

```go
// Package invowkfile provides types and parsing for invowkfile.cue command definitions.
//
// This package uses internal/cueutil for CUE parsing implementation details.
// External consumers should use the exported Parse() and ParseBytes() functions;
// the CUE parsing internals are not part of the public API.
```

### Rationale

- Go allows this; it's a valid design choice
- Hiding CUE parsing implementation is appropriate (not exposing CUE library details)
- External consumers use high-level `Parse()` functions, not CUE internals
- Promotes `cueutil` to `pkg/` would expose unnecessary complexity

### Alternatives Rejected

- **Promote cueutil to pkg/cueutil**: Would expose internal CUE patterns. External consumers don't need to understand the 3-step parse pattern.
- **Duplicate cueutil in each pkg/ package**: Violates DRY, creates maintenance burden.

---

## Finding 4: Package Documentation Coverage

### Packages WITH Documentation

**Has doc.go**:
- `pkg/invowkmod/doc.go` ✅
- `pkg/platform/doc.go` ✅
- `internal/uroot/doc.go` ✅

**Has inline package comment**:
- `cmd/invowk` (root.go) ✅
- `internal/config` (config.go) ✅
- `internal/container` (engine.go) ✅
- `internal/cueutil` (parse.go) ✅
- `internal/discovery` (discovery.go) ✅
- `internal/runtime` (runtime.go) ✅
- `internal/sshserver` (server.go) ✅
- `internal/testutil` (testutil.go) ✅
- `internal/testutil/invowkfiletest` (command.go) ✅

### Packages MISSING Documentation

| Package | Purpose (for doc.go) |
|---------|----------------------|
| `internal/core/serverbase` | Shared server state machine base type |
| `internal/issue` | ActionableError type for user-friendly errors |
| `internal/tui` | Terminal UI components (Bubble Tea models) |
| `internal/tuiserver` | HTTP server for child process TUI requests |
| `pkg/invowkfile` | Invowkfile parsing, types, and validation |

### Decision

**Add doc.go files to all 5 packages** with content as specified below.

### Proposed doc.go Content

**internal/core/serverbase/doc.go**:
```go
// SPDX-License-Identifier: MPL-2.0

// Package serverbase provides a reusable state machine and lifecycle infrastructure
// for long-running server components.
//
// This package extracts common patterns from SSH and TUI servers including:
// atomic state reads, mutex-protected transitions, WaitGroup tracking, and
// context-based cancellation.
package serverbase
```

**internal/issue/doc.go**:
```go
// SPDX-License-Identifier: MPL-2.0

// Package issue provides actionable error handling with user-friendly messages.
//
// This package defines error types that include remediation steps and Markdown-formatted
// guidance, improving the user experience when errors occur during CLI operations.
package issue
```

**internal/tui/doc.go**:
```go
// SPDX-License-Identifier: MPL-2.0

// Package tui provides terminal UI components built on Charm libraries.
//
// This package implements reusable TUI components (choose, confirm, input, filter,
// table, pager, etc.) using Bubble Tea models and huh forms for interactive
// command-line experiences.
package tui
```

**internal/tuiserver/doc.go**:
```go
// SPDX-License-Identifier: MPL-2.0

// Package tuiserver provides an HTTP server for TUI rendering requests from child processes.
//
// When commands run in containers or subprocesses, they can request TUI components
// (choose, confirm, input) via HTTP. The server forwards requests to the parent
// Bubble Tea program for rendering as overlays.
package tuiserver
```

**pkg/invowkfile/doc.go**:
```go
// SPDX-License-Identifier: MPL-2.0

// Package invowkfile provides types and parsing for invowkfile.cue command definitions.
//
// An invowkfile defines commands with implementations for different runtimes (native,
// virtual, container) and platforms. This package handles CUE schema validation,
// parsing to Go structs, and command/implementation selection.
//
// This package uses internal/cueutil for CUE parsing implementation details.
// External consumers should use the exported Parse() and ParseBytes() functions;
// the CUE parsing internals are not part of the public API.
package invowkfile
```

---

## Summary of Decisions

| Finding | Decision | Effort |
|---------|----------|--------|
| 6 files over 600 lines | Split each file | Medium |
| Container execute duplication | Extract helper method | Low |
| Discovery method duplication | Refactor to call common method | Low |
| Clock interface duplication | Keep current design | None |
| Style definitions duplication | Create styles.go | Medium |
| Container engines | No action (already consolidated) | None |
| pkg/ importing internal/ | Accept with documentation | Low |
| 5 packages missing doc.go | Add doc.go files | Low |
