## Why

During every interactive command (`-i`), the TUI server listens on `0.0.0.0` (`internal/tuiserver/server.go:108`), so anyone on the LAN can reach the port. A per-session bearer token is the only guard. The token check is a plain string comparison (`server.go:250-252`). `/health` and unknown paths answer anyone without authentication (`server.go:87-89`, `:233-237`). The server binds this wide only so that containers can reach it through the engine's host alias (`internal/runtime/container_prepare.go:121-128`). Native and virtual runs do not need it at all. This is tracker item N5 (change S3), scheduled for v0.17.0.

## What Changes

- **BREAKING (narrow)**: the TUI server no longer listens on every interface.
  - Native, virtual-sh and virtual-lua runs bind loopback only.
  - Container runs bind loopback plus, at most, the one address the engine routes to the host:
    - local rootful Docker on Linux: loopback plus the default bridge gateway;
    - Docker Desktop: loopback only, because the engine forwards `host.docker.internal` to the host;
    - ephemeral rootless Podman with a pasta binary: loopback only, with exactly one forwarded port (`--network=pasta:-T,<port>`).
- For rootful Docker and rootless Podman, `INVOWK_TUI_ADDR` inside the container carries the IP literal that the server listens on, instead of a hostname alias that the engine may resolve elsewhere. Docker Desktop keeps `host.docker.internal`.
- **BREAKING (narrow)**: other local engine modes have no narrow route, so `invowk cmd … -i` with the container runtime fails before it starts a container. The error names the reason and the new opt-in, `container.tui_server_bind: "all-interfaces"`. These modes are:
  - rootful Podman;
  - a persistent container under rootless Podman;
  - rootless Podman without pasta;
  - rootless Docker;
  - Snap;
  - a failed engine query;
  - any mode not listed.

  Invowk never widens the bind on its own.
- **BREAKING (narrow)**: with a remote engine (a `tcp://` or `ssh://` Docker endpoint, `podman-remote`, `podman --remote` or Podman machine), or Flatpak without host networking, interactive container runs fail with an error that explains that the engine's containers cannot reach this host. A wide bind never reached the host there, so the opt-in neither applies nor is offered.
- Every request to the TUI server, including `/health` and unknown paths, needs the bearer token. The token is checked first, with `crypto/subtle.ConstantTimeCompare`. The in-tree client (`internal/tuiclient/client.go:155-173`) sends it on `/health` too.
- Container integration tests on `debian:stable-slim` exercise the production path. They prove that the container reaches the narrow bind, and check from the host that the server listens on no other address.

## Migration notes

- Nothing changes for native and virtual runs, local rootful Docker on Linux, Docker Desktop, or ephemeral rootless Podman runs where pasta is installed. Interactive mode keeps working. Under rootful Docker and rootless Podman, `INVOWK_TUI_ADDR` now carries an IP literal instead of `host.docker.internal` / `host.containers.internal`. Scripts that read it should treat it as an opaque URL.
- Some local configurations now fail fast. They are interactive container runs under rootful Podman, persistent containers under rootless Podman, rootless Podman without pasta, rootless Docker and Snap.
  - For rootless Podman, installing pasta (the `passt` package) restores ephemeral interactive runs without widening the bind.
  - To restore the previous exposure knowingly, set `container: tui_server_bind: "all-interfaces"` in `config.cue`. Invowk then warns at every such run that the port is reachable from the network.
- Interactive container runs against remote engines (including Podman machine), and from Flatpak without `--share=network`, now fail. Their TUI components never reached the host before either. Run those commands without `-i`, or use a local engine.
- Anything that probed `/health` without the token now gets `401`. No shipped invowk command calls `/health`; the `invowk tui *` commands already send the token on every request.
- Downgrading below v0.17.0 with `container.tui_server_bind` set fails config validation. Remove the key first.

## Capabilities

### New Capabilities

- `tui-server-exposure`: the network exposure of the interactive TUI server. It covers which addresses the server binds for each runtime and engine mode, how containers are pointed at it, fail-safe handling of restorable and no-route modes, the explicit opt-in, and authentication of every request.

### Modified Capabilities

(none)

## Impact

- Code:
  - `internal/tuiserver`: multi-listener bind, plus constant-time auth over the whole mux;
  - `internal/tuiclient`: the token on `/health`;
  - `internal/app/commandadapters/interactive.go`: the bind plan is chosen before `Start`, with the policy injected through the constructor;
  - `internal/containerplan`: pure plan selection;
  - `internal/runtime`: container exposure plan and pasta forward;
  - `internal/container`: engine network facts, plus a `RunOptions` network mode;
  - `internal/config` and `config_schema.cue`: `container.tui_server_bind`.
- Tests:
  - unit tests for the plan selection, the server and the adapter;
  - real-engine container integration tests (`debian:stable-slim`), with the expected plan pinned per CI job;
  - updated `config_show`/`config_dump` goldens.
- Docs:
  - `website/docs/advanced/interactive-mode.mdx` and `website/docs/reference/config-schema.mdx`, with their pt-BR translations;
  - README;
  - the e13 comments in `sonar-project.properties` and `.sonarcloud.properties`;
  - `.claude/issues/cst-adoption-gaps.md` N5.
- No new dependencies.
