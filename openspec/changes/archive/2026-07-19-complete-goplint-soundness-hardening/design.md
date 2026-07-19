## Context

The `harden-goplint-soundness` implementation introduced a formal protocol domain, conditional validation relations, SSA alias analysis, an interprocedural solver, checked constraint refinement, a reference interpreter, generated graphs, fuzz targets, targeted mutations, determinism checks, and blocking repository gates. The user authorized archiving that predecessor on 2026-07-14 after its then-current gates passed. A read-only audit nevertheless found a split semantic authority: production reduces conditional validation to checked call positions and propagates a simpler validation state; constructor identity can still fall back to type equality and an undirected historical assignment graph; unresolved mutation after validation is ignored; refinement joins state without replacing its representative path; missing SSA and alias ambiguity do not consistently reach fail-closed aggregation; recursion is bounded concrete-stack exploration; and compiled legacy evaluators and facts remain. This corrective change uses the archived artifacts and synchronized canonical specs as immutable dependency evidence rather than reopening or rewriting the predecessor archive.

The evidence stack has corresponding blind spots. Catalog validation is not total, generated bounds do not drive most generator dimensions, the solver differential bypasses production extraction, determinism tests synthetic helpers rather than reordered package analysis, two fuzz targets have tiny self-comparing state spaces, and mutation kills are not preceded by a clean control run with exact failure attribution.

This design corrects the existing property-relative contract. It does not broaden the lint policy or promise whole-Go soundness. The supported boundary remains explicit; anything relevant outside that boundary becomes blocking inconclusive.

## Goals / Non-Goals

**Goals:**

- Make one conditional, identity-aware abstract domain authoritative from Go source extraction through diagnostic aggregation.
- Eliminate the audited false negatives for validation failure, wrong return identity, post-validation unknown mutation, witness/state mismatch, missing SSA, generic constraints, recursion, and no-return aliases.
- Make every claim in the semantic catalog and evidence index mechanically complete and tied to production behavior.
- Require independent, non-vacuous, causal evidence and preserve reproducible final proof.
- Delete alternate production semantics rather than adding modes or compatibility shims.

**Non-Goals:**

- Adding lint categories, CLI modes, policy exceptions, or user-facing features.
- Supporting arbitrary reflection, `unsafe`, dynamic dispatch, concurrency, pointer arithmetic, assembly, or whole-program effects precisely; relevant occurrences remain inconclusive.
- Adding an external SMT solver or general-purpose points-to framework.
- Preserving legacy `ValidatesTypeFact`, legacy CFG traversal, AST protocol evaluation, or stable IDs derived from those paths.
- Claiming absence of diagnostics is a proof beyond the documented property and abstraction boundary.

## Decisions

### 1. Use one production protocol state and edge-conditioned transfer

The production exploded supergraph will propagate a single state product per tracked fact: validation obligation, hazards, uncertainty set, and identity relation. A `Validate()` invocation creates a relation between a receiver identity and an error-result identity. The transfer is attached to the specific successor edge that proves that result nil; the call node itself is identity. Non-nil and unknown edges preserve the obligation, with unknown adding uncertainty.

All category adapters will submit origins, sinks, and protected effects to this engine. They may not precompute a boolean checked-call set or substitute a category-local state machine. A build-time architecture test will inventory declared domain types and production entry points, failing if the formal transfer is reachable only from tests or if another production validation state exists.

Alternative considered: retain checked positions and annotate their dominant region. Rejected because region reconstruction is a second semantics and again loses the explicit receiver/result relation at calls, summaries, and joins.

### 2. Represent identity as flow-sensitive SSA/object facts with explicit uncertainty

Identities will be interned for SSA values, allocations, parameters, receivers, result slots, addressable fields with static selectors, and proven copies. Each instruction updates a must-alias environment; rebinding, allocation, stores, dynamic addressing, incompatible phi inputs, and escaping operations kill relations that are no longer universal. Constructor validation follows the object reaching the exact return slot. There is no empty-key/type-only fallback and no undirected whole-body assignment closure.

Go SSA is not a full memory SSA, so the precise fragment is deliberately bounded. When a relevant store, interface conversion, dynamic index, closure capture, pointer flow, or external effect prevents a unique must-alias answer, the state gains `ambiguous-identity`, `escaped-heap-mutation`, `concurrent-mutation`, or another stable uncertainty reason. Validation never erases uncertainty. A known summary may establish that a call preserves or conditionally validates an identity; otherwise a relevant unresolved call after validation remains inconclusive.

Alternative considered: add exclusions to the existing assignment graph. Rejected because historical undirected reachability cannot express kills or the identity active at a particular program point.

### 3. Make SSA availability an explicit prerequisite result

SSA construction will return a typed result distinguishing available SSA from build failure, missing function/closure, unsupported instruction, and incomplete dependency information. Protocol entry points must consume that result before analysis. They may narrow the affected obligation slice, but may not continue with empty aliases, AST matching, type-only identity, or a nil analysis object when required information is absent.

Generic normalization will inspect complete `go/types` type sets. A supported primitive underlying term continues to create an obligation even when the interface also declares methods. Mixed or unsupported terms that affect the decision yield a stable inconclusive reason. This preserves the property conservatively without pretending unsupported syntax is outside the rule.

Alternative considered: make `buildssa.Analyzer` a global prerequisite. Rejected for now because goplint intentionally avoids building SSA for irrelevant packages; an explicit on-demand result retains that performance boundary while making failure observable.

### 4. Tabulate finite summaries instead of keying semantics by concrete call stacks

The solver will follow standard finite-fact tabulation. Work items are path edges keyed by procedure, node, entry fact, current fact, and conditional edge function. Procedure summaries map entry facts to exit facts/effects and are reused at every matching call site. Recursive calls add dependencies on summaries and converge through the monotone worklist; the concrete call stack is not part of the fixed-point key. Call-site identity remains on call and return edges so results cannot flow to unrelated continuations.

Resource limits bound work and evidence size, not semantics. If the worklist cannot reach a fixed point, the affected obligation is inconclusive. Recursion tests must assert final analyzer outcomes and summary reuse, not merely that a recursive edge exists.

Known no-return behavior is also SSA-local. Alias resolution uses the reaching function value at each call site. A continuation is pruned only when every realizable reaching definition resolves to a soundly modeled no-return target; conditional or ambiguous rebinding remains returning or inconclusive as appropriate.

Alternative considered: increase the call-depth budget. Rejected because it delays rather than establishes convergence and cannot satisfy the finite-summary contract.

### 5. Preserve provenance as part of the abstract contribution

Each propagated contribution will carry an immutable witness ID over fully qualified interprocedural edges: procedure, source and destination nodes, edge kind, call site, fact identity, and SSA predicate provenance. Joins combine abstract states and retain a deterministic antichain of contributing witness IDs. They never attach a worsened state to an unrelated earlier path.

Refinement consumes one exact contribution. Extraction rejects missing nodes, non-edges, unmatched calls/returns, cross-procedure index collisions, and silently skipped transitions. Checked UNSAT discharges only the hash of that witness, fact, state contribution, and normalized constraint evidence. Other contributions remain queued and independently aggregated.

To keep evidence bounded, deterministic subsumption may discard a witness only when the retained witness has the same state/fact contribution and is no less conservative under a documented ordering. Hitting the witness bound produces inconclusive.

Alternative considered: replace the representative path whenever a join worsens. Rejected because one path is insufficient when several incomparable contributions require separate feasibility decisions.

### 6. Make deadlines and budgets cooperative and fail closed

Constraint extraction, solving, evidence checking, refinement, generation, and tabulation loops will check a shared context or deterministic budget at bounded intervals and again before accepting SAT/UNSAT or emitting a discharge. A result completed after deadline is unknown even if its computation found a contradiction. Tests use injectable clocks/budgets for deterministic deadline boundaries rather than sleeps.

Alternative considered: check context only around synchronous calls. Rejected because a long synchronous solver can return an UNSAT result after its semantic deadline and incorrectly discharge a witness.

### 7. Delete every alternate semantic authority atomically

After adapters use the canonical state, identity, summary, and witness APIs, implementation will delete legacy traversal constants, legacy DFS entry points, nil traversal-context behavior, AST/type-only protocol fallbacks, old compatibility code, and unconditional `ValidatesTypeFact`. The replacement fact is versioned, slot-sensitive, identity-sensitive, and conditional on result state. Incompatible or missing relevant facts produce inconclusive.

A production-source architecture gate will use Go syntax/type information plus a narrow forbidden-symbol manifest. Text search remains a secondary stale-reference check. Test-only models live in clearly separate packages and cannot be selected by production flags or hidden variables.

Alternative considered: keep legacy code for rollback. Rejected because the prior change explicitly chose source reversion as rollback and because dormant semantic authorities routinely re-enter baselines, fixtures, or helper paths.

### 8. Enforce total semantic-catalog and oracle ownership

The registry becomes the source of category identity and semantic kind. Validation enforces a bijection among registered categories, rule contracts, live owner keys, and required oracle layers. Structural and cross-artifact categories receive real analyzer boundary fixtures appropriate to their predicate/relation. Protocol categories additionally require must-report, must-not-report, and must-be-inconclusive fixtures plus generated, metamorphic, fuzz, mutation, and determinism coverage. Coverage for `unvalidated-boundary-request` is added explicitly.

Owner keys resolve through typed registrations that the analyzer routing table itself uses; non-nil `[]any` function values are insufficient ownership proof. The gate fails missing and extra entries in either direction.

Alternative considered: document that only protocol categories use the oracle matrix. Rejected because the declared contract requires every registered category to have independent boundary evidence, and partial iteration is how the omission passed.

### 9. Make independent evidence exercise the claimed dimensions and production boundary

The bounded manifest will define actual enumerable dimensions: procedures, nodes, identities, call sites, call depth, branch topology, recursion, operations, conditional results, aliases/kills, unresolved effects, and supported constraints. The generator derives its corpus and exact cardinality from those values; it emits a machine-checked feature census. Unsupported combinations are rejected by explicit well-formedness rules, not silently hardcoded away. Reducing or ignoring a dimension fails a manifest-sensitivity test.

The independent interpreter remains in its separate package, but comparison gains an end-to-end harness that generates Go source, runs real parsing/type checking/SSA extraction/analyzer reporting, and compares normalized outcomes. Solver-core differential tests remain useful but are labeled as component evidence.

Determinism runs the real analyzer repeatedly while varying file order, package scheduling, worklist insertion order, map construction order, and equivalent parallel execution, then compares normalized findings, facts, witnesses, reasons, and evidence. Fuzz targets decode bytes into variable well-formed semantic structures and check against the reference model or explicit algebraic/metamorphic properties, not merely the production result against itself.

Alternative considered: expand only the existing fixed generator's slot count. Rejected because more operations over one topology do not test extraction, identity, call/return, joins, or refinement claims.

### 10. Require causal mutation evidence and reproducible completion proof

The targeted mutation runner first executes every selected guard unmutated and requires success. For each named mutant it verifies the exact anchor and semantic transformation, compiles the mutant, runs only the declared guards, and accepts a kill only when an expected guard fails with a structured assertion identifying the intended semantic mismatch. Compile failures, timeouts, crashes, unrelated test failures, and generic `FAIL` text are distinct non-kill outcomes. A post-run control proves restoration and repeatability.

Completion uses the existing temporary-index/synthetic-tree method but records evidence under the change directory: synthetic tree ID, base commit, diff identity, Go/tool versions, exact commands, per-gate outcomes, catalog counts, generator cardinality/census, mutant attribution, and counterexample inventory. Evidence files are generated or command-captured where possible and checked for freshness. Documentation maps every guarantee to both a production symbol and a blocking evidence surface.

Alternative considered: rely on CI logs and checked task boxes. Rejected because they are not durable proof of the exact uncommitted synthetic tree and do not expose vacuous gate coverage.

## Risks / Trade-offs

- **[Risk] Correct fail-closed handling increases inconclusive findings.** → Add stable reason-specific fixtures and improve precision only inside the documented domain; do not add warning/off controls.
- **[Risk] Provenance antichains and summary tabulation increase memory.** → Intern identities and edges, use deterministic same-contribution subsumption, benchmark state/witness cardinality, and map exhaustion to inconclusive.
- **[Risk] End-to-end generated analysis is expensive.** → Keep a reviewed blocking bound for every change, partition deterministic shards, and reserve larger bounds for scheduled/manual runs without weakening the blocking minimum.
- **[Risk] Removing legacy facts changes cross-package findings and IDs.** → Migrate producers and consumers atomically, reject mixed facts, generate an old-to-new report, and review the baseline only after counterexamples pass.
- **[Risk] Architecture checks become brittle text bans.** → Prefer Go AST/type/routing-table assertions; keep forbidden-symbol text scans narrow and secondary.
- **[Risk] The independent model repeats the written mistake.** → Combine reference comparison with adversarial explicit fixtures, metamorphic relations, causal mutants, and implementation-independent review of the normative transfer table.
- **[Risk] The active correction drifts from the already archived predecessor.** → Treat this change as dependent on the archived `harden-goplint-soundness` artifacts, record a requirement/task reconciliation against them, validate their synchronized canonical result alongside this change, and keep only this corrective change unarchived until the stricter requirements pass.

## Migration Plan

1. Add failing end-to-end counterexamples for every audited production gap and failing meta-tests for every vacuous evidence claim; record the expected red baseline without changing production behavior.
2. Replace checked-position propagation with edge-conditioned production state, migrate identity and uncertainty handling, and make SSA/generic failures explicit.
3. Introduce finite summary tabulation and contribution-paired witnesses/refinement; migrate all protocol adapters and real analyzer fixtures.
4. Replace conditional facts and no-return resolution, then delete all legacy production paths and make architecture absence checks blocking.
5. Make catalog coverage total; rebuild bounded generation, end-to-end comparison, determinism, fuzz, and causal mutation evidence.
6. Update gates, baselines, documentation, and evidence mappings only after semantic counterexamples and architecture checks pass.
7. Run nested-module and repository validation, then materialize the exact intended diff in a clean synthetic-tree worktree and record the complete proof bundle.

Rollback is a source revert of this change together with any dependent unarchived hardening commits. No runtime switch, legacy fact reader, or fallback engine will be retained for rollback.

## Open Questions

None. The change deliberately chooses conservative inconclusive outcomes where current analysis cannot prove the required identity, effect, path, or predicate relation.
