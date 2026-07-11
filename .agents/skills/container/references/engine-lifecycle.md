# Container Engine And Lifecycle Reference

Use this reference for engine contracts, Docker/Podman differences, sandbox
forwarding, persistent containers, and Podman lifecycle coordination.

## Contents

- [Engine Shape](#engine-shape)
- [Base CLI Engines](#base-cli-engines)
- [Persistent Target Planning](#persistent-target-planning)
- [Podman Sysctl Mitigation](#podman-sysctl-mitigation)
- [Serialization Fallback](#serialization-fallback)
- [Sandbox Wrapper](#sandbox-wrapper)
- [Key Files](#key-files)

## Engine Shape

`internal/container/engine.go` defines portable runtime operations:

```go
type Engine interface {
    Build(context.Context, BuildOptions) error
    Run(context.Context, RunOptions) (*RunResult, error)
    InspectContainer(context.Context, ContainerName) (*ContainerInfo, error)
    Create(context.Context, CreateOptions) (*CreateResult, error)
    Start(context.Context, ContainerID) error
    Exec(context.Context, ContainerID, []string, RunOptions) (*RunResult, error)
    Remove(context.Context, ContainerID, bool) error
    ImageExists(context.Context, ImageTag) (bool, error)
    RemoveImage(context.Context, ImageTag, bool) error
    Name() string
    Version(context.Context) (string, error)
    Available() bool
}
```

Interactive execution is intentionally separate:

```go
type CommandPreparer interface {
    BinaryPath() string
    BuildRunArgs(RunOptions) []string
    PrepareRunCommand(context.Context, RunOptions) (*exec.Cmd, func(), error)
}
```

Keep vendor-specific inspection and command helpers on `BaseCLIEngine` or the
concrete engine rather than widening `Engine`.

## Base CLI Engines

Docker and Podman embed `BaseCLIEngine`. Its injected seams include the command
executor, volume formatter, environment overrides, and run-argument
transformer. Use those options in tests; do not restore package-global command
mutation.

Docker mostly delegates to the base. Podman additionally owns:

- executable discovery (`podman`, then `podman-remote`);
- SELinux `:z` volume labeling when `/sys/fs/selinux` exists;
- rootless `--userns=keep-id` insertion for run/create commands;
- local-Linux sysctl override configuration;
- lifecycle serialization when that override cannot protect execution.

Executable discovery uses `exec.LookPath`; shell aliases and functions are not
visible to Invowk.

## Persistent Target Planning

`internal/containerplan.ResolvePersistentTarget` deterministically chooses the
CLI override, configured name, or derived command namespace and applies
`create_if_missing`. It must remain pure.

Runtime code consumes that plan and performs inspect/create/start/exec/remove.
When the engine implements `LifecycleCoordinator`, acquire the coordinator for
the whole state-changing operation. This keeps persistent paths aligned with
transient Podman serialization.

## Podman Sysctl Mitigation

Rootless local Podman can race while `crun` writes
`net.ipv4.ping_group_range`. On Linux with a local Podman executable,
`sysctlOverrideOpts()` creates a temporary `containers.conf` containing:

```toml
[containers]
default_sysctls = []
```

It supplies the path through `CONTAINERS_CONF_OVERRIDE`; every subprocess opens
the same path independently. `BaseCLIEngine.Close()` removes the temporary file.

Do not enable this mechanism for:

- non-Linux hosts, where Podman runs inside a VM; or
- `podman-remote`, where a client-side environment override cannot configure the
  remote service that invokes `crun`.

`isRemotePodman()` checks executable names and resolved symlinks.

## Serialization Fallback

When the sysctl override is inactive, Podman run and persistent lifecycle paths
serialize:

- Linux uses blocking `flock(2)` on
  `$XDG_RUNTIME_DIR/invowk-podman.lock`, falling back to `os.TempDir()`.
- Non-Linux falls back to an in-process mutex because host `flock` cannot
  coordinate the Podman VM.

`SysctlOverrideChecker` reports whether serialization can be skipped.
`LifecycleCoordinator` exposes an operation-scoped lease. Both are forwarded by
`SandboxAwareEngine` when the wrapped engine implements them.

Prepared commands must return a cleanup function that holds the lease until the
interactive child has exited.

## Sandbox Wrapper

`SandboxAwareEngine` decorates Docker/Podman in Flatpak or Snap environments so
engine commands execute on the host. Paths presented to the host engine differ
from sandbox paths, so every newly introduced engine operation must be forwarded
and customized consistently.

`CmdCustomizer` applies command environment overrides to sandbox-created
`exec.Cmd` values that bypass `BaseCLIEngine.CreateCommand`. Keep build, run,
remove, image, and interactive preparation paths aligned.

## Key Files

| File | Responsibility |
| --- | --- |
| `internal/container/engine.go` | Engine and coordinator contracts, options, factories |
| `internal/container/engine_base.go` | Shared CLI behavior and command customization |
| `internal/container/docker.go` | Docker adapter |
| `internal/container/podman.go` | Podman adapter and rootless behavior |
| `internal/container/podman_sysctl_linux.go` | Linux override-file setup |
| `internal/container/podman_sysctl_other.go` | Non-Linux no-op |
| `internal/container/run_lock_*.go` | Cross-process lock or fallback signal |
| `internal/container/run_serialization.go` | Leases and fallback mutex |
| `internal/container/sandbox_engine.go` | Flatpak/Snap decorator |
| `internal/containerplan/persistent.go` | Pure persistent planning |
| `internal/runtime/container_persistent.go` | Persistent state mutation flow |
| `internal/runtime/container_prepare.go` | Interactive command preparation and cleanup composition |
