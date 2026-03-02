# goplint Long-Range Maintainability Refactor

> **STATUS: PENDING** - planned 2026-03-01.
> Scope: `run()` modularization + declarative flag-table rewrite.

## Why This Refactor Is Needed

The current analyzer is functionally correct, but it still carries maintainability hotspots that will keep slowing future work:

1. **Flag wiring is duplicated across many places**
   - Package-level bindings in `tools/goplint/goplint/analyzer.go`
   - `init()` registration (`Analyzer.Flags.BoolVar` / `Var`)
   - `runConfig` fields and `newRunConfig()` copy logic
   - `--check-all` expansion logic
   - `integration_test.go` reset list and explicit assertions
   - Result: every new mode requires synchronized edits in multiple files and tests.
2. **`run()` mixes orchestration, collection, and execution**
   - One function currently handles config loading, needs planning, AST traversal, per-node analysis, and post-traversal execution.
   - This increases review overhead and makes behavior changes riskier.
3. **Mode-growth cost is too high**
   - The analyzer has many supplementary modes, and each new mode now adds more branching in both production and tests.
   - We need a model where "add one mode" means one declarative row plus one checker hook.

## Goals

1. Make analyzer flag behavior **declarative and single-source**.
2. Split `run()` into explicit phases with small helper functions.
3. Keep current behavior exactly the same (no semantic change) while lowering drift risk.
4. Keep `go/analysis` compatibility (`Analyzer.Flags` remains the external contract).

## Non-Goals

1. No diagnostic category changes.
2. No changes to exception syntax, baseline format, or CFA semantics.
3. No performance micro-optimization unless required by refactor safety.

## Target Design

## A. Declarative Flag Table

Add a new internal declaration table for all supplementary mode flags.

### Proposed Types

```go
type modeKey string

type modeSpec struct {
    Key          modeKey
    FlagName     string
    Usage        string
    Default      bool
    InCheckAll   bool
}
```

For string/path flags (`config`, `baseline`), keep explicit tracked bindings but centralize metadata alongside mode specs.

### Behavior Rules

1. A single `modeSpecs` table defines every bool mode flag.
2. `registerModeFlags(Analyzer.Flags)` iterates the table to call `BoolVar`.
3. `newRunConfig()` uses the same table to snapshot enabled modes.
4. `--check-all` expansion toggles modes by `InCheckAll == true`.
5. `integration_test` reset/completeness checks iterate the same specs rather than hardcoding each flag.

### Benefits

1. Removes multiple drift-prone manual lists.
2. Makes `--check-all` inclusion policy obvious and reviewable.
3. Makes future mode addition mostly additive.

## B. `run()` Modularization

Refactor `run()` into explicit phases and small data structs.

### Proposed Flow

```text
run()
  -> newRunContext(pass)
  -> collectDeclData(ctx)
  -> runPostTraversalChecks(ctx)
  -> runAuditAndAdvisoryChecks(ctx)
```

### Proposed Structs

```go
type runContext struct {
    pass *analysis.Pass
    cfg  *ExceptionConfig
    bl   *BaselineConfig
    rc   runConfig
    insp *inspector.Inspector
    needs collectionNeeds
    data  collectedData
}

type collectionNeeds struct {
    constructors bool
    structFields bool
    optionTypes  bool
    withFuncs    bool
    methods      bool
    constOnly    bool
}

type collectedData struct {
    namedTypes         []namedTypeInfo
    exportedStructs    []exportedStructInfo
    methodSeen         map[string]*methodInfo
    constructorDetails map[string]*constructorFuncInfo
    optionTypes        map[string]string
    withFunctions      map[string][]withFuncInfo
    constantOnlyTypes  map[string]bool
}
```

### Extraction Plan

1. `resolveRunInputs(pass, rc)` loads config/baseline and inspector.
2. `deriveCollectionNeeds(rc)` computes all `need*` booleans once.
3. `initCollectedData(needs)` allocates only required maps.
4. `walkDecls(ctx)` performs `GenDecl`/`FuncDecl` traversal.
5. `runSupplementaryChecks(ctx)` handles post-traversal checks in stable order.
6. `runAuditChecks(ctx)` handles `audit-exceptions`, `audit-review-dates`, and advisory checks.

### Benefits

1. Clear orchestration boundaries and simpler reasoning.
2. Smaller units for targeted tests.
3. Easier future insertion of new checks without bloating `run()`.

## Implementation Plan (Ready to Execute)

## Phase 1: Declarative Flag Table

1. Add `tools/goplint/goplint/flags.go` with `modeKey`, `modeSpec`, `modeSpecs`.
2. Move bool-flag registration from `init()` into `registerModeFlags`.
3. Rework `runConfig` to store enabled mode values from `modeSpecs`.
4. Rework `newRunConfig()` and `--check-all` expansion to iterate specs.
5. Update tests:
   - `TestResetFlagsCompleteness` should verify all declared specs reset to defaults.
   - `TestNewRunConfig` should verify `InCheckAll` behavior declaratively.

## Phase 2: `run()` Modularization

1. Add `tools/goplint/goplint/analyzer_run.go` for orchestration helpers.
2. Introduce `runContext`, `collectionNeeds`, `collectedData`.
3. Extract collection planning and data initialization helpers.
4. Extract traversal body to `walkDecls`.
5. Extract post-traversal and audit/advisory execution helpers.
6. Keep public function names unchanged where possible to minimize churn.

## Phase 3: Safety and Cleanup

1. Keep a temporary parity test to compare old/new check-all expansion (remove after confidence if redundant).
2. Update `tools/goplint/AGENTS.md` and `tools/goplint/CLAUDE.md` architecture notes after code lands.
3. Ensure integration tests still rely on `Analyzer.Flags.Set()` and remain non-parallel.

## File-Level Change Matrix

1. `tools/goplint/goplint/flags.go` (new): declarative flag specs + helpers.
2. `tools/goplint/goplint/analyzer.go`: slim `init()`, slim `run()`, delegate helpers.
3. `tools/goplint/goplint/analyzer_run.go` (new): run context, needs planning, traversal orchestration.
4. `tools/goplint/goplint/integration_test.go`: declarative reset/assertion helpers.
5. `tools/goplint/CLAUDE.md` and `tools/goplint/AGENTS.md`: architecture update after implementation.

## Acceptance Criteria

1. Adding a new bool mode requires:
   - one `modeSpecs` entry,
   - one checker implementation hook,
   - zero manual edits to reset/check-all hardcoded lists.
2. `run()` is orchestration-only (high-level flow and error handling), with detailed logic in helpers.
3. `--check-all` inclusion/exclusion remains exactly unchanged from current behavior.
4. Existing diagnostics and baseline suppression behavior remain unchanged.

## Verification Plan

```bash
# Fast targeted safety checks first
cd tools/goplint && go test ./goplint -run 'TestResetFlagsCompleteness|TestNewRunConfig|TestCheckAll|TestCFADoesNotAffectCheckAll'
cd tools/goplint && go test ./goplint
cd tools/goplint && go test -race ./goplint

# Repository gates
make check-baseline
make check-types-all
make lint
make test
```

## Risks and Mitigations

1. **Risk:** silent mode behavior drift after migration.
   - **Mitigation:** table-driven assertions for `--check-all` and defaults.
2. **Risk:** lost checker invocation ordering.
   - **Mitigation:** keep execution order in one explicit helper with comments and dedicated tests.
3. **Risk:** accidental `Analyzer.Flags` state coupling regressions in tests.
   - **Mitigation:** preserve sequential integration tests and central reset helper.

## Rollout Strategy

1. Land Phase 1 and Phase 2 together if review bandwidth allows.
2. If needed, split into two commits:
   - Commit A: declarative flag-table + tests.
   - Commit B: `run()` modularization with no behavior changes.
3. Run full gate commands before merge to confirm zero regressions.
