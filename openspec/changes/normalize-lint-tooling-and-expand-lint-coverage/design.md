## Context

Invowk currently has two golangci-lint configurations: the root module config in `.golangci.toml` and the nested `tools/goplint/.golangci.toml` config. `make lint` runs both, but CI and pre-commit only run the root config. The root config also relies on `default = "standard"` plus an explicit enable list, while `tools/goplint` uses `default = "none"` with a fully explicit list.

The repository already has a strong precedent for exact Go tool pinning: mutation testing resolves `go-mutesting` through the root `go.mod` tool directive and verifies the resolved module version before execution. Golangci-lint should use the same pattern so local commands do not depend on whichever binary happens to be installed on `PATH`.

The exploration that led to this change found these current gaps:

- The latest checked golangci-lint release is `v2.12.2`, and the repo already pins that version in CI and pre-commit, but local `make lint` used an ambient `2.11.4` binary.
- CI and pre-commit do not run the nested `tools/goplint` golangci-lint config.
- The configured golangci-lint v2 formatter sections are not enforced by current gates.
- Test parallelism rules overclaim enforcement because `tparallel` is enabled but missing-`t.Parallel()` enforcement is not.
- Root `errcheck` global exclusions include test/fixture rationale even though global exclusions can affect production code.
- Goplint exception governance exists but is not fully wired into the quality gate stack.

## Goals / Non-Goals

**Goals:**

- Make golangci-lint version resolution exact, current, and shared by local, CI, and pre-commit gates.
- Make full lint coverage mean root module plus `tools/goplint` module in every advertised full lint path.
- Enforce configured formatters for both modules.
- Convert root golangci-lint configuration to an explicit effective linter set or otherwise make the effective set verifiable and documented.
- Add high-signal linters in one implementation pass and clean the resulting findings.
- Align test parallelism, context propagation, type assertion safety, and exported documentation policy with actual lint enforcement.
- Make lint exclusions, nolints, and goplint exceptions narrow enough to audit.
- Update agent-facing documentation and run the required verification gates.

**Non-Goals:**

- Change Invowk CLI behavior, runtime behavior, or public package APIs except where source cleanup is needed to satisfy lint.
- Add broad subjective style gates such as line length, variable-name length, or package-comment style unless they are required by the selected high-signal linters.
- Enable high-churn complexity or duplication gates as blockers unless the implementation fully cleans their findings in this same change.
- Replace custom goplint analyzer gates with golangci-lint; both gate families remain necessary.

## Decisions

### Use a root Go tool pin for golangci-lint

Add golangci-lint as a root `go.mod` tool dependency using the exact current release selected during implementation, then resolve it with `go tool -n golangci-lint`. This mirrors the mutation-testing tooling model and keeps the binary under Go module version control.

Alternatives considered:

- Keep using `golangci/golangci-lint-action` plus pre-commit's external hook. This keeps action-level caching but leaves local Makefile behavior dependent on separate installation unless another wrapper is added.
- Add only a local version check around `PATH` resolution. This is simpler but still requires contributors to install the exact binary manually.

### Introduce one thin lint wrapper

Create a small repository script for golangci-lint resolution, version verification, module-root selection, and command dispatch. Makefile, CI, and pre-commit should call this wrapper or Make targets that call it.

The wrapper should support at least:

- root run
- `tools/goplint` run
- root formatter check
- `tools/goplint` formatter check
- root config verification
- `tools/goplint` config verification
- effective-linter inspection for audit/debugging

This avoids duplicating version checks and `cd tools/goplint` logic across workflows.

### Make CI and pre-commit route through the same targets

CI should run the same normalized Make targets used locally rather than a narrower action-only invocation. Pre-commit should use local hooks that invoke the normalized root and nested-module lint/format targets. File filters can remain for convenience, but they must not falsely imply different coverage when `always_run` is used.

### Enforce formatter checks explicitly

Golangci-lint v2 formatters are separate from `golangci-lint run`, so the implementation must add explicit formatter checks. The formatter target should use diff/check mode and fail when formatting changes would be produced.

### Prefer explicit linter sets

Change the root config from `default = "standard"` plus explicit enables to `default = "none"` with an explicit complete list, unless implementation proves a better deterministic mechanism. This avoids hidden drift if golangci-lint changes the `standard` set.

The nested `tools/goplint` config already follows this pattern and should keep doing so.

### Add high-signal linters as blocking gates after cleanup

Enable and clean findings for:

- `godoclint` or the selected golangci-lint-supported exported-documentation linter
- `intrange`
- missing-`t.Parallel()` enforcement, expected to be `paralleltest`
- actionable context propagation checking, expected to be `contextcheck`
- unchecked type assertion detection, using either `errcheck.check-type-assertions = true` or a dedicated linter such as `forcetypeassert`

The implementation should avoid enabling `containedctx` as a global blocker because Invowk intentionally stores context in several server/runtime state structures. Complexity, duplication, preallocation, and highly subjective style linters should remain out of the blocking set unless this same implementation cleans and justifies their findings.

### Scope exclusions before adding more linters

Global exclusions should be reserved for behavior that is genuinely acceptable in production code. Test-only and fixture-only cases should become path-scoped exclusion rules. New lint exceptions should prefer local `//nolint:<linter> // rationale` comments over broad config-level exclusions.

### Treat goplint exceptions as governed debt

The empty goplint baseline means long-lived accepted debt now lives primarily in `tools/goplint/exceptions.toml`. The quality gate should check malformed, stale, or overdue exceptions, and documentation should clearly distinguish baseline-suppressed findings from always-visible hard blockers.

## Risks / Trade-offs

- Large test parallelism cleanup could expose order-dependent tests → Keep exceptions local and rationale-backed, but complete the cleanup in this change.
- Type assertion hardening may produce noisy findings in TUI or analyzer internals → Convert assertions where practical and use local, invariant-specific suppressions where framework or analyzer contracts make the assertion safe.
- Context propagation linting may flag intentional fallback contexts → Fix real propagation gaps and locally justify intentional fresh contexts.
- Root config conversion from `standard` to `none` could accidentally drop a default linter → Before and after conversion, capture the effective linter list and ensure all previously enabled standard linters remain explicitly listed.
- Routing CI through Make targets may lose golangci-lint-action convenience features → The gain is parity; any desired caching can be reintroduced only if it still calls the normalized versioned path.
- Adding golangci-lint to `go.mod` may increase tool dependency churn → This is consistent with repository version-pinning policy and easier to audit than ambient binaries.

## Migration Plan

1. Re-check the current upstream golangci-lint latest release and choose the exact version for this implementation.
2. Add golangci-lint as a root Go tool dependency and document it in version-pinning guidance.
3. Add the normalized wrapper and update Makefile targets for lint, format, config verification, and effective-linter inspection.
4. Update CI and pre-commit to use the normalized targets for root and `tools/goplint`.
5. Convert root linter defaults to an explicit linter set without dropping existing effective linters.
6. Scope existing global exclusions and require auditable nolints.
7. Enable selected high-signal linters and clean or locally justify all findings.
8. Add goplint exception governance to the quality gates and clean documentation drift.
9. Run final validation: config verification, full lint, formatter checks, goplint baseline/exception checks, agent-doc sync, and any targeted tests touched by cleanup.

Rollback is straightforward because this change affects repository tooling and source cleanup rather than runtime behavior: revert the tooling/config changes and any cleanup-only source edits as one coherent change if the gate proves unsuitable before merge.

## Open Questions

None. The implementation should make narrow local judgment calls for individual lint findings, but the required end state is fully specified.
