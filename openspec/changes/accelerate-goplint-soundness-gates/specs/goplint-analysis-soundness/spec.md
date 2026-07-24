## MODIFIED Requirements

### Requirement: Soundness gates are deterministic and performance bounded
The canonical soundness gate SHALL produce deterministic findings, reasons, witnesses, refinement evidence, work-unit censuses, and normalized aggregate evidence for identical inputs, and SHALL enforce documented smoke and certification performance budgets without converting exhausted analysis into safety.

#### Scenario: Repeated, reordered, or concurrently scheduled analysis is deterministic
- **WHEN** the same packages are analyzed repeatedly, in a different package/worklist order, or through a different valid concurrent schedule
- **THEN** normalized findings and evidence MUST be byte-for-byte stable after permitted timing and resource fields are removed

#### Scenario: Performance regression exceeds the certified budget
- **WHEN** canonical solver benchmarks or full repository scans exceed their reviewed certification time or memory thresholds
- **THEN** the certified performance gate MUST fail
- **AND** maintainers MUST optimize the canonical path rather than re-enable a weaker semantic mode, shrink a semantic population, or omit a required analyzer phase

#### Scenario: Consumer smoke measurement detects a catastrophic regression
- **WHEN** the consumer profile runs its single-sample full-scan and algorithmic smoke checks
- **THEN** it MUST compare them with separately named conservative smoke limits
- **AND** passing smoke MUST NOT be described as statistically stable performance certification

#### Scenario: Generated evidence uses reviewed nontrivial bounds
- **WHEN** the bounded reference-model or targeted mutation gate runs
- **THEN** it MUST load a versioned reviewed manifest that fixes the generated graph dimensions and named non-equivalent mutations
- **AND** the graph bounds MUST cover at least two procedures, four protocol nodes per procedure, two tracked identities, two call sites, call depth two, a branch join, and a recursion edge
- **AND** every well-formed normalized program admitted by those bounds MUST be compared with the independent interpreter
- **AND** every named targeted soundness mutant MUST be killed

#### Scenario: Certification evidence uses reviewed reproducible limits
- **WHEN** the canonical certified performance gate runs for semantic, completion, scheduled, or release assurance
- **THEN** the threshold manifest MUST declare time, byte, and allocation limits for solver, alias, refinement, generated-graph, and full-scan workloads
- **AND** it MUST record the reviewed Go toolchain and runner class
- **AND** the gate MUST compare the median of five fresh runs against those limits

#### Scenario: Resource controls constrain execution
- **WHEN** CPU, memory, worker, timeout, or shard controls constrain analysis
- **THEN** they MUST affect scheduling or produce an explicit blocking failure
- **AND** they MUST NOT convert a violation, inconclusive outcome, missing population, timeout, or resource exhaustion into a safe result
