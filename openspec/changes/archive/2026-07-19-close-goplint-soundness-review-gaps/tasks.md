## 1. Lock the Review Findings as Failing Evidence

- [x] 1.1 Record `complete-goplint-soundness-hardening` as the required predecessor and prevent either change from being archived as soundness-complete before this task ledger and its exact-tree proof finish.
- [x] 1.2 Add a production-boundary counterexample where `consume(mutate(&value))` follows successful validation and prove the inner mutation cannot be skipped.
- [x] 1.3 Add nested and sibling call-order counterexamples covering multiple effects, summaries, no-return behavior, and unresolved calls in one source expression.
- [x] 1.4 Add constructor counterexamples for conditional deferred validation, overwritten returned errors, multiple defers, and unresolved deferred effects.
- [x] 1.5 Add returned, stored, passed, and callback-style closure counterexamples with local casts plus capture-dependent obligations.
- [x] 1.6 Add baseline tests proving every protocol inconclusive category and outcome remains visible even when a matching category or stable ID is present in baseline data.
- [x] 1.7 Add adversarial assurance tests for generic category credit, marker-only evidence, no-op aggregate recipes, nonempty-but-unrelated fuzz seeds, post-hoc evidence corruption, disconnected solver dimensions, unscheduled profiles, reference-only benchmarks, and stale clean-tree records.
- [x] 1.8 Run the new counterexamples against the unchanged analyzer and retain the expected failing observations without weakening their assertions.

## 2. Model Every Call in Go Evaluation Order

- [x] 2.1 Define the ordered SSA call-event and source-node mapping used by interprocedural graph construction, including stable call-site identity and deterministic ordering.
- [x] 2.2 Expand each CFG node into call and matching-return micro-nodes for every mapped nested and sibling call before reconnecting its continuation.
- [x] 2.3 Apply summaries, mutation, replacement, escape, terminal, and unresolved-effect transfer independently at every expanded call site.
- [x] 2.4 Preserve realizable call/return matching, conditional validation relations, witnesses, and refinement provenance across the expanded micro-node chain.
- [x] 2.5 Emit blocking inconclusive when a relevant source call cannot be mapped to a unique ordered SSA event or conservatively summarized.
- [x] 2.6 Remove `firstCallExprInNode` and every one-call-per-node assumption from production graph construction and post-validation effect analysis.
- [x] 2.7 Add deterministic unit, integration, recursion, no-return, mutation, race, and benchmark coverage for nested and sibling call expansion.
- [x] 2.8 Re-run the call-order counterexamples and prove each now reports the required violation or stable inconclusive outcome.

## 3. Make Deferred Constructor Validation Path-Sensitive

- [x] 3.1 Represent deferred calls and closures as ordered effects applied in LIFO order at each realizable constructor return.
- [x] 3.2 Reuse canonical procedure summaries and conditional error-result relations to analyze deferred validation rather than scanning assignment syntax.
- [x] 3.3 Establish deferred validation only when every successful return path executes the matching validation and returns that invocation's nil error result.
- [x] 3.4 Preserve unvalidated or inconclusive state across conditional skips, early exits, panics, ambiguous captures, unresolved calls, aliases, and result overwrites.
- [x] 3.5 Remove `containsDeferredConstructorValidation` and any unconditional defer-node validation transfer after the canonical return-path implementation is active.
- [x] 3.6 Add positive tests for unconditional deferred validation and negative/inconclusive tests for conditional, overwritten, multi-defer, helper, alias, and unresolved cases.
- [x] 3.7 Re-run the deferred-validation counterexamples and prove no successful unvalidated return is classified as discharged.

## 4. Analyze Escaping Closure Procedures

- [x] 4.1 Register every function literal body as an analyzable procedure independent of visible invocation shape in the enclosing function.
- [x] 4.2 Analyze local casts, validation, protected uses, and constructor obligations inside returned, stored, passed, IIFE, `go`, and `defer` closures.
- [x] 4.3 Connect known closure invocation sites through ordinary realizable call/return edges without duplicating local diagnostics.
- [x] 4.4 Resolve captured identities through SSA closure bindings and emit stable blocking inconclusive for relevant ambiguous capture or external effects.
- [x] 4.5 Remove the production assumption and fixtures that classify detached function literals as non-executable solely because no same-body call is visible.
- [x] 4.6 Add cross-package, nested-closure, recursive-closure, method-value, capture-rebinding, returned-callback, stored-callback, race, and determinism tests.
- [x] 4.7 Re-run every escaping-closure counterexample and prove local violations are reported and capture-dependent uncertainty fails closed.

## 5. Make Protocol Inconclusives Unsuppressible

- [x] 5.1 Change every protocol inconclusive category to the always-visible baseline policy and add a catalog invariant rejecting suppressible protocol uncertainty.
- [x] 5.2 Route inconclusive reporting through an always-visible diagnostic path that cannot consult baseline or exception suppression.
- [x] 5.3 Make baseline parsing fail on protocol inconclusive sections, entries, or outcome metadata even when stable IDs and messages are otherwise valid.
- [x] 5.4 Make baseline update refuse to serialize protocol inconclusives and leave the command unsuccessful while any are present.
- [x] 5.5 Remove every protocol inconclusive entry from `tools/goplint/baseline.toml` and resolve each exposed repository finding through sound analyzer precision or code correction without moving it to another suppression surface.
- [x] 5.6 Prove `check-baseline`, `check-goplint-full-scan`, pre-commit, CI, and the aggregate soundness gate fail on an injected or real inconclusive regardless of baseline contents.
- [x] 5.7 Update baseline documentation and agent guidance to state that policy findings may be baselined but proof uncertainty is always visible and blocking.

## 6. Make Category Coverage Specific and Executable

- [x] 6.1 Replace semantic-kind predicates and artifact markers with typed evidence registrations keyed by exact category, layer, semantic feature, executable command or test, and expected observation.
- [x] 6.2 Make the semantic census consume observations emitted by executed evidence and reject missing, duplicate, extra, stale, marker-only, or zero-population registrations bidirectionally.
- [x] 6.3 Add category-specific production-boundary must-report, must-not-report, and must-be-inconclusive fixtures for cast validation, constructor validation, use-before-validation, and boundary requests.
- [x] 6.4 Add category-specific independent-model cases and metamorphic relations that do not reuse production transfer, join, summary, or reporting helpers.
- [x] 6.5 Add decoded fuzz seeds whose observed structures and independent properties correspond exactly to each historical counterexample and registered protocol category.
- [x] 6.6 Add category-specific causal mutants that exercise production extraction, propagation, refinement where applicable, aggregation, and reporting.
- [x] 6.7 Add real-analyzer determinism observations per protocol category across file, package, worklist, map, and equivalent scheduling reorderings.
- [x] 6.8 Replace lexical production-symbol reachability with routing evidence proving each declared transfer, uncertainty reason, and category owner executes through a real analyzer path.

## 7. Make Aggregate Gate Execution Causal

- [x] 7.1 Define one reviewed machine-readable manifest for every aggregate soundness subgate, its exact command vector, required evidence outputs, and minimum nonzero populations.
- [x] 7.2 Implement an aggregate runner that executes the manifest commands, validates current-tree evidence, and rejects missing, skipped, stale, empty, or duplicate results.
- [x] 7.3 Make `check-goplint-soundness` delegate to the aggregate runner and remove target-header or marker-string presence as completion proof.
- [x] 7.4 Add self-tests that replace each required command with a successful no-op and prove the gate contract rejects every replacement.
- [x] 7.5 Add self-tests for removed dependencies, empty corpora and category sets, forged markers, stale outputs, skipped integrations, and commands that exit successfully without observations.
- [x] 7.6 Preserve causal mutation rules: clean control success, exact mutant anchor and transformation, declared guard selection, structured intended mismatch, restoration, and repeatability.
- [x] 7.7 Reject compilation failure, timeout, crash, unrelated test failure, generic `FAIL` text, or pre-existing control failure as mutation or adversarial proof.

## 8. Integrate Oracle, Fuzz, Scheduled, and Benchmark Evidence

- [x] 8.1 Introduce a test-only injection seam that corrupts witness, refinement, summary, or reason evidence before production validation and aggregation.
- [x] 8.2 Replace the post-execution `got.Reason` edit with perturbations that prove production rejects or reports corrupted evidence through the real analyzer boundary.
- [x] 8.3 Extend the independent program model and interpreter so facts, aliases, constraints, procedures, call sites, and realizable call/return matching jointly affect outcomes.
- [x] 8.4 Extend production differential execution to consume the same declared integrated dimensions without calling the independent interpreter for expected values.
- [x] 8.5 Replace nonempty-seed and generic-structure coverage predicates with exact decoded feature and property observations for every audit-matrix mapping.
- [x] 8.6 Make the scheduled oracle enumerate a manifest-derived strict superset, compare every case with the production analyzer, and self-check its derived count.
- [x] 8.7 Wire the scheduled oracle to a documented Make target and scheduled CI workflow with blocking failure semantics.
- [x] 8.8 Split reference-interpreter component benchmarks from generated-analyzer benchmarks and make the analyzer benchmark cover parse, type, SSA, graph, propagation, aggregation, and reporting.
- [x] 8.9 Update performance manifests and thresholds from five fresh reviewed runs without weakening fail-closed resource behavior.
- [x] 8.10 Run focused oracle, metamorphic, fuzz-seed, mutation, determinism, scheduled-profile, race, repeat, and benchmark tests and retain their machine-readable observations.

## 9. Verify Exact-Tree Evidence and Documentation

- [x] 9.1 Implement `check-goplint-clean-tree-evidence` to recompute the temporary-index synthetic tree and intended-diff identity without modifying the caller's real index.
- [x] 9.2 Validate base commit, synthetic tree, diff hash, Go and tool versions, manifest identities, task-state identity, counterexample inventory, commands, observations, and per-gate outcomes.
- [x] 9.3 Reject evidence generated before any intended content, artifact, task checkbox, manifest, baseline, or required gate changed.
- [x] 9.4 Add failure-injection tests for tree drift, untracked-file drift, missing results, stale manifests, wrong toolchain, zero evidence populations, non-causal mutants, and incomplete task state.
- [x] 9.5 Prove the verifier leaves the caller's index and worktree byte-for-byte unchanged on success and every failure path.
- [x] 9.6 Add the freshness verifier to the aggregate gate, pre-completion workflow, and retained evidence index without creating a recursive dependency during evidence generation.
- [x] 9.7 Update goplint README, `tools/goplint/AGENTS.md`, root agent/rule command docs, Make help, CI comments, and evidence mappings to state only production-backed guarantees.
- [x] 9.8 Reconcile every contradicted completion claim in `complete-goplint-soundness-hardening` with this follow-up's counterexample, implementation, and blocking evidence task.

## 10. Complete the Combined Soundness Proof

- [x] 10.1 Run formatting and focused tests for ordered calls, deferred validation, closures, unsuppressible inconclusives, category observations, aggregate self-tests, integrated oracle/fuzzing, and evidence freshness.
- [x] 10.2 Run all `tools/goplint` tests, race tests, repeat tests, fuzz-seed checks, causal mutation profile, determinism checks, scheduled oracle, and analyzer benchmarks.
- [x] 10.3 Run `make check-baseline`, `make check-goplint-exceptions`, `make check-goplint-full-scan`, and prove no protocol inconclusive is hidden or migrated.
- [x] 10.4 Run both-module formatting, lint, test, Windows build, licensing, file-length, agent-doc, and `git diff --check` gates required by repository governance.
- [x] 10.5 Run strict validation for both active changes and all canonical OpenSpec content, and fix every artifact or requirement inconsistency.
- [x] 10.6 Materialize `HEAD` plus the complete intended diff in a clean detached worktree through a temporary index and run the corrected `make check-goplint-soundness` aggregate there.
- [x] 10.7 Record the exact combined proof bundle, verify it with `check-goplint-clean-tree-evidence`, and confirm the caller's real index and worktree were not mutated.
- [x] 10.8 Mark tasks complete only from retained evidence, sync/archive `complete-goplint-soundness-hardening` first, then sync/archive this dependent change after final strict validation.
