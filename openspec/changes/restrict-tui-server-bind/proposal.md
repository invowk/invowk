## Why

During every interactive command (`-i`), the TUI server listens on `0.0.0.0` (`internal/tuiserver/server.go:108`), so anyone on the LAN can reach the port. A per-session bearer token is the only guard. The token check is a plain string comparison (`server.go:250-252`). `/health` and unknown paths answer anyone without authentication (`server.go:87-89`, `:233-237`). The server binds this wide only so that containers can reach it through the engine's host alias (`internal/runtime/container_prepare.go:121-128`). Native and virtual runs do not need it at all. This is tracker item N5 (change S3), scheduled for v0.17.0.

## What Changes

- **BREAKING (narrow)**: the TUI server no longer listens on every interface.
  - Native, virtual-sh and virtual-lua runs bind loopback only.
  - Container runs bind loopback plus, at most, the one address the engine routes to the host:
    - local rootful Docker on Linux: loopback plus the default bridge gateway;
    - Docker Desktop: loopback only, because the engine forwards `host.docker.internal` to the host;
    - ephemeral rootless Podman with a pasta binary: loopback only, with exactly one forwarded port (`--network=pasta:-T,<port>`).
- **BREAKING (narrow)**: for rootful Docker and rootless Podman, `INVOWK_TUI_ADDR` inside the container carries the IP literal that the server listens on, instead of a hostname alias that the engine may resolve elsewhere. Docker Desktop keeps `host.docker.internal`.
- **BREAKING**: other engine modes have no narrow route. There, `invowk cmd … -i` with the container runtime fails before start, with exit status 1. invowk opens no listener and does not provision, create or start the command's container. The error names the reason, the new opt-in `container.tui_server_bind: "all-interfaces"`, and that the command runs without `-i`. invowk does not fall back to running without a TUI server: a real session showed that `name=$(invowk tui input)` then draws nothing and hands the script terminal control sequences as its value. These modes are:
  - a remote engine (a `tcp://` or `ssh://` Docker endpoint, `podman-remote`, `podman --remote` or Podman machine). Here the error adds that the opt-in helps only if the engine's host alias leads back to this machine;
  - rootful Podman;
  - a persistent container under rootless Podman;
  - rootless Podman without pasta;
  - rootless Docker;
  - Snap;
  - a failed engine query;
  - any mode not listed.

  Invowk never widens the bind on its own.
- The opt-in also applies to Docker Desktop, the only plan that ships without a local probe, so users there have a remedy if its loopback assumption is wrong. It never changes the verified rootful Docker and rootless Podman plans, or host runtimes. It is read only from user configuration (`--ivk-config`, `INVOWK_CONFIG`, the user config file), never from an invowkfile or module.
- In Flatpak without host networking, interactive container runs also fail before start. The error explains that the sandbox's network is private; no bind is reachable from the engine's containers there, so the opt-in neither applies nor is offered.
- The bind plan is validated before any listener opens: a narrow plan lists `127.0.0.1` first and only specified IPv4 addresses, and a wildcard plan exists only where the opt-in applies. An invalid plan fails the run.
- Non-interactive container runs (without `-i`) do not change: no server, no engine query, no warning.
- **BREAKING (narrow)**: every request to the TUI server, including `/health` and unknown paths, needs the bearer token. The token is checked first, with `crypto/subtle.ConstantTimeCompare`. The in-tree client (`internal/tuiclient/client.go:155-173`) sends it on `/health` too.
- The TUI client bounds the connect with a 5-second dial timeout. A server that the container cannot reach gives an error that names the address and the opt-in within seconds, instead of a hang. It does not fall back to direct rendering.
- Container integration tests on `debian:stable-slim` exercise the production path. They prove that the container reaches the narrow bind, check from the host that the server listens on no other address, and check that a no-route interactive run fails before any container starts.

## Migration notes

- Nothing changes for native and virtual runs, local rootful Docker on Linux, Docker Desktop, or ephemeral rootless Podman runs where pasta is installed. Interactive mode keeps its overlay. Under rootful Docker and rootless Podman, `INVOWK_TUI_ADDR` now carries an IP literal instead of `host.docker.internal` / `host.containers.internal`. Scripts that read it should treat it as an opaque URL.
- **BREAKING**: `invowk cmd … -i` fails before start, with exit status 1, for container runs under remote engines (including Podman machine), rootful Podman, persistent containers under rootless Podman, rootless Podman without pasta, rootless Docker, Snap, and Flatpak without `--share=network`. The same commands without `-i` run as before.
  - For rootless Podman, installing pasta (the `passt` package) restores the overlay for ephemeral runs without widening the bind.
  - To restore the previous exposure knowingly, set `container: tui_server_bind: "all-interfaces"` in `config.cue`. Invowk then warns at every such run that the port is reachable from the network. Set it in the file that invowk actually reads: under `--ivk-config` or `INVOWK_CONFIG`, that file alone, since the user config file is then not read. For a remote engine this helps only when its host alias leads back to this machine. The opt-in does not apply in Flatpak without network.
- If `invowk tui *` inside a container reports that the TUI server is unreachable (for example under Docker Desktop, or behind a host firewall), the same opt-in restores the previous behaviour.
- Anything that probed `/health` or any other path without the token now gets `401`. No shipped invowk command calls `/health`; the `invowk tui *` commands already send the token on every request.
- Downgrading below v0.17.0 with `container.tui_server_bind` set fails config validation. Remove the key first.

## Capabilities

### New Capabilities

- `tui-server-exposure`: the network exposure of the interactive TUI server. It covers which addresses the server binds for each runtime and engine mode, how containers are pointed at it, the refusal of interactive runs in modes without a narrow route, the explicit opt-in and where it may be set, the client's bounded connect, and authentication of every request.

### Modified Capabilities

(none)

## Impact

- Code:
  - `internal/tuiserver`: multi-listener bind, plus constant-time auth over the whole mux;
  - `internal/tuiclient`: the token on `/health`, and a bounded dial with an actionable unreachable error;
  - `internal/app/commandadapters/interactive.go`: the bind plan is chosen and validated before `Start`; a refused or invalid plan fails the run before `PrepareInteractive`;
  - `internal/containerplan`: pure plan selection;
  - `internal/runtime`: container exposure planner (reading the policy from the runtime's config) and pasta forward;
  - `internal/container`: engine network facts, plus a `RunOptions` network mode;
  - `internal/config` and `config_schema.cue`: `container.tui_server_bind`.
- Tests:
  - unit tests for the plan selection, the server and the adapter;
  - real-engine container integration tests (`debian:stable-slim`), with the expected plan pinned per CI job;
  - updated `config_show`/`config_dump` goldens.
- Probe: `openspec/changes/restrict-tui-server-bind/probe/` holds the one-command Docker Desktop (row 6) reachability probe for native Windows (PowerShell) and WSL2 or macOS (bash).
- Docs:
  - `website/docs/advanced/interactive-mode.mdx` and `website/docs/reference/config-schema.mdx`, with their pt-BR translations;
  - README;
  - the e13 comments in `sonar-project.properties` and `.sonarcloud.properties`;
  - `.claude/issues/cst-adoption-gaps.md` N5, and the new N7 (SSH host-callback bind);
  - `tasks/next/formal-phase3-closeout.md` (S5332 wording).
- No new dependencies.
