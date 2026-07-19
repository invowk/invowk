# Performance Threshold Calibration

Date: 2026-07-15

## Environment

- Base commit: `c544dfc81cb486d1c17949cee92b82bc9cda3910`
- Combined state: base commit plus the active development diff
- Go toolchain: `go1.26.5 linux/amd64`
- Kernel: `Linux 7.1.3-200.fc44.x86_64 x86_64`
- CPU: `Intel(R) Core(TM) Ultra 9 285K`
- Available Go CPUs: 24
- Physical memory: 100542705664 bytes
- Threshold runner class: `fedora-44-x86_64-24cpu-reviewed`
- Samples per metric: 5

## Command

```text
make check-goplint-soundness
```

The aggregate run passed production integration, counterexamples,
architecture absence, catalog completeness, the component and end-to-end
oracles, deterministic fuzz seeds, refinement, real determinism, all 18 causal
mutants, race/repeat, and the baseline-backed full scan before running the
five-sample benchmark threshold stage.

## Median results

| Surface | Time | Bytes/op or peak bytes | Allocs/op | Other |
|---|---:|---:|---:|---|
| Canonical solver | 45915 ns/op | 37662 B/op | 512 | — |
| Recursive tabulation | 63464 ns/op | 50980 B/op | 702 | 5 path edges/op; 1 summary reuse/op |
| Alias join | 12952 ns/op | 11424 B/op | 98 | — |
| Refinement evidence | 10097 ns/op | 11120 B/op | 18 | — |
| Generated graphs | 4153505 ns/op | 2060667 B/op | 48837 | 41 graphs/op |
| Repository full scan | 221800 ms | 4110467072 peak bytes | — | canonical production packages |

All analyzer microbenchmarks and the repository peak-memory threshold passed.
The prior 120000 ms repository wall-time threshold failed because it was below
the fresh five-run median of the completed corrective analyzer. The reviewed
replacement is 300000 ms: 35 percent above the observed median while retaining
a finite blocking regression bound. The memory limit remains 6442450944 bytes,
57 percent above the observed median. No semantic selection, package scope,
sample count, or benchmark workload was reduced during recalibration.

## Recalibration verification

After updating only the reviewed runner class and repository wall-time limit,
`make check-goplint-benchmarks` passed all five samples and every blocking
threshold. The verification medians were 43513 ns/op for the canonical solver,
60562 ns/op for recursive tabulation, 12894 ns/op for alias join, 9871 ns/op
for refinement evidence, and 4005633 ns/op for generated graphs. Repository
wall time remained within 300000 ms and peak memory remained within
6442450944 bytes. The final synthetic clean-tree evidence retains the complete
command outcome and log digest for the aggregate gate.

The final clean-tree aggregate run recorded medians of 43099 ns/op for the
canonical solver, 58813 ns/op for recursive tabulation, 13121 ns/op for alias
join, 9953 ns/op for refinement evidence, and 4125263 ns/op for generated
graphs. The canonical repository full-scan median was 227940 ms with
4159369216 peak bytes. Every value passed its checked-in threshold.
