# CUE and Schemas

## Schema Locations

- `pkg/invkfile/invkfile_schema.cue` defines invkfile structure.
- `pkg/invkmod/invkmod_schema.cue` defines invkmod structure.
- `internal/config/config_schema.cue` defines config.

## Rules

- All CUE structs must be closed (use `close({ ... })`) so unknown fields cause validation errors.
- When adding new CUE struct fields or definitions, always include appropriate validation constraints (e.g., `strings.MaxRunes()`, regex patterns with `=~`, range constraints like `>=0 & <=255`) - not just type declarations. This ensures defense-in-depth validation.

## Common Pitfall

- **Unclosed CUE structs** - Always use `close({ ... })` for CUE definitions.
