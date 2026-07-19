## MODIFIED Requirements

### Requirement: Goplint exception governance is enforced
Invowk SHALL keep `tools/goplint` baseline and exception governance aligned with its lint, type-system, canonical semantic-analysis, and soundness-assurance quality gates.

#### Scenario: Goplint lint and soundness gates run together
- **WHEN** repository lint gates run
- **THEN** `tools/goplint` golangci-lint checks MUST run in addition to the custom goplint analyzer gates
- **THEN** the canonical goplint soundness-assurance gate MUST run without a legacy, alternate, fallback, or weakened semantic path
- **THEN** neither custom analyzer nor soundness gates MUST be treated as a replacement for the module's golangci-lint config

#### Scenario: Accepted goplint exceptions are reviewable
- **WHEN** a goplint exception is kept in `tools/goplint/exceptions.toml`
- **THEN** it MUST include a reason that explains why the exception remains acceptable
- **THEN** long-lived or broad exceptions MUST include a review date or equivalent review mechanism

#### Scenario: Stale goplint exception audit is part of quality gates
- **WHEN** repository quality gates run locally or in CI
- **THEN** stale, overdue, malformed, or unsupported goplint exceptions MUST be reported
- **THEN** the gate MUST fail for stale or malformed exceptions unless the design explicitly marks a temporary advisory transition inside this same change

#### Scenario: Baseline uses the only production semantics
- **WHEN** `make check-baseline` or `make update-baseline` invokes goplint
- **THEN** it MUST use the same canonical production analysis and fail-closed aggregation as the blocking full repository scan
- **THEN** it MUST NOT retain a legacy fact reader, AST fallback, alternate evaluator, hidden selector, or mode-specific stable-ID path
- **THEN** stable finding ID changes MUST be reported and reviewed before the baseline is accepted

#### Scenario: Goplint baseline wording matches behavior
- **WHEN** baseline tooling, goplint documentation, or agent guidance describes baseline behavior
- **THEN** it MUST distinguish baseline-suppressed categories from always-visible hard-blocking categories
- **THEN** stale statements about accepted counts, alternate semantics, or advisory soundness scans MUST be removed

#### Scenario: Canonical full scan is blocking
- **WHEN** the repository goplint full scan runs locally, in pre-commit, or in CI
- **THEN** violations, blocking inconclusive outcomes, malformed evidence, incomplete catalog/oracle coverage, surviving or non-causal soundness mutants, legacy-path detections, and analyzer failures MUST fail the gate
- **THEN** the workflow MUST NOT downgrade or mask those outcomes

### Requirement: Documentation and verification remain synchronized
Invowk SHALL update documentation and validation so contributors can run, understand, and trust the lint and canonical goplint soundness-assurance gates.

#### Scenario: Command documentation lists complete lint workflow
- **WHEN** contributors read `.agents/rules/commands.md`, `AGENTS.md`, Make help, or goplint documentation
- **THEN** they MUST see how to run root lint, `tools/goplint` lint, formatter and config checks, exception governance, full scan, and the aggregate soundness-assurance gate
- **THEN** documented commands and guarantee claims MUST match implemented targets, CI jobs, production code paths, and retained evidence

#### Scenario: Agent documentation sync check passes
- **WHEN** implementation changes `AGENTS.md`, `.agents/rules/`, or `.agents/skills/`
- **THEN** `make check-agent-docs` MUST pass before the change is complete

#### Scenario: Final validation proves production semantics and evidence integrity
- **WHEN** this change is complete
- **THEN** maintainers MUST run both-module lint, formatter, config, test, race, repeat, baseline, exception, full-scan, performance, and agent-document gates
- **THEN** maintainers MUST run real-analyzer counterexamples, catalog completeness, bounded independent oracle, meaningful deterministic fuzz seeds, real package-order determinism, causal targeted mutation, legacy-path absence, and strict OpenSpec validation
- **THEN** every required gate MUST pass in the recorded clean synthetic-tree worktree without optimistic uncertainty, hidden compatibility behavior, missing evidence, or unreviewed baseline drift
