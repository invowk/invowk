## MODIFIED Requirements

### Requirement: Mutation run profiles
Invowk SHALL provide named mutation run profiles for changed-line PR feedback, scheduled broad scans, baseline updates, dry-runs, and focused single-mutant reruns, scoped to the root Go module. Goplint's mutation profile migrates to the `invowk/goplint` repository.

#### Scenario: Changed-line PR profile runs
- **WHEN** a pull request mutation profile runs against a branch with Go production-line changes
- **THEN** Invowk SHALL mutate only changed eligible Go source lines relative to the configured base ref
- **THEN** the profile SHALL exit successfully when no eligible mutations are generated for the change

#### Scenario: Scheduled full profile runs
- **WHEN** a scheduled or manual full mutation profile runs
- **THEN** Invowk SHALL use the curated target manifest for the root module
- **THEN** the profile SHALL write reports for the root-module profile

#### Scenario: Baseline update profile runs intentionally
- **WHEN** maintainers run the baseline update profile
- **THEN** Invowk SHALL regenerate the accepted-survivor baseline for the root-module profile
- **THEN** the command SHALL make clear that the baseline update is an intentional maintenance operation

#### Scenario: Single mutant rerun is supported
- **WHEN** maintainers provide a stable escaped-mutant ID to the focused rerun profile
- **THEN** Invowk SHALL rerun only that mutant for the root-module profile
- **THEN** the command SHALL preserve enough report output to guide a targeted killing test

#### Scenario: Goplint mutation coverage lives with goplint
- **WHEN** maintainers need mutation testing over goplint's analyzer sources
- **THEN** the `invowk/goplint` repository MUST provide the equivalent curated-manifest profile, baseline, and workflow
- **THEN** invowk automation MUST NOT offer a `goplint` module selection it can no longer execute

### Requirement: Target selection and exclusions
Invowk SHALL select mutation targets deliberately so mutation testing measures production Go behavior rather than generated files, fixtures, docs, website assets, test-only support code, or surfaces that require a separate high-assurance oracle.

#### Scenario: Root module target manifest excludes non-production surfaces
- **WHEN** the root module mutation profile resolves its targets
- **THEN** it SHALL use a curated set of eligible production packages under `internal/` and `pkg/` for the initial full profile
- **THEN** it SHALL exclude `tests/`, `website/`, `docs/`, `samples/`, `specs/`, `openspec/`, generated artifacts, testdata fixtures, and Go test files
- **THEN** large adapter, runtime, TUI, audit, or container surfaces SHALL remain opt-in until advisory timing and survivor data justify adding them to a baselineable full profile

#### Scenario: Packages without local test ownership are visible
- **WHEN** a target manifest includes production packages or file targets whose owning package has no local Go tests
- **THEN** the mutation workflow SHALL either exclude them with an explicit rationale or report them as not covered rather than hiding them silently
