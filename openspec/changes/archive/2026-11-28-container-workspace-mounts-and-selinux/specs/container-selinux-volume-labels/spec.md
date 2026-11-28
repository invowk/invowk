# Spec Delta

## Purpose

Updates `container-selinux-volume-labels` so `workspace_mount.selinux_label` can control `:z`/`:Z`/no-relabel on `/workspace`, and operator `config.cue` can disable container SELinux confinement via `--security-opt label=disable`.

## MODIFIED Requirements

### Requirement: Docker volumes are labeled when the daemon enforces SELinux
The Docker engine SHALL append the SELinux shared label `:z` to every volume mount when the Docker daemon reports `name=selinux` in its `docker info` security options and neither an explicit `:z`/`:Z` label nor an opt-out (`workspace_mount.selinux_label: "none"`, `"private"` which emits `:Z`, or operator `container.selinux_labeling: "disabled"`) applies, and SHALL preserve an explicit `:z` or `:Z` already present on the mount. Podman volume formatting SHALL follow the same `workspace_mount.selinux_label` and operator `container.selinux_labeling` rules when `/sys/fs/selinux` is present.

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

## ADDED Requirements

### Requirement: Operator config may disable container SELinux labeling
`internal/config/config_schema.cue` (`#ContainerConfig`) SHALL provide an operator-only setting (such as `container.selinux_labeling: *"enabled" | "disabled"`) that passes `--security-opt label=disable` to the container engine and omits automatic `:z` volume relabeling. This capability SHALL NOT be exposed in `invowkfile.cue` or `invowkmod.cue`.

#### Scenario: Operator config disables SELinux labeling for read-only inspection
- **WHEN** `config.cue` sets `container: { selinux_labeling: "disabled" }`
- **THEN** `docker run` / `podman run` SHALL include `--security-opt label=disable` and SHALL NOT append `:z` to bind-mounts
