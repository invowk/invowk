## Why

`goplint` has strong operational governance, but its protocol analyses can currently treat a failed `Validate()` call as successful validation, conflate distinct same-typed objects, discharge feasible paths through unversioned constraints, and report false positives around non-returning calls. The repository should not describe those results as sound or `proven-safe` until one canonical analysis pipeline has explicit semantics, conservative uncertainty handling, and independent evidence for its conclusions.

## What Changes

- Define a formal, machine-checked soundness contract for every goplint protocol/dataflow rule, including concrete obligations, abstract states, transfer and join rules, object identity, call/return behavior, uncertainty, and the exact conditions under which `violation` and `inconclusive` may be emitted or a checked `discharged-infeasible` trace may be recorded.
- Make validation success control-flow-sensitive: a value becomes validated only on paths where the relevant `Validate()` result is proven nil, including selector calls, method values, helper summaries, constructors, and cross-package facts. Continuing after a non-nil result, merely assigning the error, or validating a different object must not discharge the obligation.
- Replace type-only and flow-insensitive receiver matching with SSA-versioned, allocation/object-sensitive tracking with assignment kill semantics; ambiguous alias or points-to relationships remain inconclusive rather than optimistic.
- Correct the interprocedural core to use realizable, call/return-matched paths, context-sensitive summaries, accurate no-return behavior, generic call/type-parameter handling, and fail-closed unresolved-call behavior.
- Replace the current unversioned Phase C predicate checker with SSA-versioned feasibility/refinement semantics. Only a soundly established UNSAT result may discharge a witness; unsupported predicates, resource exhaustion, solver failures, and incomplete refinement remain inconclusive.
- Build independent, adversarial, generated, metamorphic, fuzz, and mutation-backed semantic oracles that cover every registered category and specifically lock the known validation-result, method-value, alias-rebinding, wrong-object, generic, no-return, cross-package, and feasibility counterexamples.
- Make the semantic catalog executable and complete: declared implementation entrypoints must resolve, all registered categories must have explicit semantic/oracle ownership, stale provenance must fail, and proof-status terminology must match actual guarantees.
- **BREAKING**: remove the legacy/compare interprocedural engines, AST protocol-analysis backend, order-only UBV mode, disabled alias mode, off/one-shot feasibility and refinement modes, and warning/off handling for inconclusive soundness-critical outcomes. Remove their CLI flags and production code paths instead of retaining aliases or compatibility shims.
- Use one mandatory production pipeline by default: SSA-backed escape semantics, corrected IFDS/IDE-style interprocedural analysis, SSA/object-sensitive alias tracking, iterative feasibility refinement, and fail-closed inconclusive outcomes. Resource limits remain configurable only when exhaustion cannot become `safe`.
- Replace legacy differential and legacy-pinned baseline gates with canonical-engine soundness, oracle, determinism, performance, and full-scan gates; make local, pre-commit, baseline, and CI invocations exercise the same semantics.
- Update goplint documentation and roadmap material to describe only implemented guarantees and current evidence, removing phase-rollout and legacy-mode guidance.

## Capabilities

### New Capabilities

- `goplint-analysis-soundness`: Defines the canonical analyzer semantics, validation-success and object-identity obligations, interprocedural and feasibility guarantees, independent oracle coverage, proof-status vocabulary, and clean removal of alternate analysis modes.

### Modified Capabilities

- `lint-tooling-quality-gates`: Replaces legacy-pinned and advisory goplint gates with one blocking canonical-engine contract shared by baseline generation, local validation, pre-commit, and CI.

## Impact

- Affects `tools/goplint/goplint/`, its CLI flags, semantic catalog/schema, fixtures, tests, benchmarks, baseline and exception workflows, and helper scripts.
- Affects `Makefile`, pre-commit hooks, `.github/workflows/lint.yml`, agent command guidance, `tools/goplint/README.md`, and `docs/goplint/`.
- Removes unsupported command-line modes immediately; repository automation and any external goplint invocations using those flags must migrate to the flagless canonical pipeline.
- May require a carefully governed feasibility-solver dependency or an exact decision procedure for the formally supported predicate fragment; any selected dependency must follow repository version-pinning and cross-platform tooling policy.
- Intentionally accepts additional `inconclusive` failures and analysis cost where necessary to prevent optimistic safety claims.
