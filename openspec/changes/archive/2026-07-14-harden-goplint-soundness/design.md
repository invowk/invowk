## Context

The current goplint protocol checks grew through four rollout phases: an AST/CFG evaluator, a custom IFDS-style path, an opt-in predicate/refinement layer, and opt-in SSA must-alias enrichment. The rollout controls remain embedded in production flags, tests, baseline targets, CI, and documentation. The resulting default is internally mixed: IFDS is selected by default, baseline maintenance still pins the legacy evaluator, feasibility and SSA aliasing are disabled, and the semantic oracle itself forces legacy behavior.

The soundness audit identified five root causes rather than isolated fixture misses:

1. A syntactically observed `Validate()` call is treated as successful without proving its error is nil.
2. receiver/return matching can fall back to type equality and a flow-insensitive undirected alias graph.
3. feasibility predicates identify mutable Go objects rather than SSA versions, so reassignment can create false contradictions.
4. the interprocedural graph and result mapping do not consistently preserve realizable call/return paths or known non-returning behavior.
5. the semantic catalog and compatibility gates are coupled to implementations they are meant to validate.

This design replaces the staged rollout with one production semantics. It treats soundness as property-relative: goplint proves only its documented validation protocols over supported Go constructs. Unsupported constructs, incomplete call resolution, and exhausted budgets produce blocking inconclusive outcomes; they never silently become safe.

## Goals / Non-Goals

**Goals:**

- Make successful validation, receiver identity, alias kills, calls, returns, and path feasibility explicit in one formal abstract semantics.
- Use one SSA-backed, call/return-matched interprocedural engine for cast validation, use-before-validation, and constructor validation.
- Make alias and predicate reasoning SSA-versioned and conservative under ambiguity.
- Fix known generic, method-value, cross-package, and no-return blind spots.
- Replace self-referential rollout checks with independent semantic, generated, metamorphic, fuzz-seed, mutation, determinism, and performance gates.
- Remove production legacy code and semantic-mode flags in the same change.
- Ensure documentation and result vocabulary never claim a stronger proof than the analyzer produced.

**Non-Goals:**

- Turning goplint into a general Go verifier, whole-program theorem prover, security sandbox, or replacement for the Go compiler.
- Adding new lint categories or extending Invowk's value-type policy.
- Proving arbitrary reflection, `unsafe`, assembly, dynamically loaded code, or unresolved external behavior. Relevant uncertainty in these areas is inconclusive.
- Keeping legacy behavior available for rollback, comparison, baseline stability, or external callers.
- Adding a native/cgo or separately installed SMT solver dependency in this change.

## Decisions

### 1. Define one property-relative semantic contract

Each registered category will declare its semantic kind:

- **Structural** rules define an AST/type predicate and diagnostic location.
- **Protocol** rules define concrete events, tracked identities, abstract states, transfer functions, joins, call/return behavior, terminal behavior, escape behavior, and uncertainty boundaries.
- **Cross-artifact** rules define the compared domains, normalization, and equality/subset relation.

The protocol state uses a must-property lattice per SSA/object identity. Validation is established only when every incoming executable path carries validation success for that same identity. Joins therefore discard `validated` when any predecessor lacks it. Budget exhaustion, unsupported instructions, ambiguous points-to relationships, or unresolved relevant calls yield `inconclusive`.

The catalog becomes executable metadata backed by Go identifiers or registered rule keys. A validation test resolves every declared implementation owner, requires every category to have semantic and oracle ownership, and rejects stale or duplicate declarations. Commit snapshots are generated evidence, not manually maintained semantic claims.

Alternative considered: retain the current prose/JSON skeleton and add fixtures. Rejected because populated strings and implementation-derived fixtures cannot establish that the declared semantics exist or are complete.

### 2. Model validation as a conditional effect tied to an object

The canonical IR records a `Validate()` invocation as `(receiver identity, result identity)`. The receiver transitions to validated only on an edge where the result is established nil. Supported idioms include:

- `if err := value.Validate(); err != nil { return ... }`, with validation established after the terminating failure branch;
- `if err := value.Validate(); err == nil { use(value) }` on the true branch;
- explicit error assignment followed by a dominating nil check;
- constructor returns such as `return value, value.Validate()`, where the value is valid only for the caller's nil-error continuation;
- selector calls, interface-resolved calls, and method values whose captured receiver is recoverable from SSA.

Merely evaluating, assigning, logging, blank-assigning, or returning a validation error without a nil-conditioned continuation does not establish validation on that path. Helper summaries carry conditional postconditions such as “argument slot 0 is validated when result slot 0 is nil”; they do not carry an unconditional type-level `validates` bit.

Alternative considered: strengthen only `check-validate-usage`. Rejected because result-use syntax cannot prove control-flow success and would leave constructor/helper false negatives intact.

### 3. Track SSA values and abstract objects, never type-only targets

Tracked identities consist of SSA values plus abstract allocation/parameter/result objects. Phi nodes and copies create explicit relations. Stores, reassignment, and new allocations kill must-alias facts. A validation effect transfers only across a proven must-alias relation. May-alias without must-alias is inconclusive when it can affect the obligation.

The constructor analysis removes the type-only matcher and undirected historical assignment graph. It proves that the validated identity reaches the corresponding return slot. Cross-package facts are package-qualified, format-versioned, parameter/result-slot-sensitive, and conditional on returned error state. Filtered packages still export required facts.

Generic calls represented by `IndexExpr`/`IndexListExpr` and instantiated signatures are normalized through `go/types`; type parameters use their type sets/underlying basic terms when deciding whether a raw primitive obligation exists. Method-set resolution consistently observes pointer/value receivers and exported `Validate`, without treating lowercase lookalikes as the protocol method.

Alternative considered: retain the local graph and add reassignment exclusions. Rejected because successive exclusions would remain a partial alias analysis with no clear proof boundary.

### 4. Replace the custom interprocedural path with a realizable-path tabulation solver

The canonical solver uses an exploded supergraph over procedures, nodes, and finite protocol facts. Calls have call edges, return sites, and summary edges; returns propagate only to the matching caller context. A call-to-return edge exists only for a formally modeled effect and never as an unconditional bypass. Recursion converges through finite summary facts and a worklist fixed point rather than depth-first cycle guesses.

IDE-style edge functions carry conditional validation/error relations while IFDS-style facts carry object typestate and escape state. The same engine serves cast validation, UBV escape semantics, and constructor return validation; category adapters supply origins and sinks but cannot substitute an alternate evaluator.

Known terminal calls are represented through a single `mayReturn` contract built from CFG/compiler-recognized behavior, intrinsics, and analyzed function bodies. `panic`, `os.Exit`, `log.Fatal`/`Fatalf`/`Fatalln`, and soundly resolved aliases are covered by tests. Unknown calls are assumed to return unless non-returning behavior is proven; relevant unresolved effects remain inconclusive.

Alternative considered: repair the existing IFDS adapter while retaining the legacy evaluator for fallbacks. Rejected because dual semantic authorities caused the current oracle and baseline divergence and make clean soundness claims impossible.

### 5. Use SSA-versioned feasibility with checked evidence and iterative refinement

The current `smt` name is removed. The replacement is an exact decision procedure for a documented, finite predicate fragment over SSA versions: equality/inequality, nil tests, supported boolean combinations, and type/value constants needed by current witnesses. Each UNSAT decision produces normalized contradiction evidence that a separate checker validates before a witness may be discharged. Mutable source variables never share a constraint subject after SSA renaming.

Unsupported predicates, missing SSA, query or iteration limits, internal failures, and evidence-check failures return `unknown`, which maps to blocking inconclusive. SAT preserves the witness. Iterative refinement replays the candidate path, identifies the abstraction responsible for a spurious counterexample, and adds supported predicates until the witness is feasible, soundly discharged, or the resource budget is exhausted. A one-pass mode does not exist.

This deliberately prefers a small auditable decision procedure over an external SMT dependency. It closes the unsoundness and terminology gap without cgo, an ambient solver binary, or cross-platform version drift. The public metadata names the backend `ssa-constraints`, not SMT. A future general SMT backend would require a separate proposal and the same checked-evidence contract.

Alternative considered: add Z3 immediately. Rejected because deployment and solver trust would grow substantially while the current required predicate fragment is small; calling an external solver “state of the art” would not by itself make extraction, SSA modeling, or result handling sound.

### 6. Use accurate result vocabulary

The analyzer emits:

- `violation` when a feasible supported path demonstrates the prohibited protocol event;
- `inconclusive` when a relevant path cannot be classified soundly;
- `discharged-infeasible` only in structured refinement traces with checked UNSAT evidence.

`proven-safe` is removed. Absence of a diagnostic means no violation was found under the documented supported model; it is not an unrestricted proof about arbitrary Go execution. Inconclusive outcomes for protocol categories are always blocking and cannot be downgraded by a CLI mode. Existing exception/baseline governance remains visible, but canonical soundness gates run unsuppressed over the semantic oracle and the repository's protocol-critical test corpus.

Alternative considered: keep `proven-safe` for completed traversals. Rejected because the label invites whole-language interpretations beyond goplint's declared abstraction boundary.

### 7. Remove semantic mode selection at the API boundary

The following flags and their backing production paths are deleted, with unknown-flag failure as the only migration behavior:

- `--ubv-mode` (escape semantics is canonical);
- `--cfg-backend` (SSA is canonical; missing SSA is inconclusive);
- `--cfg-interproc-engine` (the corrected tabulation solver is canonical);
- `--cfg-inconclusive-policy` (protocol inconclusive is blocking);
- `--cfg-feasibility-engine` (SSA constraints are canonical);
- `--cfg-refinement-mode` (iterative refinement is canonical);
- `--cfg-alias-mode` (SSA/object-sensitive tracking is canonical).

State, witness-size, refinement-iteration, query, and timeout limits remain configurable performance controls. Their only semantic effect is to turn unfinished work into inconclusive; no value can make analysis optimistic. Internal tests may instantiate narrowly scoped components directly, but no legacy analyzer implementation or hidden runtime selector remains.

### 8. Make the oracle independent and layered

The verification strategy has six layers:

1. table tests for lattice laws, transfers, joins, kills, conditional effects, and summary composition;
2. an independent test-only reference interpreter over a small normalized protocol IR, compared against the production solver on exhaustively generated bounded graphs;
3. hand-authored must-report, must-not-report, and must-be-inconclusive Go fixtures for every protocol rule and every historical counterexample;
4. metamorphic transformations such as alpha-renaming, branch inversion, equivalent nil-check forms, selector/method-value forms, and alias-copy/rebind variants;
5. Go fuzz targets with committed seed corpora for graph construction, fact propagation, constraint normalization/evidence checking, catalog decoding, and finding determinism;
6. targeted mutation profiles proving tests kill removed nil-branch, alias-kill, matched-return, terminal-call, generic-normalization, and UNKNOWN-to-safe guards.

The reference interpreter and expectation manifests do not call production transfer functions. Every registered category has an explicit oracle strategy; structural categories need predicate boundary cases, while protocol categories require all six applicable layers. Repositories gates run deterministic seeds and bounded exhaustive generation on every change, with longer fuzz/mutation runs in their existing dedicated workflows.

### 9. Replace rollout gates atomically

Baseline generation/checking, the regular full scan, pre-commit, and CI all invoke the same flagless canonical semantics. The advisory full scan becomes blocking. `check-ifds-compat` is deleted; `check-semantic-spec`, `check-cfg-refinement`, and `check-cfg-alias` are consolidated or rewritten as a canonical `check-goplint-soundness` gate with subchecks that remain directly runnable.

The baseline is regenerated only after the semantic corpus passes, and its diff is reviewed. Stable finding IDs may change when old metadata encoded mode names; the migration records an explicit old-to-new ID report. Baseline compatibility does not justify retaining an old solver.

### 10. Make proof boundaries and completion evidence explicit

Protocol outcomes aggregate per obligation. A supported feasible witness is a `violation` even when a different path is unresolved. When there is no feasible violation, any unresolved relevant path makes the obligation `inconclusive`. Only when every relevant path is classified and no feasible violation exists may the analyzer emit no diagnostic. `discharged-infeasible` is trace-only evidence for one checked UNSAT witness; it is never a top-level safe result and never suppresses uncertainty on another path.

An effect is relevant only when it lies on the obligation slice: it is forward-reachable from the obligation origin, can reach the protected sink or constructor return, and may validate, mutate, alias, escape, consume, terminate, or constrain feasibility for a tracked identity. Unknown behavior outside that slice does not poison unrelated obligations. An unresolved call or instruction on the slice is inconclusive whenever any conservative effect it may have could change the result.

The initial object domain is field-sensitive for SSA values, allocations, parameters, receivers, result slots, copies, phi nodes, and statically resolved field/index addresses. Interface wrapping, closures, and pointer flows are tracked only when SSA resolution preserves a unique must-alias identity. Dynamic indices, unresolved interface dispatch, reflection, `unsafe`, concurrent mutation, and escaped heap mutation are outside the precise alias fragment; when relevant, they produce stable inconclusive reasons rather than type-based matching.

The `ssa-constraints` fragment contains SSA-versioned nil tests, boolean values, and equality or inequality between one SSA subject and a typed nil, boolean, string, or integer constant. It supports logical negation and short-circuit conjunction/disjunction after CFG edge normalization. Pointer and interface comparisons are supported only against nil or when both operands resolve to the same interned identity. Arithmetic, ordering, floating-point semantics, dynamic type assertions, reflection, and predicates containing unresolved calls are unsupported. Normalized UNSAT evidence records the subject, typed constants, contradicting atoms, CFG edges, and extraction provenance; a separately implemented checker reparses that evidence and accepts only a contradiction derivable in this fragment.

Generated-oracle bounds live in a versioned machine-readable manifest. The initial manifest must cover, at minimum, two procedures, four protocol nodes per procedure, two tracked identities, two call sites, call depth two, a branch join, and a recursion edge. The generator enumerates every well-formed normalized program admitted by that manifest; reducing a bound requires an explicit reviewed manifest change. The targeted mutation manifest is likewise versioned, names each non-equivalent soundness mutation, and requires zero survivors.

Performance budgets live in the reviewed benchmark-threshold manifest and cover time, bytes, and allocations for solver, alias, refinement, generated-graph, and full-scan workloads. The manifest records the Go toolchain and CI runner class used for review. The blocking benchmark gate compares the median of five fresh runs with those limits; changing a limit requires an explanatory review note and cannot re-enable a weaker semantic mode.

The final clean-checkout proof uses an isolated detached worktree materialized from a synthetic tree object that contains exactly `HEAD` plus the intended implementation diff. The synthetic tree is built with a temporary Git index so the caller's branch and index are not mutated. The worktree must be clean before validation, and all required goplint gates run there without relying on build artifacts from the development worktree.

## Risks / Trade-offs

- **[Risk] The canonical pipeline initially reports more inconclusive outcomes.** → Treat them as actionable model gaps, classify their reasons deterministically, and close repository-local cases before enabling the blocking full scan; never add a warning/off escape hatch.
- **[Risk] SSA/object tracking and tabulation increase memory and runtime.** → Use finite interned facts, summary reuse, deterministic worklists, resource budgets, and existing benchmark thresholds; budget exhaustion remains inconclusive.
- **[Risk] A small constraint fragment leaves feasible paths unresolved.** → Return unknown and retain the finding/inconclusive; extend the fragment only with formal semantics and oracle coverage in a later change.
- **[Risk] The test reference model repeats production mistakes.** → Keep its representation and implementation separate, combine it with metamorphic, adversarial, fuzz, and mutation evidence, and require review of semantic changes against the written transfer rules.
- **[Risk] Clean removal breaks scripts or developer invocations.** → Inventory every repository occurrence, update them atomically, document the removed flags and flagless replacement, and intentionally provide no compatibility alias.
- **[Risk] Cross-package fact format changes produce mixed results in analysis drivers.** → Version facts and fail inconclusive on incompatible/missing relevant versions; goplint runs compile one tool version across the full package graph.
- **[Risk] “Sound” is interpreted as all-Go whole-program soundness.** → State the supported semantic boundary in the spec, README, diagnostics, and evidence; remove `proven-safe` and all unqualified soundness claims.

## Migration Plan

1. Add the formal domains, independent reference model, and counterexample fixtures while the current code still builds; use direct component tests rather than adding another production mode.
2. Implement the canonical SSA identities, conditional validation effects, realizable-path solver, terminal summaries, and SSA-constraint refinement behind internal APIs.
3. Move all protocol adapters and semantic-oracle tests to the canonical APIs, close every false negative/positive and inconclusive regression, and satisfy performance budgets.
4. Delete legacy/compare, AST, order, off, once, and policy-downgrade code plus their flags and tests. Do not leave deprecated constants, hidden flags, aliases, or fallback branches.
5. Replace scripts, Make targets, CI, pre-commit, baseline invocations, metadata, and documentation; regenerate and review stable IDs/baseline with the canonical engine.
6. Run the full nested-module test/race/fuzz-seed/mutation/lint/soundness/benchmark suite, root repository gates, `make check-agent-docs`, and OpenSpec strict validation.

Rollback is a source revert of the complete change, not a runtime legacy switch. The change must not merge in a state where repository automation depends on removed paths or where two production semantic engines coexist.

## Open Questions

None. A general external SMT solver, deeper context sensitivity beyond the finite tabulation contract, and broader points-to domains require separate evidence and proposals rather than blocking this soundness closure.
