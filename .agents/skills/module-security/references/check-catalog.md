# Security Check Catalog

Detailed check specifications for each scanner subagent. Each section corresponds
to one `checks_*.go` file in `internal/audit/` and one subagent in Phase 2.

## Table of Contents

- [Lock File Integrity](#lock-file-integrity) — `checks_lockfile.go`
- [Script Path & Content](#script-path--content) — `checks_script.go`
- [Network Access Detection](#network-access-detection) — `checks_network.go`
- [Environment Variable Risks](#environment-variable-risks) — `checks_env.go`
- [Symlink Detection](#symlink-detection) — `checks_symlink.go`
- [Module Metadata Analysis](#module-metadata-analysis) — `checks_module.go`
- [Finding Correlation Rules](#finding-correlation-rules) — Phase 3 cross-checks

---

## Lock File Integrity

**Scanner subagent:** Lock File Integrity Scanner
**Key files:** `pkg/invowkmod/lockfile.go`, `content_hash.go`, `resolver_cache.go`

| Check | What | How | Severity | Finding |
|-------|------|-----|----------|---------|
| Hash mismatch | Content hash in lock file vs actual module content | Reuse `invowkmod.computeModuleHash()` (needs exported accessor or shared helper) | Critical | "Module content hash mismatch — module may have been tampered with" |
| Orphaned lock entries | Lock file entries without corresponding `requires` | Cross-reference lock file modules against `invowkmod.cue:requires` | Low | "Lock file contains entry not in requires" |
| Missing lock entries | `requires` entries without lock file entry | Cross-reference `requires` against lock file | Medium | "Required module has no lock file entry — run `invowk module sync`" |
| Lock file version | Unknown lock file version | Check `version` field against known versions (currently v2.0) | High | "Unknown lock file version — may have been crafted" |
| Lock file size | Crafted multi-GB lock file for DoS | `os.Stat` before `parseLockFileCUE` (match 5MB CUE guard) | Medium | "Lock file exceeds size limit" |
| TOCTOU gap | Hash verified then module modified before load | Check hash → use in atomic sequence | High | "Time-of-check-time-of-use gap in hash verification" |

**Implementation notes:**
- `parseLockFileCUE()` is a line-by-line parser (not full CUE evaluation) with brace-depth tracking
- `computeModuleHash()` at line 91 in `content_hash.go` must skip symlinks during walk
- `fspath.AtomicWriteFile()` required for all lock file writes (crash safety)

---

## Script Path & Content

**Scanner subagent:** Script Path & Content Analyzer
**Key files:** `pkg/invowkfile/implementation.go` (lines 266–329)

### Path Checks

| Check | What | How | Severity | Finding |
|-------|------|-----|----------|---------|
| Path traversal | Script fields using `../` or absolute paths | Parse invowkfile, inspect `Implementation.Script` via `IsScriptFile()` heuristic | High | "Script references path outside module boundary" (SC-01) |
| Absolute paths in modules | Module script fields with absolute paths (`/usr/bin/...`) | Reject absolute paths for module commands (root invowkfile may allow them) | High | "Module script uses absolute path — bypasses module boundary" |
| Missing bounds check | `GetScriptFilePathWithModule()` lacks `filepath.Rel` containment | Compare against `ValidateContainerfilePath` and `ValidateEnvFilePath` patterns | High | "Script path resolution unbounded — no containment check" |

### Content Checks

| Check | What | How | Severity | Finding |
|-------|------|-----|----------|---------|
| Obfuscation | `base64 -d`, `eval`, backtick execution, `$()` with encoded content | Regex scan of resolved script content | High | "Script contains obfuscation pattern" |
| Remote execution | `curl \| sh`, `wget -O- \| bash`, piped remote execution | Regex: `(curl\|wget\|fetch).*\|.*(sh\|bash\|python)` | Critical | "Script downloads and executes remote code" |
| Large scripts | Scripts exceeding 5MB (matching CUE guard) | `os.Stat` before read | Medium | "Script file unusually large" |
| Interpreter paths | Arbitrary shebang / `interpreter` field values | Check against known-safe interpreters; flag unusual paths | Medium | "Unusual interpreter path — no allowlist enforcement" (SC-08) |

**Regex patterns for content scanning:**
```
# Remote execution (Critical)
(curl|wget|fetch)\s+[^\|]*\|\s*(sh|bash|zsh|python|perl|ruby)
(curl|wget)\s+.*-[sS].*\|\s*(sh|bash)

# Obfuscation (High)
base64\s+(-d|--decode)
\beval\b\s+[\$\"\']
\$\(.*base64
echo\s+[A-Za-z0-9+/=]{20,}\s*\|\s*base64

# Encoded content indicators
\\x[0-9a-fA-F]{2}{3,}
\$'\x[0-9a-fA-F]
```

---

## Network Access Detection

**Scanner subagent:** Network/Env Exfiltration Scanner (shared with env checks)
**Scanned as part of the combined exfiltration subagent.**

| Check | What | How | Severity | Finding |
|-------|------|-----|----------|---------|
| Network commands | `curl`, `wget`, `nc`, `ncat`, `socat` usage | Keyword scan in script content | Medium | "Script uses network access command" |
| Encoded URLs | Base64-encoded URLs, hex-encoded domains | Regex for `aHR0c` (base64 "http") and `\x` sequences | High | "Script contains encoded URL" |
| DNS exfiltration | `dig`, `nslookup`, `host` with variable interpolation | Regex: `(dig\|nslookup\|host).*\$` | High | "Possible DNS exfiltration pattern" |
| Reverse shell | `bash -i >& /dev/tcp/`, `nc -e`, `python -c ... socket` | Regex patterns for common reverse shell one-liners | Critical | "Reverse shell pattern detected" |

**Regex patterns:**
```
# DNS exfiltration (High)
(dig|nslookup|host)\s+.*\$[\{(]?[A-Z_]

# Reverse shell (Critical)
bash\s+-i\s+>&\s*/dev/tcp/
\bnc\b.*-e\s*/bin/(ba)?sh
python[23]?\s+-c\s+.*socket.*connect
```

---

## Environment Variable Risks

**Scanner subagent:** Network/Env Exfiltration Scanner (shared with network checks)

| Check | What | How | Severity | Finding |
|-------|------|-----|----------|---------|
| Sensitive var access | Reading `$HOME`, `$SSH_AUTH_SOCK`, `$AWS_*`, `$GITHUB_TOKEN`, etc. | Regex scan for known sensitive variable names | Medium | "Script accesses sensitive environment variable" |
| `env_inherit_mode: all` | Module commands inheriting full host environment | Parse invowkfile env config | Low | "Command inherits all host environment variables" |
| Token extraction | Scripts reading token/secret vars and writing to files or pipes | Pattern: `$TOKEN.*>` or `echo.*$SECRET.*\|` | High | "Script may extract credential to external sink" |

**Sensitive variable patterns:**
```
# Credentials (Medium → High when correlated with network)
\$(AWS_SECRET_ACCESS_KEY|AWS_SESSION_TOKEN|GITHUB_TOKEN|GH_TOKEN)
\$(SSH_AUTH_SOCK|GPG_AGENT_INFO)
\$(DATABASE_URL|REDIS_URL|MONGODB_URI)
\$\{?(API_KEY|SECRET_KEY|PRIVATE_KEY|ACCESS_TOKEN)\}?

# System paths (Medium)
\$(HOME|USERPROFILE)
\$(PATH|LD_PRELOAD|LD_LIBRARY_PATH)
```

---

## Symlink Detection

**Scanner subagent:** Symlink Detector

| Check | What | How | Severity | Finding |
|-------|------|-----|----------|---------|
| Symlinks in modules | Symlinks inside `.invowkmod` directories | `filepath.WalkDir` with `d.Type()&os.ModeSymlink` | High | "Symlink found in module directory — may reference content outside module boundary" (SC-05) |
| Symlink targets outside boundary | Where symlinks point (inside vs outside module) | `os.Readlink` + `filepath.Rel` check | Critical | "Symlink points outside module boundary" |
| Dangling symlinks | Symlinks pointing to nonexistent targets | `os.Stat` after `os.Readlink` | Low | "Dangling symlink in module directory" |
| Symlink chains | Symlinks pointing to other symlinks | Follow chain with depth limit (max 10) | Medium | "Symlink chain detected — may obscure final target" |

**Implementation notes:**
- Two different `copyDir` implementations exist with different symlink handling:
  - `pkg/invowkmod/resolver_cache.go:copyDir` — **skips symlinks** (safe)
  - `internal/provision/helpers.go:CopyDir` — **follows symlinks** via `os.ReadDir` (unsafe, SC-05)
- The symlink scanner should flag the `CopyDir` divergence if it detects symlinks in module trees

---

## Module Metadata Analysis

**Scanner subagent:** Module Metadata Analyzer

| Check | What | How | Severity | Finding |
|-------|------|-----|----------|---------|
| Dependency chain depth | Unusually deep transitive dependency chains | Walk `requires` transitively, count depth | Medium | "Deep dependency chain (depth N) — increases supply-chain attack surface" |
| Namespace collision | Modules with similar IDs (typosquatting) | Levenshtein distance between module IDs in the dependency tree | Medium | "Module ID similar to another module — possible typosquatting" |
| Global module warnings | Modules in `~/.invowk/cmds/` with no integrity verification | Check `IsGlobalModule` flag, report all global modules | Info | "Global module has no content hash verification" (SC-10) |
| Undeclared transitive deps | Modules used transitively but not in root `requires` | Same logic as `checkMissingTransitiveDeps()` in `resolver_deps.go` | Medium | "Transitive dependency not declared in root invowkmod.cue" |
| Version pinning | Modules without explicit version constraints | Check `requires` entries for version presence | Low | "Module dependency has no version constraint" |

---

## Finding Correlation Rules

Phase 3 cross-checks combine findings from Phase 2 scanners to detect compound threats.
The Finding Correlator subagent reads all Phase 2 outputs and applies these rules.

| Correlation | Input Findings | Output | Severity |
|-------------|---------------|--------|----------|
| Credential exfiltration | Env: sensitive var access + Network: network command | "Potential credential exfiltration — reads sensitive env vars and has network access" | High → Critical (escalation) |
| Path + symlink escape | Script: path traversal + Symlink: external target | "Combined path traversal and symlink escape — high confidence data access" | Critical |
| Obfuscated exfiltration | Script: obfuscation + Network: encoded URL | "Obfuscated network access — likely deliberate evasion" | Critical |
| Trust chain weakness | Module: deep deps + Lock: missing entries | "Deep dependency chain with unverified modules — elevated supply-chain risk" | High (escalation) |
| Interpreter + traversal | Script: unusual interpreter + Script: path traversal | "Arbitrary interpreter with path traversal — potential arbitrary code execution" | Critical |

**Escalation rules:**
- Medium + Medium in same module → High (compound risk)
- High + any in same module → Critical (active threat indicator)
- Findings spanning 3+ categories in one module → always Critical
