# Go Vet Analyzers — Complete Reference

## Overview

`go vet` runs a suite of static analyzers on Go source code. Unlike the race
detector (runtime), `go vet` is purely static — it analyzes code without executing
it.

`go test` runs only a high-confidence subset before running the test binary:
`atomic`, `bools`, `buildtag`, `directive`, `errorsas`, `ifaceassert`, `nilfunc`,
`printf`, `stdversion`, `stringintconv`, `tests` (Go 1.27 `go help test`). The
test-critical analyzers below (`testinggoroutine`, `copylocks`, `lostcancel`,
`waitgroup`) run only under `go vet`, `go test -vet=all`, or golangci-lint's
`govet` (`make lint`).

## Analyzers Relevant to Test Code

### Critical for Tests

#### `testinggoroutine`

Detects calls to `(*testing.T).Fatal`, `(*testing.T).FailNow`, and similar
methods from goroutines other than the test goroutine.

```go
// FLAGGED: t.Fatal calls runtime.Goexit, which only exits THIS goroutine
func TestBad(t *testing.T) {
    go func() {
        t.Fatal("boom") // vet: call to (*T).Fatal from a non-test goroutine
    }()
}
```

**Fix**: Use `t.Error` + channel to communicate failures, or restructure to
avoid `Fatal` in goroutines.

#### `copylocks`

Detects copying of values containing `sync.Mutex`, `sync.RWMutex`, `sync.WaitGroup`,
or other types with a `Lock` method.

```go
// FLAGGED: mutex is copied by value
func TestBad(t *testing.T) {
    var mu sync.Mutex
    mu2 := mu // vet: assignment copies lock value
}
```

**Test relevance**: commonly triggered when passing test helper structs by value
instead of by pointer.

#### `lostcancel`

Detects `context.WithCancel`/`WithTimeout`/`WithDeadline` calls where the
cancel function is never called on all paths.

```go
// FLAGGED: cancel function never called
func TestBad(t *testing.T) {
    ctx, _ := context.WithCancel(t.Context()) // vet: the cancel function is not used
    doSomething(ctx)
}
```

**Fix**: Always `defer cancel()` immediately after creation.

#### `waitgroup`

Detects `sync.WaitGroup.Add` called inside the goroutine it's synchronizing
(instead of before the goroutine launch).

```go
// FLAGGED: Add called in goroutine — race with Wait
var wg sync.WaitGroup
go func() {
    wg.Add(1) // vet: WaitGroup.Add called inside goroutine
    defer wg.Done()
    // ...
}()
wg.Wait()
```

**Fix**: Call `wg.Add(1)` before the `go` statement.

### Important for Tests

#### `loopclosure`

Reports nothing for this repo: since Go 1.22 semantics (`go.mod` is 1.27), each
iteration has a fresh loop variable, so closures and goroutines cannot capture a
stale value.

#### `printf`

Detects format string mismatches in `fmt.Printf`, `t.Errorf`, `t.Fatalf`, etc.

```go
t.Errorf("expected %d, got %s", "hello", 42) // vet: wrong arg types
```

#### `atomic`

Detects assigning the result of an `atomic` add back to the same variable.

```go
var count int64
count = atomic.AddInt64(&count, 1) // vet: direct assignment to atomic value
```

### Structural Analyzers

| Analyzer | Flags | Example |
|----------|-------|---------|
| `composites` | Unkeyed composite literals of struct types **imported from another package** (local types are not flagged) | `pkg.Result{42, "ok"}` → `pkg.Result{Code: 42, Message: "ok"}` |
| `structtag` | Malformed struct field tags | ``Name string `json:name` `` → `` `json:"name"` `` |
| `assign` | Useless self-assignments | `x = x` |
| `unreachable` | Code after `return`, `panic`, etc. | — |
| `unmarshal` | Non-pointer passed to `json.Unmarshal` and similar | `json.Unmarshal(data, result)` → `&result` |

### Low-Priority Analyzers

| Analyzer | Purpose |
|----------|---------|
| `appends` | Detects `append` calls with only one argument |
| `asmdecl` | Assembly/Go function declaration mismatches |
| `bools` | Detects boolean expression mistakes |
| `buildtag` | Validates `//go:build` lines |
| `cgocall` | Detects passing Go pointers to C |
| `defers` | Detects defer pitfalls |
| `directive` | Validates Go tool directives |
| `errorsas` | Validates `errors.As` target type |
| `framepointer` | Assembly frame pointer checks |
| `hostport` | Detects address strings built with `fmt.Sprintf` that break IPv6 (use `net.JoinHostPort`) |
| `httpresponse` | Detects `http.Response.Body` leak |
| `ifaceassert` | Detects impossible interface assertions |
| `nilfunc` | Detects comparison of function to nil |
| `shift` | Detects shifts equal to or exceeding width |
| `sigchanyzer` | Detects `signal.Notify` with unbuffered channel |
| `slog` | Validates `slog` key-value pairs |
| `stdmethods` | Detects misspelled standard method names |
| `stdversion` | Detects use of std symbols newer than the file's Go version |
| `stringintconv` | Detects `string(int)` conversions |
| `tests` | Validates `Test*` function signatures |
| `timeformat` | Detects wrong time format strings |
| `unsafeptr` | Detects misuse of `unsafe.Pointer` |
| `unusedresult` | Detects discarded results of certain functions |

## Running `go vet`

```bash
# Run all analyzers on all packages
go vet ./...

# Run one analyzer
go vet -testinggoroutine ./...

# go test runs only the high-confidence subset; run every analyzer with
go test -vet=all ./...

# -vettool substitutes a different analysis binary (not a selector)
go vet -vettool=/path/to/analyzer ./...

# Disable vet during test (not recommended)
go test -vet=off ./...
```

## Relationship with `golangci-lint`

The project uses `golangci-lint` (configured in `.golangci.toml`) which bundles
`go vet` analyzers along with many additional linters. Running `make lint` covers
all `go vet` checks plus project-specific linters.

Key additional linters relevant to tests (`.golangci.toml`, `default = "none"`
plus an explicit list):
- `paralleltest` — missing `t.Parallel()` calls in tests and subtests
- `tparallel` — `t.Parallel()` placement and parent/subtest consistency
- `thelper` — helpers that do not begin with `t.Helper()` (`begin = true`;
  parameter name and position are not enforced)
- `usetesting` — configured for contexts only (`context-background = true`,
  `context-todo = true`): flags `context.Background()`/`context.TODO()` in tests,
  so a structurally forced use needs `//nolint:usetesting // <reason>`
- `noctx` — `http.Get` without context (should use `http.NewRequestWithContext`)
- `govet` — golangci-lint wrapper for `go vet`, with `shadow` also enabled
- Disabled for `_test.go` files: `errcheck`, `forcetypeassert`, `contextcheck`,
  `goconst`, `forbidigo`, `errchkjson`, `dupword`, `makezero`; `gosec` skips
  tests (`tests = false`)
- `testifylint` is not enabled; the repo does not use testify directly
