# Invkpack Packs

## Samples (`packs/` directory)

The `packs/` directory contains sample packs that serve as reference implementations and validation tests.

- A pack is a `.invkpack` directory containing `invkpack.cue` (metadata) and `invkfile.cue` (commands).
- The invkpack schema lives in `pkg/invkfile/invkpack_schema.cue` and the invkfile schema in `pkg/invkfile/invkfile_schema.cue`.
- Always update sample packs when the invkpack schema, validation rules, or pack behavior changes.
- Packs should demonstrate pack-specific features (script file references, cross-platform paths, requirements).
- After pack-related changes, run validation: `go run . pack validate packs/<pack-name>.invkpack --deep`.

### Current Sample Packs

- `io.invowk.sample.invkpack` - Minimal cross-platform pack with a simple greeting command.

### Pack Validation Checklist

When modifying pack-related code, verify:
1. All packs in `packs/` pass validation: `go run . pack validate packs/*.invkpack --deep`.
2. Pack naming conventions and pack ID matching are enforced.
3. `invkpack.cue` is required and parsed; `invkfile.cue` contains only commands.
4. Script path resolution works correctly (forward slashes, relative paths).
5. Nested pack detection works correctly.
6. The `pkg/pack/` tests pass: `go test -v ./pkg/pack/...`.

## Common Pitfall

- **Stale sample packs** - Update packs in `packs/` after pack-related changes.
