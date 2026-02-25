# primitivelint

A standalone `go/analysis` analyzer that detects bare primitive types (`string`, `int`, `float64`, `[]string`, `map[string]string`, etc.) in struct fields, function parameters, and return types. Enforces the project's DDD Value Type convention where named types (e.g., `type CommandName string`) should be used instead of raw primitives.

## Purpose

Replaces the manual full-codebase scan that agents performed via `/improve-type-system`. Instead of reading every Go file, agents can run `make check-types-all-json` to get a structured JSON report of all DDD compliance gaps.

## Quick Reference

| Task | Command |
|------|---------|
| Build | `make build-primitivelint` |
| Run (human output) | `make check-types` |
| Run (JSON for agents) | `make check-types-json` |
| **Run all DDD checks** | **`make check-types-all`** |
| **Run all DDD checks (JSON)** | **`make check-types-all-json`** |
| **Check baseline (regression gate)** | **`make check-baseline`** |
| **Update baseline** | **`make update-baseline`** |
| Run tests | `cd tools/primitivelint && go test ./primitivelint/` |
| Run tests (race) | `cd tools/primitivelint && go test -race ./primitivelint/` |
| Audit stale exceptions | `make build-primitivelint && ./bin/primitivelint -audit-exceptions -config=tools/primitivelint/exceptions.toml ./...` |
| Check missing IsValid | `make build-primitivelint && ./bin/primitivelint -check-isvalid -config=tools/primitivelint/exceptions.toml ./...` |
| Check missing String | `make build-primitivelint && ./bin/primitivelint -check-stringer -config=tools/primitivelint/exceptions.toml ./...` |
| Check missing constructors | `make build-primitivelint && ./bin/primitivelint -check-constructors -config=tools/primitivelint/exceptions.toml ./...` |
| Check constructor signatures | `make build-primitivelint && ./bin/primitivelint -check-constructor-sig -config=tools/primitivelint/exceptions.toml ./...` |
| Check functional options | `make build-primitivelint && ./bin/primitivelint -check-func-options -config=tools/primitivelint/exceptions.toml ./...` |
| Check immutability | `make build-primitivelint && ./bin/primitivelint -check-immutability -config=tools/primitivelint/exceptions.toml ./...` |

## Diagnostic Categories

Each diagnostic emitted by the analyzer carries a `category` field (visible in `-json` output). Agents should use this for programmatic filtering:

| Category | Flag | Description |
|----------|------|-------------|
| `primitive` | (always active) | Bare primitive in struct field / function param / return type |
| `missing-isvalid` | `--check-isvalid` or `--check-all` | Named type missing `IsValid()` method |
| `missing-stringer` | `--check-stringer` or `--check-all` | Named type missing `String()` method |
| `missing-constructor` | `--check-constructors` or `--check-all` | Exported struct missing `NewXxx()` constructor |
| `wrong-constructor-sig` | `--check-constructor-sig` or `--check-all` | Constructor `NewXxx()` returns wrong type |
| `missing-func-options` | `--check-func-options` or `--check-all` | Struct should use or complete functional options |
| `missing-immutability` | `--check-immutability` or `--check-all` | Struct with constructor has exported mutable fields |
| `stale-exception` | `--audit-exceptions` | TOML exception pattern matched nothing |

The `--check-all` flag enables `--check-isvalid`, `--check-stringer`, `--check-constructors`, `--check-constructor-sig`, `--check-func-options`, and `--check-immutability` in a single invocation. It deliberately excludes `--audit-exceptions` which is a config maintenance tool with per-package false positives.

## Architecture

```
tools/primitivelint/
├── main.go                 # singlechecker entry point + --update-baseline mode
├── exceptions.toml         # ~85 intentional primitive exception patterns
├── baseline.toml           # accepted findings baseline (generated)
├── primitivelint/
│   ├── analyzer.go             # analysis.Analyzer + run() wiring + basic supplementary modes
│   ├── analyzer_structural.go  # structural analysis: constructor-sig, func-options, immutability
│   ├── baseline.go             # baseline TOML loading + matching + writing
│   ├── config.go               # exception TOML loading + pattern matching + match counting
│   ├── inspect.go              # struct/func AST visitors + helpers
│   ├── typecheck.go            # isPrimitive() / isPrimitiveUnderlying() / isOptionFuncType()
│   ├── *_test.go               # unit + integration tests
│   └── testdata/src/           # analysistest fixture packages (21 packages)
```

**Separate Go module**: `tools/primitivelint/` has its own `go.mod` to avoid adding `golang.org/x/tools` and `github.com/BurntSushi/toml` to the main project's dependencies.

## What Gets Flagged

- Struct fields: `Name string`, `Items []string`, `Data map[string]string`
- Function/method parameters: `func Foo(name string)`
- Function/method return types: `func Bar() (string, error)`

## What Does NOT Get Flagged

- **Named types**: `type CommandName string` — these ARE the DDD Value Types
- **`bool`**: Exempt by design decision — marginal DDD value
- **`[]byte`**: I/O boundary type, not a domain type
- **`error`**: Interface, not a primitive
- **Interface method signatures**: `type Service interface { ... }` — AST-level exclusion
- **`String()`/`Error()`/`GoString()`/`MarshalText()` returns**: Interface contract obligations
- **Test files**: `_test.go` files skipped entirely
- **`init()`/`main()`/`Test*`/`Benchmark*`/`Fuzz*`/`Example*`**: Skipped functions

## Exception Mechanism

Two layers, used together:

### 1. TOML Config (`exceptions.toml`) — primary

```toml
[settings]
skip_types = ["bool", "error", "context.Context", "any"]
exclude_paths = ["specs/", "internal/testutil/"]

[[exceptions]]
pattern = "ExecuteRequest.Name"
reason = "Cobra + interface boundary"

[[exceptions]]
pattern = "uroot.*.name"
reason = "display-only labels in 12+ unexported structs"
```

**Pattern syntax**: Dot-separated segments, `*` matches any single segment.
- `Type.Field` — 2-segment, matches after stripping package prefix
- `pkg.Type.Field` — 3-segment, exact match
- `*.Field` — wildcard, any type with that field name
- `pkg.*.*` — broad wildcard, all fields/params in a package

### 2. Inline Directives — fallback for one-offs

```go
type Foo struct {
    Bar string //primitivelint:ignore -- display-only
    Baz int    //nolint:primitivelint
}
```

## Supplementary Modes

Seven additional analysis modes complement the primary primitive detection:

### `--check-all`

Enables all DDD compliance checks (`--check-isvalid`, `--check-stringer`, `--check-constructors`, `--check-constructor-sig`, `--check-func-options`, `--check-immutability`) in a single invocation. This is the recommended flag for comprehensive DDD compliance checks. Deliberately excludes `--audit-exceptions` (a config maintenance tool with per-package false positives).

### `--audit-exceptions`

Reports exception patterns that matched **zero locations** in the current package's analysis pass. Detects stale entries that accumulate in `exceptions.toml` after refactors remove the code they excepted.

**Limitation**: Since `go/analysis` runs per-package, `--audit-exceptions` reports stale exceptions per-package. An exception matching in package A is reported as "stale" in package B. For a true global audit, pipe output through `sort -u` or look at the package where the exception is expected to match.

### `--check-isvalid`

Reports named non-struct types (`type Foo string`, `type Bar int`) that lack an `IsValid() (bool, []error)` method. Only checks types backed by primitives (string, int, bool, float). Skips struct types (which use composite `IsValid()` delegation), interface types, and type aliases (`type X = Y`, which inherit methods from the aliased type). For unexported types, also checks for `isValid()` (lowercase), matching the project convention.

### `--check-stringer`

Reports named non-struct types lacking a `String() string` method. Same scope as `--check-isvalid`. Recognizes both value and pointer receivers.

### `--check-constructors`

Reports **exported** struct types that have no `NewXxx()` constructor function in the same package. Unexported structs and non-struct types are skipped.

### `--check-constructor-sig`

Reports `NewXxx()` constructor functions whose return type does not match the struct they construct. For example, `NewConfig()` must return `*Config` or `Config` — returning `*Server` is flagged. Handles multi-return patterns like `(*Config, error)` by checking the first non-error return type. Skips interface return types (can't match to a struct name). Only checks constructors that exist — missing constructors are `--check-constructors`' concern.

### `--check-func-options`

Two sub-checks for the [functional options pattern](https://dave.cheney.net/2014/10/17/functional-options-for-friendly-apis):

**Detection**: Flags exported structs whose `NewXxx()` constructor has more than 3 non-option parameters. Suggests converting to functional options. Skips structs that already have an option type.

**Completeness**: For structs that already have a functional options type (`type XxxOption func(*Xxx)`), verifies:
- The constructor `NewXxx()` accepts `...XxxOption` as a variadic parameter
- Each unexported field has a corresponding `WithFieldName()` function

Option types are detected by function signature (`func(*TargetStruct)`), not naming convention, so both `type Option func(*Server)` and `type ServerOption func(*Server)` are recognized.

### `--check-immutability`

Reports exported struct fields on types that have a `NewXxx()` constructor. If a struct uses a constructor pattern, its fields should be unexported (accessed via getter methods). Each exported field is flagged individually. Structs without constructors are not checked (they may be DTOs/config types where exported fields are intentional).

### Exception integration

All supplementary modes respect the TOML exception config:
- `--check-isvalid`: excepted via `pkg.TypeName.IsValid`
- `--check-stringer`: excepted via `pkg.TypeName.String`
- `--check-constructors`: excepted via `pkg.StructName.constructor`
- `--check-constructor-sig`: excepted via `pkg.StructName.constructor-sig`
- `--check-func-options`: excepted via `pkg.StructName.func-options`
- `--check-immutability`: excepted via `pkg.StructName.immutability`

## Baseline Comparison

The baseline system prevents DDD compliance regressions. A committed `baseline.toml` records all accepted findings; only **new** findings (not in the baseline) are reported as errors.

### Usage

```bash
make check-baseline    # Compare current state against baseline (CI gate)
make update-baseline   # Regenerate baseline from current state
```

### How it works

- **`--baseline=path`**: Analyzer flag. Loaded per-package in `run()`, suppresses findings whose message matches a baseline entry. Only new findings are reported.
- **`--update-baseline=path`**: main() flag. Runs self as subprocess with `-json`, collects all findings, writes sorted TOML. Uses subprocess because `singlechecker.Main()` calls `os.Exit()` — no post-analysis aggregation is possible within the framework.

### Baseline TOML format

```toml
[primitive]
messages = [
    "struct field pkg.Foo.Bar uses primitive type string",
]

[missing-constructor]
messages = [
    "exported struct pkg.Config has no NewConfig() constructor",
]
```

Sections: `[primitive]`, `[missing-isvalid]`, `[missing-stringer]`, `[missing-constructor]`, `[wrong-constructor-sig]`, `[missing-func-options]`, `[missing-immutability]`. Empty sections are omitted. Messages sorted alphabetically for stable diffs.

### When to update

Run `make update-baseline` after:
- Converting bare primitives to DDD Value Types
- Adding new exceptions to `exceptions.toml`
- Intentionally adding code that uses primitives at boundaries

### CI integration

The `primitivelint-baseline` job in `lint.yml` runs `make check-baseline`. During rollout it uses `continue-on-error: true` (advisory). To promote to required: remove `continue-on-error` and add to branch protection.

### Pre-commit hook

The `primitivelint-baseline` local hook in `.pre-commit-config.yaml` runs `make check-baseline` advisory (always exits 0). Install with `make install-hooks`.

## Gotchas

- **`types.Alias` (Go 1.22+)**: Type aliases (`type X = string`) are transparent — `isPrimitive` must call `types.Unalias()` to resolve them. Without this, aliases silently pass the linter.
- **Generic pointer receivers**: `*Container[T]` is `StarExpr{X: IndexExpr{...}}` in the AST. `receiverTypeName` must recurse through `StarExpr` to find the type name inside `IndexExpr`. A naive `StarExpr → Ident` check misses this.
- **Flag binding variables**: The `-config` and supplementary mode flags are package-level variables bound via `BoolVar`/`StringVar` (required by the `go/analysis` framework). However, `run()` never reads or mutates these directly — it reads them once via `newRunConfig()` into a local `runConfig` struct, and the `--check-all` expansion happens on the local struct. Integration tests use `Analyzer.Flags.Set()` + `resetFlags()` instead of manual save/restore. Tests must NOT use `t.Parallel()` — they share the `Analyzer.Flags` FlagSet.
- **`primitiveTypeName` needs `Unalias` too**: Even after `isPrimitive` correctly detects an alias as primitive, the diagnostic message must show the resolved type (`string`), not the alias name (`MyAlias`). Call `types.Unalias()` before `types.TypeString()`.
- **Qualified name format**: The analyzer prefixes all names with the package name (`pkg.Type.Field`, `pkg.Func.param`). Exception patterns can be 2-segment (matched after stripping the package prefix) or 3-segment (exact match).
- **CI is advisory (rollout)**: Both `primitivelint` and `primitivelint-baseline` jobs in `lint.yml` use `continue-on-error: true` during rollout. To promote the baseline check to required: remove `continue-on-error` and add to branch protection.
- **Per-package execution**: `go/analysis` analyzers run per-package. `--audit-exceptions` reports stale exceptions per-package — an exception that matches in package A but not package B will only be reported as stale during B's analysis. For a global stale audit, run against the full module (`./...`).

## Test Architecture

- **Unit tests** (`config_test.go`, `typecheck_test.go`, `inspect_test.go`): White-box (same package), test all helper functions in isolation
- **E2E analysistest** (`analyzer_test.go`): Runs analyzer against 10 fixture packages in `testdata/src/`
- **Integration tests** (`integration_test.go`): Exercises full pipeline with TOML config loaded and supplementary modes; NOT parallel due to shared `Analyzer.Flags` state. Uses `setFlag()`/`resetFlags()` helpers for declarative flag management. Covers:
  - `TestAnalyzerWithConfig` — TOML exception patterns
  - `TestAnalyzerWithRealExceptionsToml` — real `exceptions.toml` parse validation
  - `TestCheckIsValid` — `--check-isvalid` mode
  - `TestCheckStringer` — `--check-stringer` mode
  - `TestCheckConstructors` — `--check-constructors` mode
  - `TestCheckConstructorSig` — `--check-constructor-sig` mode (wrong return types)
  - `TestCheckFuncOptions` — `--check-func-options` mode (detection + completeness)
  - `TestCheckImmutability` — `--check-immutability` mode (exported fields with constructor)
  - `TestAuditExceptions` — `--audit-exceptions` stale entry detection
  - `TestCheckAll` — `--check-all` combined mode (all 8 categories in one fixture)
  - `TestBaselineSuppression` — `--baseline` mode (known findings suppressed, new ones reported)
