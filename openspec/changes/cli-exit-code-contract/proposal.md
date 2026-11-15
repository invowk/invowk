## Why

Invowk's exit status cannot tell a wrapper whether invowk rejected the command line or whether the work itself failed. A typo under a command group (`invowk cmd test nonexistent`) or a built-in group (`invowk config bogus`) exits **0** and prints help to stdout. Every other usage error (unknown flag or command, bad flag value, wrong argument count, unknown `@source`, ambiguous command) exits **1**, the same status as a config, discovery or dependency failure. `Execute` maps every error that is not an `ExitError` to 1 (`cmd/invowk/root.go:95-105`). `invowk agent` goes the other way and exits **2** on every failure, because it reuses audit's scan-error constant (`cmd/invowk/agent.go:72`).

CST and other callers that wrap invowk in systemd units, shell wrappers or CI need a documented contract. This change is C1 in the CST adoption track (`.claude/issues/cst-adoption-gaps.md`, Issue 1). It ships in the bundled BREAKING release (target `v0.17.0`).

## What Changes

- **BREAKING**: Invowk exits with status **2** whenever it rejects the command line before running any user code. The cases:
  - unknown flag, a flag value that does not parse, a missing flag value;
  - an invalid value for a reserved `--ivk-*` flag, including a malformed `--ivk-env-var` (no `=`, or an empty name), which used to be skipped with a warning and exit 0;
  - unknown root command (`invowk bogus`), also with `--help`, `-h` or `--version`;
  - an unmatched token under a discovered command group (`invowk cmd test nonexistent`) or a built-in group (`invowk config bogus`, `invowk module bogus`, `invowk tui bogus`, `invowk agent bogus`);
  - Cobra argument-arity errors on built-in commands (`invowk init a b c`, `invowk completion bogus`);
  - unknown command under `invowk cmd`, including in watch mode;
  - an ambiguous command name;
  - `@source` or `--ivk-from` naming a source that does not exist, or a malformed source name such as a path (`@./m.invowkmod`, `--ivk-from 1bad`), on every path including watch mode (design.md, Decision 13);
  - an invalid `--ivk-runtime` value, or a runtime the command does not allow;
  - declared-argument errors (`deps.ArgumentValidationError`) and invowkfile flag-validation errors (`deps.FlagValidationError`);
  - `invowk cmd @src` with no command name;
  - a conflict decided from flags and environment variables alone: an invalid `INVOWK_NO_CWD`, `--ivk-watch` with `--ivk-dry-run`, `invowk config set` while `--ivk-config` or `INVOWK_CONFIG` is active, and `invowk agent cmd create|change --verify` under `--ivk-no-cwd` (design.md, Decision 11).
- A command or source that is not found while discovery skipped an invowkfile or module that failed to parse or load keeps exit **1**, not 2: the command may be defined in the skipped file, and the contract classes invalid invowkfiles as 1 (design.md, Decision 14; pending maintainer confirmation).
- `--ivk-from @m` and `--ivk-from @invowkfile` on a discovered leaf now resolve like `--ivk-from m`; the leaf and watch paths passed the raw flag text and exited 1.
- **BREAKING**: Usage errors under a command group no longer print help to stdout with status 0. They print an `unknown command` error, `Did you mean` suggestions (Cobra `SuggestionsFor`) and the group's available subcommands, all on **stderr**, and exit 2. A bare group with no arguments (`invowk cmd test`, `invowk config`) still prints help to stdout and exits 0.
- **BREAKING**: `invowk agent` follows the same contract. Argument and flag errors (invalid name or module ID, mutually exclusive flags, missing description, invalid `--llm-*` value, unknown prompt format or operation) exit **2**. Every other agent failure (LLM detection or verification, a command or module that already exists or does not exist, `requires --force`, I/O, and a failed `--ivk-config` / `INVOWK_CONFIG` source) exits **1** instead of 2. `invowk audit` keeps 0 / 1 / 2.
- A typed CLI-layer `UsageError`, plus one pure function that maps an error to an exit code. `Execute` uses that function, so it can be unit-tested.
- Discovered-command registration is fixed so that a command which is also the prefix of a command from another source (leaf `w2` in the invowkfile, `w2 sub` in a module) is always registered as a runnable leaf. Today it is shadowed by a help-only group node, and `invowk cmd w2` prints help instead of running. Without this fix, the new group rule would turn `invowk cmd w2 x` into exit 2 for a command that exists. Shell-completion candidates do not change; only `w2`'s completion description becomes the leaf's own.
- The `invowk cmd` listing becomes deterministic: within each source, commands are listed alphabetically by full command name (sorted in `internal/discovery`). It used to follow map iteration order (maintainer decisions 10 and 12).
- An ambiguous nested name reached through a group node (`invowk cmd deploy prod`, ambiguous across sources, with a unique sibling `deploy dev`) renders the ambiguity message and exits 2 instead of printing help.
- A documented exit-code contract:

  | Code | Meaning |
  |---|---|
  | 0 | success |
  | N | the user command's own exit code, forwarded |
  | 2 | invowk rejected the command line |
  | 1 | invowk failed for another reason, including S1's fatal explicit-config error (`--ivk-config` or `INVOWK_CONFIG` naming a missing or invalid file) |

  The documentation lists the per-command exceptions (`audit`, `tui confirm`, `tui spin`, `validate`). It states that 2 can also be a script's own code, and that container-engine codes 125/126 are forwarded after retries.
- A testscript helper that asserts an exact exit code, because `! exec` passes for any non-zero status.
- Not in this change:
  - rejecting unmatched tokens after a leaf that also has children (`invowk cmd w typo` still runs `w` with `typo` as `$1`); deferred to C3 `command-contract-schema` (design.md, Decision 6);
  - `invowk help <unknown-topic>`, which still exits 0 (follow-up);
  - S1's explicit-config failure itself, which S1 implements; C1 only lists its status (1) and moves `invowk agent`'s code for it from 2 to 1.

## Migration notes

For the v0.17.0 release notes. Per the maintainer decision, each change keeps its own notes and the release assembles them.

> **Exit codes for usage errors changed (C1).** Invowk now exits with status `2` when it rejects the command line: unknown command, flag or `@source`; bad flag value; wrong number of arguments; ambiguous command name; or a runtime the command does not allow. These used to exit `1`. Typos under a command group used to print help and exit `0`. They now print an error with suggestions to stderr and exit `2`.
>
> Status `1` now means only that invowk failed for another reason, such as config, discovery, dependency checks, an unsupported platform or a missing container engine. A non-zero status from your own command is still forwarded unchanged.
>
> What to update:
> - Scripts that test `$? -eq 1` to detect a mistyped invocation should test for `2`, or for any non-zero status.
> - Scripts that ran `invowk cmd <group> <word>` and expected help on stdout should run `invowk cmd <group>` (still status 0) or `invowk cmd <group> --help`.
> - A malformed `--ivk-env-var` value (`NAME` without `=`, or `=value` with no name) is now an error with status `2`. It used to be skipped with a warning, so the variable was silently never set. Write `NAME=value`; `NAME=` still sets an empty value.
> - `invowk agent` used to exit `2` on every failure. It now exits `2` only for argument and flag errors, and `1` for every other failure, such as LLM errors or a command or module that already exists. Scripts that test `$? -eq 2` after `invowk agent …` to detect any failure should test for any non-zero status.
> - Your own commands can also exit with `2`. If you must tell the two apart, read stderr, or give your scripts codes other than 1 and 2.
> - `invowk agent` render errors (an unknown `--format` or prompt operation) exit `2`.
> - A failed `--ivk-config` or `INVOWK_CONFIG` source now makes `invowk agent` exit `1` like every other command, not `2`. `invowk audit` keeps `2` for it.
> - Other invalid reserved `--ivk-*` values also exit `2` (they exited `1`): an unknown `--ivk-runtime` or `--ivk-env-inherit-mode`, an invalid `--ivk-env-inherit-allow`/`--ivk-env-inherit-deny` name, a whitespace-only `--ivk-workdir`, and an empty `--ivk-env-file=` (which used to exit `0`).
> - An invalid `--ivk-container-name` exits `2` with an `invalid container name` error instead of `1` with `Invalid request: 1 field error(s)`.
> - Flag and environment conflicts exit `2`: an invalid `INVOWK_NO_CWD` value, `--ivk-watch` with `--ivk-dry-run` (it exited `1`), `invowk config set` while `--ivk-config` or `INVOWK_CONFIG` is active, and `invowk agent cmd create`/`change --verify` under `--ivk-no-cwd`.
> - Shell completion: `invowk __complete cmd ""` now shows a prefix leaf's own description (for example `w2` when a module also defines `w2 sub`) instead of `Commands under 'w2'`, because that leaf is now registered as a runnable command.
> - The `invowk cmd` listing now lists each source's commands alphabetically by full command name. It used to follow an unspecified order that could change between runs. Scripts that parse the listing should not rely on declaration order.
> - An unknown root command exits `2` also when `--help`, `-h` or `--version` follows it (`invowk bogus --help`); it exited `1`.
> - A malformed `@source` or `--ivk-from` value, such as a path (`@./m.invowkmod`) or `1bad`, exits `2` with an `invalid source` error instead of `1` with `Invalid request: 1 field error(s)`. Name a source as `invowkfile` or a module name (`m` or `m.invowkmod`).
> - `--ivk-from @<source>` after a leaf command (`invowk cmd w2 sub --ivk-from @m`) now works; it exited `1`.
> - A command that cannot be found because an invowkfile or module failed to parse or load still exits `1` (the invowkfile is invalid), not `2`.
> - `invowk audit` keeps `0` = clean, `1` = findings, `2` = scan error. Its argument errors now also exit `2` instead of `1`.

## Capabilities

### New Capabilities
- `cli-exit-codes`: the process exit-code contract of the `invowk` binary. It covers success, forwarding a user command's code, rejecting the command line (2), other invowk failures (1), per-command exceptions, where usage-error output goes (stdout or stderr) for command groups, and the registration rules that keep existing commands reachable.

### Modified Capabilities
- `config-source-resolution` (from S1, `harden-config-trust`): "Explicit config failures outside the exit-1 flows" now has `invowk agent` exit 1 for a failed explicit config source, per the general contract; `audit` keeps 2.

## Impact

- **Code** (`cmd/invowk/`, plus one sort in `internal/discovery`):
  - `root.go`: two edits only, the body of `Execute` and one `installUsageGuards(rootCmd)` call in `NewRootCommand`, so the S1 rebase stays simple.
  - New `usage_error.go`: `UsageError`, `newUsageError`, `ExitCodeUsage`, `exitCodeForError`, `installUsageGuards` (root flag-error func and `RunE`, built-in group guards), `classifyUnknownRootCommand`, `notFoundError` and the shared group-rejection helper.
  - `internal/agentcmd/module.go`: `ValidateModuleTarget`, the argv-only module-target check that `resolveModulePath` also uses, so `agent mod remove`/`change` reject a malformed ID with 2 before any LLM detection.
  - `cmd_discovery.go`: group nodes, plus the registration-order fix.
  - `app.go` (`renderAndWrapServiceError`), `cmd.go`, `cmd_execute_helpers.go` (reserved-flag validation, `parseEnvVarFlags`) and `cmd_watch.go`: wrap the usage-class errors.
  - `agent.go` and `llm_flags.go`: per-site exit codes; the shared LLM helpers take the caller's codes so `audit.go` keeps 2.
  - `cmd_watch.go` (watch/dry-run conflict), `discovery_scope.go` (invalid `INVOWK_NO_CWD`) and `config.go` (`config set` refusal): wrapped as usage errors.
  - `internal/discovery/discovery_commands.go`: `DiscoveredCommandSet.Analyze` sorts each source's commands by full name.
- **Tests**:
  - Go unit tests for the mapper, the guards, group and registration behaviour, reserved flags, and agent codes.
  - A new exact-exit-code command in the `tests/cli` testscript harness.
  - New txtar cases, with virtual/native mirrors for user-command cases. Existing txtar cases that relied on group help with extra tokens, on a skipped malformed `--ivk-env-var`, or on agent exit 2 need triage.
- **Docs**:
  - `website/docs/reference/cli.mdx` (Exit Codes, `:724-730`, and the audit line at `:568`), `website/docs/security/audit.mdx:184` and `website/docs/advanced/llm-assisted-authoring.mdx`, plus their pt-BR copies under `website/i18n/pt-BR/docusaurus-plugin-content-docs/current/`.
  - `README.md`: a new general "Exit codes" section and the audit "Exit Codes" table (`README.md:3024`).
  - The CLI skill (`.agents/skills/cli/SKILL.md`, "Error Handling & Exit Codes").
  - `website/versioned_docs` is not edited.
- **Ordering**: S1 (`harden-config-trust`, `f4969e90`) merged first; this branch is rebased onto it. Line citations in design.md's Context tables are against `c0a000da`, where they were observed.
- **Completion and listing**: shell-completion candidates must not change (`completion.txtar`, `__complete` paths); the `invowk cmd` listing order changes to alphabetical within each source (`virtual_multi_source.txtar` / `native_multi_source.txtar`, Test 6).
