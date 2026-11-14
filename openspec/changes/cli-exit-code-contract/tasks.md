## 1. Baseline and test harness

- [ ] 1.1 Rebase onto current main. If the base has moved past `85f1eb7b`, re-check every `file:line` citation in design.md, especially `cmd/invowk/root.go` and any custom fang error handler. Planned landing order is S1 → S2 → C2 → C1, but implementation does not wait for it.
- [ ] 1.2 Record before-state help output for `invowk --help`, `invowk config --help`, `invowk cmd --help` and `invowk --version`, for the help-rendering diff in 4.4.
- [ ] 1.3 Add an `exitcode <want> <cmd> <args…>` custom command to `testscript.Params.Cmds` in `tests/cli/cmd_test.go`, for both `TestCLI` and the container runner. Build it on `ts.Exec` (go-internal v1.16.0, `testscript/testscript.go:1211`), which runs in the script's cwd and environment and saves stdout/stderr for later `stdout`/`stderr` assertions. Read the status with `errors.As(err, *exec.ExitError)`. Reject `!` negation and fail when the status differs from `<want>`. Done when a throwaway txtar asserts `exitcode 0 invowk --version` and `exitcode 1 invowk cmd nope` on current code, then is removed.

## 2. Core types and mapping

- [ ] 2.1 Add `cmd/invowk/usage_error.go` with `UsageError` (`Err`, `Error()` delegating, `Unwrap()`), a constructor that refuses to wrap an `*ExitError`, and an `ExitCodeUsage types.ExitCode = 2` constant. Include the SPDX header.
- [ ] 2.2 Add a pure `exitCodeForError(err error) types.ExitCode` (nil → 0, `ExitError` → its code, `UsageError` → 2, otherwise 1). Make `Execute` (`root.go:95-105`) call `os.Exit(int(exitCodeForError(err)))`.
- [ ] 2.3 Unit tests: a table for `exitCodeForError` (nil, `ExitError` 0/2/42, `UsageError`, `UsageError{ServiceError}`, wrapped plain error) and the constructor invariant.

## 3. Usage-error sources

- [ ] 3.1 Root `SetFlagErrorFunc` that wraps pflag errors in `UsageError`. Unit test: an unknown flag on a nested child resolves to 2 through `exitCodeForError`.
- [ ] 3.2 Root `Args` validator that returns `UsageError` with Cobra's exact `unknown command %q for %q` text and suggestions, plus a help `RunE`. Unit test: `invowk bogus` gives 2 with the text intact, and bare root still renders help with a nil error.
- [ ] 3.3 `installUsageGuards(root)` at the end of `NewRootCommand`. It sets the group guard as `Args` on every non-root command that has subcommands and `Args == nil`, adds a help `RunE` where the command is not runnable, and wraps non-nil `Args` validators. Before it runs, give `invowk cmd` (`cmd.go:72`) and every discovered group node an explicit `Args: cobra.ArbitraryArgs`. Unit test on a synthetic tree:
  - a group with no args gives help and nil;
  - a non-runnable group (`module`-like) and a runnable help-fallback group (`config`-like) each give `UsageError` for a token, with the subcommand list on stderr and empty stdout;
  - a `MaximumNArgs` error becomes `UsageError`;
  - a parent with explicit `ArbitraryArgs` is left alone.
- [ ] 3.4 Make `renderAndWrapServiceError` (`app.go:263-332`) return `&UsageError{Err: newServiceError(...)}` for `ArgumentValidationError`, `FlagValidationError`, `AmbiguousCommandError`, `SourceNotFoundError`, `RuntimeNotAllowedError`, and `ClassifiedError` with kinds `CommandNotFound`/`CommandAmbiguous`. Extend `cmd/invowk/cmd_execute_error_classifier_test.go`: each usage kind maps to 2, the other kinds map to 1, and `errors.AsType[*ServiceError]` still succeeds.
- [ ] 3.5 Wrap `errNoCommandSpecified` (all return sites), the invalid `--ivk-runtime` value error (`cmd_execute_helpers.go:113`), `WatchCommandNotFoundError` (`cmd_watch.go:87`) and the CLI `AmbiguousCommandError` returned from group nodes. `cmd_watch_test.go:147` (`errors.Is`) must still pass.

## 4. Discovered group nodes and registration

- [ ] 4.1 Register in `SimpleName` order inside `registerDiscoveredCommands` (`cmd_discovery.go:55`). Unit test: invowkfile leaf `w2` plus module `w2 sub` builds a runnable `w2` with child `sub`, for either source order.
- [ ] 4.2 Rewrite the group `RunE` (`cmd_discovery.go:85-111`) in the Decision 3 order: no args gives help; then `--ivk-from`; then `checkAmbiguousCommand(prefix+args)` on every call; then subcommands on stderr plus `UsageError` with `SuggestionsFor`. Unit tests for each branch, including an ambiguous `deploy prod` reached through group `deploy`, where a unique sibling `deploy dev` keeps the group node registered.
- [ ] 4.3 Leave the leaf `Args: cobra.ArbitraryArgs` (`cmd_discovery.go:178`) unchanged, and add a unit test that pins `w typo` forwarding.
- [ ] 4.4 Diff help output against 1.2. Any change outside the intended ones is a bug.

## 5. CLI integration tests (txtar)

- [ ] 5.1 Built-in cases (exempt from mirrors), in a new `tests/cli/testdata/exit_codes_builtin.txtar` using `exitcode`:
  - 2: `invowk --bogus`, `invowk bogus`, `invowk config bogus`, `invowk module bogus`, `invowk tui bogus`, `invowk init a b c`, `invowk completion bogus`, `invowk audit a b c`;
  - 0: `invowk module`, `invowk config`;
  - 1: `invowk audit` with findings.

  Assert stdout is empty for the group typos.
- [ ] 5.2 `virtual_exit_codes.txtar` plus the `native_exit_codes.txtar` mirror, with platform-split CUE (bash, PowerShell):
  - 2: group typo with a suggestion and an empty stdout; unknown leaf flag; bad typed-flag value; missing and extra declared args; `cmd nope`; `cmd @bogus x`; `--ivk-runtime bogus`; runtime not allowed; `--ivk-watch nope`; script `exit 2`;
  - 0: bare group, which shows help on stdout;
  - 42: script `exit 42`;
  - 1: unsatisfied tool dependency;
  - pinned: `w typo` runs `w`.
- [ ] 5.3 Add `exitcode 2` assertions for the ambiguous cases to `virtual_ambiguity.txtar` and `native_ambiguity.txtar`. Cover the top-level ambiguous name and a nested ambiguous name `deploy prod` reached through group `deploy`, with a unique sibling `deploy dev` present. Add the cross-source prefix-leaf case to `virtual_multi_source.txtar` and `native_multi_source.txtar`.
- [ ] 5.4 Add exact codes 1 and 42 to `container_exitcode.txtar` (container tests are exempt from mirrors).
- [ ] 5.5 Completion: add `__complete config ""` and `__complete cmd test ""` checks to `completion.txtar` that expect the subcommand lists and status 0. The existing assertions must stay unchanged.
- [ ] 5.6 Run `make test-cli` (it runs with `-race`) and triage every failure. Look especially at `exec invowk cmd <group> <token>` lines that expected success, and at `virtual_args`/`native_args`, `virtual_disambiguation`/`native_disambiguation`, `virtual_multi_source_full`/`native_multi_source_full`, `version_help`, `agent_cmd_create` and `dogfooding_invowkfile`. Fix a test only when the spec says the old expectation was wrong.

## 6. Documentation

- [ ] 6.1 Replace the "Exit Codes" section of `website/docs/reference/cli.mdx` (`:724-730`) with the contract table, the per-command exceptions, the collision note and the 125/126 note. Update `:568` if its wording changes.
- [ ] 6.2 Change `website/docs/security/audit.mdx:184` so that argument errors exit 2 (they used to exit 1), and align the README audit "Exit Codes" table (`README.md:3024`).
- [ ] 6.3 Add a general "Exit codes" subsection to `README.md` with the same contract.
- [ ] 6.4 Mirror 6.1 and 6.2 in `website/i18n/pt-BR/docusaurus-plugin-content-docs/current/reference/cli.mdx` and `.../security/audit.mdx`. Do not edit `website/versioned_docs`.
- [ ] 6.5 Add one sentence to `website/docs/flags-and-arguments/positional-arguments.mdx`, and its pt-BR copy, saying that a group typo exits 2 while tokens after a leaf are forwarded.
- [ ] 6.6 Check `website/src/components/Snippet/data/cli.ts` for any snippet that shows exit statuses or group help, and update it if needed.
- [ ] 6.7 Update `.agents/skills/cli/SKILL.md` "Error Handling & Exit Codes" to cover `UsageError`, `exitCodeForError` and the contract, then run `make check-agent-docs`.
- [ ] 6.8 Check `docs/diagrams/sequences/execution-main.d2` ("Output + Exit code", `:92`) and `website/docs/architecture/execution-flow.mdx`. Change them only if they state specific codes, and run `make render-diagrams` if a D2 source changes.

## 7. Verification gates

- [ ] 7.1 `make tidy` and `make license-check`.
- [ ] 7.2 `make lint`, fixing every finding.
- [ ] 7.3 `make check-baseline` (triage any new goplint finding with the maintainer) and `make check-file-length`.
- [ ] 7.4 `make test` (full suite, not short).
- [ ] 7.5 `make test-cli`.
- [ ] 7.6 `cd website && npm run build`.
- [ ] 7.7 `openspec validate cli-exit-code-contract --strict`.
