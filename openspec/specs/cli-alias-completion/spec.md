# cli-alias-completion Specification

## Purpose

Defines `--alias` and `--prefix` flags for `invowk completion`, enabling standalone CLI wrapper commands to use shell tab-completion backed by `invowk __complete` without collisions or shell injection.

## Requirements

### Requirement: invowk completion supports --alias and --prefix across all shells
`invowk completion [bash|zsh|fish|powershell]` SHALL accept optional `--alias <name>` and `--prefix <tokens>` flags. Generated scripts SHALL register completion for `<name>`, use namespace-safe helper identifiers that do not collide with `invowk`, and prepend `<prefix>` tokens (including scope flags like `-c <path>` and `--ivk-no-cwd`, plus `cmd <source>`) to `invowk __complete[NoDesc]`.

#### Scenario: Bash completion with alias and scoped prefix queries __complete properly
- **WHEN** the user runs `invowk completion bash --alias cst --prefix "-c /etc/cst/config.cue --ivk-no-cwd cmd cst"`
- **THEN** stdout SHALL contain a valid Bash v2 completion script bound to `cst` whose probe invokes `invowk __complete` with `-c`, `/etc/cst/config.cue`, `--ivk-no-cwd`, `cmd`, and `cst`

#### Scenario: Zsh, Fish, and PowerShell support --alias and --prefix
- **WHEN** `invowk completion <shell> --alias cst --prefix "cmd cst"` runs for `zsh`, `fish`, and `powershell`
- **THEN** each output script SHALL bind completions to `cst`, use non-colliding helper identifiers, and prepend `cmd` `cst` to `__complete`

### Requirement: --alias and --prefix reject unsafe values with usage error
`--alias` SHALL accept only safe command identifier tokens (alphanumeric, `.`, `_`, `-`, starting with alphanumeric), and `--prefix` without `--alias` or values containing NUL/newlines/unclosed quotes SHALL fail with exit status `2`. Emitted `--prefix` tokens SHALL be shell-escaped for the target shell.

#### Scenario: Unsafe alias rejected with exit status 2
- **WHEN** the user runs `invowk completion bash --alias "cst; rm -rf /"`
- **THEN** invowk SHALL exit with status `2` and print an invalid `--alias` error
