---
name: review-tests
description: Comprehensive test suite review and audit for invowk. Evaluates 102 checklist items across 8 surfaces — structural hygiene, parallelism/context, test patterns/assertions, integration gating, testscript quality, virtual/native mirrors, coverage guardrails, and TUI/domain-specific testing. Detects both coverage gaps (missing branches, error paths, untested exports) and low-value tests (circular assertions, excessive mocking, dead tests). Use this skill whenever reviewing test quality, checking test coverage, auditing the test suite, preparing for releases, or evaluating whether tests adequately cover recent code changes. Always use this skill for any test review task, even if the user doesn't explicitly say "review tests" — any mention of checking test quality, verifying test coverage, or ensuring tests are comprehensive should trigger this skill.
disable-model-invocation: false
---

# Test Suite Review and Audit

This skill orchestrates a structured review of the entire test suite to evaluate semantic
comprehensiveness, signal-to-noise ratio, and coverage completeness. It is review-only —
use the `/testing` skill for writing tests and the `test-writer` agent for generating
testscript pairs.

## Purpose and Scope

**Review**: All `*_test.go` files in `cmd/`, `internal/`, `pkg/`, `tests/`, `tools/`;
all `.txtar` files in `tests/cli/testdata/`; CI workflows; SonarCloud configuration.

**Do NOT**:
- Edit test files (use the `testing` skill for that after review identifies issues)
- Write new tests (use the `test-writer` agent)
- Review production code quality (use code-review agents)
- Flag intentional exceptions as errors (check `references/known-exceptions.md`)

---

## Review Surfaces

The review covers 8 surfaces. Each maps to a distinct testing dimension.

### SS1: Structural Hygiene
File size limits, SPDX license headers, naming conventions, import ordering, test helper
documentation. Read `references/test-file-inventory.md` for the complete file enumeration.

### SS2: Parallelism and Context
`t.Parallel()` rules, `t.Context()` usage, unsafe patterns (global state mutation),
container test context, shared mock safety. Read `references/pattern-catalog.md` §1-2
and `references/known-exceptions.md` for legitimate deviations.

### SS3: Test Patterns and Assertions
Table-driven tests, error assertion quality, behavioral contracts, cross-platform path
handling, flakiness patterns, `Validate()` usage. Read `references/pattern-catalog.md` §1-2.

### SS4: Integration Test Gating
`testing.Short()` guards, container 5-layer timeout strategy, container image policy,
CI race detector, Windows config isolation. Read `references/pattern-catalog.md` §3.

### SS5: Testscript (txtar) Quality
CUE correctness in inline invowkfiles, workspace isolation, skip guards, regex assertions,
dual-channel error checks, line endings. Read `references/pattern-catalog.md` §4.

### SS6: Virtual/Native Mirrors and Platform
Mirror completeness, exemption freshness, platform-split CUE, command-path alignment,
`skipOnWindows` legitimacy. Read `references/test-file-inventory.md` pairing table.

### SS7: Coverage and Guardrails
Guardrail test health (5 tests), stale/unnecessary exemptions, test helper consolidation,
SonarCloud gate, CI coverage configuration. Read `references/coverage-expectations.md`.

### SS8: TUI and Domain-Specific Testing
TUI component model testing, container mock patterns, goplint analyzer tests, server
state machine coverage, benchmark gating, watch/provision tests.

---

## Consistency Principles

These principles ensure that running the review multiple times produces the same results.
They apply to both the coordinator and all subagents.

1. **Checklist-driven review** — Each subagent follows its surface's checklist from
   `references/surface-checklists.md`. Every checklist item gets a status (PASS/FAIL/N-A).
   This is the primary review activity — open-ended exploration is secondary.

2. **Pre-assigned severity** — Each checklist item has a severity level defined in
   `surface-checklists.md`. Subagents use that severity, not their own judgment. The reason:
   subjective severity classification is the second-largest source of run-to-run variance
   (after scope sampling). Fixing severity at definition time eliminates this.

3. **Deterministic file traversal** — Each checklist enumerates the exact files to review.
   Subagents check all listed files, not a sample. For surfaces with many files (SS1-SS3),
   the checklist specifies the file scope; `test-file-inventory.md` has the full enumeration.

4. **Structured context passing** — The coordinator passes programmatic check results to
   subagents using the Context Block format defined below, not free-form prose. This ensures
   every subagent receives identical context in an identical format.

5. **Complete reporting** — Every checklist item must appear in the subagent's output. Items
   that pass are reported as PASS with brief evidence. Items that cannot be checked are N/A
   with an explanation. The coordinator verifies completeness during merge.

---

## Orchestration Strategy

### Step 1: Run Programmatic Checks

Run automated checks first to catch mechanical issues. See `references/verification-commands.md`
for full details and failure triage.

```bash
# Parallel group 1 (fast file-level)
make check-file-length
make license-check
grep -rn 'context\.Background()' --include='*_test.go' cmd/ internal/ pkg/
grep -rn 'time\.Sleep' --include='*_test.go' cmd/ internal/ pkg/

# Parallel group 2 (targeted test runs)
make lint
go test -v -run TestBuiltinCommandTxtarCoverage ./cmd/invowk/...
go test -v -run TestVirtualRuntimeMirrorCoverage ./tests/cli/...
go test -v -run TestVirtualNativeCommandPathAlignment ./tests/cli/...

# Sequential (comprehensive)
make test-short
find cmd/ internal/ pkg/ tests/ tools/ -name '*_test.go' -exec wc -l {} + | awk '$1 > 900 { print }'
```

Record results in the **Context Block** format:

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
approaching-limit-files : PASS | FAIL (files: ...)
==========================
```

### Step 2: Spawn 8 Parallel Subagents

Spawn one subagent per surface. Each subagent receives:
1. The Context Block from Step 1
2. Its assigned surface checklist section from `references/surface-checklists.md`
3. The structured output format from `references/structured-output-format.md`

| Subagent | Surface | References to Read | Focus |
|----------|---------|-------------------|-------|
| **SA-1: Structural Hygiene** | SS1 | `test-file-inventory.md`, `surface-checklists.md` §SS1 | File size, headers, naming, imports, helpers |
| **SA-2: Parallelism & Context** | SS2 | `pattern-catalog.md`, `known-exceptions.md`, `surface-checklists.md` §SS2 | t.Parallel() rules, t.Context(), unsafe patterns |
| **SA-3: Test Patterns** | SS3 | `pattern-catalog.md`, `known-exceptions.md`, `surface-checklists.md` §SS3 | Table-driven tests, assertions, behavioral contracts |
| **SA-4: Integration Gating** | SS4 | `pattern-catalog.md`, `surface-checklists.md` §SS4 | testing.Short(), container timeouts, CI config |
| **SA-5: Testscript Quality** | SS5 | `test-file-inventory.md`, `pattern-catalog.md`, `surface-checklists.md` §SS5 | CUE correctness, skip guards, workspace isolation |
| **SA-6: Mirrors & Platform** | SS6 | `test-file-inventory.md`, `known-exceptions.md`, `surface-checklists.md` §SS6 | Mirror completeness, platform-split CUE, alignment |
| **SA-7: Coverage & Guardrails** | SS7 | `coverage-expectations.md`, `surface-checklists.md` §SS7 | Guardrail tests, exemptions, SonarCloud, helpers |
| **SA-8: TUI & Domain** | SS8 | `pattern-catalog.md`, `known-exceptions.md`, `surface-checklists.md` §SS8 | TUI models, container mocks, goplint, servers |

#### Subagent Prompt Template

Use this template when spawning each subagent. Consistent prompting is important because
variation in how the task is described to subagents is a source of run-to-run inconsistency.

```
You are reviewing test surface SS{N}: {Surface Name} for the invowk project.

## Your Task
Follow the checklist in `references/surface-checklists.md` §SS{N} item by item. For each
checklist item, report PASS, FAIL, or N/A with evidence. Then generate findings for
all FAIL items using the format in `references/structured-output-format.md`.

## Reference Files to Read
{list of reference files from the dispatch table above}

## Programmatic Check Results (from coordinator)
{paste the Context Block here}

## Per-Item Procedure
For each checklist item:
1. Read the test file(s) specified in the checklist's File Scope column
2. Apply the check criteria (consult `references/pattern-catalog.md` for detailed patterns)
3. For semantic checks: assess whether the test exercises meaningful behavioral contracts
4. Check `references/known-exceptions.md` — is a deviation intentional?

### Platform-Specific Analysis

For findings that may be platform-specific, consult the relevant platform skill:
- Windows failures: `windows-testing` (process lifecycle, file system, timers)
- macOS failures: `macos-testing` (APFS, kqueue, timer coalescing)
- Linux/container failures: `linux-testing` (inotify, cgroups, container timeouts)
- Cross-platform test toolchain: `go-testing` (race detector, flags, vet)
5. Record status: PASS (with evidence), FAIL (generate finding), or N/A (with reason)

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
2. Findings list (one entry per FAIL item, plus semantic findings)
```

### Step 3: Merge and Report

The coordinator:
1. **Verifies completeness** — Each subagent reported on all checklist items for its surface
2. **Collects** findings from SA-1 through SA-8
3. **Deduplicates** by (file, test function/line) — keep higher severity on conflicts
4. **Cross-checks** against `references/known-exceptions.md`
5. **Sorts** by severity (ERROR first), then by surface
6. **Assigns** sequential IDs (RT-001, RT-002, ...) to the merged list
7. **Merges** checklist tables into a unified Checklist Completion summary
8. **Produces** the final report (see `references/structured-output-format.md`)

---

## Detailed References

Read these when working on the corresponding review surface:

- **[references/surface-checklists.md](references/surface-checklists.md)** — Per-surface
  enumerated verification items (102 total across 8 surfaces). This is the primary review driver.
- **[references/test-file-inventory.md](references/test-file-inventory.md)** — Deterministic
  enumeration of all test files with line counts, categories, and pairing tables.
- **[references/pattern-catalog.md](references/pattern-catalog.md)** — Required patterns,
  anti-patterns, container patterns, testscript patterns, and flakiness signatures.
- **[references/coverage-expectations.md](references/coverage-expectations.md)** — SonarCloud
  gates, guardrail test inventory, CI coverage configuration, per-package expectations.
- **[references/known-exceptions.md](references/known-exceptions.md)** — Registry of
  intentional pattern deviations (do NOT flag as errors).
- **[references/structured-output-format.md](references/structured-output-format.md)** —
  Finding report template, checklist status format, severity definitions, merge procedure.
- **[references/verification-commands.md](references/verification-commands.md)** — Full
  command reference with expected output and failure triage.

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
