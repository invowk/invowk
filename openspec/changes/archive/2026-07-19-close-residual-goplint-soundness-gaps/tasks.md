> Apply-run scope override (2026-07-17): execute and verify the existing mutation profile, but do not add or expand mutants, mutation fixtures, profiles, targets, modes, or mutation-flow surface area. For mixed tasks, complete the semantic or assurance work and verify it through the unchanged existing profile; the excluded expansion is documented in retained evidence.

## 1. Lock Dependencies and Reproduce the Review Findings

- [x] 1.1 Record `complete-goplint-soundness-hardening` and `close-goplint-soundness-review-gaps` as required predecessors and block soundness-complete archive while any of their tasks or this ledger remain incomplete.
- [x] 1.2 Create a machine-readable residual-finding inventory mapping every reviewed issue to its production symbol, minimal counterexample, expected outcome, evidence layer, and owning requirement.
- [x] 1.3 Add a production-boundary counterexample for an unvalidated successful constructor return in the `else` branch of `if err != nil` and retain the initial false-negative observation.
- [x] 1.4 Add same-package and cross-package counterexamples where mutation followed by conditional validation is accepted after the caller discards, overwrites, or mismatches the helper error.
- [x] 1.5 Add a package-level stored-function-literal counterexample proving its local cast/use obligation is omitted by the current `FuncDecl`-only route.
- [x] 1.6 Add closure exception and inline-ignore counterexamples proving current pre-analysis suppression can hide protocol inconclusive outcomes.
- [x] 1.7 Add post-validation pointer-channel, aggregate-store, indirect-store, and immutable-value-copy counterexamples with exact violation, inconclusive, and safe expectations.
- [x] 1.8 Add malformed imported-fact counterexamples for wrong function identity, target/source/result slots, condition slots, slot roles, and incompatible types.
- [x] 1.9 Add stable-ID collision and source-layout counterexamples for equal package leaf names and raw token-position drift.
- [x] 1.10 Add directive counterexamples for type/declaration typos, missing values, invalid values, duplicate directives, conflicts, and unsupported attachment locations.
- [x] 1.11 Add adversarial evidence tests for reporting-only stage credit, fixture-order pseudo-metamorphism, label-only fuzzing, unrelated determinism credit, missing regex-selected tests, hard-coded populations, unrelated expected-test failure, and omitted proof paths.
- [x] 1.12 Run every new counterexample against the unchanged implementation and retain the expected red or false-credit observations without weakening assertions.

## 2. Make Constructor Success Classification Edge Sensitive

- [x] 2.1 Replace ancestor-condition return classification with a typed SSA/CFG relation between each return edge, returned object slot, and returned error identity.
- [x] 2.2 Classify a return unsuccessful only when its exact reaching edge proves the exact returned error non-nil.
- [x] 2.3 Preserve obligations for nil, unknown, ambiguous, mismatched, overwritten, or phi-joined error results and map unresolved relations to blocking inconclusive.
- [x] 2.4 Handle `then`/`else`, inverted nil checks, nested conditions, switches, early returns, named results, and multi-result calls through the same result relation.
- [x] 2.5 Make an empty returned-object target set safe only after proving there is no realizable successful non-nil object return.
- [x] 2.6 Remove `constructorReturnHasProvenNonNilError`, its condition-text helper, and every production caller after edge-sensitive classification is active.
- [x] 2.7 Add unit, integration, cross-package, recursion, determinism, race, and benchmark coverage for the new successful-return classification.
- [x] 2.8 Re-run all constructor identity, deferred validation, result-overwrite, and historical counterexamples and prove exact expected outcomes.

## 3. Preserve Conditional Summary Semantics End to End

- [x] 3.1 Refactor summary composition so validation effects retain `Condition`, `ConditionResultSlot`, target/source identity, and effect order instead of applying a synthetic nil result.
- [x] 3.2 Use the canonical conditional edge-transfer API for same-package, imported, recursive, constructor, cast, and post-validation summary application.
- [x] 3.3 Keep mutation-before-validation distinct from validation-before-mutation through export, serialization, import, caching, composition, and caller transfer.
- [x] 3.4 Apply conditional validation only on a continuation proving the exact call result nil; discarded, overwritten, non-nil, unknown, or unrelated results remain invalidated or inconclusive.
- [x] 3.5 Preserve complete ordered effects across recursive and mutually recursive summary fixed points without republishing an optimistic validation state.
- [x] 3.6 Delete or narrow `protocolStateFrom` and any helper that can collapse conditional validation into an unconditional proven state.
- [x] 3.7 Add exact same-package and cross-package tests for checked success, discarded errors, overwritten errors, result aliases, multiple effects, recursion, and incompatible facts.
- [x] 3.8 Add causal mutants for summary condition, condition-result slot, target identity, effect order, and post-validation application, with production-boundary guards for each. Apply-run resolution: the explicit no-expansion override excluded new mutants. Tasks 3.1-3.7 retain the semantic fixes and production-boundary guards; the unchanged profile was regression evidence only. No new summary mutation credit is claimed.
- [x] 3.9 Re-run the two currently surviving post-validation mutants and require structured intended-mismatch kills. Verified by the unchanged blocking profile: `post-validation-summary/drop-constructor-transfer` and `post-validation-unknown/drop-cast-transfer` both produced their declared structured mismatches.

## 4. Analyze Every Procedure Root and Preserve Unsuppressible Uncertainty

- [x] 4.1 Build one deterministic package-wide inventory of function declarations and every function literal, including literals in package initializer expressions.
- [x] 4.2 Associate package-level literals with their real SSA procedure and initializer context without synthesizing AST declarations or source positions.
- [x] 4.3 Route each inventory entry through canonical protocol analysis independent of visible invocation shape, while retaining ordinary call/return edges for known invocations.
- [x] 4.4 Deduplicate nested literals through stable procedure identity so discovery from package and enclosing-function paths produces one analysis and diagnostic per obligation.
- [x] 4.5 Emit stable blocking inconclusive when relevant package-level procedure, initializer, capture, or SSA identity cannot be resolved.
- [x] 4.6 Refactor function and closure reporting to classify protocol outcome before consulting exception, inline-ignore, or baseline policy.
- [x] 4.7 Route every inconclusive through the always-visible sink and limit policy suppression to otherwise suppressible definite findings.
- [x] 4.8 Add an architecture census of every protocol entry point and reject any route containing a pre-classification suppression return.
- [x] 4.9 Add package-initializer, stored-callback, nested closure, duplicate-discovery, capture, exception, inline-ignore, determinism, race, and benchmark coverage.
- [x] 4.10 Re-run all closure and inconclusive-suppression counterexamples and prove local violations or blocking uncertainty remain visible exactly once.

## 5. Close Post-Validation Escape and Imported-Fact Gaps

- [x] 5.1 Extend post-validation non-call transfer to classify pointer/channel sends, aggregate storage, indirect stores, package storage, closure capture, and other supported mutable escapes of the tracked identity.
- [x] 5.2 Preserve validation for immutable value copies only when no mutable alias or address of the original identity escapes.
- [x] 5.3 Reuse one typed identity/effect classifier across cast, use-before-validation, constructor, summary, and boundary analysis instead of category-local escape shortcuts.
- [x] 5.4 Bind `ProtocolSummaryFact` validation to the attached `types.Func` package path, function identity, signature, parameter/result counts, slot roles, and compatible slot types.
- [x] 5.5 Reject missing, out-of-range, malformed, legacy, or incompatible target, source, and condition-result slots before any effect is applied.
- [x] 5.6 Make every relevant dependent obligation blocking inconclusive when a fact is absent or incompatible; never reinterpret the remaining subset as a complete no-effect summary.
- [x] 5.7 Add fact format round-trip, cross-package import, filtered-package export, malformed-object attachment, and unsupported-condition tests.
- [x] 5.8 Add exact pointer escape versus value-copy tests across channels, aggregates, helpers, closures, package state, recursion, and post-validation protected uses.
- [x] 5.9 Add causal mutants for each post-validation non-call escape and fact-compatibility decision and require zero survivors. Apply-run resolution: no per-escape or per-fact mutants were added. Tasks 5.1-5.8 retain the semantic and boundary coverage, and the unchanged profile passed as regression evidence only. No exhaustive escape/fact mutation coverage is claimed.

## 6. Make Finding IDs and Directives Globally Correct

- [x] 6.1 Inventory every finding-ID producer and classify whether it uses full import-path identity, package leaf names, raw positions, message text, or category-local ad hoc parts.
- [x] 6.2 Centralize stable ID construction on full package path plus source-local semantic identity and migrate every structural, protocol, and cross-artifact emitter.
- [x] 6.3 Remove raw `token.Pos`, file-set ordering, diagnostic message, and package leaf name from stable ID inputs.
- [x] 6.4 Add collision tests using two import paths with identical package leaf/type/member/category identities and prove baseline isolation.
- [x] 6.5 Add metamorphic stable-ID tests for unrelated file insertion, deletion, reorder, formatting, and preceding declaration length changes.
- [x] 6.6 Generate a deterministic repeated-scan old-to-new stable-ID migration and collision report before changing the baseline.
- [x] 6.7 Update baseline entries only after every semantic and assurance gate passes, and reject unexplained ID churn or collisions. The accepted migration binds two 66-finding scans, all 65 additions and 24 removals to reviewed exact-set digests, and reports zero collisions or duplicates; the resulting baseline contains 66 `gpl4` entries.
- [x] 6.8 Centralize directive parsing across file, declaration, type, field, function, method, and parameter documentation.
- [x] 6.9 Validate directive name, attachment, required arguments, argument domain, duplication, and conflicts before any directive consumer runs.
- [x] 6.10 Make unknown, misspelled, incomplete, misplaced, duplicate, or conflicting directives deterministic actionable failures.
- [x] 6.11 Add exhaustive directive oracle fixtures and causal mutants for every supported attachment and parameterized directive. The exhaustive directive oracle is implemented; directive-mutant expansion was excluded by the apply-run override. The unchanged manifest contains no directive-specific mutant, so no directive mutation credit is claimed.
- [x] 6.12 Re-run baseline, exception, full-scan, stable-ID determinism, and directive-governance gates until repeated output is byte-identical. The two canonically sorted complete streams are byte-identical at SHA-256 `ff89eb76450e37b1d64d0b073af195b290980b099e1b488d34747b0106f83b70`; baseline, exception, full-scan, determinism, directive, semantic-node-key, and collision checks passed repeated runs.

## 7. Replace False-Credit Category Evidence

- [x] 7.1 Replace non-cast fixture-order tests with substantive per-category metamorphic transformations over programs or independently represented semantic cases.
- [x] 7.2 Define each metamorphic relation's expected invariant or predictable outcome change and make the producer emit observations from the transformed executions.
- [x] 7.3 Replace label-only category fuzz seeds with decoded variable semantic structures covering relevant identities, aliases, branches, procedures, calls, returns, effects, and constraints.
- [x] 7.4 Make each category fuzz target compare with an independent model/relation, add it to scheduled fuzz execution, and retain decoded feature/property observations.
- [x] 7.5 Run every protocol category through the real analyzer under each credited file, package, map, worklist, and equivalent-schedule perturbation.
- [x] 7.6 Stop inheriting category determinism from unrelated global corpora and derive dimensions from category-specific executed cases.
- [x] 7.7 Relabel fixed explicit fixture comparisons as independent boundary-oracle evidence unless they execute a genuinely independent interpreter.
- [x] 7.8 Add or extend independent executable models only where the registry continues to require integrated independent-model credit.
- [x] 7.9 Make evidence observations derive category, feature, boundary, stages, dimensions, properties, and populations from producer-emitted case records.
- [x] 7.10 Make the registry and census reject claimed stages or properties absent from those executed case records.
- [x] 7.11 Add adversarial tests for reordered fixtures, relabeled seeds, disconnected structures, unrelated deterministic programs, forged stage lists, and fixed-fixture model claims.
- [x] 7.12 Re-run the exact category-by-layer census and require every retained credit to identify an executable production or independent boundary.

## 8. Make Mutation and Aggregate Execution Causal

- [x] 8.1 Add category-specific causal mutants for every applicable extraction, identity, graph, propagation, refinement, aggregation, and reporting boundary.
- [x] 8.2 Restrict each mutation observation to the exact stages changed and exercised by that mutant.
- [x] 8.3 Define a structured guard mismatch record binding mutant ID, declared concern, expected semantic observation, actual observation, and failing assertion.
- [x] 8.4 Reject expected-test-name failures caused by setup, unrelated assertions, generic failure, panic, timeout, compilation, environment, or pre-existing control failure.
- [x] 8.5 Retain clean pre-controls, exact source anchors and hashes, compile separation, source restoration, repeatability, and post-controls for every mutant.
- [x] 8.6 Add mutation-runner self-tests for each causal and non-causal outcome, including a failure inside the expected test for the wrong assertion.
- [x] 8.7 Create a reusable subgate census helper that enumerates required tests/cases before execution and records uniquely observed current-run members.
- [x] 8.8 Migrate architecture, refinement, determinism, aggregate-contract, and every other hard-coded-population script to the census helper.
- [x] 8.9 Derive subgate populations from observations and reject missing, duplicate, skipped, zero, stale, or hard-coded members.
- [x] 8.10 Add aggregate adversarial tests that delete or rename each required test while preserving its command and report path.
- [x] 8.11 Run the complete blocking mutation profile and require zero survivors, zero invalid kills, exact structured attribution, and unchanged source digests. The unchanged 27-mutant profile passed with 27 kills, zero survivors, zero invalid outcomes, 33 structured mismatch records, repeat count two, and successful source restoration; see `evidence/targeted-mutation-run.v1.json`.
- [x] 8.12 Run the aggregate core profile and prove every subgate report is current, nonempty, command-bound, and causally populated.

## 9. Reconcile Documentation and Exact-Tree Evidence

- [x] 9.1 Update the cross-change reconciliation so every predecessor claim, residual finding, production correction, requirement, task, and blocking evidence surface has one current authority.
- [x] 9.2 Add a complete-diff census comparing all tracked and untracked changes with the clean-tree path selection.
- [x] 9.3 Require machine-readable reviewed exclusions for unrelated changed paths and fail on every silent omission.
- [x] 9.4 Extend the clean-tree evidence schema and verifier to bind all three task ledgers, artifacts, counterexamples, manifests, observations, mutation attributions, populations, tools, and documentation claims.
- [x] 9.5 Add failure injection for omitted tracked paths, omitted untracked paths, unjustified exclusions, stale task state, stale artifacts, wrong archive order, and partial predecessor records.
- [x] 9.6 Update goplint README, semantic reference, evidence index, `tools/goplint/AGENTS.md`, root agent/rule command docs, Make help, and CI comments to state only the corrected guarantees.
- [x] 9.7 Run `make check-agent-docs` and documentation stale-reference scans after every agent/rule/skill or guarantee-map edit.
- [x] 9.8 Finish the remaining predecessor tasks only from current retained evidence; do not copy prior checkmarks or v1/v2 proof outcomes forward.

## 10. Verify the Combined Corrective Tree

- [x] 10.1 Run formatting and focused unit/integration tests for constructor edges, summary conditions, package procedures, suppression ordering, escape effects, facts, IDs, directives, evidence producers, mutation attribution, and subgate census.
- [x] 10.2 Run `go test -count=1 ./...` and `go test -race -count=1 ./...` from `tools/goplint`, plus reviewed repeat-count tests.
- [x] 10.3 Run deterministic fuzz-seed replay, scheduled fuzz targets, blocking and scheduled independent oracles, category determinism, and the complete causal mutation profile.
- [x] 10.4 Run reference-component and generated-analyzer benchmarks from five fresh reviewed runs and update thresholds only from justified measurements.
- [x] 10.5 Run `make check-baseline`, `make check-goplint-exceptions`, `make check-goplint-full-scan`, and prove no protocol inconclusive or collided identity is hidden or migrated. Two independent repetitions passed after migration; the baseline contains no inconclusive or legacy IDs, the migration reports no collision or duplicate, and both global audits found 0/949 stale patterns across 43 packages.
- [x] 10.6 Run both-module formatting, golangci-lint, config, build, test, Windows build, licensing, file-length, agent-doc, and `git diff --check` gates.
- [x] 10.7 Run strict OpenSpec validation for all three active changes and all canonical specs; fix every requirement, artifact, task, and dependency inconsistency.
- [x] 10.8 Assemble `HEAD` plus the complete reviewed tracked and untracked diff through a temporary index, materialize a clean detached worktree, and run the corrected core aggregate and repository gates there.
- [x] 10.9 Record the v3 combined proof bundle and verify it with the freshness checker while proving the caller's real index and worktree remain byte-for-byte unchanged.
- [x] 10.10 Mark tasks complete only from retained evidence and leave the three changes ready for explicit sync/archive authorization in dependency order.
