## Context

Config loading today (main `85f1eb7b`, branch `feat/harden-config-trust`):

- `internal/config/config.go:154-224` `loadWithOptions` runs in this order:
  1. `opts.ConfigFilePath` (the `--ivk-config` flag).
  2. Otherwise `<ConfigDir>/config.cue` (`configDirFrom`, `config.go:88-118`, which honours `XDG_CONFIG_HOME`, `APPDATA` and macOS Application Support).
  3. Otherwise `config.cue` relative to the cwd, or `opts.BaseDir` (`config.go:196-211`).
  4. Otherwise defaults.
- `LoadOptions.BaseDir` (`internal/config/provider.go:23-26`, validated at `:81-85`) exists only to feed step 3. Production code never sets it. It appears only in tests.
- `internal/app/commandsvc/config_fallback.go:20-60` `LoadConfigWithFallback` wraps every discovery-path load:
  - A missing explicit file becomes a **warning** plus defaults (`:27-31`). `tests/cli/testdata/config_override_flag.txtar` Test 4 asserts this.
  - An invalid explicit file becomes an "error" diagnostic plus defaults.
  - Both are non-fatal: `cmd/invowk/app.go:416-430` only renders them.
- The `--ivk-config` value lives in `rootFlags.configPath` (`cmd/invowk/root.go:34-35,66`). It is threaded through the context (`internal/app/commandadapters/discovery.go:143-170`, `cmd/invowk/app.go:361-367`). It is also pre-scanned from raw argv before Cobra parses flags, so dynamic command registration sees it (`cmd/invowk/cmd.go:144-149,256-283`).
- Other config consumers already fail closed on any provider error: `audit` (`internal/audit/scanner.go:122-129`) and LLM resolution (`internal/app/llmconfig/resolve.go:108-114`, via `cmd/invowk/llm_flags.go:80-93`). With env resolution in the provider (D2), they inherit `INVOWK_CONFIG` and its fail-closed behaviour with no further change. `loadUIConfig` (`cmd/invowk/cmd.go:375-383`) discards the discovery config diagnostics and must propagate the new error.
- Callers that bypass the flag:
  - `invowk validate` calls `contextWithConfigPath(ctx, "")` (`cmd/invowk/validate.go:106`).
  - `config show`, `config dump` and `config set` call `LoadOptions{}` (`cmd/invowk/config.go:102,117,273`).
  - `config path` prints only `ConfigDir()` (`cmd/invowk/config.go:250-269`).
  - `config set` saves with `config.Save(cfg, "")` to the user file. So today `config set` run inside a repo that has `./config.cue` copies that repo config into the user config.
- Discovery (`internal/discovery/discovery_files.go:142-209`) checks sources in this order:
  1. The cwd invowkfile.
  2. Cwd `*.invowkmod` modules and their vendored modules.
  3. Config `includes`.
  4. Provisioned modules.
  5. Provisioned global modules.
  6. `~/.invowk/cmds`.
  
  Steps 1 and 2 run only when `d.baseDir != ""` (`:155`). `WithBaseDir("")` sets `baseDirSet=true`, which skips `os.Getwd()` (`internal/discovery/discovery.go:139-145,225-239`). The only other `baseDir` uses are those two calls (`discovery_files.go:156,161`).
- `DiscoveryService.newDiscovery` (`internal/app/commandadapters/discovery.go:397-415`) is the production construction point. `internal/audit/scan_context.go:576-584` builds its own discovery rooted at the audit target.
- House conventions:
  - Root flags use the `--ivk-` prefix (`root.go:65-67`, `cmd.go:134-139`).
  - Env overrides use the `INVOWK_` prefix and come below an explicit flag and above config, as in `INVOWK_LLM_URL` (`internal/app/llmconfig/resolve.go:326-339`). There `getenv` is injected (`resolve.go:74,269-272`) and `ChangedFlags` gates the override.
- Nested-invocation env filter: `internal/runtime/runtime.go:675-705` (`shouldFilterEnvVar`) drops `INVOWK_ARG_*`, `INVOWK_FLAG_*`, `INVOWK_SSH_*`, `INVOWK_TUI_*` and the metadata variables. The container runtime builds env with `EnvInheritNone` (`internal/runtime/container_exec.go:127`).

## Goals / Non-Goals

**Goals:**
- No config ever comes from the cwd.
- `INVOWK_CONFIG` names the config file. Precedence is flag > env > user file > defaults, the same for every command.
- Explicit config sources fail closed.
- `--ivk-no-cwd` / `INVOWK_NO_CWD` remove the cwd command sources (invowkfile and sibling modules) for every command-source discovery flow.
- One migration section, bundled into `v0.17.0`.

**Non-Goals:**
- An opt-in "trust this directory's config" mode (rejected by the maintainer).
- `/etc/invowk/config.cue`, a system config, or owner and permission checks under `euid==0` (Issue 3b, deferred).
- The formal "trust sources" model, covering which files may add commands or change config. It is planned as a separate change and is not part of this one.
- Exit-code numbering. This change only says "non-zero"; the 1-versus-2 contract belongs to C1 `cli-exit-code-contract`.
- Making an invalid *user* config file fatal. Falling back to defaults drops capabilities rather than adding them, so it stays as it is.
- Changing what `audit`, `module *`, `init` or `validate <path>` operate on.
- Making an invalid cwd invowkfile fatal (N3, owned by C3).
- Passing `--ivk-no-cwd` into containers.

## Decisions

### D1. Remove the cwd fallback outright, and remove `LoadOptions.BaseDir`

Delete `config.go:196-211`, `LoadOptions.BaseDir` and its validation (`provider.go:23-26,81-85`). Update the `LoadOptions.Validate` doc comment ("All three fields").

- Alternatives considered:
  - An opt-in fallback (`trust_cwd_config`, or an env var) — rejected by the maintainer, because one env var in a CI image would bring the injection back.
  - Keeping `BaseDir` as dead API — rejected, because it would only document a source that no longer exists.

### D2. Resolve `INVOWK_CONFIG` inside the config provider, not at the CLI boundary

`fileProvider` gets an injectable `getenv` (`NewProvider()` uses `os.Getenv`; tests use a `NewProviderWithEnv`-style option). `loadWithOptions` resolves in this order:

| Order | Source | Condition | Missing / invalid |
|---|---|---|---|
| 1 | `--ivk-config` | `opts.ConfigFilePath != ""` | fatal `FileNotFoundError` / `LoadError` tagged `SourceFlag` |
| 2 | `INVOWK_CONFIG` | `getenv("INVOWK_CONFIG") != ""` and `!opts.UserConfigOnly` | non-absolute → fatal `InvalidConfigEnvError`; missing/invalid → fatal, tagged `SourceEnv` |
| 3 | user file | `<ConfigDir>/config.cue` exists | `LoadError` (caller falls back to defaults, as today) |
| 4 | defaults | — | — |

Lower-precedence sources are never evaluated. When the flag is set, `INVOWK_CONFIG` is neither read nor validated.

`LoadResult` gains `Source` (a value type `ConfigSourceKind`: `flag|env|user|default`, with `Validate()`), and `SourcePath` keeps its meaning. The error types carry the source kind so the CLI can name the flag or the variable.

- Rationale: callers already bypass `rootFlags.configPath` (`validate.go:106`, `config.go:102,117`). Putting the env lookup in the provider covers them all at one point, and the precedence can be unit-tested without touching process env.
- Alternative: default `rootFlags.configPath` from env in `PersistentPreRunE` and in `explicitConfigPathFromArgs`. Rejected: it misses `LoadOptions{}` callers, and it erases the flag/env distinction that the error messages need.
- `UserConfigOnly` (new `LoadOptions` bool) is set by `config set`. It skips step 2 and requires `ConfigFilePath == ""`, so `set` never uses an explicitly selected file as its base (D6).

### D3. Absolute-path requirement for `INVOWK_CONFIG` only

The value is checked with `filepath.IsAbs` for the host OS. A relative env value would make the config depend on the cwd again: an `.envrc` or CI default such as `INVOWK_CONFIG=config.cue` would pick up whichever repo you are in. `--ivk-config` stays relative-capable, because it is typed on that invocation. `config_override_flag.txtar` uses relative paths, and the user's intent is explicit.

- Alternative: resolve relative values against `$HOME`. Rejected as surprising.

### D4. Explicit-source errors are fatal everywhere (BREAKING, second facet)

`LoadConfigWithFallback` (`config_fallback.go:27-44`) stops returning defaults plus a diagnostic when `configPath != ""` or the provider reports `SourceEnv`. It returns an error, and the CLI turns that error into a non-zero exit before any discovery or execution. The `DiscoveryService.loadConfig` cache (`commandadapters/discovery.go:354-380`) caches and returns the error. `validateCommandTree`/`listCommands` and `registerDiscoveredCommands` must surface it. In the pre-parse registration path, the error is deferred and reported from `RunE`, so `--help` and completion fail in a predictable way.

The user-file path (no explicit source) keeps the current diagnostic-plus-defaults behaviour.

- Rationale: the task requires `INVOWK_CONFIG` to fail loudly. A strict env var next to a lenient flag would be incoherent, and a wrapper that runs `invowk -c /etc/x.cue …` with a typo silently loses its `includes`. Since we are already breaking things for `v0.17.0`, both are fixed together.
- Consequence: `config_override_flag.txtar` Test 4 flips to `! exec` with an error on stderr. Go tests `cmd/invowk/cmd_context_test.go:456` and `cmd_execute_runtime_test.go:91` (config-load diagnostics) are reviewed and updated.
- Alternative: keep the flag lenient and make only the env strict. Rejected for the reason above. It is listed as an open question so the maintainer can confirm.

### D5. `--ivk-no-cwd` reuses `WithBaseDir("")`

- New root persistent flag `--ivk-no-cwd` (bool) next to `--ivk-config` in `root.go:65-67`, stored in `rootFlags.noCwd`.
- Effective value: if the flag was `Changed`, the flag's value, and the env is not parsed; otherwise `INVOWK_NO_CWD` parsed with `strconv.ParseBool`. Empty means false; any other value is an error that names the variable. The switch is resolved only in the flows listed below, so an invalid value cannot break `--version` or `config path`. This is the same "explicit flag beats env" gate as `llmconfig` `ChangedFlags`.
- Threaded like the config path: `ContextWithNoCwd` / `NoCwdFromContext` in `commandadapters/discovery.go`. `DiscoveryService.newDiscovery` appends `discovery.WithBaseDir("")` when it is set. This overrides an injected `s.baseDir` and is applied last. No change to `internal/discovery` is needed: `discovery_files.go:155` already skips both cwd sources, and `New` skips `os.Getwd()` when `baseDirSet`. The cache key must include the flag, or each context gets its own cache, so a no-cwd result is never reused for a cwd result.
- Pre-parse: add `explicitNoCwdFromArgs(args)` next to `explicitConfigPathFromArgs` (`cmd.go:256-283`). It accepts `--ivk-no-cwd`, `--ivk-no-cwd=<bool>` and env fallback, and stops at `--`. `firstTopLevelToken` already skips unknown `-…` tokens (`cmd.go:315-316`), so `shouldRegisterDiscoveredCommands` needs no change. A test pins this.
- Not translated to env for children. Nested invocations see the switch only when the user set `INVOWK_NO_CWD` (D8).

Per-command scope:

| Command / flow | Effect of `--ivk-no-cwd` | Mechanism |
|---|---|---|
| `invowk cmd` list / run / `@src` / `--ivk-from` / `--ivk-dry-run` | cwd invowkfile and cwd modules are skipped | `DiscoveryService` context |
| dynamic leaf registration (`cmd.go:144-149`) | same | `explicitNoCwdFromArgs` |
| completion `__complete cmd …` | same | goes through the same registration and `completeCommands` |
| `--ivk-watch` | discovery is scoped. The watch root is the resolved command's own dir (`commandsvc/watch.go:196-204`), so it is unchanged | — |
| `invowk validate` (no arg) | workspace validation without cwd sources. The header states "current-directory sources disabled" | `DiscoveryService` |
| `invowk validate <path>` | unaffected (explicit target) | path validation does not use discovery |
| `invowk audit [path]` | unaffected; the path (default `.`) is the scan target, not a command source (`audit.go:226`, `scan_context.go:576-584`) | — |
| `invowk agent cmd create/change --verify` | rejected with an error. Verify dry-runs `@invowkfile` from the cwd (`agent.go:690-721`) | check before generation |
| `invowk internal check-cmd` (hidden) | scoped (`internal_check_cmd.go:28-33`) | `DiscoveryService` |
| `invowk module list` | cwd modules omitted; lists the modules that would provide commands (`cmd/invowk/module.go:102` → `DiscoveryService.DiscoverModules`, `commandadapters/discovery.go:319`) | `DiscoveryService` |
| other `module` subcommands, `init`, `config *`, `tui *` | unaffected (operate on `./invowkmod.cue` or targets, `module_deps.go:172…`) | — |

- Alternative: a discovery-level `WithoutCurrentDir()` option. Not needed now, because `WithBaseDir("")` already gives exactly this behaviour. It can be revisited if the trust-sources model needs a named source set.
- Alternative: make `--ivk-no-cwd` also drop `includes`, a "trusted only" mode. Rejected: includes come from the selected config, which this change makes trustworthy.

### D6. `config` subcommands

- `newConfigCommand(app)` gains `rootFlags`.
- `config show` and `config dump` use the standard precedence. `show` prints the source kind (for example "Config file: /x.cue (from INVOWK_CONFIG)"), or "defaults".
- `config path` prints the config directory, the *user* config file, and an extra "Effective config file: <path> (from --ivk-config|INVOWK_CONFIG[, missing])" line when the flag or env selects another file. It does not load or validate that file, so it still works when the variable is broken. A relative `INVOWK_CONFIG` is printed with an "invalid: must be absolute" marker instead of failing. The commands dir and host key lines are unchanged.
- `config set` loads with `UserConfigOnly` and saves to the user file. If the flag or env is also in effect, it prints a one-line note that the edit does not change the effective file.
- `config init` is unchanged (user file).
- Also fix the `config` Long help (`config.go:52-57`), which hard-codes the Linux path, and the `--ivk-config` usage string (`root.go:66`), which should mention `INVOWK_CONFIG`.

### D7. `validate` honours `--ivk-config`

Replace `contextWithConfigPath(cmd.Context(), "")` (`validate.go:106`) with the root flag value. This needs `rootFlags` in `newValidateCommand`. Without it, `INVOWK_CONFIG` would apply to `validate` while `--ivk-config` would not, which contradicts the precedence table.

### D8. Env inheritance by nested invocations

Do not add `INVOWK_CONFIG` or `INVOWK_NO_CWD` to `shouldFilterEnvVar`. A script that calls `invowk cmd …` keeps the caller's trust posture, which is the safe direction. Containers never inherit host env (`container_exec.go:127`), so a host-only absolute `INVOWK_CONFIG` cannot reach a container and break nested calls there.

## Blast radius (verified)

- Go tests that exercise the removed fallback or `BaseDir`:
  - `internal/config/config_test.go`: `BaseDir` at lines 223, 326, 365, 391, 421, 451, 485, 559, 578, 594, 618, 634, and the "reports local fallback source" case at `:607-626`.
  - `internal/config/config_mutation_test.go:107-155` (`TestConfigMutationLoadSourcePrecedence`, both subtests).
  - `internal/config/provider_test.go:22-110` (`BaseDir` validation cases).
  
  Benchmarks and `cmd/invowk/cmd_app_discovery_cache_test.go:121` use `discovery.WithBaseDir`, which is unaffected.
- CLI txtar:
  - No archive places `config.cue` at `$WORK` root (`grep -- '-- config.cue --'` finds nothing). The only config fixture is `custom-config/config.cue` in `config_override_flag.txtar`.
  - That file's Test 4 (`:24-28`) flips.
  - `config_path.txtar` / `config_show.txtar` / `config_init.txtar` assert `stdout 'config.cue'` and stay valid, but `config_path` gains a test for the effective line.
  - testscript builds a fresh env (WORK, PATH, HOME=/no-home, TMPDIR, plus only `GOCOVERDIR`/`GORACE` passed through; `github.com/rogpeppe/go-internal/testscript/testscript.go:489-520`), so ambient `INVOWK_*` cannot leak into txtar tests. Go unit tests do inherit the env, so env access must be injected.
- Repo dogfooding: the repo root has no `config.cue`. `samples/` and `modules/` do not reference a cwd config. Container provisioning reads only the loaded `Config` (`internal/provision/config.go:36-38`), so there is no cwd dependency.
- Mutation baseline: `tools/mutation/baselines/root-baseline.json` has 0 survivors for `internal/config/config.go`. No baseline churn is expected. Mutation targets are not run in this workflow.
- Docs:
  - `website/docs/configuration/overview.mdx:21` and the pt-BR copy (`website/i18n/pt-BR/docusaurus-plugin-content-docs/current/configuration/overview.mdx`).
  - `website/docs/reference/cli.mdx:11-21` Global Flags and the pt-BR copy.
  - README `## Configuration` (`README.md:2506-2512`) and the flags example (`:2685-2692`).
  - `cmd/invowk/root.go:66` usage and `cmd/invowk/config.go:52-57` Long.
  - `docs/diagrams/c4/container.d2:419-428` (config_file tooltip), `docs/diagrams/flowcharts/discovery-flow.d2` (cwd priority, now conditional), `docs/diagrams/sequences/execution-main.d2:38-41` (config load can now abort), `docs/architecture/flowchart-discovery.md:23` and `c4-container.md:93`.
  - `.agents/skills/discovery/SKILL.md` (source table, lines 30-56).
  - The current Snippet data in `website/src/components/Snippet/data/config.ts` is updated only if a snippet shows the flag table. `versions/*` and `versioned_docs` are never edited.

## Risks / Trade-offs

- [Users relying on `./config.cue` lose their config silently, since there is no error and defaults apply] → Migration notes, and a release-notes callout. Add a **verbose-mode-only** hint when the cwd contains `config.cue` and no explicit source is set? Rejected: probing the cwd again for a file we no longer trust adds a code path that the formal model would then have to cover. Docs only.
- [Scripts using `-c missing.cue` that worked by accident now fail] → Intended. The migration notes state it.
- [Developers with `INVOWK_CONFIG` exported break Go unit tests] → The provider takes an injected `getenv`. Tests never read the process env.
- [Cache reuse across no-cwd and cwd contexts] → Unit test that one context with no-cwd never sees cwd commands, even after a cached cwd lookup.
- [Pre-parse argv scan drifts from Cobra parsing (`-c` short forms, `--`)] → Table-driven tests for `explicitNoCwdFromArgs`, mirroring the existing `explicitConfigPathFromArgs` tests.
- [A discovery call site that skips the no-cwd context silently brings the cwd back] → Every `app.Discovery.*` call in `cmd/invowk` is listed in task 4.3 (`cmd.go:376,398`, `cmd_discovery.go:38,305,363`, `internal_check_cmd.go:33`, `module.go:102`, `validate.go:125`). A unit test drives each flow with an injected cwd invowkfile under no-cwd. The context is set once at the earliest entry (RunE/PersistentPreRunE), so later helpers inherit it.
- [The 1000-line limit] → `internal/config/config.go` has 586 lines and `config_test.go` has 691. New tests go in a new `config_source_test.go` if needed.
- [goplint findings on new types (`ConfigSourceKind`, env error)] → Add `Validate()`/`String()` per the typesystem skill. New findings are triaged with the user (checklist step 8).

## Migration Plan

- Ship in `v0.17.0`, bundled with C1 and F12. The release notes take the migration text from `proposal.md` verbatim.
- Rollback: revert the squash commit. No persisted state or schema changes.

## Open Questions

1. D4: confirm that `--ivk-config` also becomes fail-closed (Test 4 flips), not only `INVOWK_CONFIG`.
2. D3: is rejecting relative `INVOWK_CONFIG` acceptable, or should relative values be allowed for direnv-style setups?
3. D6/D7: fixing `validate` and `config show/dump/path` to honour `--ivk-config` fixes a pre-existing gap. Confirm it belongs in this change rather than a separate fix.
4. The flag forms (`-c`, `--ivk-no-cwd`) do not reach nested `invowk` calls made by scripts. Only the env vars do (D8). Issue 7's wrapper (`exec invowk -c <cfg> --ivk-no-cwd cmd cst "$@"`) would lose both settings in nested calls. Should the wrapper docs (C4) recommend the env forms, or should invowk export `INVOWK_CONFIG`/`INVOWK_NO_CWD` to native/virtual children when the flags are used?
5. Mirror policy for `discovery_no_cwd.txtar`: add a mirror exemption to `.agents/rules/testing.md` (task 5.5), or write `virtual_`/`native_` mirrors instead?
6. Should `--ivk-no-cwd` emit a verbose-mode line ("current-directory sources disabled") in `invowk cmd` listings, the way `validate` does?
