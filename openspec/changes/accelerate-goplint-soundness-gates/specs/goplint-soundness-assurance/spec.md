## ADDED Requirements

### Requirement: Soundness execution plans are immutable and exhaustive
Every concurrent or distributed goplint soundness run SHALL be governed by one versioned immutable execution plan that binds all required work and evidence to the exact analyzed tree.

#### Scenario: Execution plan is created
- **WHEN** a soundness profile is planned
- **THEN** the plan MUST bind the profile, workspace, manifest, registry, toolchain, commands, resource policy, complete work-unit census, dependencies, and expected reports
- **THEN** identical inputs and timing metadata MUST produce byte-identical normalized plans

#### Scenario: Required work is missing or duplicated
- **WHEN** aggregate results omit a planned work unit, contain a duplicate, overlap a shard census, or report an unplanned unit
- **THEN** aggregation MUST fail before any soundness claim is accepted
- **THEN** completed unrelated work MUST NOT compensate for the census defect

#### Scenario: Worker identity differs from the plan
- **WHEN** a worker observes a different workspace, manifest, registry, toolchain, command, binary, or test census from the execution plan
- **THEN** the worker result MUST be rejected
- **THEN** the plan MUST be regenerated or the worker environment corrected before execution can pass

### Requirement: Concurrent and distributed evidence preserves causal bindings
Parallel local workers and distributed CI workers SHALL emit the same category-specific causal observations as serial execution, and final aggregation SHALL accept them only after exact-plan and no-gap validation.

#### Scenario: Independent subgates run concurrently
- **WHEN** manifest dependencies and resource reservations allow multiple subgates to run at once
- **THEN** each subgate MUST receive an isolated observation and report directory
- **THEN** concurrent output ordering or completion order MUST NOT affect normalized evidence or verdicts

#### Scenario: CI matrix workers return evidence bundles
- **WHEN** work units execute on separate CI runners
- **THEN** each bundle MUST contain its plan identity, exact work-unit identity, command and input digests, required populations, observations, and terminal outcome
- **THEN** the final aggregator MUST recompute the complete expected set and reject missing, foreign, stale, or conflicting bundles

#### Scenario: Worker is canceled after another failure
- **WHEN** fail-fast cancellation stops a required worker
- **THEN** the aggregate run MUST fail and record the canceled population as absent
- **THEN** partial evidence from the canceled worker MUST NOT be accepted as complete

### Requirement: Optimized profiles preserve their declared semantic populations
Execution reuse, sharding, scheduling, and profile routing SHALL reduce duplicated work and wall time without reducing any semantic, oracle, mutation, fuzz-seed, determinism, race, repeat, scan, benchmark, or completion population declared by the selected profile.

#### Scenario: Semantic profile replaces the serial core topology
- **WHEN** the semantic profile runs through optimized local or distributed execution
- **THEN** its validated population census MUST equal the pre-optimization canonical core census
- **THEN** shared repository-audit evidence MUST identify every prior consumer it replaces

#### Scenario: Completion profile runs
- **WHEN** exhaustive completion is requested
- **THEN** every semantic-profile population and clean-tree freshness requirement MUST be present
- **THEN** performance certification, exact-tree binding, and retained proof verification MUST remain blocking

#### Scenario: Consumer profile runs
- **WHEN** conservative change classification selects the consumer profile
- **THEN** the canonical repository analyzer MUST still execute against every governed root package
- **THEN** all violations, always-visible inconclusive findings, baseline regressions, and exception-governance failures MUST remain blocking
- **THEN** the result MUST state that unchanged analyzer soundness populations were not re-executed and MUST NOT claim completion evidence
