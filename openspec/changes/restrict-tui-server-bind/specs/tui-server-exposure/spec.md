# Spec Delta

## Purpose

Bounds the network exposure of the TUI server that interactive commands start. It defines which addresses the server binds for each runtime and container engine mode, how containers are pointed at it, how invowk fails safe when no narrow route exists, the explicit opt-in for a wide bind, and how every request authenticates.

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
- **THEN** invowk SHALL fail the run before it starts a container, with an error that names `container.tui_server_bind`

### Requirement: Rootless Podman uses a single forwarded port
For an ephemeral container under local rootless Podman with a pasta binary, the TUI server SHALL listen on `127.0.0.1` only, and the container SHALL run with `--network=pasta:-T,<port>`. `INVOWK_TUI_ADDR` SHALL be `http://127.0.0.1:<port>`, and the container SHALL gain no other route to host loopback.

#### Scenario: Ephemeral rootless Podman run
- **WHEN** an interactive container command runs a new container under rootless Podman with pasta
- **THEN** a process in the container SHALL reach the server at `127.0.0.1:<port>` and SHALL NOT reach any other port bound on host loopback

### Requirement: Docker Desktop binds loopback only
Under a local Docker Desktop engine, the TUI server SHALL listen on `127.0.0.1` only, and `INVOWK_TUI_ADDR` SHALL use the engine alias `host.docker.internal`, which the engine forwards to the host.

#### Scenario: Docker Desktop run
- **WHEN** an interactive container command runs under a local Docker Desktop engine
- **THEN** the server SHALL listen on `127.0.0.1` only, and `INVOWK_TUI_ADDR` SHALL be `http://host.docker.internal:<port>`

### Requirement: Restorable modes fail before start
For a local engine mode without a narrow plan, an interactive container run SHALL fail before it creates or starts a container, unless the opt-in is set. The error SHALL name the reason and `container.tui_server_bind`. These modes are rootful Podman, persistent rootless Podman, rootless Podman without pasta, rootless Docker, Snap, a failed engine query, and any unlisted mode.

#### Scenario: Persistent container under rootless Podman
- **WHEN** a user runs an interactive command that targets a persistent container under rootless Podman, without the opt-in
- **THEN** invowk SHALL exit non-zero with an error that names `container.tui_server_bind`, and SHALL NOT create, start or exec into any container

#### Scenario: Rootless Podman without pasta
- **WHEN** an interactive container command runs under rootless Podman, no pasta binary is available, and the opt-in is not set
- **THEN** the run SHALL fail before it starts a container, with an error that names `container.tui_server_bind` and pasta

### Requirement: No-route modes fail without offering the opt-in
When the engine is remote, or invowk runs in Flatpak without host networking, an interactive container run SHALL fail before it starts a container, whatever `container.tui_server_bind` is set to. The error SHALL say that the host's TUI server cannot be reached from the engine's containers, and SHALL NOT suggest the opt-in.

#### Scenario: Remote engine
- **WHEN** the selected engine is remote (a `tcp://` or `ssh://` Docker endpoint, `podman-remote`, `podman --remote` or Podman machine)
- **THEN** the run SHALL fail before it starts a container, and the error SHALL NOT name `all-interfaces`

#### Scenario: Remote engine with the opt-in
- **WHEN** the engine is remote and `container.tui_server_bind` is `"all-interfaces"`
- **THEN** the run SHALL still fail, and the server SHALL NOT bind a wildcard address

### Requirement: Wide bind requires explicit opt-in
The config key `container.tui_server_bind` SHALL accept `"restricted"` (default) and `"all-interfaces"`. Only `"all-interfaces"` SHALL make the server listen on every IPv4 interface, and only for a restorable mode. Then `INVOWK_TUI_ADDR` SHALL use the engine alias, and each such run SHALL print a stderr warning that the port is reachable from the network.

#### Scenario: Default configuration
- **WHEN** `config.cue` does not set `container.tui_server_bind`
- **THEN** the restricted bind rules SHALL apply

#### Scenario: Opt-in in a restorable mode
- **WHEN** `container.tui_server_bind` is `"all-interfaces"` and an interactive command runs a persistent container under rootless Podman
- **THEN** the server SHALL listen on `0.0.0.0`, and invowk SHALL print the exposure warning to stderr

#### Scenario: Opt-in in a mode with a narrow plan
- **WHEN** `container.tui_server_bind` is `"all-interfaces"` and an interactive container command runs under local rootful Docker on Linux
- **THEN** the server SHALL listen on `127.0.0.1` and the bridge gateway only, and SHALL print no exposure warning

#### Scenario: Opt-in ignored for host runtimes
- **WHEN** `container.tui_server_bind` is `"all-interfaces"` and an interactive command runs with the native runtime
- **THEN** the server SHALL still listen on `127.0.0.1` only

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
