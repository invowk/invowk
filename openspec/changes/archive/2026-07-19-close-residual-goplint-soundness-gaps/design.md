## Context

Two active corrective changes already define and implement goplint's property-relative soundness contract. `complete-goplint-soundness-hardening` introduced the canonical SSA/IFDS/IDE pipeline and `close-goplint-soundness-review-gaps` corrected ordered calls, deferred validation, closure procedures, unsuppressible uncertainty, and evidence causality. Both remain unarchived and their combined proof is incomplete.

The latest adversarial review found residual places where implementation shortcuts bypass those declared semantics:

- constructor returns are classified from ancestor condition text without distinguishing the `then` and `else` control-flow edges;
- summary application collapses result-conditioned validation into unconditional validation after mutation;
- protocol routing begins only at function declarations, omitting package-level stored function literals;
- closure exceptions and inline ignores run before uncertainty classification;
- post-validation non-call escape handling and imported fact validation do not cover every relevant identity/effect shape;
- stable IDs and directive validation can alias or disappear because they depend on package leaf names, file-set positions, or incomplete directive traversal;
- several assurance producers award category, stage, fuzz, metamorphic, determinism, or population credit without executing the claimed semantic boundary;
- the targeted mutation profile currently has surviving post-validation mutants, and the v2 exact-tree completion record does not exist.

This change depends on both active predecessors and corrects their combined implementation. It does not replace their artifacts, broaden the supported language fragment, or introduce another protocol engine.

## Goals / Non-Goals

**Goals:**

- Remove every confirmed residual false negative and suppression path from the review.
- Make successful-return, summary, closure, escape, imported-fact, finding-ID, and directive behavior follow one typed, fail-closed authority.
- Make category evidence and aggregate populations derive from executed semantic observations rather than declarations or test names.
- Make mutation attribution prove the intended mismatch and require zero survivors.
- Bind completion evidence to every intended tracked and untracked change across all three dependent changes.
- Preserve the existing property boundary: unsupported relevant behavior is blocking inconclusive.

**Non-Goals:**

- Adding lint categories, CLI flags, semantic modes, compatibility readers, policy exceptions, or advisory downgrade paths.
- Claiming soundness for arbitrary reflection, `unsafe`, assembly, external mutation, dynamic loading, or whole-program concurrency.
- Replacing Go SSA, adding an external theorem prover, or introducing a second points-to/dataflow framework.
- Solving repository findings by baseline, exception, inline-ignore, or evidence-label migration.
- Reopening or rewriting the archived `harden-goplint-soundness` change.

## Decisions

### 1. Model constructor success from the exact return edge and result identities

Constructor obligations will classify a return from the SSA/CFG edge reaching that return and the exact error result value on that edge. The implementation will remove the ancestor-condition heuristic that treats both branches of `if err != nil` alike. A return is excluded as unsuccessful only when the active edge proves the returned error non-nil; nil, unknown, ambiguous, or mismatched error identity cannot remove the object-return obligation.

If the result relation cannot be reconstructed, the return remains in scope and the affected obligation becomes inconclusive. An empty target set is safe only when the function has no realizable successful non-nil object return, not when target extraction discarded one.

Alternative considered: teach the AST ancestor scan to recognize whether a return is under `Body` or `Else`. Rejected because nested conditions, inverted predicates, aliases, switches, and SSA phi values would remain a second incomplete success semantics.

### 2. Preserve conditional summary effects as ordered relations

Summary application will consume `Condition`, `ConditionResultSlot`, target slot, and effect order directly. A validation effect conditioned on result slot `r == nil` changes typestate only on a caller continuation proving the corresponding call result nil. Mutation followed by conditional validation therefore remains invalidated or uncertain when the caller discards, overwrites, or cannot relate the error.

The generic summary-to-state helper may compose unconditional effects, but it cannot convert a conditional validation into a proven state by supplying a synthetic nil result. Same-package summaries, imported facts, recursive summaries, constructor paths, cast paths, and post-validation effects will use the same conditional transfer API.

Alternative considered: special-case `MutateThenValidate`-style helpers at call sites. Rejected because it would duplicate summary semantics and miss other ordered effect combinations.

### 3. Build one package-wide procedure-root inventory

Protocol analysis will start from a package-wide inventory of function declarations and every function literal body, including literals in package variable initializers. Nested, returned, stored, passed, deferred, and concurrently launched literals remain ordinary procedures. Package initialization provides execution context for initializer expressions, while each literal receives local protocol analysis independent of whether an invocation is visible.

The routing table will consume this inventory rather than requiring an `*ast.FuncDecl` root. Duplicate discovery must be impossible, and missing SSA/procedure identity for a relevant literal is blocking inconclusive rather than silent omission.

Alternative considered: wrap package-level literals in synthetic `FuncDecl` nodes for the existing route. Rejected because synthetic AST ownership and positions would corrupt SSA association, finding identity, and call-site provenance.

### 4. Classify proof outcomes before applying policy suppression

All function and closure protocol entry points will run semantic classification first. Violation findings may then consult their documented policy surface; inconclusive outcomes always use the dedicated always-visible sink and never consult exceptions, inline ignores, or baselines. Shared reporting helpers will enforce this ordering so closure-specific code cannot drift.

Architecture and behavioral tests will enumerate every protocol entry point and reject any pre-analysis suppression branch. Existing exception behavior for definite structural/policy findings remains unchanged.

Alternative considered: reject exception configuration for any function containing a closure. Rejected because it changes policy scope and still leaves ordering duplicated.

### 5. Validate every relevant escape and imported fact against typed identity

Post-validation non-call transfer will conservatively recognize pointer/channel sends, aggregate storage, indirect stores, package storage, and other supported escapes of the tracked identity. A value copy remains safe when identity cannot be mutated through it; an address, pointer-like alias, or unresolved relevant store becomes invalidated or blocking inconclusive.

Imported `ProtocolSummaryFact` validation will be bound to the attached `types.Func` signature. Target, source, and condition-result slots must exist and have compatible roles and types; function/package identity and effect conditions must match the supported fact version. Malformed relevant facts are incompatible and blocking, never silently skipped during application.

Alternative considered: keep schema-only nonnegative slot validation. Rejected because syntactically valid but impossible slots can convert a relevant call into an apparently resolved no-effect call.

### 6. Make directives and finding IDs globally semantic

Stable finding IDs will include the full import path plus source-local semantic identity. Raw `token.Pos`, file-set ordering, and package leaf names cannot be identity inputs. A migration report will map old to new IDs before any baseline change; collisions or unexplained churn fail validation.

Directive discovery will inspect every supported attachment location, including declaration and type documentation. Unknown names, misspellings, duplicate/conflicting directives, and missing required arguments fail visibly. Known parameterized directives cannot be accepted and then silently ignored because their value is absent.

Alternative considered: retain leaf-qualified IDs and add collision checks only for the current repository. Rejected because dependency/package composition can introduce future collisions and the ID contract is global.

### 7. Credit evidence only from its executed semantic observation

Each evidence producer will emit observations derived from the cases it actually executed:

- metamorphic evidence must transform a program while preserving or predictably changing semantics, not reorder independent fixtures;
- fuzz evidence must decode variable semantic structure and check an independent relation, not classify seed labels;
- per-category determinism must run that category under every credited reorder dimension;
- mutation evidence must name the exact mutated production stage and cannot claim unexercised stages;
- fixed explicit fixtures remain valuable but are labeled boundary-oracle evidence, not an executable independent model.

The registry/census will reject an observation whose claimed stages, properties, dimensions, category, or boundary are not present in producer-emitted case data.

Alternative considered: keep broad observations and document which portions are indirect. Rejected because indirect declarations are the current false-credit mechanism.

### 8. Census aggregate tests and causally attribute mutations

Every script that reports a test or run population will first enumerate the exact required tests/cases and fail on missing, duplicate, skipped, or zero populations. `go test -run` success without a matching test is not evidence. Population reports will be generated from observed executions rather than constants.

The mutation runner retains clean controls, exact anchors, compile checks, restoration, and repeatability, and adds structured mismatch attribution from the expected guard. Failure of the expected test name for setup, unrelated assertion, generic output, panic, timeout, or compilation remains an invalid mutant outcome. Every category receives causal mutants for each applicable production stage, and the blocking profile requires zero survivors.

Alternative considered: accept exact failing test names as sufficient attribution. Rejected because any unrelated failure inside the expected test currently counts as a semantic kill.

### 9. Prove the complete intended diff and enforce archive order

The clean-tree path selection will be checked against the complete repository delta for the three dependent changes, including untracked content and final task/artifact state. Explicit exclusions must be machine-readable, justified as unrelated, and reviewed; silently omitted changed paths invalidate freshness.

Completion order is fixed:

1. implement and verify all remaining tasks in `complete-goplint-soundness-hardening`;
2. implement and verify all remaining tasks in `close-goplint-soundness-review-gaps`;
3. implement this change and reconcile every review finding with production and evidence;
4. materialize one exact combined synthetic tree, run the corrected core aggregate and repository gates, and retain the v3 proof record;
5. verify freshness from the caller checkout, then sync/archive the changes in dependency order with strict validation after each step.

Alternative considered: generate a proof independently for each active change. Rejected because the implementation is one overlapping uncommitted tree and independent path lists could omit cross-change interactions.

## Risks / Trade-offs

- **[Risk] Correct branch and summary handling exposes more violations or inconclusives.** → Treat them as evidence of the prior false negative; improve precision only within the existing contract and never suppress proof uncertainty.
- **[Risk] Package-wide literal routing duplicates diagnostics or increases analysis cost.** → Use stable procedure identity and a single visited inventory, then add package-initializer benchmarks and deterministic duplicate checks.
- **[Risk] More conservative escape/fact handling increases cross-package uncertainty.** → Add exact value-copy versus mutable-alias fixtures and improve slot/type precision without reintroducing fallback semantics.
- **[Risk] Stable-ID correction causes large baseline churn.** → Produce a deterministic collision/migration report, require repeated byte-identical scans, and review each changed identity before baseline acceptance.
- **[Risk] Real metamorphic, fuzz, mutation, and determinism evidence is expensive.** → Keep a reviewed nonzero blocking corpus, shard larger scheduled runs, and bind both profiles to the same semantic producers.
- **[Risk] Three dependent active changes complicate task and archive bookkeeping.** → Maintain an explicit cross-change reconciliation and one combined proof; do not check tasks or archive from artifact readiness alone.

## Migration Plan

1. Record failing production-boundary counterexamples and adversarial evidence tests for every confirmed finding before changing implementation.
2. Correct constructor return semantics, summary condition composition, package-wide procedure routing, suppression ordering, escape effects, and fact validation.
3. Migrate finding IDs and directive validation with deterministic reports; resolve newly visible findings without new suppression.
4. Replace false-credit evidence producers and hard-coded aggregate populations; add stage-specific causal mutants and structured mismatch attribution.
5. Run focused tests after each semantic correction, then all nested-module tests, race/repeat, fuzz seeds, scheduled oracle, determinism, mutation, benchmarks, full scan, lint, Windows build, licensing, file-length, and agent-document gates.
6. Reconcile and finish the predecessor task ledgers, generate the complete combined synthetic-tree proof, verify freshness, and archive only in dependency order.

Rollback is a source revert of this dependent corrective implementation. No alternate analyzer, legacy fact reader, suppression compatibility, or evidence bypass will be retained as a rollback mode.

## Open Questions

None. When the implementation cannot prove an exact return, identity, condition, effect, directive, fact, or evidence relation inside the existing property boundary, the required outcome is visible failure or blocking inconclusive.
