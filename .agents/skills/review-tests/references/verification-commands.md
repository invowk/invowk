# Verification Commands Reference

Run these automated checks BEFORE manual review to catch mechanical issues.
Any failure should be investigated and recorded as a finding before proceeding.

## Execution Order

Run checks in this order (fastest first, parallelizable checks grouped):

1. Parallel Group 1 (fast file-level): PC-01 + PC-02 + PC-03 + PC-04
2. Parallel Group 2 (targeted test runs): PC-05 + PC-06 + PC-07 + PC-08
3. Sequential: PC-09 (comprehensive), PC-10 (file scan)

## 1. File Length Check (PC-01)

Command: `make check-file-length`
What it checks: All Go files under 1000 lines (hard limit).
Expected: Exit 0.
Failure triage: Split oversized files by concern using `<package>_<concern>_test.go` naming.

## 2. License Headers (PC-02)

Command: `make license-check`
What it checks: SPDX-License-Identifier: MPL-2.0 as first line of every .go file.
Expected: Exit 0.
Failure triage: Add `// SPDX-License-Identifier: MPL-2.0` as the first line followed by a blank line.

## 3. Stale context.Background() (PC-03)

Command: `grep -rn 'context\.Background()' --include='*_test.go' cmd/ internal/ pkg/`
What it checks: Usage of context.Background() in test files. Should use t.Context() (Go 1.24+).
Expected: Only matches in TestMain, env.Defer callbacks, or package-level init (with comments explaining why).
Failure triage: Check each match. If it's inside a Test* function, it should use t.Context(). Consult `known-exceptions.md` for legitimate uses.

## 4. Flakiness-Prone time.Sleep (PC-04)

Command: `grep -rn 'time\.Sleep' --include='*_test.go' cmd/ internal/ pkg/`
What it checks: Usage of time.Sleep in test files.
Expected: Zero matches. Use clock injection or channel synchronization.
Failure triage: Replace with testutil.NewFakeClock + Advance(), or channel-based synchronization.

## 5. Linting (PC-05)

Command: `make lint`
What it checks: golangci-lint with project configuration (.golangci.toml). Covers tparallel, modernize, exhaustive, and 30+ other linters.
Expected: Exit 0.
Failure triage: Fix all linter violations. tparallel issues indicate missing/misplaced t.Parallel(). modernize issues indicate stale Go patterns.

## 6. Built-in Command txtar Coverage (PC-06)

Command: `go test -v -run TestBuiltinCommandTxtarCoverage ./cmd/invowk/...`
What it checks: Every non-hidden, runnable, leaf built-in command has at least one txtar test. Two-way verification: stale exemptions (command removed) and unnecessary exemptions (command now covered).
Expected: PASS.
Failure triage: Add a `.txtar` file in `tests/cli/testdata/` with `exec invowk <command>`, or add to `builtinTxtarCoverageExemptions` with documented reason.

## 7. Virtual/Native Mirror Coverage (PC-07)

Command: `go test -v -run TestVirtualRuntimeMirrorCoverage ./tests/cli/...`
What it checks: Every non-exempt `virtual_*.txtar` has a `native_*.txtar` mirror.
Expected: PASS.
Failure triage: Create the missing native mirror using the `/native-mirror` skill, or add an exemption to `runtime_mirror_exemptions.json` with justification.

## 8. Mirror Command-Path Alignment (PC-08)

Command: `go test -v -run TestVirtualNativeCommandPathAlignment ./tests/cli/...`
What it checks: Virtual/native mirror pairs exercise the same invowk command paths.
Expected: PASS.
Failure triage: Align the command paths between virtual and native txtar pairs.

## 9. Short Test Suite (PC-09)

Command: `make test-short`
What it checks: Basic compilation and all unit tests (skips integration tests).
Expected: Exit 0, all tests pass.
Failure triage: Fix failing tests. Common issues: missing imports after refactor, stale test expectations.

## 10. Approaching 1000-Line Limit (PC-10)

Command: `find cmd/ internal/ pkg/ tests/ tools/ -name '*_test.go' -exec wc -l {} + | awk '$1 > 900 { print }'`
What it checks: Test files approaching the 1000-line hard limit (900+ lines signals need to plan a split).
Expected: No matches (or only files recently split that are in progress).
Failure triage: Split by concern using `<package>_<concern>_test.go` naming. Follow File Splitting Protocol: create new, delete from source, clean imports, `go build` before `make test`.

## 11. Error String Matching (PC-11)

Command: `grep -rn 'strings\.Contains(.*\.Error()' --include='*_test.go' cmd/ internal/ pkg/`
What it checks: Usage of `strings.Contains(err.Error(), ...)` for error assertions. Should use `errors.Is` / `errors.As` for sentinel errors or typed error assertions.
Expected: Only matches where no sentinel exists (CUE engine messages, stdlib errors) or where the check is content-verification alongside an `errors.Is` (verifying a specific value appears in the message). Consult `known-exceptions.md`.
Failure triage: Replace with `errors.Is(err, ErrFoo)` for sentinel errors, `errors.As(err, &target)` for typed errors. String matching is acceptable ONLY for unstructured third-party error text (CUE engine messages, `strconv` errors) with a comment explaining why.

## 12. Missing Error-Path txtar Assertion (PC-12)

Command: `grep -B0 -A1 '! exec' tests/cli/testdata/*.txtar | grep -B1 '^--$' | grep '! exec'`
What it checks: Error-path txtar tests (`! exec ...`) that lack any stdout/stderr assertion on the next line. Error tests should assert BOTH output channels.
Expected: Zero matches. Every `! exec` should be followed by `stdout`, `stderr`, `! stdout .`, or `! stderr .`.
Failure triage: Add `! stdout .` and/or `stderr 'expected text'` after `! exec` lines. See `pattern-catalog.md` § 4 for the dual-channel error assertion pattern.

## 13. Non-Platform-Split Native CUE (PC-13)

Command: `for f in tests/cli/testdata/native_*.txtar; do if ! grep -q 'platforms:.*windows' "$f" 2>/dev/null; then echo "$f: missing Windows platform block"; fi; done`
What it checks: Native txtar implementations must use platform-split CUE with separate Linux/macOS and Windows implementation blocks.
Expected: Zero output. Every `native_*.txtar` must have at least one `platforms: [{name: "windows"}]` block.
Failure triage: Split the single implementation into two: `[{name: "linux"}, {name: "macos"}]` with bash script, and `[{name: "windows"}]` with PowerShell script. Use `Write-Output` and `$env:VAR` in the Windows block.

## 14. Hardcoded Unix Absolute Paths in Test Assertions (PC-14)

Command: `grep -rn '"/usr/\|"/tmp/\|"/etc/\|"/home/\|"/bin/' --include='*_test.go' cmd/ internal/ pkg/`
What it checks: Hardcoded Unix absolute paths in test assertions and fixtures. These are cross-platform blind spots.
Expected: Only matches listed in `known-exceptions.md` Hardcoded Path Exceptions (container-internal paths in `filepaths_test.go`, POSIX-only `dirname_test.go`).
Failure triage: Replace with `filepath.Join(t.TempDir(), ...)` for fixture paths, or add `skipOnWindows` with documented reason.

## Context Block Format

Record all results in this format and pass verbatim to all 8 subagents:

```
PROGRAMMATIC CHECK RESULTS
==========================
file-length           : PASS | FAIL (files: ...)
license-check         : PASS | FAIL (files: ...)
context-background    : PASS | FAIL (count: N, files: ...)
time-sleep            : PASS | FAIL (count: N, files: ...)
err-string-matching   : PASS | FAIL (count: N, files: ...)
hardcoded-unix-paths  : PASS | FAIL (count: N, files: ...)
txtar-error-assertion : PASS | FAIL (files: ...)
native-platform-split : PASS | FAIL (files: ...)
lint                  : PASS | FAIL (detail)
txtar-coverage        : PASS | FAIL (detail)
mirror-coverage       : PASS | FAIL (detail)
mirror-alignment      : PASS | FAIL (detail)
test-short            : PASS | FAIL (detail)
approaching-limit-files : PASS | FAIL (files: ...)
==========================
```
