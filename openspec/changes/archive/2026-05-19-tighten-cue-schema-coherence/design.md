## Context

Invowk intentionally splits validation across CUE schemas, post-decode Go validators, and a small runtime preflight layer for targeted diagnostics. That split is healthy, but four current contracts create avoidable confusion:

- `env_inherit_allow` can be configured without `env_inherit_mode: "allow"`, even though the runtime ignores the allowlist outside allow mode.
- CUE and full invowkfile structure validation require flag and argument descriptions, while `Flag.Validate()` and `Argument.Validate()` still allow name-only values.
- Container runtimes already require exactly one source, but that contract is expressed mainly through preflight and Go validation rather than the CUE runtime variant itself.
- `requires.path` in `invowkmod.cue` uses an overly broad CUE regex for traversal prevention, even though Go performs the real cross-platform path normalization and security checks.

## Goals / Non-Goals

**Goals:**

- Make no-op environment allowlists impossible by requiring `env_inherit_mode: "allow"` whenever `env_inherit_allow` is present.
- Make flag and argument descriptions mandatory in every validation path, including direct Go value validation.
- Express the container runtime source invariant in CUE where statically possible while preserving actionable errors and Go defense-in-depth.
- Relax module requirement path schema validation so harmless names containing consecutive dots are accepted, with traversal and absolute-path rejection owned by Go.
- Update tests and documentation so schema comments, user docs, generated examples, and validator behavior agree.

**Non-Goals:**

- No new environment inheritance modes or shorthand syntax.
- No implicit conversion from an allowlist to `env_inherit_mode: "allow"`.
- No attempt to implement complete filesystem path validation in CUE.
- No change to the runtime execution model, container engine behavior, or module dependency resolution model.

## Decisions

1. Require explicit allow mode for allowlists.

   The implementation will reject any runtime configuration that sets `env_inherit_allow` while `env_inherit_mode` is omitted, `"none"`, or `"all"`. This makes the author choose the inheritance policy explicitly instead of relying on a field that would otherwise be ignored. An implicit-mode design was rejected because it would silently change environment exposure, especially when CLI or config overrides are involved.

2. Make descriptions mandatory at the value-object boundary.

   `Flag.Validate()` and `Argument.Validate()` will reject empty or whitespace-only descriptions, matching CUE and full invowkfile structure validation. Test fixtures and generated examples should provide real descriptions instead of relying on name-only helper values. Making descriptions optional everywhere was rejected because command help quality is part of the user-facing contract already expressed by the schema.

3. Model container source selection with closed CUE variants.

   The container runtime schema should be refactored into a shared container base plus two closed source variants: one requiring `image` and forbidding `containerfile`, and one requiring `containerfile` and forbidding `image`. The existing preflight layer may remain as a best-effort diagnostics path for literal user configs, and Go validation must keep the same invariant for direct Go construction and defense-in-depth. This preserves good error messages without leaving the schema under-specified.

4. Treat module requirement path CUE validation as a portable prefilter.

   The `path` field in `#ModuleRequirement` should keep simple shape constraints such as string type, length, and obvious absolute-root rejection, but it should stop rejecting every `..` substring. `SubdirectoryPath.Validate()` remains the canonical owner of traversal, absolute path, Windows drive, UNC/rooted path, and separator-normalization checks. Schema comments should say this directly.

5. Verify at behavioral boundaries, not only field sync.

   The change should add or update behavioral sync tests for CUE-vs-Go examples, value-level validation tests for flags and arguments, parser/preflight tests for runtime diagnostics, and documentation/snippet checks for user-visible contract changes.

## Risks / Trade-offs

- **Existing fixtures with name-only flags or arguments fail** -> Update test helpers and fixtures to include descriptions, then keep the new failures as useful signal.
- **CUE disjunction errors become noisier for container runtimes** -> Keep or adapt runtime preflight so common mistakes still produce targeted messages.
- **Nested runtime schema variants confuse schema sync extraction** -> Update schema sync helpers or tests to explicitly union the fields from the container source variants.
- **Relaxed CUE path filtering looks like weaker security** -> Document the ownership split and add Go validation tests proving traversal and absolute paths still fail.
- **Breaking validation may surprise users with previously ignored allowlists** -> Document the migration: add `env_inherit_mode: "allow"` next to intentional allowlists, or remove stale allowlists.

## Migration Plan

Existing invowkfiles that intentionally use `env_inherit_allow` must add `env_inherit_mode: "allow"` in the same runtime block. Programmatic construction of `Flag` and `Argument` values must set `Description` before calling `Validate()` or embedding them in an invowkfile. Module requirement paths that were rejected only because a safe segment contained consecutive dots will become valid without user changes.

## Open Questions

None.
