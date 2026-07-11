# Audit Implementation Review Guide

Review checklist for the `invowk audit` scanner implementation in `internal/audit/`
and `cmd/invowk/audit.go`. Use this when modifying, extending, or reviewing the
audit Go code for correctness, performance, or security.

## Table of Contents

- [Architecture Quick Reference](#architecture-quick-reference)
- [Correctness: Per-Checker Review](#correctness-per-checker-review)
- [Correctness: Regex Pattern Audit](#correctness-regex-pattern-audit)
- [Concurrency Safety](#concurrency-safety)
- [Performance Considerations](#performance-considerations)
- [Scanner Self-Defense](#scanner-self-defense)
- [CLI Layer Review](#cli-layer-review)
- [Test Coverage Analysis](#test-coverage-analysis)
- [Known Issues and Opportunities](#known-issues-and-opportunities)

---

## Architecture Quick Reference

```
cmd/invowk/audit.go           CLI adapter: flags, exit codes, rendering
         │
         ▼
internal/audit/scanner.go      Scanner.Scan() orchestrator
         │
    ┌────┴────┐
    │         │
    ▼         ▼
scan_context  runCheckers ──→ DefaultCheckers() goroutines (concurrent)
    │                              │
    ▼                              ▼
BuildScanContext          correlator.go ──→ compound findings
(discovery + parse)                │
                                   ▼
                              Report assembly
```

Before using this guide, derive the current scanner inventory:

```bash
rg -n 'func DefaultCheckers|New.*Checker|type .*Checker' internal/audit
find internal/audit -maxdepth 1 -type f -name '*.go' | sort
```

**Key contracts:**
- `ScanContext` is immutable after `BuildScanContext()` returns — checkers share it concurrently
- `Checker.Check()` must be safe for concurrent calls with the same `*ScanContext`
- `Scanner.Scan()` returns partial results on checker failure (non-nil report + non-nil error)
- Exit codes: 0 (clean), 1 (findings via `ExitError`), 2 (scan error via `ExitError`)

---

## Correctness: Per-Checker Review

For each checker, verify these properties:

### LockFileChecker (`checks_lockfile.go`)

| Check | What to Verify |
|-------|---------------|
| `checkSize` | Uses `os.Stat` before parse — correct DoS guard. Verify `maxLockFileSize` (5 MiB) matches `pkg/invowkmod` CUE guard |
| `checkVersion` | Tests both v1.0 and v2.0 recognition. Verify against `invowkmod.LockFileVersionV1` and `LockFileVersionV2` constants |
| `checkHashMismatches` | Calls `invowkmod.ComputeModuleHash` — verify it's the exported accessor, not duplicated logic |
| `checkOrphanedEntries` | Cross-references lock modules vs vendored IDs via `ExtractModuleIDFromNamespace`. Verify namespace extraction matches lock file format |
| `checkMissingEntries` | Builds `reqKey` as `gitURL + "#" + path`. Verify this key format matches how lock entries are keyed |
| Missing lock file | Modules without lock files are correctly skipped (`if mod.LockFile == nil { continue }`) |

**Attention point:** `checkMissingEntries` uses `strings.Contains` for matching — could false-positive
if one module's git URL is a substring of another's. Consider exact key matching.

### ScriptChecker (`checks_script.go`)

| Check | What to Verify |
|-------|---------------|
| `checkScriptPath` | Only checks file-based scripts (`ref.IsFile`). Inline scripts with `../` in body caught by `checkObfuscation` |
| `checkScriptFileSize` | Only checks scripts with `ref.ModulePath != ""`. Standalone scripts skip size check — intentional? |
| `checkRemoteExecution` | Two regex patterns: piped `curl\|sh` and silent `-sS` variant |
| `checkObfuscation` | 5 regex patterns in table-driven loop. Deduplication: `pathTraversalPattern` check skips if already caught by `checkScriptPath` |
| Path concatenation | `checkScriptFileSize` builds path with `/` separator — verify cross-platform correctness (should use `filepath.Join`) |

**Attention point:** Recheck `checkScriptFileSize` path construction whenever
script path validation changes. It should preserve cross-platform behavior and
match the rest of the codebase's path helpers.

### NetworkChecker (`checks_network.go`)

| Check | What to Verify |
|-------|---------------|
| Reverse shell suppression | When reverse shell found, generic network command check is skipped (`len(reverseShellFindings) == 0`). Correct priority |
| `base64HTTPPattern` | Matches literal `aHR0c` (base64 of "http"). Could false-positive on legitimate base64 content |
| `dnsExfilPattern` | Requires variable interpolation (`$[{(]?[A-Z_]`). Pure DNS lookups without vars won't trigger — correct |
| Empty content | Early `continue` when `content == ""` — correct |

### EnvChecker (`checks_env.go`)

| Check | What to Verify |
|-------|---------------|
| `checkEnvInheritMode` | Checks `ref.Runtimes[i].EnvInheritMode == invowkfile.EnvInheritAll`. Verify this constant exists |
| `sensitiveVarPattern` | Named credentials: AWS, GitHub, SSH, DB. Review if list is current |
| `genericSecretPattern` | Catches `API_KEY`, `SECRET_KEY`, etc. with optional `{}` braces around var name |
| `tokenExtractionPattern` | Detects `$TOKEN[^|>]*[|>]` — piped or redirected. The `[^|>]*` gap allows whitespace/flags between var and pipe |
| Double finding | Same script could match both `sensitiveVarPattern` and `genericSecretPattern` — intentional (different severity rationale) |

### SymlinkChecker (`checks_symlink.go`)

| Check | What to Verify |
|-------|---------------|
| `WalkDir` callback | Uses `d.Type()&os.ModeSymlink` — correct lstat-based check (WalkDir does NOT follow symlinks) |
| Boundary check | `filepath.Rel` + `strings.HasPrefix(rel, "..")` — verify this handles `..` at various positions |
| Chain detection | Iterates up to `maxSymlinkChainDepth` (10) with `os.Readlink` + `os.Lstat`. Correct chain-following |
| Dangling check | Uses `os.Stat(path)` — follows symlinks, so `os.IsNotExist` means target doesn't exist. Correct |
| Walk error handling | Line 103-106: `_ = err` discards non-cancel walk errors silently. Consider structured warning |
| Windows | Skipped in tests via `runtime.GOOS` guard. Symlinks on Windows behave differently |

### LuaChecker (`checks_lua.go`)

| Check | What to Verify |
|-------|----------------|
| Runtime selection | Only virtual-lua implementations are inspected |
| Disabled APIs | Findings match APIs actually disabled or replaced by the Lua bridge |
| Sensitive env reads | Both `os.getenv` and `invowk.env` forms are covered without treating ordinary variables as secrets |
| Host binaries | Wildcard and network-capable explicit allowlists preserve the documented native-host-execution warning |
| Filesystem reach | Full access and broad path mappings match current virtual filesystem semantics |

### ModuleMetadataChecker (`checks_module.go`)

| Check | What to Verify |
|-------|---------------|
| Typosquatting | O(n^2) Levenshtein over all module IDs. Self-comparison excluded (`thisID == otherID`) |
| Levenshtein | Two-row algorithm, space-efficient. Byte comparison (not rune) — could give wrong distances for non-ASCII module IDs |
| Dependency "depth" | Actually counts `len(requires)` (number of direct deps), not depth. Naming is slightly misleading |
| Version pinning | Checks for `""`, `"*"`, `">=0.0.0"`. Other loose constraints (e.g., `>0.0.0`) not caught |
| Undeclared transitive | Cross-references vendored module requires against root declared deps. Key is `GitURL` string |

---

## Correctness: Regex Pattern Audit

Regex patterns are the most brittle part of the scanner. When reviewing or modifying:

### False Positive Risks

| Pattern | False Positive Scenario |
|---------|----------------------|
| `remoteExecPattern` | Legitimate `curl ... \| jq` (not piping to shell). Currently only matches shell interpreters — acceptable |
| `base64HTTPPattern` (`aHR0c`) | Any content containing the substring `aHR0c` in comments, documentation, or test data |
| `networkCommandPattern` (`\b(curl\|wget\|nc\|ncat\|socat)\b`) | Any mention of these tools, even in comments within scripts |
| `sensitiveVarPattern` | Scripts that intentionally access credentials for authorized operations |
| `pathTraversalPattern` (`\.\./`) | Legitimate relative path references in non-module scripts |

### False Negative Risks

| Pattern | Evasion Technique |
|---------|------------------|
| `remoteExecPattern` | Using `python3 -c "import urllib..."` instead of `curl\|bash` |
| `reverseShellBashPattern` | Alternative devices: `/dev/udp/`, or using `exec 5<>/dev/tcp/` |
| `base64DecodePattern` | Using `openssl base64 -d` instead of `base64 -d` |
| `evalPattern` | Using `source /dev/stdin <<< "..."` instead of `eval` |
| `hexSequencePattern` | Using octal (`\077`) or unicode (`\u0041`) escapes instead of hex |
| `dnsExfilPattern` | Using `getent hosts` or Python's `socket.gethostbyname` instead of `dig`/`nslookup` |

### Regex Compilation

All patterns are compiled at package init (top-level `var` block with `regexp.MustCompile`).
This is correct — avoids recompilation per call. When adding new patterns, always use
package-level `var` declarations, never compile inside `Check()`.

---

## Concurrency Safety

### ScanContext Immutability Contract

`ScanContext` is documented as immutable after construction. Verify:

1. **No mutation after `BuildScanContext` returns** — all fields are set during construction,
   `scripts` is pre-computed via `buildScriptRefs()`
2. **Accessor methods return slice headers** — `Modules()`, `Invowkfiles()`, `AllScripts()`
   return the underlying slices directly. Callers that `append` would corrupt shared state.
   Current checkers are well-behaved, but this is a latent safety issue.
   **Recommendation:** Return copies or use read-only wrapper types for defense-in-depth.
3. **No pointer mutation** — checkers receive `*ScanContext` but should never write through it.
   Go's type system doesn't prevent this — rely on convention and review.

### Goroutine Fan-Out in `runCheckers`

Inspect the current `runCheckers` implementation for this pattern:

```go
results := make([]result, len(s.checkers))  // pre-allocated, index-safe
var wg sync.WaitGroup
for i, checker := range s.checkers {
    wg.Add(1)
    go func(idx int, c Checker) { ... }(i, checker)
}
wg.Wait()
```

- **Correct:** Pre-allocated results slice avoids race on append
- **Correct:** Index-based assignment (`results[idx]`) is goroutine-safe (distinct indices)
- **Correct:** Loop variables captured via function parameters, not closure
- **Note:** Context cancellation check is only at goroutine start — a long-running checker
  should periodically check `ctx.Done()` within its `Check()` method. Verify this
  for every checker returned by `DefaultCheckers()`.

### Correlator Thread Safety

`Correlator.Correlate()` runs after all checkers complete (sequential). It receives the
collected findings slice — no concurrent access. Safe by design.

---

## Performance Considerations

### Hot Paths

1. **`BuildScanContext`** — Most expensive operation. Calls `discovery.DiscoverAll()` which
   walks the filesystem and parses CUE. For large module trees, this dominates scan time.
   Not parallelized internally — consider if discovery supports concurrent parsing.

2. **`buildScriptRefs`** — Iterates all commands × implementations. Pre-computed once, shared
   across checkers. Correct amortization.

3. **Regex matching in checkers** — Each checker iterates `sc.AllScripts()` and runs multiple
   regex matches per script. For codebases with thousands of scripts, this is O(scripts × patterns).
   The compiled `regexp.Regexp` objects are efficient, but if performance becomes an issue,
   consider combining patterns into single alternation regexps.

### Levenshtein Algorithm

The Levenshtein helper uses two-row dynamic programming, O(n×m) time and O(m)
space. Inspect the current implementation rather than relying on historical
line numbers. It is correct for small inputs (module ID strings are typically
<100 chars). The quadratic
outer loop over all module pairs is the actual concern — O(k^2) where k = number of modules.
For typical invowk projects (<20 modules), this is negligible.

### Memory Allocation Patterns

- `ScanContext` pre-computes `scripts` once — correct, avoids per-checker allocation
- Checkers use `append` to grow findings slices — idiomatic, small allocations
- `Report.AllFindings()` allocates a new merged slice on each call — fine for one-shot
  rendering, but `FilterBySeverity` calls `AllFindings()` internally, creating a second
  allocation if both are called. For the current CLI usage (single render pass), acceptable.

---

## Scanner Self-Defense

The scanner analyzes untrusted module content. Review these attack surfaces against the
scanner itself:

### Crafted Input Protection

| Attack | Protection | File:Line |
|--------|-----------|-----------|
| Giant lock file DoS | `checkSize` with 5 MiB guard before parse | `checks_lockfile.go` |
| Giant script file DoS | `checkScriptFileSize` with 5 MiB guard | `checks_script.go` |
| Directory traversal in scan path | `filepath.Abs` on scan path input | Recheck current `BuildScanContext` path normalization |
| Symlink chain loop | `maxSymlinkChainDepth` limit | Recheck current symlink checker |
| ReDoS via regex | All patterns use simple alternation/character classes, no nested quantifiers — safe |
| Crafted module ID for Levenshtein | Module IDs are validated by CUE schema before reaching audit — bounded length |

### Missing Protections

| Gap | Risk | Mitigation |
|-----|------|-----------|
| No file count limit | A crafted directory with millions of tiny files could exhaust memory during `WalkDir` | Low: `os.ReadDir` in `loadDirectoryTree` only scans one level; `SymlinkChecker.WalkDir` is the risk |
| No total script content memory limit | Many large (but under 5 MiB each) scripts could accumulate | Low: script content is stored in `invowkfile.ScriptContent` from parse, already in memory |
| `os.Stat` TOCTOU in lock file checker | File could change between `Stat` (size check) and `LoadLockFile` (parse) | Very low: lock files are rarely modified during scan |
| Error swallowing in `loadSingleModule` | Parse errors for invowkfile and lock file are silently ignored | By design (modules without these files are valid), but errors from corrupt files are lost |

---

## CLI Layer Review

Inspect the current `cmd/invowk/audit.go` rendering and CLI adapter code.

### Exit Code Contract

| Code | Constant | Condition |
|------|----------|-----------|
| 0 | `auditExitClean` | `report.HasFindings(minSev)` returns false |
| 1 | `auditExitFindings` | Findings exist at or above threshold |
| 2 | `auditExitError` | Scan error or invalid flags |

**Verify:** `runAudit` returns `nil` (exit 0) only when no findings. Returns `&ExitError{Code: auditExitFindings}` when findings exist but no scan error. Returns `&ExitError{Code: auditExitError}` on severity parse failure or fatal scan error.

### JSON Output Schema

```json
{
  "findings": [{"severity": "...", "category": "...", ...}],
  "compound_threats": [{"severity": "...", ...}],
  "summary": {
    "total": 0, "critical": 0, "high": 0, "medium": 0, "low": 0, "info": 0,
    "modules_scanned": 0, "invowkfiles_scanned": 0, "scripts_scanned": 0,
    "duration_ms": 0
  }
}
```

**Verify:** `auditJSONOutput` struct tags match this schema. The `compound_threats` field
uses `omitempty` — empty array omitted from JSON (not `null`). This is correct for clean scans.

### Rendering Quality

- `renderAuditText` uses lipgloss styling. Verify styles degrade gracefully when
  `NO_COLOR` or non-TTY output is detected (lipgloss handles this automatically)
- `groupBySeverity` + `severityOrder` ensures Critical → High → Medium → Low → Info ordering
- `formatDuration` should handle sub-millisecond durations (fast scans)

---

## Test Coverage Analysis

### Live Test Inventory

Derive test inventory and coverage from the checkout:

```bash
find internal/audit -maxdepth 1 -type f -name '*_test.go' -print | sort
find tests/cli/testdata -maxdepth 1 -type f -name '*audit*.txtar' -print | sort
rg -n '^func Test' internal/audit tests/cli
go test ./internal/audit
```

Confirm every checker returned by `DefaultCheckers()` has positive, negative,
cancellation where applicable, and boundary/false-positive coverage. Inspect
CLI tests for clean output, findings, JSON schema, severity filtering,
diagnostics, and error exit behavior. Never preserve counts from an earlier
checkout as evidence.

### Coverage Questions

Re-evaluate these from the live tests rather than treating them as known gaps:

- Scan-path routing across CUE files, module directories, and directory trees.
- Structured diagnostics for invalid or partially readable scan inputs.
- Discovery-result deduplication and vendored-module loading.
- Exact lock-entry matching and script-size/path boundary behavior.
- CLI exit 2 paths, `--include-global`, partial checker failures, and compound
  threats in JSON.

### Test Pattern Notes

- Checker tests use `newTestScanContext` / `newSingleScriptContext` factory helpers from
  `scan_context_test_helper_test.go`. When adding new checker tests, use these factories.
- Symlink tests have `runtime.GOOS` guard for Windows. Other checkers don't need it
  (they operate on parsed data, not filesystem).
- CLI txtar tests are minimal — they verify exit codes and key output strings, not full output.
  This is appropriate for smoke testing but may miss rendering regressions.

---

## Review Opportunities

Re-derive opportunities from source on each review. Pay particular attention to
exact structured lock-entry matching, platform-aware script paths, defensive
copies from `ScanContext`, and rune-safe edit distance if module IDs can contain
non-ASCII text. Do not carry a resolved issue forward because an older version
of this guide named it.

### Performance Opportunities

1. **Combined regex patterns** — Checkers with multiple patterns could combine
   related passes when profiling shows regex traversal is material.

2. **Early termination** — When `--severity critical` is used, checkers could skip checks
   that can only produce findings below Critical. Currently all checks run regardless.

3. **Parallel `BuildScanContext`** — If the discovery package supports concurrent parsing,
   `loadDirectoryTree` could parallelize module loading.

### Security Opportunities

1. **File count limit** — Add a configurable maximum number of files to scan to prevent
   resource exhaustion on crafted directory structures.

2. **Structured warnings** — Replace silent `continue` on parse errors with structured
   warnings in the Report (new field: `Warnings []string`), so users know when scan
   coverage was incomplete.

3. **Content-based detection** — Currently all script analysis is regex-based. For higher
   accuracy, consider AST-based analysis for shell scripts using `mvdan.cc/sh/v3/syntax`
   (already a dependency) to detect patterns like subshell nesting, process substitution,
   and indirect variable expansion.
