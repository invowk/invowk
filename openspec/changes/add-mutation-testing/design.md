## Context

Invowk's current quality gates already include unit tests, CLI `testscript` suites, container/runtime integration tests, race-enabled CI, coverage artifacts, lint, goplint baseline checks, and benchmark smoke tests. Mutation testing should therefore add a targeted test-oracle signal instead of becoming another blanket full-suite run.

The repository has two Go modules that matter for mutation testing:

- the root Invowk module under `cmd/`, `internal/`, and `pkg/`;
- the nested `tools/goplint` module, which already has separate CI lanes and semantic compatibility gates.

The primary tool should be `github.com/jonbaldie/go-mutesting/v2/cmd/go-mutesting`, pinned to an exact version. It currently provides the features Invowk needs most: coverage-aware MSI, changed-line mutation, baselines, GitHub annotations, compact JSON summaries, agent-focused escaped-mutant JSON, per-test filtering, and stable single-mutant reruns. Gremlins and gomu remain useful comparison tools, but Gremlins is still 0.x and warns about very large Go modules, while gomu is younger and less proven for this repo's needs.

## Goals / Non-Goals

**Goals:**

- Add reproducible local and CI mutation-testing entrypoints.
- Keep all external tooling pinned and verifiable under Invowk's version-pinning rules.
- Provide fast PR feedback for changed Go lines.
- Provide broader scheduled mutation scans for root-module and `tools/goplint` quality trends.
- Support a baseline workflow that fails on new escaped mutants without blocking adoption on existing survivors.
- Produce artifacts that maintainers and agents can use to write targeted killing tests.
- Avoid unsafe source mutation in dirty local worktrees.

**Non-Goals:**

- Do not replace `make test`, `make lint`, `make check-baseline`, race tests, coverage uploads, or Sonar checks.
- Do not run full container, CLI, and cross-platform test matrices for every mutant by default.
- Do not require mutation testing to run on Windows or macOS initially.
- Do not mutate generated docs, website code, samples, OpenSpec files, or test fixtures.
- Do not make every PR fail on historical survivors during initial rollout.

## Decisions

### Decision: Use pinned go-mutesting as the primary engine

Use `go-mutesting` as the first-class mutation engine and record it as a pinned Go tool dependency where practical. CI shall install or run the exact pinned version and verify the binary before use.

Alternatives considered:

- Gremlins: mature enough to compare, but 0.x compatibility and large-module runtime warnings make it a weaker primary fit.
- gomu: promising and fast-moving, but younger and less proven.
- A custom Invowk mutator: too much maintenance surface for the first implementation.

### Decision: Add an Invowk wrapper layer

Expose mutation runs through Make targets backed by repository scripts rather than raw workflow YAML. The wrapper should own target selection, clean-worktree checks, baseline/report paths, default flags, and module-specific invocation.

The wrapper should define at least these run profiles:

- `pr`: changed-line mutation against the PR base with annotations and no hard failure while advisory.
- `full`: curated package-manifest scan for scheduled or manual runs.
- `baseline-update`: regenerate the accepted-survivor baseline intentionally.
- `dry-run`: count generated mutants without executing tests.
- `rerun`: rerun a single escaped mutant by stable ID.

Alternatives considered:

- Direct CI commands only: faster to write but hard to keep consistent locally.
- One Make target only: too coarse for PR, scheduled, and focused developer workflows.

### Decision: Split root-module and goplint mutation profiles

Root-module mutation runs should start with a curated, high-signal production package seed from `internal/` and `pkg/`, excluding test-only, generated, fixture, docs, website, samples, OpenSpec, and cross-module helper surfaces unless explicitly selected. Large CLI adapter, runtime, TUI, audit, and container surfaces should be added after advisory timing data shows they are baselineable, or covered through changed-line PR and focused high-assurance profiles. `tools/goplint` should run from its own module root with its own config and reports.

Alternatives considered:

- Single `./...` run from repository root: misses nested-module nuance and risks mutating inappropriate surfaces.
- Only root-module mutation: leaves a high-value analyzer with its own test stack outside the mutation signal.

### Decision: Keep PR mode fast and scheduled mode broad

PR mode should use changed-line filtering and ignore no-mutation PRs. It should publish GitHub annotations and JSON artifacts, and it can become blocking only after advisory data proves the signal is stable. Scheduled mode should run broader package manifests and record summaries, escaped-mutant reports, and baselines.

Alternatives considered:

- Blocking full-tree mutation on every PR: too expensive and likely noisy for Invowk's current scale.
- Scheduled-only mutation: useful for audits but too delayed for code review feedback.

### Decision: Use baselines for adoption, with tightening over time

Initial rollout should capture known escaped mutants in committed baseline files. CI should fail only on new escapes when the corresponding profile is blocking. Maintainers can shrink the baseline as killing tests are added.

Alternatives considered:

- Immediate score thresholds: likely to block unrelated work before the team understands equivalent/noisy mutants.
- Permanent advisory-only reporting: safer but weaker than the requested robust quality gate.

### Decision: Do not combine mutation testing with race mode by default

Mutation runs should use normal Go test execution by default. Existing CI already runs race-enabled tests; multiplying every mutant by race overhead would make mutation feedback much slower. Focused profiles may pass explicit test flags when diagnosing concurrency-sensitive mutants.

Alternatives considered:

- Always use `-race`: high confidence, but likely impractical for PR and scheduled mutation budgets.
- Never allow race flags: too restrictive for focused investigations.

## Risks / Trade-offs

- Tool instability or upstream regressions -> Pin exact versions, verify binary versions, and keep upgrade work routed through version-pinning policy.
- Equivalent or low-value mutants create noise -> Use baselines, per-mutator disable lists, source-line ignores, and periodic survivor review.
- Runtime becomes too slow -> Use changed-line PR mode, package manifests, per-test coverage filtering, worker limits, and scheduled full scans.
- Source mutation can disturb local worktrees -> Require clean worktrees for local write-mode runs or execute in a temporary worktree/copy.
- Package-local execution can miss cross-package or CLI-only oracles -> Add high-assurance custom exec profiles for selected command/runtime/discovery/schema surfaces after the first package-level rollout.
- Container and CLI mutation runs can be flaky or expensive -> Keep them out of default mutation profiles and introduce focused integration profiles only when a package-level survivor proves it needs that oracle.
- Baselines can become a hiding place for weak tests -> Store them intentionally, review them in scheduled reports, and provide tasks for shrinking them.

## Migration Plan

1. Add pinned mutation tooling and local wrapper commands without enabling a blocking gate.
2. Run dry-runs and advisory PR scans to tune package manifests, exclusions, mutators, workers, and timeouts.
3. Generate initial baseline files for accepted survivors.
4. Enable PR annotations and report artifact uploads.
5. Add scheduled full scans for root-module and `tools/goplint` profiles.
6. After advisory data is stable, make PR mode fail only on new escaped mutants for changed Go lines.
7. Tighten package thresholds or baseline size targets incrementally.

Rollback is straightforward: disable the mutation workflow or make it advisory while leaving local tools and reports available. Since mutation testing does not change Invowk runtime behavior, rollback does not require user-facing migration.

## Implementation Notes

- Selected `github.com/jonbaldie/go-mutesting/v2/cmd/go-mutesting` `v2.7.0` on 2026-06-01 after verifying the current upstream release state and available module versions.
- The tool does not expose a `--version` flag, so repository automation verifies the embedded module version with `go version -m "$(go tool -n go-mutesting)"`.
- Initial committed baselines start empty; rollout CI remains advisory while scheduled and pull-request reports establish timing and survivor data. The first real blocking all-module run on 2026-06-01 confirmed that a blanket empty-baseline full scan is too broad for initial adoption, so the root full manifest now starts from a smaller baselineable seed.

## Open Questions

- Which initial packages should be in the scheduled full-scan manifest after timing data is collected?
- Should high-assurance custom exec profiles run CLI `testscript` for a very small surface in this change, or be proposed separately after package-level mutation data exists?
- Should report summaries be published only as artifacts, or also appended to GitHub step summaries?
