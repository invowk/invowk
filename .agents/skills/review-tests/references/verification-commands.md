# Verification Commands Reference

Run these automated checks BEFORE manual review to catch mechanical issues.
Any failure should be investigated and recorded as a finding before proceeding.

## Execution Order

Run checks in this order (fastest first, parallelizable checks grouped):

1. Parallel Group 1 (fast file-level): PC-01 + PC-02 + PC-03 + PC-04
2. Parallel Group 2 (targeted test runs): PC-05 + PC-06 + PC-07 + PC-08 + PC-15
3. Sequential: PC-09 + PC-10
4. Pattern scans: PC-11 + PC-12 + PC-13 + PC-14

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

Command: `rg -n 'context\.Background\(\)' cmd internal pkg tests tools -g '*_test.go'`
What it checks: Usage of context.Background() in test files. Should use t.Context() (Go 1.24+).
Expected: Only matches in TestMain, env.Defer callbacks, or package-level init (with comments explaining why).
Failure triage: Check each match. If it's inside a Test* function, it should use t.Context(). Consult `known-exceptions.md` for legitimate uses.

## 4. Flakiness-Prone time.Sleep (PC-04)

Command: `rg -n 'time\.Sleep' cmd internal pkg tests tools -g '*_test.go'`
What it checks: Usage of time.Sleep in test files.
Expected: Only matches classified as KEEP per `pattern-catalog.md` § "time.Sleep Classification" (event separation, latency simulation, poll-helper testing). See `known-exceptions.md` § "time.Sleep Exceptions" for the current registry.
Failure triage: Classify each match. Sleeps in assertion logic (waiting for a condition) are ERROR — replace with channels/polling. Sleeps for event separation (fsnotify coalescing), latency simulation (debounce/serialization testing), or poll-helper self-tests are legitimate and should be added to `known-exceptions.md` if not already listed.

## 5. Linting (PC-05)

Command: `make lint`
What it checks: golangci-lint with project configuration (.golangci.toml). Covers tparallel, modernize, exhaustive, and 30+ other linters.
Expected: Exit 0.
Failure triage: Fix all linter violations. tparallel issues indicate missing/misplaced t.Parallel(). modernize issues indicate stale Go patterns.

## 6. Built-in Command txtar Coverage (PC-06)

Command: `go test -v -run 'TestBuiltinCommandTxtarCoverage|TestTUIExemptionTmuxCoverage' ./cmd/invowk/...`
What it checks: Every non-hidden, runnable, leaf built-in command has at least one txtar test, and every TUI txtar exemption has tmux e2e coverage. Two-way verification: stale exemptions (command removed) and unnecessary exemptions (command now covered).
Expected: PASS.
Failure triage: Add a `.txtar` file in `tests/cli/testdata/` with `exec invowk <command>`, or add to `builtinTxtarCoverageExemptions` with documented reason.

## 7. Virtual/Native Mirror Coverage (PC-07)

Command: `go test -v -run TestShRuntimeMirrorCoverage ./tests/cli/...`
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

Command: `rg --files cmd internal pkg tests tools -g '*_test.go' | sort | xargs -r wc -l | awk '$2 != "total" && $1 > 900 { print }'`
What it checks: Test files approaching the 1000-line hard limit (900+ lines signals need to plan a split).
Expected: No matches (or only files recently split that are in progress).
Failure triage: Split by concern using `<package>_<concern>_test.go` naming. Follow File Splitting Protocol: create new, delete from source, clean imports, `go build` before `make test`.

## 11. Error String Matching (PC-11)

Command: `rg -n 'strings\.Contains\([^\n]*\.Error\(\)' cmd internal pkg tests tools -g '*_test.go'`
What it checks: Usage of `strings.Contains(err.Error(), ...)` for error assertions. Should use `errors.Is` / `errors.As` for sentinel errors or typed error assertions.
Expected: Most matches will be KEEP (not findings). Classify each occurrence per `pattern-catalog.md` § "Error Assertion Classification". Only report CONVERTIBLE cases where a sentinel error exists in the error chain via `%w` wrapping. Consult `known-exceptions.md` § "Error String Matching Exceptions" for the full registry of legitimate patterns.
Failure triage: For CONVERTIBLE cases, replace with `errors.Is(err, ErrFoo)` or `errors.As(err, &target)`. For KEEP cases (ValidationErrors flattening, Error() format tests, CUE library errors, supplementary checks alongside errors.Is, external/OS errors), leave as-is — string matching is correct. The key decision rule: trace the error to its production source. If `fmt.Errorf("...: %w", sentinel)` wraps a sentinel, it's CONVERTIBLE. Everything else is KEEP.

## 12. Missing Error-Path txtar Assertion (PC-12)

Command: `rg -n '! exec' tests/cli/testdata -g '*.txtar'`
What it checks: Produces the complete live set of error-path commands for semantic
inspection. Review the following assertion block for both stdout and stderr;
single-line grep adjacency is not sufficient because comments and conditions may
legitimately intervene.
Expected: Every match has meaningful output-channel assertions, subject to the
documented container incidental-stderr exception.
Failure triage: Add `! stdout .` and/or a specific `stderr` assertion. See
`pattern-catalog.md` section 4.

## 13. Non-Platform-Split Native CUE (PC-13)

Command: `go test -v -run 'TestShRuntimeMirrorCoverage|TestVirtualNativeCommandPathAlignment' ./tests/cli/...`
What it checks: The repository parser and exemption registry enforce mirror
coverage and command-path alignment. During manual review, also inspect each
native implementation for a Unix/Windows platform split; do not infer valid CUE
structure from a one-line grep.
Expected: PASS, with no stale or missing exemptions.
Failure triage: Use `native-mirror` to repair the pair or update the exemption
JSON only when divergence is intentional and justified.

## 14. Hardcoded Unix Absolute Paths in Test Assertions (PC-14)

Command: `rg -n '"/(usr|tmp|etc|home|bin)/' cmd internal pkg tests tools -g '*_test.go'`
What it checks: Hardcoded Unix absolute paths in test assertions and fixtures. These are cross-platform blind spots.
Expected: Only matches listed in `known-exceptions.md` Hardcoded Path Exceptions (container-internal paths in `filepaths_test.go`, POSIX-only `dirname_test.go`).
Failure triage: Replace with `filepath.Join(t.TempDir(), ...)` for fixture paths, or add `skipOnWindows` with documented reason.

## 15. Issue Template Guardrail (PC-15)

Command: `go test -v -run TestIssueTemplates_NoStaleGuidance ./internal/issue/...`
What it checks: Embedded issue templates do not contain stale guidance or deprecated command names.
Expected: PASS.
Failure triage: Update the issue template text or the stale-token list if the guidance intentionally changed.

## Context Block Format

Record all results in this format and pass verbatim to each surface subagent,
including agents dispatched in later capacity-aware waves:

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
tui-tmux-coverage     : PASS | FAIL (detail)
mirror-coverage       : PASS | FAIL (detail)
mirror-alignment      : PASS | FAIL (detail)
issue-template-guard  : PASS | FAIL (detail)
test-short            : PASS | FAIL (detail)
approaching-limit-files : PASS | FAIL (files: ...)
==========================
```
