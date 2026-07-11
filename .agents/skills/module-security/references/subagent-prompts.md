# Subagent Prompt Templates

Use these prompts only for Phase 3 interpretation. Phase 1 uses the compiled
scanner. Symbol searches locate code; they do not prove a mitigation works.

## Threat Model Drift Checker

**When:** Every full module-security audit.

```
You are verifying Invowk's SC-01..SC-10 module-security threat model.
Work read-only. A function name or grep match is discovery evidence, never
sufficient proof of behavior.

For each surface:

1. Locate the current implementation and all call sites with `rg`.
2. Trace the relevant input from its trust boundary to the enforcement or
   explicitly documented by-design behavior.
3. Locate focused tests. Read the assertions and identify the positive case,
   denied/negative case, and boundary case. For path/symlink controls, require
   both a blocked escape and a legitimate symlinked-root case where applicable.
4. Run the smallest focused Go tests that exercise those assertions. Record the
   exact command and result. If no focused test exists, do not mark a mitigation
   CONFIRMED merely because code is present.
5. Compare the resulting behavior with the status in the current Known Attack
   Surfaces table.

Surface routing:

- SC-01: script path containment in `pkg/invowkfile` and audit script checks.
- SC-02: virtual host-binary allowlist/wildcard policy across virtual-sh and
  virtual-lua execution paths.
- SC-03: Invowk directory container mount mode and documented host-write reach.
- SC-04: SSH/TUI credential propagation into container/virtual environments.
- SC-05: symlink handling in provision/copy and audit scan-context artifact paths.
- SC-06: `--ivk-env-var` precedence in runtime environment construction.
- SC-07: custom-check script content reaching native host execution.
- SC-08: interpreter validation and the execution call path that consumes it.
- SC-09: root invowkfile command-scope behavior in dependency validation.
- SC-10: global-module trust and vendored hash verification boundaries.

Run package tests selected from the located assertions, for example:
`go test ./pkg/invowkfile -run TestName` or
`go test ./internal/audit -run TestName`. Do not invent a test name; derive it
from source.

Output one record per surface:

SC-NN: CONFIRMED|DRIFTED|INCOMPLETE (status)
Implementation: file:symbol and call-path summary
Behavior proof: focused test name and assertion summary
Command: exact test command and PASS|FAIL
Reason: one sentence

Definitions:
- CONFIRMED: implementation, call path, and focused behavior proof agree.
- DRIFTED: current behavior contradicts the documented surface/status.
- INCOMPLETE: implementation may exist, but behavior proof or call-path
  evidence is missing or failed.

Report only threat-model accuracy. Do not re-report scanner findings or general
code-quality observations.
```

## Supply-Chain Reviewer

**When:** A diff touches module/security scope.

```
You are reviewing a module-system diff for supply-chain risk. Work read-only.

Inputs:
- Phase 1 JSON findings: {paste summary and medium+ findings}
- Diff or changed files: {paste raw diff or exact range}

Do not re-report Phase 1 findings. Answer only:

1. Does the diff weaken an SC-01..SC-10 mitigation? Trace the changed call path
   and run existing focused tests. For path boundaries, verify evaluated paths
   are compared with evaluated roots and tests cover both blocked escape and
   legitimate symlinked-root behavior.
2. Does the diff add a new route from untrusted module content to host
   execution, file writes, secrets, or network access outside the current
   default checkers? Derive the checker list from `DefaultCheckers()`; do not use
   a remembered count.
3. Does the diff change command visibility, global-module trust, dependency
   declarations, or vendored hash enforcement?

For each issue, report Title, SC-ID or NEW-SURFACE, File:Line, call path,
behavioral evidence, and test result. If all three are clean, report:
`No additional supply-chain risks beyond scanner findings`.
```

## Documentation Drift Checker

**When:** The threat-model checker reports DRIFTED or INCOMPLETE.

```
You are checking documentation affected by verified module-security drift.
Work read-only.

Inputs:
- DRIFTED/INCOMPLETE records with implementation and test evidence

Check current references in:
1. `.agents/agents/supply-chain-reviewer.md`
2. `.agents/skills/module-security/SKILL.md`
3. `AGENTS.md` security-model sections
4. User documentation named by the code-to-doc sync map for the changed surface

Use search only to locate claims. Compare each claim with the supplied
behavioral evidence. Report File:Line, current claim, evidence-backed correction,
and owning source of truth. Do not infer a new status from symbol presence.

If no claim is stale, report `No documentation drift detected`.
```
