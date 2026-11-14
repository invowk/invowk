All design decisions are settled (design.md "Maintainer decisions (2026-11-14)"). Names such as `SelectSource` or `contextWithDiscoveryScope` are the design's; keep them unless goplint or lint forces a rename, and never change the behaviour they specify.

## 1. Config provider: remove cwd fallback, add source selection (internal/config)

- [x] 1.1 Delete the cwd/`BaseDir` fallback branch in `loadWithOptions` (`internal/config/config.go:196-211`). Remove `LoadOptions.BaseDir` and its validation (`provider.go:23-26,81-85`). Fix the `LoadOptions.Validate` doc comment.
- [x] 1.2 Add the `ConfigSourceKind` value type (`flag`, `env`, `user`, `default`) with `String()`/`Validate()`. Add `Source ConfigSourceKind` to `LoadResult` and check it in `LoadResult.Validate()`.
- [x] 1.3 Add `SelectSource(opts LoadOptions, getenv func(string) string) SourceSelection` (pure, no filesystem access) per design D2: `flag` when `ConfigFilePath != ""`; else `env` when `INVOWK_CONFIG` is non-empty, with `Problem` = `ErrConfigEnvNotAbsolute` when `!filepath.IsAbs`; else `implicit`, which `loadWithOptions` turns into `user` or `default` from file existence. `SourceSelection` gets `Validate()`.
- [x] 1.4 Add `ExplicitConfigError{Source, Path, Err}` and sentinel `ErrExplicitConfig`; `Unwrap() []error` returns `Err` and the sentinel. Wrap every flag or env failure in it: `FileNotFoundError`, `LoadError`, `cfg.Validate()` failures (including include-collection errors) and the relative-env problem.
- [x] 1.5 Inject `getenv` into `fileProvider`: `NewProvider()` uses `os.Getenv`; add `NewProviderWithEnv(getenv)`. `loadWithOptions` uses `SelectSource` and never evaluates a lower-precedence source (no user-file stat when env is selected).
- [x] 1.6 New `internal/config/config_source_test.go` (SPDX header), all through the injected `getenv`:
  - precedence flag > env > user > defaults, no merging, and `LoadResult.Source` for each;
  - flag masks a relative or missing env value; env masks an invalid user file;
  - empty env is unset;
  - relative env → `ErrExplicitConfig` and `ErrConfigEnvNotAbsolute`;
  - missing, invalid-CUE and failing-`Validate` files for flag and env → `ErrExplicitConfig`, `errors.Is(…, ErrConfigFileNotFound)` still true for missing files;
  - a cwd `config.cue` (with `default_runtime`, `includes`, `llm.api.base_url`, `llm.api.api_key_env`) is ignored: a non-parallel test that `t.Chdir`s into a temp dir, then asserts defaults and `Source == default`;
  - `SelectSource` table tests.
- [x] 1.7 Update the fallback-dependent tests:
  - `config_test.go`: drop `BaseDir` at lines 223, 326, 365, 391, 421, 451, 485, 559, 578, 594, 618, 634; replace "reports local fallback source" (`:607-626`) with "cwd config is not a source".
  - `config_mutation_test.go:107-154`: rewrite `TestConfigMutationLoadSourcePrecedence` around the new precedence.
  - `provider_test.go`: remove `TestLoadOptions_Validate_InvalidBaseDir` (`:69`) and the `BaseDir` parts of `AllValid` (`:22`), `MultipleInvalid` (`:80`) and `MixedEmptyAndInvalid` (`:101`).
- [x] 1.8 Check: `go test ./internal/config/...` passes.

## 2. Fail-closed explicit sources (commandsvc / commandadapters / cmd)

- [x] 2.1 Change `commandsvc.ConfigFallbackFunc` (`internal/app/commandsvc/service.go:55-57`) to return `(*config.Config, []Diagnostic, error)`. `LoadConfigWithFallback` returns `(nil, nil, err)` when `errors.Is(err, config.ErrExplicitConfig)`, and keeps defaults plus a diagnostic for user-file errors. Update the wiring at `cmd/invowk/app.go:147`.
- [x] 2.2 Add the error result to `Service.loadConfig` (`service.go:594-602`, caller `:382`), `commandsvc.ConfigAwareCommandDiscovery` (`types.go:229-233`), `DiscoveryService.loadConfig` (`commandadapters/discovery.go:354-380`) and `DiscoveryService.LoadConfigForCommand` (`:242-245`). Add `cfgErr error` to the request cache so the error is cached with `cfg`/`cfgDiags`.
- [x] 2.3 Return the config error, before constructing a `Discovery`, from `DiscoverCommandSet`, `DiscoverAndValidateCommandSet`, `DiscoverModules` and `GetCommand`; return it from `Service.Execute` before resolution; change the `loadUIConfig` interface and body (`cmd/invowk/cmd.go:375-383`) to return it. Keep pre-parse registration best-effort (`cmd_discovery.go:39-43`) and completion's `ShellCompDirectiveError`.
- [x] 2.4 Render the error with the `config_load_failed` issue template (`internal/issue`) plus a hint per source ("check the --ivk-config path" / "fix or unset INVOWK_CONFIG; it must be an absolute path"). In the `cmd`, `validate`, `module list` and `config` flows return it from `RunE` as a plain error so the process exits 1 (`root.go:104`). Leave `audit` (`auditExitError`) and `agent` (`ExitError` from `resolveLLMForCommand`) on their existing error codes.
- [x] 2.5 Update tests that assert fail-open behaviour: `cmd/invowk/cmd_app_additional_test.go:131-210` (`TestLoadConfigWithFallback*`), `cmd/invowk/cmd_context_test.go:~440-460`, and `cmd/invowk/cmd_execute_runtime_test.go:38-41` (`testConfigFallback`) and `:~80-95`. Add fatal cases for flag and env (missing, invalid CUE, relative env), a user-file fallback case, and a cache test that the error is returned on every lookup of one request.
- [x] 2.6 Check: `go test ./internal/app/commandsvc/... ./internal/app/commandadapters/... && go test -run 'TestLoadConfigWithFallback|TestExecute|Config' ./cmd/invowk/` passes.

## 3. Callers honour the precedence; `config` subcommands

- [x] 3.1 Add `Getenv func(string) string` to the `App` dependencies (default `os.Getenv`) and build the default provider with `config.NewProviderWithEnv(Getenv)` (`cmd/invowk/app.go:118`).
- [x] 3.2 Update the `--ivk-config` usage text (`cmd/invowk/root.go:66`) to mention `INVOWK_CONFIG` and the OS-specific user config file.
- [x] 3.3 `invowk validate`: pass `rootFlags` into `newValidateCommand` and use the root config path instead of `""` (`cmd/invowk/validate.go:106`).
- [x] 3.4 `config` subcommands (`cmd/invowk/config.go`): `newConfigCommand(app, rootFlags)`.
  - `show` (`:102`) and `dump` (`:117`) load with `LoadOptions{ConfigFilePath: rootFlags.configPath}`; `show` prints the source per design D6 ("(from INVOWK_CONFIG)", "(from --ivk-config)", "(user config)", "defaults").
  - `path` (`:250-269`) adds `Effective config file: <path> (from --ivk-config|INVOWK_CONFIG)` with `[missing]` / `[invalid: must be absolute]` markers, using `SelectSource` and a stat only; it exits 0 in those cases.
  - `set` (`:273-290`): call `SelectSource` first; for `flag` or `env` return the refusal error from design D6 without reading or writing any file; otherwise load with `LoadOptions{}` and save to the user file.
  - `init`: unchanged.
  - Rewrite the Long help (`:54-59`): the three OS paths (Linux with `XDG_CONFIG_HOME`), `INVOWK_CONFIG`, `--ivk-config`, and the precedence.
- [x] 3.5 Check that `audit` (`cmd/invowk/audit.go:244,292-294`), `agent` (`agent.go:525,699`), `llm_flags.go:69-95` and `cmd_execute_helpers.go:140,229,271,314` reach the provider with an empty `ConfigFilePath` when no flag is given, so env resolution applies. Add a unit test for audit + injected `INVOWK_CONFIG`.
- [x] 3.6 Unit tests with an injected env: `config show` and `dump` under flag and env; `config show` with a missing explicit file fails; `config path` effective line, `[missing]` and `[invalid: must be absolute]`; `config set` refuses under flag and under env and leaves both files untouched; `config set` without an explicit source writes the user file; `validate` honours `--ivk-config`.
- [x] 3.7 Check: `go test -run 'Config|Validate|Audit' ./cmd/invowk/` passes.

## 4. `--ivk-no-cwd` / `INVOWK_NO_CWD`

- [x] 4.1 Add the root persistent flag `--ivk-no-cwd` (`root.go:65-67`) and `rootFlags.noCwd`. Add `resolveNoCwd` per design D5 (flag when `Changed`; else `strconv.ParseBool(INVOWK_NO_CWD)` via `App.Getenv`; empty is off; else `InvalidNoCwdEnvError` with sentinel `ErrInvalidNoCwdEnv`, naming the variable and the accepted values) and the `noCwdOrigin` value type (`--ivk-no-cwd` / `INVOWK_NO_CWD`).
- [x] 4.2 Add `ContextWithNoCwd`/`NoCwdFromContext` in `internal/app/commandadapters/discovery.go`. Change `newDiscovery(cfg)` to `newDiscovery(ctx, cfg)` and append `discovery.WithBaseDir("")` last when no-cwd is on. Record `noCwd` in `discoveryRequestCache`; `ContextWithDiscoveryRequestCache` reuses a parent cache only when the value matches.
- [x] 4.3 Add `contextWithDiscoveryScope(ctx, rootFlags, noCwd)` in `cmd/invowk/app.go` and use it at every `contextWithConfigPath(…, rootFlags.configPath)` site: `cmd_discovery.go:37,87,144,304`, `cmd_execute_helpers.go:229`, `cmd_watch.go:68`, `internal_check_cmd.go:33`, `validate.go:106` (workspace branch), and the `module list` entry before `module.go:102`. Each affected entry calls `resolveNoCwd` once and returns its error. Re-grep `app.Discovery\.` and `contextWithConfigPath(` before closing the task; no direct `contextWithConfigPath` call may remain in an affected flow.
- [x] 4.4 Add `explicitNoCwdFromArgs(args) (value, set bool, err error)` for the pre-parse registration step (`cmd.go:144-149`): `--ivk-no-cwd`, `--ivk-no-cwd=true|false`, before or after `cmd`, stop at `--`, env fallback when absent; on any error skip registration. Table tests next to the `explicitConfigPathFromArgs` tests, plus a test that `firstTopLevelToken`/`shouldRegisterDiscoveredCommands` skip the flag.
- [x] 4.5 `agent cmd create|change --verify`: call `resolveNoCwd` only when `--verify` is set, and reject (also on a resolution error) in `validateAgentCmdCreateOptions` / `validateAgentCmdChangeOptions` (`cmd/invowk/agent.go:390,432`) when no-cwd is on, before any LLM call; the error names `--verify` and `--ivk-no-cwd`. Unit test that no completer is invoked.
- [x] 4.6 Verbose line (decision 6): when no-cwd is on, `listCommands` prints `current-directory sources disabled (<origin>)` on stdout in the verbose "Discovery Sources" section (`cmd_discovery.go:402`), and `runWorkspaceValidation` prints the same line after the path line (`validate.go:132`) regardless of verbosity.
- [x] 4.7 Unit tests in `cmd/invowk` and `commandadapters`:
  - cwd invowkfile and modules are skipped, while configured `includes` (injected config with an absolute temp path), the user commands dir and provisioned entries are kept;
  - `@invowkfile` gives source-not-found;
  - `--ivk-no-cwd=false` beats `INVOWK_NO_CWD=1`; `--ivk-no-cwd` skips an invalid env;
  - an invalid env value is rejected only by affected flows (`config path` and `--version` succeed);
  - cache isolation: a no-cwd context never sees cwd commands after a cached cwd lookup;
  - each flow in 4.3 under no-cwd with an injected cwd invowkfile;
  - the verbose and validate lines (present when on, absent in non-verbose listing).
- [x] 4.8 Add a comment next to `shouldFilterEnvVar` (`internal/runtime/runtime.go:676`) that `INVOWK_CONFIG` and `INVOWK_NO_CWD` are intentionally not filtered and not exported (design D8). Add a runtime env test: with `env_inherit_mode` `all` both pass with the caller's values; with `none` neither does; the flag forms are never added to the child env.
- [x] 4.9 Check: `go test ./internal/app/commandadapters/... ./internal/runtime/... && go test -run 'NoCwd|ExplicitNoCwd|FirstTopLevel|Agent' ./cmd/invowk/` passes.

## 5. CLI integration tests (tests/cli/testdata, inline CUE)

Each new file starts with `cd $WORK` and `env XDG_CONFIG_HOME=$WORK/.config` like `config_path.txtar`. Fixtures that must not be discovered by accident go in subdirectories. No `ubuntu`/`alpine` images; these tests use no containers.

- [x] 5.1 `config_cwd_ignored.txtar`: in `$WORK/proj/`, `config.cue` sets `default_runtime: "virtual-sh"`, with no user config. Assert `cmd` runs the native implementation, `config show` reports defaults and does not name `proj/config.cue`, and stderr is empty.
- [x] 5.2 `config_env_override.txtar` (Linux user file via `XDG_CONFIG_HOME`; guard OS-specific lines with `[linux]`/`[!windows]` as needed):
  - `env INVOWK_CONFIG=$WORK/env/config.cue` takes effect and a user config at `$WORK/.config/invowk/config.cue` is ignored;
  - `--ivk-config` beats the env;
  - `env INVOWK_CONFIG=` behaves as unset;
  - `env INVOWK_CONFIG=config.cue` fails (`! exec`, stderr names `INVOWK_CONFIG` and "absolute"), and the script does not run;
  - a missing absolute env file fails, stderr names the path and `INVOWK_CONFIG`;
  - `config path` shows `Effective config file:` with `INVOWK_CONFIG`, `[missing]` and `[invalid: must be absolute]` cases, exit 0;
  - `config set default_runtime virtual-sh` fails under the env and under `--ivk-config`, and `exists`/`! exists` show the user file was not created;
  - `config show` and `validate` with a missing explicit file exit 1;
  - `audit .` with a missing absolute `INVOWK_CONFIG` exits 2 (audit error), not 1;
  - `__complete cmd ''` with a missing explicit file offers no cwd commands.
- [x] 5.3 Update `config_override_flag.txtar` Test 4 (`:24-28`): `! exec`, stderr names `nonexistent.cue` and `--ivk-config`, no `RUNTIME=` on stdout. Add an invalid-CUE case.
- [x] 5.4 Mirrored pair `virtual_discovery_no_cwd.txtar` / `native_discovery_no_cwd.txtar` (decision 5). Same script section and identical `exec invowk …` lines; only the CUE implementations differ (virtual-sh vs platform-split native). Cover:
  - `--ivk-no-cwd` before and after `cmd`, and `env INVOWK_NO_CWD=1`, hide the cwd invowkfile and module commands;
  - a `$WORK/.invowk/cmds/g.invowkmod` command still runs (`HOME=$WORK`);
  - `cmd @invowkfile local` fails;
  - `--ivk-no-cwd=false` overrides the env;
  - `env INVOWK_NO_CWD=yes` makes `cmd` fail, naming the variable;
  - `cmd local -- --ivk-no-cwd` passes the token to the command;
  - `--ivk-verbose --ivk-no-cwd cmd` prints `current-directory sources disabled (--ivk-no-cwd)`, and the non-verbose listing does not.
- [x] 5.5 `no_cwd_builtins.txtar` (runtime-independent, no mirror needed):
  - `validate` with no arg does not report an invalid cwd file and prints `current-directory sources disabled`, while `validate ./invowkfile.cue` still reports it;
  - `module list` omits a cwd module and lists a `$WORK/.invowk/cmds` one;
  - `env INVOWK_NO_CWD=yes` then `config path` and `--version` succeed;
  - `__complete --ivk-no-cwd cmd ''` omits cwd commands;
  - `audit .` output is unchanged by `--ivk-no-cwd`.
- [x] 5.6 Check: `flock <heavy.lock> make test-cli` passes, including `TestShRuntimeMirrorCoverage` and `TestVirtualNativeCommandPathAlignment`.

## 6. Documentation, i18n, diagrams, agent docs

- [ ] 6.1 `website/docs/configuration/overview.mdx`: replace line 21 with the precedence list (flag > `INVOWK_CONFIG` > user file > defaults), the absolute-path rule, fail-closed behaviour with exit 1, the `config set` refusal, and an explicit "the current directory is never read" note. Add a short `--ivk-no-cwd` / `INVOWK_NO_CWD` section that recommends the env forms for wrappers whose commands call `invowk` again, and a `:::warning` migration admonition.
- [ ] 6.2 Mirror 6.1 in `website/i18n/pt-BR/docusaurus-plugin-content-docs/current/configuration/overview.mdx`.
- [ ] 6.3 `website/docs/reference/cli.mdx` Global Flags (`:11-21`): add a `--ivk-no-cwd` row, update the `--ivk-config` row (`:18`), and add an "Environment variables" subsection (`INVOWK_CONFIG`, `INVOWK_NO_CWD`). Mirror it in pt-BR `reference/cli.mdx`.
- [ ] 6.4 Note that cwd sources (1–2) are skipped under `--ivk-no-cwd` in `website/docs/configuration/options.mdx:52-53`, `reference/config-schema.mdx:72-73`, `core-concepts/commands-and-namespaces.mdx:67-68`, `modules/overview.mdx:42,117-118` and their pt-BR copies.
- [ ] 6.5 README:
  - `## Configuration` (`README.md:2506-2512`): precedence, `INVOWK_CONFIG`, no cwd config, fail-closed, `config set` refusal.
  - The flags examples (`:2685-2692`): `--ivk-no-cwd`.
  - The discovery order lists (`:22-23`, `:485`, `:2068-2069`): cwd sources are skipped under `--ivk-no-cwd`.
- [ ] 6.6 Snippets: `website/src/components/Snippet/data/config.ts:35-36` already shows `--ivk-config … config show` and stays; add an `INVOWK_CONFIG` example there only if 6.1/6.3 use a snippet. Do not edit `versions/*` or `versioned_docs`.
- [ ] 6.7 Diagrams:
  - `docs/diagrams/c4/container.d2:419-428` config_file tooltip: flag/env/user path, no cwd.
  - `docs/diagrams/flowcharts/discovery-flow.d2:28-31`: priorities 1–2 conditional on "no-cwd off".
  - `docs/diagrams/sequences/execution-main.d2:38-39`: config load can abort (explicit source, exit 1).
  - `docs/architecture/flowchart-discovery.md:23-25` and `c4-container.md:28,93` text.
- [ ] 6.8 `.agents/skills/discovery/SKILL.md` (precedence table and "Key Behavior", lines 31-58): `--ivk-no-cwd` and the `WithBaseDir("")` mechanism. Note config precedence in `.agents/skills/cli/SKILL.md` if it documents config loading.
- [ ] 6.9 Check: `make render-diagrams` (D2 edits), `make check-agent-docs` (`.agents/` changed) and `cd website && npm run build` pass.

## 7. Completion gates (.agents/rules/checklist.md)

Run in order; if any step changes code, restart from 7.1. Wrap heavy targets in the workflow's `flock` lock.

- [ ] 7.1 Review the changed code for reuse, simplification and efficiency (`/simplify`), and fix what it finds.
- [ ] 7.2 `make tidy` and `make license-check` (new Go files carry SPDX headers).
- [ ] 7.3 `make lint`. Fix every issue, including pre-existing ones.
- [ ] 7.4 `make test` (full suite; it already runs `make test-cli`).
- [ ] 7.5 `make check-baseline`. Triage every new goplint finding (new value types `ConfigSourceKind`, `SourceSelection`, `noCwdOrigin`, error types) per checklist step 5; ask the maintainer when both Invowk and goplint are plausible; never baseline an inconclusive.
- [ ] 7.6 `make check-file-length` (`internal/config/config.go`, `config_test.go`, `cmd/invowk/cmd.go`, `cmd_discovery.go`, `config.go` stay under 1000 lines).
- [ ] 7.7 `make sonar-local`. Fix real issues; suppress false positives in both `sonar-project.properties` and `.sonarcloud.properties`.
- [ ] 7.8 Conditional checks: `go test -run Sync ./pkg/invowkfile/ ./pkg/invowkmod/ ./internal/config/` (`internal/config` types change); `python3 scripts/formal.py correspondence` (bound directories `internal/runtime`, `internal/discovery` are touched or adjacent); `for m in samples/invowkmods/*.invowkmod; do go run . validate "$m"; done` (discovery behaviour); `cd website && npm run build`; `make check-agent-docs`.
- [ ] 7.9 Docs and diagrams match behaviour (checklist step 9): re-read 6.1–6.8 against the final code; `make render-diagrams` after any further D2 edit.
- [ ] 7.10 Runtime mirrors (checklist step 10): `virtual_discovery_no_cwd.txtar` has its `native_` mirror and no exemption was added.
- [ ] 7.11 `openspec validate harden-config-trust --strict` passes with 0 warnings.
- [ ] 7.12 `/learn` (`.agents/skills/learn/`).
