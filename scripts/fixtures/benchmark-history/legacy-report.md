# Invowk Benchmark Report

> Generated at: 2026-01-05 00:00:00 UTC

## Run Metadata

| Field | Value |
|---|---|
| Mode | `short` |
| Startup Samples | `40` per scenario |
| Go Benchmark Count | `5` |
| Branch | `main` |
| Commit | `legacy1` |
| Platform | `Linux 5.15 x86_64` |
| Go Version | `go version go1.25.0 linux/amd64` |
| Binary | `./bin/invowk` |

## Startup Timings

| Scenario | Mean (ms) | Min (ms) | Max (ms) | Samples |
|---|---:|---:|---:|---:|
| Version (--version) | 14.00 | 13.00 | 15.00 | 40 |
| Cmd List (cmd) | 55.00 | 53.00 | 58.00 | 40 |

## Go Benchmarks (`internal/benchmark`)

| Benchmark | Samples | Mean ns/op | Min ns/op | Max ns/op | Mean ms/op | Mean iters/run | Est run (ms) | Est total (s) | Mean B/op | Mean allocs/op |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| `BenchmarkCUEParsing-24` | 5 | 200.00 | 190.00 | 210.00 | 0.000200 | 1000.00 | 1.000 | 5.000000 | 35.00 | 3.00 |

## Raw Startup Timing Data

```text
Version (--version)	14.00	13.00	15.00	40
Cmd List (cmd)	55.00	53.00	58.00	40
```

## Raw Go Benchmark Output

```text
goos: linux
goarch: amd64
cpu: Legacy Fixture CPU
BenchmarkCUEParsing-24 5 200 ns/op 35 B/op 3 allocs/op
PASS
```
