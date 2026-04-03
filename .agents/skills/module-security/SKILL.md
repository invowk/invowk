---
name: module-security
description: >-
  Module system security auditing, supply-chain attack prevention, the
  `invowk audit` subcommand, and code quality review of the audit scanner
  implementation in `internal/audit/`. Spawns up to 10 parallel subagents
  across 4 phases: context gathering (2), deterministic scanning (5),
  correlation & deep review (2), and report assembly. Use when reviewing
  module code for vulnerabilities, implementing security scanning, working on
  supply-chain hardening, or when any changes touch module discovery, vendoring,
  lock files, script resolution, or command scope enforcement. Also use when
  reviewing or improving the `internal/audit/` Go code for correctness,
  performance, or security — load `references/implementation-review.md` for
  the full review checklist. Even for quick security questions about the module
  system, use this skill — it ensures consistent threat model awareness across
  conversations.
---

# Module Security

Security auditing and supply-chain attack prevention for invowk's module system.
This skill orchestrates parallel subagents for thorough, deterministic security
analysis — each subagent loads only the context it needs and produces structured
findings that are correlated in a later phase.

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

## Workflow Overview

```
Input (audit request, code review, security scan)
    │
    ▼
Phase 1: Context Gathering ──────── 2 parallel subagents
    │   ├── Threat Model Validator
    │   └── Module Tree Scanner
    │
    ▼
Phase 2: Deterministic Scans ────── up to 5 parallel subagents
    │   ├── Lock File Integrity Scanner
    │   ├── Script Path & Content Analyzer
    │   ├── Symlink Detector
    │   ├── Network/Env Exfiltration Scanner
    │   └── Module Metadata Analyzer
    │
    ▼
Phase 3: Correlation & Deep Review ── up to 3 parallel subagents
    │   ├── Finding Correlator (always)
    │   ├── Supply-Chain Reviewer (code changes only)
    │   └── Documentation Drift Checker (when threat model drifted)
    │
    ▼
Phase 4: Report Assembly ──────── main agent synthesizes
```

**Total subagent capacity:** up to 10 parallel subagents across 4 phases.

---

## Phase 1: Context Gathering

**Always runs.** Both subagents launch in parallel at the start of every audit.
Their outputs are required by Phase 2 — wait for both to complete before proceeding.

### Subagent 1: Threat Model Validator

Verifies the 10 attack surfaces (SC-01..SC-10) are still accurate — file paths
exist, line numbers match, status reflects current code.

**Prompt template:** `references/subagent-prompts.md` § "Threat Model Validator"

**Why this is a subagent:** The validator reads 10+ files across the codebase.
Running it in the main context would consume significant tokens on file content
that only matters for validation, not for the rest of the audit.

### Subagent 2: Module Tree Scanner

Discovers all modules, parses metadata, and produces a structured inventory.

**Prompt template:** `references/subagent-prompts.md` § "Module Tree Scanner"

**Why this is a subagent:** Module discovery requires recursive directory walking
and CUE parsing. The structured output feeds every Phase 2 scanner without
requiring them to re-discover modules independently.

### Phase 1 Output

The main agent receives:
1. **Threat model validation report** — SC-01..SC-10 status (confirmed / drifted)
2. **Module inventory** — structured list of all modules, scripts, lock files

If the Threat Model Validator reports status changes, flag them for the Phase 3
Documentation Drift Checker.

---

## Phase 2: Deterministic Scans

**Always runs.** All 5 subagents launch in parallel, each receiving the Module
Tree Scanner output from Phase 1. Each subagent reads only its section of
`references/check-catalog.md` to minimize context usage.

### Subagent Dispatch Table

| # | Subagent | Check Catalog Section | Input | Produces |
|---|----------|----------------------|-------|----------|
| 1 | Lock File Integrity Scanner | § Lock File Integrity | Module inventory | Hash mismatches, orphans, missing entries, version issues |
| 2 | Script Path & Content Analyzer | § Script Path & Content | Module inventory + script files | Path traversal, obfuscation, remote execution, large files |
| 3 | Symlink Detector | § Symlink Detection | Module inventory (paths only) | Symlinks, boundary escapes, dangling links, chains |
| 4 | Network/Env Exfiltration Scanner | § Network + § Env | Module inventory + script content | Network commands, encoded URLs, DNS exfil, sensitive vars |
| 5 | Module Metadata Analyzer | § Module Metadata | Module inventory | Dep depth, typosquatting, global trust, undeclared transitive |

**Prompt templates:** `references/subagent-prompts.md` § "Phase 2: Deterministic Scans"

### Finding Output Format

All Phase 2 subagents use the same structured output format:

```
- **Module**: {module_id}
- **File**: {path}:{line}
- **Check**: {check_name}
- **Severity**: {Info|Low|Medium|High|Critical}
- **Title**: {one-line description}
- **Detail**: {explanation}
- **Recommendation**: {fix preserving functionality}
- **Correlation tag**: {tag for Phase 3 cross-checks}  (Network/Env scanner only)
```

### When to Skip Scanners

Not every audit needs all 5 scanners. Skip when:

| Scanner | Skip When |
|---------|-----------|
| Lock File Integrity | No modules have lock files |
| Script Path & Content | No script-file references (all inline scripts) |
| Symlink Detector | Quick audit of a single known-safe module |
| Network/Env Exfiltration | Script content already reviewed manually |
| Module Metadata | Single module with no dependencies |

When in doubt, run all 5 — the overhead is minimal since they run in parallel.

---

## Phase 3: Correlation & Deep Review

Runs after Phase 2 completes. Subagent count varies by context (1–3).

### Subagent 6: Finding Correlator (always)

Reads all Phase 2 outputs and applies correlation rules to detect compound
threats that individual scanners cannot see.

**Key correlations:**

| Rule | Inputs | Escalation |
|------|--------|-----------|
| Credential exfiltration | env_sensitive + network in same module | Medium → Critical |
| Path + symlink escape | path traversal + external symlink target | High → Critical |
| Obfuscated exfiltration | obfuscation + encoded URL | High → Critical |
| Trust chain weakness | deep deps + missing lock entries | Medium → High |
| Interpreter + traversal | unusual interpreter + path traversal | Medium → Critical |

**Escalation rules:**
- Medium + Medium in same module → High
- High + any in same module → Critical
- 3+ categories in one module → always Critical

**Prompt template:** `references/subagent-prompts.md` § "Finding Correlator"

### Subagent 7: Supply-Chain Reviewer (code changes only)

**When:** Only when the audit is triggered by code changes (PRs, diffs) that
touch module system files listed in the Scope table above.

Spawns the existing `supply-chain-reviewer` agent (`.agents/agents/supply-chain-reviewer.md`)
with the diff and relevant Phase 2 findings as additional context.

**Prompt template:** `references/subagent-prompts.md` § "Supply-Chain Reviewer"

### Subagent 8: Documentation Drift Checker (optional)

**When:** Only if Phase 1's Threat Model Validator found discrepancies (status
changes, line number shifts, missing files).

Checks all documentation surfaces for stale security references.

**Prompt template:** `references/subagent-prompts.md` § "Documentation Drift Checker"

---

## Phase 4: Report Assembly

The main agent synthesizes all subagent outputs into the final report.
No subagents — this is the orchestrator's responsibility.

### Report Structure

```
Module Security Audit — {scan_path}
Scanned: {N} modules, {N} scripts ({duration})

▲ CRITICAL ({count})
  {SC-ID}  {title}
           File: {path}:{line}
           {detail}
           Fix: {recommendation}

● HIGH ({count})
  ...

◆ MEDIUM ({count})
  ...

○ LOW ({count})
  ...

ℹ INFO ({count})
  ...

═══ Compound Threats ═══
  {correlation findings from Phase 3}

═══ Threat Model Status ═══
  {SC-01..SC-10 validation summary from Phase 1}

Summary: {critical} critical, {high} high, {medium} medium, {low} low, {info} info
```

### Deduplication

Phase 3 correlations may produce findings that overlap with Phase 2 individual
findings. When assembling:
1. Keep the correlated finding (higher severity) as the primary entry
2. Remove the individual findings that were consumed by the correlation
3. Add a note: "Escalated from: {original finding titles}"

---

## Known Attack Surfaces

These 10 surfaces represent the current threat model. The Phase 1 Threat Model
Validator checks their accuracy at the start of every audit.

| ID | Surface | Severity | Key File(s) | Status |
|----|---------|----------|-------------|--------|
| SC-01 | Script path traversal | High | `pkg/invowkfile/implementation.go:364-456` | Mitigated |
| SC-02 | Virtual shell host PATH fallback | Medium | `internal/runtime/virtual.go:344-357` | By-design |
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

When reviewing module-related changes, the skill adapts the 4-phase workflow:

1. **Phase 1** — Threat Model Validator + Module Tree Scanner (parallel)
2. **Phase 2** — Run relevant scanners based on changed files (skip unrelated)
3. **Phase 3** — Finding Correlator + Supply-Chain Reviewer with diff (parallel)
4. **Phase 4** — Report with recommendations

**File → Scanner mapping** for targeted Phase 2:

| Changed File | Scanners to Run |
|-------------|----------------|
| `pkg/invowkmod/lockfile.go`, `content_hash.go` | Lock File Integrity |
| `pkg/invowkfile/implementation.go`, `pkg/invowkfile/runtime.go` | Script Path & Content |
| `internal/provision/helpers.go` | Symlink Detector |
| `internal/runtime/virtual.go`, `container_exec.go` | Network/Env Exfiltration |
| `internal/discovery/`, `internal/app/deps/` | Module Metadata |
| Multiple areas | All 5 scanners |

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
  severity classifications, and implementation notes. Subagents read only their
  relevant section to minimize context usage.
- **[references/subagent-prompts.md](references/subagent-prompts.md)** —
  Complete prompt templates for all 10 subagents across 4 phases. Adapt these
  to the specific audit context before spawning.
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
