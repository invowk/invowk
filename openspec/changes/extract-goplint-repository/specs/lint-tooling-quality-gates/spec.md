## MODIFIED Requirements

### Requirement: Lint automation covers every Go module
Invowk SHALL lint the root Go module — its only Go module after the goplint extraction — anywhere the repository advertises full lint coverage, and SHALL NOT advertise coverage of external tool repositories it does not lint.

#### Scenario: Make lint covers the root module
- **WHEN** maintainers run `make lint`
- **THEN** linting MUST run against the root module with the root golangci-lint config
- **THEN** lint automation MUST NOT reference a nested `tools/goplint` module that no longer exists in this repository

#### Scenario: CI lint coverage matches Make lint coverage
- **WHEN** the lint workflow runs in GitHub Actions
- **THEN** it MUST lint the root module and fail on config-validation failures or lint findings
- **THEN** workflow comments and job names MUST NOT imply coverage of the external goplint repository

#### Scenario: Goplint lint coverage lives with goplint
- **WHEN** maintainers inspect where goplint's own Go sources are linted
- **THEN** the `invowk/goplint` repository MUST provide equivalent golangci-lint, formatter, and config-verification gates for its module
- **THEN** invowk documentation MUST point to that repository for goplint-internal lint coverage

### Requirement: Goplint exception governance is enforced
Invowk SHALL keep its goplint baseline and exception governance — stored under the repository-owned `.goplint/` directory — aligned with its lint, type-system, and canonical semantic-analysis quality gates, executed against the exact pinned goplint tool version.

#### Scenario: Consumer gates run against the pinned analyzer
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

#### Scenario: Canonical full scan is blocking
- **WHEN** the repository goplint full scan runs locally, in pre-commit, or in CI
- **THEN** violations, blocking inconclusive outcomes, malformed evidence, incomplete required evidence for the selected profile, and analyzer failures MUST fail the gate
- **THEN** the workflow MUST NOT downgrade or mask those outcomes

### Requirement: Automatic goplint assurance routing is conservative and change aware
Repository automation SHALL classify every changed invowk path into exactly one reviewed change class — `documentation` or `consumer` — through a versioned invowk-owned ownership manifest, SHALL run the consumer tier for any diff containing a consumer-class or unmatched path, and SHALL fail closed to the consumer tier whenever classification is missing, malformed, or ambiguous. Analyzer-semantics and harness assurance tiers are governed by the `invowk/goplint` repository against its own tree.

#### Scenario: Documentation and bookkeeping changes route to the documentation tier
- **WHEN** a diff changes only paths in the `documentation` class
- **THEN** automation MUST NOT trigger a repository audit or analyzer execution for that diff

#### Scenario: Consumer code changes run the consumer tier
- **WHEN** a diff changes any root-module code, configuration under `.goplint/`, or any path not matched by a documentation rule
- **THEN** automation MUST run one blocking canonical repository audit with baseline and exception governance against the pinned analyzer

#### Scenario: Change context is missing or ambiguous
- **WHEN** the merge base, changed-path census, ownership manifest, or event context is missing, malformed, stale, or ambiguous
- **THEN** routing MUST select the consumer tier
- **THEN** it MUST NOT silently skip analysis

#### Scenario: Pre-commit runs the routed consumer ceiling
- **WHEN** the local pre-commit hook routes a staged diff
- **THEN** it MUST execute at most the documentation or consumer tier locally
- **THEN** explicit Make targets MUST remain available to run the consumer gates on demand

#### Scenario: Analyzer assurance is pinned, not re-derived
- **WHEN** invowk changes do not modify goplint itself
- **THEN** invowk automation MUST NOT re-execute goplint's semantic soundness populations
- **THEN** analyzer-soundness assurance MUST derive from the pinned goplint release, whose own repository gates those populations before release

### Requirement: Documentation and verification remain synchronized
Invowk SHALL update documentation and validation so contributors can run, understand, and trust the lint and goplint consumer gates against the pinned analyzer version.

#### Scenario: Command documentation lists complete lint workflow
- **WHEN** contributors read `.agents/rules/commands.md`, `AGENTS.md`, Make help, or goplint consumer documentation
- **THEN** they MUST see how to run root lint, formatter and config checks, exception governance, baseline comparison, full scan, and the consumer performance smoke
- **THEN** documented commands and guarantee claims MUST match implemented targets, CI jobs, and the pinned tool version

#### Scenario: Agent documentation sync check passes
- **WHEN** implementation changes `AGENTS.md`, `.agents/rules/`, or `.agents/skills/`
- **THEN** `make check-agent-docs` MUST pass before the change is complete

#### Scenario: Goplint-internal assurance documentation lives with goplint
- **WHEN** contributors need the analyzer-soundness, harness, or completion-evidence documentation
- **THEN** invowk documentation MUST point to the `invowk/goplint` repository as the authoritative source
- **THEN** invowk MUST NOT retain stale copies that can drift from the tool's actual behavior

### Requirement: Goplint gate performance is observable and regression bounded
Invowk SHALL enforce a consumer performance smoke against its live tree so that goplint scan cost over invowk's codebase remains within reviewed catastrophic-regression limits, while statistical performance certification of the analyzer is governed by the `invowk/goplint` repository against a pinned invowk reference corpus.

#### Scenario: Consumer smoke bounds live-tree scan cost
- **WHEN** the consumer performance smoke runs locally or in CI
- **THEN** it MUST measure one full repository scan with the pinned analyzer against reviewed wall-time and peak-memory limits stored in `.goplint/`
- **THEN** exceeding a catastrophic limit MUST fail the gate

#### Scenario: Smoke is not certification
- **WHEN** the consumer smoke passes
- **THEN** automation and documentation MUST NOT present it as analyzer performance certification
- **THEN** certification claims MUST reference the goplint repository's multi-sample certification against its reference corpus

## REMOVED Requirements

### Requirement: Soundness evidence is category-specific and causally executed
**Reason**: Goplint self-verification; migrates to the `invowk/goplint` repository's docs-guard-governed assurance documentation and CI gates.

### Requirement: Aggregate soundness orchestration rejects vacuous subgates
**Reason**: Goplint harness behavior; migrates with the distributed soundness executor to `invowk/goplint`.

### Requirement: Independent evidence exercises integrated production semantics
**Reason**: Goplint self-verification (oracles, counterexamples); migrates to `invowk/goplint`.

### Requirement: Clean-tree completion evidence is freshness checked
**Reason**: The v4 retained record binds invowk git objects and is retired at cutover; its successor (v5) is defined over the goplint tree and governed in `invowk/goplint`.

### Requirement: Finding identities are globally scoped and source-layout independent
**Reason**: Analyzer contract; governed in `invowk/goplint`. Invowk continues to rely on it through the pinned version and its baseline review scenario in the modified exception-governance requirement.

### Requirement: Goplint directives are total and fail visibly
**Reason**: Analyzer contract; governed in `invowk/goplint`. Invowk's `//goplint:` directives continue to be interpreted by the pinned analyzer.

### Requirement: Semantic evidence credit matches executed production stages
**Reason**: Goplint evidence-registry semantics; migrates to `invowk/goplint`.

### Requirement: Aggregate subgate populations are executable censuses
**Reason**: Goplint harness behavior; migrates to `invowk/goplint`.

### Requirement: Mutation kills prove the intended mismatch
**Reason**: Goplint mutation-kernel assurance; migrates to `invowk/goplint`.

### Requirement: Completion proof covers the complete dependent diff
**Reason**: Completion-evidence machinery; migrates to `invowk/goplint` with the v5 record.

### Requirement: Mutation-kernel coverage contract is a documented blocking subgate
**Reason**: Goplint mutation-kernel assurance; migrates to `invowk/goplint`.

### Requirement: Local goplint execution is resource aware and bounded
**Reason**: Property of the soundness-gate executor, which migrates to `invowk/goplint`. Invowk's remaining consumer gates are single-audit and need no resource-aware scheduling.

### Requirement: Race and repeat execution uses exhaustive balanced work units
**Reason**: Goplint test-suite assurance; migrates to `invowk/goplint`.

### Requirement: Goplint documentation is statically anchored to executable evidence
**Reason**: docs-guard and its governed documents migrate to `invowk/goplint`, re-anchored to that repository's Makefile and tree.
