---
name: module-security
description: >-
  Module-system security auditing and supply-chain hardening for invowk.
  Use when running or implementing `invowk audit` (`internal/audit/`,
  `cmd/invowk/audit.go`), reviewing module vulnerabilities or SC-01..SC-10
  threat-model drift, supply-chain review of module diffs, security docs
  consistency, or changing module discovery, vendoring, lock files, script
  resolution, module symlink handling (copy/provision/scan), or command scope
  enforcement. Starts from `invowk audit --format json`.
---

# Module Security

**Tool-first, LLM-second.** The compiled scanner (`invowk audit`) is the
deterministic source of findings: identical input gives identical output. Use
LLM analysis only for interpretive work a regex scanner cannot do.

Go changes here follow the `go` skill (error handling, context propagation,
declaration order) and the path-loaded testing, licensing (SPDX) and
package-design rules; on conflict, AGENTS.md governance precedence applies.

## Path Boundary Guardrail

When changing scanner code that resolves file-based scripts, module roots, or
symlinks, compare like with like: lexical paths before following symlinks, and
evaluated paths after following symlinks. A target resolved with
`filepath.EvalSymlinks` must be checked against an evaluated module/root path,
not the raw path from discovery or tests. This prevents false boundary escapes
on macOS (`/var` -> `/private/var`) and Windows short/long temp path aliases
while preserving symlink-escape blocking.

Regression tests for this area must cover:
- a normal in-module script,
- a symlink/traversal escape that stays blocked,
- a legitimate script read through a symlinked module/root path.

## Scope

| Code Area | What to Audit |
|-----------|--------------|
| `pkg/invowkmod/` | Lock files, content hashes, vendoring, module operations |
| `pkg/invowkfile/` | Script path resolution, validation, filesystem checks |
| `internal/provision/` | Container provisioning, module copying, symlink handling |
| `internal/app/modulecache/` | Module cache `copyDir` (SC-05 surface) |
| `internal/runtime/virtual_policy.go`, `internal/runtime/sh.go`, `internal/runtime/lua.go`, `internal/runtime/lua_io.go` | Virtual host-binary policy, virtual-lua bridge semantics, and path harness |
| `internal/app/deps/` | Command scope enforcement |
| `internal/discovery/` | Global module trust, `IsGlobalModule` propagation |
| `internal/audit/` | Audit scanner package; derive current file inventory with `find internal/audit -maxdepth 1 -type f -name '*.go' | sort` |
| `internal/audit/checks_llm.go`, `internal/audit/llm_prompt.go`, `internal/audit/prompts/`, `internal/auditllm/`, `internal/llm/` | Opt-in LLM audit path (`--llm`, `--llm-provider`); untrusted module content flows into the prompts |
| `cmd/invowk/audit.go` | Top-level CLI command, optional LLM audit flags, and diagnostic rendering |

---

## Workflow

1. **Phase 1, deterministic scan:** run `invowk audit --format json`. Always
   first; the single source of truth for findings.
2. **Phase 2, triage:** main agent, no subagents. Classify each finding.
3. **Phase 3, interpretive review:** only for gaps Phase 2 found, at most 3
   subagents (Threat Model Drift Checker always; Supply-Chain Reviewer for code
   changes; Documentation Drift Checker when drift is reported).
4. **Phase 4, report:** main agent synthesizes.

Phases 1-2 are fully deterministic.

---

## Phase 1: Deterministic Scan

**Always runs first. This is the single source of truth for findings.**

### Step 1: Build the binary (if needed)

If no built `invowk` binary is available in PATH, build it:

```bash
go build -o /tmp/invowk-audit-bin .
AUDIT_BIN=/tmp/invowk-audit-bin
```

If a trusted repo-local binary already exists, set `AUDIT_BIN=./bin/invowk` or
`AUDIT_BIN=$(command -v invowk)`. Use the same binary for all Phase 1 scans.

### Step 2: Run the scanner

Always use JSON format and `--severity info` to capture all findings; the
default `--severity` is `low`, and filtering happens in Phase 2:

```bash
"$AUDIT_BIN" audit --format json --severity info {scan_path}
```

For broader scans that include global modules:

```bash
"$AUDIT_BIN" audit --format json --severity info --include-global {scan_path}
```

Do not hide stderr on routine runs. If the command exits 2, stderr is evidence
for a scan error and must be included in the report.

**Exit codes** (`runAudit` in `cmd/invowk/audit.go`):
- 0: no *confirmed* finding at or above `--severity` after triage. Suppressed
  by-design findings never count.
- 1: confirmed findings at or above the threshold
  (`triage.HasConfirmedFindings(minSev)`).
- 2: invalid flags or LLM configuration, scanner construction failure,
  unknown `--format`, render failure, or a fatal scan error
  (`audit.ScanFailureIsFatal`).

Non-fatal checker failures print `warning: some checkers failed: …` on stderr
and still exit 0 or 1, so read stderr on every run.

### Step 3: Parse the JSON output

The scanner produces structured JSON with this schema (`auditJSONOutput` in
`cmd/invowk/audit.go`):

```json
{
  "findings": [
    {
      "severity": "critical|high|medium|low|info",
      "category": "integrity|path-traversal|exfiltration|execution|trust|obfuscation",
      "code": "stable machine-readable finding code",
      "surface_id": "file path or SC-ID",
      "surface_kind": "root_invowkfile|local_module|vendored_module|global_module",
      "checker_name": "producing checker name",
      "file_path": "affected file",
      "line": 0,
      "title": "one-line description",
      "description": "detailed explanation",
      "recommendation": "fix suggestion",
      "escalated_from": ["original finding titles (compound findings only)"],
      "escalated_from_codes": ["original finding codes (compound findings only)"]
    }
  ],
  "compound_threats": [...],
  "suppressed_findings": [
    {
      "finding": {...},
      "disposition": "suppressed",
      "rule": "R7",
      "rationale": "why the deterministic policy suppressed it"
    }
  ],
  "suppressed_compound_threats": [...],
  "diagnostics": [
    {
      "severity": "warning|error",
      "code": "stable diagnostic code",
      "message": "diagnostic text",
      "path": "optional path"
    }
  ],
  "summary": {
    "total": 0, "suppressed": 0, "critical": 0, "high": 0, "medium": 0, "low": 0, "info": 0,
    "modules_scanned": 0, "invowkfiles_scanned": 0, "scripts_scanned": 0,
    "duration_ms": 0
  }
}
```

Top-level keys other than `findings` and `summary`, and the finding fields
`surface_id`, `surface_kind`, `file_path`, `line`, `escalated_from` and
`escalated_from_codes`, are `omitempty`, so parsers must tolerate missing
keys. A missing `surface_kind` is treated as `root_invowkfile` by triage.

### What Phase 1 covers (no LLM needed)

The Go scanner already handles all of these deterministically
(`DefaultCheckers()` in `internal/audit/scanner.go`):

| Checker | What It Does | Source |
|---------|-------------|--------|
| LockFileChecker | Hash mismatches, orphans, missing entries, unknown or hashless v1.0 version, size, unparseable lock, missing lock with dependencies | `checks_lockfile.go` |
| ScriptChecker | Path traversal, remote exec, obfuscation, file size | `checks_script.go` |
| NetworkChecker | Reverse shells, DNS exfil, encoded URLs, network commands | `checks_network.go` |
| EnvChecker | Sensitive vars, env_inherit_mode, token extraction | `checks_env.go` |
| LuaChecker | virtual-lua disabled API references, bridge env reads, host-binary opt-outs, network-capable host binaries, and full virtual filesystem access | `checks_lua.go` |
| SymlinkChecker | Symlinks, boundary escapes, dangling, chains | `checks_symlink.go` |
| ModuleMetadataChecker | Dependency fan-out (>5 direct requires), typosquatting, global trust, version pinning, undeclared transitive or vendored modules, invowkfile parse failure | `checks_module.go` |
| Correlator | Compound threat escalation from the current rule set | `correlator.go`, `correlator_constants.go` |

`LLMChecker` (`checks_llm.go`) is opt-in: `--llm` or `--llm-provider` adds it
to the scanner; it is not in `DefaultCheckers()`.

---

## Phase 2: Classify & Triage

**Main agent work. No subagents.** The R1-R12 policy is implemented in
`internal/audit` via `ClassifyReportFindings`; `invowk audit --format json`
emits confirmed findings in `findings` / `compound_threats` and by-design
suppressed findings in `suppressed_findings` /
`suppressed_compound_threats`. Use those fields first instead of manually
re-implementing the table.

When reviewing code changes to `internal/audit/`, verify the classifier still
matches this table. For external scan output, parse the Phase 1 JSON and place
each finding into one of three buckets:

### Bucket 1: Confirmed findings (report as-is)

Findings that reflect real security concerns for the scan target, such as the
always-report codes in R2-R5. Include them verbatim in the final report.

### Bucket 2: Expected/by-design findings (suppress with explanation)

Findings that the Go scanner correctly flags but that are expected for this
specific project context. Suppress these from the report but list them in a
"Suppressed (by-design)" section with justification.

**Classification is deterministic. Do not use judgment; follow the table
exactly.** Rules match on finding **codes** (titles shown for reading text
reports), and `classifyFinding` in `internal/audit/triage.go` evaluates them
in this order, first match wins:

1. R1, then R2-R5 (any surface).
2. Root surface only (`surface_kind` is `root_invowkfile` or omitted): R7, R8,
   R9; then, if every `escalated_from_codes` entry is suppressed, R10 or R11 by
   code, otherwise R6.
3. R12 for everything else, including unknown surface kinds.

| Rule | Surface | Match | Classification | Rationale |
|------|---------|-------|----------------|-----------|
| R1 | `local_module`, `vendored_module`, `global_module` | any finding | **Confirmed** | Modules are the supply-chain surface |
| R2 | any | `network-execution-reverse-shell-pattern-detected` ("Reverse shell pattern detected") | **Confirmed** | Always report regardless of location |
| R3 | any | `script-execution-script-downloads-and-executes-remote-code` ("Script downloads and executes remote code") | **Confirmed** | Always report regardless of location |
| R4 | any | `lockfile-integrity-module-content-hash-mismatch` ("Module content hash mismatch") | **Confirmed** | Tamper indicator, always report |
| R5 | any | `symlink-path-traversal-symlink-points-outside-module-boundary` ("Symlink points outside module boundary") | **Confirmed** | Escape vector, always report |
| R7 | root | `env-exfiltration-command-uses-default-env-inheritance-all-host-variables` ("Command uses default env inheritance (all host variables)") | **Suppressed** | Root invowkfile is user-controlled, env inheritance is the default |
| R8 | root | `env-exfiltration-script-accesses-sensitive-environment-variable` ("Script accesses sensitive environment variable") | **Suppressed** | SSH/container commands intentionally forward credentials |
| R9 | root | `network-exfiltration-possible-dns-exfiltration-pattern` ("Possible DNS exfiltration pattern") | **Suppressed** | container env command legitimately uses DNS |
| R10 | root | `correlator-credential-exfiltration` ("Potential credential exfiltration") and every constituent code suppressed | **Suppressed** | Correlator artifact from by-design constituents |
| R11 | root | `correlator-high-plus-other` ("High-severity finding combined with other issues") and every constituent code suppressed | **Suppressed** | Generic escalation of by-design constituents |
| R6 | root | any other compound whose constituent codes are all suppressed | **Suppressed** | Compound escalation of by-design constituents |
| R12 | any | any other finding | **Confirmed** | Default: report it |

**Compound inheritance** (`compoundConstituentsSuppressed`): a compound is
suppressed only when every code in `escalated_from_codes` is suppressed; any
confirmed constituent keeps it confirmed. Plain findings are classified before
compound ones. The suppressed-code set is global to the report: any suppressed
instance of a code counts, whatever its surface. A compound with empty
`escalated_from_codes` is never suppressed.

### Bucket 3: Scanner gaps (flag for Phase 3 investigation)

Areas the Go scanner cannot cover that need interpretive review:

| Gap | Why Scanner Can't Cover It | Phase 3 Handler |
|-----|---------------------------|-----------------|
| Threat-model status or key files drifted | Scanner checks content, not SKILL.md accuracy | Threat Model Drift Checker |
| New attack surface not in SC-01..SC-10 | Scanner only checks known patterns | Threat Model Drift Checker |
| SC-08 interpreter basename residual | No compiled checker inspects interpreters; only the parse-time allowlist does | Threat Model Drift Checker |
| Code change introduces regression in mitigation | Scanner checks current state, not diffs | Supply-Chain Reviewer |
| Documentation references stale security status | Scanner doesn't read docs | Documentation Drift Checker |

---

## Phase 3: Interpretive Review

**Runs only for gaps identified in Phase 2.** Maximum 3 subagents, each doing
work that genuinely requires reading comprehension and contextual judgment.

### Subagents

Use `references/subagent-prompts.md` for the full prompts. Keep this file to the
dispatch rules so the prompt text has one source of truth.

| Subagent | When | Purpose |
|----------|------|---------|
| Threat Model Drift Checker | Always in Phase 3 | Verify SC-01..SC-10 through current call paths and focused behavior tests; report drift or incomplete proof. |
| Supply-Chain Reviewer | Only for diffs touching module/security scope | Check whether the change regresses a mitigation, adds an uncovered attack surface, or changes trust boundaries. |
| Documentation Drift Checker | Only when threat-model drift is reported | Find stale references in `AGENTS.md`, `.agents/agents/supply-chain-reviewer.md`, module-security docs, and related user docs. |

---

## Phase 4: Report Assembly

The main agent synthesizes all outputs into the final report. No subagents.

### Report Structure

```
Module Security Audit — {scan_path}
Scanner: invowk audit v{version} (deterministic, {duration}ms)
Scanned: {N} modules, {N} invowkfiles, {N} scripts

▲ CRITICAL ({count})
  {title}
         File: {path}:{line}
         {description}
         Fix: {recommendation}

● HIGH ({count})
  ...

◆ MEDIUM ({count})
  ...

═══ Suppressed (by-design) ═══
  {count} findings suppressed — see breakdown:
  - {N}× "Command uses default env inheritance" (root invowkfile, Info)
  - {N}× "Script accesses sensitive env variable" (SSH commands, Medium)
  Justification: Root invowkfile commands are user-controlled; credential
  forwarding in SSH/container commands is documented and intentional.

═══ Threat Model Status ═══
  SC-01: CONFIRMED (Mitigated) — cite current function/symbol evidence
  SC-02: CONFIRMED (Partial) — cite current function/symbol evidence
  ...

═══ Supply-Chain Review ═══ (if applicable)
  {Supply-Chain Reviewer output}

Summary: {critical} critical, {high} high, {medium} medium
         {suppressed} suppressed (by-design)
         All 10 attack surfaces: {confirmed}/{total} confirmed
```

### Deduplication Rules

1. Phase 1 findings are the source of truth — never duplicate them
2. Phase 3 subagents must only add NEW information not in Phase 1
3. If a supply-chain reviewer finding matches a Phase 1 finding, keep Phase 1's
   version (it has structured fields) and append the reviewer's commentary

### Determinism Contract

Running this workflow twice on the same codebase at the same commit MUST produce:
- **Identical** Phase 1 output (same JSON, same findings, same counts)
- **Identical** Phase 2 classification (same suppress/confirm/flag decisions)
- **Nearly identical** Phase 3 output (subagent analysis may vary slightly in
  phrasing but NOT in which surfaces are CONFIRMED vs DRIFTED, because each
  verdict rests on a traced call path and a focused behavior test, not on
  grep presence)

If a run produces different findings from a previous run on the same commit,
inspect Phase 3 evidence. Tighten call-path requirements and focused behavior
tests rather than adding more subagents or treating symbol presence as proof.

---

## Known Attack Surfaces

These 10 surfaces are the current threat model and this table is their
canonical record. The Phase 3 Threat Model Drift Checker verifies them at every
audit through call-path tracing and focused behavior tests.

| ID | Surface | Severity | Key File(s) | Status |
|----|---------|----------|-------------|--------|
| SC-01 | Script path traversal | High | `pkg/invowkfile/implementation.go`, script path validation helpers; modelled by `formal/alloy/ModulePathContainment.als` (findings F13, F17) | Mitigated (findings F13/F17 open) |
| SC-02 | Virtual host-binary policy | Medium | `internal/runtime/virtual_policy.go`, `internal/runtime/sh.go`, `internal/runtime/lua.go` | Partial |
| SC-03 | InvowkDir R/W volume mount | Medium | `internal/runtime/container*.go` | By-design |
| SC-04 | SSH token and TUI credentials in container/virtual env | Medium | `internal/runtime/container*.go`, `internal/runtime/runtime.go`, interactive adapters | Partial |
| SC-05 | Provision `CopyDir` symlink handling | Medium | `internal/provision/helpers.go` `CopyDir`, `internal/app/modulecache/cache.go` `copyDir`, scan context artifact copy paths; modelled by `formal/alloy/ModulePathContainment.als` (`copySkipsInnerLinks`) | Mitigated (residual: `copyDir` skips inner links but follows a symlinked source root, per the model's abstraction note) |
| SC-06 | `--ivk-env-var` priority override | Low | `internal/runtime/env_builder.go` | By-design |
| SC-07 | Custom-check `script.content` host execution | High | `internal/app/deps/checks.go`, `internal/app/commandadapters/dependency_host.go` | Partial |
| SC-08 | Arbitrary interpreter paths | Medium | `pkg/invowkfile/interpreter_spec.go`, `pkg/invowkfile/runtime.go` | Mitigated (allowlist in Validate; residual: it checks the interpreter basename only, so an absolute path such as `/tmp/python3` passes) |
| SC-09 | Root invowkfile scope bypass | Low | `internal/app/deps/deps.go` | By-design |
| SC-10 | Global module trust (no integrity) | Medium | `internal/discovery/discovery_files.go`, command-scope lock checks | Partial |

**Status legend:** Open (no mitigation), Partial (gaps remain), Mitigated (fixed, residual gap only), By-design (intentional, document only)

Before citing status, run the behavior-backed Phase 3 prompts and current
scanner tests.
Do not preserve date-stamped status notes as evidence if the code has moved.

---

## References

Load both `check-catalog.md` and `implementation-review.md` when reviewing or
modifying `internal/audit/` or `cmd/invowk/audit.go`. Neither is needed to run
an audit.

- [`references/check-catalog.md`](references/check-catalog.md): what each
  compiled checker and correlation rule detects, with severities, finding
  titles, and pattern families.
- [`references/implementation-review.md`](references/implementation-review.md):
  per-checker correctness, regex false positives/negatives, concurrency and
  `ScanContext` immutability, performance, scanner self-defense, CLI exit and
  JSON contract, and live test inventory commands.
- [`references/subagent-prompts.md`](references/subagent-prompts.md): Phase 3
  prompts, which require call-path inspection and focused behavior tests
  rather than grep-presence assertions.

Derive the current implementation inventory from source. Do not copy file
counts, line counts, or checker lists into a report without regenerating them.

---

## Security Review Workflow (for Code Changes)

When reviewing module-related changes, adapt the workflow:

1. **Phase 1** — Run `"$AUDIT_BIN" audit --format json --severity info .`
2. **Phase 2** — Classify findings; identify which are new vs pre-existing
   (compare against a baseline run or previous audit output if available)
3. **Phase 3** — Threat Model Drift Checker (always) + Supply-Chain Reviewer
   with the diff (code changes only)
4. **Phase 4** — Report focusing on NEW findings introduced by the change

**File → Relevant SC-IDs** for targeted Phase 3 review:

| Changed File | SC-IDs to Verify |
|-------------|------------------|
| `pkg/invowkmod/lockfile.go`, `content_hash.go` | SC-10 (integrity) |
| `pkg/invowkfile/implementation.go`, `runtime.go` | SC-01 (path traversal), SC-08 (interpreters) |
| `internal/provision/helpers.go`, `internal/app/modulecache/` | SC-05 (symlinks) |
| `internal/runtime/virtual_policy.go`, `internal/runtime/sh.go`, `internal/runtime/lua.go`, `container_exec.go` | SC-02, SC-03, SC-04 (runtime surfaces) |
| `internal/discovery/`, `internal/app/deps/` | SC-09, SC-10 (scope, trust) |

---

## Common Pitfalls

| Pitfall | Fix |
|---------|-----|
| Regressing symlink skipping in module copy/provision paths | Keep copy helpers and scan-context artifact/clone paths from following symlinked module content outside trusted roots |
| Reading a module script without the containment check | `ScriptFilePath.Validate` rejects absolute paths and `..` segments at parse time, and `ScriptChecker` still flags both (High). `GetScriptFilePathWithModule` only resolves, by design; file I/O must go through `ResolveScriptWithModule` / `ResolveScriptWithFSAndModule` (`validateScriptPathContainment`, SC-01) |
| New `checks_*.go` missing SPDX header | `// SPDX-License-Identifier: MPL-2.0` as first line |
| Hiding corrupt module files | Keep `loadScannedModule` recording `InvowkfileParseErr` / `LockFileParseErr` so they surface as "Module invowkfile failed to parse" and "Lock file present but unparseable"; modules without these files stay valid |
| Scan context hides invalid modules/files | Preserve structured diagnostics in `Report.Diagnostics` and CLI JSON/text output |
| Exposing shared `ScanContext` state | Keep `Modules()`, `Invowkfiles()` and `AllScripts()` returning clones (immutability contract); checkers share the context concurrently |
| Levenshtein runs O(n²) on module list | Acceptable for small module sets; for >100 modules consider short-circuiting or length pre-filter |

## Related Skills

`cli` (Cobra wiring, flags, styled output), `discovery` (precedence, vendored
module scanning), `container` (volume mounts, provisioning), `dep-audit` (Go
dependencies in `go.mod`, not invowk modules), `testing` and `go-testing`
(test patterns, testscript, race detector, coverage).

## Skill Integrity Check

After changing this skill or the audit checker inventory, run the following.
The validator is wired into no make target or CI job, so this instruction is
its only enforcement:

```bash
python3 .agents/skills/module-security/scripts/validate-references.py
make check-agent-docs
```
