## ADDED Requirements

### Requirement: Finding identities are globally scoped and source-layout independent
Every stable goplint finding ID SHALL include full package-path semantic identity and SHALL remain invariant under unrelated file ordering, file length, token-position, message, and package-leaf collisions. Baseline lookup MUST NOT suppress findings from a different import path or semantic source.

#### Scenario: Equal package leaf names do not collide
- **WHEN** two import paths contain the same package leaf, type, member, and diagnostic category
- **THEN** their stable finding IDs MUST differ by full package identity
- **AND** baselining one finding MUST NOT suppress the other

#### Scenario: Unrelated source edits do not rotate IDs
- **WHEN** unrelated declarations or files are inserted, removed, reordered, or reformatted without changing a finding's semantic identity
- **THEN** that finding's stable ID MUST remain byte-identical
- **AND** raw `token.Pos` or file-set ordinal values MUST NOT participate in the ID

#### Scenario: Stable-ID migration is reviewed
- **WHEN** correcting an ID algorithm changes existing repository IDs
- **THEN** tooling MUST produce a deterministic old-to-new migration and collision report from repeated identical scans
- **AND** unexplained churn or a collision MUST block baseline acceptance

### Requirement: Goplint directives are total and fail visibly
Goplint SHALL validate directive names, attachment locations, arguments, duplication, and conflicts across field, declaration, type, function, method, and file documentation. An unknown, misspelled, incomplete, misplaced, or conflicting directive MUST produce an actionable failure rather than silently disabling or weakening a check.

#### Scenario: Type-level typo is rejected
- **WHEN** type or declaration documentation contains an unknown directive resembling `enum-cue`, `nonzero`, `path-domain`, or another supported directive
- **THEN** goplint MUST report the unknown directive at its source location
- **AND** the associated check MUST NOT silently disappear

#### Scenario: Parameterized directive requires a value
- **WHEN** a known parameterized directive is present without its required value or with an invalid value
- **THEN** directive validation MUST fail before the directive consumer runs
- **AND** recognizing only the directive name MUST NOT count as valid configuration

#### Scenario: Duplicate or conflicting directives fail
- **WHEN** the same declaration contains duplicate or mutually incompatible goplint directives
- **THEN** goplint MUST emit one deterministic actionable configuration error
- **AND** traversal order MUST NOT choose one directive silently

### Requirement: Semantic evidence credit matches executed production stages
Every goplint evidence observation SHALL derive its category, semantic feature, boundary, stages, dimensions, properties, and population from cases actually executed by its producer. Declarations, labels, fixture ordering, hard-coded counts, or a final reporting mutation MUST NOT award credit for unobserved semantics.

#### Scenario: Metamorphic evidence transforms semantics-bearing input
- **WHEN** a category claims metamorphic coverage
- **THEN** the producer MUST apply a documented semantics-preserving or predictably semantics-changing transformation to a program or semantic model
- **AND** merely reversing independent fixture order MUST NOT satisfy the relation

#### Scenario: Fuzz evidence decodes variable semantic structure
- **WHEN** a category claims fuzz coverage
- **THEN** input bytes MUST control reviewed variable facts, identities, aliases, branches, procedures, calls, returns, effects, or constraints relevant to that category
- **AND** the target MUST check an independent semantic property rather than only labels, determinism, or internal consistency

#### Scenario: Determinism credit is category specific
- **WHEN** a category claims file, package, map, worklist, or equivalent-schedule determinism
- **THEN** that category's real analyzer cases MUST execute under every credited reordering
- **AND** a stable unrelated global corpus MUST NOT transfer determinism credit to the category

#### Scenario: Mutation stages match the mutated boundary
- **WHEN** a category mutant changes only diagnostic reporting
- **THEN** its observation MAY claim reporting-stage mutation evidence only
- **AND** extraction, identity, graph, propagation, refinement, or aggregation credit MUST require separate causal mutants through those applicable stages

#### Scenario: Fixed fixtures retain honest labels
- **WHEN** expected outcomes come from explicit declarative fixtures rather than an executable independent interpreter
- **THEN** the evidence MUST be labeled independent boundary-oracle evidence
- **AND** it MUST NOT claim an integrated independent-model comparison

### Requirement: Aggregate subgate populations are executable censuses
Every aggregate subgate SHALL prove that each required test, case, shard, category, seed, mutant, benchmark, or other population member exists and executed in the current run. A successful command with no matching tests or with a hard-coded population MUST fail evidence validation.

#### Scenario: Missing regex-selected test fails
- **WHEN** a subgate's `go test -run` pattern matches no test or omits any required named test
- **THEN** the subgate MUST fail before emitting a successful report
- **AND** Go's zero-exit no-tests behavior MUST NOT count as execution evidence

#### Scenario: Population comes from observations
- **WHEN** a subgate reports a nonzero population
- **THEN** the report count MUST be derived from uniquely observed current-run members
- **AND** a constant count, duplicate observation, skipped shard, or prior report MUST be rejected

#### Scenario: Contract mutation removes a test
- **WHEN** adversarial gate tests delete or rename each required test while leaving the command and hard-coded report path intact
- **THEN** the owning subgate and aggregate runner MUST fail

### Requirement: Mutation kills prove the intended mismatch
The targeted mutation gate SHALL accept a kill only when clean controls pass, the exact declared transformation compiles, the expected guard observes the declared semantic mismatch, restoration succeeds, and repeated post-controls remain clean. Expected-test-name failure alone MUST NOT establish causality.

#### Scenario: Expected test fails for unrelated assertion
- **WHEN** the named guard fails because of setup, unrelated assertion, environmental error, or a mismatch different from the mutant's declared concern
- **THEN** the runner MUST classify the mutant invalid rather than killed
- **AND** no semantic observation or stage credit may be emitted

#### Scenario: Every blocking mutant is killed causally
- **WHEN** the blocking profile completes
- **THEN** every selected mutant MUST have zero survivors and one structured intended-mismatch attribution
- **AND** compilation failure, timeout, panic, generic `FAIL`, missing test, or pre-existing failure MUST remain non-kill outcomes

#### Scenario: Post-validation mutants cannot survive
- **WHEN** mutation removes post-validation summary or unresolved-effect transfer
- **THEN** a production-boundary guard MUST fail for the exact lost violation or inconclusive outcome
- **AND** both post-validation mutants identified by this review MUST be killed before completion

### Requirement: Completion proof covers the complete dependent diff
The goplint soundness completion proof SHALL materialize and verify every intended tracked and untracked change across `complete-goplint-soundness-hardening`, `close-goplint-soundness-review-gaps`, and `close-residual-goplint-soundness-gaps`. Omitted changed paths, incomplete task state, stale artifacts, or out-of-order archives MUST invalidate completion.

#### Scenario: Changed path is omitted from proof selection
- **WHEN** any changed or untracked repository path is absent from the proof selection without an explicit reviewed unrelated-path exclusion
- **THEN** materialization or freshness verification MUST fail
- **AND** the omitted content MUST NOT remain outside the synthetic-tree identity silently

#### Scenario: Combined proof follows final task state
- **WHEN** any predecessor or current task, manifest, counterexample, baseline, evidence producer, documentation claim, or artifact changes after proof generation
- **THEN** freshness verification and the completion profile MUST fail until the proof is regenerated

#### Scenario: Archive order preserves dependencies
- **WHEN** the combined final tree passes every required gate and freshness check
- **THEN** maintainers MUST synchronize and archive the three changes in dependency order with strict validation after each transition
- **AND** artifact readiness or a partially complete task ledger MUST NOT authorize an earlier archive

#### Scenario: Completion record is one combined authority
- **WHEN** completion evidence is retained
- **THEN** one reviewed record MUST bind the exact base, full intended diff, synthetic tree, toolchain, task ledgers, commands, observations, mutation attributions, populations, and outcomes
- **AND** separate partial records MUST NOT substitute for the combined proof
