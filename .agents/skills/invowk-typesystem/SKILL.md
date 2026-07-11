---
name: invowk-typesystem
description: Invowk type-system and goplint guidance for value types across cmd/*, internal/*, and pkg/*, including Validate() contracts, primitive-wrapper value objects, aliases/re-exports, sentinel errors, Invalid*Error wrappers, check-types/check-types-all findings, DDD compliance, and baseline updates. Use when adding, reviewing, refactoring, or documenting value types.
---

# Invowk Typesystem

Use this skill when:
- Creating or refactoring value types in `cmd/*`, `pkg/*`, or `internal/*`
- Debugging `Validate()` behavior, sentinel error wrapping, or `Invalid*Error` shapes
- Resolving goplint findings related to primitive wrappers, DDD compliance, and type safety
- Documenting type-system changes for agent guidance

---

## Mandatory Workflow

1. Read `references/value-type-patterns.md` to apply canonical design rules.
2. Check `references/type-catalog.md` before introducing a new type.
3. If a new type is still needed, follow the creation checklist in `references/maintenance-workflow.md`.
4. Refresh the catalog with `scripts/extract_value_types.sh` and reconcile the docs.
5. Run the required checks listed below.

The extractor's repository-wide Go surface is `cmd/`, `internal/`, and `pkg/`.
Keep all three roots aligned whenever its detection logic changes.

---

## Canonical Value-Type Contract

Invowk value types should follow this contract unless a domain-specific exception is justified:

- A dedicated type (prefer primitive wrapper where possible) with domain intent in the name.
- A sentinel error variable, usually `ErrInvalid<Type>`.
- A typed error struct, usually `Invalid<Type>Error`, that wraps the sentinel via `Unwrap()`.
- `Validate() error` for programmatic validation.
- `String()` for wrappers where string rendering is used operationally.

Reference implementation patterns are in `references/value-type-patterns.md`.

---

## References

- `references/type-catalog.md`
  Comprehensive catalog of all current value types, including:
  - all types implementing `Validate() error`
  - all primitive-wrapper value objects
  - alias/re-export type mappings

- `references/value-type-patterns.md`
  Design patterns, naming/error conventions, goplint directives, and anti-patterns.

- `references/maintenance-workflow.md`
  Update protocol, verification commands, and drift-prevention checks.

---

## Required Checks After Typesystem Changes

- `make check-baseline`
- `make check-types` for targeted DDD checks when production type shapes changed
- `make check-types-all` / `make check-types-all-json` for broad type-system sweeps
- `make test`
- `make lint`
- `make check-file-length`
- `make check-agent-docs` (if `AGENTS.md` or `.agents/skills/*` changed)

For docs-only type-catalog refreshes, use at minimum:
- `make check-agent-docs`
