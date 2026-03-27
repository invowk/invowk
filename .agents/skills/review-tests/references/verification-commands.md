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

## 10. 800-Line Soft Limit (PC-10)

Command: `find cmd/ internal/ pkg/ tests/ tools/ -name '*_test.go' -exec wc -l {} + | awk '$1 > 800 { print }'`
What it checks: Test files exceeding 800-line soft limit (project convention from testing rules).
Expected: No matches (or only files recently split that are in progress).
Failure triage: Split by concern using `<package>_<concern>_test.go` naming. Follow File Splitting Protocol: create new, delete from source, clean imports, `go build` before `make test`.

## Context Block Format

Record all results in this format and pass verbatim to all 8 subagents:

```
PROGRAMMATIC CHECK RESULTS
==========================
file-length         : PASS | FAIL (files: ...)
license-check       : PASS | FAIL (files: ...)
context-background  : PASS | FAIL (count: N, files: ...)
time-sleep          : PASS | FAIL (count: N, files: ...)
lint                : PASS | FAIL (detail)
txtar-coverage      : PASS | FAIL (detail)
mirror-coverage     : PASS | FAIL (detail)
mirror-alignment    : PASS | FAIL (detail)
test-short          : PASS | FAIL (detail)
800-line-files      : PASS | FAIL (files: ...)
==========================
```
