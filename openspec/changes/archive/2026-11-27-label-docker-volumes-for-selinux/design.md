## Context

`internal/container/podman.go` builds its engine with `WithVolumeFormatter(makeSELinuxLabelAdder(isSELinuxPresent))`, where `isSELinuxPresent` checks the local `/sys/fs/selinux`. `NewDockerEngine` added no formatter, so Docker volume mounts were never labeled.

## Decisions

### D1. Ask the daemon, not the host
Docker's daemon is often not local to the CLI: a toolbox or dev container uses the host socket, and Docker Desktop runs the daemon in a VM. A local `/sys/fs/selinux` check would be wrong in both directions. `docker info --format '{{json .SecurityOptions}}'` reports what the daemon actually enforces (for example `["name=seccomp,profile=builtin","name=selinux","name=cgroupns"]`). The result is parsed as JSON and matched on the exact `name=selinux` field.

*Alternative considered:* reuse `isSELinuxPresent`. Rejected for the reason above.

### D2. Lazy and cached
The engine is constructed before any command runs and may never mount a volume. `sync.OnceValue` wraps the probe, so the daemon is asked on the first volume format and at most once per engine, bounded by the existing `availabilityTimeout`.

### D3. Fail open to today's behaviour
If `docker info` fails, the probe returns false and mounts stay unlabeled, which is exactly the behaviour before this change. Labeling never introduces a new failure mode.

### D4. Same labeler as Podman
The existing `makeSELinuxLabelAdder` already handles `:ro`, multiple options, and explicit `:z`/`:Z`. Docker reuses it, so both engines produce identical mount strings for the same input.

## Risks / Trade-offs

- **[`:z` relabels the host directory]** → This matches Podman's existing behaviour and is what SELinux Docker hosts require. `:Z` (private) is not used, because mounts may be shared between containers.
- **[An extra `docker info` call per engine]** → Called only when a volume is formatted, and cached.
