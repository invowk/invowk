# Security Check Catalog

Detailed check specifications for each compiled scanner checker. Derive the
current default list from `DefaultCheckers()` in `internal/audit/scanner.go`;
this catalog explains behavior and must not be used as a substitute for the
source inventory.

Finding titles below are the current ones. For the complete, authoritative
list use `internal/audit/finding_codes.go` (every stable code) plus:

```bash
rg -n 'Title|title' internal/audit/checks_*.go internal/audit/correlator*.go
```

That search also catches the positional titles in the ScriptChecker
obfuscation table and titles held in constants (`reverseShellFindingTitle`,
`scriptSensitiveEnvVarTitle`, `obfuscatedNetworkAccessTitle`,
`highSeverityCombinedIssuesTitle`).

## Table of Contents

- [Lock File Integrity](#lock-file-integrity) — `checks_lockfile.go`
- [Script Path & Content](#script-path--content) — `checks_script.go`
- [Network Access Detection](#network-access-detection) — `checks_network.go`
- [Environment Variable Risks](#environment-variable-risks) — `checks_env.go`
- [Virtual Lua Risks](#virtual-lua-risks) — `checks_lua.go`
- [Symlink Detection](#symlink-detection) — `checks_symlink.go`
- [Module Metadata Analysis](#module-metadata-analysis) — `checks_module.go`
- [Finding Correlation Rules](#finding-correlation-rules) — compiled correlator, runs in Phase 1 (`correlator.go`)

---

## Lock File Integrity

**Checker:** `LockFileChecker`

**Key files:** `pkg/invowkmod/lockfile.go`, `lockfile_parser.go`, `content_hash.go`, `verify.go`, `vendored_policy.go`

The lock snapshot comes from `invowkmod.InspectLockFile`: `os.Stat`, then a
size guard (`invowkmod.LockFileSizeLimit`, 5 MiB) before reading, then
`parseLockFile`, which uses the CUE parser and evaluator.

| Check | What | How | Severity | Finding |
|-------|------|-----|----------|---------|
| Unparseable lock | Lock present but stat-ok and unreadable, oversized, or invalid CUE | `LockFileParseErr` from the snapshot; skips every other lock check for that module | Medium | "Lock file present but unparseable" |
| No lock with dependencies | Module declares `requires` but has no lock | `mod.LockFile == nil` and non-empty requires | High | "Module has dependencies but no lock file" |
| Vendored without lock | `invowk_modules/` content but no lock | `mod.LockFile == nil` and vendored modules present | Medium | "Vendored modules present without lock file" |
| Lock file size | Crafted multi-GB lock file for DoS | `checkSize` on the snapshot size and stat error | Medium / Low | "Lock file exceeds size limit" / "Lock file size could not be verified" |
| Lock file version | Unknown version; v1.0 has no content hashes | `checkVersion` against `invowkmod.LockFileVersionV1` / `LockFileVersionV2` | High / Medium (High when vendored modules exist) | "Unknown lock file version"; "Lock file uses v1.0 format without content hashes"; "Vendored modules cannot be verified — V1 lock file has no content hashes" |
| Ambiguous entries | One module ID with several lock entries | `invowkmod.FindAmbiguousLockedModuleEntries` | Medium | "Ambiguous lock file entries for same module" |
| Hash mismatch | Content hash in lock file vs actual vendored content | `invowkmod.EvaluateVendoredModuleHash`, injected through `WithHashEvaluator` | Critical | "Module content hash mismatch" |
| Hash evaluation gaps | Vendored module missing from the lock, or hash not computable / status unknown | Same evaluation result | Medium / High / High | "Vendored module has no lock file entry"; "Vendored module hash could not be computed"; "Vendored module hash status is unknown" |
| Orphaned lock entries | Lock file entries without corresponding `requires` | `invowkmod.OrphanedLockedModuleEntries`; first 10 reported individually | Low | "Orphaned lock file entry"; "Additional orphaned lock file entries" |
| Missing lock entries | `requires` entries without an exact lock file entry | `invowkmod.MissingLockedModuleRequirementKeys` | Medium | "Required module has no lock file entry" |

**Implementation notes:**
- Size-guard reachability: `InspectLockFile` turns an oversized lock into a
  parse error, and a stat failure leaves no parsed lock. `Check` `continue`s on
  both branches before `checkSize` runs, so through `BuildScanContext` an
  oversized lock currently surfaces as "Lock file present but unparseable".
  Verify reachability before relying on the two `checkSize` titles.
- `computeModuleHash()` / exported hash helpers must skip symlinks during walk
- `fspath.AtomicWriteFile()` required for all lock file writes (crash safety)

---

## Script Path & Content

**Checker:** `ScriptChecker`

**Key files:** `pkg/invowkfile/implementation.go`, `pkg/invowkfile/script_file_path.go`

Parse-time validation (`ScriptFilePath.Validate`) already rejects absolute
paths and `..` segments; these checks report the same conditions on scanned
content, and runtime containment lives in `validateScriptPathContainment`.

### Path Checks

| Check | What | How | Severity | Finding |
|-------|------|-----|----------|---------|
| Path traversal | Module `script.file` values using `../` or `..\` | Inspect explicit `Implementation.Script.File` references in module context | High | "Script references path outside module boundary" (SC-01) |
| Absolute paths in modules | Module script fields with absolute paths (`/usr/bin/...`) | Reject absolute paths for module commands (root invowkfile may allow them) | High | "Module script uses absolute path" |

### Content Checks

| Check | What | How | Severity | Finding |
|-------|------|-----|----------|---------|
| Remote execution | `curl \| sh`, `bash <(curl …)`, download then run | Three patterns: `remoteExecPattern`, `processSubstitutionPattern`, `downloadExecPattern` | Critical | "Script downloads and executes remote code" |
| Obfuscation | base64 decode, dynamic `eval`, base64 in a subshell, long encoded string piped to base64 | `base64DecodePattern`, `evalPattern`, `base64SubshPattern`, `encodedPipePattern` | High | "Script contains base64 decode"; "Script uses eval with dynamic content"; "Script uses base64 in subshell"; "Script pipes encoded content to base64" |
| Hex escapes | Three or more `\xNN` sequences | `hexSequencePattern` | High | "Script contains hex escape sequences" |
| Content path traversal | `../` in module script content not already reported by the path checks (root invowkfile scripts are skipped) | `pathTraversalPattern` when `ref.ModulePath` is set | Medium | "Script content contains path traversal" |
| Large scripts | Module scripts exceeding 5 MiB (matching the CUE guard) | Pre-computed file size on `ScriptRef`; standalone scripts skipped | Medium | "Script file unusually large" |

**Not scanned: interpreters (SC-08).** No compiled checker inspects
interpreter values. SC-08 is enforced only by the `InterpreterSpec` allowlist
at `Validate` (`pkg/invowkfile/interpreter_spec.go`), which checks the
basename, so an absolute path such as `/tmp/python3` passes. The correlator
title "Unusual interpreter with path traversal" actually pairs ScriptChecker
execution and path-traversal findings (see below).

**Patterns:** the top-level `var` block in `internal/audit/checks_script.go`
is the only source. Remote-exec patterns cover `curl`, `wget` and `fetch`
piped or substituted into `sh`, `bash`, `zsh`, `ash`, `dash`, `python`, `perl`,
`ruby`, `node`, `php` and PowerShell, plus `-o`/`--output-document` downloads
followed by `sh`, `bash` or `chmod +x`.

---

## Network Access Detection

**Checker:** `NetworkChecker`

| Check | What | How | Severity | Finding |
|-------|------|-----|----------|---------|
| Network commands | `curl`, `wget`, `aria2c`, `nc`, `ncat`, `socat` usage | `networkCommandPattern` keyword scan; skipped when a reverse shell was found | Medium | "Script uses network access command" |
| Encoded URLs | Base64-encoded URLs | `base64HTTPPattern` matches `aHR0c` (base64 "http"); hex escapes belong to ScriptChecker | High | "Script contains encoded URL" |
| DNS exfiltration | `dig`, `nslookup`, `host` with variable interpolation or backtick expansion | `dnsExfilPattern` | High | "Possible DNS exfiltration pattern" |
| Reverse shell | `bash -i >& /dev/tcp/`, `nc`/`ncat` `-e`/`-c /bin/sh`, `python -c … socket … connect` | `reverseShellBashPattern`, `reverseShellNcPattern`, `reverseShellPythonPattern` | Critical | "Reverse shell pattern detected" |

**Patterns:** the top-level `var` block in `internal/audit/checks_network.go`.
Gotcha from the code: `fetch` is deliberately excluded from
`networkCommandPattern` to avoid false positives on `git fetch`.

---

## Environment Variable Risks

**Checker:** `EnvChecker`

| Check | What | How | Severity | Finding |
|-------|------|-----|----------|---------|
| Sensitive var access | Named credentials: AWS, GitHub, SSH agent, GPG, database URLs, Vault, Docker, npm, PyPI, Azure, GCP, `INVOWK_SSH_TOKEN`, `INVOWK_TUI_*` | `sensitiveVarPattern` | Medium | "Script accesses sensitive environment variable" |
| Generic secret access | `API_KEY`, `SECRET_KEY`, `PRIVATE_KEY`, `ACCESS_TOKEN`, `AUTH_TOKEN`, `PASSWORD`, with optional braces | `genericSecretPattern` | Medium | "Script accesses generic secret variable" |
| Explicit `env_inherit_mode: all` | Command inherits the full host environment on purpose | Runtime `EnvInheritMode == invowkfile.EnvInheritAll` | Low | "Command inherits all host environment variables" |
| Default inheritance | native or virtual-sh runtime with no `env_inherit_mode` (defaults to all); the R7 target | Empty `EnvInheritMode` | Info | "Command uses default env inheritance (all host variables)" |
| Token extraction | Credential variable followed by a pipe or redirect | `tokenExtractionPattern` (credential variable, then a later `\|` or `>`) | High | "Script may extract credential to external sink" |

**Patterns:** the top-level `var` block in `internal/audit/checks_env.go`.
Gotcha from the code: keep `tokenExtractionPattern` in sync with
`sensitiveVarPattern` whenever a credential name is added.

---

## Virtual Lua Risks

**Checker:** `LuaChecker`

`LuaChecker` examines only commands with a `virtual-lua` runtime. Confirm the
current patterns and finding codes in `internal/audit/checks_lua.go` and
`internal/audit/finding_codes.go` before changing this catalog.

| Check | What | Severity |
|-------|------|----------|
| Disabled API reference | Lua content names APIs that the embedded runtime disables or replaces | Low |
| Sensitive env read | `os.getenv` or `invowk.env` reads a credential-like name | Medium |
| Host-binary wildcard | `allowed_binaries: ["*"]` permits arbitrary native host execution | High |
| Network-capable host binary | Explicit allowlist includes tools such as `curl`, `ssh`, or `rsync` | Medium |
| Full filesystem access | Virtual filesystem policy grants broad host filesystem reach | High |
| Broad path mapping | Restricted mode maps a root, home, empty, or traversal-capable path | Medium |

These findings describe host reach from virtual-lua; they do not imply that the
virtual runtime is a security sandbox. Recommend the container runtime when
execution isolation is required.

---

## Symlink Detection

**Checker:** `SymlinkChecker`

Symlink facts are collected during scan-context construction
(`internal/audit/scan_context_artifacts.go`, bounded artifact walker) and
reported by the checker.

| Check | What | How | Severity | Finding |
|-------|------|-----|----------|---------|
| Symlinks in modules | Symlinks inside `.invowkmod` directories | Batched `ReadDir` with `DirEntry.Type()&os.ModeSymlink` | High | "Symlink found in module directory" (SC-05) |
| Symlink targets outside boundary | Where symlinks point (inside vs outside module) | `os.Readlink` + boundary check against the module root | Critical | "Symlink points outside module boundary" |
| Unreadable target | `os.Readlink` failed, so the boundary check could not run | Recorded read error | Medium | "Symlink target could not be read — boundary check bypassed" |
| Dangling symlinks | Symlinks pointing to nonexistent targets | `os.Stat` after `os.Readlink` | Low | "Dangling symlink in module directory" |
| Symlink chains | Symlinks pointing to other symlinks | Follow chain with depth limit (`maxSymlinkChainDepth`, 10) | Medium | "Symlink chain detected" |
| Incomplete walk | The module directory walk stopped early | Recorded walk error | Low | "Module directory walk incomplete" |

**Implementation notes:**
- Module copy, provisioning, and scan-context artifact/clone paths must avoid
  following symlinked module content outside trusted roots.
- Treat any future change that follows symlinks during module copy/provisioning
  or scan artifact handling as SC-05 drift.

---

## Module Metadata Analysis

**Checker:** `ModuleMetadataChecker`

| Check | What | How | Severity | Finding |
|-------|------|-----|----------|---------|
| Invowkfile parse failure | Module invowkfile exists but does not parse, so content checks cannot see it | `InvowkfileParseErr` from scan-context loading | Medium | "Module invowkfile failed to parse" |
| Wide dependency fan-out | More than `maxDependencyFanOut` (5) direct `requires` | `checkDependencyFanOut` counts direct requires, not graph depth | Medium | "Wide dependency fan-out" |
| Namespace collision | Modules with similar IDs (typosquatting) | Levenshtein distance ≤ `typosquatLevenshteinThreshold` (3) between module IDs | Medium | "Module ID similar to another module" |
| Global module warnings | Modules in `~/.invowk/cmds/` with no integrity verification | Check `IsGlobalModule` flag, report all global modules | Info | "Global module has no content hash verification" (SC-10) |
| Undeclared transitive deps | Vendored modules' requires missing from the root `requires` | `checkUndeclaredTransitive` via `invowkmod.CheckMissingVendoredTransitiveDeps` (the same `MissingTransitiveDepDiagnostic` that `invowk module sync` reports through `MissingTransitiveDepError`) | Medium | "Transitive dependency not declared in root invowkmod.cue" |
| Undeclared vendored module | A module in `invowk_modules/` with no matching `requires` entry | Lock identity when a lock exists, source ID otherwise | Medium | "Vendored module not declared in requires" |
| Version pinning | Constraints that accept any version (`""`, `*`, `>=0.0.0`, `>0`, …) | `unpinnedVersions` set | Low | "Module dependency has no version constraint" |

---

## Finding Correlation Rules

The compiled correlator (`DefaultRules()` in `internal/audit/correlator.go`)
groups findings by scan surface and combines them into compound threats during
Phase 1.

| Correlation | Input Findings | Output | Severity |
|-------------|---------------|--------|----------|
| Credential exfiltration | EnvChecker + NetworkChecker findings on one surface | "Potential credential exfiltration" | Critical |
| Path + symlink escape | ScriptChecker + SymlinkChecker findings | "Combined path traversal and symlink escape" | Critical |
| Obfuscated exfiltration | ScriptChecker obfuscation + NetworkChecker exfiltration | "Obfuscated network access detected" | Critical |
| Trust chain weakness | ModuleMetadataChecker fan-out/undeclared codes + LockFileChecker missing-integrity codes | "Trust chain weakness — dependency graph with missing integrity" | High |
| Interpreter + traversal | ScriptChecker execution + ScriptChecker path-traversal | "Unusual interpreter with path traversal" | Critical |

**Escalation rules** (same surface, applied after the named rules; first
match wins, and none applies when the surface already has a Critical finding):
- Findings spanning 3+ categories → Critical ("Multiple security concern categories detected")
- High + any other finding → Critical ("High-severity finding combined with other issues")
- Medium + Medium → High ("Multiple medium-severity findings compound risk")
