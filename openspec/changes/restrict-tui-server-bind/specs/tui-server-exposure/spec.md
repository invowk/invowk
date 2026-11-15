# Spec Delta

## Purpose

Bounds the network exposure of the TUI server that interactive commands start. It defines which addresses the server binds for each runtime and container engine mode, how containers are pointed at it, how invowk refuses an interactive run when no narrow route exists, the explicit opt-in for a wide bind and where it may be set, and how every request authenticates.

## ADDED Requirements

### Requirement: Loopback-only bind for host runtimes
When an interactive command runs with the native, virtual-sh or virtual-lua runtime, the TUI server SHALL listen only on `127.0.0.1`, and `INVOWK_TUI_ADDR` SHALL be `http://127.0.0.1:<port>`.

#### Scenario: Native interactive run
- **WHEN** a user runs an interactive command with the native runtime
- **THEN** the server SHALL accept connections on `127.0.0.1`, and a connection to the same port on any non-loopback host address SHALL be refused

#### Scenario: Virtual interactive run
- **WHEN** a user runs an interactive command with virtual-sh or virtual-lua
- **THEN** `INVOWK_TUI_ADDR` SHALL be `http://127.0.0.1:<port>` and the server SHALL bind no other address

### Requirement: Rootful Docker binds loopback and the bridge gateway
Under local rootful Docker on Linux whose default bridge gateway is assigned to a local interface, the TUI server SHALL listen on `127.0.0.1` and on that gateway, at one port, and on no other address. `INVOWK_TUI_ADDR` SHALL be `http://<gateway-ip>:<port>`.

#### Scenario: Default bridge
- **WHEN** an interactive container command runs under local rootful Docker on Linux
- **THEN** the server SHALL listen on `127.0.0.1` and the bridge gateway only, and an authenticated `/health` request from the container to `INVOWK_TUI_ADDR` SHALL get `200`

#### Scenario: Gateway is not a local address
- **WHEN** the bridge gateway that Docker reports is not assigned to any local interface, as in a devcontainer that uses the host's Docker socket
- **THEN** the interactive run SHALL fail before any container starts, with an error that names `container.tui_server_bind`

### Requirement: Rootless Podman uses a single forwarded port
For an ephemeral container under local rootless Podman with a pasta binary, the TUI server SHALL listen on `127.0.0.1` only, and the container SHALL run with `--network=pasta:-T,<port>`. `INVOWK_TUI_ADDR` SHALL be `http://127.0.0.1:<port>`, and the container SHALL gain no other route to host loopback. This plan is verified on a local rootless Podman 6 host and in the CI Podman job, whose runner reports a pasta executable.

#### Scenario: Ephemeral rootless Podman run
- **WHEN** an interactive container command runs a new container under rootless Podman with pasta
- **THEN** a process in the container SHALL reach the server at `127.0.0.1:<port>` and SHALL NOT reach any other port bound on host loopback

### Requirement: Docker Desktop binds loopback only
Under a local Docker Desktop engine without the opt-in, the TUI server SHALL listen on `127.0.0.1` only, and `INVOWK_TUI_ADDR` SHALL use the engine alias `host.docker.internal`, which the engine forwards to the host.

#### Scenario: Docker Desktop run
- **WHEN** an interactive container command runs under a local Docker Desktop engine, without the opt-in
- **THEN** the server SHALL listen on `127.0.0.1` only, and `INVOWK_TUI_ADDR` SHALL be `http://host.docker.internal:<port>`

### Requirement: Interactive runs in modes without a narrow route fail before start
For an engine mode without a narrow plan, unless the opt-in applies, an interactive container run SHALL fail before invowk provisions, creates or starts the command's container and before it opens any TUI server listener. invowk SHALL exit with status 1. The error SHALL name the reason, `container.tui_server_bind: "all-interfaces"`, and that the command can run without `-i`.

#### Scenario: Modes covered
- **WHEN** the engine mode is a remote engine, rootful Podman, persistent rootless Podman, rootless Podman without pasta, rootless Docker, Snap, a failed engine query or any unlisted mode, and the opt-in is not set
- **THEN** the interactive container run SHALL follow this requirement

#### Scenario: Persistent container under rootless Podman
- **WHEN** a user runs an interactive command that targets a persistent container under rootless Podman, without the opt-in
- **THEN** invowk SHALL exit with status 1 before it provisions, creates or starts the container, SHALL open no TUI server listener, and stderr SHALL contain an error naming `container.tui_server_bind`

#### Scenario: Rootless Podman without pasta
- **WHEN** an interactive container command runs under rootless Podman, no pasta binary is available, and the opt-in is not set
- **THEN** the run SHALL fail before start, and the error SHALL name `container.tui_server_bind` and pasta

#### Scenario: Remote engine
- **WHEN** the selected engine is remote (a `tcp://` or `ssh://` Docker endpoint, `podman-remote`, `podman --remote` or Podman machine) and the opt-in is not set
- **THEN** the run SHALL fail before start, and the error SHALL name `container.tui_server_bind` and say the opt-in helps only if the engine's host alias leads back to this machine

#### Scenario: The same command without -i
- **WHEN** the user runs the same command in the same engine mode without `-i`
- **THEN** the command SHALL run as before this change

#### Scenario: Refusal precedes dependency checks
- **WHEN** such a command declares container-scoped `depends_on` checks and runs with `-i`
- **THEN** the run SHALL fail before any dependency check runs

#### Scenario: Interactive mode from the config default
- **WHEN** interactive mode comes only from `ui.interactive: true` in the config, without `-i` or `--ivk-interactive` on the command line, and the mode has no narrow route
- **THEN** invowk SHALL run the command non-interactively, print one warning that names the reason and `--ivk-interactive=false`, and SHALL NOT fail the run

### Requirement: No fallback to a run without a TUI server
invowk SHALL NOT run an interactive container command without a TUI server in place of a narrow or opted-in plan. A delegating `invowk tui` component whose value a script captures renders into the capture instead of the terminal when no server is advertised, so the script would continue with a corrupted value.

#### Scenario: Captured prompt in a mode without a narrow route
- **WHEN** an interactive container command whose script runs `name=$(invowk tui input)` is started in a mode without a narrow route and without the opt-in
- **THEN** the run SHALL fail before start, and the script SHALL NOT run

### Requirement: Flatpak without host networking refuses interactive container runs without offering the opt-in
When invowk runs in Flatpak without host networking, an interactive container run SHALL fail before start, whatever `container.tui_server_bind` is set to. invowk SHALL exit with status 1. The error SHALL say that the sandbox's network is private, and SHALL NOT suggest the opt-in.

#### Scenario: Flatpak without network
- **WHEN** invowk runs in a Flatpak whose `[Context] shared` lacks `network` and an interactive container command runs
- **THEN** the run SHALL fail before any container starts, and the error SHALL NOT name `all-interfaces`

#### Scenario: Flatpak without network, with the opt-in
- **WHEN** the same run has `container.tui_server_bind` set to `"all-interfaces"`
- **THEN** the run SHALL still fail before start, and invowk SHALL NOT bind a wildcard address

### Requirement: An invalid bind plan is refused before any listener opens
Before the TUI server starts, invowk SHALL check the run's bind plan. A narrow plan SHALL list `127.0.0.1` first and only specified IPv4 addresses. A wildcard plan SHALL appear only for a mode the opt-in applies to. A refused plan SHALL carry no address. If the plan breaks any of these rules, the interactive run SHALL fail with status 1 before any listener opens and before any container starts.

#### Scenario: Unspecified address in a narrow plan
- **WHEN** a narrow plan lists `0.0.0.0`
- **THEN** the interactive run SHALL fail before any listener opens

#### Scenario: Wildcard plan outside the opt-in
- **WHEN** a wildcard plan is produced for a host runtime, a verified narrow mode or Flatpak without network
- **THEN** the interactive run SHALL fail before any listener opens

### Requirement: Non-interactive runs are unchanged
A container run without `-i` SHALL start no TUI server, SHALL NOT query the engine's network facts, and SHALL print no TUI exposure warning or error.

#### Scenario: Non-interactive container run
- **WHEN** a user runs a container command without `-i`, in any engine mode
- **THEN** no TUI server SHALL start, no engine network facts SHALL be queried, and stderr SHALL contain no TUI exposure warning or error

### Requirement: Wide bind requires explicit opt-in
The config key `container.tui_server_bind` SHALL accept `"restricted"` (default) and `"all-interfaces"`. Only `"all-interfaces"` SHALL make the server listen on `0.0.0.0`, and only for a restorable mode or Docker Desktop. Then `INVOWK_TUI_ADDR` SHALL use the engine alias, and each such run SHALL print a stderr warning that the port is reachable from the network.

#### Scenario: Default configuration
- **WHEN** `config.cue` does not set `container.tui_server_bind`
- **THEN** the restricted bind rules SHALL apply

#### Scenario: Opt-in in a restorable mode
- **WHEN** `container.tui_server_bind` is `"all-interfaces"` and an interactive command runs a persistent container under rootless Podman
- **THEN** the server SHALL listen on `0.0.0.0`, `INVOWK_TUI_ADDR` SHALL use `host.containers.internal`, and invowk SHALL print the exposure warning to stderr

#### Scenario: Opt-in under Docker Desktop
- **WHEN** `container.tui_server_bind` is `"all-interfaces"` and an interactive container command runs under a local Docker Desktop engine
- **THEN** the server SHALL listen on `0.0.0.0`, `INVOWK_TUI_ADDR` SHALL use `host.docker.internal`, and invowk SHALL print the exposure warning to stderr

#### Scenario: Opt-in in a verified narrow mode
- **WHEN** `container.tui_server_bind` is `"all-interfaces"` and an interactive container command runs under local rootful Docker on Linux
- **THEN** the server SHALL listen on `127.0.0.1` and the bridge gateway only, and SHALL print no exposure warning

#### Scenario: Opt-in ignored for host runtimes
- **WHEN** `container.tui_server_bind` is `"all-interfaces"` and an interactive command runs with the native runtime
- **THEN** the server SHALL still listen on `127.0.0.1` only

### Requirement: The opt-in is read only from user configuration
invowk SHALL read `container.tui_server_bind` only from the config source that the `config-source-resolution` capability selects: the `--ivk-config` flag, the `INVOWK_CONFIG` environment variable or the per-user config file. When `--ivk-config` or `INVOWK_CONFIG` selects a file, invowk SHALL read the key only from that file and SHALL NOT read it from the per-user config file. An invowkfile or a module SHALL NOT be able to set it.

#### Scenario: Opt-in in the selected config source
- **WHEN** the config file selected by `INVOWK_CONFIG` sets `container.tui_server_bind: "all-interfaces"`
- **THEN** invowk SHALL apply the opt-in to the interactive container runs it selects

#### Scenario: Opt-in in the --ivk-config file
- **WHEN** the file named by `--ivk-config` sets `container.tui_server_bind: "all-interfaces"`
- **THEN** invowk SHALL apply the opt-in to the interactive container runs it selects

#### Scenario: User config file ignored under an explicit source
- **WHEN** the per-user config file sets `container.tui_server_bind: "all-interfaces"` and `INVOWK_CONFIG` or `--ivk-config` selects a file that does not set the key
- **THEN** the restricted bind rules SHALL apply

#### Scenario: Opt-in in an invowkfile
- **WHEN** an invowkfile or a module's invowkfile sets `tui_server_bind` in a container runtime block
- **THEN** validation SHALL reject the invowkfile, and no run SHALL bind a wildcard address because of it

### Requirement: An unreachable server fails fast with an actionable error
When `INVOWK_TUI_ADDR` is set but the server cannot be reached, a delegating `invowk tui` command SHALL fail within a bounded connect timeout, not hang for the interaction timeout. Its error SHALL name the address and `container.tui_server_bind`. It SHALL NOT fall back to rendering directly, for the reason given in "No fallback to a run without a TUI server".

#### Scenario: Container cannot reach a loopback-only listener
- **WHEN** a delegating `invowk tui` command runs with `INVOWK_TUI_ADDR` pointing at an address that refuses or drops the connection, as when a Docker Desktop alias does not reach a loopback-only listener
- **THEN** the command SHALL exit non-zero within the connect timeout, with an error that names the address and `container.tui_server_bind`

### Requirement: Every request requires the session token
The TUI server SHALL require `Authorization: Bearer <session token>` on every request, whatever the path, and SHALL check it before the method or body. It SHALL compare in constant time. A missing or wrong token SHALL get `401` with the body `unauthorized`.

#### Scenario: Unauthenticated health probe
- **WHEN** a client sends `GET /health` without the session token
- **THEN** the server SHALL answer `401` with the body `unauthorized`

#### Scenario: Authenticated health probe
- **WHEN** the in-tree TUI client checks availability with the session token
- **THEN** the server SHALL answer `200`

#### Scenario: Wrong method without token
- **WHEN** a client sends `GET /tui` without the token
- **THEN** the server SHALL answer `401`, not `405`

#### Scenario: Unknown path without token
- **WHEN** a client sends `GET /` or any unregistered path without the token
- **THEN** the server SHALL answer `401`, not `404`
