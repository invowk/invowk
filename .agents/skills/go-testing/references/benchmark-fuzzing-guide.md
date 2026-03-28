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

```bash
# Run all benchmarks in a package
go test -bench=. ./internal/benchmark/...

# Run specific benchmark
go test -bench=BenchmarkParseCUE ./pkg/cueutil/...

# Skip regular tests, run only benchmarks
go test -run=^$ -bench=. ./...

# Benchmark with memory profiling
go test -bench=BenchmarkParseCUE -benchmem ./pkg/cueutil/...

# Benchmark for specific duration
go test -bench=. -benchtime=5s ./...

# Benchmark for specific iteration count
go test -bench=. -benchtime=100x ./...

# Benchmark smoke test (CI: verify benchmarks compile and run)
go test -run=^$ -bench=. -benchtime=1x -short -count=1 ./...
```

### Benchmark Output Format

```
BenchmarkParseCUE-8    50000    23456 ns/op    4096 B/op    42 allocs/op
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

The project uses Profile-Guided Optimization with `default.pgo` in the repository
root. Benchmark profiles feed PGO:

```bash
# Generate PGO profile from benchmarks
make pgo-profile-parse-discovery

# Profile without previous PGO bias
go test -bench=. -cpuprofile=default.pgo -pgo=off ./internal/benchmark/...
```

Key commands:
- `make pgo-profile` — full profile (includes container benchmarks)
- `make pgo-profile-short` — skip container benchmarks
- `make pgo-profile-parse-discovery` — focused on parser/discovery hot paths
- `make pgo-audit` — validate profile freshness

## Fuzzing API

### Writing a Fuzz Test

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

- Seed corpus: `f.Add()` calls in test code
- Generated corpus: `testdata/fuzz/FuzzName/` (auto-created by fuzzer)
- Crashing inputs: `testdata/fuzz/FuzzName/` (saved on failure)
- Commit the corpus: generated failing inputs become regression tests

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

- Only supports `string`, `[]byte`, `int`, `uint`, `float32`, `float64`, `bool`,
  and `rune` as fuzz parameters (no structs or slices of structs)
- Cannot fuzz multiple packages simultaneously
- Fuzzing with `-race` is very slow (2-20x per-iteration overhead)
- Corpus files are plain text; binary data is hex-encoded
