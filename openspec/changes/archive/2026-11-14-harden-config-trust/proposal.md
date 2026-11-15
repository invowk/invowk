## Why

With no user config file (normal for root, CI and many users), invowk loads `./config.cue` from the current directory (`internal/config/config.go:196-211`). So a cloned repository can set invowk's `includes` (modules from any path), `llm.api.base_url` plus `llm.api.api_key_env` (sending a user's API key to a host it picks), `container.auto_provision.binary_path` and `virtual` runtime policy. That is config injection, with a credential-exfiltration path.

Explicit sources also fail open: a missing `--ivk-config` file gives a warning and defaults (`internal/app/commandsvc/config_fallback.go:27-44`). And nothing says "only trusted sources may add commands", which wrappers and root-run tooling need (CST adoption review, Issue 3a).

## What Changes

- **BREAKING:** invowk no longer reads `config.cue` from the current directory or from any other implicit location. Config comes only from `--ivk-config`, `INVOWK_CONFIG`, or the per-user config file (XDG/APPDATA/Application Support). Otherwise defaults apply. `LoadOptions.BaseDir` is removed.
- **New:** the `INVOWK_CONFIG` environment variable names the config file. Precedence is `--ivk-config` > `INVOWK_CONFIG` > user config file > built-in defaults, and only the selected source is read. An empty value counts as unset. A relative value is rejected.
- **BREAKING:** an explicit config source (`--ivk-config` or `INVOWK_CONFIG`) that is missing, unreadable, invalid CUE or fails validation stops the invocation with **exit code 1** (`audit` and `agent` use their own error code) and an error that names the source and the path. Today it is a warning or error diagnostic, followed by a silent fallback to defaults. Only the implicit user config file keeps today's behaviour when invalid: a diagnostic, then defaults.
- **New:** the root persistent flag `--ivk-no-cwd` and the env var `INVOWK_NO_CWD` (boolean, `strconv.ParseBool` values) turn off discovery of the current-directory `invowkfile.cue` and of sibling `*.invowkmod` modules. Configured `includes`, `~/.invowk/cmds` and provisioned modules still apply. This affects `invowk cmd` (run, list, completion, dynamic registration, watch, dry-run), workspace `invowk validate`, `invowk module list` and hidden internal discovery commands. Commands that take an explicit target are not affected: `audit [path]`, `validate <path>`, the other `module` subcommands, `init`, `config *`, `tui *`. `agent cmd create|change --verify` is rejected under the switch, because it dry-runs the cwd invowkfile. With `--ivk-verbose`, the `invowk cmd` listing prints "current-directory sources disabled (<origin>)"; workspace `invowk validate` always prints it.
- **BREAKING (fix):** `invowk validate`, `config show` and `config dump` now honour `--ivk-config` and `INVOWK_CONFIG`; today `validate` hard-codes an empty path (`cmd/invowk/validate.go:106`) and the `config` subcommands load with no options (`cmd/invowk/config.go:102,117`). They therefore also fail with exit 1 on a broken explicit source. `config show` reports the effective source. `config path` adds the effective config file without loading it.
- **BREAKING:** `config set` writes only the user config file and refuses with an error while `--ivk-config` or `INVOWK_CONFIG` is active, so an explicit or repository file can never be copied into the user config. `config init` stays user-file only.
- `INVOWK_CONFIG` and `INVOWK_NO_CWD` are not filtered from, and not exported to, child processes: nested `invowk` calls see them only when the caller set them, under the normal `env_inherit_mode` rules. The flag forms do not reach nested calls. Exporting them is deferred (design "Deferred").
- Docs, i18n (pt-BR), README, CLI help, architecture diagrams and the discovery skill are updated. The migration text below is this change's contribution to the `v0.17.0` release notes; there is no shared migration file.

## Migration notes

Release-notes text, ready to paste:

> **Config is no longer read from the current directory.** Earlier versions loaded `./config.cue` when no user config file existed. This let any repository you `cd` into change invowk's includes, LLM endpoint and API-key variable, and container provisioning. If you relied on it, choose one of:
> - pass the file explicitly: `invowk --ivk-config ./config.cue cmd …`;
> - export it once per shell or CI job: `export INVOWK_CONFIG="$PWD/config.cue"` (must be an absolute path; a relative value is rejected);
> - move it to your user config file (`invowk config path` shows where).
>
> **Explicit config errors are now fatal.** If `--ivk-config` or `INVOWK_CONFIG` names a file that is missing or invalid, invowk stops with an error (exit code 1; `invowk audit` and `invowk agent` use their own error code) instead of warning and continuing with defaults. This now also applies to `invowk validate`, `invowk config show` and `invowk config dump`, which previously ignored `--ivk-config`. Fix the path, or unset the variable.
>
> **Config base directories must be absolute.** A relative `XDG_CONFIG_HOME` or `APPDATA` is now ignored (the next base, such as `~/.config`, is used), and a relative home directory or `USERPROFILE` is an error, so the user config file and `~/.invowk/cmds` can no longer resolve under the current directory. Set these variables to absolute paths.
>
> **`invowk config set` refuses while an explicit config is active.** It edits only your user config file. If `--ivk-config` is given or `INVOWK_CONFIG` is set, it exits with an error instead of copying that file into your user config. Unset the variable (or drop the flag), or edit the selected file directly.
>
> **New: `--ivk-no-cwd` / `INVOWK_NO_CWD=1`.** These ignore the current directory's `invowkfile.cue` and `*.invowkmod` modules. Only configured `includes`, `~/.invowk/cmds` and provisioned modules provide commands. Use them in wrappers, CI and anything run as root. Prefer the environment forms when your commands call `invowk` again, because the flags do not reach nested calls: `INVOWK_CONFIG=/etc/x/invowk.cue INVOWK_NO_CWD=1 invowk cmd …`. `invowk agent cmd create|change --verify` cannot be combined with the switch.

## Capabilities

### New Capabilities

- `config-source-resolution`: where invowk may read its configuration from, the precedence order, how a missing or invalid source is handled, and which file each `config` subcommand reads or writes. The current directory is never a source.
- `cwd-discovery-scope`: the `--ivk-no-cwd` / `INVOWK_NO_CWD` switch, which command sources it removes, which commands it applies to, and how it reaches nested invocations.

### Modified Capabilities

(none — no existing spec covers config loading or discovery sources)

## Impact

- Code:
  - `internal/config/config.go`, `provider.go`: fallback and `BaseDir` removed; `SelectSource`, `ConfigSourceKind`, `ExplicitConfigError` and injected `getenv` added; `LoadResult.Source`.
  - `internal/app/commandsvc/config_fallback.go`, `service.go`, `types.go`: explicit-source errors become fatal (`ConfigFallbackFunc` and `ConfigAwareCommandDiscovery` gain an error result).
  - `internal/app/commandadapters/discovery.go`: config error caching, no-cwd context and cache scoping, `newDiscovery(ctx, cfg)`.
  - `cmd/invowk/root.go`, `app.go`, `cmd.go`, `cmd_discovery.go`, `cmd_execute_helpers.go`, `cmd_watch.go`, `internal_check_cmd.go`, `module.go`, `validate.go`, `config.go`, `agent.go`: flag, env resolution, pre-parse argv scan, scope threading, `config` subcommand behaviour.
  - `internal/runtime/runtime.go`: comment only (D8).
- Tests:
  - `internal/config/config_test.go`, `config_mutation_test.go` and `provider_test.go` (cwd-fallback and `BaseDir` cases); new `config_source_test.go`.
  - `cmd/invowk/cmd_app_additional_test.go`, `cmd_context_test.go`, `cmd_execute_runtime_test.go` (fail-open assertions).
  - `tests/cli/testdata/config_override_flag.txtar` Test 4 flips from warning to failure.
  - New txtar tests, including the mirrored pair `virtual_discovery_no_cwd.txtar` / `native_discovery_no_cwd.txtar`.
- Docs:
  - `website/docs/configuration/overview.mdx:21` and the pt-BR copy.
  - `website/docs/reference/cli.mdx` Global Flags, en and pt-BR.
  - Discovery-order lists in website docs and README.
  - README Configuration and flags sections.
  - `docs/diagrams/c4/container.d2`, `docs/diagrams/flowcharts/discovery-flow.d2`, `docs/diagrams/sequences/execution-main.d2`, `docs/architecture/flowchart-discovery.md`, `docs/architecture/c4-container.md`.
  - `.agents/skills/discovery/SKILL.md`.
- Users: anyone who relied on `./config.cue`, on `--ivk-config` failing open, or on `config set` while `INVOWK_CONFIG` is exported. No schema change, no new dependency.
- Out of scope: the formal "trust sources" model (separate later change), `/etc/invowk` system config and ownership checks (Issue 3b), the CLI exit-code contract beyond exit 1 for explicit-config errors (C1), and exporting the env forms to children (deferred).
