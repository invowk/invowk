# Proposal: Program Resume Token Hygiene (`OSC7577_TOKEN`)

## Why

Terminals implementing the Program Resume Protocol (OSC 7577, supported starting in Bubble Tea `v2.2.0` / `github.com/charmbracelet/x/ansi` `v0.12.0`) inject an authentication secret (`OSC7577_TOKEN`) into the initial process environment of each pane. Today Invowk runs on Bubble Tea `v2.1.1`, does not isolate `OSC7577_TOKEN` at container, nested-PTY (`-i`), virtual-runtime, or SSH callback boundaries, and does not flag `OSC7577_TOKEN` exfiltration in `invowk audit` — creating a container-escape / host-command-registration vector under `env_inherit_mode: "all"` and breaking the OSC 7577 nested-terminal contract in `-i`.

## What Changes

- Upgrade `charm.land/bubbletea/v2` from `v2.1.1` to `v2.2.0` and `github.com/charmbracelet/x/ansi` from `v0.11.9` to `v0.12.0` in `go.mod`/`go.sum` (exact pins, vulnerability-checked per `go-dependency-maintenance`).
- Strip `OSC7577_TOKEN` unconditionally from container runtime environments (`internal/runtime/container_exec.go`), even when `env_inherit_mode` is `"all"` or `"allow"` explicitly names `OSC7577_TOKEN`.
- Strip `OSC7577_TOKEN` from child environments in interactive mode (`invowk cmd -i`) across all runtimes (`native`, `virtual-sh`, `virtual-lua`, `container`), because the `-i` viewer runs an `x/vt` emulated PTY that drops child OSC sequences and MUST NOT pass the outer terminal's resume token to children.
- Strip `OSC7577_TOKEN` from virtual runtimes (`virtual-sh` and `virtual-lua`) in non-interactive runs as well, preventing restricted virtual scripts from using `OSC7577_TOKEN` to schedule host binaries outside `allowed_binaries` on terminal restart.
- Preserve `OSC7577_TOKEN` pass-through in non-interactive `native` runs (subject to standard `env_inherit_mode` / `env_inherit_deny` rules), so transparent host TUI launches (`nvim`, `claude`, `lazygit`) continue working with OSC 7577.
- Strip `OSC7577_TOKEN` from `internal/sshserver` host-callback sessions before merging `sess.Environ()`.
- Add `OSC7577_TOKEN` detection to `invowk audit` (`internal/audit/checks_env.go`) as a sensitive variable and credential-extraction target.
- Non-breaking change: no CLI flags, CUE schema fields, or non-interactive `native` defaults change; only security/isolation boundaries and nested PTY environments stop leaking `OSC7577_TOKEN`.

## Capabilities

### New Capabilities

- `program-resume-token-isolation`: Defines how Invowk isolates `OSC7577_TOKEN` across container, virtual, SSH-callback, and non-interactive native runtimes, upgrades Bubble Tea/ANSI dependencies to `v2.2.0`/`v0.12.0`, and detects `OSC7577_TOKEN` access/exfiltration in `invowk audit`.

### Modified Capabilities

- `interactive-mode-terminal`: Adds the requirement that `invowk cmd -i` strips the outer terminal's `OSC7577_TOKEN` from the child command's environment across all runtimes.

## Impact

- Dependencies: `go.mod`, `go.sum` (`charm.land/bubbletea/v2 v2.2.0`, `github.com/charmbracelet/x/ansi v0.12.0`).
- Code: `internal/runtime/` (env filtering helpers, `container_exec.go`, `sh.go`, `lua.go`, `virtual_interactive.go`, `native_helpers.go`), `internal/app/commandadapters/` (`interactive_session.go`, `sh_interactive.go`, `lua_interactive.go`), `internal/sshserver/server.go`, `internal/audit/checks_env.go`.
- Tests & Docs: unit and CLI tests for each boundary, `make vulncheck`, and runtime/security documentation updates.
