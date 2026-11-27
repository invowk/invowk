# Spec Delta

## Purpose

Defines how Invowk isolates the OSC 7577 Program Resume Protocol secret (`OSC7577_TOKEN`) across execution boundaries, preserves transparent pass-through for non-interactive native runs, and detects token exfiltration in module audits.

## ADDED Requirements

### Requirement: Container runtime strips OSC7577_TOKEN unconditionally
The container runtime SHALL remove `OSC7577_TOKEN` from the environment passed to containers, regardless of `env_inherit_mode`, `env_inherit_allow`, or runtime overrides, across both ephemeral and persistent containers.

#### Scenario: Container with env_inherit_mode all does not receive OSC7577_TOKEN
- **GIVEN** `OSC7577_TOKEN` is set in Invowk's host environment
- **WHEN** a container command runs with `env_inherit_mode: "all"` (or `--ivk-env-inherit-mode all`)
- **THEN** `OSC7577_TOKEN` SHALL be absent from the environment inside the container

#### Scenario: Container with explicit allowlist does not receive OSC7577_TOKEN
- **GIVEN** `OSC7577_TOKEN` is set in Invowk's host environment
- **WHEN** a container command runs with `env_inherit_mode: "allow"` and `env_inherit_allow: ["OSC7577_TOKEN"]`
- **THEN** `OSC7577_TOKEN` SHALL be absent from the environment inside the container

### Requirement: Virtual runtimes strip OSC7577_TOKEN from command environments
Both `virtual-sh` and `virtual-lua` SHALL remove `OSC7577_TOKEN` from the constructed command environment and subprocess environment in both non-interactive and interactive modes.

#### Scenario: Non-interactive virtual-sh command does not see OSC7577_TOKEN
- **GIVEN** `OSC7577_TOKEN` is set in Invowk's host environment
- **WHEN** a command runs via `virtual-sh` with default inheritance (`EnvInheritAll`)
- **THEN** `OSC7577_TOKEN` SHALL be unset inside the virtual shell and any allowed host binary it invokes

#### Scenario: Non-interactive virtual-lua command does not see OSC7577_TOKEN
- **GIVEN** `OSC7577_TOKEN` is set in Invowk's host environment
- **WHEN** a command runs via `virtual-lua` with default inheritance (`EnvInheritAll`)
- **THEN** `os.getenv("OSC7577_TOKEN")` and any allowed host binary spawned from Lua SHALL NOT observe `OSC7577_TOKEN`

### Requirement: Native non-interactive commands preserve OSC7577_TOKEN transparently
In non-interactive `native` runs, Invowk SHALL NOT unconditionally strip `OSC7577_TOKEN`; inheritance SHALL follow standard `env_inherit_mode`, `env_inherit_allow`, and `env_inherit_deny` policy rules.

#### Scenario: Native launcher preserves OSC7577_TOKEN
- **GIVEN** `OSC7577_TOKEN` is set in Invowk's host environment
- **WHEN** a non-interactive `native` command runs with default inheritance (`EnvInheritAll`)
- **THEN** the child process SHALL receive `OSC7577_TOKEN` unchanged

#### Scenario: Native command respects explicit env_inherit_deny
- **GIVEN** `OSC7577_TOKEN` is set in Invowk's host environment
- **WHEN** a non-interactive `native` command specifies `env_inherit_deny: ["OSC7577_TOKEN"]` or `env_inherit_mode: "none"`
- **THEN** `OSC7577_TOKEN` SHALL be absent from the child environment

### Requirement: SSH callback server strips OSC7577_TOKEN from host environment
The SSH callback server SHALL remove `OSC7577_TOKEN` from its inherited host environment before launching interactive shells or single commands for connected clients.

#### Scenario: SSH callback command does not inherit Invowk terminal token
- **GIVEN** Invowk runs with `OSC7577_TOKEN` set in its own environment and enables `host_ssh`
- **WHEN** a client executes an interactive shell or command over the SSH callback server
- **THEN** the spawned process SHALL NOT inherit the host terminal's `OSC7577_TOKEN`

### Requirement: Module audit flags OSC7577_TOKEN access and exfiltration
`invowk audit` SHALL treat references to `OSC7577_TOKEN` in command scripts as sensitive credential variable access (`Medium` severity) and piping or redirecting `OSC7577_TOKEN` to external sinks as credential extraction (`High` severity).

#### Scenario: Script reading OSC7577_TOKEN triggers sensitive variable finding
- **WHEN** `invowk audit` scans a module script containing `$OSC7577_TOKEN` or `${OSC7577_TOKEN}`
- **THEN** the audit report SHALL include a `Medium` severity finding for sensitive environment variable access

#### Scenario: Script piping or redirecting OSC7577_TOKEN triggers extraction finding
- **WHEN** `invowk audit` scans a module script containing `$OSC7577_TOKEN | ...` or `$OSC7577_TOKEN > ...`
- **THEN** the audit report SHALL include a `High` severity finding for potential credential extraction

### Requirement: Bubble Tea and ANSI dependencies are upgraded and vulnerability-checked
Invowk SHALL pin `charm.land/bubbletea/v2` to `v2.2.0` and `github.com/charmbracelet/x/ansi` to `v0.12.0` in `go.mod`/`go.sum`, and verify zero reachable vulnerabilities via `make vulncheck`.

#### Scenario: Dependency pin and vulnerability scan pass
- **WHEN** `go.mod` pins `charm.land/bubbletea/v2 v2.2.0` and `github.com/charmbracelet/x/ansi v0.12.0`
- **THEN** `go.mod` and `go.sum` SHALL be tidy and `make vulncheck` SHALL report zero reachable vulnerabilities
