## Context

The container runtime currently prepares an image, mounts the invowkfile directory at `/workspace`, executes the command with `RunOptions.Remove = true`, and cleans up per-run resources. That gives users a fresh disposable container but prevents workflows that intentionally keep package caches, build outputs, toolchains, or manual setup in a long-lived container.

The current container abstraction in `internal/container/` exposes `Build`, `Run`, `Remove`, image operations, and run-argument helpers through `Engine`. `BaseCLIEngine` already has `ExecArgs` and `Exec`, but `Engine` and `SandboxAwareEngine` do not expose exec as part of the common contract. Persistent targeting also needs container inspect, create, and start operations so `internal/runtime/` can make idempotent lifecycle decisions without shelling out ad hoc.

## Goals / Non-Goals

**Goals:**

- Preserve disposable container execution as the default behavior.
- Add an opt-in persistent target for container runtimes through a container-runtime-local invowkfile setting.
- Support idempotent managed container creation when `create_if_missing` is enabled.
- Support explicit one-shot targeting of a pre-existing container through an `ivk-` CLI flag.
- Generate deterministic, portable container names when the user does not configure one.
- Keep per-execution secrets and dynamic environment values out of persistent container creation.
- Provide clear errors for missing targets, invalid names, unmanaged conflicts, drift, and stopped external containers.

**Non-Goals:**

- No automatic deletion, mutation, recreation, or migration of existing persistent containers.
- No generalized container management CLI in this change.
- No change to the Linux-container-only policy or Debian-based reference image guidance.
- No support for Alpine or Windows container images.
- No guarantee that persistent containers are clean or reproducible between runs.

## Decisions

### Runtime-local configuration

Add a `persistent` block under the container runtime configuration:

```cue
runtimes: [{
  name: "container"
  image: "debian:stable-slim"

  persistent: {
    create_if_missing: true
    name: "invowk-my-project-build"
  }
}]
```

`persistent` enables persistent targeting for that runtime. `create_if_missing` defaults to `false`; when true, Invowk may create a managed persistent container if the effective name is missing. `name` is optional; when omitted, Invowk derives a deterministic name.

Alternative considered: a top-level persistent-container setting. Rejected because persistence is specific to the container runtime's image, mounts, ports, host mappings, and provisioning behavior.

### Target name precedence

Resolve the effective persistent container name in this order:

1. `--ivk-container-name`
2. `runtime.persistent.name`
3. derived name

The CLI flag enables persistent targeting for the invocation even when the invowkfile has no `persistent` block. A missing CLI-only target fails because the flag is primarily for pre-existing containers. If the invowkfile also enables `persistent.create_if_missing`, the same effective name may be used for managed creation.

Alternative considered: separate flags for "target existing" and "create with this name". Rejected for initial scope because one name override plus explicit `create_if_missing` keeps the user model small.

### Portable derived names

Derived names use this shape:

```text
invowk-<normalized-command-namespace>-<hash>
```

The readable namespace uses the command's full namespace, preferring `CommandInfo.ModuleID` when available and otherwise using the root invowkfile source identity, plus `CommandInfo.Name`. The hash includes the command namespace and source identity, including the invowkfile or module path, so commands with the same namespace in different roots do not collide silently.

Derived names always start with `invowk-`. All effective names use a strict portable subset accepted by Docker and Podman container-name flags: lowercase ASCII letters, digits, `.`, `_`, and `-`; they contain no spaces, slashes, or colons; and derived names are capped with room for the hash suffix. Explicit names are validated with the same portable grammar and rejected rather than rewritten, but they are not required to use the `invowk-` prefix because the CLI override can target user-owned pre-existing containers.

Alternative considered: use the exact command namespace as the container name. Rejected because namespaces can contain spaces or characters that are valid to Invowk but not portable container names.

### Managed and external targets

Invowk distinguishes managed persistent containers from external containers by labels. Managed containers get labels such as:

- `dev.invowk.managed=true`
- `dev.invowk.persistent=true`
- `dev.invowk.command.namespace=<canonical namespace>`
- `dev.invowk.command.source=<source identity hash>`
- `dev.invowk.container.spec=<fingerprint>`

The spec fingerprint covers creation-time state: engine-visible image, volumes, ports, extra hosts, default idle command, and relevant provisioning identity. It excludes per-run command, arguments, dynamic environment, stdin/stdout/stderr, temp script paths, and host callback tokens.

Behavior by target state:

| Target state | Config-derived or config-named target | CLI override target |
| --- | --- | --- |
| Missing | Create only when `persistent.create_if_missing` is true; otherwise fail | Create only when `persistent.create_if_missing` is true; otherwise fail |
| Exists with matching Invowk labels | Start if stopped, then exec | Start if stopped, then exec |
| Exists with Invowk labels but different fingerprint | Fail with drift guidance | Fail with drift guidance |
| Exists without Invowk labels | Fail to avoid hijacking a user's container | Require running, then exec as an external target |
| Exists external but stopped | Fail | Fail |

Alternative considered: always start external containers. Rejected because starting a user-managed container can have side effects outside Invowk's contract.

### Engine lifecycle contract

Extend `container.Engine` with the lifecycle operations persistent targeting needs:

- `InspectContainer(ctx, nameOrID)` returning ID, name, running/stopped state, labels, and enough metadata for drift checks.
- `Create(ctx, CreateOptions)` for a named, labeled, non-removed persistent container.
- `Start(ctx, idOrName)` for stopped managed containers.
- `Exec(ctx, idOrName, command, RunOptions)` promoted from `BaseCLIEngine` to the common interface.

`SandboxAwareEngine` must wrap all new operations through the same host-spawn path used by build/run/remove/image operations. Tests should cover Docker, Podman, and sandbox argument construction without depending on a live engine for unit coverage.

Alternative considered: keep persistent lifecycle logic in `internal/runtime/` by calling `docker` or `podman` directly. Rejected because it would bypass the engine abstraction, sandbox wrapper, SELinux formatting, Podman remote behavior, and existing test seams.

### Split creation-time and exec-time data

Persistent execution reuses the existing preparation pipeline but separates data into two phases:

- Creation-time: image or provisioned image, container name, labels, volumes, ports, extra hosts, and idle command.
- Exec-time: command script, positional args, workdir, env, stdin/stdout/stderr, interactive/TTY flags, and host SSH/TUI callback environment.

Invowk must pass dynamic values through `exec -e` and must revoke host callback tokens after each execution. Persistent container creation must not bake `INVOWK_SSH_*`, `INVOWK_FLAG_*`, user env-file values, or other per-run values into the container.

Alternative considered: create a new container for each persistent exec and commit state afterward. Rejected because it is slower, engine-specific, and does not actually target a user-visible long-lived container.

### Idempotency and races

Managed creation uses inspect-before-create, create, start, and then exec. To handle concurrent invocations, Invowk should acquire a per-engine/per-container-name lock around inspect/create/start where supported, reusing the existing runtime-lock style where practical. If the create step still races and receives a name-conflict error, Invowk re-inspects the target and proceeds only if labels and fingerprint match.

Existing managed containers are idempotent targets. Invowk starts stopped managed containers and executes the command. It never removes or recreates them as part of command execution.

Alternative considered: let the engine name conflict be the user's error. Rejected because two simultaneous first runs should be safe and predictable.

### Provisioning and force rebuild

When a managed persistent container must be created, provisioning runs before creation and the resulting image identity becomes part of the managed container fingerprint. The persistent path must not clean up a provisioned image while a managed persistent container depends on it.

For an existing managed persistent container, `--ivk-force-rebuild` must not silently rebuild or replace the container. It may affect only the image/provisioning step for a missing target that is about to be created. If the desired creation-time spec changes, Invowk reports drift and tells the user to choose a new name or remove/recreate the container intentionally.

### CLI, dry-run, and service propagation

Add `--ivk-container-name` to the `invowk cmd` command tree and discovered leaves. Propagate it through the CLI adapter request, command service request, execution context builder, and runtime execution context as a typed optional container name.

Dry-run output should include the effective persistent target, whether the name came from CLI/config/derivation, whether creation would be allowed, and whether the command would use disposable or persistent container execution.

### Tests and documentation

Implementation should include:

- Unit tests for schema parsing, name validation/derivation, request propagation, engine argument construction, lifecycle state decisions, fingerprint drift, and secret separation.
- Testscript coverage for CLI flag plumbing, dry-run output, invowkfile schema validation, and user-facing errors.
- Live Docker/Podman integration tests gated behind the existing container-test infrastructure for create-if-missing, state preservation across runs, external override targeting, stopped managed restart, stopped external failure, and cleanup by Invowk labels.
- Documentation for the invowkfile setting, CLI flag, lifecycle behavior, external-target expectations, drift handling, and the stateful nature of persistent containers.

## Risks / Trade-offs

- Persistent containers retain state and can hide reproducibility bugs. Mitigation: keep disposable execution as default and document persistent mode as stateful.
- Pre-existing containers may not have the expected `/workspace` mount. Mitigation: external targets are explicit via CLI, and docs must state the mount/workdir expectations.
- Volumes, ports, image, and host mappings cannot be safely changed after creation. Mitigation: fingerprint creation-time state and fail on drift instead of mutating containers.
- Host callback secrets could outlive a run if placed in creation-time env. Mitigation: pass dynamic values only through exec-time env and revoke tokens in the existing cleanup path.
- Container engines differ in inspect/create output. Mitigation: use minimal JSON fields and add Docker/Podman-specific fixture tests before live integration tests.
- Concurrent first runs can race on the same derived name. Mitigation: lock around lifecycle operations and re-inspect after name conflicts.

## Migration Plan

1. Add schema and Go types while keeping default disposable execution unchanged.
2. Add CLI flag and request propagation with no behavior change unless persistent targeting is requested.
3. Extend the engine interface and sandbox wrapper with inspect/create/start/exec.
4. Add persistent lifecycle resolution in the container runtime.
5. Add tests from unit level outward to live engine integration.
6. Update documentation and examples.

Rollback is straightforward before release because the feature is additive. If a persistent container is created during testing, cleanup is manual or test-owned by labels; Invowk must not remove user persistent containers automatically.

## Open Questions

- Should Invowk eventually provide an explicit cleanup command for managed persistent containers, or keep cleanup outside this feature?
- Should managed persistent containers support an explicit "recreate on drift" flag in a later change, or is manual removal enough?
- Should external CLI targets require a documented label opt-in in the future, or is explicit `--ivk-container-name` sufficient consent?
