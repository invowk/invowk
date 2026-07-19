# goplint-analysis-soundness Specification

## Purpose
Define the canonical, fail-closed semantic contract and independent evidence required for Invowk's goplint protocol analyses.
## Requirements
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

### Requirement: Every executable call and closure is conservatively modeled
Goplint SHALL model every relevant call expression in Go evaluation order and SHALL analyze every function literal body as an executable procedure independent of whether its invocation is syntactically visible in the enclosing function. A nested, sibling, returned, stored, passed, deferred, or concurrently launched closure or call MUST NOT be omitted from protocol transfer; incomplete ordering, identity, capture, or effect information MUST be blocking inconclusive.

#### Scenario: Nested mutation precedes the outer protected use
- **WHEN** a validated tracked value is passed by reference to a mutating inner call used as an argument of an outer protected call
- **THEN** the inner call's mutation or unresolved effect MUST transfer before the outer protected use
- **AND** the prior validation MUST be invalidated or the obligation MUST be blocking inconclusive

#### Scenario: Sibling calls each receive realizable call and return edges
- **WHEN** one source expression contains multiple relevant sibling calls
- **THEN** every call MUST receive its own ordered call-site identity, conservative transfer, and matching return continuation
- **AND** no sibling effect MAY be skipped because another call appears first in AST preorder

#### Scenario: Returned closure body is analyzed
- **WHEN** a function returns, stores, or passes a closure containing a protocol obligation
- **THEN** goplint MUST analyze the closure body as its own procedure and report its local violation or inconclusive outcome
- **AND** absence of an invocation visible in the enclosing function MUST NOT classify the closure body as non-executable or safe

#### Scenario: Unresolved closure capture fails closed
- **WHEN** a closure obligation depends on a captured identity or effect that cannot be resolved through the supported SSA semantics
- **THEN** the affected obligation MUST be blocking inconclusive with a stable reason
- **AND** the closure MUST NOT be silently skipped

### Requirement: Deferred constructor validation is proven on every successful return
Goplint SHALL establish constructor validation through a deferred closure only when canonical path analysis proves that every realizable successful return executes the matching validation, propagates that exact invocation's nil result to the constructor's returned error slot, and preserves the relation through all later deferred effects. Syntactic presence of validation and result assignments MUST NOT establish this proof.

#### Scenario: Conditional deferred validation does not discharge the constructor
- **WHEN** a deferred closure calls `Validate()` and assigns its result only under a condition that can be false on a successful constructor return
- **THEN** the constructor MUST remain unvalidated on that path
- **AND** goplint MUST report a violation or blocking inconclusive according to the supported path semantics

#### Scenario: Deferred result overwrite invalidates propagation
- **WHEN** a deferred closure assigns the validation result to the named error return and a later reachable effect can overwrite or disconnect that result
- **THEN** the validation MUST NOT discharge the successful-return obligation

#### Scenario: Deferred validation follows LIFO path semantics
- **WHEN** multiple deferred calls or closures affect the returned object or error slot
- **THEN** goplint MUST apply their summaries in Go's LIFO execution order on each constructor return
- **AND** unresolved relevant deferred effects MUST be blocking inconclusive

### Requirement: Protocol inconclusive outcomes are always visible
Every protocol inconclusive outcome SHALL be a hard-blocking diagnostic outside baseline and exception suppression. Baseline parsing, baseline updates, analyzer reporting, full scans, pre-commit, and CI MUST preserve the diagnostic even when stale suppression data names its category or stable finding ID.

#### Scenario: Baseline cannot suppress an inconclusive category
- **WHEN** a baseline contains an entry for a protocol inconclusive category or an inconclusive outcome
- **THEN** baseline validation MUST fail with an actionable error
- **AND** the analyzer MUST still emit the blocking inconclusive diagnostic

#### Scenario: Baseline update excludes inconclusives
- **WHEN** baseline update tooling observes protocol inconclusive outcomes
- **THEN** it MUST refuse to serialize them as accepted baseline entries
- **AND** the update MUST remain unsuccessful while those outcomes exist

#### Scenario: Blocking scans expose existing uncertainty
- **WHEN** the canonical repository scan encounters a previously baselined inconclusive finding
- **THEN** the scan MUST fail until stronger analysis classifies it or the underlying code removes the uncertainty
- **AND** migration to another suppression mechanism MUST NOT satisfy the gate

### Requirement: Successful constructor returns follow exact control-flow evidence
Goplint SHALL retain a constructor return obligation unless the exact control-flow edge and returned error identity prove that return unsuccessful. Branch ancestry, condition text, type equality, a different error value, or an empty extracted target set MUST NOT substitute for edge-sensitive result evidence.

#### Scenario: Else branch is a successful return
- **WHEN** an unvalidated object is returned from the `else` branch of `if err != nil` together with that branch's nil `err`
- **THEN** goplint MUST retain the returned-object obligation and report a violation or blocking inconclusive
- **AND** the ancestor non-nil condition MUST NOT classify the `else` return as unsuccessful

#### Scenario: Inverted and nested result checks preserve polarity
- **WHEN** successful and unsuccessful constructor returns are separated by inverted, nested, switch-derived, or phi-joined error conditions
- **THEN** each return MUST be classified from its realizable edge and exact returned error value
- **AND** uncertainty about the relation MUST be blocking inconclusive rather than exclusion of the return

#### Scenario: Empty return target set requires proof
- **WHEN** constructor identity extraction produces no returned-object target
- **THEN** goplint MAY classify the constructor safe only after proving that no realizable successful return contains a non-nil object
- **AND** discarded, ambiguous, or unresolved return identity MUST produce blocking inconclusive

### Requirement: Protocol summaries preserve conditional effect relations
Goplint SHALL preserve each summary effect's target identity, source identity, condition, condition-result slot, and execution order through export, import, recursion, composition, and caller transfer. A conditional validation effect MUST apply only on a caller continuation that proves the matching result condition.

#### Scenario: Discarded helper error does not restore validation
- **WHEN** a helper mutates a validated object, conditionally validates it, and the caller discards or overwrites the helper's returned error
- **THEN** the caller's prior validation MUST remain invalidated or blocking inconclusive
- **AND** summary application MUST NOT synthesize a nil result for the conditional validation effect

#### Scenario: Checked helper success restores validation
- **WHEN** the caller proves the exact helper error result nil after an ordered mutation-then-validation summary
- **THEN** the matching object MAY become validated on that nil continuation
- **AND** non-nil, unknown, mismatched, or unrelated error continuations MUST remain unvalidated or inconclusive

#### Scenario: Summary order remains observable
- **WHEN** two summaries contain validation-before-mutation and mutation-before-validation effects respectively
- **THEN** goplint MUST produce the distinct typestate required by each ordered sequence
- **AND** normalization MUST NOT sort, merge, or collapse away the condition or order

### Requirement: Protocol routing covers every package procedure root
Goplint SHALL discover and analyze every function declaration and function literal body in an analyzed package, including literals in package initializers, independent of whether an invocation is syntactically visible. Relevant missing procedure or SSA identity MUST be blocking inconclusive.

#### Scenario: Package-level stored closure is analyzed
- **WHEN** a package variable initializer stores a function literal containing a cast, validation, protected use, constructor obligation, or escape
- **THEN** goplint MUST analyze that literal body and emit its required violation or inconclusive outcome
- **AND** routing through a `GenDecl` MUST NOT silently omit the literal because it is not a `FuncDecl`

#### Scenario: Nested closure is reported once
- **WHEN** a function literal is reachable both from the package procedure inventory and from an enclosing procedure's closure discovery
- **THEN** stable procedure identity MUST cause one semantic analysis and one diagnostic result per obligation
- **AND** discovery order MUST NOT duplicate or suppress the finding

#### Scenario: Unresolved package literal fails closed
- **WHEN** a relevant package-level literal cannot be associated with a unique SSA procedure or initializer context
- **THEN** the affected obligation MUST emit a stable blocking inconclusive outcome
- **AND** the body MUST NOT be treated as non-executable or safe

### Requirement: Protocol uncertainty is classified before suppression
Every protocol entry point SHALL classify the semantic outcome before consulting exception, inline-ignore, or baseline policy. Inconclusive outcomes MUST use the always-visible reporting path regardless of whether the obligation occurs in a function declaration, nested closure, escaping closure, package-level literal, constructor, or boundary request.

#### Scenario: Excepted closure still reports inconclusive
- **WHEN** a closure matches an exception or inline-ignore directive and its protocol analysis is inconclusive
- **THEN** goplint MUST emit the inconclusive diagnostic
- **AND** the policy match MAY affect only an otherwise suppressible definite finding

#### Scenario: Every protocol route has the same ordering
- **WHEN** architecture validation enumerates protocol analysis and reporting entry points
- **THEN** each entry point MUST classify uncertainty before any policy suppression branch
- **AND** a route that can return before inconclusive classification MUST fail the architecture gate

### Requirement: Post-validation escape and fact uncertainty remain conservative
Goplint SHALL invalidate or make blocking inconclusive every relevant post-validation mutation, replacement, mutable escape, indirect store, or incompatible imported fact that can affect the tracked identity. Value copies MAY remain safe only when the copied form cannot mutate the validated identity.

#### Scenario: Mutable channel or aggregate escape after validation is blocking
- **WHEN** a validated object's address or mutable alias is sent on a channel or stored into an aggregate before a protected use or successful return
- **THEN** goplint MUST invalidate the validation or report escaped-heap or concurrent-mutation inconclusive
- **AND** post-validation state MUST NOT absorb the escape as an identity effect

#### Scenario: Immutable value copy remains precise
- **WHEN** a validated non-pointer value is copied into a channel, aggregate, or callee slot and no alias to the original identity escapes
- **THEN** goplint MUST preserve the original object's validated state
- **AND** the mutable-escape rule MUST NOT create unrelated uncertainty

#### Scenario: Imported fact slots match the attached signature
- **WHEN** an imported protocol fact names a function, target slot, source slot, or condition-result slot absent from or incompatible with the attached function signature
- **THEN** fact validation MUST reject it as incompatible
- **AND** every relevant dependent obligation MUST be blocking inconclusive rather than silently skipping the impossible effect

#### Scenario: Imported fact identity is exact
- **WHEN** a fact's package path, function identity, format, condition vocabulary, or slot role differs from the object to which it is attached
- **THEN** goplint MUST reject the fact before summary application
- **AND** no partial compatible subset MAY be treated as a complete resolved summary

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
