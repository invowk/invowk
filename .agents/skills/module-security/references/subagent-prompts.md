# Subagent Prompt Templates

Prompt templates for the Phase 3 interpretive subagents. Phase 1 (deterministic
scan) uses `invowk audit --format json` directly — no subagent needed.

## Table of Contents

- [Subagent A: Threat Model Drift Checker](#subagent-a-threat-model-drift-checker)
- [Subagent B: Supply-Chain Reviewer](#subagent-b-supply-chain-reviewer)
- [Subagent C: Documentation Drift Checker](#subagent-c-documentation-drift-checker)

---

## Subagent A: Threat Model Drift Checker

**When:** Always runs (every audit invocation).
**Purpose:** Verify that the 10 attack surfaces (SC-01..SC-10) are still accurate
by running concrete grep commands — not by reading code and making judgment calls.

```
You are verifying the module security threat model for the invowk project.

## Your Task

Run EACH grep command below and report EXACTLY what the output shows.
Do NOT interpret code, assess quality, or suggest improvements.

## Verification Commands

For each SC-ID, run the grep command and report the result using the
exact output format specified.

SC-01 Script path traversal:
  Run: grep -n "validateScriptPathContainment" pkg/invowkfile/implementation.go
  → Lines found: report line numbers
  → No output: report "NOT FOUND"

SC-02 Virtual shell host PATH fallback:
  Run: grep -n "interp.ExecHandlers\|execHandler" internal/runtime/virtual.go
  → Lines found: report line numbers (ExecHandlers registers middleware, execHandler method falls through to next() for host PATH)
  → No output: report "NOT FOUND"

SC-03 InvowkDir R/W volume mount:
  Run: grep -n "invowkDir" internal/runtime/container_exec.go
  → Lines found: report first match line number
  → No output: report "NOT FOUND"

SC-04 SSH token in container env:
  Run: grep -n "SSH_AUTH_SOCK\|INVOWK_SSH" internal/runtime/container_exec.go
  → Lines found: report count and line numbers
  → No output: report "NOT FOUND"

SC-05 Provision CopyDir symlink handling:
  Run: grep -n "ModeSymlink\|skipSymlinks\|symlink" internal/provision/helpers.go
  → Lines found: report line numbers where skip logic appears
  → No output: report "NOT FOUND — symlink skip may have been removed"

SC-06 --ivk-env-var priority override:
  Run: grep -n "ivk-env-var\|IvkEnvVar" internal/runtime/env_builder.go
  → Lines found: report line numbers
  → No output: report "NOT FOUND"

SC-07 check_script host shell execution:
  Run: grep -n "exec\.Command\|os/exec" internal/app/deps/checks.go
  → Lines found: report line numbers
  → No output: report "NOT FOUND"

SC-08 Arbitrary interpreter paths:
  Run: grep -n "allowedInterpreters\|Validate" pkg/invowkfile/interpreter_spec.go
  → Lines found: report line numbers
  → No output: report "NOT FOUND — allowlist may have been removed"

SC-09 Root invowkfile scope bypass:
  Run: grep -n "CanCall\|CommandScope" internal/app/deps/deps.go
  → Lines found: report line numbers
  → No output: report "NOT FOUND"

SC-10 Global module trust:
  Run: grep -rn "IsGlobalModule\|detectModuleShadowing\|VerifyVendoredModuleHashes" internal/discovery/
  → Lines found: report which functions exist and where
  → No output: report "NOT FOUND"

## Output Format

Report EXACTLY in this format, one line per surface:

SC-01: {CONFIRMED|DRIFTED} ({status}) — {function_name} at line {N}
SC-02: {CONFIRMED|DRIFTED} ({status}) — {function_name} at line {N}
...

Where:
- CONFIRMED = grep found the expected function/pattern
- DRIFTED = grep found nothing, or the pattern has moved significantly
- {status} = Mitigated|Partial|By-design|Open (from the threat model table)

## Rules (CRITICAL for determinism)
- Do NOT add subjective commentary
- Do NOT report "new findings" or "observations"
- Do NOT assess code quality
- Do NOT suggest improvements
- Report ONLY what the grep commands show
- If a grep returns no results, that IS the finding — report it as DRIFTED
- If a grep returns results, report the line numbers — that IS the confirmation
```

---

## Subagent B: Supply-Chain Reviewer

**When:** Only when reviewing code changes (PRs, diffs) that touch module system files.
**Agent file:** `.agents/agents/supply-chain-reviewer.md`

```
You are the supply-chain security reviewer for a code change touching
the invowk module system.

## Deterministic Scanner Results
The following findings were produced by `invowk audit --format json`.
These are ALREADY KNOWN — do NOT re-report them.

{paste Phase 1 JSON summary and findings at medium+}

## Diff to Review
{paste git diff or list of changed files}

## Your Task

Answer ONLY these three questions:

1. **Mitigation regression?** Does this diff remove or weaken any existing
   mitigation for SC-01..SC-10? Check if functions like
   `validateScriptPathContainment`, `VerifyVendoredModuleHashes`,
   `InterpreterSpec.Validate()` are modified in ways that reduce protection.

2. **New attack surface?** Does this diff introduce a new way for
   untrusted module content to reach host execution, file writes, or
   network access that is NOT already covered by the 6 Go checkers
   (LockFile, Script, Network, Env, Symlink, ModuleMetadata)?

3. **Trust boundary change?** Does this diff change who can invoke what?
   Look for changes to `CanCall()`, `CommandScope`, `IsGlobalModule`,
   or the `requires` resolution logic.

## Output Format

For each question, answer with one of:
- "No issues found" — if the diff does not affect this area
- A structured finding with: Title, Affected SC-ID, File:Line, Description

## Rules (CRITICAL for determinism)
- Do NOT re-report findings from the scanner output above
- Do NOT scan for regex patterns (the Go scanner already does that)
- Do NOT report general code quality issues
- ONLY report genuinely new risks that the compiled scanner cannot detect
- If the diff is clean on all three questions: report
  "No additional supply-chain risks beyond scanner findings"
```

---

## Subagent C: Documentation Drift Checker

**When:** Only if Subagent A found any DRIFTED status.

```
You are checking for documentation drift in invowk's module security docs.

## Drifted Surfaces
{paste only the DRIFTED lines from Subagent A output}

## Your Task

For EACH drifted surface, check if these three files reference the old status:

1. `.agents/agents/supply-chain-reviewer.md`
2. `.agents/skills/module-security/SKILL.md` § "Known Attack Surfaces" table
3. `.claude/CLAUDE.md` § "Virtual Runtime Security Model"

For each file, grep for the SC-ID and check if the documented status matches
what Subagent A found.

## Output Format

For each stale reference found:
- **File**: {path}
- **Line**: {number}
- **Current text**: {what it says now}
- **Should be**: {what it should say based on Subagent A findings}

If no documents reference stale status: "No documentation drift detected."

## Rules
- Check ONLY the three files listed above
- Report ONLY status mismatches, not formatting or style issues
- If you cannot find an SC-ID reference in a file, skip it (not all files
  reference all surfaces)
```
