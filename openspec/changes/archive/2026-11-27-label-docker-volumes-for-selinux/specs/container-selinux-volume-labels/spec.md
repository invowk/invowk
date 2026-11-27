## ADDED Requirements

### Requirement: Docker volumes are labeled when the daemon enforces SELinux
The Docker engine SHALL append the SELinux shared label `:z` to every volume mount when the Docker daemon reports `name=selinux` in its `docker info` security options, and SHALL preserve an explicit `:z` or `:Z` already present on the mount.

#### Scenario: SELinux daemon
- **WHEN** `docker info --format '{{json .SecurityOptions}}'` lists `name=selinux`
- **THEN** a mount `/host:/container` SHALL be passed to `docker run` as `/host:/container:z`, and `/host:/container:ro` as `/host:/container:ro,z`

#### Scenario: Daemon without SELinux
- **WHEN** the daemon's security options do not include `name=selinux`
- **THEN** volume mounts SHALL be passed unchanged

#### Scenario: Explicit label
- **WHEN** a mount already carries `:Z`
- **THEN** it SHALL be passed unchanged

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
