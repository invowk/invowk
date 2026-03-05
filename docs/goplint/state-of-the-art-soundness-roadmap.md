# goplint State-of-the-Art Soundness Roadmap

Date: 2026-03-05  
Correctness posture: **Soundness-first**  
Current baseline: [Current Techniques and Semantics](./current-techniques-and-semantics.md)

## Goal

Define long-term, textbook-grade static-analysis upgrades for goplint's problem space, with priority on minimizing false negatives even when this increases inconclusive outcomes and runtime cost.

## Problem-Space Semantics (What Must Be Proven)

The core obligations goplint enforces can be stated as safety properties:

1. Cast-validation obligation:
- For conversion from raw primitive input to validatable named type, every executable path to externally meaningful use/return should pass validation.

2. Use-before-validate prohibition:
- A value requiring validation must not be consumed before validation on any feasible path.

3. Constructor return obligation:
- Constructors returning validatable values must validate on all relevant return paths.

4. Delegation completeness obligation:
- Struct `Validate()` implementations should cover all validatable fields under declared semantics.

5. Cross-artifact consistency obligation:
- Go enum validation domains and CUE schema disjunction domains should be extensionally aligned.

6. Governance obligation:
- Suppression should remain semantically stable and auditable (deterministic IDs, policy-aware baselines).

## State-of-the-Art Techniques to Adopt (Long Term)

| Technique / Theory | Why It Is SOTA for This Problem Space | goplint-Relevant Upgrade |
|---|---|---|
| Monotone dataflow framework and MFP reasoning (Kildall) | Canonical foundation for proving convergence and conservative fixed points | Formalize each check as transfer functions over CFG states, with explicit join/meet rules |
| Abstract interpretation (Cousot) with Galois-connected abstractions | Standard framework for sound static analysis under abstraction | Define abstract domains for validation state, alias targets, and escape state; encode sound abstraction boundaries |
| Widening/narrowing discipline | Textbook method to guarantee termination while recovering precision | Replace ad hoc budget fallback with theory-backed convergence controls where possible |
| IFDS (interprocedural finite distributive subset) | Gold-standard precise interprocedural dataflow for finite facts | Model typestate facts (validated/unvalidated/escaped) as IFDS facts for cast, UBV, constructor checks |
| IDE (interprocedural distributive environment) | Extends IFDS with value environments and edge functions | Track richer per-value state transformations (for example helper effects and contextual validation guarantees) |
| Weighted pushdown systems (WPDS) | Strong model for recursion and call/return matching with dataflow weights | Improve recursion + context precision beyond local memo summaries while preserving stack discipline |
| SMT-backed path feasibility (DPLL(T), SMT-LIB) | Standard for discharging infeasible paths and reducing spurious alarms | Validate or refute suspicious CFG witnesses before reporting unsafe/inconclusive |
| CEGAR (counterexample-guided abstraction refinement) | Canonical loop for refining abstractions only where needed | Turn inconclusive witnesses into refinement triggers instead of permanent uncertainty |
| Symbolic execution for witness replay | High-signal way to validate path-level counterexamples | Replay emitted witness paths to triage infeasible or underconstrained reports |
| Alias/points-to analysis hierarchy | Necessary for correctness in receiver/argument tracking across assignments and calls | Replace/augment local alias heuristics with principled points-to precision tiers |
| Relational abstract domains (for example Octagons) | Improves precision for path conditions and numeric guards | Better handling of guard-dependent validation/use ordering in complex control flow |

## Target Architecture (Soundness-First)

### 1. Formal property layer

Define each goplint rule family as a formal safety property:
- state space,
- transfer relation,
- proof obligation,
- sound default on uncertainty.

This turns rule evolution into semantics-preserving design instead of heuristic growth.

### 2. Unified interprocedural analysis engine

Evolve from selective call summaries toward an IFDS/IDE-style engine:
- call/return matched propagation,
- explicit context handling,
- reusable fact domains across checks.

### 3. Uncertainty and refinement layer

Keep conservative defaults but add refinement:
- generate witness,
- test feasibility with SMT/symbolic replay,
- refine abstraction (CEGAR) for recurring spurious/inconclusive patterns.

### 4. Governance-integrated soundness controls

Keep baseline/exception model, but require stronger metadata:
- proof status (`proven-safe`, `unsafe`, `inconclusive-refined`, `inconclusive-unrefined`),
- reason provenance,
- refinement history for high-churn findings.

## Phased Adoption Plan

## Phase A: Semantic Formalization and Oracle Hardening

Deliverables:
- Rule-spec document mapping each category to formal state and transfer rules.
- Property-focused test corpus with must-report and must-not-report proofs.
- Explicit soundness contracts for `inconclusive` handling.

Acceptance criteria:
- Every CFA-backed category has a machine-checkable spec skeleton.
- Regression suite includes counterexamples for known historical misses.

## Phase B: IFDS/IDE-Style Interprocedural Core

Deliverables:
- New interprocedural solver path for cast/UBV/constructor-validates.
- Fact domain definitions and edge-function semantics.
- Compatibility mode for comparing old/new engines.

Acceptance criteria:
- Equal or fewer false negatives on curated correctness suite.
- No silent downgrade when recursion depth/call complexity increases.

## Phase C: Feasibility + CEGAR Refinement

Deliverables:
- SMT feasibility checker for witness paths.
- Refinement loop for recurring inconclusive classes.
- Diagnostics tagged with refinement outcome metadata.

Acceptance criteria:
- Measurable reduction in persistent inconclusive findings without adding false negatives.
- Clear explanation artifacts for each refined decision.

## Phase D: Precision Domains and Alias Upgrade

Deliverables:
- Configurable alias analysis tiers.
- Optional relational domains for guard-heavy paths.
- Cost controls and benchmark gates.

Acceptance criteria:
- Better precision on alias-sensitive fixtures with no soundness regression.
- Runtime growth stays within defined budget envelopes.

## Long-Term Defaults (Soundness-First)

1. Keep inconclusive-as-failing by default for CI-critical checks.
2. Treat suppression as policy debt requiring explicit review cadence.
3. Prefer conservative unsafe/inconclusive classification over optimistic safe inference.
4. Require path witness metadata for every uncertain outcome.

## Suggested Measurement Dashboard

1. Correctness metrics:
- False-negative rate on seeded violation suites (target: monotonic decrease).
- Unsafe findings later disproven by refinement (target: controlled and decreasing).

2. Uncertainty metrics:
- Inconclusive rate per category and cause (`state-budget`, `depth-budget`, `recursion-cycle`, `unresolved-target`).
- Refinement success ratio.

3. Performance metrics:
- Time per package and per check family.
- Solver/refinement overhead percentiles.

4. Governance metrics:
- Baseline growth/shrink trends.
- Exception churn and overdue review counts.

## Priority Recommendation

For utmost correctness in this codebase, the highest leverage sequence is:

1. Formalize semantics and build property-oriented test oracles.
2. Move cast/UBV/constructor checks to an IFDS/IDE-style interprocedural foundation.
3. Add SMT+CEGAR refinement to reduce spurious/inconclusive cases without sacrificing soundness.
4. Upgrade alias/path-condition precision selectively where data justifies complexity.

## Primary References

### Go analysis framework (implementation substrate)
- `go/analysis`: https://pkg.go.dev/golang.org/x/tools/go/analysis
- `go/cfg`: https://pkg.go.dev/golang.org/x/tools/go/cfg
- `go/ssa`: https://pkg.go.dev/golang.org/x/tools/go/ssa

### Core static-analysis theory
- Kildall, *A Unified Approach to Global Program Optimization* (POPL 1973): https://courses.compute.dtu.dk/02242/archive/f2024/topics/unbounded-static-analysis/Kildall-1973.PDF
- Cousot and Cousot, abstract interpretation resources: https://www.di.ens.fr/~cousot/AI/
- Cousot et al., *The Octagon Abstract Domain* (SAS 2006 tutorial): https://www.di.ens.fr/~cousot/publications.www/CousotCousotRadhiaM-invited-tutorial-SAS-2006.pdf

### Interprocedural precision theory
- Reps, Horwitz, Sagiv, IFDS (POPL 1995): https://theory.stanford.edu/~srirams/papers/ifdssolver.pdf
- Sagiv, Reps, Horwitz, IDE (TOPLAS 1996): https://www.sciencedirect.com/science/article/pii/S0304397596000722
- Reps, Schwoon, Jha, Melski, WPDS (SCP 2005): https://www.sciencedirect.com/science/article/pii/S0167642304000861

### Feasibility/refinement methods
- Clarke et al., CEGAR (CAV 2000 chapter): https://doi.org/10.1007/10722167_15
- Barrett et al., *Satisfiability Modulo Theories* (Handbook chapter / DPLL(T) context): https://theory.stanford.edu/~barrett/pubs/BSST09.pdf
- SMT-LIB standard initiative: https://smt-lib.org
- King, *Symbolic Execution and Program Testing* (CACM 1976): https://research.ibm.com/publications/symbolic-execution-and-program-testing

### Alias/points-to precision
- Steensgaard, *Points-to Analysis in Almost Linear Time* (MSR TR / POPL 1996): https://www.microsoft.com/en-us/research/publication/points-to-analysis-in-almost-linear-time/
