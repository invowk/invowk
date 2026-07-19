## MODIFIED Requirements

### Requirement: Documentation and verification remain synchronized
Invowk SHALL update documentation and validation so contributors can run, understand, and trust the lint and canonical goplint soundness gates, including the generation and verification of the retained exact-tree evidence bundle used for completion claims.

#### Scenario: Command documentation lists complete lint workflow
- **WHEN** contributors read `.agents/rules/commands.md`, `AGENTS.md`, Make help, or goplint documentation
- **THEN** they MUST see how to run root lint, `tools/goplint` lint, formatter checks, config verification, goplint exception checks, the canonical goplint soundness gate, and both the generation and verification of the retained exact-tree evidence bundle used by `make check-goplint-soundness-complete`
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

## ADDED Requirements

### Requirement: Mutation-kernel coverage contract is a documented blocking subgate
Invowk SHALL document the blocking mutation-kernel category-coverage contract in the same documentation surfaces that describe other blocking goplint gates, so contributors can see that mutation coverage is a first-class blocking requirement rather than an implementation detail.

#### Scenario: Kernel coverage subgate appears in documentation
- **WHEN** contributors read `.agents/rules/commands.md`, `.agents/rules/checklist.md`, `tools/goplint/AGENTS.md`, or `tools/goplint/README.md`
- **THEN** they MUST see that the blocking mutation profile MUST cover every semantic category whose registered rule in `tools/goplint/spec/semantic-rules.v1.json` requires the `mutation` evidence layer
- **AND** they MUST see the exact command that runs the kernel-coverage subgate
- **AND** they MUST see that the subgate cannot be baselined, excepted, or inline-ignored

#### Scenario: Repo hygiene excludes goplint test binaries
- **WHEN** contributors build or run goplint tests locally
- **THEN** the resulting `tools/goplint/**/*.test` binaries MUST be ignored by `.gitignore`
- **AND** `git status` MUST NOT list them as untracked candidates for accidental staging
