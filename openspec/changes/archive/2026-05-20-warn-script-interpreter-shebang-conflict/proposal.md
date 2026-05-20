## Why

Invowk already defines that an explicit `script.interpreter` takes precedence over a shebang in the resolved script bytes, but that precedence can surprise users when `script.file` points to an executable with a different shebang. The UX should make intentional overrides visible while still preserving explicit configuration as the source of truth.

## What Changes

- Add advisory diagnostics when an explicit `script.interpreter` overrides a different shebang discovered in resolved script content.
- Show interpreter provenance in dry-run output so users can see whether execution uses an explicit interpreter, an auto-detected shebang, or default shell behavior.
- Keep the execution contract unchanged: explicit `script.interpreter` continues to override shebang detection instead of becoming a validation error.
- Add focused tests for inline scripts, module-contained `script.file`, custom checks, native/container behavior, and virtual-runtime shell compatibility.
- Update user-facing docs to explain the precedence and the warning clearly.

## Capabilities

### New Capabilities
- `script-interpreter-diagnostics`: Advisory diagnostics and dry-run visibility for interpreter precedence, including explicit interpreter overrides of script shebangs.

### Modified Capabilities
- None.

## Impact

- Affected schema/contracts: new OpenSpec capability covering interpreter diagnostics; no breaking schema change.
- Affected code: script interpreter resolution helpers, validation or diagnostic collection paths, dry-run rendering, custom-check dependency resolution, and runtime planning/tests.
- Affected docs/tests: invowkfile interpreter docs, dry-run expectations, runtime/custom-check tests, and module `script.file` fixtures.
