## Why

The uncommitted `complete-goplint-soundness-hardening` implementation materially strengthened goplint, but a read-only review reproduced three production false negatives and found that blocking inconclusive outcomes and several assurance layers can still be suppressed or satisfied vacuously. The soundness claim therefore remains false under its own supported-property contract even though the aggregate gate is green.

## What Changes

- Treat every relevant call expression in Go evaluation order so nested or sibling calls cannot bypass mutation, escape, summary, or fail-closed effect handling.
- Require deferred constructor validation to be proven on every successful return path, including conditional execution and later overwrites of the named error result.
- Analyze executable escaping closures as independent function bodies or emit blocking inconclusive when their protocol obligations cannot be followed soundly.
- Make protocol inconclusive outcomes always visible and reject their presence in baseline suppression data and baseline-assisted blocking scans.
- Replace marker-only and semantic-kind-only assurance credit with category-specific, causally executed evidence for production routing, independent oracles, metamorphic properties, fuzz seeds, mutations, determinism, and reporting.
- Make aggregate-gate validation prove that required recipes execute their intended checks, including adversarial no-op recipe tests.
- Make generated evidence perturb production evidence, integrate aliases, constraints, and realizable call/return behavior into differential fuzzing, and distinguish analyzer benchmarks from reference-interpreter benchmarks.
- Add a blocking freshness verifier for the exact clean synthetic-tree proof and reconcile all documentation and task claims with what the gates actually establish.
- Keep the work strictly corrective: no new lint categories, CLI modes, broader language promises, policy features, or compatibility paths.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `goplint-analysis-soundness`: Strengthen the existing supported-semantics contract for nested call evaluation, deferred validation, escaping closures, and always-visible inconclusive outcomes.
- `lint-tooling-quality-gates`: Require category-specific causal assurance, non-vacuous aggregate recipes, integrated solver fuzzing, real analyzer evidence perturbation and benchmarking, and clean-tree evidence freshness.

## Impact

- Depends on the still-open `complete-goplint-soundness-hardening` change and must be implemented against the same uncommitted tree before either change is archived as soundness-complete.
- Affects `tools/goplint/goplint/` interprocedural graph construction, effect transfer, constructor analysis, closure collection, category policy, baseline handling, diagnostics, independent oracle integration, fuzzing, mutations, determinism, and benchmarks.
- Affects `tools/goplint/cmd/gate-contract/`, the soundness Make targets, clean-tree evidence tooling, semantic coverage artifacts, baseline data, documentation, and OpenSpec completion evidence.
- Existing hidden inconclusive findings will become blocking until resolved or the analyzer gains enough proof to classify them; this is the intended correction to the declared contract.
