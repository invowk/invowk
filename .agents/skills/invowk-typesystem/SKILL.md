---
name: invowk-typesystem
description: Invowk type-system and goplint guidance for value types across cmd/*, internal/*, and pkg/*, including Validate() contracts, primitive-wrapper value objects, aliases/re-exports, sentinel errors, Invalid*Error wrappers, check-types/check-types-all findings, DDD compliance, baseline updates, and exception triage. Use when adding, reviewing, refactoring, or documenting value types, or resolving goplint type findings.
---

# Invowk Typesystem

Also use this skill when debugging `Validate()` behavior, sentinel error
wrapping, or `Invalid*Error` shapes, and when documenting type-system changes
for agent guidance.

---

## Mandatory Workflow

1. Read `references/value-type-patterns.md` to apply canonical design rules.
2. Check `references/type-catalog.md` before introducing a new type.
3. If a new type is still needed, follow the creation checklist in `references/maintenance-workflow.md`.
4. Refresh the catalog with `scripts/extract_value_types.sh` and reconcile the docs.
5. Run the required checks listed below.

When changing the extractor's detection logic, keep its three roots aligned
(`references/maintenance-workflow.md` § 1).

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
  Generated catalog of all current value types, including:
  - all types implementing `Validate() error`
  - all primitive-wrapper value objects
  - alias/re-export type mappings

- `references/value-type-patterns.md`
  Design patterns, naming/error conventions, the in-repo `//goplint:` directive
  names (semantics live in the goplint repository), and anti-patterns.

- `references/maintenance-workflow.md`
  Update protocol, catalog freshness check, and drift-prevention checks.

---

## Required Checks After Typesystem Changes

- `make check-baseline` (analyzer-behavior changes belong in the standalone
  `github.com/invowk/goplint` repository, which owns its own soundness gates)
- `make check-types` for targeted DDD checks when production type shapes changed
- `make check-types-all` / `make check-types-all-json` for broad type-system sweeps
- `make test`, `make lint`, `make check-file-length`, and the rest of
  `.agents/rules/checklist.md` (including `make check-agent-docs` when
  `AGENTS.md` or `.agents/skills/*` changed)

## Triaging New Findings by Precedent

Before baselining or excepting a new finding, look for how the repository already
handles the same category. Build `./bin/goplint` first (`make build-goplint`;
every `check-types*` and `check-baseline` target also builds it), then list the
new findings in the touched packages with
`./bin/goplint -check-all -json -config=.goplint/exceptions.toml <pkgs>` and
filter out messages already in `.goplint/baseline.toml`.

- **`nonzero-value-field` on error-struct context fields** (for example
  `ModuleKey`, `Version`): baseline them, as the existing
  `ContentHashMismatchError` fields are. Error fields are always set, not optional.
- **Primitive parameters on new resolver helpers:** make the helper a method of
  the owning service when that service already has a scoped exception (for
  example `modulesync.Resolver.*.*`). Do not add a free function that needs a new
  exception.
- **`unvalidated-cast-inconclusive` after growing a function with
  `//goplint:ignore` casts:** move the casts and their single use into a minimal
  helper that keeps the original shape. Inconclusive findings can never be
  baselined, excepted, or ignored.
- **CLI display text returned as `string`:** add a `.return.*` exception, as
  `invowk.formatDuration` has.

The soundness gate is canonical and flagless. Do not use removed analyzer
backend, engine, alias, refinement, UBV, or inconclusive-policy selectors in
local commands or documentation.

For docs-only type-catalog refreshes, use at minimum:
- `make check-agent-docs`
