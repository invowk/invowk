## MODIFIED Requirements

### Requirement: Goplint exception governance is enforced
Invowk SHALL keep `tools/goplint` baseline and exception governance aligned with its lint, type-system, and canonical semantic-analysis quality gates.

#### Scenario: Goplint lint and soundness gates run together
- **WHEN** repository lint gates run
- **THEN** `tools/goplint` golangci-lint checks MUST run in addition to the custom goplint analyzer gates
- **THEN** the canonical goplint soundness gate MUST run without a legacy, alternate, or weakened semantic mode
- **THEN** neither custom analyzer gates nor soundness gates MUST be treated as a replacement for the module's golangci-lint config

#### Scenario: Accepted goplint exceptions are reviewable
- **WHEN** a goplint exception is kept in `tools/goplint/exceptions.toml`
- **THEN** it MUST include a reason that explains why the exception remains acceptable
- **THEN** long-lived or broad exceptions MUST include a review date or equivalent review mechanism

#### Scenario: Stale goplint exception audit is part of quality gates
- **WHEN** repository quality gates run locally or in CI
- **THEN** stale, overdue, malformed, or unsupported goplint exceptions MUST be reported
- **THEN** the gate MUST fail for stale or malformed exceptions unless the design explicitly marks a temporary advisory transition inside this same change

#### Scenario: Baseline uses canonical semantics
- **WHEN** `make check-baseline` or `make update-baseline` invokes goplint
- **THEN** it MUST use the same flagless canonical semantic pipeline as the blocking full repository scan
- **THEN** it MUST NOT pin a removed engine, backend, alias, feasibility, refinement, UBV, or inconclusive-policy mode
- **THEN** stable finding ID changes caused by the migration MUST be reported and reviewed before the baseline is accepted

#### Scenario: Goplint baseline wording matches behavior
- **WHEN** baseline tooling, goplint documentation, or agent guidance describes baseline behavior
- **THEN** it MUST distinguish baseline-suppressed categories from always-visible hard-blocking categories
- **THEN** stale statements about nonzero accepted baseline findings, legacy-pinned semantics, or advisory canonical scans MUST be removed

#### Scenario: Canonical full scan is blocking
- **WHEN** the repository goplint full scan runs locally, in pre-commit, or in CI
- **THEN** violations, blocking inconclusive outcomes, malformed structured evidence, and analyzer failures MUST fail the gate
- **THEN** the workflow MUST NOT downgrade the canonical scan to an advisory warning

### Requirement: Documentation and verification remain synchronized
Invowk SHALL update documentation and validation so contributors can run, understand, and trust the lint and canonical goplint soundness gates.

#### Scenario: Command documentation lists complete lint workflow
- **WHEN** contributors read `.agents/rules/commands.md`, `AGENTS.md`, Make help, or goplint documentation
- **THEN** they MUST see how to run root lint, `tools/goplint` lint, formatter checks, config verification, goplint exception checks, and the canonical goplint soundness gate
- **THEN** the documented commands MUST match implemented targets and CI jobs
- **THEN** removed rollout phases and semantic-mode flags MUST NOT remain documented as supported behavior

#### Scenario: Agent documentation sync check passes
- **WHEN** implementation changes `AGENTS.md`, `.agents/rules/`, or `.agents/skills/`
- **THEN** `make check-agent-docs` MUST pass before the change is complete

#### Scenario: Final validation proves the one-shot gates
- **WHEN** this change is complete
- **THEN** maintainers MUST run the normalized full lint gate for both Go modules
- **THEN** maintainers MUST run formatter checks and golangci-lint config verification for both modules
- **THEN** maintainers MUST run goplint baseline and exception governance checks using canonical semantics
- **THEN** maintainers MUST run the canonical semantic contract, independent oracle, deterministic fuzz-seed, mutation, race, full-scan, and benchmark gates
- **THEN** all required gates MUST pass without legacy paths, advisory canonical scans, unresolved lint findings, or optimistic inconclusive handling
