## 1. Freeze Audit Findings as Failing End-to-End Contracts

- [x] 1.1 Create a machine-readable audit matrix mapping every finding to its violated requirement, production symbols, minimal counterexample, expected outcome, implementation task, and blocking gate.
- [x] 1.2 Add real-analyzer fixtures proving a non-nil validation branch that successfully returns or reaches a protected use remains a violation while only the matching nil branch validates.
- [x] 1.3 Add constructor fixtures for historical alias rebinding (`ret = a; ret = b`), validation of a different same-typed object, and validation followed by a same-typed literal return.
- [x] 1.4 Add real-analyzer fixtures proving unresolved mutation, replacement, escape, concurrent access, and escaped-heap effects after validation produce the appropriate blocking inconclusive outcomes when relevant.
- [x] 1.5 Add join/refinement fixtures where an infeasible validated path and feasible unvalidated path contribute to one node, including caller/callee block-index collisions and multiple incomparable unsafe witnesses.
- [x] 1.6 Add analyzer-level missing-SSA and missing-function/closure fixtures that prove every dependent protocol category is inconclusive without an AST, empty-alias, or type-only fallback.
- [x] 1.7 Add ambiguous copy, phi, store, interface, dynamic-index, closure, and pointer-flow fixtures that distinguish must-alias validation from relevant may-alias inconclusive outcomes.
- [x] 1.8 Add recursive and mutually recursive real-analyzer fixtures that assert final outcomes and finite summary reuse rather than graph-edge existence or depth-budget termination.
- [x] 1.9 Add conditionally rebound no-return alias fixtures proving only the reaching call-site SSA value can prune a continuation.
- [x] 1.10 Add method-bearing primitive type-parameter fixtures and unsupported mixed-constraint fixtures proving obligations are preserved or become inconclusive rather than disappearing.
- [x] 1.11 Add an injectable-deadline fixture proving an UNSAT computation or evidence check completed after timeout cannot discharge a witness.
- [x] 1.12 Run the new counterexample and meta-contract tests against the current implementation, confirm each intended gap fails for the expected reason, and record the red baseline without weakening expectations.

## 2. Make the Conditional Protocol Domain Authoritative

- [x] 2.1 Inventory every production validation, hazard, uncertainty, and identity state and delete or plan migration for any state machine parallel to the declared protocol domain.
- [x] 2.2 Replace checked-validation call-position maps with a canonical invocation relation containing receiver identity, error-result identity, call site, summary provenance, and successor-specific result facts.
- [x] 2.3 Attach validation transfers only to CFG/supergraph edges proving the matching result nil; keep the call node, non-nil edge, and unknown edge non-validating.
- [x] 2.4 Route direct selector calls, interface-resolved calls, method values, helper summaries, constructors, and cross-package facts through the same conditional-effect representation.
- [x] 2.5 Make protocol aggregation propagate validation, hazards, and uncertainty as one monotone product and ensure validation never clears pre-existing uncertainty.
- [x] 2.6 Migrate cast validation, use-before-validation, constructor validation, boundary request validation, closure, and helper adapters to the canonical transfer API.
- [x] 2.7 Add executable production-integration assertions that every declared domain transfer and uncertainty reason is reachable from a real analyzer path and no simplified production domain remains.
- [x] 2.8 Run the conditional-validation counterexamples, protocol law tests, adapter integration tests, and full analyzer suite until nil/non-nil edge behavior is exact and deterministic.

## 3. Replace Type and Historical Alias Matching with SSA/Object Identity

- [x] 3.1 Define one interned identity key for SSA values, allocations, parameters, receivers, result slots, static field addresses, and proven copies, including package/procedure qualification.
- [x] 3.2 Implement instruction-ordered must-alias environments with explicit rebinding, allocation, store, phi, interface, closure, and escape kill rules.
- [x] 3.3 Replace constructor return-target type fallback and undirected whole-function assignment closure with identity-at-return-slot queries.
- [x] 3.4 Apply validation and summary effects only across must-alias identities valid at the effect's program point; never infer identity from equal Go types alone.
- [x] 3.5 Convert relevant unresolved may-alias relationships to stable blocking uncertainty and prove irrelevant alias ambiguity is excluded by the obligation slice.
- [x] 3.6 Introduce a typed on-demand SSA result that distinguishes build failure, missing function, missing closure, unsupported instruction, and incomplete dependency information.
- [x] 3.7 Require every protocol adapter to consume the typed SSA result and emit scoped inconclusive outcomes instead of continuing with nil SSA or empty enrichment.
- [x] 3.8 Normalize complete generic type sets and method sets so method-bearing primitive constraints create obligations and unsupported/mixed relevant terms become inconclusive.
- [x] 3.9 Add unit laws for alias bind/kill/join and end-to-end tests for rebinding, allocations, result slots, fields, phi nodes, interfaces, closures, and generics.

## 4. Model Unknown and Post-Validation Effects Conservatively

- [x] 4.1 Define a relevance query over forward reachability, sink reachability, tracked identity, and possible validate/mutate/replace/escape/consume/terminate/constraint effects.
- [x] 4.2 Remove the solver condition that records unresolved-call uncertainty only while validation is required; allow relevant unknown effects to add uncertainty in every validation state.
- [x] 4.3 Add explicit summary effects for known pure, preserving, conditional-validation, mutation, replacement, escape, consume, and terminal calls.
- [x] 4.4 Treat unresolved external calls, interface dispatch, reflection, `unsafe`, concurrent mutation, and escaped-heap mutation as inconclusive whenever their conservative effect can change the obligation.
- [x] 4.5 Prove through negative-slice tests that unknown behavior on unrelated identities or unreachable sink paths does not poison an obligation.
- [x] 4.6 Add monotonicity and aggregation tests proving a later validation cannot erase unresolved mutation and a feasible violation still outranks unrelated uncertainty.

## 5. Implement Finite-Summary Realizable-Path Tabulation

- [x] 5.1 Replace concrete call-stack snapshot keys with finite path-edge keys over procedure, node, entry fact, current fact, and conditional edge function.
- [x] 5.2 Implement deterministic procedure summaries from entry facts to exit facts/effects and dependency-driven reuse at every matching call site.
- [x] 5.3 Propagate returns only through the call site that established the callee dependency, including multiple callers and recursive contexts.
- [x] 5.4 Iterate recursive and mutually recursive summary dependencies to a monotone fixed point without using call-depth exhaustion as successful convergence.
- [x] 5.5 Preserve conditional validation, hazards, uncertainty, mutation, terminal, and identity effects across call, return, summary, and call-to-return edges.
- [x] 5.6 Map state, worklist, summary, and evidence exhaustion to stable inconclusive outcomes without emitting an optimistic partial result.
- [x] 5.7 Rework no-return alias resolution to use reaching SSA function values at each call site and retain continuations for conditionally rebound or ambiguous aliases.
- [x] 5.8 Add solver-level differential tests and real-analyzer recursion/matched-return/no-return tests that assert convergence, summary counts, and normalized outcomes.
- [x] 5.9 Benchmark tabulation state count, summary reuse, time, bytes, and allocations against reviewed thresholds without adding a weaker runtime path.

## 6. Pair Joined State Contributions with Exact Refinement Witnesses

- [x] 6.1 Define fully qualified witness edges containing procedure, source/destination nodes, edge kind, call site, tracked fact, state contribution, and SSA predicate provenance.
- [x] 6.2 Replace one representative path per joined snapshot with a deterministic antichain of witness IDs for incomparable state/fact contributions.
- [x] 6.3 Define and test sound witness subsumption that can discard only an equivalent or less-conservative contribution; map antichain bound exhaustion to inconclusive.
- [x] 6.4 Make refinement replay reject missing nodes, non-edges, cross-procedure index confusion, unmatched calls/returns, changed facts, and any silently skipped transition.
- [x] 6.5 Bind discharged-witness hashes to the exact witness, fact, contributed state, normalized constraints, and checked contradiction evidence.
- [x] 6.6 Ensure checked UNSAT removes only its exact contribution and re-queues or aggregates every other feasible, unresolved, or incomparable contribution.
- [x] 6.7 Add cooperative context/budget checks inside extraction, solving, evidence checking, refinement, and tabulation loops and immediately before accepting SAT/UNSAT or discharge.
- [x] 6.8 Use injectable clocks or deterministic operation budgets to test timeout boundaries without sleeps, including late UNSAT, late evidence acceptance, and cancellation during iteration.
- [x] 6.9 Run join, interprocedural-collision, multiple-witness, evidence-corruption, timeout, query, iteration, and state-budget counterexamples through the real analyzer.

## 7. Remove Alternate Production Semantics and Legacy Facts

- [x] 7.1 Inventory compiled production references to legacy CFG modes, nil traversal contexts, legacy DFS helpers, AST/type-only fallbacks, compatibility adapters, and unconditional validation facts.
- [x] 7.2 Delete `cfgTraversalModeLegacy`, `cfgTraversalModeUBVOrder`, legacy wildcard/nil-context behavior, and every production caller of the legacy DFS evaluator after canonical migration.
- [x] 7.3 Delete unconditional `ValidatesTypeFact` export/import/registration and replace it with one versioned receiver/parameter/result-slot and error-condition-sensitive fact.
- [x] 7.4 Make missing, malformed, legacy, or incompatible relevant facts blocking inconclusive and update filtered-package fact export tests.
- [x] 7.5 Delete remaining alternate evaluator, fallback, compatibility, hidden selector, and mode-derived stable-ID code from non-test builds.
- [x] 7.6 Add an AST/type-aware architecture gate over production sources and routing tables, backed by a narrow forbidden-symbol/stale-reference scan.
- [x] 7.7 Add tests proving removed symbols cannot be selected indirectly and test-only models instantiate canonical components without production mode switches.
- [x] 7.8 Run repository-wide code, fixture, script, workflow, documentation, and generated-file searches and remove every stale support claim or semantic dependency.

## 8. Make the Semantic Catalog and Boundary Oracles Total

- [x] 8.1 Make the registered category table the single source of category identity, semantic kind, live routing owner, and required oracle layers.
- [x] 8.2 Replace administrative non-nil function-value ownership with typed owner registrations used by the analyzer routing table.
- [x] 8.3 Enforce a bijection among registered categories, rule contracts, owner registrations, oracle-matrix entries, and semantic-kind-specific evidence requirements.
- [x] 8.4 Add real-analyzer boundary fixtures for every structural and cross-artifact category, with explicit neighboring must-report and must-not-report cases.
- [x] 8.5 Add complete must-report, must-not-report, and must-be-inconclusive entries for every protocol category, including `unvalidated-boundary-request`.
- [x] 8.6 Add catalog mutation/meta-tests proving a missing rule, missing oracle, extra oracle, duplicate owner, wrong semantic kind, stale owner, or missing required layer fails the gate.
- [x] 8.7 Emit a deterministic coverage census by category and layer and require zero uncovered or administratively owned categories.

## 9. Rebuild Bounded and End-to-End Independent Oracles

- [x] 9.1 Extend the versioned bounds schema to describe actual enumerable procedures, nodes, identities, call sites, call depth, topologies, branch joins, recursion, operations, conditional results, aliases/kills, unknown effects, and supported constraints.
- [x] 9.2 Refactor generation so every declared dimension controls enumeration or an explicit well-formedness restriction; remove hard-coded topology and manually asserted feature coverage.
- [x] 9.3 Derive and verify exact admitted-program cardinality plus a generated feature census, and add sensitivity tests proving each manifest dimension changes the corpus as declared.
- [x] 9.4 Extend the independent reference interpreter for the reviewed conditional-effect, identity, uncertainty, summary, recursion, mutation, and refinement fragment without importing production helpers.
- [x] 9.5 Add an end-to-end generated-Go harness that exercises parsing, type checking, SSA extraction, graph construction, production propagation, refinement, aggregation, and diagnostics before comparison with the independent result.
- [x] 9.6 Retain solver-core differential tests as explicitly labeled component evidence and prevent them from satisfying the end-to-end coverage requirement alone.
- [x] 9.7 Partition the blocking bounded corpus deterministically, enforce reviewed minimum bounds on every change, and provide larger scheduled/manual bounds without weakening the blocking set.
- [x] 9.8 Add independent review tests that intentionally perturb extraction, edge conditioning, identity, matching returns, joins, uncertainty, and evidence to prove the end-to-end oracle detects each class.

## 10. Strengthen Determinism, Fuzz, and Mutation Evidence

- [x] 10.1 Build a determinism harness that runs the real analyzer repeatedly with reordered packages, files, worklist insertion, map construction, and equivalent parallel scheduling.
- [x] 10.2 Compare normalized findings, facts, reasons, fully qualified witnesses, summary evidence, and refinement evidence byte-for-byte and remove helper-only sorting as proof of analyzer determinism.
- [x] 10.3 Redesign graph and propagation fuzz decoders to generate variable well-formed topologies, identities, aliases, branches, calls, returns, recursion, effects, and constraints within hard safety bounds.
- [x] 10.4 Give every fuzz target an independent reference, algebraic, metamorphic, round-trip, or preservation oracle; prohibit production-against-itself determinism as the sole correctness property.
- [x] 10.5 Add committed seeds for every historical counterexample and boundary class, minimize reproductions, and keep deterministic seed execution blocking.
- [x] 10.6 Add an unmutated preflight for every targeted mutation profile and require all declared guard tests to pass before any mutant is evaluated.
- [x] 10.7 Require exact mutant anchor/transformation verification and structured mutant-to-guard failure attribution; classify compile errors, crashes, timeouts, unrelated failures, and generic failure text as non-kills.
- [x] 10.8 Add a post-mutation control run that proves restoration and repeatability, and reject manifests whose guards are non-causal or already failing.
- [x] 10.9 Expand the targeted manifest to cover production-domain wiring, nil-edge conditioning, identity kills, post-validation unknown effects, witness/state pairing, matched summaries, recursion, no-return aliases, generic constraints, missing SSA, timeout, catalog completeness, bounds sensitivity, and determinism.
- [x] 10.10 Run deterministic seeds, bounded generation, real determinism, and the complete causal targeted mutation profile repeatedly and require stable zero-survivor results.

## 11. Integrate Blocking Gates, Performance, Baseline, and Documentation

- [x] 11.1 Refactor `make check-goplint-soundness` so production integration, counterexamples, architecture absence, catalog completeness, end-to-end oracle, real determinism, fuzz seeds, causal mutation, race/repeat, full scan, and benchmarks are explicit blocking subchecks.
- [x] 11.2 Add gate self-tests proving omission, empty selection, zero generated programs, zero categories, skipped analyzer integration, non-causal mutation, or missing evidence cannot produce success.
- [x] 11.3 Update CI and pre-commit triggers so every relevant production, test, manifest, catalog, script, workflow, and documentation change runs the appropriate blocking assurance surfaces.
- [x] 11.4 Recalibrate reviewed solver, identity, summary, witness, refinement, generated-analysis, determinism, and full-scan time/bytes/allocation thresholds using median-of-five fresh runs and recorded toolchain/runner metadata.
- [x] 11.5 Generate and review stable finding-ID and fact-format migration reports, then update the baseline only after all semantic and architecture gates pass.
- [x] 11.6 Update goplint README, semantic reference, evidence index, agent rules/skills, Make help, workflow comments, schemas, and examples to map each guarantee to a production symbol and blocking proof surface.
- [x] 11.7 Remove or qualify claims supported only by model/unit helpers, and state the property-relative boundary and fail-closed unsupported behavior consistently.
- [x] 11.8 Run `make check-agent-docs`, schema/catalog validators, documentation stale-reference scans, and all lint/config/formatter checks required by touched surfaces.

## 12. Complete Cross-Change and Clean-Tree Verification

- [x] 12.1 Reconcile this change with every requirement and completed task in `harden-goplint-soundness`, recording which prior claims were corrected, replaced, or proven end to end.
- [x] 12.2 Run all focused historical counterexamples and require the exact violation, inconclusive, no-finding, identity, witness, and refinement evidence expected by the audit matrix.
- [x] 12.3 Run `go test -count=1 ./...`, `go test -race -count=1 ./...`, and reviewed repeat-count tests from the `tools/goplint` module.
- [x] 12.4 Run both-module golangci-lint, formatter, config, build, test, Windows-build, licensing, file-length, baseline, exception, full-scan, and benchmark gates required by repository rules.
- [x] 12.5 Run catalog completeness, bounded end-to-end reference comparison, deterministic fuzz seeds, real package/worklist determinism, complete causal targeted mutation, and aggregate `make check-goplint-soundness` gates.
- [x] 12.6 Generate a synthetic tree from exactly `HEAD` plus the intended combined diff using a temporary index, materialize a clean detached worktree, and rerun every required gate without development-worktree artifacts.
- [x] 12.7 Record the base commit, synthetic tree, diff identity, toolchain, commands, outcomes, category/layer census, generated cardinality/features, mutation attribution, counterexample matrix, performance results, and baseline/ID review under this change's evidence directory.
- [x] 12.8 Run `git diff --check`, strict OpenSpec validation for `complete-goplint-soundness-hardening` and all canonical specs, and verify the archived `2026-07-14-harden-goplint-soundness` artifacts remain present and reconciled with their synchronized canonical specs; fix every inconsistency before marking the active change ready to archive.
- [x] 12.9 Review the final production tree for forbidden legacy authorities and review every documentation/evidence claim against live production routing and gate coverage.
- [x] 12.10 Preserve the user-authorized `2026-07-14-harden-goplint-soundness` archive, keep `complete-goplint-soundness-hardening` unarchived until the stricter assurance contract and all recorded clean-tree evidence pass, then synchronize and archive only the active corrective change.
