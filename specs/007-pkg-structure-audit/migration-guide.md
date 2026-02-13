# Migration Guide: Go Package Structure & Organization Audit

**Date**: 2026-01-30
**Branch**: `007-pkg-structure-audit`

This document provides the step-by-step sequence for restructuring the codebase. Each phase is atomic and self-contained with all tests passing after completion.

---

## Migration Principles

1. **Atomic per-package migrations**: Each migration step is a self-contained unit
2. **Tests pass after each step**: Never leave the codebase in a broken state
3. **Git bisect friendly**: Each commit represents a complete, working state
4. **Behavior preservation**: No functional changes during restructuring

---

## Phase 1: Documentation (No Code Changes)

Add doc.go files to packages missing documentation. This phase has zero risk of breaking anything.

### Step 1.1: Add doc.go to internal/core/serverbase

```bash
# Create file
cat > internal/core/serverbase/doc.go << 'EOF'
// SPDX-License-Identifier: MPL-2.0

// Package serverbase provides a reusable state machine and lifecycle infrastructure
// for long-running server components.
//
// This package extracts common patterns from SSH and TUI servers including:
// atomic state reads, mutex-protected transitions, WaitGroup tracking, and
// context-based cancellation.
package serverbase
EOF

# Verify
make lint && make test
git add internal/core/serverbase/doc.go
git commit -S -m "docs(serverbase): add doc.go with package documentation"
```

### Step 1.2: Add doc.go to internal/issue

```bash
cat > internal/issue/doc.go << 'EOF'
// SPDX-License-Identifier: MPL-2.0

// Package issue provides actionable error handling with user-friendly messages.
//
// This package defines error types that include remediation steps and Markdown-formatted
// guidance, improving the user experience when errors occur during CLI operations.
package issue
EOF

make lint && make test
git add internal/issue/doc.go
git commit -S -m "docs(issue): add doc.go with package documentation"
```

### Step 1.3: Add doc.go to internal/tui

```bash
cat > internal/tui/doc.go << 'EOF'
// SPDX-License-Identifier: MPL-2.0

// Package tui provides terminal UI components built on Charm libraries.
//
// This package implements reusable TUI components (choose, confirm, input, filter,
// table, pager, etc.) using Bubble Tea models and huh forms for interactive
// command-line experiences.
package tui
EOF

make lint && make test
git add internal/tui/doc.go
git commit -S -m "docs(tui): add doc.go with package documentation"
```

### Step 1.4: Add doc.go to internal/tuiserver

```bash
cat > internal/tuiserver/doc.go << 'EOF'
// SPDX-License-Identifier: MPL-2.0

// Package tuiserver provides an HTTP server for TUI rendering requests from child processes.
//
// When commands run in containers or subprocesses, they can request TUI components
// (choose, confirm, input) via HTTP. The server forwards requests to the parent
// Bubble Tea program for rendering as overlays.
package tuiserver
EOF

make lint && make test
git add internal/tuiserver/doc.go
git commit -S -m "docs(tuiserver): add doc.go with package documentation"
```

### Step 1.5: Add doc.go to pkg/invowkfile

```bash
cat > pkg/invowkfile/doc.go << 'EOF'
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
EOF

make lint && make test
git add pkg/invowkfile/doc.go
git commit -S -m "docs(invowkfile): add doc.go with package documentation"
```

**Phase 1 Verification**:
```bash
make lint && make test && make license-check
```

---

## Phase 2: Style Consolidation

Create shared styles.go to eliminate style definition duplication.

### Step 2.1: Create cmd/invowk/styles.go

Create the shared styles file with color palette and base styles:

```go
// SPDX-License-Identifier: MPL-2.0

package cmd

import "github.com/charmbracelet/lipgloss"

// Color palette constants for consistent CLI output.
const (
    ColorPrimary   = lipgloss.Color("#7C3AED") // Purple - titles/headers
    ColorMuted     = lipgloss.Color("#6B7280") // Gray - subtitles/secondary
    ColorSuccess   = lipgloss.Color("#10B981") // Green - success indicators
    ColorError     = lipgloss.Color("#EF4444") // Red - errors
    ColorWarning   = lipgloss.Color("#F59E0B") // Amber - warnings
    ColorHighlight = lipgloss.Color("#3B82F6") // Blue - commands/highlights
)

// Base styles for CLI output. Use these directly or derive variants with
// additional styling (margins, padding, etc.).
var (
    TitleStyle    = lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary)
    SubtitleStyle = lipgloss.NewStyle().Foreground(ColorMuted)
    SuccessStyle  = lipgloss.NewStyle().Foreground(ColorSuccess)
    ErrorStyle    = lipgloss.NewStyle().Bold(true).Foreground(ColorError)
    WarningStyle  = lipgloss.NewStyle().Foreground(ColorWarning)
    CmdStyle      = lipgloss.NewStyle().Foreground(ColorHighlight)
)
```

### Step 2.2: Update root.go to use shared styles

Replace local style definitions with references to styles.go.

### Step 2.3: Update module.go to use shared styles

Replace local style definitions; derive module-specific variants:
```go
var (
    moduleTitleStyle  = TitleStyle.MarginBottom(1)
    moduleIssueStyle  = ErrorStyle.PaddingLeft(2)
    moduleDetailStyle = SubtitleStyle.PaddingLeft(2)
)
```

### Step 2.4: Update remaining files

Update config.go, cmd_discovery.go, cmd_render.go to use shared styles.

**Phase 2 Verification**:
```bash
make lint && make test
git add cmd/invowk/styles.go
git add cmd/invowk/root.go cmd/invowk/module.go cmd/invowk/config.go
git add cmd/invowk/cmd_discovery.go cmd/invowk/cmd_render.go
git commit -S -m "refactor(cmd): consolidate style definitions into styles.go

- Add styles.go with shared color palette and base styles
- Update root.go, module.go, config.go to use shared styles
- Update cmd_discovery.go, cmd_render.go to use shared styles
- Eliminate magic hex strings across 5+ files"
```

---

## Phase 3: Code Duplication Fixes

### Step 3.1: Refactor discovery.go (DiscoverCommands)

Refactor `DiscoverCommands()` to call `DiscoverCommandSet()`:

```go
func (d *Discovery) DiscoverCommands() ([]*CommandInfo, error) {
    commandSet, err := d.DiscoverCommandSet()
    if err != nil {
        return nil, err
    }

    // Sort and return commands
    sort.Slice(commandSet.Commands, func(i, j int) bool {
        return commandSet.Commands[i].Name < commandSet.Commands[j].Name
    })
    return commandSet.Commands, nil
}
```

**Verification**:
```bash
make lint && make test
git add internal/discovery/discovery.go
git commit -S -m "refactor(discovery): eliminate DiscoverCommands/DiscoverCommandSet duplication

- Refactor DiscoverCommands() to call DiscoverCommandSet()
- Remove ~70 lines of duplicated logic
- Behavior unchanged; tests pass"
```

### Step 3.2: Refactor container_exec.go

Extract shared preparation logic into `prepareContainerExecution()`:

1. Create helper method that returns preparation result struct
2. Update `Execute()` to use helper
3. Update `ExecuteCapture()` to use helper

**Verification**:
```bash
make lint && make test
git add internal/runtime/container_exec.go
git commit -S -m "refactor(runtime): extract prepareContainerExecution helper

- Extract shared preparation logic from Execute/ExecuteCapture
- Reduce ~260 lines of duplication to ~30 lines
- Behavior unchanged; tests pass"
```

---

## Phase 4: File Splits (Largest First)

### Step 4.1: Split pkg/invowkfile/validation.go (753 lines)

Analyze the file to identify logical sections, then split:

1. Read the file to understand its structure
2. Identify validation categories (runtime, deps, structure, etc.)
3. Create new files for each category
4. Move code with all necessary imports
5. Run tests after each move

**Expected Result**:
- `validation.go` (~350 lines) - Core validation orchestration
- `validation_runtime.go` (~200 lines) - Runtime-specific validation
- `validation_deps.go` (~200 lines) - Dependency validation

### Step 4.2: Split pkg/invowkmod/resolver.go (726 lines)

Analyze and split by resolution phase:

1. Read the file to understand its structure
2. Identify resolution phases (load, resolve deps, cache)
3. Create new files for each phase
4. Move code with all necessary imports

**Expected Result**:
- `resolver.go` (~400 lines) - Resolution orchestration
- `resolver_deps.go` (~200 lines) - Dependency resolution
- `resolver_cache.go` (~150 lines) - Cache management

### Step 4.3: Split internal/discovery/discovery.go (715 lines)

Split by discovery type:

**Expected Result**:
- `discovery.go` (~350 lines) - Core discovery orchestration
- `discovery_files.go` (~200 lines) - File-level discovery
- `discovery_commands.go` (~200 lines) - Command aggregation

### Step 4.4: Split cmd/invowk/cmd_execute.go (643 lines)

Split by execution phase:

**Expected Result**:
- `cmd_execute.go` (~350 lines) - Command execution entry point
- `cmd_execute_helpers.go` (~300 lines) - Helper functions

### Step 4.5: Split pkg/invowkfile/invowkfile_validation.go (631 lines)

Evaluate whether to:
- Merge with validation.go then split
- Keep separate with its own split

### Step 4.6: Split internal/sshserver/server.go (627 lines)

Split by lifecycle phase:

**Expected Result**:
- `server.go` (~350 lines) - Core server implementation
- `server_lifecycle.go` (~150 lines) - Start/Stop/Wait methods
- `server_auth.go` (~150 lines) - Authentication and key management

---

## Phase 5: Final Verification

After all migrations complete:

```bash
# Full test suite
make lint
make test
make test-cli
make license-check
make tidy

# Module validation
go run . module validate modules/*.invowkmod --deep

# Verify no files exceed limits
find . -name "*.go" -not -name "*_test.go" -exec wc -l {} \; | sort -rn | head -20
find . -name "*_test.go" -exec wc -l {} \; | sort -rn | head -10

# Verify no circular dependencies
go mod graph | grep invowk-cli

# Verify all packages have documentation
for dir in cmd/invowk internal/* internal/*/* pkg/*; do
    if [ -d "$dir" ] && ls "$dir"/*.go >/dev/null 2>&1; then
        if ! grep -l "^// Package" "$dir"/*.go >/dev/null 2>&1; then
            echo "MISSING DOC: $dir"
        fi
    fi
done
```

---

## Commit Message Templates

### Documentation additions
```
docs(<package>): add doc.go with package documentation
```

### Style consolidation
```
refactor(cmd): consolidate style definitions into styles.go

- Add styles.go with shared color palette and base styles
- Update <files> to use shared styles
- Eliminate magic hex strings across <N> files
```

### Code duplication fixes
```
refactor(<package>): eliminate <pattern> duplication

- <What was done>
- Reduce ~N lines of duplicated logic
- Behavior unchanged; tests pass
```

### File splits
```
refactor(<package>): split <file>.go into focused files

- Extract <concern> into <new-file>.go
- Extract <concern> into <new-file>.go
- Original file now ~N lines (was M lines)
- All files under 600-line limit
```

---

## Rollback Plan

If any phase introduces issues:

1. Identify the failing commit with `git bisect`
2. Revert the specific commit: `git revert <commit>`
3. Document the issue for re-attempt
4. Continue with other phases if possible

Since each phase is atomic, rollback affects only that phase.

---

## Progress Tracking

| Phase | Step | Description | Status |
|-------|------|-------------|--------|
| 1 | 1.1 | doc.go for serverbase | ⬜ |
| 1 | 1.2 | doc.go for issue | ⬜ |
| 1 | 1.3 | doc.go for tui | ⬜ |
| 1 | 1.4 | doc.go for tuiserver | ⬜ |
| 1 | 1.5 | doc.go for invowkfile | ⬜ |
| 2 | 2.1 | Create styles.go | ⬜ |
| 2 | 2.2 | Update root.go | ⬜ |
| 2 | 2.3 | Update module.go | ⬜ |
| 2 | 2.4 | Update remaining files | ⬜ |
| 3 | 3.1 | Refactor discovery.go | ⬜ |
| 3 | 3.2 | Refactor container_exec.go | ⬜ |
| 4 | 4.1 | Split validation.go | ⬜ |
| 4 | 4.2 | Split resolver.go | ⬜ |
| 4 | 4.3 | Split discovery.go | ⬜ |
| 4 | 4.4 | Split cmd_execute.go | ⬜ |
| 4 | 4.5 | Split invowkfile_validation.go | ⬜ |
| 4 | 4.6 | Split server.go | ⬜ |
| 5 | - | Final verification | ⬜ |
