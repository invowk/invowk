# Invkfile Examples

## Built-in Examples (`invkfile.cue` at project root)

- Always update the example file when invkfile definitions or features are added, modified, or removed.
- All commands should be idempotent and not cause any side effects on the host.
- No commands should be related to building invowk itself or manipulating any of its source code.
- Examples should range from simple (e.g., native "hello-world") to complex (e.g., container "hello-world" with `enable_host_ssh`).
- Examples should cover different features, such as:
  - Native vs. container execution.
  - Volume mounts for container execution.
  - Environment variables.
  - Host SSH access enabled vs. disabled.
  - Capabilities checks (with and without alternatives).
  - Tools checks (with and without alternatives).
  - Custom checks (with and without alternatives).
- Pack metadata does not belong in invkfile examples; it lives in `invkpack.cue` for packs.
