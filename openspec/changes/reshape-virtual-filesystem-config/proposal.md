## Why

The pre-clean-break virtual filesystem draft put legacy `allowed_paths` at implementation scope even though the setting is only meaningful for `virtual-*` runtimes and its values are platform-specific. This change removes that draft shape and replaces it with an explicit virtual-family filesystem namespace under each platform, making the schema contract match how users think about OS paths and virtual runtime safety.

## What Changes

- **BREAKING** Remove the legacy implementation-level `allowed_paths` field from CUE, Go types, generation, validation, runtime wiring, docs, snippets, samples, tests, and OpenSpec active-contract text.
- **BREAKING** Add platform-level virtual filesystem config at `implementations[].platforms[].virtual.filesystem`.
- Add `platforms[].virtual.filesystem.access` with allowed values `"restricted"` and `"full"`, defaulting to `"restricted"`.
- Add `platforms[].virtual.filesystem.paths` as a map of logical uppercase path names to resolved path roots.
- In `"restricted"` mode, virtual file I/O is limited to implicit safe roots plus the named `paths` roots.
- In `"full"` mode, virtual file I/O may access the host filesystem through the virtual path validator; named `paths` remain bridge handles and do not define the full access boundary.
- Keep virtual host binary policy in selected runtime config: `runtimes[].allowed_binaries` and `runtimes[].binary_lookup_mode` remain runtime-local, because they control virtual runtime host-binary execution rather than platform path mapping.
- Keep `virtual` valid only as a namespace, never as a runtime selector.
- Update dry-run output, audit findings, deterministic/LLM audit prompts, README, website current docs, Portuguese i18n current docs, snippets, samples, testscript fixtures, generated CUE, and OpenSpec artifacts so the legacy field appears only in explicit rejection/absence checks.
- Add comprehensive schema, parser, generation, runtime, dry-run, audit, CLI, docs, and stale-reference tests for the final shape.
- Do not add compatibility aliases, dual-read decoding, deprecation warnings, tombstone fields, migration shims, or ignored old fields.

## Capabilities

### New Capabilities
- `virtual-filesystem-access`: Defines platform-scoped virtual filesystem access modes, named path mappings, bridge exposure, validation, runtime behavior, docs, tests, and clean-break removal of the old implementation-level shape.

### Modified Capabilities
- `cue-schema-coherence`: The invowkfile schema, Go structs, generators, validators, docs, and tests must remain coherent while virtual filesystem configuration moves from the legacy implementation-level field to `platforms[].virtual.filesystem`.

## Impact

- CUE schema: `pkg/invowkfile/invowkfile_schema.cue`.
- Go model and validation: `pkg/invowkfile` platform config, implementation config, path mapping value types, parser/decoder, generation, sync tests, and behavioral validation.
- Runtime execution: `internal/runtime` virtual path resolver, path validator, shell/Lua bridge injection, u-root path checks, and runtime environment projection.
- Command service and CLI: dry-run planning/rendering, validation diagnostics, generated examples, `init` output, and CLI testscript fixtures.
- Audit and security docs: `internal/audit`, deterministic Lua/virtual findings, LLM prompts, module-security guidance, and security documentation.
- Documentation and examples: README, website current docs, Portuguese current i18n docs, snippets, samples, schema reference, runtime-mode pages, and authoring guidance.
- Verification: OpenSpec validation, schema sync tests, targeted Go tests, CLI testscript coverage, docs checks, agent-doc checks if touched, repo guardrails, and stale-shape searches for legacy field names.
