# goplint-soundness-assurance Specification

## Purpose
Define the production semantic, fail-closed analysis, and evidence-integrity requirements for goplint's property-relative soundness assurance.
## Requirements
### Requirement: Production propagation implements the declared protocol domain
Goplint SHALL propagate the declared validation, hazard, uncertainty, conditional-error, and object-identity domain through the production analyzer. A validation effect MUST apply only to the matching receiver identity on a CFG edge where that invocation's error result is proven nil; a call-position boolean or unconditional validation transfer MUST NOT substitute for the conditional relation.

#### Scenario: Validation failure continuation remains unvalidated
- **WHEN** a `Validate()` invocation's non-nil continuation can reach a protected use or successful constructor return
- **THEN** the tracked receiver MUST remain unvalidated on that continuation
- **AND** the analyzer MUST report a violation for a supported feasible path

#### Scenario: Validation success affects only the nil edge
- **WHEN** one successor proves the validation result nil and another proves it non-nil
- **THEN** only the nil successor MUST receive the validation effect
- **AND** both successors MUST retain the same receiver/result relation through direct calls, method values, helpers, and constructor result slots

#### Scenario: Production and formal domains cannot diverge
- **WHEN** a protocol-domain state, effect, join, or uncertainty reason is declared executable
- **THEN** a production analyzer path MUST consume that declaration or an explicitly generated equivalent
- **AND** dead model-only domains or parallel simplified production domains MUST fail an architecture test

### Requirement: Object identity is flow-sensitive and fail closed
Goplint SHALL associate validation, use, mutation, escape, and return obligations with SSA-versioned values and abstract objects rather than type equality or whole-function assignment connectivity. Rebinding, allocation, stores, phi joins, interface wrapping, and unresolved alias effects MUST preserve only proven must-alias relations; relevant ambiguity MUST be blocking inconclusive.

#### Scenario: Historical assignment does not survive rebinding
- **WHEN** a return variable first references object A and is later rebound to object B
- **THEN** validating object A MUST NOT validate object B or the return slot that contains B

#### Scenario: Missing return identity does not fall back to type
- **WHEN** the analyzer cannot identify the object reaching a constructor return slot
- **THEN** it MUST emit a stable inconclusive outcome
- **AND** validation of another same-typed value or a same-typed literal MUST NOT discharge the obligation

#### Scenario: Ambiguous identity blocks optimistic classification
- **WHEN** copies, phi nodes, stores, dynamic indices, interfaces, closures, or escaped pointers leave more than one relevant possible identity
- **THEN** the analyzer MUST retain only facts common to every incoming must-alias relation
- **AND** any remaining result-relevant ambiguity MUST be inconclusive rather than safe

### Requirement: Unknown effects can invalidate an established property
Goplint SHALL track uncertainty independently of whether an object was previously validated. A relevant unresolved call, concurrent access, reflection, `unsafe`, escaped-heap mutation, or unsupported instruction that can mutate, replace, consume, or escape a tracked object MUST produce blocking inconclusive unless its effect is soundly summarized.

#### Scenario: Unknown mutation after validation is inconclusive
- **WHEN** a validated object is passed to an unresolved call before its protected use or successful return
- **THEN** the analyzer MUST NOT treat the prior validated state as absorbing
- **AND** it MUST report inconclusive when the unresolved call may invalidate the property

#### Scenario: Irrelevant unknown behavior does not poison the obligation
- **WHEN** an unresolved effect is outside the obligation's identity and control-flow slice
- **THEN** it MUST NOT make the unrelated obligation inconclusive
- **AND** relevance MUST be decided conservatively from reachability, identity, and possible effect classes

#### Scenario: Missing SSA blocks every dependent proof
- **WHEN** SSA construction fails or a required function, closure, value, or instruction cannot be resolved
- **THEN** every obligation whose proof depends on that SSA information MUST be inconclusive
- **AND** no AST, type-only, empty-alias, or nil-analysis fallback MAY classify it as safe

### Requirement: Refinement evidence remains paired with its exact state and path
Goplint SHALL preserve provenance for each abstract-state contribution across joins. A witness MUST identify interprocedural edges, call sites, SSA subjects, tracked facts, and the contributed state; checked UNSAT evidence MAY discharge only that exact witness contribution.

#### Scenario: Joined unsafe state keeps its feasible witness
- **WHEN** an infeasible validated path and a feasible unvalidated path meet at a join
- **THEN** the joined unvalidated contribution MUST retain the feasible path's provenance
- **AND** UNSAT evidence for the validated path MUST NOT suppress the unvalidated contribution

#### Scenario: Interprocedural witnesses cannot collide by block index
- **WHEN** caller and callee CFGs contain equal local block indexes
- **THEN** witness identity MUST distinguish procedure, node, edge kind, and call site
- **AND** refinement MUST reject missing, non-adjacent, mismatched-call, or silently skipped witness transitions

#### Scenario: Timeout cannot return a late discharge
- **WHEN** the refinement deadline expires during extraction, solving, or evidence checking
- **THEN** the result MUST be unknown and the obligation MUST be inconclusive
- **AND** an UNSAT result produced or accepted after the deadline MUST NOT discharge a witness

### Requirement: Interprocedural analysis converges through finite summaries
Goplint SHALL use finite-fact tabulation with procedure summaries and realizable call/return matching. Recursion MUST converge to a fixed point without treating an ever-growing concrete call stack as the semantic state, and depth or state exhaustion MUST remain inconclusive.

#### Scenario: Recursive obligation reaches a fixed point
- **WHEN** a self-recursive or mutually recursive call graph affects a tracked obligation
- **THEN** repeated entry/exit fact pairs MUST reuse and refine finite summaries until no fact changes
- **AND** successful completion MUST NOT depend on reaching a call-stack depth cutoff

#### Scenario: Returns reach only matching continuations
- **WHEN** a callee is reached from multiple call sites or recursive contexts
- **THEN** its return facts MUST propagate only through realizable matching return edges
- **AND** summary reuse MUST preserve conditional validation, uncertainty, mutation, and terminal effects

#### Scenario: No-return aliases are resolved at the call site
- **WHEN** a variable referring to a known no-return function is conditionally assigned or rebound
- **THEN** the analyzer MUST use the SSA value active at the specific call site
- **AND** it MUST prune the continuation only when every realizable reaching definition proves non-returning behavior

### Requirement: Generic and unsupported forms preserve obligations conservatively
Goplint SHALL normalize generic calls, instantiated signatures, method sets, and type-parameter type sets through `go/types`. A primitive-constrained type parameter MUST create the corresponding protocol obligation even when its constraint also declares methods; unsupported or mixed relevant type sets MUST be inconclusive rather than treated as having no obligation.

#### Scenario: Method-bearing primitive constraint is analyzed
- **WHEN** a type parameter has a supported primitive underlying term and also declares methods
- **THEN** the primitive conversion obligation MUST still be created
- **AND** method declarations alone MUST NOT make the primitive term disappear

#### Scenario: Unsupported mixed constraint is inconclusive
- **WHEN** the analyzer cannot normalize every relevant term or method-set form needed for a protocol decision
- **THEN** it MUST emit a stable inconclusive reason
- **AND** it MUST NOT silently return no obligation

### Requirement: One production semantic authority remains
Goplint production code SHALL contain one protocol-analysis pipeline and one conditional summary-fact format. Legacy traversal modes, nil-context behavior, AST protocol fallbacks, order-only evaluators, unconditional type-level validation facts, compatibility shims, and hidden runtime selectors MUST be absent from compiled non-test code.

#### Scenario: Legacy implementation inventory is empty
- **WHEN** the architecture gate scans and type-checks production sources
- **THEN** no legacy mode constant, legacy DFS entry point, nil-context compatibility branch, alternate evaluator, or unconditional `ValidatesTypeFact` path may remain
- **AND** test helpers MUST instantiate canonical components directly rather than selecting a hidden production mode

#### Scenario: Conditional facts reject incompatible data
- **WHEN** a dependency exports a missing, legacy, malformed, or incompatible protocol fact
- **THEN** the importing obligation MUST be inconclusive when the fact is relevant
- **AND** the analyzer MUST NOT reinterpret the fact as unconditional validation

### Requirement: Soundness evidence is complete, independent, and causal
Every registered goplint category SHALL have machine-enforced semantic ownership and real-analyzer boundary expectations. Protocol categories SHALL additionally have independent model comparisons, substantive metamorphic and fuzz properties, and causal targeted mutation guards that exercise production extraction, propagation, aggregation, and reporting.

#### Scenario: Registry and oracle coverage are bijective
- **WHEN** a category is added, removed, renamed, or changes semantic kind
- **THEN** validation MUST require exactly one live owner, one rule contract, and every oracle layer required for that kind
- **AND** a missing `rules` or oracle-matrix entry MUST fail even when all existing entries are internally valid

#### Scenario: Bounded manifest controls the admitted corpus
- **WHEN** the generated-oracle manifest declares procedure, node, identity, call-site, depth, branch-join, recursion, operation, or effect dimensions
- **THEN** changing any dimension MUST change or deliberately constrain the enumerated corpus
- **AND** the gate MUST derive and verify the exact admitted-program count and feature census without manually asserting coverage

#### Scenario: End-to-end analyzer is compared independently
- **WHEN** generated or hand-authored protocol programs are checked
- **THEN** production AST/type/SSA extraction, graph construction, propagation, refinement, aggregation, and diagnostic rendering MUST participate
- **AND** the expected outcome MUST come from an independently implemented model or explicit semantic fixture, not a production transfer helper

#### Scenario: Mutation kill is causally attributable
- **WHEN** the targeted mutation gate evaluates a named soundness mutant
- **THEN** the selected guard tests MUST pass in an unmutated control run
- **AND** the mutated run MUST fail for the expected guard and semantic mismatch
- **AND** compile failures, unrelated failures, generic `FAIL` output, or pre-existing failures MUST NOT count as a kill

#### Scenario: Determinism exercises real analysis order
- **WHEN** the determinism gate repeats analysis with reordered packages, files, worklists, maps, and equivalent scheduling
- **THEN** normalized findings, facts, reasons, witnesses, and refinement evidence from the real analyzer MUST be byte-identical
- **AND** a helper that sorts preconstructed findings MUST NOT satisfy this requirement

#### Scenario: Fuzzing explores semantic structure
- **WHEN** protocol fuzz targets run beyond their committed seed corpus
- **THEN** input bytes MUST generate variable graph shapes, identities, aliases, branches, calls, returns, effects, and supported constraints within reviewed bounds
- **AND** each target MUST check an independent semantic property or model relation in addition to determinism and internal consistency

### Requirement: Completion evidence is reproducible and claim accurate
The change SHALL preserve machine-readable evidence for counterexamples, architecture absence checks, catalog coverage, generated bounds, determinism, fuzz seeds, mutation causality, race/repeat tests, full scans, performance limits, and strict OpenSpec validation. Documentation MUST map each claimed guarantee to the production implementation and the blocking gate that proves it.

#### Scenario: Clean synthetic-tree proof is recorded
- **WHEN** implementation is declared complete
- **THEN** all required gates MUST run from a clean detached worktree materialized from exactly `HEAD` plus the intended diff
- **AND** the evidence record MUST include the synthetic tree identity, toolchain, commands, outcomes, and reviewed counterexample inventory without mutating the caller's index

#### Scenario: Green unit model cannot justify a production claim
- **WHEN** documentation describes a formal domain, oracle bound, deterministic analysis, mutation kill, recursion convergence, or fail-closed reason
- **THEN** the evidence index MUST identify a production integration test or architecture gate for that claim
- **AND** model-only or helper-only tests MUST be labeled as supporting evidence rather than end-to-end proof

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

### Requirement: Completion record identity separates semantic content from prose
The retained completion record SHALL bind the proof tree through two reviewed digests — a semantic-content digest covering every path class that any gate executes or reads, and a prose digest covering documentation-class paths — so that evidence validity follows the content that produced it.

#### Scenario: Aggregate evidence binds only to semantic content
- **WHEN** a completion record is generated or re-bound
- **THEN** the retained aggregate report MUST be attributable to the semantic-content digest alone
- **THEN** re-binding MUST NOT modify, truncate, or re-attribute any retained report, observation, population, or mutation attribution

#### Scenario: Re-binding is rejected when semantic content drifted
- **WHEN** re-binding is requested and the current semantic-content digest differs from the retained one
- **THEN** re-binding MUST fail closed and require full regeneration with fresh gate execution
- **THEN** the failure MUST name the drifted semantic paths

#### Scenario: Record format migration stays visible
- **WHEN** a verifier encounters a retained record in the previous single-digest format
- **THEN** it MUST reject the record with an explicit migration notice rather than silently accepting or reinterpreting it

### Requirement: Harness-tier assurance proves orchestration integrity
Changes confined to gate orchestration SHALL be proven by evidence-preservation obligations rather than semantic population re-execution: the module test suites, one shared repository audit, and normalized-report parity between the serial reference and parallel executors over a reviewed fixture.

#### Scenario: Parity check guards evidence handling
- **WHEN** the harness tier runs
- **THEN** the serial-reference and parallel executors MUST produce byte-identical normalized reports for the reviewed fixture manifest
- **THEN** any divergence, lost observation, or population mismatch MUST fail the tier

#### Scenario: Harness tier never substitutes for semantic assurance
- **WHEN** a diff also touches any analyzer-semantics path, or the event is a schedule, release, completion, or exhaustive dispatch
- **THEN** the harness tier MUST NOT be selected as the final assurance answer
- **THEN** the applicable semantic or completion profile MUST run

### Requirement: Task-ledger completion expectations have one reviewed source
The reviewed completion plan SHALL be the single source of truth for expected task-ledger names, paths, dependency order, and permitted pending identifiers; no code constant, schema constant, or fixture MAY duplicate those expected values.

#### Scenario: Closing a change edits one reviewed file
- **WHEN** a change's task-ledger expectation transitions, including archiving the change directory
- **THEN** only the reviewed completion plan MUST require editing
- **THEN** structural validation — dependency ordering, archived-predecessor policy, well-formed pending sets — MUST remain enforced by code and schema without embedding the expected values

#### Scenario: Ledger validation stays fail closed
- **WHEN** the live repository task ledgers differ from the reviewed plan's expectations
- **THEN** evidence generation and verification MUST fail with the exact ledger, expected state, and observed state
- **THEN** the mismatch MUST NOT be baselinable or exceptable

