# Spec Delta

## RENAMED Requirements

- FROM: `### Requirement: Interactive runs in modes without a narrow route fail before start`
- TO: `### Requirement: Interactive runs in modes without a narrow route run without a TUI server`

- FROM: `### Requirement: Flatpak without host networking refuses interactive container runs without offering the opt-in`
- TO: `### Requirement: Flatpak without host networking runs without a TUI server and without offering the opt-in`

- FROM: `### Requirement: An unreachable server fails fast with an actionable error`
- TO: `### Requirement: An unreachable server falls back to direct rendering within a short bound`

## MODIFIED Requirements

### Requirement: Interactive runs in modes without a narrow route run without a TUI server
For an engine mode without a narrow plan, unless the opt-in applies, an interactive container run SHALL NOT fail for that reason. invowk SHALL open no TUI server listener, SHALL inject neither `INVOWK_TUI_ADDR` nor `INVOWK_TUI_TOKEN`, SHALL print exactly one stderr warning that names the reason and `container.tui_server_bind: "all-interfaces"`, and SHALL run the command interactively.

#### Scenario: Modes covered
- **WHEN** the engine mode is a remote engine, rootful Podman, persistent rootless Podman, rootless Podman without pasta, rootless Docker, Snap, a failed engine query or any unlisted mode, and the opt-in is not set
- **THEN** the interactive container run SHALL follow this requirement

#### Scenario: Persistent container under rootless Podman
- **WHEN** a user runs an interactive command that targets a persistent container under rootless Podman, without the opt-in
- **THEN** the command SHALL run, invowk's exit code SHALL be the command's, the container SHALL see no `INVOWK_TUI_ADDR` or `INVOWK_TUI_TOKEN`, and stderr SHALL contain the warning naming `container.tui_server_bind` exactly once

#### Scenario: Captured prompt without a server
- **WHEN** the command's script runs `name=$(invowk tui input)` and the user types `abc` and presses Enter
- **THEN** the prompt SHALL be visible in the interactive view, and `$name` SHALL be exactly `abc`

#### Scenario: Rootless Podman without pasta
- **WHEN** an interactive container command runs under rootless Podman, no pasta binary is available, and the opt-in is not set
- **THEN** the command SHALL run without a TUI server, and the warning SHALL name `container.tui_server_bind` and pasta

#### Scenario: Remote engine
- **WHEN** the selected engine is remote (a Docker endpoint that is not empty, `unix://`, `npipe://` or `fd://`, which includes `tcp://`, `ssh://` and a scheme-less `host:port`; `podman-remote`, `podman --remote` or Podman machine) and the opt-in is not set
- **THEN** the command SHALL run without a TUI server, and the warning SHALL name `container.tui_server_bind` and say the opt-in helps only if the engine's host alias leads back to this machine

#### Scenario: The same command without -i
- **WHEN** the user runs the same command in the same engine mode without `-i`
- **THEN** the command SHALL run non-interactively, with no TUI exposure warning

#### Scenario: Refusal precedes dependency checks
- **WHEN** such a command declares container-scoped `depends_on` checks, runs with `-i`, and its bind plan is invalid
- **THEN** the run SHALL fail before any dependency check runs; with a valid no-server plan, the checks SHALL run and the warning SHALL print once

#### Scenario: Interactive mode from the config default
- **WHEN** interactive mode comes only from `ui.interactive: true` in the config, and the mode has no narrow route
- **THEN** invowk SHALL behave as for an explicit `-i`: it SHALL run the command interactively without a TUI server and print the same single warning

### Requirement: Flatpak without host networking runs without a TUI server and without offering the opt-in
When invowk runs in Flatpak without host networking, an interactive container run SHALL run without a TUI server, whatever `container.tui_server_bind` is set to. invowk SHALL inject no `INVOWK_TUI_*` variables and SHALL print exactly one stderr warning that says the sandbox's network is private, without suggesting the opt-in.

#### Scenario: Flatpak without network
- **WHEN** invowk runs in a Flatpak whose `[Context] shared` lacks `network` and an interactive container command runs
- **THEN** the command SHALL run without a TUI server, and the warning SHALL NOT name `all-interfaces`

#### Scenario: Flatpak without network, with the opt-in
- **WHEN** the same run has `container.tui_server_bind` set to `"all-interfaces"`
- **THEN** invowk SHALL still open no listener, and SHALL NOT bind a wildcard address

### Requirement: An unreachable server falls back to direct rendering within a short bound
When `INVOWK_TUI_ADDR` is set but the server is unreachable, a delegating `invowk tui` command SHALL decide within a 2-second reach bound. It SHALL print one stderr warning that names the address and `container.tui_server_bind`, and SHALL then render directly. A `401` or the caller's own cancellation SHALL stay an error and SHALL NOT fall back.

#### Scenario: Container cannot reach a loopback-only listener
- **WHEN** a delegating `invowk tui input` runs with `INVOWK_TUI_ADDR` pointing at an address that refuses or drops the connection
- **THEN** within the reach bound it SHALL print the warning naming the address and `container.tui_server_bind`, and SHALL render on the terminal

#### Scenario: A proxy accepts the connection but cannot reach the host
- **WHEN** the peer at `INVOWK_TUI_ADDR` accepts the TCP connection and then resets it, closes it or never answers
- **THEN** the command SHALL fall back to direct rendering within the reach bound, with the same warning

#### Scenario: Rejected token
- **WHEN** the health check gets `401`
- **THEN** the command SHALL fail with a server error and SHALL NOT render directly

### Requirement: The client checks reachability before each component request
Before each component request, the TUI client SHALL send an authenticated `GET /health` bounded by the reach timeout. A refused, dropped, reset, closed or silent connection, or an answer other than `200` or `401`, SHALL count as unreachable. A `401` SHALL stay a server error, and the caller's own cancellation SHALL NOT be reported as unreachable. The client SHALL NOT send TUI requests through an HTTP proxy from the environment.

#### Scenario: Rejected token
- **WHEN** the health check gets `401`
- **THEN** the command SHALL fail with a server error that is not the unreachable error

#### Scenario: Proxy in the container environment
- **WHEN** `HTTP_PROXY` is set in the environment of a delegating `invowk tui` command
- **THEN** the request and its bearer token SHALL go directly to `INVOWK_TUI_ADDR`

#### Scenario: Reach bound
- **WHEN** the server at `INVOWK_TUI_ADDR` never answers the health check
- **THEN** the client SHALL treat it as unreachable within 2 seconds

## REMOVED Requirements

### Requirement: No fallback to a run without a TUI server
**Reason**: Its premise was that a delegating `invowk tui` component whose value a script captures renders into the capture. `tui-component-terminal-io` removes that defect, and the gate test showed a byte-exact captured value in a no-server `-i` session.
**Migration**: None for users. Modes without a narrow route now run without a server (see "Interactive runs in modes without a narrow route run without a TUI server").
