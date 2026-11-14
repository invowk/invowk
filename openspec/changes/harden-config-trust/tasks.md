## 0. Confirm open questions

- [ ] 0.1 Get maintainer answers to design.md "Open Questions" 1–6 (fail-closed `--ivk-config`, relative `INVOWK_CONFIG`, `validate`/`config` honouring the flag, nested propagation of the flag forms, mirror exemption, verbose no-cwd line). Update specs and design if any answer differs from the current decision before starting section 1.

## 1. Config provider: remove cwd fallback, add source kinds (internal/config)

- [ ] 1.1 Delete the cwd/`BaseDir` fallback branch in `loadWithOptions` (`internal/config/config.go:196-211`). Remove `LoadOptions.BaseDir` and its validation (`provider.go:23-26,81-85`). Fix the `LoadOptions.Validate` doc comment.
- [ ] 1.2 Add a `ConfigSourceKind` value type (`flag`, `env`, `user`, `default`) with `String()`/`Validate()`. Add a `Source` field to `LoadResult` and include it in `LoadResult.Validate()`.
- [ ] 1.3 Add an injectable `getenv` to `fileProvider`: `NewProvider()` uses `os.Getenv`, plus a constructor or option for tests. Implement `INVOWK_CONFIG` resolution per the design D2 table: empty means unset, a relative path is rejected with a new typed error (sentinel + `Unwrap`), and missing or invalid files are reported with the source kind attached to `FileNotFoundError`/`LoadError`.
- [ ] 1.4 Add `LoadOptions.UserConfigOnly`. It skips env resolution and is invalid together with a non-empty `ConfigFilePath`, so `LoadOptions.Validate` rejects that combination.
- [ ] 1.5 Unit tests (new `internal/config/config_source_test.go` if `config_test.go` would pass 1000 lines). Cover:
  - The precedence table: flag > env > user > defaults, with no merging.
  - Empty env behaves as unset.
  - Relative env is rejected.
  - Missing or invalid env and flag files return typed errors with the source kind.
  - A cwd `config.cue` is ignored. Use a non-parallel test that `t.Chdir`s into a temp dir containing `config.cue`, then assert defaults and `Source == default`.
  - `UserConfigOnly` ignores env.
  
  All tests use the injected `getenv`, never the process env.
- [ ] 1.6 Update or remove the fallback-dependent tests:
  - `config_test.go`: drop `BaseDir` at lines 223, 326, 365, 391, 421, 451, 485, 559, 578, 594, 618, 634, and replace "reports local fallback source" (`:607-626`) with "cwd config is not a source".
  - `config_mutation_test.go:107-155`: rewrite `TestConfigMutationLoadSourcePrecedence` around the new precedence.
  - `provider_test.go:22-110`: the `BaseDir` cases.
- [ ] 1.7 `go test ./internal/config/...` passes.

## 2. Fail-closed explicit sources (commandsvc / commandadapters / cmd)

- [ ] 2.1 Change `LoadConfigWithFallback` (`internal/app/commandsvc/config_fallback.go`) so that an error from an explicit source (flag or env) is returned as a fatal error instead of defaults plus a diagnostic. Keep the user-file behaviour.
- [ ] 2.2 Propagate the error through `DiscoveryService.loadConfig` and its cache (`internal/app/commandadapters/discovery.go:354-380`) and through `validateCommandTree`, `listCommands`, `executeRequest`, `completeCommands` and `loadUIConfig` (`cmd/invowk/cmd.go:375-383`, which today discards the diagnostics), so the invocation exits non-zero before discovery or execution. For pre-parse `registerDiscoveredCommands`, defer the error and report it from `RunE`.
- [ ] 2.3 Render an actionable error that names `--ivk-config` or `INVOWK_CONFIG` and the path. Reuse the `config_load_failed` issue template (`internal/issue`) and extend it with a hint for each source.
- [ ] 2.4 Update the Go tests that assert the old warning or defaults behaviour (`cmd/invowk/cmd_context_test.go:~456`, `cmd/invowk/cmd_execute_runtime_test.go:~91`, `internal/app/commandsvc` tests of `LoadConfigWithFallback`), and add fatal-path cases for flag and env.

## 3. Callers honour the precedence

- [ ] 3.1 Update `--ivk-config` usage text (`cmd/invowk/root.go:66`) to mention `INVOWK_CONFIG` and the OS-specific default.
- [ ] 3.2 `invowk validate`: pass `rootFlags` into `newValidateCommand` and use the root config path instead of `""` (`cmd/invowk/validate.go:106`).
- [ ] 3.3 `config` subcommands (`cmd/invowk/config.go`): pass `rootFlags`.
  - `show` and `dump` use the standard precedence, and `show` prints the source kind.
  - `path` adds an "Effective config file" line with its source (flag or env), without loading the file. A missing file or a relative env value is marked instead of failing.
  - `set` loads with `UserConfigOnly`, saves to the user file, and prints a note when an explicit source is active.
  - Fix the `config` Long help (`:52-57`) to list all three OS paths, the env var and the flag.
- [ ] 3.4 Check that `audit` (`cmd/invowk/audit.go:244,293`), `agent` (`agent.go:525,699`), `llm_flags.go:71-81` and `cmd_execute_helpers.go` reach the provider with an empty `ConfigFilePath` when no flag is given, so env resolution applies. Add a unit test for audit + `INVOWK_CONFIG`.
- [ ] 3.5 Unit tests for `config show`, `config path` and `config set` with an injected env, covering `config set` not copying an `INVOWK_CONFIG` file into the user file.

## 4. `--ivk-no-cwd` / `INVOWK_NO_CWD`

- [ ] 4.1 Add the root persistent flag `--ivk-no-cwd` (`root.go:65-67`) and `rootFlags.noCwd`. Resolve the effective value as the flag when `Changed`, otherwise `INVOWK_NO_CWD` via `strconv.ParseBool` (empty means false). An invalid value is a typed error that names the variable and the accepted values.
- [ ] 4.2 Add `ContextWithNoCwd`/`NoCwdFromContext` in `internal/app/commandadapters/discovery.go` (plus `cmd/invowk/app.go` wrappers). In `newDiscovery`, append `discovery.WithBaseDir("")` last when it is set. Make sure the request discovery cache cannot serve a cwd result to a no-cwd context.
- [ ] 4.3 Thread the switch into every discovery entry in `cmd/invowk`. Context setters: `cmd.go` RunE, `cmd_discovery.go:37,87,144,304`, `cmd_execute_helpers.go:229,314`, `cmd_watch.go:68`. Every `app.Discovery.*` call site: `cmd.go:376,398`, `cmd_discovery.go:38,305,363`, `internal_check_cmd.go:33`, `module.go:102` (`module list`), `validate.go:125`. Re-grep `app.Discovery\.` before closing the task.
- [ ] 4.4 Add `explicitNoCwdFromArgs` for the pre-parse registration step (`cmd.go:144-149`), handling `--ivk-no-cwd`, `--ivk-no-cwd=true|false`, a position before or after `cmd`, a stop at `--`, and the env fallback. Add table tests next to the existing `explicitConfigPathFromArgs` tests, plus a test that `firstTopLevelToken`/`shouldRegisterDiscoveredCommands` skip the flag.
- [ ] 4.5 `agent cmd create|change --verify`: return a clear error when no-cwd is on (`cmd/invowk/agent.go:~690-721`). Add a unit test.
- [ ] 4.6 `validate` workspace header: print "current-directory sources disabled (--ivk-no-cwd)" when it is on.
- [ ] 4.7 Unit tests in `cmd/invowk` and `commandadapters`:
  - cwd invowkfile and modules are skipped, while includes, the user dir and provisioned entries are kept.
  - `@invowkfile` gives source-not-found.
  - The explicit `--ivk-no-cwd=false` flag beats the env.
  - An invalid env value is rejected.
  - Cache isolation.
- [ ] 4.8 Confirm (test or code comment) that `INVOWK_CONFIG` and `INVOWK_NO_CWD` are intentionally absent from `shouldFilterEnvVar` (`internal/runtime/runtime.go:675`), per design D8.

## 5. CLI integration tests (tests/cli/testdata, inline CUE)

- [ ] 5.1 `config_cwd_ignored.txtar`: `$WORK/config.cue` sets `default_runtime: "virtual-sh"` and `includes` pointing at a `$WORK/inc/x.invowkmod`, with no user config. Assert:
  - `cmd` runs the native implementation.
  - The included module is not listed.
  - `config show` reports defaults.
  
  Put the fixture in a subdirectory, per the workspace-contamination rule.
- [ ] 5.2 `config_env_override.txtar`. Cover:
  - `INVOWK_CONFIG=$WORK/env/config.cue` takes effect.
  - A user config at `$HOME/.config/invowk/config.cue` (and the Windows `APPDATA` and macOS paths via platform conditions) is ignored when the env is set.
  - `--ivk-config` beats the env.
  - Empty `INVOWK_CONFIG` behaves as unset.
  - A relative `INVOWK_CONFIG` fails (`! exec`, stderr names the variable).
  - A missing env file fails and the script does not run.
  - `config path` shows the effective file.
- [ ] 5.3 Update `config_override_flag.txtar` Test 4 (`:24-28`): `! exec`, stderr names `nonexistent.cue`, no `RUNTIME=` on stdout. Add an invalid-CUE case.
- [ ] 5.4 `discovery_no_cwd.txtar`. Cover:
  - `--ivk-no-cwd` before and after `cmd`, and `INVOWK_NO_CWD=1`, hide the cwd invowkfile and module commands.
  - An include module and a `$HOME/.invowk/cmds` module still run.
  - `@invowkfile` fails.
  - `--ivk-no-cwd=false` overrides the env.
  - `INVOWK_NO_CWD=yes` fails.
  - `validate` with no arg does not report an invalid cwd file, while `validate ./invowkfile.cue` does.
  - `module list` omits a cwd module.
  - `INVOWK_NO_CWD=yes invowk config path` succeeds.
  - `__complete --ivk-no-cwd cmd ""` omits cwd commands.
  
  Use virtual-sh only, and register the file as a discovery mirror exemption (5.5).
- [ ] 5.5 Add `discovery_no_cwd.txtar` to the mirror-exemption table in `.agents/rules/testing.md` (discovery/diagnostics row) and to the exemption list in `cmd/invowk`/`tests/cli` mirror-coverage checks if one exists. Then run `make check-agent-docs`.
- [ ] 5.6 `make test-cli` passes.

## 6. Documentation, i18n, diagrams, agent docs

- [ ] 6.1 `website/docs/configuration/overview.mdx`: replace line 21 with the precedence list (flag > `INVOWK_CONFIG` > user file > defaults), the absolute-path rule, fail-closed behaviour and an explicit "the current directory is never read" note. Add a short `--ivk-no-cwd` / `INVOWK_NO_CWD` section and a `:::warning` migration admonition.
- [ ] 6.2 Mirror 6.1 in `website/i18n/pt-BR/docusaurus-plugin-content-docs/current/configuration/overview.mdx`.
- [ ] 6.3 `website/docs/reference/cli.mdx` Global Flags: add a `--ivk-no-cwd` row, update the `--ivk-config` row, and add an "Environment variables" subsection (`INVOWK_CONFIG`, `INVOWK_NO_CWD`). Mirror it in pt-BR `reference/cli.mdx`.
- [ ] 6.4 Note under discovery precedence that cwd sources (1–2) are skipped with `--ivk-no-cwd`, in `website/docs/configuration/options.mdx:~53`, `reference/config-schema.mdx:~73`, `core-concepts/commands-and-namespaces.mdx:~68`, `modules/overview.mdx:~118` and their pt-BR copies.
- [ ] 6.5 README:
  - `## Configuration` (`README.md:~2506-2512`): precedence, `INVOWK_CONFIG`, no cwd config.
  - The flags examples (`:~2685-2692`): `--ivk-no-cwd`.
  - The discovery order lists (`:~23`, `:~485`, `:~2069`): note that cwd sources are skipped under `--ivk-no-cwd`.
- [ ] 6.6 Update the current Snippet data (`website/src/components/Snippet/data/config.ts` or `reference` data) only if a snippet lists global flags or config locations. Do not edit `versions/*` or `versioned_docs`.
- [ ] 6.7 Diagrams:
  - `docs/diagrams/c4/container.d2` config_file tooltip: flag/env/user path.
  - `docs/diagrams/flowcharts/discovery-flow.d2`: priorities 1–2 conditional on "no-cwd off".
  - `docs/diagrams/sequences/execution-main.d2`: config load can abort.
  - `docs/architecture/flowchart-discovery.md:23` and `c4-container.md:93` text.
  
  Then run `make render-diagrams` to regenerate the renders.
- [ ] 6.8 `.agents/skills/discovery/SKILL.md`: source table note for `--ivk-no-cwd` and the `WithBaseDir("")` mechanism. Note config precedence in the cli skill if it documents config loading. Run `make check-agent-docs`.
- [ ] 6.9 Add the migration notes text from `proposal.md` to the place the release bundle collects BREAKING notes, as the maintainer instructs. Do not create a new top-level file without asking.
- [ ] 6.10 `cd website && npm run build` passes.

## 7. Verification gates

- [ ] 7.1 `/simplify` on the changed code. If it changes anything, restart from 7.2.
- [ ] 7.2 `make tidy` and `make license-check` (new Go files carry SPDX headers).
- [ ] 7.3 `make lint`. Fix every issue, including pre-existing ones.
- [ ] 7.4 `make test` (full, not short).
- [ ] 7.5 `make test-cli`.
- [ ] 7.6 `make check-baseline`. Triage any new goplint findings (new value types, env error type) with the maintainer, per checklist step 8.
- [ ] 7.7 `make check-file-length` (`internal/config/config.go`, `config_test.go`, `cmd/invowk/cmd.go` stay under 1000 lines).
- [ ] 7.8 `make check-agent-docs` (`.agents/` changed).
- [ ] 7.9 `make sonar-local`, and review any new issues.
- [ ] 7.10 `openspec validate harden-config-trust --strict` still passes after any spec edits from section 0.
