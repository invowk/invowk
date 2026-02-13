# Feature Specification: Module-Aware Command Discovery

**Feature Branch**: `001-module-cmd-discovery`
**Created**: 2026-01-21
**Status**: Draft
**Input**: User description: "Add wider and deeper cmd discovery that discovers cmds from all Modules (but NOT Modules' dependencies) and the invowkfile in a directory, with canonical namespace representation for ambiguity resolution"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Discover Commands from Multiple Sources (Priority: P1)

As a user working in a directory that contains both an `invowkfile.cue` and one or more `<module>.invowkmod` directories, I want `invowk cmd` to list all available commands from both the invowkfile and all modules (excluding their transitive dependencies), so that I can see and use all commands available in my workspace.

**Why this priority**: This is the core feature - users need to be able to discover commands from multiple sources without manually navigating to each module. Without this, the feature has no value.

**Independent Test**: Can be fully tested by creating a directory with an invowkfile.cue and two module directories, running `invowk cmd`, and verifying all commands from all three sources appear in the output.

**Acceptance Scenarios**:

1. **Given** a directory with `invowkfile.cue` containing cmd "hello" and `foo.invowkmod` containing cmd "greet", **When** I run `invowk cmd`, **Then** I see both "hello" and "greet" listed.
2. **Given** a directory with only `bar.invowkmod` containing cmds "build" and "test", **When** I run `invowk cmd`, **Then** I see both "build" and "test" listed.
3. **Given** a directory with `invowkfile.cue` and no modules, **When** I run `invowk cmd`, **Then** behavior remains unchanged from current implementation (only invowkfile cmds shown).
4. **Given** `foo.invowkmod` requires `bar.invowkmod` as a dependency, **When** I run `invowk cmd` from the directory containing `foo.invowkmod`, **Then** I see only cmds from `foo.invowkmod`, NOT cmds from `bar.invowkmod`.

---

### User Story 2 - Transparent Namespace for Unambiguous Commands (Priority: P1)

As a user, I want to execute commands using simple names (e.g., `invowk cmd hello`) when there is no ambiguity, so that my workflow remains simple and unchanged from the current behavior.

**Why this priority**: Equal priority with P1 because users should not need to learn or use canonical namespaces unless absolutely necessary. This preserves backward compatibility and user experience.

**Independent Test**: Can be tested by creating a setup with unique command names across all sources and verifying that simple command invocation works.

**Acceptance Scenarios**:

1. **Given** cmd "hello" exists only in `invowkfile.cue`, **When** I run `invowk cmd hello`, **Then** the command executes without requiring the canonical namespace.
2. **Given** cmd "greet" exists only in `foo.invowkmod`, **When** I run `invowk cmd greet`, **Then** the command executes without requiring the canonical namespace.
3. **Given** all command names are unique across all sources, **When** I run `invowk cmd`, **Then** the listing shows simplified names (current format) without canonical namespace prefixes.

---

### User Story 3 - Canonical Namespace for Ambiguous Commands (Priority: P2)

As a user, when two or more commands share the same name across different sources (invowkfile or modules), I want `invowk cmd` to clearly show me which source each command comes from and require me to specify the source when executing, so that I can unambiguously select the correct command.

**Why this priority**: This is the conflict resolution mechanism. Less common than unique commands but essential when conflicts occur.

**Independent Test**: Can be tested by creating a setup with duplicate command names and verifying the listing format changes and execution requires disambiguation.

**User-Facing Disambiguation Syntax**:

Users can disambiguate using two equivalent syntaxes:

1. **@ prefix notation**: `invowk cmd @<source> <cmd-name>`
   - For modules: `invowk cmd @foo deploy` or `invowk cmd @foo.invowkmod deploy`
   - For root invowkfile: `invowk cmd @invowkfile deploy` or `invowk cmd @invowkfile.cue deploy`

2. **--from flag**: `invowk cmd --from <source> <cmd-name>` (flag must appear before command name)
   - For modules: `invowk cmd --from foo deploy` or `invowk cmd --from foo.invowkmod deploy`
   - For root invowkfile: `invowk cmd --from invowkfile deploy` or `invowk cmd --from invowkfile.cue deploy`

**Acceptance Scenarios**:

1. **Given** cmd "deploy" exists in both `invowkfile.cue` and `foo.invowkmod`, **When** I run `invowk cmd`, **Then** both commands are listed with their source annotation (e.g., `deploy (@foo)` and `deploy (@invowkfile)`).
2. **Given** cmd "deploy" exists in both `invowkfile.cue` and `foo.invowkmod`, **When** I run `invowk cmd deploy`, **Then** I receive an error message listing the ambiguous options with their disambiguation syntax.
3. **Given** cmd "deploy" exists in both `invowkfile.cue` and `foo.invowkmod`, **When** I run `invowk cmd @foo deploy`, **Then** the module's deploy command executes.
4. **Given** cmd "deploy" exists in both `invowkfile.cue` and `foo.invowkmod`, **When** I run `invowk cmd --from foo deploy`, **Then** the module's deploy command executes.
5. **Given** cmd "deploy" exists in both `invowkfile.cue` and `foo.invowkmod`, **When** I run `invowk cmd @invowkfile deploy`, **Then** the invowkfile's deploy command executes.
6. **Given** cmd "deploy" exists in both `invowkfile.cue` and `foo.invowkmod`, **When** I run `invowk cmd --from invowkfile deploy`, **Then** the invowkfile's deploy command executes.

---

### User Story 4 - Subcommand Ambiguity Handling (Priority: P2)

As a user with hierarchical commands (commands with subcommands), when a parent command or subcommand name is ambiguous, I want the canonical namespace to resolve the ambiguity at the appropriate level, so that I can navigate command hierarchies from multiple sources.

**Why this priority**: Hierarchical commands are common in real-world usage; ambiguity at any level in the hierarchy must be handled.

**Independent Test**: Can be tested by creating commands with shared subcommand paths (e.g., `deploy staging` in both sources) and verifying disambiguation works.

**Acceptance Scenarios**:

1. **Given** `invowkfile.cue` has "deploy staging" and `foo.invowkmod` has "deploy prod", **When** I run `invowk cmd deploy staging`, **Then** the command executes unambiguously (staging is unique).
2. **Given** both `invowkfile.cue` and `foo.invowkmod` have "deploy staging", **When** I run `invowk cmd deploy staging`, **Then** I receive an error with canonical namespaces for disambiguation.

---

### Edge Cases

- What happens when a module has an invalid `invowkmod.cue` or `invowkfile.cue`? The module should be skipped with a warning, and valid sources should still be processed.
- What happens when the directory contains zero invowkfiles and zero valid modules? Display an appropriate message indicating no commands are available.
- What happens when canonical namespace input contains typos or references non-existent modules? Display a clear error message suggesting valid canonical namespaces.
- How are nested modules (modules inside module directories) handled? Following existing behavior, nested modules are detected but not automatically included in parent discovery - only first-level `.invowkmod` directories in the target directory are scanned.
- What if a module is named `invowkfile.invowkmod`? The module is rejected with a warning since `invowkfile` is a reserved source name for disambiguation.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST discover commands from `invowkfile.cue` in the target directory (if present and valid).
- **FR-002**: System MUST discover commands from all first-level `<module>.invowkmod` directories in the target directory (if present and valid).
- **FR-003**: System MUST NOT discover commands from modules' dependencies (transitive dependency commands are excluded).
- **FR-004**: System MUST maintain an internal canonical namespace representation in the format: `<module-id | null>:<invowkfile-path>:<cmd-name>`.
- **FR-005**: System MUST display simplified command names (without canonical namespace) when all command names are unique across all sources.
- **FR-006**: System MUST display canonical namespace prefixes in the command listing when command name conflicts exist.
- **FR-007**: System MUST allow execution of unambiguous commands using simplified names only.
- **FR-008**: System MUST reject execution of ambiguous commands with simplified names and display available canonical namespace options.
- **FR-009**: System MUST allow execution of ambiguous commands when the user provides disambiguation via `@<source>` prefix notation.
- **FR-009a**: System MUST allow execution of ambiguous commands when the user provides disambiguation via `--from <source>` flag, which must appear immediately after `invowk cmd` and before the command name.
- **FR-009b**: System MUST accept both short source names (e.g., `foo`, `invowkfile`) and full names (e.g., `foo.invowkmod`, `invowkfile.cue`) for disambiguation.
- **FR-009c**: System MUST allow explicit source disambiguation (`@source` or `--from`) even for unambiguous commands, executing the command normally without warning or error.
- **FR-010**: System MUST gracefully handle invalid modules by logging a warning and continuing discovery from valid sources.
- **FR-011**: System MUST preserve the current `invowk cmd` behavior when only an `invowkfile.cue` exists (backward compatibility).
- **FR-012**: System MUST output warnings to stderr by default for invalid modules and ambiguity errors during discovery.
- **FR-013**: System MUST support a verbose mode (via existing `--verbose` flag or equivalent) that outputs discovery source details (which invowkfile/modules were scanned, command counts per source).
- **FR-014**: System MUST display command listings grouped by source with section headers, showing invowkfile commands first, then module commands in alphabetical order by module name.
- **FR-015**: System MUST reject modules named `invowkfile.invowkmod` with a warning, as `invowkfile` is a reserved source name for disambiguation.

### Key Entities

- **Command Source**: Represents where a command originates from - either the root invowkfile or a specific module. Contains: source type (invowkfile|module), module ID (if applicable), invowkfile path.
- **Canonical Namespace**: The fully-qualified internal identifier for a command that uniquely identifies it across all sources. User-facing disambiguation uses `@<source>` prefix or `--from <source>` flag, where source is the module short name (e.g., `foo`) or `invowkfile` for the root invowkfile.
- **Discovered Command**: A command along with its source information and canonical namespace, used internally to track and resolve commands.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can list commands from an invowkfile and up to 10 modules in under 2 seconds on standard hardware.
- **SC-002**: 100% of unique command names execute successfully using simplified names without canonical namespace.
- **SC-003**: 100% of ambiguous command names are detected and reported with clear disambiguation options.
- **SC-004**: Existing workflows using only `invowkfile.cue` continue to work identically (zero regressions).
- **SC-005**: Users can disambiguate and execute any conflicting command using canonical namespace in a single additional attempt.

## Clarifications

### Session 2026-01-21

- Q: What happens when user uses explicit source disambiguation (`@source` or `--from`) for a command that is NOT ambiguous? → A: Allow explicit source specification for any command, even when unambiguous (execute normally).
- Q: What verbosity/observability approach should discovery use? → A: Warnings by default (invalid modules, ambiguity errors), with verbose flag available for debugging discovery sources.
- Q: How should commands be ordered/grouped when listed from multiple sources? → A: Group by source with headers (invowkfile first, then modules alphabetically).
- Q: Where must `--from` flag appear when the command has arguments? → A: `--from` must appear immediately after `invowk cmd` (before command name).
- Q: What if a user creates a module named `invowkfile.invowkmod` (conflicts with reserved source name)? → A: Reject/warn about `invowkfile.invowkmod` module name during discovery (reserved name).

## Assumptions

- Module validation uses existing `invowk module validate` logic for determining module validity.
- The `@` character does not appear as the first character of valid command names (reserved for source disambiguation).
- First-level modules means `.invowkmod` directories directly inside the target directory, not recursively nested.
- The discovery order (invowkfile first, then modules alphabetically) is implementation-defined but does not affect user-facing behavior since conflicts show all options.
- Short source names (without `.invowkmod` or `.cue` extension) are unambiguous within a single directory context.
