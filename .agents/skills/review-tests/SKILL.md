---
name: review-tests
description: Comprehensive test suite review and audit for invowk (audit only, no edits). Runs a fixed checklist across 8 surfaces — structural hygiene, parallelism/context, test patterns/assertions, integration gating, testscript quality, virtual/native mirrors, coverage guardrails, and TUI/domain-specific testing — and detects coverage gaps (missing branches, error paths, untested exports) and low-value tests (circular assertions, excessive mocking, dead tests). Use when reviewing test quality, checking test coverage, auditing the test suite, preparing for releases, or evaluating whether tests adequately cover recent code changes. Always use it for any test review task, even if the user doesn't say "review tests" — including checking test quality, verifying test coverage, or ensuring tests are comprehensive.
---

# Test Suite Review and Audit

This skill orchestrates a structured, read-only review of the whole test suite for semantic
comprehensiveness, signal-to-noise ratio, and coverage completeness. A request such as "review
the tests and tell me what to fix" belongs here: report findings with fix hints, then hand the
fixes to the `testing` skill (or the `test-writer` agent for new testscript pairs).

## Purpose and Scope

**Review**: All `*_test.go` files in `cmd/`, `internal/`, `pkg/`, `tests/`, `tools/`;
all `.txtar` files in `tests/cli/testdata/`; CI workflows; SonarCloud configuration.

**Do NOT**:
- Edit test files (use the `testing` skill for that after review identifies issues)
- Write new tests (use the `test-writer` agent)
- Review production code quality (use code-review agents)
- Flag intentional exceptions as errors (check `references/known-exceptions.md`)
- Flag accepted patterns as errors (check `references/accepted-patterns.md`)

---

## Consistency Principles

These principles make repeated runs produce the same results. They apply to the coordinator
and to every subagent.

1. **Checklist-driven review** — Each subagent follows its surface's checklist in
   `references/surface-checklists.md`, and every checklist item gets a status under the
   status rule below. This is the primary review activity; open-ended exploration is secondary.

2. **Pre-assigned severity** — Each checklist item has a severity in `surface-checklists.md`.
   Subagents use that severity, not their own judgment, because subjective severity is the
   second-largest source of run-to-run variance (after scope sampling).

3. **Deterministic file traversal** — Generate a sorted live inventory at the start of every
   review (`references/test-file-inventory.md`). Each checklist defines the scope; subagents
   check every current file in that scope, never a stale snapshot or a sample.

4. **Structured context passing** — The coordinator passes programmatic check results to
   subagents in the Context Block format below, not free-form prose, so every subagent receives
   identical context in an identical format.

5. **Complete reporting** — Every checklist item must appear in the subagent's output, with
   evidence for PASS and SKIP and a reason for N/A. The coordinator verifies completeness
   during merge.

**Status rule** (the same wording is used in every reference file):
- **FAIL** if any deviation in the item's scope matches no registry entry; each such
  deviation becomes a finding.
- **SKIP** only when every deviation for the item matches an entry in `known-exceptions.md`
  or `accepted-patterns.md`, each one cited in the evidence.
- **PASS** otherwise, citing any registry-matched occurrences in the evidence.
- **N/A** when nothing in the item's scope applies, with the reason.

Registry-matched occurrences are never findings and never carry a severity.

---

## Orchestration Strategy

### Step 1: Run Programmatic Checks

Run automated checks first to catch mechanical issues. Per-check expectations and failure
triage (PC-01 to PC-15) are in `references/verification-commands.md`.

```bash
# Parallel group 1 (fast file-level)
make check-file-length                                                                # PC-01
make license-check                                                                    # PC-02
rg -n 'context\.Background\(\)' cmd internal pkg tests tools -g '*_test.go'           # PC-03
rg -n 'time\.Sleep' cmd internal pkg tests tools -g '*_test.go'                       # PC-04
rg -n 'strings\.Contains\([^\n]*\.Error\(\)' cmd internal pkg tests tools -g '*_test.go'  # PC-11
rg -n '"/(usr|tmp|etc|home|bin)/' cmd internal pkg tests tools -g '*_test.go'         # PC-14

# Parallel group 1b (txtar-specific)
rg -n '! exec' tests/cli/testdata -g '*.txtar'                                        # PC-12
go test -v -run 'TestShRuntimeMirrorCoverage|TestVirtualNativeCommandPathAlignment' ./tests/cli/...  # PC-07, PC-08, PC-13

# Parallel group 2 (targeted test runs)
make lint                                                                             # PC-05
go test -v -run 'TestBuiltinCommandTxtarCoverage|TestTUIExemptionTmuxCoverage' ./cmd/invowk/...      # PC-06
go test -v -run TestIssueTemplates_NoStaleGuidance ./internal/issue/...               # PC-15

# Sequential (comprehensive)
make test-short                                                                       # PC-09
rg --files cmd internal pkg tests tools -g '*_test.go' | sort | xargs -r wc -l | awk '$2 != "total" && $1 > 900 { print }'  # PC-10
```

Record results in the **Context Block** format. This is the only copy of the template; pass it
verbatim to every surface subagent, including agents dispatched in later capacity-aware waves.

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
native-platform-split : guardrail PASS | FAIL (detail); manual T6-C06 inspection pending
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

`mirror-coverage`, `mirror-alignment` and `native-platform-split` all come from the single
group 1b test run. The `native-platform-split` row records only that guardrail result: a
passing run does not prove the Unix/Windows split, so SA-6 must still inspect every native
implementation for T6-C06 (PC-13).

### Step 2: Dispatch One Independent Subagent per Surface

Assign one subagent per surface. Run them in capacity-aware waves when fewer than eight
subagent slots are available; never collapse multiple surfaces into one reviewer merely to
finish in one wave, because a merged reviewer samples less. Each subagent receives:
1. The Context Block from Step 1
2. Its assigned surface checklist section from `references/surface-checklists.md`
3. The structured output format from `references/structured-output-format.md`

| Subagent | Surface | Focus | References to Read |
|----------|---------|-------|-------------------|
| **SA-1: Structural Hygiene** | SS1 | File size limits, SPDX headers, naming conventions, import ordering, test helper documentation | live inventory (`test-file-inventory.md`), `surface-checklists.md` §SS1 |
| **SA-2: Parallelism & Context** | SS2 | `t.Parallel()` rules, `t.Context()` usage, unsafe patterns (global state mutation), container test context, shared mock safety | `pattern-catalog.md` §1-2, `known-exceptions.md`, `accepted-patterns.md`, `surface-checklists.md` §SS2 |
| **SA-3: Test Patterns** | SS3 | Table-driven tests, error assertion quality, behavioral contracts, cross-platform path handling, flakiness patterns, `Validate()` usage | `pattern-catalog.md` §1-2b, `known-exceptions.md`, `accepted-patterns.md`, `surface-checklists.md` §SS3 |
| **SA-4: Integration Gating** | SS4 | `testing.Short()` guards, container timeout and serialization layers, container image policy, CI race detector, Windows config isolation | `pattern-catalog.md` §3, `coverage-expectations.md` (CI flags), `surface-checklists.md` §SS4 |
| **SA-5: Testscript Quality** | SS5 | CUE correctness in inline invowkfiles, workspace isolation, skip guards, regex assertions, dual-channel error checks, line endings | live inventory (`test-file-inventory.md`), `pattern-catalog.md` §4, `surface-checklists.md` §SS5 |
| **SA-6: Mirrors & Platform** | SS6 | Mirror completeness, exemption freshness, platform-split CUE, command-path alignment, `skipOnWindows` legitimacy | live mirror pairs (`test-file-inventory.md`), `known-exceptions.md`, `accepted-patterns.md`, `surface-checklists.md` §SS6 |
| **SA-7: Coverage & Guardrails** | SS7 | Guardrail test health, stale/unnecessary exemptions, test helper consolidation, SonarCloud gate, CI coverage configuration | `coverage-expectations.md`, `surface-checklists.md` §SS7 |
| **SA-8: TUI & Domain** | SS8 | TUI component model testing, container mock patterns, server state machine coverage, benchmark gating, watch/provision tests | `pattern-catalog.md`, `known-exceptions.md`, `accepted-patterns.md`, `surface-checklists.md` §SS8 |

#### Subagent Prompt Template

Use this template when spawning each subagent. Variation in how the task is described to
subagents is a source of run-to-run inconsistency.

```
You are reviewing test surface SS{N}: {Surface Name} for the invowk project.

## Your Task
Follow the checklist in `references/surface-checklists.md` §SS{N} item by item. For each
checklist item, report PASS, FAIL, SKIP, or N/A with evidence, using the status rule in the
skill. Then generate findings for all FAIL items using the format in
`references/structured-output-format.md`.

## Reference Files to Read
{list of reference files from the dispatch table above}

## Programmatic Check Results (from coordinator)
{paste the Context Block here}

## Per-Item Procedure
For each checklist item:
1. Read the test file(s) specified in the checklist's File Scope column
2. Apply the check criteria (consult `references/pattern-catalog.md` for detailed patterns)
3. For semantic checks: assess whether the test exercises meaningful behavioral contracts
4. For each deviation, check `references/known-exceptions.md` (permanent exceptions) and
   `references/accepted-patterns.md` (accepted patterns). A deviation that matches either
   registry is not a finding; cite the entry in the item's evidence.
5. Record the status: FAIL if any deviation matches no registry entry (one finding per such
   deviation); SKIP only if every deviation matches a registry entry, each cited; otherwise
   PASS with evidence; N/A with the reason when nothing in scope applies.

## Platform-Specific Analysis
For findings that may be platform-specific, consult the relevant platform skill:
- Windows failures: `windows-testing` (process lifecycle, file system, timers)
- macOS failures: `macos-testing` (APFS, kqueue, timer coalescing)
- Linux/container failures: `linux-testing` (inotify, cgroups, container timeouts)
- Cross-platform test toolchain: `go-testing` (race detector, flags, vet)

## Semantic Analysis (in addition to checklist)
After completing the checklist, scan your file scope for:
- Exported functions without corresponding Test* coverage
- Error-returning functions where only the happy path is tested
- Switch/if branches with no exercising test case
- Low-value tests: circular assertions, struct-storage tests, always-skipped tests

Report these as additional findings using the checklist's nearest-match severity.
Set the Source field to "semantic" to distinguish from checklist findings.

## Output
1. Checklist Status table (every item, no omissions)
2. Findings list (one entry per unmatched deviation in a FAIL item, plus semantic findings)
```

### Step 3: Merge and Report

This is the canonical merge procedure. The coordinator:
1. **Verifies completeness** — Each subagent reported on all checklist items for its surface
2. **Collects** findings from SA-1 through SA-8
3. **Deduplicates** by (file, test function/line) — keep higher severity on conflicts
4. **Cross-checks** against `references/known-exceptions.md` (permanent exceptions)
5. **Cross-checks** against `references/accepted-patterns.md` (conditionally-accepted patterns).
   Drop any finding that matches either registry, cite the entry in the item's evidence, and
   re-derive that item's status under the status rule.
6. **Runs the reconsideration protocol** — cadence review and trigger evaluation for accepted
   patterns (see `references/accepted-patterns.md` § "Reconsideration Protocol")
7. **Sorts** by severity (ERROR first), then by surface
8. **Assigns** sequential IDs (RT-001, RT-002, ...) to the merged list
9. **Merges** checklist tables into a unified Checklist Completion summary
10. **Produces** the final report with the Patterns for Reconsideration appendix
    (see `references/structured-output-format.md`)

---

## Recurrence Prevention

When the same checklist item fails in 3+ consecutive audit rounds despite being "fixed",
the finding must be promoted to a programmatic check. This closes the loop and prevents
subagent sampling from missing known patterns.

### Promotion Criteria

A finding is eligible for promotion when ALL of these hold:
1. The same checklist item ID (e.g., T3-C05) appears as FAIL in 3+ consecutive rounds
2. The pattern can be detected by a grep, find, or single-command check
3. The check has a near-zero false-positive rate (< 5% with `known-exceptions.md` filtering)

### Promotion Workflow

1. **Document the pattern**: Add a grep/find command to `references/verification-commands.md`
   as a new PC-NN entry (follow the existing format)
2. **Add to Step 1**: Add the command to the appropriate parallel group in this file
3. **Update Context Block**: Add a new result row to the Step 1 Context Block in this file
   (the only copy of the template)
4. **Add to pattern-catalog.md**: If the pattern needs subagent explanation, add an entry
5. **Update this table**: Record the promotion below

### Currently Promoted Patterns

| Checklist Item | Programmatic Check | Round Promoted | Description |
|---|---|---|---|
| T2-C05 / T2-C06 | PC-03 | Original | `context.Background()` in test functions |
| T3-C03 | PC-04 | Original | `time.Sleep` in tests |
| T3-C05 | PC-11 | Round 8 | `strings.Contains(err.Error())` patterns |
| T5-C13 | PC-12 | Round 8 | Missing txtar error-path assertions |
| T6-C06 | PC-13 | Round 8 | Non-platform-split native CUE |
| T3-C10 | PC-14 | Round 8 | Hardcoded Unix absolute paths |

---

## Detailed References

- [`references/surface-checklists.md`](references/surface-checklists.md) — per-surface verification items with pre-assigned severity; the primary review driver.
- [`references/test-file-inventory.md`](references/test-file-inventory.md) — commands for the live sorted file list, line counts, categories and mirror pairs.
- [`references/pattern-catalog.md`](references/pattern-catalog.md) — required patterns, anti-patterns, error-assertion and `time.Sleep` classification, container and testscript patterns, flakiness signatures, lint directives.
- [`references/coverage-expectations.md`](references/coverage-expectations.md) — SonarCloud gate, CI coverage flags, guardrail test inventory, per-package expectations.
- [`references/known-exceptions.md`](references/known-exceptions.md) — permanent, intentional deviations (not findings).
- [`references/accepted-patterns.md`](references/accepted-patterns.md) — conditionally-accepted patterns with reconsideration triggers and confidence levels (not findings, but periodically re-evaluated).
- [`references/structured-output-format.md`](references/structured-output-format.md) — report header, checklist status format, finding template, severity definitions, summary tables.
- [`references/verification-commands.md`](references/verification-commands.md) — PC-01 to PC-15 commands with expected output and failure triage.

---

## Related Skills

| Skill | When to Use |
|---|---|
| `/testing` | After review identifies issues — for writing new tests, fixing test patterns, setting up testscript |
| `/go-testing` | For deep Go test toolchain knowledge — flags, race detector, vet analyzers, context/parallelism decision frameworks |
| `/windows-testing` | When review surfaces Windows-specific failures — process lifecycle, file system, timer resolution |
| `/macos-testing` | When review surfaces macOS-specific failures — APFS, kqueue, timer coalescing, /tmp symlink |
| `/linux-testing` | When review surfaces Linux/container failures — inotify limits, container timeout strategy, cgroups |
| `test-writer` agent | For generating new testscript virtual/native `.txtar` pairs |
| `/native-mirror` | For creating native runtime mirrors from virtual tests |
| `/tmux-testing` | For TUI e2e test development |
