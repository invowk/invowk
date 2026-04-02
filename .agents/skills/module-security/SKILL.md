---
name: module-security
description: >-
  Module system security auditing, supply-chain attack prevention, and the
  `invowk module audit` subcommand. Use when reviewing module code for
  vulnerabilities, implementing security scanning, working on supply-chain
  hardening, or when any changes touch module discovery, vendoring, lock files,
  script resolution, or command scope enforcement. Also use when a user asks
  about module security, trust boundaries, or how to verify third-party modules.
---

# Module Security

Security auditing and supply-chain attack prevention for invowk's module system. This skill provides implementation guidance for hardening the module system and building the `invowk module audit` subcommand.

For code review of security-sensitive changes, spawn the `supply-chain-reviewer` agent (`.agents/agents/supply-chain-reviewer.md`).

## Scope

Use this skill when working on:
- `pkg/invowkmod/` — lock files, content hashes, vendoring, module operations
- `pkg/invowkfile/` — script path resolution, validation, filesystem checks
- `internal/provision/` — container provisioning, module copying
- `internal/runtime/virtual.go` — host PATH fallback behavior
- `internal/app/deps/` — command scope enforcement
- `internal/discovery/` — global module trust, `IsGlobalModule` propagation
- `internal/audit/` — the new audit scanner package (to be created)
- `cmd/invowk/module_audit.go` — the new CLI command (to be created)

## Normative Quick Rules

- `.agents/rules/go-patterns.md` — error handling, context propagation, declaration ordering
- `.agents/rules/testing.md` — test coverage, testscript patterns
- `.agents/rules/licensing.md` — SPDX headers on all new Go files
- `.agents/rules/package-design.md` — package boundaries for `internal/audit/`
- If this skill conflicts with a rule, **the rule wins**.

## Known Attack Surfaces

These 10 surfaces represent the current threat model for the module system. Each has an ID for cross-referencing with the `supply-chain-reviewer` agent.

| ID | Surface | Severity | Key File(s) | Status | Recommended Action |
|----|---------|----------|-------------|--------|--------------------|
| SC-01 | Script path traversal | High | `pkg/invowkfile/implementation.go:308` | Open | Add `filepath.Rel` bounds check matching `ValidateContainerfilePath` |
| SC-02 | Virtual shell host PATH fallback | Medium | `internal/runtime/virtual.go:345` | By-design | Document in module author guidelines; never claim "sandboxed" |
| SC-03 | InvowkDir R/W volume mount | Medium | `internal/runtime/container_exec.go:118` | By-design | Consider read-only default with opt-in write; document risk |
| SC-04 | SSH token in container env | Medium | `internal/runtime/container_exec.go` | Partial | Minimize token lifetime; consider file-based injection |
| SC-05 | Provision `CopyDir` follows symlinks | High | `internal/provision/helpers.go:123` | Open | Add `d.Type()&os.ModeSymlink` check (match `resolver_cache.go`) |
| SC-06 | `--ivk-env-var` priority override | Low | `internal/runtime/env_builder.go` | By-design | Document as intentional; warn in `--help` text |
| SC-07 | `check_script` host shell execution | High | `cmd/invowk/cmd_validate_checks.go` | Open | Run checks in virtual runtime or container; bound execution time |
| SC-08 | Arbitrary interpreter paths | Medium | `pkg/invowkfile/implementation.go` | Open | Add advisory warning in audit; consider allowlist for modules |
| SC-09 | Root invowkfile scope bypass | Low | `internal/app/deps/deps.go:199` | By-design | Document; root commands are user-authored, not third-party |
| SC-10 | Global module trust (no integrity) | Medium | `internal/discovery/discovery_files.go:122` | Open | Add optional content hash manifest for `~/.invowk/cmds/` |

### Status Categories

- **Open** — No mitigation; needs code change or explicit risk acceptance with documentation
- **Partial** — Some mitigation exists but attack surface remains
- **By-design** — Intentional behavior; document clearly, do not "fix"

### Triage Guidance

When deciding whether to fix an open surface or accept the risk:

1. **Who controls the input?** If only the local user (root invowkfile, global modules), the risk is lower. If a third-party module author controls it (script paths, vendored content), the risk is higher.
2. **What's the blast radius?** Host filesystem access (SC-01, SC-05) is worse than information disclosure (SC-04).
3. **Does a fix break UX?** Script path restrictions (SC-01) may break legitimate `../shared-scripts/` patterns. Provide migration guidance and consider an `--allow-external-scripts` opt-in.

## `invowk module audit` Subcommand Architecture

This section defines the complete architecture for a new `invowk module audit` command that scans module trees for security risks.

### CLI Layer

**File:** `cmd/invowk/module_audit.go`

```go
// Pattern: newModuleAuditCommand() factory → runModuleAudit() handler
// Registration: module.go → modCmd.AddCommand(newModuleAuditCommand(app))

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
            if len(args) > 0 {
                auditPath = args[0]
            }
            return runModuleAudit(cmd.Context(), app, auditPath, format, minSeverity, includeGlobal)
        },
    }
    cmd.Flags().StringVar(&format, "format", "text", "Output format: text, json")
    cmd.Flags().StringVar(&minSeverity, "severity", "low", "Minimum severity to report: low, medium, high, critical")
    cmd.Flags().BoolVar(&includeGlobal, "include-global", false, "Include global modules (~/.invowk/cmds/) in scan")
    return cmd
}
```

**Handler responsibilities:**
1. Resolve `auditPath` to absolute path
2. Load config via `app` (for global modules dir, includes)
3. Create `audit.Scanner` with options
4. Call `scanner.Scan(ctx, opts)` to get `*audit.Report`
5. Filter report by `minSeverity`
6. Render report (text or JSON) — rendering stays in CLI layer per package-design rules
7. Return exit code: `nil` (0) if clean, `audit.FindingsError` (1) if findings, wrapped error (2) on scan failure

**Registration in `module.go`:**
```go
modCmd.AddCommand(newModuleAuditCommand(app))
```

### Domain Layer: `internal/audit/`

The audit package is internal because it depends on invowk's module structures and is not intended for external consumption.

#### Package Structure

| File | Purpose |
|------|---------|
| `doc.go` | Package comment + SPDX header |
| `scanner.go` | `Scanner` struct, `Scan()` method, check orchestration |
| `findings.go` | `Finding`, `Severity`, `Category` types |
| `report.go` | `Report` type, filtering, sorting |
| `checks_lockfile.go` | Lock file integrity: hash verification, version validation, orphan detection |
| `checks_script.go` | Script analysis: path traversal, obfuscation, suspicious patterns |
| `checks_network.go` | Network access: `curl`/`wget`/`nc` detection, encoded URLs, DNS patterns |
| `checks_env.go` | Env var risks: sensitive var reading, exfiltration correlation |
| `checks_symlink.go` | Symlink detection in module trees |
| `checks_module.go` | Module metadata: dependency chain, namespace collision, global trust |

#### Core Types

```go
// Severity levels ordered by increasing severity.
type Severity int

const (
    SeverityInfo     Severity = iota // Accepted design decisions, informational
    SeverityLow                      // Hardening opportunities
    SeverityMedium                   // Information disclosure, DoS potential
    SeverityHigh                     // Data exfiltration, integrity bypass
    SeverityCritical                 // Remote code execution, arbitrary file write
)

// Category groups related findings for filtering and reporting.
type Category string

const (
    CategoryIntegrity    Category = "integrity"     // Hash mismatches, lock file issues
    CategoryPathTraversal Category = "path-traversal" // Script/file path escaping boundaries
    CategoryExfiltration Category = "exfiltration"   // Network access, env var leakage
    CategoryExecution    Category = "execution"      // Arbitrary code execution vectors
    CategoryTrust        Category = "trust"          // Trust model weaknesses
    CategoryObfuscation  Category = "obfuscation"    // Encoded payloads, eval patterns
)

// Finding represents a single security issue detected during audit.
type Finding struct {
    Severity       Severity
    Category       Category
    SurfaceID      string          // SC-01 through SC-10 (if applicable)
    FilePath       string          // Absolute path to the affected file
    Line           int             // Line number (0 if not applicable)
    Title          string          // Short description (one line)
    Description    string          // Detailed explanation
    Recommendation string          // Suggested fix preserving functionality
}

// ScanOptions configures the audit scan.
type ScanOptions struct {
    Path           string          // Root path to scan (absolute)
    IncludeGlobal  bool            // Scan ~/.invowk/cmds/ modules
    GlobalModDir   string          // Path to global modules directory
}

// Report aggregates findings from a scan.
type Report struct {
    ScanPath  string
    Findings  []Finding
    ScannedModules int
    ScannedScripts int
    Duration  time.Duration
}
```

#### Scanner Architecture

```go
type Scanner struct{}

func NewScanner() *Scanner { return &Scanner{} }

func (s *Scanner) Scan(ctx context.Context, opts ScanOptions) (*Report, error) {
    // 1. Discover all modules in the scan path
    // 2. Load invowkmod.cue and lock file for each module
    // 3. Parse invowkfile.cue for each module
    // 4. Run all check functions, collecting findings
    // 5. Return report
}
```

Each check function has the signature:
```go
func check<Name>(ctx context.Context, state *scanState) []Finding
```

The `scanState` struct holds pre-loaded data to avoid redundant I/O:
```go
type scanState struct {
    modules    []discoveredModule      // All discovered modules
    lockFiles  map[string]*invowkmod.LockFile // Module path → lock file
    invowkfiles map[string]*invowkfile.Invowkfile // Module path → parsed invowkfile
    scripts    map[string][]byte       // Script path → content (lazy-loaded)
}
```

#### Check Details

**`checks_lockfile.go`** — Integrity verification

| Check | What | How | Finding |
|-------|------|-----|---------|
| Hash mismatch | Content hash in lock file vs actual module content | Reuse `invowkmod.computeModuleHash()` (unexported — needs accessor or move to exported helper) | Critical: "Module content hash mismatch — module may have been tampered with" |
| Orphaned lock entries | Lock file entries without corresponding `requires` | Cross-reference lock file modules against `invowkmod.cue:requires` | Low: "Lock file contains entry not in requires" |
| Missing lock entries | `requires` entries without lock file entry | Cross-reference `requires` against lock file | Medium: "Required module has no lock file entry — run `invowk module sync`" |
| Lock file version | Unknown lock file version | Check `version` field against known versions | High: "Unknown lock file version — may have been crafted" |

**`checks_script.go`** — Script analysis

| Check | What | How | Finding |
|-------|------|-----|---------|
| Path traversal | Script fields using `../` or absolute paths | Parse invowkfile, inspect `Implementation.Script` via `IsScriptFile()` heuristic | High: "Script references path outside module boundary" |
| Obfuscation | `base64 -d`, `eval`, backtick execution, `$()` with encoded content | Regex scan of resolved script content | High: "Script contains obfuscation pattern" |
| Suspicious commands | `curl \| sh`, `wget -O- \| bash`, piped remote execution | Regex: `(curl\|wget\|fetch).*\|.*(sh\|bash\|python)` | Critical: "Script downloads and executes remote code" |
| Large scripts | Scripts exceeding 5MB (matching CUE guard) | `os.Stat` before read | Medium: "Script file unusually large" |

**`checks_network.go`** — Network access detection

| Check | What | How | Finding |
|-------|------|-----|---------|
| Network commands | `curl`, `wget`, `nc`, `ncat`, `socat` usage | Keyword scan in script content | Medium: "Script uses network access command" |
| Encoded URLs | Base64-encoded URLs, hex-encoded domains | Regex for `aHR0c` (base64 "http") and `\x` sequences | High: "Script contains encoded URL" |
| DNS exfiltration | `dig`, `nslookup`, `host` with variable interpolation | Regex: `(dig\|nslookup\|host).*\$` | High: "Possible DNS exfiltration pattern" |

**`checks_env.go`** — Environment variable risks

| Check | What | How | Finding |
|-------|------|-----|---------|
| Sensitive var access | Reading `$HOME`, `$SSH_AUTH_SOCK`, `$AWS_*`, `$GITHUB_TOKEN`, etc. | Regex scan for known sensitive variable names | Medium: "Script accesses sensitive environment variable" |
| Env + network | Script reads sensitive vars AND uses network commands | Cross-correlate network and env findings | High: "Potential credential exfiltration — reads sensitive env vars and has network access" |
| `env_inherit_mode: all` | Module commands inheriting full host environment | Parse invowkfile env config | Low: "Command inherits all host environment variables" |

**`checks_symlink.go`** — Symlink detection

| Check | What | How | Finding |
|-------|------|-----|---------|
| Symlinks in modules | Symlinks inside `.invowkmod` directories | `filepath.WalkDir` with `d.Type()&os.ModeSymlink` | High: "Symlink found in module directory — may reference content outside module boundary" |
| Symlink targets | Where symlinks point (inside vs outside module) | `os.Readlink` + `filepath.Rel` check | Critical (if outside): "Symlink points outside module boundary" |

**`checks_module.go`** — Module metadata analysis

| Check | What | How | Finding |
|-------|------|-----|---------|
| Dependency chain depth | Unusually deep transitive dependency chains | Walk `requires` transitively, count depth | Medium: "Deep dependency chain (depth N) — increases supply-chain attack surface" |
| Namespace collision | Modules with similar IDs (typosquatting) | Levenshtein distance between module IDs in the dependency tree | Medium: "Module ID similar to another module — possible typosquatting" |
| Global module warnings | Modules in `~/.invowk/cmds/` with no integrity verification | Check `IsGlobalModule` flag, report all global modules | Info: "Global module has no content hash verification" |
| Undeclared transitive deps | Modules used transitively but not in root `requires` | Same logic as `checkMissingTransitiveDeps()` in `resolver_deps.go` | Medium: "Transitive dependency not declared in root invowkmod.cue" |

### Exit Codes

| Code | Meaning |
|------|---------|
| 0 | No findings at or above the severity threshold |
| 1 | Findings detected at or above the severity threshold |
| 2 | Scan error (invalid path, parse failure, I/O error) |

The CLI handler should use a sentinel error type for exit code 1:
```go
type FindingsError struct {
    Count int
}
func (e *FindingsError) Error() string {
    return fmt.Sprintf("%d security finding(s) detected", e.Count)
}
```

### Output Formats

**Text format** (default): Styled using existing `module*.go` icon palette, grouped by severity (critical first), with color-coded severity labels.

```
Module Security Audit — /path/to/project
Scanned: 5 modules, 12 scripts (1.2s)

▲ CRITICAL (1)
  SC-01  Script references path outside module boundary
         File: modules/utils.invowkmod/invowkfile.cue:15
         Script field "../../../etc/cron.d/payload" escapes module directory.
         Fix: Use module-relative paths only; move shared scripts into the module.

● HIGH (2)
  SC-05  Symlink points outside module boundary
         File: modules/data.invowkmod/config → /etc/shadow
         Symlink target resolves outside the module directory.
         Fix: Remove symlink; copy the file into the module if needed.
  ...

◆ MEDIUM (1)
  ...

Summary: 1 critical, 2 high, 1 medium, 0 low
```

**JSON format**: Structured output for CI integration, matching the `Report` struct.

### Testing

**Required:** `tests/cli/testdata/module_audit.txtar` — the `TestBuiltinCommandTxtarCoverage` guardrail will fail without it. The test should:
1. Set up a module tree with known vulnerabilities (path traversal, symlinks, suspicious scripts)
2. Run `invowk module audit` and verify findings in stdout
3. Test `--format json` output
4. Test `--severity` filtering
5. Test exit codes (0 for clean, 1 for findings)

**Unit tests:** Each `checks_*.go` file should have a corresponding `checks_*_test.go` with test cases using crafted malicious inputs.

## Security Review Workflow

When reviewing module-related changes, follow this human-in-the-loop process:

1. **Spawn the `supply-chain-reviewer` agent** to identify which attack surfaces are affected
2. **Run the relevant checklists** from the agent's review areas
3. **If implementing fixes**, follow the patterns in this skill for the correct package structure and Cobra conventions
4. **Run `invowk module audit`** on the test modules to verify fixes detect the intended issues
5. **Update the threat model table** in both the agent and this skill if the status changes

## Common Pitfalls

| Pitfall | Symptom | Fix |
|---------|---------|-----|
| `CopyDir` in `internal/provision/helpers.go` follows symlinks | Symlink escape during container provisioning | Add `d.Type()&os.ModeSymlink` check (match `pkg/invowkmod/resolver_cache.go:copyDir`) |
| Script path accepts absolute paths in module context | Module reads arbitrary host files via `script: "/etc/shadow"` | Add `filepath.Rel` bounds check in `GetScriptFilePathWithModule` |
| Lock file parsed without size guard | DoS via crafted multi-GB lock file | Add size check before `parseLockFileCUE` (match 5MB CUE guard) |
| Audit scanner reads script without size limit | OOM on crafted script | Apply `os.Stat` size check before `os.ReadFile` |
| New `checks_*.go` file missing SPDX header | `make license-check` fails | Add `// SPDX-License-Identifier: MPL-2.0` as first line |
| `module audit` not registered in `module.go` | `TestBuiltinCommandTxtarCoverage` fails | Add `modCmd.AddCommand(newModuleAuditCommand(app))` |
| `computeModuleHash` is unexported | Audit scanner can't reuse it | Add exported accessor or move to shared helper in `pkg/invowkmod/` |

## Related Skills

| Skill | When to Consult |
|-------|-----------------|
| `cli` | Cobra command registration pattern, flag wiring, styled output, hidden commands |
| `discovery` | Module discovery precedence, vendored module scanning, collision detection |
| `container` | Container security, volume mount patterns, provisioning |
| `dep-audit` | Go dependency auditing (complementary — `dep-audit` handles `go.mod`, this handles `invowkmod.cue`) |
| `testing` | Test patterns, testscript CLI tests, txtar structure |
| `go-testing` | Go test execution model, race detector, coverage |
