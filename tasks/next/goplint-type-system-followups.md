# goplint & Type System Deferred Improvements

> **STATUS: PENDING** — items deferred from the 2026-02-28 goplint precision session.
> Each item is independently implementable as a separate PR.

## Type System Gaps

### 2E. Type `EnvConfig.Vars` Map Keys as `EnvVarName`

**Problem:** `EnvConfig.Vars` is `map[string]string` but keys should be `EnvVarName`.
**Fix:** Change field type in `pkg/invowkfile/env.go`, update `GetVars()` return type,
add conversion at CUE parse boundary, update ~5 downstream consumers.
**Impact:** Baseline -2 to -4.
**Files:** `pkg/invowkfile/env.go`, `pkg/invowkfile/parse.go`, ~5 consumer files.

### 4B. Type `cueutil.ValidationError` Fields

**Problem:** `FilePath`, `CUEPath`, `Message`, `Suggestion` are bare `string`.
**Fix:** Change `FilePath` → `types.FilesystemPath`, add new `CUEPath string` type in
`pkg/cueutil/`, change `Message`/`Suggestion` → `types.DescriptionText`.
**Impact:** Baseline -4 to -6.
**Files:** `pkg/cueutil/` (~3 construction sites, ~6-8 consumer files).

## Enforcement Improvements

### 3A. New Mode: `--audit-exceptions --global`

**Problem:** `--audit-exceptions` reports per-package (a pattern valid in pkg A appears
stale in pkg B). Manual deduplication required.
**Fix:** New `main.go` flag using the established subprocess pattern. Run
`-audit-exceptions -json` as subprocess, aggregate findings across all packages, report
patterns that never matched in any package.
**Files:** `tools/goplint/main.go` (~150 lines).

### 4E. Constructor Validates: Follow Method Calls on Return Type

**Problem:** `bodyCallsValidateTransitive` only follows same-package non-method functions.
If a constructor calls `result.Setup()` which internally calls `result.Validate()`, this
is missed.
**Fix:** Extend `bodyCallsValidateTransitive` to follow method calls on variables whose
type matches `returnTypeName`.
**Files:** `tools/goplint/goplint/analyzer_constructor_validates.go`, test fixtures.

### 4I. New Advisory Mode: `--suggest-validate-all`

**Problem:** Only 15 structs currently annotated with `//goplint:validate-all`. Some
structs with `Validate()` + validatable fields may be missing the directive.
**Fix:** Advisory mode reporting structs that have `Validate()` + validatable fields
but no `//goplint:validate-all` directive.
**Files:** `tools/goplint/goplint/analyzer.go` (~100 lines new mode).

## Structural Improvements

### 3D. Create Typed Path Wrapper `pkg/fspath/`

**Problem:** ~38 `//goplint:ignore` sites in `pkg/invowkfile/` and `pkg/invowkmod/` all
do the same pattern: `FilesystemPath(filepath.Join(string(path), ...))`.
**Fix:** Create `pkg/fspath/` with `Join`, `Dir`, `Abs`, `Rel` wrappers that accept/return
`types.FilesystemPath`. Centralizes the single `//goplint:ignore` inside the wrapper.
**Files:** New `pkg/fspath/fspath.go`, `pkg/fspath/fspath_test.go`, ~38 call-site updates.

## Baseline Reclassification

### 4F. TUI/Container/Env Map — Baseline-to-Exception Migration

**Problem:** 38 TUI + 4 container + 19 env map baseline findings are genuine framework
boundaries with no domain semantics beyond rendering/exec.
**Fix:** Move ~61 findings from baseline to exceptions with documented reasons. Reduces
baseline from 240 to ~179 with better signal-to-noise.
**Files:** `tools/goplint/exceptions.toml`, `tools/goplint/baseline.toml`.

### 4G. Constructor Exception Granularity

**Problem:** 15 `pkg.*.constructor` blanket patterns hide structs that may now warrant
constructors after type migration.
**Fix:** Replace blanket patterns with specific struct-level exceptions after auditing
each exported struct.
**Files:** `tools/goplint/exceptions.toml`.

### 4H. Cast-Validation Exception Tightening

**Problem:** 7 `pkg.*.cast-validation` blanket patterns were added during initial CFA
rollout and may mask legitimate unvalidated casts in new code.
**Fix:** Replace blanket patterns with specific function-level exceptions.
**Files:** `tools/goplint/exceptions.toml`.
