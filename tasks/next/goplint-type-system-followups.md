# goplint & Type System Deferred Improvements

> **STATUS: COMPLETED** — all items from the 2026-02-28 goplint precision session.
> Items marked DONE were completed across `chore/lint-modern-go-enforcement` and
> `chore/goplint-type-system-followups-v2` branches.

## Type System Gaps

### 2E. Type `EnvConfig.Vars` Map Keys as `EnvVarName` — DONE

Changed `Vars map[string]string` → `map[EnvVarName]string` in `pkg/invowkfile/env.go`.
`GetVars()` converts keys back to `map[string]string` for backward compatibility with
`maps.Copy` consumers. Baseline -5 (240 → 235).

### 4B. Type `cueutil.ValidationError` Fields — DONE

Changed `FilePath` → `types.FilesystemPath`, `CUEPath` → new `CUEPath` type,
`Message`/`Suggestion` → `types.DescriptionText`. New `pkg/cueutil/cuepath.go` DDD type.

## Enforcement Improvements

### 3A. New Mode: `--audit-exceptions --global` — DONE

Subprocess-based aggregation in `main.go`. Reports patterns stale in ALL packages.

### 4E. Constructor Validates: Follow Method Calls on Return Type — DONE

Extended `bodyCallsValidateTransitive` to follow `*ast.SelectorExpr` method calls on
variables whose type matches `returnTypeName`. Uses existing `findMethodBody()`.

### 4I. New Advisory Mode: `--suggest-validate-all` — DONE

Reports structs with `Validate()` + validatable fields but no `//goplint:validate-all`
directive. NOT included in `--check-all` (advisory only).

## Structural Improvements

### 3D. Create Typed Path Wrapper `pkg/fspath/` — DONE

Created `pkg/fspath/` with `Join`, `JoinStr`, `Dir`, `Abs`, `Clean`, `FromSlash`, `IsAbs`.
Converted ~10 call sites in `pkg/invowkmod/` and `pkg/invowkfile/`. Also tightened
discovery cast-validation exceptions from 2 blanket patterns to 6 specific ones.

## Baseline Reclassification — DONE

### 4F. TUI/Container/Env Map — Baseline-to-Exception Migration — DONE

Migrated ALL 235 baseline findings to specific exception patterns with documented reasons.
Covers TUI (40), container (24), runtime (40), invowk (28), invowkfile (57), invowkmod (21),
provision (11), platform (4), sshserver (2), cueutil (3), config (1), fspath (1),
discovery (1), serverbase (1), plus 1 unvalidated-cast. Baseline: 235 → 0.
**Files:** `tools/goplint/exceptions.toml`, `tools/goplint/baseline.toml`.

### 4G. Constructor Exception Granularity — DONE (RETAINED)

Audited all 15 `pkg.*.constructor` blanket patterns via `--audit-exceptions --global`.
All 15 are ACTIVE (suppress 106 exported DTO structs across the codebase). The audit
tool falsely reported them as stale because it doesn't enable `--check-constructors` in
its subprocess invocation. Updated documentation in `exceptions.toml` to clarify.
**Files:** `tools/goplint/exceptions.toml`.

### 4H. Cast-Validation Exception Tightening — DONE

Replaced 8 blanket `pkg.{*,*.*}.cast-validation` patterns with specific function-level patterns:
- **container (2 blankets → 0):** 0 findings — all casts handled by auto-skip contexts.
- **provision (2 blankets → 0):** 0 findings — all casts handled by auto-skip contexts.
- **invowkfile (2 blankets → 0):** 0 findings — all casts handled by auto-skip contexts.
- **invowkmod (2 blankets → 7 specific):** Filesystem/parse boundary casts in findModuleInDir,
  parseLockFileCUE, parseModuleKey, resolveIdentifier, Resolver.loadTransitiveDeps,
  Resolver.resolveOne, Resolver.List.
- **invowk (2 blankets → 8 specific):** CLI boundary casts in loadConfigWithFallback,
  normalizeSourceName, runInvowkfilePathValidation, runModulePathValidation,
  runModuleAdd, runModuleArchive, runModuleCreate, runModuleImport.
- **TUI/serverbase:** Retained (all casts are framework-boundary — bubbletea/huh values).
- **discovery:** Already tightened (6 specific patterns from 3D work).
**Files:** `tools/goplint/exceptions.toml`.
