# Coverage Expectations and Guardrails

Coverage targets, guardrail test inventory, CI configuration, and SonarCloud integration.

---

## SonarCloud Quality Gate

**Project key**: `invowk_invowk` (org `invowk`)

### Source and Test Inclusions

From `sonar-project.properties`:
- **Sources**: `cmd`, `internal`, `pkg` (explicit directories, not root `.`)
- **Tests**: `cmd`, `internal`, `pkg`, `tests`
- **Test inclusions**: `**/*_test.go`
- **Exclusions**: `**/testdata/**`, `**/vendor/**`, `specs/**`, `modules/**`, `third_party/**`, `builtin/**`, `tools/goplint/**`
- **CPD exclusions**: `**/*_test.go`, `cmd/invowk/app_validate.go`, `internal/app/commandsvc/types_validate.go`

### Suppressed Rules

| ID | Rule | Scope | Reason |
|---|---|---|---|
| e1 | `go:S100` | `**/*_test.go` | Test functions use underscores (idiomatic `TestFoo_Bar`) |
| e2 | `go:S1192` | `**/*_test.go` | Test files repeat string literals in assertions/fixtures |
| e3 | `godre:S8196` | `**/*.go` | Domain interfaces use descriptive names, not `-er` suffix |
| e4 | `godre:S8242` | `**/*.go` | Context in struct is intentional lifecycle ownership |
| e5 | `go:S3776` | `**/*.go` | Cognitive complexity inherent to validation/TUI/container/resolution |
| e6 | `go:S2612` | `**/*_test.go` | Test fixtures use broad temp-file permissions intentionally |
| e7 | `go:S1313` | `**/*_test.go` | Test fixtures use RFC 1918 IPs without network connections |

IDs must be gapless (`e1, e2, e3, ...`). Adding a new suppression requires using the next ID.

### Coverage Gate

SonarCloud uses automatic analysis (GitHub App). Unit tests under `cmd/`, `internal/`, `pkg/`
move the coverage gate. Tests under `tests/cli/` do NOT contribute to SonarCloud coverage
(they are CLI integration tests; use `make test-cli-cover` for CLI-specific coverage).

---

## CI Coverage Configuration

### Test Flags

| Workflow | Coverage Flag | Race | Timeout | Retry |
|---|---|---|---|---|
| `ci.yml` main tests | `-coverprofile=coverage.out` | `-race` | `15m` (gotestsum) | `--rerun-fails --rerun-fails-max-failures 5` |
| `ci.yml` CLI tests | — | `-race` | `10m` | No retry (deterministic) |
| `make test` | Same as CI via gotestsum | `-race` | — | `--rerun-fails` if gotestsum available |
| `make test-short` | — | — | — | — |
| `make test-cli-cover` | `GOCOVERDIR=...` + `-cover` build | `-race` | `10m` | — |

### CLI Coverage Collection

`make test-cli-cover`:
1. Builds invowk binary with `-cover` flag
2. Sets `GOCOVERDIR` per-test for coverage data collection
3. Runs CLI tests: `go test -race -timeout 10m ./tests/cli/...`
4. Merges per-test coverage data using `go tool covdata textfmt`
5. Output: `cli-coverage.out`

**Caveat**: `go tool covdata textfmt` does NOT recurse into subdirectories — use
`find -printf '%h\n'` to discover coverage directories.

**Caveat**: Go binaries built with `-cover` emit `warning: GOCOVERDIR not set` to stderr
when `GOCOVERDIR` is absent. This breaks `! stderr .` assertions in testscript. Only add
`-cover` when `GOCOVERDIR` is set (conditional in `TestMain`).

---

## Guardrail Test Inventory

Five guardrail tests enforce coverage mandates programmatically. These are checked by
subagent SA-7 (Coverage & Guardrails).

### 1. TestBuiltinCommandTxtarCoverage

**File**: `cmd/invowk/coverage_test.go`

Verifies every non-hidden, runnable, leaf built-in Cobra command has at least one
`.txtar` test in `tests/cli/testdata/` with `exec invowk <command>`.

**Two-way verification**:
- Stale exemptions: command no longer exists in Cobra tree
- Unnecessary exemptions: command is now covered by txtar tests

**Current exemptions** (9 TUI commands):

| Command | Reason |
|---|---|
| `tui input` | Interactive TTY; E2E via tmux |
| `tui choose` | Interactive TTY; E2E via tmux |
| `tui confirm` | Interactive TTY; E2E via tmux |
| `tui write` | Interactive TTY; E2E via tmux |
| `tui filter` | Interactive TTY; E2E via tmux |
| `tui file` | Interactive TTY; E2E via tmux |
| `tui table` | Interactive TTY; E2E via tmux |
| `tui spin` | Interactive TTY; E2E via tmux |
| `tui pager` | Interactive TTY; E2E via tmux |

### 2. TestTUIExemptionTmuxCoverage

**File**: `cmd/invowk/coverage_test.go`

Verifies every TUI txtar exemption has a corresponding tmux e2e marker in
`tests/cli/tui_tmux_test.go`. Prevents silent loss of e2e coverage when TUI commands
are exempt from txtar tests.

**Marker format**: `" tui <command> "` must exist as a substring in `tui_tmux_test.go`.

### 3. TestVirtualRuntimeMirrorCoverage

**File**: `tests/cli/runtime_mirror_test.go`

Verifies every non-exempt `virtual_*.txtar` has a `native_*.txtar` mirror.

**Exemption source**: `tests/cli/runtime_mirror_exemptions.json`

### 4. TestVirtualNativeCommandPathAlignment

**File**: `tests/cli/runtime_mirror_test.go`

Verifies virtual/native mirror pairs exercise the same set of `invowk` command paths.
Prevents drift where one mirror adds test cases that the other misses.

### 5. TestIssueTemplates_NoStaleGuidance

**File**: `internal/issue/issue_test.go`

Scans embedded `.md` issue templates for stale tokens (deprecated CLI subcommands,
Alpine-specific commands). Prevents issue templates from suggesting outdated workflows.

---

## Per-Package Coverage Expectations

### High-Coverage Targets (core domain logic)

These packages contain critical business logic where gaps are most dangerous:

| Package | Focus | Why Critical |
|---|---|---|
| `pkg/invowkfile/` | Schema parsing, validation | User-facing configuration correctness |
| `pkg/invowkmod/` | Module metadata, operations | Module resolution, vendoring, locking |
| `internal/runtime/` | All three runtimes | Command execution correctness |
| `internal/discovery/` | Module/command discovery | Correct precedence and collision detection |
| `internal/app/commandsvc/` | Command execution service | Core execution pipeline |
| `internal/app/deps/` | Dependency validation | Prerequisite checking |
| `internal/config/` | Configuration management | Config loading and defaults |

### Infrastructure Packages (lower but adequate coverage)

| Package | Focus | Coverage Notes |
|---|---|---|
| `internal/testutil/` | Test helpers | Tested transitively through consumers |
| `internal/benchmark/` | PGO benchmarks | Benchmarks test performance, not correctness |
| `internal/core/serverbase/` | Server state machine | State transitions tested; lifecycle tested through SSH/TUI servers |
| `cmd/invowk/` | CLI adapters | Thin wrappers; behavior tested via integration tests |

### Separate Module (different coverage gate)

| Package | Focus | Coverage Notes |
|---|---|---|
| `tools/goplint/` | Custom linter | Own `go.mod`; excluded from SonarCloud production coverage |
