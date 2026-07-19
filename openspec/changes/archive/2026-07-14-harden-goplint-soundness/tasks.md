## 1. Freeze the Soundness Contract and Counterexamples

- [x] 1.1 Inventory every registered goplint category, classify it as structural, protocol, or cross-artifact, and record its live implementation owner and independent oracle strategy in the semantic catalog.
- [x] 1.2 Replace free-form implementation-entrypoint strings with resolvable registered owner keys and make catalog validation reject missing, duplicate, stale, or unowned categories.
- [x] 1.3 Define the canonical protocol domain, partial order, joins, validation/error relations, escape states, uncertainty reasons, fail-closed obligation aggregation, relevance slice, alias boundary, and result vocabulary in code and synchronized semantic documentation.
- [x] 1.4 Add executable law tests for domain ordering, idempotent/commutative/associative joins, transfer monotonicity, alias kills, conditional effects, and unknown-to-inconclusive behavior.
- [x] 1.5 Add must-report fixtures for continued execution after validation failure, assigned/logged/blank validation errors, discarded method-value errors, wrong same-typed constructor targets, alias rebinding, generic constructors/type parameters, unmatched returns, and mutable-variable feasibility.
- [x] 1.6 Add must-not-report fixtures for terminating validation-error branches, nil-only continuations, value-plus-validation-error constructor returns, sound aliases, recursive summaries, and known non-returning calls including aliases.
- [x] 1.7 Add must-be-inconclusive fixtures for unsupported predicates, unresolved relevant calls, ambiguous may-alias identities, incompatible cross-package facts, missing SSA, and exhausted resource budgets.

## 2. Introduce Canonical SSA Identities and Validation Effects

- [x] 2.1 Implement interned SSA value and abstract object identities for allocations, parameters, receivers, results, copies, and phi nodes without type-only fallback matching.
- [x] 2.2 Implement flow-sensitive must-alias propagation with explicit kill behavior for stores, rebinding, new allocations, and path joins; classify relevant may-alias ambiguity as inconclusive.
- [x] 2.3 Represent each `Validate()` invocation as a receiver/result relation and transition the receiver to validated only on a control-flow edge proving the associated error nil.
- [x] 2.4 Model direct selector calls, interface-resolved calls, captured method values, explicit error variables, inverted nil checks, and dominating terminating failure branches through the same conditional-effect API.
- [x] 2.5 Change helper and method summaries to package-qualified, format-versioned, receiver/parameter/result-slot-sensitive conditional effects instead of unconditional validates-type facts.
- [x] 2.6 Change constructor tracking to prove that the conditionally validated object reaches the corresponding return slot, including `return value, value.Validate()` semantics.
- [x] 2.7 Ensure filtered packages export required conditional facts and make incompatible or missing relevant fact versions produce deterministic inconclusive reasons.

## 3. Normalize Generics, Methods, and Raw Sources

- [x] 3.1 Normalize generic `IndexExpr` and `IndexListExpr` calls through instantiated `go/types.Signature` data for constructor names, result slots, and trailing-error checks.
- [x] 3.2 Recognize supported primitive terms in type-parameter type sets when identifying raw-to-validatable conversions, while treating unsupported or mixed constraints conservatively.
- [x] 3.3 Centralize Go method-set resolution for direct calls, interface calls, pointer/value receivers, and method values, and exclude lowercase or unrelated same-signature methods from the Validate protocol.
- [x] 3.4 Add cross-package and filtered-package integration tests combining generics, pointer/value method sets, conditional summaries, and identity-preserving return slots.

## 4. Replace the Interprocedural Core

- [x] 4.1 Build a deterministic exploded supergraph over procedures, nodes, call sites, return sites, and finite protocol facts with explicit call, return, call-to-return, and summary edges.
- [x] 4.2 Implement worklist tabulation and summary reuse so returns reach only matching caller continuations and recursion converges through finite facts rather than DFS cycle guesses.
- [x] 4.3 Implement IDE-style conditional edge functions for validation/error relations and IFDS-style propagation for object typestate and UBV escape facts.
- [x] 4.4 Require every call-to-return edge to model all relevant effects; unresolved relevant effects must become inconclusive and must not use an unconditional bypass.
- [x] 4.5 Centralize the `mayReturn` contract using compiler/CFG behavior, modeled intrinsics, and analyzed bodies; cover `panic`, `os.Exit`, `log.Fatal`, `log.Fatalf`, `log.Fatalln`, and soundly resolved aliases.
- [x] 4.6 Make unknown calls conservatively returning and emit a stable inconclusive reason whenever their unresolved effect can change a tracked obligation.
- [x] 4.7 Migrate cast-validation, UBV escape, constructor-validation, closure, and helper-summary adapters to the canonical solver and delete adapter fallbacks to alternate evaluators.
- [x] 4.8 Re-run the no-return regression through the real analyzer and require zero continuation-only findings while retaining violations on genuinely returning paths.

## 5. Replace Phase C with Checked SSA Refinement

- [x] 5.1 Define and document the supported SSA constraint fragment for SSA-versioned nil, boolean, string, and integer equality/inequality atoms plus normalized negation/conjunction/disjunction, with pointer/interface restrictions and all unsupported syntax mapping to unknown.
- [x] 5.2 Extract path predicates using SSA value versions so source-level reassignment cannot merge distinct constraint subjects.
- [x] 5.3 Implement the exact `ssa-constraints` decision procedure and normalized UNSAT contradiction evidence without retaining the misleading SMT engine name.
- [x] 5.4 Implement a separate evidence checker and forbid witness discharge when evidence is missing, malformed, unsupported, timed out, or rejected.
- [x] 5.5 Implement symbolic witness replay and iterative predicate refinement that terminates in retained SAT, checked UNSAT discharge, or blocking inconclusive at a resource limit.
- [x] 5.6 Replace `proven-safe` and old Phase C metadata with deterministic `violation`, `inconclusive`, and `discharged-infeasible` evidence, including SSA subject and refinement provenance.
- [x] 5.7 Add reassignment, branch inversion, unsupported-predicate, timeout, query-limit, iteration-limit, and evidence-corruption tests proving no unknown or unfinished result becomes safe.

## 6. Build Independent and Adversarial Oracles

- [x] 6.1 Implement a test-only normalized protocol IR and reference interpreter that does not import or call production transfer, join, summary, feasibility, or solver functions.
- [x] 6.2 Add a versioned bounds manifest covering at least two procedures, four nodes per procedure, two identities, two call sites, call depth two, a branch join, and recursion; exhaustively generate every admitted well-formed graph and compare canonical solver outcomes with the independent interpreter.
- [x] 6.3 Add metamorphic test generation for alpha-renaming, nil-check equivalence, branch inversion, selector/method-value equivalence, harmless statement insertion, alias copies, alias rebindings, and changed error continuations.
- [x] 6.4 Add Go fuzz targets with committed seed corpora for supergraph construction, tabulation, fact serialization, constraint normalization/evidence checking, semantic-catalog decoding, and finding determinism.
- [x] 6.5 Add deterministic CI execution for fuzz seed corpora and bounded generated graphs, while wiring longer fuzz runs into the repository's appropriate scheduled/manual validation path.
- [x] 6.6 Add a versioned targeted mutation manifest and profiles for nil-branch success, receiver identity, alias kills, matched returns, terminal pruning, generic normalization, fact versioning, and unknown-to-safe guards.
- [x] 6.7 Require every targeted soundness mutant to be killed and update mutation documentation/baselines without weakening survivor policy.
- [x] 6.8 Add repeated-run and reordered-worklist/package tests that compare normalized findings, reasons, witnesses, facts, and refinement evidence byte-for-byte.

## 7. Remove Legacy Modes and Production Paths

- [x] 7.1 Delete `--ubv-mode` and order-only UBV production branches, retaining escape semantics directly in the canonical solver.
- [x] 7.2 Delete `--cfg-backend` and the AST protocol-analysis backend; make unavailable SSA a blocking inconclusive outcome rather than a fallback.
- [x] 7.3 Delete `--cfg-interproc-engine`, legacy evaluators, compare trackers/classifiers, compatibility code, and every hidden or test-only runtime engine selector.
- [x] 7.4 Delete `--cfg-alias-mode` and alias-off paths after canonical SSA/object identity tracking covers all protocol adapters.
- [x] 7.5 Delete `--cfg-feasibility-engine`, `--cfg-refinement-mode`, the off and one-shot branches, and the old unversioned predicate checker after checked iterative refinement is canonical.
- [x] 7.6 Delete `--cfg-inconclusive-policy` and warning/off paths for protocol outcomes so inconclusive is unconditionally blocking.
- [x] 7.7 Retain only resource-budget flags whose exhaustion maps to inconclusive, rename Phase C-specific limits where necessary, and test that no setting disables a semantic layer.
- [x] 7.8 Add CLI tests proving every removed flag fails as unknown and repository-wide scans proving no deprecated constant, flag, hidden alias, fallback branch, or legacy-mode fixture remains.

## 8. Replace Repository Gates and Baseline Semantics

- [x] 8.1 Replace `check-ifds-compat` and rollout-specific scripts with a blocking `check-goplint-soundness` entrypoint plus directly runnable semantic, oracle, refinement, determinism, mutation, and benchmark subchecks.
- [x] 8.2 Update `make check-baseline` and `make update-baseline` to invoke flagless canonical semantics and remove all legacy engine pinning.
- [x] 8.3 Make the canonical full repository scan blocking in `.github/workflows/lint.yml` and remove advisory output, dual-run logic, and rollout comments.
- [x] 8.4 Update pre-commit and local lint orchestration so the same canonical soundness gate and nested-module lint path run for relevant changes.
- [x] 8.5 Generate and review an old-to-new stable finding ID report, regenerate the baseline only after semantic gates pass, and ensure the accepted baseline does not conceal migration regressions.
- [x] 8.6 Update benchmark coverage and the reviewed threshold manifest with time, bytes, allocations, Go toolchain, CI runner class, and median-of-five enforcement for the mandatory solver, aliases, refinement, generated graphs, and repository full scan.

## 9. Synchronize Documentation and Governance

- [x] 9.1 Rewrite `tools/goplint/README.md` to document the flagless canonical pipeline, supported property boundary, blocking uncertainty, result vocabulary, resource controls, and replacement verification commands.
- [x] 9.2 Replace stale phase/rollout claims across `docs/goplint/` with an implementation-current semantic reference and evidence index; remove support claims for legacy, compare, AST, order, off, once, SMT, and `proven-safe` behavior.
- [x] 9.3 Update `AGENTS.md`, `.agents/rules/commands.md`, relevant goplint/type-system/testing skills, Make help, and workflow comments to reference the canonical soundness gate and removed flags consistently.
- [x] 9.4 Update semantic schemas, structured finding documentation, examples, and any generated snapshots to use live owner keys and the new evidence vocabulary.
- [x] 9.5 Run `make check-agent-docs` and direct semantic-catalog/schema validators, fixing every index, symlink, command, and documentation drift finding.

## 10. Complete End-to-End Verification

- [x] 10.1 Run focused tests for each historical counterexample and capture evidence that weak validation handling and wrong-object/generic gaps now report while known no-return continuations do not.
- [x] 10.2 Run all `tools/goplint` tests with `-count=1`, the race detector, deterministic fuzz seeds, bounded reference-model generation, and the targeted mutation profile.
- [x] 10.3 Run `make check-goplint-soundness`, baseline, exception governance, benchmark thresholds, `make lint`, and the blocking canonical full repository scan.
- [x] 10.4 Run root-module tests, Windows build checks, file-length/licensing checks, and any other pre-completion gates required by the touched Go, testing, workflow, and agent-documentation rules.
- [x] 10.5 Search the repository for all removed flags, modes, labels, compatibility scripts, stale commit snapshots, and advisory language; permit only explicit migration-history references that do not imply support.
- [x] 10.6 Run `git diff --check`, strict OpenSpec validation for `harden-goplint-soundness`, and a final rerun of every required goplint gate in a clean detached worktree materialized from a synthetic tree containing exactly `HEAD` plus the intended diff, using a temporary Git index that does not mutate the caller's branch or index.
