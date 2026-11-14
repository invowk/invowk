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
- [Review Opportunities](#review-opportunities)

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
- Exit codes: 0 (no confirmed findings after triage), 1 (confirmed findings via `ExitError`), 2 (scan or configuration error via `ExitError`)

---

## Correctness: Per-Checker Review

For each checker, verify these properties:

### LockFileChecker (`checks_lockfile.go`)

| Check | What to Verify |
|-------|---------------|
| Unparseable / missing lock | A `LockFileParseErr` yields "Lock file present but unparseable" and skips the other checks; no lock with `requires` or with vendored content is flagged, not skipped |
| `checkSize` | Compares the snapshot size with `invowkmod.LockFileSizeLimit` (5 MiB), the same constant `InspectLockFile` enforces before reading. Both `continue` branches above run first, so verify this path is still reachable |
| `checkVersion` | Tests both v1.0 and v2.0 recognition. Verify against `invowkmod.LockFileVersionV1` and `LockFileVersionV2` constants |
| `checkHashMismatches` | Calls the injected `VendoredHashEvaluator` (default `invowkmod.EvaluateVendoredModuleHash`, set by `WithHashEvaluator`) — verify no hashing logic is duplicated in the audit package |
| `checkOrphanedEntries` | Uses `invowkmod.OrphanedLockedModuleEntries`; reports the first 10 individually, then one summary finding |
| `checkMissingEntries` | Uses exact keys from `invowkmod.MissingLockedModuleRequirementKeys`. Verify it stays exact, with no substring matching |

### ScriptChecker (`checks_script.go`)

| Check | What to Verify |
|-------|---------------|
| `checkScriptPath` | Only checks file-based scripts (`ref.IsFile`). Inline scripts with `../` in body caught by `checkObfuscation` |
| `checkScriptFileSize` | Only checks scripts with `ref.ModulePath != ""`. Standalone scripts skip size check — intentional? |
| `checkRemoteExecution` | Three patterns: piped `curl\|sh`, process substitution `bash <(curl …)`, and download-then-execute |
| `checkObfuscation` | 5 regex patterns in table-driven loop. Deduplication: `pathTraversalPattern` check skips if already caught by `checkScriptPath` |

**Attention point:** `checkScriptFileSize` uses the pre-resolved `ref.ScriptPath`.
Whenever script path validation changes, recheck that it keeps cross-platform
behavior and matches the rest of the codebase's path helpers.

### NetworkChecker (`checks_network.go`)

| Check | What to Verify |
|-------|---------------|
| Reverse shell suppression | When reverse shell found, generic network command check is skipped (`len(reverseShellFindings) == 0`). Correct priority |
| `base64HTTPPattern` | Matches literal `aHR0c` (base64 of "http"). Could false-positive on legitimate base64 content |
| `dnsExfilPattern` | Requires variable interpolation (`$[{(]?[A-Za-z_]`) or backtick expansion. Pure DNS lookups without either won't trigger — correct |
| Empty content | Early `continue` when `content == ""` — correct |

### EnvChecker (`checks_env.go`)

| Check | What to Verify |
|-------|---------------|
| `checkEnvInheritMode` | Checks `ref.Runtimes[i].EnvInheritMode == invowkfile.EnvInheritAll`. Verify this constant exists |
| `sensitiveVarPattern` | Named credentials: AWS, GitHub, SSH, DB. Review if list is current |
| `genericSecretPattern` | Catches `API_KEY`, `SECRET_KEY`, etc. with optional `{}` braces around var name |
| `tokenExtractionPattern` | Detects a credential variable followed by a pipe or redirect (`[^\|>]*[\|>]`). The `[^\|>]*` gap allows whitespace/flags between var and pipe |
| Double finding | Same script could match both `sensitiveVarPattern` and `genericSecretPattern` — intentional (different severity rationale) |

### SymlinkChecker (`checks_symlink.go`)

| Check | What to Verify |
|-------|---------------|
| Bounded artifact walker | Uses batched `ReadDir(n)` and `DirEntry.Type()&os.ModeSymlink`; symlinked directories are reported but never followed |
| Boundary check | `isWithinBoundary` (`filepath.Rel`, then rejects `..` and a `..` + separator prefix) — verify this handles `..` at various positions and that a `..foo` name is not treated as an escape |
| Chain detection | Iterates up to `maxSymlinkChainDepth` (10) with `os.Readlink` + `os.Lstat`. Correct chain-following |
| Dangling check | Uses `os.Stat(path)` — follows symlinks, so `os.IsNotExist` means target doesn't exist. Correct |
| Walk error handling | Open, read, close, visitor, cancellation, and artifact-budget errors fail scan-context construction rather than yielding partial clean results |
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
| `checkDependencyFanOut` | Counts direct `requires` against `maxDependencyFanOut` (5); it measures fan-out, not graph depth |
| Version pinning | `unpinnedVersions` set (`""`, `*`, `>=0.0.0`, `>0`, `>=0`, `>0.0.0`, `>=0.0.0-0`, `*.*`, `*.*.*`). Review for other any-version forms |
| Undeclared transitive | Delegates to `invowkmod.CheckMissingVendoredTransitiveDeps`; vendored-module declaration uses lock identity when a lock exists |

---

## Correctness: Regex Pattern Audit

Regex patterns are the most brittle part of the scanner. When reviewing or modifying:

### False Positive Risks

| Pattern | False Positive Scenario |
|---------|----------------------|
| `remoteExecPattern` | Legitimate `curl ... \| jq` (not piping to an interpreter). Currently only matches interpreters (shells, python, perl, ruby, node, php, PowerShell) — acceptable |
| `base64HTTPPattern` (`aHR0c`) | Any content containing the substring `aHR0c` in comments, documentation, or test data |
| `networkCommandPattern` (`\b(curl\|wget\|aria2c\|nc\|ncat\|socat)\b`) | Any mention of these tools, even in comments within scripts |
| `sensitiveVarPattern` | Scripts that intentionally access credentials for authorized operations |
| `pathTraversalPattern` (`\.\./`) | Legitimate relative path references inside module scripts (only module scripts are scanned) |

### False Negative Risks

| Pattern | Evasion Technique |
|---------|------------------|
| `remoteExecPattern` | Using `python3 -c "import urllib..."` instead of `curl\|bash` |
| `reverseShellBashPattern` | Alternative devices: `/dev/udp/`, or using `exec 5<>/dev/tcp/` |
| `base64DecodePattern` | Using `openssl base64 -d` instead of `base64 -d` |
| `evalPattern` | Using `source /dev/stdin <<< "..."` instead of `eval` |
| `hexSequencePattern` | Using octal (`\077`) or unicode (`\u0041`) escapes instead of hex |
| `dnsExfilPattern` | Using `getent hosts` or Python's `socket.gethostbyname` instead of `dig`/`nslookup` |
| `sensitiveVarPattern`, `tokenExtractionPattern` | Brace form `${GITHUB_TOKEN}`: both match only `$NAME`, so braced named credentials evade them (`genericSecretPattern` allows braces, but only for generic names) |

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
2. **Accessor methods return clones** — `Modules()`, `Invowkfiles()`, `AllScripts()`
   return cloned snapshots, so a checker cannot corrupt shared state through them.
   Keep new accessors cloning too.
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
- **Note:** Every default checker checks `ctx.Done()` per module or script inside
  `Check()`. Verify that any new checker returned by `DefaultCheckers()` does the same.

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
| Giant lock file DoS | `invowkmod.InspectLockFile` stats and rejects files over 5 MiB before reading; surfaces as "Lock file present but unparseable" | `pkg/invowkmod/lockfile.go`, `checks_lockfile.go` |
| Giant script file DoS | `checkScriptFileSize` with 5 MiB guard | `checks_script.go` |
| Directory traversal in scan path | `filepath.Abs` on scan path input | Recheck current `BuildScanContext` path normalization |
| Symlink chain loop | `maxSymlinkChainDepth` limit | Recheck current symlink checker |
| Huge artifact tree | Scan-wide Lua and symlink budgets plus bounded `ReadDir(n)` batches; exhaustion fails closed | `artifact_walker.go`, `artifact_limits.go` |
| ReDoS via regex | All patterns use simple alternation/character classes, no nested quantifiers — safe |
| Crafted module ID for Levenshtein | Module IDs are validated by CUE schema before reaching audit — bounded length |

### Missing Protections

| Gap | Risk | Mitigation |
|-----|------|-----------|
| No total script content memory limit | Many large (but under 5 MiB each) scripts could accumulate | Low: script content is stored in `invowkfile.ScriptContent` from parse, already in memory |
| `os.Stat` TOCTOU in lock file loading | File could change between `os.Stat` (size check) and `os.ReadFile` in `invowkmod.InspectLockFile` | Very low: lock files are rarely modified during scan |

---

## CLI Layer Review

Inspect the current `cmd/invowk/audit.go` rendering and CLI adapter code.

### Exit Code Contract

| Code | Constant | Condition |
|------|----------|-----------|
| 0 | `auditExitClean` | `triage.HasConfirmedFindings(minSev)` returns false (suppressed by-design findings never count) |
| 1 | `auditExitFindings` | Confirmed findings exist at or above threshold |
| 2 | `auditExitError` | Invalid flags or LLM configuration, scanner construction failure, unknown `--format`, render failure, or fatal scan error |

**Verify:** `runAudit` returns `nil` (exit 0) only when no confirmed finding meets the threshold. A non-nil scan error with a report that `audit.ScanFailureIsFatal` does not classify as fatal only prints `warning: some checkers failed` and keeps the 0/1 decision. Fatal scan errors, severity parse failures, and invalid scanner or LLM configuration return `&ExitError{Code: auditExitError}`.

### JSON Output Schema

The canonical schema is Phase 1 Step 3 in `../SKILL.md`, mirroring
`auditJSONOutput` in `cmd/invowk/audit.go`. **Verify:** the struct tags still
match it. `compound_threats` (like every top-level key except `findings` and
`summary`, and the finding fields `surface_id`, `surface_kind`, `file_path`,
`line`, `escalated_from`, `escalated_from_codes`) uses `omitempty` — an empty array is omitted from JSON (not `null`). This is
correct for clean scans.

### Rendering Quality

- `renderAuditText` uses lipgloss styling. Verify styles degrade gracefully when
  `NO_COLOR` or non-TTY output is detected (lipgloss handles this automatically)
- `groupBySeverity` + `auditSeverityDisplayOrder()` ensure Critical → High → Medium → Low → Info ordering
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

Re-derive opportunities from source on each review. Rune-safe edit distance is
still open if module IDs can contain non-ASCII text. Do not carry a resolved
issue forward because an older version of this guide named it (closed items:
`.agents/memorials/skills-security-formal-module-security.md`).

### Performance Opportunities

1. **Combined regex patterns** — Checkers with multiple patterns could combine
   related passes when profiling shows regex traversal is material.

2. **Early termination** — When `--severity critical` is used, checkers could skip checks
   that can only produce findings below Critical. Currently all checks run regardless.

3. **Parallel `BuildScanContext`** — If the discovery package supports concurrent parsing,
   `loadDirectoryTree` could parallelize module loading.

### Security Opportunities

1. **Broader discovery budgets** — The scan-wide, batched Lua-file and symlink
   budgets cover recursive artifact walks, not every discovery or parsing
   operation. If profiling exposes another unbounded surface, extend resource
   accounting without weakening the existing typed, fail-closed behavior.

2. **Content-based detection** — Currently all script analysis is regex-based. For higher
   accuracy, consider AST-based analysis for shell scripts using `mvdan.cc/sh/v3/syntax`
   (already a dependency) to detect patterns like subshell nesting, process substitution,
   and indirect variable expansion.
