# Feature Specification: CUE Library Usage Optimization

**Feature Branch**: `004-cue-lib-optimization`
**Created**: 2026-01-30
**Status**: Draft
**Input**: User description: "Ensure the way we're using the official CUElang library for Go is the most optimal and robust possible, with minimal manipulation from Invowk's side. As much as possible from schema validation, string extraction etc. must be done by the CUE lib itself when possible and optimal, with the utmost type-safety (all of this must be codified as a rule in @.claude/rules/)."

## Clarifications

### Session 2026-01-30

- Q: When CUE `Decode()` encounters a disjunction with incompatible types, how should the system behave? → A: Refactor to tagged unions—require all disjunctions to use a `_type` discriminator field for clean Go decoding.
- Q: What mechanism should enforce Go struct tag alignment with CUE schema field names? → A: Code generation—generate Go structs from CUE schemas so tags are correct by construction.
- Q: How should the system behave when CUE files approach the 10MB script limit? → A: Fail fast—reject files exceeding a configurable threshold at parse time with a clear error.
- Q: How should validation errors in deeply nested CUE structures be formatted? → A: Path-prefixed—format as `path.to.field: <message>` showing full JSON path to the error.
- Q: How should the codebase handle potential CUE library API changes? → A: Version pinning—pin CUE version in go.mod, document upgrade review process in rules file.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Reliable Schema Validation (Priority: P1)

As a developer extending Invowk's CUE schemas, I want schema validation to be robust and maintainable so that schema changes don't silently break parsing or introduce type mismatches between CUE and Go.

**Why this priority**: Schema validation is the foundation of Invowk's configuration system. If validation is fragile or inconsistent between CUE and Go, users will experience confusing errors or, worse, silent data loss.

**Independent Test**: Can be tested by modifying a CUE schema field and verifying that the corresponding Go struct tag sync tests fail immediately, preventing the mismatch from reaching users.

**Acceptance Scenarios**:

1. **Given** a CUE schema with a field `default_shell`, **When** the corresponding Go struct tag is changed to `defaultShell` (incorrect), **Then** the test suite detects and reports the mismatch before code is merged.
2. **Given** a CUE schema field is renamed from `commands` to `cmds`, **When** invowkfile files using the old field name are parsed, **Then** validation fails with a clear error message mentioning the field name change.
3. **Given** a new field is added to the CUE schema, **When** the Go struct is not updated with the corresponding field, **Then** tests fail with an explicit message about the missing field.

---

### User Story 2 - Minimal Redundant Validation (Priority: P2)

As a maintainer, I want validation logic to live in one authoritative location (preferably CUE) so that I don't need to update multiple places when validation rules change.

**Why this priority**: Redundant validation (same check in CUE and Go) increases maintenance burden and creates inconsistency risks. Consolidating validation reduces code complexity and ensures consistent behavior.

**Independent Test**: Can be tested by searching for validation patterns (regex checks, format validation) and verifying each exists in only one layer (CUE or Go, not both, unless explicitly justified).

**Acceptance Scenarios**:

1. **Given** a validation rule exists in the CUE schema (e.g., env var name regex), **When** reviewing the Go code, **Then** no duplicate validation exists in Go code for the same constraint.
2. **Given** a validation rule cannot be expressed in CUE (e.g., "exactly one of A or B"), **When** the rule is implemented in Go, **Then** a comment explains why Go-level validation is necessary.
3. **Given** redundant validation is identified, **When** it is removed from Go code, **Then** all existing tests continue to pass with only CUE-level validation.

---

### User Story 3 - Type-Safe Value Extraction (Priority: P2)

As a developer, I want value extraction from CUE to use the CUE library's type-safe mechanisms rather than manual string parsing so that type errors are caught early and code is simpler.

**Why this priority**: Manual string extraction from CUE values bypasses type safety and creates parsing bugs. Using CUE's `Decode()` properly ensures type safety.

**Independent Test**: Can be tested by grepping for manual CUE value extraction patterns (e.g., `String()`, `Int64()`) and verifying they're only used when `Decode()` cannot apply.

**Acceptance Scenarios**:

1. **Given** a CUE value needs to be extracted to a Go type, **When** `Decode()` can handle the conversion, **Then** `Decode()` is used instead of manual value extraction methods.
2. **Given** a complex CUE value structure, **When** it is extracted to Go, **Then** the extraction happens in a single `Decode()` call to a well-typed struct, not field-by-field.
3. **Given** manual value extraction (`String()`, `Int64()`) is necessary, **When** reviewing the code, **Then** a comment explains why `Decode()` was not suitable.

---

### User Story 4 - Documented Best Practices (Priority: P3)

As a contributor, I want clear rules about how to use CUE in this codebase so that new code follows established patterns and doesn't introduce anti-patterns.

**Why this priority**: Without documented rules, different contributors will use CUE differently, leading to inconsistent patterns. A rules file in `.claude/rules/` provides guidance for both humans and AI assistants.

**Independent Test**: Can be tested by verifying the rules file exists and covers the key patterns (schema compilation, validation, decoding, when Go validation is acceptable).

**Acceptance Scenarios**:

1. **Given** a contributor wants to add a new CUE-based configuration, **When** they consult `.claude/rules/cue.md`, **Then** they find clear guidance on schema creation, validation patterns, and Go struct alignment.
2. **Given** the rules file exists, **When** a developer submits code with non-idiomatic CUE usage, **Then** code review can point to the specific rule being violated.
3. **Given** a new CUE pattern is discovered that improves the codebase, **When** it is implemented, **Then** the rules file is updated to document the pattern.

---

### Edge Cases

- **Disjunctions with incompatible types**: CUE schemas SHOULD use discriminator fields for disjunctions that decode to different Go types. The existing `name` field pattern (e.g., `RuntimeConfig.Name`) is sufficient; explicit `_type` fields are not required. (Research confirmed current pattern is acceptable.)
- **Large CUE files**: Files exceeding 5MB MUST be rejected at parse time with a clear error message before CUE processing begins, preventing wasted resources. (Configurability deferred to avoid over-engineering; 5MB is sufficient for all reasonable use cases.)
- **Deeply nested validation errors**: Errors MUST be formatted with full JSON path prefix (e.g., `cmds[0].implementations[2].script: value exceeds max length`) to enable direct navigation to the error location.
- **CUE library version changes**: The CUE library version MUST be pinned in `go.mod`. The rules file MUST document an upgrade review process (test coverage verification, API change audit) before version bumps.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: All CUE schema definitions MUST use `close({...})` to reject unknown fields.
- **FR-002**: All Go structs that decode from CUE MUST have JSON tags matching CUE field names exactly (snake_case).
- **FR-003**: Go structs that decode from CUE MUST have schema sync tests verifying JSON tag alignment with CUE field names. (Research showed code generation is not production-ready; sync tests achieve the same goal with simpler tooling.)
- **FR-004**: Validation rules expressible in CUE (type constraints, regex patterns, length limits via `strings.MaxRunes()`, range constraints, enum values, required fields, mutual exclusivity) MUST be defined in CUE schemas, not duplicated in Go code.
- **FR-005**: Go-level validation MUST only exist for constraints that CUE cannot express (with documenting comments).
- **FR-006**: CUE values MUST be extracted using `Decode()` to typed Go structs, not manual field-by-field extraction.
- **FR-007**: Manual value extraction methods (`String()`, `Int64()`, etc.) MUST only be used with explicit justification.
- **FR-008**: A rules file MUST exist at `.claude/rules/cue.md` documenting CUE usage patterns for this project.
- **FR-009**: CUE schemas MUST use defense-in-depth constraints (regex patterns, length limits, range constraints) for all string and numeric fields.
- **FR-010**: CUE error messages MUST include full JSON path prefix (e.g., `cmds[0].script: <error>`) before being returned to users.
- **FR-011**: CUE disjunctions decoding to different Go types SHOULD use discriminator fields. The existing `name` field pattern is sufficient; no additional `_type` fields are required.
- **FR-012**: CUE file parsing MUST reject files exceeding 5MB with a clear error before processing.
- **FR-013**: The rules file MUST document the CUE library upgrade review process (test verification, API change audit).

### Key Entities

- **CUE Schema**: Defines the structure and constraints for configuration files (invowkfile.cue, invowkmod.cue, config.cue).
- **Go Struct**: Target type for CUE decoding; must have JSON tags matching CUE field names.
- **Validation Rule**: A constraint that must be satisfied; lives in CUE schema or Go code (not both).
- **Rules File**: Documentation at `.claude/rules/cue.md` defining CUE usage patterns.
- **Schema Sync Test**: Test that verifies Go struct JSON tags match CUE schema field names using reflection at CI time.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of Go structs decoding from CUE have schema sync tests verifying JSON tag alignment with CUE field names.
- **SC-002**: Zero redundant validation rules exist (same constraint in both CUE and Go) unless explicitly justified with comments.
- **SC-003**: All manual CUE value extraction (`String()`, `Int64()`) has documented justification in adjacent comments.
- **SC-004**: The rules file `.claude/rules/cue.md` covers all 5 key patterns: schema creation, validation, decoding, struct alignment, and exceptions.
- **SC-005**: Build passes with zero warnings related to CUE usage patterns.
- **SC-006**: The rules file `.claude/rules/cue.md` is self-contained: a developer unfamiliar with the codebase can read it and correctly implement a new CUE-based configuration without additional guidance.

## Assumptions

- The CUE library (cuelang.org/go) version is pinned; upgrades require explicit review per the documented upgrade process in `.claude/rules/cue.md`.
- The current three-layer validation approach (CUE schema → Go-level → Tests) is sound and should be preserved where necessary.
- Manual CUE generation (Go → CUE string) is a known limitation of the library and will remain for now.
- Performance of current CUE parsing (~1-3ms per parse) is acceptable and optimization is not a primary goal.
- The existing closed struct pattern (`close({...})`) is the correct approach for strict validation.
