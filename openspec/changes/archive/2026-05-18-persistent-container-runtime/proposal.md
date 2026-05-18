## Why

Invowk's container runtime currently behaves like a disposable execution target, which is safe but awkward for workflows that intentionally keep build caches, toolchains, package stores, or interactive setup inside a long-lived container. Persistent container targeting lets users keep that state while preserving Invowk's default ephemeral behavior and making repeat runs idempotent.

## What Changes

- Add a container-runtime-local `persistent` invowkfile setting that enables targeting a long-lived container instead of an auto-removed run container.
- Allow `persistent.create_if_missing` to create a managed persistent container only when the configured or derived container name does not already exist.
- Allow an optional `persistent.name` field for an explicit managed container name.
- Derive a deterministic OCI-compatible name when no explicit name is configured, based on `invowk` plus the command's full namespace and a collision-resistant source identity suffix.
- Add a new `ivk-` CLI flag that overrides the persistent container target name for a single command invocation, including targeting a pre-existing container.
- Preserve the current disposable container runtime as the default when persistent mode and the CLI override are not used.
- Fail clearly on name conflicts, stopped external containers, incompatible managed container drift, or invalid explicit names instead of mutating or deleting a user's container.

## Capabilities

### New Capabilities

- `persistent-container-runtime`: Defines persistent container targeting for `runtime = container`, including invowkfile configuration, derived names, CLI override behavior, idempotent creation, execution targeting, drift handling, and user-facing documentation.

### Modified Capabilities

- None.

## Impact

- Affects invowkfile schema and parsing in `pkg/invowkfile/` and related schema sync tests.
- Affects command flag plumbing in `cmd/invowk/` and execution request propagation through `internal/app/commandsvc/` and `internal/app/execute/`.
- Extends `internal/container/` engine capabilities for inspect, create, start, and exec against existing containers.
- Changes `internal/runtime/` container execution flow when persistent targeting is enabled or overridden.
- Adds unit, CLI integration, and container-engine integration coverage for managed and external persistent targets.
- Updates website and reference documentation so users understand persistent containers are stateful and opt in.
