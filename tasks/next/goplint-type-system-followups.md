# goplint & Type System Deferred Improvements

> **STATUS: PARTIALLY COMPLETED** — items from the 2026-02-28 goplint precision session.
> Items marked DONE were completed in the `chore/lint-modern-go-enforcement` branch.
> Remaining items are independently implementable as separate PRs.

## Type System Gaps

### 2E. Type `EnvConfig.Vars` Map Keys as `EnvVarName` — DONE

Changed `Vars map[string]string` → `map[EnvVarName]string` in `pkg/invowkfile/env.go`.
`GetVars()` converts keys back to `map[string]string` for backward compatibility with
`maps.Copy` consumers. Baseline -5 (240 → 235).

### 4B. Type `cueutil.ValidationError` Fields — DONE

Changed `FilePath` → `types.FilesystemPath`, `CUEPath` → new `CUEPath` type,
`Message`/`Suggestion` → `types.DescriptionText`. New `pkg/cueutil/cuepath.go` DDD type.

## Enforcement Improvements

### 3A. New Mode: `--audit-exceptions --global` — DONE

Subprocess-based aggregation in `main.go`. Reports patterns stale in ALL packages.

### 4E. Constructor Validates: Follow Method Calls on Return Type — DONE

Extended `bodyCallsValidateTransitive` to follow `*ast.SelectorExpr` method calls on
variables whose type matches `returnTypeName`. Uses existing `findMethodBody()`.

### 4I. New Advisory Mode: `--suggest-validate-all` — DONE

Reports structs with `Validate()` + validatable fields but no `//goplint:validate-all`
directive. NOT included in `--check-all` (advisory only).

## Structural Improvements

### 3D. Create Typed Path Wrapper `pkg/fspath/` — DONE

Created `pkg/fspath/` with `Join`, `JoinStr`, `Dir`, `Abs`, `Clean`, `FromSlash`, `IsAbs`.
Converted ~10 call sites in `pkg/invowkmod/` and `pkg/invowkfile/`. Also tightened
discovery cast-validation exceptions from 2 blanket patterns to 6 specific ones.

## Baseline Reclassification — PENDING (follow-up PR)

### 4F. TUI/Container/Env Map — Baseline-to-Exception Migration

**Problem:** 38 TUI + 4 container + 19 env map baseline findings are genuine framework
boundaries with no domain semantics beyond rendering/exec.
**Fix:** Move ~61 findings from baseline to exceptions with documented reasons. Reduces
baseline from 235 to ~174 with better signal-to-noise.
**Files:** `tools/goplint/exceptions.toml`, `tools/goplint/baseline.toml`.

### 4G. Constructor Exception Granularity

**Problem:** 15 `pkg.*.constructor` blanket patterns hide structs that may now warrant
constructors after type migration.
**Fix:** Replace blanket patterns with specific struct-level exceptions after auditing
each exported struct.
**Files:** `tools/goplint/exceptions.toml`.

### 4H. Cast-Validation Exception Tightening

**Problem:** 7 `pkg.*.cast-validation` blanket patterns (excluding discovery, already
tightened) were added during initial CFA rollout and may mask legitimate unvalidated
casts in new code.
**Fix:** Replace blanket patterns with specific function-level exceptions.
**Files:** `tools/goplint/exceptions.toml`.
