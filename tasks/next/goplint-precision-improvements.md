# goplint Precision Improvements — Future Work

> **STATUS: PENDING** — new improvements identified during the 2026-02-28 follow-up session.
> Each item is independently implementable as a separate PR.

## CFA Enhancements

### Use-Before-Validate Detection

**Problem:** CFA checks "path-to-return-without-validate" but not "use-before-validate."
A function that uses `x` before calling `x.Validate()` in the same block is silently
accepted — the unvalidated domain value enters a function call before being checked.

**Fix:** New `--check-use-before-validate` flag (OFF by default, included in `--check-all`).
Add `hasUseBeforeValidateInBlock` and `hasPathWithUseBeforeValidate` to `cfa.go`. New
category `use-before-validate` for baseline/exception separation.

**Risk:** Moderate FP risk — requires careful "what counts as a use" heuristic. `x.String()`
before `x.Validate()` may be acceptable; `useFunc(x)` before validate is the real target.
Start gated behind a flag, iterate on FP feedback.

**Files:**
- `tools/goplint/goplint/cfa.go` — `hasUseBeforeValidateInBlock`, `isVarUse`, `hasPathWithUseBeforeValidate`
- `tools/goplint/goplint/cfa_cast_validation.go` — call new check after existing CFA check
- `tools/goplint/goplint/analyzer.go` — add flag, category constant, wire into `--check-all`
- `tools/goplint/goplint/testdata/src/cfa_castvalidation/` — update ValidateAfterUse fixture
- `tools/goplint/goplint/cfa_integration_test.go` — `TestCheckUseBeforeValidateCFA`
- `tools/goplint/goplint/integration_test.go` — reset in `resetFlags`

**Effort:** 3-4 hours. **Impact:** Medium-high (eliminates a class of security-relevant FNs).

## Directive Enhancements

### Cross-Package Constructor-Validates Directive

**Problem:** `--check-constructor-validates` only follows same-package function calls and
method calls on the return type. Cross-package helper functions that call `Validate()` on
behalf of a constructor are not tracked — produces false positives.

**Example:**
```go
// package util
func ValidateServer(s *Server) error { return s.Validate() }

// package myapp — flagged as missing Validate even though util delegates
func NewServer() (*Server, error) {
    s := &Server{}
    return s, util.ValidateServer(s)
}
```

**Fix:** New `//goplint:validates-type=TypeName` directive. Place on a cross-package helper
function to tell the analyzer "this function validates TypeName." In `bodyCallsValidateTransitive`,
when encountering a cross-package call, check if the callee has the directive and if
TypeName matches `returnTypeName`.

**Files:**
- `tools/goplint/goplint/inspect.go` — add `"validates-type"` to `knownDirectiveKeys`
- `tools/goplint/goplint/analyzer_constructor_validates.go` — extend `bodyCallsValidateTransitive`
- `tools/goplint/CLAUDE.md` — document the new directive
- `tools/goplint/goplint/testdata/src/constructorvalidates_cross/` — new cross-package fixture
- `tools/goplint/goplint/integration_test.go` — `TestConstructorValidatesCrossPackage`

**Effort:** 3-4 hours. **Impact:** Medium (eliminates cross-package helper delegation FPs).

## Auto-Skip Context Expansions

### `strings.*` Comparison Calls

**Problem:** `strings.Contains(string(x), CommandName(raw))` is comparison-like but not
auto-skipped. The cast is used in a display/comparison context, not as a domain value.

**Fix:** Add `isStringsComparisonCall(pass, call, argIdx)` that checks if the outer call is
`strings.Contains/HasPrefix/HasSuffix/EqualFold` and the cast is the first argument.
Only skip comparison functions — `strings.Replace`, `strings.Split` should still flag.

**Files:**
- `tools/goplint/goplint/analyzer_cast_validation.go` — add helper, extend `isAutoSkipContext`
- `tools/goplint/goplint/testdata/src/castvalidation/` — add comparison fixture cases
- `tools/goplint/goplint/testdata/src/cfa_castvalidation/` — CFA variants

**Effort:** 2 hours. **Impact:** Low (uncommon pattern, but clean to add).

## Test Coverage

### `isAutoSkipAncestor` Depth Limit Verification

**Problem:** The `maxAncestorDepth = 5` bound in `isAutoSkipAncestor` has no test fixture
verifying the boundary. A cast at exactly depth 5 should be auto-skipped; at depth 6
it should be flagged.

**Fix:** Add fixture cases to `testdata/src/castvalidation/castvalidation.go` and
`testdata/src/cfa_castvalidation/cfa_castvalidation.go` with a DDD cast nested at
exactly depth 5 and depth 6 inside composite literals within `fmt.Sprintf`.

**Files:**
- `tools/goplint/goplint/testdata/src/castvalidation/castvalidation.go`
- `tools/goplint/goplint/testdata/src/cfa_castvalidation/cfa_castvalidation.go`

**Effort:** 1 hour. **Impact:** Low (test coverage, no behavioral change).
