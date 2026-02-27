---
name: invowk-typesystem
description: Invowk type-system guidance for all value types across pkg/* and internal/*, including Validate() contracts, primitive-wrapper value objects, aliases/re-exports, sentinel errors, and Invalid*Error wrappers. Use when adding, reviewing, refactoring, or documenting value types.
disable-model-invocation: false
metadata:
  short-description: Canonical operating guide for Invowk's value-type architecture.
  ownership: "Repo-wide type-system conventions and catalogs"
  audience:
    - "Agents implementing new domain/value types"
    - "Agents reviewing Validate()/error-shape consistency"
    - "Agents documenting type-system changes"
  trigger-patterns:
    - "value type"
    - "Validate"
    - "ErrInvalid"
    - "Invalid*Error"
    - "primitive wrapper"
    - "typesystem"
    - "DDD type"
    - "type safety"
    - "goplint baseline type findings"
  scope:
    includes:
      - pkg/types
      - pkg/invowkfile
      - pkg/invowkmod
      - pkg/platform
      - internal/config
      - internal/runtime
      - internal/discovery
      - internal/issue
      - internal/container
      - internal/tui
      - internal/sshserver
      - internal/tuiserver
      - internal/core/serverbase
      - internal/watch
      - internal/app/execute
    excludes:
      - generated files
      - non-Go docs unless explicitly requested
  outputs:
    - "Type-system impact analysis"
    - "Updated value-type catalogs"
    - "Validation and guardrail checklist"
  maintenance:
    source-of-truth: references/type-catalog.md
    refresh-script: scripts/extract_value_types.sh
    refresh-trigger: "Any addition/removal/rename/semantic change of a value type"
---

# Invowk Typesystem

Use this skill when:
- Creating or refactoring value types in `pkg/*` or `internal/*`
- Debugging `Validate()` behavior, sentinel error wrapping, or `Invalid*Error` shapes
- Resolving goplint findings related to primitive wrappers and type safety
- Documenting type-system changes for agent guidance

---

## Mandatory Workflow

1. Read `references/value-type-patterns.md` to apply canonical design rules.
2. Check `references/type-catalog.md` before introducing a new type.
3. If a new type is still needed, follow the creation checklist in `references/maintenance-workflow.md`.
4. Refresh the catalog with `scripts/extract_value_types.sh` and reconcile the docs.
5. Run the required checks listed below.

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
  - all types implementing `Validate() (bool, []error)`
  - all primitive-wrapper value objects
  - alias/re-export type mappings

- `references/value-type-patterns.md`
  Design patterns, naming/error conventions, and anti-patterns.

- `references/maintenance-workflow.md`
  Update protocol, verification commands, and drift-prevention checks.

---

## Required Checks After Typesystem Changes

- `make check-baseline`
- `go test ./...`
- `make check-agent-docs` (if `AGENTS.md` or `.agents/skills/*` changed)

For docs-only type-catalog refreshes, use at minimum:
- `make check-agent-docs`

