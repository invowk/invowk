# Feature Specification: Go Package Structure & Organization Audit

**Feature Branch**: `007-pkg-structure-audit`
**Created**: 2026-01-30
**Status**: Draft
**Input**: User description: "Identify opportunities for better Go package/directory structure and file organization with great care about semantics, concerns/responsibilities, encapsulation, testability, and optimization for agentic coding."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Agentic Developer Navigates Codebase (Priority: P1)

An AI coding agent (Claude Code, Copilot, etc.) needs to understand and modify the Invowk codebase efficiently. Clear package boundaries, focused files, and predictable organization reduce context consumption and improve accuracy.

**Why this priority**: AI agents are the primary development interface for this project. Poor organization directly impacts development velocity and quality.

**Independent Test**: Can be tested by having an AI agent perform a codebase exploration and measuring context efficiency and accuracy of understanding.

**Acceptance Scenarios**:

1. **Given** an AI agent with no prior codebase knowledge, **When** it explores the package structure, **Then** it can understand each package's purpose within 1 file read per package (doc.go or main file).
2. **Given** an AI agent editing a feature, **When** it needs to find related code, **Then** the dependency graph leads to predictable locations within 2 hops.
3. **Given** a codebase exploration request, **When** files are under 600 lines and focused on single concerns, **Then** the agent consumes less context per modification.

---

### User Story 2 - Developer Understands Package Responsibilities (Priority: P1)

A developer (human or AI) wants to know what each package does without reading implementation details. Clear naming, documentation, and focused responsibilities make this possible.

**Why this priority**: Equal to P1 because understanding and navigation are tightly coupled activities.

**Independent Test**: A developer can correctly describe each package's responsibility by reading only the package name and doc.go comment.

**Acceptance Scenarios**:

1. **Given** a package directory, **When** a developer reads the doc.go file, **Then** they understand the package's single primary responsibility.
2. **Given** two packages with related functionality, **When** comparing their names and docs, **Then** the boundary between them is unambiguous.
3. **Given** a package with multiple concerns, **When** the package is refactored, **Then** each resulting package has a single clear responsibility.

---

### User Story 3 - Contributor Adds New Feature (Priority: P2)

A contributor adding a new feature should know exactly where to place new code based on established patterns and package contracts.

**Why this priority**: Depends on P1 being complete (clear package responsibilities enable confident placement decisions).

**Independent Test**: A contributor can add a new runtime type or TUI component following established patterns without asking where to put the code.

**Acceptance Scenarios**:

1. **Given** the need to add a new execution runtime, **When** examining `internal/runtime/`, **Then** the pattern for adding runtimes is evident from existing code.
2. **Given** the need to add a new TUI component, **When** examining `internal/tui/`, **Then** the file naming and structure pattern is consistent and clear.
3. **Given** shared utility code, **When** deciding between `internal/` packages, **Then** the appropriate home is obvious from package names.

---

### User Story 4 - Test Writer Achieves Coverage (Priority: P2)

A developer writing tests should find test utilities easily and understand the testing patterns used across packages.

**Why this priority**: Good testability follows from good package design; this validates the structural improvements.

**Independent Test**: A developer can write tests for a new package using existing test utilities without duplicating helpers.

**Acceptance Scenarios**:

1. **Given** a new package needing tests, **When** examining `internal/testutil/`, **Then** reusable helpers are discoverable and documented.
2. **Given** two similar packages, **When** comparing their test files, **Then** they follow the same organizational pattern.
3. **Given** integration tests, **When** examining `tests/cli/`, **Then** adding new CLI tests follows an established pattern.

---

### Edge Cases

- **Tightly-coupled multi-concern packages**: When a package has genuinely multiple tightly-coupled concerns (e.g., `discovery` does file discovery + command aggregation), keep them together but document the dual responsibility explicitly in `doc.go` with clear internal file separation. Forcing artificial splits on tightly-coupled concerns creates indirection that harms navigation.
- **Cross-domain bridging code**: Use interface decoupling. The lower-level domain (`invowkmod`) defines interfaces for what it needs from command sources. The higher-level domain (`invowkfile`) implements these interfaces. This avoids circular dependencies and supports future 1:N relationships (e.g., one Module referencing multiple Invowkfiles).
- **Domain logic in utility packages**: Migrate domain-specific logic to the appropriate domain package; keep utility packages (`testutil`, `cueutil`) generic and reusable. Domain-specific helpers belong in the domain they serve, not in shared utilities.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST split files exceeding 600 lines into focused, smaller files with clear single responsibilities.
- **FR-002**: System MUST eliminate code duplication identified in `container_exec.go`, `discovery.go`, and container engine implementations.
- **FR-003**: System MUST consolidate the `Clock` interface duplication between `testutil` and `sshserver` packages.
- **FR-004**: System MUST address style definition duplication between `cmd/invowk/root.go` and `cmd/invowk/module.go`.
- **FR-005**: System MUST evaluate and document whether `pkg/` packages importing `internal/cueutil` is acceptable or requires restructuring.
- **FR-006**: System MUST ensure every package has a `doc.go` file with a clear package-level comment explaining its single responsibility.
- **FR-007**: System MUST maintain the clean layered dependency graph with no circular dependencies.
- **FR-008**: System MUST preserve all existing public API contracts in `pkg/` packages.
- **FR-009**: System MUST maintain or improve test coverage ratios for modified packages.
- **FR-010**: System MUST follow the file size limits defined in `.claude/rules/testing.md` (800 lines max for test files, 600 lines preferred for source files).

### Key Entities

- **Package**: A Go package with a single responsibility, clear dependencies, and appropriate encapsulation.
- **File**: A Go source file focused on a single concern within its package, ideally under 600 lines.
- **Dependency Graph**: The directed acyclic graph of package imports, organized in layers from leaf packages to top-level cmd.
- **Public API**: Exported types, functions, and interfaces in `pkg/` packages that external consumers could use.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: All source files are under 600 lines (current violations: 6 files exceeding this threshold).
- **SC-002**: All test files are under 800 lines (current compliance: achieved).
- **SC-003**: Zero identified code duplication patterns remain (current issues: 4 patterns identified).
- **SC-004**: Every package directory contains a `doc.go` or has package documentation in the primary file.
- **SC-005**: AI agents exploring the codebase can identify each package's purpose from its name and doc.go alone. **Verification**: Post-restructure qualitative checklist:
  - [ ] Agent identifies package purpose from doc.go alone (no implementation reads needed)
  - [ ] Agent finds related code within 2 dependency hops
  - [ ] Agent correctly predicts file location for new feature additions
  - [ ] Agent consumes ≤1 file read per package during exploration
- **SC-006**: No new circular dependencies are introduced; layered dependency structure is maintained.
- **SC-007**: Test coverage ratios remain stable or improve for all modified packages.
- **SC-008**: All existing tests pass after restructuring.

## Constraints & Tradeoffs

### Migration Strategy

**Constraint**: Atomic per-package migrations. Each package restructuring must be a self-contained unit with all tests passing after completion. No cross-package changes in a single migration step.

**Rationale**: Reduces risk, enables git bisect if issues arise, and keeps each change reviewable independently.

### Rejected Alternatives

- **Big-bang restructure**: High risk, hard to review, difficult to bisect failures.
- **Phased by concern**: Leaves packages in intermediate states; harder to reason about.

## Assumptions

1. **No external consumers of pkg/ packages**: The project uses `pkg/` as an organizational convention rather than for external consumption, so `pkg/` importing `internal/` is acceptable if documented.
2. **File splitting preserves behavior**: Refactoring large files into smaller ones maintains identical functionality and public API.
3. **doc.go is the preferred location for package documentation**: Following Go conventions, `doc.go` is the canonical place for package-level comments.
4. **AI agent optimization is a valid design driver**: Structuring code for agentic consumption is an explicit project goal per CLAUDE.md.

## Clarifications

### Session 2026-01-30

- Q: When a package has genuinely multiple tightly-coupled concerns, how should we resolve this? → A: Keep together if coupling is genuine; document dual responsibility explicitly in doc.go with clear internal file separation.
- Q: How should code be organized when it bridges two domains? → A: Use interface decoupling; lower-level domain defines interfaces, higher-level domain implements them. Supports future 1:N relationships without circular dependencies.
- Q: What happens when a utility package grows to have domain-specific logic? → A: Migrate domain-specific logic to the domain package; utilities stay generic and reusable.
- Q: What is the migration strategy for file splits and package restructuring? → A: Atomic per-package migrations; each package migration is a separate testable unit with tests passing after each.
- Q: How should we verify that the restructuring improves agentic navigation? → A: Post-restructure agent test with qualitative checklist based on acceptance scenarios.
