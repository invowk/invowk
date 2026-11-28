# Proposal: Raw Command Argument Passthrough (`passthrough: true`)

## Why

Wrapping existing CLIs (such as `cst perf`, `cst sync`, or `cst btrfs ...`) in Invowk without `--` currently requires duplicating every downstream flag in `invowkfile.cue` and manually reconstructing CLI options from `INVOWK_FLAG_*` environment variables, because Cobra rejects unknown flags before `--` with exit status `2` and intercepts `--help`/`-h`.

## What Changes

- Add an opt-in `passthrough?: bool` field on `#Command` in `pkg/invowkfile/invowkfile_schema.cue` (default `false`), mutually exclusive with `flags`, `args`, and child subcommands in both CUE schema validation and `ValidateCommandTree` (`pkg/invowkfile/command_tree.go`).
- Enforce a clean, hardened CLI boundary: all `--ivk-*` flags positioned **before** the command path belong to Invowk, while every token **after** the command path (including `--help`, `-h`, unknown flags, and `--`) is forwarded verbatim to the script (`$@`, `ARGn`/`ARGC`, `invowk.args`).
- Make leaf-only `--ivk-*` execution flags (`--ivk-workdir`, `--ivk-env-var`, `--ivk-env-file`, `--ivk-env-inherit-mode`, `--ivk-env-inherit-allow`, `--ivk-env-inherit-deny`) available before the command path on `invowk cmd` so passthrough commands remain controllable via `--ivk-*`.
- Handle source-disambiguated invocations (`@source` and `--ivk-from`) so ambiguous passthrough commands do not get intercepted by `invowk cmd`'s own flag parser when forwarding `--help` or downstream flags.
- Preserve `invowk help cmd <name>` for inspecting Invowk metadata and clearly annotate passthrough commands in `invowk cmd` listings.

## Capabilities

### New Capabilities

- `command-arg-passthrough`: Defines `passthrough: true` schema constraints, mutual exclusivity with `flags`/`args`/subcommands, pre-command `--ivk-*` parsing, `@source`/`--ivk-from` passthrough routing, verbatim runtime argument forwarding, and listing/help behavior.

### Modified Capabilities

(none)

## Impact

- Schema & Model: `pkg/invowkfile/invowkfile_schema.cue`, `pkg/invowkfile/command.go`, `pkg/invowkfile/command_tree.go`, schema-sync tests.
- CLI Boundary: `cmd/invowk/cmd.go`, `cmd/invowk/cmd_discovery.go`, `cmd/invowk/cmd_execute_helpers.go`.
- Runtimes & Docs: `internal/runtime/` verification across `native`, `virtual-sh`, `virtual-lua`, `container` (noting `cmd.exe` Windows limitations), `tests/cli/` testscript coverage, and `website/` docs.
