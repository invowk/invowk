## ADDED Requirements

### Requirement: Soundness evidence is category-specific and causally executed
Every soundness evidence layer SHALL identify the exact goplint category and semantic feature it exercises, SHALL execute through its declared production or independent boundary, and SHALL emit a machine-verifiable observation consumed by the blocking gate. Semantic-kind predicates, source-file existence, test-name markers, nonempty fuzz seeds, or shared generic evidence MUST NOT award category coverage by themselves.

#### Scenario: New protocol category has no inherited evidence
- **WHEN** a protocol category is added without category-specific production, independent-model, metamorphic, fuzz, mutation, and determinism observations
- **THEN** the semantic census and aggregate soundness gate MUST fail
- **AND** generic evidence registered for other protocol categories MUST NOT satisfy the missing layers

#### Scenario: Category evidence reaches extraction and reporting
- **WHEN** a protocol category claims production-boundary coverage
- **THEN** its evidence MUST exercise applicable source extraction, identity and graph construction, propagation, refinement, aggregation, and diagnostic reporting
- **AND** a direct component call or marker-only artifact MUST be labeled supporting evidence rather than end-to-end proof

#### Scenario: Historical fuzz seed proves its declared feature
- **WHEN** the audit matrix maps a historical counterexample to a committed fuzz seed
- **THEN** the gate MUST decode that seed, observe the exact declared semantic structure, and demonstrate the independent property that detects the counterexample
- **AND** nonempty input or an unrelated shared graph shape MUST NOT count as coverage

### Requirement: Aggregate soundness orchestration rejects vacuous subgates
The aggregate goplint soundness gate SHALL execute every required subgate through a canonical machine-readable manifest and SHALL validate the expected evidence from that execution. Target names, dependency declarations, recipe text, test definitions, or marker strings alone MUST NOT prove that a subgate ran.

#### Scenario: Required recipe is replaced by a no-op
- **WHEN** an adversarial gate test replaces any required subgate command with a successful no-op
- **THEN** the gate contract MUST fail because the required evidence was not produced by the declared command

#### Scenario: Empty evidence population cannot pass
- **WHEN** a subgate executes with zero admitted programs, categories, seeds, mutants, deterministic reorderings, benchmarks, or counterexamples where a nonzero population is required
- **THEN** the subgate and aggregate gate MUST fail

#### Scenario: Unrelated failure is not causal evidence
- **WHEN** a mutation or adversarial run fails for compilation, timeout, crash, unrelated test failure, or a pre-existing failing control
- **THEN** the gate MUST reject the result as non-causal
- **AND** it MUST NOT report the intended semantic guard as proven

### Requirement: Independent evidence exercises integrated production semantics
Generated comparison, fuzzing, perturbation, scheduled profiles, and analyzer benchmarks SHALL exercise the integrated production analyzer dimensions named by their manifests. The independent reference model MUST represent the corresponding facts, aliases, constraints, call sites, and realizable call/return behavior without calling production semantic helpers.

#### Scenario: Evidence corruption enters before production validation
- **WHEN** an end-to-end perturbation corrupts witness, refinement, reason, or summary evidence
- **THEN** corruption MUST be injected before production evidence checking and aggregation
- **AND** editing the analyzer result after execution MUST NOT satisfy the perturbation requirement

#### Scenario: Differential fuzzing couples solver dimensions
- **WHEN** a fuzz program declares aliases, constraints, procedures, call sites, or return edges
- **THEN** both the production analyzer and independent interpreter MUST use those dimensions in the compared outcome
- **AND** realizability, alias, and constraint properties MUST NOT be checked only as disconnected component laws

#### Scenario: Scheduled profile compares the real analyzer
- **WHEN** the scheduled oracle profile runs
- **THEN** it MUST enumerate a manifest-derived strict superset of the blocking corpus and compare every admitted case with the production analyzer
- **AND** a documented Make or CI surface MUST invoke it with a derived, self-checked case count

#### Scenario: Generated-analysis benchmark measures the analyzer
- **WHEN** a benchmark is reported as generated analyzer performance
- **THEN** it MUST include parsing, typing, SSA extraction, graph construction, propagation, aggregation, and reporting for generated programs
- **AND** reference-interpreter-only timing MUST be named and budgeted separately

### Requirement: Clean-tree completion evidence is freshness checked
The soundness workflow SHALL provide a blocking verifier that recomputes the exact synthetic tree and intended-diff identity and validates that every required result was produced for that tree after final artifact and task state. A retained evidence file without successful freshness verification MUST NOT satisfy completion.

#### Scenario: Intended diff changes after evidence generation
- **WHEN** any intended tracked or untracked content changes after the clean-tree proof is recorded
- **THEN** the freshness verifier and aggregate soundness gate MUST fail until the proof is rerun for the new synthetic tree

#### Scenario: Required result is absent or stale
- **WHEN** the evidence record omits a required subgate, counterexample, category observation, mutant attribution, manifest identity, toolchain identity, or final task-state identity
- **THEN** the verifier MUST reject the record with the missing or mismatched field

#### Scenario: Verification preserves the caller index
- **WHEN** the freshness verifier materializes and checks the intended tree
- **THEN** it MUST use a temporary index or equivalent isolated mechanism
- **AND** the caller's real index and worktree contents MUST remain byte-for-byte unchanged
