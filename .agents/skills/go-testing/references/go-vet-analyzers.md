# Go Vet Analyzers — Complete Reference

## Overview

`go vet` runs a suite of static analyzers on Go source code. Unlike the race
detector (runtime), `go vet` is purely static — it analyzes code without executing
it. It runs automatically as part of `go test` (since Go 1.10).

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

Detects closures that capture loop variables. **Mitigated in Go 1.22+** (loop
variable semantics change), but still relevant for `go` statements inside loops
where the goroutine may execute after the loop advances.

```go
// Go < 1.22: FLAGGED. Go >= 1.22: safe for range loops, still risky for goroutines.
for _, v := range values {
    go func() {
        fmt.Println(v) // may capture stale value if goroutine runs late
    }()
}
```

#### `printf`

Detects format string mismatches in `fmt.Printf`, `t.Errorf`, `t.Fatalf`, etc.

```go
t.Errorf("expected %d, got %s", "hello", 42) // vet: wrong arg types
```

#### `atomic`

Detects non-atomic operations on `sync/atomic` values.

```go
var count int64
count = atomic.LoadInt64(&count) + 1 // vet: should use atomic.AddInt64
```

### Structural Analyzers

#### `composites`

Flags composite literals without field names.

```go
// FLAGGED: unkeyed struct literal
return Result{42, "ok", nil} // vet: unkeyed fields
// FIX:
return Result{Code: 42, Message: "ok", Error: nil}
```

#### `structtag`

Validates struct field tag syntax.

```go
type Config struct {
    Name string `json:name` // vet: struct field tag `json:name` not compatible
}
```

#### `assign`

Detects useless assignments (`x = x`).

#### `unreachable`

Detects unreachable code after `return`, `panic`, `os.Exit`, etc.

#### `unmarshal`

Detects passing non-pointer values to `json.Unmarshal` and similar.

```go
var result []string
json.Unmarshal(data, result) // vet: non-pointer passed to Unmarshal
```

### Low-Priority Analyzers

| Analyzer | Purpose |
|----------|---------|
| `asmdecl` | Assembly/Go function declaration mismatches |
| `bools` | Detects boolean expression mistakes |
| `buildtag` | Validates `//go:build` lines |
| `cgocall` | Detects passing Go pointers to C |
| `defers` | Detects defer pitfalls |
| `directive` | Validates Go tool directives |
| `errorsas` | Validates `errors.As` target type |
| `framepointer` | Assembly frame pointer checks |
| `httpresponse` | Detects `http.Response.Body` leak |
| `ifaceassert` | Detects impossible interface assertions |
| `nilfunc` | Detects comparison of function to nil |
| `shift` | Detects shifts equal to or exceeding width |
| `sigchanysize` | Detects `signal.Notify` with unbuffered channel |
| `slog` | Validates `slog` key-value pairs |
| `stdmethods` | Detects misspelled standard method names |
| `stringintconv` | Detects `string(int)` conversions |
| `tests` | Validates `Test*` function signatures |
| `timeformat` | Detects wrong time format strings |
| `unsafeptr` | Detects misuse of `unsafe.Pointer` |
| `unusedresult` | Detects discarded results of certain functions |

## Running `go vet`

```bash
# Run all analyzers on all packages
go vet ./...

# Run specific analyzer
go vet -vettool=$(which analysistool) ./...

# Run as part of go test (automatic)
go test ./...  # go vet runs implicitly

# Disable vet during test (not recommended)
go test -vet=off ./...
```

## Relationship with `golangci-lint`

The project uses `golangci-lint` (configured in `.golangci.toml`) which bundles
`go vet` analyzers along with many additional linters. Running `make lint` covers
all `go vet` checks plus project-specific linters.

Key additional linters relevant to tests:
- `tparallel` — missing `t.Parallel()` in subtests
- `testifylint` — testify assertion best practices (if testify were used)
- `noctx` — `http.Get` without context (should use `http.NewRequestWithContext`)
- `errcheck` — unchecked errors (including in test code)
- `govet` — golangci-lint wrapper for `go vet`
