## ADDED Requirements

### Requirement: Goplint has one explicit soundness contract
Goplint SHALL define each registered analysis category as a structural predicate, protocol safety property, or cross-artifact relation, and SHALL machine-check that every category has resolvable implementation ownership and an appropriate independent oracle. Protocol properties MUST define tracked identities, abstract states, transfer and join behavior, interprocedural behavior, terminal behavior, uncertainty boundaries, and result vocabulary.

#### Scenario: Semantic catalog is complete and resolvable
- **WHEN** the semantic contract gate inspects the category registry and catalog
- **THEN** every registered category MUST have exactly one semantic owner and oracle strategy
- **AND** every declared implementation owner MUST resolve to a live registered Go symbol or key
- **AND** stale, duplicate, missing, or unresolvable entries MUST fail the gate

#### Scenario: Protocol semantics are machine checked
- **WHEN** a protocol rule's abstract domain or transfer contract changes
- **THEN** lattice, transfer, join, kill, call, return, and uncertainty laws MUST be exercised by executable contract tests
- **AND** the change MUST NOT be accepted solely because implementation-derived fixtures pass

#### Scenario: Unsupported Go behavior is not claimed safe
- **WHEN** relevant behavior uses an unsupported instruction, unresolved call effect, ambiguous identity, incompatible fact, reflection, `unsafe`, or exhausted analysis budget
- **THEN** the protocol outcome MUST be inconclusive
- **AND** the analyzer MUST NOT classify the path or enclosing obligation as safe

### Requirement: Validation requires a proven successful result
Goplint SHALL establish validation for a tracked value only on control-flow paths where the `Validate() error` result associated with that same value is proven nil. Observing, assigning, blank-assigning, logging, or otherwise using a validation error without a nil-conditioned continuation MUST NOT establish validation.

#### Scenario: Terminating error branch establishes validation afterward
- **WHEN** code calls `value.Validate()`, checks that its error is non-nil, and every non-nil branch terminates before a later protected use
- **THEN** the analyzer MUST treat `value` as validated on the surviving nil-result path

#### Scenario: Non-terminating error branch remains unsafe
- **WHEN** code calls `value.Validate()`, observes a non-nil error, and can continue from that branch to a protected use or successful constructor return
- **THEN** the analyzer MUST report a validation protocol violation for that feasible path

#### Scenario: Assigned or blank validation result is insufficient
- **WHEN** a validation result is assigned, blank-assigned, logged, or passed elsewhere without a dominating proof that it is nil
- **THEN** the receiver MUST remain unvalidated on that path

#### Scenario: Method value keeps its receiver and result relation
- **WHEN** a `Validate` method value is captured and later invoked
- **THEN** the analyzer MUST relate the returned error to the captured receiver
- **AND** it MUST apply the same nil-result requirements as a direct selector call

#### Scenario: Helper summary is conditional
- **WHEN** a helper validates a receiver or argument and returns an error
- **THEN** its summary MUST identify the affected receiver or parameter slot and condition the validation effect on the relevant nil result
- **AND** callers MUST NOT receive an unconditional type-level validation effect

#### Scenario: Constructor returns value and validation error together
- **WHEN** a constructor returns a value and that value's validation error in corresponding result slots
- **THEN** the value MUST be considered valid only on caller continuations where the returned error is nil

### Requirement: Object identity and alias effects are SSA-sensitive
Goplint SHALL track protocol obligations by SSA value and abstract object identity. It MUST apply validation effects only to proven must-alias identities, model assignment and allocation kill semantics, and MUST NOT use type equality alone to associate a validation call with a returned or consumed object.

#### Scenario: Rebinding kills a must-alias relation
- **WHEN** an alias initially refers to a tracked object and is then rebound to another object before validation
- **THEN** validating the rebound alias MUST NOT validate the original object

#### Scenario: Same-typed allocation is not the return target
- **WHEN** a constructor allocates two values of the same type, validates one, and returns the other
- **THEN** validation of the first allocation MUST NOT discharge the returned allocation's obligation

#### Scenario: Copy and phi identities are modeled explicitly
- **WHEN** SSA copies or phi nodes connect identities across paths
- **THEN** the analyzer MUST preserve only must-alias facts valid on every applicable incoming path
- **AND** uncertain may-alias relationships relevant to validation MUST be inconclusive

#### Scenario: Cross-package summaries identify slots and conditions
- **WHEN** validation behavior crosses a package boundary
- **THEN** the exported fact MUST be package-qualified, format-versioned, parameter/result-slot-sensitive, and conditional on any error result
- **AND** a filtered package MUST still export facts needed by analyzed dependents

### Requirement: Canonical interprocedural analysis follows realizable paths
Goplint SHALL use one SSA-backed, finite-fact interprocedural solver whose call and return propagation preserves matching caller context. The solver MUST NOT use an unconditional call bypass or return a callee result to unrelated call sites.

#### Scenario: Return flows to the matching caller
- **WHEN** the same callee is invoked from multiple call sites or recursive contexts
- **THEN** validation and escape facts from a callee return MUST propagate only to realizable matching continuations

#### Scenario: Modeled call-to-return edge preserves effects
- **WHEN** the solver uses a call-to-return or summary edge
- **THEN** the edge MUST have an explicit conservative effect for every relevant fact
- **AND** it MUST NOT bypass a possible validation, escape, mutation, or terminal effect

#### Scenario: Recursion reaches a conservative fixed point
- **WHEN** mutually recursive or self-recursive calls affect a tracked obligation
- **THEN** the solver MUST converge through finite summary facts
- **AND** incomplete convergence or exhausted budgets MUST be inconclusive rather than safe

#### Scenario: Known non-returning call terminates the path
- **WHEN** a path invokes `panic`, `os.Exit`, a supported `log.Fatal` variant, or a soundly resolved alias with proven non-returning behavior
- **THEN** no protocol finding MUST be emitted solely from a continuation that cannot execute

#### Scenario: Unknown call is not assumed terminal or harmless
- **WHEN** a relevant call cannot be resolved or summarized
- **THEN** the analyzer MUST assume it may return
- **AND** any unresolved relevant protocol effect MUST produce an inconclusive outcome

### Requirement: Generic and method-set forms preserve protocol obligations
Goplint SHALL normalize instantiated generic syntax and type-parameter constraints through Go type information when identifying raw primitive sources, constructors, return slots, receiver methods, and validation effects.

#### Scenario: Instantiated generic constructor is analyzed
- **WHEN** a constructor call is expressed through generic index syntax such as `NewBox[T]()`
- **THEN** constructor error-use and validation obligations MUST be analyzed from its instantiated signature

#### Scenario: Primitive-constrained type parameter creates an obligation
- **WHEN** a type parameter's type set has a supported raw primitive underlying term and is converted into a validatable named type
- **THEN** the same cast-validation obligation MUST apply as for the corresponding concrete primitive

#### Scenario: Method set is resolved consistently
- **WHEN** `Validate` is available through a pointer or value method set
- **THEN** direct calls, interface-resolved calls, and method values MUST use the Go method-set rules consistently
- **AND** an unrelated lowercase or same-signature method MUST NOT satisfy the `Validate` protocol

### Requirement: Feasibility and refinement are SSA-versioned and fail closed
Goplint SHALL evaluate path feasibility over SSA-versioned subjects using an exact documented constraint fragment. Only UNSAT results with independently checked contradiction evidence MAY discharge a witness. SAT MUST retain the witness, and unsupported, failed, timed-out, budget-exhausted, or unverified results MUST be inconclusive.

#### Scenario: Reassignment creates distinct constraint subjects
- **WHEN** a source variable is assigned different values along one path
- **THEN** predicates before and after assignment MUST refer to distinct SSA versions
- **AND** the analyzer MUST NOT infer UNSAT merely because the source-level variable name has different equalities

#### Scenario: Checked UNSAT evidence discharges a witness
- **WHEN** the supported constraint procedure returns UNSAT for a candidate witness
- **THEN** a separate evidence checker MUST validate the normalized contradiction before discharge
- **AND** the trace MUST record the checked evidence and SSA subjects

#### Scenario: Unsupported predicate remains inconclusive
- **WHEN** witness feasibility depends on a predicate outside the documented constraint fragment
- **THEN** the result MUST be unknown and the protocol outcome MUST remain inconclusive

#### Scenario: Refinement iterates without optimistic fallback
- **WHEN** replay identifies a spurious counterexample that can be refined with supported predicates
- **THEN** refinement MUST iterate until the witness is retained, soundly discharged, or a resource limit is reached
- **AND** reaching a resource limit MUST be inconclusive

### Requirement: Result vocabulary reflects actual evidence
Goplint SHALL use `violation`, `inconclusive`, and structured `discharged-infeasible` refinement evidence according to the documented semantics. It MUST NOT emit or document `proven-safe`, and absence of a finding MUST NOT be described as an unrestricted proof of arbitrary Go behavior.

#### Scenario: Obligation outcomes aggregate fail closed
- **WHEN** at least one supported feasible path demonstrates the prohibited event
- **THEN** the obligation result MUST be `violation` even if a different path is unresolved
- **WHEN** no feasible violation is established but any relevant path remains unresolved
- **THEN** the obligation result MUST be `inconclusive`
- **AND** `discharged-infeasible` MUST remain trace-only evidence for the checked witness and MUST NOT suppress uncertainty on another path

#### Scenario: Feasible prohibited path is a violation
- **WHEN** a supported feasible path reaches a protected use or successful return without successful validation
- **THEN** the analyzer MUST emit a violation with deterministic path and identity evidence

#### Scenario: Incomplete proof is blocking inconclusive
- **WHEN** a protocol obligation cannot be classified under the supported semantics
- **THEN** goplint MUST emit a blocking inconclusive result with a stable reason
- **AND** no CLI option MAY downgrade it to warning, hide it, or reclassify it as safe

#### Scenario: Documentation qualifies the soundness boundary
- **WHEN** documentation describes goplint correctness or soundness
- **THEN** it MUST identify the supported property and abstraction boundary
- **AND** it MUST distinguish no reported violation from whole-program proof

### Requirement: Only the canonical analysis pipeline is available
Goplint SHALL expose and execute one production protocol-analysis pipeline: SSA-backed UBV escape semantics, realizable-path interprocedural analysis, SSA/object-sensitive aliases, iterative SSA-constraint refinement, and blocking inconclusive outcomes. Production code MUST NOT retain alternate evaluator or fallback paths.

#### Scenario: Legacy semantic flags are removed
- **WHEN** a caller passes `--ubv-mode`, `--cfg-backend`, `--cfg-interproc-engine`, `--cfg-inconclusive-policy`, `--cfg-feasibility-engine`, `--cfg-refinement-mode`, or `--cfg-alias-mode`
- **THEN** goplint MUST reject the unknown flag
- **AND** it MUST NOT accept a deprecated alias or compatibility spelling

#### Scenario: Resource controls cannot weaken semantics
- **WHEN** a caller changes state, witness, iteration, query, or timeout limits
- **THEN** unfinished analysis MUST become blocking inconclusive
- **AND** no resource setting MAY disable a canonical semantic layer or produce an optimistic result

#### Scenario: No alternate production engine remains
- **WHEN** maintainers inspect compiled goplint production paths
- **THEN** legacy, compare, AST protocol evaluation, order-only UBV, alias-off, feasibility-off, and one-shot refinement implementations MUST be absent
- **AND** tests MUST exercise components directly without a hidden runtime mode selector

### Requirement: Independent oracles cover the analyzer contract
Goplint SHALL validate its semantic contract with independent and adversarial evidence that does not reuse production transfer functions as the expected-value source. Every registered category MUST have boundary fixtures, and protocol categories MUST additionally have generated reference-model comparisons and applicable metamorphic, fuzz-seed, and mutation evidence.

#### Scenario: Historical counterexamples are locked
- **WHEN** the semantic gate runs
- **THEN** it MUST cover ignored or weakly handled validation errors, method values, alias rebinding, wrong same-typed objects, generic calls and type parameters, cross-package conditional summaries, known no-return calls, unmatched returns, and reassignment-sensitive feasibility

#### Scenario: Bounded graphs match an independent reference model
- **WHEN** bounded protocol graphs are generated exhaustively
- **THEN** the production solver outcomes MUST match a separately implemented test-only reference interpreter
- **AND** the reference interpreter MUST NOT call production transfer, join, summary, or feasibility functions

#### Scenario: Metamorphic equivalents preserve outcomes
- **WHEN** fixtures are transformed through alpha-renaming, equivalent nil-check forms, branch inversion, selector/method-value forms, or semantics-preserving alias copies
- **THEN** their normalized outcomes MUST remain equivalent
- **AND** alias rebindings or changed error continuations MUST change outcomes where required by the formal semantics

#### Scenario: Fuzz and mutation guards remain effective
- **WHEN** deterministic fuzz seeds and the goplint mutation profile run
- **THEN** they MUST cover graph construction, propagation, constraint evidence, catalog decoding, finding determinism, and the soundness-critical nil-branch, alias-kill, matched-return, terminal, generic, and unknown-to-safe guards
- **AND** surviving targeted soundness mutants MUST fail the gate

### Requirement: Soundness gates are deterministic and performance bounded
The canonical soundness gate SHALL produce deterministic findings, reasons, witnesses, and refinement evidence for identical inputs, and SHALL enforce documented performance budgets without converting exhausted analysis into safety.

#### Scenario: Repeated and reordered analysis is deterministic
- **WHEN** the same packages are analyzed repeatedly or in a different package/worklist order
- **THEN** normalized findings and evidence MUST be byte-for-byte stable after permitted timing fields are removed

#### Scenario: Performance regression exceeds the budget
- **WHEN** canonical solver benchmarks exceed their reviewed time or memory thresholds
- **THEN** the performance gate MUST fail
- **AND** maintainers MUST optimize the canonical path rather than re-enable a weaker semantic mode

#### Scenario: Generated evidence uses reviewed nontrivial bounds
- **WHEN** the bounded reference-model or targeted mutation gate runs
- **THEN** it MUST load a versioned reviewed manifest that fixes the generated graph dimensions and named non-equivalent mutations
- **AND** the graph bounds MUST cover at least two procedures, four protocol nodes per procedure, two tracked identities, two call sites, call depth two, a branch join, and a recursion edge
- **AND** every well-formed normalized program admitted by those bounds MUST be compared with the independent interpreter
- **AND** every named targeted soundness mutant MUST be killed

#### Scenario: Benchmark evidence uses reviewed reproducible limits
- **WHEN** the canonical performance gate runs
- **THEN** the threshold manifest MUST declare time, byte, and allocation limits for solver, alias, refinement, generated-graph, and full-scan workloads
- **AND** it MUST record the reviewed Go toolchain and CI runner class
- **AND** the gate MUST compare the median of five fresh runs against those limits
