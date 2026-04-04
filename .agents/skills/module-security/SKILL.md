---
name: module-security
description: >-
  Module system security auditing, supply-chain attack prevention, the
  `invowk audit` subcommand, and code quality review of the audit scanner
  implementation in `internal/audit/`. Deterministic scanning backbone:
  always runs `invowk audit --format json` first (compiled Go scanner with
  regex patterns, severity classifications, structured output), then applies
  interpretive analysis only for things the tool cannot decide — threat model
  drift, supply-chain code review of diffs, documentation consistency. Use
  when reviewing module code for vulnerabilities, implementing security
  scanning, working on supply-chain hardening, or when any changes touch
  module discovery, vendoring, lock files, script resolution, or command
  scope enforcement. Also use when reviewing or improving the
  `internal/audit/` Go code for correctness, performance, or security —
  load `references/implementation-review.md` for the full review checklist.
  Even for quick security questions about the module system, use this skill
  — it ensures consistent threat model awareness across conversations.
---

# Module Security

Security auditing and supply-chain attack prevention for invowk's module system.

**Design principle: tool-first, LLM-second.** The compiled Go scanner
(`invowk audit`) is the deterministic backbone — it produces identical output
for identical input every time. LLM analysis is reserved for genuinely
interpretive tasks that a regex scanner cannot do. This eliminates the variance
that comes from asking LLMs to pattern-match code.

## Normative Quick Rules

- `.agents/rules/go-patterns.md` — error handling, context propagation, declaration ordering
- `.agents/rules/testing.md` — test coverage, testscript patterns
- `.agents/rules/licensing.md` — SPDX headers on all new Go files
- `.agents/rules/package-design.md` — package boundaries for `internal/audit/`
- If this skill conflicts with a rule, **the rule wins**.

## Scope

| Code Area | What to Audit |
|-----------|--------------|
| `pkg/invowkmod/` | Lock files, content hashes, vendoring, module operations |
| `pkg/invowkfile/` | Script path resolution, validation, filesystem checks |
| `internal/provision/` | Container provisioning, module copying, symlink handling |
| `internal/runtime/virtual.go` | Host PATH fallback behavior |
| `internal/app/deps/` | Command scope enforcement |
| `internal/discovery/` | Global module trust, `IsGlobalModule` propagation |
| `internal/audit/` | Audit scanner package (14 production files, ~1,893 lines) |
| `cmd/invowk/audit.go` | Top-level CLI command (registered in `root.go`) |

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
```

### Step 2: Run the scanner

Run the compiled scanner against the target path. Always use JSON format and
`--severity info` to capture all findings (filtering happens in Phase 2):

```bash
invowk audit --format json --severity info {scan_path} 2>/dev/null
```

For broader scans that include global modules:

```bash
invowk audit --format json --severity info --include-global {scan_path} 2>/dev/null
```

**Exit codes:** 0 = no findings, 1 = findings detected, 2 = scan error.

### Step 3: Parse the JSON output

The scanner produces structured JSON with this schema:

```json
{
  "findings": [
    {
      "severity": "critical|high|medium|low|info",
      "category": "integrity|path-traversal|exfiltration|execution|trust|obfuscation",
      "surface_id": "file path or SC-ID",
      "checker_name": "producing checker name",
      "file_path": "affected file",
      "line": 0,
      "title": "one-line description",
      "description": "detailed explanation",
      "recommendation": "fix suggestion",
      "escalated_from": ["original finding titles (compound findings only)"]
    }
  ],
  "compound_threats": [...],
  "summary": {
    "total": 0, "critical": 0, "high": 0, "medium": 0, "low": 0, "info": 0,
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
| SymlinkChecker | Symlinks, boundary escapes, dangling, chains | `checks_symlink.go` |
| ModuleMetadataChecker | Dep depth, typosquatting, global trust, version pinning | `checks_module.go` |
| Correlator | Compound threat escalation (5 named rules + severity rules) | `correlator.go` |

---

## Phase 2: Classify & Triage

**Main agent work. No subagents.** Parse the Phase 1 JSON and classify each
finding into one of three buckets:

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
| R2 | Finding title is **"Reverse shell pattern detected"** | **Confirmed** | Always report regardless of location |
| R3 | Finding title is **"Script downloads and executes remote code"** | **Confirmed** | Always report regardless of location |
| R4 | Finding title is **"Module content hash mismatch"** | **Confirmed** | Tamper indicator, always report |
| R5 | Finding title is **"Symlink points outside module boundary"** | **Confirmed** | Escape vector, always report |
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

### Subagent A: Threat Model Drift Checker (always runs, 1 agent)

Verifies the 10 attack surfaces (SC-01..SC-10) against current code. Unlike
the old approach that asked an LLM to "assess status", this checker uses
concrete verification steps.

**Prompt template:**

```
You are verifying the module security threat model for invowk.
Your job is to run CONCRETE CHECKS — not to assess or interpret.

For each attack surface below, run the specific verification command
and report ONLY what the command output shows.

## Verification Steps (run each command, report output)

SC-01 Script path traversal:
  grep -n "validateScriptPathContainment" pkg/invowkfile/implementation.go
  → If function exists: CONFIRMED (Mitigated)
  → If not found: DRIFTED (mitigation may have been removed)

SC-02 Virtual shell host PATH fallback:
  grep -n "interp.ExecHandlers\|execHandler" internal/runtime/virtual.go
  → If present: CONFIRMED (By-design, host fallback active via next() chain)
  → If not found: DRIFTED (execution model changed)

SC-03 InvowkDir R/W volume mount:
  grep -n "invowkDir" internal/runtime/container_exec.go
  → If present: CONFIRMED (By-design)
  → If not found: DRIFTED

SC-04 SSH token in container env:
  grep -n "SSH_AUTH_SOCK\|INVOWK_SSH" internal/runtime/container_exec.go
  → Report count and line numbers. Status: Partial if found

SC-05 Provision CopyDir symlink handling:
  grep -n "os.ModeSymlink\|SkipSymlink\|symlink" internal/provision/helpers.go
  → If skip logic exists: CONFIRMED (Mitigated)
  → If follows symlinks: DRIFTED (regression)

SC-06 --ivk-env-var priority override:
  grep -n "ivk-env-var\|IvkEnvVar" internal/runtime/env_builder.go
  → If present: CONFIRMED (By-design)

SC-07 check_script host shell execution:
  grep -n "exec.Command\|os/exec" internal/app/deps/checks.go
  → If present: CONFIRMED (Partial — host exec still used)
  → If removed: status changed

SC-08 Arbitrary interpreter paths:
  grep -n "allowedInterpreters\|Validate" pkg/invowkfile/interpreter_spec.go
  → If allowlist exists: CONFIRMED (Mitigated)
  → If no allowlist: DRIFTED (regression)

SC-09 Root invowkfile scope bypass:
  grep -n "CanCall\|CommandScope" internal/app/deps/deps.go
  → If scope check exists: CONFIRMED (By-design)

SC-10 Global module trust:
  grep -n "IsGlobalModule\|detectModuleShadowing\|VerifyVendoredModuleHashes" internal/discovery/discovery_files.go
  → Report which functions exist. Status: Partial if shadowing detection present

## Output Format (EXACTLY this format, one line per surface)

SC-01: CONFIRMED (Mitigated) — validateScriptPathContainment at line 446
SC-02: CONFIRMED (By-design) — interp.ExecHandlers(r.execHandler) at line 320
...

## Rules
- Do NOT add subjective commentary or "new findings"
- Do NOT assess code quality or suggest improvements
- Report ONLY what the grep commands show
- If a grep returns no results, report "NOT FOUND" — that IS the finding
```

### Subagent B: Supply-Chain Reviewer (only for code changes)

**When:** Only when the audit is triggered by code changes (PRs, diffs) that
touch module system files listed in the Scope table.

Spawns the existing `supply-chain-reviewer` agent
(`.agents/agents/supply-chain-reviewer.md`) with the diff and Phase 1 findings
as context.

**Prompt template:**

```
You are the supply-chain security reviewer for a code change touching
the invowk module system.

## Deterministic Scan Results (from `invowk audit`)
{paste Phase 1 JSON summary — just summary + findings at medium+}

## Diff to Review
{paste git diff or list of changed files}

## Your Task
Focus ONLY on these questions:
1. Does this diff regress any existing mitigation? (map to SC-01..SC-10)
2. Does this diff introduce a new attack surface not covered by the scanner?
3. Does this diff change trust boundaries (who can invoke what)?

## Rules
- Do NOT re-scan for patterns the Go scanner already checks
  (regex patterns, env vars, network commands — those are in Phase 1)
- Do NOT report findings already present in the Phase 1 output
- ONLY report genuinely new risks that the compiled scanner cannot detect
- If nothing new: report "No additional supply-chain risks beyond scanner findings"
```

### Subagent C: Documentation Drift Checker (only when drift detected)

**When:** Only if Subagent A found any DRIFTED status.

```
You are checking for documentation drift in invowk's module security docs.

## Drifted Surfaces (from Subagent A)
{paste only the DRIFTED lines}

## Your Task
For each drifted surface, check if these documents reference the old status:
1. `.agents/agents/supply-chain-reviewer.md`
2. `.agents/skills/module-security/SKILL.md` § "Known Attack Surfaces"
3. `CLAUDE.md` § "Virtual Runtime Security Model"

## Output Format (one entry per stale reference)
- **File**: {path}
- **Line**: {number}
- **Current text**: {what it says}
- **Should be**: {what it should say based on Subagent A findings}

If no documents reference stale status: "No documentation drift detected."
```

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
  SC-01: CONFIRMED (Mitigated) — validateScriptPathContainment at line 446
  SC-02: CONFIRMED (By-design) — interp.ExecHandlers(r.execHandler) at line 320
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
| SC-01 | Script path traversal | High | `pkg/invowkfile/implementation.go:364-456` | Mitigated |
| SC-02 | Virtual shell host PATH fallback | Medium | `internal/runtime/virtual.go:320,345-357` | By-design |
| SC-03 | InvowkDir R/W volume mount | Medium | `internal/runtime/container_exec.go:118` | By-design |
| SC-04 | SSH token and TUI credentials in container/virtual env | Medium | `internal/runtime/container_exec.go:443, runtime.go:573-575` | Partial |
| SC-05 | Provision `CopyDir` symlink handling | Medium | `internal/provision/helpers.go:131-170` | Mitigated |
| SC-06 | `--ivk-env-var` priority override | Low | `internal/runtime/env_builder.go` | By-design |
| SC-07 | `check_script` host shell execution | High | `internal/app/deps/checks.go:70-72` | Partial |
| SC-08 | Arbitrary interpreter paths | Medium | `pkg/invowkfile/interpreter_spec.go, runtime.go:435-438` | Mitigated (allowlist in Validate) |
| SC-09 | Root invowkfile scope bypass | Low | `internal/app/deps/deps.go:199-201` | By-design |
| SC-10 | Global module trust (no integrity) | Medium | `internal/discovery/discovery_files.go:119-131` | Partial |

**Status legend:** Open (no mitigation), Partial (gaps remain), Mitigated (fixed, residual gap only), By-design (intentional, document only)

**2026-04-02 audit notes:**
- SC-01 upgraded to Mitigated: `validateScriptPathContainment()` now implemented in `implementation.go:446-456`, called from both `ResolveScriptWithModule` and `ResolveScriptWithFSAndModule`. Uses `filepath.Rel` + `strings.HasPrefix("..")`. Residual: root invowkfile scripts bypass containment when `modulePath == ""` (by design — user controls root invowkfile).
- SC-05 upgraded to Mitigated: both `CopyDir` implementations (`resolver_cache.go:copyDir` and `provision/helpers.go:CopyDir`) now skip symlinks. Residual: `os.Stat` on the `src` dir argument itself follows symlinks.
- SC-10 upgraded to Partial: `detectModuleShadowing()` warning added in `discovery_files.go` for local-vs-global collisions. No cryptographic integrity verification — shadowing detection is warning-level only.
- SC-08 upgraded to Mitigated: `InterpreterSpec.Validate()` now enforces an allowlist of known safe interpreters and rejects shell metacharacters. Bare `env` requires full path (`/usr/bin/env` or `/bin/env`). Residual: absolute paths to allowlisted interpreters in attacker-controlled directories (e.g., `/tmp/python3`) pass the `filepath.Base` check, since only the base name is validated against the allowlist.

**2026-04-03 audit notes:**
- SC-06 edge case: `--ivk-env-var` can override security-relevant variables set by modules themselves (e.g., `INVOWK_SSH_ENABLED`), not only host credentials flowing into scripts. The env_builder docstring warns about host credentials but not about module-set variable overrides.
- SC-08 edge case: The allowlist check uses `filepath.Base()` on the interpreter path, so `/tmp/python3` would pass validation if `python3` is in the allowlist. Only the base name is validated — directory components are not checked. Low residual risk since the attacker-placed binary must be named identically to an allowlisted interpreter.
- SC-10 edge case: `detectModuleShadowing` only checks `IsGlobalModule` vs non-global. No detection of two global modules with the same ID (e.g., via different includes). The shadowing warning is diagnostic-only with no `--strict-module-trust` flag to promote it to a fatal error.
- SC-10 improvement: `VerifyVendoredModuleHashes` is now wired into the discovery execution path via `appendModulesWithVendored`, providing automatic tamper detection before vendored modules are loaded. Hash mismatches abort discovery with a hard error.
- Audit scanner improvement: `LockFileChecker` now detects vendored modules present without a lock file, even when `requires` is empty. Closes gap where manually placed `invowk_modules/` content bypassed integrity enforcement.

---

## `invowk audit` Subcommand Architecture

Implementation reference for the top-level audit command. Read
`references/check-catalog.md` for check specifications that map to
`internal/audit/checks_*.go`. For code quality review guidance, load
`references/implementation-review.md`.

### CLI Layer: `cmd/invowk/audit.go`

```go
func newAuditCommand(app *App) *cobra.Command {
    var (
        format        string
        minSeverity   string
        includeGlobal bool
    )
    cmd := &cobra.Command{
        Use:   "audit [path]",
        Short: "Scan for security risks",
        Long: `Analyze invowkfiles and modules for supply-chain vulnerabilities, script
injection, path traversal, suspicious patterns, and lock file integrity issues.

The audit scans standalone invowkfiles, local modules, vendored dependencies,
and optionally global modules in ~/.invowk/cmds/.

Exit codes:
  0  No findings at or above the severity threshold
  1  Findings detected
  2  Scan error`,
        Args: cobra.MaximumNArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            auditPath := "."
            if len(args) > 0 { auditPath = args[0] }
            return runAudit(cmd, app, auditPath, format, minSeverity, includeGlobal)
        },
    }
    cmd.Flags().StringVar(&format, "format", "text", "output format: text, json")
    cmd.Flags().StringVar(&minSeverity, "severity", "low", "minimum severity: info, low, medium, high, critical")
    cmd.Flags().BoolVar(&includeGlobal, "include-global", false, "include ~/.invowk/cmds/ in scan")
    return cmd
}
```

**Exit codes:** 0 = clean, 1 = findings, 2 = scan error (via `ExitError` with typed codes).
**Registration:** `rootCmd.AddCommand(newAuditCommand(app))` in `root.go`.

### Domain Layer: `internal/audit/`

| File | Purpose |
|------|---------|
| `doc.go` | Package comment + SPDX header |
| `severity.go` | `Severity` iota enum, `ParseSeverity()`, JSON marshaling, `InvalidSeverityError` |
| `types.go` | `Category`, `Finding`, `Report` types, filtering, sorting, counting |
| `errors.go` | Sentinels (`ErrScanContextBuild`, `ErrCheckerFailed`, `ErrNoScanTargets`), typed wrappers |
| `checker.go` | `Checker` interface (`Name`, `Category`, `Check`) |
| `scan_context.go` | `ScanContext`, `BuildScanContext`, `ScannedModule`, `ScriptRef`, discovery integration |
| `scanner.go` | `Scanner` struct, `Scan()`, concurrent `runCheckers`, functional options |
| `correlator.go` | `Correlator`, `CorrelationRule`, named rules + severity escalation |
| `checks_lockfile.go` | Lock file integrity (hash, version, orphans, size guard) |
| `checks_script.go` | Script path traversal + content analysis (remote exec, obfuscation) |
| `checks_network.go` | Network access, reverse shells, DNS exfiltration, encoded URLs |
| `checks_env.go` | Env var risk, credential extraction, `env_inherit_mode` |
| `checks_symlink.go` | Symlink detection, boundary checking, chain depth |
| `checks_module.go` | Module metadata (deps, typosquatting, global trust, version pinning) |

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

// Checker interface — all 6 checkers implement this
type Checker interface {
    Name() string
    Category() Category
    Check(ctx context.Context, sc *ScanContext) ([]Finding, error)
}

type Finding struct {
    Severity       Severity
    Category       Category
    SurfaceID      string          // SC-01..SC-10 (if applicable)
    CheckerName    string          // producing checker's Name()
    FilePath       types.FilesystemPath
    Line           int
    Title          string
    Description    string
    Recommendation string
    EscalatedFrom  []string        // compound findings only
}
```

---

## Security Review Workflow (for Code Changes)

When reviewing module-related changes, adapt the workflow:

1. **Phase 1** — Run `invowk audit --format json --severity info .`
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
| `internal/runtime/virtual.go`, `container_exec.go` | SC-02, SC-03, SC-04 (runtime surfaces) |
| `internal/discovery/`, `internal/app/deps/` | SC-09, SC-10 (scope, trust) |

---

## Common Pitfalls

| Pitfall | Fix |
|---------|-----|
| `CopyDir` in `internal/provision/helpers.go` follows symlinks | Both `CopyDir` impls now skip symlinks (SC-05 Mitigated); residual: `os.Stat` on `src` dir argument itself follows symlinks |
| Script path accepts absolute paths in module context | `ScriptChecker` now detects this (SeverityHigh); `GetScriptFilePathWithModule` still allows it at parse time |
| New `checks_*.go` missing SPDX header | `// SPDX-License-Identifier: MPL-2.0` as first line |
| `loadSingleModule` silently swallows invowkfile parse errors | Line 155: `if parseErr == nil` discards parse failures — intended (module without invowkfile) but consider logging |
| `loadDirectoryTree` silently skips invalid modules | Line 219: `continue` on `loadSingleModule` error loses the error — add structured warning to report |
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
- Test coverage gap analysis (10 test files, ~1,369 lines)

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
