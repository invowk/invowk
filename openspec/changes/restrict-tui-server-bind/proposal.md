## Why

During every interactive command (`-i`), the TUI server listens on `0.0.0.0` (`internal/tuiserver/server.go:108`), so anyone on the LAN can reach the port. A per-session bearer token is the only guard. The token check is a plain string comparison (`server.go:250-252`), and `/health` answers anyone without authentication (`server.go:89`, `:233-237`), which lets a LAN scan find the port. The server binds this wide only so that containers can reach it through the engine's host alias (`internal/runtime/container_prepare.go:121-128`). Native and virtual runs do not need it at all. This is tracker item N5 (change S3), scheduled for v0.17.0.

## What Changes

- **BREAKING (narrow)**: the TUI server no longer listens on every interface. Native, virtual-sh and virtual-lua runs bind loopback only. Container runs bind loopback plus the address the engine actually routes the host alias to:
  - rootful Docker on Linux: the default bridge gateway. Rootful Podman gets the same plan only once a probe verifies it;
  - Docker Desktop: loopback, because the engine forwards the alias there;
  - rootless Podman with a pasta binary: loopback, with a single forwarded port (`--network pasta:-T,<port>`).
- `INVOWK_TUI_ADDR` inside a container names the address the server actually bound, instead of a hostname alias the engine may resolve elsewhere.
- **BREAKING (narrow)**: some configurations have no route to a loopback or bridge address. These are:
  - a persistent container under rootless Podman;
  - rootless Podman without pasta (slirp4netns only);
  - rootless Docker;
  - a remote engine (`DOCKER_HOST`, `podman --remote`, Podman machine);
  - Snap, and Flatpak without host networking;
  - any mode not yet verified that would need a non-loopback bind. In those, `invowk cmd … -i` with the container runtime fails before it starts the container. The error names the new opt-in, `container.tui_server_bind: "all-interfaces"`. Invowk never widens the bind on its own.
- Token checks on `/tui` use `crypto/subtle.ConstantTimeCompare`.
- `/health` requires the same bearer token. The in-tree client (`internal/tuiclient/client.go:155-173`) sends it.
- Container integration tests on `debian:stable-slim` prove that the narrow bind is reachable from the container and that the server does not listen on any other interface.

## Migration notes

- Nothing changes for native, virtual and rootful Docker runs, or for ephemeral rootless Podman runs where pasta is installed. Interactive mode keeps working; only `INVOWK_TUI_ADDR` now carries an IP literal instead of `host.docker.internal` / `host.containers.internal`. Scripts that read `INVOWK_TUI_ADDR` should treat it as an opaque URL.
- Some configurations now fail fast. They include interactive persistent containers under rootless Podman, rootless Podman without pasta, rootless Docker, remote engines (including Podman machine), Snap, and Flatpak without `--share=network`. For rootless Podman, installing pasta (`passt`) restores ephemeral interactive runs without widening the bind. To restore the previous exposure knowingly, set `container: tui_server_bind: "all-interfaces"` in `config.cue`. Invowk then warns at every interactive container run that the port is reachable from the network.
- An `invowk` binary older than v0.17.0 that runs inside a container and calls `/health` without a token gets `401`. It then treats the TUI server as unavailable. The auto-provisioned binary is the host's own and is unaffected.

## Capabilities

### New Capabilities

- `tui-server-exposure`: the network exposure of the interactive TUI server: which addresses it binds per runtime and engine, how containers are pointed at it, fail-safe handling of unroutable setups and the explicit opt-in, and authentication of every endpoint.

### Modified Capabilities

(none)

## Impact

- Code: `internal/tuiserver` (multi-listener bind, constant-time auth, authenticated `/health`), `internal/tuiclient` (token on `/health`), `internal/app/commandadapters/interactive.go` (choose the bind plan before `Start`), `internal/runtime` (container exposure plan, pasta forward), `internal/container` (engine network facts, `RunOptions` network mode), `internal/config` and `config_schema.cue` (`container.tui_server_bind`).
- Tests: unit tests for the plan selection and the server; real-engine container integration tests (`debian:stable-slim`).
- Docs: `website/docs/advanced/interactive-mode.mdx` and its pt-BR translation, the config schema reference, README; the e13 comments in `sonar-project.properties` and `.sonarcloud.properties`; `.claude/issues/cst-adoption-gaps.md` N5.
- No new dependencies.
