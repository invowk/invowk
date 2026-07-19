## ADDED Requirements

### Requirement: Blocking mutation kernel spans every semantic category
The blocking targeted-mutation profile SHALL include at least one causal-kill mutant per semantic category whose entry in `tools/goplint/spec/semantic-rules.v1.json` requires the `mutation` evidence layer, so that no mutation-required semantic category can regress silently.

#### Scenario: Blocking mutation kernel covers every registered category
- **WHEN** the blocking mutation profile is loaded
- **THEN** the selected mutants' registered category, `changed_stages`, and `expected_mismatches` metadata MUST bind at least one causal mutant to every category whose registered rule requires the `mutation` evidence layer
- **AND** the gate MUST fail when any mutation-required category has zero mutants in the blocking profile

#### Scenario: Kernel coverage is asserted by a subgate
- **WHEN** `make check-goplint-soundness-core` runs
- **THEN** a mutation-kernel-coverage subgate MUST census-count kernel mutants by mutation-required category
- **AND** produce structured evidence that binds each covered category to the exact mutant IDs that cover it
- **AND** fail the aggregate soundness gate when any category is uncovered, when the census is empty, or when the census is a successful no-op

#### Scenario: Kernel coverage cannot be baselined or excepted
- **WHEN** a category is uncovered in the blocking mutation kernel
- **THEN** the failure MUST NOT be suppressible by baseline, exception, or inline-ignore mechanisms
- **AND** the resolution MUST be to add a covering mutant to the blocking profile rather than to weaken the coverage contract

### Requirement: Completion-proof evidence is regeneratable and documented
The retained exact-tree run record required by `make check-goplint-soundness-complete` SHALL have a documented generation command that maintainers can invoke against the reviewed path selection and command plan before verifying it, so completion claims are reproducible from the documented workflow alone.

#### Scenario: Generation command is documented alongside verification
- **WHEN** maintainers read `.agents/rules/commands.md`, `tools/goplint/AGENTS.md`, `tools/goplint/README.md`, or `tools/goplint/CLAUDE.md`
- **THEN** they MUST see both the command that generates the retained exact-tree run record and the command that verifies it
- **AND** the documented generation command MUST match the implementation used by maintainers to bind a completion claim
- **AND** the documentation MUST state that record generation invokes the `core` profile rather than the `complete` profile to avoid recursive freshness verification

#### Scenario: Generation command refreshes the run record the verifier consumes
- **WHEN** the documented generation command runs with the reviewed `paths` and `plan` inputs on a clean synthetic tree
- **THEN** it MUST produce the `run` record under `tools/goplint/testdata/gates/` at the path referenced by `make check-goplint-clean-tree-evidence`
- **AND** `make check-goplint-clean-tree-evidence` MUST succeed against the reviewed inputs and generated record without modifying the caller's Git index or worktree

#### Scenario: Missing or stale record fails closed
- **WHEN** `make check-goplint-soundness-complete` runs against a missing or stale run record
- **THEN** the gate MUST fail with a message that identifies the missing or stale artifact and references the documented generation command
- **AND** the failure MUST NOT be suppressible by baseline, exception, or inline-ignore mechanisms
