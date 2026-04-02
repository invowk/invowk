---
name: module-security
description: >-
  Module system security auditing, supply-chain attack prevention, and the
  `invowk module audit` subcommand. Spawns up to 9 parallel subagents across
  4 phases: context gathering (2), deterministic scanning (5), correlation &
  deep review (2), and report assembly. Use when reviewing module code for
  vulnerabilities, implementing security scanning, working on supply-chain
  hardening, or when any changes touch module discovery, vendoring, lock files,
  script resolution, or command scope enforcement. Also use when a user asks
  about module security, trust boundaries, or how to verify third-party modules.
  Even for quick security questions about the module system, use this skill —
  it ensures consistent threat model awareness across conversations.
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
| `internal/audit/` | Audit scanner package (to be created) |
| `cmd/invowk/module_audit.go` | CLI command (to be created) |

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
| SC-01 | Script path traversal | High | `pkg/invowkfile/implementation.go:363-444` | Partial |
| SC-02 | Virtual shell host PATH fallback | Medium | `internal/runtime/virtual.go:344-355` | By-design |
| SC-03 | InvowkDir R/W volume mount | Medium | `internal/runtime/container_exec.go:118` | By-design |
| SC-04 | SSH token in container env | Medium | `internal/runtime/container_exec.go:438, runtime.go:571-575` | Partial |
| SC-05 | Provision `CopyDir` symlink handling | Medium | `internal/provision/helpers.go:132-156` | Mitigated |
| SC-06 | `--ivk-env-var` priority override | Low | `internal/runtime/env_builder.go` | By-design |
| SC-07 | `check_script` host shell execution | High | `internal/app/deps/checks.go:70-72` | Partial |
| SC-08 | Arbitrary interpreter paths | Medium | `pkg/invowkfile/runtime.go:452, pkg/invowkfile/implementation.go` | Open |
| SC-09 | Root invowkfile scope bypass | Low | `internal/app/deps/deps.go:199-201` | By-design |
| SC-10 | Global module trust (no integrity) | Medium | `internal/discovery/discovery_files.go:119-124` | Open |

**Status legend:** Open (no mitigation), Partial (gaps remain), Mitigated (fixed, residual gap only), By-design (intentional, document only)

**2026-04-02 audit notes:**
- SC-05 upgraded to Mitigated: both `CopyDir` implementations (`resolver_cache.go:copyDir` and `provision/helpers.go:CopyDir`) now skip symlinks. Residual: `os.Stat` on the `src` dir argument itself follows symlinks.
- SC-10: `detectModuleShadowing()` warning added in `discovery_files.go` for local-vs-global collisions.

---

## `invowk module audit` Subcommand Architecture

Implementation guidance for the CLI command. Read `references/check-catalog.md`
for the full check specifications that map to `internal/audit/checks_*.go` files.

### CLI Layer: `cmd/invowk/module_audit.go`

```go
func newModuleAuditCommand(app *App) *cobra.Command {
    var (
        format      string // "text" or "json"
        minSeverity string // "low", "medium", "high", "critical"
        includeGlobal bool
    )
    cmd := &cobra.Command{
        Use:   "audit [path]",
        Short: "Scan modules for security risks",
        Long: `Analyze module trees for supply-chain vulnerabilities, script injection,
path traversal, suspicious patterns, and lock file integrity issues.

Exit codes:
  0  No findings at or above the severity threshold
  1  Findings detected
  2  Scan error`,
        Args: cobra.MaximumNArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            auditPath := "."
            if len(args) > 0 { auditPath = args[0] }
            return runModuleAudit(cmd.Context(), app, auditPath, format, minSeverity, includeGlobal)
        },
    }
    cmd.Flags().StringVar(&format, "format", "text", "Output format: text, json")
    cmd.Flags().StringVar(&minSeverity, "severity", "low", "Minimum severity: low, medium, high, critical")
    cmd.Flags().BoolVar(&includeGlobal, "include-global", false, "Include ~/.invowk/cmds/ in scan")
    return cmd
}
```

**Exit codes:** 0 = clean, 1 = findings (`FindingsError`), 2 = scan error.
**Registration:** `modCmd.AddCommand(newModuleAuditCommand(app))` in `module.go`.

### Domain Layer: `internal/audit/`

| File | Purpose |
|------|---------|
| `doc.go` | Package comment + SPDX header |
| `scanner.go` | `Scanner` struct, `Scan()`, check orchestration |
| `findings.go` | `Finding`, `Severity`, `Category` types |
| `report.go` | `Report` type, filtering, sorting, deduplication |
| `checks_lockfile.go` | Lock file integrity (hash, version, orphans) |
| `checks_script.go` | Script path traversal + content analysis |
| `checks_network.go` | Network access detection |
| `checks_env.go` | Env var risk analysis |
| `checks_symlink.go` | Symlink detection and boundary checking |
| `checks_module.go` | Module metadata (deps, collisions, trust) |
| `correlate.go` | Finding correlation rules (compound threats) |

### Core Types

```go
type Severity int
const (
    SeverityInfo Severity = iota
    SeverityLow
    SeverityMedium
    SeverityHigh
    SeverityCritical
)

type Category string
const (
    CategoryIntegrity    Category = "integrity"
    CategoryPathTraversal Category = "path-traversal"
    CategoryExfiltration Category = "exfiltration"
    CategoryExecution    Category = "execution"
    CategoryTrust        Category = "trust"
    CategoryObfuscation  Category = "obfuscation"
)

type Finding struct {
    Severity       Severity
    Category       Category
    SurfaceID      string   // SC-01..SC-10 (if applicable)
    FilePath       string
    Line           int
    Title          string
    Description    string
    Recommendation string
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
| `CopyDir` in `internal/provision/helpers.go` follows symlinks | Add `d.Type()&os.ModeSymlink` check (match `resolver_cache.go`) |
| Script path accepts absolute paths in module context | Add `filepath.Rel` bounds check in `GetScriptFilePathWithModule` |
| Lock file parsed without size guard | Add size check before `parseLockFileCUE` (match 5MB CUE guard) |
| Audit scanner reads script without size limit | Apply `os.Stat` size check before `os.ReadFile` |
| `computeModuleHash` is unexported | Add exported accessor in `pkg/invowkmod/` |
| New `checks_*.go` missing SPDX header | `// SPDX-License-Identifier: MPL-2.0` as first line |
| `module audit` not registered | `modCmd.AddCommand(newModuleAuditCommand(app))` |

## Reference Files

- **[references/check-catalog.md](references/check-catalog.md)** —
  Full check specifications for each scanner subagent. Includes regex patterns,
  severity classifications, and implementation notes. Subagents read only their
  relevant section to minimize context usage.
- **[references/subagent-prompts.md](references/subagent-prompts.md)** —
  Complete prompt templates for all 10 subagents across 4 phases. Adapt these
  to the specific audit context before spawning.

## Related Skills

| Skill | When to Consult |
|-------|-----------------|
| `cli` | Cobra command registration, flag wiring, styled output |
| `discovery` | Module discovery precedence, vendored module scanning |
| `container` | Container security, volume mount patterns, provisioning |
| `dep-audit` | Go dependency auditing (complementary — handles `go.mod`) |
| `testing` | Test patterns, testscript CLI tests, txtar structure |
| `go-testing` | Go test execution model, race detector, coverage |
