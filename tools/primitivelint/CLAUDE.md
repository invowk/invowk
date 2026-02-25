# primitivelint

A standalone `go/analysis` analyzer that detects bare primitive types (`string`, `int`, `float64`, `[]string`, `map[string]string`, etc.) in struct fields, function parameters, and return types. Enforces the project's DDD Value Type convention where named types (e.g., `type CommandName string`) should be used instead of raw primitives.

## Purpose

Replaces the manual full-codebase scan that agents performed via `/improve-type-system`. Instead of reading every Go file, agents can run `make check-types-json` to get a structured list of remaining primitive usages.

## Quick Reference

| Task | Command |
|------|---------|
| Build | `make build-primitivelint` |
| Run (human output) | `make check-types` |
| Run (JSON for agents) | `make check-types-json` |
| Run tests | `cd tools/primitivelint && go test ./primitivelint/` |
| Run tests (race) | `cd tools/primitivelint && go test -race ./primitivelint/` |

## Architecture

```
tools/primitivelint/
├── main.go                 # singlechecker entry point
├── exceptions.toml         # ~80 intentional primitive exceptions
├── primitivelint/
│   ├── analyzer.go         # analysis.Analyzer + run() wiring
│   ├── config.go           # TOML loading + pattern matching
│   ├── inspect.go          # struct/func AST visitors + helpers
│   ├── typecheck.go        # isPrimitive() type resolution
│   ├── *_test.go           # unit + integration tests
│   └── testdata/src/       # analysistest fixture packages
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

### 2. Inline Directives — fallback for one-offs

```go
type Foo struct {
    Bar string //primitivelint:ignore -- display-only
    Baz int    //nolint:primitivelint
}
```

## Gotchas

- **`types.Alias` (Go 1.22+)**: Type aliases (`type X = string`) are transparent — `isPrimitive` must call `types.Unalias()` to resolve them. Without this, aliases silently pass the linter.
- **Generic pointer receivers**: `*Container[T]` is `StarExpr{X: IndexExpr{...}}` in the AST. `receiverTypeName` must recurse through `StarExpr` to find the type name inside `IndexExpr`. A naive `StarExpr → Ident` check misses this.
- **`configPath` is global mutable state**: The `-config` flag sets a package-level `var`. Integration tests that modify it must NOT use `t.Parallel()` — they share the variable across all analyzer invocations in the process.
- **`primitiveTypeName` needs `Unalias` too**: Even after `isPrimitive` correctly detects an alias as primitive, the diagnostic message must show the resolved type (`string`), not the alias name (`MyAlias`). Call `types.Unalias()` before `types.TypeString()`.
- **Qualified name format**: The analyzer prefixes all names with the package name (`pkg.Type.Field`, `pkg.Func.param`). Exception patterns can be 2-segment (matched after stripping the package prefix) or 3-segment (exact match).
- **CI is advisory**: The `primitivelint` job in `lint.yml` uses `continue-on-error: true` during rollout. It does not block PRs.

## Test Architecture

- **Unit tests** (`config_test.go`, `typecheck_test.go`, `inspect_test.go`): White-box (same package), test all helper functions in isolation
- **E2E analysistest** (`analyzer_test.go`): Runs analyzer against 10 fixture packages in `testdata/src/`
- **Integration tests** (`integration_test.go`): Exercises full pipeline with TOML config loaded; NOT parallel due to `configPath` global state
