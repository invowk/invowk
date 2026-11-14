## Why

When no user config file exists, invowk loads `./config.cue` from the current directory (`internal/config/config.go:196-211`). That is the normal state for root, for CI and for many users. So running invowk inside a cloned repository is enough to load config from that repository. The config controls the following:
- `includes`, which load modules from any absolute path.
- `llm.api.base_url` together with `llm.api.api_key_env`. These send an API key from the user's environment to a host the repository chooses.
- `container.auto_provision.binary_path`, the binary copied into containers.
- `virtual` runtime policy.

This is repository-controlled config injection, and one form of it can exfiltrate credentials. Explicit config sources also fail open: if `--ivk-config` names a missing file, invowk prints a warning and runs with defaults (`internal/app/commandsvc/config_fallback.go:27-44`). There is also no way to say "only trusted sources may add commands", which wrappers and root-run tooling need (CST adoption review, Issue 3a).

## What Changes

- **BREAKING:** invowk no longer reads `config.cue` from the current directory or from any other implicit location. Config comes only from `--ivk-config`, `INVOWK_CONFIG`, or the per-user config file (XDG/APPDATA/Application Support). Otherwise defaults apply. `LoadOptions.BaseDir` is removed.
- **New:** the `INVOWK_CONFIG` environment variable names the config file. Precedence is `--ivk-config` > `INVOWK_CONFIG` > user config file > built-in defaults. An empty value counts as unset. A relative value is rejected.
- **BREAKING:** an explicit config source (`--ivk-config` or `INVOWK_CONFIG`) that is missing, unreadable, invalid CUE or fails validation stops the invocation with a non-zero exit and an actionable error. Today it is a warning or error diagnostic, followed by a silent fallback to defaults. Only the implicit user config file keeps today's behaviour when invalid: a diagnostic, then defaults.
- **New:** the root persistent flag `--ivk-no-cwd` and the env var `INVOWK_NO_CWD` (boolean, `strconv.ParseBool` values) turn off discovery of the current-directory `invowkfile.cue` and of sibling `*.invowkmod` modules. Configured `includes`, `~/.invowk/cmds` and provisioned modules still apply. This affects `invowk cmd` (run, list, completion, dynamic registration, watch, dry-run), workspace `invowk validate`, `invowk module list` and hidden internal discovery commands. Commands that take an explicit target are not affected: `audit [path]`, `validate <path>`, the other `module` subcommands, `init`, `config *`.
- `invowk validate` honours `--ivk-config`; today it hard-codes an empty path (`cmd/invowk/validate.go:106`). `config show` and `config dump` report the effective source. `config path` prints the effective config file. `config set` and `config init` always target the user config file.
- Docs, i18n (pt-BR), README, CLI help, architecture diagrams and the discovery skill are updated. A migration section is added, to be bundled into the `v0.17.0` release notes with the other BREAKING changes (C1 exit codes, F12 admission).

### Migration notes (release-notes text)

> **Config is no longer read from the current directory.** Earlier versions loaded `./config.cue` when no user config file existed. This let any repository you `cd` into change invowk's includes, LLM endpoint and API-key variable, and container provisioning. If you relied on it, choose one of:
> - pass the file explicitly: `invowk --ivk-config ./config.cue cmd …`;
> - export it once per shell or CI job: `export INVOWK_CONFIG="$PWD/config.cue"` (must be an absolute path);
> - move it to your user config file (`invowk config path` shows where).
>
> **Explicit config errors are now fatal.** If `--ivk-config` or `INVOWK_CONFIG` names a file that is missing or invalid, invowk exits non-zero instead of warning and continuing with defaults. Fix the path, or unset the variable.
>
> **New: `--ivk-no-cwd` / `INVOWK_NO_CWD=1`.** These ignore the current directory's `invowkfile.cue` and `*.invowkmod` modules. Only configured `includes` and `~/.invowk/cmds` provide commands. Use them in wrappers, CI and anything run as root: `invowk -c /etc/x/invowk.cue --ivk-no-cwd cmd …`.

## Capabilities

### New Capabilities

- `config-source-resolution`: where invowk may read its configuration from, the precedence order, and how a missing or invalid source is handled. The current directory is never a source.
- `cwd-discovery-scope`: the `--ivk-no-cwd` / `INVOWK_NO_CWD` switch, which command sources it removes, and which commands it applies to.

### Modified Capabilities

(none — no existing spec covers config loading or discovery sources)

## Impact

- Code:
  - `internal/config/config.go`, `provider.go`: fallback and `BaseDir` removed, env resolution added, source kind reported.
  - `internal/app/commandsvc/config_fallback.go`: explicit-source errors become fatal.
  - `internal/app/commandadapters/discovery.go`: no-cwd option.
  - `cmd/invowk/root.go`, `cmd.go`: flag and pre-parse argv scan.
  - `cmd/invowk/validate.go`, `config.go`, `audit.go`, `agent.go`.
- Tests:
  - `internal/config/config_test.go`, `config_mutation_test.go` and `provider_test.go` (cwd-fallback cases).
  - `tests/cli/testdata/config_override_flag.txtar` Test 4 flips from warning to failure.
  - New txtar tests.
- Docs:
  - `website/docs/configuration/overview.mdx:21` and the pt-BR copy.
  - `website/docs/reference/cli.mdx` Global Flags, en and pt-BR.
  - README Configuration and flags sections.
  - `docs/diagrams/c4/container.d2`, `docs/diagrams/flowcharts/discovery-flow.d2`, `docs/architecture/flowchart-discovery.md`.
  - `.agents/skills/discovery/SKILL.md`.
- Users: anyone who relied on `./config.cue` or on `--ivk-config` failing open. No schema change, no new dependency.
- Out of scope: the formal "trust sources" model (separate later change), `/etc/invowk` system config and ownership checks (Issue 3b), and exit-code numbering (C1).
