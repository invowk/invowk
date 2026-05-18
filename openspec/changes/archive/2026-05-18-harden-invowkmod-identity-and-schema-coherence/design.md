## Context

Invowk modules are user-facing filesystem directories named `<module-id>.invowkmod`, while remote dependencies are fetched from Git repositories that may have ordinary names such as `tools.git`. The current resolver path mixes those identities: the requirement `git_url` comment says the repository must end in `.invowkmod`, cache paths are derived from the repository URL and optional subpath, module discovery can pick the first child `.invowkmod` directory when `path` is omitted, and default namespaces can be derived from the repository or path basename instead of the parsed module metadata.

The CUE schemas are mostly well structured, but a few fields do not line up with the Go validators. `#ModuleRequirement.version` documents `>=1.0.0` but uses a permissive prefix regex that rejects some documented operators and accepts trailing junk. `ParseInvowkmodBytes` decodes CUE and validates requirement paths, but it does not call the full metadata `Validate()` method. Root-level invowkfile `workdir` accepts whitespace-only strings even though command and implementation workdirs do not. Config and invowkfile both define a `#DurationString` helper name with different maximum lengths.

## Goals / Non-Goals

**Goals:**

- Keep the local installed module invariant: materialized Invowk modules are directories named `<module-id>.invowkmod`.
- Allow Git repository roots with ordinary names to be module source roots when they contain `invowkmod.cue` and `invowkfile.cue`.
- Allow explicit Git subpaths to select modules, while requiring subpath module directories to be real `.invowkmod` directories.
- Make `invowkmod.cue: module` the canonical module identity for default namespaces, cache/vendor directory names, and collision detection.
- Preserve lock-file source identity through `git_url` plus normalized optional `path`.
- Align CUE schemas, Go validators, schema sync tests, samples, and docs for module requirements and workdir validation.
- Make duration schema helper names honest: definitions with the same name have the same limit, otherwise the name is specific to the field.

**Non-Goals:**

- Do not require Git repositories to be renamed to `.invowkmod.git`.
- Do not add automatic transitive dependency resolution; the explicit-only dependency model remains unchanged.
- Do not change the public lock file key shape unless required for unambiguous source identity.
- Do not introduce a general module registry, module proxy, or non-Git source type.
- Do not change duration parsing semantics beyond the naming and schema/Go parity fixes described here.

## Decisions

### Separate source identity from module identity

Treat `git_url` and `path` as source identity only. The source identity answers "where did this dependency come from?" and remains the basis for lock keys and duplicate requirement detection. Treat the parsed `invowkmod.cue: module` value as module identity. The module identity answers "what module is this?" and becomes the default namespace and local materialized directory name.

Alternative considered: continue deriving default module names from the repository or subpath basename. Rejected because `tools.git` can legally contain `module: "com.acme.devtools"`, and the repository name is not a stable module identity.

### Add a source-selection helper instead of reusing installed-module discovery

Replace resolver use of broad `LocateModuleInDir` scanning with a stricter source-selection helper:

- No `path`: the repository root is selected only if it contains `invowkmod.cue` and `invowkfile.cue`.
- No `path` and only child `.invowkmod` directories: fail with an actionable error telling the user to set `path`.
- With `path`: the normalized path must be relative, must not traverse, its basename must end in `.invowkmod`, and it must contain `invowkmod.cue` and `invowkfile.cue`.
- With `path`: the directory prefix before `.invowkmod` must match the parsed `module` value.

This keeps root repositories flexible without weakening the rule that actual module directories end in `.invowkmod`.

Alternative considered: keep "first child `.invowkmod` wins" for convenience. Rejected because it is ambiguous in monorepos and silently depends on filesystem ordering.

### Canonicalize cache and vendor directories

After selecting the source module and parsing metadata, compute the canonical directory name as `<module-id>.invowkmod`. Cache the module under a source/version scoped parent that ends in that canonical directory name, and vendor by copying the canonical cached module directory to `invowk_modules/<module-id>.invowkmod`.

The lock file should continue to key modules by `ModuleRef.Key()` (`git_url` plus normalized `path`) because that is the dependency declaration identity. Lock entries already carry `module_id`, `namespace`, `command_source_id`, and `content_hash`; those fields should reflect the parsed metadata and canonical materialization.

Alternative considered: cache and vendor under the repository basename. Rejected because it reintroduces invalid local module names for ordinary repositories and makes two repositories with the same module ID look like different installed modules.

### Detect module identity collisions explicitly

During sync/vendor resolution, detect when two distinct source identities resolve to the same `module` ID. If the source identity or content hash differs, fail with a collision error that names both requirements and the canonical directory. If the duplicate is the same source identity, keep existing deduplication behavior.

Alias values can disambiguate command namespaces, but aliases do not change module identity and must not allow two different modules to overwrite the same canonical local directory.

Alternative considered: let aliases also affect vendor directory names. Rejected because it would make the installed filesystem identity depend on an importing module's local preference.

### Parse metadata before deriving namespaces

Resolve the Git version, fetch the source, select the source module, parse `invowkmod.cue`, run full `Invowkmod.Validate()`, then derive namespace and command source identity. The default namespace is `<module-id>@<resolved-version>`. An explicit alias remains the namespace override.

For compatibility, lock-only loading should be able to reconstruct the canonical cache path from lock entries that include `module_id`. If an older lock entry lacks a required canonical field, report an upgrade/resync action instead of guessing from the repository basename.

Alternative considered: keep namespace derivation before metadata parsing. Rejected because it makes metadata advisory rather than authoritative.

### Tighten schema and Go validation together

Update `pkg/invowkmod/invowkmod_schema.cue` so requirement `version` uses the same accepted syntax as `ValidateDeclaredSemVerConstraint()`, including `>=` and `<=`, and rejects trailing junk. After CUE decode, call full `meta.Validate()` in `ParseInvowkmodBytes` so Git URL, version, alias, path, module identity, and version metadata all pass the Go value-type validators.

Keep Go-only path validation comments because cross-platform traversal and normalization belong in Go. Add `[GO-ONLY]` comments where runtime/config invariants intentionally remain outside CUE.

Alternative considered: rely only on CUE for requirement validation. Rejected because path normalization, typed value-object consistency, and reusable Go validators already exist and should be authoritative after decode.

### Normalize root workdir validation

Change root-level `workdir` in `pkg/invowkfile/invowkfile_schema.cue` to use `#NonWhitespaceString & strings.MaxRunes(4096)`, matching command and implementation workdir intent. Add parse-level tests because the current parse path does not run every structural validator.

Alternative considered: keep whitespace-only workdir and let runtime path resolution fail later. Rejected because the schema can catch this cheaply and consistently.

### Rename config-only duration schema definition

Rename `internal/config/config_schema.cue` `#DurationString` to a field-specific name such as `#LLMTimeoutDurationString`, keeping the existing 64-rune limit and aligning behavioral sync tests with `LLMTimeout`. The invowkfile schema keeps `#DurationString` with its existing 32-rune limit for implementation timeouts and watch debounce values.

Rule: schema definitions with the same helper name must have the same maximum length. If limits differ, names must differ.

Alternative considered: reduce config `llm.timeout` to 32 runes to match invowkfile. Rejected because the existing config Go type and tests already define a 64-rune `LLMTimeout`; renaming preserves behavior while removing the misleading shared name.

### Update docs, samples, and tests as part of the contract

Update docs and examples so they no longer teach `.invowkmod.git` as a Git URL requirement. Show both ordinary root repositories and subpath modules. Update schema sync tests, module sync unit tests, vendor tests, lock tests, CLI testscript coverage, docs snippets, and any generated command/reference output touched by the changed user-facing behavior.

Alternative considered: treat docs as a follow-up. Rejected because the current confusion is partly documentation-driven.

## Risks / Trade-offs

- Existing repositories that relied on implicit first-child module selection will fail until they add `path`. Mitigation: emit a specific error that lists discovered child `.invowkmod` candidates and tells the user which `path` value to declare.
- Existing cache entries may be left under old repository-derived directories. Mitigation: cache under canonical paths going forward; when a lock entry has enough metadata, recache or verify canonical content without mutating lock identity.
- A repository root with ordinary name is not itself named `.invowkmod` while being used as source input. Mitigation: document the distinction between source checkout root and materialized local module directory, and enforce `.invowkmod` only on installed/canonicalized directories and explicit subpaths.
- Collision detection can reject configurations that previously overwrote or shadowed silently. Mitigation: provide an error that identifies both source requirements, resolved module ID, and the canonical directory.
- Tightened schema validation can reject previously accepted malformed values. Mitigation: cover the new errors in tests and docs; the rejected values were already contrary to the intended Go validators.

## Migration Plan

1. Add canonical module directory/name helpers and stricter source selection without changing lock-file write behavior.
2. Parse module metadata before namespace/cache/vendor derivation and update resolver tests for root and subpath source cases.
3. Canonicalize cache and vendor materialization to `<module-id>.invowkmod`, then add collision checks.
4. Tighten CUE schemas and parse-level Go validation, then update schema sync and behavioral tests.
5. Update docs, samples, snippets, and CLI/testscript coverage.
6. Run targeted Go tests for `pkg/invowkmod`, `pkg/invowkfile`, `internal/config`, `internal/app/modulesync`, `internal/app/moduleops`, `internal/discovery`, and affected CLI tests.
7. Run schema sync checks, docs checks, OpenSpec validation, `make check-baseline`, `make test`, and `git diff --check`; refresh PGO only if the repository gates require it.

Rollback before release is straightforward: restore repository/path-derived cache/vendor behavior and the previous schema comments. After release, rollback would require documenting that canonicalized cache/vendor directories may already exist and can be safely regenerated from lock files.

## Open Questions

- Should the first release emit a deprecation-style diagnostic for implicit child modules before making that case a hard error, or is a hard error acceptable because the behavior is ambiguous today?
- Should canonical source cache parent directories include a hash of the full source identity to avoid long or scheme-normalized paths, while keeping lock keys human-readable?
- Should the CLI expose a repair/resync command for old cache layouts, or is normal `module sync` enough to repopulate canonical cache entries?
