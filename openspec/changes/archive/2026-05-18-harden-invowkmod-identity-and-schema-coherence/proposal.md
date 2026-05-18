## Why

Invowk currently blurs Git source identity, module identity, local module directory naming, and schema validation responsibility in a few places. This makes ordinary repositories such as `tools.git` confusing, leaves some invalid `invowkmod.cue` values accepted during parse, and creates small but real drift between CUE schemas, Go validators, tests, and docs.

## What Changes

- Define Git URLs as source locations only; they SHALL NOT be required to end in `.invowkmod` or imply the module identity.
- Define `invowkmod.cue: module` as the canonical module identity used for default namespaces and canonical local cache/vendor directory names.
- Preserve the Invowk module directory invariant by materializing installed modules as `<module-id>.invowkmod`, even when the Git repository root has an ordinary name such as `tools`.
- Support both ordinary Git repository roots that contain `invowkmod.cue` and explicit subpath modules whose selected directory name ends in `.invowkmod`.
- Stop implicitly selecting an arbitrary child `.invowkmod` directory when a Git requirement omits `path`; users SHALL set `path` for monorepo or subdirectory modules.
- Detect canonical module name collisions when different source requirements resolve to the same `<module-id>.invowkmod` with different source identity or content.
- Tighten `invowkmod` requirement validation so CUE and Go agree on supported version constraint syntax, required metadata validation, and Git URL responsibilities.
- Tighten root-level `workdir` validation so it rejects whitespace-only values consistently with command and implementation workdirs.
- Document CUE versus Go-only validation boundaries for runtime mutual exclusivity and other dynamic checks.
- Rename the config-only duration schema definition to make its 64-rune `llm.timeout` limit explicit; duration schema definitions with the same name MUST have the same limit.
- Update all relevant unit tests, schema sync tests, CLI/testscript coverage, samples, and documentation to reflect the new contracts.

## Capabilities

### New Capabilities

- `invowkmod-source-identity`: Defines how module requirements resolve Git sources, source subpaths, canonical module identity, cache/vendor materialization, default namespaces, collision detection, and user-facing docs.
- `cue-schema-coherence`: Defines schema and Go-validation coherence for `invowkmod.cue`, `invowkfile.cue`, and config CUE schemas, including version constraints, root workdir validation, Go-only validation comments, and duration definition naming.

### Modified Capabilities

- None.

## Impact

- Affects module metadata parsing and validation in `pkg/invowkmod/`.
- Affects module sync, source fetching, cache path construction, vendoring, lock identity, and collision handling in `internal/app/modulesync/` and related module-cache code.
- Affects default command namespace derivation and module discovery behavior where canonical module identity is surfaced.
- Affects CUE schemas and schema sync tests in `pkg/invowkmod/`, `pkg/invowkfile/`, and `internal/config/`.
- Affects CLI integration tests for module sync/vendor behavior and user-facing validation errors.
- Affects samples, website/reference documentation, and any docs that describe Git requirements, module directory naming, namespaces, workdir validation, runtime mutual exclusivity, or duration strings.
