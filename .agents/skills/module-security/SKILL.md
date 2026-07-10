---
name: module-security
description: >-
  Module system security auditing and supply-chain hardening for invowk.
  Use when reviewing module vulnerabilities, implementing `invowk audit`,
  improving `internal/audit/`, or changing module discovery, vendoring, lock
  files, script resolution, command scope enforcement, or related docs. Runs
  `invowk audit --format json` first, then adds interpretive review only for
  threat model drift, supply-chain code review, or documentation consistency.
---

# Module Security

Security auditing and supply-chain attack prevention for invowk's module system.

**Design principle: tool-first, LLM-second.** The compiled Go scanner
(`invowk audit`) is the deterministic backbone — it produces identical output
for identical input every time. LLM analysis is reserved for genuinely
interpretive tasks that a regex scanner cannot do. This eliminates the variance
that comes from asking LLMs to pattern-match code.

## Normative Quick Rules

- `.agents/skills/go/SKILL.md` — error handling, context propagation, declaration ordering
- `.agents/rules/testing.md` — test coverage, testscript patterns
- `.agents/rules/licensing.md` — SPDX headers on all new Go files
- `.agents/rules/package-design.md` — package boundaries for `internal/audit/`
- If this skill conflicts with a rule, **the rule wins**.

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
| `internal/runtime/virtual_policy.go`, `internal/runtime/sh.go`, `internal/runtime/lua.go`, `internal/runtime/lua_io.go` | Virtual host-binary policy, virtual-lua bridge semantics, and path harness |
| `internal/app/deps/` | Command scope enforcement |
| `internal/discovery/` | Global module trust, `IsGlobalModule` propagation |
| `internal/audit/` | Audit scanner package; derive current file inventory with `find internal/audit -maxdepth 1 -type f -name '*.go' | sort` |
| `cmd/invowk/audit.go` | Top-level CLI command, optional LLM audit flags, and diagnostic rendering |

---

## Workflow Overview

```
Input (audit request, code review, security scan)
    │
    ▼
Phase 1: Deterministic Scan ────── run `invowk audit --format json`
    │   (compiled Go scanner, ~40ms, structured JSON output)
    │
    ▼
Phase 2: Classify & Triage ────── main agent, no subagents
    │   (parse JSON, classify findings, identify false positives)
    │
    ▼
Phase 3: Interpretive Review ──── 1-3 subagents (only when needed)
    │   ├── Threat Model Drift Checker (always, 1 agent)
    │   ├── Supply-Chain Reviewer (code changes only)
    │   └── Documentation Drift Checker (when drift detected)
    │
    ▼
Phase 4: Report Assembly ──────── main agent synthesizes
```

**Key difference from subjective scanning:** Phases 1-2 are fully deterministic.
Phase 3 only runs interpretive analysis for tasks the Go scanner cannot do.
This reduces subagent count from 10 to at most 3, and eliminates the primary
source of non-determinism (LLMs doing pattern matching).

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

Run the compiled scanner against the target path. Always use JSON format and
`--severity info` to capture all findings (filtering happens in Phase 2):

```bash
"$AUDIT_BIN" audit --format json --severity info {scan_path}
```

For broader scans that include global modules:

```bash
"$AUDIT_BIN" audit --format json --severity info --include-global {scan_path}
```

Do not hide stderr on routine runs. If the command exits 2, stderr is evidence
for a scan error and must be included in the report.

**Exit codes:** 0 = no findings, 1 = findings detected, 2 = scan error.

### Step 3: Parse the JSON output

The scanner produces structured JSON with this schema:

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

### What Phase 1 covers (no LLM needed)

The Go scanner already handles all of these deterministically:

| Checker | What It Does | Source |
|---------|-------------|--------|
| LockFileChecker | Hash mismatches, orphans, missing entries, version, size | `checks_lockfile.go` |
| ScriptChecker | Path traversal, remote exec, obfuscation, file size | `checks_script.go` |
| NetworkChecker | Reverse shells, DNS exfil, encoded URLs, network commands | `checks_network.go` |
| EnvChecker | Sensitive vars, env_inherit_mode, token extraction | `checks_env.go` |
| LuaChecker | virtual-lua disabled API references, bridge env reads, host-binary opt-outs, network-capable host binaries, and full virtual filesystem access | `checks_lua.go` |
| SymlinkChecker | Symlinks, boundary escapes, dangling, chains | `checks_symlink.go` |
| ModuleMetadataChecker | Dep depth, typosquatting, global trust, version pinning | `checks_module.go` |
| Correlator | Compound threat escalation (5 named rules + severity rules) | `correlator.go` |

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

Findings that reflect real security concerns for the specific scan target.
Include these verbatim in the final report.

**Examples:**
- `curl|bash` in a module script → real remote code execution risk
- Symlink pointing outside module boundary → real escape vector
- Lock file hash mismatch → real tamper indicator
- Reverse shell pattern → always a confirmed finding

### Bucket 2: Expected/by-design findings (suppress with explanation)

Findings that the Go scanner correctly flags but that are expected for this
specific project context. Suppress these from the report but list them in a
"Suppressed (by-design)" section with justification.

**Classification is deterministic.** Apply these rules in order — the FIRST
matching rule wins. Do not use judgment; follow the table exactly.

| Rule # | Condition | Classification | Rationale |
|--------|-----------|---------------|-----------|
| R1 | Finding is in a **module** (not root invowkfile) | **Confirmed** | Modules are the supply-chain surface |
| R2 | Finding code is **`network-execution-reverse-shell-pattern-detected`** | **Confirmed** | Always report regardless of location |
| R3 | Finding code is **`script-execution-script-downloads-and-executes-remote-code`** | **Confirmed** | Always report regardless of location |
| R4 | Finding code is **`lockfile-integrity-module-content-hash-mismatch`** | **Confirmed** | Tamper indicator, always report |
| R5 | Finding code is **`symlink-path-traversal-symlink-points-outside-module-boundary`** | **Confirmed** | Escape vector, always report |
| R6 | Finding has `escalated_from` field (compound threat) AND all constituent findings are suppressed by R7-R10 | **Suppressed** | Compound escalation of by-design constituents |
| R7 | Finding is on root invowkfile AND title is **"Command uses default env inheritance (all host variables)"** | **Suppressed** | Root invowkfile is user-controlled, env inheritance is the default |
| R8 | Finding is on root invowkfile AND title is **"Script accesses sensitive environment variable"** | **Suppressed** | SSH/container commands intentionally forward credentials |
| R9 | Finding is on root invowkfile AND title is **"Possible DNS exfiltration pattern"** | **Suppressed** | container env command legitimately uses DNS |
| R10 | Finding is on root invowkfile AND title is **"Potential credential exfiltration"** AND has `escalated_from` containing only suppressed constituent titles | **Suppressed** | Correlator artifact from by-design constituents |
| R11 | Finding is on root invowkfile AND title is **"High-severity finding combined with other issues"** AND all `escalated_from` entries are suppressed | **Suppressed** | Generic escalation of by-design constituents |
| R12 | Any other finding | **Confirmed** | Default: report it |

**Key principle:** Compound/escalated findings (those with `escalated_from`)
inherit the classification of their constituents. If ALL constituents are
suppressed, the compound finding is also suppressed. If ANY constituent is
confirmed, the compound finding is confirmed.

### Bucket 3: Scanner gaps (flag for Phase 3 investigation)

Areas the Go scanner cannot cover that need interpretive review:

| Gap | Why Scanner Can't Cover It | Phase 3 Handler |
|-----|---------------------------|-----------------|
| Threat model line numbers drifted | Scanner checks content, not SKILL.md accuracy | Threat Model Drift Checker |
| New attack surface not in SC-01..SC-10 | Scanner only checks known patterns | Threat Model Drift Checker |
| Code change introduces regression in mitigation | Scanner checks current state, not diffs | Supply-Chain Reviewer |
| Documentation references stale security status | Scanner doesn't read docs | Documentation Drift Checker |

---

## Phase 3: Interpretive Review

**Runs only for gaps identified in Phase 2.** Maximum 3 subagents, each doing
work that genuinely requires reading comprehension and contextual judgment.

### Subagents

Use `references/subagent-prompts.md` for the full prompts. Keep this file to the
dispatch rules so the grep patterns and prompt text have one source of truth.

| Subagent | When | Purpose |
|----------|------|---------|
| Threat Model Drift Checker | Always in Phase 3 | Verify SC-01..SC-10 with concrete grep commands and report drift only. |
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
  SC-02: CONFIRMED (By-design) — cite current function/symbol evidence
  ...

═══ Supply-Chain Review ═══ (if applicable)
  {Subagent B output}

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
  phrasing but NOT in which surfaces are CONFIRMED vs DRIFTED, because the
  verification uses grep commands with concrete expected output)

If a run produces different findings from a previous run on the same commit,
the problem is in Phase 3 subagent analysis. The fix is to make the subagent
prompt more concrete (add specific grep patterns), not to add more subagents.

---

## Known Attack Surfaces

These 10 surfaces represent the current threat model. The Phase 3 Threat Model
Drift Checker verifies their accuracy via grep commands at every audit.

| ID | Surface | Severity | Key File(s) | Status |
|----|---------|----------|-------------|--------|
| SC-01 | Script path traversal | High | `pkg/invowkfile/implementation.go`, script path validation helpers | Mitigated |
| SC-02 | Virtual host-binary policy | Medium | `internal/runtime/virtual_policy.go`, `internal/runtime/sh.go`, `internal/runtime/lua.go` | Partial |
| SC-03 | InvowkDir R/W volume mount | Medium | `internal/runtime/container*.go` | By-design |
| SC-04 | SSH token and TUI credentials in container/virtual env | Medium | `internal/runtime/container*.go`, `internal/runtime/runtime.go`, interactive adapters | Partial |
| SC-05 | Provision `CopyDir` symlink handling | Medium | `internal/provision/helpers.go`, scan context artifact copy paths | Mitigated |
| SC-06 | `--ivk-env-var` priority override | Low | `internal/runtime/env_builder.go` | By-design |
| SC-07 | Custom-check `script.content` host execution | High | `internal/app/deps/checks.go`, `internal/app/commandadapters/dependency_host.go` | Partial |
| SC-08 | Arbitrary interpreter paths | Medium | `pkg/invowkfile/interpreter_spec.go`, `pkg/invowkfile/runtime.go` | Mitigated (allowlist in Validate) |
| SC-09 | Root invowkfile scope bypass | Low | `internal/app/deps/deps.go` | By-design |
| SC-10 | Global module trust (no integrity) | Medium | `internal/discovery/discovery_files.go`, command-scope lock checks | Partial |

**Status legend:** Open (no mitigation), Partial (gaps remain), Mitigated (fixed, residual gap only), By-design (intentional, document only)

Before citing status, re-run the Phase 3 grep prompts and current scanner tests.
Do not preserve date-stamped status notes as evidence if the code has moved.

---

## `invowk audit` Subcommand Architecture

Implementation reference for the top-level audit command. Read
`references/check-catalog.md` for check specifications that map to
`internal/audit/checks_*.go`. For code quality review guidance, load
`references/implementation-review.md`.

### CLI Layer: `cmd/invowk/audit.go`

`newAuditCommand(app *App, rootFlags *rootFlagValues)` owns the top-level
`invowk audit [path]` command, text/JSON output selection, severity filtering,
global-module inclusion, and optional LLM analysis flags. The command text
documents both provider-based LLM analysis (`--llm-provider`) and configured or
OpenAI-compatible API analysis (`--llm`).

**Exit codes:** 0 = clean, 1 = findings, 2 = scan error (via `ExitError` with typed codes).
**Registration:** `rootCmd.AddCommand(newAuditCommand(app, flags))` in `root.go`.

### Domain Layer: `internal/audit/`

| File | Purpose |
|------|---------|
| `doc.go` | Package comment + SPDX header |
| `severity.go` | `Severity` iota enum, `ParseSeverity()`, JSON marshaling, `InvalidSeverityError` |
| `types.go` | `Category`, `Finding`, `Report` types, filtering, sorting, counting |
| `errors.go` | Sentinels (`ErrScanContextBuild`, `ErrCheckerFailed`, `ErrNoScanTargets`), typed wrappers |
| `checker.go` | `Checker` interface (`Name`, `Category`, `Check`) |
| `scan_context.go`, `scan_context_artifacts.go`, `scan_context_clone.go`, `scan_files.go` | `ScanContext`, `BuildScanContext`, artifact/clone/file scanning helpers, `ScannedModule`, `ScriptRef`, discovery integration |
| `scanner.go` | `Scanner` struct, `Scan()`, concurrent `runCheckers`, functional options |
| `correlator.go` | `Correlator`, `CorrelationRule`, named rules + severity escalation |
| `checks_lockfile.go` | Lock file integrity (hash, version, orphans, size guard) |
| `checks_script.go` | Script path traversal + content analysis (remote exec, obfuscation) |
| `checks_network.go` | Network access, reverse shells, DNS exfiltration, encoded URLs |
| `checks_env.go` | Env var risk, credential extraction, `env_inherit_mode` |
| `checks_symlink.go` | Symlink detection, boundary checking, chain depth |
| `checks_module.go` | Module metadata (deps, typosquatting, global trust, version pinning) |
| `checks_llm.go`, `llm_prompt.go`, `llm_errors.go`, `triage.go` | Optional semantic LLM-backed analysis, prompt building, error handling, and deterministic triage |
| `finding_codes.go` | Stable finding-code derivation/catalog |
| `types.go` | `Finding`, `Report`, `Diagnostic`, `SurfaceKind`, and report filtering/sorting helpers |

### Core Types

```go
type Severity int  // severity.go — ordered for < / > comparison
const (
    SeverityInfo Severity = iota
    SeverityLow
    SeverityMedium
    SeverityHigh
    SeverityCritical
)

type Category string  // types.go
const (
    CategoryIntegrity    Category = "integrity"
    CategoryPathTraversal Category = "path-traversal"
    CategoryExfiltration Category = "exfiltration"
    CategoryExecution    Category = "execution"
    CategoryTrust        Category = "trust"
    CategoryObfuscation  Category = "obfuscation"
)

// Checker interface — all built-in checkers from DefaultCheckers implement this
type Checker interface {
    Name() string
    Category() Category
    Check(ctx context.Context, sc *ScanContext) ([]Finding, error)
}

type Finding struct {
    Severity       Severity
    Category       Category
    SurfaceID      string          // SC-01..SC-10 (if applicable)
    SurfaceKind    SurfaceKind     // root/local/vendored/global trust boundary
    CheckerName    string          // producing checker's Name()
    FilePath       types.FilesystemPath
    Line           int
    Title          string
    Description    string
    Recommendation string
    EscalatedFrom  []string        // compound findings only
    EscalatedFromCodes []FindingCode
}
```

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
| `internal/provision/helpers.go` | SC-05 (symlinks) |
| `internal/runtime/virtual_policy.go`, `internal/runtime/sh.go`, `internal/runtime/lua.go`, `container_exec.go` | SC-02, SC-03, SC-04 (runtime surfaces) |
| `internal/discovery/`, `internal/app/deps/` | SC-09, SC-10 (scope, trust) |

---

## Common Pitfalls

| Pitfall | Fix |
|---------|-----|
| Regressing symlink skipping in module copy/provision paths | Keep copy helpers and scan-context artifact/clone paths from following symlinked module content outside trusted roots |
| Script path accepts absolute paths in module context | `ScriptChecker` now detects this (SeverityHigh); `GetScriptFilePathWithModule` still allows it at parse time |
| New `checks_*.go` missing SPDX header | `// SPDX-License-Identifier: MPL-2.0` as first line |
| `loadSingleModule` silently swallows invowkfile parse errors | Line 155: `if parseErr == nil` discards parse failures — intended (module without invowkfile) but consider logging |
| Scan context hides invalid modules/files | Preserve structured diagnostics in `Report.Diagnostics` and CLI JSON/text output |
| `ScanContext` methods expose mutable slices | `Modules()` / `Invowkfiles()` return slice headers — callers can `append` and corrupt shared state (low risk since checkers are well-behaved, but violates immutability contract) |
| Levenshtein runs O(n²) on module list | Acceptable for small module sets; for >100 modules consider short-circuiting or length pre-filter |

## Audit Implementation Review

**When to load:** Any code changes to `internal/audit/` or `cmd/invowk/audit.go`
— for correctness, performance, or security review of the scanner implementation itself.

Read **[references/implementation-review.md](references/implementation-review.md)** for:
- Per-checker correctness checklist (regex accuracy, false positive/negative analysis)
- Concurrency safety review (goroutine fan-out, `ScanContext` immutability contract)
- Performance considerations (regex compilation, Levenshtein, `ScanContext` building)
- Scanner self-defense (crafted input, DoS protection, TOCTOU windows)
- CLI rendering review (`cmd/invowk/audit.go` exit codes, JSON schema)
- Test coverage gap analysis for the current `internal/audit/*_test.go` inventory

This reference is only loaded when reviewing or modifying the audit Go code — the
agent-orchestrated security audit workflow above does not need it.

## Reference Files

- **[references/check-catalog.md](references/check-catalog.md)** —
  Full check specifications for each scanner subagent. Includes regex patterns,
  severity classifications, and implementation notes. Reference for understanding
  what the Go scanner checks and how to extend it.
- **[references/subagent-prompts.md](references/subagent-prompts.md)** —
  Prompt templates for Phase 3 interpretive subagents. These are concrete,
  grep-based verification prompts — not open-ended assessment requests.
- **[references/implementation-review.md](references/implementation-review.md)** —
  Correctness, performance, and security review checklist for the compiled
  `internal/audit/` Go scanner and `cmd/invowk/audit.go` CLI layer. Load when
  reviewing or improving the audit code itself (not for running audits).

## Related Skills

| Skill | When to Consult |
|-------|-----------------|
| `cli` | Cobra command registration, flag wiring, styled output |
| `discovery` | Module discovery precedence, vendored module scanning |
| `container` | Container security, volume mount patterns, provisioning |
| `dep-audit` | Go dependency auditing (complementary — handles `go.mod`) |
| `testing` | Test patterns, testscript CLI tests, txtar structure |
| `go-testing` | Go test execution model, race detector, coverage |
