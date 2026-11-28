# Proposal: Standalone CLI Alias Shell Completion (`completion --alias --prefix`)

## Why

Teams that expose a standalone CLI wrapper around an Invowk module (for example `exec invowk -c <cfg> --ivk-no-cwd cmd cst "$@"`) currently cannot generate shell tab-completion bound to `cst <subcommand>` because `invowk completion <shell>` (`cmd/invowk/completion.go`) only generates completions for the `invowk` command name itself, while multi-call `argv[0]` dispatch remains intentionally rejected.

## What Changes

- Add `--alias <name>` and `--prefix <tokens>` flags to `invowk completion [bash|zsh|fish|powershell]` (`cmd/invowk/completion.go`).
- Upgrade Bash completion generation from `GenBashCompletion` (v1) to `GenBashCompletionV2` so Cobra's dynamic `__complete` protocol works cleanly with alias and prefix rewriting across Bash, Zsh, Fish, and PowerShell (updating `tests/cli/testdata/completion.txtar` accordingly).
- Allow `--prefix` to include Invowk's own scope flags (e.g. `--prefix "-c /etc/cst/invowk.cue --ivk-no-cwd cmd cst"`) with strict input validation and per-shell quoting/escaping so generated completion scripts invoke `invowk __complete[NoDesc] <prefix...> <user-args...>` safely without shell injection.

## Migration notes

- `invowk completion bash` now emits Cobra Bash completion V2 (`# bash completion V2 for ...`), which requires `bash-completion` v2+ (standard on modern Linux/macOS distributions) instead of v1 static completions.

## Capabilities

### New Capabilities

- `cli-alias-completion`: Defines `--alias` and `--prefix` flags on `invowk completion`, `GenBashCompletionV2` dynamic completion, validation of alias names, safe shell-escaped `__complete` prefix rewriting across `bash`, `zsh`, `fish`, and `powershell`, and wrapper pattern documentation.

### Modified Capabilities

(none)

## Impact

- CLI: `cmd/invowk/completion.go` and new completion rewriting/escaping helpers + unit tests (`cmd/invowk/completion_test.go`).
- Tests & Docs: `tests/cli/` testscript verification of generated scripts and `__complete` invocation for all 4 shells, plus `website/` CLI reference docs illustrating the wrapper pattern alongside `passthrough: true` and `--ivk-color`.
