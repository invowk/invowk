# lint-tooling-quality-gates Specification

## Purpose
Define the repository-wide lint tooling, module coverage, and blocking quality-gate contracts used by local development, pre-commit, and CI, including goplint baseline, exception, semantic, and soundness assurance.
## Requirements
### Requirement: Golangci-lint version is normalized
Invowk SHALL resolve golangci-lint through one exact repository-governed version contract used by local Make targets, pre-commit hooks, and GitHub Actions.

#### Scenario: Latest release is checked before changing the pin
- **WHEN** maintainers implement or update the golangci-lint pin
- **THEN** they MUST check the current upstream golangci-lint release
- **THEN** the selected version MUST be an exact version
- **THEN** the implementation notes or updated version-pinning documentation MUST record the selected version and where it is enforced

#### Scenario: Local lint does not use an unverified ambient binary
- **WHEN** `make lint` or any lint subtarget runs locally
- **THEN** it MUST invoke the repository-normalized golangci-lint binary or fail before linting with an actionable version mismatch message
- **THEN** it MUST NOT silently use an arbitrary `golangci-lint` binary from `PATH`

#### Scenario: CI and pre-commit use the same golangci-lint version
- **WHEN** GitHub Actions or pre-commit runs golangci-lint
- **THEN** the resolved golangci-lint version MUST match the repository-normalized version
- **THEN** a version mismatch MUST fail before lint results are trusted

#### Scenario: Version documentation remains synchronized
- **WHEN** the golangci-lint version changes
- **THEN** `AGENTS.md`, `.agents/rules/version-pinning.md`, workflow configuration, pre-commit configuration, Make targets, and any wrapper or tool-pin files that mention golangci-lint MUST describe the same version source and current version

### Requirement: Lint automation covers every Go module
Invowk SHALL lint the root Go module — its only Go module after the goplint extraction — anywhere the repository advertises full lint coverage, and SHALL NOT advertise coverage of external tool repositories it does not lint.

#### Scenario: Make lint covers both modules
- **WHEN** maintainers run `make lint`
- **THEN** linting MUST run against the root module — the repository's only Go module after the goplint extraction — with the root golangci-lint config
- **THEN** lint automation MUST NOT reference a nested `tools/goplint` module that no longer exists in this repository

#### Scenario: CI lint coverage matches Make lint coverage
- **WHEN** the lint workflow runs in GitHub Actions
- **THEN** it MUST lint the root module and fail on config-validation failures or lint findings
- **THEN** workflow comments and job names MUST NOT imply coverage of the external goplint repository

#### Scenario: Pre-commit lint coverage matches changed Go module surfaces
- **WHEN** pre-commit runs golangci-lint hooks
- **THEN** it MUST run the root-module lint gate
- **THEN** goplint's own sources MUST be linted by the `invowk/goplint` repository's equivalent gates, not by invowk hooks

#### Scenario: Nested module boundaries are explicit
- **WHEN** maintainers inspect lint automation
- **THEN** the automation MUST make clear the repository has a single Go module and that the pinned `github.com/invowk/goplint` tool dependency is linted in its own repository

### Requirement: Golangci-lint formatter policy is enforced
Invowk SHALL enforce every configured golangci-lint v2 formatter policy instead of leaving formatter sections as documentation-only configuration.

#### Scenario: Formatter check runs for root module
- **WHEN** the repository formatter check runs
- **THEN** it MUST run golangci-lint formatting in diff or check mode for the root module using `.golangci.toml`
- **THEN** it MUST fail when formatting changes would be produced

#### Scenario: Formatter check runs for tools/goplint module
- **WHEN** the repository formatter check runs
- **THEN** it MUST run golangci-lint formatting in diff or check mode for `tools/goplint` using `tools/goplint/.golangci.toml`
- **THEN** it MUST fail when formatting changes would be produced

#### Scenario: Formatter automation is part of the regular quality gate
- **WHEN** `make lint`, CI linting, or pre-commit quality gates run
- **THEN** formatter checks MUST run as part of the same quality gate or through an explicitly documented companion target/hook required for completion
- **THEN** generated-file exclusions MUST match the formatter config rather than separate ad hoc skip logic

### Requirement: Linter configs are deterministic and non-contradictory
Invowk SHALL keep golangci-lint configuration deterministic, internally consistent, and truthful about what each linter and exclusion enforces.

#### Scenario: Enabled linters are explicit
- **WHEN** maintainers inspect root or `tools/goplint` golangci-lint config
- **THEN** each blocking linter MUST be explicitly listed
- **THEN** the config MUST NOT rely on an implicit default set whose contents can change without visible config changes unless the design documents that trade-off and all tooling verifies the effective linter set

#### Scenario: Config verification passes
- **WHEN** maintainers run the lint validation target
- **THEN** golangci-lint config verification MUST pass for the root config
- **THEN** golangci-lint config verification MUST pass for `tools/goplint/.golangci.toml`

#### Scenario: Effective linter sets are inspectable
- **WHEN** maintainers audit lint coverage
- **THEN** they MUST be able to inspect the effective enabled linter set for each module
- **THEN** differences between the root and `tools/goplint` linter sets MUST be intentional, documented in config comments or design notes, and based on module-specific behavior rather than drift

#### Scenario: Comments match enforcement scope
- **WHEN** a linter setting, exclusion, or workflow comment explains a policy
- **THEN** the described scope MUST match the actual golangci-lint scope
- **THEN** a test-only or fixture-only rationale MUST NOT be attached to a global production-code exclusion

### Requirement: Lint exclusions and nolints are narrow and auditable
Invowk SHALL prefer fixing code over broad suppression and SHALL keep every remaining lint suppression scoped, specific, and reviewable.

#### Scenario: Global exclusions are not test-only suppressions
- **WHEN** golangci-lint excludes a function, linter, or path globally
- **THEN** the exclusion rationale MUST apply to production code as well as tests
- **THEN** test-only or fixture-only exclusions MUST be moved to path-scoped exclusion rules or removed

#### Scenario: Nolints identify linter and rationale
- **WHEN** source code uses `//nolint`
- **THEN** the directive MUST name the specific linter or linters being suppressed
- **THEN** the directive MUST include or be adjacent to a human-readable rationale unless the linter config explicitly documents a narrower acceptable pattern

#### Scenario: Stale suppressions fail lint
- **WHEN** a `//nolint` directive no longer suppresses an active finding
- **THEN** golangci-lint or a companion lint validation step MUST fail
- **THEN** the stale suppression MUST be removed before the quality gate is considered passing

### Requirement: High-signal Go linters are enabled after cleanup
Invowk SHALL expand lint coverage for readability, maintainability, modern Go idioms, and subtle correctness gotchas without accepting unresolved findings.

#### Scenario: Exported documentation linting is enabled
- **WHEN** golangci-lint runs on the root module or `tools/goplint`
- **THEN** exported-symbol documentation linting MUST be enabled where supported by golangci-lint
- **THEN** existing findings MUST be fixed or suppressed with local, specific rationale

#### Scenario: Modern integer range idioms are enforced
- **WHEN** golangci-lint runs on the root module or `tools/goplint`
- **THEN** integer range modernization checks MUST be enabled where supported by the selected golangci-lint version
- **THEN** existing findings MUST be fixed unless a local suppression documents why the older loop form is clearer or required

#### Scenario: Missing test parallelism is enforced
- **WHEN** golangci-lint runs on Go tests in the root module or `tools/goplint`
- **THEN** the lint gate MUST enforce that eligible tests and subtests call `t.Parallel()`
- **THEN** tests that cannot run in parallel because they mutate process-global state, rely on serial filesystem state, require exclusive external resources, or exercise order-sensitive integration behavior MUST be locally excluded with rationale

#### Scenario: Context propagation findings are enforced or locally justified
- **WHEN** golangci-lint runs on production Go code
- **THEN** context propagation linting MUST be enabled where it produces actionable findings
- **THEN** every remaining context propagation exception MUST be local and explain why a new or fallback context is intentional

#### Scenario: Unchecked type assertions are hardened
- **WHEN** golangci-lint runs on production Go code
- **THEN** unchecked type assertions MUST be reported by lint tooling
- **THEN** existing unchecked type assertions MUST be converted to checked assertions or locally suppressed with a rationale that explains the invariant

#### Scenario: Noisy linters are not enabled as broad contradictory gates
- **WHEN** maintainers consider complexity, duplication, preallocation, contained-context, or highly subjective style linters
- **THEN** they MUST NOT enable them as blocking gates unless existing findings are cleaned or narrowly excluded
- **THEN** any excluded linter MUST be documented as intentionally omitted rather than accidentally forgotten

### Requirement: Test parallelism policy is truthful
Invowk SHALL align agent-facing test parallelism rules with the actual linters and exceptions used by the repository.

#### Scenario: Rules name the correct linter responsibilities
- **WHEN** `.agents/rules/testing.md` or related agent guidance describes test parallelism enforcement
- **THEN** it MUST distinguish missing-`t.Parallel()` enforcement from `tparallel` placement and subtest checks
- **THEN** it MUST name the enabled linter or validation step responsible for each behavior

#### Scenario: Parallelism exceptions are intentional
- **WHEN** a test is excluded from missing-`t.Parallel()` enforcement
- **THEN** the exclusion MUST identify the specific shared state, timing, platform, external resource, or integration behavior that prevents safe parallel execution
- **THEN** broad package-wide exclusions MUST be avoided unless every test in the package shares the same constraint

### Requirement: Goplint exception governance is enforced
Invowk SHALL keep its goplint baseline and exception governance — stored under the repository-owned `.goplint/` directory — aligned with its lint, type-system, and canonical semantic-analysis quality gates, executed against the exact pinned goplint tool version.

#### Scenario: Goplint lint and the required routed profile run together
- **WHEN** repository lint gates run
- **THEN** the goplint analyzer gates MUST execute the analyzer built from the exact version pinned in the root `go.mod` tool dependency
- **THEN** a version mismatch between the resolved analyzer and the pinned version MUST fail before analysis results are trusted

#### Scenario: Accepted goplint exceptions are reviewable
- **WHEN** a goplint exception is kept in `.goplint/exceptions.toml`
- **THEN** it MUST include a reason that explains why the exception remains acceptable
- **THEN** long-lived or broad exceptions MUST include a review date or equivalent review mechanism

#### Scenario: Stale goplint exception audit is part of quality gates
- **WHEN** repository quality gates run locally or in CI
- **THEN** stale, overdue, malformed, or unsupported goplint exceptions MUST be reported
- **THEN** stale matching MUST consume the exact canonical repository-audit result when package analysis has already run
- **THEN** review-date validation MUST NOT repeat package loading or analyzer traversal
- **THEN** the gate MUST fail for stale or malformed exceptions unless the design explicitly marks a temporary advisory transition inside this same change

#### Scenario: Baseline uses the only production semantics
- **WHEN** `make check-baseline` or `make update-baseline` invokes goplint
- **THEN** it MUST use the same canonical production analysis and fail-closed aggregation as the blocking full repository scan
- **THEN** a read-only baseline check MUST reuse an exact-tree canonical repository-audit result when one exists in the same execution plan
- **THEN** baseline data MUST live in `.goplint/baseline.toml`, and stable finding ID changes MUST be reported and reviewed before the baseline is accepted

#### Scenario: Goplint baseline wording matches behavior
- **WHEN** baseline tooling, goplint documentation, or agent guidance describes baseline behavior
- **THEN** it MUST distinguish baseline-suppressed categories from always-visible hard-blocking categories
- **THEN** stale statements about removed in-tree soundness machinery MUST NOT remain

#### Scenario: Canonical full scan is blocking
- **WHEN** the repository goplint full scan runs locally, in pre-commit, or in CI
- **THEN** violations, blocking inconclusive outcomes, malformed evidence, incomplete required evidence for the selected profile, and analyzer failures MUST fail the gate
- **THEN** the workflow MUST NOT downgrade or mask those outcomes

### Requirement: Documentation and verification remain synchronized
Invowk SHALL update documentation and validation so contributors can run, understand, and trust the lint and goplint consumer gates against the pinned analyzer version.

#### Scenario: Command documentation lists complete lint workflow
- **WHEN** contributors read `.agents/rules/commands.md`, `AGENTS.md`, Make help, or goplint consumer documentation
- **THEN** they MUST see how to run root lint, formatter and config checks, exception governance, baseline comparison, full scan, and the consumer performance smoke
- **THEN** documented commands and guarantee claims MUST match implemented targets, CI jobs, and the pinned tool version

#### Scenario: Agent documentation sync check passes
- **WHEN** implementation changes `AGENTS.md`, `.agents/rules/`, or `.agents/skills/`
- **THEN** `make check-agent-docs` MUST pass before the change is complete

#### Scenario: Final validation proves production semantics and evidence integrity
- **WHEN** a change to the goplint consumer surface is complete
- **THEN** maintainers MUST run lint, test, baseline, exception, full-scan, and agent-document gates against the pinned analyzer
- **THEN** analyzer-internal evidence integrity is proven by the `invowk/goplint` repository's gates before the pinned version exists

#### Scenario: Documented completion commands match the implementation
- **WHEN** contributors read `.agents/rules/commands.md`, `AGENTS.md`, Make help, or goplint consumer documentation
- **THEN** documented commands MUST match implemented targets and CI jobs
- **THEN** removed in-tree soundness machinery MUST NOT remain documented as supported invowk behavior; invowk documentation MUST point to the `invowk/goplint` repository as the authoritative source

### Requirement: Automatic goplint assurance routing is conservative and change aware
Repository automation SHALL classify every changed invowk path into exactly one reviewed change class — `documentation` or `consumer` — through a versioned invowk-owned ownership manifest, SHALL run the consumer tier for any diff containing a consumer-class or unmatched path, and SHALL fail closed to the consumer tier whenever classification is missing, malformed, or ambiguous. Analyzer-semantics and harness assurance tiers are governed by the `invowk/goplint` repository against its own tree.

#### Scenario: Documentation and bookkeeping changes route to the documentation tier
- **WHEN** a diff changes only paths in the `documentation` class
- **THEN** automation MUST NOT trigger a repository audit or analyzer execution for that diff

#### Scenario: Consumer code changes without analyzer ownership changes
- **WHEN** a diff changes any root-module code, configuration under `.goplint/`, or any path not matched by a documentation rule
- **THEN** automation MUST run one blocking canonical repository audit with baseline and exception governance against the pinned analyzer

#### Scenario: Harness-only changes route to the harness tier
- **WHEN** gate orchestration, execution planners, or distributed plumbing change
- **THEN** those surfaces now live in the `invowk/goplint` repository, whose own routing governs the harness tier; invowk automation treats its remaining gate wiring as consumer-class

#### Scenario: Analyzer-semantics changes select the semantic profile
- **WHEN** analyzer production semantics, tests, evidence producers, manifests, schemas, or thresholds change
- **THEN** those surfaces live in the `invowk/goplint` repository, whose own gates run the semantic profile before any version invowk can pin

#### Scenario: Analyzer or assurance ownership changes
- **WHEN** ownership of analyzer or assurance surfaces changes
- **THEN** the `invowk/goplint` repository's ownership manifest governs the routing consequence; invowk's manifest governs only documentation-vs-consumer classification of invowk paths

#### Scenario: Completion event requires exhaustive evidence
- **WHEN** a completion proof, release, or scheduled certification runs for the analyzer
- **THEN** the `invowk/goplint` repository's completion profile — including clean-tree freshness — MUST be blocking there; invowk's release gates run the consumer tier

#### Scenario: Executable inputs are never classified as documentation
- **WHEN** invowk's ownership manifest assigns classes to path families
- **THEN** every file a consumer gate reads as input — `.goplint/` configuration, manifests, baselines, thresholds — MUST NOT match a documentation rule

#### Scenario: Change context is missing or ambiguous
- **WHEN** the merge base, changed-path census, ownership manifest, or event context is missing, malformed, stale, or ambiguous
- **THEN** routing MUST select the consumer tier
- **THEN** it MUST NOT silently skip analysis

#### Scenario: Pre-commit execution is capped below the semantic tier
- **WHEN** the local pre-commit hook routes a staged diff
- **THEN** it MUST execute at most the documentation or consumer tier locally
- **THEN** explicit Make targets MUST remain available to run the consumer gates on demand

#### Scenario: Continuous integration routes from the cumulative pull-request diff
- **WHEN** a pull-request event triggers the lint workflow
- **THEN** the goplint consumer gates MUST run against the pinned analyzer for the full change
- **THEN** analyzer-soundness assurance MUST derive from the pinned goplint release, not from re-executing goplint's semantic populations in invowk

### Requirement: One exact-tree repository analysis serves all read-only audit consumers
The goplint quality gate SHALL execute at most one canonical package/analyzer traversal per exact execution plan for full-scan enforcement, baseline comparison, and stale-exception matching.

#### Scenario: Multiple audit verdicts are required
- **WHEN** full-scan, baseline, and stale-exception verdicts are required for the same tree
- **THEN** one canonical superset analysis MUST emit a machine-readable repository-audit result
- **THEN** each verdict MUST be derived from that result without loading or analyzing the packages again

#### Scenario: Audit input identity differs
- **WHEN** the workspace, analyzer binary, toolchain, package census, flags, baseline, exception configuration, or semantic manifest differs from the repository-audit binding
- **THEN** the consumer MUST reject the result
- **THEN** a fresh canonical analysis MUST run before any verdict can pass

#### Scenario: Review dates are audited
- **WHEN** exception review dates are checked
- **THEN** the audit MUST parse and validate configuration without loading Go packages
- **THEN** malformed or overdue entries MUST remain blocking

### Requirement: Goplint gate performance is observable and regression bounded
Invowk SHALL enforce a consumer performance smoke against its live tree so that goplint scan cost over invowk's codebase remains within reviewed catastrophic-regression limits, while statistical performance certification of the analyzer is governed by the `invowk/goplint` repository against a pinned invowk reference corpus.

#### Scenario: Work unit completes
- **WHEN** the consumer performance smoke runs locally or in CI
- **THEN** it MUST measure one full repository scan with the pinned analyzer against reviewed wall-time and peak-memory limits stored in `.goplint/`
- **THEN** exceeding a catastrophic limit MUST fail the gate

#### Scenario: Optimized executor is compared with the serial reference
- **WHEN** executor-level performance properties of the analyzer need proof
- **THEN** the `invowk/goplint` repository's harness gates govern them; invowk relies on the pinned release

#### Scenario: Consumer profile performance is accepted
- **WHEN** the consumer smoke passes
- **THEN** automation and documentation MUST NOT present it as analyzer performance certification
- **THEN** certification claims MUST reference the goplint repository's multi-sample certification against its reference corpus

