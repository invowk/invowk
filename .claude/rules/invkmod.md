# Invkmod Modules

## Samples (`modules/` directory)

The `modules/` directory contains sample modules that serve as reference implementations and validation tests.

- A module is a `.invkmod` directory containing `invkmod.cue` (metadata) and `invkfile.cue` (commands).
- The invkmod schema lives in `pkg/invkmod/invkmod_schema.cue` and the invkfile schema in `pkg/invkfile/invkfile_schema.cue`.
- Always update sample modules when the invkmod schema, validation rules, or module behavior changes.
- Modules should demonstrate module-specific features (script file references, cross-platform paths, requirements).
- After module-related changes, run validation: `go run . module validate modules/<module-name>.invkmod --deep`.

### Current Sample Modules

- `io.invowk.sample.invkmod` - Minimal cross-platform module with a simple greeting command.

### Module Validation Checklist

When modifying module-related code, verify:
1. All modules in `modules/` pass validation: `go run . module validate modules/*.invkmod --deep`.
2. Module naming conventions and module ID matching are enforced.
3. `invkmod.cue` is required and parsed; `invkfile.cue` contains only commands.
4. Script path resolution works correctly (forward slashes, relative paths).
5. Nested module detection works correctly.
6. The `pkg/invkmod/` tests pass: `go test -v ./pkg/invkmod/...`.

## Common Pitfall

- **Stale sample modules** - Update modules in `modules/` after module-related changes.
