## Why

The uncommitted `complete-goplint-soundness-hardening` and `close-goplint-soundness-review-gaps` implementations substantially strengthened goplint, but a new read-only audit found additional production false negatives, suppressible proof uncertainty, and assurance gates that still accept vacuous or non-causal evidence. Goplint therefore cannot yet satisfy its own property-relative soundness contract or be archived as soundness-complete.

## What Changes

- Correct constructor success-path classification so branch polarity, returned error identity, and exact return-object identity cannot hide an unvalidated successful return.
- Preserve conditional validation and result-slot relations when protocol summaries are exported, imported, composed, and applied after mutation or replacement.
- Route every executable function literal through protocol analysis, including package-level stored closures, and keep closure-local inconclusive outcomes outside exception, inline-ignore, and baseline suppression.
- Make post-validation escape, mutation, malformed cross-package fact, and unresolved-effect handling conservatively invalidate proof or become blocking inconclusive.
- Make stable finding identities package-path-scoped and source-layout-independent, and make malformed or misspelled goplint directives fail visibly instead of silently disabling checks.
- Replace reporting-only mutation credit, label-only fuzz evidence, fixture-order pseudo-metamorphism, unrelated determinism credit, and hard-coded subgate populations with category-specific causal observations from the claimed production or independent boundary.
- Require every mutation kill to identify the intended semantic mismatch and every aggregate subgate to prove that its named tests and nonzero populations actually executed.
- Make exact-tree evidence cover the complete intended tracked and untracked diff, finish the two predecessor proof ledgers in dependency order, and regenerate one freshness-verified combined proof before any soundness-complete archive.
- Keep the work strictly corrective: no new lint categories, user-facing modes, broader Go-language support claims, policy features, or compatibility paths.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `goplint-analysis-soundness`: Close residual production false negatives in successful-return classification, conditional summary application, closure routing, post-validation effects, fact validation, directive handling, and stable identity.
- `lint-tooling-quality-gates`: Require non-vacuous category evidence, causal mutation attribution, executable test/population censuses, complete intended-diff selection, and a freshness-verified combined proof across the dependent changes.

## Impact

- Depends on both active predecessors, `complete-goplint-soundness-hardening` and `close-goplint-soundness-review-gaps`; neither predecessor nor this change may be archived as soundness-complete until the combined final tree satisfies the stricter proof.
- Affects `tools/goplint/goplint/` constructor identity, conditional summaries, closure routing/reporting, post-validation effects, protocol facts, directive parsing, finding IDs, and associated production-boundary fixtures.
- Affects `tools/goplint/cmd/targeted-mutation/`, semantic evidence producers, fuzz/metamorphic/determinism tests, aggregate subgate scripts, clean-tree materialization and verification, Make targets, CI comments/workflows, documentation, and retained OpenSpec evidence.
- May expose previously suppressed or missed findings. Those findings must be corrected or remain blocking inconclusive; they must not be migrated to another suppression surface.
