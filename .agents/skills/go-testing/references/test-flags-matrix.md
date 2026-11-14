# Go Test Flags — Complete Matrix

## All `go test` Flags

### Test Selection

| Flag | Syntax | Default | Purpose |
|------|--------|---------|---------|
| `-run` | `-run <regex>` | `""` (all) | Run only tests/examples matching regex |
| `-skip` | `-skip <regex>` | `""` (none) | Skip tests/benchmarks/fuzz tests matching regex; split by `/` like `-run` |
| `-bench` | `-bench <regex>` | `""` (none) | Run benchmarks matching regex; `.` for all |
| `-fuzz` | `-fuzz <regex>` | `""` (none) | Run fuzz tests matching regex |
| `-list` | `-list <regex>` | — | List tests/benchmarks/examples; no execution |
| `-tags` | `-tags <list>` | `""` | Comma-separated build tags |

### Execution Control

| Flag | Syntax | Default | Purpose |
|------|--------|---------|---------|
| `-count` | `-count N` | `1` | Run each test N times; any explicit `-count` disables caching (idiom: `-count=1`) |
| `-parallel` | `-parallel N` | `GOMAXPROCS` | Max concurrent `t.Parallel()` tests per package |
| `-timeout` | `-timeout D` | `10m` | Kill test binary after duration (0 = no limit) |
| `-short` | `-short` | `false` | Set `testing.Short()` to true |
| `-v` | `-v` | `false` | Verbose output; show all test logs |
| `-failfast` | `-failfast` | `false` | Stop after first test failure |
| `-shuffle` | `-shuffle on\|off\|N` | `off` | Randomize test order; N is seed |

### Race Detection

| Flag | Syntax | Default | Purpose |
|------|--------|---------|---------|
| `-race` | `-race` | `false` | Enable race detector (implies `-covermode=atomic`) |

### Coverage

| Flag | Syntax | Default | Purpose |
|------|--------|---------|---------|
| `-cover` | `-cover` | `false` | Enable coverage analysis |
| `-coverprofile` | `-coverprofile <file>` | `""` | Write coverage profile to file |
| `-coverpkg` | `-coverpkg <patterns>` | test package | Comma-separated package patterns for coverage |
| `-covermode` | `-covermode set\|count\|atomic` | `set` | Coverage mode (forced to `atomic` with `-race`) |

### Output

| Flag | Syntax | Default | Purpose |
|------|--------|---------|---------|
| `-json` | `-json` | `false` | JSON-encoded output (one event per line) |
| `-o` | `-o <file>` | — | Save a copy of the test binary; the test still runs unless `-c` is given |
| `-c` | `-c` | `false` | Compile test binary but don't run |

### Benchmark Control

| Flag | Syntax | Default | Purpose |
|------|--------|---------|---------|
| `-benchtime` | `-benchtime D\|Nx` | `1s` | Duration or iteration count per benchmark |
| `-benchmem` | `-benchmem` | `false` | Report memory allocs in benchmark output |
| `-cpuprofile` | `-cpuprofile <file>` | — | Write CPU profile |
| `-memprofile` | `-memprofile <file>` | — | Write memory profile |
| `-blockprofile` | `-blockprofile <file>` | — | Write goroutine blocking profile |
| `-mutexprofile` | `-mutexprofile <file>` | — | Write mutex contention profile |
| `-memprofilerate` | `-memprofilerate N` | `512*1024` | Sample 1 alloc per N bytes |
| `-blockprofilerate` | `-blockprofilerate N` | `1` | Sample 1 block event per N ns |
| `-mutexprofilefraction` | `-mutexprofilefraction N` | `1` | Sample 1 mutex event per N |
| `-trace` | `-trace <file>` | — | Write execution trace |

### Fuzzing Control

| Flag | Syntax | Default | Purpose |
|------|--------|---------|---------|
| `-fuzztime` | `-fuzztime D\|Nx` | — | Total fuzzing duration or iteration count |
| `-fuzzminimizetime` | `-fuzzminimizetime D\|Nx` | `60s` | Max time to minimize failing input |

## Flag Interactions

### Critical Combinations

| Combination | Behavior | Notes |
|-------------|----------|-------|
| `-race` + `-cover` | Forces `-covermode=atomic` | Cannot use `-covermode=set` with `-race` |
| `-race` + `-parallel=1` | Serializes but still detects races | Useful for debugging; slower |
| `-count=1` + cache | **Bypasses cache entirely** | Use to force re-execution |
| Default package-list run | Uses cached results when only cacheable flags are present | Use `-count=1` to force re-execution |
| `-run` + `-skip` | `-run` selects, then `-skip` excludes | `-skip` applied after `-run` |
| `-run` + subtests | Matches `/`-separated names | `-run TestFoo/case_one` |
| `-v` + gotestsum | **Required** for `--rerun-fails` | Without `-v`, parallel subtests misreported |
| `-failfast` + `-race` | Stops on first FAIL, but race reports may still print | Race reports are asynchronous |
| `-short` + `-race` | Skips slow tests; race detector still active | Good for quick local checks |
| `-timeout 0` | No timeout | **Dangerous** — tests can hang forever |
| `-shuffle on` + `-count N` | Different shuffle seed each count | Good for finding order-dependent flakes |
| `-bench .` + `-run ^$` | Run only benchmarks, skip tests | Canonical pattern for benchmark-only runs |
| `-fuzz` + `-race` | Fuzz with race detection | High CPU overhead; useful for concurrent targets |

### CI-Specific Combinations (from invowk CI)

The source of truth is the "Run tests" steps in `.github/workflows/ci.yml` and
`.github/workflows/release.yml`; read them before quoting a lane. Summary as of
this writing:

- `ci.yml`
  - Full, non-CLI packages and the isolated `./internal/runtime` package:
    `-race -timeout 15m -coverprofile=...`, `--rerun-fails
    --rerun-fails-max-failures 5`, `INVOWK_TEST_CONTAINER_PARALLEL=2`.
  - Full CLI (`./tests/cli/...`): `-race -timeout 10m`, intentionally **no**
    `--rerun-fails` ("so the container CLI suite is no longer hidden behind
    generic rerun-fails behavior"). Do not "fix" it by adding reruns.
  - Short (Windows and macOS legs): `-race -short -v`, rerun max 3, default
    timeout.
  - Short CLI (Windows and macOS): `-race -timeout 5m`, no reruns.
- `release.yml`: full non-CLI `-race -timeout 15m`, rerun max 5 (does not split
  out `internal/runtime`); full CLI `-race -timeout 10m` and short CLI
  `-race -timeout 5m`, both with `--rerun-fails --rerun-fails-max-failures 3`;
  short `-race -short -v`, rerun max 3.
- Several lanes run `--rerun-fails` without `-v`, contrary to the gotestsum rule
  in `SKILL.md`; do not copy them as templates.

**Benchmark smoke test** (`ci.yml`, one Ubuntu leg; compile/run check, no
regression tracking). Do not add `-short`: every `internal/benchmark` benchmark
skips under `testing.Short()`.
```
go test ./internal/benchmark -run=^$ -bench="$BENCH_REGEX" -benchmem -benchtime=1x -count=1
```

## Caching Behavior

Caching applies only in package-list mode (`go test ./...`, `go test ./pkg`),
never in local-directory mode (`go test` with no package arguments).

### When a Cached Result Is Reused

- Every test flag on the command line must be cacheable: `-benchtime`,
  `-coverprofile`, `-cpu`, `-failfast`, `-fullpath`, `-list`, `-outputdir`,
  `-parallel`, `-run`, `-short`, `-skip`, `-timeout`, `-v`. Any other test flag
  or argument, including any `-count` value, disables caching.
- A successful cached result is reused regardless of `-timeout`.

### What Goes Into the Cache Key

| Factor | Cache Key |
|--------|-----------|
| Go source files | Hash of `.go` files in package and dependencies (part of the test binary) |
| Build flags (`-race`, `-tags`, `-cover` instrumentation) | Different test binary, therefore a separate cache entry; they do **not** disable caching |
| Cacheable test flags | Part of the key (each combination is a separate entry) |
| Environment variables | Variables read by `os.Getenv` during the test must be unchanged |
| Files read via `os.Open` | Files within the module opened during the cached run must be unchanged |

### Cache Bypass Patterns

```bash
# Force re-execution (most common)
go test -count=1 ./...

# Clean entire test cache
go clean -testcache

# Specific package cache bypass
go test -count=1 ./internal/runtime/...
```

## Timeout Behavior

The `-timeout` flag applies per test binary (per package). When the timeout fires:
1. The test binary panics with `panic: test timed out after D`, lists the
   running tests and dumps all goroutine stacks.
2. The test binary exits with status 2; `go test` reports `FAIL` for that
   package and itself exits 1, which is indistinguishable from an ordinary
   test failure by exit code alone.

**Important**: When one package times out with gotestsum, ALL results for that
package become `(unknown)`. This can mask other failures. The timeout should be
generous enough to never fire during normal execution while still catching genuine
hangs.

**Default timeout recommendations by package type:**
| Package Type | Recommended Timeout | Rationale |
|---|---|---|
| Unit tests only | 10m (default) | Sufficient for any unit test suite |
| Unit + integration | 15m | Container operations need headroom |
| CLI integration (testscript) | 10m | Testscript tests are process-per-test |
