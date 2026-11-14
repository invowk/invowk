## Why

Invowk's exit status cannot tell a wrapper whether invowk rejected the command line or whether the work itself failed. A typo under a command group (`invowk cmd test nonexistent`) or a built-in group (`invowk config bogus`) exits **0** and prints help to stdout. Every other usage error (unknown flag or command, bad flag value, wrong argument count, unknown `@source`, ambiguous command) exits **1**, the same status as a config, discovery or dependency failure. `Execute` maps every error that is not an `ExitError` to 1 (`cmd/invowk/root.go:95-105`). `invowk agent` goes the other way and exits **2** on every failure, because it reuses audit's scan-error constant (`cmd/invowk/agent.go:72`).

CST and other callers that wrap invowk in systemd units, shell wrappers or CI need a documented contract. This change is C1 in the CST adoption track (`.claude/issues/cst-adoption-gaps.md`, Issue 1). It ships in the bundled BREAKING release (target `v0.17.0`).

## What Changes

- **BREAKING**: Invowk exits with status **2** whenever it rejects the command line before running any user code. The cases:
  - unknown flag, a flag value that does not parse, a missing flag value;
  - an invalid value for a reserved `--ivk-*` flag, including a malformed `--ivk-env-var` (no `=`, or an empty name), which used to be skipped with a warning and exit 0;
  - unknown root command (`invowk bogus`);
  - an unmatched token under a discovered command group (`invowk cmd test nonexistent`) or a built-in group (`invowk config bogus`, `invowk module bogus`, `invowk tui bogus`, `invowk agent bogus`);
  - Cobra argument-arity errors on built-in commands (`invowk init a b c`, `invowk completion bogus`);
  - unknown command under `invowk cmd`, including in watch mode;
  - an ambiguous command name;
  - `@source` or `--ivk-from` naming a source that does not exist;
  - an invalid `--ivk-runtime` value, or a runtime the command does not allow;
  - declared-argument errors (`deps.ArgumentValidationError`) and invowkfile flag-validation errors (`deps.FlagValidationError`);
  - `invowk cmd @src` with no command name.
- **BREAKING**: Usage errors under a command group no longer print help to stdout with status 0. They print an `unknown command` error, `Did you mean` suggestions (Cobra `SuggestionsFor`) and the group's available subcommands, all on **stderr**, and exit 2. A bare group with no arguments (`invowk cmd test`, `invowk config`) still prints help to stdout and exits 0.
- **BREAKING**: `invowk agent` follows the same contract. Argument and flag errors (invalid name or module ID, mutually exclusive flags, missing description, invalid `--llm-*` value, unknown prompt format or operation) exit **2**. Every other agent failure (LLM detection or verification, a command or module that already exists or does not exist, `requires --force`, I/O) exits **1** instead of 2. `invowk audit` keeps 0 / 1 / 2.
- A typed CLI-layer `UsageError`, plus one pure function that maps an error to an exit code. `Execute` uses that function, so it can be unit-tested.
- Discovered-command registration is fixed so that a command which is also the prefix of a command from another source (leaf `w2` in the invowkfile, `w2 sub` in a module) is always registered as a runnable leaf. Today it is shadowed by a help-only group node, and `invowk cmd w2` prints help instead of running. Without this fix, the new group rule would turn `invowk cmd w2 x` into exit 2 for a command that exists. The `invowk cmd` listing order and shell-completion output do not change.
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
  - S1's explicit-config failure itself, which S1 implements; C1 only lists its status.

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
> - `invowk audit` keeps `0` = clean, `1` = findings, `2` = scan error. Its argument errors now also exit `2` instead of `1`.

## Capabilities

### New Capabilities
- `cli-exit-codes`: the process exit-code contract of the `invowk` binary. It covers success, forwarding a user command's code, rejecting the command line (2), other invowk failures (1), per-command exceptions, where usage-error output goes (stdout or stderr) for command groups, and the registration rules that keep existing commands reachable.

### Modified Capabilities
None. No existing spec in `openspec/specs/` states CLI exit-code requirements (checked 2026-11-14 on `c0a000da`).

## Impact

- **Code** (`cmd/invowk/` only; domain packages unchanged):
  - `root.go`: two edits only, the body of `Execute` and one `installUsageGuards(rootCmd)` call in `NewRootCommand`, so the S1 rebase stays simple.
  - New `usage_error.go`: `UsageError`, `newUsageError`, `ExitCodeUsage`, `exitCodeForError`, `installUsageGuards` (root flag-error func, root `Args` and `RunE`, built-in group guards) and the shared group-rejection helper.
  - `cmd_discovery.go`: group nodes, plus the registration-order fix.
  - `app.go` (`renderAndWrapServiceError`), `cmd.go`, `cmd_execute_helpers.go` (reserved-flag validation, `parseEnvVarFlags`) and `cmd_watch.go`: wrap the usage-class errors.
  - `agent.go` and `llm_flags.go`: per-site exit codes; the shared LLM helpers take the caller's codes so `audit.go` keeps 2.
- **Tests**:
  - Go unit tests for the mapper, the guards, group and registration behaviour, reserved flags, and agent codes.
  - A new exact-exit-code command in the `tests/cli` testscript harness.
  - New txtar cases, with virtual/native mirrors for user-command cases. Existing txtar cases that relied on group help with extra tokens, on a skipped malformed `--ivk-env-var`, or on agent exit 2 need triage.
- **Docs**:
  - `website/docs/reference/cli.mdx` (Exit Codes, `:724-730`, and the audit line at `:568`), `website/docs/security/audit.mdx:184` and `website/docs/advanced/llm-assisted-authoring.mdx`, plus their pt-BR copies under `website/i18n/pt-BR/docusaurus-plugin-content-docs/current/`.
  - `README.md`: a new general "Exit codes" section and the audit "Exit Codes" table (`README.md:3024`).
  - The CLI skill (`.agents/skills/cli/SKILL.md`, "Error Handling & Exit Codes").
  - `website/versioned_docs` is not edited.
- **Ordering**: S1 (`harden-config-trust`) merges first; this branch is then rebased onto it, because both edit `cmd/invowk/root.go`. Line citations are against main `c0a000da` and must be re-checked on rebase.
- **Completion and listing**: shell-completion output and the `invowk cmd` listing must not change (`completion.txtar`, `__complete` paths).
