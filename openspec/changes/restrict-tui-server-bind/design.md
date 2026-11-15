# Design

## Context

See proposal.md (Why). Current behaviour, cited against `origin/main` at f4969e90:

- **Bind.** `Server.Start` listens on `0.0.0.0:0` (`internal/tuiserver/server.go:107-108`) with a single listener and one `http.Server` (`:91-96`, `:128`). The type comment says so (`:34-35`).
- **Token.** It is 32 bytes from `crypto/rand`, hex-encoded (`server.go:71`, `:322-329`). `/tui` checks it with `!=` (`:250-252`), and only after the method check (`:244-247`). `TestServerMethodNotAllowed` pins that order: `GET /tui` without a token returns `405` (`server_test.go:322-350`).
- **Health.** `/health` is registered without authentication (`server.go:89`) and answers `200 ok` to anyone (`:233-237`). The only caller is `tuiclient.IsAvailableContext`, which already holds the token (`internal/tuiclient/client.go:110`, `:137-140`) but does not send it (`:155-173`). It sends the token only on component requests (`:502`).
- **Startup order.** The interactive adapter creates and starts the server (`internal/app/commandadapters/interactive.go:47`, `:57`). Only afterwards does it ask the runtime for a host alias (`:66-71`). It then sets `ctx.TUI` (`:73-74`) and calls `PrepareInteractive` (`:76`).
- **Host alias.** The container runtime's alias is `host.containers.internal` for Podman and `host.docker.internal` otherwise (`internal/runtime/container_prepare.go:121-128`, constants at `internal/runtime/container.go:20-22`). For TUI and SSH it always adds `--add-host host.docker.internal:host-gateway` (`internal/runtime/container_exec.go:189-194`). The runtime injects the URL into the container env as `INVOWK_TUI_ADDR` (`container_exec.go:132-133`, `internal/runtime/runtime.go:530-536`).
- **Persistent containers.** They take their extra hosts at creation (`internal/runtime/container_persistent.go:160-172`), so their network setup is fixed when they are created.
- **Engine options.** `RunOptions` has no network-mode field (`internal/container/engine.go:253-254`, the only network-related fields are `ExtraHosts` and `Ports`; validation at `:536-547`).
- **Config.** `#ContainerConfig` holds only `auto_provision` (`internal/config/config_schema.cue:66-72`).
- **Sandboxes.** Flatpak and Snap are detected in `pkg/platform/sandbox.go:89`. Engines in a sandbox run on the host through `flatpak-spawn --host` / `snap run --shell` (`internal/container/sandbox_engine.go:458-483`).

### Reachability probes (2026-11-14, Fedora 45 Silverblue host)

Each probe started Python listeners on three host addresses: `127.0.0.1`, `172.17.0.1` (docker0) and `192.168.1.109` (LAN). A `debian:stable-slim` container then tried to connect to each with bash `/dev/tcp`.

| Engine / mode | Alias resolves to | Reachable bind | Status |
|---|---|---|---|
| Docker 29.9.2 rootful, default bridge, `--add-host …:host-gateway` | `172.17.0.1` (= `docker network inspect bridge` gateway) | `172.17.0.1` only; not loopback | verified |
| Podman 6.1.3 rootless, pasta (default), `host.containers.internal` | `169.254.1.2` (pasta `--map-guest-addr`) | LAN address only; not loopback, not docker0 | verified |
| Same, with `--add-host host.docker.internal:host-gateway` | `169.254.1.2` | LAN address only | verified |
| Podman rootless, `--network pasta:-T,<port>`, target `127.0.0.1:<port>` in the container | n/a | host loopback, that port only; a second loopback port was refused | verified |
| Podman rootless, `--network pasta:--map-host-loopback,169.254.1.3` | n/a | all of host loopback | verified (rejected, see D3) |
| Podman 6 `slirp4netns` | n/a | n/a | removed in Podman 6 ("slirp4netns support has been removed") |

Podman documents that pasta passes `--map-guest-addr 169.254.1.2` by default for `host.containers.internal`, together with `--no-map-gw`, and that `pasta:-T,<port>` forwards one TCP port from container to host loopback (docs.podman.io, podman-run(1) `--network`).

The probes ran from the `devbox` toolbox, which shares the host network namespace, with Podman reached through `flatpak-spawn --host`. That matches the Flatpak `--share=network` path.

## Goals / Non-Goals

**Goals:**
- The TUI server never listens on a wildcard address unless the user opts in explicitly (spec: Container bind follows the engine's host route; Wide bind requires explicit opt-in).
- Each supported engine mode gets one plan that is reachable by construction. A mode we have not verified that would need a non-loopback bind fails fast.

**Non-Goals:**
- The SSH host-callback server. It already defaults to `127.0.0.1` (`internal/sshserver/server.go:127`, `:191`; used unchanged by `internal/app/commandadapters/host_access.go:52`). Per the probes above, that bind is unreachable from rootful Docker and rootless Podman containers. `tests/cli/testdata/container_callback.txtar:15-29` only asserts env vars, so no test notices. This is a separate defect. It should become its own change that reuses this design's exposure plan, not part of S3.
- TLS. Sonar go:S5332 stays an accepted design choice for local IPC.
- IPv6 binds and Windows-container or Alpine images (repository invariants).

## Maintainer decisions (2026-11-14)

1. Restrict the bind to loopback plus the container engine's bridge gateway. Cover Docker and Podman, rootful and rootless, including pasta/slirp4netns where `host.containers.internal` maps to the host, and Flatpak/Snap-wrapped engines.
2. Compare tokens with `crypto/subtle.ConstantTimeCompare`.
3. Authenticate `/health`, or reduce it to a non-identifying response; this design decides which (D5).
4. Add container integration tests, on `debian:stable-slim` only.
5. If a configuration cannot reach the server without a wide bind, fail safe with an explicit opt-in or a clear error. Never widen silently.
6. Sonar go:S5332 at `server.go` is resolved in SonarCloud as Accepted. Tracker N5 wording changes from "false positive" to "Accepted (real gap tracked as N5/S3)".
7. Ship in v0.17.0. Merge order: S1 -> C1 -> F1 -> S3 -> S4.

## Decisions

### D1. Pick the bind plan before `Start`

A new optional runtime port, `TUIExposurePlanner`, returns a `TUIExposurePlan` before the server starts. Runtimes that do not implement it (native, virtual) get the loopback plan. The plan holds:
- the bind addresses (`[]netip.Addr`, loopback always first);
- the host the container should dial (an IP literal);
- an optional network mode for the container (`pasta:-T,<port>`);
- an `Unroutable` reason.

`interactive.go` calls the planner before `tuiserver.New`/`Start`, which moves the alias lookup now at `:66-71` ahead of `:57`. An unroutable plan without the opt-in returns the actionable error before anything starts. `HostServiceAddressProvider` stays for SSH.
- *Alternative:* start wide, then restrict. Rejected: that opens a window of exposure, and the plan has to exist before the port does.

### D2. Ask the engine for its network facts; never hard-code addresses

A `container` method collects the facts behind each plan, through the existing sandbox-aware engine, so Flatpak and Snap are covered:
- Docker: `docker info --format '{{.OperatingSystem}}|{{json .SecurityOptions}}'`, `docker context inspect` (endpoint scheme) and `docker network inspect bridge` (gateway). Probed locally: `Fedora Linux …|[…selinux…]`, `unix:///run/host/run/docker.sock`, `172.17.0.1`.
- Podman: `podman info --format '{{.Host.Security.Rootless}}|{{.Host.RootlessNetworkCmd}}|{{.Host.ServiceIsRemote}}'` (probed: `true|pasta|false`) and, when rootful, `podman network inspect podman` (gateway).

Plan selection is a pure function in `containerplan` (or `runtime`), tested table-driven.

| Facts | Plan |
|---|---|
| native / virtual-sh / virtual-lua | loopback |
| Docker, local, rootful, Linux, gateway on a local interface | loopback + gateway; URL host = gateway IP |
| Podman, local, rootful, gateway on a local interface | loopback + gateway; URL host = gateway IP (unverified, task 4.3) |
| Docker Desktop (`OperatingSystem` = "Docker Desktop") or Podman machine | loopback; URL host = engine alias (unverified, task 4.4) |
| Podman, rootless, pasta, ephemeral container | loopback; network `pasta:-T,<port>`; URL host = `127.0.0.1` |
| Podman rootless + persistent container; Docker rootless (`name=rootless`); remote endpoint (`tcp://`, `ssh://`, `ServiceIsRemote`); gateway not on a local interface; Flatpak without host network; facts query fails | unroutable |

"Gateway on a local interface" means `net.InterfaceAddrs()` contains the IP. That check catches custom `host-gateway-ip` setups and remote daemons that report a gateway invowk cannot bind.

### D3. Rootless Podman: forward one port with `pasta:-T`

We rejected two alternatives:
- Binding the LAN address that `--map-guest-addr` lands on. It is reachable only on the LAN-facing interface, which is the exposure we are removing.
- `--map-host-loopback`. It gives the container all of host loopback, so local databases and agents become reachable from the container, which weakens the container boundary.

`-T,<port>` forwards exactly the server's port (probe row 4). It needs the port at container start. The port is known then, because `Start` precedes `PrepareInteractive` (`interactive.go:57`, `:76`).

Persistent containers are created once with a fixed network, and every interactive run picks a fresh port, so they are unroutable for now (open question Q1). `RunOptions` gets a validated `NetworkMode` field, passed as `--network`. It applies only to `run`, never to `exec`.

### D4. One port on every bound address

The server listens on `127.0.0.1:0`, reads the port, then listens on `<gateway>:<port>`. If the second listen fails with `EADDRINUSE`, it closes both and retries, up to 5 times, then fails with the last error. One `http.Server` serves every listener. Each `Serve` goroutine is tracked through `serverbase` (`AddGoroutine`/`DoneGoroutine`, as at `server.go:123-135`), and `Stop` closes all of them.
- *Alternative:* a different port per address. Rejected: the container gets one URL, and per-address URLs add plumbing for no gain.

### D5. Authenticate `/health`; check the token first, in constant time

Both handlers share an auth middleware. It runs before the method check and compares `subtle.ConstantTimeCompare([]byte(header), []byte("Bearer "+token))`. The only length leak is the fixed token length. A failure answers `401` with the body `unauthorized`.

`tuiclient.IsAvailableContext` sends the header. We chose this over a non-identifying `204` because the only caller already holds the token, and an open `/health` lets a scanner confirm the port is live even without a body. The `405`-before-auth order goes away with it; update `TestServerMethodNotAllowed` to send the token.

### D6. The URL carries the bound IP

For bridge plans the container dials the gateway IP literal, not the alias. That keeps reachability independent of how a given engine version or daemon config resolves `host-gateway`. It also keeps working for persistent Docker containers created with the old `host-gateway` mapping. `HostServiceAddress()` keeps serving SSH unchanged.

### D7. Opt-in config, with a warning

Add `container.tui_server_bind: *"restricted" | "all-interfaces"` to `#ContainerConfig`, with a typed Go field and the schema sync tests. `"all-interfaces"` turns an unroutable or bridge plan into `0.0.0.0` plus the engine alias (today's behaviour). It prints a one-line stderr warning at every interactive container run, and it never applies to host runtimes. There is no CLI flag or environment variable; `INVOWK_CONFIG` already selects a config file.

## Risks / Trade-offs

- [Other containers on the same bridge can reach `<gateway>:<port>`] → They still need the 256-bit token, and the port lives only for the interactive run. Documented as residual exposure.
- [Local users on the host can reach loopback] → Same as today; the token guards it.
- [Rootless persistent interactive users regress to an error] → The error names the opt-in. Q1 tracks a narrow fix.
- [The engine facts query adds latency to every interactive container run] → One or two engine CLI calls, made only for `-i` container runs. Cache them per process.
- [Rows marked unverified may be wrong] → Loopback-only rows fail closed: the TUI is unreachable, nothing is exposed. Bridge rows require the gateway to be a local interface. Tasks 4.3 and 4.4 verify both or move them to unroutable.
- [An older in-container `invowk` gets `401` on `/health`] → Auto-provisioning installs the host binary, so only hand-installed old binaries are affected. Noted in the migration notes.

## Migration Plan

Ship in v0.17.0 after F1 (merge order in decision 7). Roll back by reverting the change. The new config key would then be rejected as unknown by the closed schema, so release notes tell users of `"all-interfaces"` to remove it if they downgrade.

## Open Questions

- **Q1.** Persistent containers under rootless Podman could reserve a fixed forward port at creation (recorded as a container label, `pasta:-T,<P>`), and the server would bind `127.0.0.1:<P>` on each run. Deferred: the fail-safe error is already correct, and this adds port-conflict handling.
- **Q2.** Rootless Docker (RootlessKit with slirp4netns or pasta): which host address does `host-gateway` reach, and can a narrow bind serve it? It stays unroutable until it is probed.
- **Q3.** Snap strict confinement: does the snap share the host network namespace for listeners? It is covered by the "facts query fails / not local" rule until it is probed.
- **Q4.** The SSH host-callback reachability defect from Non-Goals: file it as its own change after S3.
