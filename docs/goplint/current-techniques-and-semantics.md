# goplint Current Techniques, Concepts, and Theories

Date: 2026-03-05  
Repository commit analyzed: `9027b0b`  
Primary scope: `tools/goplint/`

## Scope and Method

This document inventories the techniques, concepts, and theories used by the current `goplint` implementation, based on direct source inspection and local test execution.

Primary inspected sources:
- `tools/goplint/goplint/*.go`
- `tools/goplint/main.go`
- `tools/goplint/README.md`

Validation run during analysis:
- `cd tools/goplint && GOCACHE=/tmp/go-build go test ./goplint/...`

## Semantic Model of What goplint Checks Today

`goplint` enforces DDD-oriented type and protocol contracts through multiple analysis families:

1. Structural/type-shape checks (AST + `go/types`):
- Bare primitive usage in struct fields, function params, and returns.
- Missing/wrong `Validate()`, `String()`, constructors, constructor signatures.
- Functional-option completeness, immutability, struct `Validate()` presence/signature.
- Redundant conversion detection.

2. Protocol/dataflow checks (CFG path analysis):
- Cast-validation: conversion to validatable type must be validated on all return paths.
- Use-before-validate (same-block and cross-block): no consumption before validation.
- Constructor-validates: constructor return paths for validatable types require validation.
- Inconclusive outcomes are explicit and policy-controlled (`error|warn|off`).

3. Cross-artifact and cross-package consistency checks:
- Validate delegation completeness across struct fields.
- Nonzero-type pointer usage via cross-package facts.
- Enum sync between Go `Validate()` switch cases and CUE disjunction members.

4. Governance and regression controls:
- Exception config + inline directives.
- Deterministic finding IDs and category registry.
- Baseline suppression by semantic ID.
- JSONL finding stream + subprocess-based baseline generation and verification.

## Technique Inventory (Current Implementation)

| Technique Name | Type | Where Implemented | Current Role |
|---|---|---|---|
| Modular analyzer plugin architecture (`go/analysis`) | Concept | `goplint/analyzer.go` | Integrates checks as one analyzer with standard facts/diagnostics pipeline |
| Syntax-directed AST traversal | Technique | `goplint/analyzer_run.go`, `goplint/inspect.go` | Collects declarations, signatures, and directive-local signals |
| Type-driven filtering and classification (`go/types`) | Technique | `goplint/typecheck.go`, `goplint/inspect_contracts.go` | Distinguishes primitive/raw/named/interface-contract semantics |
| Declarative mode-flag table | Technique | `goplint/flags.go` | Keeps CLI policy, `--check-all` expansion, and run config synchronized |
| Request-scoped analyzer state isolation | Concept | `goplint/analyzer.go`, `goplint/flags.go` | Avoids global mutable state and supports parallel test execution |
| Package include filtering with fact-only fallback | Technique | `goplint/analyzer_run.go` | Suppresses diagnostics for excluded packages while still exporting facts |
| Exception algebra with glob-style path patterns | Technique | `goplint/config.go` | Controlled suppression for intentional boundary violations |
| Inline directive micro-language | Technique | `goplint/inspect.go` + directive helpers | Localized, code-adjacent suppression/behavior modifiers |
| Cross-package fact propagation (`analysis.Fact`) | Technique | `goplint/analyzer_nonzero.go`, `goplint/analyzer_constructor_validates.go` | Transfers nonzero and validates-type semantics across package boundaries |
| CFG construction over function bodies | Technique | `goplint/cfa.go`, `goplint/path_backend.go` | Basis for path-sensitive protocol checks |
| Dual backend strategy (`ssa` vs `ast`) | Technique | `goplint/path_backend.go`, `goplint/cfa.go` | Trades precision for robustness via backend selection |
| No-return pruning with alias tracking | Technique | `goplint/cfa.go` | Improves path precision by recognizing terminating calls and rebindings |
| Path-sensitive DFS from definition sites | Technique | `goplint/cfa_cast_validation.go`, `goplint/analyzer_constructor_validates_cfa.go` | Finds unsafe return-reaching paths lacking required validation |
| Tri-state path outcomes (`safe/unsafe/inconclusive`) | Concept | `goplint/cfa_outcome.go` | Makes uncertainty explicit instead of silently assuming safety |
| Budgeted exploration (states/depth) | Technique | `goplint/cfa.go`, `goplint/analyzer_run.go` | Bounds traversal cost and prevents runaway analysis |
| Adaptive budget scaling by reachable CFG size | Technique | `goplint/cfa_budget.go` | Normalizes exploration budget across small/large functions |
| SCC-aware cycle handling (Tarjan) | Technique/Theory | `goplint/cfa_traversal.go` | Avoids unsound cycle skipping and improves revisit policy in loops |
| Memoized traversal state keyed by mode/target/state | Technique | `goplint/cfa_traversal.go` | Reduces repeated path work and stabilizes results |
| Interprocedural callee target summaries (slot-aware) | Technique | `goplint/cfa_summary.go` | Models receiver/arg validation effects across helper calls |
| Recursion-cycle detection in summary derivation | Technique | `goplint/cfa_summary.go` | Prevents infinite interprocedural recursion and marks uncertainty |
| Closure-classification for synchronous path semantics | Technique | `goplint/cfa_collect.go`, `goplint/cfa.go` | Separates immediate/defer/go closure effects for validation ordering |
| Condition-context sensitivity for validate evidence | Technique | `goplint/cfa_ubv.go`, `goplint/analyzer_validate_delegation.go` | Avoids counting conditionally non-guaranteed validations as unconditional proof |
| Witness metadata generation for uncertain paths | Technique | `goplint/cfa_outcome.go`, `goplint/cfa_reporting.go` | Emits machine-readable path/call-chain evidence on inconclusive findings |
| Category registry as single source of diagnostic policy | Concept | `goplint/categories.go` | Centralizes category taxonomy and baseline suppressibility |
| Deterministic semantic finding IDs | Technique | `goplint/finding.go` | Stable suppression keys independent from message text |
| Baseline-as-policy (ID-only suppression) | Concept | `goplint/baseline.go` | Prevents regression by allowing only accepted historical findings |
| Structured finding sink (JSONL) for fail-closed baseline generation | Technique | `goplint/finding_sink.go`, `main.go` | Ensures baseline generation validates stream integrity vs analyzer output |
| Subprocess orchestration with tolerant diagnostic exit handling | Technique | `main.go` | Supports update-baseline/global-audit workflows without corrupting outputs |
| CUE/Go enum cross-validation | Technique | `goplint/analyzer_enum_sync.go` | Compares Go switch domain to CUE disjunction domain |

## Theoretical Posture in the Current Design

### 1. Conservative static analysis (soundness-leaning in CFA checks)

Current CFA checks intentionally prefer conservative treatment when proof is incomplete:
- all calls may return in AST backend,
- inconclusive classification when traversal budgets or recursion prevent proof,
- explicit policy for whether inconclusive findings fail or warn.

This aligns with a conservative over-approximation mindset for safety checks.

### 2. Graph-theoretic control-flow reasoning

The engine relies on CFG traversal and SCC decomposition:
- path existence from cast/constructor points to return blocks,
- cycle-aware revisit strategy,
- DFS with memoized state.

This is graph algorithm theory applied to static program safety rules.

### 3. Protocol/typestate flavor checks

Several checks are effectively protocol-state obligations:
- "constructed value must be validated before use",
- "constructor must validate before returning target",
- "field-validatable members must be delegated by struct Validate".

The implementation does this with syntactic+CFG reasoning rather than a formal typestate lattice.

### 4. Governance-aware analysis

Suppression and regression are first-class semantics:
- exception patterns and directives are policy inputs,
- baseline IDs encode accepted debt,
- diagnostics are categorized by suppressibility policy.

This provides operational correctness controls beyond pure analysis precision.

## Correctness Boundaries and Trade-offs (Current)

1. Soundness is not absolute end-to-end.
- Bounded exploration can produce inconclusive outcomes.
- `--cfg-inconclusive-policy=off` can intentionally hide uncertain paths.

2. Interprocedural precision is selective.
- Summary analysis focuses on receiver/argument target slots relevant to current protocols.
- It is not a full general-purpose interprocedural dataflow solver.

3. Path feasibility is structural, not constraint-complete.
- CFG/control context reasoning exists, but there is no SMT-backed feasibility discharge.

4. Suppression is policy-powerful.
- Exceptions and baseline entries are operationally necessary, but can hide real issues if overused.

5. Cross-package knowledge is fact-based and scoped.
- Facts improve practical correctness for specific rules, but do not yet provide broad abstract-state exchange.

## Evidence Index

Key files for current semantics:
- `tools/goplint/goplint/analyzer_run.go`
- `tools/goplint/goplint/flags.go`
- `tools/goplint/goplint/cfa.go`
- `tools/goplint/goplint/cfa_traversal.go`
- `tools/goplint/goplint/cfa_outcome.go`
- `tools/goplint/goplint/cfa_summary.go`
- `tools/goplint/goplint/cfa_cast_validation.go`
- `tools/goplint/goplint/cfa_ubv.go`
- `tools/goplint/goplint/analyzer_constructor_validates_cfa.go`
- `tools/goplint/goplint/analyzer_validate_delegation.go`
- `tools/goplint/goplint/analyzer_nonzero.go`
- `tools/goplint/goplint/analyzer_enum_sync.go`
- `tools/goplint/goplint/config.go`
- `tools/goplint/goplint/baseline.go`
- `tools/goplint/goplint/categories.go`
- `tools/goplint/goplint/finding.go`
- `tools/goplint/goplint/finding_sink.go`
- `tools/goplint/main.go`

## See Also

- [State-of-the-Art Soundness Roadmap](./state-of-the-art-soundness-roadmap.md)
