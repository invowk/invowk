---
name: fixer
description: >-
  Platform-aware bug diagnosis and authorized fix workflow with parallel
  subagents for complex failures. Use when investigating or fixing test
  failures, CI failures, flaky tests, runtime errors, race conditions, or
  platform-specific bugs. Diagnosis-only requests remain read-only; apply a
  fix only when the user asks for implementation or fixing.
  Also user-invocable as /fixer with an issue description, error output,
  PR number, or CI run URL. Spawns up to 3 parallel diagnostic subagents
  based on failure type: platform investigator (consults windows-testing,
  macos-testing, linux-testing), code path tracer, and pattern matcher.
  Produces a structured diagnosis report with root cause, recommended fix, and
  prevention, and verifies changes when implementation is authorized.
---

# Fixer Skill

Platform-aware bug diagnosis and fix workflow. Produce structured root cause
analysis first, then apply and verify changes only when the request authorizes
implementation.

## Normative Precedence

1. `.agents/rules/checklist.md` — mandatory verification after any fix.
2. `.agents/rules/testing.md` — test policy for new/modified tests.
3. `.agents/skills/go/SKILL.md` — code quality for production fixes.
4. This skill — diagnosis workflow, subagent orchestration, platform routing.

## Authorization Boundary

Classify the request before gathering evidence:

- **Diagnose, investigate, explain, review, or report**: remain read-only. Trace
  the root cause and recommend a fix, but do not edit files or mutate external
  state.
- **Fix, implement, repair, or make the checks pass**: diagnose first, then
  apply the smallest authorized fix and verify it.
- **Ambiguous request**: gather read-only evidence and report the diagnosis.
  Ask before editing only when the requested outcome cannot reasonably
  establish mutation authority.

Explicit `/fixer` invocation selects this workflow; it does not by itself
broaden the user's requested scope or authorize unrelated changes.

Arguments can be:
- A description of the issue: `/fixer the watcher tests are flaking on macOS`
- Error output pasted inline
- A GitHub PR or run reference: `/fixer check CI failures on PR #42`
- A test name: `/fixer TestContainerExec hangs on ubuntu-latest/podman`

## Workflow Overview

```
Input (error, CI failure, user description)
    │
    ▼
Phase 1: Evidence Gathering ──── up to 2 parallel subagents
    │
    ▼
Phase 2: Diagnosis ──────────── up to 3 parallel subagents
    │                            (platform investigator, code tracer, pattern matcher)
    ▼
Phase 3: Fix Design ─────────── recommend fix + prevention
    │
    ▼
Phase 4: Apply & Verify ─────── only when implementation is authorized
```

---

## Phase 1: Evidence Gathering

Collect all available evidence before spawning diagnostic agents. The quality
of the diagnosis depends entirely on the quality of the evidence.

### What to Gather

| Source | How to Access | What to Extract |
|--------|--------------|----------------|
| User-provided error output | Read from conversation | Error message, stack trace, test name |
| CI run logs | `gh run view <id> --log-failed` | Failing tests, platform, exit codes |
| CI JUnit artifacts | `gh run download <id> --pattern 'test-results-*'` | Test names, durations, rerun status |
| Local test output | `go test -v -run TestName ./pkg/...` | Full output with timing |
| Race detector report | From CI or local `-race` run | Goroutine stacks, access locations |
| Git blame | `git log --oneline -10 -- <file>` | Recent changes to failing code |
| GitHub PR checks | `gh pr checks <pr-number> --json name,bucket,state,workflow,link` | Which CI jobs failed |

### Platform Detection

Before spawning diagnostic agents, determine which platform(s) are affected:

| Signal | Platform | Skill to Consult |
|--------|----------|-----------------|
| `windows-latest` in CI job name | Windows | `windows-testing` |
| `macos-15` in CI job name | macOS | `macos-testing` |
| `ubuntu-*` in CI job name | Linux | `linux-testing` |
| Exit code 137 | Linux (OOM) | `linux-testing` |
| `TerminateProcess` or exit code 1 with timeout | Windows | `windows-testing` |
| `ERROR_SHARING_VIOLATION` or `Access is denied` | Windows | `windows-testing` |
| `ENOSPC` from fsnotify | Linux | `linux-testing` |
| kqueue-related or watcher event missed | macOS | `macos-testing` |
| Container test hang | Linux | `linux-testing` |
| `/private/tmp` path mismatch | macOS | `macos-testing` |
| `\r\n` or CRLF in assertion mismatch | Windows | `windows-testing` |
| `sync.Once` race in lipgloss | Windows | `windows-testing` |
| Timer/sleep-based flakiness | macOS or Windows | `macos-testing` + `windows-testing` |
| `-race` timeout on large suite | Windows | `windows-testing` |
| gotestsum false-FAIL on parallel subtests | macOS | `macos-testing` |
| `flock` contention | Linux | `linux-testing` |
| CUE thread-safety race | Any (serial subtests) | `go-testing` |
| `os.UserHomeDir` ignores `HOME`, or expected `~/.invowk/cmds` is missing on Windows | Windows | `windows-testing` |

If the failure is platform-independent (logic bug, missing nil check, wrong
algorithm), skip platform investigation entirely.

### CI Failure Fast Path

When the input is a CI failure (PR checks, run URL, or "CI is failing"):

```bash
# Get failing checks for a PR
gh pr checks <pr-number> --json name,bucket,state,workflow,link \
  --jq '.[] | select(.bucket == "fail")'

# View failed run logs (most recent)
gh run list --branch $(git branch --show-current) --status failure --limit 1 --json databaseId --jq '.[0].databaseId'
gh run view <run-id> --log-failed 2>&1 | head -200

# Download test artifacts
gh run download <run-id> --pattern 'test-results-*' -D /tmp/ci-artifacts/
```

Extract from the logs: test name, package, platform (from job name), error type
(panic, timeout, race, assertion failure), and exit code.

---

## Phase 2: Diagnosis

Spawn parallel subagents based on the evidence. Not every failure needs all
agents — spawn only what's relevant.

### Subagent Dispatch Table

| Subagent | When to Spawn | Focus |
|----------|--------------|-------|
| **Platform Investigator** | Platform-specific symptoms detected | Read the relevant platform skill's Failure Matrix. Match symptoms to known causes. Check if the fix pattern is documented. |
| **Code Path Tracer** | Failing test identified | Read the failing test AND the production code it exercises. Trace the execution path. Identify data flow, concurrency, and resource lifecycle. |
| **Pattern Matcher** | Multiple failures or flaky behavior | Search git log for similar past fixes (`git log --oneline --grep="fix(test)" -20`). Check if the same test failed before. Look for patterns across the failure set. |

### Subagent Prompts

Read [references/subagent-dispatch.md](references/subagent-dispatch.md) and use
the standard prompt for the selected role. Pass only the failure evidence and
task-local context needed for an independent diagnosis; do not leak the parent
agent's suspected cause or preferred fix.

### When NOT to Use Subagents

For simple, obvious bugs (typo, missing nil check, wrong variable name), skip
the subagent overhead entirely. Apply the fix directly and verify. Use subagents
when:
- The root cause is not immediately obvious
- The failure is platform-specific or intermittent
- Multiple tests fail simultaneously
- The failure involves concurrency or timing

---

## Phase 3: Fix Design

After diagnosis, design the fix with three components. For diagnosis-only
requests, stop after reporting these components.

### 1. Root Cause Report

Write a concise report before coding:

```markdown
## Root Cause

**Symptom**: {what the user sees}
**Cause**: {what's actually wrong}
**Platform**: {all | windows | macos | linux}
**Category**: {race condition | resource leak | timing | assertion | logic | configuration}

## Fix

{describe the change — what, where, and why}

## Prevention

{what test, guard, or pattern prevents recurrence}
```

### 2. The Fix Itself

Apply the minimum change that resolves the root cause. Follow the project's
existing patterns:

- **Race condition**: add synchronization (mutex, channel, `atomic`) or make
  tests serial (`//nolint:tparallel` with justification). Consult
  `go-testing` § "Parallelism Decision Framework".
- **Timing flakiness**: replace `time.Sleep` with event-based synchronization
  (channels, `sync.WaitGroup`, poll loops with deadlines). Consult
  `macos-testing` § "Timer Coalescing" and `windows-testing` § "Timer Resolution".
- **Platform mismatch**: add `skipOnWindows`, `[!windows]`/`[windows]`
  conditionals, or platform-agnostic assertions. Consult the relevant platform
  skill's Failure Matrix.
- **Container hang**: add `ContainerTestContext`, `WaitDelay`, or engine health
  probe. Consult `linux-testing` § "Container Test Infrastructure".
- **Context misuse**: replace `context.Background()` with `t.Context()` or
  bounded context. Consult `go-testing` § "Context Patterns Decision Tree".

### 3. Prevention

Every fix should include at least one prevention measure:

| Prevention Type | When to Use | Example |
|----------------|------------|---------|
| Regression test | Logic bugs, nil checks | Add test case to existing table-driven test |
| Platform guard | OS-specific behavior | `skipOnWindows: true` with reason |
| Linter rule | Pattern-level prevention | New golangci-lint check or goplint rule |
| CI configuration | Timeout/resource issues | Adjust `-timeout`, `-parallel`, semaphore |
| Documentation | Subtle gotcha | Add entry to platform skill Failure Matrix |

### 4. Test-Writing Guardrails

When the fix involves writing or modifying test code, apply the testing skill's
Pre-Write Checklist (`.agents/skills/testing/SKILL.md` § "Pre-Write Checklist")
before committing. The most common "fix creates new problem" patterns are:

| Mistake | Consequence | Prevention |
|---------|-------------|------------|
| Adding `t.Parallel()` without checking safety | Data races, crashes, multi-round follow-up | Consult `go-testing` § Parallelism Decision Framework BEFORE adding |
| Leaving stale `//nolint:` after removing suppressed code | `nolintlint` CI failure | Run `make lint` after any code removal near `//nolint` directives |
| New test helper without `t.Helper()` | Confusing failure locations in CI output | First statement in any helper that calls `t.Error`/`t.Fatal` |
| `t.Errorf` before nil dereference | Panic in test binary | Use `t.Fatalf` when next line dereferences the result |
| Orphaned imports after moving tests | Compilation failure | Run `go build ./path/...` after moving test functions |

---

## Phase 4: Apply & Verify

Enter this phase only when the user authorized implementation. After applying
the fix, run verification. A diagnosis-only request ends after Phase 3.

### Targeted Verification

Run the minimum verification that confirms the fix:

```bash
# Run the specific failing test
go test -v -race -run TestName ./path/to/package/...

# Run the full package (catches side effects)
go test -v -race ./path/to/package/...

# If the fix touches multiple packages
go test -v -race ./path/to/pkg1/... ./path/to/pkg2/...
```

### Full Verification (before commit)

Read and follow `.agents/rules/checklist.md`; the exact gate set depends on the
changed surface. For ordinary Go fixes, the usual minimum is:

1. `make lint` — fix ALL issues
2. `make test` — full test suite (NOT `-short`)
3. `make check-baseline` — no new goplint findings
4. `make check-file-length` — all files under 1000 lines

If the fix required new tests or modified test infrastructure, also run:
- `make test-cli` (if CLI tests changed)
- `make sonar-local` (if production code changed)
- `make check-agent-docs` (if `.agents/` files changed)

### Cross-Platform Verification

If the bug was platform-specific, verify the fix doesn't break other platforms.
Since local testing is limited to the current OS:

- Check that any new `skipOnWindows`/`[!windows]` guards are justified
- Verify assertions are platform-agnostic (use `filepath.Join`, not hardcoded separators)
- If adding a `*_<platform>_test.go` file, ensure a corresponding `*_<other_platform>_test.go` exists or the behavior is covered by the generic test

---

## Failure Category Quick Reference

For rapid diagnosis, map the failure type to the right investigation path:

### Test Failures

| Category | First Action | Skills to Consult |
|----------|-------------|------------------|
| Assertion mismatch | Read test + production code | `testing`, `go-testing` |
| Race detector report | Read the two conflicting goroutine stacks | `go-testing` (race-detector-guide.md) |
| Timeout (exit code 1) | Check `-timeout` value and test duration | `go-testing` (test-flags-matrix.md) |
| Timeout (exit code 137) | OOM killer — check memory pressure | `linux-testing` (OOM Killer section) |
| Panic / nil pointer | Read stack trace, trace nil source | `go` |
| Flaky (passes sometimes) | Check for timing, concurrency, or platform issues | platform skills + `go-testing` |

### CI-Specific Failures

| Category | First Action | Skills to Consult |
|----------|-------------|------------------|
| Windows-only failure | Check path separators, CRLF, TerminateProcess | `windows-testing` |
| macOS-only failure | Check kqueue, timer coalescing, /tmp symlink | `macos-testing` |
| Container test hang | Check WaitDelay, ContainerTestContext | `linux-testing` |
| All tests `(unknown)` | Binary killed — timeout or OOM | `go-testing` + `linux-testing` |
| Lint failure | Read linter output, check `.golangci.toml` | `go` |
| Baseline regression | Run `make check-types-json`, triage findings | `tools/goplint/AGENTS.md` |

### Build Failures

| Category | First Action | Skills to Consult |
|----------|-------------|------------------|
| Compilation error | Read error message, check imports | — (straightforward) |
| CUE schema mismatch | Run `/schema-sync-check` | `cue` |
| Missing dependency | `make tidy` | — |

---

## Relationship to Other Commands and Skills

| Command/Skill | Relationship |
|---|---|
| `/fix-it` | Legacy command — simple one-line prompt. The `fixer` skill provides the structured version with subagents and platform awareness. |
| `/fix-it-simple` | Legacy command — concise variant. Use when you want a quick fix without the full workflow. |
| `ci-readiness` agent | Runs pre-commit checks. The fixer skill uses these same checks in Phase 4 verification. |
| `code-reviewer` agent | Reviews code diffs. Can be spawned as an additional subagent in Phase 2 for complex fixes. |
| `go-testing` skill | Primary reference for test toolchain issues. The fixer consults this for context/parallelism patterns. |
| `windows-testing` / `macos-testing` / `linux-testing` | Platform diagnostic lookup tables. The fixer routes to these based on failure symptoms. |
| `testing` skill | Invowk-specific test patterns. The fixer consults this for testscript and container testing. |

---

## Reference Files

- **[references/ci-failure-diagnosis.md](references/ci-failure-diagnosis.md)** —
  How to query CI failures with `gh` CLI, parse JUnit XML artifacts, interpret
  rerun reports, and map CI job names to platforms. Read when diagnosing CI failures.
- **[references/subagent-dispatch.md](references/subagent-dispatch.md)** —
  Detailed subagent configurations, prompt templates for edge cases, and
  dispatch decision flowchart. Read when spawning subagents for complex failures.
- **[references/failure-pattern-catalog.md](references/failure-pattern-catalog.md)** —
  Comprehensive catalog of failure patterns seen in the invowk project, organized
  by category. Each entry has: symptom, root cause, fix template, and platform
  skill cross-reference. Read for pattern matching during diagnosis.
