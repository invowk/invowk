# lint-tooling-quality-gates Specification

## Purpose
TBD - created by archiving change normalize-lint-tooling-and-expand-lint-coverage. Update Purpose after archive.
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
Invowk SHALL lint the root Go module and the nested `tools/goplint` Go module anywhere the repository advertises full lint coverage.

#### Scenario: Make lint covers both modules
- **WHEN** maintainers run `make lint`
- **THEN** linting MUST run against the root module with the root golangci-lint config
- **THEN** linting MUST run against `tools/goplint` from the `tools/goplint` module root with that module's golangci-lint config

#### Scenario: CI lint coverage matches Make lint coverage
- **WHEN** the lint workflow runs in GitHub Actions
- **THEN** it MUST lint the root module and `tools/goplint`
- **THEN** it MUST fail if either module's golangci-lint config fails validation or produces lint findings
- **THEN** workflow comments and job names MUST NOT imply coverage that is not actually executed

#### Scenario: Pre-commit lint coverage matches changed Go module surfaces
- **WHEN** pre-commit runs golangci-lint hooks
- **THEN** it MUST run the root-module lint gate when root-module Go files or shared lint configuration changes are staged
- **THEN** it MUST run the `tools/goplint` lint gate when `tools/goplint` Go files or its lint configuration changes are staged
- **THEN** it MUST provide a supported path to run both module lint gates together

#### Scenario: Nested module boundaries are explicit
- **WHEN** maintainers inspect lint automation
- **THEN** the automation MUST make clear that root `go list ./...` does not include `tools/goplint`
- **THEN** the nested-module lint invocation MUST NOT depend on accidental traversal from the root module

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
Invowk SHALL keep `tools/goplint` baseline and exception governance aligned with its lint and type-system quality gates.

#### Scenario: Goplint lint runs with regular lint gates
- **WHEN** repository lint gates run
- **THEN** `tools/goplint` golangci-lint checks MUST run in addition to custom goplint analyzer gates
- **THEN** the custom analyzer gates MUST NOT be treated as a replacement for the module's golangci-lint config

#### Scenario: Accepted goplint exceptions are reviewable
- **WHEN** a goplint exception is kept in `tools/goplint/exceptions.toml`
- **THEN** it MUST include a reason that explains why the exception remains acceptable
- **THEN** long-lived or broad exceptions MUST include a review date or equivalent review mechanism

#### Scenario: Stale goplint exception audit is part of quality gates
- **WHEN** repository quality gates run locally or in CI
- **THEN** stale, overdue, malformed, or unsupported goplint exceptions MUST be reported
- **THEN** the gate MUST fail for stale or malformed exceptions unless the design explicitly marks a temporary advisory transition inside this same change

#### Scenario: Goplint baseline wording matches behavior
- **WHEN** `make check-baseline`, goplint documentation, or agent guidance describes baseline behavior
- **THEN** it MUST distinguish baseline-suppressed categories from always-visible hard-blocking categories
- **THEN** stale statements about nonzero accepted baseline findings MUST be removed when the current baseline is empty

### Requirement: Documentation and verification remain synchronized
Invowk SHALL update documentation and validation so contributors can run, understand, and trust the lint quality gates.

#### Scenario: Command documentation lists complete lint workflow
- **WHEN** contributors read `.agents/rules/commands.md`, `AGENTS.md`, or Make help for linting
- **THEN** they MUST see how to run root lint, `tools/goplint` lint, formatter checks, config verification, and goplint exception checks
- **THEN** the documented commands MUST match implemented targets and CI jobs

#### Scenario: Agent documentation sync check passes
- **WHEN** implementation changes `AGENTS.md`, `.agents/rules/`, or `.agents/skills/`
- **THEN** `make check-agent-docs` MUST pass before the change is complete

#### Scenario: Final validation proves the one-shot gate
- **WHEN** this change is complete
- **THEN** maintainers MUST run the normalized full lint gate for both modules
- **THEN** maintainers MUST run formatter checks for both modules
- **THEN** maintainers MUST run golangci-lint config verification for both modules
- **THEN** maintainers MUST run goplint baseline and exception governance checks
- **THEN** all required gates MUST pass without relying on unresolved lint findings
