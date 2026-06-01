## ADDED Requirements

### Requirement: Pinned mutation-testing toolchain
Invowk SHALL use an exact pinned Go mutation-testing tool version for local and CI mutation runs, and SHALL NOT install or invoke an unversioned or `latest` mutation-testing tool in repository automation.

#### Scenario: Tool version is pinned
- **WHEN** maintainers inspect mutation-testing configuration, Make targets, scripts, and workflows
- **THEN** every mutation-testing tool install or invocation SHALL reference an exact version or a Go tool dependency resolved through `go.mod`
- **THEN** no mutation-testing automation SHALL use `@latest`, floating tags, or unversioned install scripts

#### Scenario: Tool version is verified in CI
- **WHEN** a mutation-testing CI job installs or resolves the mutation-testing tool
- **THEN** the job SHALL verify the resolved binary version before running mutants
- **THEN** a mismatch between the expected and resolved version SHALL fail the job before mutation execution

#### Scenario: Version policy stays synchronized
- **WHEN** the pinned mutation-testing tool version changes
- **THEN** Invowk SHALL update every workflow, script, Make target, and agent-facing version-pinning rule that references the version

### Requirement: Mutation run profiles
Invowk SHALL provide named mutation run profiles for changed-line PR feedback, scheduled broad scans, baseline updates, dry-runs, and focused single-mutant reruns.

#### Scenario: Changed-line PR profile runs
- **WHEN** a pull request mutation profile runs against a branch with Go production-line changes
- **THEN** Invowk SHALL mutate only changed eligible Go source lines relative to the configured base ref
- **THEN** the profile SHALL exit successfully when no eligible mutations are generated for the change

#### Scenario: Scheduled full profile runs
- **WHEN** a scheduled or manual full mutation profile runs
- **THEN** Invowk SHALL use curated target manifests for the root module and the `tools/goplint` module
- **THEN** the profile SHALL write separate reports for each module profile that ran

#### Scenario: Baseline update profile runs intentionally
- **WHEN** maintainers run the baseline update profile
- **THEN** Invowk SHALL regenerate the accepted-survivor baseline for the selected profile
- **THEN** the command SHALL make clear that the baseline update is an intentional maintenance operation

#### Scenario: Single mutant rerun is supported
- **WHEN** maintainers provide a stable escaped-mutant ID to the focused rerun profile
- **THEN** Invowk SHALL rerun only that mutant for the selected module profile
- **THEN** the command SHALL preserve enough report output to guide a targeted killing test

### Requirement: Safe local execution
Invowk SHALL protect developer worktrees from accidental source mutation during local mutation-testing runs.

#### Scenario: Dirty worktree is rejected
- **WHEN** a local mutation run would rewrite Go source files in place and the worktree contains uncommitted tracked changes outside approved report or baseline outputs
- **THEN** Invowk SHALL fail before mutation execution with an actionable message

#### Scenario: Clean local run restores sources
- **WHEN** a local mutation run completes, fails, or is interrupted after mutating source files
- **THEN** Invowk SHALL restore mutated source files to their pre-run contents or run the mutation tool in an isolated temporary worktree

#### Scenario: Dry-run does not mutate sources
- **WHEN** maintainers run the mutation dry-run profile
- **THEN** Invowk SHALL count or list candidate mutants without executing tests against mutated source files
- **THEN** the command SHALL NOT leave source changes in the worktree

### Requirement: Target selection and exclusions
Invowk SHALL select mutation targets deliberately so mutation testing measures production Go behavior rather than generated files, fixtures, docs, website assets, or test-only support code.

#### Scenario: Root module target manifest excludes non-production surfaces
- **WHEN** the root module mutation profile resolves its targets
- **THEN** it SHALL include eligible production packages under `cmd/`, `internal/`, and `pkg/`
- **THEN** it SHALL exclude `tests/`, `website/`, `docs/`, `samples/`, `specs/`, `openspec/`, generated artifacts, testdata fixtures, and Go test files

#### Scenario: Goplint target manifest runs from nested module
- **WHEN** the `tools/goplint` mutation profile runs
- **THEN** Invowk SHALL execute it from the `tools/goplint` module root with that module's dependency graph
- **THEN** its reports and baseline SHALL be kept separate from the root-module mutation reports and baseline

#### Scenario: Packages without local test ownership are visible
- **WHEN** a target manifest includes production packages that have no local Go tests
- **THEN** the mutation workflow SHALL either exclude them with an explicit rationale or report them as not covered rather than hiding them silently

### Requirement: Baseline and gate behavior
Invowk SHALL support brownfield adoption by distinguishing known accepted surviving mutants from new escaped mutants.

#### Scenario: Known survivors do not fail new-code gate
- **WHEN** a blocking mutation profile runs with a baseline
- **THEN** mutants already accepted in the baseline SHALL NOT fail the job
- **THEN** newly escaped mutants outside the baseline SHALL fail the job

#### Scenario: Advisory mode never blocks implementation
- **WHEN** a mutation profile is configured as advisory
- **THEN** escaped mutants SHALL be reported through logs, annotations, and artifacts
- **THEN** the job SHALL NOT fail solely because mutants escaped

#### Scenario: Blocking mode fails on new escapes
- **WHEN** a mutation profile is configured as blocking
- **THEN** the job SHALL fail when a new escaped mutant is detected or when configured mutation score gates are not met

#### Scenario: Baselines can shrink
- **WHEN** maintainers add tests that kill previously accepted survivors
- **THEN** the baseline update profile SHALL allow the accepted-survivor baseline to be regenerated with those mutants removed

### Requirement: Reports and annotations
Invowk SHALL produce machine-readable mutation-testing reports, human-readable CI output, and GitHub annotations suitable for review and agent-assisted test improvement.

#### Scenario: PR report artifacts are uploaded
- **WHEN** a pull request mutation profile runs in CI
- **THEN** Invowk SHALL upload summary and escaped-mutant report artifacts for the profile
- **THEN** the artifacts SHALL be named so root-module and `tools/goplint` reports are distinguishable

#### Scenario: Escaped mutants are annotated
- **WHEN** a pull request mutation profile finds escaped mutants on changed lines
- **THEN** CI SHALL emit GitHub annotations that identify the affected file, line, mutator, and profile

#### Scenario: Agent-focused report is available
- **WHEN** a mutation profile finds escaped mutants
- **THEN** Invowk SHALL produce a report containing stable mutant IDs, diffs, surrounding context, and nearby test hints when the selected tool supports that output

#### Scenario: Summary metrics are machine-readable
- **WHEN** a mutation profile completes
- **THEN** Invowk SHALL produce machine-readable summary metrics including total mutants, killed mutants, escaped mutants, skipped or errored mutants, and mutation score when the selected tool supports those fields

### Requirement: CI integration
Invowk SHALL integrate mutation testing with GitHub Actions without increasing the cost of the regular test matrix by default.

#### Scenario: Regular test workflow remains separate
- **WHEN** `make test` or the existing CI test matrix runs
- **THEN** mutation testing SHALL NOT run as part of those commands unless a mutation-specific target or workflow is selected

#### Scenario: Pull request workflow fetches diff base
- **WHEN** a pull request mutation workflow uses changed-line filtering
- **THEN** checkout SHALL fetch enough Git history to compute the configured base diff accurately

#### Scenario: Scheduled workflow runs broad scans
- **WHEN** the scheduled mutation workflow runs
- **THEN** it SHALL execute the configured broad scan profiles and upload reports even when no pull request context exists

#### Scenario: Workflow permissions are minimal
- **WHEN** mutation-testing workflows run in GitHub Actions
- **THEN** they SHALL request only the permissions required for checkout, annotations, artifacts, and any explicitly configured report publication

### Requirement: Test execution boundaries
Invowk SHALL keep default mutation runs focused on package-level Go tests while allowing explicitly selected high-assurance execution profiles for cross-package or CLI-only oracles.

#### Scenario: Default mutation profile avoids race and container overhead
- **WHEN** a default mutation profile executes tests for each mutant
- **THEN** it SHALL NOT pass `-race`, run container engine tests, or run CLI `testscript` suites unless explicitly configured for that profile

#### Scenario: Focused high-assurance profile can use custom execution
- **WHEN** maintainers select a high-assurance mutation profile for a package whose behavior is primarily tested through cross-package or CLI tests
- **THEN** Invowk SHALL allow that profile to use an explicit custom execution command or test flag set

#### Scenario: Integration profiles are opt-in
- **WHEN** a mutation profile would require Docker, Podman, tmux, network access, platform-specific runners, or long-running CLI fixtures
- **THEN** that profile SHALL be opt-in and SHALL document its expected prerequisites and runtime cost

### Requirement: Documentation and validation
Invowk SHALL document mutation-testing usage and validate the wrapper logic without requiring a full mutation scan in normal validation.

#### Scenario: Developer commands are documented
- **WHEN** maintainers read agent-facing command documentation
- **THEN** it SHALL list the mutation-testing Make targets or scripts, required tools, baseline update workflow, and report locations

#### Scenario: Wrapper logic has tests
- **WHEN** repository script tests run
- **THEN** they SHALL validate mutation target resolution, dirty-worktree protection, report path calculation, and command construction without executing a full mutation scan

#### Scenario: OpenSpec and agent docs remain synchronized
- **WHEN** implementation changes `AGENTS.md`, `.agents/rules/`, or `.agents/skills/` for mutation-testing guidance
- **THEN** `make check-agent-docs` SHALL pass before the change is complete
