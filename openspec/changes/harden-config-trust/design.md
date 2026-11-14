## Context

Config loading today (branch `feat/harden-config-trust`, rebased on main `c0a000da`; Go line numbers re-checked after the rebase):

- `internal/config/config.go:154-224` `loadWithOptions` runs in this order:
  1. `opts.ConfigFilePath` (the `--ivk-config` flag, `:164-175`).
  2. Otherwise `<ConfigDir>/config.cue` (`:177-194`). `configDirFrom` (`config.go:89-119`) honours `XDG_CONFIG_HOME` on Linux, `APPDATA`/`USERPROFILE` on Windows and `~/Library/Application Support` on macOS.
  3. Otherwise `config.cue` relative to the cwd, or `opts.BaseDir` (`config.go:196-211`).
  4. Otherwise defaults.
- `LoadOptions.BaseDir` (`internal/config/provider.go:23-26`, validated at `:81-85`) exists only to feed step 3. A repo-wide grep finds no production caller that sets it; every `config.LoadOptions{` literal outside tests (`cmd/invowk/audit.go:292`, `cmd.go:382`, `config.go:102,117,273`, `internal/app/commandsvc/config_fallback.go:22`, `internal/app/llmconfig/resolve.go:109`) sets at most `ConfigFilePath`.
- Error types: `FileNotFoundError` and `LoadError` (`config.go:44-55`, `Error`/`Unwrap` at `:65-75`), built by `cueLoadError` (`:264-269`). `ErrConfigFileNotFound` and `ErrConfigLoadFailed` are the sentinels (`:38-41`).
- `internal/app/commandsvc/config_fallback.go:20-60` `LoadConfigWithFallback` wraps every discovery-path load:
  - A missing explicit file becomes a **warning** plus defaults (`:27-31`). `tests/cli/testdata/config_override_flag.txtar` Test 4 (`:24-28`) asserts this.
  - An invalid explicit file becomes an "error" diagnostic plus defaults (`:27-44`).
  - Both are non-fatal: diagnostics are only rendered (`cmd/invowk/app.go:416-430`).
- Five signatures carry config as `(cfg, diags)` with no error, so the fail-closed change must touch all of them:
  - `commandsvc.ConfigFallbackFunc` (`internal/app/commandsvc/service.go:55-57`), wired at `cmd/invowk/app.go:147`;
  - `commandsvc.Service.loadConfig` (`service.go:594-602`), called at `:382`;
  - `commandadapters.DiscoveryService.loadConfig` (`internal/app/commandadapters/discovery.go:354-380`), whose request cache stores `cfg`/`cfgDiags`/`hasConfig`;
  - `DiscoveryService.LoadConfigForCommand` (`discovery.go:242-245`), the implementation of `commandsvc.ConfigAwareCommandDiscovery` (`internal/app/commandsvc/types.go:229-233`);
  - the anonymous `LoadConfig(context.Context) (*config.Config, []discovery.Diagnostic)` interface in `loadUIConfig` (`cmd/invowk/cmd.go:375-383`), which discards the diagnostics.
- The `--ivk-config` value lives in `rootFlags.configPath` (`cmd/invowk/root.go:34-35,66`). It is threaded through the context by `ContextWithConfigPath` (`internal/app/commandadapters/discovery.go:159-162`, wrapped by `cmd/invowk/app.go:361-363`) and `DiscoveryRequestScope.Begin` (`discovery.go:143-145`). It is also pre-scanned from raw argv before Cobra parses flags, so dynamic command registration sees it (`cmd/invowk/cmd.go:144-149`, `explicitConfigPathFromArgs` at `:256-283`).
- Dynamic registration is best-effort: `registerDiscoveredCommands` returns silently on any discovery error (`cmd/invowk/cmd_discovery.go:36-43`). Completion returns `ShellCompDirectiveError` on a discovery error (`cmd_discovery.go:302-308`).
- Errors returned from `RunE` exit 1 unless they are an `ExitError` (`cmd/invowk/root.go:86-105`).
- Other config consumers already fail closed on any provider error: `audit` (`internal/audit/scanner.go:122-129`, options built at `cmd/invowk/audit.go:244,292-294`) and LLM resolution (`internal/app/llmconfig/resolve.go:108-114`, via `cmd/invowk/llm_flags.go:69-95` and `agent.go:515-528`). With env resolution inside the provider (D2), they inherit `INVOWK_CONFIG` and its fail-closed behaviour with no further change.
- Callers that bypass the flag:
  - `invowk validate` calls `contextWithConfigPath(ctx, "")` (`cmd/invowk/validate.go:106`).
  - `config show`, `config dump` and `config set` load with `LoadOptions{}` (`cmd/invowk/config.go:102,117,273`).
  - `config path` prints only `ConfigDir()`-derived paths (`cmd/invowk/config.go:250-269`).
  - `config set` saves with `config.Save(cfg, "")` to the user file (`config.go:290`). So today `config set` run inside a repo that has `./config.cue` (and no user file) copies that repo config into the user config.
- Discovery (`internal/discovery/discovery_files.go:142-209`) checks sources in this order:
  1. The cwd invowkfile.
  2. Cwd `*.invowkmod` modules and their vendored modules.
  3. Config `includes`.
  4. Provisioned modules.
  5. Provisioned global modules.
  6. `~/.invowk/cmds`.

  Steps 1 and 2 run only when `d.baseDir != ""` (`:155`); the only other `baseDir` reads in the package are those two calls (`:156,161`) and the constructor. `WithBaseDir("")` sets `baseDirSet=true`, which skips `os.Getwd()` (`internal/discovery/discovery.go:136-145,226-240`).
- `DiscoveryService.newDiscovery(cfg)` (`internal/app/commandadapters/discovery.go:397-415`) is the production construction point. It takes no context. `internal/audit/scan_context.go:576-584` builds its own discovery rooted at the audit target.
- Every `app.Discovery.*` call in `cmd/invowk` (non-test): `cmd.go:376,398`, `cmd_discovery.go:38,305,363`, `internal_check_cmd.go:33`, `module.go:102`, `validate.go:125`. `contextWithConfigPath(…, rootFlags.configPath)` call sites: `cmd_discovery.go:37,87,144,304`, `cmd_execute_helpers.go:229`, `cmd_watch.go:68`; `cmd_execute_helpers.go:314` reads it back.
- `firstTopLevelToken` (`cmd/invowk/cmd.go:289-320`) skips any unknown `-…` token (`:313-314`), so a new bool root flag needs no change there.
- Verbose listing prints a "Discovery Sources" section on stdout (`cmd/invowk/cmd_discovery.go:383,402-…`). Workspace validation prints a header with the cwd path on stdout (`cmd/invowk/validate.go:131-133`).
- House conventions:
  - Root flags use the `--ivk-` prefix (`root.go:65-67`).
  - Env overrides use the `INVOWK_` prefix and come below an explicit flag and above config, as in `INVOWK_LLM_URL` (`internal/app/llmconfig/resolve.go:326-339`). There `getenv` is injected (`resolve.go:74,269-272`) and `ChangedFlags` gates the override.
- Child environment:
  - `FilterInvowkEnvVars`/`shouldFilterEnvVar` (`internal/runtime/runtime.go:657-672,676-727`) drop `INVOWK_ARG_*`, `INVOWK_FLAG_*`, `INVOWK_SSH_*`, `INVOWK_TUI_*`, the metadata variables and `ARGC`/`ARGn`. Every other `INVOWK_*` variable passes.
  - `buildHostEnv` (`internal/runtime/env.go:70-111`) applies that filter, then the resolved `env_inherit_mode` (`none` returns nothing, `allow` keeps only allowlisted names). The mode comes from the runtime default, then the implementation's runtime block, then CLI overrides (`resolveEnvInheritConfig`, `env.go:30-60`).
  - The container runtime passes `EnvInheritNone` as its *default* (`internal/runtime/container_exec.go:127`, `env_builder.go:34`); `resolveEnvInheritConfig` can still override it. How the SSH callback path builds its env was not checked (decision 4 defers it).
- CLI tests: testscript sets `HOME=$WORK`, and on Windows `APPDATA=$WORK/appdata` and `USERPROFILE=$WORK` (`tests/cli/cmd_test.go:41-55`). Existing config tests set `env XDG_CONFIG_HOME=$WORK/.config` (`config_path.txtar:5`, `config_dump.txtar:5`). testscript has no custom commands here (`cmd_test.go:199-204`) and does not expand `$WORK` inside archive files, so no txtar uses an absolute `includes` path today.
- Mirror checks: `TestShRuntimeMirrorCoverage` and `TestVirtualNativeCommandPathAlignment` (`tests/cli/runtime_mirror_test.go:63,156`). The second compares the `exec invowk …` command paths of each pair (`:212-280`); its normaliser treats the token after any `--ivk-*` flag without `=` as the flag's value (`:262-268`).

## Goals / Non-Goals

**Goals:**
- No config ever comes from the cwd.
- `INVOWK_CONFIG` names the config file. Precedence is flag > env > user file > defaults, the same for every config-reading command.
- Explicit config sources fail closed with exit code 1.
- `--ivk-no-cwd` / `INVOWK_NO_CWD` remove the cwd command sources (invowkfile and sibling modules) for every command-source discovery flow.
- Migration text for the release notes lives in this change's `proposal.md` "Migration notes" section.

**Non-Goals:**
- An opt-in "trust this directory's config" mode (rejected by the maintainer).
- `/etc/invowk/config.cue`, a system config, or owner and permission checks under `euid==0` (Issue 3b, deferred).
- The formal "trust sources" model, covering which files may add commands or change config. It is planned as a separate change.
- The CLI exit-code contract as a whole (C1 `cli-exit-code-contract`). This change fixes exit 1 for explicit-config errors (decision 1) and leaves the other new failures at "non-zero".
- Making an invalid *user* config file fatal. Falling back to defaults drops capabilities rather than adding them, so it stays as it is.
- Changing what `audit`, `module *` (except `list`), `init` or `validate <path>` operate on.
- Making an invalid cwd invowkfile fatal (N3, owned by C3).
- Exporting `INVOWK_CONFIG`/`INVOWK_NO_CWD` to child processes (decision 4; see "Deferred").

## Maintainer decisions (2026-11-14)

Approved by the maintainer; binding for implementation.

1. `--ivk-config` naming a missing or invalid file **fails**, like `INVOWK_CONFIG`. `tests/cli/testdata/config_override_flag.txtar` Test 4 flips. The fatal explicit-config error exits **1**; C1's exit-code table records it (C1 decision 7).
2. A relative `INVOWK_CONFIG` is **rejected**.
3. `validate` and `config show`/`dump`/`path` **honour** the explicit source (`--ivk-config`/`INVOWK_CONFIG`). `config set` writes only the user file and does not read the explicit source; when an explicit source is active, `config set` **refuses** with an error (otherwise the "copy repo config into user config" bug returns). `config init` stays user-file only.
4. Do **not** export `INVOWK_CONFIG`/`INVOWK_NO_CWD` to child processes in this change. Document the env forms for wrappers (caller-set variables already inherit by default). Recorded under "Deferred" with the two checks that would decide it.
5. Write `virtual_` and `native_` mirrors for `discovery_no_cwd` (discovery tests are mirrored in practice: `native_ambiguity`, `native_multi_source`). No rule edit, no exemption.
6. Add the verbose "current-directory sources disabled" line.
7. (C1 decision 6, applies here too) There is no shared migration file. This change keeps its migration text in `proposal.md` "Migration notes"; the release assembles them.

## Decisions

### D1. Remove the cwd fallback outright, and remove `LoadOptions.BaseDir`

Delete `config.go:196-211`, `LoadOptions.BaseDir` and its validation (`provider.go:23-26,81-85`). Update the `LoadOptions.Validate` doc comment, which counts the fields.

- Alternatives considered:
  - An opt-in fallback (`trust_cwd_config`, or an env var) — rejected by the maintainer, because one env var in a CI image would bring the injection back.
  - Keeping `BaseDir` as dead API — rejected, because it would only document a source that no longer exists.

### D2. Resolve `INVOWK_CONFIG` inside the config provider, not at the CLI boundary

`fileProvider` gets an injectable `getenv`: `NewProvider()` uses `os.Getenv`; tests use `NewProviderWithEnv(getenv)`. Source selection is one pure function, `config.SelectSource(opts LoadOptions, getenv func(string) string) SourceSelection`, used by `loadWithOptions` and by the CLI (`config path`, `config set`). It never touches the filesystem.

| Order | Source | Selected when | Missing / invalid |
|---|---|---|---|
| 1 | `--ivk-config` | `opts.ConfigFilePath != ""` | fatal `ExplicitConfigError{Source: flag}` |
| 2 | `INVOWK_CONFIG` | `getenv("INVOWK_CONFIG") != ""` | relative → fatal `ExplicitConfigError{Source: env}` wrapping `ErrConfigEnvNotAbsolute`; missing/invalid → fatal `ExplicitConfigError{Source: env}` |
| 3 | user file | otherwise, and `<ConfigDir>/config.cue` exists | `LoadError` (caller falls back to defaults, as today) |
| 4 | defaults | otherwise | — |

- Lower-precedence sources are never evaluated. When the flag is set, `INVOWK_CONFIG` is neither read nor validated; when the env is selected, the user file is never stat'ed.
- `SourceSelection` carries `Kind` (`ConfigSourceKind`: `flag|env|user|default`, a value type with `String()`/`Validate()`), the raw `Path`, and `Problem error` (set only for a relative env value). `LoadResult` gains `Source ConfigSourceKind`; `SourcePath` keeps its meaning; `LoadResult.Validate()` checks `Source`.
- `ExplicitConfigError{Source ConfigSourceKind, Path types.FilesystemPath, Err error}` wraps **every** failure of an explicit source: `FileNotFoundError`, `LoadError`, `cfg.Validate()` failures and the relative-env error. `Unwrap() []error` returns `Err` and the new sentinel `ErrExplicitConfig`, so existing `errors.Is(err, ErrConfigFileNotFound)` checks keep working. Callers decide fatality with `errors.Is(err, config.ErrExplicitConfig)`, never by inspecting `configPath` (that would miss the env source).
- Rationale: callers already bypass `rootFlags.configPath` (`validate.go:106`, `config.go:102,117`). Putting the env lookup in the provider covers them all at one point, and the precedence can be unit-tested without touching process env.
- Alternative: default `rootFlags.configPath` from env in each command and in `explicitConfigPathFromArgs`. Rejected: it misses `LoadOptions{}` callers, and it erases the flag/env distinction that the error messages need.
- Alternative: a `LoadOptions.UserConfigOnly` flag for `config set` (earlier draft). Dropped: decision 3 makes `config set` refuse when an explicit source is active, so a plain `LoadOptions{}` load after that check already yields the user file or defaults.

### D3. Absolute-path requirement for `INVOWK_CONFIG` only

The value is checked with `filepath.IsAbs` for the host OS. A relative env value would make the config depend on the cwd again: an `.envrc` or CI default such as `INVOWK_CONFIG=config.cue` would pick up whichever repo you are in. `--ivk-config` stays relative-capable, because it is typed on that invocation; `config_override_flag.txtar` uses relative paths.

- Alternative: resolve relative values against `$HOME`. Rejected as surprising.

### D4. Explicit-source errors are fatal everywhere (BREAKING, second facet)

- `ConfigFallbackFunc` becomes `func(ctx, provider config.Loader, configPath string) (*config.Config, []Diagnostic, error)`. `LoadConfigWithFallback` returns `(nil, nil, err)` when `errors.Is(err, config.ErrExplicitConfig)`; otherwise it keeps today's defaults-plus-diagnostic behaviour (user file).
- `Service.loadConfig`, `DiscoveryService.loadConfig`, `DiscoveryService.LoadConfigForCommand`, the `ConfigAwareCommandDiscovery` interface and the `loadUIConfig` interface all gain the `error` result. The request cache gains a `cfgErr error` field next to `cfg`/`cfgDiags`, so one invocation loads config once and every later lookup returns the same error.
- Every discovery entry (`DiscoverCommandSet`, `DiscoverAndValidateCommandSet`, `DiscoverModules`, `GetCommand`) returns the error before constructing a `Discovery`. `Service.Execute` returns it before resolving anything. `loadUIConfig` returns it instead of discarding it.
- The CLI returns the error from `RunE` as a plain error, so `Execute` exits **1** (`root.go:104`). It is rendered with the `config_load_failed` issue template (`internal/issue`) plus a hint per source: "check the --ivk-config path" or "fix or unset INVOWK_CONFIG (it must be an absolute path)".
- What each flow does with a broken explicit source:
  - `invowk cmd <name>`, listing, `--ivk-dry-run`, `--ivk-watch`, workspace `validate`, `module list`, `config show`/`dump`, `audit`, `agent`: exit 1 before discovery or execution.
  - Pre-parse registration: stays best-effort (`cmd_discovery.go:39-43`), registers nothing, and the `cmd` `RunE` then hits the same cached error and exits 1.
  - Completion: returns `ShellCompDirectiveError` with no candidates, as for any discovery error today.
  - `invowk cmd --help`: Cobra renders help without the discovered subcommands and exits 0, because help never runs `RunE`. This is documented, not changed.
  - `config path`: never loads, so it still works (D6).
- The user-file path (no explicit source) keeps the current diagnostic-plus-defaults behaviour.
- Consequence: `config_override_flag.txtar` Test 4 flips to `! exec`. Go tests that assert the old behaviour are updated: `cmd/invowk/cmd_app_additional_test.go:131-210` (`TestLoadConfigWithFallback*`), `cmd/invowk/cmd_context_test.go:~440-460` (config-load diagnostic count) and `cmd/invowk/cmd_execute_runtime_test.go:38-41,~80-95` (`testConfigFallback` wrapper and diagnostic assertions).
- Rationale: a strict env var next to a lenient flag would be incoherent, and a wrapper that runs `invowk -c /etc/x.cue …` with a typo silently loses its `includes`. Decision 1 confirms both are fixed together.

### D5. `--ivk-no-cwd` reuses `WithBaseDir("")`

- New root persistent flag `--ivk-no-cwd` (bool) next to `--ivk-config` in `root.go:65-67`, stored in `rootFlags.noCwd`.
- Effective value: `resolveNoCwd(flagSet *pflag.FlagSet, rootFlags, getenv) (on bool, origin noCwdOrigin, err error)` in `cmd/invowk`. If the flag was `Changed`, its value decides and the env is not parsed (origin `--ivk-no-cwd`). Otherwise `INVOWK_NO_CWD` is parsed with `strconv.ParseBool` (origin `INVOWK_NO_CWD`); empty means off; any other value is an `InvalidNoCwdEnvError` (sentinel `ErrInvalidNoCwdEnv`) that names the variable and the accepted values. It is called only from the affected flows (table below), so an invalid value cannot break `--version` or `config path`. Same "explicit flag beats env" gate as `llmconfig` `ChangedFlags`.
- `App` gains an injected `Getenv func(string) string` (default `os.Getenv`) so `cmd/invowk` tests never read the process env; the provider is constructed from the same function.
- Threading: `commandadapters.ContextWithNoCwd(ctx, on)` / `NoCwdFromContext(ctx)`. Because `newDiscovery(cfg)` has no context, its signature becomes `newDiscovery(ctx, cfg)`; when `NoCwdFromContext(ctx)` is true it appends `discovery.WithBaseDir("")` **last**, overriding an injected `s.baseDir`. No change to `internal/discovery` is needed: `discovery_files.go:155` already skips both cwd sources, and `New` skips `os.Getwd()` when `baseDirSet`.
- Cache strategy (one only): `discoveryRequestCache` records the `noCwd` value it was created under. `ContextWithDiscoveryRequestCache` reuses a parent's cache only when that value matches `NoCwdFromContext(ctx)`; otherwise it attaches a fresh cache. `ContextWithNoCwd` must therefore be applied before `ContextWithConfigPath`, and a cwd result can never be served to a no-cwd context.
- One cmd-level helper, `contextWithDiscoveryScope(ctx, rootFlags, noCwd bool) context.Context`, replaces the `contextWithConfigPath(…, rootFlags.configPath)` call sites listed in Context, so no entry can set one without the other.
- Pre-parse: `explicitNoCwdFromArgs(args) (value, set bool, err error)` next to `explicitConfigPathFromArgs` (`cmd.go:256-283`). It accepts `--ivk-no-cwd` and `--ivk-no-cwd=<bool>`, before or after `cmd`, and stops at `--`. When the flag is absent the caller falls back to `INVOWK_NO_CWD`. A malformed flag value or an invalid env value makes registration skip (register nothing); the `RunE` path then reports the error.
- Verbose line (decision 6): when the switch is on, `listCommands` prints `current-directory sources disabled (<origin>)` on stdout inside the verbose "Discovery Sources" section (`cmd_discovery.go:402`), and `runWorkspaceValidation` prints the same line on stdout right after the path line (`validate.go:132`), with or without `--ivk-verbose`.
- Not exported to children (D8).

Per-command scope:

| Command / flow | Effect of `--ivk-no-cwd` | Mechanism |
|---|---|---|
| `invowk cmd` list / run / `@src` / `--ivk-from` / `--ivk-dry-run` | cwd invowkfile and cwd modules are skipped | `contextWithDiscoveryScope` |
| dynamic leaf registration (`cmd.go:144-149`) | same | `explicitNoCwdFromArgs` + env |
| completion `__complete cmd …` | same | registration and `completeCommands` (`cmd_discovery.go:302-308`) |
| `--ivk-watch` | discovery is scoped. The watch root is the resolved command's own dir (`internal/app/commandsvc/watch.go:196-227`), so it is unchanged | `cmd_watch.go:68` |
| `invowk validate` (no arg) | workspace validation without cwd sources; header line printed | `validate.go:106,125` |
| `invowk validate <path>` | unaffected (explicit target) | path validation does not use discovery |
| `invowk audit [path]` | unaffected; the path (default `.`, `audit.go:226`) is the scan target, not a command source (`scan_context.go:576-584`) | — |
| `invowk agent cmd create/change --verify` | rejected before generation. Verify dry-runs `@invowkfile` from the cwd (`verifyGeneratedCommand`, `agent.go:688-706`, called for create at `:380-384` and for change at `:422-426`) | check in `validateAgentCmdCreateOptions`/`validateAgentCmdChangeOptions` (`agent.go:390,432`) |
| `invowk agent module * --verify` | unaffected (validates the generated module path) | — |
| `invowk internal check-cmd` (hidden) | scoped | `internal_check_cmd.go:28-33` |
| `invowk module list` | cwd modules omitted (`module.go:102` → `DiscoveryService.DiscoverModules`, `commandadapters/discovery.go:319`) | `contextWithDiscoveryScope` |
| other `module` subcommands, `init`, `config *`, `tui *` | unaffected (operate on `./invowkmod.cue` or explicit targets) | — |

- Exit codes: the invalid-env error and the `--verify` rejection are plain errors (exit 1 today via `root.go:104`; `agent` currently maps its errors to `auditExitError`). Their final numbers belong to C1's table and are reported to C1, not decided here.
- Alternative: a discovery-level `WithoutCurrentDir()` option. Not needed now, because `WithBaseDir("")` already gives exactly this behaviour. It can be revisited if the trust-sources model needs a named source set.
- Alternative: make `--ivk-no-cwd` also drop `includes`, a "trusted only" mode. Rejected: includes come from the selected config, which this change makes trustworthy.

### D6. `config` subcommands

- `newConfigCommand(app)` becomes `newConfigCommand(app, rootFlags)`.
- `config show` and `config dump` load with `LoadOptions{ConfigFilePath: rootFlags.configPath}` (standard precedence, fail closed). `show` prints the source: "Config file: /x.cue (from INVOWK_CONFIG)", "(from --ivk-config)", "(user config)", or "defaults".
- `config path` prints the config directory, the *user* config file (as today), and, when `SelectSource` returns `flag` or `env`, an extra line `Effective config file: <path> (from --ivk-config|INVOWK_CONFIG)` with a `[missing]` marker when the file does not exist and an `[invalid: must be absolute]` marker for a relative env value. It never loads or validates the file and always exits 0 for these cases. The commands dir and host key lines are unchanged.
- `config set` (decision 3): before loading, it calls `SelectSource`. If the kind is `flag` or `env` (any non-empty value, even a relative one), it returns an error such as `config set edits the user config file <user path>, but INVOWK_CONFIG selects <path>; unset INVOWK_CONFIG or edit that file directly` (the flag variant names `--ivk-config`), and reads or writes nothing. Otherwise it loads with `LoadOptions{}` and saves to the user file as today. Plain error → exit 1 today; number recorded for C1.
- `config init` is unchanged (user file only, ignores explicit sources).
- Also fix the `config` Long help (`config.go:54-59`), which gives the Linux path as `~/.config/invowk/config.cue` without `XDG_CONFIG_HOME` and names neither the flag nor the env var, and the `--ivk-config` usage string (`root.go:66`), which should mention `INVOWK_CONFIG` and the OS-specific default.

### D7. `validate` honours `--ivk-config`

Replace `contextWithConfigPath(cmd.Context(), "")` (`validate.go:106`) with `contextWithDiscoveryScope(cmd.Context(), rootFlags, noCwd)`. This needs `rootFlags` in `newValidateCommand`. Without it, `INVOWK_CONFIG` would apply to `validate` while `--ivk-config` would not, which contradicts the precedence. `validate <path>` resolves the switch only for the workspace (no-arg) branch.

### D8. Env inheritance by nested invocations (decision 4)

Do not add `INVOWK_CONFIG` or `INVOWK_NO_CWD` to `shouldFilterEnvVar`, and do not export them. A script that calls `invowk cmd …` keeps the caller's env-form settings under the runtime's normal `env_inherit_mode` rules (`all` passes them, `none` drops them, `allow` passes them only when allowlisted). The flag forms do not reach children; wrappers that need nested calls to keep the posture use the env forms (`INVOWK_CONFIG=/abs/path INVOWK_NO_CWD=1 invowk cmd …`). A comment next to `shouldFilterEnvVar` and a unit test pin this.

## Deferred

- **Exporting `INVOWK_CONFIG`/`INVOWK_NO_CWD` to children when the flags are used** (decision 4). Two checks decide it:
  1. How the container and SSH-callback runtimes build the child env. A host absolute path exported into a container breaks nested calls there under decision 1 (missing explicit file → exit 1). The container default is `EnvInheritNone` but overridable (`env.go:30-60`); the SSH-callback path is unchecked.
  2. Whether exporting would bypass `env_inherit_mode: "none"`/`"allow"`. Injected variables must not override a command's opt-out.

## Blast radius (verified)

- Go tests that exercise the removed fallback or `BaseDir`:
  - `internal/config/config_test.go`: `BaseDir` at lines 223, 326, 365, 391, 421, 451, 485, 559, 578, 594, 618, 634, and the "reports local fallback source" case at `:607-626`.
  - `internal/config/config_mutation_test.go:107-154` (`TestConfigMutationLoadSourcePrecedence`).
  - `internal/config/provider_test.go:22-122` (`BaseDir` in `AllValid`, `InvalidBaseDir` at `:69`, `MultipleInvalid` at `:80`, `MixedEmptyAndInvalid` at `:101`).

  Benchmarks and `cmd/invowk/cmd_app_discovery_cache_test.go:121` use `discovery.WithBaseDir`, which is unaffected.
- Go tests that assert fail-open explicit config: see D4.
- CLI txtar:
  - No archive places `config.cue` at `$WORK` root (`grep -- '-- config.cue --'` finds nothing). The only config fixture is `custom-config/config.cue` in `config_override_flag.txtar`.
  - That file's Test 4 (`:24-28`) flips.
  - `config_path.txtar` / `config_show.txtar` / `config_init.txtar` assert `stdout 'config.cue'` and stay valid.
  - testscript builds a fresh env, so ambient `INVOWK_*` cannot leak into txtar tests. Go unit tests do inherit the env, so env access is injected (D2, D5).
  - Absolute `includes` cannot be written as a txtar fixture (Context); include behaviour under `--ivk-no-cwd` is proven by a Go test with an injected config, and the txtar tests use the user commands dir (`$WORK/.invowk/cmds/…`, since `HOME=$WORK`).
  - Mirrors: the pair `virtual_discovery_no_cwd.txtar`/`native_discovery_no_cwd.txtar` must keep identical `exec invowk …` lines so `TestVirtualNativeCommandPathAlignment` passes. Runtime-independent built-in checks (`validate`, `module list`, `config path`, `__complete`, `audit`) go in a separate non-virtual `no_cwd_builtins.txtar`, which needs no mirror.
- Repo dogfooding: the repo root has no `config.cue`. `samples/` and `modules/` do not reference a cwd config. Container provisioning reads only the loaded `Config` (`internal/provision/config.go:36-38`).
- Formal bindings: `internal/discovery/**` and `internal/runtime/**` are bound (`.agents/rules/formal-bindings.md`; `formal/alloy/ScopeConstruction.als:25` names `discovery_files.go`). This change does not edit `internal/discovery`, and touches `internal/runtime` only with a comment and a test, so no table changes; `python3 scripts/formal.py correspondence` confirms.
- Mutation baseline: `tools/mutation/baselines/root-baseline.json` has 0 survivors for `internal/config/config.go`. Mutation targets are not run in this workflow.
- Docs:
  - `website/docs/configuration/overview.mdx:21` and the pt-BR copy (`website/i18n/pt-BR/docusaurus-plugin-content-docs/current/configuration/overview.mdx`).
  - `website/docs/reference/cli.mdx:11-21` Global Flags (the `--ivk-config` row is `:18`) and the pt-BR copy.
  - Discovery-order lists that name the cwd sources: `website/docs/configuration/options.mdx:52-53`, `reference/config-schema.mdx:72-73`, `core-concepts/commands-and-namespaces.mdx:67-68`, `modules/overview.mdx:42,117-118`, and their pt-BR copies.
  - README `## Configuration` (`README.md:2506-2512`), the flags example (`:2685-2692`) and the discovery-order lists (`:22-23`, `:485`, `:2068-2069`).
  - `cmd/invowk/root.go:66` usage and `cmd/invowk/config.go:54-59` Long.
  - `docs/diagrams/c4/container.d2:419-428` (config_file tooltip), `docs/diagrams/flowcharts/discovery-flow.d2:28-31` (priority 1, now conditional), `docs/diagrams/sequences/execution-main.d2:38-39` (config load can now abort), `docs/architecture/flowchart-discovery.md:23-25` and `docs/architecture/c4-container.md:28,93`.
  - `.agents/skills/discovery/SKILL.md` (precedence table and "Key Behavior", lines 31-58 after the rebase).
  - `website/src/components/Snippet/data/config.ts:35-36` already shows `invowk --ivk-config /path/to/config.cue config show`, which this change makes true; `config.ts:65-71` (`config set`) gains no change. `versions/*` and `versioned_docs` are never edited.

## Risks / Trade-offs

- [Users relying on `./config.cue` lose their config silently, since there is no error and defaults apply] → Migration notes and a release-notes callout. A verbose-mode hint when the cwd contains `config.cue` was rejected: probing the cwd again for a file we no longer trust adds a code path that the formal model would then have to cover. Docs only.
- [Scripts using `-c missing.cue` that worked by accident now fail] → Intended (decision 1). The migration notes state it.
- [`config set` now refuses in shells or CI jobs that export `INVOWK_CONFIG`] → Intended (decision 3); the error names the variable and the migration notes say to unset it or edit the file directly.
- [Developers with `INVOWK_CONFIG`/`INVOWK_NO_CWD` exported break Go unit tests] → The provider and `App` take an injected `getenv`. Tests never read the process env.
- [Cache reuse across no-cwd and cwd contexts] → D5 cache strategy, plus a unit test that a no-cwd context never sees cwd commands after a cached cwd lookup.
- [Pre-parse argv scan drifts from Cobra parsing (`-c` short forms, `--`)] → Table-driven tests for `explicitNoCwdFromArgs`, mirroring the existing `explicitConfigPathFromArgs` tests.
- [A discovery call site that skips the no-cwd context silently brings the cwd back] → All sites go through `contextWithDiscoveryScope`; a unit test drives each flow in the Context list with an injected cwd invowkfile under no-cwd; `app.Discovery\.` is re-grepped before the task closes.
- [Nested calls lose the flag forms] → Documented (D8, decision 4); export is deferred with its deciding checks.
- [The 1000-line limit] → `internal/config/config.go` has 586 lines, `config_test.go` 691, `cmd/invowk/cmd.go` 411. New tests go in `internal/config/config_source_test.go`.
- [goplint findings on new types (`ConfigSourceKind`, `SourceSelection`, `ExplicitConfigError`, `InvalidNoCwdEnvError`, `noCwdOrigin`)] → Add `Validate()`/`String()` per the typesystem skill. New findings are triaged per checklist step 5; inconclusives are never baselined.

## Migration Plan

- Ship in `v0.17.0`, together with the other BREAKING changes (C1, F12). The release notes take this change's text from `proposal.md` "Migration notes" (decision 7); there is no shared migration file.
- Rollback: revert the squash commit. No persisted state or schema changes.

## Open Questions

None. Questions 1–6 of the first draft are answered by the maintainer decisions above (1 → decision 1, 2 → decision 2, 3 → decision 3, 4 → decision 4 and "Deferred", 5 → decision 5, 6 → decision 6).
