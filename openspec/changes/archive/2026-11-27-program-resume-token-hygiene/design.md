# Design: Program Resume Token Hygiene (`OSC7577_TOKEN`)

## Context

See `proposal.md` for motivation. Current codebase observations:

- `go.mod:7` pins `charm.land/bubbletea/v2 v2.1.1` and `go.mod:93` pins `github.com/charmbracelet/x/ansi v0.11.9`. Bubble Tea `v2.2.0` (PR #1908) depends on `x/ansi v0.12.0`, which defines `ansi.ProgramResumeTokenEnv` (`"OSC7577_TOKEN"`).
- `internal/runtime/env.go:70-111` (`buildHostEnv`) filters host environment variables through `FilterInvowkEnvVars(environ())` (`internal/runtime/runtime.go:674`, `shouldFilterEnvVar` at `internal/runtime/runtime.go:699-750`). Today `shouldFilterEnvVar` filters `INVOWK_ARG_*`, `INVOWK_FLAG_*`, `INVOWK_SSH_*`, `INVOWK_TUI_*`, metadata vars, `ARGC`, and `ARGn`, but not `OSC7577_TOKEN`.
- `internal/runtime/container_exec.go:129-136` calls `r.envBuilder.Build(ctx, invowkfile.EnvInheritNone)`, merges `provisionEnv`, and adds TUI env when `opts.interactiveTUI` is set. When `env_inherit_mode: "all"` or `"allow"` (naming `OSC7577_TOKEN`) is configured, `buildHostEnv` forwards `OSC7577_TOKEN` into the container.
- `internal/runtime/sh.go:449` and `internal/runtime/lua.go:188` call `r.envBuilder.Build(ctx, invowkfile.EnvInheritAll)`.
- `internal/runtime/virtual_interactive.go:113-128` (`buildVirtualInteractiveSubprocess`) builds `env` with `EnvInheritAll` and marshals it to `envJSON`, while `internal/app/commandadapters/sh_interactive.go:66` and `internal/app/commandadapters/lua_interactive.go:55` set `cmd.Env = runtime.FilterInvowkEnvVars(os.Environ())` on the helper subprocess (`internal exec-virtual-sh` / `exec-virtual-lua`).
- `internal/runtime/native_helpers.go:117` (`configureCommandDirAndEnv`) builds the native command environment using `EnvInheritAll` for both non-interactive and `PrepareInteractive` (`internal/runtime/native.go:170`).
- `internal/app/commandadapters/interactive_session.go:74-96` (`startInteractivePty`) creates the `-i` PTY and invokes `prepareInteractiveCmd(cmd, pty)` (`internal/app/commandadapters/interactive_session_unix.go:67-83`). Meanwhile `internal/tui/interactive_screen.go:76` runs `vt.NewEmulator`, which absorbs child OSC sequences rather than forwarding them to the outer terminal.
- `internal/sshserver/server.go:282,331` constructs `cmd.Env = append(os.Environ(), sess.Environ()...)` directly from `os.Environ()`.
- `internal/audit/checks_env.go:22-41` defines `sensitiveVarPattern` and `tokenExtractionPattern` without `OSC7577_TOKEN`.

## Goals / Non-Goals

**Goals:**
- Upgrade `charm.land/bubbletea/v2` to `v2.2.0` and `github.com/charmbracelet/x/ansi` to `v0.12.0` cleanly.
- Guarantee `OSC7577_TOKEN` cannot cross container, virtual-runtime, SSH-callback, or `-i` nested-PTY boundaries.
- Keep non-interactive `native` runtime transparent for `OSC7577_TOKEN` under standard `env_inherit_*` controls.
- Detect `OSC7577_TOKEN` reads and exfiltration in `invowk audit`.

**Non-Goals:**
- Do NOT emit OSC 7577 from Invowk yet (handled by `watch-mode-program-resume` and `declarative-command-program-resume`).
- Do NOT implement OSC 7577 pass-through inside `internal/tui/interactive_screen.go` (pass-through would violate the OSC 7577 threat model for containers and environment-wrapped commands).

## Decisions

### D1. Dedicated token-stripping helpers instead of adding `OSC7577_TOKEN` to `FilterInvowkEnvVars`
`FilterInvowkEnvVars` (`internal/runtime/runtime.go:674`) is invoked inside `buildHostEnv` (`internal/runtime/env.go:92`) for **all** runtimes, including non-interactive `native`. Putting `OSC7577_TOKEN` into `shouldFilterEnvVar` would break transparent OSC 7577 pass-through for native host wrappers (e.g., `invowk cmd edit` -> `nvim`).

Instead:
- Introduce `StripProgramResumeToken(env map[string]string)` and `FilterProgramResumeToken(environ []string) []string` in `internal/runtime` referencing `ansi.ProgramResumeTokenEnv` (`"OSC7577_TOKEN"`).
- Note: also decide whether `StripProgramResumeToken` runs **after** `envBuilder.Build` so it overrides even an explicit `env_inherit_allow: ["OSC7577_TOKEN"]` or `env.vars` entry on restricted boundaries. For `container`, `virtual-sh`, `virtual-lua`, and `-i`, calling `StripProgramResumeToken(env)` after `envBuilder.Build` closes every path cleanly.
- Alternative considered: filtering inside `buildHostEnv` via a parameter on `EnvInheritConfig` — rejected as unnecessary churn across `EnvBuilder` signatures when boundary helpers are clearer and compose with `cmd.Env` slices in `interactive_session.go` and `sshserver`.

### D2. Boundary enforcement matrix

| Boundary | Non-interactive | Interactive (`-i`) | Where enforced |
|---|---|---|---|
| `native` | **Keep** (subject to `env_inherit_*`) | **Strip** | `interactive_session.go` (`startInteractivePty` / `prepareInteractiveCmd`) + `native_helpers.go` or adapter boundary |
| `virtual-sh` | **Strip** | **Strip** | `sh.go:449`, `virtual_interactive.go:113`, `sh_interactive.go:66`, `interactive_session.go` |
| `virtual-lua` | **Strip** | **Strip** | `lua.go:188`, `virtual_interactive.go:113`, `lua_interactive.go:55`, `interactive_session.go` |
| `container` | **Strip** | **Strip** | `container_exec.go:129-136`, `interactive_session.go` |
| `sshserver` | **Strip** | **Strip** | `internal/sshserver/server.go:282,331` |

Important nuance for `prepareInteractiveCmd` (`internal/app/commandadapters/interactive_session.go:80`):
- When `cmd.Env == nil`, `os/exec` inherits the parent process's `os.Environ()`. Therefore, stripping at the PTY boundary must explicitly check: if `cmd.Env == nil`, initialize `cmd.Env = FilterProgramResumeToken(os.Environ())`; if `cmd.Env != nil`, filter `cmd.Env = FilterProgramResumeToken(cmd.Env)`.
- Notice `NativeRuntime.PrepareInteractive` (`internal/runtime/native.go:170`) shares `configureCommandDirAndEnv` (`internal/runtime/native_helpers.go:108`) with non-interactive `NativeRuntime`. Enforcing the `-i` strip in `startInteractivePty` (plus `buildVirtualInteractiveSubprocess` / `{sh,lua}_interactive.go` for `envJSON`) guarantees non-interactive `native` stays untouched without duplicating `configureCommandDirAndEnv`.

### D3. Dependency upgrade (`bubbletea/v2 v2.2.0`, `x/ansi v0.12.0`)
Upgrade `charm.land/bubbletea/v2` (`v2.1.1` -> `v2.2.0`) and `github.com/charmbracelet/x/ansi` (`v0.11.9` -> `v0.12.0`) in root `go.mod`/`go.sum`. Verify compatibility across `internal/tui/` and `internal/app/commandadapters/` (note `tea.EnvMsg` / `View.ProgramResume` are additive in `v2.2.0`) and run `make vulncheck` per `go-dependency-maintenance`.

### D4. `invowk audit` pattern updates
Update `sensitiveVarPattern` and `tokenExtractionPattern` in `internal/audit/checks_env.go:22-41` to include `OSC7577_TOKEN`, matching the existing treatment of `INVOWK_SSH_TOKEN` and `INVOWK_TUI_TOKEN`. Note `checks_env.go:20-41` keeps both regexes synchronized by comment convention.

## Risks / Trade-offs

- **Risk**: A user intentionally using `virtual-sh` with `allowed_binaries: ["*"]` as a shell wrapper expects `nvim` inside `virtual-sh` to receive `OSC7577_TOKEN`.
  -> **Mitigation**: Document clearly that `native` runtime is the transparent host-launcher surface; `virtual-*` prioritizes policy isolation because its default `allowed_binaries` is empty and restricting token propagation avoids policy bypasses via OSC 7577 restart commands. Revisit only if explicit demand emerges (see Open questions).
- **Risk**: `FilterProgramResumeToken` on `cmd.Env` when `cmd.Env == nil` could drop `nil` semantics if tests expect `cmd.Env == nil`.
  -> **Mitigation**: Only materialize `os.Environ()` if `os.LookupEnv(ansi.ProgramResumeTokenEnv)` is actually present (or always strip safely in `startInteractivePty` while updating unit tests that assert `cmd.Env`).

## Maintainer decisions

- **2026-11-27**: Split Program Resume Protocol work into three OpenSpec changes (`program-resume-token-hygiene`, `watch-mode-program-resume`, `declarative-command-program-resume`), prioritizing Options 1 and 2 for implementation while parking Option 3.
- **2026-11-27**: Preserve `OSC7577_TOKEN` transparently in non-interactive `native` commands so host TUIs launched via `invowk cmd` continue supporting OSC 7577.

## Open questions

- **Virtual non-interactive policy**: Default recommendation in this design is **unconditional stripping** of `OSC7577_TOKEN` in `virtual-sh` and `virtual-lua`. If the maintainer prefers conditional pass-through only when `allowed_binaries` includes `"*"` and `filesystem_access` is `"full"`, update `program-resume-token-isolation` before implementing Task Group 2.
