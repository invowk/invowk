# Benchmark and Fuzzing Guide

## Benchmark API

### The `b.Loop()` Pattern (Go 1.24+)

Go 1.24 introduced `b.Loop()` as the preferred iteration mechanism, replacing the
traditional `for i := 0; i < b.N; i++` loop. The compiler cannot optimize away
code inside `b.Loop()`, making benchmarks more reliable.

```go
// Go 1.24+ PREFERRED
func BenchmarkFoo(b *testing.B) {
    b.ReportAllocs()
    for b.Loop() {
        result := doWork()
        _ = result // prevent compiler optimization
    }
}

// LEGACY (still works, but less reliable for trivial benchmarks)
func BenchmarkFoo(b *testing.B) {
    b.ReportAllocs()
    for i := 0; i < b.N; i++ {
        result := doWork()
        _ = result
    }
}
```

### Key Benchmark Methods

| Method | Purpose | When to Use |
|--------|---------|-------------|
| `b.ReportAllocs()` | Include alloc stats | Call once before loop; always use |
| `b.ResetTimer()` | Reset after expensive setup | After fixture creation |
| `b.StopTimer()` / `b.StartTimer()` | Pause timing | Around non-benchmarked setup within loop |
| `b.RunParallel(body)` | Parallel benchmark | For concurrency-sensitive code |
| `b.SetParallelism(p)` | Set goroutine count for RunParallel | Default: `GOMAXPROCS`; multiply by p |
| `b.ReportMetric(n, unit)` | Custom metrics | `b.ReportMetric(float64(size), "bytes/op")` |
| `b.Elapsed()` | Time since timer start | Useful with `b.StopTimer` |
| `b.Context()` | Benchmark context | Cancelled when benchmark ends |

### Running Benchmarks

The repo's benchmarks live in `./internal/benchmark` (for example
`BenchmarkCUEParsing`, `BenchmarkDiscovery`, `BenchmarkFullPipeline`), and they
skip under `testing.Short()`: never pass `-short` when you want them to run.

```bash
# Run all benchmarks in the package, skipping regular tests
go test -run=^$ -bench=. ./internal/benchmark/

# Run specific benchmark with memory stats
go test -run=^$ -bench='^BenchmarkCUEParsing$' -benchmem ./internal/benchmark/

# Benchmark for specific duration / iteration count
go test -run=^$ -bench=. -benchtime=5s ./internal/benchmark/
go test -run=^$ -bench=. -benchtime=100x ./internal/benchmark/

# Benchmark smoke test (ci.yml form with a shortened regex; ci.yml lists more
# benchmarks in BENCH_REGEX): verify benchmarks compile and run
go test ./internal/benchmark -run=^$ -bench='^Benchmark(CUEParsing|Discovery.*|FullPipeline)$' -benchmem -benchtime=1x -count=1
```

### Benchmark Output Format

```
BenchmarkCUEParsing-8  50000    23456 ns/op    4096 B/op    42 allocs/op
```

Fields: `Name-GOMAXPROCS  Iterations  ns/op  B/op  allocs/op`

### Parallel Benchmarks

Use `b.RunParallel` to benchmark code under concurrent load:

```go
func BenchmarkConcurrentLookup(b *testing.B) {
    cache := NewCache()
    b.ResetTimer()
    b.RunParallel(func(pb *testing.PB) {
        for pb.Next() {
            cache.Get("key")
        }
    })
}
```

### PGO Integration

Benchmark profiles feed `default.pgo` in the repository root. Regenerate it only
through the Make targets (`make pgo-profile`, `make pgo-profile-short`,
`make pgo-profile-parse-discovery`, `make pgo-audit`); do not write
`default.pgo` with a hand-rolled `-cpuprofile` command. Procedure:
`.agents/runbooks/pgo-and-benchmarks.md`.

## Fuzzing API

### Writing a Fuzz Test

The repo has no `Fuzz*` targets today; `FuzzParseVersion`, `ParseVersion` and
`./pkg/types` below are illustrative names.

```go
func FuzzParseVersion(f *testing.F) {
    // Seed corpus — known-good inputs
    f.Add("1.0.0")
    f.Add("0.1.0-alpha.1")
    f.Add("2.0.0-rc.1+build.123")

    f.Fuzz(func(t *testing.T, input string) {
        result, err := ParseVersion(input)
        if err != nil {
            return // invalid input is expected; just don't crash
        }
        // Property-based testing: round-trip
        if result.String() != input {
            t.Errorf("round-trip failed: %q -> %q", input, result.String())
        }
    })
}
```

### Running Fuzz Tests

```bash
# Fuzz for 30 seconds
go test -fuzz=FuzzParseVersion -fuzztime=30s ./pkg/types/...

# Fuzz for 1000 iterations
go test -fuzz=FuzzParseVersion -fuzztime=1000x ./pkg/types/...

# Fuzz with race detector
go test -fuzz=FuzzParseVersion -fuzztime=30s -race ./pkg/types/...

# Run fuzz corpus as regression tests (no fuzzing)
go test -run=FuzzParseVersion ./pkg/types/...
```

### Corpus Management

- Seed corpus: `f.Add()` calls in test code, plus files you add under
  `testdata/fuzz/<FuzzName>/`.
- Generated corpus: kept in the build cache (`$GOCACHE/fuzz`), not in the repo.
- Failing inputs: written to `testdata/fuzz/<FuzzName>/` on failure; plain
  `go test` then replays them.
- Commit failing inputs and useful seeds as regression tests; do not commit the
  generated cache corpus.

### Fuzz Test Design Patterns

**1. Round-trip testing**: encode → decode → compare
```go
f.Fuzz(func(t *testing.T, data []byte) {
    encoded := encode(data)
    decoded, err := decode(encoded)
    if err != nil {
        t.Fatalf("round-trip failed: %v", err)
    }
    if !bytes.Equal(data, decoded) {
        t.Error("data mismatch")
    }
})
```

**2. Differential testing**: compare two implementations
```go
f.Fuzz(func(t *testing.T, input string) {
    r1 := implOld(input)
    r2 := implNew(input)
    if r1 != r2 {
        t.Errorf("divergence on %q: old=%v new=%v", input, r1, r2)
    }
})
```

**3. Crash-only testing**: just don't panic
```go
f.Fuzz(func(t *testing.T, input []byte) {
    _ = parseUntrusted(input) // should never panic
})
```

### Fuzzing Limitations

- Allowed fuzz parameter types (`go doc testing.F.Fuzz`): `[]byte`, `string`,
  `bool`, `byte`, `rune`, `float32`, `float64`, `int`, `int8`, `int16`, `int32`,
  `int64`, `uint`, `uint8`, `uint16`, `uint32`, `uint64` (no structs or slices
  of structs).
- Cannot fuzz multiple packages simultaneously (`-fuzz` needs a single package).
- Fuzzing with `-race` is very slow (2-20x per-iteration overhead).
- Corpus files use the `go test fuzz v1` format: one Go literal per argument
  (for example `string("abc")`, `[]byte("\x00")`), not hex dumps.
