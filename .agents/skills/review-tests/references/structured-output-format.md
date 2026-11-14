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
Programmatic Checks: PC-01 through PC-15 (see verification-commands.md)
```

## Checklist Status (per-subagent output)

Each subagent produces a checklist table BEFORE listing findings.
Every item from `surface-checklists.md` for the assigned surface must appear
-- no item may be omitted.

Status values (status rule, same wording as `SKILL.md`):
- **FAIL** if any deviation in the item's scope matches no registry entry; a finding entry is
  required for each such deviation.
- **SKIP** only when every deviation for the item matches an entry in `known-exceptions.md` or
  `accepted-patterns.md`, each one cited in the evidence.
- **PASS** otherwise, with evidence, citing any registry-matched occurrences.
- **N/A** when nothing in the item's scope applies, with the reason.

Format:

| Check ID | Description | Status | Evidence / Finding ID |
|----------|-------------|--------|-----------------------|
| T1-C01   | ...         | PASS   | Verified in X files   |
| T1-C02   | ...         | FAIL   | RT-003                |
| T2-C01   | ...         | SKIP   | All 4 tests without `t.Parallel()` use `t.Setenv`: known-exceptions.md "Parallelism Exceptions" |
| T1-C03   | ...         | N/A    | No container tests    |

## Finding Entry Template

11 fields per finding:

| Field | Description |
|---|---|
| **ID** | `RT-{NNN}` -- sequential within the report |
| **Check ID** | The checklist item (e.g., `T3-C05`) |
| **Surface** | One of: StructuralHygiene, ParallelismContext, TestPatterns, IntegrationGating, TxtarQuality, Mirrors, CoverageGuardrails, DomainTesting |
| **Severity** | ERROR / WARNING / INFO -- use pre-assigned severity from checklist |
| **File** | Path relative to repo root |
| **Line(s)** | Line number(s) or test function name |
| **Evidence** | Brief quote of what the code does (keep concise) |
| **Expected** | What the code should do based on the pattern catalog |
| **Rationale** | Why this is a finding (one sentence) |
| **Fix Hint** | Actionable remediation guidance (code pattern or command to run) |
| **Source** | `checklist` or `semantic` (whether from checklist item or semantic analysis) |

Example (illustrative file and test names):

```
RT-042
  Check ID : T2-C03
  Surface  : ParallelismContext
  Severity : ERROR
  File     : internal/config/config_env_test.go
  Line(s)  : TestLoadConfig_EnvOverride (line 41)
  Evidence : Calls t.Parallel() and then t.Setenv("XDG_CONFIG_HOME", dir)
  Expected : Tests that mutate process environment run serially (no t.Parallel())
  Rationale: t.Setenv panics in a parallel test, and process-wide env mutation races with other parallel tests
  Fix Hint : Remove t.Parallel() and add a comment explaining why the test is serial, or pass the directory through a request-scoped parameter instead of the environment
  Source   : checklist
```

## Severity Definitions

| Severity | Criteria | Action Required |
|---|---|---|
| **ERROR** | Would cause test failures, race conditions, flaky CI, or false sense of safety | Must fix before merge |
| **WARNING** | Suboptimal but not broken: missing parallelism, stale patterns, incomplete coverage | Should fix soon |
| **INFO** | Style improvement, minor hygiene, non-blocking suggestion | Fix when convenient |

Deviations documented in `known-exceptions.md` or `accepted-patterns.md` are not findings and
carry no severity; they appear only as citations in checklist evidence (accepted patterns are
periodically reconsidered).

## Semantic Analysis Findings

After the checklist, subagents scan for issues no checklist item covers: missing test coverage
(exported functions without Test*, error-returning functions with only the happy path tested,
switch/if branches with no exercising case) and low-value tests (circular assertions such as
constant == literal, struct-storage tests, always-skipped tests with an unconditional `t.Skip`,
excessive mocking with 80%+ mock setup and a trivial assertion). Semantic findings use the same
entry template with Source set to `semantic` and the nearest-match checklist severity.

## Summary Table

End the report with aggregate counts:

### Severity Breakdown

| Severity | Count |
|----------|-------|
| ERROR    | N     |
| WARNING  | N     |
| INFO     | N     |

### Checklist Completion

| Surface | SA | Total | PASS | FAIL | SKIP | N/A |
|---------|----|-------|------|------|------|-----|
| SS1 StructuralHygiene    | SA-1 | N | N | N | N | N |
| SS2 ParallelismContext   | SA-2 | N | N | N | N | N |
| SS3 TestPatterns         | SA-3 | N | N | N | N | N |
| SS4 IntegrationGating    | SA-4 | N | N | N | N | N |
| SS5 TxtarQuality         | SA-5 | N | N | N | N | N |
| SS6 Mirrors              | SA-6 | N | N | N | N | N |
| SS7 CoverageGuardrails   | SA-7 | N | N | N | N | N |
| SS8 DomainTesting        | SA-8 | N | N | N | N | N |

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

The canonical merge steps are in `SKILL.md` Step 3. The accepted-patterns reconsideration
sub-steps (cadence check, trigger evaluation, flagging, Last Reviewed updates, promotion,
demotion, removal) are in `accepted-patterns.md` § "Reconsideration Protocol".
