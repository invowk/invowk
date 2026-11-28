# container-selinux-volume-labels Specification

## Purpose
Defines Docker SELinux `:z` volume mount labeling derived from cached `docker info` security options (`internal/container/docker.go`) and engine connection environment forwarding for container CLI tests (`tests/cli/`).

## Requirements

### Requirement: Docker volumes are labeled when the daemon enforces SELinux
Docker (`name=selinux`) and Podman (`/sys/fs/selinux`) SHALL append `:z` to volume mounts by default when SELinux is enforced, honor `workspace_mount.selinux_label` (`"shared"` -> `:z`, `"private"` -> `:Z`, `"none"` -> no relabel) and operator `container.selinux_labeling: "disabled"`, and preserve any explicit `:z` or `:Z` already present on a mount.

#### Scenario: SELinux daemon
- **WHEN** `docker info --format '{{json .SecurityOptions}}'` lists `name=selinux` and default `selinux_label: "auto"` applies
- **THEN** a mount `/host:/container` SHALL be passed to `docker run` as `/host:/container:z`, and `/host:/container:ro` as `/host:/container:ro,z`

#### Scenario: Daemon without SELinux
- **WHEN** the daemon's security options do not include `name=selinux`
- **THEN** volume mounts SHALL be passed unchanged

#### Scenario: Explicit label
- **WHEN** a mount already carries `:Z` or `workspace_mount.selinux_label` is `"private"`
- **THEN** it SHALL carry `:Z` without appending `:z`

#### Scenario: Workspace opt-out skips relabeling on host checkout
- **WHEN** a command specifies `workspace_mount: { readonly: true, selinux_label: "none" }`
- **THEN** the `/workspace` bind-mount SHALL NOT receive `:z` or `:Z`, leaving host SELinux xattrs untouched

### Requirement: Operator config may disable container SELinux labeling
`internal/config/config_schema.cue` (`#ContainerConfig`) SHALL provide an operator-only setting (such as `container.selinux_labeling: *"enabled" | "disabled"`) that passes `--security-opt label=disable` to the container engine and omits automatic `:z` volume relabeling. This capability SHALL NOT be exposed in `invowkfile.cue` or `invowkmod.cue`.

#### Scenario: Operator config disables SELinux labeling for read-only inspection
- **WHEN** `config.cue` sets `container: { selinux_labeling: "disabled" }`
- **THEN** `docker run` / `podman run` SHALL include `--security-opt label=disable` and SHALL NOT append `:z` to bind-mounts

### Requirement: SELinux detection asks the Docker daemon once
The Docker engine SHALL decide SELinux labeling from the daemon's reported security options, not from the local host's `/sys/fs/selinux`, SHALL query the daemon at most once per engine, and SHALL treat any query failure as "no SELinux".

#### Scenario: Remote daemon
- **WHEN** the Docker CLI reaches a daemon on another host or VM through `DOCKER_HOST` or a context
- **THEN** labeling SHALL follow that daemon's security options

#### Scenario: Unreachable daemon
- **WHEN** `docker info` fails or the Docker binary is missing
- **THEN** volume mounts SHALL be passed unchanged

### Requirement: Container CLI tests reach the configured engine
The CLI container test harness SHALL forward `DOCKER_HOST`, `DOCKER_CONTEXT`, `DOCKER_TLS_VERIFY`, `DOCKER_CERT_PATH`, `CONTAINER_HOST`, and `CONTAINER_CONNECTION` into each testscript environment when they are set in the test process, and SHALL NOT forward other variables through this mechanism.

#### Scenario: Toolbox using the host socket
- **WHEN** the test process has `DOCKER_HOST=unix:///run/host/run/docker.sock`
- **THEN** the `invowk` binary under testscript SHALL see the same `DOCKER_HOST`
