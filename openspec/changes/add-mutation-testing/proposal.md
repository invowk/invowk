## Why

Invowk already has broad unit, integration, CLI, analyzer, and platform test coverage, but coverage and passing tests do not prove that assertions would catch realistic logic regressions. Mutation testing gives the project a stronger test-quality signal by intentionally introducing small source changes and verifying that the existing test suite detects them.

## What Changes

- Add a mutation-testing quality workflow for Go production code and the nested `tools/goplint` module.
- Introduce pinned mutation-testing tooling, local entrypoints, and CI entrypoints that match Invowk's existing version-pinning and verification rules.
- Provide fast PR feedback for changed Go lines, plus a broader scheduled scan for package-level quality trends.
- Establish a baseline process so existing surviving mutants can be accepted temporarily while new escaped mutants are surfaced or gated.
- Produce machine-readable reports and GitHub annotations suitable for reviewer triage and targeted test improvements.
- Keep mutation testing separate from the normal `make test` and race-test gates so it adds signal without making every regular test run prohibitively slow.

## Capabilities

### New Capabilities

- `mutation-testing`: Defines the mutation-testing workflows, target selection, tool pinning, reporting, baselining, and CI quality gates for Invowk.

### Modified Capabilities

None.

## Impact

- Build and developer tooling: Make targets and scripts for local mutation runs, dry-runs, report generation, baseline updates, and focused mutant reruns.
- CI: A new mutation-testing workflow or job set for advisory PR checks, scheduled full scans, artifacts, and eventual quality gates.
- Dependencies and pinning: A pinned mutation-testing CLI, preferably recorded as a Go tool dependency and mirrored in agent-facing version documentation.
- Test infrastructure: Package manifests and execution profiles that avoid expensive container, CLI, race, and platform lanes unless explicitly selected.
- Documentation: Agent-facing command and version-pinning rules, plus developer-facing guidance for interpreting reports and improving tests.
