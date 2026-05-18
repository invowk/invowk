## 1. Module Identity Primitives

- [x] 1.1 Add a canonical module directory helper that converts a validated module ID to `<module-id>.invowkmod`
- [x] 1.2 Add tests for canonical directory names, invalid module IDs, RDNS module IDs, and names already containing `.invowkmod`
- [x] 1.3 Add typed errors for ambiguous source selection, missing root module files, invalid subpath module directory, subpath/module mismatch, and canonical module collisions
- [x] 1.4 Update module value-type tests to prove aliases never change canonical module identity

## 2. Source Module Selection

- [x] 2.1 Replace resolver use of broad `modulecache.LocateModuleInDir` with a source-selection helper for fetched Git repositories
- [x] 2.2 Implement root source selection requiring `invowkmod.cue` and `invowkfile.cue` at the repository root when `path` is omitted
- [x] 2.3 Implement omitted-path ambiguity detection that reports discovered child `.invowkmod` candidates and tells users to set `path`
- [x] 2.4 Implement explicit subpath selection requiring a relative, traversal-safe path whose basename ends in `.invowkmod`
- [x] 2.5 Enforce explicit subpath basename prefix equals parsed `invowkmod.cue: module`
- [x] 2.6 Add source-selection tests for ordinary root repo, `.invowkmod.git` repo, explicit subpath success, no-path child-only failure, non-suffix subpath failure, traversal failure, and basename/module mismatch

## 3. Resolver, Cache, and Lock Behavior

- [x] 3.1 Parse selected `invowkmod.cue` metadata before deriving namespace, command source ID, cache path, or vendor path
- [x] 3.2 Derive default dependency namespace from `<module-id>@<resolved-version>` and keep alias as the only namespace override
- [x] 3.3 Canonicalize module cache destinations so cached module directories end in `<module-id>.invowkmod`
- [x] 3.4 Preserve lock keys as Git URL plus normalized optional subpath while storing parsed `module_id`, namespace, command source ID, version, commit, and content hash
- [x] 3.5 Update lock-only dependency loading to locate or repopulate canonical cache paths from lock entries with `module_id`
- [x] 3.6 Add canonical module collision detection for distinct source identities resolving to the same module ID
- [x] 3.7 Add resolver tests for ordinary `tools.git` root cached as metadata module ID, default namespace from metadata, alias override, lock key preservation, lock-only canonical cache lookup, duplicate same-source dedupe, and different-source collision

## 4. Vendoring and Discovery Integration

- [x] 4.1 Update `moduleops.VendorModules` to copy resolved canonical cache directories directly into `invowk_modules/<module-id>.invowkmod`
- [x] 4.2 Ensure vendor pruning tracks canonical directory basenames and does not preserve stale repository-derived names
- [x] 4.3 Ensure vendored hash verification continues to match lock entries by module ID and source lock key
- [x] 4.4 Update discovery and vendored-module tests that currently assume vendored module names come from cache or repository basenames
- [x] 4.5 Add vendor tests for ordinary root repo canonical destination, explicit subpath canonical destination, canonical collision failure, prune of old noncanonical destination, and content-hash verification after canonical copy

## 5. CUE and Go Validation Coherence

- [x] 5.1 Update `pkg/invowkmod/invowkmod_schema.cue` so `requires[*].version` accepts `>=` and `<=`, rejects trailing junk, and matches the Go declared semver constraint validator
- [x] 5.2 Update `pkg/invowkmod/invowkmod_schema.cue` `git_url` comments and examples so Git repository names are not required to end in `.invowkmod`
- [x] 5.3 Update `ParseInvowkmodBytes` to run full metadata `Validate()` after CUE decode while preserving actionable requirement path errors
- [x] 5.4 Add invowkmod parser and behavioral sync tests for accepted version constraints, rejected malformed constraints, ordinary Git URLs, unsupported URL schemes, invalid aliases, invalid paths, and invalid metadata
- [x] 5.5 Change root-level `pkg/invowkfile/invowkfile_schema.cue` `workdir` to use `#NonWhitespaceString & strings.MaxRunes(4096)`
- [x] 5.6 Add invowkfile parse-level tests for whitespace-only root `workdir`, valid root `workdir`, and omitted root `workdir`
- [x] 5.7 Add or update `[GO-ONLY]` comments for runtime `image`/`containerfile` exclusivity, LLM provider/API exclusivity, and cross-platform path traversal validation
- [x] 5.8 Rename config schema `#DurationString` to `#LLMTimeoutDurationString` while preserving the 64-rune `LLMTimeout` limit
- [x] 5.9 Update config schema sync and behavioral tests so same helper names imply same limits and `llm.timeout` continues to match the Go `LLMTimeout` validator

## 6. CLI, Samples, and Documentation

- [x] 6.1 Add or update CLI testscript coverage for `invowk module sync` and `invowk module vendor` with ordinary root Git repositories and explicit subpath modules
- [x] 6.2 Add CLI/testscript coverage for omitted-path child-module failure, non-suffix subpath failure, subpath/module mismatch, and canonical collision errors
- [x] 6.3 Update sample `invowkmod.cue` files and fixtures so dependency examples no longer imply Git repositories must end in `.invowkmod`
- [x] 6.4 Update README and website module dependency docs to explain Git source identity, `path`, module identity, canonical local directory naming, aliases, collisions, and lock keys
- [x] 6.5 Update schema/reference docs for requirement version syntax, root `workdir`, Go-only validation boundaries, and config `llm.timeout` duration naming
- [x] 6.6 Regenerate or update docs snippets and generated references affected by module dependency or schema examples

## 7. Compatibility and Migration Tests

- [x] 7.1 Add tests proving existing lock entries with `module_id` can vendor through canonical cache paths without changing lock keys
- [x] 7.2 Add tests proving old repository-derived cache directories are not used to produce invalid vendored module names
- [x] 7.3 Add user-facing error tests that include the corrective action for implicit child modules: set `path` to the desired `.invowkmod` directory
- [x] 7.4 Add tests that root repositories with ordinary names are accepted only as source roots and are materialized locally under canonical `.invowkmod` directory names

## 8. Verification

- [x] 8.1 Run targeted Go tests for `pkg/invowkmod`, `pkg/invowkfile`, `internal/config`, `internal/app/modulesync`, `internal/app/moduleops`, and `internal/discovery`
- [x] 8.2 Run affected CLI testscript suites for module sync, module vendor, discovery, and schema-validation behavior
- [x] 8.3 Run schema sync checks for changed CUE schemas and Go structs
- [x] 8.4 Run documentation/snippet validation for changed README, website, samples, and generated references
- [x] 8.5 Run `openspec status --change harden-invowkmod-identity-and-schema-coherence` and any available strict validation for the change
- [x] 8.6 Run repository completion gates including `make check-baseline`, `make test`, and `git diff --check`; run `make check-agent-docs` only if agent docs, rules, or skills changed
- [x] 8.7 Refresh PGO artifacts only if changed resolver/discovery hot paths trigger the repository PGO gate
