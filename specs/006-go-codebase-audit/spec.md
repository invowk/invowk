# Feature Specification: Go Codebase Quality Audit

**Feature Branch**: `006-go-codebase-audit`
**Created**: 2026-01-30
**Status**: Draft
**Input**: User description: "Review entire Go codebase for abstractions, file organization, correctness, CUE handling, and error messages"

## Clarifications

### Session 2026-01-30

- Q: When splitting large files introduces potential import cycles, which resolution strategy should be applied? → A: Create a new `internal/core` or `internal/shared` package for extracted types.
- Q: For extracting shared patterns (server state machine, container engine logic), which approach should be used? → A: Composition via struct embedding, with functional options pattern for constructors and non-mandatory configuration.
- Q: What error context elements should be mandatory in all user-facing error messages? → A: Operation + resource + suggestion by default; full error chain added with --verbose.
- Q: When adding new CUE schema constraints, how should backward compatibility be handled? → A: Strict enforcement immediately; invalid existing configs must be fixed.
- Q: What is the minimum test coverage threshold for modified/extracted code? → A: No numeric threshold; require tests for all public APIs and critical paths.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Improved Code Maintainability for Contributors (Priority: P1)

A contributor wants to make changes to the Invowk codebase. After the audit improvements are applied, they can navigate files more easily because large monolithic files (1000+ lines) have been split into focused, single-responsibility files under 600 lines. They can understand the codebase architecture faster because common patterns are extracted into reusable abstractions.

**Why this priority**: Code maintainability directly impacts development velocity and contributor onboarding. Large files are difficult for both humans and AI coding agents to reason about effectively.

**Independent Test**: Can be tested by measuring average file size, counting files over 600 lines, and verifying that identified duplication patterns have been consolidated.

**Acceptance Scenarios**:

1. **Given** a file currently exceeds 800 lines, **When** the audit improvements are applied, **Then** the file is split into multiple focused files each under 600 lines while preserving all functionality and test coverage.
2. **Given** duplicate code patterns exist across multiple files (e.g., server state machine, container engine methods), **When** improvements are applied, **Then** shared abstractions exist and individual implementations only contain their unique logic.
3. **Given** a new contributor wants to understand runtime execution, **When** they read the runtime package, **Then** they find clear separation between common orchestration and runtime-specific implementations.

---

### User Story 2 - Enhanced CUE Schema Validation (Priority: P2)

A user writes an `invkfile.cue` with configuration errors. After the audit improvements, they receive earlier and clearer validation errors because additional constraints have been moved from Go validation to CUE schemas, providing immediate feedback during parsing rather than delayed runtime errors.

**Why this priority**: Moving validation to CUE schemas provides defense-in-depth and catches errors earlier in the user workflow. This improves user experience without requiring runtime execution.

**Independent Test**: Can be tested by verifying that CUE schemas contain the specified constraints and that malformed inputs are rejected at parse time.

**Acceptance Scenarios**:

1. **Given** a user provides an excessively long `image` field (over 512 characters), **When** they parse the invkfile, **Then** CUE validation fails with a clear length error before any Go code runs.
2. **Given** a user provides an empty description field, **When** they parse the invkfile, **Then** CUE validation rejects it with the same pattern applied to flag/arg descriptions.
3. **Given** CUE schemas have new constraints, **When** schema sync tests run, **Then** all constraints are verified to match corresponding Go struct expectations.

---

### User Story 3 - Actionable Error Messages (Priority: P3)

A user encounters an error while running a command. After the audit improvements, error messages consistently include context about what operation failed, which file or resource was involved, and suggestions for how to resolve the issue.

**Why this priority**: Users currently receive inconsistent error messages - some are detailed and helpful while others are generic "failed to X" messages. Standardized, actionable errors reduce support burden and improve user autonomy.

**Independent Test**: Can be tested by triggering known error conditions and verifying error messages contain required context elements.

**Acceptance Scenarios**:

1. **Given** configuration loading fails, **When** the error is displayed, **Then** users see the error regardless of verbose mode (not silently ignored).
2. **Given** a shell is not found during native execution, **When** the error is displayed without --verbose, **Then** the message includes operation ("find shell"), resource (list of attempted shells), and suggestion (installation hints). **When** displayed with --verbose, **Then** the full error chain is also shown.
3. **Given** a container image build fails, **When** the error is displayed, **Then** the message includes operation ("build container"), resource (image/containerfile path), and suggestion (check Dockerfile syntax or path).

---

### User Story 4 - Reduced Code Duplication (Priority: P2)

A maintainer needs to fix a bug in the server state machine logic. After the audit improvements, they fix it in one place (the shared base implementation) rather than having to apply the same fix to both SSH server and TUI server separately.

**Why this priority**: Code duplication creates maintenance burden and risks introducing inconsistencies when fixes are applied to one location but forgotten in another.

**Independent Test**: Can be tested by identifying shared patterns and verifying single implementations exist.

**Acceptance Scenarios**:

1. **Given** SSH server and TUI server share identical state machine patterns, **When** improvements are applied, **Then** a shared base server type or mixin exists that both servers use.
2. **Given** Docker and Podman engines have 95% identical code, **When** improvements are applied, **Then** a base CLI engine implementation exists with engine-specific overrides only for differences (version format, SELinux labels).
3. **Given** the CUE parsing 3-step flow is repeated across invkfile, invkmod, and config packages, **When** improvements are applied, **Then** a shared CUE parser utility exists.

---

### Edge Cases

- What happens when splitting a large file introduces import cycles? Create a new `internal/core` or `internal/shared` package for extracted types that multiple packages need to import, breaking the cycle by centralizing shared dependencies.
- How does extracting abstractions affect test coverage? All extracted code must maintain or improve test coverage, with sync tests verifying interface contracts.
- What if a CUE schema change is not backward compatible? Constraints are enforced strictly and immediately. Existing configurations that violate new constraints must be fixed by users. Clear error messages will indicate the constraint violated and the valid range.

## Requirements *(mandatory)*

### Functional Requirements

#### File Organization

- **FR-001**: System MUST split files exceeding 800 lines into focused files under 600 lines each, organized by logical concern (e.g., `module.go` split into `module_validate.go`, `module_create.go`, `module_alias.go`, etc.)
- **FR-002**: System MUST maintain all existing functionality and test coverage after file splits
- **FR-003**: System MUST update any import statements or references affected by file reorganization
- **FR-003a**: System MUST resolve import cycles introduced by file splits by extracting shared types to a new `internal/core` or `internal/shared` package

#### Code Abstraction

- **FR-004**: System MUST extract the server state machine pattern (Created → Starting → Running → Stopping → Stopped/Failed) into a reusable base type using struct embedding that SSH server and TUI server can compose
- **FR-005**: System MUST extract common container engine logic (Build, Run argument construction) into a base CLI engine type using struct embedding, with Docker and Podman embedding it and providing only their unique variations
- **FR-006**: System MUST extract the CUE 3-step parsing pattern (compile schema → unify → decode) into a reusable utility function or type
- **FR-007**: System MUST extract environment building logic into smaller, composable functions for each precedence level
- **FR-007a**: System MUST use the functional options pattern for constructors of extracted types, allowing sensible defaults with optional configuration via `With*` functions

#### CUE Schema Enhancement

- **FR-008**: System MUST add `strings.MaxRunes()` length constraints to CUE schemas for: `default_shell` (1024), `description` (10240), `image` (512), `interpreter` (1024), `default_value` for flags/args (4096)
- **FR-009**: System MUST add non-empty-with-content validation to all `description` fields (including `#Command.description`, `#Flag.description`, `#Argument.description`) using the same pattern as flag/arg descriptions
- **FR-010**: System MUST maintain all existing schema sync tests and add tests for new constraints

#### Error Handling

- **FR-011**: System MUST surface configuration loading errors regardless of verbose mode
- **FR-012**: System MUST include in all user-facing errors by default: (1) operation that failed, (2) resource/file involved, (3) actionable suggestion for resolution
- **FR-013**: System MUST include the full wrapped error chain when --verbose is enabled, in addition to the default context
- **FR-014**: System MUST use the existing `issue.Issue` system consistently for CLI error rendering

#### Quality Assurance

- **FR-015**: System MUST pass all existing tests after changes
- **FR-016**: System MUST pass linting (`make lint`) after changes
- **FR-017**: System MUST have tests for all public APIs and critical paths in modified/extracted code (no numeric coverage threshold)
- **FR-018**: System MUST update documentation sync map entries if file structure changes affect documented paths

### Key Entities

- **File**: A Go source file with line count, package membership, and logical concerns
- **Abstraction**: A reusable type, interface, or function that consolidates duplicate logic
- **CUE Constraint**: A validation rule in CUE schema (length limit, regex pattern, required field)
- **Error Context**: Metadata attached to errors (operation, resource, suggestions)

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: No production Go file exceeds 700 lines after the audit (reduced from current max of 1,118 lines)
- **SC-002**: The number of files over 500 lines is reduced by at least 40% (from current 62 files over 400 lines)
- **SC-003**: Duplicate code patterns (server state machine, container engine methods) are consolidated, reducing total lines in affected packages by at least 15%
- **SC-004**: All CUE schemas include length constraints on string fields that previously had none
- **SC-005**: 100% of user-facing error messages include context about which operation failed
- **SC-006**: All existing tests continue to pass after changes
- **SC-007**: Code contributors can locate functionality faster due to focused, well-named files
