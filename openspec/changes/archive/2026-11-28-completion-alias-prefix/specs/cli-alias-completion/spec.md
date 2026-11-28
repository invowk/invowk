# Spec Delta

## Purpose

Defines `--alias` and `--prefix` flags for `invowk completion`, enabling standalone CLI wrapper commands to use shell tab-completion backed by `invowk __complete` without collisions or shell injection.

## ADDED Requirements

### Requirement: invowk completion supports --alias and --prefix across all shells
`invowk completion [bash|zsh|fish|powershell]` SHALL accept optional `--alias <name>` and `--prefix <tokens>` flags. Generated scripts SHALL register completion for `<name>`, use namespace-safe completion function names that do not collide with `invowk`'s own completions, and invoke `invowk __complete[NoDesc]` with `<prefix>` tokens prepended before the user's typed words. `--prefix` SHALL support Invowk scope flags (such as `-c <path>` and `--ivk-no-cwd`) as well as `cmd <source>` prefixes.

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
