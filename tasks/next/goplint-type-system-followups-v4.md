# goplint & Type System Follow-Up Improvements (v4)

> **STATUS: PENDING** — planned 2026-03-01.
> Follows from `goplint-type-system-followups.md` (v3, COMPLETED).

## Context

All items from the previous goplint precision session are COMPLETED. The baseline
is at 0 findings. This plan identifies the next round of improvements to eliminate
false positives/negatives, close test coverage gaps, and extend CFA precision to
constructor validation.

## Improvements (6 items, dependency-ordered)

### 1. `bytes` Package Comparison Auto-Skip (FP fix, low complexity)

**Problem**: `strings.Contains/HasPrefix/HasSuffix/EqualFold` are auto-skipped as
comparison contexts, but their `bytes` package mirrors are not. Casts inside
`bytes.Contains([]byte(string(DddType(x))), ...)` are incorrectly flagged.

**Changes**:
- `tools/goplint/goplint/analyzer_cast_validation.go`:
  - Add `bytesComparisonFuncs` map (`Contains`, `HasPrefix`, `HasSuffix`, `EqualFold`)
  - Add `isBytesComparisonCall()` (same pattern as `isStringsComparisonCall`)
  - Add `isBytesComparisonCall` to `isAutoSkipCall()` disjunction
- `testdata/src/castvalidation/castvalidation.go`: Add `BytesContainsAutoSkip` fixture
- `testdata/src/cfa_castvalidation/cfa_castvalidation.go`: Add `BytesContainsAutoSkipCFA` fixture

**Reuse**: `isPackageFuncInSet()` in `analyzer_cast_validation.go`

---

### 2. `--check-all` Exclusion Tests (test gap, low complexity)

**Problem**: Existing tests verify `auditExceptions` and `suggestValidateAll` exclusion
separately, but `checkUseBeforeValidateCross`, `checkEnumSync`, and `auditReviewDates`
exclusions have no explicit tests.

**Changes**:
- `integration_test.go`:
  - Consolidate the two existing exclusion subtests into a single
    `"check-all does NOT enable opt-in and audit modes"` subtest asserting ALL
    five excluded modes: `suggestValidateAll`, `checkUseBeforeValidateCross`,
    `checkEnumSync`, `auditExceptions`, `auditReviewDates`

---

### 3. Cross-Block UBV Mutual Exclusion Test (test gap, low complexity)

**Problem**: Same-block UBV priority over cross-block UBV is only implicitly tested.
No test exercises a case where both same-block AND cross-block uses exist, verifying
only same-block fires.

**Changes**:
- `testdata/src/use_before_validate_cross/use_before_validate_cross.go`:
  - Add `SameBlockPriorityWithCrossBlockUse`: cast + same-block use + successor
    block use + validate after both. `want` annotation: `"used before Validate()
    in same block"` (NOT `"across blocks"`). The `analysistest` framework ensures
    cross-block UBV does NOT fire.

---

### 4. IIFE Validate Recognition in CFA (FP fix, medium complexity)

**Problem**: `containsValidateCall` skips ALL non-deferred closures. Immediately-invoked
function expressions (IIFEs) like `func() { x.Validate() }()` execute synchronously --
they have the same execution guarantee as inline code. Currently false positives.

**Changes**:
- `cfa.go`:
  - Add `collectImmediateClosureLits(body) map[*ast.FuncLit]bool` -- scans for
    `*ast.CallExpr` whose `Fun` is `*ast.FuncLit` NOT under `*ast.GoStmt`/`*ast.DeferStmt`.
    Strategy: first pass collects go/defer FuncLits, second pass marks IIFEs.
  - Update doc comment on `containsValidateCall` to document that `deferredLits`
    includes both deferred AND immediately-invoked closures.
- `cfa_cast_validation.go` (lines 69-72): Merge IIFE closures into syncLits map.
- `cfa_closure.go` (lines 56-59): Same merge pattern.
- `cfa_test.go`: Unit tests for `collectImmediateClosureLits` (IIFE, goroutine,
  defer, mixed, nil body).
- `testdata/src/cfa_castvalidation/`: Add `IIFEValidateCoversOuter` (NOT flagged)
  and `IIFEWithoutValidate` (FLAGGED).

**Compartmentalization**: All new code in CFA files. No AST mode changes.

---

### 5. Constructor-Validates CFA Multi-Path Checking (FN fix, medium-high complexity)

**Problem**: `inspectConstructorValidates` uses `bodyCallsValidateOnType` which finds
Validate() ANYWHERE in the body. A constructor that validates on only ONE return path
is not flagged:
```go
func NewFoo(name string, fast bool) (*Foo, error) {
    f := &Foo{Name: Name(name)}
    if fast {
        return f, nil  // unvalidated return!
    }
    return f, f.Validate()
}
```

**Changes**:
- `analyzer_constructor_validates.go`:
  - Add `noCFA bool` parameter to `inspectConstructorValidates`
  - CFA logic: build CFG, collect syncLits, DFS from entry checking ALL return paths
    for `.Validate()` on return type. If unvalidated path found, still check transitive
    (for delegated helpers). If both fail, flag.
  - Add `constructorHasUnvalidatedReturnPath(pass, body, returnTypeName) bool`
  - Add `blockContainsValidateOnType(pass, block, returnTypeName, syncLits) bool`
- `analyzer.go` (line 422): Pass `rc.noCFA` to `inspectConstructorValidates`
- `testdata/src/constructorvalidates/`: Add `MultiPath` struct, `NewMultiPath`
  (flagged by CFA), `NewMultiPathAllPaths` (NOT flagged)

**Import**: `analyzer_constructor_validates.go` needs `gocfg "golang.org/x/tools/go/cfg"`.
Architecturally sound -- reuses general CFA utilities from `cfa.go`.

---

### 6. Documentation & Memory Updates

- `tools/goplint/CLAUDE.md`: CFA section (IIFE), auto-skip list (`bytes.*`),
  constructor-validates section (CFA multi-path), gotchas
- Memory files: Update goplint patterns

---

## Implementation Order

1. Improvement 1 (bytes auto-skip) -- standalone
2. Improvement 2 (check-all exclusion tests) -- standalone
3. Improvement 3 (UBV mutual exclusion test) -- standalone
4. Improvement 4 (IIFE validate recognition) -- CFA, before #5
5. Improvement 5 (constructor-validates CFA) -- depends on #4
6. Improvement 6 (documentation) -- after all code

## Verification

```bash
# After each improvement:
cd tools/goplint && go test ./goplint/
cd tools/goplint && go test -race ./goplint/

# After all improvements:
make check-baseline
make check-types-all
make lint
make test
```

## Critical Files

| File | Changes |
|------|---------|
| `tools/goplint/goplint/analyzer_cast_validation.go` | bytes auto-skip |
| `tools/goplint/goplint/cfa.go` | IIFE collection, doc updates |
| `tools/goplint/goplint/cfa_cast_validation.go` | Merge IIFE closures |
| `tools/goplint/goplint/cfa_closure.go` | Merge IIFE closures |
| `tools/goplint/goplint/analyzer_constructor_validates.go` | CFA multi-path |
| `tools/goplint/goplint/analyzer.go` | Pass noCFA |
| `tools/goplint/goplint/integration_test.go` | Exclusion tests |
| `tools/goplint/goplint/cfa_test.go` | IIFE unit tests |
| `testdata/src/castvalidation/castvalidation.go` | bytes fixture |
| `testdata/src/cfa_castvalidation/cfa_castvalidation.go` | bytes + IIFE |
| `testdata/src/use_before_validate_cross/` | UBV mutual exclusion |
| `testdata/src/constructorvalidates/` | Multi-path constructor |
