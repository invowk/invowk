# Design

## Context

See proposal.md (Why). Current behaviour, cited against `origin/main` at f4969e90:

- **Bind.** `Server.Start` listens on `0.0.0.0:0` (`internal/tuiserver/server.go:107-108`) with a single listener and one `http.Server` (`:91-96`, `:128`). The type comment says so (`:34-35`). Readiness is signalled from inside the one serve goroutine (`:123-135`, `TransitionToRunning` at `:127`).
- **Token.** It is 32 bytes from `crypto/rand`, hex-encoded (`server.go:71`, `:322-329`). `/tui` checks it with `!=` (`:250-252`), and only after the method check (`:244-247`). `TestServerMethodNotAllowed` pins that order: `GET /tui` without a token returns `405` (`internal/tuiserver/server_test.go:322-350`).
- **Health and unknown paths.** `/health` is registered without authentication (`server.go:89`) and answers `200 ok` to anyone (`:233-237`). Any other path falls through to the `http.ServeMux` default `404`, also unauthenticated (`:87-89`). The only client of `/health` is `tuiclient.IsAvailableContext` (`internal/tuiclient/client.go:155-173`). It holds the token (`:110`, `:137-140`) but does not send it, and it has no production caller (only `internal/tuiclient/client_test.go:532-556`). The shipped `invowk tui *` commands go straight to component requests (for example `cmd/invowk/tui_input.go:70`), which send the token (`client.go:502`).
- **Client timeout.** The client's only bound is `http.Client.Timeout: 10 * time.Minute` (`client.go:141-142`), sized for user interaction. A dial to an address that drops packets therefore hangs until the kernel's TCP connect timeout, and the error says only "failed to send request" (`client.go:504-506`).
- **Direct-rendering fallback.** `tuiclient.NewClientFromEnv()` returns nil when `INVOWK_TUI_ADDR` is unset (`client.go:117`). Every delegating `invowk tui *` command (`choose`, `confirm`, `file`, `filter`, `input`, `pager`, `spin`, `table`, `write`) then renders in its own terminal instead (for example `cmd/invowk/tui_confirm.go:66-80`). The container runtime injects the TUI variables only when `ctx.TUI` holds them (`internal/runtime/runtime.go:530-536`) and adds the host-gateway mapping only when `ctx.TUI.ServerURL` is set (`internal/runtime/container_exec.go:189-194`).
- **Startup order.** The interactive adapter creates and starts the server (`internal/app/commandadapters/interactive.go:47`, `:57`). Only afterwards does it ask the runtime for a host alias (`:66-71`). It then sets `ctx.TUI` (`:73-74`) and calls `PrepareInteractive` (`:76`). The bridge goroutine and the deferred `Stop` assume a server (`:59-63`, `:88-90`). `InteractiveExecutor` is a stateless struct (`:23`, `:34-37`) built at app wiring time, before any config is loaded (`cmd/invowk/app.go:144`). The adapter only runs when the request is interactive: a non-interactive request goes straight to `session.Execute` (`internal/app/commandsvc/dispatch.go:110-113`).
- **Config reaches the runtime.** The container runtime is built per request from the loaded `*config.Config` (`internal/app/commandadapters/runtime_registry.go:43`, `:98`; `internal/runtime/container.go:99-120`). That config comes from the sources of the `config-source-resolution` capability (`--ivk-config`, `INVOWK_CONFIG`, the user config file; `openspec/specs/config-source-resolution/spec.md:24-26`). Invowkfile and module schemas are closed, so they cannot carry config keys.
- **Host alias.** The container runtime's alias is `host.containers.internal` for Podman and `host.docker.internal` otherwise (`internal/runtime/container_prepare.go:121-128`, constants at `internal/runtime/container.go:19-23`). A sandbox-wrapped engine reports the wrapped engine's name (`internal/container/sandbox_engine.go:65-67`). For TUI and SSH the runtime adds `--add-host host.docker.internal:host-gateway` (`internal/runtime/container_exec.go:189-194`). It injects the URL into the container env as `INVOWK_TUI_ADDR` (`container_exec.go:132-133`, `internal/runtime/runtime.go:530-536`).
- **Persistent containers.** Managed ones take their extra hosts at creation (`internal/runtime/container_persistent.go:161-172`), so their network is fixed then. `--ivk-container-name` may also target an external, running container that invowk did not create (`container_persistent.go:137-151`). Invowk does not know that container's network. TUI values are passed at exec time (`openspec/specs/persistent-container-runtime/spec.md:109-111`).
- **Engine options.** `RunOptions` has no network-mode field (`internal/container/engine.go:226-255`). Its only network fields are `ExtraHosts` and `Ports`, validated at `:536-547`. The Podman `--userns=keep-id` transformer takes the first non-flag argument as the image. It skips the values of a fixed set of flags only (`internal/container/podman.go:200-230`, list at `:213-216`).
- **Engine availability.** `DockerEngine.Available` runs `docker version` against the daemon (`internal/container/docker.go:53-61`). The container runtime is available only when its engine is (`internal/runtime/container.go:184-186`). A dead `DOCKER_HOST` therefore fails before any interactive code runs.
- **Config.** `#ContainerConfig` holds only `auto_provision` (`internal/config/config_schema.cue:66-71`). `GenerateCUE` writes the container block by hand (`internal/config/config.go:592-600`).
- **Sandboxes.** Flatpak and Snap are detected in `pkg/platform/sandbox.go:89`. Engines in a sandbox run on the host through `flatpak-spawn --host` or `snap run --shell` (`internal/container/sandbox_engine.go:458-483`).
- **CI.** The Podman job runs on `ubuntu-latest`. It starts the user `podman.socket`, and the Podman CLI runs locally (`.github/workflows/ci.yml:161-167`, matrix at `:73-74`). The Docker job uses rootful Docker on `ubuntu-latest` (`:67-68`, `:169-174`). Neither job installs OS packages.

### Reachability probes (2026-11-14, Fedora 45 Silverblue host)

Each probe started TCP listeners on host addresses: `127.0.0.1`, `172.17.0.1` (docker0) and `192.168.1.109` (LAN). A `debian:stable-slim` container then tried to connect to each one with bash `/dev/tcp`.

| Engine / mode | Alias resolves to | Reachable from the container | Status |
|---|---|---|---|
| Docker 29.9.2 rootful, default bridge, `--add-host …:host-gateway` | `172.17.0.1` (= `docker network inspect bridge` gateway) | gateway yes; loopback no; LAN no in the proposal run, yes in the review re-run | verified twice |
| Podman 6.1.3 rootless, pasta (default), `host.containers.internal` | `169.254.1.2` (pasta `--map-guest-addr`) | LAN address only; not loopback, not docker0 | verified (proposal) |
| Same, with `--add-host host.docker.internal:host-gateway` | `169.254.1.2` | LAN address only | verified (proposal) |
| Podman rootless, `--network pasta:-T,<port>`, target `127.0.0.1:<port>` in the container, with invowk's flags (`--userns=keep-id`, `-p 127.0.0.1:<h>:<c>`, `--add-host host.docker.internal:host-gateway`) | n/a | host loopback, that port only; a second loopback port was refused | verified twice (proposal; review re-run on Podman 6.1.3 with `--userns=keep-id` and `--add-host`, without `-p`) |
| Podman rootless, `--network pasta:--map-host-loopback,169.254.1.3` | n/a | all of host loopback | verified (rejected, see D3) |
| Podman 6 `slirp4netns` | n/a | n/a | removed in Podman 6 ("slirp4netns support has been removed") |
| invowk S3 build, Docker 29.9.2 rootful, `TestInteractiveTUIExposureContainerIntegration` | n/a | plan row 8; `INVOWK_TUI_ADDR` = `http://172.17.0.1:<port>`; `/health` 200 with token, 401 without; every other local IPv4 address refused from the host | verified (2026-11-15; calibrated by forcing the wildcard bind: the host-side check then failed on the LAN and other bridge addresses) |
| invowk S3 build, host Podman 6.1.3 rootless with pasta (through a `flatpak-spawn --host` shim), same test | n/a | plan row 11; `INVOWK_TUI_ADDR` = `http://127.0.0.1:<port>`; 200/401 as above; a second host loopback port blocked; persistent target (row 10) ran without a server | verified (2026-11-15; calibrated by also forwarding the second port: the check then failed) |
| CI Podman job (`ubuntu-latest`, Podman 5.7.0), first run 43677325230 | n/a | `podman network backend: true\|pasta\|false\|/usr/bin/pasta`; the integration test selected row 11 (the job's first literal `12` failed on purpose), so the job pins row 11 | observed (2026-11-15) |
| Docker 29.9.2 rootful, SSH host-callback server (`enable_host_ssh: true`), invowk at f4969e90 | `172.17.0.1` | `host.docker.internal:<ssh port>` refused | verified (2026-11-14, one run of a `debian:stable-slim` command using bash `/dev/tcp`) |

The LAN result under Docker changed between the two runs, so it depends on the host firewall state. Exposure tests therefore check from the host side, never only from inside a container (task 5.2). `podman info --format '{{.Host.Pasta.Executable}}'` returned `/usr/bin/pasta`. On Podman 6 the `.Host.Slirp4NetNS` field no longer exists.

Vendor documentation:
- Podman: pasta passes `--map-guest-addr 169.254.1.2` by default for `host.containers.internal`, together with `--no-map-gw`. `pasta:-T,<port>` forwards one TCP port from the container to host loopback (docs.podman.io, podman-run(1) `--network`).
- Docker Desktop: `host.docker.internal` "resolves to the internal IP address of your host". The example server binds all interfaces, so the docs do not show that a loopback-only listener is reachable (docs.docker.com, Desktop networking how-tos).
- Rootless Docker: RootlessKit has a `--disable-host-loopback` option, whose enforcement under pasta Engine 29.8.0 fixed. The fetched docs do not say what `host-gateway` reaches in rootless mode.
- Go on Windows: `syscall.EADDRINUSE` is an invented `APPLICATION_ERROR` value (Go 1.27.4 `src/syscall/zerrors_windows.go:19`, `:154`), and `Errno.Is` (`syscall_windows.go:189`) does not map Winsock's `WSAEADDRINUSE` to it.

The probes ran from the `devbox` toolbox, which shares the host network namespace. Podman was reached through `flatpak-spawn --host`. This is the same topology as Flatpak with `--share=network`.

## Goals / Non-Goals

**Goals:**
- The TUI server never listens on a wildcard address unless the user opts in explicitly, and only in a mode where the opt-in can help.
- Each engine mode has one fixed v0.17.0 behaviour: either a narrow plan that is reachable by construction, or a run without a TUI server, where the command's `invowk tui *` calls render directly and invowk prints one warning.
- Interactive container runs never fail because no narrow route exists. Non-interactive runs do not change at all.

**Non-Goals:**
- The SSH host-callback server. It defaults to `127.0.0.1` (`internal/sshserver/server.go:124-127`, `:189-191`), and `internal/app/commandadapters/host_access.go:52` uses that default. One real run confirmed that the bind is unreachable from a rootful Docker container (probe table, last row). `tests/cli/testdata/container_callback.txtar:15-29` only asserts env vars, so no test notices. The defect is tracked as N7 in `.claude/issues/cst-adoption-gaps.md` and gets its own change (decision 11).
- TLS. Sonar go:S5332 stays an accepted design choice for local IPC.
- IPv6 binds, and Windows-container or Alpine images (repository invariants).
- A reachability preflight from inside the container.

## Maintainer decisions (2026-11-14)

Items 1-6 restate the maintainer's own decisions from the brief. Items 7-14 were delegated: the maintainer said that advisor-reviewed recommendations proceed without asking. They were taken in the spec review. Items 15-21 are the advisor-reviewed adjustments of the second review; where they conflict with an earlier item, they win, and the earlier item says so.

1. Restrict the bind to loopback plus the container engine's bridge gateway. Cover Docker and Podman, rootful and rootless, including pasta/slirp4netns where `host.containers.internal` maps to the host, and Flatpak/Snap-wrapped engines.
2. Compare tokens with `crypto/subtle.ConstantTimeCompare`.
3. Authenticate `/health`, or reduce it to a non-identifying response; this design decides which (D5).
4. Add container integration tests, on `debian:stable-slim` only.
5. If a configuration cannot reach the server without a wide bind, fail safe with an explicit opt-in or a clear error. Never widen silently. *(Refined by 15: "fail safe" means no server and a degraded run, not a failed run.)*
6. Ship in v0.17.0. Sonar go:S5332 at `server.go` is resolved in SonarCloud as Accepted. Tracker N5 wording changes from "false positive" to "Accepted (real gap tracked as N5/S3)".
7. *(delegated)* Every row in D2 is fixed v0.17.0 behaviour. A later probe that would change a row becomes a new change with its own spec delta. Tasks here only record evidence.
8. *(delegated; behaviour superseded by 15)* Modes without a narrow plan come in two kinds. **Restorable** modes are those where today's wide bind may reach the server; their warning names the opt-in. For remote endpoints (row 3) the warning adds that the opt-in helps only when the engine's host alias leads back to this machine, as with a local VM or a loopback-forwarded daemon. **No-route** is only Flatpak without host networking: the sandbox's listeners live in a private network namespace, so no bind is reachable from host-side containers. Its warning says so, does not offer the opt-in, and the opt-in does not apply.
9. *(delegated; behaviour superseded by 15)* Rootful Podman is restorable, because no rootful probe was possible (no passwordless `sudo`). Under the governing rule (D2), a non-loopback bind needs a probe (Q1).
10. *(delegated; behaviour superseded by 15)* Persistent containers under rootless Podman, rootless Docker and Snap are restorable. Persistent rootless Podman would need a fixed forward port at creation. Rootless Docker and Snap have not been probed.
11. *(delegated)* The SSH host-callback defect becomes its own change after S3 and reuses D1-D2. It is tracked as N7 (decision 21).
12. *(delegated)* `/health` and unknown paths require the token (D5); no reduced unauthenticated response.
13. *(delegated; refined by 19)* The CI Podman job installs no packages, because pinned apt versions disappear from Ubuntu mirrors and the version-pinning rule has no apt exception. Instead, each CI job declares the plan row it expects as a literal in `ci.yml` (task 5.1).
14. *(delegated)* External persistent containers (`--ivk-container-name`) get the plan of their engine mode. The plan never binds wider than that mode, so an external container on a network that cannot reach the bound address sees connection failures, not wider exposure. The docs say so.
15. *(adjustment 1)* A mode without a narrow plan, restorable or no-route, does **not** fail the run. Invowk starts no TUI server (no listener at all), injects neither `INVOWK_TUI_ADDR` nor `INVOWK_TUI_TOKEN`, prints exactly one stderr warning that names the reason, and runs the command. Restorable warnings name `container.tui_server_bind: "all-interfaces"`; the Flatpak-without-network warning does not. The command's `invowk tui *` calls render directly in its own terminal (D8).
16. *(adjustment 2)* Non-interactive container runs (without `-i`) are untouched: no server, no engine facts query, no warning (D11).
17. *(adjustment 3)* `container.tui_server_bind` is read only from the config sources of `config-source-resolution` (`--ivk-config`, `INVOWK_CONFIG`, the user config file), never from an invowkfile or a module (D10).
18. *(adjustment 4)* Docker Desktop (row 6) ships loopback-only on vendor documentation. If its alias cannot reach the listener, `invowk tui *` inside the container fails within a bounded connect timeout with an error that names the opt-in, instead of hanging (D9). A macOS/Windows probe of row 6 is a release prerequisite outside this change.
19. *(adjustment 5)* The first CI run of the Podman job prints the network backend (whether `podman info` reports a pasta executable). The implementing PR sets the job's expected row literal from that observation: `11` with pasta, `12` without. If the runner has no pasta, the spec and README say the pasta plan (row 11) is verified locally only.
20. *(adjustment 6)* The SSH host-callback claim was checked with one real container run (probe table, last row). It is an existing bug with a vacuous test, tracked as N7, and S3 does not fix it.
21. The bind policy reaches the planner through the container runtime's `*config.Config`, not through the `NewInteractiveExecutor` constructor. The executor is built before any config is loaded (`cmd/invowk/app.go:144`), while the container runtime is built per request from the loaded config (`runtime_registry.go:43`). The warning goes to the request's `ctx.IO.Stderr` (`internal/runtime/runtime.go:100-101`). `InteractiveExecutor` therefore stays stateless.

## Decisions

### D1. Pick the bind plan before `Start`

A new optional port, `tuiExposurePlanner` (declared by its consumer, the interactive adapter in `internal/app/commandadapters`), returns a `containerplan.TUIExposurePlan` before the server starts. Runtimes that do not implement it (native, virtual) get the loopback plan. The container runtime implements it from its engine and its `*config.Config` (decision 21). The plan has one of three kinds:
- **narrow**: the bind addresses (`[]netip.Addr`, loopback always first), the host the container dials (an IP literal, or the engine alias for Docker Desktop), and an optional container network mode (`pasta:-T,<port>`; the port is filled in after `Start`);
- **wide** (opt-in only): the explicit wildcard bind (D4, D7), the engine alias, and the exposure warning;
- **none**: no server. It carries the warning text and whether the opt-in is offered (restorable) or not (no-route).

`interactive.go` calls the planner before `tuiserver.New`/`Start`, which moves the alias lookup now at `:66-71` ahead of `:57`. For a `none` plan it writes the warning once to `ctx.IO.Stderr` before the alternate screen starts, leaves `ctx.TUI` empty, skips the server, the bridge goroutine and `Stop`, and runs the prepared command as usual (D8). `HostServiceAddressProvider` stays for SSH.
- *Alternative:* start wide, then restrict. Rejected: that opens a window of exposure, and the plan has to exist before the port does.
- *Alternative:* fail the run before any container starts (the first review's design). Rejected by decision 15: every delegating `invowk tui *` command already renders directly when no server is advertised, so a failure would break runs that work without the overlay.

### D2. Ask the engine for its network facts; never hard-code addresses

A `container` method collects the facts behind each plan. It goes through the existing sandbox-aware engine, so Flatpak and Snap are covered:
- Docker: `docker info --format '{{.OperatingSystem}}|{{json .SecurityOptions}}'`, the endpoint (`DOCKER_HOST` when set, otherwise `docker context inspect`), and `docker network inspect bridge` for the gateway. Probed locally: `Fedora Linux …|["name=seccomp,…","name=selinux","name=cgroupns"]` and `172.17.0.1`.
- Podman: `podman info --format '{{.Host.Security.Rootless}}|{{.Host.RootlessNetworkCmd}}|{{.Host.ServiceIsRemote}}|{{.Host.Pasta.Executable}}'`. Probed: `true|pasta|false|/usr/bin/pasta`.
- Sandbox: `platform.DetectSandbox()`, and for Flatpak whether `/.flatpak-info` `[Context] shared` contains `network`.

Plan selection is a pure function in `internal/containerplan`, tested table-driven. Facts are cached per container runtime instance (one per request, so once per invowk process in practice) and queried only from the planner, so only interactive container runs pay for them.

**Governing rule.** A plan that binds a non-loopback address ships only for a mode a probe has verified (row 8). A loopback-only plan can never widen exposure, so row 6 ships on vendor documentation alone. If the documentation is wrong, the TUI is unreachable, not exposed, and the client says so (D9).

Rows are evaluated top to bottom; the first match wins. "No server (restorable)" and "no server (no-route)" are `none` plans (D1, D8).

| # | Facts | v0.17.0 behaviour | Evidence |
|---|---|---|---|
| 1 | native / virtual-sh / virtual-lua | bind `127.0.0.1`; URL `127.0.0.1` | n/a |
| 2 | Flatpak without `network` in `[Context] shared` | no server (no-route); the warning offers no opt-in | the sandbox has its own network namespace |
| 3 | remote endpoint: Docker `tcp://` or `ssh://` (context or `DOCKER_HOST`); Podman `ServiceIsRemote` = true (`podman-remote`, `podman --remote`, Podman machine) | no server (restorable); the warning says the opt-in helps only if the alias leads back to this machine | containers run on another host or in a VM |
| 4 | the facts query fails (including the Podman `docker` shim, whose `info` template differs) | no server (restorable); the warning quotes the failure | fail-safe |
| 5 | Snap sandbox | no server (restorable) | not probed (decision 10) |
| 6 | Docker, `OperatingSystem` = "Docker Desktop", local endpoint (`unix://`, `npipe://`) | bind `127.0.0.1`; URL host `host.docker.internal` | vendor docs; not probed (decision 18) |
| 7 | Docker rootless (`SecurityOptions` contains `name=rootless`) | no server (restorable) | decision 10 |
| 8 | Docker rootful, Linux, bridge gateway on a local interface | bind `127.0.0.1` + gateway; URL host = gateway IP | verified (probe row 1, twice) |
| 9 | Podman rootful | no server (restorable) | decision 9 |
| 10 | Podman rootless, persistent target | no server (restorable) | D3, decision 10 |
| 11 | Podman rootless, ephemeral, `.Host.Pasta.Executable` non-empty | bind `127.0.0.1`; network `pasta:-T,<port>`; URL `127.0.0.1` | verified on Podman 6.1.3 (probe row 4) |
| 12 | Podman rootless, no pasta binary (slirp4netns-only Podman 4.x/5.x) | no server (restorable); the warning also names pasta (`passt`) | slirp4netns cannot forward one port, and `allow_host_loopback` exposes all of host loopback (rejected like `--map-host-loopback`, D3) |
| 13 | anything else, including a gateway that is not on a local interface | no server (restorable) | fail-safe |

"Gateway on a local interface" means that `net.InterfaceAddrs()` contains the IP. This check sends several setups to row 13, because their reported gateway lives in another namespace or VM: Docker-outside-of-Docker devcontainers, Colima, Rancher Desktop, Lima, and custom `host-gateway-ip` configurations.

### D3. Rootless Podman: forward one port with `pasta:-T`

We rejected two alternatives:
- Binding the LAN address that `--map-guest-addr` lands on. That address is reachable from the LAN-facing interface, which is the exposure we are removing.
- `--map-host-loopback`. It gives the container all of host loopback, so local databases and agents become reachable from the container. That weakens the container boundary.

`-T,<port>` forwards exactly the server's port (probe row 4). Pasta needs the port when the container starts. It is known by then, because `Start` precedes `PrepareInteractive` (`interactive.go:57`, `:76`). A persistent container is created once with a fixed network, while every interactive run picks a fresh port, so persistent targets fall to row 10.

`RunOptions` gets a validated `NetworkMode` field. It is emitted as the single argument `--network=<mode>`, on `run` only, never on `exec` or `create`. One argument keeps the image scan at `podman.go:213-216` correct; as a separate argument, `pasta:-T,…` would be taken as the image.

### D4. One port on every bound address

The server listens on `127.0.0.1:0` and reads the port. It then listens on each other plan address at that port. If a later listen fails, it closes every listener opened so far and starts over, up to 5 attempts, then fails with the last error. It retries on any error, not only `EADDRINUSE`, so the code stays platform-neutral: on Windows, `errors.Is(err, syscall.EADDRINUSE)` does not match Winsock's `WSAEADDRINUSE` (see Context, vendor documentation).

The opt-in's wide plan (D7) is the one exception. It is a single listener on `0.0.0.0:0`, admitted only through an explicit wildcard plan kind. It cannot be combined with other addresses, since `0.0.0.0` and `127.0.0.1` cannot share a port. `New` rejects an unspecified address in any other plan.

All listens complete before any serve goroutine starts. One `http.Server` serves every listener, and each `Serve` goroutine is tracked with `AddGoroutine`/`DoneGoroutine`, as at `server.go:123-135`. The server transitions to Running exactly once, after every goroutine is launched. A `Serve` error on any listener fails the server and shuts down the others, which keeps the single-listener semantics at `:128-134`. `Stop` closes every listener.
- *Alternative:* a different port per address. Rejected: the container gets one URL, and per-address URLs add plumbing for no gain.

### D5. Authenticate every request, first, in constant time

The auth middleware wraps the whole mux, so `/tui`, `/health` and unknown paths all require the token. It runs before the method check and compares `subtle.ConstantTimeCompare([]byte(header), []byte("Bearer "+token))`. The only length it leaks is the fixed token length. A failure gets `401` with the body `unauthorized`.

We chose this over a non-identifying `204` on `/health`. A TCP handshake already shows that the port is open, so hiding liveness is not the goal. The goal is that a request without the token gets no application response at all, not even the mux's default `404`. Nothing in production calls `/health`, so the change breaks no shipped binary. `TestServerMethodNotAllowed` is updated to send the token.

### D6. The URL carries the bound IP

For bridge and pasta plans (rows 8 and 11), the container dials an IP literal that the server listens on, not the alias. Reachability then does not depend on how an engine version or daemon config resolves `host-gateway`. It also keeps working for persistent Docker containers created with the old mapping. Docker Desktop (row 6) keeps `host.docker.internal`, because only the engine knows where the alias leads, and the server binds loopback only there. The opt-in keeps today's alias. `HostServiceAddress()` keeps serving SSH unchanged.

### D7. Opt-in config, with a warning

Add `container.tui_server_bind: *"restricted" | "all-interfaces"` to `#ContainerConfig`, with a typed Go value type, the JSON tag, `GenerateCUE` output and the schema sync tests.

`"all-interfaces"` applies to restorable `none` plans and to Docker Desktop (row 6). There it binds `0.0.0.0` through the wildcard plan (D4) and uses the engine alias (today's behaviour), and invowk prints a one-line stderr warning at each such run. Row 6 is included because it is the only plan that ships unprobed: if its loopback assumption is wrong, the opt-in is the user's remedy. The verified plans (rows 8 and 11) keep their narrow bind, host runtimes always bind loopback, and Flatpak without network (row 2) stays a no-server run. There is no CLI flag or environment variable; `INVOWK_CONFIG` already selects a config file.

### D8. No-server runs render directly

A `none` plan runs the command through the same interactive path, minus the server. Because `ctx.TUI` stays empty, the container gets neither `INVOWK_TUI_*` variable nor the host-gateway mapping (`runtime.go:530-536`, `container_exec.go:189-194`). Each delegating `invowk tui *` call in the command finds no server (`client.go:117`) and renders in the command's own terminal (`cmd/invowk/tui_confirm.go:66-80`), which the interactive adapter shows in its PTY view. The overlay rendering of the parent TUI is the only thing lost.

The warning is one line, written once per run, before the alternate screen starts, so the alternate screen cannot hide it. Its text names the mode's reason; restorable modes add `container.tui_server_bind: "all-interfaces"` (and, for row 3, the alias caveat; for row 12, pasta), and row 2 says the sandbox's network is private without offering the opt-in.
- *Alternative:* fall back to a non-interactive run. Rejected: the user asked for `-i`, and the PTY view still serves the command's own prompts.

### D9. The client bounds the connect, not the interaction

`tuiclient` gets a dial timeout of 5 seconds on its transport. The 10-minute overall timeout stays, because a component request waits for the user. A dial failure (refused, timed out or unroutable) becomes `ErrTUIServerUnreachable`. Its message names the address, says that a container engine may not route to the host, and names `container.tui_server_bind: "all-interfaces"`. Under Docker Desktop (row 6), if the alias does not reach a loopback-only listener, `invowk tui *` in the container therefore fails within seconds with an actionable error.

### D10. The opt-in is user configuration only

The planner reads `container.tui_server_bind` from the container runtime's `*config.Config` (decision 21), which only the `config-source-resolution` sources produce. Neither the invowkfile nor the module schema gains the key, and both are closed, so a repository or module that sets it fails validation instead of widening the bind. A test pins both halves: the config key changes the plan, and the same key in an invowkfile's container runtime block is rejected.

### D11. Non-interactive runs are untouched

The planner runs only inside the interactive adapter, and the adapter runs only for interactive requests (`dispatch.go:110-113`). Engine facts are queried lazily by the planner, never at construction (`NewInteractiveExecutor`, `NewContainerRuntime`). A non-interactive container run therefore starts no server, queries no facts and prints no warning. A test pins this with a spy facts provider that must see zero calls.

## Risks / Trade-offs

- [Other containers on the same bridge can reach `<gateway>:<port>`] → They still need the 256-bit token, and the port lives only for the interactive run. Documented as residual exposure.
- [On-link LAN hosts can route to the bridge gateway] → Linux accepts packets for any local address on any interface unless a firewall drops them. A host on the same L2 segment that adds a route to `172.17.0.0/16` through this machine can therefore reach `<gateway>:<port>`. That is far narrower than `0.0.0.0`, and the token still guards it. Documented.
- [Local users on the host can reach loopback] → Same as today; the token guards it.
- [Modes without a narrow plan lose the overlay rendering] → The run still works and `invowk tui *` renders directly (D8). The warning names the opt-in; for rootless Podman it also names pasta. Q1 tracks persistent rootless Podman.
- [A user misses the warning] → It is printed before the alternate screen, once per run, to stderr. The command's behaviour does not depend on it.
- [Docker Desktop's alias may not reach a loopback-only listener] → Then the TUI is unreachable, not exposed. The client fails within the dial timeout and names the opt-in (D9), and `"all-interfaces"` restores today's behaviour. Most macOS and Windows users are on this row, so a probe there is a release prerequisite outside this change.
- [Docker Desktop with WSL2 integration] → invowk runs in a WSL distribution while the alias targets the Windows host. The row 6 caveat applies.
- [The engine facts query adds latency to every interactive container run] → One to three engine CLI calls, only for `-i` container runs, cached per runtime instance.
- [A host firewall blocks bridge-to-host traffic to the gateway port] → The run's `invowk tui *` calls fail within the dial timeout, not exposed. The docs name the firewall as a possible cause.

## Migration Plan

Ship in v0.17.0. Roll back by reverting the change. The closed schema would then reject the new config key as unknown, so the release notes tell users of `"all-interfaces"` to remove it before they downgrade.

## Open Questions

- **Q1.** Persistent containers under rootless Podman, and rootful Podman. Persistent rootless Podman could reserve a fixed forward port at creation (a container label plus `pasta:-T,<P>`), with the server bound to `127.0.0.1:<P>`. Rootful Podman needs a probe, either of pasta or of its bridge gateway. Both stay no-server runs in v0.17.0 (decisions 9-10, 15). A follow-up would modify this capability and `persistent-container-runtime`. The exec-time rule there (`spec.md:109-111`) needs no MODIFIED delta in S3.
- **Q2.** Rootless Docker: which host address does `host-gateway` reach under RootlessKit, and can a narrow bind serve it? It is a no-server run in v0.17.0; a later change with probe evidence may revisit it (decision 7).
- **Q3.** Snap: do the snap's listeners share the host network namespace with the engine's containers? It is a no-server run in v0.17.0; a later change with probe evidence may revisit it (decision 7).
