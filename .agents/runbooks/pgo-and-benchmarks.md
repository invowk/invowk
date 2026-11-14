# PGO and Benchmarks

Procedures for the profile-guided optimization profile and Bencher benchmark data. Bencher CI, thresholds and alerts: `.agents/skills/bencher/`.

## PGO profile

- `default.pgo` lives in the repository root, is committed, and is picked up automatically by the Go toolchain. Benchmark source: `internal/benchmark/benchmark_test.go`.
- Targets:
  - `make pgo-profile`: full profile (includes container benchmarks)
  - `make pgo-profile-short`: passes `-short`. Every benchmark in `internal/benchmark` currently calls `b.Skip` under `testing.Short()`, so this target trains on no benchmark workload; do not use it to regenerate `default.pgo` until that changes (the Makefile help and `README.md` still describe it as "no container benchmarks").
  - `make pgo-profile-parse-discovery`: focused parser/discovery hot-path profile
  - `make pgo-audit`: validates profile freshness and required symbols
- Profile generation trains with `-pgo=off` so the new profile is not biased by the committed one.
- Regenerate after major hot-path changes (CUE parsing, runtime execution, discovery), when adding or changing runtimes, and before a major release.
- Never hand-edit `default.pgo`; regenerate it with the Make targets.

Staleness automation, three layers:
1. A Claude Code PostToolUse hook (`.claude/settings.json`) warns when a non-test file in `pkg/cueutil/`, `pkg/invowkfile/`, `pkg/invowkmod/` or `internal/discovery/` is edited. Another PostToolUse hook flags edits to `default.pgo`/`go.sum` after the fact; it cannot block them. Both are Claude Code only.
2. The pre-commit `pgo-staleness` hook warns (advisory, exit 0) when those hot-path files are staged without `default.pgo`.
3. The CI `pgo-sanity` job regenerates the profile and pushes a fix commit to the PR branch.

Prefer local regeneration when warned: `make pgo-profile-parse-discovery && git add default.pgo`.

## Benchmark data (Bencher Metric Format)

- `make bench-bmf`: startup + `internal/benchmark` BMF JSON, short mode, no container benchmarks.
- `make bench-bmf-full`: the same in full mode, including container benchmarks.
- Outputs: `artifacts/benchmarks/invowk.bmf.json` (BMF JSON) and `artifacts/benchmarks/go-bench.txt` (raw `go test -bench` output).
- Ad-hoc hot-path snapshot (benchmarks skip under `-short`):

```bash
go test -run='^$' -bench='^Benchmark(CUEParsing|Discovery|FullPipeline)$' -benchtime=1s ./internal/benchmark/
```
