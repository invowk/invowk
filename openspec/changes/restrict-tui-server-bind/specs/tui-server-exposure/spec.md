# Spec Delta

## Purpose

Bounds the network exposure of the TUI server that interactive commands start. It defines which addresses the server binds for each runtime and container engine, how containers are pointed at it, how invowk fails safe when no narrow route exists, and how every endpoint authenticates.

## ADDED Requirements

### Requirement: Loopback-only bind for host runtimes
When an interactive command runs with the native, virtual-sh or virtual-lua runtime, the TUI server SHALL listen only on the IPv4 loopback address, and `INVOWK_TUI_ADDR` SHALL name that address.

#### Scenario: Native interactive run
- **WHEN** a user runs an interactive command with the native runtime
- **THEN** the TUI server SHALL accept connections on `127.0.0.1` only, and a connection to the same port on any non-loopback host address SHALL be refused

#### Scenario: Virtual interactive run
- **WHEN** a user runs an interactive command with virtual-sh or virtual-lua
- **THEN** `INVOWK_TUI_ADDR` SHALL be `http://127.0.0.1:<port>` and the server SHALL bind no other address

### Requirement: Container bind follows the engine's host route
For a container run, the TUI server SHALL listen on loopback plus at most one more address: the local host address the engine routes container traffic to. It SHALL bind a non-loopback address only for an engine mode whose reachability has been verified. It SHALL never bind a wildcard address without the opt-in, and SHALL use one port on every address it binds.

#### Scenario: Rootful Docker on Linux
- **WHEN** an interactive container command runs with local rootful Docker on Linux on the default bridge network
- **THEN** the server SHALL listen on `127.0.0.1` and on that bridge's gateway address, and on no other address

#### Scenario: Unverified mode that needs a bridge bind
- **WHEN** an interactive container command runs with an engine mode that would need a non-loopback bind and is not verified (for example, rootful Podman before its probe)
- **THEN** invowk SHALL treat the configuration as unroutable

#### Scenario: Docker Desktop
- **WHEN** the engine is a local Docker Desktop, which forwards the host alias to host loopback
- **THEN** the server SHALL listen on `127.0.0.1` only

#### Scenario: Gateway is not a local address
- **WHEN** the gateway the engine reports is not assigned to any local interface
- **THEN** invowk SHALL treat the configuration as unroutable

### Requirement: Rootless Podman uses a single forwarded port
For an ephemeral container under rootless Podman with a pasta binary available, the TUI server SHALL listen on loopback only. The container SHALL forward only the server's port from its own loopback to host loopback, and SHALL gain no other route to host loopback.

#### Scenario: Ephemeral rootless Podman run
- **WHEN** an interactive container command runs a new container under rootless Podman with pasta
- **THEN** a process in the container SHALL reach the server at `127.0.0.1:<port>` and SHALL NOT reach any other port bound on host loopback

### Requirement: Container address names the bound address
`INVOWK_TUI_ADDR` inside a container SHALL carry an IP literal and port the server actually listens on and the container can reach, not a hostname the engine might resolve to a different address.

#### Scenario: Rootful Docker address
- **WHEN** an interactive container command runs under rootful Docker on Linux
- **THEN** `INVOWK_TUI_ADDR` SHALL be `http://<bridge-gateway-ip>:<port>`, and an authenticated `/health` request to it from the container SHALL succeed

### Requirement: Unroutable configurations fail before start
When no narrow bind can reach the container, an interactive container run SHALL fail before it creates or starts a container unless the user opted in. The error SHALL name the reason and the opt-in. Unroutable cases include remote engines, Snap, Flatpak without host networking, and unverified modes that need a non-loopback bind.

#### Scenario: Persistent container under rootless Podman
- **WHEN** a user runs an interactive command that targets a persistent container under rootless Podman, without the opt-in
- **THEN** invowk SHALL exit non-zero before it runs anything in the container, with an error that names `container.tui_server_bind`

#### Scenario: Rootless Podman without pasta
- **WHEN** an interactive container command runs under rootless Podman, no pasta binary is available, and there is no opt-in
- **THEN** the run SHALL fail before it starts the container, with an error that names `container.tui_server_bind` and pasta

#### Scenario: Remote engine
- **WHEN** the selected engine is remote (for example `DOCKER_HOST=tcp://…`, `podman --remote` or Podman machine) and there is no opt-in
- **THEN** the run SHALL fail before it starts the container, with the same actionable error

### Requirement: Wide bind requires explicit opt-in
The config key `container.tui_server_bind` SHALL accept `"restricted"` (default) and `"all-interfaces"`. Only `"all-interfaces"` SHALL make the server listen on every interface. With it, each interactive container run SHALL print a warning to stderr that the port is reachable from the network.

#### Scenario: Default configuration
- **WHEN** `config.cue` does not set `container.tui_server_bind`
- **THEN** the restricted bind rules SHALL apply

#### Scenario: Opt-in set
- **WHEN** `container.tui_server_bind` is `"all-interfaces"` and an interactive container command runs
- **THEN** the server SHALL listen on all IPv4 interfaces and invowk SHALL print the exposure warning to stderr

#### Scenario: Opt-in ignored for host runtimes
- **WHEN** `container.tui_server_bind` is `"all-interfaces"` and an interactive command runs with the native runtime
- **THEN** the server SHALL still listen on loopback only

### Requirement: Every endpoint requires the session token
Every TUI server endpoint, including `/health`, SHALL require `Authorization: Bearer <session token>` and SHALL check it before it inspects the method or body. It SHALL compare the token in constant time. A missing or wrong token SHALL get `401` with a body that does not identify invowk.

#### Scenario: Unauthenticated health probe
- **WHEN** a client sends `GET /health` without the session token
- **THEN** the server SHALL answer `401`, and the body SHALL not contain "ok" or the word "invowk"

#### Scenario: Authenticated health probe
- **WHEN** the in-tree TUI client checks availability with the session token
- **THEN** the server SHALL answer `200`

#### Scenario: Wrong method without token
- **WHEN** a client sends `GET /tui` without the token
- **THEN** the server SHALL answer `401`, not `405`
