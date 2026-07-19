## Context

`complete-goplint-soundness-hardening` replaced much of goplint's earlier protocol analysis with a canonical SSA-backed, finite-summary, fail-closed pipeline and added extensive oracle, fuzz, mutation, determinism, race, benchmark, and clean-tree evidence. Its exact uncommitted synthetic tree passes the aggregate gate. A subsequent adversarial review nevertheless reproduced three silent false negatives: an inner mutating call hidden by an outer call, a conditional deferred constructor validation accepted as unconditional, and an unchecked conversion inside a returned closure that is never analyzed. The same review found that protocol inconclusives are baseline-suppressible and that multiple assurance layers award coverage through marker presence or generic predicates rather than causal execution.

This change is a corrective dependency of `complete-goplint-soundness-hardening`. It does not broaden the supported property or add analyzer policy. It makes the implementation and proof stack satisfy the already-declared contract.

## Goals / Non-Goals

**Goals:**

- Eliminate the three reproduced false negatives and lock them with production-boundary counterexamples.
- Make nested and sibling call effects follow Go evaluation order and fail closed when the order or effect cannot be modeled.
- Prove deferred validation on every successful constructor return rather than recognizing it syntactically.
- Analyze every executable closure body independently, including returned, stored, and passed closures.
- Make every protocol inconclusive diagnostic impossible to baseline, suppress, downgrade, or omit from blocking scans.
- Replace declarative coverage credit with category-specific, executable, causal evidence.
- Make aggregate orchestration, differential fuzzing, evidence perturbation, scheduled profiles, benchmarks, and clean-tree proof demonstrate the production claims they advertise.

**Non-Goals:**

- Adding lint categories, CLI modes, advisory policies, or compatibility paths.
- Claiming whole-program Go soundness or precise support for arbitrary reflection, `unsafe`, dynamic dispatch, concurrency, or external mutation.
- Resolving every current inconclusive through broader language modeling; unresolved relevant behavior remains blocking.
- Replacing the canonical protocol domain or introducing a second analyzer engine.

## Decisions

### 1. Expand every relevant call into an ordered interprocedural event

The interprocedural graph will no longer select one `CallExpr` per AST CFG node. It will derive the ordered call events for the node from typed SSA instruction order, map them back to the source node, and expand them into a chain of call and matching return micro-nodes before the original continuation. Nested argument calls therefore transfer before the outer call, and sibling argument calls follow the order established by the compiled SSA. Each call receives its own call-site identity, summary lookup, mutation/escape effect, uncertainty reason, and realizable return edge.

If a relevant source call cannot be associated with a unique ordered SSA event, the affected obligation becomes inconclusive. AST preorder is not a semantic fallback.

Alternative considered: recursively collect every AST `CallExpr` and reverse preorder for nested arguments. Rejected because Go evaluation order and compiler lowering are not captured soundly by a generic tree-order heuristic.

### 2. Analyze deferred constructor validation with the canonical path engine

A deferred closure will not establish validation because its body contains a matching validation assignment and a separate assignment to a named error. At each constructor return, the analyzer will execute the deferred closure summary in LIFO order. Validation is established only if every realizable deferred path reaching that successful return executes the matching `Validate()`, propagates that invocation's result to the returned error slot, and does not overwrite or disconnect the result afterward.

Conditional execution, early return or panic inside the closure, ambiguous captures, unresolved calls, and result overwrites remain unvalidated or inconclusive according to the ordinary protocol domain. The existing syntactic recognizer is removed after the canonical path is active.

Alternative considered: add condition and overwrite checks to the syntactic recognizer. Rejected because it would create a second path semantics and would continue to miss loops, helper calls, aliases, and nested control flow.

### 3. Treat every function literal as an analyzable procedure

All function literals with bodies will be registered as procedures during collection, independent of whether their invocation is visible in the enclosing body. IIFEs, `go`, `defer`, same-body calls, returned closures, stored callbacks, and closures passed to another function therefore receive ordinary local protocol analysis. Known invocation sites additionally receive interprocedural call/return edges.

Local obligations inside an escaping closure are reported from that closure's body. Obligations depending on captured values use SSA closure bindings and must-alias facts; incomplete capture or external invocation effects are blocking inconclusive when relevant. A closure is not skipped merely because the analyzer cannot prove when it runs.

Alternative considered: classify returned closures as non-executable until a local call is found. Rejected because returned and registered callbacks are executable code and silent omission violates package-level analysis expectations.

### 4. Separate suppressible policy findings from always-visible proof uncertainty

All protocol inconclusive categories will use the always-visible baseline policy. Baseline parsing and update paths will reject entries for always-visible categories, and diagnostic reporting will bypass suppression for inconclusive outcomes even if a stale or malformed baseline row exists. Blocking scans will fail while any inconclusive remains.

Existing inconclusive baseline entries must be removed. Each resulting finding is either resolved by stronger proof or remains visible and blocks completion; it is not migrated to another suppression mechanism.

Alternative considered: retain baseline suppression for known repository debt while documenting it. Rejected because the canonical specification explicitly forbids hiding an incomplete proof.

### 5. Register executable evidence per category and layer

The semantic evidence registry will identify an exact category, evidence layer, executable test or command, semantic feature IDs, and expected observations. A predicate such as `SemanticKind == protocol`, a file path, or a marker substring cannot award coverage. The census will require the declared evidence to emit machine-readable observations for that category during the owning gate.

Constructor validation, cast validation, use-before-validation, and boundary-request categories will each have category-specific production-boundary positive, negative, and inconclusive cases. Protocol layers will additionally identify independent-model cases, metamorphic relations, meaningful decoded fuzz seeds, causal mutants, and determinism observations that actually exercise the category.

Alternative considered: retain generic layer registrations and add comments listing intended categories. Rejected because that is the mechanism by which unsupported coverage currently passes.

### 6. Make aggregate and adversarial gates causal

The aggregate soundness runner will use one machine-readable subgate manifest containing command vectors, required evidence outputs, and non-zero corpus/category/mutant expectations. The Make target delegates to this runner. A subgate succeeds only when its command executes successfully and its expected evidence is produced and validated for the current tree.

Contract tests will mutate each required command to a no-op, remove each dependency, empty each admitted corpus, forge marker-only evidence, and introduce an unrelated failing guard. Every mutation must make the aggregate contract fail. Merely finding a Make target header, test name, or marker is insufficient.

Alternative considered: make the existing Makefile parser recognize more recipe strings. Rejected because textual recipe matching remains easy to satisfy without executing the claimed proof.

### 7. Exercise integrated production semantics in oracle, fuzz, schedule, and benchmark evidence

Evidence corruption will be injected through a test-only seam before production evidence validation and aggregation; tests will not edit the analyzer result after execution. Differential fuzz programs and their independent interpreter will both model facts, aliases, constraints, call sites, and realizable call/return matching as one integrated state. Historical seed coverage requires decoding a seed and observing the exact declared semantic feature and property failure, not merely nonempty bytes.

The scheduled profile will run a strict superset of the blocking generated corpus, compare every case with the production analyzer, derive its count from the manifest, and be invoked by a documented Make or CI surface. Generated-analysis benchmarks will execute the analyzer harness through parse, type, SSA, propagation, aggregation, and reporting; reference-interpreter benchmarks remain separately labeled component measurements.

Alternative considered: preserve component-only fuzz and benchmark evidence under broader names. Rejected because the current names and completion claims explicitly refer to the analyzer boundary.

### 8. Verify proof freshness against the exact intended tree

A blocking clean-tree evidence verifier will recompute the temporary-index synthetic tree and intended-diff identity without mutating the caller's index. It will reject mismatched tree or diff hashes, missing required gates, stale toolchain or manifest identities, incomplete counterexample inventories, non-causal mutation outcomes, and evidence produced before the final artifact/task state.

The follow-up will record its own evidence and reference the predecessor change as a dependency. Neither change is archived as soundness-complete until the combined exact tree passes the corrected aggregate gate and strict validation.

Alternative considered: rely on the currently matching evidence file and rerun manually before archive. Rejected because freshness is itself a stated blocking requirement and later drift is otherwise invisible.

## Risks / Trade-offs

- **[Risk] Modeling every call increases graph size and runtime.** → Reuse per-call-site summaries, preserve finite facts, add nested-call benchmarks, and fail closed on reviewed resource limits rather than skipping events.
- **[Risk] Analyzing escaping closures increases findings.** → Distinguish local closure obligations from capture-dependent uncertainty and keep stable reason codes so every result is actionable.
- **[Risk] Making inconclusives visible prevents the repository scan from going green immediately.** → Treat that as intended evidence of incomplete proof; improve precision only where justified and do not weaken the policy.
- **[Risk] Category-specific evidence substantially increases test inventory.** → Generate registry/census plumbing while keeping expected semantics independently authored and require exact bidirectional coverage.
- **[Risk] A test-only evidence injection seam could enter production behavior.** → Keep the seam in test harness construction or injected interfaces compiled only through tests; add an architecture guard against production flags or runtime selectors.
- **[Risk] Two active dependent changes complicate archive order.** → Implement and validate this follow-up against the current uncommitted tree, sync/archive `complete-goplint-soundness-hardening` first, then sync/archive this dependent correction without splitting the proven implementation tree.

## Migration Plan

1. Add failing production-boundary counterexamples and adversarial gate self-tests before changing analyzer or assurance behavior.
2. Expand ordered call events, replace deferred validation recognition, and analyze all closure procedures; run focused protocol and counterexample tests after each correction.
3. Make inconclusive categories always visible, remove prohibited baseline rows, and triage every newly exposed repository finding without suppression.
4. Replace category/layer marker registrations and aggregate target parsing with executable evidence and adversarial no-op validation.
5. Integrate the reference model, fuzz dimensions, evidence perturbation, scheduled profile, analyzer benchmarks, and freshness verification.
6. Update documentation and both changes' evidence/task reconciliation, then run nested-module and repository gates.
7. Materialize the exact combined intended diff in a clean synthetic-tree worktree and retain the corrected proof before archive.

Rollback is a source revert of this dependent change. No mode flag, legacy call selector, syntactic deferred-validation fallback, closure omission policy, or suppressible-inconclusive compatibility path will be retained.

## Open Questions

None. Where the current analyzer cannot conservatively model a relevant call, deferred path, closure capture, or evidence relation, the required outcome is blocking inconclusive.
