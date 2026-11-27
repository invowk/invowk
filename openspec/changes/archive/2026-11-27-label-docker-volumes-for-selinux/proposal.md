## Why

On hosts where the Docker daemon runs with SELinux (Fedora, RHEL, CentOS Stream, Fedora Silverblue/Kinoite), every container command that mounts the workspace fails with "Permission denied". Files on the host carry labels such as `tmp_t` or `user_home_t`, and the confined container process (`container_t`) may not read them. The Podman engine already appends `:z` to volume mounts, which relabels them to `container_file_t`. The Docker engine never did.

CI runs on Ubuntu, which has no SELinux, so this path was never exercised. It surfaced on a Fedora Silverblue toolbox that talks to the host Docker daemon: five `internal/runtime` container integration tests and the whole `TestContainerCLI` suite failed there on a clean `main`.

## What Changes

- The Docker engine applies the same SELinux volume labeler as Podman (`:z`, preserving any explicit `:z`/`:Z`), when the Docker daemon reports `name=selinux` in `docker info` security options.
- The daemon is asked rather than the local host, because the daemon may run elsewhere (a toolbox using the host socket, or a Docker Desktop VM). The answer is cached per engine. Any probe failure means "no SELinux", which keeps today's unlabeled mounts.
- `NewDockerEngineWithSELinuxCheck` mirrors `NewPodmanEngineWithSELinuxCheck` for tests.
- The CLI container test harness forwards engine connection variables (`DOCKER_HOST`, `DOCKER_CONTEXT`, `DOCKER_TLS_VERIFY`, `DOCKER_CERT_PATH`, `CONTAINER_HOST`, `CONTAINER_CONNECTION`) into testscript, so a daemon reachable only through them is visible to the binary under test.
- Architecture docs and the component diagram describe SELinux labeling for both engines.

## Capabilities

### New Capabilities

- `container-selinux-volume-labels`: SELinux volume labeling for Docker and Podman engines, and how each detects SELinux.

### Modified Capabilities

(none)

## Impact

- **Code:** `internal/container/docker.go`; tests in `internal/container/engine_docker_selinux_test.go`; `tests/cli/cmd_container_test.go` and `tests/cli/container_harness_test.go`.
- **Behaviour:** on SELinux Docker hosts, workspace mounts now work. `:z` relabels the mounted host directory to `container_file_t`, as Podman already does. On non-SELinux hosts nothing changes.
- **Docs:** `docs/architecture/c4-component-container.md`, the website page and its pt-BR translation, `docs/diagrams/c4/component-container.d2` and its render.
