## Why

The completed `harden-goplint-soundness` change added substantial formal and verification machinery, but a post-implementation audit found that production analysis still admits concrete false negatives and that several green gates do not exercise the guarantees they claim. The user subsequently authorized archiving that predecessor on 2026-07-14. This corrective change therefore treats the archived artifacts and synchronized canonical specs as dependency evidence, closes every audited semantic, architectural, and evidence-integrity gap before goplint is described as property-sound, and remains active until the stricter proof is complete.

## What Changes

- Make the documented conditional validation domain the domain actually propagated by production: validation applies only on the matching nil-error edge, never at the call node or on a failing continuation.
- Replace constructor type-only and historical undirected alias matching with SSA-versioned object/return-slot identity and kill-aware must-alias relations.
- Make unresolved relevant effects conservative even after validation, including mutation, replacement, escape, concurrent access, and escaped-heap effects that may invalidate the established property.
- Preserve an exact relationship between every joined abstract state and its realizable interprocedural witness so refinement can discharge only the state/path pair proven infeasible.
- Make missing SSA, ambiguous identity, unsupported generic constraints, unresolved effects, rejected evidence, and actual timeout/budget exhaustion blocking inconclusive outcomes at every production entry point.
- Replace call-stack-depth recursion with finite summary tabulation and make no-return alias resolution flow-sensitive at each call site.
- Remove all remaining compiled legacy CFG modes, nil-context fallbacks, legacy DFS evaluators, unconditional `ValidatesTypeFact` compatibility, and other alternate semantic authorities.
- Enforce catalog and oracle completeness for every registered category, including `unvalidated-boundary-request`, with real analyzer boundary fixtures and explicit must-report, must-not-report, and must-be-inconclusive expectations.
- Make bounded generation derive every claimed graph dimension from the reviewed manifest and compare the end-to-end production analyzer, not only a solver-core adapter, with an independent reference model.
- Replace synthetic determinism claims with repeated and reordered real package analysis; strengthen fuzzing to generate meaningful graph, identity, branch, call/return, alias, and refinement shapes with independent properties.
- Make targeted mutation evidence causal through an unmutated control run, exact mutant-to-guard attribution, and rejection of pre-existing or unrelated failures.
- Record reproducible completion evidence from a clean synthetic-tree worktree and make documentation describe only guarantees exercised by the production path and blocking gates.
- Do not add lint categories, broader Go-language support, new user-facing modes, or weaker compatibility paths.

## Capabilities

### New Capabilities

- `goplint-soundness-assurance`: Defines the production-integration, fail-closed, realizable-witness, finite-summary, and independent-evidence requirements needed for goplint's existing protocol rules to satisfy their declared soundness boundary.

### Modified Capabilities

- `lint-tooling-quality-gates`: Strengthens goplint's blocking quality gates so catalog coverage, real-analyzer determinism, bounded generation, fuzz properties, causal mutation evidence, legacy-path absence, and clean-tree completion proof cannot pass vacuously.

## Impact

- Affects `tools/goplint/goplint/` protocol domains, SSA/alias construction, constructor tracking, interprocedural tabulation, no-return modeling, refinement witnesses, uncertainty aggregation, generic normalization, and summary facts.
- Removes remaining legacy production code and may change internal fact formats, diagnostic reasons, stable finding IDs, and repository-local baseline entries; no user-facing feature or semantic-mode replacement is introduced.
- Affects semantic catalogs and schemas, analyzer fixtures, the independent oracle and generator, fuzz targets/corpora, mutation tooling/manifests, determinism checks, benchmarks, Make targets, pre-commit/CI workflows, and goplint documentation.
- Requires an implementation review against the archived `harden-goplint-soundness` artifacts so the predecessor and corrective change remain coherent; this change is not complete until every prior task claim contradicted by the audit is either implemented or replaced by a stricter verified contract. The predecessor archive is preserved and is not reopened as part of this work.
