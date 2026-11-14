# Module-security skill history

Moved out of `.agents/skills/module-security/` on 2026-11-14. None of this is normative; the skill and its references describe current behaviour.

## Workflow before the deterministic scanner

The audit originally fanned out to about 10 LLM subagents that pattern-matched module code. The compiled `invowk audit` scanner replaced that pattern matching, which cut the subagent count to at most 3 (Phase 3 only) and removed the main source of run-to-run variance.

## Design-era check-catalog rows

These rows described checks that were planned, or gaps that existed, when the catalog was first written. They are not current scanner findings.

- **TOCTOU "check"** (High, "Time-of-check-time-of-use gap in hash verification"): listed as a lock-file check; the current scanner has no such finding. The residual Stat-then-parse window stays in `references/implementation-review.md` under Missing Protections.
- **Unbounded script path resolution** (High, "Script path resolution unbounded — no containment check"): `GetScriptFilePathWithModule()` lacked a `filepath.Rel` containment check. Containment now lives in `validateScriptPathContainment`, applied by `ResolveScriptWithModule` / `ResolveScriptWithFSAndModule`. `ScriptFilePath.Validate` also rejects absolute paths and `..` segments at parse time.
- **Line-based lock parser**: `parseLockFileCUE()` was a line-by-line parser with brace-depth tracking. `pkg/invowkmod/lockfile_parser.go` `parseLockFile` now uses the CUE parser and evaluator.
- **Hash accessor need**: the hash-mismatch row said the checker must reuse `invowkmod.computeModuleHash()` through a new exported accessor. The checker now calls `invowkmod.EvaluateVendoredModuleHash`, injected through `WithHashEvaluator`.
- **Interpreter-path row in ScriptChecker** (Medium, "Unusual interpreter path — no allowlist enforcement", SC-08): the current ScriptChecker has no interpreter check. SC-08 is enforced by the `InterpreterSpec` allowlist in `pkg/invowkfile/interpreter_spec.go`.
- **Env "system paths" patterns** (`$HOME`, `$USERPROFILE`, `$PATH`, `LD_PRELOAD`, `LD_LIBRARY_PATH`): documented as Medium; the current `checks_env.go` has no such patterns.
- **Dependency chain depth** ("Deep dependency chain (depth N)"): it counted `len(requires)` rather than depth. It is now `checkDependencyFanOut`, titled "Wide dependency fan-out" (more than 5 direct requires).

## Closed implementation-review items

Each item was an open finding in `references/implementation-review.md` and has since been resolved in code.

- **Structured warnings**: the proposal for a `Warnings []string` report field became `Report.Diagnostics` and the CLI `diagnostics` output.
- **Slice-header accessors**: `ScanContext.Modules()`, `Invowkfiles()` and `AllScripts()` returned the underlying slices. They now return clones.
- **Error swallowing in `loadSingleModule`**: invowkfile and lock-file parse errors were silently dropped. `loadScannedModule` now records `InvowkfileParseErr` and `LockFileParseErr`, surfaced as "Module invowkfile failed to parse" and "Lock file present but unparseable".
- **Dependency "depth" naming**: see the fan-out rename above.
- **`/` path concatenation** in `checkScriptFileSize`: it now uses the pre-resolved `ref.ScriptPath`.
- **`strings.Contains` lock-key matching** in `checkMissingEntries`: a git URL that was a substring of another could produce a false positive. The checker now uses exact keys from `invowkmod.MissingLockedModuleRequirementKeys`.
- **Missing lock files skipped**: modules without a lock file were skipped. Now a module with `requires` and no lock gets "Module has dependencies but no lock file", and vendored content without a lock gets "Vendored modules present without lock file".
- **Two remote-exec patterns**, including a silent `-sS` variant: now three patterns (pipe, process substitution, download-then-execute).
- **Context checked only at goroutine start**: every default checker now checks `ctx.Done()` per module or script.
- **Version pinning missing `>0.0.0`**: `unpinnedVersions` now also covers `>0`, `>=0`, `>0.0.0`, `>=0.0.0-0`, `*.*` and `*.*.*`.
