## Why

Invowk's CUE schemas should describe the authoritative user-facing shape of each document, with closed structs, concrete defaults, and constraints visible in CUE instead of relying on a parallel patch-only mental model. The current config schema validates a partial overlay while Go owns the effective defaults, and the invowkfile schema still carries explicit old-field rejection entries that we no longer want to preserve.

## What Changes

- **BREAKING**: Treat `internal/config/config_schema.cue` as the full effective config schema, including default values and constraints for the fields that `DefaultConfig()` materializes today.
- **BREAKING**: Derive `DefaultConfig()` from CUE evaluation so CUE owns the default contract instead of duplicating defaults in Go.
- **BREAKING**: Generate config files with the full explicit effective shape, including fields whose values equal CUE defaults.
- **BREAKING**: Remove legacy unsupported-field declarations such as `commands?: _|_` and old module metadata tombstones from `pkg/invowkfile/invowkfile_schema.cue`.
- Ensure every user-facing CUE struct remains closed; unknown fields are rejected by `close(...)` rather than by enumerating old field names.
- Keep `invowkfile.cue`, `invowkmod.cue`, and `config.cue` semantically aligned: schemas describe complete closed shapes, while Go validation remains responsible only for filesystem, cross-platform, runtime-environment, and other dynamic checks.
- Update parser/config loading behavior, sync tests, behavioral tests, docs, and samples so the new schema contract is explicit and verified.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `cue-schema-coherence`: Config schema semantics become full-shape/default-bearing instead of patch-only, closed-struct requirements become explicit, and legacy unsupported fields are removed from schemas.

## Impact

- Affected schemas: `internal/config/config_schema.cue`, `pkg/invowkfile/invowkfile_schema.cue`, and any schema comments or generated examples that describe config defaults or legacy invowkfile fields.
- Affected Go code: config decode/default application code, config patch DTOs if still needed internally, schema sync tests, behavioral schema tests, and parser tests.
- Affected docs/samples: configuration reference, invowkfile examples, migration snippets, and any docs that mention `commands`, inline module metadata in `invowkfile.cue`, or config defaults.
- No new external dependencies are expected.
