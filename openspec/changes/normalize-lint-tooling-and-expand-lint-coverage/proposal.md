## Why

Invowk has a strong Go linting stack, but the current automation does not enforce the same lint surfaces everywhere, and some configured policies are either inert or broader than their comments imply. This change turns linting into a single coherent quality gate that uses one normalized golangci-lint version, covers both Go modules, enforces configured formatting, and expands high-signal readability and maintainability checks without contradictory defaults or hidden exclusions.

## What Changes

- Normalize golangci-lint version resolution so local Make targets, pre-commit hooks, and GitHub Actions use the same pinned version instead of relying on the ambient `PATH`.
- Update lint automation so the root module and the nested `tools/goplint` module are both linted wherever the repository advertises full lint coverage.
- Add explicit formatter enforcement for the configured golangci-lint v2 formatter sections.
- Remove or scope global lint exclusions whose comments describe test-only or fixture-only behavior.
- Add low-noise linters that improve exported documentation, modern Go idioms, and maintainability signal after cleaning their current findings.
- Pilot missing-`t.Parallel()` enforcement in the most tractable scope first, then make the resulting policy and automation truthful.
- Strengthen goplint exception governance so accepted exceptions are auditable and documented as part of the quality gate story.
- Update agent-facing rules and command documentation so they describe the actual lint, format, version-pin, and goplint audit contracts.

## Capabilities

### New Capabilities

- `lint-tooling-quality-gates`: Defines the repository linting, formatting, tool-version, nested-module, linter-selection, exclusion-governance, and goplint exception-audit contracts.

### Modified Capabilities

None.

## Impact

- Affected automation includes `Makefile`, `.github/workflows/lint.yml`, `.pre-commit-config.yaml`, golangci-lint configuration files, and version-pinning documentation.
- Affected Go quality gates include root-module linting, `tools/goplint` linting, formatter checks, goplint baseline/exception checks, and any source cleanup required to enable the selected linters.
- Affected agent documentation includes `AGENTS.md` and `.agents/rules/*` only where wording must match the implemented lint contracts.
- No user-facing CLI behavior or public Invowk runtime API is expected to change.
