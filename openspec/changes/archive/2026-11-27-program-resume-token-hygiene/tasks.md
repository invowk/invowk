# Tasks

## 1. Dependency Upgrade

- [x] 1.1 Update `charm.land/bubbletea/v2` to `v2.2.0` and `github.com/charmbracelet/x/ansi` to `v0.12.0` in `go.mod`/`go.sum`, run `make tidy`, and verify `make vulncheck` reports zero reachable vulnerabilities
- [x] 1.2 Run TUI package tests (`go test ./internal/tui/...`) to verify compatibility with Bubble Tea `v2.2.0` and `x/ansi v0.12.0`

## 2. Runtime Token Isolation (`container`, `virtual-sh`, `virtual-lua`, `native`)

- [x] 2.1 Add shared `StripProgramResumeToken` / `FilterProgramResumeToken` helpers in `internal/runtime` using `ansi.ProgramResumeTokenEnv` (`OSC7577_TOKEN`) and cover them with unit tests
- [x] 2.2 Strip `OSC7577_TOKEN` unconditionally in `ContainerRuntime` (`internal/runtime/container_exec.go`) after building the container env, and add tests proving `env_inherit_mode: "all"` and explicit `env_inherit_allow: ["OSC7577_TOKEN"]` never forward it
- [x] 2.3 Strip `OSC7577_TOKEN` in `ShRuntime` (`internal/runtime/sh.go`), `LuaRuntime` (`internal/runtime/lua.go`), and `buildVirtualInteractiveSubprocess` (`internal/runtime/virtual_interactive.go`), and add unit tests verifying both non-interactive and interactive virtual runs drop it while non-interactive `native` preserves it under normal `env_inherit_*` rules
- [x] 2.4 Verify runtime unit tests pass with `go test ./internal/runtime/...`

## 3. Interactive PTY Boundary (`-i`) and SSH Callback Server

- [x] 3.1 Strip `OSC7577_TOKEN` from `cmd.Env` at the `-i` PTY boundary (`internal/app/commandadapters/interactive_session.go`) and from helper subprocess envs (`internal/app/commandadapters/sh_interactive.go`, `lua_interactive.go`), adding unit tests for both `cmd.Env == nil` and populated `cmd.Env`
- [x] 3.2 Strip `OSC7577_TOKEN` from inherited `os.Environ()` in `internal/sshserver/server.go` (`runInteractiveShell` and `runCommand`) and add unit tests verifying SSH callback sessions do not receive the outer terminal's token
- [x] 3.3 Verify adapter and SSH server tests pass with `go test ./internal/app/commandadapters/... ./internal/sshserver/...`

## 4. Audit Checker & Documentation

- [x] 4.1 Add `OSC7577_TOKEN` to `sensitiveVarPattern` and `tokenExtractionPattern` in `internal/audit/checks_env.go` and add unit tests covering `Medium` and `High` findings in `internal/audit/checks_env_test.go` (verifying with `go test ./internal/audit/...`)
- [x] 4.2 Update runtime/security documentation (`website/docs/` as applicable per `.agents/skills/docs/SKILL.md`) to document `OSC7577_TOKEN` isolation across `container`, `virtual-*`, `-i`, and transparent pass-through in non-interactive `native`, and verify `cd website && npm run build` if website files changed

## 5. Final Verification Gates

- [x] 5.1 Run full completion checklist from `.agents/rules/checklist.md`: `make tidy`, `make license-check`, `make lint`, `make test`, `make check-baseline`, `make check-file-length`, and `make sonar-local`
