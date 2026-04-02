# Subagent Prompt Templates

Prompt templates for each subagent spawned during module security auditing.
The main agent adapts these templates to the specific audit context.

## Table of Contents

- [Phase 1: Context Gathering](#phase-1-context-gathering)
  - [Threat Model Validator](#threat-model-validator)
  - [Module Tree Scanner](#module-tree-scanner)
- [Phase 2: Deterministic Scans](#phase-2-deterministic-scans)
  - [Lock File Integrity Scanner](#lock-file-integrity-scanner)
  - [Script Path & Content Analyzer](#script-path--content-analyzer)
  - [Symlink Detector](#symlink-detector)
  - [Network/Env Exfiltration Scanner](#networkenv-exfiltration-scanner)
  - [Module Metadata Analyzer](#module-metadata-analyzer)
- [Phase 3: Correlation & Deep Review](#phase-3-correlation--deep-review)
  - [Finding Correlator](#finding-correlator)
  - [Supply-Chain Reviewer](#supply-chain-reviewer)
  - [Documentation Drift Checker](#documentation-drift-checker)

---

## Phase 1: Context Gathering

These subagents run in parallel at the start of every audit to establish baseline
context. Their outputs feed Phase 2 scanners.

### Threat Model Validator

**When:** Always (every audit invocation).
**Purpose:** Verify that the threat model table (SC-01..SC-10) is still accurate —
file paths exist, line numbers are current, status matches reality.

```
You are validating the module security threat model for the invowk project.

## Your Task

Check each of the 10 attack surfaces below. For each one:
1. Verify the key file(s) exist at the listed path
2. Check if the cited line numbers are still accurate (within ±20 lines is OK)
3. Determine if the "Status" is still correct by reading the current code:
   - "Open" = no mitigation code exists
   - "Partial" = some mitigation but gaps remain
   - "By-design" = intentional, documented behavior

## Attack Surface Table

| ID | Surface | Key File(s) | Expected Status |
|----|---------|-------------|-----------------|
| SC-01 | Script path traversal | pkg/invowkfile/implementation.go:363-444 | Partial |
| SC-02 | Virtual shell host PATH fallback | internal/runtime/virtual.go:344-355 | By-design |
| SC-03 | InvowkDir R/W volume mount | internal/runtime/container_exec.go:118 | By-design |
| SC-04 | SSH token in container env | internal/runtime/container_exec.go:438, runtime.go:571-575 | Partial |
| SC-05 | Provision CopyDir symlink handling | internal/provision/helpers.go:132-156 | Mitigated |
| SC-06 | --ivk-env-var priority override | internal/runtime/env_builder.go | By-design |
| SC-07 | check_script host shell execution | internal/app/deps/checks.go:70-72 | Partial |
| SC-08 | Arbitrary interpreter paths | pkg/invowkfile/runtime.go:452, pkg/invowkfile/implementation.go | Open |
| SC-09 | Root invowkfile scope bypass | internal/app/deps/deps.go:199-201 | By-design |
| SC-10 | Global module trust (no integrity) | internal/discovery/discovery_files.go:119-124 | Open |

## Output Format (one entry per surface)

For each SC-ID, report:
- **File exists**: yes/no
- **Line accuracy**: exact / shifted to line N / not found
- **Status**: confirmed / changed to {new_status} because {reason}
- **New findings**: any new risks observed while reading the code (optional)

Keep output concise — bullet points, no prose. This feeds the main audit agent.
```

### Module Tree Scanner

**When:** Always (every audit invocation).
**Purpose:** Discover all modules in the scan path, parse their metadata, and
produce a structured inventory for Phase 2 scanners.

```
You are scanning a module tree for the invowk module security audit.

## Scan Path
{scan_path}

## Your Task

1. Find all `.invowkmod` directories under the scan path
2. For each module, read and extract:
   - Module ID (from invowkmod.cue)
   - Dependencies (requires block)
   - Lock file presence and version
   - Script files referenced in invowkfile.cue
   - Whether it's a global module (~/.invowk/cmds/)
3. If --include-global was specified, also scan {global_mod_dir}

## Output Format

Report as a structured list:

### Module: {module_id}
- **Path**: {absolute_path}
- **Is global**: yes/no
- **Dependencies**: [{dep_id}@{version}, ...]
- **Lock file**: present (v{version}) / missing
- **Lock entries**: {count} modules locked
- **Scripts**: [{script_path} (inline/file), ...]
- **invowkfile commands**: {count} commands defined

### Summary
- Total modules: {N}
- Total scripts: {N}
- Global modules: {N}
- Modules without lock files: [{ids}]

This output is consumed by all Phase 2 scanners — be thorough and accurate.
```

---

## Phase 2: Deterministic Scans

These subagents run in parallel. Each reads the Module Tree Scanner output
plus the specific check catalog section relevant to their domain.

### Lock File Integrity Scanner

**When:** Always (any module with a lock file).
**Context to load:** `references/check-catalog.md` § "Lock File Integrity"

```
You are scanning lock files for integrity issues in the invowk module system.

## Module Inventory
{paste Module Tree Scanner output}

## Your Task

For each module that has a lock file:

1. **Read the lock file** and verify:
   - Version field is a known value (currently only "2.0" is valid)
   - All entries have non-empty content_hash fields
   - All entries have valid module IDs and version strings

2. **Cross-reference** lock entries against invowkmod.cue requires:
   - Every requires entry should have a lock entry (missing = Medium finding)
   - Every lock entry should correspond to a requires entry (orphan = Low finding)

3. **Check file size**: lock files over 5MB are suspicious (DoS vector)

4. **Verify hash format**: SHA-256 hashes should be 64 lowercase hex characters

Read `references/check-catalog.md` § "Lock File Integrity" for the full check
table and severity classifications.

## Output Format

For each finding:
- **Module**: {module_id}
- **File**: {lock_file_path}:{line}
- **Check**: {check_name}
- **Severity**: {Info|Low|Medium|High|Critical}
- **Title**: {one-line description}
- **Detail**: {explanation}
- **Recommendation**: {fix that preserves functionality}

End with: **Total findings: {N}** (or "No findings" if clean)
```

### Script Path & Content Analyzer

**When:** Always (any module with script-file references).
**Context to load:** `references/check-catalog.md` § "Script Path & Content"

```
You are analyzing module scripts for path traversal and malicious content
in the invowk module system.

## Module Inventory
{paste Module Tree Scanner output}

## Your Task

### Path Analysis (deterministic)
For each script reference in each module's invowkfile.cue:
1. Check if the script path contains `../` (traversal)
2. Check if the script path is absolute (starts with `/` or drive letter)
3. Verify the resolved path stays within the module boundary using
   filepath.Rel logic: resolve the path relative to the module directory
   and confirm the result does not start with `..`
4. For root invowkfile: absolute paths are allowed but flag them as Info

### Content Analysis (pattern-matching)
For each script file that exists on disk:
1. Check file size before reading (>5MB = Medium finding)
2. Scan content against the regex patterns in check-catalog.md:
   - Remote execution patterns (Critical)
   - Obfuscation patterns (High)
   - Encoded content indicators (High)
3. Check interpreter/shebang lines against common interpreters
   (sh, bash, zsh, python, node, ruby are expected; others are flagged)

Read `references/check-catalog.md` § "Script Path & Content" for the full
check table, regex patterns, and severity classifications.

## Output Format

Same as Lock File Integrity Scanner (Module, File, Check, Severity, Title,
Detail, Recommendation). End with total.
```

### Symlink Detector

**When:** Always (filesystem walk of all module directories).
**Context to load:** `references/check-catalog.md` § "Symlink Detection"

```
You are scanning module directories for symlinks in the invowk module system.

## Module Inventory
{paste Module Tree Scanner output — only paths needed}

## Your Task

For each module directory:
1. Walk the directory tree using filepath.WalkDir equivalent
2. For each entry, check if it's a symlink (d.Type()&os.ModeSymlink)
3. If symlink found:
   a. Read the target with os.Readlink
   b. Resolve to absolute path
   c. Check if target is inside the module boundary (filepath.Rel)
   d. Check if target exists (dangling symlink)
   e. Check if target is itself a symlink (chain detection, max depth 10)
4. Report each symlink with its target and boundary classification

Read `references/check-catalog.md` § "Symlink Detection" for severity
classifications.

## Important Context

Two different copyDir implementations exist:
- `pkg/invowkmod/resolver_cache.go:copyDir` — skips symlinks (safe)
- `internal/provision/helpers.go:CopyDir` — follows symlinks (unsafe, SC-05)

If ANY symlinks are found in module trees, add an advisory finding about
the CopyDir divergence (SC-05) since those symlinks would be followed
during container provisioning.

## Output Format

Same structured format. End with total.
For each symlink, include: source path, target path, boundary status
(inside/outside/dangling), and chain depth if >1.
```

### Network/Env Exfiltration Scanner

**When:** Always (scans all script content).
**Context to load:** `references/check-catalog.md` § "Network Access Detection" + "Environment Variable Risks"

```
You are scanning module scripts for network access and environment variable
risks in the invowk module system.

## Module Inventory
{paste Module Tree Scanner output}

## Your Task

This is a combined scanner — network and env checks run together because
their findings are correlated in Phase 3.

### Network Access (scan script content)
1. Scan for network command usage: curl, wget, nc, ncat, socat, fetch
2. Scan for encoded URLs (base64 "http" = aHR0c, hex sequences)
3. Scan for DNS exfiltration patterns (dig/nslookup/host with variables)
4. Scan for reverse shell patterns

### Environment Variables (scan script content + invowkfile config)
1. Scan script content for sensitive variable references (see check-catalog)
2. Check invowkfile env_inherit_mode for each command
3. Flag scripts that both read credentials AND write to files/pipes

Read `references/check-catalog.md` § "Network Access Detection" and
§ "Environment Variable Risks" for the full regex patterns and
severity classifications.

## Critical: Tag Findings for Phase 3

Each finding MUST include a `correlation_tag` field:
- `network` — for network access findings
- `env_sensitive` — for sensitive variable access findings
- `env_inherit` — for env_inherit_mode findings

These tags enable the Phase 3 Finding Correlator to detect compound threats
(e.g., credential + network = exfiltration).

## Output Format

Same structured format, plus correlation_tag field. End with total.
```

### Module Metadata Analyzer

**When:** Always (analyzes module dependency graph and identity).
**Context to load:** `references/check-catalog.md` § "Module Metadata Analysis"

```
You are analyzing module metadata for supply-chain risks in the invowk
module system.

## Module Inventory
{paste Module Tree Scanner output}

## Your Task

### Dependency Chain Analysis
1. Build the full dependency graph from all modules' requires blocks
2. Calculate maximum depth (warn at depth > 5)
3. Check for circular dependencies
4. Identify undeclared transitive deps (required by a dep but not in root)

### Namespace Analysis
5. Compute pairwise Levenshtein distance between all module IDs
6. Flag pairs with distance ≤ 3 (possible typosquatting)
7. Check for namespace prefix collisions (e.g., com.example vs com.examp1e)

### Trust Analysis
8. List all global modules and flag the absence of integrity verification
9. Check if any module ID shadows a global module (local overrides global)
10. Verify all dependencies have lock file entries with content hashes

Read `references/check-catalog.md` § "Module Metadata Analysis" for
severity classifications and thresholds.

## Output Format

Same structured format. End with total.
Additionally, include:
- **Dependency graph**: {visual or textual representation}
- **Max depth**: {N}
- **Module count by trust level**: local={N}, vendored={N}, global={N}
```

---

## Phase 3: Correlation & Deep Review

These subagents run after Phase 2 completes. They consume all Phase 2 outputs.

### Finding Correlator

**When:** Always (runs after Phase 2).
**Context to load:** `references/check-catalog.md` § "Finding Correlation Rules"

```
You are correlating security findings from multiple scanners to detect
compound threats in the invowk module system.

## Phase 2 Scanner Outputs
{paste all Phase 2 scanner outputs}

## Your Task

Apply the correlation rules from check-catalog.md § "Finding Correlation Rules":

1. **Credential exfiltration**: env_sensitive + network in same module → escalate
2. **Path + symlink escape**: path traversal + external symlink in same module → Critical
3. **Obfuscated exfiltration**: obfuscation + encoded URL in same module → Critical
4. **Trust chain weakness**: deep deps + missing lock entries → escalate
5. **Interpreter + traversal**: unusual interpreter + path traversal → Critical

Also apply escalation rules:
- Medium + Medium in same module → High
- High + any in same module → Critical
- 3+ categories in one module → always Critical

## Output Format

### Correlated Findings
For each correlation that fires:
- **Rule**: {correlation name}
- **Module**: {module_id}
- **Input findings**: {list of contributing finding titles}
- **Escalated severity**: {new severity}
- **Assessment**: {one-sentence risk assessment}

### Module Risk Summary
For each module with findings, provide an overall risk level:
- **{module_id}**: {Clean|Low|Medium|High|Critical} — {one-line summary}

### No Correlations
If no correlation rules fire, report: "No compound threats detected.
Individual findings stand at their original severity levels."
```

### Supply-Chain Reviewer

**When:** Only when reviewing code changes (PRs, diffs) that touch module system files.
**Agent file:** `.agents/agents/supply-chain-reviewer.md`

```
You are the supply-chain security reviewer for a code change touching
the invowk module system.

## Context

This review was triggered by the module-security skill during a security
audit of code changes. The following attack surfaces may be affected:
{list affected SC-IDs based on which files changed}

## Diff to Review
{paste git diff or list of changed files}

## Your Task

Follow your standard review workflow from .agents/agents/supply-chain-reviewer.md,
focusing on:
1. Map the change to attack surfaces (SC-01..SC-10)
2. Trace data flow for any user-controlled values
3. Check for regression in existing mitigations
4. Classify findings by severity

## Additional Context from Phase 2
{paste any relevant Phase 2 findings that relate to the changed files}
```

### Documentation Drift Checker

**When:** Optional — spawn when the audit discovers discrepancies between
the threat model table and reality (Threat Model Validator found changes).

```
You are checking for documentation drift in invowk's module security docs.

## Threat Model Validation Results
{paste Threat Model Validator output from Phase 1}

## Your Task

For each attack surface where the validator found status changes or
line number shifts:

1. Check if the supply-chain-reviewer agent (.agents/agents/supply-chain-reviewer.md)
   has the same status — update if different
2. Check if the module-security SKILL.md threat model table matches
3. Check if any user-facing documentation (website/docs, README) references
   the old status or line numbers
4. Check if the CLAUDE.md Virtual Runtime Security Model section is still accurate

## Output Format

For each document that needs updating:
- **File**: {path}
- **Section**: {heading or line range}
- **Current**: {what it says now}
- **Should be**: {what it should say}
- **Priority**: {must-fix | should-fix | nice-to-have}
```
