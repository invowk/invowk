# Design: Standalone CLI Alias Shell Completion (`completion --alias --prefix`)

## Context

Current behavior in `main`:
- `cmd/invowk/completion.go:12-60` implements `invowk completion [bash|zsh|fish|powershell]` using Cobra's `GenBashCompletion(os.Stdout)` (`completion.go:49`), `GenZshCompletion(os.Stdout)` (`completion.go:51`), `GenFishCompletion(os.Stdout, true)` (`completion.go:53`), and `GenPowerShellCompletionWithDesc(os.Stdout)` (`completion.go:55`).
- `cmd/invowk/cmd_discovery.go:369-424` (`completeCommands`) provides next-token completion for `invowk cmd ...` via Cobra's `__complete` / `__completeNoDesc` plumbing, reading `--ivk-config` and `--ivk-no-cwd` (`cmd_discovery.go:371-375`).
- `GenBashCompletion` (v1) generates static/legacy bash completion code that does not use Cobra's v2 `__complete` protocol cleanly when rewriting command names and injected prefix tokens; `GenBashCompletionV2` (`cobra v1.10.2` in `go.mod:30`) uses the unified `__complete` hook like Zsh, Fish, and PowerShell.

## Goals / Non-Goals

**Goals:**
- Generate shell completion scripts for wrapper commands (`cst <subcommand>`) across `bash`, `zsh`, `fish`, and `powershell` without adding multi-call `argv[0]` dispatch to `invowk`.
- Support `--prefix` containing both Invowk scope flags (e.g., `-c /etc/cst/config.cue --ivk-no-cwd`) and subcommand prefixes (`cmd cst`) so wrapper completions resolve the exact scoped module rather than leaking cwd commands.
- Guarantee generated scripts are immune to shell command injection from untrusted `--alias` / `--prefix` values.

**Non-Goals:**
- Changing multi-call `argv[0]` rejection (wrappers remain small explicit shell `exec` scripts).
- Completing downstream flags of `passthrough: true` commands (beyond Invowk subcommand/command-tree tokens).

## Decisions

### D1. Flag contract & validation
- Add two optional flags to `newCompletionCommand()` (`cmd/invowk/completion.go:12`):
  - `--alias string`: wrapper command name to register completions for (validated as a safe identifier, e.g., `^[a-zA-Z0-9][a-zA-Z0-9._-]*$` — rejects spaces, quotes, metacharacters, slashes; returns `UsageError` exit `2` if invalid).
  - `--prefix string`: space-separated tokens (or repeatable flag / shell-split tokens; reject `--prefix` without `--alias` or vice versa if ambiguous, or allow `--alias` alone defaulting `--prefix` to `"cmd <alias>"` — let's require `--alias` whenever `--prefix` is set, and default `--prefix` to `""` when `--alias` is given alone or allow explicit `--prefix "cmd cst"`).
- Reject `--alias` or `--prefix` values containing NUL bytes, newlines, or unbalanced quotes, and return `UsageError` (`2`) per `openspec/specs/cli-exit-codes/spec.md`.

### D2. `GenBashCompletionV2` + `cmd.OutOrStdout()` + shell-specific `__complete` rewriting
- Switch `cmd/invowk/completion.go:49-55` from `os.Stdout` to `cmd.OutOrStdout()` (keeping CLI/unit output capture clean) and upgrade Bash generation (`cmd/invowk/completion.go:49`) to `cmd.Root().GenBashCompletionV2(cmd.OutOrStdout(), true)`. Also update `tests/cli/testdata/completion.txtar:8` (`bash completion V2 for invowk`).
- When `--alias` / `--prefix` are provided:
  - Generate the Cobra script to an in-memory buffer and rewrite the Cobra completion template entries cleanly per shell:
    1. Rename the bound command / function identifiers from `invowk` to the validated alias `<alias>` so sourcing both `invowk` and `<alias>` completions in the same shell session causes zero function-name collisions.
    2. Rewrite the completion probe invocation from `invowk __complete[NoDesc] ...` to `invowk __complete[NoDesc] <escaped-prefix-tokens...> ...`, using per-shell literal single/argument quoting (`sh` single-quote escaping for bash/zsh/fish; PowerShell single-quote escaping `''` for powershell).
  - Validate with golden/behavioral tests that sourcing the generated `bash`, `zsh`, and `fish` scripts (when shells are available in CI/local) registers `<alias>` without colliding with `invowk`'s own completion functions.

### D3. Verification against `completeCommands`
- Verify that `invowk __complete -c <cfg> --ivk-no-cwd cmd cst ""` (the exact probe emitted by the rewritten script) runs `shouldRegisterDiscoveredCommands` (`cmd/invowk/cmd.go:283-285`) and `completeCommands` (`cmd/invowk/cmd_discovery.go:369-375`) properly, including when paired with `--ivk-color <val>` handled in `cli-color-and-quiet-exit-framing` (`firstTopLevelToken`). Document/test that `invowk __complete -c <cfg> --ivk-no-cwd cmd cst ""` registers discovered commands cleanly when generating alias completions.

## Risks / Trade-offs

- **Risk**: Cobra's generated completion scripts vary slightly across Cobra minor updates, breaking fragile string replacements.
  -> **Mitigation**: Keep replacement anchored to Cobra's documented function/command tokens and add unit tests asserting both exact `__complete` command rewriting and no leftover un-namespaced `__start_invowk` / `_invowk` collisions (and test live `bash`/`zsh`/`fish` evaluation where available).
- **Risk**: Upgrading `GenBashCompletion` (v1) to `GenBashCompletionV2` changes bash completion script output for users of `invowk completion bash`.
  -> **Mitigation**: `GenBashCompletionV2` is Cobra's standard modern bash completion (requires bash-completion v2, standard on modern Linux/macOS); document in release notes and verify existing `completion` tests.

## Maintainer decisions

- **2026-11-27**: Confirmed `--alias` and `--prefix` flags on `invowk completion`, upgrading Bash to `GenBashCompletionV2`, supporting Invowk scope flags (`-c`, `--ivk-no-cwd`) inside `--prefix` with strict validation and per-shell escaping, and keeping multi-call `argv[0]` dispatch rejected.

## Open questions

- Confirm whether `--alias <name>` without `--prefix` should default `--prefix` to `""` (alias for root `invowk`) or `"cmd <name>"` — default `""` is cleaner and more predictable (users pass `--prefix "cmd <name>"` explicitly when wrapping a module).
