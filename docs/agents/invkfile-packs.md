# Invkfile Examples and Packs

## Built-in Examples (invkfile.cue at project root)

- Always update the example file when there are any invkfile definition changes or features added/modified/removed.
- All commands should be idempotent and not cause any side effects on the host.
- No commands should be related to building invowk itself or manipulating any of its source code.
- Examples should range from simple (e.g.: native "hello-world") to complex (e.g.: container "hello-world" with the enable_host_ssh feature).
- Examples should illustrate the use of different features of Invowk, such as:
  - Native vs. Container execution.
  - Volume mounts for Container execution.
  - Environment variables.
  - Host SSH access enabled vs. disabled.
  - Capabilities checks (with and without alternatives).
  - Tools checks (with and without alternatives).
  - Custom checks (with and without alternatives).

## Sample Packs (packs/ directory)

The `packs/` directory contains sample invowk packs that serve as reference implementations and validation tests for the pack feature.

### Maintenance Requirements

- **Always update sample packs** when the design and/or implementation of invowk packs changes.
- All packs in this directory must remain valid and pass `invowk pack validate --deep`.
- Packs should demonstrate pack-specific features (script file references, cross-platform paths, etc.).
- Run validation after any pack-related changes: `go run . pack validate packs/<pack-name> --deep`.

### Current Sample Packs

- `io.invowk.sample.invkpack` - Minimal cross-platform pack with a simple greeting command.

### Pack Validation Checklist

When modifying pack-related code, verify:
1. All packs in `packs/` pass validation: `go run . pack validate packs/*.invkpack --deep`.
2. Pack naming conventions are correctly enforced.
3. Script path resolution works correctly (forward slashes, relative paths).
4. Nested pack detection works correctly.
5. The `pkg/pack/` tests pass: `go test -v ./pkg/pack/...`.

## Common Pitfall

- **Stale sample packs** - Update packs in `packs/` after pack-related changes.
