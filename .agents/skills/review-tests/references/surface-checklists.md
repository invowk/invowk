# Surface Verification Checklists

Each surface has a numbered checklist of verification items. Subagents report PASS, FAIL, or N/A
for every item — no item may be skipped. Findings are generated from FAIL items only.

Severity is pre-assigned per item to eliminate subjective classification. The severity levels
(ERROR, WARNING, INFO) follow the definitions in `structured-output-format.md`.

---

## §SS1: Structural Hygiene

**File scope**: All `*_test.go` files in `cmd/`, `internal/`, `pkg/`, `tests/`, `tools/`

**Reference**: `test-file-inventory.md` for deterministic file enumeration.

| ID | Check | File Scope | Severity |
|---|---|---|---|
| T1-C01 | All `*_test.go` files have `// SPDX-License-Identifier: MPL-2.0` as first line | All test files | ERROR |
| T1-C02 | No test file exceeds 900 lines (approaching 1000-line hard limit) | All test files | WARNING |
| T1-C03 | No test file exceeds 1000 lines (hard limit from `check-file-length`) | All test files | ERROR |
| T1-C04 | Split files follow `<package>_<concern>_test.go` naming convention | Files in packages with multiple `_test.go` files | INFO |
| T1-C05 | Test file imports follow 3-group ordering (stdlib, external, internal) | All test files | INFO |
| T1-C06 | No duplicate `Test*` function names across files in same package | All test files per package | ERROR |
| T1-C07 | No `*_test.go` file has a duplicate `// Package` doc comment (only one per package, usually in `doc.go`) | All test files | WARNING |
| T1-C08 | Test helpers in `*_test.go` are marked with `t.Helper()` | All test files | WARNING |
| T1-C09 | Test function names use `TestTypeName_Method` or `TestFunctionName` convention | All test files | INFO |
| T1-C10 | No orphaned test helper files (helpers used in 0 test functions) | All test files | INFO |
| T1-C11 | `internal/testutil/` exported functions are documented with doc comments | `internal/testutil/` | WARNING |
| T1-C12 | `internal/testutil/invowkfiletest/` exported functions are documented with doc comments | `internal/testutil/invowkfiletest/` | WARNING |
| T1-C13 | No `*_test.go` files exist outside expected directories (`cmd/`, `internal/`, `pkg/`, `tests/`, `tools/`) | Root and other directories | WARNING |
| T1-C14 | Files that were recently split have cleaned-up imports (no orphaned imports) | Recently split files (check git log) | INFO |

**Total items**: 14

---

## §SS2: Parallelism and Context

**File scope**: All `*_test.go` files in `cmd/`, `internal/`, `pkg/`

**References**: `pattern-catalog.md` §1-2, `known-exceptions.md` (Parallelism Exceptions, Context Exceptions).

| ID | Check | File Scope | Severity |
|---|---|---|---|
| T2-C01 | All top-level `Test*` functions call `t.Parallel()` as first statement (unless in `known-exceptions.md`) | All test files | ERROR |
| T2-C02 | All table-driven subtests inside parallel parents also call `t.Parallel()` | Tests with `t.Run()` inside parallel parents | ERROR |
| T2-C03 | No `t.Parallel()` in tests that use `os.Chdir`, `os.Setenv`, `t.Setenv`, `SetHomeDir`, `MustSetenv`, `MustChdir` | Tests mutating global state | ERROR |
| T2-C04 | No `tt := tt` or `tc := tc` loop-variable rebinding (Go 1.22+ per-iteration semantics) | All table-driven tests | WARNING |
| T2-C05 | `t.Context()` used as default context in test functions (not `context.Background()`) | All test functions | WARNING |
| T2-C06 | `context.Background()` only used in `TestMain`, package-level init, or `env.Defer()` callbacks — with comment explaining why | All test files | WARNING |
| T2-C07 | `b.Context()` used in benchmarks (not `t.Context()` or `context.Background()`) | All benchmark files | WARNING |
| T2-C08 | Tests using `os.MkdirTemp` + `defer os.RemoveAll` do not have `t.Parallel()` subtests (use `t.TempDir()` instead) | Tests with temp dirs + parallel subtests | ERROR |
| T2-C09 | Container integration tests use `testutil.ContainerTestContext()` not bare `t.Context()` | `internal/runtime/container*_test.go` | ERROR |
| T2-C10 | Container integration tests acquire `testutil.ContainerSemaphore()` after `t.Parallel()` and `testing.Short()` skip | `internal/runtime/container*_test.go`, `internal/container/*_test.go` | ERROR |
| T2-C11 | No shared `MockCommandRecorder` across parallel subtests (each needs own instance) | `internal/container/*mock*_test.go` | ERROR |
| T2-C12 | No `maps.Copy` needed where `for k, v := range` loop clones full map (modernize linter) | All test files | INFO |

**Total items**: 12

---

## §SS3: Test Patterns and Assertions

**File scope**: All `*_test.go` files in `cmd/`, `internal/`, `pkg/`

**References**: `pattern-catalog.md` §1-2, `known-exceptions.md` (Test Helper Exceptions).

| ID | Check | File Scope | Severity |
|---|---|---|---|
| T3-C01 | Functions with 3+ test cases use table-driven test pattern (`tests := []struct{...}`) | All test files | WARNING |
| T3-C02 | No `reflect.DeepEqual` on typed slices — use `slices.Equal` instead | All test files | WARNING |
| T3-C03 | No `time.Sleep()` in test assertions (use clock injection or channels) | All test files | ERROR |
| T3-C04 | No hardcoded Unix paths (`/foo/bar`) in assertions without `skipOnWindows` or `filepath.Join()` | All test files | WARNING |
| T3-C05 | Error assertions use `errors.Is` or `errors.As`, not string matching on `err.Error()` | Tests checking errors | WARNING |
| T3-C06 | Tests verify behavioral contracts, not struct field storage (no trivial constant == literal tests) | All test files | WARNING |
| T3-C07 | No `Validate()` call results discarded (`_ = x.Validate()` or bare `x.Validate()`) | All test files | WARNING |
| T3-C08 | `goruntime` import alias used when both `runtime` package and `runtime.GOOS` needed | `internal/runtime/*_test.go`, `cmd/invowk/*_test.go` | INFO |
| T3-C09 | Cross-platform path validator tests include all 6 vector categories (Unix abs, Windows drive abs, UNC, slash traversal, backslash traversal, valid relative) | `pkg/types/*_test.go`, `pkg/fspath/*_test.go`, `pkg/invowkmod/*_test.go` | WARNING |
| T3-C10 | Absolute path fixtures use `t.TempDir()` + `filepath.Join()`, not hardcoded `/foo/bar` | Tests using absolute paths as fixtures | WARNING |
| T3-C11 | No circular/trivial tests (zero-value == zero, constant == literal without behavioral purpose) | All test files | WARNING |
| T3-C12 | `t.TempDir()` preferred over `os.MkdirTemp` + manual cleanup | All test files | INFO |
| T3-C13 | Tests that scan for prohibited patterns (guardrail tests) are documented with clear rationale | `cmd/invowk/coverage_test.go`, `internal/issue/issue_test.go` | INFO |
| T3-C14 | `t.Fatalf` used (not `t.Errorf`) when continuing after failure would cause nil pointer dereference | Tests with dependent assertions | INFO |

**Total items**: 14

---

## §SS4: Integration Test Gating

**File scope**: `internal/runtime/*_test.go`, `internal/container/*_test.go`, `tests/cli/*_test.go`, `.github/workflows/ci.yml`

**References**: `pattern-catalog.md` §3, `coverage-expectations.md`.

| ID | Check | File Scope | Severity |
|---|---|---|---|
| T4-C01 | All tests requiring external resources (container engine, network) check `testing.Short()` | Integration test files | ERROR |
| T4-C02 | Container tests use all 5 timeout layers (per-test deadline, cleanup, CI timeout, semaphore, bounded context) | `tests/cli/container_test.go`, `internal/runtime/container*_test.go` | WARNING |
| T4-C03 | Container test `Setup` sets `HOME` to `env.WorkDir` (not `/no-home`) | `tests/cli/container_test.go` | ERROR |
| T4-C04 | Container availability check runs actual smoke test (`debian:stable-slim` pull + `echo ok`), not just CLI version probe | `tests/cli/helpers_test.go` | WARNING |
| T4-C05 | Container cleanup uses `env.Defer()` (not `t.Cleanup()` in testscript context) | `tests/cli/container_test.go` | WARNING |
| T4-C06 | `testing.Short()` skip message follows convention: `"skipping integration test in short mode"` | Integration test files | INFO |
| T4-C07 | No `AcquireContainerSuiteLock` in `internal/runtime` tests (semaphore alone suffices) | `internal/runtime/container*_test.go` | ERROR |
| T4-C08 | Windows testscript setup sets `APPDATA` and `USERPROFILE` to test-scoped paths | `tests/cli/helpers_test.go` | WARNING |
| T4-C09 | Container image used in tests is `debian:stable-slim` (unless testing language-specific runtime) | All container test files | WARNING |
| T4-C10 | CI workflow `ci.yml` runs separate test steps for non-CLI, internal/runtime, and CLI | `.github/workflows/ci.yml` | INFO |
| T4-C11 | CI workflow uses `-race` flag for all test runs | `.github/workflows/ci.yml` | WARNING |
| T4-C12 | CI workflow uses `gotestsum --rerun-fails` with `--rerun-fails-max-failures 5` for non-CLI, but NOT for CLI tests | `.github/workflows/ci.yml` | INFO |

**Total items**: 12

---

## §SS5: Testscript (txtar) Quality

**File scope**: All `*.txtar` files in `tests/cli/testdata/`

**References**: `test-file-inventory.md` (txtar enumeration), `pattern-catalog.md` §4.

| ID | Check | File Scope | Severity |
|---|---|---|---|
| T5-C01 | All `.txtar` files have a descriptive `# Test:` comment at the top | All txtar files | WARNING |
| T5-C02 | Container txtar files include `[!container-available] skip` guard | `container_*.txtar` | ERROR |
| T5-C03 | Sandbox-sensitive txtar files include `[in-sandbox] skip` guard | Affected txtar files | WARNING |
| T5-C04 | Tests with embedded `invowkfile.cue` use `cd $WORK` (not `cd $PROJECT_ROOT`) | Non-dogfooding txtar files | ERROR |
| T5-C05 | Embedded CUE `implementations:` blocks have `platforms:` field (structural validity) | All txtar with inline CUE | ERROR |
| T5-C06 | No `[GOOS:windows]` condition used (use built-in `[windows]` instead) | All txtar files | ERROR |
| T5-C07 | `stdout`/`stderr` regex patterns do not have unescaped parentheses or brackets | All txtar files | WARNING |
| T5-C08 | Tests that need `--` flag separator use it correctly (invowk flags before `--`, command args after) | Txtar files with `--` | WARNING |
| T5-C09 | No `env.Cd` set in test Setup function (tests must control own working directory) | `tests/cli/*_test.go` | ERROR |
| T5-C10 | Virtual runtime tests declare all platforms: `[{name: "linux"}, {name: "macos"}, {name: "windows"}]` | `virtual_*.txtar` | WARNING |
| T5-C11 | Container txtar files use `platforms: [{name: "linux"}]` only (not macos/windows) | `container_*.txtar` | ERROR |
| T5-C12 | No workspace contamination: tests needing broken fixtures isolate into subdirectories | Txtar files creating invalid fixtures | WARNING |
| T5-C13 | CLI error tests check both stdout (handler formatting) and stderr (Cobra error rendering) | Error-path txtar files | WARNING |
| T5-C14 | No placeholder environment variables set in setup (only production-used vars) | `tests/cli/*_test.go` Setup functions | INFO |
| T5-C15 | Txtar files use LF line endings (enforced by `.gitattributes` `*.txtar text eol=lf`) | All txtar files | WARNING |

**Total items**: 15

---

## §SS6: Virtual/Native Mirrors and Platform

**File scope**: `tests/cli/testdata/virtual_*.txtar`, `tests/cli/testdata/native_*.txtar`, `tests/cli/runtime_mirror_exemptions.json`

**References**: `test-file-inventory.md` (pairing table), `known-exceptions.md` (Mirror Exemptions).

| ID | Check | File Scope | Severity |
|---|---|---|---|
| T6-C01 | Every non-exempt `virtual_*.txtar` has a corresponding `native_*.txtar` mirror | All virtual txtar files | ERROR |
| T6-C02 | Mirror exemptions in `runtime_mirror_exemptions.json` are not stale (all referenced files exist) | `runtime_mirror_exemptions.json` | ERROR |
| T6-C03 | No superseded exemptions (native mirror now exists but exemption not removed) | `runtime_mirror_exemptions.json` | WARNING |
| T6-C04 | No orphan `native_*.txtar` without corresponding `virtual_*.txtar` (unless standalone native test) | All native txtar files | WARNING |
| T6-C05 | Virtual/native mirror pairs exercise the same set of invowk command paths | Paired virtual + native files | ERROR |
| T6-C06 | Native implementations use platform-split CUE: separate Linux/macOS and Windows blocks | All `native_*.txtar` | ERROR |
| T6-C07 | Native Windows implementations use `Write-Output` (not `echo`), `$env:VAR` (not `$VAR`) | `native_*.txtar` Windows blocks | WARNING |
| T6-C08 | Virtual and native tests produce identical `stdout` assertions for the same test cases | Paired virtual + native files | ERROR |
| T6-C09 | `skipOnWindows` used only for semantically meaningless tests, not implementation gaps | Tests with `skipOnWindows` | WARNING |
| T6-C10 | Cross-platform path assertions use `filepath.Join()`, not hardcoded separators | Go test files with path assertions | WARNING |
| T6-C11 | `[windows] skip` in txtar used only for genuine platform limitations, with documented reason | Txtar files with `[windows] skip` | WARNING |
| T6-C12 | Command-path exemptions in `runtime_mirror_exemptions.json` have valid justifications | `runtime_mirror_exemptions.json` | INFO |

**Total items**: 12

---

## §SS7: Coverage and Guardrails

**File scope**: `cmd/invowk/coverage_test.go`, `tests/cli/runtime_mirror_test.go`, `internal/issue/issue_test.go`, `internal/testutil/`, `sonar-project.properties`, `.github/workflows/ci.yml`

**References**: `coverage-expectations.md`.

| ID | Check | File Scope | Severity |
|---|---|---|---|
| T7-C01 | `TestBuiltinCommandTxtarCoverage` passes: every non-hidden leaf command has txtar or documented exemption | `cmd/invowk/coverage_test.go` | ERROR |
| T7-C02 | `TestTUIExemptionTmuxCoverage` passes: every TUI exemption has tmux e2e marker | `cmd/invowk/coverage_test.go` | ERROR |
| T7-C03 | No stale entries in `builtinTxtarCoverageExemptions` (commands that no longer exist) | `cmd/invowk/coverage_test.go` | ERROR |
| T7-C04 | No unnecessary entries in `builtinTxtarCoverageExemptions` (commands now covered by txtar) | `cmd/invowk/coverage_test.go` | WARNING |
| T7-C05 | `tests/cli/tui_tmux_test.go` covers all 9 TUI commands (input, choose, confirm, write, filter, file, table, spin, pager) | `tests/cli/tui_tmux_test.go` | ERROR |
| T7-C06 | CI runs with `-coverprofile=coverage.out` for SonarCloud gate | `.github/workflows/ci.yml` | WARNING |
| T7-C07 | CI runs with `-race` flag on all platforms | `.github/workflows/ci.yml` | WARNING |
| T7-C08 | SonarCloud `sonar.test.inclusions` covers all test file locations | `sonar-project.properties` | WARNING |
| T7-C09 | Test helpers in `internal/testutil/` are not duplicated in individual package test files | `internal/testutil/` vs all test files | WARNING |
| T7-C10 | `invowkfiletest` helpers are used by multi-package consumers (not just one file) | `internal/testutil/invowkfiletest/` | INFO |
| T7-C11 | Issue template guardrail test (`TestIssueTemplates_NoStaleGuidance`) is current and not bypassed | `internal/issue/issue_test.go` | INFO |

**Total items**: 11

---

## §SS8: TUI and Domain-Specific Testing

**File scope**: `internal/tui/*_test.go`, `internal/container/*mock*_test.go`, `tools/goplint/**/*_test.go`, `internal/benchmark/*_test.go`, `internal/sshserver/*_test.go`, `internal/core/serverbase/*_test.go`, `internal/watch/*_test.go`, `internal/provision/*_test.go`

**References**: `pattern-catalog.md` §1-3, `known-exceptions.md`.

| ID | Check | File Scope | Severity |
|---|---|---|---|
| T8-C01 | All TUI components in `internal/tui/` have corresponding `*_test.go` files | `internal/tui/` | ERROR |
| T8-C02 | TUI tests cover model state transitions (`Init()`, `Update()` with various messages) | `internal/tui/*_test.go` | WARNING |
| T8-C03 | TUI tests cover edge cases: empty inputs, very long inputs, unicode, special characters | `internal/tui/*_test.go` | WARNING |
| T8-C04 | Container mock tests use per-test `MockCommandRecorder` instances (not shared globals) | `internal/container/*mock*_test.go` | ERROR |
| T8-C05 | Container mock tests inject via `WithExecCommand()` functional option pattern | `internal/container/*mock*_test.go` | WARNING |
| T8-C06 | `tools/goplint` tests use per-test analyzer instances (not shared process-wide state) | `tools/goplint/**/*_test.go` | WARNING |
| T8-C07 | Semaphore tokens in goplint test helpers released via `defer` in same call (not `t.Cleanup`) | `tools/goplint/**/*_test.go` | WARNING |
| T8-C08 | Benchmark tests (`internal/benchmark/`) are gated with `testing.Short()` where appropriate | `internal/benchmark/*_test.go` | WARNING |
| T8-C09 | SSH server tests use sequential subtests (host key collision avoidance) | `internal/sshserver/*_test.go` | WARNING |
| T8-C10 | `internal/core/serverbase/` tests cover state machine transitions (Created/Starting/Running/Stopping/Stopped) | `internal/core/serverbase/*_test.go` | WARNING |
| T8-C11 | Watch tests (`internal/watch/`) handle platform-specific behavior (Windows fatal pattern) | `internal/watch/*_test.go` | INFO |
| T8-C12 | Provision tests (`internal/provision/`) verify layer provisioning with proper container semaphore usage | `internal/provision/*_test.go` | WARNING |

**Total items**: 12

---

## Grand Total

| Surface | Items | ERROR | WARNING | INFO |
|---|---|---|---|---|
| SS1: Structural Hygiene | 14 | 3 | 6 | 5 |
| SS2: Parallelism & Context | 12 | 7 | 4 | 1 |
| SS3: Test Patterns & Assertions | 14 | 1 | 9 | 4 |
| SS4: Integration Test Gating | 12 | 3 | 6 | 3 |
| SS5: Testscript Quality | 15 | 6 | 8 | 1 |
| SS6: Virtual/Native Mirrors | 12 | 5 | 6 | 1 |
| SS7: Coverage & Guardrails | 11 | 4 | 5 | 2 |
| SS8: TUI & Domain-Specific | 12 | 2 | 9 | 1 |
| **Grand Total** | **102** | **31** | **53** | **18** |
