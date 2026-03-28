# Go Test Flags — Complete Matrix

## All `go test` Flags

### Test Selection

| Flag | Syntax | Default | Purpose |
|------|--------|---------|---------|
| `-run` | `-run <regex>` | `""` (all) | Run only tests/examples matching regex |
| `-skip` | `-skip <regex>` | `""` (none) | Skip tests matching regex (Go 1.22+) |
| `-bench` | `-bench <regex>` | `""` (none) | Run benchmarks matching regex; `.` for all |
| `-fuzz` | `-fuzz <regex>` | `""` (none) | Run fuzz tests matching regex |
| `-list` | `-list <regex>` | — | List tests/benchmarks/examples; no execution |
| `-tags` | `-tags <list>` | `""` | Comma-separated build tags |

### Execution Control

| Flag | Syntax | Default | Purpose |
|------|--------|---------|---------|
| `-count` | `-count N` | `1` | Run each test N times; 0 = use cache |
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
| `-o` | `-o <file>` | — | Compile test binary to file; no execution |
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
| `-count=0` | Uses cached results if available | Rarely used explicitly |
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

**Linux full mode (non-CLI packages):**
```
-race -timeout 15m -v
INVOWK_TEST_CONTAINER_PARALLEL=2
gotestsum --rerun-fails --rerun-fails-max-failures 5
```

**Linux full mode (runtime package — isolated):**
```
-race -timeout 15m -v
gotestsum --rerun-fails --rerun-fails-max-failures 5
```

**Linux full mode (CLI integration tests):**
```
-race -timeout 10m -v
gotestsum --rerun-fails --rerun-fails-max-failures 5
```

**Windows/macOS short mode:**
```
-race -short -v -timeout 15m
gotestsum --rerun-fails --rerun-fails-max-failures 3
```

**Windows/macOS CLI tests:**
```
-race -timeout 5m -v
gotestsum --rerun-fails --rerun-fails-max-failures 3
```

**Benchmark smoke test (CI validation, no regression tracking):**
```
go test -run=^$ -bench=. -benchtime=1x -short -count=1
```

## Caching Behavior

### What Invalidates the Cache

| Factor | Cache Key |
|--------|-----------|
| Go source files | Hash of `.go` files in package and dependencies |
| Test flags | Each unique flag combination is a separate key |
| Environment variables | Variables read by `os.Getenv` during test |
| Files read via `os.Open` | Heuristic — files accessed during last cached run |
| `-race` flag | Separate key (different binary) |
| `-cover` flag | Separate key (different instrumentation) |
| `-count` flag | `-count=1` bypasses cache; other values are keys |
| Build tags | Each tag set is a separate key |

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
1. The test binary receives a stack dump of all goroutines
2. The binary exits with status 1
3. The `panic: test timed out` message includes the goroutine stacks

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
| TUI tests with `-race` on Windows | 15m | 288 tests + race overhead |
