# Test Review Findings Report Format

Use this format for all test review findings. The structured format enables
deduplication across subagents and prioritized triage.

## Report Header

```
TEST REVIEW FINDINGS REPORT
============================
Date: YYYY-MM-DD
Reviewer: review-tests coordinator + 8 subagents
Surfaces: SS1-SS8 (StructuralHygiene, ParallelismContext, TestPatterns,
          IntegrationGating, TxtarQuality, Mirrors, CoverageGuardrails,
          DomainTesting)
Programmatic Checks: PC-01 through PC-10 (see verification-commands.md)
```

## Checklist Status (per-subagent output)

Each subagent produces a checklist table BEFORE listing findings.
Every item from `surface-checklists.md` for the assigned surface must appear
-- no item may be omitted.

Status values: PASS (with evidence), FAIL (finding entry required), N/A (explain why).

Format:

| Check ID | Description | Status | Evidence / Finding ID |
|----------|-------------|--------|-----------------------|
| T1-C01   | ...         | PASS   | Verified in X files   |
| T1-C02   | ...         | FAIL   | RT-003                |
| T1-C03   | ...         | N/A    | No container tests    |

## Finding Entry Template

11 fields per finding:

| Field | Description |
|---|---|
| **ID** | `RT-{NNN}` -- sequential within the report |
| **Check ID** | The checklist item (e.g., `T3-C05`) |
| **Surface** | One of: StructuralHygiene, ParallelismContext, TestPatterns, IntegrationGating, TxtarQuality, Mirrors, CoverageGuardrails, DomainTesting |
| **Severity** | ERROR / WARNING / INFO / SKIP -- use pre-assigned severity from checklist |
| **File** | Path relative to repo root |
| **Line(s)** | Line number(s) or test function name |
| **Evidence** | Brief quote of what the code does (keep concise) |
| **Expected** | What the code should do based on the pattern catalog |
| **Rationale** | Why this is a finding (one sentence) |
| **Fix Hint** | Actionable remediation guidance (code pattern or command to run) |
| **Source** | `checklist` or `semantic` (whether from checklist item or semantic analysis) |

Example:

```
RT-042
  Check ID : T2-C03
  Surface  : ParallelismContext
  Severity : ERROR
  File     : internal/runtime/virtual_test.go
  Line(s)  : TestVirtualExec/subtest_env (line 187)
  Evidence : Subtest captures range variable `tc` without t.Parallel()
  Expected : Either call t.Parallel() + copy loop var, or run sequentially with comment
  Rationale: Range variable capture in parallel subtests causes flaky tests and data races
  Fix Hint : Add `t.Parallel()` at subtest top; Go 1.22+ loop var semantics make copy unnecessary
  Source   : checklist
```

## Severity Definitions

| Severity | Criteria | Action Required |
|---|---|---|
| **ERROR** | Would cause test failures, race conditions, flaky CI, or false sense of safety | Must fix before merge |
| **WARNING** | Suboptimal but not broken: missing parallelism, stale patterns, incomplete coverage | Should fix soon |
| **INFO** | Style improvement, minor hygiene, non-blocking suggestion | Fix when convenient |
| **SKIP** | Intentional deviation documented in `known-exceptions.md` or `accepted-patterns.md` | No action needed (accepted patterns are periodically reconsidered) |

## Semantic Analysis Findings

After completing the checklist, subagents scan for issues not covered by
specific checklist items:

- **Missing test coverage**: Exported functions without Test*, error-returning
  functions with only happy path tested, switch/if branches with no exercising
  test case
- **Low-value tests**: Circular assertions (constant == literal), struct-storage
  tests (field set/get only), always-skipped tests (unconditional t.Skip),
  excessive mocking (80%+ mock setup, trivial assertion)

Semantic findings use the same entry template but set Source to `semantic` and
use the nearest-match checklist severity.

## Summary Table

End the report with aggregate counts:

### Severity Breakdown

| Severity | Count |
|----------|-------|
| ERROR    | N     |
| WARNING  | N     |
| INFO     | N     |
| SKIP     | N     |

### Checklist Completion

| Surface | SA | Total | PASS | FAIL | N/A |
|---------|----|-------|------|------|-----|
| SS1 StructuralHygiene    | SA-1 | N | N | N | N |
| SS2 ParallelismContext   | SA-2 | N | N | N | N |
| SS3 TestPatterns         | SA-3 | N | N | N | N |
| SS4 IntegrationGating    | SA-4 | N | N | N | N |
| SS5 TxtarQuality         | SA-5 | N | N | N | N |
| SS6 Mirrors              | SA-6 | N | N | N | N |
| SS7 CoverageGuardrails   | SA-7 | N | N | N | N |
| SS8 DomainTesting        | SA-8 | N | N | N | N |

### Priority Fix List

Top 10 findings by severity (ERROR first, then WARNING), with file paths and
fix hints for immediate action.

### Patterns for Reconsideration

Appended when `references/accepted-patterns.md` has entries due for review or with
triggered reconsideration conditions. This section is informational -- it does not
create findings, but surfaces patterns the team should consciously re-evaluate.

```
PATTERNS FOR RECONSIDERATION
==============================
[None this round / list of entries with trigger or cadence status]
==============================
```

Each entry includes: pattern description, current confidence level, trigger status
or cadence note, and recommended action (re-evaluate, promote, demote, or remove).

## Merge Procedure (for coordinator)

1. Verify completeness (all checklist items reported by every subagent)
2. Collect findings from SA-1 through SA-8
3. Deduplicate by (file, test function/line) -- keep higher severity
4. Cross-check against `known-exceptions.md` (permanent exceptions)
5. Cross-check against `accepted-patterns.md` (conditionally-accepted patterns)
6. Run accepted-patterns reconsideration protocol:
   a. For each registry entry, check if review cadence has elapsed
   b. Evaluate reconsideration triggers against current codebase state
   c. Flag entries due for review or with triggered conditions
   d. Update Last Reviewed dates for entries that pass review
   e. Promote eligible entries (EXPERIMENTAL -> PROVISIONAL -> SETTLED)
   f. Demote entries whose triggers fired (any level -> EXPERIMENTAL)
   g. Remove entries for patterns no longer present in codebase
7. Sort by severity (ERROR first), then by surface
8. Assign sequential IDs (RT-001, RT-002, ...)
9. Merge checklist tables into unified completion summary
10. Produce summary, checklist completion, priority fix list, and
    Patterns for Reconsideration appendix
